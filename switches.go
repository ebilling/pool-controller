package main

import (
	"github.com/stianeikeland/go-rpio"
	"fmt"
	"time"
)

type State uint8

const (
	STATE_DISABLED State  = iota
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
		rpio.Pin(SolarLED))
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
	return &p
}

func (p *Switches) GetStartTime() time.Time {
	return p.pump.GetStartTime()
}

func (p *Switches) GetStopTime() time.Time {
	return p.pump.GetStopTime()
}

func (p *Switches) Disable() {
	p.StopAll()
	p.state = STATE_DISABLED
}

func (p *Switches) StopAll() {
	p.sweep.TurnOff()
	p.pump.TurnOff()
	p.solar.TurnOff()
	p.solarLed.Low()
	if p.state != STATE_DISABLED {
		p.state = STATE_OFF
	}
}

func (p *Switches) StartPump() {
	if p.state == STATE_DISABLED {
		return
	}
	p.pump.TurnOn()
	p.sweep.TurnOff()
	p.solar.TurnOff()
	p.solarLed.Low()
	p.state = STATE_PUMP
}

func (p *Switches) StartSweep() {
	if p.state == STATE_DISABLED {
		return
	}
	p.pump.TurnOn()
	p.sweep.TurnOn()
	p.solar.TurnOff()
	p.solarLed.Low()
	p.state = STATE_SWEEP
}

func (p *Switches) StartSolar() {
	if p.state == STATE_DISABLED {
		return
	}
	p.StartPump()
	p.solar.TurnOn()
	p.solarLed.High()
	p.state = STATE_SOLAR
}

func (p *Switches) StartSolarMixing() {
	if p.state == STATE_DISABLED {
		return
	}
	p.StartSweep()
	p.solarLed.High()
	p.solar.TurnOn()
	p.state = STATE_SOLAR_MIXING
}

func (p *Switches) StartPumpManual() {
	p.StartPump()
	p.manualOp = p.GetStartTime()
}

func (p *Switches) StartSweepManual() {
	p.StartSweep()
	p.manualOp = p.GetStartTime()
}

func (p *Switches) StopAllManual() {
	p.StopAll()
	p.manualOp = p.GetStopTime()
}

func (p *Switches) GetState() State {
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
