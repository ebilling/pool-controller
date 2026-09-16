// Package rctime is the production RC thermometer conversion and timing window.
// Capacitor is 100 nF (0.1 µF).
package rctime

import (
	"errors"
	"math"
	"time"
)

const (
	// NanoFarads is the production sensing capacitor.
	NanoFarads = 100
	// Microfarads is NanoFarads expressed in µF.
	Microfarads = float64(NanoFarads) / 1000.0
)

// MinTime and MaxTime are the accepted capacitor charge durations.
var (
	MinTime = time.Duration(NanoFarads) * time.Microsecond
	MaxTime = time.Duration(NanoFarads) / 10 * time.Millisecond
	// DischargeTime is how long the pin is driven low to empty the capacitor
	// before a measurement.
	DischargeTime = 2 * MaxTime
)

// ErrNoEdge means the capacitor never charged past the input threshold within
// MaxTime, so there is no reading to report.
var ErrNoEdge = errors.New("rctime: no rising edge within the charge window")

// EdgePin is the GPIO surface needed to time one charge cycle from userspace.
type EdgePin interface {
	OutputLow()
	InputRisingFloat()
	WaitForEdge(time.Duration) bool
}

// ChargeMeter is implemented by pins that can time a charge cycle against a
// clock that excludes this process's scheduling delay, such as the kernel edge
// timestamps carried by GPIO character device events.
//
// Discharge prefers it whenever it is available. With the 100 nF capacitor a
// full charge is roughly 1 ms, so the delay between the interrupt firing and
// this process being scheduled is a first-order error in the userspace path,
// not noise that averaging can remove.
type ChargeMeter interface {
	ChargeTime(discharge, timeout time.Duration) (time.Duration, error)
}

// Discharge times one charge cycle: drive the pin low to drain the capacitor,
// release it, and measure how long the thermistor takes to charge it past the
// input threshold.
func Discharge(p EdgePin) (time.Duration, error) {
	if m, ok := p.(ChargeMeter); ok {
		return m.ChargeTime(DischargeTime, MaxTime)
	}

	p.OutputLow()
	time.Sleep(DischargeTime)
	start := time.Now()
	p.InputRisingFloat()
	ok := p.WaitForEdge(MaxTime)
	dt := time.Since(start)
	p.OutputLow()
	if !ok {
		return 0, ErrNoEdge
	}
	return dt, nil
}

// InRange reports whether a duration is inside the production acceptance window.
func InRange(dt time.Duration) bool {
	return dt >= MinTime && dt <= MaxTime
}

// Ohms converts a charge duration to resistance using the live adjust factor.
func Ohms(dischargeTime time.Duration, adjust float64) float64 {
	uSec := adjust * float64(dischargeTime) / float64(time.Microsecond)
	return uSec / Microfarads
}

// Temp converts thermistor resistance to Celsius using the production curve.
func Temp(ohms float64) float64 {
	const a = 79463.85
	const b = 0.1453676
	const c = 2.517178e-15
	const d = -132.2399
	if ohms == 0.0 {
		return 0.0
	}
	return d + (a-d)/(1+math.Pow(ohms/c, b))
}
