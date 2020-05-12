package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSameDay(t *testing.T) {
	tfmt := "2006-01-02 15:04 -0700"
	testdata := []struct {
		name     string
		now      string
		days     []time.Weekday
		expected bool
	}{
		{"MondayMidDay", "2006-01-02 15:04 -0700", []time.Weekday{time.Tuesday, time.Wednesday}, false},
		{"MidnightMonday", "2006-01-02 23:59 -0700", []time.Weekday{time.Tuesday, time.Wednesday}, false},
		{"TuesdayMidnightAM", "2006-01-03 00:00 -0700", []time.Weekday{time.Tuesday, time.Wednesday}, true},
		{"TuesdayMidDay", "2006-01-03 13:15 -0700", []time.Weekday{time.Tuesday, time.Wednesday}, true},
		{"WednesdayMidnight", "2006-01-04 23:59 -0700", []time.Weekday{time.Tuesday, time.Wednesday}, true},
		{"ThursdayMidnight", "2006-01-05 00:00 -0700", []time.Weekday{time.Tuesday, time.Wednesday}, false},
	}
	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			tm, err := time.Parse(tfmt, td.now)
			assert.Nil(t, err)
			assert.Equal(t, td.expected, sameday(tm, td.days), "Expected %s to be in %+v", tm.Weekday(), td.days)
		})
	}
}

func TestIsNow(t *testing.T) {
	tfmt := "2006-01-02 15:04 -0700"
	clockTxt := "2006-01-10 13:15 -0600" // the time being fed into the test
	clock, err := time.Parse(tfmt, clockTxt)
	assert.Nil(t, err)
	twodays := []time.Weekday{time.Tuesday, time.Wednesday}
	testdata := []struct {
		name     string
		now      string
		days     []time.Weekday
		expected bool
		state    State
	}{
		{"MondayMidDay", "2006-01-01 15:04 -0700", twodays, false, STATE_OFF},
		{"MidnightMonday", "2006-01-02 23:59 -0700", twodays, false, STATE_OFF},
		{"TuesdayMidnightAM", "2006-01-03 00:00 -0700", twodays, false, STATE_OFF},
		{"JustBeforeWindow", "2006-01-03 12:14 -0700", twodays, false, STATE_OFF},
		{"WindowBorder", "2006-01-03 12:15 -0700", twodays, true, STATE_SWEEP},
		{"InWindow", "2006-01-03 12:40 -0700", twodays, true, STATE_SWEEP},
		{"EndOfWindow", "2006-01-03 13:14 -0700", twodays, true, STATE_SWEEP},
		{"EndWindowBorder", "2006-01-03 13:15 -0700", twodays, false, STATE_OFF},
		{"WednesdayMidnight", "2006-01-04 23:59 -0700", twodays, false, STATE_OFF},
	}
	for _, td := range testdata {
		t.Run(td.name, func(t *testing.T) {
			tm, err := time.Parse(tfmt, td.now)
			assert.Nil(t, err)
			s := &Schedule{
				Events: []*ScheduleEvent{{Start: tm, Runtime: 60, Days: td.days, State: STATE_SWEEP}},
			}
			run, state := s.IsNow(clock)
			assert.Equal(t, td.expected, run, "Expected run=%t at %s", td.expected, tm)
			assert.Equal(t, td.state, state, "Expected %s found %s", td.state, state)
		})
	}

}
