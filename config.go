package main

import (
	"sync"
	"time"
	"os"
)

type Config struct {
	lock  sync.Mutex
	path  string
	data  JSONmap
	mtime time.Time
}

func NewConfig(path string) *Config {
	c := Config{
		lock: sync.Mutex{},
		path: path,
		data: NewJSONmap(),
	}
	c.Update()
	return &c
}

func (c *Config) Update() {
	c.lock.Lock()
	defer c.lock.Unlock()
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

func (c *Config) Write() {
	c.lock.Lock()
	defer c.lock.Unlock()
	check(c.data.Write(c.path, 0644), "Could not write config to %s", c.path)
	c.mtime = time.Now()
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
