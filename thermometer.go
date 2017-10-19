package main

import (
        "github.com/brutella/hc/accessory"
	"fmt"
	"math"
	"sync"
	"time"
)

type Thermometer interface {
	Name() string
	Temperature() float64
	Update() error
	Accessory() *accessory.Accessory
}

type SelectiveThermometer struct {
	name        string
	filter      func () (bool)
	thermometer Thermometer
	accessory   *accessory.Thermometer
	temperature float64
}

func NewSelectiveThermometer(name string, manufacturer string, thermometer Thermometer,
	filter func () bool) (*SelectiveThermometer) {
	acc := accessory.NewTemperatureSensor(AccessoryInfo(name, manufacturer),
		0.0, -20.0, 100.0, 1.0)
	thermometer.Update()
	acc.TempSensor.CurrentTemperature.SetValue(thermometer.Temperature())
	return &SelectiveThermometer{
		name:         name,
		thermometer:  thermometer,
		filter:       filter,
		temperature:  thermometer.Temperature(),
		accessory:    acc,
	}	
}

func (t *SelectiveThermometer) Name() string {
	return t.name
}

func (t *SelectiveThermometer) Temperature() float64 {
	return t.temperature
}

func (t *SelectiveThermometer) Update() error {
	if (t.filter()) {
		t.temperature = t.thermometer.Temperature()
		t.accessory.TempSensor.CurrentTemperature.SetValue(t.temperature)
	}
	return nil
}

func (t *SelectiveThermometer) Accessory() *accessory.Accessory {
	return t.accessory.Accessory
}

type GpioThermometer struct {
	name        string
	mutex       sync.Mutex
	pin         PiPin
	gpio        uint8
	microfarads float64
	temperature float64
	updated     time.Time
	accessory   *accessory.Thermometer
}

func NewGpioThermometer(name string, manufacturer string,
	gpio uint8, capacitance_uF float64) (*GpioThermometer) {
	acc := accessory.NewTemperatureSensor(AccessoryInfo(name, manufacturer),
		0.0, -20.0, 100.0, 1.0)
	th := GpioThermometer{
		name:        name,
		mutex:       sync.Mutex{},
		pin:         NewGpio(gpio),
		gpio:        gpio,
		microfarads: capacitance_uF,
		temperature: float64(0.0), // TODO: Remove and use accessory storage only
		updated:     time.Now().Add(-24 * time.Hour),
		accessory:   acc,
	}
	return &th
}

func (t *GpioThermometer) Name() string {
	return t.name
}

func (t *GpioThermometer) Accessory() (*accessory.Accessory) {
	return t.accessory.Accessory
}

func (t *GpioThermometer) getDischargeTime() (time.Duration) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	
	//Discharge the capacitor (low temps could make this really long)
	t.pin.Output()
	t.pin.Low()
	time.Sleep(500 * time.Millisecond)

	// Start polling
	start := time.Now()
	t.pin.Input()
	timeout := start.Add(500 * time.Millisecond)
	for time.Now().Before(timeout) {
		r := t.pin.Read()
		if r == High {
			return time.Since(start)
		}
		time.Sleep(time.Microsecond*100)
	}
	Error("Thermometer read timed out")
	return time.Duration(0)
}

func (t *GpioThermometer) getOhms(dischargeTime time.Duration) (float64) {
	uSec := float64(dischargeTime) / 1000.0
	return 2 * uSec / t.microfarads
}

func (t *GpioThermometer) getTemp(ohms float64) (float64) {
	const a = 79463.85
	const b = 0.1453676
	const c = 2.517178E-15
	const d = -132.2399
	if ohms == 0.0 {
		return 0.0
	}
	return d + (a - d)/(1 + math.Pow(ohms/c, b))
}

func (t *GpioThermometer) inRange(dischargeTime time.Duration) (bool) {
	const minTime = 3 * time.Millisecond
	const maxTime = 500 * time.Millisecond
	//TODO: add stddev detection here
	return dischargeTime > minTime && dischargeTime < maxTime
}

func (t *GpioThermometer) Temperature() (float64) {
	if time.Now().After(t.updated.Add(time.Minute)) {
		t.Update()
	}
	return t.temperature
}

func (t *GpioThermometer) Update() (error) {
	var dischargeTime time.Duration
	// Ignore bad results, try again
	for i := 0; i < 5; i++ {
		dischargeTime = t.getDischargeTime()
		if t.inRange(dischargeTime) {
			break
		}
	}
	temp := t.getTemp(t.getOhms(dischargeTime))
	if temp != 0.0 {
		t.temperature = temp
		t.accessory.TempSensor.CurrentTemperature.SetValue(temp)
		t.updated = time.Now()
		return nil
	}
	return fmt.Errorf("Could not update temperature successfully")
}

// Converts a temperature in Celsius to Farenheit
func toFarenheit(celsius float64) (float64) {
	return  (celsius * 9.0 / 5.0) + 32.0
}

// Converts a temperature in Farenheit to Celsius
func toCelsius(farenheit float64) (float64) {
	return farenheit - 32.0 * 5.0 / 9.0
}
