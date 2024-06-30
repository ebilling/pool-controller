package main

import (
	"time"
)

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
type Pull int

const (
	// PullNoChange does not change the previous pull resistor setting
	PullNoChange Pull = 0
	// Float lets the input flow directly, resistance is handled elswhere.
	Float Pull = 1
	// PullDown applies pull-down resistance to the pin
	PullDown Pull = 2
	// PullUp applies pull-up resistance to the pin
	PullUp Pull = 3
)

func (p Pull) String() string {
	switch p {
	case PullNoChange:
		return "No Change"
	case Float:
		return "Float"
	case PullDown:
		return "Pull Down"
	case PullUp:
		return "Pull Up"
	default:
		return "Unknown"
	}
}

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
	Notifications(Pull, Edge, GpioState) <-chan Notification
	// Watch registers a handler to be called when a notification is received.
	Watch(NotificationHandler, Pull, Edge, GpioState) error
}

// Notification represents a change in the state of the pin.
type Notification struct {
	Pin   uint8
	Time  time.Time
	Value GpioState
}

// String returns the string representation of the notification.
func (n Notification) String() string {
	return n.Time.Format("2006-01-02 15:04:05") + " Pin " + string(n.Pin) + " is " + n.Value.String()
}

// Direction refers to the usage of the pin.  Is it being used for input or output?
type Direction bool

const (
	// Input means that the value of the pin will be read and is controlled externally.
	Input Direction = false
	// Output means that the value of the pin will be written to and is controlled internally.
	Output Direction = true
)

// String returns the string representation of the direction.
func (d Direction) String() string {
	if d == Input {
		return "Input"
	}
	return "Output"
}
