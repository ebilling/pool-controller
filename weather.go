package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// Service is a simple interface for making a call to a weather service.
type Service interface {
	Read(string) string
}

type WeatherData struct {
	zipcode string
	updated time.Time
	data    JSONmap
	service Service
}

type Weather struct {
	service Service
	ttl     time.Duration
	backoff time.Time
	cache   map[string]*WeatherData
}

func NewWeather(appId string, ttl time.Duration) *Weather {
	service := WUService{appId: appId}
	w := Weather{
		service: &service,
		ttl:     ttl,
		backoff: time.Now().Add(-1 * time.Hour),
		cache:   make(map[string]*WeatherData),
	}
	return &w
}

// Weather Underground Service API
type WUService struct {
	appId string
}

func (w *WUService) Read(zip string) string {
	if __test__ || w.appId == "" {
		return "" // Don't hit WU for a test, or if unconfigured
	}
	// Return cached value
	url := fmt.Sprintf("http://api.wunderground.com/api/%s/conditions/q/%s.json",
		w.appId, zip)
	Debug("Sending request to WeatherUnderground: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		Error("WeatherUnderground returned error: %s",
			err.Error())
		return ""
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	str := string(body[:])
	Debug("Weather Underground returned: %s", str)
	return str
}

func newWeatherData(zip string, service Service) *WeatherData {
	data := WeatherData{
		zipcode: zip,
		updated: time.Now().Add(-24 * time.Hour),
		data:    NewJSONmap(),
		service: service,
	}
	return &data
}

func (w *Weather) GetWeatherByZip(zipcode string) *JSONmap {
	if zipcode == "" {
		return nil
	}
	data, present := w.cache[zipcode]
	Debug("GetWeatherByZip cached(%t)", present)
	if present && time.Now().Before(data.updated.Add(w.ttl)) {
		Debug("Returning cached data: %v", data.data)
		return &data.data
	}
	if !present {
		data = newWeatherData(zipcode, w.service)
		w.cache[zipcode] = data
	}
	// Don't keep sending requests when they are not going through
	if w.backoff.Add(5 * time.Minute).Before(time.Now()) {
		Debug("GetWeatherByZip - Getting new data")
		err := data.Update()
		w.backoff = time.Now()
		if err != nil {
			Error("Failed to get data for %s: %s", zipcode, err.Error())
			return nil
		}
	}
	return &data.data
}

func (w *WeatherData) Update() error {
	Info("Updating Weather Forecast for %s", w.zipcode)
	response := w.service.Read(w.zipcode)
	if response == "" {
		return fmt.Errorf("Error getting data from weather service")
	}
	err := w.data.readString(response)
	if err != nil {
		return fmt.Errorf("Issue reading data from weather service: %s Response(%s)",
			err.Error(), response)
	}
	w.updated = time.Now()
	Debug("WeatherData Updated: %v", w)
	return nil
}

func (w *Weather) getFloat(zipcode string, name string) float64 {
	co := w.GetWeatherByZip(zipcode)
	if co == nil {
		Error("Could not retrieve weather data for %s", zipcode)
		return 0.0
	}
	return co.GetFloat(name)
}

func (w *Weather) GetCurrentTempC(zipcode string) float64 {
	if zipcode == "" {
		return 0.0
	}
	return w.getFloat(zipcode, "current_observation.temp_c")
}

func (w *Weather) GetSolarRadiation(zipcode string) float64 {
	if zipcode == "" {
		return 0.0
	}
	return w.getFloat(zipcode, "current_observation.solarradiation")
}
