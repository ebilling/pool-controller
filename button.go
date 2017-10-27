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
		bouncetime: 200 * time.Millisecond,
		done:       make(chan bool),
	}
	pin.InputEdge(PullUp, FallingEdge)
	return &b
}

func (b *Button) Start() {
	started := make(chan bool)
	go b.RunLoop(started)
	<-started
}

func (b *Button) RunLoop(started chan bool) {
	start := time.Now()
	started <- true
	for true {
		select {
		case <-b.done:
			return // End job
		default: // Required to not block
			break
		}
		if b.pin.WaitForEdge(time.Second) {
			if start.Add(b.bouncetime).Before(time.Now()) {
				start = time.Now() // filter noise of up/down
				state := b.pin.Read()
				if state == Low {
					Debug("Button Pushed: Running Callback")
					b.callback()
				}
			}
		}
	}
}

func (b *Button) Stop() {
	b.done <- true
}
