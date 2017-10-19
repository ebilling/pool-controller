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
	tempRrd       *Rrd
	pumpRrd       *Rrd
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

func NewPoolPumpController(config *Config) *PoolPumpController {
	ppc := PoolPumpController {
		config:     config,
		weather:    NewWeather(config.GetString(configAppId), 15 * time.Minute),
		switches:   NewSwitches(mftr),
		waterTemp:  therm(config, "Water Temp", waterGpio),
		roofTemp:   therm(config, "Roof Temp", roofGpio),
		solar:      SolarVariables{
			target: 25.0,
			deltaT: 5.0,
			tolerance: 0.5,
		},
		tempRrd:    NewRrd(config.GetString("homekit.data")+"/rrd/temperature.rrd"),
		pumpRrd:    NewRrd(config.GetString("homekit.data")+"/rrd/pumpstatus.rrd"),
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
	count := fmt.Sprintf("%d", which)
	r.grapher.Def(vname, r.path, name, count)
	if name == "solar" {
		r.grapher.CDef(vname, cname + "=" + vname + ",10,/")
	} else {
		r.grapher.CDef(vname, cname + "=9,5,/,"+ vname+ ",*,32,+")
	}
	r.grapher.Line(2.0, cname, colorStr(colorid), title)
}

func (ppc *PoolPumpController) createRrds() {
	ppc.tempRrd.grapher.SetTitle("Temperatures and Solar Radiation")
	ppc.tempRrd.grapher.SetVLabel("Degrees Farenheit")
	ppc.tempRrd.grapher.SetRightAxis(1, 0.0)
	ppc.tempRrd.grapher.SetRightAxisLabel("dekawatts/sqm")
	ppc.tempRrd.grapher.SetSize(700, 300) // Config?
	ppc.tempRrd.grapher.SetImageFormat("PNG")

	tc := ppc.tempRrd.Creator()
	ppc.tempRrd.addTemp("weather", "Weather",      0,  0)
	ppc.tempRrd.addTemp("pool",    "Pool",         1,  1)
	ppc.tempRrd.addTemp("pump",    "Pump",         3,  2)
	ppc.tempRrd.addTemp("roof",    "Roof",         2,  3)
	ppc.tempRrd.addTemp("target",  "Target",       6,  4)
	ppc.tempRrd.addTemp("solar",   "SolRad w/sqm", 5,  5)
	ppc.tempRrd.AddStandardRRAs()
	tc.Create(false) // fails if already exists

	ppc.pumpRrd.grapher.SetTitle("Pump Activity")
	ppc.pumpRrd.grapher.SetVLabel("Status Code")
	ppc.pumpRrd.grapher.SetRightAxis(1, 0.0)
	ppc.pumpRrd.grapher.SetRightAxisLabel("Status Code")
	ppc.pumpRrd.grapher.SetSize(700, 300) // Config?
	ppc.pumpRrd.grapher.SetImageFormat("PNG")

	pc := ppc.pumpRrd.Creator()
	pc.DS("status", "GAUGE", "300", "-1", "10")
	ppc.pumpRrd.grapher.Def("t1", ppc.pumpRrd.path, "status", "0")
	ppc.tempRrd.grapher.Line(2.0, "t1", colorStr(0), "Pump Status")
	pc.DS("solar", "GAUGE", "300", "-1", "10")
	ppc.pumpRrd.grapher.Def("t2", ppc.pumpRrd.path, "solar", "1")
	ppc.tempRrd.grapher.Line(2.0, "t2", colorStr(4), "Solar Status")
	ppc.pumpRrd.AddStandardRRAs()
	pc.Create(false) // fails if already exists	
}

// Writes updates to RRD files and generates cached graphs
func (ppc *PoolPumpController) UpdateRrd() {
	ppc.tempRrd.Updater().Update(fmt.Sprintf("N:%f:%f:%f:%f:%f:%f",
		ppc.WeatherC(), ppc.runningTemp.Temperature(),
		ppc.waterTemp.Temperature(), ppc.roofTemp.Temperature(),
		ppc.solar.target, ppc.weather.GetCurrentTempC(
			ppc.config.GetString(configZip))))

	ppc.tempRrd.Grapher().SaveGraph("/tmp/temps.png", time.Now().Add(24 * time.Hour), time.Now())
		
	ppc.pumpRrd.Updater().Update(fmt.Sprintf("N:%d:%d",
		ppc.switches.State(),
		ppc.switches.solar.Status()))
	ppc.pumpRrd.Grapher().SaveGraph("/tmp/pumps.png", time.Now().Add(24 * time.Hour), time.Now())

}

func (ppc *PoolPumpController) Stop() {
	ppc.done <- true
}

func (ppc *PoolPumpController) WeatherC() float64 {
	return ppc.weather.GetCurrentTempC(ppc.config.GetString(configZip))
}	

func (ppc *PoolPumpController) Status() string {
	return fmt.Sprintf(
		"CurrentTemp(%0.1f) Pool(%0.1f) Solar(%s) Pump(%s) Sweep(%s) Pool(%0.1f) Pump(%0.1f) Roof(0.1f)",
		ppc.WeatherC(), ppc.switches.pump.Status(),
		ppc.switches.sweep.Status(), ppc.switches.solar.Status(),
		ppc.runningTemp.Temperature(), ppc.waterTemp.Temperature(),
		ppc.roofTemp.Temperature())
}
