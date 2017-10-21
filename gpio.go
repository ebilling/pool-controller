package main

import (
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/host"
	"strconv"
	"time"
)

type GpioState bool

const (
	Low GpioState = false
	High GpioState = true
)

func (s GpioState) State() gpio.Level {
	if s == Low {
		return gpio.Low
	}
	return gpio.High
}

func (s GpioState) String() string {
	return s.State().String()
}

type Edge int
const (
    NoEdge      Edge = 0
    RisingEdge  Edge = 1
    FallingEdge Edge = 2
    BothEdges   Edge = 3
)

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

type Pull int
const (
    Float        Pull = 0 // Let the input float
    PullDown     Pull = 1 // Apply pull-down
    PullUp       Pull = 2 // Apply pull-up
    PullNoChange Pull = 3 // Do not change the previous pull resistor setting
)

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

type PiPin interface {
	Input()
	InputEdge(Pull, Edge)
	Output(GpioState)
	Read() GpioState
	WaitForEdge(time.Duration) bool
}

type Gpio struct {
	gpio     uint8
	pin      gpio.PinIO
}

func NewGpio(gpio uint8) (*Gpio) {
	g := Gpio{
		gpio:      gpio,
		pin:       gpioreg.ByName(strconv.Itoa(int(gpio))),
	}
	gpioreg.Register(g.pin, false)
	g.Output(Low)
	return &g
}

func GpioInit() error {
	if _, err := host.Init(); err != nil {
		return err
	}
	return nil
}

func (g *Gpio) Input() {
	Debug("Setting gpio(%d) to Input(%s, %s)", g.gpio, Float, NoEdge)
	g.pin.In(gpio.Float, gpio.NoEdge)
}

func (g *Gpio) InputEdge(p Pull, e Edge) {
	Debug("Setting gpio(%d) to Input(%s, %s)", g.gpio, p, NoEdge)
	g.pin.In(p.Pull(), e.Edge())
}

func (g *Gpio) Output(s GpioState) {
	Debug("Output setting gpio(%d) to %s", g.gpio, s)
	g.pin.Out(s.State())
}

func (g *Gpio) Read() GpioState {
	if g.pin.Read() == gpio.High {
		return High
	}
	return Low
}

func (g *Gpio) WaitForEdge(timeout time.Duration) bool {
	return g.pin.WaitForEdge(timeout)
}
