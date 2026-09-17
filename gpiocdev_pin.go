package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/ebilling/pool-controller/internal/gpiocdev"
)

// This is the GPIO backend. It talks to /dev/gpiochip* and times thermistor
// charges from the kernel's edge timestamp so this process's scheduling delay
// is not part of the measurement.

var (
	cdevMu   sync.Mutex
	cdevChip *gpiocdev.Chip
)

// cdevOpenChip opens the GPIO controller once and shares it between pins.
func cdevOpenChip() (*gpiocdev.Chip, error) {
	cdevMu.Lock()
	defer cdevMu.Unlock()
	if cdevChip != nil {
		return cdevChip, nil
	}
	chip, err := gpiocdev.Default()
	if err != nil {
		return nil, err
	}
	Info("GPIO: using character device %s", chip)
	cdevChip = chip
	return chip, nil
}

// cdevManagedPins are the BCM lines this process claims. Leftover
// /sys/class/gpio exports keep them busy and RequestLine fails with EBUSY
// until they are unexported.
func cdevManagedPins() []uint8 {
	return []uint8{
		5, // power LED in main.go
		roofGpio,
		waterGpio,
		buttonGpio,
		solarLedGpio,
		solarFwdGpio,
		solarRevGpio,
		pumpGpio,
		sweepGpio,
	}
}

func releaseLeftoverSysfs(chip *gpiocdev.Chip, pins []uint8) {
	for _, n := range pins {
		info, err := chip.LineInfo(uint32(n))
		if err != nil || !info.Used || info.Consumer != "sysfs" {
			continue
		}
		if err := gpiocdev.UnexportSysfs(uint32(n)); err != nil {
			Warn("GPIO: could not unexport leftover sysfs gpio(%d): %s", n, err)
			continue
		}
		Info("GPIO: released leftover sysfs export on gpio(%d)", n)
	}
}

// CdevGpioInit opens the GPIO controller.
func CdevGpioInit() error {
	chip, err := cdevOpenChip()
	if err != nil {
		return err
	}
	releaseLeftoverSysfs(chip, cdevManagedPins())
	return nil
}

// cdevPin implements PiPin against a single GPIO character device line.
//
// A failed claim leaves line nil rather than aborting startup. Every method
// guards for it so a single bad pin cannot panic the controller.
type cdevPin struct {
	gpio uint8
	line *gpiocdev.Line
}

// newCdevPin claims one line, addressed by BCM GPIO number.
func newCdevPin(gpio uint8) PiPin {
	p := &cdevPin{gpio: gpio}
	chip, err := cdevOpenChip()
	if err != nil {
		Error("GPIO: cannot open controller for gpio(%d): %s", gpio, err)
		return p
	}
	line, err := chip.RequestLine(uint32(gpio), "pool-controller")
	if err != nil {
		releaseLeftoverSysfs(chip, []uint8{gpio})
		line, err = chip.RequestLine(uint32(gpio), "pool-controller")
	}
	if err != nil {
		Error("GPIO: cannot claim gpio(%d): %s", gpio, err)
		return p
	}
	p.line = line
	return p
}

// Input sets the pin to be read from, with edge detection off.
func (p *cdevPin) Input() {
	p.InputEdge(Float, NoEdge)
}

// InputEdge sets the pin to be read from and alerts WaitForEdge on the edge.
func (p *cdevPin) InputEdge(pull Pull, e Edge) {
	if p.line == nil {
		return
	}
	Debug("Setting gpio(%d) to Input(%s, %s)", p.gpio, pull, e)
	if err := p.line.SetInput(cdevBias(pull), cdevEdges(e)); err != nil {
		Error("GPIO: gpio(%d) input(%s, %s): %s", p.gpio, pull, e, err)
	}
}

// Output sets the pin to be written to.
func (p *cdevPin) Output(s GpioState) {
	if p.line == nil {
		return
	}
	Debug("Output setting gpio(%d) to %s", p.gpio, s)
	if err := p.line.SetOutput(s == High); err != nil {
		Error("GPIO: gpio(%d) output(%s): %s", p.gpio, s, err)
	}
}

// Read returns the current state of the pin.
func (p *cdevPin) Read() GpioState {
	if p.line == nil {
		return Low
	}
	high, err := p.line.Value()
	if err != nil {
		Error("GPIO: gpio(%d) read: %s", p.gpio, err)
		return Low
	}
	if high {
		return High
	}
	return Low
}

// WaitForEdge blocks until the configured edge arrives or timeout elapses.
func (p *cdevPin) WaitForEdge(timeout time.Duration) bool {
	if p.line == nil {
		return false
	}
	_, ok, err := p.line.WaitEdge(timeout)
	if err != nil {
		Error("GPIO: gpio(%d) wait for edge: %s", p.gpio, err)
		return false
	}
	return ok
}

// Pin returns the GPIO number of the pin.
func (p *cdevPin) Pin() uint8 {
	return p.gpio
}

// ChargeTime satisfies rctime.ChargeMeter, letting the thermometer time a
// charge cycle from the kernel's edge timestamp.
func (p *cdevPin) ChargeTime(discharge, timeout time.Duration) (time.Duration, error) {
	if p.line == nil {
		// newCdevPin already logged why the claim failed. Return the error
		// rather than a zero duration, which would be read as 0 ohms.
		return 0, fmt.Errorf("gpio(%d) was never claimed", p.gpio)
	}
	return p.line.ChargeTime(discharge, timeout)
}

func cdevBias(p Pull) gpiocdev.Bias {
	switch p {
	case PullUp:
		return gpiocdev.BiasPullUp
	case PullDown:
		return gpiocdev.BiasPullDown
	case PullNoChange:
		return gpiocdev.BiasAsIs
	default:
		// Float: the divider supplies the bias, so disable the internal pull.
		return gpiocdev.BiasDisabled
	}
}

func cdevEdges(e Edge) gpiocdev.Edges {
	switch e {
	case RisingEdge:
		return gpiocdev.RisingEdge
	case FallingEdge:
		return gpiocdev.FallingEdge
	case BothEdges:
		return gpiocdev.BothEdges
	default:
		return gpiocdev.NoEdges
	}
}
