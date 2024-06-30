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
	gpio uint
	pin  rpio.Pin
}

// NewGpio creates a new PiPin for a given gpio value.
func NewGpio(pin uint) PiPin {
	return &Gpio{
		gpio: pin,
		pin:  rpio.Pin(pin),
	}
}

// Input sets the pin to be read from.
func (g *Gpio) Input() {
	g.pin.Input()
}

// Close releases the resources related to the pin.
func (g *Gpio) Close() {
	// no-op
}

// Output sets the pin to be written to.
func (g *Gpio) Output(s GpioState) {
	g.pin.Output()
	if s == High {
		g.pin.Write(rpio.High)
		return
	}
	g.pin.Write(rpio.Low)
}

// Read returns the current state of the pin
func (g *Gpio) Read() (GpioState, error) {
	v := g.pin.Read()
	if v == rpio.High {
		return High, nil
	}
	return Low, nil
}

// Write sets the state of the pin
func (g *Gpio) Write(s GpioState) error {
	if s == High {
		g.pin.Write(rpio.High)
	}
	g.pin.Write(rpio.Low)
	return nil
}

// Notifications returns a channel of notifications for the pin.
func (g *Gpio) Notifications(p Pull, e Edge, s GpioState) <-chan Notification {
	notify := make(chan Notification, 100)
	g.Watch(func(n Notification) error {
		notify <- n
		return nil
	}, p, e, s)
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

func rPull(p Pull) rpio.Pull {
	switch p {
	case PullDown:
		return rpio.PullDown
	case PullUp:
		return rpio.PullUp
	case PullNoChange:
		return rpio.PullNone
	default:
		return rpio.PullOff
	}
}

type stats struct {
	detections bool
	highs      int
	lows       int
}

type stateCounter struct {
	state GpioState
	count int
	time  time.Time
}

// Watch registers a handler to be called when a notification is received.
func (g *Gpio) Watch(h NotificationHandler, p Pull, e Edge, s GpioState) error {
	g.pin.Pull(rPull(p))
	go func() {
		start := time.Now()
		end := start.Add(time.Second / 4)
		detections := stats{detections: true}
		g.Output(s)
		g.Input()
		scnt := stateCounter{state: Low}
		for i := 0; time.Now().Before(end); i++ {
			val := Low
			if g.pin.Read() == rpio.High {
				val = High
				detections.highs++
			} else {
				detections.lows++
			}
			if i == 0 {
				scnt.state = val
				scnt.time = time.Now()
			}
			if val != scnt.state {
				Info("state change detected[%d]: %s -> %s after %d polls %s", g.gpio, scnt.state, val, i, time.Since(scnt.time))
				scnt.state = val
				scnt.count = 1
				scnt.time = time.Now()
				n := Notification{
					Pin:   g.Pin(),
					Time:  time.Now(),
					Value: val,
				}
				Info("Sending Notification: %s", n)
				err := h(n)
				if err != nil {
					Info("Handler Error: watcher exited after %s: pin(%d) d(%d/%d) %v", time.Since(start), g.gpio, detections.lows, detections.highs, err)
					break
				}
			}
		}
		g.pin.Detect(rpio.NoEdge)
		Info("watcher exited after %s: pin(%d) d(%d/%d)",
			time.Since(start), g.gpio, detections.lows, detections.highs)
	}()
	return nil
}

// Pin returns the GPIO number of the pin.
func (g *Gpio) Pin() uint {
	return g.gpio
}
