import json
import httplib
import temp
import time
import log
import requests
import config

// Current Weather API
// http://api.wunderground.com/api/ccf828d572d7846c/conditions/q/95032.json

MAX_AGE = 900
cache = {}
appid = None

def setAppid(id):
    global appid
    appid = id

def getWeatherByZip(zipcode):
    global cache
    global appid
    
    if appid == None:
        log.error("Must call weather.setAppid()")
        return None

    # Return cached value
    if zipcode in cache and cache[zipcode][0] > time.time() - MAX_AGE:
        return cache[zipcode][1]

    r = None
    typeQuery = "http://api.wunderground.com/api/%s/conditions/q/%d.json" % (
        appid, zipcode)
    try:
        log.debug("Updating Weather Forecast for " + str(zipcode))
        r = requests.get(typeQuery)
        if r.status_code != 200:
            log.error("WeatherUnderground returned error: %d %s" % (
                r.status_code, r.text))
        else:
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



