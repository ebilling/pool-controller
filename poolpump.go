package main

import (
	"errors"
	"fmt"
	"sync"
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

	solarMotorTime = 30 * time.Second
)

// The PoolPumpController manages the relays that control the pumps based on
// data from temperature probes and the weather.
type PoolPumpController struct {
	mu          sync.RWMutex
	config      *Config
	switches    *Switches
	pumpTemp    Thermometer
	runningTemp Thermometer
	roofTemp    Thermometer
	thermostat  *PoolThermostat
	button      *Button
	tempRrd     *Rrd
	pumpRrd     *Rrd
	done        chan struct{}
	stopped     chan struct{}
	stopOnce    sync.Once
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
		tempRrd:  NewRrd(*config.dataDirectory + "/temperature.rrd"),
		pumpRrd:  NewRrd(*config.dataDirectory + "/pumpstatus.rrd"),
		done:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}
	if config.simulate != nil && *config.simulate {
		ppc.pumpTemp = NewSimulatedThermometer("Pump", mftr, *config.simPumpTemp)
		ppc.roofTemp = NewSimulatedThermometer("Roof", mftr, *config.simRoofTemp)
	} else {
		ppc.pumpTemp = NewGpioThermometer("Pump", mftr, waterGpio)
		ppc.roofTemp = NewGpioThermometer("Roof", mftr, roofGpio)
	}
	ppc.SyncAdjustments()
	ppc.runningTemp = RunningWaterThermometer(ppc.pumpTemp, ppc.switches)
	ppc.thermostat = NewPoolThermostat(&ppc)
	return &ppc
}

// Update the solar configuration parameters from the config file (if changed)
// and updates the values of the Thermometers.
func (ppc *PoolPumpController) Update() error {
	var updateErrors []error
	pumpErr := ppc.pumpTemp.Update()
	if pumpErr != nil {
		updateErrors = append(updateErrors, fmt.Errorf("pump temp update failed: %w", pumpErr))
	}
	if err := ppc.roofTemp.Update(); err != nil {
		updateErrors = append(updateErrors, fmt.Errorf("roof temp update failed: %w", err))
	}
	// The selective pool temperature is derived from the pump thermometer. Do
	// not stamp it with a stale pump reading after a failed physical sample.
	if pumpErr == nil {
		if err := ppc.runningTemp.Update(); err != nil {
			updateErrors = append(updateErrors, fmt.Errorf("running temp update failed: %w", err))
		}
	}
	if ppc.button != nil {
		if ppc.config.cfg.ButtonDisabled {
			ppc.button.Disable()
		} else {
			ppc.button.Enable()
		}
	}
	if ppc.thermostat != nil {
		ppc.thermostat.Sync()
	}
	return errors.Join(updateErrors...)
}

// A return value of 'True' indicates that the pool is too hot and the roof is cold
// (probably at night), running the pumps with solar on would help bring the water
// down to the target temperature.
func (ppc *PoolPumpController) shouldCool() bool {
	if ppc.config.cfg.SolarDisabled || ppc.config.cfg.CoolDisabled {
		return false
	}
	return ppc.pumpTemp.Temperature() > (ppc.config.cfg.Target+ppc.config.cfg.Tolerance) &&
		ppc.pumpTemp.Temperature() > (ppc.roofTemp.Temperature()+ppc.config.cfg.DeltaT)
}

// A return value of 'True' indicates that the pool is too cool and the roof is hot, running
// the pumps with solar on would help bring the water up to the target temperature.
func (ppc *PoolPumpController) shouldWarm() bool {
	if ppc.config.cfg.SolarDisabled || ppc.config.cfg.HeatDisabled {
		Debug("shouldWarm: solarDisabled(%t) heatDisabled(%t)",
			ppc.config.cfg.SolarDisabled, ppc.config.cfg.HeatDisabled)
		return false
	}

	waterCold := ppc.pumpTemp.Temperature() < (ppc.config.cfg.Target - ppc.config.cfg.Tolerance)
	roofHot := ppc.pumpTemp.Temperature() < (ppc.roofTemp.Temperature() - ppc.config.cfg.DeltaT)
	warm := waterCold && roofHot
	if warm {
		Info("ShouldWarm: %t waterCold(%t) roofHot(%t)", warm, waterCold, roofHot)
		Info("Temp(%0.3f) < %0.3f {Target(%0.3f) - Tolerance(%0.3f)} : WaterCold(%t)",
			ppc.pumpTemp.Temperature(),
			ppc.config.cfg.Target-ppc.config.cfg.Tolerance,
			ppc.config.cfg.Target,
			ppc.config.cfg.Tolerance,
			waterCold)
		Info("Temp(%0.3f) < %0.3f {Roof(%0.3f) - DeltaT(%0.3f)} : RoofHot(%t)",
			ppc.pumpTemp.Temperature(),
			ppc.roofTemp.Temperature()-ppc.config.cfg.DeltaT,
			ppc.roofTemp.Temperature(),
			ppc.config.cfg.DeltaT,
			roofHot)
	}
	return warm
}

