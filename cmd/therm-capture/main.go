// therm-capture times the production 100 nF RC thermometers on a Pi.
// Stop pool-controller first so two processes do not drive the same pins.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/ebilling/pool-controller/internal/gpiocdev"
	"github.com/ebilling/pool-controller/internal/rctime"
)

const (
	defaultPumpGPIO = 15
	defaultRoofGPIO = 14
	defaultAdjust   = 1.75 // deployed real.conf
)

type sample struct {
	Time    time.Time `json:"time"`
	Name    string    `json:"name"`
	GPIO    uint8     `json:"gpio"`
	Driver  string    `json:"driver"`
	Ns      int64     `json:"ns"`
	Timeout bool      `json:"timeout"`
	InRange bool      `json:"in_range"`
	Ohms    float64   `json:"ohms"`
	TempC   float64   `json:"temp_c"`
	Adjust  float64   `json:"adjust"`
	CapNF   int       `json:"cap_nF"`
	// ConfigNs is how long SET_CONFIG took. It is a diagnostic, not an
	// error bar: the charge duration is clocked from before that ioctl.
	ConfigNs int64 `json:"config_ns"`
	// Err records why a sample failed, if it did.
	Err string `json:"err,omitempty"`
}

// prober times one charge cycle on one thermistor.
type prober interface {
	measure() (dt, config time.Duration, err error)
	close()
}

// cdevPin times the charge from the kernel's edge timestamp.
type cdevPin struct {
	line *gpiocdev.Line
}

func (p *cdevPin) measure() (time.Duration, time.Duration, error) {
	c, err := p.line.MeasureCharge(rctime.DischargeTime, rctime.MaxTime)
	if err != nil {
		return 0, 0, err
	}
	return c.Duration, c.Config, nil
}

func (p *cdevPin) close() { p.line.Close() }

func openCdev(chip *gpiocdev.Chip, n uint8) (prober, error) {
	line, err := chip.RequestLine(uint32(n), "therm-capture")
	if err != nil {
		return nil, err
	}
	return &cdevPin{line: line}, nil
}

func takeSample(name string, n uint8, driver string, p prober, adjust float64) sample {
	s := sample{
		Time:   time.Now().UTC(),
		Name:   name,
		GPIO:   n,
		Driver: driver,
		Adjust: adjust,
		CapNF:  rctime.NanoFarads,
	}
	dt, config, err := p.measure()
	if err != nil {
		s.Timeout = true
		s.Err = err.Error()
		return s
	}
	s.Ns = dt.Nanoseconds()
	s.ConfigNs = config.Nanoseconds()
	s.InRange = rctime.InRange(dt)
	if s.InRange {
		s.Ohms = rctime.Ohms(dt, adjust)
		s.TempC = rctime.Temp(s.Ohms)
	}
	return s
}

func summarize(name string, samples []sample) {
	var ns, unc []float64
	timeouts, inRange := 0, 0
	reasons := map[string]int{}
	for _, s := range samples {
		if s.Timeout {
			timeouts++
			if s.Err != "" {
				reasons[s.Err]++
			}
			continue
		}
		ns = append(ns, float64(s.Ns))
		unc = append(unc, float64(s.ConfigNs))
		if s.InRange {
			inRange++
		}
	}
	fmt.Fprintf(os.Stderr, "%s: n=%d timeouts=%d in_range=%d", name, len(samples), timeouts, inRange)
	if len(ns) == 0 {
		fmt.Fprintln(os.Stderr)
		for reason, count := range reasons {
			fmt.Fprintf(os.Stderr, "  %d x %s\n", count, reason)
		}
		return
	}
	mean, med, stdev := stats(ns)
	uMean, _, _ := stats(unc)
	fmt.Fprintf(os.Stderr, " duration_us mean=%.1f median=%.1f stdev=%.1f setinput_us=%.1f\n",
		mean/1000, med/1000, stdev/1000, uMean/1000)
	for reason, count := range reasons {
		fmt.Fprintf(os.Stderr, "  %d x %s\n", count, reason)
	}
}

func stats(v []float64) (mean, median, stdev float64) {
	if len(v) == 0 {
		return 0, 0, 0
	}
	sorted := append([]float64(nil), v...)
	sort.Float64s(sorted)
	sum := 0.0
	for _, x := range sorted {
		sum += x
	}
	mean = sum / float64(len(sorted))
	median = sorted[len(sorted)/2]
	var ss float64
	for _, x := range sorted {
		d := x - mean
		ss += d * d
	}
	if len(sorted) > 1 {
		stdev = math.Sqrt(ss / float64(len(sorted)-1))
	}
	return mean, median, stdev
}

