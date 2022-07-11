package main

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/brutella/hc/accessory"
)

// Thermometer reads a thermal resistance thermometer using the timings of a capacitor charge/discharge cycle
type Thermometer interface {
	Name() string
	Temperature() float64
	Update() error
	Calibrate(float64) error
	Accessory() *accessory.Accessory
}

// SelectiveThermometer filters out certain data from a Thermometer to produce a better reading
type SelectiveThermometer struct {
	name        string
	filter      func() bool
	thermometer Thermometer
	accessory   *accessory.Thermometer
}

// NewSelectiveThermometer creates a SelectiveThermometer
func NewSelectiveThermometer(name string, manufacturer string, thermometer Thermometer,
	filter func() bool) *SelectiveThermometer {
	acc := accessory.NewTemperatureSensor(AccessoryInfo(name, manufacturer),
		0.0, -20.0, 100.0, 1.0)
	thermometer.Update()
	acc.TempSensor.CurrentTemperature.SetValue(thermometer.Temperature())
	return &SelectiveThermometer{
		name:        name,
		thermometer: thermometer,
		filter:      filter,
		accessory:   acc,
	}
}

// Name returns the name of the SelectiveThermometer
func (t *SelectiveThermometer) Name() string {
	return t.name
}

// Calibrate runs a calibration operation the thermometer
func (t *SelectiveThermometer) Calibrate(a float64) error {
	return fmt.Errorf("Not supported")
}

// Temperature returns the current temperature
func (t *SelectiveThermometer) Temperature() float64 {
	return t.accessory.TempSensor.CurrentTemperature.GetValue()
}

// Update attempts to update the thermometer temperature
func (t *SelectiveThermometer) Update() error {
	if t.filter() {
		t.accessory.TempSensor.CurrentTemperature.SetValue(
			t.thermometer.Temperature())
	}
	return nil
}

// Accessory returns the Apple HomeKit accessory
func (t *SelectiveThermometer) Accessory() *accessory.Accessory {
	return t.accessory.Accessory
}

// GpioThermometer is used to measure the temperature of a given resistive thermometer
// using a capacitor.
type GpioThermometer struct {
	name        string
	mutex       sync.Mutex
	pin         PiPin
	microfarads float64
	adjust      float64
	updated     time.Time
	history     History
	accessory   *accessory.Thermometer
}

// NewGpioThermometer returns a GpioThermometer
func NewGpioThermometer(name string, manufacturer string, gpio uint8) *GpioThermometer {
	return newGpioThermometer(name, manufacturer,
		NewGpio(gpio))
}

// MillisecondFloat returns the float64 value associated with time.Millisecond time.Duration
const MillisecondFloat float64 = float64(time.Millisecond)

// Return the number of milliseconds represented by a given time.Duration
func ms(t time.Duration) float64 {
	return float64(t) / float64(time.Millisecond)
}

// Return the number of microseconds represented by a given time.Duration
func us(t time.Duration) float64 {
	return float64(t) / float64(time.Microsecond)
}

func newGpioThermometer(name string, manufacturer string, pin PiPin) *GpioThermometer {
	acc := accessory.NewTemperatureSensor(AccessoryInfo(name, manufacturer),
		0.0, -20.0, 100.0, 1.0)
	th := GpioThermometer{
		name:        name,
		mutex:       sync.Mutex{},
		pin:         pin,
		microfarads: 0.1,
		adjust:      2.5,
		history:     *NewHistory(100),
		updated:     time.Now().Add(-24 * time.Hour),
		accessory:   acc,
	}
	return &th
}

// SetAdjustment provides a multiplier to the teperature sensor
func (t *GpioThermometer) SetAdjustment(a float64) {
	t.adjust = a
}

// Name returns the name of the GpioThermometer
func (t *GpioThermometer) Name() string {
	return t.name
}

// Accessory returns the Apple HomeKit accessory related to the GpioThermometer
func (t *GpioThermometer) Accessory() *accessory.Accessory {
	return t.accessory.Accessory
}

func (t *GpioThermometer) getDischargeTime() time.Duration {
	pull := PullUp
	edge := RisingEdge
	t.mutex.Lock()
	defer t.mutex.Unlock()
	Info("getDischargeTime:%s", t.name)
	//Discharge the capacitor (low temps could make this really long)
	t.pin.Output(Low)
	time.Sleep(time.Second)
	// Start polling
	start := time.Now()
	//t.pin.InputEdge(PullDown, RisingEdge) // Original
	t.pin.InputEdge(pull, edge) // new
	if !t.pin.WaitForEdge(5 * time.Second) {
		Info("Thermometer %s, WaitForEdge(%s, %s) timed out", t.name, pull, edge)
		return time.Duration(0)
	}
	dt := time.Now().Sub(start)
	t.pin.Output(Low)
	Info("Discharge time for %s: %s", t.name, dt)
	return dt
}

