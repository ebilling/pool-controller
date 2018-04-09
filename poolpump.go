package main

import (
	"fmt"
	"time"
)

const (
	mftr       = "Bonnie Labs"
	waterGpio  = 25
	roofGpio   = 24
	buttonGpio = 18
)

type SolarVariables struct {
	target    *float64
	deltaT    *float64
	tolerance *float64
}

// The PoolPumpController manages the relays that control the pumps based on
// data from temperature probes and the weather.
type PoolPumpController struct {
	config      *Config
	zipcode     string
	weather     *Weather
	switches    *Switches
	pumpTemp    Thermometer
	runningTemp Thermometer
	roofTemp    Thermometer
	solar       SolarVariables
	button      *Button
	tempRrd     *Rrd
	pumpRrd     *Rrd
	done        chan bool
}

// Creates a thermometer that remembers the temperature of the water when the
// pumps were running.  This is more reprsentative of the actual water temperature,
// as the water temperature probe is near the pump, not actually in the pool.
func RunningWaterThermometer(t Thermometer, s *Switches) *SelectiveThermometer {
	return NewSelectiveThermometer("Pool", mftr, t, func() bool {
		return s.State() > STATE_OFF
	})
}

func NewPoolPumpController(config *Config) *PoolPumpController {
	ppc := PoolPumpController{
		config:   config,
		zipcode:  *config.zip,
		weather:  NewWeather(*config.WUappId, 20*time.Minute),
		switches: NewSwitches(mftr),
		pumpTemp: NewGpioThermometer("Pumphouse", mftr, waterGpio),
		roofTemp: NewGpioThermometer("Poolhouse Roof", mftr, roofGpio),
		solar: SolarVariables{
			target:    config.target,
			deltaT:    config.deltaT,
			tolerance: config.tolerance,
		},
		tempRrd: NewRrd(*config.data_dir + "/temperature.rrd"),
		pumpRrd: NewRrd(*config.data_dir + "/pumpstatus.rrd"),
		done:    make(chan bool),
	}
	ppc.SyncAdjustments()
	ppc.runningTemp = RunningWaterThermometer(ppc.pumpTemp, ppc.switches)
	return &ppc
}

// Updates the solar configuration parameters from the config file (if changed)
// and updates the values of the Thermometers.
func (ppc *PoolPumpController) Update() {
	ppc.config.Save(*ppc.config.data_dir + server_conf)
	ppc.pumpTemp.Update()
	ppc.roofTemp.Update()
	ppc.runningTemp.Update()
	if *ppc.config.button_disabled {
		ppc.button.Disable()
	} else {
		ppc.button.Enable()
	}
}

// A return value of 'True' indicates that the pool is too hot and the roof is cold
// (probably at night), running the pumps with solar on would help bring the water
// down to the target temperature.
func (ppc *PoolPumpController) shouldCool() bool {
	if *ppc.config.solar_disabled {
		return false
	}
	return ppc.pumpTemp.Temperature() > *ppc.solar.target+*ppc.solar.tolerance &&
		ppc.pumpTemp.Temperature() > ppc.roofTemp.Temperature()+*ppc.solar.deltaT
}

// A return value of 'True' indicates that the pool is too cool and the roof is hot, running
// the pumps with solar on would help bring the water up to the target temperature.
func (ppc *PoolPumpController) shouldWarm() bool {
	if *ppc.config.solar_disabled {
		return false
	}
	return ppc.pumpTemp.Temperature() < *ppc.solar.target-*ppc.solar.tolerance &&
		ppc.pumpTemp.Temperature() < ppc.roofTemp.Temperature()-*ppc.solar.deltaT
}

