package main

import (
	"fmt"
	"time"
)

const (
	mftr            = "Bonnie Labs"
	configTarget    = "temp.target"
	configDeltaT    = "temp.minDeltaT"
	configTolerance = "temp.tolerance"
	configAppId     = "weather.appid"
	configZip       = "weather.zip"
	waterGpio       = 24
	roofGpio        = 25
)

type SolarVariables struct {
	target            float64
	deltaT            float64
	tolerance         float64
}

type PoolPumpController struct {
	config      *Config
	weather     *Weather
	switches    *Switches
	waterTemp   Thermometer
	roofTemp    Thermometer
	solar       SolarVariables
	pin         string
	done        chan bool
}

func Therm(config *Config, name string, gpio uint32) (*GpioThermometer) {
	cap := 10.0
	capName := fmt.Sprintf("capacitance.gpio.%d", gpio)
	if config.Contains(capName) {
		cap = config.GetFloat(capName)
	}
	return NewGpioThermometer(name, mftr, gpio, cap)
}

func NewPoolPumpController(path string) *PoolPumpController {
	config := NewConfig(path)
	ppc := PoolPumpController {
		config:     config,
		weather:    NewWeather(config.GetString(configAppId), 15 * time.Minute),
		switches:   NewSwitches(mftr),
		waterTemp:  Therm(config, "Water Temp", waterGpio),
		roofTemp:   Therm(config, "Roof Temp", roofGpio),
		solar:      SolarVariables{target: 25.0, deltaT: 5.0, tolerance: 0.5},
		done:       make(chan bool),
	}
	ppc.pin = config.Get("homekit.pin").(string)
	Info("Homekit Pin: %s", ppc.pin)
	return &ppc
}

func (ppc *PoolPumpController) Update() {
	ppc.config.Update()
	if ppc.config.Contains(configTarget) {
		ppc.solar.target = ppc.config.GetFloat(configTarget)
	}
	if ppc.config.Contains(configDeltaT) {
		ppc.solar.deltaT = ppc.config.GetFloat(configDeltaT)
	}
	if ppc.config.Contains(configTolerance) {
		ppc.solar.tolerance = ppc.config.GetFloat(configTolerance)
	}
	ppc.waterTemp.Update()
	ppc.roofTemp.Update()
}

func (ppc *PoolPumpController) RunLoop() {
	interval := 10 * time.Second
	for tries := 0; true; tries++ {
		if tries % 10 == 0 {
			Info("Homekit service for PoolPumpController still running")
		}
		select {
		case <- time.After(interval):
			ppc.Update()
		case <- ppc.done:
			break
		}
	}
}

func (ppc *PoolPumpController) shouldCool() bool {
	return  ppc.waterTemp.Temperature() > ppc.solar.target + ppc.solar.tolerance &&
		ppc.waterTemp.Temperature() > ppc.roofTemp.Temperature() + ppc.solar.deltaT
}

func (ppc *PoolPumpController) shouldWarm() bool {
	return  ppc.waterTemp.Temperature() < ppc.solar.target - ppc.solar.tolerance &&
		ppc.waterTemp.Temperature() < ppc.roofTemp.Temperature() - ppc.solar.deltaT
}

func (ppc *PoolPumpController) StartSolarIfNeeded() {
	state := ppc.switches.GetState()
	temp := ppc.weather.GetCurrentTempC(ppc.config.Get(configZip).(string))
	if state == STATE_DISABLED || ppc.switches.ManualState() {
		return
	}
	if ppc.shouldCool() || ppc.shouldWarm() {
		// Wide deltaT between target and temp or when it's cold, run sweep 
		if ppc.waterTemp.Temperature() < ppc.solar.target - ppc.solar.deltaT ||
			temp < ppc.solar.target ||
			ppc.waterTemp.Temperature() > ppc.solar.target + ppc.solar.tolerance {
			ppc.switches.StartSolarMixing()
		} else {
			ppc.switches.StartSolar()
		}
	}
}

func (ppc *PoolPumpController) Start() {
	go ppc.RunLoop()
}

func (ppc *PoolPumpController) Stop() {
	ppc.done <- true
}
