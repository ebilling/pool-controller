// Package gpiocdev drives Raspberry Pi GPIO through the Linux GPIO character
// device (uAPI v2), with no cgo and no third-party driver.
//
// The RC thermometer needs an edge timestamp it can trust. Polling
// /sys/class/gpio only reports that an edge happened, so the elapsed time
// includes however long it took to wake this process. With the 100 nF sensing
// capacitor a full charge is around 1 ms, so a late wakeup is a first-order
// error rather than noise.
//
// uAPI v2 line events carry a kernel timestamp taken in the GPIO interrupt
// handler, which removes the wakeup from the measurement, and waiting with a
// raw poll(2) keeps the runtime poller out of the path entirely.
package gpiocdev

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

// ErrUnsupported is returned on platforms without the GPIO character device.
var ErrUnsupported = errors.New("gpiocdev: GPIO character device requires Linux")

// ErrTimeout is returned when no edge arrives within the timeout.
var ErrTimeout = errors.New("gpiocdev: timed out waiting for edge")

// errInterrupted reports a signal during poll(2) so the wait can be resumed.
var errInterrupted = errors.New("gpiocdev: poll interrupted")

// Bias selects the line's internal pull resistor.
type Bias uint64

// Bias values. BiasAsIs leaves whatever the previous owner configured.
const (
	BiasAsIs     Bias = 0
	BiasDisabled Bias = Bias(flagBiasDisabled)
	BiasPullUp   Bias = Bias(flagBiasPullUp)
	BiasPullDown Bias = Bias(flagBiasPullDown)
)

// Edges selects which transitions generate line events.
type Edges uint64

// Edge detection values.
const (
	NoEdges     Edges = 0
	RisingEdge  Edges = Edges(flagEdgeRising)
	FallingEdge Edges = Edges(flagEdgeFalling)
	BothEdges   Edges = Edges(flagEdgeRising | flagEdgeFalling)
)

// Event is one edge reported by the kernel.
type Event struct {
	// TimestampNS is CLOCK_MONOTONIC, recorded by the kernel when the
	// interrupt fired rather than when this process read it.
	TimestampNS uint64
	// Rising distinguishes the two edges when both are enabled.
	Rising bool
	// LineSeqno counts events on this line; a jump means events were dropped
	// because the kernel's buffer overflowed.
	LineSeqno uint32
}

// Charge is one RC charge measurement.
type Charge struct {
	// Duration is the kernel edge timestamp minus CLOCK_MONOTONIC sampled
	// immediately before GPIO_V2_LINE_SET_CONFIG. On the Pi 3 the pinmux
	// happens at the start of that ioctl (empirically ~0% through it), so
	// this is the charge time plus a small, stable syscall/lock overhead.
	// It does not include the time to wake this process after the interrupt.
	Duration time.Duration
	// Config is how long SET_CONFIG took. It is a diagnostic, not an error
	// bar on Duration: using the pre-ioctl clock means ioctl-length jitter
	// does not land in Duration.
	Config time.Duration
	// Event is the underlying edge.
	Event Event
}

// Chip is an open GPIO controller, e.g. /dev/gpiochip0.
type Chip struct {
	fd    int
	path  string
	name  string
	label string
	lines uint32
}

// Open opens a specific GPIO character device by path.
func Open(path string) (*Chip, error) {
	fd, err := sysOpen(path)
	if err != nil {
		return nil, fmt.Errorf("gpiocdev: open %s: %w", path, err)
	}
	c := &Chip{fd: fd, path: path}
	var info chipInfo
	if err := sysIoctl(fd, ioctlChipInfo, unsafe.Pointer(&info)); err != nil {
		sysClose(fd)
		return nil, fmt.Errorf("gpiocdev: chip info %s: %w", path, err)
	}
	c.name = cstring(info.Name[:])
	c.label = cstring(info.Label[:])
	c.lines = info.Lines
	return c, nil
}

// Default opens the controller that owns the Pi header pins. On a Pi the
// bcm2835/2711 controller labels itself "pinctrl-*", and its line offsets are
// the BCM GPIO numbers. Other chips on the system (expanders, the Pi 5's RP1
// south bridge) are only used if no pinctrl controller is present.
func Default() (*Chip, error) {
	paths, err := filepath.Glob("/dev/gpiochip*")
	if err != nil {
		return nil, fmt.Errorf("gpiocdev: %w", err)
	}
	if len(paths) == 0 {
		if _, statErr := os.Stat("/dev"); statErr != nil {
			return nil, ErrUnsupported
		}
		return nil, errors.New("gpiocdev: no /dev/gpiochip* found; is this a Pi with GPIO support?")
	}
	sort.Strings(paths)

	var chosen *Chip
	var problems []string
	for _, path := range paths {
		c, err := Open(path)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		if strings.HasPrefix(c.label, "pinctrl-") {
			if chosen != nil {
				chosen.Close()
			}
			return c, nil
		}
		if chosen == nil {
			chosen = c
		} else {
			c.Close()
		}
	}
	if chosen != nil {
		return chosen, nil
	}
	return nil, fmt.Errorf("gpiocdev: no usable GPIO chip: %s", strings.Join(problems, "; "))
}

