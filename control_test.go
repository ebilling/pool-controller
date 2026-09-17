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
		waterUpdated:   now,
		roofUpdated:    now,
		target:         30,
		tolerance:      0.5,
		deltaT:         5,
		dailyFrequency: 1,
		runTime:        2,
		lastStart:      now.Add(-time.Hour),
		lastStop:       now.Add(-48 * time.Hour),
		sweepStart:     now.Add(-time.Hour),
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

// The water temperature is left over from the last run while the pumps are
// off, so solar is committed to only after circulating long enough to measure
// the pool as it is now.
func TestSolarDemandStartsWithAPumpOnlySample(t *testing.T) {
	now := time.Date(2026, time.September, 16, 14, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.water = 20
	in.roof = 40

	got := decideControl(in)
	if !got.change || got.state != PUMP {
		t.Fatalf("decision=%+v, want a PUMP sample before solar", got)
	}
}

func TestSampleRunCirculatesBeforeDeciding(t *testing.T) {
	now := time.Date(2026, time.September, 16, 14, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = PUMP
	in.lastStart = now.Add(-time.Minute)
	in.water = 20
	in.roof = 40

	got := decideControl(in)
	if got.change {
		t.Fatalf("decision=%+v, want the sample to finish first", got)
	}
}

func TestSampleRunEngagesSolarWhenDemandSurvives(t *testing.T) {
	now := time.Date(2026, time.September, 16, 14, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = PUMP
	in.lastStart = now.Add(-poolSampleTime - time.Minute)
	in.water = 20
	in.roof = 40

	got := decideControl(in)
	if !got.change || got.state != MIXING {
		t.Fatalf("decision=%+v, want solar once the fresh reading agrees", got)
	}
}

// This is the case that wasted a run: the pool turns out not to need solar
// once it is actually measured. It stops without waiting out the hour that
// applies to demand-driven runs, but still finishes its minimum run.
func TestSampleRunStopsWhenFreshWaterNeedsNothing(t *testing.T) {
	now := time.Date(2026, time.September, 16, 14, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = PUMP
	in.lastStart = now.Add(-minimumPumpRun - time.Minute)

	got := decideControl(in)
	if !got.change || got.state != OFF {
		t.Fatalf("decision=%+v, want OFF once the sample shows no demand", got)
	}
}

func TestSampleRunFinishesItsMinimumRunBeforeStopping(t *testing.T) {
	now := time.Date(2026, time.September, 16, 14, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = PUMP
	in.lastStart = now.Add(-poolSampleTime - time.Minute)

	got := decideControl(in)
	if got.change {
		t.Fatalf("decision=%+v, want the pump to finish its minimum run", got)
	}
}

// Adding the sweep pump and moving the solar valve are not motor cycling, so
// escalating is never held back.
func TestEscalatingToMixingIsNotHeldBack(t *testing.T) {
	now := time.Date(2026, time.September, 16, 14, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = PUMP
	in.lastStart = now.Add(-poolSampleTime - time.Minute)
	in.sweepStart = now.Add(-3 * time.Hour)
	in.water = 20
	in.roof = 40

	got := decideControl(in)
	if !got.change || got.state != MIXING {
		t.Fatalf("decision=%+v, want MIXING without waiting", got)
	}
}

// MIXING starts the sweep pump long after the main one, so backing out of it
// is timed against the sweep's own start.
func TestBackingOutOfMixingWaitsForTheSweepPump(t *testing.T) {
	now := time.Date(2026, time.September, 16, 14, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = MIXING
	in.lastStart = now.Add(-3 * time.Hour)
	in.sweepStart = now.Add(-2 * time.Minute)

	got := decideControl(in)
	if got.change {
		t.Fatalf("decision=%+v, want to wait out the sweep pump's minimum run", got)
	}

	in.sweepStart = now.Add(-minimumPumpRun - time.Minute)
	got = decideControl(in)
	if !got.change || got.state != OFF {
		t.Fatalf("decision=%+v, want OFF once both pumps have run long enough", got)
	}
}

// Disabling is a safety request, not a policy decision, so it is immediate.
func TestDisablingIgnoresMinimumRun(t *testing.T) {
	now := time.Date(2026, time.September, 16, 14, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = MIXING
	in.lastStart = now.Add(-time.Minute)
	in.sweepStart = now.Add(-time.Minute)
	in.disabled = true

	got := decideControl(in)
	if !got.change || got.state != DISABLED {
		t.Fatalf("decision=%+v, want an immediate DISABLED", got)
	}
}

// A probe keeps reporting its last good value when sampling fails, so a dead
// roof sensor must not be read as a reason to keep heating, nor as a reason to
// shut down: hold until it reads again.
func TestStaleReadingsHoldTheCurrentState(t *testing.T) {
	now := time.Date(2026, time.September, 16, 14, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = MIXING
	in.lastStart = now.Add(-2 * time.Hour)
	in.roofUpdated = now.Add(-maxReadingAge - time.Minute)

	got := decideControl(in)
	if got.change {
		t.Fatalf("decision=%+v, want to hold state while a probe is unreadable", got)
	}
}

func TestStaleReadingsDoNotStartSolar(t *testing.T) {
	now := time.Date(2026, time.September, 16, 14, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.water = 20
	in.roof = 40
	in.roofUpdated = now.Add(-maxReadingAge - time.Minute)

	got := decideControl(in)
	if got.change {
		t.Fatalf("decision=%+v, an unreadable probe is not a reason to run", got)
	}
}

// Cleaning is scheduled by the clock, so it does not depend on the probes.
func TestStaleReadingsStillAllowDailyCleaning(t *testing.T) {
	now := time.Date(2026, time.September, 16, 4, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.roofUpdated = now.Add(-maxReadingAge - time.Minute)
	in.waterUpdated = now.Add(-maxReadingAge - time.Minute)

	got := decideControl(in)
	if !got.change || got.state != SWEEP {
		t.Fatalf("decision=%+v, want SWEEP regardless of probe health", got)
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
