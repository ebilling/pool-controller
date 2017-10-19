package main

import (
	"fmt"
	"testing"
	"time"
)

type Direction bool
const (
	Input Direction = false
	Output Direction = true
)

type TestPin struct {
	state        GpioState
	direction    Direction
	sleepTime    time.Duration
	inputTime    time.Time
}

func (p *TestPin) Input() {
	p.direction = Input
	p.inputTime = time.Now()
	Debug("Setting Fake pin to Input mode %v", p)
}

func (p *TestPin) Output() {
	p.direction = Output
	Debug("Setting Fake pin to Output mode %v", p)
}

func (p *TestPin) High() {
	p.state = High
	Debug("Setting Fake pin state to High %v", p)
}

func (p *TestPin) Low() {
	p.state = Low
	Debug("Setting Fake pin state to Low %v", p)
}

func (p *TestPin) Read() GpioState {
	now := time.Now()
	sleeptime := p.inputTime.Add(p.sleepTime)
	if p.sleepTime > 0 && now.After(sleeptime) {
		p.state = High
	}
	return p.state
}

func (p *TestPin) PullUp() {}

func (p *TestPin) PullDown() {}

func (p *TestPin) PullOff() {}

func (p *TestPin) String() string {
	state := "Low"
        if p.state == High {
		state = "High"
	}
	direction := "Input"
	if p.direction == Output {
		direction = "Output"
	}
	return fmt.Sprintf("TestPin: {State: %s, Direction: %s, Duration: %d, InputTime: %s}",
		state, direction, p.sleepTime, timeStr(p.inputTime))
}

func TestGpioThermometer(t *testing.T) {
	sleeptime := 100 * time.Millisecond
	pin := TestPin{
		state:        Low,
		direction:    Input,
		sleepTime:    sleeptime,
		inputTime:    time.Now(),
	}
	therm := NewGpioThermometer("Test Thermometer", mftr, 22, 10.0)
	therm.pin = &pin

	Debug("States (H=%d, L=%d) Direction (I=%d, O=%d)", High, Low,
		Input, Output)
	Debug("Therm: %v", therm)
	
	t.Run("getDischargeTime", func (t *testing.T) {
		d := therm.getDischargeTime()
		if d / 1000000 != 100 {
			t.Errorf("Expected %dms got %dms",
			sleeptime/1000000, d/1000000)
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

	t.Run("Update/Temperature", func (t *testing.T) {
		expected := 12
		therm.Update()
		therm.temperature = float64(expected)
		if int(therm.Temperature()) != expected {
			t.Errorf("Expected %d, found %0.1f",
				expected, therm.Temperature())
		}
		pin.sleepTime = 10100 * time.Microsecond
		therm.Update()
		expected = 65
		if int(therm.Temperature()) != expected {
			t.Errorf("Expected %d, found %0.1f",
				expected, therm.Temperature())
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

func checkPinState(t *testing.T, pin PiPin, dir Direction, state GpioState) {
	tpin := pin.(*TestPin)
	if tpin.direction != dir {
		t.Errorf("Pin direction %s, expected %s",
			DirectionStr(tpin.direction), DirectionStr(dir))
	}
	if tpin.state != state {
		t.Errorf("Pin state %s, expected %s",
			StateStr(tpin.state), StateStr(state))
	}
}

func TestGpioRelay(t *testing.T) {
	pin := &TestPin{}
	relay := newRelay(pin, "Test Pin", mftr)
	start := relay.startTime
	stop := relay.stopTime
	
	t.Run("NewRelay", func(t *testing.T) {
		checkPinState(t, relay.pin, Output, Low)
	})
	
	t.Run("TurnOn", func(t *testing.T) {
		relay.TurnOn()
		checkPinState(t, relay.pin, Output, High)
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
		checkPinState(t, pin, Output, Low)
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
	checkPinState(t, pumps.pump.pin, Output, pumpState)
	checkPinState(t, pumps.sweep.pin, Output, sweepState)
	checkPinState(t, pumps.solar.pin, Output, solarState)
	checkPinState(t, pumps.solarLed, Output, solarState)
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
		t.Errorf("Expected State %s, found %s", state.String(), pumps.State().String())
	}
	if pumps.ManualState() != manual {
		t.Errorf("Expected Manual %t found %t", manual, pumps.ManualState())
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
		pumpTest(t, pumps, STATE_OFF, Low, Low, Low,
			false, false, false, startTime, stopTime)
	})

	t.Run("StartPump", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_PUMP, false)
		pumpTest(t, pumps, STATE_PUMP, High, Low, Low,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartSweep", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_SWEEP, false)
		pumpTest(t, pumps, STATE_SWEEP, High, High, Low,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartPumpAfterSweep", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_PUMP, false)
		pumpTest(t, pumps, STATE_PUMP, High, Low, Low,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartSolar", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_SOLAR, false)
		pumpTest(t, pumps, STATE_SOLAR, High, Low, High,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartSolarMixing", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_SOLAR_MIXING, false)
		pumpTest(t, pumps, STATE_SOLAR_MIXING, High, High, High,
			true, false, false, startTime, stopTime)
	})	
	
        t.Run("StartManualPump", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_PUMP, true)
		pumpTest(t, pumps, STATE_PUMP, High, Low, Low,
			true, false, true, startTime, stopTime)
		pumps.SetState(STATE_SOLAR, false)
		pumpTest(t, pumps, STATE_PUMP, High, Low, Low,
			true, false, true, startTime, stopTime)
		
	})

	t.Run("StartManualSweep", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_SWEEP, true)
		pumpTest(t, pumps, STATE_SWEEP, High, High, Low,
			true, false, true, startTime, stopTime)
		pumps.SetState(STATE_SOLAR, false)
		pumpTest(t, pumps, STATE_SWEEP, High, High, Low,
			true, false, true, startTime, stopTime)
	})

	t.Run("StopAllManual", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(STATE_OFF, true)
		pumpTest(t, pumps, STATE_OFF, Low, Low, Low,
			false, true, true, startTime, stopTime)
		pumps.SetState(STATE_SOLAR_MIXING, false)
		pumpTest(t, pumps, STATE_OFF, Low, Low, Low,
			false, true, true, startTime, stopTime)
	})

	t.Run("Disable", func (t *testing.T) {
		t.Run("Disabled", func (t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.Disable()
			pumpTest(t, pumps, STATE_DISABLED, Low, Low, Low,
				false, true, true, startTime, stopTime)
		})
		
		t.Run("StartPump", func (t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.SetState(STATE_SOLAR_MIXING, false)
			pumpTest(t, pumps, STATE_DISABLED, Low, Low, Low,
				false, false, true, startTime, stopTime)
		})

		t.Run("Disabled", func (t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.SetState(STATE_PUMP, true)
			pumpTest(t, pumps, STATE_DISABLED, Low, Low, Low,
				false, false, true, startTime, stopTime)
		})
	})
	t.Run("Enable", func (t *testing.T) {
		t.Run("Disabled", func (t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.Disable()
			pumpTest(t, pumps, STATE_DISABLED, Low, Low, Low,
				false, true, true, startTime, stopTime)
		})
		t.Run("Enabled", func (t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.Enable()
			pumpTest(t, pumps, STATE_OFF, Low, Low, Low,
				false, true, true, startTime, stopTime)
		})	
	})
}
