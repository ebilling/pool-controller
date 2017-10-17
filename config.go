package main

import (
	"time"
	"os"
)

type Config struct {
	path  string
	data  JSONmap
	mtime time.Time
}

func NewConfig(path string) *Config {
	c := Config{
		path: path,
		data: NewJSONmap(),		
	}
	c.Update()
	return &c
}

func (c *Config) Update() {
	fi, err := os.Stat(c.path)
	if err != nil {
		Error("Error stat'ing config file %s: %s", c.path, err.Error())
		return
	}
	if fi.ModTime().After(c.mtime) {
		err := c.data.readFile(c.path)
		if err != nil {
			Error("Error reading config file(%s): %s",
				c.path, err.Error())
		}
		c.mtime = fi.ModTime()
	}
}

func (c *Config) Contains(fullname string) bool {
	return c.data.Contains(fullname)
}

func (c *Config) Get(fullname string) (interface{}) {
	return c.data.Get(fullname)
}

func (c *Config) GetString(fullname string) (string) {
	return c.data.Get(fullname).(string)
}

func (c *Config) GetFloat(fullname string) (float64) {
	return c.data.GetFloat(fullname)
}
