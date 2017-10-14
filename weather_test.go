package main

import (
	"testing"
	"reflect"
)


func TestWeather(t *testing.T) {

}

func TestGetWeatherByZip(t *testing.T) {
	
}

func testGetCurrentTempC(t *testing.T, w *Weather) {
	val := w.GetCurrentTempC("95032")
	if reflect.TypeOf(val).Kind() != reflect.Float64 {
		t.Fatal("should have returned a float")
	}
}

func testGetSolarRadiation(t *testing.T, w *Weather) {
	val := w.GetSolarRadiation("95032")
	if reflect.TypeOf(val).Kind() != reflect.Float64 {
		t.Fatal("should have returned a float")
	}
}

