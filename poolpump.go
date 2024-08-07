package main

import (
	"fmt"
	"time"
)

const (
	mftr = "Bonnie Labs"

	// Do not use GPIO4 for thermistors

	roofGpio     = 14
	waterGpio    = 15
	buttonGpio   = 18
	solarLedGpio = 21
	solarFwdGpio = 22
	solarRevGpio = 23
	pumpGpio     = 24
	sweepGpio    = 25

	// HOTROOF is the temperature at which the roof is considered hot enough
	// to run the pumps if heating is needed
	HOTROOF = 47.5 // 117F
	// COLDROOF is the temperature at which the roof is considered cold enough
	// to run the pumps if cooling is needed
	COLDROOF       = 18.5 // 65F
	solarMotorTime = 30 * time.Second
)

// The PoolPumpController manages the relays that control the pumps based on
// data from temperature probes.
type PoolPumpController struct {
	config      *Config
	switches    *Switches
	pumpTemp    Thermometer
	runningTemp Thermometer
	roofTemp    Thermometer
	button      *Button
	tempRrd     *Rrd
	pumpRrd     *Rrd
	done        chan bool
}

// RunningWaterThermometer creates a thermometer that remembers the temperature of the water when the
// pumps were running.  This is more reprsentative of the actual water temperature,
// as the water temperature probe is near the pump, not actually in the pool.
func RunningWaterThermometer(t Thermometer, s *Switches) *SelectiveThermometer {
	return NewSelectiveThermometer("Pool", mftr, t, func() bool {
		return s.State() > OFF
	})
}

// NewPoolPumpController creates a new pump controller
func NewPoolPumpController(config *Config) *PoolPumpController {
	ppc := PoolPumpController{
		config:   config,
		switches: NewSwitches(mftr),
		pumpTemp: NewGpioThermometer("Pump", mftr, waterGpio),
		roofTemp: NewGpioThermometer("Roof", mftr, roofGpio),
		tempRrd:  NewRrd(*config.dataDirectory + "/temperature.rrd"),
		pumpRrd:  NewRrd(*config.dataDirectory + "/pumpstatus.rrd"),
		done:     make(chan bool),
	}
	ppc.SyncAdjustments()
	ppc.runningTemp = RunningWaterThermometer(ppc.pumpTemp, ppc.switches)
	return &ppc
}

// Update the solar configuration parameters from the config file (if changed)
// and updates the values of the Thermometers.
func (ppc *PoolPumpController) Update() error {
	err := ppc.pumpTemp.Update()
	if err != nil {
		return fmt.Errorf("pump temp update failed: %w", err)
	}
	err = ppc.roofTemp.Update()
	if err != nil {
		return fmt.Errorf("roof temp update failed: %w", err)
	}
	err = ppc.runningTemp.Update()
	if err != nil {
		return fmt.Errorf("running temp update failed: %w", err)
	}
	if ppc.config.cfg.ButtonDisabled {
		ppc.button.Disable()
	} else {
		ppc.button.Enable()
	}
	return nil
}

// A return value of 'True' indicates that the pool is too hot and the roof is cold
// (probably at night), running the pumps with solar on would help bring the water
// down to the target temperature.
func (ppc *PoolPumpController) shouldCool() bool {
	if ppc.config.cfg.SolarCoolingDisabled {
		return false
	}
	if ppc.config.cfg.SolarDisabled {
		Debug("shouldCool: disabled(%t)", ppc.config.cfg.SolarDisabled)
		return false
	}
	waterHot := ppc.pumpTemp.Temperature() > ppc.config.cfg.Target+ppc.config.cfg.Tolerance
	roofCold := ppc.roofTemp.Temperature() < COLDROOF
	cool := waterHot && roofCold
	if cool {
		Info("ShouldCool: %t waterHot(%t) roofCold(%t)", cool, waterHot, roofCold)
	}
	return cool
}

// A return value of 'True' indicates that the pool is too cool and the roof is hot, running
// the pumps with solar on would help bring the water up to the target temperature.
func (ppc *PoolPumpController) shouldWarm() bool {
	if ppc.config.cfg.SolarDisabled {
		Debug("shouldWarm: disabled(%t)", ppc.config.cfg.SolarDisabled)
		return false
	}

	waterCold := ppc.pumpTemp.Temperature() < ppc.config.cfg.Target-ppc.config.cfg.Tolerance
	roofHot := ppc.roofTemp.Temperature() > HOTROOF
	warm := waterCold && roofHot
	if warm || doDebug {
		Info("ShouldWarm: %t waterCold(%t) roofHot(%t)", warm, waterCold, roofHot)
	}
	return warm
}

func adjustedRunTime(runTime float64, sinceTime time.Time) float64 {
	runSeconds := runTime * 3600
	return (runSeconds - float64(time.Since(sinceTime).Seconds())) / 3600
}

