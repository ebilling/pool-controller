package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/brutella/hc/accessory"
	"github.com/stretchr/testify/assert"
)

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
	if !assert.Equal(t, dir, tpin.direction, "%s Pin direction %s, expected %s", name, DirectionStr(tpin.direction), DirectionStr(dir)) {
		return false
	}
	if !assert.Equal(t, state, tpin.state, "%s Pin state %s, expected %s", name, StateStr(tpin.state), StateStr(state)) {
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
		if !checkPinState(t, "TestPin", relay.pin, Output, Low) {
			t.Errorf("")
		}
	})

	t.Run("TurnOn", func(t *testing.T) {
		relay.TurnOn()
		if !checkPinState(t, "TestPin", relay.pin, Output, High) {
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
		if !checkPinState(t, "TestPin", pin, Output, Low) {
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
	if !checkPinState(t, "Solar", pumps.solar.statusLED, Output, solarState) {
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
	if pumps.ManualState(1.0) != manual {
		t.Errorf("Expected Manual %t found %t",
			manual, pumps.ManualState(1.0))
	}
}

func TestGpioSwitchesBasic(t *testing.T) {
	pumpPin := &TestPin{}
	sweepPin := &TestPin{}
	solarFwdPin := &TestPin{}
	solarRevPin := &TestPin{}
	solarLedPin := &TestPin{}
	solarValve := &SolarValve{
		fwdRelay:  newRelay(solarFwdPin, "", ""),
		revRelay:  newRelay(solarRevPin, "", ""),
		statusLED: solarLedPin,
		timeout:   time.Microsecond,
		accessory: accessory.NewSwitch(AccessoryInfo("Test Solar Valve", mftr)),
	}
	solarValve.TurnOff() // set to off so we can test it
	pumps := newSwitches(
		newRelay(pumpPin, "Test Pump", mftr),
		newRelay(sweepPin, "Test Sweep", mftr),
		solarValve,
	)

	startTime := pumps.GetStartTime()
	stopTime := pumps.GetStopTime()
	t.Run("Initialized", func(t *testing.T) {
		pumpTest(t, pumps, OFF, Low, Low, Low,
			false, false, false, startTime, stopTime)
	})

	t.Run("StartPump", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(PUMP, false, 1.0)
		pumpTest(t, pumps, PUMP, High, Low, Low,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartSweep", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(SWEEP, false, 1.0)
		pumpTest(t, pumps, SWEEP, High, High, Low,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartPumpAfterSweep", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(PUMP, false, 1.0)
		pumpTest(t, pumps, PUMP, High, Low, Low,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartSolar", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(SOLAR, false, 1.0)
		pumpTest(t, pumps, SOLAR, High, Low, High,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartSolarMixing", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(MIXING, false, 1.0)
		pumpTest(t, pumps, MIXING, High, High, High,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartManualPump", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(PUMP, true, 1.0)
		pumpTest(t, pumps, PUMP, High, Low, Low,
			true, false, true, startTime, stopTime)
		pumps.SetState(SOLAR, false, 1.0)
		pumpTest(t, pumps, PUMP, High, Low, Low,
			true, false, true, startTime, stopTime)

	})

	t.Run("StartManualSweep", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(SWEEP, true, 1.0)
		pumpTest(t, pumps, SWEEP, High, High, Low,
			true, false, true, startTime, stopTime)
		pumps.SetState(SOLAR, false, 1.0)
		pumpTest(t, pumps, SWEEP, High, High, Low,
			true, false, true, startTime, stopTime)
	})

	t.Run("StopAllManual", func(t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.SetState(OFF, true, 1.0)
		pumpTest(t, pumps, OFF, Low, Low, Low,
			false, true, true, startTime, stopTime)
		pumps.SetState(MIXING, false, 1.0)
		pumpTest(t, pumps, OFF, Low, Low, Low,
			false, true, true, startTime, stopTime)
	})

	t.Run("Disable", func(t *testing.T) {
		t.Run("Disabled", func(t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.Disable()
			pumpTest(t, pumps, DISABLED, Low, Low, Low,
				false, true, true, startTime, stopTime)
		})

		t.Run("StartPump", func(t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.SetState(MIXING, false, 1.0)
			pumpTest(t, pumps, DISABLED, Low, Low, Low,
				false, false, true, startTime, stopTime)
		})

		t.Run("Disabled", func(t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.SetState(PUMP, true, 1.0)
			pumpTest(t, pumps, DISABLED, Low, Low, Low,
				false, false, true, startTime, stopTime)
		})
	})
	t.Run("Enable", func(t *testing.T) {
		t.Run("Disabled", func(t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.Disable()
			pumpTest(t, pumps, DISABLED, Low, Low, Low,
				false, true, true, startTime, stopTime)
		})
		t.Run("Enabled", func(t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.Enable()
			pumpTest(t, pumps, OFF, Low, Low, Low,
				false, true, true, startTime, stopTime)
		})
	})
}

func NewTestPin(gpio uint8) PiPin {
	return (PiPin)(&TestPin{
		sleepTime: 20 * time.Millisecond,
		pin:       gpio,
		wake:      make(chan bool),
	})
}

type TestPin struct {
	state       GpioState
	edge        Edge
	direction   Direction
	sleepTime   time.Duration
	inputTime   time.Time
	wake        chan bool
	pin         uint8
	waitForWake bool
}

func (p *TestPin) Close() {
}

func (p *TestPin) Input() {
	p.direction = Input
	p.inputTime = time.Now()
	p.edge = NoEdge
}

func (p *TestPin) InputEdge(pull Pull, e Edge) {
	p.direction = Input
	p.inputTime = time.Now()
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

func (p *TestPin) WaitForEdge(interval time.Duration) (time.Duration, bool) {
	if p.waitForWake {
		state := <-p.wake
		return time.Since(p.inputTime), state
	}
	if p.sleepTime > interval {
		return interval, false
	}
	return p.sleepTime, true
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
