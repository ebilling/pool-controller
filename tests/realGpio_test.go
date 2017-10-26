package main

import (
	"fmt"
	"testing"
	"time"
)

const (
	LED      = 5  // Pin 29
	RELAY    = 13 // Pin 33
	CAP4700  = 19 // Pin 35: Also PCM capable
	CAP10000 = 6  // Pin 31
	SWITCH   = 26 // Pin 37
)

func GpioStr(g PiPin) string {
	switch g.Pin() {
	case LED:
		return "LED"
	case RELAY:
		return "RELAY"
	case CAP4700:
		return "CAP4700"
	case CAP10000:
		return "CAP10000"
	case SWITCH:
		return "SWITCH"
	default:
		return "UNKNOWN"
	}
	return ""
}

var Led PiPin        // Setup: GPIO -> <1k Resistor -> LED -> GND
var TestRelay *Relay // Setup GPIO -> 4.7k Resistor -> Relay Board
var Cap4700 PiPin    // Setup: +3.3v -> 4.7k Resistor -> GPIO -> 10uF capacitor -> GND
var Cap10000 PiPin   // Setup: +3.3v -> 10k Resistor -> GPIO -> 10uF capacitor -> GND
var Switch PiPin     // Setup: GPIO -> Button Switch -> GND

func ExpectedState(t *testing.T, gpio PiPin, exp GpioState) {
	if val := gpio.Read(); val != exp {
		t.Errorf("%s: Expected %s but found %s", GpioStr(gpio), exp, val)
	}
}

func TestInitilization(t *testing.T) {
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
	Info("Running %s", t.Name())
	for i := 0; i < 6; i++ {
		time.Sleep(time.Second / 5)
		Led.Output(High)
		ExpectedState(t, Led, High)
		time.Sleep(time.Second / 5)
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
	Info("Running %s", t.Name())
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
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
	Info("Running %s", t.Name())
	therm := NewGpioThermometer("Fixed 4.7kOhm ResistorTest", "TestManufacturer", CAP4700, 10.0)

	t.Run("Calibrate Cap4700", func(t *testing.T) {
		Info("Running %s", t.Name())
		c, err := therm.Calibrate(4700)
		if err != nil {
			t.Errorf("Failure to Calibrate successfully: %s", err.Error())
		}
		Debug("Setting calibration for %0.3f", c)
		therm.SetAdjustment(c)
	})
	t.Run("Temperature Cap4700", func(t *testing.T) {
		Info("Running %s", t.Name())
		err := therm.Update()
		if err != nil {
			t.Errorf("Thermometer update failed: %s", err.Error())
		}
		if therm.Temperature() > 44.1 || therm.Temperature() < 43.1 {
			t.Errorf("Thermometer value off: %0.1f, expected 43.6",
				therm.Temperature())
		}
	})

	therm = NewGpioThermometer("Fixed 10kOhm ResistorTest", "TestManufacturer", CAP10000, 10.0)
	t.Run("Calibrate Cap10000", func(t *testing.T) {
		Info("Running %s", t.Name())
		c, err := therm.Calibrate(10000)
		if err != nil {
			t.Errorf("Failure to Calibrate successfully: %s", err.Error())
		}
		Debug("Setting calibration for %0.3f", c)
		therm.SetAdjustment(c)
	})
	t.Run("Temperature Cap4700", func(t *testing.T) {
		Info("Running %s", t.Name())
		err := therm.Update()
		if err != nil {
			t.Errorf("Thermometer update failed: %s", err.Error())
		}
		if therm.Temperature() > 25.4 || therm.Temperature() < 24.4 {
			t.Errorf("Thermometer value off: %0.1f, expected 24.9",
				therm.Temperature())
		}
	})
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
		Info("Running %s", t.Name())
		runRelayTestOn(t, r)
		time.Sleep(sleep)
		runRelayTestOff(t, r)
	})
}

func TestRelays(t *testing.T) {
	Info("Running %s", t.Name())
	GpioInit()
	TestRelay = NewRelay(RELAY, "Relay", "Testing")
	runRelayTest(t, TestRelay, time.Second)
}
