package main

import (
	"errors"
	"flag"
	"testing"
	"time"
)

func restoreGpio(t *testing.T) {
	t.Helper()
	origProvider := gpioProvider
	origInit := gpioInitFn
	t.Cleanup(func() {
		gpioProvider = origProvider
		gpioInitFn = origInit
	})
}

func TestEnableSimulationSkipsHostInit(t *testing.T) {
	restoreGpio(t)
	EnableSimulation()
	if err := GpioInit(); err != nil {
		t.Fatalf("simulated GpioInit: %v", err)
	}
	pin := NewGpio(pumpGpio)
	pin.Output(High)
	if pin.Read() != High {
		t.Fatal("expected sim pin to retain output state")
	}
	if pin.Pin() != pumpGpio {
		t.Fatalf("pin number: got %d", pin.Pin())
	}
}

func TestSimPinWaitForEdgeTimesOut(t *testing.T) {
	restoreGpio(t)
	EnableSimulation()
	pin := NewGpio(buttonGpio)
	start := time.Now()
	if pin.WaitForEdge(20 * time.Millisecond) {
		t.Fatal("simulated button should not see an edge")
	}
	if time.Since(start) < 15*time.Millisecond {
		t.Fatal("WaitForEdge should honor the timeout when no edge is scripted")
	}
}

func TestSimulatedThermometer(t *testing.T) {
	th := NewSimulatedThermometer("Pump", mftr, 24.0)
	if th.Name() != "Pump" {
		t.Fatalf("name: %s", th.Name())
	}
	if th.Temperature() != 24.0 {
		t.Fatalf("temp: %v", th.Temperature())
	}
	if err := th.Update(); err != nil {
		t.Fatal(err)
	}
	th.SetTemperature(18.5)
	if th.Temperature() != 18.5 {
		t.Fatalf("updated temp: %v", th.Temperature())
	}
	th.updateErr = errors.New("sensor missing")
	if err := th.Update(); err == nil {
		t.Fatal("expected scripted update error")
	}
	if th.Accessory() == nil {
		t.Fatal("missing HomeKit accessory")
	}
}

func TestNewPoolPumpControllerSimulate(t *testing.T) {
	restoreGpio(t)
	EnableSimulation()
	fs := flag.NewFlagSet("sim-ppc", flag.PanicOnError)
	cfg := NewConfig(fs, []string{
		"-simulate",
		"-sim-pump-temp", "22.5",
		"-sim-roof-temp", "48",
		"-data_dir", t.TempDir(),
	})
	ppc := NewPoolPumpController(cfg)
	if _, ok := ppc.pumpTemp.(*SimulatedThermometer); !ok {
		t.Fatalf("pump thermometer type %T", ppc.pumpTemp)
	}
	if _, ok := ppc.roofTemp.(*SimulatedThermometer); !ok {
		t.Fatalf("roof thermometer type %T", ppc.roofTemp)
	}
	if err := ppc.pumpTemp.Update(); err != nil {
		t.Fatal(err)
	}
	if err := ppc.roofTemp.Update(); err != nil {
		t.Fatal(err)
	}
	if ppc.pumpTemp.Temperature() != 22.5 {
		t.Fatalf("pump temp %v", ppc.pumpTemp.Temperature())
	}
	if ppc.roofTemp.Temperature() != 48 {
		t.Fatalf("roof temp %v", ppc.roofTemp.Temperature())
	}
}
