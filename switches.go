package main

import (
	"fmt"
	"time"
)

// State refers to the current state of the system,
// are the pumps running or is the solar system engaged.
type State int8

const (
	// STATE_DISABLED means that the pumps are not allowed to run
	STATE_DISABLED State = iota - 1
	// STATE_OFF means that the pumps are OFF
	STATE_OFF
	// STATE_PUMP means that the main pump is running
	STATE_PUMP
	// STATE_SWEEP means that the main and sweep pumps are in operation
	STATE_SWEEP
	// STATE_SOLAR means that the main pump is running and the water is flowing to the solar panels
	STATE_SOLAR
	// STATE_SOLAR_MIXING means that the main and sweep pump are running with the solar panels in the flow
	// this allows for maximum mixing of the water at depth
	STATE_SOLAR_MIXING
)

// SolarLED is the GPIO number of the LED
const SolarLED uint8 = 6

func (s State) String() string {
	switch s {
	case STATE_DISABLED:
		return "Disabled"
	case STATE_OFF:
		return "Off"
	case STATE_PUMP:
		return "Pump Running"
	case STATE_SWEEP:
		return "Cleaning"
	case STATE_SOLAR:
		return "Solar Running"
	case STATE_SOLAR_MIXING:
		return "Solar Mixing"
	default:
		return "Unknown"
	}
}

// Switches controls all of the relays in the system
type Switches struct {
	state    State
	pump     *Relay
	sweep    *Relay
	solar    *Relay
	solarLed PiPin
	manualOp time.Time
}

func (p *Switches) String() string {
	return fmt.Sprintf(
		"Pump: {State: %s,\nPump: {%s},\nSweep: {%s},\nSolar: {%s},\nManualOp: %s}",
		p.state.String(), p.pump.String(), p.sweep.String(), p.solar.String(),
		timeStr(p.manualOp))
}

// NewSwitches sets up the switches that are configured
func NewSwitches(manufacturer string) *Switches {
	NewRelay(Relay4, "Unused", manufacturer)
	return newSwitches(
		NewRelay(Relay1, "Pool Pump", manufacturer),
		NewRelay(Relay2, "Pool Sweep", manufacturer),
		NewRelay(Relay3, "Solar", manufacturer),
		NewGpio(SolarLED))
}

func newSwitches(pump *Relay, sweep *Relay, solar *Relay, solarLed PiPin) *Switches {
	p := Switches{
		state:    STATE_OFF,
		pump:     pump,
		sweep:    sweep,
		solar:    solar,
		solarLed: solarLed,
		manualOp: time.Now().Add(time.Hour * -24),
	}
	solarLed.Output(Low)
	p.bindHK()
	return &p
}

func (p *Switches) bindHK() {
	p.pump.accessory.Switch.On.OnValueRemoteUpdate(func(on bool) {
		if on == true {
			p.SetState(STATE_PUMP, true, 1.0)
		} else {
			p.StopAll(true)
		}
	})

	p.sweep.accessory.Switch.On.OnValueRemoteUpdate(func(on bool) {
		state := p.state
		switch p.state {
		case STATE_SOLAR:
			if on {
				state = STATE_SOLAR_MIXING
			}
			break
		case STATE_SOLAR_MIXING:
			if !on {
				state = STATE_SOLAR
			}
			break
		default:
			if on {
				state = STATE_SWEEP
			} else {
				state = STATE_PUMP
			}
		}
		p.SetState(state, true, 1.0)
	})

	p.solar.accessory.Switch.On.OnValueRemoteUpdate(func(on bool) {
		state := p.state
		switch p.state {
		case STATE_SWEEP:
		case STATE_SOLAR_MIXING:
			if on {
				state = STATE_SOLAR_MIXING
			} else {
				state = STATE_SWEEP
			}
			break
		case STATE_SOLAR:
			if !on {
				state = STATE_PUMP
			}
			break
		default:
			if on {
				state = STATE_SOLAR
			} else {
				state = STATE_OFF
			}
		}
		p.SetState(state, true, 1.0)
	})

}

// GetStartTime returns the start time of the last pump run (could be still running)
func (p *Switches) GetStartTime() time.Time {
	return p.pump.GetStartTime()
}

// GetStopTime returns the stop time of the last pump run
func (p *Switches) GetStopTime() time.Time {
	return p.pump.GetStopTime()
}

// Enable re-enables the pumps after having been disabled
func (p *Switches) Enable() {
	if p.state == STATE_DISABLED {
		p.state = STATE_OFF
		p.StopAll(true)
	}
}

// Disable turns the pumps off and puts them in a state that will not allow them to run
func (p *Switches) Disable() {
	p.StopAll(true)
	p.state = STATE_DISABLED
}

func turnOn(relay *Relay, on bool) {
	if on {
		relay.TurnOn()
	} else {
		relay.TurnOff()
	}
}

func (p *Switches) setSwitches(pumpOn, sweepOn, solarOn, isManual bool, state State) {
	if solarOn {
		p.solarLed.Output(High)
	} else {
		p.solarLed.Output(Low)
	}
	turnOn(p.solar, solarOn)
	turnOn(p.pump, pumpOn)
	turnOn(p.sweep, sweepOn)
	if isManual {
		if p.GetStartTime().After(p.GetStopTime()) {
			p.manualOp = p.GetStartTime()
		} else {
			p.manualOp = p.GetStopTime()
		}
	}
	p.state = state
}

// StopAll turns off all pumps
func (p *Switches) StopAll(manual bool) {
	state := STATE_OFF
	if p.state == STATE_DISABLED {
		state = STATE_DISABLED
	}
	p.setSwitches(false, false, false, manual, state)
}

// SetState sets the pump pins to particular values corresponding to a State
func (p *Switches) SetState(s State, manual bool, runtime float64) {
	if p.state == s {
		return // Nothing to do here
	}
	if p.state == STATE_DISABLED {
		Info("Disabled, can't change state from %s to %s",
			p.state, s)
		return
	}
	if p.ManualState(runtime) && !manual {
		Debug("Manual override, can't change state from %s to %s",
			p.state, s)
		return // Don't override a manual operation
	}
	Info("State change from %s to %s", p.state, s)
	switch s {
	case STATE_DISABLED:
		p.Disable()
		return
	case STATE_OFF:
		p.StopAll(manual)
		return
	case STATE_PUMP:
		p.setSwitches(true, false, false, manual, s)
		return
	case STATE_SWEEP:
		p.setSwitches(true, true, false, manual, s)
		return
	case STATE_SOLAR:
		p.setSwitches(true, false, true, manual, s)
		return
	case STATE_SOLAR_MIXING:
		p.setSwitches(true, true, true, manual, s)
		return
	}
}

// State returns the current State of the system
func (p *Switches) State() State {
	return p.state
}

// DurationFromHours converts a given number of hours to a duration.  If hours < minHours,
// the duration of minHours is returned
func DurationFromHours(hours float64, minHours float64) time.Duration {
	if hours < minHours {
		hours = minHours
	}
	return time.Duration(hours * float64(time.Hour))
}

// ManualState returns true if the pumps were started or stopped manually
func (p *Switches) ManualState(runtime float64) bool {
	if time.Now().Sub(p.manualOp) > DurationFromHours(runtime, 2.0) {
		return false
	}
	if p.manualOp.Equal(p.GetStartTime()) && p.GetStartTime().After(p.GetStopTime()) {
		return true
	}
	if p.manualOp.Equal(p.GetStopTime()) && p.GetStopTime().After(p.GetStartTime()) {
		return true
	}
	return false
}