// Close releases the controller. Lines already requested from it stay valid.
func (c *Chip) Close() error {
	if c.fd < 0 {
		return nil
	}
	err := sysClose(c.fd)
	c.fd = -1
	return err
}

// Name is the kernel device name, e.g. "gpiochip0".
func (c *Chip) Name() string { return c.name }

// Label is the driver label, e.g. "pinctrl-bcm2835".
func (c *Chip) Label() string { return c.label }

// Lines is the number of lines the controller exposes.
func (c *Chip) Lines() uint32 { return c.lines }

func (c *Chip) String() string {
	return fmt.Sprintf("%s (%s, %d lines)", c.name, c.label, c.lines)
}

// LineInfo describes a line, including who owns it, without claiming it.
type LineInfo struct {
	Offset uint32
	// Name is the line's name from the device tree, e.g. "GPIO15" or "RXD0".
	Name string
	// Consumer is the driver or process holding the line, empty if free.
	Consumer string
	// Used reports whether anything currently holds the line. A line can be
	// used without a consumer name, which is typical of pins claimed by
	// pinctrl for an alternate function such as a UART.
	Used   bool
	Input  bool
	Output bool
}

func (li LineInfo) String() string {
	s := fmt.Sprintf("line %2d", li.Offset)
	if li.Name != "" {
		s += fmt.Sprintf(" %-12q", li.Name)
	}
	switch {
	case li.Output:
		s += " output"
	case li.Input:
		s += " input "
	}
	if !li.Used {
		return s + " free"
	}
	consumer := li.Consumer
	if consumer == "" {
		consumer = "an unnamed kernel driver (alternate pin function?)"
	}
	return s + " IN USE by " + consumer
}

// LineInfo reads a line's current ownership and direction.
func (c *Chip) LineInfo(offset uint32) (LineInfo, error) {
	if c.fd < 0 {
		return LineInfo{}, errors.New("gpiocdev: chip is closed")
	}
	info := lineInfo{Offset: offset}
	if err := sysIoctl(c.fd, ioctlLineInfo, unsafe.Pointer(&info)); err != nil {
		return LineInfo{}, fmt.Errorf("gpiocdev: line info %d on %s: %w", offset, c.name, err)
	}
	return LineInfo{
		Offset:   info.Offset,
		Name:     cstring(info.Name[:]),
		Consumer: cstring(info.Consumer[:]),
		Used:     info.Flags&flagUsed != 0,
		Input:    info.Flags&flagInput != 0,
		Output:   info.Flags&flagOutput != 0,
	}, nil
}

// UnexportSysfs releases a leftover /sys/class/gpio export so the character
// device can claim the line. Older builds exported pins this way and did not
// always unexport on exit; the kernel keeps the claim until reboot or this
// write.
func UnexportSysfs(offset uint32) error {
	err := os.WriteFile("/sys/class/gpio/unexport",
		[]byte(strconv.FormatUint(uint64(offset), 10)+"\n"), 0)
	if err != nil {
		return fmt.Errorf("gpiocdev: unexport GPIO %d: %w", offset, err)
	}
	return nil
}

// Line is an exclusive claim on one GPIO line.
//
// A Line is not safe for concurrent use. The line is requested as a
// high-impedance input so that claiming it cannot drive anything unexpectedly;
// call SetOutput or SetInput to configure it.
type Line struct {
	fd     int
	offset uint32
	chip   string
	flags  uint64
}

