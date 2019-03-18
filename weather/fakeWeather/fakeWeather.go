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

const canned = `{
	"response":{
		"version":"0.1",
		"termsofService":"http://www.wunderground.com/weather/api/d/terms.html",
		"features":{
			"conditions":1
		}
	},
	"current_observation":{
		"image":{
			"url":"http://icons.wxug.com/graphics/wu2/logo_130x80.png",
			"title":"Weather Underground",
			"link":"http://www.wunderground.com"
		},
		"display_location":{
			"full":"Los Gatos, CA",
			"city":"Los Gatos",
			"state":"CA",
			"state_name":"California",
			"country":"US",
			"country_iso3166":"US",
			"zip":"95032",
			"magic":"1",
			"wmo":"99999",
			"latitude":"37.22999954",
			"longitude":"-121.94999695",
			"elevation":"113.1"
		},
		"observation_location":{
			"full":"Los Gatos, Los Gatos, California",
			"city":"Los Gatos, Los Gatos",
			"state":"California",
			"country":"US",
			"country_iso3166":"US",
			"latitude":"37.229641",
			"longitude":"-121.952286",
			"elevation":"380 ft"
		},
		"estimated":{

		},
		"station_id":"KCALOSGA217",
		"observation_time":"Last Updated on October 15, 11:54 AM PDT",
		"observation_time_rfc822":"Sun, 15 Oct 2017 11:54:37 -0700",
		"observation_epoch":"1508093677",
		"local_time_rfc822":"Sun, 15 Oct 2017 11:54:43 -0700",
		"local_epoch":"1508093683",
		"local_tz_short":"PDT",
		"local_tz_long":"America/Los_Angeles",
		"local_tz_offset":"-0700",
		"weather":"Clear",
		"temperature_string":"74.7 F (23.7 C)",
		"temp_f":74.7,
		"temp_c":23.7,
		"relative_humidity":"16%",
		"wind_string":"From the WNW at 2.0 MPH Gusting to 2.5 MPH",
		"wind_dir":"WNW",
		"wind_degrees":291,
		"wind_mph":2.0,
		"wind_gust_mph":"2.5",
		"wind_kph":3.2,
		"wind_gust_kph":"4.0",
		"pressure_mb":"1021",
		"pressure_in":"30.14",
		"pressure_trend":"0",
		"dewpoint_string":"26 F (-4 C)",
		"dewpoint_f":26,
		"dewpoint_c":-4,
		"heat_index_string":"NA",
		"heat_index_f":"NA",
		"heat_index_c":"NA",
		"windchill_string":"NA",
		"windchill_f":"NA",
		"windchill_c":"NA",
		"feelslike_string":"74.7 F (23.7 C)",
		"feelslike_f":"74.7",
		"feelslike_c":"23.7",
		"visibility_mi":"10.0",
		"visibility_km":"16.1",
		"solarradiation":"264",
		"UV":"3.0",
		"precip_1hr_string":"0.00 in ( 0 mm)",
		"precip_1hr_in":"0.00",
		"precip_1hr_metric":" 0",
		"precip_today_string":"0.00 in (0 mm)",
		"precip_today_in":"0.00",
		"precip_today_metric":"0",
		"icon":"clear",
		"icon_url":"http://icons.wxug.com/i/c/k/clear.gif",
		"forecast_url":"http://www.wunderground.com/US/CA/Los_Gatos.html",
		"history_url":"http://www.wunderground.com/weatherstation/WXDailyHistory.asp?ID=KCALOSGA217",
		"ob_url":"http://www.wunderground.com/cgi-bin/findweather/getForecast?query=37.229641,-121.952286",
		"nowcast":""
	}
}`
