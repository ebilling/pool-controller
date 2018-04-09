package main

import (
	"testing"
	"time"
)

func testBoolChan(b chan bool, timeout time.Duration) bool {
	for true {
		select {
		case val := <-b:
			return val
		case <-time.After(timeout):
			return false
		}
	}
	return false
}

func TestFakeButton(t *testing.T) {
	LogTestMode()
	Info("Running %s", t.Name())
	timeout := 50 * time.Millisecond
	pushed := make(chan bool)
	pin, _ := testpin_generator(99).(*TestPin)
	pin.sleepTime = time.Second * 20 // Don't accidentally wake up and send a signal

	button := newButton(pin, func() {
		pushed <- true
	})

	button.Start()

	t.Run("FalsePush", func(t *testing.T) {
		pin.state = Low                                        // Not a push
		button.pushed = time.Now().Add(-1 * button.bouncetime) // Not a bounce
		pin.wake <- true
		if testBoolChan(pushed, timeout) != true {
			t.Errorf("Expected pushed(true), found false")
		}
	})

	t.Run("QuickPush", func(t *testing.T) {
		pin.state = High                                             // Push
		button.pushed = time.Now().Add((-1 * button.bouncetime) / 2) // Bounce
		pin.wake <- true
		if testBoolChan(pushed, timeout) != false {
			t.Errorf("Expected pushed(false), found true")
		}
	})

	t.Run("Push", func(t *testing.T) {
		tm := time.Now().Add(-1 * button.bouncetime)
		pin.state = High   // Push
		button.pushed = tm // Not a bounce
		pin.wake <- true
		if testBoolChan(pushed, timeout) != false {
			t.Errorf("Expected pushed(false), found true, pushed_t(%s), now(%s)",
				timeStr(button.pushed), timeStr(tm))
		}
	})

	button.Stop()
}
