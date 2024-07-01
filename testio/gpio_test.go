package test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	rpio "github.com/stianeikeland/go-rpio/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type samples struct {
	Edge       rpio.Edge
	Pull       rpio.Pull
	InitState  rpio.State
	Values     []*Value
	Detections []time.Time
}

type Value struct {
	Time  time.Time
	State rpio.State
}

func TestValues(t *testing.T) {
	err := rpio.Open()
	require.NoError(t, err)
	defer rpio.Close()
	pin := rpio.Pin(14)
	out := []*samples{}
	for p := rpio.PullOff; p < rpio.PullNone; p++ {
		for e := rpio.NoEdge; e < rpio.AnyEdge; e++ {
			for s := rpio.Low; s <= rpio.High; s++ {
				edgeEvents := 0
				readEvents := 0
				for poll := 0; poll < 2; poll++ {
					DoPoll := poll == 1
					pollStr := "Poll"
					if !DoPoll {
						pollStr = "Detect"
					}
					t.Run(fmt.Sprintf("Pull(%d) Edge(%d) State(%d) Poll(%s)", p, e, s, pollStr), func(t *testing.T) {
						sample := &samples{
							Edge:      e,
							Pull:      p,
							InitState: s,
						}
						out = append(out, sample)
						end := time.Now().Add(time.Second)
						pin.Detect(rpio.NoEdge) // Reset Detect
						pin.Output()
						pin.Write(s)
						assert.Equal(t, s, pin.Read(), "Failed to set initial state")
						pin.Input()
						pin.Pull(p)
						assert.Equal(t, p, pin.ReadPull(), "Failed to set pull")
						if !DoPoll {
							pin.Detect(e)
							detected := false
							for time.Now().Before(end) || edgeEvents > 100 {
								detected = pin.EdgeDetected()
								if detected {
									edgeEvents++
									pin.Detect(e)
									sample.Detections = append(sample.Detections, time.Now())
								}
								time.Sleep(time.Microsecond)
							}
							assert.Greater(t, edgeEvents, 1000, "Not enough edge events detected")
						} else {
							last := rpio.Low
							for i := 0; time.Now().Before(end) || readEvents > 100; i++ {
								stat := pin.Read()
								if i == 0 || stat != last {
									readEvents++
									sample.Values = append(sample.Values, &Value{
										Time:  time.Now(),
										State: stat,
									})
									time.Sleep(time.Microsecond)
								}
							}
							assert.Greater(t, readEvents, 1000, "Not enough read events detected")
						}
					})
				}
			}
		}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	require.NoError(t, err)
	os.WriteFile("testValues.txt", []byte(data), 0644)
}
