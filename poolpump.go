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
	buttonGpio      = 8
)

type SolarVariables struct {
	target            float64
	deltaT            float64
	tolerance         float64
}

type PoolPumpController struct {
	config        *Config
	weather       *Weather
	switches      *Switches
	waterTemp     Thermometer
	runningTemp   Thermometer
	roofTemp      Thermometer
	solar         SolarVariables
	button        *Button
	pin           string
	done          chan bool
}

func therm(config *Config, name string, gpio uint32) (*GpioThermometer) {
	cap := 10.0
	capName := fmt.Sprintf("capacitance.gpio.%d", gpio)
	if config.Contains(capName) {
		cap = config.GetFloat(capName)
	}
	return NewGpioThermometer(name, mftr, gpio, cap)
}

func RunningWaterThermometer(t Thermometer, s *Switches) (*SelectiveThermometer) {
	return NewSelectiveThermometer("Cached Pool Temp", mftr, t, func () (bool) {
		return s.State() > STATE_OFF
	})
}

func NewPoolPumpController(path string) *PoolPumpController {
	config := NewConfig(path)
	ppc := PoolPumpController {
		config:     config,
		weather:    NewWeather(
			config.GetString(configAppId), 15 * time.Minute),
		switches:   NewSwitches(mftr),
		waterTemp:  therm(config, "Water Temp", waterGpio),
		roofTemp:   therm(config, "Roof Temp", roofGpio),
		solar:      SolarVariables{
			target: 25.0,
			deltaT: 5.0,
			tolerance: 0.5,
		},
		done:       make(chan bool),
	}
	ppc.pin = config.Get("homekit.pin").(string)
	ppc.runningTemp = RunningWaterThermometer(ppc.waterTemp, ppc.switches)
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
	ppc.runningTemp.Update()
}

func (ppc *PoolPumpController) RunLoop() {
	interval := 10 * time.Second
	for tries := 0; true; tries++ {
		if tries % 10 == 0 {
			Info(ppc.Status())
		}
		select {
		case <- time.After(interval):
			ppc.Update()
			ppc.RunPumpsIfNeeded()
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

func (ppc *PoolPumpController) RunPumpsIfNeeded() {
	state := ppc.switches.State()
	if state == STATE_DISABLED || ppc.switches.ManualState() {
		return
	}
	temp := ppc.weather.GetCurrentTempC(ppc.config.Get(configZip).(string))
	if ppc.shouldCool() || ppc.shouldWarm() {
		// Wide deltaT between target and temp or when it's cold, run sweep 
		if ppc.waterTemp.Temperature() < ppc.solar.target - ppc.solar.deltaT ||
			temp < ppc.solar.target ||
			ppc.waterTemp.Temperature() > ppc.solar.target + ppc.solar.tolerance {
			ppc.switches.SetState(STATE_SOLAR_MIXING, false)
		} else {
			ppc.switches.SetState(STATE_SOLAR, false)
		}
		return
	}
	if state > STATE_OFF {
		ppc.switches.StopAll(false)
	}
}

func (ppc *PoolPumpController) Start() {	
	ppc.button = NewGpioButton(buttonGpio, func() {
		switch ppc.switches.State() {
		case STATE_DISABLED:
			break
		case STATE_OFF:
			ppc.switches.SetState(STATE_PUMP, true)
			break
		case STATE_PUMP:
			ppc.switches.SetState(STATE_SWEEP, true)
			break
		case STATE_SOLAR:
			ppc.switches.SetState(STATE_SOLAR_MIXING, true)
			break
		default:
			ppc.switches.SetState(STATE_OFF, true)
		}
	})
	ppc.button.Start()
	go ppc.RunLoop()
}

func (ppc *PoolPumpController) Stop() {
	ppc.done <- true
}

func (ppc *PoolPumpController) Status() string {
	temp := ppc.weather.GetCurrentTempC(ppc.config.Get(configZip).(string))
	return fmt.Sprintf(
		"CurrentTemp(%0.1f) Pool(%0.1f) Solar(%s) Pump(%s) Sweep(%s) Water(%0.1) Roof(0.1f)",
		temp, ppc.switches.pump.Status(),
		ppc.switches.sweep.Status(), ppc.switches.solar.Status(),
		ppc.waterTemp.Temperature(), ppc.roofTemp.Temperature())
}
