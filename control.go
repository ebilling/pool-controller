package main

import "time"

const (
	dailyWindowStartHour = 4
	dailyWindowEndHour   = 6
	minimumPumpCycle     = time.Hour
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
	target         float64
	tolerance      float64
	deltaT         float64
	dailyFrequency float64
	runTime        float64
	lastStart      time.Time
	lastStop       time.Time
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

func wantsCooling(in controlInputs) bool {
	return !in.solarDisabled && !in.coolDisabled &&
		in.water > in.target+in.tolerance &&
		in.water > in.roof+in.deltaT
}

func wantsHeating(in controlInputs) bool {
	return !in.solarDisabled && !in.heatDisabled &&
		in.water < in.target-in.tolerance &&
		in.water < in.roof-in.deltaT
}

func dailyRunDue(in controlInputs) bool {
	frequency := time.Duration(in.dailyFrequency * 24 * float64(time.Hour))
	// A run normally finishes after the morning window. Permit the next run
	// six hours early so a configured N-day cadence can start at 04:00 rather
	// than slipping to the following day.
	frequency -= 6 * time.Hour
	if frequency < 12*time.Hour {
		frequency = 12 * time.Hour
	}
	return in.now.Sub(in.lastStop) > frequency
}

func decideControl(in controlInputs) controlDecision {
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

	// A non-manual SWEEP is the daily cleaning run. Once started, its stop is
	// based only on its configured runtime; crossing the 06:00 window must not
	// truncate it.
	if in.state == SWEEP {
		runtime := DurationFromHours(in.runTime, 1.0)
		if in.now.Sub(in.lastStart) >= runtime {
			return changeState(OFF, "daily cleaning complete")
		}
		return keepState(in, "daily cleaning")
	}

	cooling := wantsCooling(in)
	heating := wantsHeating(in)
	if cooling || heating {
		desired := SOLAR
		if in.water < in.target-in.deltaT || in.water > in.target+in.tolerance {
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

	if in.now.Hour() >= dailyWindowStartHour &&
		in.now.Hour() < dailyWindowEndHour &&
		dailyRunDue(in) {
		return changeState(SWEEP, "daily cleaning due")
	}

	if in.state > OFF && in.now.Sub(in.lastStart) >= minimumPumpCycle {
		return changeState(OFF, "temperature demand ended")
	}
	return keepState(in, "minimum pump cycle")
}
