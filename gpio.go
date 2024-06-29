package main

import (
	"time"

	"github.com/ebilling/gpio"
)

// gpioProvider generates pins for the platform (used for testing)
var gpioProvider = xGpioProvider // For testing on non-test setups

// GpioState represents the current binary value of the pin.  Is it High or Low Voltage
type GpioState bool

const (
	// Low voltage registered on the pin (~0-1v)
	Low GpioState = false
	// High voltage registered on the pin (~1-3.3v)
	High GpioState = true
)

// String returns the string representation of the state.
func (s GpioState) String() string {
	if s == Low {
		return "Low"
	}
	return "High"
}

// uint returns the binary value of the pin.
func (s GpioState) uint() uint {
	if s == High {
		return 1
	}
	return 0
}

// Edge refers to the rising or falling of a voltage value on the pin.
type Edge int

const (
	// NoEdge means no change
	NoEdge Edge = 0
	// RisingEdge means that the voltage is moving from a low to a high voltage state.
	RisingEdge Edge = 1
	// FallingEdge means that the voltage is moving from a high to a low voltage state.
	FallingEdge Edge = 2
	// BothEdges means that a change is occuring in either direction.
	BothEdges Edge = 3
)

func NewEdge(e gpio.Edge) Edge {
	switch e {
	case gpio.EdgeBoth:
		return BothEdges
	case gpio.EdgeRising:
		return RisingEdge
	case gpio.EdgeFalling:
		return FallingEdge
	default:
		return NoEdge
	}
}

// String returns the string representation of the edge.
func (e Edge) String() string {
	switch e {
	case NoEdge:
		return "None"
	case RisingEdge:
		return "Rising"
	case FallingEdge:
		return "Falling"
	case BothEdges:
		return "Both"
	default:
		return "Unknown"
	}
}

// Pull refers to the configuration of the pin circuitry.
// type Pull int

// const (
// 	// PullNoChange does not change the previous pull resistor setting
// 	PullNoChange Pull = 0
// 	// Float lets the input flow directly, resistance is handled elswhere.
// 	Float Pull = 1
// 	// PullDown applies pull-down resistance to the pin
// 	PullDown Pull = 2
// 	// PullUp applies pull-up resistance to the pin
// 	PullUp Pull = 3
// )

// // Pull returns the current state of the pin's pull configuration
// func (p Pull) Pull() gpio.Pull {
// 	return gpio.Pull(p)
// }

// func (p Pull) String() string { return p.Pull().String() }

// NotificationHandler is a callback function that is called when a notification is received.
// It must be registered with the PiPin Watch method.
// The Watch method will call the handler in a new goroutine until it recieves an error from the handler or the Watcher is closed.
type NotificationHandler func(n Notification) error

// PiPin represnets a GPIO pin on the Raspberry Pi
type PiPin interface {
	// Input sets the pin to be read from.
	Input()
	// Output sets the pin to be written to and sets the initial state.
	Output(GpioState)
	// Read returns the current state of the pin
	Read() (GpioState, error)
	// Write sets the state of the pin
	Write(GpioState) error
	// Pin returns the GPIO number of the pin.
	Pin() uint8
	// Close releases the resources related to the pin.
	Close()
	// Notifications returns a channel of notifications for the pin.
	Notifications(Edge, GpioState) <-chan Notification
	// Watch registers a handler to be called when a notification is received.
	Watch(NotificationHandler, Edge, GpioState) error
}

// Notification represents a change in the state of the pin.
type Notification struct {
	Pin   uint8
	Time  time.Time
	Value GpioState
}

// Gpio implements a PiPin interface for a Raspberry Pi system.
type Gpio struct {
	pin     uint8
	gpioPin gpio.Pin
}

// SetGpioProvider allows you to change the type of GPIO for the system (useful for testing)
func SetGpioProvider(p func(uint8) PiPin) {
	gpioProvider = p
}

func xGpioProvider(pin uint8) PiPin {
	g := Gpio{
		pin: pin,
	}
	return (PiPin)(&g)
}

// NewGpio creates a new PiPin for a given gpio value.
func NewGpio(gpio uint8) PiPin {
	return gpioProvider(gpio)
}

// Input sets the pin to be read from.
func (g *Gpio) Input() {
	if g.gpioPin.Number != 0 {
		g.Close()
	}
	g.gpioPin = gpio.NewInput(uint(g.pin))
}

// Close releases the resources related to the pin.
func (g *Gpio) Close() {
	g.gpioPin.Close()
	g.gpioPin.Number = 0
}

// Output sets the pin to be written to.
func (g *Gpio) Output(s GpioState) {
	if g.gpioPin.Number != 0 {
		g.Close()
	}
	g.gpioPin = gpio.NewOutput(uint(g.pin), bool(s))
	Info("Setting pin %d to %s", g.pin, s)
}

// Read returns the current state of the pin
func (g *Gpio) Read() (GpioState, error) {
	v, err := g.gpioPin.Read()
	if v == uint(gpio.ActiveHigh) {
		return High, err
	}
	return Low, err
}

// Write sets the state of the pin
func (g *Gpio) Write(s GpioState) error {
	if s == High {
		return g.gpioPin.SetLogicLevel(gpio.ActiveHigh)
	}
	return g.gpioPin.SetLogicLevel(gpio.ActiveLow)
}

// Notifications returns a channel of notifications for the pin.
func (g *Gpio) Notifications(e Edge, s GpioState) <-chan Notification {
	notify := make(chan Notification, 100)
	g.Watch(func(n Notification) error {
		notify <- n
		return nil
	}, e, s)
	return notify
}

// Watch registers a handler to be called when a notification is received.
func (g *Gpio) Watch(h NotificationHandler, e Edge, s GpioState) error {
	w := gpio.NewWatcher()
	w.AddPinWithEdgeAndLogic(uint(g.pin), gpio.Edge(e), gpio.LogicLevel(s.uint()))
	go func() {
		for w != nil {
			select {
			case n := <-w.Notification:
				val := Low
				if n.Value == uint(gpio.ActiveHigh) {
					val = High
				}
				err := h(Notification{
					Pin:   g.pin,
					Time:  time.Now(),
					Value: val,
				})
				if err != nil {
					w.Close()
				}
			}
		}
	}()
	return nil
}

// Pin returns the GPIO number of the pin.
func (g *Gpio) Pin() uint8 {
	return g.pin
}

// Direction refers to the usage of the pin.  Is it being used for input or output?
type Direction bool

const (
	// Input means that the value of the pin will be read and is controlled externally.
	Input Direction = false
	// Output means that the value of the pin will be written to and is controlled internally.
	Output Direction = true
)