// RequestLine claims one line by its offset on the controller, which on a Pi is
// the BCM GPIO number. consumer is recorded by the kernel and shown by
// gpioinfo, so it should identify this process.
func (c *Chip) RequestLine(offset uint32, consumer string) (*Line, error) {
	if c.fd < 0 {
		return nil, errors.New("gpiocdev: chip is closed")
	}
	if c.lines > 0 && offset >= c.lines {
		return nil, fmt.Errorf("gpiocdev: %s has %d lines, cannot request offset %d", c.name, c.lines, offset)
	}
	req := lineRequest{
		NumLines: 1,
		// Deep enough to hold the edges of one measurement without the
		// kernel dropping events, small enough to stay cheap.
		EventBufferSize: 16,
	}
	req.Offsets[0] = offset
	// Leave room for the terminating NUL the kernel expects.
	copy(req.Consumer[:nameSize-1], consumer)
	req.Config.Flags = flagInput | flagBiasDisabled

	if err := sysIoctl(c.fd, ioctlGetLine, unsafe.Pointer(&req)); err != nil {
		// EBUSY only says "taken". Name the owner, because the usual cause is
		// a kernel driver holding the pin for an alternate function, and that
		// is not obvious from the errno alone.
		if sysIsBusy(err) {
			if info, infoErr := c.LineInfo(offset); infoErr == nil && info.Used {
				if info.Consumer == "sysfs" {
					return nil, fmt.Errorf("gpiocdev: %s %s; leftover /sys/class/gpio export — unexport GPIO %d first",
						c.name, info, offset)
				}
				return nil, fmt.Errorf("gpiocdev: %s %s; free it before claiming it here",
					c.name, info)
			}
		}
		return nil, fmt.Errorf("gpiocdev: request line %d on %s: %w", offset, c.name, err)
	}
	return &Line{
		fd:     int(req.Fd),
		offset: offset,
		chip:   c.name,
		flags:  req.Config.Flags,
	}, nil
}

// Close releases the line back to the kernel.
func (l *Line) Close() error {
	if l.fd < 0 {
		return nil
	}
	err := sysClose(l.fd)
	l.fd = -1
	return err
}

// Offset is the line's offset on its controller.
func (l *Line) Offset() uint32 { return l.offset }

func (l *Line) String() string { return fmt.Sprintf("%s:%d", l.chip, l.offset) }

// setConfig reconfigures the line in place, keeping the same fd and its queued
// events. outputValue applies only when the output flag is set.
func (l *Line) setConfig(flags uint64, outputValue bool) error {
	if l.fd < 0 {
		return errors.New("gpiocdev: line is closed")
	}
	cfg := lineConfig{Flags: flags}
	if flags&flagOutput != 0 {
		// Output values travel as an attribute whose mask selects which of
		// the requested lines it applies to. Only line 0 was requested.
		cfg.NumAttrs = 1
		cfg.Attrs[0].Attr.ID = attrIDOutputValues
		cfg.Attrs[0].Mask = 1
		if outputValue {
			cfg.Attrs[0].Attr.Value = 1
		}
	}
	if err := sysIoctl(l.fd, ioctlSetConfig, unsafe.Pointer(&cfg)); err != nil {
		return fmt.Errorf("gpiocdev: configure %s: %w", l, err)
	}
	l.flags = flags
	return nil
}

// SetOutput drives the line, disabling edge detection.
func (l *Line) SetOutput(high bool) error {
	return l.setConfig(flagOutput, high)
}

// SetInput stops driving the line and configures bias and edge detection.
func (l *Line) SetInput(bias Bias, edges Edges) error {
	return l.setConfig(flagInput|uint64(bias)|uint64(edges), false)
}

// SetValue changes the level of a line already configured for output. It is
// cheaper than SetOutput because it does not reconfigure the line.
func (l *Line) SetValue(high bool) error {
	if l.fd < 0 {
		return errors.New("gpiocdev: line is closed")
	}
	vals := lineValues{Mask: 1}
	if high {
		vals.Bits = 1
	}
	if err := sysIoctl(l.fd, ioctlSetValues, unsafe.Pointer(&vals)); err != nil {
		return fmt.Errorf("gpiocdev: set value %s: %w", l, err)
	}
	return nil
}

// Value reads the current level of the line.
func (l *Line) Value() (bool, error) {
	if l.fd < 0 {
		return false, errors.New("gpiocdev: line is closed")
	}
	vals := lineValues{Mask: 1}
	if err := sysIoctl(l.fd, ioctlGetValues, unsafe.Pointer(&vals)); err != nil {
		return false, fmt.Errorf("gpiocdev: get value %s: %w", l, err)
	}
	return vals.Bits&1 != 0, nil
}