// RunPumpsIfNeeded - If the water is not within the tolerance limit of the target, and the roof
// temperature would help get the temperature to be closer to the target, the pumps will be
// turned on.  If the outdoor temperature is low or the pool is very cold, the sweep will also be
// run to help mix the water as it approaches the target.
func (ppc *PoolPumpController) RunPumpsIfNeeded() {
	state := ppc.switches.State()
	if ppc.switches.ManualState(ppc.config.cfg.RunTime) {
		return
	}
	if state == DISABLED && !ppc.config.cfg.Disabled && !ppc.config.cfg.SolarDisabled {
		ppc.switches.Enable()
		return
	}
	if ppc.config.cfg.Disabled {
		if state > DISABLED {
			ppc.switches.SetState(DISABLED, false, ppc.config.cfg.RunTime)
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
	if time.Since(ppc.switches.GetStopTime()) > freqHours && time.Now().Hour() < 6 { // run in the early morning
		Log("Daily running SWEEP: %s", freqHours.String())
		ppc.switches.SetState(SWEEP, false, ppc.config.cfg.RunTime) // Clean pool
		if time.Since(ppc.switches.GetStartTime()) > runtime {
			ppc.switches.StopAll(false) // End daily
		}
		return
	}
	// If there is no reason to turn on the pumps and it's not manual, turn off
	if state > OFF && ppc.switches.GetStartTime().Add(time.Hour).Before(time.Now()) {
		ppc.switches.StopAll(false)
	}
}

// Runs calls PoolPumpController.Update() and PoolPumpController.RunPumpsIfNeeded()
// repeatedly until PoolPumpController.Stop() is called
func (ppc *PoolPumpController) runLoop() {
	interval := time.Second * 5
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer close(ppc.stopped)
	postStatus := time.Now()
	for {
		if postStatus.Before(time.Now()) {
			postStatus = time.Now().Add(5 * time.Minute)
			ppc.mu.RLock()
			Info(ppc.Status())
			ppc.mu.RUnlock()
		}
		select {
		case <-ppc.done:
			ppc.mu.Lock()
			ppc.switches.Disable()
			ppc.mu.Unlock()
			ppc.button.Stop()
			Alert("Exiting Controller")
			return
		case <-ticker.C:
			ppc.mu.Lock()
			ppc.SyncAdjustments()
			if err := ppc.Update(); err != nil {
				Error("Sensor update failed; retaining current relay state: %s", err)
				ppc.mu.Unlock()
				continue
			}
			ppc.RunPumpsIfNeeded()
			ppc.UpdateRrd()
			Debug(ppc.Status())
			ppc.mu.Unlock()
		}
	}
}

// Start finishes initializing the PoolPumpController, and kicks off the control thread.
func (ppc *PoolPumpController) Start() error {
	ppc.button = NewGpioButton(buttonGpio, func() {
		ppc.mu.Lock()
		defer ppc.mu.Unlock()
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
	ppc.stopOnce.Do(func() { close(ppc.done) })
	<-ppc.stopped
}

// PersistCalibration saves the callibration data
func (ppc *PoolPumpController) PersistCalibration() {
	t, ok := ppc.pumpTemp.(*GpioThermometer)
	if ok {
		ppc.config.cfg.PumpAdjustment = t.Adjustment()
	}
	t, ok = ppc.roofTemp.(*GpioThermometer)
	if ok {
		ppc.config.cfg.RoofAdjustment = t.Adjustment()
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
		t.SetAdjustment(ppc.config.cfg.PumpAdjustment)
	}
	t, ok = ppc.roofTemp.(*GpioThermometer)
	if ok {
		t.SetAdjustment(ppc.config.cfg.RoofAdjustment)
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
