package main

import (
	"testing"
	"time"

	"github.com/ebilling/pool-controller/internal/rctime"
)

type fixedChargeEdge struct {
	duration time.Duration
}

func (p *fixedChargeEdge) OutputLow()                     {}
func (p *fixedChargeEdge) InputRisingFloat()              {}
func (p *fixedChargeEdge) WaitForEdge(time.Duration) bool { return true }
func (p *fixedChargeEdge) ChargeTime(time.Duration, time.Duration) (time.Duration, error) {
	return p.duration, nil
}

func TestGpioThermometer(t *testing.T) {
	sleeptime := 1 * time.Millisecond
	pin := TestPin{
		state:     Low,
		direction: Input,
		sleepTime: sleeptime,
		inputTime: time.Now(),
	}
	therm := newGpioThermometer("Test Thermometer", mftr, &pin)

	t.Run("getDischargeTime", func(t *testing.T) {
		d := therm.getDischargeTime() / time.Millisecond
		s := sleeptime / time.Millisecond // Two checks, so 2x
		if d < s-5 || d > s+5 {
			t.Errorf("Expected ~%dms got %dms", s, d)
		}
	})

	t.Run("getOhms", func(t *testing.T) {
		// ohms = adjust * microseconds / microfarads (100 nF in thermometer.go).
		// The old 10 µF test-rig comments were leftover from before the capacitor change.
		expected := therm.adjust * us(100*time.Millisecond) / therm.microfarads
		o := therm.getOhms(100 * time.Millisecond)
		if int(o) != int(expected) {
			t.Errorf("Expected %0.3f k-ohms found %0.3f k-ohms",
				float64(expected)/1000.0, o/1000.0)
		}
	})
}

func TestTemperatureDoesNotSampleGPIO(t *testing.T) {
	pin := TestPin{sleepTime: 50 * time.Millisecond}
	therm := newGpioThermometer("Test Thermometer", mftr, &pin)
	therm.accessory.TempSensor.CurrentTemperature.SetValue(21.5)
	therm.updated = time.Now().Add(-time.Hour)

	start := time.Now()
	if got := therm.Temperature(); got != 21.5 {
		t.Fatalf("Temperature() = %v, want 21.5", got)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Millisecond {
		t.Fatalf("Temperature performed GPIO I/O and took %s", elapsed)
	}
	if got := therm.updated; !got.Before(start) {
		t.Fatalf("Temperature unexpectedly refreshed reading at %s", got)
	}
}

func TestRoofThermometerClampsSustainedShortPulsesAsHot(t *testing.T) {
	therm := newGpioThermometer("Roof", mftr, &TestPin{})
	edge := &fixedChargeEdge{duration: rctime.MinTime / 2}
	therm.edge = edge
	therm.SetAdjustment(1.75)

	if err := therm.Update(); err == nil {
		t.Fatal("short pulses should remain an error until high clamping is enabled")
	}

	therm.ClampShortPulsesAsHot()
	before := time.Now()
	if err := therm.Update(); err != nil {
		t.Fatalf("sustained short pulses were not clamped: %v", err)
	}
	want := therm.getTemp(therm.getOhms(rctime.MinTime))
	if got := therm.Temperature(); got != want {
		t.Fatalf("clamped temperature = %v, want measurable boundary %v", got, want)
	}
	updated, err := therm.ReadingStatus()
	if err != nil || updated.Before(before) {
		t.Fatalf("clamped reading status = (%s, %v), want a fresh successful reading", updated, err)
	}

	edge.duration = 2 * rctime.MinTime
	if err := therm.Update(); err != nil {
		t.Fatalf("measurable pulses did not recover immediately: %v", err)
	}
	if got := therm.Temperature(); got >= want {
		t.Fatalf("recovered temperature = %v, want less than clamped %v", got, want)
	}
}

func TestCalibration(t *testing.T) {
	sleeptime := 50 * time.Millisecond
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