// RunPumpsIfNeeded - If the water is not within the tolerance limit of the target, and the roof
// temperature would help get the temperature to be closer to the target, the pumps will be
// turned on.  If the outdoor temperature is low or the pool is very cold, the sweep will also be
// run to help mix the water as it approaches the target.
func (ppc *PoolPumpController) RunPumpsIfNeeded() {
	state := ppc.switches.State()
	if ppc.switches.ManualState(ppc.config.cfg.RunTime) {
		// Don't warm past the target
		if state == SOLAR && !ppc.shouldWarm() {
			ppc.switches.SetState(PUMP, true, adjustedRunTime(ppc.config.cfg.RunTime, ppc.switches.GetStartTime()))
		}
		return
	}
	if state == DISABLED && !ppc.config.cfg.Disabled && !ppc.config.cfg.SolarDisabled {
		ppc.switches.setSwitches(false, false, false, false, OFF)
		return
	}
	if ppc.config.cfg.Disabled {
		if state > DISABLED {
			ppc.switches.setSwitches(false, false, false, false, DISABLED)
		}
		return
	}

	if ppc.shouldCool() || ppc.shouldWarm() {
		// Wide deltaT between target and temp or when it's cold, run sweep
		if state == MIXING {
			return
		}
		Info("ShouldCool(%t) - ShouldWarm(%t)", ppc.shouldCool(), ppc.shouldWarm())
		if ppc.pumpTemp.Temperature() < ppc.config.cfg.Target-ppc.config.cfg.DeltaT ||
			ppc.pumpTemp.Temperature() > ppc.config.cfg.Target+ppc.config.cfg.Tolerance {
			ppc.switches.SetState(MIXING, false, ppc.config.cfg.RunTime)
		} else {
			// Just push water through the panels
			ppc.switches.SetState(SOLAR, false, ppc.config.cfg.RunTime)
		}
		return
	}

	// If the pumps havent run in a day, wait til 4AM then start them
	freqHours := DurationFromHours((ppc.config.cfg.DailyFrequency-0.25)*24.0, 12.0)
	runtime := DurationFromHours(ppc.config.cfg.RunTime, 1.0)
	if time.Since(ppc.switches.GetStopTime()) > freqHours {
		Log("Daily SWEEP running for %s every %s - %s remaining", runtime, freqHours.String(), runtime-time.Since(ppc.switches.GetStartTime()))
		ppc.switches.SetState(SWEEP, false, ppc.config.cfg.RunTime) // Clean pool
		if time.Since(ppc.switches.GetStartTime()) > runtime {
			ppc.switches.StopAll(false) // End daily
		}
		return
	}
	// If there is no reason to turn on the pumps and it's not manual, turn off after 2 hours
	if state > OFF && time.Since(ppc.switches.GetStartTime()) > 2*time.Hour {
		ppc.switches.StopAll(false)
	}
}

// Runs calls PoolPumpController.Update() and PoolPumpController.RunPumpsIfNeeded()
// repeatedly until PoolPumpController.Stop() is called
func (ppc *PoolPumpController) runLoop() {
	interval := time.Second * 5
	postStatus := time.Now()
	keepRunning := true
	for keepRunning {
		if postStatus.Before(time.Now()) {
			postStatus = time.Now().Add(5 * time.Minute)
			Info(ppc.Status())
		}
		ppc.SyncAdjustments()
		select {
		case <-ppc.done:
			ppc.button.Stop()
			// Turn off the pumps, and don't let them turn back on
			ppc.switches.Disable()
			keepRunning = false
		case <-time.After(interval):
			ppc.Update()
			ppc.RunPumpsIfNeeded()
			ppc.UpdateRrd()
			Debug(ppc.Status())
		}
	}
	Alert("Exiting Controller")
}

// Start finishes initializing the PoolPumpController, and kicks off the control thread.
func (ppc *PoolPumpController) Start() error {
	ppc.button = NewGpioButton(buttonGpio, func() {
		switch ppc.switches.State() {
		case OFF:
			ppc.switches.SetState(PUMP, true, ppc.config.cfg.RunTime)
		case PUMP:
			ppc.switches.SetState(SWEEP, true, ppc.config.cfg.RunTime)
		case SOLAR:
			ppc.switches.SetState(MIXING, true, ppc.config.cfg.RunTime)
		case DISABLED:
		default:
			ppc.switches.SetState(OFF, true, ppc.config.cfg.RunTime)
		}
	})
	// Initialize RRDs
	ppc.createRrds()

	// Start go routines
	err := ppc.Update()
	if err != nil {
		return err
	}
	ppc.button.Start()
	go ppc.runLoop()
	return nil
}

// Stop stops all of the pumps
func (ppc *PoolPumpController) Stop() {
	ppc.switches.StopAll(true)
	ppc.done <- true
}

// PersistCalibration saves the callibration data
func (ppc *PoolPumpController) PersistCalibration() {
	t, ok := ppc.pumpTemp.(*GpioThermometer)
	if ok {
		ppc.config.cfg.PumpAdjustment = t.adjust
	}
	t, ok = ppc.roofTemp.(*GpioThermometer)
	if ok {
		ppc.config.cfg.RoofAdjustment = t.adjust
	}
	err := ppc.config.Save()
	if err != nil {
		Error("Could not persist config: %v", err)
	}
}

// SyncAdjustments syncrhonizes the adjustments to temperature sensors
func (ppc *PoolPumpController) SyncAdjustments() {
	t, ok := ppc.pumpTemp.(*GpioThermometer)
	if ok {
		t.adjust = ppc.config.cfg.PumpAdjustment
	}
	t, ok = ppc.roofTemp.(*GpioThermometer)
	if ok {
		t.adjust = ppc.config.cfg.RoofAdjustment
	}
}

// Status prints the status of the system
func (ppc *PoolPumpController) Status() string {
	return fmt.Sprintf(
		"Status(%s) Button(%s) Solar(%s) Pump(%s) Sweep(%s) Manual(%t) Target(%0.1f) "+
			"Pool(%0.1f) Pump(%0.1f) Roof(%0.1f)",
		ppc.switches.State(), ppc.button.pin.Read(), ppc.switches.solar.Status(),
		ppc.switches.pump.Status(), ppc.switches.sweep.Status(),
		ppc.switches.ManualState(ppc.config.cfg.RunTime), ppc.config.cfg.Target,
		ppc.runningTemp.Temperature(), ppc.pumpTemp.Temperature(),
		ppc.roofTemp.Temperature())
}
