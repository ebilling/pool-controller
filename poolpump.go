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
	weather       *Weather
	switches      *Switches
	waterTemp     Thermometer
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
		weather:    NewWeather(config.GetString(configAppId), 15 * time.Minute),
		switches:   NewSwitches(mftr),
		waterTemp:  therm(config, "Pumphouse", waterGpio),
		roofTemp:   therm(config, "Poolhouse Roof", roofGpio),
		solar:      SolarVariables{
			target: 25.0,
			deltaT: 5.0,
			tolerance: 0.5,
		},
		tempRrd:    NewRrd(config.GetString("homekit.data")+"/temperature.rrd"),
		pumpRrd:    NewRrd(config.GetString("homekit.data")+"/pumpstatus.rrd"),
		done:       make(chan bool),
	}
	ppc.runningTemp = RunningWaterThermometer(ppc.waterTemp, ppc.switches)
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
			temp < ppc.solar.target || // Cool Weather
			ppc.waterTemp.Temperature() > ppc.solar.target + ppc.solar.tolerance {
			ppc.switches.SetState(STATE_SOLAR_MIXING, false)
		} else {
			// Just push water through the panels
			ppc.switches.SetState(STATE_SOLAR, false)
		}
		return
	}
	if time.Now().Sub(ppc.switches.GetStopTime()) > 24 * time.Hour {
		if time.Now().Sub(ppc.switches.GetStartTime()) > 2 * time.Hour {
			ppc.switches.StopAll(false) // End daily
		} else {
			ppc.switches.SetState(STATE_SWEEP, false) // Clean pool
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
		if tries % 6 == 0 {
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
	// Initialize RRDs
	ppc.createRrds()

	// Start go routines
	ppc.Update()
	ppc.button.Start()	
	go ppc.runLoop()
}

func (r *Rrd) addTemp(name, title string, colorid, which int) {
	r.creator.DS(name, "GAUGE", "300", "-273", "5000")
	vname := fmt.Sprintf("t%d", which)
	cname := fmt.Sprintf("f%d", which)
	r.grapher.Def(vname, r.path, name, "MAX")
	if name == "solar" {
		r.grapher.CDef(cname, vname + ",10,/")
	} else {
		r.grapher.CDef(cname, "9,5,/," + vname + ",*,32,+")
	}
	r.grapher.Line(2.0, cname, colorStr(colorid), title)
}

func (ppc *PoolPumpController) createRrds() {
	tg := ppc.tempRrd.grapher
	tg.SetTitle("Temperatures and Solar Radiation")
	tg.SetVLabel("Degrees Farenheit")
	tg.SetRightAxis(1, 0.0)
	tg.SetRightAxisLabel("dekawatts/sqm")
	tg.SetSize(700, 300) // Config?
	tg.SetImageFormat("PNG")

	ppc.tempRrd.addTemp("pool",    "Pool",         0,  1)
	ppc.tempRrd.addTemp("weather", "Weather",      1,  2)
	ppc.tempRrd.addTemp("roof",    "Roof",         2,  3)
	ppc.tempRrd.addTemp("pump",    "Pump",         3,  4)
	ppc.tempRrd.addTemp("solar",   "SolRad w/sqm", 5,  5)
	ppc.tempRrd.addTemp("target",  "Target",       6,  6)
	ppc.tempRrd.AddStandardRRAs()
	ppc.tempRrd.Creator().Create(false) // fails if already exists

	pg := ppc.pumpRrd.grapher
	pg.SetTitle("Pump Activity")
	pg.SetVLabel("Status Code")
	pg.SetRightAxis(1, 0.0)
	pg.SetRightAxisLabel("Status Code")
	pg.SetSize(700, 300) // Config?
	pg.SetImageFormat("PNG")

	pc := ppc.pumpRrd.Creator()
	pc.DS("status", "GAUGE", "300", "-1", "10")
	pc.DS("solar", "GAUGE", "300", "-1", "10")
	pg.Def("t1", ppc.pumpRrd.path, "status", "MAX")
	pg.Line(2.0, "t1", colorStr(0), "Pump Status")
	pg.Def("t2", ppc.pumpRrd.path, "solar", "MAX")
	pg.Line(2.0, "t2", colorStr(2), "Solar Status")
	ppc.pumpRrd.AddStandardRRAs()
	pc.Create(false) // fails if already exists
}

// Writes updates to RRD files and generates cached graphs
func (ppc *PoolPumpController) UpdateRrd() {
	hours := time.Duration(ppc.config.GetFloat("graph.scale")) * time.Hour
	err :=  ppc.tempRrd.Updater().Update(fmt.Sprintf("N:%f:%f:%f:%f:%f:%f",
		ppc.WeatherC(), ppc.runningTemp.Temperature(),
		ppc.waterTemp.Temperature(), ppc.roofTemp.Temperature(),
		ppc.solar.target, ppc.weather.GetCurrentTempC(
			ppc.config.GetString(configZip))))
	if err != nil {
		Error("Could not create TempRrd: %s", err.Error())
	}

	_, err = ppc.tempRrd.Grapher().SaveGraph("/tmp/temps.png",
		time.Now().Add(hours * -1), time.Now())
	if err != nil {
		Error("Could not create TempGraph: %s", err.Error())
	}

	solar:= 0
	if ppc.switches.solar.isOn() { solar = 1 }
	err = ppc.pumpRrd.Updater().Update(fmt.Sprintf("N:%d:%d",
		ppc.switches.State(), solar))
	if err != nil {
		Error("Could not create PumpRrd: %s", err.Error())
	}

	_, err = ppc.pumpRrd.Grapher().SaveGraph("/tmp/pumps.png",
		time.Now().Add(hours * -1), time.Now())
	if err != nil {
		Error("Could not create PumpGraph: %s", err.Error())
	}
}

func (ppc *PoolPumpController) Stop() {
	ppc.done <- true
}

func (ppc *PoolPumpController) WeatherC() float64 {
	return ppc.weather.GetCurrentTempC(ppc.config.GetString(configZip))
}	

func (ppc *PoolPumpController) Status() string {
	return fmt.Sprintf(
		"Status(%s) Solar(%s) Pump(%s) Sweep(%s) " +
		"Pool(%0.1f) Pump(%0.1f) Roof(%0.1f) CurrentTemp(%0.1f)",
		ppc.switches.State(), ppc.switches.solar.Status(),
		ppc.switches.pump.Status(), ppc.switches.sweep.Status(),
		ppc.runningTemp.Temperature(), ppc.waterTemp.Temperature(),
		ppc.roofTemp.Temperature(), ppc.WeatherC())
}
