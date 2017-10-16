package main

import (
	"github.com/stianeikeland/go-rpio"
	"testing"
	"time"
)

type TestPin struct {
	state        rpio.State
	direction    rpio.Direction
	sleepTime    time.Duration
	inputTime    time.Time
}

func (p *TestPin) Input() {
	p.direction = rpio.Input
	p.inputTime = time.Now()
	Debug("Setting Fake pin to Input mode %v", p)
}

func (p *TestPin) Output() {
	p.direction = rpio.Output
	Debug("Setting Fake pin to Output mode %v", p)
}

func (p *TestPin) High() {
	p.state = rpio.High
	Debug("Setting Fake pin state to High %v", p)
}

func (p *TestPin) Low() {
	p.state = rpio.Low
	Debug("Setting Fake pin state to Low %v", p)
}

func (p *TestPin) Read() rpio.State {
	now := time.Now()
	sleeptime := p.inputTime.Add(p.sleepTime)
	if p.sleepTime > 0 && now.After(sleeptime) {
		p.state = rpio.High
	}
	return p.state
}

func (p *TestPin) PullUp() {}

func (p *TestPin) PullDown() {}

func (p *TestPin) PullOff() {}

func TestGpioThermometer(t *testing.T) {
	sleeptime := 100 * time.Millisecond
	pin := TestPin{
		state:        rpio.Low,
		direction:    rpio.Input,
		sleepTime:    sleeptime,
		inputTime:    time.Now(),
	}
	therm := NewGpioThermometer(22, 10.0)
	therm.pin = &pin

	Debug("States (H=%d, L=%d) Direction (I=%d, O=%d)", rpio.High, rpio.Low,
		rpio.Input, rpio.Output)
	Debug("Therm: %v", therm)
	
	t.Run("getDischargeTime", func (t *testing.T) {
		d := therm.getDischargeTime()
		if d / 1000000 != 100 {
			t.Errorf("Expected %dms got %dms",
			sleeptime/1000000, d/1000000)
		}
	})

	t.Run("getOhms", func (t *testing.T) {
		expected := 20000
		o := therm.getOhms(100000000)
		if int(o) != expected {
			t.Errorf("Expected %d ohms found %d ohms",
				expected, int(o))
		}
	})

	t.Run("getTemp", func (t *testing.T) {
		expected := [][]int{{105000, -20}, {25380,5}, {9900,25},
			{3601,50}, {670,100}}
		for _, val := range expected {
			th := therm.getTemp(float64(val[0]))
			if int(th) != val[1] {
				t.Errorf("Expected %d, found %0.1f", val[1], th)
			}
		}
	})

	t.Run("Update/Temperature", func (t *testing.T) {
		expected := 12
		therm.Update()
		therm.temperature = float64(expected)
		if int(therm.Temperature()) != expected {
			t.Errorf("Expected %d, found %0.1f",
				expected, therm.Temperature())
		}
		pin.sleepTime = 10 * time.Millisecond
		therm.Update()
		expected = 65
		if int(therm.Temperature()) != expected {
			t.Errorf("Expected %d, found %0.1f",
				expected, therm.Temperature())
		}
	})

	t.Run("Stop/Done", func (t *testing.T) {
		if therm.Done() == true {
			t.Errorf("GPIO thermometer stopped, should not be")
		}
		therm.Stop()
		if therm.Done() == false {
			t.Errorf("GPIO thermometer stopped, should not be")
		}
	})
}
