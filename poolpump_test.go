package main

import (
	"flag"
	"testing"

	"github.com/brutella/hc/accessory"
)

type FakeThermometer struct {
	name           string
	temp           float64
	updateError    error
	calibrateError error
	acc            *accessory.Thermometer
}

func (t *FakeThermometer) Name() string {
	return t.name
}
func (t *FakeThermometer) Temperature() float64 {
	return t.temp
}
func (t *FakeThermometer) Update() error {
	return t.updateError
}
func (t *FakeThermometer) Calibrate(float64) error {
	return t.calibrateError
}
func (t *FakeThermometer) Accessory() *accessory.Accessory {
	if t.acc == nil {
		t.acc = accessory.NewTemperatureSensor(AccessoryInfo(t.name, "Unit Testing Intl"),
			0.0, -20.0, 100.0, 1.0)
	}
	return t.acc.Accessory
}

type TestRunPumps struct {
	pumpTemp FakeThermometer
	roofTemp FakeThermometer
	ppc      *PoolPumpController
}

func (t *TestRunPumps) setConditions(target, pump, roof float64, state State) {
	t.ppc.config.cfg.Target = target
	t.pumpTemp.temp = pump
	t.roofTemp.temp = roof
	t.ppc.switches.state = state
}

func NewTestRunPumps() *TestRunPumps {
	defaultDataDir = "/tmp"
	config := NewConfig(flag.NewFlagSet("TestPumpController", flag.PanicOnError), []string{})
	t := TestRunPumps{
		pumpTemp: FakeThermometer{name: "pool", temp: 0.0},
		roofTemp: FakeThermometer{name: "roof", temp: 0.0},
		ppc:      NewPoolPumpController(config),
	}
	t.ppc.pumpTemp = &t.pumpTemp
	t.ppc.roofTemp = &t.roofTemp
	return &t
}

func TestColdWaterHotWeather(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30.0, 15.0, 50.0, OFF)
	if trp.ppc.shouldCool() {
		t.Error("Should not be cooling")
	}
	if !trp.ppc.shouldWarm() {
		t.Error("Should be trying to warm the pool")
	}
}

func TestWarmWaterHotWeather(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30.0, 29.98, 50.0, OFF)
	if trp.ppc.shouldCool() {
		t.Error("Should not be cooling")
	}
	if trp.ppc.shouldWarm() {
		t.Error("Should not try to warm water that is already so close to the target")
	}
}

func TestHotWaterHotWeather(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30.0, 29.98, 50.0, OFF)
	if trp.ppc.shouldCool() {
		t.Error("Should not be cooling")
	}
	if trp.ppc.shouldWarm() {
		t.Error("Should not try to warm water that is already so close to the target")
	}
}

func TestColdWaterWarmWeather(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30.0, 15.0, HOTROOF+0.01, OFF)
	if trp.ppc.shouldCool() {
		t.Error("Should not be cooling")
	}
	if !trp.ppc.shouldWarm() {
		t.Error("Should be trying to warm the pool")
	}
}

func TestWarmWaterWarmWeather(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30.0, 29.98, 40.0, OFF)
	if trp.ppc.shouldCool() {
		t.Error("Should not be cooling")
	}
	if trp.ppc.shouldWarm() {
		t.Error("Should not try to warm water that is already so close to the target")
	}
}

func TestHotWaterWarmWeather(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30.0, 29.98, 40.0, OFF)
	if trp.ppc.shouldCool() {
		t.Error("Should not be cooling")
	}
	if trp.ppc.shouldWarm() {
		t.Error("Should not try to warm water that is already so close to the target")
	}
}

func TestRunPumpsIfNeeded(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()

	testdata := []struct {
		name   string
		target float64
		pump   float64
		roof   float64
		state  State
	}{
		{"", 30.0, 15.0, 50.0, MIXING},
		{"", 30.0, 27.0, 50.0, SOLAR},
		{"", 30.0, 29.98, 50.0, OFF},
		{"", 30.0, 15.0, 40.0, MIXING},
		{"", 30.0, 29.98, 30.0, OFF},
		{"", 30.0, 29.98, 20.0, OFF},
	}

	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			trp.setConditions(td.target, td.pump, td.roof, td.state)
			trp.ppc.RunPumpsIfNeeded()
			if trp.ppc.switches.state != td.state {
				t.Errorf("Expected state %v, got %v", td.state, trp.ppc.switches.state)
			}
		})
	}
}
