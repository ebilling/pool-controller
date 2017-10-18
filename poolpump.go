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

// Creates a thermometer that remembers the temperature of the water when the
// pumps were running.  This is more reprsentative of the actual water temperature,
// as the water temperature probe is near the pump, not actually in the pool.
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

// Updates the solar configuration parameters from the config file (if changed)
// and updates the values of the Thermometers.
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

// Writes updates to RRD files and generates cached graphs
func (ppc *PoolPumpController) UpdateRrd() {

}

// A return value of 'True' indicates that the pool is too hot and the roof is cold
// (probably at night), running the pumps with solar on would help bring the water
// down to the target temperature.
func (ppc *PoolPumpController) shouldCool() bool {
	return  ppc.waterTemp.Temperature() > ppc.solar.target + ppc.solar.tolerance &&
		ppc.waterTemp.Temperature() > ppc.roofTemp.Temperature() + ppc.solar.deltaT
}

// A return value of 'True' indicates that the pool is too cool and the roof is hot, running
// the pumps with solar on would help bring the water up to the target temperature.
func (ppc *PoolPumpController) shouldWarm() bool {
	return  ppc.waterTemp.Temperature() < ppc.solar.target - ppc.solar.tolerance &&
		ppc.waterTemp.Temperature() < ppc.roofTemp.Temperature() - ppc.solar.deltaT
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

// Runs calls PoolPumpController.Update() and PoolPumpController.RunPumpsIfNeeded()
// repeatedly until PoolPumpController.Stop() is called
func (ppc *PoolPumpController) runLoop() {
	interval := 10 * time.Second
	for tries := 0; true; tries++ {
		if tries % 10 == 0 {
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
	ppc.Update()
	ppc.button.Start()
	go ppc.runLoop()
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
