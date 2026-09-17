package main

import (
	"sync"
	"time"

	"github.com/brutella/hap/accessory"
)

func simPinName(n uint8) string {
	switch n {
	case 5:
		return "power-led"
	case roofGpio:
		return "roof-therm"
	case waterGpio:
		return "pump-therm"
	case buttonGpio:
		return "button"
	case solarLedGpio:
		return "solar-led"
	case solarFwdGpio:
		return "solar-fwd"
	case solarRevGpio:
		return "solar-rev"
	case pumpGpio:
		return "pump-relay"
	case sweepGpio:
		return "sweep-relay"
	default:
		return "gpio"
	}
}

func simLogOutputs(n uint8) bool {
	switch n {
	case 5, solarLedGpio, solarFwdGpio, solarRevGpio, pumpGpio, sweepGpio:
		return true
	default:
		return false
	}
}

// EnableSimulation skips the Broadcom host driver and serves in-memory pins.
// Thermometer values are scripted separately via Config -simulate flags.
func EnableSimulation() {
	gpioInitFn = func() error { return nil }
	SetGpioProvider(newSimPin)
}

type simPin struct {
	mu        sync.Mutex
	gpio      uint8
	name      string
	logOut    bool
	state     GpioState
	direction Direction
}

func newSimPin(gpio uint8) PiPin {
	return &simPin{
		gpio:   gpio,
		name:   simPinName(gpio),
		logOut: simLogOutputs(gpio),
	}
}

func (p *simPin) Input() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.direction = Input
}

func (p *simPin) InputEdge(Pull, Edge) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.direction = Input
}

func (p *simPin) Output(s GpioState) {
	p.mu.Lock()
	changed := p.direction != Output || p.state != s
	p.direction = Output
	p.state = s
	logOut := p.logOut
	name := p.name
	n := p.gpio
	p.mu.Unlock()
	if logOut && changed {
		Info("sim %s gpio(%d) Output %s", name, n, s)
	}
}

func (p *simPin) Read() GpioState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// WaitForEdge never sees a physical edge. Buttons time out; thermistors are unused in sim.
func (p *simPin) WaitForEdge(timeout time.Duration) bool {
	time.Sleep(timeout)
	return false
}

func (p *simPin) Pin() uint8 {
	return p.gpio
}

// SimulatedThermometer publishes a scripted temperature to HomeKit and the controller.
type SimulatedThermometer struct {
	name      string
	temp      float64
	updateErr error
	updated   time.Time
	accessory *accessory.Thermometer
}

// NewSimulatedThermometer returns a thermometer that does not touch GPIO.
func NewSimulatedThermometer(name, manufacturer string, temp float64) *SimulatedThermometer {
	acc := NewTemperatureSensorAccessory(name, manufacturer)
	t := &SimulatedThermometer{
		name:      name,
		temp:      temp,
		updated:   time.Now(),
		accessory: acc,
	}
	acc.TempSensor.CurrentTemperature.SetValue(temp)
	return t
}

func (t *SimulatedThermometer) Name() string { return t.name }

func (t *SimulatedThermometer) Temperature() float64 {
	return t.accessory.TempSensor.CurrentTemperature.Value()
}

func (t *SimulatedThermometer) Update() error {
	if t.updateErr != nil {
		return t.updateErr
	}
	t.accessory.TempSensor.CurrentTemperature.SetValue(t.temp)
	t.updated = time.Now()
	return nil
}

// ReadingStatus reports a simulated probe as healthy unless a failure was
// scripted for it.
func (t *SimulatedThermometer) ReadingStatus() (time.Time, error) {
	return t.updated, t.updateErr
}

func (t *SimulatedThermometer) Calibrate(float64) error {
	return nil
}

func (t *SimulatedThermometer) Accessory() *accessory.A {
	return t.accessory.A
}

func (t *SimulatedThermometer) SetTemperature(temp float64) {
	t.temp = temp
	t.accessory.TempSensor.CurrentTemperature.SetValue(temp)
}
