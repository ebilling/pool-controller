package main

import (
	"testing"
	"time"
)

func baseControlInputs(now time.Time) controlInputs {
	return controlInputs{
		now:            now,
		state:          OFF,
		water:          30,
		roof:           30,
		target:         30,
		tolerance:      0.5,
		deltaT:         5,
		dailyFrequency: 1,
		runTime:        2,
		lastStart:      now.Add(-time.Hour),
		lastStop:       now.Add(-48 * time.Hour),
	}
}

func TestDailyRunStartsOnlyInMorningWindow(t *testing.T) {
	day := time.Date(2026, time.September, 16, 0, 0, 0, 0, time.Local)
	for _, tc := range []struct {
		hour  int
		start bool
	}{
		{3, false},
		{4, true},
		{5, true},
		{6, false},
	} {
		in := baseControlInputs(day.Add(time.Duration(tc.hour) * time.Hour))
		got := decideControl(in)
		if (got.change && got.state == SWEEP) != tc.start {
			t.Errorf("hour %d: decision=%+v, want daily start=%t", tc.hour, got, tc.start)
		}
	}
}

func TestDailyRunFinishesAfterConfiguredRuntime(t *testing.T) {
	now := time.Date(2026, time.September, 16, 7, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = SWEEP
	in.lastStart = now.Add(-2 * time.Hour)

	got := decideControl(in)
	if !got.change || got.state != OFF {
		t.Fatalf("decision=%+v, want OFF after daily runtime", got)
	}
}

func TestDailyRunContinuesPastWindowUntilRuntime(t *testing.T) {
	now := time.Date(2026, time.September, 16, 7, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = SWEEP
	in.runTime = 6
	in.lastStart = now.Add(-3 * time.Hour)

	got := decideControl(in)
	if got.change {
		t.Fatalf("decision=%+v, daily run should continue past 06:00", got)
	}
}

func TestMixingStopsWhenDemandEnds(t *testing.T) {
	now := time.Now()
	in := baseControlInputs(now)
	in.state = MIXING
	in.lastStart = now.Add(-2 * time.Hour)

	got := decideControl(in)
	if !got.change || got.state != OFF {
		t.Fatalf("decision=%+v, want OFF after demand ends", got)
	}
}

func TestMixingContinuesWhileHeatingDemandExists(t *testing.T) {
	now := time.Now()
	in := baseControlInputs(now)
	in.state = MIXING
	in.water = 20
	in.roof = 40
	in.lastStart = now.Add(-2 * time.Hour)

	got := decideControl(in)
	if got.change {
		t.Fatalf("decision=%+v, active heating demand should continue", got)
	}
}

func TestSolarDisableDoesNotDisableDailyCleaning(t *testing.T) {
	now := time.Date(2026, time.September, 16, 4, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.solarDisabled = true

	got := decideControl(in)
	if !got.change || got.state != SWEEP {
		t.Fatalf("decision=%+v, want SWEEP with solar disabled", got)
	}
}

func TestDisableOverridesManualOperation(t *testing.T) {
	in := baseControlInputs(time.Now())
	in.state = PUMP
	in.manual = true
	in.disabled = true

	got := decideControl(in)
	if !got.change || got.state != DISABLED {
		t.Fatalf("decision=%+v, want safety disable", got)
	}
}