func (t *GpioThermometer) getTemp(ohms float64) float64 {
	const a = 79463.85
	const b = 0.1453676
	const c = 2.517178e-15
	const d = -132.2399
	if ohms == 0.0 {
		return 0.0
	}
	return d + (a-d)/(1+math.Pow(ohms/c, b))
}

func (t *GpioThermometer) getOhms(dischargeTime time.Duration) float64 {
	uSec := t.adjust * us(dischargeTime)
	return uSec / t.microfarads
}

// Calibrate asserts a specific resistance and calculates the proper setting
// for the adjust parameter
func (t *GpioThermometer) Calibrate(ohms float64) error {
	calculated := ohms * t.microfarads / 1000.0
	Info("Expecting %0.3f ms", calculated)

	// Take a sample of values
	h := NewHistory(20)
	for i := 0; i < 20; i++ {
		dt := t.getDischargeTime()
		if dt != 0 {
			h.Push(float64(dt))
		}
	}
	dt := time.Duration(int64(h.Median()))
	value := calculated / ms(dt)
	Info("Calculated Value (full discharge) %0.3f ms, found %0.3f ms, ratio %0.3f", calculated, ms(dt), value)
	if h.Stddev() > h.Median()*0.05 || h.Len() < 10 {
		return fmt.Errorf("Returned inconsistent data value(%0.4f) Variance(%0.2f%%) entries(%d)",
			value, 100.0*h.Stddev()/h.Median(), h.Len())
	}
	Debug("Setting adjustment to %0.3f", value)
	t.adjust = value
	return nil
}

func (t *GpioThermometer) inRange(dischargeTime time.Duration) bool {
	const minTime = 100 * time.Microsecond
	const maxTime = 10 * time.Millisecond

	// Completely bogus, ignore
	if dischargeTime < minTime || dischargeTime > maxTime {
		return false
	}
	return true
}

// Temperature returns the current temperature of the GpioThermometer
func (t *GpioThermometer) Temperature() float64 {
	// TODO: Change back to time.Minute
	if time.Now().After(t.updated.Add(time.Second)) {
		t.Update()
	}
	return t.accessory.TempSensor.CurrentTemperature.GetValue()
}

// Update updates the current temperature of the GpioThermometer
func (t *GpioThermometer) Update() error {
	Info("Update %s", t.name)
	var dischargeTime time.Duration
	h := NewHistory(3)
	for i := 0; h.Len() < 3 && i < 10; i++ {
		dischargeTime = t.getDischargeTime()
		if t.inRange(dischargeTime) {
			t.history.PushDuration(dischargeTime)
			h.PushDuration(dischargeTime)
		}
	}
	Info("h[%s]: %+v", t.name, h)
	Info("t.history[%s]: %+v", t.name, t.history)

	stdd := t.history.Stddev()
	avg := t.history.Average()
	med := t.history.Median()
	dev := stdd * 1.5
	if dev < MillisecondFloat/5 {
		dev = MillisecondFloat / 5
	} // give some wiggle room

	// Throw away bad results
	if math.Abs(avg-h.Median()) > dev {
		Info("%s Thermometer update failed: Cur(%0.1f) Med(%0.1f) Avg(%0.1f) Stdd(%0.1f)",
			t.Name(),
			h.Median()/MillisecondFloat,
			med/MillisecondFloat,
			avg/MillisecondFloat,
			stdd/MillisecondFloat)
		return fmt.Errorf("Could not update temperature successfully")
	}
	Info("%s Thermometer update succeeded: Cur(%0.1f) Med(%0.1f) Avg(%0.1f) Stdd(%0.1f)",
		t.Name(),
		h.Median()/MillisecondFloat,
		med/MillisecondFloat,
		avg/MillisecondFloat,
		stdd/MillisecondFloat)
	ohms := t.getOhms(time.Duration(int64(h.Median())))
	temp := t.getTemp(ohms)
	Info("Calculating temperature (%f) for %s: %f ohms, median %s", temp, t.name, ohms, time.Duration(int64(h.Median())))
	t.accessory.TempSensor.CurrentTemperature.SetValue(temp)
	t.updated = time.Now()
	return nil
}

// Converts a temperature in Celsius to Farenheit
func toFarenheit(celsius float64) float64 {
	return (celsius * 9.0 / 5.0) + 32.0
}

// Converts a temperature in Farenheit to Celsius
func toCelsius(farenheit float64) float64 {
	return farenheit - 32.0*5.0/9.0
}
