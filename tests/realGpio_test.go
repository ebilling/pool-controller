package main

import (
	"fmt"
	"testing"
	"time"
)

const (
	LED    = 5  // Pin 29
	CAP    = 6  // Pin 31
	RELAY  = 13 // Pin 33
	SWITCH = 19 // Pin 35: Also PCM capable
)

func GpioStr(g PiPin) string {
	switch g.Pin() {
	case LED:
		return "LED"
	case CAP:
		return "CAP"
	case RELAY:
		return "RELAY"
	case SWITCH:
		return "SWITCH"
	default:
		return "UNKNOWN"
	}
	return ""
}

var Led PiPin        // Setup: GPIO -> <1k Resistor -> LED -> GND
var Cap PiPin        // Setup: +3.3v -> 1k-20k Resistor -> GPIO -> 10uF capacitor -> GND
var TestRelay *Relay // Setup GPIO -> 4.7k Resistor -> Relay Board
var Switch PiPin     // Setup: GPIO -> Button Switch -> GND

func ExpectedState(t *testing.T, gpio PiPin, exp GpioState) {
	if val := gpio.Read(); val != exp {
		t.Errorf("%s: Expected %s but found %s", GpioStr(gpio), exp, val)
	}
}

func TestInitilization(t *testing.T) {
	EnableDebug()
	EndTestMode()
	err := GpioInit()
	t.Run("Init Host", func(t *testing.T) {
		if err != nil {
			t.Errorf("Problem initializing gpio: %s", err.Error())
		}
	})

	// Initialized GPIOs
	Led = NewGpio(LED)
	ExpectedState(t, Led, Low)
}

func TestBlinkLed(t *testing.T) {
	for i := 0; i < 6; i++ {
		Led.Output(High)
		ExpectedState(t, Led, High)
		time.Sleep(time.Second / 3)
		Led.Output(Low)
		ExpectedState(t, Led, Low)
	}
}

func doStop(button *Button, b *bool, t time.Time) {
	*b = false
	button.Stop()
	*b = true
	Info("doStop - Stopped after %d ms", time.Now().Sub(t)/time.Millisecond)
}

func TestPushButton(t *testing.T) {
	wasRun := 0
	button := NewGpioButton(SWITCH, func() {
		wasRun++
		Info("Button Pushed %d!!!", wasRun)
		Led.Output(High)
		time.Sleep(time.Second / 2)
		Led.Output(Low)
	})
	Info("Starting button test, push it 3 times!")
	button.Start()
	for i := 0; i < 30 && wasRun < 3; i++ {
		time.Sleep(time.Second)
	}
	if wasRun < 3 {
		t.Errorf("Expected 3 button pushes")
	}
	Info("Stopping button job")
	exited := false
	go doStop(button, &exited, time.Now())
	time.Sleep(time.Second)
	if !exited {
		t.Errorf("Button loop should have stopped within time allotted")
	}
	Info("Button job stopped")
}

func TestThermometer(t *testing.T) {
	therm := NewGpioThermometer("FixedResistorTest", "TestManufacturer", CAP, 10.0)
	err := therm.Update()
	if err != nil {
		t.Errorf("Thermometer update failed: %s", err.Error())
	}
	if therm.Temperature() > 44.0 || therm.Temperature() < 43.0 {
		t.Errorf("Thermometer value not within acceptable limits: %0.1f",
			therm.Temperature())
	}
}

func runRelayTestOn(t *testing.T, relay *Relay) {
	relay.TurnOn()
	Info("Testing Relay On: %s is %s", relay.Name(), relay.Status())
	if !relay.isOn() {
		t.Errorf("Relay(%s) is %s", relay.Name(), relay.Status())
	}
}

func runRelayTestOff(t *testing.T, relay *Relay) {
	relay.TurnOff()
	Info("Testing Relay Off: %s is %s", relay.Name(), relay.Status())
	if relay.isOn() {
		t.Errorf("Relay(%s) is %s", relay.Name(), relay.Status())
	}
}

func runRelayTest(t *testing.T, r *Relay, sleep time.Duration) {
	t.Run(fmt.Sprintf("%s.Test", r.Name()), func(t *testing.T) {
		runRelayTestOn(t, r)
		time.Sleep(sleep)
		runRelayTestOff(t, r)
	})
}

func TestRelays(t *testing.T) {
	EnableDebug()
	GpioInit()
	TestRelay = NewRelay(RELAY, "Relay", "Testing")
	runRelayTest(t, TestRelay, time.Second)
}
