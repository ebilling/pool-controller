package main

import (
	"fmt"
	"testing"
	"time"
)

const (
	LED = 5     // Pin 29
	CAP = 6     // Pin 31
	NEXT = 13   // Pin 33
	SWITCH = 19 // Pin 35: Also PCM capable
)

func GpioStr(g *Gpio) string {
	switch g.gpio {
	case LED:
		return "LED"
	case CAP:
		return "CAP"
	case NEXT:
		return "NEXT"
	case SWITCH:
		return "SWITCH"
	default:
		return "UNKNOWN"
	}
	return ""
}

var Led *Gpio // Setup: GPIO -> <1k Resistor -> LED -> GND
var Cap *Gpio // Setup: +3.3v -> 1k-20k Resistor -> GPIO -> 10uF capacitor -> GND
var Switch *Gpio // Setup: GPIO -> Button Switch -> GND

func ExpectedState(t *testing.T, gpio *Gpio, exp GpioState) {
	time.Sleep(time.Second/2)
	if val := gpio.Read(); val != exp {		
		t.Errorf("%s: Expected %s but found %s", GpioStr(gpio), exp, val)
	}
}

func TestInitilization(t *testing.T) {
	EnableDebug()
	t.Run("Init Host", func (t *testing.T) {
		if err := GpioInit(); err != nil {
			t.Errorf("Problem initializing gpio: %s", err.Error())
		}
	})

	// Initialized GPIOs
	Led = NewGpio(LED)
	ExpectedState(t, Led, Low)
}

func TestBlinkLed(t *testing.T) {
	for i := 0 ; i < 5 ; i++ {
		Led.Output(High)
		ExpectedState(t, Led, High)
		time.Sleep(time.Second/4)
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
	fmt.Printf("TestPushButton\n")
	button := NewGpioButton(SWITCH, func() {
		wasRun++
		Info("Button Pushed %d!!!", wasRun)
		Led.Output(High)
		time.Sleep(time.Second/2)
		Led.Output(Low)
	})
	Info("Starting button test, push it 3 times!")
	button.Start()
	for i:=0 ; i < 30 && wasRun < 3 ; i++ {
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
	if therm.Temperature() < 45.0 || therm.Temperature() > 40.0 {
		t.Errorf("Thermometer value not within acceptable limits: %0.1f",
			therm.Temperature())
	}
}
