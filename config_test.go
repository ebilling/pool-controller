package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"testing"
)

func flagTestSetup(args []string) *Config {
	flags := flag.NewFlagSet("ConfigTest"+args[0], flag.PanicOnError)
	config := NewConfig(flags, args)
	return config
}

func TestConfig(t *testing.T) {
	emptyArgs := []string{}

	flags := flag.NewFlagSet("ConfigTest", flag.PanicOnError)
	config := NewConfig(flags, emptyArgs)
	if !config.Authorized(defaultPin) {
		t.Error("Authorization failed")
	}
	if config.Authorized("bogus-password") {
		t.Error("Authorization should have failed")
	}
}

func TestConfig_forceRrd(t *testing.T) {
	c := flagTestSetup([]string{"-f"})
	if !*c.forceRrd {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_Persist(t *testing.T) {
	c := flagTestSetup([]string{"-p"})
	if *c.persist == false {
		t.Errorf("Default value was not overwritten")
	}
	if !*c.persist {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_SslCert(t *testing.T) {
	flag := "-ssl_cert"
	value := "This is my ssl cert path"
	c := flagTestSetup([]string{flag, value})
	if *c.sslCertificate == defaultSslCert {
		t.Errorf("Default value was not overwritten")
	}
	if *c.sslCertificate != value {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_SslKey(t *testing.T) {
	flag := "-ssl_key"
	value := "This is my ssl key path"
	c := flagTestSetup([]string{flag, value})
	if *c.sslPrivateKey == defaultSslKey {
		t.Errorf("Default value was not overwritten")
	}
	if *c.sslPrivateKey != value {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_DataDir(t *testing.T) {
	flag := "-data_dir"
	value := "This is my data_dir path"
	c := flagTestSetup([]string{flag, value})
	if *c.dataDirectory == defaultDataDir {
		t.Errorf("Default value was not overwritten")
	}
	if *c.dataDirectory != value {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_Simulate(t *testing.T) {
	c := flagTestSetup([]string{"-simulate", "-sim-pump-temp", "18", "-sim-roof-temp", "55", "-http_port", "8443"})
	if !*c.simulate {
		t.Errorf("simulate flag not set")
	}
	if *c.simPumpTemp != 18 {
		t.Errorf("sim-pump-temp: %v", *c.simPumpTemp)
	}
	if *c.simRoofTemp != 55 {
		t.Errorf("sim-roof-temp: %v", *c.simRoofTemp)
	}
	if *c.httpPort != 8443 {
		t.Errorf("http_port: %v", *c.httpPort)
	}
}

func TestConfig_GpioDriverDefault(t *testing.T) {
	c := NewConfig(flag.NewFlagSet("gpio-default", flag.PanicOnError), nil)
	if *c.gpioDriver != "cdev" {
		t.Errorf("gpio-driver default: %q, want cdev", *c.gpioDriver)
	}
	c = flagTestSetup([]string{"-gpio-driver", "periph"})
	if *c.gpioDriver != "periph" {
		t.Errorf("gpio-driver: %q, want periph", *c.gpioDriver)
	}
}

func TestConfig_Pidfile(t *testing.T) {
	flag := "-pid"
	value := "This is my Process ID path"
	c := flagTestSetup([]string{flag, value})
	if *c.pidfile == defaultPidFile {
		t.Errorf("Default value was not overwritten")
	}
	if *c.pidfile != value {
		t.Errorf("Flag value not persisted")
	}
}

func TestLegacyRunTimeMigratesToCleaningAndManualDefaults(t *testing.T) {
	oldName := serverConfiguration
	serverConfiguration = "/server.conf"
	defer func() { serverConfiguration = oldName }()

	dir := t.TempDir()
	if err := os.WriteFile(dir+serverConfiguration,
		[]byte(`{"RunTime":6,"DailyFrequency":2,"ThermostatMode":"auto"}`), 0600); err != nil {
		t.Fatal(err)
	}
	c := NewConfig(flag.NewFlagSet("legacy-cleaning", flag.PanicOnError),
		[]string{"-p", "-data_dir", dir})
	if c.cfg.RunTime != 2 {
		t.Fatalf("cleaning runtime=%v, want migrated two-hour requirement", c.cfg.RunTime)
	}
	if c.cfg.ManualRunTime != 6 {
		t.Fatalf("manual runtime=%v, want legacy six-hour timeout", c.cfg.ManualRunTime)
	}
	if c.cfg.CleaningConfigVersion != 1 {
		t.Fatalf("cleaning config version=%d, want 1", c.cfg.CleaningConfigVersion)
	}
}

func TestConfigSave(t *testing.T) {
	serverConfiguration = fmt.Sprintf("/test-server-%d.conf", rand.Uint32())
	testpin := "This-is-my-test-pin"
	args := []string{"-p", "-f", "-data_dir", "/tmp"}
	c := flagTestSetup(args)
	c.SetAuth("FakePassword")
	c.cfg.RoofAdjustment = 3.333
	c.cfg.Pin = testpin
	t.Run("SaveTest", func(t *testing.T) {
		err := c.Save()
		if err != nil {
			t.Error(err.Error())
		}
	})

	c = flagTestSetup([]string{"-p", "-data_dir", "/tmp"})
	t.Run("ReadTest", func(t *testing.T) {
		c.Save()
		if c.cfg.Pin != testpin {
			t.Errorf("Flag value not persisted")
		}
		if c.cfg.RoofAdjustment != 3.333 {
			t.Error("Roof adjustment not persisted")
		}
		if !c.Authorized("FakePassword") {
			t.Errorf("Auth was not persisted")
		}
	})

	if len(c.String()) < 100 {
		t.Error("Really just for coverage, but it should be at least 100 characters long...")
	}
	os.Remove("/tmp" + serverConfiguration) // Clean up the detritis

	c = flagTestSetup(args[1:])
	t.Run("NoSaveUnlessPersist", func(t *testing.T) {
		err := c.Save()
		if err != nil {
			t.Error("Expected no error")
		}
		_, err = os.Stat("/tmp" + serverConfiguration)
		if err == nil {
			t.Error("Should have returned a PathError")
		}
	})
}
