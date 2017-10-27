package main

import (
	"fmt"
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/host"
	"strconv"
	"time"
)

//var __no_gpio__ bool = false // For testing on non-test setups

type GpioState bool

const (
	Low  GpioState = false
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
	Pin() uint8
}

type Gpio struct {
	gpio uint8
	pin  gpio.PinIO
}

func NewGpio(gpio uint8) PiPin {
	//  var p PiPin
	//	if __no_gpio__ { // Special mode for when you aren't running on RaspberryPi
	//		p = (PiPin)(&TestPin{sleepTime: 20 * time.Millisecond})
	//		return p
	//	}
	g := Gpio{
		gpio: gpio,
		pin:  gpioreg.ByName(strconv.Itoa(int(gpio))),
	}
	gpioreg.Register(g.pin, false)
	return (PiPin)(&g)
}

func GpioInit() error {
	//	if __no_gpio__ {
	//		return nil
	//	}
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
	Debug("Setting gpio(%d) to Input(%s, %s)", g.gpio, p, e)
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

func (g *Gpio) Pin() uint8 {
	return g.gpio
}

type Direction bool

const (
	Input  Direction = false
	Output Direction = true
)

type TestPin struct {
	state     GpioState
	pull      Pull
	edge      Edge
	direction Direction
	sleepTime time.Duration
	inputTime time.Time
	pin       uint8
}

func (p *TestPin) Input() {
	p.direction = Input
	p.inputTime = time.Now()
	p.pull = Float
	p.edge = NoEdge
}

func (p *TestPin) InputEdge(pull Pull, e Edge) {
	p.direction = Input
	p.inputTime = time.Now()
	p.pull = pull
	p.edge = e
}

func (p *TestPin) Output(s GpioState) {
	p.direction = Output
	p.state = s
}

func (p *TestPin) Read() GpioState {
	now := time.Now()
	sleeptime := p.inputTime.Add(p.sleepTime)
	if p.sleepTime > 0 && now.After(sleeptime) {
		p.state = High
	}
	return p.state
}

func (p *TestPin) WaitForEdge(ignored time.Duration) bool {
	time.Sleep(p.sleepTime)
	return true
}

func (p *TestPin) Pin() uint8 {
	return p.pin
}

func (p *TestPin) String() string {
	direction := "Input"
	if p.direction == Output {
		direction = "Output"
	}
	return fmt.Sprintf("TestPin: {State: %s, Direction: %s, Edge: %s, Pull: %s, Duration: %d, InputTime: %s}",
		p.state, direction, p.edge, p.pull, p.sleepTime, timeStr(p.inputTime))
}
