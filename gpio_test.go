package main

import (
	"testing"
	"time"
)

func TestGpioThermometer(t *testing.T) {
	sleeptime := 100 * time.Millisecond
	pin := TestPin{
		state:        Low,
		direction:    Input,
		sleepTime:    sleeptime,
		inputTime:    time.Now(),
	}
	therm := newGpioThermometer("Test Thermometer", mftr, &pin, 10.0)

	Debug("States (H=%d, L=%d) Direction (I=%d, O=%d)", High, Low,
		Input, Output)
	Debug("Therm: %v", therm)

	t.Run("getDischargeTime", func (t *testing.T) {
		d := therm.getDischargeTime() / time.Millisecond
		s := sleeptime / time.Millisecond
		if d < s - 5 || d > s + 5 {
			t.Errorf("Expected ~%dms got %dms", s, d)
		}
	})

	t.Run("getOhms", func (t *testing.T) {
		expected := 20000
		o := therm.getOhms(100000000)
		if int(o) != expected {
			t.Errorf("Expected %d ohms found %d ohms",
				expected, int(o))
		}
	})

	t.Run("getTemp", func (t *testing.T) {
		expected := [][]int{{105000, -20}, {25380,5}, {9900,25},
			{3601,50}, {670,100}}
		for _, val := range expected {
			th := therm.getTemp(float64(val[0]))
			if int(th) != val[1] {
				t.Errorf("Expected %d, found %0.1f", val[1], th)
			}
		}
	})

	t.Run("Update", func (t *testing.T) {
		expected := 12.1234
		therm.Update()
		therm.accessory.TempSensor.CurrentTemperature.SetValue(expected)
		if therm.Temperature() != expected {
			t.Errorf("Direct Set: Expected %f, found %f",
				expected, therm.Temperature())
		}

	})

	t.Run("Filters Bad Updates", func (t *testing.T) {
		ms := time.Millisecond
		base := 60 * ms
		testTimes := []time.Duration{base, base-ms/10, base+ms/10, base, 2*base,
			base, base+ms, base-ms, base+ms/5, base/2}
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
				t.Errorf("i(%d) temp(%0.1f) old(%s) expected(%t) " +
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

func DirectionStr(d Direction) string {
	if d == Input {
		return "Input"
	}
	if d == Output {
		return "Output"
	}
	return "ERROR"
}

func StateStr(s GpioState) string {
	if s == High {
		return "High"
	}
	if s == Low {
		return "Low"
	}
	return "ERROR"
}

func checkPinState(t *testing.T, name string, pin PiPin, dir Direction, state GpioState) bool {
	tpin := pin.(*TestPin)
	if tpin.direction != dir {
		t.Errorf("%s Pin direction %s, expected %s", name,
			DirectionStr(tpin.direction), DirectionStr(dir))
		return false
	}
	if tpin.state != state {
		t.Errorf("%s Pin state %s, expected %s", name,
			StateStr(tpin.state), StateStr(state))
		return false
	}
	return true
}

func TestGpioRelay(t *testing.T) {
	pin := &TestPin{}
	relay := newRelay(pin, "Test Pin", mftr)
	start := relay.startTime
	stop := relay.stopTime

	t.Run("NewRelay", func(t *testing.T) {
		if !checkPinState(t, "TestPin", relay.pin, Output, High) { t.Errorf("") }
	})

	t.Run("TurnOn", func(t *testing.T) {
		relay.TurnOn()
		if !checkPinState(t, "TestPin", relay.pin, Output, Low) { t.Errorf("") }
		if !relay.GetStartTime().After(start) {
			t.Errorf("Start time not updated")
		}
		if !relay.GetStopTime().Equal(stop) {
			t.Errorf("Stop time should not have been updated")
		}
		if relay.Status() != "On" {
			t.Errorf("Status should have been On")
		}
	})

	start = relay.startTime
	t.Run("TurnOff", func(t *testing.T) {
		relay.TurnOff()
		if !checkPinState(t, "TestPin", pin, Output, High) { t.Errorf("") }
		if !relay.GetStartTime().Equal(start) {
			t.Errorf("Start time should not have been updated")
		}
		if !relay.GetStopTime().After(stop) {
			t.Errorf("Stop time not updated")
		}
		if relay.Status() != "Off" {
			t.Errorf("Status should have been Off")
		}
	})
}

func pumpTest(t *testing.T, pumps *Switches, state State,
	pumpState, sweepState, solarState GpioState,
	started, stopped, manual bool, startTime, stopTime time.Time) {
	if !checkPinState(t, "Pump", pumps.pump.pin, Output, pumpState) { t.Errorf("") }
	if !checkPinState(t, "Sweep", pumps.sweep.pin, Output, sweepState) { t.Errorf("") }
	if !checkPinState(t, "Solar", pumps.solar.pin, Output, solarState) { t.Errorf("") }
	if !checkPinState(t, "SolarLED", pumps.solarLed, Output, !solarState) { t.Errorf("") }
	if !pumps.pump.GetStartTime().Equal(pumps.GetStartTime()) {
		t.Errorf("Start time should be same as pump")
	}
	if started && true == pumps.GetStartTime().Equal(startTime) {
		t.Errorf("Start time should have updated")
	}
	if !started && false == pumps.GetStartTime().Equal(startTime) {
		t.Errorf("Start time should not have changed")
	}
	if stopped && true == pumps.GetStopTime().Equal(stopTime) {
		t.Errorf("Stop time should have updated")
	}
	if !stopped && false == pumps.GetStopTime().Equal(stopTime) {
		t.Errorf("Stop time should not have changed")
	}
	if pumps.State() != state {
		t.Errorf("Expected State %s, found %s",
			state.String(), pumps.State().String())
	}
	if pumps.ManualState() != manual {
		t.Errorf("Expected Manual %t found %t",
			manual, pumps.ManualState())
	}
}

func TestGpioSwitchesBasic(t *testing.T) {
	pumpPin     := &TestPin{}
	sweepPin    := &TestPin{}
	solarPin    := &TestPin{}
	solarLedPin := &TestPin{}
	pumps := newSwitches(
		newRelay(pumpPin, "Test Pump", mftr),
		newRelay(sweepPin, "Test Sweep", mftr),
		newRelay(solarPin, "Test Solar", mftr),
		solarLedPin)

	startTime := pumps.GetStartTime()
	stopTime := pumps.GetStopTime()
	t.Run("Initialized", func (t *testing.T) {
		pumpTest(t, pumps, STATE_OFF, High, High, High,
			false, false, false, startTime, stopTime)
	})

	t.Run("StartPump", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_PUMP, false)
		pumpTest(t, pumps, STATE_PUMP, Low, High, High,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartSweep", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_SWEEP, false)
		pumpTest(t, pumps, STATE_SWEEP, Low, Low, High,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartPumpAfterSweep", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_PUMP, false)
		pumpTest(t, pumps, STATE_PUMP, Low, High, High,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartSolar", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_SOLAR, false)
		pumpTest(t, pumps, STATE_SOLAR, Low, High, Low,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartSolarMixing", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_SOLAR_MIXING, false)
		pumpTest(t, pumps, STATE_SOLAR_MIXING, Low, Low, Low,
			true, false, false, startTime, stopTime)
	})

        t.Run("StartManualPump", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_PUMP, true)
		pumpTest(t, pumps, STATE_PUMP, Low, High, High,
			true, false, true, startTime, stopTime)
		pumps.SetState(STATE_SOLAR, false)
		pumpTest(t, pumps, STATE_PUMP, Low, High, High,
			true, false, true, startTime, stopTime)

	})

	t.Run("StartManualSweep", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_SWEEP, true)
		pumpTest(t, pumps, STATE_SWEEP, Low, Low, High,
			true, false, true, startTime, stopTime)
		pumps.SetState(STATE_SOLAR, false)
		pumpTest(t, pumps, STATE_SWEEP, Low, Low, High,
			true, false, true, startTime, stopTime)
	})

	t.Run("StopAllManual", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_OFF, true)
		pumpTest(t, pumps, STATE_OFF, High, High, High,
			false, true, true, startTime, stopTime)
		pumps.SetState(STATE_SOLAR_MIXING, false)
		pumpTest(t, pumps, STATE_OFF, High, High, High,
			false, true, true, startTime, stopTime)
	})

	t.Run("Disable", func (t *testing.T) {
		t.Run("Disabled", func (t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.Disable()
			pumpTest(t, pumps, STATE_DISABLED, High, High, High,
				false, true, true, startTime, stopTime)
		})

		t.Run("StartPump", func (t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.SetState(STATE_SOLAR_MIXING, false)
			pumpTest(t, pumps, STATE_DISABLED, High, High, High,
				false, false, true, startTime, stopTime)
		})

		t.Run("Disabled", func (t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.SetState(STATE_PUMP, true)
			pumpTest(t, pumps, STATE_DISABLED, High, High, High,
				false, false, true, startTime, stopTime)
		})
	})
	t.Run("Enable", func (t *testing.T) {
		t.Run("Disabled", func (t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.Disable()
			pumpTest(t, pumps, STATE_DISABLED, High, High, High,
				false, true, true, startTime, stopTime)
		})
		t.Run("Enabled", func (t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.Enable()
			pumpTest(t, pumps, STATE_OFF, High, High, High,
				false, true, true, startTime, stopTime)
		})
	})
}

func TestHistory(t *testing.T) {
	sizes := [...]int{11, 20, 50, 100, 200}
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
	var h History

	for _, sz := range sizes {
		h = *NewHistory(sz)
		if h.sz != sz {
			t.Errorf("Expected size=%d, found %d")
		}
		for _, f := range list {
			h.Push(f)
		}
	}

	t.Run("Average", func(t *testing.T) {
		avg := h.Average()
		if int32(avg*10.0) != 479 {
			t.Errorf("Average was %0.1f, expected 47.9", avg)
		}
	})
	t.Run("Median", func(t *testing.T) {
		med := h.Median()
		if int32(med*10.0) != 504 {
			t.Errorf("Median was %0.1f, expected 50.4", med)
		}
	})
	t.Run("Variance", func(t *testing.T) {
		variance := h.Variance()
		if int32(variance*10.0) != 8054 {
			t.Errorf("Variance was %0.1f, expected 805.4", variance)
		}
	})
}
