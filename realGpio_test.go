package main

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
)

//  To run these tests, you need to build a system that is set up as follows:
//      GPIO5  -> <1k Resistor  -> LED    -> GND
//      +3.3v  -> 4.7k Resistor -> GPIO19 -> 10uF capacitor -> GND
//      +3.3v  ->  10k Resistor -> GPIO6  -> 10uF capacitor -> GND
//      GPIO13 -> 4.7k Resistor -> Relay Input
//      GPIO26 -> Open Relay Terminals -> GND, relay will push the button...
//
//      SUPER IMPORTANT: Create an empty file in the current directory named TestRig
//                       to tell the test system it's ok to run these tests.

const (
	LED      = 5  // Pin 29
	RELAY    = 13 // Pin 33
	CAP4700  = 19 // Pin 35: Also PCM capable
	CAP10000 = 6  // Pin 31
	SWITCH   = 26 // Pin 37
)

var TestRig bool = false
var Led PiPin
var TestRelay *Relay
var Cap4700 PiPin
var Cap10000 PiPin
var Switch PiPin

var r10000 float64 = 9930.0
var r4700 float64 = 4600.0

func TestInitilization(t *testing.T) {
	SkipTestIfNotTestRig(t)
	err := GpioInit()
	t.Run("Init Host", func(t *testing.T) {
		if err != nil {
			t.Errorf("Problem initializing gpio: %s", err.Error())
		}
	})

	// Initialized GPIOs
	Led = NewGpio(LED)
	Led.Output(Low)
	ExpectedState(t, Led, Low)
}

func TestRelays(t *testing.T) {
	SkipTestIfNotTestRig(t)
	Info("Running %s", t.Name())
	TestRelay = NewRelay(RELAY, "Relay", "Testing")
	runRelayTest(t, TestRelay, time.Second)
}

func TestPushButton(t *testing.T) {
	SkipTestIfNotTestRig(t)
	Info("Running %s", t.Name())
	wasRun := 0
	button := NewGpioButton(SWITCH, func() {
		Led.Output(High)
		wasRun++
		Info("Button Pushed %d!!!", wasRun)
	})

	button.Start()
	for i := 0; i < 3; i++ {
		TestRelay.TurnOn()
		time.Sleep(time.Second / 4)
		TestRelay.TurnOff()
		Led.Output(Low)
		time.Sleep(3 * time.Second)
	}
	if wasRun != 3 {
		t.Errorf("Expected 3 button pushes, detected %d", wasRun)
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

func TestBlinkLed(t *testing.T) {
	SkipTestIfNotTestRig(t)
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

func TestDischargeStrategies(t *testing.T) {
	SkipTestIfNotTestRig(t)
	t.Skip("Only for experimentation, not a real test")
	Info("Running %s", t.Name())
	therm := NewGpioThermometer("Fixed 4.7kOhm ResistorTest", "TestManufacturer", CAP4700)
	pulls := []Pull{PullDown, PullUp, Float}
	edges := []Edge{RisingEdge, FallingEdge, BothEdges}
	expected := r4700 * therm.microfarads / 2
	Info("Strategy: Pull, Edge, Expected, Average, Stddev, PctVar")
	for _, p := range pulls {
		for _, e := range edges {
			h := NewHistory(10)
			for i := 0; i < 10; i++ {
				dt := discharge_us(therm, e, p)
				h.Push(us(dt))
			}
			Info("Strategy: %s, %s, %0.3f, %0.3f, %0.4f, %0.2f",
				p, e, expected, h.Average(), h.Stddev(), 100.0*h.Stddev()/h.Average())
		}
	}
}

func TestThermometer(t *testing.T) {
	SkipTestIfNotTestRig(t)
	if testing.Short() {
		t.SkipNow()
	}
	Info("Running %s", t.Name())
	therm := NewGpioThermometer("Fixed 4.7kOhm ResistorTest", "TestManufacturer", CAP4700)

	t.Run("Calibrate Cap4700", func(t *testing.T) {
		Info("Running %s", t.Name())
		err := therm.Calibrate(r4700)
		if err != nil {
			t.Errorf("Failure to Calibrate successfully: %s", err.Error())
		}
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

	therm = NewGpioThermometer("Fixed 10kOhm ResistorTest", "TestManufacturer", CAP10000)
	t.Run("Calibrate Cap10000", func(t *testing.T) {
		Info("Running %s", t.Name())
		err := therm.Calibrate(r10000)
		if err != nil {
			t.Errorf("Failure to Calibrate successfully: %s", err.Error())
		}
	})
	t.Run("Temperature Cap10000", func(t *testing.T) {
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

func discharge_us(t *GpioThermometer, e Edge, p Pull) time.Duration {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	//Discharge the capacitor (low temps could make this really long)
	t.pin.Output(Low)
	time.Sleep(300 * time.Millisecond)

	// Start polling
	start := time.Now()
	t.pin.InputEdge(p, e)
	if !t.pin.WaitForEdge(time.Second / 2) {
		Trace("Thermometer %s, Rising read timed out", t.Name())
		return 0.0
	}
	stop := time.Now()
	t.pin.Output(Low)
	return stop.Sub(start)
}

func doStop(button *Button, b *bool, t time.Time) {
	*b = false
	button.Stop()
	*b = true
	Info("doStop - Stopped after %d ms", time.Now().Sub(t)/time.Millisecond)
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
}

func SkipTestIfNotTestRig(t *testing.T) {
	if TestRig {
		return
	}

	if runtime.GOOS != "linux" {
		t.SkipNow()
	}
	if runtime.GOARCH != "arm" {
		t.SkipNow()
	}

	if _, err := os.Stat("TestRig"); err == nil {
		TestRig = true
		return
	}
	t.SkipNow()
}

func ExpectedState(t *testing.T, gpio PiPin, exp GpioState) {
	if val := gpio.Read(); val != exp {
		t.Errorf("%s: Expected %s but found %s", GpioStr(gpio), exp, val)
	}
}
