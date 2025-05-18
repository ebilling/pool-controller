package main

import (
	"fmt"
	"time"
)

// State refers to the current state of the system,
// are the pumps running or is the solar system engaged.
type State int8

const (
	// DISABLED means that the pumps are not allowed to run
	DISABLED State = iota - 1
	// OFF means that the pumps are OFF
	OFF
	// PUMP means that the main pump is running
	PUMP
	// SWEEP means that the main and sweep pumps are in operation
	SWEEP
	// SOLAR means that the main pump is running and the water is flowing to the solar panels
	SOLAR
	// MIXING means that the main and sweep pump are running with the solar panels in the flow
	// this allows for maximum mixing of the water at depth
	MIXING
)

func (s State) String() string {
	switch s {
	case DISABLED:
		return "Disabled"
	case OFF:
		return "Off"
	case PUMP:
		return "Pump Running"
	case SWEEP:
		return "Cleaning"
	case SOLAR:
		return "Solar Running"
	case MIXING:
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
	solar    *SolarValve
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
	return newSwitches(
		NewRelay(pumpGpio, "Pool Pump", manufacturer),
		NewRelay(sweepGpio, "Pool Sweep", manufacturer),
		NewSolarValve(solarFwdGpio, solarRevGpio, solarLedGpio, "Solar", manufacturer, solarMotorTime))
}

func newSwitches(pump *Relay, sweep *Relay, solar *SolarValve) *Switches {
	p := Switches{
		state:    OFF,
		pump:     pump,
		sweep:    sweep,
		solar:    solar,
		manualOp: time.Now().Add(time.Hour * -24),
	}
	p.StopAll(false)
	p.bindHK()
	return &p
}

func (p *Switches) bindHK() {
	p.pump.accessory.Switch.On.OnValueRemoteUpdate(func(on bool) {
		Log("HomeKit request to turn Pump on=%t", on)
		if on {
			p.SetState(PUMP, true, 1.0)
		} else {
			p.StopAll(true)
		}
	})

	p.sweep.accessory.Switch.On.OnValueRemoteUpdate(func(on bool) {
		Log("HomeKit request to turn Sweep on=%t", on)
		state := p.state
		switch p.state {
		case SOLAR:
			if on {
				state = MIXING
			}
		case MIXING:
			if !on {
				state = SOLAR
			}
		default:
			if on {
				state = SWEEP
			} else {
				state = PUMP
			}
		}
		p.SetState(state, true, 1.0)
	})

	p.solar.accessory.Switch.On.OnValueRemoteUpdate(func(on bool) {
		Log("HomeKit request to turn Solar on=%t", on)
		state := p.state
		switch p.state {
		case SWEEP, MIXING:
			if on {
				state = MIXING
			} else {
				state = SWEEP
			}
		case SOLAR:
			if !on {
				state = PUMP
			}
		default:
			if on {
				state = SOLAR
			} else {
				state = OFF
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
	if p.state == DISABLED {
		p.state = OFF
		p.StopAll(true)
	}
}

// Disable turns the pumps off and puts them in a state that will not allow them to run
func (p *Switches) Disable() {
	p.StopAll(true)
	p.state = DISABLED
}

// OnOff is something that can be turned off and on
type OnOff interface {
	TurnOn()
	TurnOff()
}

func turnOn(relay OnOff, on bool) {
	if on {
		relay.TurnOn()
	} else {
		relay.TurnOff()
	}
}

func (p *Switches) setSwitches(pumpOn, sweepOn, solarOn, isManual bool, state State) {
	turnOn(p.pump, pumpOn)
	turnOn(p.sweep, sweepOn)
	turnOn(p.solar, solarOn) // deal with solar valve last because it takes time
	if isManual {
		p.manualOp = time.Now()
	}
	p.state = state
}

// StopAll turns off all pumps
func (p *Switches) StopAll(manual bool) {
	state := OFF
	if p.state == DISABLED {
		state = DISABLED
	}
	p.setSwitches(false, false, false, manual, state)
}

// SetState sets the pump pins to particular values corresponding to a State
func (p *Switches) SetState(s State, manual bool, runtime float64) {
	if p.state == s {
		return // Nothing to do here
	}
	if p.state == DISABLED {
		Info("Disabled, can't change state from %s to %s",
			p.state, s)
		return
	}
	if p.ManualState(runtime) && !manual {
		Debug("Manual override, can't change state from %s to %s", p.state, s)
		return // Don't override a manual operation
	}
	Info("State change from %s to %s", p.state, s)
	switch s {
	case DISABLED:
		p.Disable()
		return
	case OFF:
		p.StopAll(manual)
		return
	case PUMP:
		p.setSwitches(true, false, false, manual, s)
		return
	case SWEEP:
		p.setSwitches(true, true, false, manual, s)
		return
	case SOLAR:
		p.setSwitches(true, false, true, manual, s)
		return
	case MIXING:
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
	if time.Since(p.manualOp) > DurationFromHours(runtime, 2.0) {
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
