package main

import (
	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/host"
	"strconv"
)

type GpioState bool

const (
	Low GpioState = false
	High GpioState = true
)

func (s GpioState) String() string {
	if s == Low {
		return "Low"
	}
	return "High"
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
	return &g
}

func GpioInit() error {
	if _, err := host.Init(); err != nil {
		return err
	}
	return nil
}

func (g *Gpio) Input() {
	Debug("Setting gpio(%d) to Input", g.gpio)
	g.pin.In(gpio.Float, gpio.NoEdge)
}

func (g *Gpio) Output() {
	Debug("Output setting gpio(%d) to Low", g.gpio)
	g.pin.Out(gpio.Low)
}

func (g *Gpio) High() {
	Debug("Turning gpio(%d) to High", g.gpio)
	g.pin.Out(gpio.High)
}

func (g *Gpio) Low() {
	Debug("Turning gpio(%d) to Low", g.gpio)
	g.pin.Out(gpio.Low)
}

func (g *Gpio) Read() GpioState {
	if g.pin.Read() == gpio.High {
		return High
	}
	return Low
}

func (g *Gpio) PullUp() {
	g.pin.In(gpio.PullUp, gpio.NoEdge)
}

func (g *Gpio) PullDown() {
	g.pin.In(gpio.PullDown, gpio.NoEdge)
}

func (g *Gpio) PullOff() {
	g.pin.In(gpio.Float, gpio.NoEdge)
}
