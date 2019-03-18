package main

import (
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/host"
	"strconv"
	"time"
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

// State returns whether the pin is in a High or Low voltage state
func (s GpioState) State() gpio.Level {
	if s == Low {
		return gpio.Low
	}
	return gpio.High
}

func (s GpioState) String() string {
	return s.State().String()
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
	// BothEdges means taht a change is occuring in either direction.
	BothEdges Edge = 3
)

// Edge returns the current edge value.
func (e Edge) Edge() gpio.Edge {
	switch e {
	case NoEdge:
		return gpio.NoEdge
	case RisingEdge:
		return gpio.RisingEdge
	case FallingEdge:
		return gpio.FallingEdge
	case BothEdges:
		return gpio.BothEdges
	}
	return gpio.NoEdge
}

func (e Edge) String() string {
	return e.Edge().String()
}

// Pull refers to the configuration of the pin circuitry.
type Pull int

const (
	// Float lets the input flow directly, resistance is handled elswhere.
	Float Pull = 0
	// PullDown applies pull-down resistance to the pin
	PullDown Pull = 1
	// PullUp applies pull-up resistance to the pin
	PullUp Pull = 2
	// PullNoChange does not change the previous pull resistor setting
	PullNoChange Pull = 3
)

// Pull returns the current state of the pin's pull configuration
func (p Pull) Pull() gpio.Pull {
	switch p {
	case Float:
		return gpio.Float
	case PullDown:
		return gpio.PullDown
	case PullUp:
		return gpio.PullUp
	case PullNoChange:
		return gpio.PullNoChange
	}
	return gpio.PullNoChange
}

func (p Pull) String() string { return p.Pull().String() }

// PiPin represnets a GPIO pin on the Raspberry Pi
type PiPin interface {
	Input()
	InputEdge(Pull, Edge)
	Output(GpioState)
	Read() GpioState
	WaitForEdge(time.Duration) bool
	Pin() uint8
}

// Gpio implements a PiPin interface for a Raspberry Pi system.
type Gpio struct {
	gpio uint8
	pin  gpio.PinIO
}

// SetGpioProvider allows you to change the type of GPIO for the system (useful for testing)
func SetGpioProvider(p func(uint8) PiPin) {
	gpioProvider = p
}

func xGpioProvider(gpio uint8) PiPin {
	g := Gpio{
		gpio: gpio,
		pin:  gpioreg.ByName(strconv.Itoa(int(gpio))),
	}
	gpioreg.Register(g.pin)
	return (PiPin)(&g)
}

// NewGpio creates a new PiPin for a given gpio value.
func NewGpio(gpio uint8) PiPin {
	return gpioProvider(gpio)
}

// GpioInit initializes the system
func GpioInit() error {
	if _, err := host.Init(); err != nil {
		return err
	}
	return nil
}

// Input sets the pin to be read from.
func (g *Gpio) Input() {
	Debug("Setting gpio(%d) to Input(%s, %s)", g.gpio, Float, NoEdge)
	g.pin.In(gpio.Float, gpio.NoEdge)
}

// InputEdge sets the pin to be read from and to alert WaitForEdge when the given Edge is found.
func (g *Gpio) InputEdge(p Pull, e Edge) {
	Debug("Setting gpio(%d) to Input(%s, %s)", g.gpio, p, e)
	g.pin.In(p.Pull(), e.Edge())
}

// Output sets the pin to be written to.
func (g *Gpio) Output(s GpioState) {
	Debug("Output setting gpio(%d) to %s", g.gpio, s)
	g.pin.Out(s.State())
}

// Read returns the current state of the pin
func (g *Gpio) Read() GpioState {
	if g.pin.Read() == gpio.High {
		return High
	}
	return Low
}

// WaitForEdge blocks while waiting for a voltage change on the pin.
func (g *Gpio) WaitForEdge(timeout time.Duration) bool {
	return g.pin.WaitForEdge(timeout)
}

// Pin returns the GPIO number of the pin.
func (g *Gpio) Pin() uint8 {
	return g.gpio
}

// Direction refers to the usage of the pin.  Is it being used for input or output?
type Direction bool

const (
	// Input means that the value of the pin will be read and is controlled externally.
	Input Direction = false
	// Output means that the value of the pin will be written to and is controlled internally.
	Output Direction = true
)
