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
	if bcrypt.CompareHashAndPassword(*config.auth, []byte(default_pin)) != nil {
		t.Errorf("Default auth should be the default homekit pin")
	}
}

func TestConfig_forceRrd(t *testing.T) {
	c := flagTestSetup([]string{"-f"})
	if *c.forceRrd == default_forceRrd {
		t.Errorf("Default value was not overwritten")
	}
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

func TestConfig_Target(t *testing.T) {
	c := flagTestSetup([]string{"-target", "67.3"})
	if *c.target == default_target {
		t.Errorf("Default value was not overwritten")
	}
	if *c.target != 67.3 {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_deltaT(t *testing.T) {
	c := flagTestSetup([]string{"-dt", "21.001"})
	if *c.deltaT == default_deltaT {
		t.Errorf("Default value was not overwritten")
	}
	if *c.deltaT != 21.001 {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_Tolerance(t *testing.T) {
	c := flagTestSetup([]string{"-tol", "2.001"})
	if *c.tolerance == default_tolerance {
		t.Errorf("Default value was not overwritten")
	}
	if *c.tolerance != 2.001 {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_PumpAdjustment(t *testing.T) {
	c := flagTestSetup([]string{"-pump_adj", "2.222"})
	if *c.adj_pump == default_adj_pump {
		t.Errorf("Default value was not overwritten")
	}
	if *c.adj_pump != 2.222 {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_RoofAdjustment(t *testing.T) {
	c := flagTestSetup([]string{"-roof_adj", "3.333"})
	if *c.adj_roof == default_adj_roof {
		t.Errorf("Default value was not overwritten")
	}
	if *c.adj_roof != 3.333 {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_SslCert(t *testing.T) {
	flag := "-ssl_cert"
	value := "This is my ssl cert path"
	c := flagTestSetup([]string{flag, value})
	if *c.ssl_cert == default_ssl_cert {
		t.Errorf("Default value was not overwritten")
	}
	if *c.ssl_cert != value {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_SslKey(t *testing.T) {
	flag := "-ssl_key"
	value := "This is my ssl key path"
	c := flagTestSetup([]string{flag, value})
	if *c.ssl_key == default_ssl_key {
		t.Errorf("Default value was not overwritten")
	}
	if *c.ssl_key != value {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_DataDir(t *testing.T) {
	flag := "-data_dir"
	value := "This is my data_dir path"
	c := flagTestSetup([]string{flag, value})
	if *c.data_dir == default_data_dir {
		t.Errorf("Default value was not overwritten")
	}
	if *c.data_dir != value {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_Pin(t *testing.T) {
	flag := "-pin"
	value := "This is my pin value"
	c := flagTestSetup([]string{flag, value})
	if *c.pin == default_pin {
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
	if *c.WUappId == default_WUappId {
		t.Errorf("Default value was not overwritten")
	}
	if *c.WUappId != value {
		t.Errorf("Flag value not persisted")
	}
}

func TestConfig_Zip(t *testing.T) {
	flag := "-zip"
	value := "This is my zip code"
	c := flagTestSetup([]string{flag, value})
	if *c.zip == default_zip {
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
	if *c.pidfile == default_pidfile {
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
		if *c.pin == default_pin {
			t.Errorf("Default value was not overwritten")
		}
		if *c.pin != testpin {
			t.Errorf("Flag value not persisted")
		}
		if *c.adj_roof != 3.333 {
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
