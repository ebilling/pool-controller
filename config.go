package main

type Config struct {
	path string
	data JSONmap
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
	c.data.readFile(c.path)
}

func (c *Config) Get(fullname string) (interface{}) {
	return c.data.Get(fullname)
}

func (c *Config) Contains(fullname string) bool {
	return c.data.Contains(fullname)
}
