package pool-controller

import (
	"encoding/json"
	"strings"
	"os"
)

type Config struct {
	path string
	config map[string]interface{}
}

func NewConfig(path string) *Config {
	c := Config{
		path: path,
		config: make(map[string]interface{})}
	file, err := os.Open(path)
	if err != nil {
		log.error("Config Open Error: " + err.Error())
		return nil
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&c.config)
	if err != nil {
		log.error("Config Decode Error: " + err.Error())
	}
	return &c
}

func (c *Config) Get(fullname string) (string, bool) {
	var m interface{}
	m = c.config
	nameSlice := strings.Split(fullname, ".")
	for _, name := range nameSlice {
		_, isMap := m.(map[string]interface{})
		if !isMap {
			return "", false
		}
		value, present := m.(map[string]interface{})[name]
		if !present {
			return "", false
		}
		m, isMap = value.(map[string]interface{})
		if !isMap {
			m = value.(string)
		}
	}
	return m.(string), true
}

func (c *Config) Contains(fullname string) bool {
	_, ret := Get(fullname)
	return ret
}
