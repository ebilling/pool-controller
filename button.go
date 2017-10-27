package main

import (
	"time"
)

type Button struct {
	pin        PiPin
	callback   func()
	bouncetime time.Duration
	done       chan bool
}

func NewGpioButton(pin uint8, callback func()) *Button {
	return newButton(NewGpio(pin), callback)
}

func newButton(pin PiPin, callback func()) *Button {
	b := Button{
		pin:        pin,
		callback:   callback,
		bouncetime: 300 * time.Millisecond,
		done:       make(chan bool),
	}
	return &b
}

func (b *Button) Start() {
	started := make(chan bool)
	go b.RunLoop(&started)
	if <-started {
		Info("Button loop started")
	}
}

func (b *Button) RunLoop(started *chan bool) {
	start := time.Now().Add(-1 * b.bouncetime)
	b.pin.Output(Low)
	b.pin.InputEdge(PullUp, RisingEdge)
	*started <- true
	for true {
		if b.pin.WaitForEdge(time.Second) {
			state := b.pin.Read()
			if start.Add(b.bouncetime).Before(time.Now()) {
				if state == High {
					start = time.Now() // filter noise of up/down
					Debug("Button Pushed: Running Callback")
					b.callback()
				} else {
					Debug("State is Low, no callback")
				}
			} else {
				Debug("Bouncetime not encountered")
			}
			Debug("Edge Detected: %s", state)
		}
		select {
		case <-b.done:
			return
		default: // Required to not block
			break
		}
	}
}

func (b *Button) Stop() {
	b.done <- true
}
