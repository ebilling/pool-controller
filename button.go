package main

import (
	"errors"
	"time"
)

// Button is a simple pushbutton that registers when the voltage on a GPIO pin changes suddenly.
type Button struct {
	pin        PiPin
	callback   func()
	bouncetime time.Duration
	pushed     time.Time
	disabled   bool
	done       chan bool
}

// NewGpioButton sets up a specific GPIO pin as a button, and runs the callback when it is pressed.
func NewGpioButton(pin uint8, callback func()) *Button {
	return newButton(NewGpio(pin), callback)
}

func newButton(pin PiPin, callback func()) *Button {
	b := Button{
		pin:        pin,
		callback:   callback,
		bouncetime: time.Second / 2,
		pushed:     time.Now().Add(-1 * time.Second),
		done:       make(chan bool),
	}
	return &b
}

// Start runs a thread in the background that monitors the button activity.
func (b *Button) Start() {
	b.pin.Watch(b.buttonHandler, PullDown, FallingEdge, Low)
}

func (b *Button) buttonHandler(n Notification) error {
	Debug("Button Handler")
	if b.IsDisabled() {
		return nil
	}
	now := time.Now() // Here for debugging purposes
	if b.pushed.Add(b.bouncetime).Before(now) {
		if n.Value == Low {
			b.pushed = now // filter noise of up/down
			Debug("Button Pushed: Running Callback")
			b.callback()
		} else {
			Debug("State is High, no callback")
		}
	}
	select {
	case <-b.done:
		return errors.New("Button stopped")
	case <-time.After(time.Second): // Required to not block
	}
	return nil
}

// Disable allows you to disable the button, ignoring any pushes that come.
func (b *Button) Disable() {
	b.disabled = true
}

// Enable re-enables a button that has been disabled, so it will no longer ignore pushes.
func (b *Button) Enable() {
	b.disabled = false
}

// IsDisabled returns true if the button is in a disabled state.
func (b *Button) IsDisabled() bool {
	return b.disabled
}

// Stop kills the thread that is monitoring the button activity.
func (b *Button) Stop() {
	b.done <- true
	Debug("Button stopped")
}