// listLines reports the thermistor lines and every other line currently held,
// which is how to tell whether something else on the system owns a pin.
func listLines(pump, roof uint8) error {
	chip, err := gpiocdev.Default()
	if err != nil {
		return err
	}
	defer chip.Close()

	fmt.Printf("%s\n\nthermistor lines:\n", chip)
	for _, l := range []struct {
		name string
		n    uint8
	}{{"pump", pump}, {"roof", roof}} {
		info, err := chip.LineInfo(uint32(l.n))
		if err != nil {
			return err
		}
		fmt.Printf("  %-5s %s\n", l.name, info)
	}

	fmt.Printf("\nall claimed lines:\n")
	claimed := 0
	for n := uint32(0); n < chip.Lines(); n++ {
		info, err := chip.LineInfo(n)
		if err != nil {
			return err
		}
		if info.Used {
			fmt.Printf("  %s\n", info)
			claimed++
		}
	}
	if claimed == 0 {
		fmt.Println("  (none)")
	}
	return nil
}

func unexportSysfs(only string, pump, roof uint8) error {
	var pins []struct {
		name string
		n    uint8
	}
	switch only {
	case "pump":
		pins = []struct {
			name string
			n    uint8
		}{{"pump", pump}}
	case "roof":
		pins = []struct {
			name string
			n    uint8
		}{{"roof", roof}}
	case "both":
		pins = []struct {
			name string
			n    uint8
		}{{"pump", pump}, {"roof", roof}}
	default:
		return fmt.Errorf("-only must be pump, roof, or both")
	}
	for _, p := range pins {
		if err := gpiocdev.UnexportSysfs(uint32(p.n)); err != nil {
			return fmt.Errorf("%s GPIO %d: %w", p.name, p.n, err)
		}
		fmt.Printf("unexported %s GPIO %d from sysfs\n", p.name, p.n)
	}
	return nil
}

func main() {
	seconds := flag.Int("seconds", 30, "how long to sample")
	count := flag.Int("n", 0, "if >0, take this many samples per channel instead of using -seconds")
	pumpGPIO := flag.Uint("pump", defaultPumpGPIO, "BCM GPIO for pump/water thermistor")
	roofGPIO := flag.Uint("roof", defaultRoofGPIO, "BCM GPIO for roof thermistor")
	only := flag.String("only", "both", "pump, roof, or both")
	adjust := flag.Float64("adjust", defaultAdjust, "timing adjust from deployed config (real.conf is 1.75)")
	list := flag.Bool("list", false, "report which GPIO lines are claimed and by whom, then exit")
	unexport := flag.Bool("unexport-sysfs", false,
		"release leftover /sys/class/gpio exports for the selected thermistor pins, then exit. "+
			"Does not touch relay/LED pins.")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "therm-capture: 100 nF RC timing dump. Stop pool-controller first.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	fmt.Fprintf(os.Stderr, "therm-capture cap=%d nF window=%s..%s go=%s %s/%s\n",
		rctime.NanoFarads, rctime.MinTime, rctime.MaxTime,
		runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(os.Stderr, "Stop pool-controller before this tool; both would drive the same GPIO.\n")

	if *unexport {
		if err := unexportSysfs(*only, uint8(*pumpGPIO), uint8(*roofGPIO)); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	if *list {
		if err := listLines(uint8(*pumpGPIO), uint8(*roofGPIO)); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return
	}

	chip, err := gpiocdev.Default()
	if err != nil {
		fmt.Fprintf(os.Stderr, "GPIO character device init failed: %v\n", err)
		os.Exit(1)
	}
	defer chip.Close()
	fmt.Fprintf(os.Stderr, "using %s\n", chip)
	open := func(n uint8) (prober, error) { return openCdev(chip, n) }

	type ch struct {
		name string
		gpio uint8
		pin  prober
	}
	var chans []ch
	add := func(name string, n uint) {
		p, err := open(uint8(n))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			os.Exit(1)
		}
		chans = append(chans, ch{name: name, gpio: uint8(n), pin: p})
	}
	switch *only {
	case "pump":
		add("pump", *pumpGPIO)
	case "roof":
		add("roof", *roofGPIO)
	case "both":
		add("pump", *pumpGPIO)
		add("roof", *roofGPIO)
	default:
		fmt.Fprintf(os.Stderr, "-only must be pump, roof, or both\n")
		os.Exit(1)
	}
	defer func() {
		for _, c := range chans {
			c.pin.close()
		}
	}()

	enc := json.NewEncoder(os.Stdout)
	deadline := time.Now().Add(time.Duration(*seconds) * time.Second)
	perCh := map[string][]sample{}
	taken := 0
	for {
		if *count > 0 && taken >= *count {
			break
		}
		if *count == 0 && time.Now().After(deadline) {
			break
		}
		for _, c := range chans {
			s := takeSample(c.name, c.gpio, "cdev", c.pin, *adjust)
			perCh[c.name] = append(perCh[c.name], s)
			if err := enc.Encode(s); err != nil {
				fmt.Fprintf(os.Stderr, "write json: %v\n", err)
				os.Exit(1)
			}
		}
		taken++
	}
	for _, c := range chans {
		summarize(c.name, perCh[c.name])
	}
}
