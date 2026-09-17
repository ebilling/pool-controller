package main

import "time"

const (
	fallbackStartHour = 4
	fallbackEndHour   = 6
	pvStartHour       = 11
	swimStartHour     = 14
	swimEndHour       = 17
	// minimumPumpRun is how long a pump motor keeps running once it starts.
	// Short runs are what wear a motor out, so an automatic decision to stop
	// one waits this out. Starting a second motor while the system is already
	// running is not a cycle and is never held back. This has to stay longer
	// than poolSampleTime, or a sampling run could stop before its reading is
	// worth anything.
	minimumPumpRun = 5 * time.Minute
	// minimumPumpRest is how long everything stays off before an automatic
	// decision starts a motor again. A minimum run alone does not bound how
	// often a motor starts, which is the wear that matters; this does.
	minimumPumpRest = 15 * time.Minute
	// poolSampleTime is how long to circulate without solar before trusting
	// the water temperature. The probe sits by the pump, not in the pool, so
	// its reading only describes the pool once flow has replaced what was
	// standing in the pipes.
	poolSampleTime = 3 * time.Minute
	// maxReadingAge is how old a temperature may be and still justify running
	// the pumps. A thermometer keeps reporting its last good value when
	// sampling fails, so a probe that dies on a hot afternoon would otherwise
	// hold a roof temperature that calls for solar all night. The control loop
	// samples every few seconds; this is a long stretch of failures. The status
	// page flags a stale reading sooner than this, so a probe going bad is
	// visible before it changes what the pumps do.
	maxReadingAge = 2 * time.Minute
)

// controlInputs is an immutable snapshot of everything needed to choose the
// next relay state. Keeping policy pure makes clock and temperature edge cases
// testable without GPIO, HomeKit, or RRD files.
type controlInputs struct {
	now            time.Time
	state          State
	manual         bool
	disabled       bool
	solarDisabled  bool
	heatDisabled   bool
	coolDisabled   bool
	water          float64
	roof           float64
	waterUpdated   time.Time
	roofUpdated    time.Time
	target         float64
	tolerance      float64
	deltaT         float64
	dailyFrequency float64
	runTime        float64
	lastStart      time.Time
	lastStop       time.Time
	sweepStart     time.Time
	lastCleaning   time.Time
}

type controlDecision struct {
	state  State
	change bool
	reason string
}

func thermostatDisables(mode ThermostatMode) (heat, cool bool) {
	switch mode {
	case ThermostatOff:
		return true, true
	case ThermostatHeat:
		return false, true
	case ThermostatCool:
		return true, false
	default:
		return false, false
	}
}

func keepState(in controlInputs, reason string) controlDecision {
	return controlDecision{state: in.state, reason: reason}
}

func changeState(state State, reason string) controlDecision {
	return controlDecision{state: state, change: true, reason: reason}
}

// readingsFresh reports whether both probes produced a reading recently enough
// to act on. Without this, a temperature the controller can no longer measure
// still looks like a reason to run the pumps.
func readingsFresh(in controlInputs) bool {
	return in.now.Sub(in.roofUpdated) <= maxReadingAge &&
		in.now.Sub(in.waterUpdated) <= maxReadingAge
}

func wantsCooling(in controlInputs) bool {
	return !in.solarDisabled && !in.coolDisabled && readingsFresh(in) &&
		in.water > in.target+in.tolerance &&
		in.water > in.roof+in.deltaT
}

func wantsHeating(in controlInputs) bool {
	return !in.solarDisabled && !in.heatDisabled && readingsFresh(in) &&
		in.water < in.target-in.tolerance &&
		in.water < in.roof-in.deltaT
}

func cleaningRuntime(hours float64) time.Duration {
	return DurationFromHours(hours, 2.0)
}

func cleaningDue(in controlInputs) bool {
	if in.lastCleaning.IsZero() {
		return true
	}
	frequency := time.Duration(in.dailyFrequency * 24 * float64(time.Hour))
	return !in.now.Before(in.lastCleaning.Add(frequency))
}

func hourInRange(now time.Time, start, end int) bool {
	return now.Hour() >= start && now.Hour() < end
}

