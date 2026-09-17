package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/brutella/hap/accessory"
)

type FakeThermometer struct {
	name           string
	temp           float64
	updateError    error
	calibrateError error
	updated        time.Time
	acc            *accessory.Thermometer
}

func (t *FakeThermometer) Name() string {
	return t.name
}
func (t *FakeThermometer) Temperature() float64 {
	return t.temp
}
func (t *FakeThermometer) Update() error {
	return t.updateError
}
func (t *FakeThermometer) Calibrate(float64) error {
	return t.calibrateError
}

// ReadingStatus reports a reading taken just now unless a test sets a time,
// so tests about temperature policy do not all have to think about freshness.
func (t *FakeThermometer) ReadingStatus() (time.Time, error) {
	if t.updated.IsZero() {
		return time.Now(), t.updateError
	}
	return t.updated, t.updateError
}
func (t *FakeThermometer) Accessory() *accessory.A {
	if t.acc == nil {
		t.acc = NewTemperatureSensorAccessory(t.name, "Unit Testing Intl")
	}
	return t.acc.A
}

type TestRunPumps struct {
	pumpTemp FakeThermometer
	roofTemp FakeThermometer
	ppc      *PoolPumpController
}

func (t *TestRunPumps) setConditions(target, pump, roof, outside float64, state State) {
	t.ppc.config.cfg.Target = target
	t.pumpTemp.temp = pump
	t.roofTemp.temp = roof
	t.ppc.runningTemp = &t.pumpTemp
	t.ppc.switches.state = state
}

func NewTestRunPumps() *TestRunPumps {
	defaultDataDir = "/tmp"
	config := NewConfig(flag.NewFlagSet("TestPumpController", flag.PanicOnError), []string{})
	t := TestRunPumps{
		pumpTemp: FakeThermometer{name: "pool", temp: 0.0},
		roofTemp: FakeThermometer{name: "roof", temp: 0.0},
		ppc:      NewPoolPumpController(config),
	}
	t.ppc.pumpTemp = &t.pumpTemp
	t.ppc.roofTemp = &t.roofTemp
	t.ppc.runningTemp = &t.pumpTemp
	return &t
}

func TestColdWaterHotWeather(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30.0, 15.0, 50.0, 33.0, OFF)
	if trp.ppc.shouldCool() {
		t.Error("Should not be cooling")
	}
	if !trp.ppc.shouldWarm() {
		t.Error("Should be trying to warm the pool")
	}
}

func TestWarmWaterHotWeather(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30.0, 29.98, 50.0, 33.0, OFF)
	if trp.ppc.shouldCool() {
		t.Error("Should not be cooling")
	}
	if trp.ppc.shouldWarm() {
		t.Error("Should not try to warm water that is already so close to the target")
	}
}

func TestHotWaterHotWeather(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30.0, 29.98, 50.0, 33.0, OFF)
	if trp.ppc.shouldCool() {
		t.Error("Should not be cooling")
	}
	if trp.ppc.shouldWarm() {
		t.Error("Should not try to warm water that is already so close to the target")
	}
}

func TestColdWaterWarmWeather(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30.0, 15.0, 40.0, 29.0, OFF)
	if trp.ppc.shouldCool() {
		t.Error("Should not be cooling")
	}
	if !trp.ppc.shouldWarm() {
		t.Error("Should be trying to warm the pool")
	}
}

func TestWarmWaterWarmWeather(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30.0, 29.98, 40.0, 29.0, OFF)
	if trp.ppc.shouldCool() {
		t.Error("Should not be cooling")
	}
	if trp.ppc.shouldWarm() {
		t.Error("Should not try to warm water that is already so close to the target")
	}
}

func TestHotWaterWarmWeather(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30.0, 29.98, 40.0, 29.0, OFF)
	if trp.ppc.shouldCool() {
		t.Error("Should not be cooling")
	}
	if trp.ppc.shouldWarm() {
		t.Error("Should not try to warm water that is already so close to the target")
	}
}

func TestRunPumpsIfNeeded(t *testing.T) {
	SetGpioProvider(NewTestPin)
	//trp := NewTestRunPumps()

	t.Run("", func(t *testing.T) {
	})

}

func TestRecordCleaningCompletionPersistsQualifyingMixing(t *testing.T) {
	dir := t.TempDir()
	persist := true
	now := time.Date(2026, time.September, 16, 13, 0, 0, 0, time.Local)
	start := now.Add(-2 * time.Hour)
	sweep := newRelay(&TestPin{}, "", "")
	sweep.startTime = start
	sweep.on = true
	config := &Config{
		persist:       &persist,
		dataDirectory: &dir,
		cfg: &PersistedConfig{
			RunTime:       2,
			ManualRunTime: 6,
		},
	}
	ppc := &PoolPumpController{
		config: config,
		switches: &Switches{
			state: MIXING,
			sweep: sweep,
		},
		now: func() time.Time { return now },
	}

	ppc.now = func() time.Time { return now.Add(-time.Second) }
	ppc.recordCleaningCompletion()
	if !config.cfg.LastCleaningCompleted.IsZero() {
		t.Fatal("a short sweep interval counted as cleaning")
	}

	ppc.now = func() time.Time { return now }
	ppc.recordCleaningCompletion()
	if got := config.cfg.LastCleaningCompleted; !got.Equal(now) {
		t.Fatalf("completion=%s, want %s", got, now)
	}

	data, err := os.ReadFile(filepath.Join(dir, serverConfiguration))
	if err != nil {
		t.Fatal(err)
	}
	var saved PersistedConfig
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if !saved.LastCleaningCompleted.Equal(now) {
		t.Fatalf("persisted completion=%s, want %s", saved.LastCleaningCompleted, now)
	}

	ppc.now = func() time.Time { return now.Add(time.Hour) }
	ppc.recordCleaningCompletion()
	if got := config.cfg.LastCleaningCompleted; !got.Equal(now) {
		t.Fatalf("one continuous sweep recorded twice: %s", got)
	}
}
