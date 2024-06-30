package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/brutella/hc/accessory"
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

// SolarValve controls two relays at the same time.
// Only one can be engaged at any one time, and will shut off after a given timeout
type SolarValve struct {
	fwdRelay  *Relay
	revRelay  *Relay
	status    bool // true==ON
	statusLED PiPin
	accessory *accessory.Switch
	mtx       sync.Mutex
	timeout   time.Duration
	cid       int // command id, distinguishes the calls to cleanup
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
func NewRelay(pin uint, name string, manufacturer string) *Relay {
	gpin := NewGpio(pin)
	gpin.Output(Low) // start in off position
	return newRelay(gpin, name, manufacturer)
}

func newRelay(pin PiPin, name string, manufacturer string) *Relay {
	relay := Relay{
		name:      name,
		pin:       pin,
		startTime: time.Now(),
		stopTime:  time.Now(),
		enabled:   true,
	}
	if name != "" {
		relay.accessory = accessory.NewSwitch(AccessoryInfo(name, manufacturer))
	}
	relay.TurnOff()
	return &relay
}

// Accessory returns the Apple HomeKit accessory associated with the relay
func (r *Relay) Accessory() *accessory.Accessory {
	if r == nil || r.accessory == nil {
		return nil
	}
	return r.accessory.Accessory
}

// Name returns the name of the Relay
func (r *Relay) Name() string {
	if r == nil {
		return ""
	}
	return r.name
}

// String returns the state of the Relay
func (r *Relay) String() string {
	if r == nil {
		return ""
	}
	return fmt.Sprintf(
		"Relay: { Name: %s, Pin: %v, StartTime: %s, StopTime: %s, Accessory: %v}",
		r.Name(), r.pin, timeStr(r.startTime), timeStr(r.stopTime), r.accessory)
}

// TurnOn flips the output to HIGH voltage (>1V)
func (r *Relay) TurnOn() {
	Trace("TurnOn %s", r.name)
	r.pin.Write(High)
	r.startTime = time.Now()
	if r.accessory != nil {
		r.accessory.Switch.On.SetValue(true)
	}
}

// TurnOff flips the output to LOW voltage (<1V)
func (r *Relay) TurnOff() {
	Trace("TurnOff %s", r.name)
	r.pin.Write(Low)
	r.stopTime = time.Now()
	if r.accessory != nil {
		r.accessory.Switch.On.SetValue(false)
	}
}

func (r *Relay) isOn() bool {
	val, err := r.pin.Read()
	if err != nil {
		Info("Relay %s read error: %v", r.name, err)
		os.Exit(1)
	}
	if val == High {
		if r.accessory != nil {
			r.accessory.Switch.On.SetValue(true)
		}
		return true
	}
	if r.accessory != nil {
		r.accessory.Switch.On.SetValue(false)
	}
	return false
}

// Status returns "On" if at HIGH voltage or "Off" if at LOW voltage
func (r *Relay) Status() string {
	val, err := r.pin.Read()
	if err != nil {
		Info("Relay %s read error: %v", r.name, err)
		os.Exit(1)
	}
	if val == High {
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

// NewSolarValve creates a special controller for the Solar Valve operation
// When set to ON, it runs the motor forward for 15 seconds
// When set to OFF, it runs the motor in reverse for 15 seconds
func NewSolarValve(forward uint, reverse uint, ledPin uint, name string, manufacturer string, timeout time.Duration) *SolarValve {
	r := &SolarValve{
		fwdRelay:  newRelay(NewGpio(forward), "", ""),
		revRelay:  newRelay(NewGpio(reverse), "", ""),
		statusLED: NewGpio(ledPin),
		accessory: accessory.NewSwitch(AccessoryInfo(name, manufacturer)),
		timeout:   timeout,
	}
	r.TurnOff()
	return r
}

// String returns a string that describes the current state of the Solar Valve
func (s *SolarValve) String() string {
	return fmt.Sprintf("Forward: %s, Reverse: %s, Status: %s", s.fwdRelay.String(), s.revRelay.String(), s.Status())
}

func (s *SolarValve) cleanup() {
	cid := s.cid // capture the cid before sleep
	time.Sleep(s.timeout)
	if s.cid == cid {
		// there isn't another one running
		s.fwdRelay.TurnOff()
		s.revRelay.TurnOff()
	}
}

func (s *SolarValve) setState(fwd bool) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.cid++
	go s.cleanup()
	s.statusLED.Output(GpioState(fwd))
	if fwd {
		s.revRelay.TurnOff()
		time.Sleep(time.Millisecond)
		s.fwdRelay.TurnOn()
	} else {
		s.fwdRelay.TurnOff()
		time.Sleep(time.Millisecond)
		s.revRelay.TurnOn()
	}
	s.status = fwd
}

// TurnOn runs the motor for the valve forward for timeout seconds
func (s *SolarValve) TurnOn() {
	s.setState(true)
}

// TurnOff runs the motor for the valve in reverse for timeout seconds
func (s *SolarValve) TurnOff() {
	s.setState(false)
}

// Status returns "On" if at HIGH voltage or "Off" if at LOW voltage
func (s *SolarValve) Status() string {
	if s.status {
		return "On"
	}
	return "Off"
}

// Accessory returns the accessory of the SolarValve
func (s *SolarValve) Accessory() *accessory.Accessory {
	return s.accessory.Accessory
}

func (s *SolarValve) isOn() bool {
	if s.status {
		s.accessory.Switch.On.SetValue(true)
	} else {
		s.accessory.Switch.On.SetValue(false)
	}
	return s.status
}

// GetStartTime returns the time the relay was last set to HIGH voltage
func (s *SolarValve) GetStartTime() time.Time {
	return s.fwdRelay.startTime
}

// GetStopTime returns the time the relay was last set to LOW voltage
func (s *SolarValve) GetStopTime() time.Time {
	return s.revRelay.stopTime
}