func inSwimWindow(now time.Time) bool {
	return hourInRange(now, swimStartHour, swimEndHour)
}

func inPreSwimMixWindow(now time.Time) bool {
	return hourInRange(now, pvStartHour, swimStartHour)
}

func canStartPreSwimSweep(in controlInputs) bool {
	cutoff := time.Date(in.now.Year(), in.now.Month(), in.now.Day(),
		swimStartHour, 0, 0, 0, in.now.Location()).Add(-minimumPumpRun)
	return in.now.Before(cutoff)
}

func inPreferredCleaningStartWindow(in controlInputs) bool {
	start := time.Date(in.now.Year(), in.now.Month(), in.now.Day(),
		pvStartHour, 0, 0, 0, in.now.Location())
	cutoff := time.Date(in.now.Year(), in.now.Month(), in.now.Day(),
		swimStartHour, 0, 0, 0, in.now.Location()).Add(-cleaningRuntime(in.runTime))
	return !in.now.Before(start) && !in.now.After(cutoff)
}

func sweepCompletedThisRun(in controlInputs) bool {
	return !in.lastCleaning.IsZero() && !in.lastCleaning.Before(in.sweepStart)
}

// pumpRunning reports whether the main pump circulates in a given state.
func pumpRunning(s State) bool {
	return s == PUMP || s == SWEEP || s == SOLAR || s == MIXING
}

// sweepRunning reports whether the sweep pump, a second motor, runs in a
// given state.
func sweepRunning(s State) bool {
	return s == SWEEP || s == MIXING
}

// limitPumpCycling paces the motors. It holds back an automatic change that
// would stop a motor that has not run long enough yet, or start everything
// again too soon after a stop. Each pump is timed separately, because MIXING
// starts the sweep pump well after the main one.
//
// Adding a motor to a system that is already running is not a cycle, so it
// passes straight through, as does moving the solar valve. Only automatic
// decisions arrive here: a request from the button or from HomeKit is applied
// directly and stays immediate.
func limitPumpCycling(in controlInputs, d controlDecision) controlDecision {
	if !d.change || d.state == DISABLED {
		return d
	}
	if !pumpRunning(in.state) && pumpRunning(d.state) &&
		in.now.Sub(in.lastStop) < minimumPumpRest {
		return keepState(in, "pumps have not rested long enough to start")
	}
	if pumpRunning(in.state) && !pumpRunning(d.state) &&
		in.now.Sub(in.lastStart) < minimumPumpRun {
		return keepState(in, "pump has not run long enough to stop")
	}
	if sweepRunning(in.state) && !sweepRunning(d.state) &&
		in.now.Sub(in.sweepStart) < minimumPumpRun {
		return keepState(in, "sweep pump has not run long enough to stop")
	}
	return d
}

func decideControl(in controlInputs) controlDecision {
	return limitPumpCycling(in, decidePumpState(in))
}

