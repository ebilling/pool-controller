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
		lastCleaning:   now.Add(-12 * time.Hour),
	}
}

func TestCleaningStartsInPVWindowOrFallback(t *testing.T) {
	day := time.Date(2026, time.September, 16, 0, 0, 0, 0, time.Local)
	for _, tc := range []struct {
		hour  int
		start bool
	}{
		{3, false},
		{4, true},
		{5, true},
		{6, false},
		{10, false},
		{11, true},
		{12, true},
		{13, false},
	} {
		in := baseControlInputs(day.Add(time.Duration(tc.hour) * time.Hour))
		in.lastCleaning = day.Add(-3 * 24 * time.Hour)
		got := decideControl(in)
		if (got.change && got.state == SWEEP) != tc.start {
			t.Errorf("hour %d: decision=%+v, want cleaning start=%t", tc.hour, got, tc.start)
		}
	}
}

func TestCleaningFrequencyStartsAtLastQualifyingSweep(t *testing.T) {
	completed := time.Date(2026, time.September, 14, 12, 0, 0, 0, time.Local)
	in := baseControlInputs(completed.Add(48*time.Hour - time.Second))
	in.dailyFrequency = 2
	in.lastCleaning = completed
	if cleaningDue(in) {
		t.Fatal("cleaning became due before two full days elapsed")
	}
	in.now = completed.Add(48 * time.Hour)
	if !cleaningDue(in) {
		t.Fatal("cleaning did not become due two days after completion")
	}
}

func TestSolarOnlyStopDoesNotPostponeCleaning(t *testing.T) {
	now := time.Date(2026, time.September, 16, 11, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.lastCleaning = now.Add(-3 * 24 * time.Hour)
	in.lastStop = now.Add(-20 * time.Minute)

	got := decideControl(in)
	if !got.change || got.state != SWEEP {
		t.Fatalf("decision=%+v, want cleaning based on sweep completion, not main-pump stop", got)
	}
}

func TestSweepContinuesUntilItQualifiesAsCleaning(t *testing.T) {
	now := time.Date(2026, time.September, 16, 7, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = SWEEP
	in.sweepStart = now.Add(-time.Hour)
	in.lastCleaning = now.Add(-3 * 24 * time.Hour)

	got := decideControl(in)
	if got.change {
		t.Fatalf("decision=%+v, sweep should remain continuous until it qualifies", got)
	}
}

func TestMixingStopsWhenDemandEnds(t *testing.T) {
	now := time.Date(2026, time.September, 16, 18, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = MIXING
	in.lastStart = now.Add(-2 * time.Hour)
	in.sweepStart = now.Add(-2 * time.Hour)
	in.lastCleaning = now.Add(-time.Minute)

	got := decideControl(in)
	if !got.change || got.state != OFF {
		t.Fatalf("decision=%+v, want OFF after demand ends", got)
	}
}

func TestMixingContinuesWhileHeatingDemandExists(t *testing.T) {
	now := time.Date(2026, time.September, 16, 12, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = MIXING
	in.water = 20
	in.roof = 40
	in.lastStart = now.Add(-2 * time.Hour)
	in.sweepStart = now.Add(-time.Hour)

	got := decideControl(in)
	if got.change {
		t.Fatalf("decision=%+v, active heating demand should continue", got)
	}
}

func TestSolarDisableDoesNotDisableDailyCleaning(t *testing.T) {
	now := time.Date(2026, time.September, 16, 4, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.solarDisabled = true
	in.lastCleaning = now.Add(-3 * 24 * time.Hour)

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
	if !got.change || got.state != SOLAR {
		t.Fatalf("decision=%+v, want solar once the fresh reading agrees", got)
	}
}

func TestSolarDemandStartsPreSwimMixingInPVWindow(t *testing.T) {
	now := time.Date(2026, time.September, 16, 11, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.water = 20
	in.roof = 40

	got := decideControl(in)
	if !got.change || got.state != MIXING {
		t.Fatalf("decision=%+v, want MIXING before the swim window", got)
	}
}

func TestPreSwimWindowDestatifiesAlreadyRunningSolar(t *testing.T) {
	now := time.Date(2026, time.September, 16, 11, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = SOLAR
	in.lastStart = now.Add(-time.Hour)

	got := decideControl(in)
	if !got.change || got.state != MIXING {
		t.Fatalf("decision=%+v, want to mix the surface layer before swimming", got)
	}
}

func TestPreSwimHotPumpSampleStartsSweepBeforeBeingTrusted(t *testing.T) {
	now := time.Date(2026, time.September, 16, 11, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = PUMP
	in.lastStart = now.Add(-poolSampleTime - time.Minute)

	got := decideControl(in)
	if !got.change || got.state != SWEEP {
		t.Fatalf("decision=%+v, want to destatify an apparent hot surface reading", got)
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

func TestUselessSolarStopsAfterMinimumMotorRun(t *testing.T) {
	now := time.Date(2026, time.September, 16, 18, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = SOLAR
	in.lastStart = now.Add(-2 * time.Minute)

	if got := decideControl(in); got.change {
		t.Fatalf("decision=%+v, main pump has not completed its minimum run", got)
	}

	in.lastStart = now.Add(-minimumPumpRun - time.Minute)
	got := decideControl(in)
	if !got.change || got.state != OFF {
		t.Fatalf("decision=%+v, useless solar should stop after five minutes", got)
	}
}

// A minimum run does not bound how often a motor starts, so a cold start
// waits out a rest period as well.
func TestStartingAgainWaitsForTheRestPeriod(t *testing.T) {
	now := time.Date(2026, time.September, 16, 14, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.water = 20
	in.roof = 40
	in.lastStop = now.Add(-2 * time.Minute)

	got := decideControl(in)
	if got.change {
		t.Fatalf("decision=%+v, want to let the pumps rest first", got)
	}

	in.lastStop = now.Add(-minimumPumpRest - time.Minute)
	got = decideControl(in)
	if !got.change || got.state != PUMP {
		t.Fatalf("decision=%+v, want a sample once the pumps have rested", got)
	}
}

// Adding the sweep pump and moving the solar valve are not motor cycling, so
// escalating is never held back.
func TestEscalatingToMixingIsNotHeldBack(t *testing.T) {
	now := time.Date(2026, time.September, 16, 11, 30, 0, 0, time.Local)
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

func TestSwimWindowDropsSweepButKeepsUsefulSolar(t *testing.T) {
	now := time.Date(2026, time.September, 16, 14, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.state = MIXING
	in.lastStart = now.Add(-3 * time.Hour)
	in.sweepStart = now.Add(-3 * time.Hour)
	in.water = 20
	in.roof = 40

	got := decideControl(in)
	if !got.change || got.state != SOLAR {
		t.Fatalf("decision=%+v, want SOLAR with sweep removed at 14:00", got)
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
	now := time.Date(2026, time.September, 16, 18, 0, 0, 0, time.Local)
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
	in.lastCleaning = now.Add(-3 * 24 * time.Hour)

	got := decideControl(in)
	if !got.change || got.state != SWEEP {
		t.Fatalf("decision=%+v, want SWEEP regardless of probe health", got)
	}
}

func TestCleaningNeverStartsInSwimWindow(t *testing.T) {
	now := time.Date(2026, time.September, 16, 15, 0, 0, 0, time.Local)
	in := baseControlInputs(now)
	in.lastCleaning = now.Add(-3 * 24 * time.Hour)

	got := decideControl(in)
	if got.change {
		t.Fatalf("decision=%+v, sweep must not start during the swim window", got)
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
