package main

import (
	"time"

	rpio "github.com/stianeikeland/go-rpio/v4"
)

// OpenGPIO opens the GPIO interface.
func OpenGPIO() error {
	return rpio.Open()
}

// CloseGPIO closes the GPIO interface.
func CloseGPIO() error {
	return rpio.Close()
}

// Gpio implements a PiPin interface for a Raspberry Pi system.
type Gpio struct {
	gpioPin rpio.Pin
}

// NewGpio creates a new PiPin for a given gpio value.
func NewGpio(pin uint8) PiPin {
	return &Gpio{
		gpioPin: rpio.Pin(pin),
	}
}

// Input sets the pin to be read from.
func (g *Gpio) Input() {
	g.gpioPin.Input()
}

// Close releases the resources related to the pin.
func (g *Gpio) Close() {
	// no-op
}

// Output sets the pin to be written to.
func (g *Gpio) Output(s GpioState) {
	g.gpioPin.Output()
}

// Read returns the current state of the pin
func (g *Gpio) Read() (GpioState, error) {
	v := g.gpioPin.Read()
	if v == rpio.High {
		return High, nil
	}
	return Low, nil
}

// Write sets the state of the pin
func (g *Gpio) Write(s GpioState) error {
	if s == High {
		g.gpioPin.Write(rpio.High)
	}
	g.gpioPin.Write(rpio.Low)
	return nil
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

func rEdge(e Edge) rpio.Edge {
	switch e {
	case RisingEdge:
		return rpio.RiseEdge
	case FallingEdge:
		return rpio.FallEdge
	case BothEdges:
		return rpio.AnyEdge
	default:
		return rpio.NoEdge
	}
}

// Watch registers a handler to be called when a notification is received.
func (g *Gpio) Watch(h NotificationHandler, e Edge, s GpioState) error {
	go func() {
		start := time.Now()
		detections := 0
		nodetections := 0
		lows := 0
		highs := 0
		g.gpioPin.Output()
		g.gpioPin.Write(rpio.Low)
		g.gpioPin.Input()
		g.gpioPin.Detect(rEdge(e))
		for i := 0; i < 1000; i++ {
			val := Low
			if g.gpioPin.EdgeDetected() {
				detections++
				if g.gpioPin.Read() == rpio.High {
					val = High
					highs++
				} else {
					lows++
				}
				err := h(Notification{
					Pin:   g.Pin(),
					Time:  time.Now(),
					Value: val,
				})
				if err != nil {
					break
				}
			} else {
				// Testing
				nodetections++
				if g.gpioPin.Read() == rpio.High {
					highs++
				} else {
					lows++
				}
			}
		}
		g.gpioPin.Detect(rpio.NoEdge)
		Info("watcher exited after %s: pin(%d) d(%d) nd(%d) highs(%d) lows(%d)",
			time.Since(start), g.gpioPin, detections, nodetections, highs, lows)
	}()
	return nil
}

// Pin returns the GPIO number of the pin.
func (g *Gpio) Pin() uint8 {
	return uint8(g.gpioPin)
}