func decidePumpState(in controlInputs) controlDecision {
	if in.disabled {
		if in.state != DISABLED {
			return changeState(DISABLED, "controller disabled")
		}
		return keepState(in, "controller disabled")
	}
	if in.manual {
		return keepState(in, "manual override")
	}
	if in.state == DISABLED {
		return changeState(OFF, "controller enabled")
	}

	cooling := wantsCooling(in)
	heating := wantsHeating(in)

	// Swimming wins over automatic cleaning and mixing. Preserve the main
	// pump and solar valve when possible, but get the sweep out of the pool.
	if inSwimWindow(in.now) && sweepRunning(in.state) {
		if in.state == MIXING {
			if cooling || heating || !readingsFresh(in) {
				return changeState(SOLAR, "swim window: stopping sweep pump")
			}
			return changeState(OFF, "swim window: solar demand ended")
		}
		if cooling || heating {
			return changeState(SOLAR, "swim window: stopping sweep pump")
		}
		if !readingsFresh(in) {
			return changeState(PUMP, "swim window: stopping sweep pump")
		}
		return changeState(OFF, "swim window: stopping cleaning")
	}

	// Once equipment is running, an unreadable probe is not evidence that the
	// current operation should change. Scheduled cleaning may still start
	// while probes are stale because it does not depend on temperature.
	if in.state > OFF && !readingsFresh(in) {
		return keepState(in, "holding state until temperatures can be measured")
	}

	// A non-manual PUMP is the sampling run started below. Let it circulate
	// before its reading is used, then fall through and decide again.
	if in.state == PUMP && in.now.Sub(in.lastStart) < poolSampleTime {
		return keepState(in, "sampling pool temperature")
	}

	// Solar already running at the start of the pre-swim window is exactly
	// when the pump-side probe is most likely to be seeing the cover-heated
	// surface layer. Start the sweep before believing that apparent bulk
	// temperature, even if the transient reading says demand just ended.
	if in.state == SOLAR && inPreSwimMixWindow(in.now) &&
		canStartPreSwimSweep(in) {
		return changeState(MIXING, "destatifying solar-heated surface water")
	}

	// A sweep in progress is one continuous cleaning/destatification run,
	// whether the solar valve is open or not. Before swimming, use solar when
	// it is useful and otherwise keep sweeping until the water is mixed well
	// enough to trust and the run qualifies as cleaning.
	if sweepRunning(in.state) {
		if (cooling || heating) && inPreSwimMixWindow(in.now) {
			if in.state != MIXING {
				return changeState(MIXING, "pre-swim solar mixing")
			}
			return keepState(in, "pre-swim solar mixing")
		}
		if !sweepCompletedThisRun(in) {
			if in.state != SWEEP {
				return changeState(SWEEP, "destatifying pool before solar")
			}
			return keepState(in, "continuous sweep run")
		}
		if cooling || heating {
			return changeState(SOLAR, "sweep complete; solar demand continues")
		}
		return changeState(OFF, "continuous sweep run complete")
	}

	// PUMP here is an automatic sample that was started because the previous
	// bulk reading called for solar. During the pre-swim period, a hot result
	// can just be the surface skin. Destatify before accepting "no demand."
	if in.state == PUMP && inPreSwimMixWindow(in.now) &&
		canStartPreSwimSweep(in) && !cooling && !heating {
		return changeState(SWEEP, "destatifying before trusting pool temperature")
	}

	if cooling || heating {
		// While the pumps are off, the water temperature is left over from the
		// end of the last run and can be hours old. Circulate without solar
		// first, so this is decided on what the pool reads now.
		if in.state == OFF {
			if inPreSwimMixWindow(in.now) && canStartPreSwimSweep(in) {
				return changeState(MIXING, "pre-swim solar mixing")
			}
			reason := "sampling pool temperature before solar heating"
			if cooling {
				reason = "sampling pool temperature before solar cooling"
			}
			return changeState(PUMP, reason)
		}
		desired := SOLAR
		if inPreSwimMixWindow(in.now) && canStartPreSwimSweep(in) {
			desired = MIXING
		}
		if in.state != desired {
			reason := "solar heating"
			if cooling {
				reason = "solar cooling"
			}
			return changeState(desired, reason)
		}
		return keepState(in, "temperature demand continues")
	}

	if cleaningDue(in) &&
		(inPreferredCleaningStartWindow(in) ||
			hourInRange(in.now, fallbackStartHour, fallbackEndHour)) {
		return changeState(SWEEP, "continuous sweep cleaning due")
	}

	// Turning off is a decision about temperature, so it needs a temperature
	// worth believing. Hold the current state until the probes read again
	// rather than acting on a value the controller can no longer measure.
	if !readingsFresh(in) {
		return keepState(in, "holding state until temperatures can be measured")
	}

	// The sampling run has served its purpose: the demand check above has now
	// seen a current reading and found nothing to do. It was a measurement
	// rather than a demand-driven run, so it does not wait out the minimum
	// cycle. The fresh reading it produced is what keeps this from starting
	// over immediately.
	if in.state == PUMP {
		return changeState(OFF, "pool temperature does not call for solar")
	}

	if in.state > OFF {
		return changeState(OFF, "temperature demand ended")
	}
	return keepState(in, "no automatic demand")
}
