package main

import (
	"fmt"
	"testing"
)

func TestJSONmap(t *testing.T) {
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
}

func checkErr(t *testing.T, err error) {
	if err != nil {
		t.Errorf("Unexpected Error: %s", err.Error)
	}
}

func TestLog(t *testing.T) {
	t.Run("Alert", func(t *testing.T) {
		checkErr(t, Alert("testing %s", "testval"))})
	t.Run("Crit", func(t *testing.T) {
		checkErr(t, Crit("testing %s", "testval"))})
	t.Run("Emerg", func(t *testing.T) {
		checkErr(t, Emerg("testing %s", "testval"))})
	t.Run("Error", func(t *testing.T) {
		checkErr(t, Error("testing %s", "testval"))})
	t.Run("Notice", func(t *testing.T) {
		checkErr(t, Notice("testing %s", "testval"))})
	t.Run("Warn", func(t *testing.T) {
		checkErr(t, Warn("testing %s", "testval"))})
	t.Run("Info", func(t *testing.T) {
		checkErr(t, Info("testing %s", "testval"))})
	t.Run("Log", func(t *testing.T) {
		checkErr(t, Log("testing %s", "testval"))})
	t.Run("Trace", func(t *testing.T) {
		checkErr(t, Trace("testing %s", "testval"))})
}

func TestStats(t *testing.T) {
	list := []float64{71.0,36.3,54.3,52.3,56.2,39.1,14.6,56.7,
		95.0,5.3,13.0,33.7,1.4,14.4,88.2,16.0,57.2,73.5,10.5,
		70.2,64.3,73.3,14.2,44.4,14.2,72.6,29.5,52.5,72.5,
		39.5,56.1,13.4,74.2,85.0,61.2,12.4,52.0,12.0,1.5,49.8,
		21.5,94.4,58.9,18.3,98.0,43.4,62.1,81.9,71.7,68.8,
		66.1,79.9,0.1,87.2,68.3,81.8,96.6,19.4,95.1,27.5,8.8,
		77.3,82.1,81.6,61.2,28.3,25.7,2.7,74.3,5.0,68.9,46.7,
		9.0,62.2,44.6,26.2,14.6,86.1,33.4,1.4,33.1,21.4,28.5,
		96.3,41.0,33.4,56.5,84.3,37.3,97.0,40.0,43.8,88.3,
		13.3,14.1,50.6,54.5,43.8,33.2,50.4}

	t.Run("Average", func(t *testing.T) {
		avg := Average(list)
		if int32(avg*10.0) != 479 {
			t.Errorf("Average was %0.1f, expected 47.9", avg)
		}
	})
	t.Run("Median", func(t *testing.T) {
		med := Median(list)
		if int32(med*10.0) != 504 {
			t.Errorf("Median was %0.1f, expected 50.4", med)
		}
	})
	t.Run("Variance", func(t *testing.T) {
		variance := Variance(list)
		if int32(variance*10.0) != 8054 {
			t.Errorf("Variance was %0.1f, expected 805.4", variance)
		}
	})
}
