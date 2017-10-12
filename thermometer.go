package pool-controller

import (
	"os"
	"strconv"
)

type Thermometer struct {
	path      string
	temperature float64
	done        chan bool
}

func NewThermometer(path string) *Thermometer {
	th := Thermometer{
		path: path,
		done: make(chan bool),
	}
	th.readTemperature()
	return &th
}

func (t *Thermometer) Stop() {
	t.done <- true
}

func (t *Thermometer) Temperature() float64 {
	return t.temperature
}

func (t *Thermometer) readTemperature() float64 {
	file, err := os.Open(t.path)
	if err != nil {
		log.error(err)
	}
	defer file.Close()

	data := make([]byte, 100)
	count, err := file.Read(data)
	if err != nil {
		log.error(err)
	}
	if count < 3 {
		log.error("Temperature doesn't seem to be valid")
	}

	celsius, err := strconv.ParseFloat(string(data[:count]), 64)
	if err != nil {
		log.error("Could not convert temperature from device: " + err.Error())
	}
	t.temperature = celsius
	
	return t.temperature
}
