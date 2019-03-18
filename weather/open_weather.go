package weather

import (
	"bytes"
	"encoding/json"
	"errors"
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

// SolarRadiation returns a bad approximation to solar radiation based on the cloud cover (does not know if the sun is up)
func (r *OWResponse) SolarRadiation() float64 {
	if r.Clouds != nil {
		return 500.0 * (100.0 - r.Clouds.Percentage) / 100.0
	}
	return 0.0
}

// TempC returns the temperature if it is available
func (r *OWResponse) TempC() float64 {
	if r.Conditions != nil {
		return r.Conditions.TemperatureC
	}
	return 0.0
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
	if co == nil {
		return &Data{}, errors.New("invalid response")
	}

	return &Data{
		Zipcode:        zipcode,
		Updated:        time.Unix(co.ObservationEpoch, 0),
		CurrentTempC:   co.TempC(),
		SolarRadiation: co.SolarRadiation(),
		Description:    "OpenWeatherMap current observation",
	}, nil
}
