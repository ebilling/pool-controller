package main

import (
	"testing"
	"fmt"
	"time"
)

func runTestOn(t *testing.T, relay *Relay) {
	relay.TurnOn()
	Info("Testing Relay On: %s is %s", relay.Name(), relay.Status())
	if !relay.isOn() {
		t.Errorf("Relay(%s) is %s", relay.Name(), relay.Status())
	}
}

func runTestOff(t *testing.T, relay *Relay) {
	relay.TurnOff()
	Info("Testing Relay Off: %s is %s", relay.Name(), relay.Status())
	if relay.isOn() {
		t.Errorf("Relay(%s) is %s", relay.Name(), relay.Status())
	}
}

func runTest(t *testing.T, r *Relay, sleep time.Duration) {
	t.Run(fmt.Sprintf("%s.Test", r.Name()), func(t *testing.T) {
		runTestOn(t, r)
		time.Sleep(sleep)
		runTestOff(t,r)
	})
}

func TestRelays(t *testing.T) {
	EnableDebug()
	GpioInit()
	//	pump := NewRelay(Relay1, "Pump", "Testing")
//	sweep := NewRelay(Relay2, "Sweep", "Testing")
//	solar := NewRelay(Relay3, "Solar", "Testing")

	empty := NewRelay(13, "Empty", "Testing")

	runTest(t, empty, 15 * time.Second)
}
