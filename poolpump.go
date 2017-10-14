package main

import (
	"github.com/brutella/hc/accessory"
	"os"
	"strconv"
	"time"
)

var mftr = "Bonnie Labs"

type Temp struct {
	therm *Thermometer
	acc   *accessory.Thermometer
}

type PoolPumpController struct {
	config      Config
	pump        *accessory.Switch
	sweep       *accessory.Switch
	waterTemp   *Temp
	roofTemp    *Temp
	pin         string
	done        chan bool
}

func NewTemp(data Config, key string, name string) *Temp {
	info := accessory.Info{
		Name: name,
		Manufacturer: mftr,
	}
	th := NewThermometer(key)
	t := Temp{
		therm: th,
		acc:   accessory.NewTemperatureSensor(info, th.Temperature(), 0.0, 100.0, 1.0),
	}
	return &t
}

func (t *Temp) Update(data *Config) {
	t.acc.TempSensor.CurrentTemperature.SetValue(t.therm.Update(data))
}

func NewPoolPumpController(path string) *PoolPumpController {
	config := *NewConfig(path)
	ppc := PoolPumpController {
		config:    config,
		done:      make(chan bool),
		waterTemp: nil,
		roofTemp:  nil,
	}

	pumpinfo := accessory.Info{
		Name:         "Pool Pump",
		Manufacturer: mftr,
	}

	sweepinfo := accessory.Info{
		Name:         "Pool Sweep",
		Manufacturer: mftr,
	}

	ppc.pump = accessory.NewSwitch(pumpinfo)
	ppc.pump.Switch.On.OnValueRemoteUpdate(func(on bool) {
		if on == true {
			ppc.turnPumpOn()
		} else {
			ppc.turnAllOff()
		}
	})

	ppc.sweep = accessory.NewSwitch(sweepinfo)
	ppc.sweep.Switch.On.OnValueRemoteUpdate(func(on bool) {
		if on == true {
			ppc.turnSweepOn()
		} else {
			ppc.turnAllOff()
		}
	})

	ppc.pin = config.Get("homekit.pin").(string)
	Info("Homekit Pin: %s", ppc.pin)

	return &ppc
}

func (ppc *PoolPumpController) cmd(command string) {
	if !ppc.config.Contains("path.cmdfifo") {
		Error("path.cmdfifo not specified in the configuration file")
		return
	}
	path := ppc.config.Get("path.cmdfifo").(string)
	fifo, err := os.OpenFile(path, os.O_RDWR, 0666)
	if err != nil {
		Error("Command Open Error: %s", err.Error())
		return
	}
	defer fifo.Close()
	Debug("Writing command: %s", command)
	_, err = fifo.WriteString(command + "\n")
	if err != nil {
		Error("Command Write Error: %s", err.Error())
	}
}

func (ppc *PoolPumpController) turnPumpOn() {
	Info("Turning Pump On")
	ppc.cmd("PUMP_ON")
}

func (ppc *PoolPumpController) turnSweepOn() {
	Info("Turning Sweep On")
	ppc.cmd("SWEEP_ON")
}

func (ppc *PoolPumpController) turnAllOff() {
	Info("Turning Pumps Off")
	ppc.cmd("OFF")
}

//TODO update the temperature in the accessory
func (ppc *PoolPumpController) Update() {
	tdatapath := ppc.config.Get("path.temperature").(string)
	tdata := NewConfig(tdatapath)
	if tdata != nil {
		if ppc.waterTemp == nil {
			ppc.waterTemp = NewTemp(*tdata, "waterTempC", "Water Temp")
		}
		if ppc.roofTemp == nil {
			ppc.roofTemp = NewTemp(*tdata, "roofTempC", "Roof Temp")
		}
		ppc.waterTemp.Update(tdata)
		ppc.roofTemp.Update(tdata)
	}

	statusPath := ppc.config.Get("path.status").(string)
	file, err := os.Open(statusPath)
	if err != nil {
		Error("Error opening file %s: %s", statusPath, err.Error())
		return
	}
	defer file.Close()
	data := make([]byte, 100)
	count, err := file.Read(data)
	if err != nil {
		Error("Error reading file %s: %s", statusPath, err.Error())
		return
	}
	if count < 1 {
		Error("Read status doesn't seem to be valid (%d): for %s", count, statusPath)
	}

	status, err := strconv.ParseInt(string(data[:count]), 10, 64)
	if err != nil {
		Error("Could not convert status: %s", err.Error())
	}
	if status <= 0 {
		ppc.pump.Switch.On.SetValue(false)
		ppc.sweep.Switch.On.SetValue(false)
	} else if status%2 == 1 {
		ppc.pump.Switch.On.SetValue(true)
		ppc.sweep.Switch.On.SetValue(false)
	} else {
		ppc.pump.Switch.On.SetValue(true)
		ppc.sweep.Switch.On.SetValue(true)
	}
}

func (ppc *PoolPumpController) RunLoop() {
	interval := 5 * time.Second
	tries := 0
	for {
		if tries % 12 == 0 {
			Info("Homekit service for PoolPumpController still running")
		}
		select {
		case <- time.After(interval):
			ppc.Update()
		case <- ppc.done:
			break
		}
		tries++
	}
}

func (ppc *PoolPumpController) Start() {
	go ppc.RunLoop()
}

func (ppc *PoolPumpController) Stop() {
	ppc.done <- true
}
