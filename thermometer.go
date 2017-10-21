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
	microfarads float64
	updated     time.Time
	history     History
	accessory   *accessory.Thermometer
}

func NewGpioThermometer(name string, manufacturer string,
	gpio uint8, capacitance_uF float64) (*GpioThermometer) {
	return newGpioThermometer(name, manufacturer,
		NewGpio(gpio), capacitance_uF)
}

func newGpioThermometer(name string, manufacturer string,
	pin PiPin, capacitance_uF float64) (*GpioThermometer) {
	acc := accessory.NewTemperatureSensor(AccessoryInfo(name, manufacturer),
		0.0, -20.0, 100.0, 1.0)
	th := GpioThermometer{
		name:            name,
		mutex:           sync.Mutex{},
		pin:             pin,
		microfarads:     capacitance_uF,
		history:        *NewHistory(100),
		updated:         time.Now().Add(-24 * time.Hour),
		accessory:       acc,
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
	t.pin.Output(Low)
	time.Sleep(300 * time.Millisecond)

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

	// Completely bogus, ignore
	if dischargeTime < minTime || dischargeTime > maxTime {
		return false
	}
	return true
}

func (t *GpioThermometer) Temperature() (float64) {
	if time.Now().After(t.updated.Add(time.Minute)) {
		t.Update()
	}
	return t.accessory.TempSensor.CurrentTemperature.GetValue()
}

func (t *GpioThermometer) Update() (error) {
	var dischargeTime time.Duration
	h := NewHistory(3)
	tries := 0
	for i := 0; h.Len() < 3; i++ {
		tries++
		dischargeTime = t.getDischargeTime()
		if t.inRange(dischargeTime) {
			t.history.PushDuration(dischargeTime)
			h.PushDuration(dischargeTime)
		}
	}

	// DEBUG 
	Debug("%s Update() took %d tries to find %d results", t.Name(), tries, h.Len())

	stdd := t.history.Stddev()
	avg  := t.history.Average()
	med  := t.history.Median()

	// Throw away bad results
	if math.Abs(avg - h.Median()) > stdd * 1.5 {
		Info("%s failed to update: Cur(%0.1f) Med(%0.1f) Avg(%0.1f) Stdd(%0.1f)",
			t.Name(),
			h.Median()/float64(time.Millisecond),
			med/float64(time.Millisecond),
			avg/float64(time.Millisecond),
			stdd/float64(time.Millisecond))
		return fmt.Errorf("Could not update temperature successfully")
	}
	temp := t.getTemp(t.getOhms(dischargeTime))
	t.accessory.TempSensor.CurrentTemperature.SetValue(temp)
	t.updated = time.Now()
	return nil
}

func (t *GpioThermometer) cleanData() {
	
}

// Converts a temperature in Celsius to Farenheit
func toFarenheit(celsius float64) (float64) {
	return  (celsius * 9.0 / 5.0) + 32.0
}

// Converts a temperature in Farenheit to Celsius
func toCelsius(farenheit float64) (float64) {
	return farenheit - 32.0 * 5.0 / 9.0
}
