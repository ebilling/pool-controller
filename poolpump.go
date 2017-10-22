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
	waterGpio       = 25
	roofGpio        = 24
	buttonGpio      = 18
)

type SolarVariables struct {
	target            float64
	deltaT            float64
	tolerance         float64
}

// The PoolPumpController manages the relays that control the pumps based on
// data from temperature probes and the weather.
type PoolPumpController struct {
	config        *Config
	zipcode       string
	weather       *Weather
	switches      *Switches
	pumpTemp      Thermometer
	runningTemp   Thermometer
	roofTemp      Thermometer
	solar         SolarVariables
	button        *Button
	tempRrd       *Rrd
	pumpRrd       *Rrd
	done          chan bool
}

func therm(config *Config, name string, gpio uint8) (*GpioThermometer) {
	cap := 10.0
	capName := fmt.Sprintf("capacitance.gpio.%d", gpio)
	if config.Contains(capName) {
		cap = config.GetFloat(capName)
	}
	return NewGpioThermometer(name, mftr, gpio, cap)
}

// Creates a thermometer that remembers the temperature of the water when the
// pumps were running.  This is more reprsentative of the actual water temperature,
// as the water temperature probe is near the pump, not actually in the pool.
func RunningWaterThermometer(t Thermometer, s *Switches) (*SelectiveThermometer) {
	return NewSelectiveThermometer("Pool", mftr, t, func () (bool) {
		return s.State() > STATE_OFF
	})
}

func NewPoolPumpController(config *Config) *PoolPumpController {
	ppc := PoolPumpController {
		config:     config,
		zipcode:    config.GetString(configZip),
		weather:    NewWeather(config.GetString(configAppId), 20 * time.Minute),
		switches:   NewSwitches(mftr),
		pumpTemp:  therm(config, "Pumphouse", waterGpio),
		roofTemp:   therm(config, "Poolhouse Roof", roofGpio),
		solar:      SolarVariables{
			target:    25.0,
			deltaT:    5.0,
			tolerance: 0.5,
		},
		tempRrd:    NewRrd(config.GetString("homekit.data")+"/temperature.rrd"),
		pumpRrd:    NewRrd(config.GetString("homekit.data")+"/pumpstatus.rrd"),
		done:       make(chan bool),
	}
	ppc.runningTemp = RunningWaterThermometer(ppc.pumpTemp, ppc.switches)
	return &ppc
}

// Updates the solar configuration parameters from the config file (if changed)
// and updates the values of the Thermometers.
func (ppc *PoolPumpController) Update() {
	ppc.config.Update()
	if ppc.config.Contains(configZip) {
		ppc.zipcode = ppc.config.GetString(configZip)
	}
	if ppc.config.Contains(configTarget) {
		ppc.solar.target = ppc.config.GetFloat(configTarget)
	}
	if ppc.config.Contains(configDeltaT) {
		ppc.solar.deltaT = ppc.config.GetFloat(configDeltaT)
	}
	if ppc.config.Contains(configTolerance) {
		ppc.solar.tolerance = ppc.config.GetFloat(configTolerance)
	}
	if ppc.config.Contains("Debug") && ppc.config.GetString("Debug") == "True" {
		EnableDebug()
	} else {
		DisableDebug()
	}
	ppc.pumpTemp.Update()
	ppc.roofTemp.Update()
	ppc.runningTemp.Update()
}

// A return value of 'True' indicates that the pool is too hot and the roof is cold
// (probably at night), running the pumps with solar on would help bring the water
// down to the target temperature.
func (ppc *PoolPumpController) shouldCool() bool {
	return  ppc.pumpTemp.Temperature() > ppc.solar.target + ppc.solar.tolerance &&
		ppc.pumpTemp.Temperature() > ppc.roofTemp.Temperature() + ppc.solar.deltaT
}

