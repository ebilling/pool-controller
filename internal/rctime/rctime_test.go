package rctime

import (
	"testing"
	"time"
)

func TestTempKnownPoints(t *testing.T) {
	cases := []struct {
		ohms float64
		want int
	}{
		{105000, -20},
		{25380, 5},
		{9900, 25},
		{3601, 50},
		{670, 100},
	}
	for _, c := range cases {
		got := int(Temp(c.ohms))
		if got != c.want {
			t.Errorf("Temp(%v)=%d want %d", c.ohms, got, c.want)
		}
	}
}

func TestOhmsScale(t *testing.T) {
	const adjust = 2.5
	dt := 100 * time.Millisecond
	want := adjust * 100000 / Microfarads
	got := Ohms(dt, adjust)
	if int(got) != int(want) {
		t.Errorf("Ohms=%v want %v", got, want)
	}
}

func TestInRange(t *testing.T) {
	if InRange(MinTime - 1) {
		t.Fatal("below min")
	}
	if !InRange(MinTime) || !InRange(MaxTime) {
		t.Fatal("bounds should be accepted")
	}
	if InRange(MaxTime + 1) {
		t.Fatal("above max")
	}
}
