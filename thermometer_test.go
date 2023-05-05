package main

import (
	"testing"
	"time"
)

func TestGpioThermometer(t *testing.T) {
	sleeptime := time.Millisecond
	pin := TestPin{
		state:     Low,
		direction: Input,
		sleepTime: sleeptime,
		inputTime: time.Now(),
	}
	therm := newGpioThermometer("Test Thermometer", mftr, &pin)

	t.Run("getDischargeTime", func(t *testing.T) {
		d := therm.getDischargeTime()
		s := sleeptime
		slop := s / 10
		if d < s-slop || d > s+slop {
			t.Errorf("Expected ~%s got %s", s, d)
		}
	})

	t.Run("getOhms", func(t *testing.T) {
		expected := 1000 * therm.adjust
		o := therm.getOhms(100 * time.Microsecond)
		if int(o) != int(expected) {
			t.Errorf("Expected %0.3f k-ohms found %0.3f k-ohms",
				float64(expected)/1000.0, o/1000.0)
		}
	})
}

func TestCalibration(t *testing.T) {
	sleeptime := 50 * time.Microsecond
	pin := TestPin{
		state:     Low,
		direction: Input,
		sleepTime: sleeptime,
		inputTime: time.Now(),
	}
	therm := newGpioThermometer("Test Thermometer", mftr, &pin)

	t.Run("Calibrate", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping calibrate test")
		}
		orig := therm.adjust
		err := therm.Calibrate(10000)
		if err != nil {
			t.Error("Unexpected error", err)
		}
		if therm.adjust == orig {
			t.Errorf("Adjustment should have changed after calibrate")
		}

		if therm.adjust < 1.8 || therm.adjust > 2.2 {
			t.Errorf("Expected ~2.0, found %0.3f", therm.adjust)
		}
	})

	t.Run("getTemp", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping calibrate test")
		}
		expected := [][]int{{105000, -20}, {25380, 5}, {9900, 25},
			{3601, 50}, {670, 100}}
		for _, val := range expected {
			th := therm.getTemp(float64(val[0]))
			if int(th) != val[1] {
				t.Errorf("Expected %d, found %0.1f", val[1], th)
			}
		}
	})

	t.Run("Update", func(t *testing.T) {
		expected := 12.1234
		therm.Update()
		therm.accessory.TempSensor.CurrentTemperature.SetValue(expected)
		if therm.Temperature() != expected {
			t.Errorf("Direct Set: Expected %f, found %f",
				expected, therm.Temperature())
		}

	})

	t.Run("Filters Bad Updates", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping calibrate test")
		}
		ms := time.Millisecond
		base := 60 * ms
		testTimes := []time.Duration{base, base - ms/10, base + ms/10, base, 2 * base,
			base, base + ms, base - ms, base + ms/5, base / 3}
		expected := []bool{true, true, true, true, false,
			true, true, true, true, false}
		// Seed the data
		for _, val := range testTimes {
			pin.sleepTime = val
			therm.Update()
		}
		// Try again, and big variances should be spotted
		for i, val := range testTimes {
			pin.sleepTime = val
			old := therm.updated
			therm.Update()
			if (therm.updated == old) == expected[i] {
				t.Errorf("Error: i(%d) temp(%0.1f) old(%s) expected(%t) "+
					"Current(%0.1f) med(%0.1f) avg(%0.1f) stdd(%0.1f)",
					i, therm.Temperature(), timeStr(old), expected[i],
					float64(pin.sleepTime)/float64(time.Millisecond),
					therm.history.Median()/float64(time.Millisecond),
					therm.history.Average()/float64(time.Millisecond),
					therm.history.Stddev()/float64(time.Millisecond))
			}
		}
	})
}
