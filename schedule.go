package main

import (
	"time"
)

// Schedule enables certain States at certain times
type Schedule struct {
	Events []*ScheduleEvent
}

// ScheduleEvent tells the system to go into a specific state at a specific time
type ScheduleEvent struct {
	Start   time.Time      // uses localtime to set up a particular hour and minute
	Runtime int            // minutes to run
	Days    []time.Weekday // 0-6 Sunday=0
	State   State          // State to enter at the requested time
}

// IsNow returns true if the event is active now, and the State requested by they event.  First active event wins.
func (s *Schedule) IsNow(t time.Time) (bool, State) {
	for _, se := range s.Events {
		now, state := se.IsNow(t)
		if now {
			return now, state
		}
	}
	return false, OFF
}

func sameday(t time.Time, days []time.Weekday) bool {
	// Is it the right day?
	for _, d := range days {
		if t.Weekday() == d {
			return true
		}
	}
	return false
}

// IsNow returns true if the event is active now, and the State requested by they event
func (se *ScheduleEvent) IsNow(t time.Time) (bool, State) {
	schedTime := time.Date(t.Year(), t.Month(), t.Day(), se.Start.UTC().Hour(), se.Start.UTC().Minute(), 0, 0, time.UTC)
	Debug("SchedTime:", schedTime.Weekday(), schedTime, "- In:", t.UTC().Weekday(), t.UTC())
	if !sameday(t.In(se.Start.Location()), se.Days) {
		return false, OFF
	}
	diff := schedTime.Sub(t.UTC())
	Debug("Diff:", diff, "Runtime:)", se.Runtime)
	if diff >= 0 && diff < time.Duration(se.Runtime)*time.Minute {
		return true, se.State
	}
	return false, OFF
}
