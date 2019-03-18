package weather

import (
	"fmt"
	"sync"
	"time"
)

// Service is a simple interface for making a call to a weather service.
type Service interface {
	Read(string) (*Data, error)
}

// Data represents basic information about weather in a specific area
type Data struct {
	Zipcode        string
	Updated        time.Time
	CurrentTempC   float64
	SolarRadiation float64
	Description    string
}

// Weather is a provider of basic weather information
type Weather struct {
	service Service
	ttl     time.Duration
	backoff time.Time
	cache   map[string]*Data
	mtx     sync.Mutex
}

// NewWeatherFromService is used for testing
func NewWeatherFromService(service Service) *Weather {
	return &Weather{
		service: service,
		ttl:     time.Hour,
		backoff: time.Now().Add(-1 * time.Hour),
		cache:   make(map[string]*Data),
	}
}

// NewWeather provides a weather underground service.
func NewWeather(appID string, ttl time.Duration) *Weather {
	service := &WUService{appID: appID}
	w := Weather{
		service: service,
		ttl:     ttl,
		backoff: time.Now().Add(-1 * time.Hour),
		cache:   make(map[string]*Data),
	}
	return &w
}

// GetWeatherByZip makes a call to the service and updates the weather if the cache has expired
func (w *Weather) GetWeatherByZip(zipcode string) (*Data, error) {
	if zipcode == "" {
		return &Data{}, fmt.Errorf("Cannot return weather for empty zipcode")
	}
	data, present := w.cache[zipcode]
	if data != nil && present && time.Now().Before(data.Updated.Add(w.ttl)) {
		return data, nil
	}
	// Don't keep sending requests when they are not going through
	if w.backoff.Add(5 * time.Minute).Before(time.Now()) {
		var err error
		w.backoff = time.Now()
		data, err = w.service.Read(zipcode)
		if err != nil {
			return &Data{}, err
		}
		w.mtx.Lock()
		defer w.mtx.Unlock()
		w.cache[zipcode] = data
	}
	return data, nil
}
