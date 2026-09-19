package main

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/brutella/hap/accessory"
	"github.com/ebilling/pool-controller/internal/rctime"
)

// Thermometer reads a thermal resistance thermometer using the timings of a capacitor charge/discharge cycle
type Thermometer interface {
	Name() string
	Temperature() float64
	Update() error
	Calibrate(float64) error
	Accessory() *accessory.A
	// ReadingStatus reports when this thermometer last took a reading and the
	// error from its most recent attempt. Temperature keeps returning the last
	// good value when sampling fails, so callers that act on a reading have to
	// ask how old it is.
	ReadingStatus() (time.Time, error)
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
	acc := NewTemperatureSensorAccessory(name, manufacturer)
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
	return t.accessory.TempSensor.CurrentTemperature.Value()
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
func (t *SelectiveThermometer) Accessory() *accessory.A {
	return t.accessory.A
}

// ReadingStatus reports the status of the thermometer being filtered. This one
// holds its value on purpose whenever the filter rejects a reading, so the
// probe behind it is the only meaningful health signal.
func (t *SelectiveThermometer) ReadingStatus() (time.Time, error) {
	return t.thermometer.ReadingStatus()
}

// GpioThermometer is used to measure the temperature of a given resistive thermometer
// using a capacitor.
type GpioThermometer struct {
	name                  string
	mutex                 sync.Mutex
	updateMutex           sync.Mutex
	stateMutex            sync.RWMutex
	pin                   PiPin
	edge                  rctime.EdgePin
	microfarads           float64
	adjust                float64
	updated               time.Time
	lastErr               error
	clampShortPulsesAsHot bool
	shortPulseClampActive bool
	history               History
	accessory             *accessory.Thermometer
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
	acc := NewTemperatureSensorAccessory(name, manufacturer)
	th := GpioThermometer{
		name:        name,
		mutex:       sync.Mutex{},
		pin:         pin,
		edge:        newEdgePin(pin),
		microfarads: rctime.Microfarads,
		adjust:      2.5,
		history:     *NewHistory(100),
		updated:     time.Now().Add(-24 * time.Hour),
		accessory:   acc,
	}
	return &th
}

// SetAdjustment provides a multiplier to the teperature sensor
func (t *GpioThermometer) SetAdjustment(a float64) {
	t.stateMutex.Lock()
	defer t.stateMutex.Unlock()
	t.adjust = a
}

// Adjustment returns the current calibration multiplier.
func (t *GpioThermometer) Adjustment() float64 {
	t.stateMutex.RLock()
	defer t.stateMutex.RUnlock()
	return t.adjust
}

// ClampShortPulsesAsHot treats a sustained set of pulses below the supported
// timing window as a temperature above the measurable range. This is suitable
// for the roof probe, where an NTC thermistor's pulse gets shorter as it gets
// hotter and knowing "hot enough" is sufficient to control solar heating.
func (t *GpioThermometer) ClampShortPulsesAsHot() {
	t.stateMutex.Lock()
	t.clampShortPulsesAsHot = true
	t.stateMutex.Unlock()
}

// Name returns the name of the GpioThermometer
func (t *GpioThermometer) Name() string {
	return t.name
}

// Accessory returns the Apple HomeKit accessory related to the GpioThermometer
func (t *GpioThermometer) Accessory() *accessory.A {
	return t.accessory.A
}

func (t *GpioThermometer) getDischargeTime() time.Duration {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	dt, err := rctime.Discharge(t.edge)
	if err != nil {
		Debug("Thermometer %s could not measure a charge time: %s", t.name, err)
		return 0
	}
	Debug("Discharge time for %s: %s", t.name, dt)
	return dt
}

type gpioEdgePin struct{ pin PiPin }

func (p gpioEdgePin) OutputLow() { p.pin.Output(Low) }

func (p gpioEdgePin) InputRisingFloat() { p.pin.InputEdge(Float, RisingEdge) }

func (p gpioEdgePin) WaitForEdge(d time.Duration) bool { return p.pin.WaitForEdge(d) }

// timedEdgePin is a pin whose driver can time the charge cycle itself, against
// a clock that does not include this process's scheduling delay.
type timedEdgePin struct {
	gpioEdgePin
	rctime.ChargeMeter
}

// newEdgePin adapts a PiPin for rctime, preferring the driver's own
// kernel-timestamped measurement when it offers one.
func newEdgePin(pin PiPin) rctime.EdgePin {
	if m, ok := pin.(rctime.ChargeMeter); ok {
		return timedEdgePin{gpioEdgePin{pin}, m}
	}
	return gpioEdgePin{pin}
}

func (t *GpioThermometer) getTemp(ohms float64) float64 {
	return rctime.Temp(ohms)
}

func (t *GpioThermometer) getOhms(dischargeTime time.Duration) float64 {
	t.stateMutex.RLock()
	adjust := t.adjust
	t.stateMutex.RUnlock()
	return rctime.Ohms(dischargeTime, adjust)
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
		return fmt.Errorf("returned inconsistent data value(%0.4f) variance(%0.2f%%) entries(%d)",
			value, 100.0*h.Stddev()/h.Median(), h.Len())
	}
	Debug("Setting adjustment to %0.3f", value)
	t.SetAdjustment(value)
	return nil
}

