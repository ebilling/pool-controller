package weather

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenWeatherService is a Open Weather Map Service API
type OpenWeatherService struct {
	appID string
}

// OWResponse is the outermost wrapper of the Open Weather API response
type OWResponse struct {
	ObservationEpoch int64         `json:"dt"`
	Conditions       *OWConditions `json:"main"`
	Clouds           *OWCloudiness `json:"clouds"`
}

//OWCloudiness is the cloudiness
type OWCloudiness struct {
	Percentage float64 `json:"all"`
}

// OWConditions is the current weather status.
type OWConditions struct {
	TemperatureC float64 `json:"temp"`
}

// Read sends a request to the API and converts the data expects "95125,us" as an input
func (w *OpenWeatherService) Read(zipCommaCountry string) (*Data, error) {
	if w.appID == "" {
		return nil, fmt.Errorf("Cannot make request to Weather Underground without an AppID")
	}
	// Return cached value
	url := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?zip=%s&appid=%s&units=metric", zipCommaCountry, w.appID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := bytes.NewBuffer(nil)
	n, cerr := io.Copy(buf, resp.Body)
	fmt.Printf("Copied %d bytes (%v): %s", n, cerr, buf.String())
	response := &OWResponse{}
	err = json.Unmarshal(buf.Bytes(), &response)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Result: %+v", response)
	return w.convert(zipCommaCountry, response)
}

func (w *OpenWeatherService) convert(zipcode string, co *OWResponse) (*Data, error) {
	solarradiation := 500.0 * (100.0 - co.Clouds.Percentage) / 100.0
	return &Data{
		Zipcode:        zipcode,
		Updated:        time.Unix(co.ObservationEpoch, 0),
		CurrentTempC:   co.Conditions.TemperatureC,
		SolarRadiation: solarradiation,
		Description:    "OpenWeatherMap current observation",
	}, nil
}
