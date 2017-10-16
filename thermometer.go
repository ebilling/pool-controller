package main

import (
	"github.com/stianeikeland/go-rpio"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"
)

type Thermometer interface {
	Temperature() float64
	Update() error
	Stop()
	Done() bool	
}

// JsonThermometer gets a temperature from a file saved in JSON format
type JsonThermometer struct {
	path        string
	key         string
	temperature float64
	done        bool
}

func NewJsonThermometer(path string, key string) *JsonThermometer {
	th := JsonThermometer{
		path:        path,
		key:         key,
		done:        false,
		temperature: 0.0,
	}
	return &th
}

func (t *JsonThermometer) Stop() {
	t.done = true
}

func (t *JsonThermometer) Done() bool {
	return t.done
}

func (t *JsonThermometer) Temperature() float64 {
	return t.temperature
}

func (t *JsonThermometer) Update() error {
	data := NewJSONmap()
	data.readFile(t.path)
	if data.Contains(t.key) {
		temp := data.Get(t.key).(string)
		celsius, err := strconv.ParseFloat(temp, 64)
		if err != nil {
			return fmt.Errorf("Temperature not valid: key(%s) %s",
				t.key, temp)
		}
		t.temperature = celsius
		return nil
	} else {
		return fmt.Errorf("Could not fetch temp for key(%s)", t.key)
	}
	
	return nil	
}

// To enable testing
type PiPin interface {
	Input()
	Output()
	High()
	Low()
	Read() rpio.State
	PullUp()
	PullDown()
	PullOff()
}

type GpioThermometer struct {
	mutex       sync.Mutex
	pin         PiPin
	gpio        uint32
	microfarads float64
	temperature float64
	updated     time.Time
	done        bool	
}

func NewGpioThermometer(gpio uint32, capacitance_uF float64) (*GpioThermometer) {
	th := GpioThermometer{
		mutex:       sync.Mutex{},
		pin:         rpio.Pin(gpio),
		gpio:        gpio,
		microfarads: capacitance_uF,
		temperature: float64(0.0),
		updated:     time.Now().Add(-24 * time.Hour), // yesterday
		done:        false,
	}
	return &th
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
		if r == rpio.High {
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

func (t *GpioThermometer) Stop() {
	t.done = true
}

func (t *GpioThermometer) Done() bool {
	return t.done
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
