package main

import (
	"github.com/stianeikeland/go-rpio"
	"time"
)

const (
	Relay1 uint8 = 17
	Relay2       = 27
	Relay3       = 22
	Relay4       = 23
)

var RELAYS = [...]uint8{Relay1, Relay2, Relay3, Relay4}

type Relay struct {
	name       string
	pin        PiPin
	startTime  time.Time
	stopTime   time.Time
}

func NewRelay(gpio uint8, name string) (*Relay) {
	relay := Relay{
		name:      name,
		pin:       rpio.Pin(gpio),
		startTime: time.Now().Add(time.Hour * -24),
		stopTime:  time.Now().Add(time.Hour * -24),
	}
	relay.pin.Output()
	relay.pin.Low()
	return &relay
}

func (r *Relay) TurnOn() {
	Trace("TurnOn %s", r.name)
	r.pin.High()
	r.startTime = time.Now()
}

func (r *Relay) TurnOff() {
	Trace("TurnOff %s", r.name)
	r.pin.Low()
	r.stopTime = time.Now()
}

func (r *Relay) Status() string {
	if r.pin.Read() == rpio.High {
		return "On"
	}
	return "Off"
}

func (r *Relay) GetStartTime() time.Time {
	return r.startTime
}

func (r *Relay) GetStopTime() time.Time {
	return r.stopTime
}
