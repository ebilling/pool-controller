package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestSchedule(t *testing.T) {
	data, err := ioutil.ReadFile("/Users/eric/server.json")
	assert.Nil(t, err)
	cfg := &PersistedConfig{}
	err = json.Unmarshal(data, cfg)
	assert.Nil(t, err)
	cfg.Schedule = &Schedule{
		Events: []*ScheduleEvent{
			{time.Now(), 180, []time.Weekday{time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday}, PUMP},
		},
	}
	data, err = json.Marshal(cfg)
	assert.Nil(t, err)
	err = ioutil.WriteFile("/Users/eric/server.conf", data, 0600)
	assert.Nil(t, err)
}
