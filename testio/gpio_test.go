package test

import (
	"time"

	rpio "github.com/stianeikeland/go-rpio/v4"
)

type samples struct {
	Edge      rpio.Edge
	Pull      rpio.Pull
	InitState rpio.State
	Values    []struct {
		Time  time.Time
		State rpio.State
	}
	Detections []time.Time
}

func TestValues() {
	pin := rpio.Pin(14)
	out := []samples{}
	for p := rpio.PullOff; p <= rpio.PullNone; p++ {
		for e := rpio.NoEdge; e <= rpio.AnyEdge; e++ {
			for s := rpio.Low; s <= rpio.High; s++ {
				sample := samples{
					Edge:      e,
					Pull:      p,
					InitState: s,
				}
				out = append(out, sample)
				end := time.Now().Add(time.Second)
				pin.Output()
				pin.Pull(p)
				pin.Input()
				pin.Detect(e)
				detected := false
				for time.Now().Before(end) {
					detected = pin.EdgeDetected()
					if detected {
						pin.Detect(e)
						sample.Detections = append(sample.Detections, time.Now())
					}
					time.Sleep(time.Microsecond)
				}
				pin.Detect(rpio.NoEdge) // Reset
				end  = time.Now().Add(time.Second)
				last := rpio.Low
				for i := 0; time.Now().Before(end); i++ {
					stat := pin.Read()
					if i == 0  || stat != last {
					sample.Values = append(sample.Values, struct {
						Time  time.Time
						State rpio.State
					}{
						Time:  time.Now(),
						State: stat,
					})
					time.Sleep(time.Microsecond)
				}
			}
		}
	}
	data := fmt.Sprintf("%+v", out)
	ioutil.WriteFile("testValues.txt", []byte(data), 0644)
}
