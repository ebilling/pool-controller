package main

import (
	"fmt"
	"testing"
	"time"
)

func TestGpioThermometer(t *testing.T) {
	LogTestMode()
	sleeptime := 100 * time.Millisecond
	pin := TestPin{
		state:     Low,
		direction: Input,
		sleepTime: sleeptime,
		inputTime: time.Now(),
	}
	therm := newGpioThermometer("Test Thermometer", mftr, &pin)

	Debug("Therm: %v", therm)

	t.Run("getDischargeTime", func(t *testing.T) {
		d := therm.getDischargeTime() / time.Millisecond
		s := sleeptime / time.Millisecond // Two checks, so 2x
		if d < s-5 || d > s+5 {
			t.Errorf("Expected ~%dms got %dms", s, d)
		}
	})

	t.Run("getOhms", func(t *testing.T) {
		expected := 10000 * therm.adjust
		o := therm.getOhms(100 * time.Millisecond)
		if int(o) != int(expected) {
			t.Errorf("Expected %0.3f k-ohms found %0.3f k-ohms",
				float64(expected)/1000.0, o/1000.0)
		}
	})

	t.Run("getTemp", func(t *testing.T) {
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
		ms := time.Millisecond
		base := 60 * ms
		testTimes := []time.Duration{base, base - ms/10, base + ms/10, base, 2 * base,
			base, base + ms, base - ms, base + ms/5, base / 2}
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
				t.Errorf("i(%d) temp(%0.1f) old(%s) expected(%t) "+
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
		if !checkPinState(t, "TestPin", relay.pin, Output, High) {
			t.Errorf("")
		}
	})

	t.Run("TurnOn", func(t *testing.T) {
		relay.TurnOn()
		if !checkPinState(t, "TestPin", relay.pin, Output, Low) {
			t.Errorf("")
		}
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
		if !checkPinState(t, "TestPin", pin, Output, High) {
			t.Errorf("")
		}
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
	if !checkPinState(t, "Pump", pumps.pump.pin, Output, pumpState) {
		t.Errorf("")
	}
	if !checkPinState(t, "Sweep", pumps.sweep.pin, Output, sweepState) {
		t.Errorf("")
	}
	if !checkPinState(t, "Solar", pumps.solar.pin, Output, solarState) {
		t.Errorf("")
	}
	if !checkPinState(t, "SolarLED", pumps.solarLed, Output, !solarState) {
		t.Errorf("")
	}
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
	pumpPin := &TestPin{}
	sweepPin := &TestPin{}
	solarPin := &TestPin{}
	solarLedPin := &TestPin{}
	pumps := newSwitches(
		newRelay(pumpPin, "Test Pump", mftr),
		newRelay(sweepPin, "Test Sweep", mftr),
		newRelay(solarPin, "Test Solar", mftr),
		solarLedPin)

	startTime := pumps.GetStartTime()
	stopTime := pumps.GetStopTime()
	t.Run("Initialized", func(t *testing.T) {
		pumpTest(t, pumps, STATE_OFF, High, High, High,
			false, false, false, startTime, stopTime)
	})

	t.Run("StartPump", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_PUMP, false)
		pumpTest(t, pumps, STATE_PUMP, Low, High, High,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartSweep", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_SWEEP, false)
		pumpTest(t, pumps, STATE_SWEEP, Low, Low, High,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartPumpAfterSweep", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_PUMP, false)
		pumpTest(t, pumps, STATE_PUMP, Low, High, High,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartSolar", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_SOLAR, false)
		pumpTest(t, pumps, STATE_SOLAR, Low, High, Low,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartSolarMixing", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_SOLAR_MIXING, false)
		pumpTest(t, pumps, STATE_SOLAR_MIXING, Low, Low, Low,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartManualPump", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_PUMP, true)
		pumpTest(t, pumps, STATE_PUMP, Low, High, High,
			true, false, true, startTime, stopTime)
		pumps.SetState(STATE_SOLAR, false)
		pumpTest(t, pumps, STATE_PUMP, Low, High, High,
			true, false, true, startTime, stopTime)

	})

	t.Run("StartManualSweep", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_SWEEP, true)
		pumpTest(t, pumps, STATE_SWEEP, Low, Low, High,
			true, false, true, startTime, stopTime)
		pumps.SetState(STATE_SOLAR, false)
		pumpTest(t, pumps, STATE_SWEEP, Low, Low, High,
			true, false, true, startTime, stopTime)
	})

	t.Run("StopAllManual", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_OFF, true)
		pumpTest(t, pumps, STATE_OFF, High, High, High,
			false, true, true, startTime, stopTime)
		pumps.SetState(STATE_SOLAR_MIXING, false)
		pumpTest(t, pumps, STATE_OFF, High, High, High,
			false, true, true, startTime, stopTime)
	})

	t.Run("Disable", func(t *testing.T) {
		t.Run("Disabled", func(t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.Disable()
			pumpTest(t, pumps, STATE_DISABLED, High, High, High,
				false, true, true, startTime, stopTime)
		})

		t.Run("StartPump", func(t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.SetState(STATE_SOLAR_MIXING, false)
			pumpTest(t, pumps, STATE_DISABLED, High, High, High,
				false, false, true, startTime, stopTime)
		})

		t.Run("Disabled", func(t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.SetState(STATE_PUMP, true)
			pumpTest(t, pumps, STATE_DISABLED, High, High, High,
				false, false, true, startTime, stopTime)
		})
	})
	t.Run("Enable", func(t *testing.T) {
		t.Run("Disabled", func(t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.Disable()
			pumpTest(t, pumps, STATE_DISABLED, High, High, High,
				false, true, true, startTime, stopTime)
		})
		t.Run("Enabled", func(t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.Enable()
			pumpTest(t, pumps, STATE_OFF, High, High, High,
				false, true, true, startTime, stopTime)
		})
	})
}

func testpin_generator(gpio uint8) PiPin {
	return (PiPin)(&TestPin{
		sleepTime: 20 * time.Millisecond,
		pin:       gpio,
		wake:      make(chan bool),
	})
}

type TestPin struct {
	state     GpioState
	pull      Pull
	edge      Edge
	direction Direction
	sleepTime time.Duration
	inputTime time.Time
	wake      chan bool
	pin       uint8
}

func (p *TestPin) Input() {
	p.direction = Input
	p.inputTime = time.Now()
	p.pull = Float
	p.edge = NoEdge
}

func (p *TestPin) InputEdge(pull Pull, e Edge) {
	p.direction = Input
	p.inputTime = time.Now()
	p.pull = pull
	p.edge = e
}

func (p *TestPin) Output(s GpioState) {
	p.direction = Output
	p.state = s
}

func (p *TestPin) Read() GpioState {
	now := time.Now()
	sleeptime := p.inputTime.Add(p.sleepTime)
	if p.sleepTime > 0 && now.After(sleeptime) {
		p.state = High
	}
	return p.state
}

func (p *TestPin) WaitForEdge(interval time.Duration) bool {
	for true {
		select {
		case <-p.wake:
			return true
		case <-time.After(p.sleepTime):
			return true
		case <-time.After(interval):
			return false
		}
	}
	return true
}

func (p *TestPin) Pin() uint8 {
	return p.pin
}

func (p *TestPin) String() string {
	direction := "Input"
	if p.direction == Output {
		direction = "Output"
	}
	return fmt.Sprintf("TestPin: {State: %s, Direction: %s, Edge: %s, Pull: %s, Duration: %d, InputTime: %s}",
		p.state, direction, p.edge, p.pull, p.sleepTime, timeStr(p.inputTime))
}
