package main

import (
	"time"
)

type Button struct {
	pin        PiPin
	callback   func()
	bouncetime time.Duration
	pushed     time.Time
	disabled   bool
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
		pushed:     time.Now().Add(-1 * time.Second),
		done:       make(chan bool),
	}
	return &b
}

func (b *Button) Start() {
	started := make(chan bool)
	go b.RunLoop(&started)
	if <-started { // Wait for loop to start
		Debug("Button loop started")
	}
}

func (b *Button) Disable() {
	b.disabled = true
}

func (b *Button) Enable() {
	b.disabled = false
}

func (b *Button) IsDisabled() bool {
	return b.disabled
}

func (b *Button) RunLoop(started *chan bool) {
	b.pin.Output(Low)
	b.pin.InputEdge(PullUp, RisingEdge)
	*started <- true
	for true {
		if b.pin.WaitForEdge(time.Second) {
			if b.IsDisabled() {
				time.Sleep(time.Second)
				continue
			}
			now := time.Now() // Here for debugging purposes
			state := b.pin.Read()
			if b.pushed.Add(b.bouncetime).Before(now) {
				if state == Low {
					b.pushed = now // filter noise of up/down
					Debug("Button Pushed: Running Callback")
					b.callback()
				} else {
					Debug("State is High, no callback")
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
	Debug("Button stopped")
}
