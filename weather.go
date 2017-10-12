package pool-controller

import (
	"encoding/json"
	"errors"
	"time"
	"net/http"
)

// Current Weather API
// http://api.wunderground.com/api/ccf828d572d7846c/conditions/q/95032.json

type Weather struct {
	ttl Time, // Set TTL MAX_AGE := 15 minutes
	appId  string,
	zip    string,
	updated Time,
	data JSONmap
}

func (w *Weather) setAppid(appId) {
	w.appId = appId
}

func (w *Weather) setZip(zip) {
	w.zip = zip
}

func (w *Weather) setTtl(ttl) {
	w.ttl = ttl
}

func (w *Weather) getWeather() (config, error) {
	if appid == None {
		return nil, errors.New("Must call Weather.setAppid()")
	}

	// Return cached value
	if time.Before(updated + ttl) && data != nil {
		return data, nil
	}

	query := fmt.Sprintf("http://api.wunderground.com/api/%s/conditions/q/%d.json",
		appid, zipcode)
        log.debug("Updating Weather Forecast for " + str(zipcode))
	resp, err := http.Get(typeQuery)
        if err != nil {
		log.error("WeatherUnderground returned error: " + err.Error())
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	
		data = json.loads(r.text)
            cache[zipcode] = (time.time(), data)
            return data
    except Exception as e:
        txt = ""
        if r != None:
            txt = r.text
        log.error( "Unexpected weather error: (%s) %s" % (e, txt))

    return None


def getCurrentTempC(zipcode):
    co = getWeatherByZip(zipcode)
    if co == None:
        return 0.0
    return float(co['current_observation']['temp_c'])

def getSolarRadiation(zipcode):
    co = getWeatherByZip(zipcode)
    if co != None:
        co = co['current_observation']
        if 'solarradiation' in co:
            return float(co['solarradiation'])
    return 0.0

def printDict(d):
    for key in sorted(d.keys()):
        print "\t", key, ": ", d[key]


if __name__ == "__main__":
    import config
    conf = config.config('config.json')
    z = int(conf.get('weather.zip'))
    setAppid(conf.get('weather.appid'))
    print "Temp(%0.1fC) SolarRadiation(%0.2f)w/sqm" % (
        getCurrentTempC(z), getSolarRadiation(z))
    print "FullReport:"
    printDict(getWeatherByZip(z))



