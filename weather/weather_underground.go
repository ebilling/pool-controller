package weather

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// WUService is a Weather Underground Service API
type WUService struct {
	appID string
}

// APIResponse is the outermost wrapper of the WU api response
type APIResponse struct {
	CurrentObservation CurrentObservation `json:"current_observation"`
}

// CurrentObservation is the current weather status.
type CurrentObservation struct {
	StationID        string  `json:"station_id"`
	ObservationEpoch string  `json:"observation_epoch"`
	Description      string  `json:"weather"`
	SolarRadiation   string  `json:"solarradiation"`
	TemperatureC     float32 `json:"temp_c"`
}

func (w *WUService) Read(zip string) (*Data, error) {
	if w.appID == "" {
		return nil, fmt.Errorf("Cannot make request to Weather Underground without an AppID")
	}
	// Return cached value
	url := fmt.Sprintf("http://api.wunderground.com/api/%s/conditions/q/%s.json", w.appID, zip)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	response := &APIResponse{}
	err = json.NewDecoder(resp.Body).Decode(response)
	if err != nil {
		return nil, err
	}

	return w.convert(zip, &response.CurrentObservation)
}

func (w *WUService) convert(zipcode string, co *CurrentObservation) (*Data, error) {
	epoch, _ := strconv.ParseInt(co.ObservationEpoch, 10, 64)
	solarradiation, _ := strconv.ParseFloat(co.SolarRadiation, 32)
	return &Data{
		Zipcode:        zipcode,
		Updated:        time.Unix(epoch, 0),
		CurrentTempC:   co.TemperatureC,
		SolarRadiation: float32(solarradiation),
		Description:    co.Description,
	}, nil
}