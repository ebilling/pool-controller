package main

import (
	"github.com/stianeikeland/go-rpio"
	"time"
)

type Button struct {
	pin        PiPin
	callback   func ()
	sleeptime  time.Duration
	bouncetime time.Duration	
	done       chan bool
}

func NewGpioButton(pin uint8, callback func()) (*Button) {
	return newButton(rpio.Pin(pin), callback)
}

func newButton(pin PiPin, callback func ()) (*Button) {
	b := Button{
		pin:          pin,
		callback:     callback,
		sleeptime:    20 * time.Millisecond,
		bouncetime:   250 * time.Millisecond,
		done:         make(chan bool),
	}
	pin.Input()
	return &b
}

func (b *Button) Start() {
	go b.RunLoop()
}

func (b *Button) RunLoop() {
	start:= time.Now()
	oldState:= b.pin.Read()
	for true {
		select {
		case done := <- b.done:
			if done {break}
		}
		time.Sleep(b.sleeptime)
		if start.Add(b.bouncetime).Before(time.Now()) {
			start = time.Now() // filter noise of up/down
			state := b.pin.Read()
			if state != oldState {
				oldState = state // State change		
				if state == rpio.High {
					b.callback()
				}
			}
		}
	}
}

func (b *Button) Stop() {
	b.done <- true
}
