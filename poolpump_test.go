package main

import (
	"flag"
	"fmt"
	"testing"

	"github.com/brutella/hc/accessory"
	"github.com/ebilling/pool-controller/weather"
	"github.com/ebilling/pool-controller/weather/fakeWeather"
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

type FakeWeatherService struct {
	temp      float64
	radiation float64
}

func (t *FakeWeatherService) Read(ignoredURL string) string {
	return fmt.Sprintf("{\"current_observation\":{\"temp_c\":%0.1f, \"solarradiation\":\"%0.1f\"}}	",
		t.temp, t.radiation)
}

type TestRunPumps struct {
	pumpTemp FakeThermometer
	roofTemp FakeThermometer
	service  FakeWeatherService
	ppc      *PoolPumpController
}

func (t *TestRunPumps) setConditions(target, pump, roof, outside float64, state State) {
	t.ppc.config.cfg.Target = target
	t.pumpTemp.temp = pump
	t.roofTemp.temp = roof
	t.service.temp = outside
	t.ppc.switches.state = state
	t.ppc.weather = weather.NewWeatherFromService(&fakeweather.TestService{})
}

func NewTestRunPumps() *TestRunPumps {
	defaultDataDir = "/tmp"
	config := NewConfig(flag.NewFlagSet("TestPumpController", flag.PanicOnError), []string{})
	t := TestRunPumps{
		pumpTemp: FakeThermometer{name: "pool", temp: 0.0},
		roofTemp: FakeThermometer{name: "roof", temp: 0.0},
		service:  FakeWeatherService{temp: 0.0, radiation: 0.0},
		ppc:      NewPoolPumpController(config),
	}
	t.ppc.pumpTemp = &t.pumpTemp
	t.ppc.roofTemp = &t.roofTemp
	t.ppc.weather = weather.NewWeatherFromService(&fakeweather.TestService{})
	return &t
}

func TestColdWaterHotWeather(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30.0, 15.0, 50.0, 33.0, STATE_OFF)
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
	trp.setConditions(30.0, 29.98, 50.0, 33.0, STATE_OFF)
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
	trp.setConditions(30.0, 29.98, 50.0, 33.0, STATE_OFF)
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
	trp.setConditions(30.0, 15.0, 40.0, 29.0, STATE_OFF)
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
	trp.setConditions(30.0, 29.98, 40.0, 29.0, STATE_OFF)
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
	trp.setConditions(30.0, 29.98, 40.0, 29.0, STATE_OFF)
	if trp.ppc.shouldCool() {
		t.Error("Should not be cooling")
	}
	if trp.ppc.shouldWarm() {
		t.Error("Should not try to warm water that is already so close to the target")
	}
}

func TestRunPumpsIfNeeded(t *testing.T) {
	SetGpioProvider(NewTestPin)
	//trp := NewTestRunPumps()

	t.Run("", func(t *testing.T) {
	})

}