// WaitEdge blocks until an edge arrives or timeout elapses, returning ok=false
// on timeout. A timeout of zero or less polls without blocking.
//
// SetInput must have enabled at least one edge first, otherwise the kernel never
// queues an event and this always times out.
func (l *Line) WaitEdge(timeout time.Duration) (Event, bool, error) {
	if l.fd < 0 {
		return Event{}, false, errors.New("gpiocdev: line is closed")
	}
	deadline, err := sysMonotonicNS()
	if err != nil {
		return Event{}, false, err
	}
	if timeout > 0 {
		deadline += uint64(timeout)
	}

	for {
		ms := 0
		if timeout > 0 {
			now, err := sysMonotonicNS()
			if err != nil {
				return Event{}, false, err
			}
			if now >= deadline {
				return Event{}, false, nil
			}
			// Round the remaining wait up so a sub-millisecond remainder
			// still gets one more chance instead of being truncated to a
			// non-blocking poll.
			remaining := deadline - now
			ms = int((remaining + uint64(time.Millisecond) - 1) / uint64(time.Millisecond))
		}

		ready, err := sysPoll(l.fd, ms)
		if errors.Is(err, errInterrupted) {
			if timeout > 0 {
				continue
			}
			return Event{}, false, nil
		}
		if err != nil {
			return Event{}, false, fmt.Errorf("gpiocdev: poll %s: %w", l, err)
		}
		if !ready {
			return Event{}, false, nil
		}
		ev, err := l.readEvent()
		if err != nil {
			return Event{}, false, err
		}
		return ev, true, nil
	}
}

// Drain discards edges already queued for this line, so a measurement is not
// resolved by a stale event from the previous cycle.
func (l *Line) Drain() error {
	if l.fd < 0 {
		return errors.New("gpiocdev: line is closed")
	}
	for {
		ready, err := sysPoll(l.fd, 0)
		if errors.Is(err, errInterrupted) {
			continue
		}
		if err != nil {
			return fmt.Errorf("gpiocdev: drain %s: %w", l, err)
		}
		if !ready {
			return nil
		}
		if _, err := l.readEvent(); err != nil {
			return err
		}
	}
}

func (l *Line) readEvent() (Event, error) {
	var raw lineEvent
	buf := (*[sizeofLineEvent]byte)(unsafe.Pointer(&raw))[:]
	n, err := sysRead(l.fd, buf)
	if err != nil {
		return Event{}, fmt.Errorf("gpiocdev: read event %s: %w", l, err)
	}
	if n != sizeofLineEvent {
		return Event{}, fmt.Errorf("gpiocdev: read event %s: got %d bytes, want %d", l, n, sizeofLineEvent)
	}
	return Event{
		TimestampNS: raw.TimestampNS,
		Rising:      raw.ID == eventRisingEdge,
		LineSeqno:   raw.LineSeqno,
	}, nil
}

// MeasureCharge times one RC charge cycle: drive the line low for discharge to
// empty the capacitor, release it, and wait up to timeout for the rising edge as
// the thermistor charges it back through the divider.
//
// The returned duration is measured from the release to the kernel's edge
// timestamp, so it excludes the time taken to schedule this process after the
// interrupt. The line is left driving low.
func (l *Line) MeasureCharge(discharge, timeout time.Duration) (Charge, error) {
	if err := l.SetOutput(false); err != nil {
		return Charge{}, err
	}
	time.Sleep(discharge)
	if err := l.Drain(); err != nil {
		return Charge{}, err
	}

	// The Pi 3 pinmux happens at the start of SET_CONFIG, not the middle:
	// correlating 200 samples against ioctl length gave slope ≈ -0.5 on the
	// midpoint estimate (pinmux at ~0% of the call). Clocking from `before`
	// keeps ioctl-duration jitter out of the measurement. Midpoint leaked
	// ~150 µs of that jitter into every sample and tripled the stdev.
	before, err := sysMonotonicNS()
	if err != nil {
		return Charge{}, err
	}
	err = l.SetInput(BiasDisabled, RisingEdge)
	after, clockErr := sysMonotonicNS()
	if err != nil {
		return Charge{}, err
	}
	if clockErr != nil {
		return Charge{}, clockErr
	}

	ev, ok, waitErr := l.WaitEdge(timeout)
	// Always leave the capacitor discharged, even on failure, so the next
	// measurement starts from a known state.
	if outErr := l.SetOutput(false); outErr != nil && waitErr == nil {
		return Charge{}, outErr
	}
	if waitErr != nil {
		return Charge{}, waitErr
	}
	if !ok {
		return Charge{}, fmt.Errorf("%w after %s on %s", ErrTimeout, timeout, l)
	}

	if ev.TimestampNS <= before {
		return Charge{}, fmt.Errorf("gpiocdev: %s edge timestamp %d precedes release %d (stale event?)",
			l, ev.TimestampNS, before)
	}
	return Charge{
		Duration: time.Duration(ev.TimestampNS - before),
		Config:   time.Duration(after - before),
		Event:    ev,
	}, nil
}

// ChargeTime is MeasureCharge reduced to the duration alone, which is the shape
// the thermometer consumes.
func (l *Line) ChargeTime(discharge, timeout time.Duration) (time.Duration, error) {
	c, err := l.MeasureCharge(discharge, timeout)
	if err != nil {
		return 0, err
	}
	return c.Duration, nil
}