func (t *GpioThermometer) inRange(dischargeTime time.Duration) bool {
	return rctime.InRange(dischargeTime)
}

// Temperature returns the current temperature of the GpioThermometer
func (t *GpioThermometer) Temperature() float64 {
	return t.accessory.TempSensor.CurrentTemperature.Value()
}

// ReadingStatus reports when a reading last succeeded and the most recent
// sampling error. Temperature deliberately does not perform GPIO I/O; the
// controller loop is the single owner of sensor sampling.
func (t *GpioThermometer) ReadingStatus() (time.Time, error) {
	t.stateMutex.RLock()
	defer t.stateMutex.RUnlock()
	return t.updated, t.lastErr
}

func (t *GpioThermometer) recordError(err error) error {
	t.stateMutex.Lock()
	t.lastErr = err
	t.stateMutex.Unlock()
	return err
}

func (t *GpioThermometer) shouldClampShortPulses() bool {
	t.stateMutex.RLock()
	defer t.stateMutex.RUnlock()
	return t.clampShortPulsesAsHot
}

func (t *GpioThermometer) recordShortPulseClamp(shortPulses, attempts int) error {
	temp := t.getTemp(t.getOhms(rctime.MinTime))
	t.accessory.TempSensor.CurrentTemperature.SetValue(temp)

	t.stateMutex.Lock()
	wasActive := t.shortPulseClampActive
	t.shortPulseClampActive = true
	t.updated = time.Now()
	t.lastErr = nil
	t.stateMutex.Unlock()

	if !wasActive {
		Warn("%s thermometer is above its measurable range: clamping to %0.1fC "+
			"after %d of %d pulses were shorter than %s",
			t.name, temp, shortPulses, attempts, rctime.MinTime)
	}
	return nil
}

// Update updates the current temperature of the GpioThermometer
func (t *GpioThermometer) Update() error {
	t.updateMutex.Lock()
	defer t.updateMutex.Unlock()

	var dischargeTime time.Duration
	h := NewHistory(5)
	shortPulses := 0
	attempts := 0
	for ; h.Len() < 5 && attempts < 20; attempts++ {
		dischargeTime = t.getDischargeTime()
		if dischargeTime > 0 && dischargeTime < rctime.MinTime {
			shortPulses++
		} else if t.inRange(dischargeTime) {
			h.PushDuration(dischargeTime)
		}
	}
	if h.Len() < 5 {
		if t.shouldClampShortPulses() && shortPulses >= 10 {
			return t.recordShortPulseClamp(shortPulses, attempts)
		}
		return t.recordError(fmt.Errorf(
			"only %d valid temperature samples out of %d attempts (%d pulses shorter than %s)",
			h.Len(), attempts, shortPulses, rctime.MinTime))
	}

	t.stateMutex.RLock()
	wasClamped := t.shortPulseClampActive
	t.stateMutex.RUnlock()
	if wasClamped {
		// Samples taken while saturated are deliberately not added to the
		// statistical history. Start a new baseline when measurable pulses
		// return so the old, cooler baseline does not reject the recovery.
		t.history = *NewHistory(100)
	}
	for _, sample := range h.data[:h.Len()] {
		t.history.Push(sample)
	}

	stdd := t.history.Stddev()
	avg := t.history.Average()
	med := t.history.Median()
	dev := stdd * 3

	// Throw away bad results
	if !wasClamped && math.Abs(avg-h.Median()) > dev {
		Info("%s Thermometer update failed: Cur(%0.3f) Med(%0.3f) Avg(%0.3f) Stdd(%0.3f) Dev(%0.3f)",
			t.Name(),
			h.Median()/MillisecondFloat,
			med/MillisecondFloat,
			avg/MillisecondFloat,
			stdd/MillisecondFloat,
			dev/MillisecondFloat)
		return t.recordError(fmt.Errorf("could not update temperature successfully"))
	}
	ohms := t.getOhms(time.Duration(int64(h.Median())))
	temp := t.getTemp(ohms)
	Debug("Calculating temperature (%f) for %s: %f ohms, median %s", temp, t.name, ohms, time.Duration(int64(h.Median())))
	t.accessory.TempSensor.CurrentTemperature.SetValue(temp)
	t.stateMutex.Lock()
	wasClamped = t.shortPulseClampActive
	t.shortPulseClampActive = false
	t.updated = time.Now()
	t.lastErr = nil
	t.stateMutex.Unlock()
	if wasClamped {
		Info("%s thermometer returned to its measurable range at %0.1fC", t.name, temp)
	}
	return nil
}

// Converts a temperature in Celsius to Farenheit
func toFarenheit(celsius float64) float64 {
	return (celsius * 9.0 / 5.0) + 32.0
}