// If the water is not within the tolerance limit of the target, and the roof temperature would
// help get the temperature to be closer to the target, the pumps will be turned on.  If the
// outdoor temperature is low or the pool is very cold, the sweep will also be run to help mix
// the water as it approaches the target.
func (ppc *PoolPumpController) RunPumpsIfNeeded() {
	state := ppc.switches.State()
	if ppc.switches.ManualState() {
		return
	}
	if state == STATE_DISABLED && !*ppc.config.disabled && !*ppc.config.solar_disabled {
		ppc.switches.setSwitches(false, false, false, false, STATE_OFF)
		return
	}
	if *ppc.config.disabled {
		if state > STATE_DISABLED {
			ppc.switches.setSwitches(false, false, false, false, STATE_DISABLED)
		}
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
		if ppc.pumpTemp.Temperature() < *ppc.solar.target-*ppc.solar.deltaT ||
			temp < *ppc.solar.target || // Cool Weather
			ppc.pumpTemp.Temperature() > *ppc.solar.target+*ppc.solar.tolerance {
			ppc.switches.SetState(STATE_SOLAR_MIXING, false)
		} else {
			// Just push water through the panels
			ppc.switches.SetState(STATE_SOLAR, false)
		}
		return
	}
	// If the pumps havent run in a day, wait til midnight then start them
	if time.Now().Sub(ppc.switches.GetStopTime()) > 24*time.Hour && time.Now().Hour() < 5 {
		ppc.switches.SetState(STATE_SWEEP, false) // Clean pool
		if time.Now().Sub(ppc.switches.GetStartTime()) > 2*time.Hour {
			ppc.switches.StopAll(false) // End daily
		}
		return
	}
	// If there is no reason to turn on the pumps and it's not manual, turn off
	if state > STATE_OFF {
		ppc.switches.StopAll(false)
	}
}

// Runs calls PoolPumpController.Update() and PoolPumpController.RunPumpsIfNeeded()
// repeatedly until PoolPumpController.Stop() is called
func (ppc *PoolPumpController) runLoop() {
	interval := 5 * time.Second
	postStatus := time.Now()
	keepRunning := true
	for keepRunning {
		if postStatus.Before(time.Now()) {
			postStatus = time.Now().Add(5 * time.Minute)
			Info(ppc.Status())
		}
		select {
		case <-ppc.done:
			ppc.button.Stop()
			// Turn off the pumps, and don't let them turn back on
			ppc.switches.Disable()
			keepRunning = false
			break
		case <-time.After(interval):
			ppc.Update()
			ppc.RunPumpsIfNeeded()
			ppc.UpdateRrd()
		}
	}
	Info("Exiting Controller")
}

// Finishes initializing the PoolPumpController, and kicks off the control thread.
func (ppc *PoolPumpController) Start() {
	ppc.button = NewGpioButton(buttonGpio, func() {
		switch ppc.switches.State() {
		case STATE_OFF:
			ppc.switches.SetState(STATE_PUMP, true)
		case STATE_PUMP:
			ppc.switches.SetState(STATE_SWEEP, true)
		case STATE_SOLAR:
			ppc.switches.SetState(STATE_SOLAR_MIXING, true)
		case STATE_DISABLED:
		default:
			ppc.switches.SetState(STATE_OFF, true)
		}
	})
	// Initialize RRDs
	ppc.createRrds()

	// Start go routines
	ppc.Update()
	ppc.button.Start()
	go ppc.runLoop()
}

func (ppc *PoolPumpController) Stop() {
	ppc.switches.StopAll(true)
	ppc.done <- true
}

func (ppc *PoolPumpController) PersistCalibration() {
	t, ok := ppc.pumpTemp.(*GpioThermometer)
	if ok {
		*ppc.config.adj_pump = t.adjust
	}
	t, ok = ppc.roofTemp.(*GpioThermometer)
	if ok {
		*ppc.config.adj_roof = t.adjust
	}
}

func (ppc *PoolPumpController) SyncAdjustments() {
	t, ok := ppc.pumpTemp.(*GpioThermometer)
	if ok {
		t.adjust = *ppc.config.adj_pump
	}
	t, ok = ppc.roofTemp.(*GpioThermometer)
	if ok {
		t.adjust = *ppc.config.adj_roof
	}
}

func (ppc *PoolPumpController) WeatherC() float64 {
	return ppc.weather.GetCurrentTempC(ppc.zipcode)
}

func (ppc *PoolPumpController) Status() string {
	return fmt.Sprintf(
		"Status(%s) Button(%s) Solar(%s) Pump(%s) Sweep(%s) Manual(%t) Target(%0.1f) "+
			"Pool(%0.1f) Pump(%0.1f) Roof(%0.1f) CurrentTemp(%0.1f)",
		ppc.switches.State(), ppc.button.pin.Read(), ppc.switches.solar.Status(),
		ppc.switches.pump.Status(), ppc.switches.sweep.Status(),
		ppc.switches.ManualState(), *ppc.config.target,
		ppc.runningTemp.Temperature(), ppc.pumpTemp.Temperature(),
		ppc.roofTemp.Temperature(), ppc.WeatherC())
}
