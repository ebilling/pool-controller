package main

import (
	"io/ioutil"
	"fmt"
	"time"
	"net/http"
)

// Current Weather API
// http://api.wunderground.com/api/ccf828d572d7846c/conditions/q/95032.json
type WeatherData struct {
	zipcode     string
	updated time.Time
	data    JSONmap
}

type Weather struct {
	appId   string
	ttl     time.Duration
	cache   map[string]*WeatherData
}

func NewWeather(ttl time.Duration, appId string) (*Weather){
	w := Weather{
		ttl: ttl,
		appId: appId,
		cache: make(map[string]*WeatherData),
	}
	return &w
}

func newWeatherData(zip string) (*WeatherData){
	data := WeatherData {
		zipcode: zip,
		data:    NewJSONmap(),
	}
	return &data
}

func (w *Weather) SetAppid(appId string) {
	w.appId = appId
}

func (w *Weather) SetTtl(ttl time.Duration) {
	w.ttl = ttl
}

func (w *Weather) GetWeatherByZip(zipcode string) (*JSONmap) {
	data, present := w.cache[zipcode]
	if present {
		return data.GetWeather(w.appId, w.ttl)
	}
	data = newWeatherData(zipcode)
	return data.GetWeather(w.appId, w.ttl)
}

func (w *WeatherData) GetWeather(appId string, ttl time.Duration) (*JSONmap) {
	// Return cached value
	if time.Now().Before(w.updated.Add(ttl)) {
		return &w.data
	}

	query := fmt.Sprintf("http://api.wunderground.com/api/%s/conditions/q/%d.json",
		appId, w.zipcode)
        Debug("Updating Weather Forecast for %s", w.zipcode)
	resp, err := http.Get(query)
        if err != nil {
		Error("WeatherUnderground returned error: %s", err.Error())
		return nil
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	
	err = w.data.readBytes(body)
	if err != nil {
		Error("Issue reading data from WeatherUnderground: %s", err.Error())
		return nil
	}
	return &w.data
}

func (w *Weather) GetCurrentTempC(zipcode string) (float64) {
	co := w.GetWeatherByZip(zipcode)
	if co == nil {
		return 0.0
	}
	return co.Get("current_observation.temp_c").(float64)
}

func (w *Weather) GetSolarRadiation(zipcode string) (float64) {
	co := w.GetWeatherByZip(zipcode)
	if co == nil {
		return 0.0
	}
        return co.Get("current_observation.solarradiation").(float64)
}
