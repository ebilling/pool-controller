package main

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"os"
)

type JSONmap struct {
	_data map[string]interface{}
}

func NewJSONmap() (JSONmap) {
	return JSONmap {
		_data: make(map[string]interface{}),
	}
}

func (m *JSONmap) readBytes(data []byte) (error) {
	return json.Unmarshal(data, &m._data)
}

func (m *JSONmap) readString(data string) (error) {
	Info("Converting string to JSONmap: %s", data)
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
	Debug("Fetching %s from JSONmap", fullname)
	if jm == nil || jm._data == nil {
		return nil, errors.New("data for JSONmap is (nil)")
	}
	m = jm._data
	nameSlice := strings.Split(fullname, ".")
	for _, name := range nameSlice {
		Debug("Looking for [%s]", name)
		_, isMap := m.(map[string]interface{})
		if !isMap {
			return nil, errors.New("No data for element: " + name + " of " + fullname)
		}
		value, present := m.(map[string]interface{})[name]
		Debug("Did we find %s: %t", name, present)
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

func (m *JSONmap) GetFloat(fullname string) (float64) {
	x, err := m.get(fullname)
	if err != nil {
		Error("Problem fetching %s, %s", fullname, err.Error())
		return 0.0
	}
	kind := reflect.TypeOf(x).Kind()
	switch kind {
	case reflect.Float64:
		return x.(float64)
	case reflect.Float32:
		return float64(x.(float32))
	case reflect.String:
		val, err := strconv.ParseFloat(x.(string), 64)
		if err == nil {
			return val
		}
	default:
		Error("Could not parse value for %s, (%v) (%v)", fullname, x, kind)
	}
	return 0.0
}
