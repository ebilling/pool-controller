package main

import (
	"fmt"
	"time"
)

type State int8

const (
	STATE_DISABLED State  = iota - 1
	STATE_OFF
	STATE_PUMP
	STATE_SWEEP
	STATE_SOLAR
	STATE_SOLAR_MIXING
)

const SolarLED uint8 = 6

func (s State) String() string {
	switch s {
	case STATE_DISABLED:
		return "STATE_DISABLED"
	case STATE_OFF:
		return "STATE_OFF"
	case STATE_PUMP:
		return "STATE_PUMP"
	case STATE_SWEEP:
		return "STATE_SWEEP"
	case STATE_SOLAR:
		return "STATE_SOLAR"
	case STATE_SOLAR_MIXING:
		return "STATE_SOLAR_MIXING"
	default:
		return "STATE_UNKNOWN"
	}
}

type Switches struct {
	state       State
	pump        *Relay
	sweep       *Relay
	solar       *Relay
	solarLed    PiPin
	manualOp    time.Time	
}

func (p *Switches) String() string {
	return fmt.Sprintf(
		"Pump: {State: %s,\nPump: {%s},\nSweep: {%s},\nSolar: {%s},\nManualOp: %s}",
		p.state.String(), p.pump.String(), p.sweep.String(), p.solar.String(),
		timeStr(p.manualOp))
}

func NewSwitches(manufacturer string) (*Switches) {
	return newSwitches(
		NewRelay(Relay1, "Pool Pump", manufacturer),
		NewRelay(Relay2, "Pool Sweep", manufacturer),
		NewRelay(Relay3, "Solar", manufacturer),
		NewGpio(SolarLED))
}

func newSwitches(pump *Relay, sweep *Relay, solar *Relay, solarLed PiPin) (*Switches) {
	p := Switches{
		state: STATE_OFF,
		pump:  pump,
		sweep: sweep,
		solar: solar,
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
			p.SetState(STATE_PUMP, true)
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
		p.SetState(state, true)
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
		p.SetState(state, true)
        })
	
}

func (p *Switches) GetStartTime() time.Time {
	return p.pump.GetStartTime()
}

func (p *Switches) GetStopTime() time.Time {
	return p.pump.GetStopTime()
}

func (p *Switches) Enable() {
	if p.state == STATE_DISABLED {
		p.state = STATE_OFF
		p.StopAll(true)
	}
}

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
	if solarOn { p.solarLed.Output(High) } else { p.solarLed.Output(Low) }
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

func (p *Switches) StopAll(manual bool) {
	state := STATE_OFF
	if p.state == STATE_DISABLED { state = STATE_DISABLED }
	p.setSwitches(false, false, false, manual, state)
}

func (p *Switches) SetState(s State, manual bool) {
	if p.state == s {
		return // Nothing to do here
	}
	if p.state == STATE_DISABLED {
		Info("Disabled, can't change state from %s to %s",
			p.state, s);
		return
	}
	if p.ManualState() && !manual {
		Debug("Manual override, can't change state from %s to %s",
			p.state, s);
		return // Don't override a manual operation
	}
	Info("State change from %s to %s", p.state, s)
	switch s {
	case STATE_DISABLED:
		p.Disable(); return
	case STATE_OFF:
		p.StopAll(manual); return
	case STATE_PUMP:
		p.setSwitches(true, false, false, manual, s); return
	case STATE_SWEEP:
		p.setSwitches(true, true, false, manual, s); return
	case STATE_SOLAR:
		p.setSwitches(true, false, true, manual, s); return
	case STATE_SOLAR_MIXING:
		p.setSwitches(true, true, true, manual, s); return
	}
}

func (p *Switches) State() State {
	return p.state
}

func (p *Switches) ManualState() bool {
	if time.Now().Sub(p.manualOp) > 2 * time.Hour {
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
