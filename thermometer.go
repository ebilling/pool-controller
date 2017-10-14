package main

import (
	"strconv"
)

type Thermometer struct {
	path      string
	key       string
	temperature float64
	done        chan bool
}

func NewThermometer(key string) *Thermometer {
	th := Thermometer{
		key: key,
		done: make(chan bool),
		temperature: 0.0,
	}
	return &th
}

func (t *Thermometer) Stop() {
	t.done <- true
}

func (t *Thermometer) Temperature() float64 {
	return t.temperature
}

func (t *Thermometer) Update(data *Config) float64 {
	if data.Contains(t.key) {
		temp := data.Get(t.key).(string)
		celsius, err := strconv.ParseFloat(temp, 64)
		if err != nil {
			Error("Temperature doesn't seem valid: key(%s) %s", t.key, temp)
		}
		t.temperature = celsius
		return celsius
	} else {
		Error("Could not fetch temp for key(%s)", t.key)
	}
	
	return t.temperature	
}
