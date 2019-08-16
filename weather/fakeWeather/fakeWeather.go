package fakeweather

import (
	"time"

	"github.com/ebilling/pool-controller/weather"
)

// TestService is used to fake a response from weather underground
type TestService struct{}

func (t *TestService) Read(ignoredURL string) (*weather.Data, error) {
	return &weather.Data{
		Zipcode:        "95014",
		Updated:        time.Now(),
		CurrentTempC:   20.0,
		SolarRadiation: 250.0,
		Description:    "bogus description",
	}, nil
}
