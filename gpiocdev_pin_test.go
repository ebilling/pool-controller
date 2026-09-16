package main

import (
	"errors"
	"testing"
	"time"

	"github.com/ebilling/pool-controller/internal/gpiocdev"
	"github.com/ebilling/pool-controller/internal/rctime"
)

// chargeMeterPin stands in for a pin whose driver times the charge cycle
// itself, the way a GPIO character device line does.
type chargeMeterPin struct {
	PiPin
	calls     int
	discharge time.Duration
	timeout   time.Duration
	dt        time.Duration
	err       error
}

func (p *chargeMeterPin) ChargeTime(discharge, timeout time.Duration) (time.Duration, error) {
	p.calls++
	p.discharge = discharge
	p.timeout = timeout
	return p.dt, p.err
}

func TestEdgePinPrefersKernelTiming(t *testing.T) {
	pin := &chargeMeterPin{PiPin: NewTestPin(15), dt: 900 * time.Microsecond}

	dt, err := rctime.Discharge(newEdgePin(pin))
	if err != nil {
		t.Fatalf("Discharge() returned %v, want no error", err)
	}
	if dt != pin.dt {
		t.Errorf("Discharge() = %s, want %s", dt, pin.dt)
	}
	if pin.calls != 1 {
		t.Errorf("ChargeTime called %d times, want 1", pin.calls)
	}
	if pin.discharge != rctime.DischargeTime || pin.timeout != rctime.MaxTime {
		t.Errorf("ChargeTime(%s, %s), want (%s, %s)",
			pin.discharge, pin.timeout, rctime.DischargeTime, rctime.MaxTime)
	}
}

func TestEdgePinReportsKernelTimingFailure(t *testing.T) {
	want := errors.New("no edge")
	pin := &chargeMeterPin{PiPin: NewTestPin(15), err: want}

	if _, err := rctime.Discharge(newEdgePin(pin)); !errors.Is(err, want) {
		t.Errorf("Discharge() returned %v, want %v", err, want)
	}
}

// A pin whose driver cannot timestamp edges has to keep working through the
// userspace path rather than losing readings.
func TestEdgePinFallsBackToUserspace(t *testing.T) {
	// The default TestPin takes 20ms to signal an edge, which is outside the
	// 10ms charge window, so use a pin that answers inside it.
	edge := newEdgePin(&TestPin{sleepTime: time.Millisecond, pin: 15, wake: make(chan bool)})
	if _, ok := edge.(rctime.ChargeMeter); ok {
		t.Fatal("a pin without ChargeTime should not be measured as a ChargeMeter")
	}

	dt, err := rctime.Discharge(edge)
	if err != nil {
		t.Fatalf("Discharge() returned %v, want no error", err)
	}
	if dt <= 0 {
		t.Errorf("Discharge() = %s, want a positive duration", dt)
	}
}

// An unclaimed line must report an error instead of reporting a charge time of
// zero, which the thermometer would read as 0 ohms and 0 degrees.
func TestUnclaimedCdevPinFailsLoudly(t *testing.T) {
	pin := &cdevPin{gpio: 15}

	if _, err := pin.ChargeTime(rctime.DischargeTime, rctime.MaxTime); err == nil {
		t.Error("ChargeTime() on an unclaimed line returned no error")
	}
	if pin.WaitForEdge(time.Millisecond) {
		t.Error("WaitForEdge() on an unclaimed line reported an edge")
	}
	if got := pin.Read(); got != Low {
		t.Errorf("Read() on an unclaimed line = %s, want %s", got, Low)
	}
	if got := pin.Pin(); got != 15 {
		t.Errorf("Pin() = %d, want 15", got)
	}
}

func TestCdevPinImplementsChargeMeter(t *testing.T) {
	var pin PiPin = &cdevPin{gpio: 15}
	if _, ok := pin.(rctime.ChargeMeter); !ok {
		t.Error("cdevPin should offer kernel-timestamped charge measurement")
	}
}

func TestCdevBiasMapping(t *testing.T) {
	tests := []struct {
		in   Pull
		want gpiocdev.Bias
	}{
		{PullUp, gpiocdev.BiasPullUp},
		{PullDown, gpiocdev.BiasPullDown},
		{PullNoChange, gpiocdev.BiasAsIs},
		// The resistor divider supplies the bias, so the internal pull has to
		// be off or it would skew the charge time.
		{Float, gpiocdev.BiasDisabled},
	}
	for _, tc := range tests {
		if got := cdevBias(tc.in); got != tc.want {
			t.Errorf("cdevBias(%s) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestCdevEdgeMapping(t *testing.T) {
	tests := []struct {
		in   Edge
		want gpiocdev.Edges
	}{
		{NoEdge, gpiocdev.NoEdges},
		{RisingEdge, gpiocdev.RisingEdge},
		{FallingEdge, gpiocdev.FallingEdge},
		{BothEdges, gpiocdev.BothEdges},
	}
	for _, tc := range tests {
		if got := cdevEdges(tc.in); got != tc.want {
			t.Errorf("cdevEdges(%s) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestCdevManagedPins(t *testing.T) {
	want := map[uint8]bool{
		5: true, 14: true, 15: true, 18: true,
		21: true, 22: true, 23: true, 24: true, 25: true,
	}
	got := cdevManagedPins()
	if len(got) != len(want) {
		t.Fatalf("cdevManagedPins() has %d entries, want %d", len(got), len(want))
	}
	for _, n := range got {
		if !want[n] {
			t.Errorf("unexpected managed pin %d", n)
		}
		delete(want, n)
	}
	for n := range want {
		t.Errorf("missing managed pin %d", n)
	}
}
