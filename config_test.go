package main

import (
	"flag"
	"fmt"
	"golang.org/x/crypto/bcrypt"
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
	if bcrypt.CompareHashAndPassword(*config.auth, []byte(defaultPin)) != nil {
		t.Errorf("Default auth should be the default homekit pin")
	}
}

func TestConfig_forceRrd(t *testing.T) {
	c := flagTestSetup([]string{"-f"})
	if !*c.forceRrd {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_Disabled(t *testing.T) {
	c := flagTestSetup([]string{"-disabled"})
	if !*c.disabled {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_SolarDisabled(t *testing.T) {
	c := flagTestSetup([]string{"-solar_disabled"})
	if !*c.solarDisabled {
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

func TestConfig_Target(t *testing.T) {
	c := flagTestSetup([]string{"-target", "67.3"})
	if *c.target == defaultTarget {
		t.Errorf("Default value was not overwritten")
	}
	if *c.target != 67.3 {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_deltaT(t *testing.T) {
	c := flagTestSetup([]string{"-dt", "21.001"})
	if *c.deltaT == defaultDeltaT {
		t.Errorf("Default value was not overwritten")
	}
	if *c.deltaT != 21.001 {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_Tolerance(t *testing.T) {
	c := flagTestSetup([]string{"-tol", "2.001"})
	if *c.tolerance == defaultTolerance {
		t.Errorf("Default value was not overwritten")
	}
	if *c.tolerance != 2.001 {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_PumpAdjustment(t *testing.T) {
	c := flagTestSetup([]string{"-pump_adj", "2.222"})
	if *c.pumpAdjustment == defaultPumpAdjustment {
		t.Errorf("Default value was not overwritten")
	}
	if *c.pumpAdjustment != 2.222 {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_RoofAdjustment(t *testing.T) {
	c := flagTestSetup([]string{"-roof_adj", "3.333"})
	if *c.roofAdjustment == defaultRoofAdjustment {
		t.Errorf("Default value was not overwritten")
	}
	if *c.roofAdjustment != 3.333 {
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

func TestConfig_Pin(t *testing.T) {
	flag := "-pin"
	value := "This is my pin value"
	c := flagTestSetup([]string{flag, value})
	if *c.pin == defaultPin {
		t.Errorf("Default value was not overwritten")
	}
	if *c.pin != value {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_WUid(t *testing.T) {
	flag := "-wuid"
	value := "This is my Weather Underground ID"
	c := flagTestSetup([]string{flag, value})
	if *c.weatherUndergroundAppID == defaultWUAppID {
		t.Errorf("Default value was not overwritten")
	}
	if *c.weatherUndergroundAppID != value {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_Zip(t *testing.T) {
	flag := "-zip"
	value := "This is my zip code"
	c := flagTestSetup([]string{flag, value})
	if *c.zip == defaultZipcode {
		t.Errorf("Default value was not overwritten")
	}
	if *c.zip != value {
		t.Errorf("Flag value not persisted")
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

func TestConfigSave(t *testing.T) {
	random := fmt.Sprintf("/tmp/test-server-%d.conf", rand.Uint32())
	testpin := "This-is-my-test-pin"
	args := []string{"-p", "-f", "-data_dir", "/tmp", "-pin", testpin, "-pump_adj", "6.666", "-roof_adj", "3.333",
		"-zip", "95436", "-tol", "1.1", "-dt", "0.25", "-target", "33.001", "-wuid", "WU_IDN_WHAT?"}
	c := flagTestSetup(args)
	c.SetAuth("FakePassword")
	t.Run("SaveTest", func(t *testing.T) {
		err := c.Save(random)
		if err != nil {
			t.Error(err.Error())
		}
	})

	c = flagTestSetup([]string{"-p"})
	t.Run("ReadTest", func(t *testing.T) {
		c.OverwriteWithSaved(random)
		if *c.pin == defaultPin {
			t.Errorf("Default value was not overwritten")
		}
		if *c.pin != testpin {
			t.Errorf("Flag value not persisted")
		}
		if *c.roofAdjustment != 3.333 {
			t.Error("Roof adjustment not persisted")
		}
		if bcrypt.CompareHashAndPassword(*c.auth, []byte("FakePassword")) != nil {
			t.Errorf("Auth was not persisted")
		}
	})

	if len(c.String()) < 100 {
		t.Error("Really just for coverage, but it should be at least 100 characters long...")
	}
	os.Remove(random) // Clean up the detritis

	c = flagTestSetup(args[1:])
	t.Run("NoSaveUnlessPersist", func(t *testing.T) {
		err := c.Save(random)
		if err != nil {
			t.Error("Expected no error")
		}
		_, err = os.Stat(random)
		if err == nil {
			t.Error("Should have returned a PathError")
		}
	})

}