// A return value of 'True' indicates that the pool is too cool and the roof is hot, running
// the pumps with solar on would help bring the water up to the target temperature.
func (ppc *PoolPumpController) shouldWarm() bool {
	return  ppc.pumpTemp.Temperature() < ppc.solar.target - ppc.solar.tolerance &&
		ppc.pumpTemp.Temperature() < ppc.roofTemp.Temperature() - ppc.solar.deltaT
}

// If the water is not within the tolerance limit of the target, and the roof temperature would
// help get the temperature to be closer to the target, the pumps will be turned on.  If the
// outdoor temperature is low or the pool is very cold, the sweep will also be run to help mix
// the water as it approaches the target.
func (ppc *PoolPumpController) RunPumpsIfNeeded() {
	state := ppc.switches.State()
	if state == STATE_DISABLED || ppc.switches.ManualState() {
		return
	}
	if state > STATE_OFF {
		if ppc.switches.GetStartTime().Add(30 * time.Minute).After(time.Now()) {
			return // Don't bounce the motors, let them run
		}
	}
	temp := ppc.weather.GetCurrentTempC(ppc.zipcode)
	if ppc.shouldCool() || ppc.shouldWarm() {
		// Wide deltaT between target and temp or when it's cold, run sweep
		if ppc.pumpTemp.Temperature() < ppc.solar.target - ppc.solar.deltaT ||
			temp < ppc.solar.target || // Cool Weather
			ppc.pumpTemp.Temperature() > ppc.solar.target + ppc.solar.tolerance {
			ppc.switches.SetState(STATE_SOLAR_MIXING, false)
		} else {
			// Just push water through the panels
			ppc.switches.SetState(STATE_SOLAR, false)
		}
		return
	}
	if time.Now().Sub(ppc.switches.GetStopTime()) > 24 * time.Hour {
		ppc.switches.SetState(STATE_SWEEP, false) // Clean pool
		if time.Now().Sub(ppc.switches.GetStartTime()) > 2 * time.Hour {
			ppc.switches.StopAll(false) // End daily
		}
		return
	}
	if state > STATE_OFF {
		ppc.switches.StopAll(false)
	}
}

// Runs calls PoolPumpController.Update() and PoolPumpController.RunPumpsIfNeeded()
// repeatedly until PoolPumpController.Stop() is called
func (ppc *PoolPumpController) runLoop() {
	interval :=  5 * time.Second
	postStatus := time.Now()
	for tries := 0; true; tries++ {
		if postStatus.Before(time.Now()) {
			postStatus = time.Now().Add(5 * time.Minute)
			Info(ppc.Status())
		}
		select {
		case <- ppc.done:
			ppc.button.Stop()
			// Turn off the pumps, and don't let them turn back on
			ppc.switches.Disable()
			break
		case <- time.After(interval):
			ppc.Update()
			ppc.RunPumpsIfNeeded()
			ppc.UpdateRrd()
		}
	}
}

// Finishes initializing the PoolPumpController, and kicks off the control thread.
func (ppc *PoolPumpController) Start(force bool) {
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
	// Initialize RRDs
	ppc.createRrds(force)

	// Start go routines
	ppc.Update()
	ppc.button.Start()
	go ppc.runLoop()
}

func (ppc *PoolPumpController) Stop() {
	ppc.done <- true
}

func (ppc *PoolPumpController) WeatherC() float64 {
	return ppc.weather.GetCurrentTempC(ppc.zipcode)
}

func (ppc *PoolPumpController) Status() string {
	return fmt.Sprintf(
		"Status(%s) Solar(%s) Pump(%s) Sweep(%s) Manual(%t) " +
		"Pool(%0.1f) Pump(%0.1f) Roof(%0.1f) CurrentTemp(%0.1f)",
		ppc.switches.State(), ppc.switches.solar.Status(),
		ppc.switches.pump.Status(), ppc.switches.sweep.Status(),
		ppc.switches.ManualState(),
		ppc.runningTemp.Temperature(), ppc.pumpTemp.Temperature(),
		ppc.roofTemp.Temperature(), ppc.WeatherC())
}
