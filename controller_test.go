package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
)

func TestJSONmap(t *testing.T) {
	LogTestMode()
	m := NewJSONmap()
	swp_str := "timer.sweep.start"
	swp_val := "1:00"
	cap_str := "capacitance.gpio.25"
	cap_val := 9.37
	fmtString := "{ \"debug\": \"False\", \"timer\": { \"sweep\": %s " +
		"\"start\": \"%s\",\"stop\": \"2:30\" } }, \"capacitance\": {" +
		" \"gpio\": { \"24\": \"10.4\", \"25\": %f } } }"
	goodStr := fmt.Sprintf(fmtString, "{", swp_val, cap_val)
	badStr := fmt.Sprintf(fmtString, "[", swp_val, cap_val)

	t.Run("BadString", func(t *testing.T) {
		err := m.readString(badStr)
		if err == nil {
			t.Errorf("Should have caught an error")
		}
	})

	t.Run("ReadString", func(t *testing.T) {
		err := m.readString(goodStr)
		if err != nil {
			t.Errorf("Well formed JSON not parsing correctly: %s",
				err.Error())
		}
	})

	t.Run("Get.String", func(t *testing.T) {
		val := m.Get(swp_str)
		if val != swp_val {
			t.Errorf("Expected (%s) found (%s)", swp_val, val)
		}
	})

	t.Run("Get.Float", func(t *testing.T) {
		val := m.Get(cap_str)
		if val != cap_val {
			t.Errorf("Expected (%f) found (%f)", cap_val, val)
		}
	})

	filename := fmt.Sprintf("/tmp/json-test-%d.txt", rand.Uint32())
	ioutil.WriteFile(filename, []byte(goodStr), 0644)

	t.Run("Read", func(t *testing.T) {
		jmap := NewJSONmap()
		jmap.readFile(filename)

		t.Run("Get.String", func(t *testing.T) {
			if !jmap.Contains(swp_str) {
				t.Error("Contains failure")
			}
			val := jmap.Get(swp_str)
			if val != swp_val {
				t.Errorf("Expected (%s) found (%s)", swp_val, val)
			}
		})

		t.Run("Get.Float", func(t *testing.T) {
			val := jmap.Get(cap_str)
			if val != cap_val {
				t.Errorf("Expected (%f) found (%f)", cap_val, val)
			}
		})

	})
	os.Remove(filename)
}

func checkErr(t *testing.T, err error) {
	if err != nil {
		t.Errorf("Unexpected Error: %s", err.Error)
	}
}

func TestLog(t *testing.T) {
	t.Run("Alert", func(t *testing.T) {
		checkErr(t, Alert("testing %s", "alert"))
	})
	t.Run("Crit", func(t *testing.T) {
		checkErr(t, Crit("testing %s", "crit"))
	})
	t.Run("Emerg", func(t *testing.T) {
		checkErr(t, Emerg("testing %s", "emerg"))
	})
	t.Run("Error", func(t *testing.T) {
		checkErr(t, Error("testing %s", "error"))
	})
	t.Run("Notice", func(t *testing.T) {
		checkErr(t, Notice("testing %s", "notice"))
	})
	t.Run("Warn", func(t *testing.T) {
		checkErr(t, Warn("testing %s", "warn"))
	})
	t.Run("Info", func(t *testing.T) {
		checkErr(t, Info("testing %s", "info"))
	})
	t.Run("Log", func(t *testing.T) {
		checkErr(t, Log("testing %s", "log"))
	})
	t.Run("Trace", func(t *testing.T) {
		checkErr(t, Trace("testing %s", "trace"))
	})
}
