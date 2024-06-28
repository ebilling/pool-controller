package main

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/brutella/hc/accessory"
)

const (
	// nanoFarads is the value of capacitor used for measuring the thermistor
	nanoFarads = 100
	// minTime and maxTime are the expectations for how long it should take for the capacitor to discharge
	minTime = time.Duration(nanoFarads) * time.Microsecond
	maxTime = time.Duration(nanoFarads) * time.Millisecond
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
	acc := accessory.NewTemperatureSensor(AccessoryInfo(name, manufacturer), 0.0, -20.0, 100.0, 1.0)
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
	return errors.New("not supported")
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
	return newGpioThermometer(name, manufacturer, NewGpio(gpio))
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
	acc := accessory.NewTemperatureSensor(AccessoryInfo(name, manufacturer), 0.0, -20.0, 100.0, 1.0)
	th := GpioThermometer{
		name:        name,
		mutex:       sync.Mutex{},
		pin:         pin,
		microfarads: float64(nanoFarads) / 1000.0,
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
	pull := Float
	edge := RisingEdge
	t.mutex.Lock()
	defer t.mutex.Unlock()
	// Discharge the capacitor (low temps could make this really long)
	t.pin.Output(Low)
	time.Sleep(2 * maxTime)
	// Set to input
	t.pin.InputEdge(pull, edge)
	dt, state := t.pin.WaitForEdge(maxTime)
	if !state {
		Info("Thermometer %s, WaitForEdge(%s, %s) timed out after %s", t.name, pull, edge, dt)
		return time.Duration(0)
	}
	t.pin.Output(Low)
	Info("Discharge time for %s: %s", t.name, dt)
	return dt
}

// getTemp uses the Steinhart-Hart equation to calculate the temperature based on the
// resistance of the thermistor
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
		return fmt.Errorf("returned inconsistent data value(%0.4f) variance(%0.2f%%) entries(%d) - %+v",
			value, 100.0*h.Stddev()/h.Median(), h.Len(), h.data)
	}
	Debug("Setting adjustment to %0.3f", value)
	t.adjust = value
	return nil
}

func (t *GpioThermometer) inRange(dischargeTime time.Duration) bool {
	// Completely bogus, ignore
	if dischargeTime < minTime || dischargeTime > maxTime {
		return false
	}
	return true
}

// Temperature returns the current temperature of the GpioThermometer
func (t *GpioThermometer) Temperature() float64 {
	if time.Since(t.updated) > time.Minute {
		t.Update()
	}
	return t.accessory.TempSensor.CurrentTemperature.GetValue()
}

// Update updates the current temperature of the GpioThermometer
func (t *GpioThermometer) Update() error {
	var dischargeTime time.Duration
	h := NewHistory(5)
	for i := 0; h.Len() < 5 && i < 20; i++ {
		dischargeTime = t.getDischargeTime()
		if t.inRange(dischargeTime) {
			t.history.PushDuration(dischargeTime)
			h.PushDuration(dischargeTime)
		}
	}

	stdd := t.history.Stddev()
	avg := t.history.Average()
	med := t.history.Median()
	dev := stdd * 3

	// Throw away bad results
	if math.Abs(avg-h.Median()) > dev {
		Info("%s Thermometer update failed: Cur(%0.3f) Med(%0.3f) Avg(%0.3f) Stdd(%0.3f) Dev(%0.3f)",
			t.Name(),
			h.Median()/MillisecondFloat,
			med/MillisecondFloat,
			avg/MillisecondFloat,
			stdd/MillisecondFloat,
			dev/MillisecondFloat)
		return fmt.Errorf("could not update temperature successfully")
	}
	ohms := t.getOhms(time.Duration(int64(h.Median())))
	temp := t.getTemp(ohms)
	Debug("Calculating temperature (%f) for %s: %f ohms, median %s", temp, t.name, ohms, time.Duration(int64(h.Median())))
	t.accessory.TempSensor.CurrentTemperature.SetValue(temp)
	t.updated = time.Now()
	return nil
}

// Converts a temperature in Celsius to Farenheit
func toFarenheit(celsius float64) float64 {
	return (celsius * 9.0 / 5.0) + 32.0
}
