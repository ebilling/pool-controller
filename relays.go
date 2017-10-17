package main

import (
	"github.com/brutella/hc/accessory"
	"github.com/stianeikeland/go-rpio"
	"fmt"
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
	accessory  *accessory.Switch
}

func AccessoryInfo(name string, manufacturer string) (accessory.Info) {
	info := accessory.Info{Name: name, Manufacturer: manufacturer}
	return info
}

func timeStr(t time.Time) string{
	return fmt.Sprintf("%02d:%02d:%02d.%09d",
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond())	
}

func NewRelay(pin uint8, name string, manufacturer string) (*Relay) {
	return newRelay(rpio.Pin(pin), name, manufacturer)
}

func newRelay(pin PiPin, name string, manufacturer string) (*Relay) {
	relay := Relay{
		name:      name,
		pin:       pin,
		startTime: time.Now().Add(time.Hour * -24),
		stopTime:  time.Now().Add(time.Hour * -24),
		accessory: accessory.NewSwitch(AccessoryInfo(name, manufacturer)),
	}
	relay.pin.Output()
	relay.pin.Low()
	return &relay
}

func (r *Relay) Accessory() (*accessory.Accessory) {
	return r.accessory.Accessory
}

func (r *Relay) Name() string {
	return r.name
}

func (r *Relay) String() string {
	return fmt.Sprintf(
		"Relay: { Name: %s, Pin: %v, StartTime: %s, StopTime: %s, Accessory: %s}",
		r.Name(), r.pin, timeStr(r.startTime), timeStr(r.stopTime), r.accessory)
}

func (r *Relay) TurnOn() {
	Trace("TurnOn %s", r.name)
	r.pin.High()
	r.startTime = time.Now()
	r.accessory.Switch.On.SetValue(true)
}

func (r *Relay) TurnOff() {
	Trace("TurnOff %s", r.name)
	r.pin.Low()
	r.stopTime = time.Now()
	r.accessory.Switch.On.SetValue(false)
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
