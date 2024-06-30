package test

import (
	"fmt"
	"os"
	"testing"
	"time"

	rpio "github.com/stianeikeland/go-rpio/v4"
	"github.com/stretchr/testify/require"
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

func TestValues(t *testing.T) {
	err := rpio.Open()
	require.NoError(t, err)
	defer rpio.Close()
	pin := rpio.Pin(14)
	out := []samples{}
	for p := rpio.PullOff; p <= rpio.PullNone; p++ {
		for e := rpio.NoEdge; e <= rpio.AnyEdge; e++ {
			edgeEvents := 0
			readEvents := 0
			for s := rpio.Low; s <= rpio.High; s++ {
				t.Run(fmt.Sprintf("Edge(%d) Pull(%d) InitState(%d)", e, p, s), func(t *testing.T) {
					sample := samples{
						Edge:      e,
						Pull:      p,
						InitState: s,
					}
					out = append(out, sample)
					end := time.Now().Add(time.Second)
					pin.Output()
					pin.Write(s)
					pin.Input()
					pin.Pull(p)
					pin.Detect(e)
					detected := false
					for time.Now().Before(end) {
						detected = pin.EdgeDetected()
						if detected {
							edgeEvents++
							pin.Detect(e)
							sample.Detections = append(sample.Detections, time.Now())
						}
						time.Sleep(time.Microsecond)
					}
					pin.Detect(rpio.NoEdge) // Reset
					end = time.Now().Add(time.Second)
					last := rpio.Low
					for i := 0; time.Now().Before(end); i++ {
						stat := pin.Read()
						if i == 0 || stat != last {
							readEvents++
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
				})
			}
		}
	}
	data := fmt.Sprintf("%+v", out)
	os.WriteFile("testValues.txt", []byte(data), 0644)
}
