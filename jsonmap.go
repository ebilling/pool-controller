package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	Debug("Converting string to JSONmap: %s", data)
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
		return nil, fmt.Errorf("data for JSONmap is (nil)")
	}
	m = jm._data
	nameSlice := strings.Split(fullname, ".")
	for _, name := range nameSlice {
		Debug("Looking for [%s]", name)
		_, isMap := m.(map[string]interface{})
		if !isMap {
			return nil, fmt.Errorf("No data for element: %s of %s", name, fullname)
		}
		value, present := m.(map[string]interface{})[name]
		Debug("Did we find %s: %t", name, present)
		if !present {
			return nil, fmt.Errorf("No data present for %s", fullname)
		}
		m, isMap = value.(map[string]interface{})
		if !isMap {
			m = value
		}
	}
	return m, nil
}

func (jm *JSONmap) set(fullname, value string) error {
	var m interface{}
	Debug("Setting %s in JSONmap to %s", fullname, value)
	if jm == nil || jm._data == nil {
		return fmt.Errorf("data for JSONmap is (nil)")
	}
	m = jm._data
	nameSlice := strings.Split(fullname, ".")
	for _, name := range nameSlice {
		Debug("Looking for [%s]", name)
		_, isMap := m.(map[string]interface{})
		if !isMap {
			return fmt.Errorf("set: No data for element: %s of %s", name, fullname)
		}
		value, present := m.(map[string]interface{})[name]
		Debug("Did we find %s: %t", name, present)
		if !present {
			return fmt.Errorf("No data present for %s", fullname)
		}
		m, isMap = value.(map[string]interface{})
		if !isMap {
			// Found it *****
			m.(map[string]interface{})[name] = fullname
		}
	}
	return nil
}

func (m *JSONmap) Write(path string, permissions uint32) error {
	data, err := json.MarshalIndent(m._data, "", "  ")
	if err != nil {
		check(err, "Could not write JSONdata to %s", path)
		return err
	}
	return ioutil.WriteFile(path, data, os.FileMode(permissions))
}

func (m *JSONmap) Get(fullname string) (interface{}) {
	val, err := m.get(fullname)
	if err != nil {
		Debug("Problem fetching %s, %s", fullname, err.Error())
	}
	return val
}

func (m *JSONmap) Set(fullname, value string) error {
	_, err := m.get(fullname)
	if err != nil {
		Debug("Problem fetching %s, %s", fullname, err.Error())
	}
	return m.set(fullname, value)
}

func (m *JSONmap) Contains(fullname string) bool {
	_, ret := m.get(fullname)
	return ret == nil
}

func (m *JSONmap) GetFloat(fullname string) (float64) {
	x, err := m.get(fullname)
	if err != nil {
		Debug("Problem fetching %s, %s", fullname, err.Error())
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
