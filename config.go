package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	default_ssl_cert  = "/etc/ssl/certs/pool-controller.crt"
	default_ssl_key   = "/etc/ssl/private/pool-controller.key"
	default_data_dir  = "/var/cache/homekit"
	default_pidfile   = "/tmp/pool-controller.pid"
	default_pin       = "74023718"
	default_WUappId   = ""
	default_zip       = ""
	default_auth      []byte
	default_target    = 30.0
	default_deltaT    = 12.0
	default_tolerance = 0.5
	default_adj_pump  = 1.0
	default_adj_roof  = 1.0
	default_forceRrd  = false
	server_conf       = "/server.conf"
)

type Config struct {
	// Commandline only
	ssl_cert *string
	ssl_key  *string
	data_dir *string
	pin      *string
	forceRrd *bool
	persist  *bool

	// Updatable
	auth      *[]byte
	WUappId   *string
	zip       *string
	target    *float64
	deltaT    *float64
	tolerance *float64
	adj_pump  *float64
	adj_roof  *float64

	// Internal
	pidfile *string
	mtime   time.Time
	ctime   time.Time
}

func NewConfig(fs *flag.FlagSet, args []string) *Config {
	// **** TODO _ SWAP BACK ONCE FLAGS WORK IN ECLIPSE ON MACOSX
	if __test__ {
		default_ssl_cert = "tests/test.crt"
		default_ssl_key = "tests/test.key"
		default_data_dir = "tmp"
		Info("In Testmode for config settings")
	}
	c := Config{
		ctime: time.Now(),
	}
	c.ssl_cert = fs.String("ssl_cert", default_ssl_cert,
		"SSL cert to use for web server and homekit server")
	c.ssl_key = fs.String("ssl_key", default_ssl_key,
		"SSL private key to use for web server and homekit server")
	c.data_dir = fs.String("data_dir", default_data_dir,
		"Directory for homekit data")
	c.pin = fs.String("pin", default_pin,
		"8-digit Homekit Pin shown to users who want to add the device")
	c.WUappId = fs.String("wuid", default_WUappId,
		"AppId provided by WeatherUnderground (https://www.wunderground.com/weather/api/)")
	c.zip = fs.String("zip", default_zip,
		"Local Zipcode.  If left blank, no weather will be fetched.")
	c.target = fs.Float64("target", default_target,
		"Sets the target temperature for the pool")
	c.deltaT = fs.Float64("dt", default_deltaT, "Sets the minimum difference in temperature "+
		"between roof and pumps to utilize solar panels")
	c.tolerance = fs.Float64("tol", default_tolerance,
		"Sets the temperature variance allowed around the target")
	c.adj_pump = fs.Float64("pump_adj", default_adj_pump,
		"Sets the measured capacitance in microFarads for the inline pump capacitor")
	c.adj_roof = fs.Float64("roof_adj", default_adj_roof,
		"Sets the measured capacitance in microFarads for the inline roof capacitor")
	c.forceRrd = fs.Bool("f", default_forceRrd,
		"force creation of new RRD files if present")
	c.pidfile = fs.String("pid", default_pidfile,
		"File to write the process id into.")
	c.persist = fs.Bool("p", false,
		"If true, any parameter values changed via web interface are saved to a file and read on "+
			"startup.  If false, any saved values will be ignored on start.  Saved changes "+
			"supercede all flags.")
	fs.Parse(args)

	default_auth = crypt(*c.pin)
	c.auth = &default_auth
	return &c
}

func crypt(s string) []byte {
	hash, _ := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
	return hash
}

func (c *Config) SetAuth(password string) {
	str := crypt(password)
	c.auth = &str
}

func (c *Config) GetAuth() []byte {
	return *c.auth
}

func (c *Config) String() string {
	return fmt.Sprintf("Config: {data_dir:\"%s\", pin:\"%s\", forceRrd:%t, auth:\"%5s...\", "+
		"WUappId:\"%s\", zip:\"%s\", target:%0.2f, deltaT:%0.2f, tolerance:%0.2f, "+
		"adj_pump:%0.2f, adj_roof:%0.2f mtime:\"%.19s\", ctime:\"%.19s\" }",
		*c.data_dir, *c.pin, *c.forceRrd, c.GetAuth(), *c.WUappId, *c.zip, *c.target,
		*c.deltaT, *c.tolerance, *c.adj_pump, *c.adj_roof, c.mtime, c.ctime)
}

func (c *Config) OverwriteWithSaved(path string) {
	Info("Running OverwriteWithSaved - Current Config: %s", c.String())
	if !*c.persist {
		return
	}
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	Info("Reading the file")
	lines := strings.Split(string(buf), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		l := strings.Split(line, ":")
		if len(l) != 2 {
			Error("Found bad line in config, aborting load: \"%s\"", line)
		}
		switch l[0] {
		case "auth":
			auth_b, err := base64.StdEncoding.DecodeString(l[1])
			if err != nil {
				Fatal("Corrupt authentication found, quiting")
			}
			c.auth = &auth_b
			break
		case "WUappId":
			c.WUappId = &l[1]
			break
		case "HomekitPin":
			c.pin = &l[1]
			break
		case "zip":
			c.zip = &l[1]
			break
		case "target":
			tgt, err := strconv.ParseFloat(l[1], 64)
			if check(err, "Could not value %s", line) == nil {
				c.target = &tgt
			}
			break
		case "deltaT":
			dt, err := strconv.ParseFloat(l[1], 64)
			if check(err, "Could not value %s", line) == nil {
				c.deltaT = &dt
			}
			break
		case "tolerance":
			tol, err := strconv.ParseFloat(l[1], 64)
			if check(err, "Could not value %s", line) == nil {
				c.tolerance = &tol
			}
			break
		case "adj_pump":
			cp, err := strconv.ParseFloat(l[1], 64)
			if check(err, "Could not value %s", line) == nil {
				c.adj_pump = &cp
			}
			break
		case "adj_roof":
			cr, err := strconv.ParseFloat(l[1], 64)
			if check(err, "Could not value %s", line) == nil {
				c.adj_roof = &cr
			}
			break
		}
	}
	Info("New Config: %s", c.String())
}

func (c *Config) Save(path string) error {
	if !*c.persist {
		return nil
	}
	out := ""
	if bytes.Compare(c.GetAuth(), default_auth) != 0 {
		out += fmt.Sprintf("auth:%s\n", base64.StdEncoding.EncodeToString(c.GetAuth()))
	}
	if *c.WUappId != default_WUappId {
		out += fmt.Sprintf("WUappId:%s\n", *c.WUappId)
	}
	if *c.pin != default_pin {
		out += fmt.Sprintf("HomekitPin:%s\n", *c.pin)
	}
	if *c.zip != default_zip {
		out += fmt.Sprintf("zip:%s\n", *c.zip)
	}
	if *c.target != default_target {
		out += fmt.Sprintf("target:%f\n", *c.target)
	}
	if *c.deltaT != default_deltaT {
		out += fmt.Sprintf("deltaT:%f\n", *c.deltaT)
	}
	if *c.tolerance != default_tolerance {
		out += fmt.Sprintf("tolerance:%f\n", *c.tolerance)
	}
	if *c.adj_pump != default_adj_pump {
		out += fmt.Sprintf("adj_pump:%f\n", *c.adj_pump)
	}
	if *c.adj_roof != default_adj_roof {
		out += fmt.Sprintf("adj_roof:%f\n", *c.adj_roof)
	}
	if len(out) > 0 {
		return ioutil.WriteFile(path, []byte(out), os.FileMode(0644))
	}
	return nil
}
