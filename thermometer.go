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
	maxTime = time.Duration(nanoFarads) * time.Millisecond / 10
)

// Thermometer reads a thermal resistance thermometer using the timings of a capacitor charge/discharge cycle
type Thermometer interface {
	Name() string
	Temperature() float64
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
func (t *SelectiveThermometer) Calibrate(_ float64) error {
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
	history     *History
	last        Notification
	calibrating bool
	accessory   *accessory.Thermometer
}

// NewGpioThermometer returns a GpioThermometer
func NewGpioThermometer(name string, manufacturer string, gpio uint) *GpioThermometer {
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
		history:     NewHistory(100),
		updated:     time.Now().Add(-24 * time.Hour),
		accessory:   acc,
	}
	th.startWatcher()
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

func (t *GpioThermometer) startWatcher() {
	t.pin.Watch(t.handler, Float, FallingEdge, High)
}

func (t *GpioThermometer) handler(n Notification) error {
	duration := n.Time.Sub(t.last.Time)
	t.last = n
	if duration < 10*time.Millisecond && duration > time.Microsecond*50 {
		t.history.PushDuration(duration)
	}
	median := t.history.Duration(t.history.Median())
	ohms := t.getOhms(median)
	temp := t.getTemp(ohms)
	Info("Temperature (%fC / %fF) for %s: %f ohms, median %s", temp, toFarenheit(temp), t.name, ohms, median)
	t.accessory.TempSensor.CurrentTemperature.SetValue(temp)
	t.updated = time.Now()
	if t.calibrating {
		return errors.New("calibrating - exiting early")
	}
	return nil
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

func (t *GpioThermometer) calibrationHandler(n Notification) error {
	duration := n.Time.Sub(t.last.Time)
	t.last = n
	if duration < 10*time.Millisecond && duration > time.Microsecond*50 {
		t.history.PushDuration(duration)
	}
	if t.history.ttl >= 20 {
		return errors.New("done calibrating")
	}
	return nil
}

// Calibrate asserts a specific resistance and calculates the proper setting
// for the adjust parameter
func (t *GpioThermometer) Calibrate(ohms float64) error {
	t.calibrating = true
	calculated := ohms * t.microfarads / 1000.0
	Info("Expecting %0.3f ms", calculated)
	h := NewHistory(20)
	t.history = h
	t.pin.Watch(t.calibrationHandler, Float, RisingEdge, Low)
	for t.history.ttl < 20 {
		time.Sleep(10 * time.Millisecond)
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
	t.calibrating = false
	t.startWatcher()
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
		// thread must have exited, restart it
		t.startWatcher()
	}

	return t.accessory.TempSensor.CurrentTemperature.GetValue()
}

// Converts a temperature in Celsius to Farenheit
func toFarenheit(celsius float64) float64 {
	return (celsius * 9.0 / 5.0) + 32.0
}
