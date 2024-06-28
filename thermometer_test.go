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
		slop := s / 20
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
	sleeptime := time.Millisecond / 2
	pin := TestPin{
		state:     Low,
		direction: Input,
		sleepTime: sleeptime,
		inputTime: time.Now(),
	}
	therm := newGpioThermometer("Test Thermometer", mftr, &pin)

	t.Run("Calibrate", func(t *testing.T) {
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
		doDebug = true
		therm = newGpioThermometer("Test Thermometer", mftr, &pin)
		if testing.Short() {
			t.Skip("Skipping calibrate test")
		}
		slop := time.Millisecond / 25
		base := time.Millisecond

		// Seed the data
		for i := 0; i < 20; i++ {
			if i%2 == 0 {
				pin.sleepTime = base + slop
			} else {
				pin.sleepTime = base - slop
			}
			therm.Update()
		}
		// we have a skew in the data now, so we should get an error on larger swings
		t.Run("small swing", func(t *testing.T) {
			pin.sleepTime = base + slop
			updateTest(t, therm, true)
		})
		t.Run("large swing", func(t *testing.T) {
			pin.sleepTime = base + base/5
			updateTest(t, therm, false)
		})
	})
}

func updateTest(t *testing.T, therm *GpioThermometer, success bool) {
	err := therm.Update()
	if (err == nil) != success {
		t.Errorf("Error:  temp(%0.6f) expected(%t) med(%0.1f) avg(%0.1f) stdd(%0.1f)",
			therm.Temperature(), success,
			therm.history.Median()/float64(time.Millisecond),
			therm.history.Average()/float64(time.Millisecond),
			therm.history.Stddev()/float64(time.Millisecond))
	}
}
