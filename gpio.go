package main

import "time"

// gpioProvider generates pins for the platform (used for testing and simulation)
var gpioProvider = newCdevPin

// gpioInitFn initializes the host GPIO driver. Simulation replaces this with a no-op.
var gpioInitFn = CdevGpioInit

// GpioState represents the current binary value of the pin.  Is it High or Low Voltage
type GpioState bool

const (
	// Low voltage registered on the pin (~0-1v)
	Low GpioState = false
	// High voltage registered on the pin (~1-3.3v)
	High GpioState = true
)

func (s GpioState) String() string {
	if s == High {
		return "High"
	}
	return "Low"
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

func (e Edge) String() string {
	switch e {
	case RisingEdge:
		return "Rising"
	case FallingEdge:
		return "Falling"
	case BothEdges:
		return "Both"
	default:
		return "None"
	}
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

func (p Pull) String() string {
	switch p {
	case PullDown:
		return "Down"
	case PullUp:
		return "Up"
	case PullNoChange:
		return "NoChange"
	default:
		return "Float"
	}
}

// PiPin represnets a GPIO pin on the Raspberry Pi
type PiPin interface {
	Input()
	InputEdge(Pull, Edge)
	Output(GpioState)
	Read() GpioState
	WaitForEdge(time.Duration) bool
	Pin() uint8
}

// SetGpioProvider allows you to change the type of GPIO for the system (useful for testing)
func SetGpioProvider(p func(uint8) PiPin) {
	gpioProvider = p
}

// NewGpio creates a new PiPin for a given gpio value.
func NewGpio(gpio uint8) PiPin {
	return gpioProvider(gpio)
}

// GpioInit initializes the system GPIO driver (or a simulation stand-in).
func GpioInit() error {
	return gpioInitFn()
}

// Direction refers to the usage of the pin.  Is it being used for input or output?
type Direction bool

const (
	// Input means that the value of the pin will be read and is controlled externally.
	Input Direction = false
	// Output means that the value of the pin will be written to and is controlled internally.
	Output Direction = true
)
