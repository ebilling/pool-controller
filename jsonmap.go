package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"os"	
)

type JSONmap struct {
	_data map[string]interface{}
}

func NewJSONmap() JSONmap {
	return JSONmap{
		_data: make(map[string]interface{}),
	}
}

func (m *JSONmap) readBytes(data []byte) (error) {
	return json.Unmarshal(data, &m._data)
}

func (m *JSONmap) readString(data string) (error) {
	return json.Unmarshal([]byte(data), &m._data)
}

func (m *JSONmap) readFile(path string) (error) {
	file, err := os.Open(path)
	if err != nil {
		Error("Config Open Error: %s", err.Error())
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&m._data)
	if err != nil {
		Error("Could not decode JSON file: %s", path)
	}
	return err
}

func (jm *JSONmap) get(fullname string) (interface{}, error) {
	var m interface{}
	m = jm._data
	nameSlice := strings.Split(fullname, ".")
	for _, name := range nameSlice {
		_, isMap := m.(map[string]interface{})
		if !isMap {
			return nil, errors.New("No data for element: " + name + " of " + fullname)
		}
		value, present := m.(map[string]interface{})[name]
		if !present {
			return nil, errors.New("No data present for " + fullname)
		}
		m, isMap = value.(map[string]interface{})
		if !isMap {
			m = value
		}
	}
	return m, nil
}

func (m *JSONmap) Get(fullname string) (interface{}) {
	val, err := m.get(fullname)
	if err != nil {
		Error("Problem fetching %s, %s", fullname, err.Error())
	}
	return val
}

func (m *JSONmap) Contains(fullname string) bool {
	_, ret := m.get(fullname)
	return ret == nil
}

func TestJSONmapQuick(t *testing.T) {
	swp_str := "timer.sweep.start"
	swp_val := "1:00"
	cap_str := "capacitance.gpio.25"
	cap_val := 9.37
	str := "{ \"debug\": \"False\", \"timer\": { \"sweep\": \"start\": \"" + swp_str +
		"\",\"stop\": \"2:30\" } }, \"capacitance\": { \"gpio\": { \"24\": \"10.4\"," +
		"\"25\": \"" + fmt.Sprintf("%f", cap_val) + "\" } } }"
	t.Name()
	m := NewJSONmap()
	t.Run("ReadString", func(t *testing.T) {m.readString(str)})

	t.Run("Get.String", func(t *testing.T) {
		if m.Get(swp_str) != swp_val {
			t.Errorf("Could not fetch appropriate value for %s", swp_str)
			t.Failed()}})
	t.Run("Get.Float", func(t *testing.T) {
		if m.Get(cap_str) != cap_val {
			t.Errorf("Could not fetch appropriate value for %s", cap_str)
			t.Failed()}})
}

