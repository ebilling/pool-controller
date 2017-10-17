package main

import (
	"github.com/stianeikeland/go-rpio"
	"fmt"
	"testing"
	"time"
)

type TestPin struct {
	state        rpio.State
	direction    rpio.Direction
	sleepTime    time.Duration
	inputTime    time.Time
}

func (p *TestPin) Input() {
	p.direction = rpio.Input
	p.inputTime = time.Now()
	Debug("Setting Fake pin to Input mode %v", p)
}

func (p *TestPin) Output() {
	p.direction = rpio.Output
	Debug("Setting Fake pin to Output mode %v", p)
}

func (p *TestPin) High() {
	p.state = rpio.High
	Debug("Setting Fake pin state to High %v", p)
}

func (p *TestPin) Low() {
	p.state = rpio.Low
	Debug("Setting Fake pin state to Low %v", p)
}

func (p *TestPin) Read() rpio.State {
	now := time.Now()
	sleeptime := p.inputTime.Add(p.sleepTime)
	if p.sleepTime > 0 && now.After(sleeptime) {
		p.state = rpio.High
	}
	return p.state
}

func (p *TestPin) PullUp() {}

func (p *TestPin) PullDown() {}

func (p *TestPin) PullOff() {}

func (p *TestPin) String() string {
	state := "Low"
        if p.state == rpio.High {
		state = "High"
	}
	direction := "Input"
	if p.direction == rpio.Output {
		direction = "Output"
	}
	return fmt.Sprintf("TestPin: {State: %s, Direction: %s, Duration: %d, InputTime: %s}",
		state, direction, p.sleepTime, timeStr(p.inputTime))
}

func TestGpioThermometer(t *testing.T) {
	sleeptime := 100 * time.Millisecond
	pin := TestPin{
		state:        rpio.Low,
		direction:    rpio.Input,
		sleepTime:    sleeptime,
		inputTime:    time.Now(),
	}
	therm := NewGpioThermometer("Test Thermometer", mftr, 22, 10.0)
	therm.pin = &pin

	Debug("States (H=%d, L=%d) Direction (I=%d, O=%d)", rpio.High, rpio.Low,
		rpio.Input, rpio.Output)
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


func checkPinState(t *testing.T, pin PiPin, dir rpio.Direction, state rpio.State) {
	tpin := pin.(*TestPin)
	if tpin.direction != dir {
		t.Errorf("Pin direction %d, expected %d", tpin.direction, dir)
	}
	if tpin.state != state {
		t.Errorf("Pin state %d, expected %d", tpin.state, state)
	}
}

func TestGpioRelay(t *testing.T) {
	pin := &TestPin{}
	relay := newRelay(pin, "Test Pin", mftr)
	start := relay.startTime
	stop := relay.stopTime
	
	t.Run("NewRelay", func(t *testing.T) {
		checkPinState(t, pin, rpio.Output, rpio.Low)
	})
	
	t.Run("TurnOn", func(t *testing.T) {
		relay.TurnOn()
		checkPinState(t, pin, rpio.Output, rpio.High)
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
		checkPinState(t, pin, rpio.Output, rpio.Low)
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

func pumpTest(t *testing.T, pumps *Switches, state State, pumpState rpio.State,
	sweepState rpio.State, solarState rpio.State, started bool, stopped bool,
	manual bool, startTime time.Time, stopTime time.Time) {
	checkPinState(t, pumps.pump.pin, rpio.Output, pumpState)
	checkPinState(t, pumps.sweep.pin, rpio.Output, sweepState)
	checkPinState(t, pumps.solar.pin, rpio.Output, solarState)
	checkPinState(t, pumps.solarLed, rpio.Output, solarState)
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
	if pumps.GetState() != state {
		t.Errorf("Expected %s, found %s", state.String(), pumps.GetState().String())
	}
	if pumps.ManualState() != manual {
		t.Errorf("Should have been manual")
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
		pumpTest(t, pumps, STATE_OFF, rpio.Low, rpio.Low, rpio.Low,
			false, false, false, startTime, stopTime)
	})

	t.Run("StartPump", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.StartPump()
		pumpTest(t, pumps, STATE_PUMP, rpio.High, rpio.Low, rpio.Low,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartSweep", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.StartSweep()
		pumpTest(t, pumps, STATE_SWEEP, rpio.High, rpio.High, rpio.Low,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartPumpAfterSweep", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.StartPump()
		pumpTest(t, pumps, STATE_PUMP, rpio.High, rpio.Low, rpio.Low,
			true, false, false, startTime, stopTime)
	})

        t.Run("StartManualPump", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.StartPumpManual()
		pumpTest(t, pumps, STATE_PUMP, rpio.High, rpio.Low, rpio.Low,
			true, false, true, startTime, stopTime)
	})

	t.Run("StartManualSweep", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.StartSweepManual()
		pumpTest(t, pumps, STATE_SWEEP, rpio.High, rpio.High, rpio.Low,
			true, false, true, startTime, stopTime)
	})

	t.Run("StopAllManual", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.StopAllManual()
		pumpTest(t, pumps, STATE_OFF, rpio.Low, rpio.Low, rpio.Low,
			false, true, true, startTime, stopTime)
	})

	t.Run("StartSolar", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.StartSolar()
		pumpTest(t, pumps, STATE_SOLAR, rpio.High, rpio.Low, rpio.High,
			true, false, false, startTime, stopTime)
	})

	t.Run("StartSolarMixing", func (t *testing.T) {
		startTime = pumps.GetStartTime()
		stopTime = pumps.GetStopTime()
		pumps.StartSolarMixing()
		pumpTest(t, pumps, STATE_SOLAR_MIXING, rpio.High, rpio.High, rpio.High,
			true, false, false, startTime, stopTime)
	})	
	
	t.Run("Disable", func (t *testing.T) {
		t.Run("Disabled", func (t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.Disable()
			pumpTest(t, pumps, STATE_DISABLED, rpio.Low, rpio.Low, rpio.Low,
				false, true, false, startTime, stopTime)
		})
		
		t.Run("StartPump", func (t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.StartPump()
			pumpTest(t, pumps, STATE_DISABLED, rpio.Low, rpio.Low, rpio.Low,
				false, false, false, startTime, stopTime)
		})

		t.Run("Disabled", func (t *testing.T) {
			startTime = pumps.GetStartTime()
			stopTime = pumps.GetStopTime()
			pumps.StartSweep()
			pumpTest(t, pumps, STATE_DISABLED, rpio.Low, rpio.Low, rpio.Low,
				false, false, false, startTime, stopTime)
		})
	})
}
