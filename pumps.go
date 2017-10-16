package main

import (
	"time"
)

type State int8

const (
	STATE_DISABLED State = -1
	STATE_OFF
	STATE_PUMP
	STATE_SWEEP
	STATE_SCHEDULED_PUMP
	STATE_SCHEDULED_SWEEP
	STATE_SOLAR
	STATE_SOLAR_MIXING
)

type Pumps struct {
	state       State
	pump        *Relay
	sweep       *Relay
	solar       *Relay
	manualOp    time.Time
}

func NewPumps() (*Pumps) {
	p := Pumps{
		state: STATE_OFF,
		pump:  NewRelay(Relay1, "Pump"),
		sweep: NewRelay(Relay2, "Sweep"),
		solar: NewRelay(Relay3, "Solar"),
		manualOp: time.Now().Add(time.Hour * -24),
	}	
	return &p
}

func (p *Pumps) GetStartTime() time.Time {
	return p.pump.GetStartTime()
}

func (p *Pumps) GetStopTime() time.Time {
	return p.pump.GetStopTime()
}

func (p *Pumps) Disable() {
	p.StopAll()
	p.state = STATE_DISABLED
}

func (p *Pumps) StopAll() {
	p.pump.TurnOff()
	p.sweep.TurnOff()
	if p.state != STATE_DISABLED {
		p.state = STATE_OFF
	}
}

func (p *Pumps) StartPump() {
	p.sweep.TurnOff()
	p.pump.TurnOn()
	if p.state != STATE_DISABLED {
		p.state = STATE_PUMP
	}
}

func (p *Pumps) StartSweep() {
	p.sweep.TurnOn()
	p.sweep.TurnOn()
	if p.state != STATE_DISABLED {
		p.state = STATE_SWEEP
	}
}

func (p *Pumps) SetState(state State) {
	p.state = state
}

func (p *Pumps) GetState() State {
	return p.state
}
