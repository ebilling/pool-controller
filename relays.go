package main

import (
	"fmt"
	"time"

	"github.com/brutella/hc/accessory"
)

const (
	// Relay1 corresponds to GPIO 17
	Relay1 uint8 = 17
	// Relay2 corresponds to GPIO 27
	Relay2 uint8 = 27
	// Relay3 corresponds to GPIO 22
	Relay3 uint8 = 22
	// Relay4 corresponds to GPIO 23
	Relay4 uint8 = 23
)

// Relay controls the behavior of a particular relay in the system.
type Relay struct {
	name      string
	pin       PiPin
	startTime time.Time
	stopTime  time.Time
	accessory *accessory.Switch
	enabled   bool
}

// AccessoryInfo tells Apple HomeKit about the device
func AccessoryInfo(name string, manufacturer string) accessory.Info {
	info := accessory.Info{Name: name, Manufacturer: manufacturer}
	return info
}

func timeStr(t time.Time) string {
	return fmt.Sprintf("%02d:%02d:%02d.%09d",
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond())
}

// NewRelay creates a relay for a given GPIO
func NewRelay(pin uint8, name string, manufacturer string) *Relay {
	return newRelay(NewGpio(pin), name, manufacturer)
}

func newRelay(pin PiPin, name string, manufacturer string) *Relay {
	relay := Relay{
		name:      name,
		pin:       pin,
		startTime: time.Now(),
		stopTime:  time.Now(),
		accessory: accessory.NewSwitch(AccessoryInfo(name, manufacturer)),
		enabled:   true,
	}
	relay.TurnOff()
	return &relay
}

// Accessory returns the Apple HomeKit accessory associated with the relay
func (r *Relay) Accessory() *accessory.Accessory {
	return r.accessory.Accessory
}

// Name returns the name of the Relay
func (r *Relay) Name() string {
	return r.name
}

// String returns the state of the Relay
func (r *Relay) String() string {
	return fmt.Sprintf(
		"Relay: { Name: %s, Pin: %v, StartTime: %s, StopTime: %s, Accessory: %v}",
		r.Name(), r.pin, timeStr(r.startTime), timeStr(r.stopTime), r.accessory)
}

// TurnOn flips the output to HIGH voltage (>1V)
func (r *Relay) TurnOn() {
	Trace("TurnOn %s", r.name)
	r.pin.Output(High)
	r.startTime = time.Now()
	r.accessory.Switch.On.SetValue(true)
}

// TurnOff flips the output to LOW voltage (<1V)
func (r *Relay) TurnOff() {
	Trace("TurnOff %s", r.name)
	r.pin.Output(Low)
	r.stopTime = time.Now()
	r.accessory.Switch.On.SetValue(false)
}

func (r *Relay) isOn() bool {
	if r.pin.Read() == High {
		r.accessory.Switch.On.SetValue(true)
		return true
	}
	r.accessory.Switch.On.SetValue(false)
	return false
}

// Status returns "On" if at HIGH voltage or "Off" if at LOW voltage
func (r *Relay) Status() string {
	if r.pin.Read() == High {
		return "On"
	}
	return "Off"
}

// GetStartTime returns the time the relay was last set to HIGH voltage
func (r *Relay) GetStartTime() time.Time {
	return r.startTime
}

// GetStopTime returns the time the relay was last set to LOW voltage
func (r *Relay) GetStopTime() time.Time {
	return r.stopTime
}
