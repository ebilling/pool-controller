package main

import (
	"flag"
	"fmt"
	"github.com/brutella/hc/accessory"
	"testing"
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
	*t.ppc.config.target = target
	t.pumpTemp.temp = pump
	t.roofTemp.temp = roof
	t.service.temp = outside
	t.ppc.switches.state = state
	t.ppc.weather.cache = make(map[string]*WeatherData)
}

func NewTestRunPumps() *TestRunPumps {
	config := NewConfig(flag.NewFlagSet("TestPumpController", flag.PanicOnError), []string{})
	t := TestRunPumps{
		pumpTemp: FakeThermometer{name: "pool", temp: 0.0},
		roofTemp: FakeThermometer{name: "roof", temp: 0.0},
		service:  FakeWeatherService{temp: 0.0, radiation: 0.0},
		ppc:      NewPoolPumpController(config),
	}
	t.ppc.pumpTemp = &t.pumpTemp
	t.ppc.roofTemp = &t.roofTemp
	t.ppc.weather.service = &t.service
	return &t
}

func TestShouldCoolorWarm(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()

	t.Run("ColdWater,HotWeather", func(t *testing.T) {
		trp.setConditions(30.0, 15.0, 50.0, 33.0, STATE_OFF)
		if trp.ppc.shouldCool() {
			t.Error("Should not be cooling")
		}
		if !trp.ppc.shouldWarm() {
			t.Error("Should be trying to warm the pool")
		}
	})

	t.Run("WarmWater,HotWeather", func(t *testing.T) {
		trp.setConditions(30.0, 29.98, 50.0, 33.0, STATE_OFF)
		if trp.ppc.shouldCool() {
			t.Error("Should not be cooling")
		}
		if trp.ppc.shouldWarm() {
			t.Error("Should not try to warm water that is already so close to the target")
		}
	})

	t.Run("HotWater,HotWeather", func(t *testing.T) {
		trp.setConditions(30.0, 29.98, 50.0, 33.0, STATE_OFF)
		if trp.ppc.shouldCool() {
			t.Error("Should not be cooling")
		}
		if trp.ppc.shouldWarm() {
			t.Error("Should not try to warm water that is already so close to the target")
		}
	})

	t.Run("ColdWater,WarmWeather", func(t *testing.T) {
		trp.setConditions(30.0, 15.0, 40.0, 29.0, STATE_OFF)
		if trp.ppc.shouldCool() {
			t.Error("Should not be cooling")
		}
		if !trp.ppc.shouldWarm() {
			t.Error("Should be trying to warm the pool")
		}
	})

	t.Run("WarmWater,WarmWeather", func(t *testing.T) {
		trp.setConditions(30.0, 29.98, 40.0, 29.0, STATE_OFF)
		if trp.ppc.shouldCool() {
			t.Error("Should not be cooling")
		}
		if trp.ppc.shouldWarm() {
			t.Error("Should not try to warm water that is already so close to the target")
		}
	})

	t.Run("HotWater,WarmWeather", func(t *testing.T) {
		trp.setConditions(30.0, 29.98, 40.0, 29.0, STATE_OFF)
		if trp.ppc.shouldCool() {
			t.Error("Should not be cooling")
		}
		if trp.ppc.shouldWarm() {
			t.Error("Should not try to warm water that is already so close to the target")
		}
	})

}

func TestRunPumpsIfNeeded(t *testing.T) {
	SetGpioProvider(NewTestPin)
	//trp := NewTestRunPumps()

	t.Run("", func(t *testing.T) {
	})

}
