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
	defaultSslCert        = "/etc/ssl/certs/pool-controller.crt"
	defaultSslKey         = "/etc/ssl/private/pool-controller.key"
	defaultDataDir        = "/var/cache/homekit"
	defaultPidFile        = "/tmp/pool-controller.pid"
	defaultPin            = "74023718"
	defaultWUAppID        = ""
	defaultZipcode        = ""
	defaultAuth           []byte
	defaultTarget         = 30.0
	defaultDeltaT         = 12.0
	defaultTolerance      = 0.5
	defaultPumpAdjustment = 1.0
	defaultRoofAdjustment = 1.0
	serverConfiguration   = "/server.conf"
)

// Config holds various configuration entries for the system.
type Config struct {
	// Commandline only
	sslCertificate *string
	sslPrivateKey  *string
	dataDirectory  *string
	forceRrd       *bool
	persist        *bool

	// Updatable
	disabled                *bool
	buttonDisabled          *bool
	solarDisabled           *bool
	auth                    *[]byte
	weatherUndergroundAppID *string
	zip                     *string
	pin                     *string
	target                  *float64
	deltaT                  *float64
	tolerance               *float64
	pumpAdjustment          *float64
	roofAdjustment          *float64

	// Internal
	pidfile *string
	mtime   time.Time
	ctime   time.Time
}

// NewConfig creates a config objects based on a given flagset and arguments.
func NewConfig(fs *flag.FlagSet, args []string) *Config {
	// **** TODO _ SWAP BACK ONCE FLAGS WORK IN ECLIPSE ON MACOSX
	// if __test__ {
	// 	defaultSslCert = "tests/test.crt"
	// 	defaultSslKey = "tests/test.key"
	// 	defaultDataDir = "tmp"
	// 	Info("In Testmode for config settings")
	// }
	c := Config{
		ctime: time.Now(),
	}
	c.sslCertificate = fs.String("ssl_cert", defaultSslCert,
		"SSL cert to use for web server and homekit server")
	c.sslPrivateKey = fs.String("ssl_key", defaultSslKey,
		"SSL private key to use for web server and homekit server")
	c.dataDirectory = fs.String("data_dir", defaultDataDir,
		"Directory for homekit data")
	c.pin = fs.String("pin", defaultPin,
		"8-digit Homekit Pin shown to users who want to add the device")
	c.weatherUndergroundAppID = fs.String("wuid", defaultWUAppID,
		"AppId provided by WeatherUnderground (https://www.wunderground.com/weather/api/)")
	c.zip = fs.String("zip", defaultZipcode,
		"Local Zipcode.  If left blank, no weather will be fetched.")
	c.target = fs.Float64("target", defaultTarget,
		"Sets the target temperature for the pool")
	c.deltaT = fs.Float64("dt", defaultDeltaT, "Sets the minimum difference in temperature "+
		"between roof and pumps to utilize solar panels")
	c.tolerance = fs.Float64("tol", defaultTolerance,
		"Sets the temperature variance allowed around the target")
	c.pumpAdjustment = fs.Float64("pump_adj", defaultPumpAdjustment,
		"Sets the measured capacitance in microFarads for the inline pump capacitor")
	c.roofAdjustment = fs.Float64("roof_adj", defaultRoofAdjustment,
		"Sets the measured capacitance in microFarads for the inline roof capacitor")
	c.pidfile = fs.String("pid", defaultPidFile,
		"File to write the process id into.")
	c.forceRrd = fs.Bool("f", false,
		"force creation of new RRD files if present")
	c.persist = fs.Bool("p", false,
		"If true, any parameter values changed via web interface are saved to a file and read on "+
			"startup.  If false, any saved values will be ignored on start.  Saved changes "+
			"supercede all flags.")
	c.buttonDisabled = fs.Bool("button_disabled", false,
		"If true, the button the controller will be ignored.")
	c.disabled = fs.Bool("disabled", false,
		"Turns off the pumps and does not allow them to operate.")
	c.solarDisabled = fs.Bool("solar_disabled", false,
		"Turns off the solar.  The system will not attempt to reach the target temperature.")
	fs.Parse(args)

	defaultAuth = crypt(*c.pin)
	c.auth = &defaultAuth
	return &c
}

func crypt(s string) []byte {
	hash, _ := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
	return hash
}

// SetAuth saves the given password in a crypt form.
func (c *Config) SetAuth(password string) {
	str := crypt(password)
	c.auth = &str
}

// GetAuth returns a password that has been stored in a crypt form.
func (c *Config) GetAuth() []byte {
	return *c.auth
}

func (c *Config) String() string {
	return fmt.Sprintf("Config: {data_dir:\"%s\", pin:\"%s\", forceRrd:%t, "+
		"auth:\"%5s...\", WUappId:\"%s\", zip:\"%s\", target:%0.2f, deltaT:%0.2f, "+
		"tolerance:%0.2f, adj_pump:%0.2f, adj_roof:%0.2f disabled:%t "+
		"solar_disabled:%t mtime:\"%.19s\", ctime:\"%.19s\" }",
		*c.dataDirectory, *c.pin, *c.forceRrd,
		c.GetAuth(), *c.weatherUndergroundAppID, *c.zip, *c.target, *c.deltaT,
		*c.tolerance, *c.pumpAdjustment, *c.roofAdjustment, *c.disabled,
		*c.solarDisabled, c.mtime, c.ctime)
}

// OverwriteWithSaved if persist is true, the configuration stored in the config file will be used to
// overwrite whatever settings have been made.
func (c *Config) OverwriteWithSaved(path string) {
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
			authB64, err := base64.StdEncoding.DecodeString(l[1])
			if err != nil {
				Fatal("Corrupt authentication found, quiting")
			}
			c.auth = &authB64
			break
		case "disabled":
			if l[1] == "true" {
				*c.disabled = true
			} else {
				*c.disabled = false
			}
			break
		case "button_disabled":
			if l[1] == "true" {
				*c.buttonDisabled = true
			} else {
				*c.buttonDisabled = false
			}
			break
		case "solar_disabled":
			if l[1] == "true" {
				*c.solarDisabled = true
			} else {
				*c.solarDisabled = false
			}
			break
		case "WUappId":
			c.weatherUndergroundAppID = &l[1]
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
				c.pumpAdjustment = &cp
			}
			break
		case "adj_roof":
			cr, err := strconv.ParseFloat(l[1], 64)
			if check(err, "Could not value %s", line) == nil {
				c.roofAdjustment = &cr
			}
			break
		}
	}
}

// Save commits the current configuration settings to the configuration file so they aren't lost on restart.
func (c *Config) Save(path string) error {
	if !*c.persist {
		return nil
	}
	out := ""
	if bytes.Compare(c.GetAuth(), defaultAuth) != 0 {
		out += fmt.Sprintf("auth:%s\n", base64.StdEncoding.EncodeToString(c.GetAuth()))
	}
	if *c.weatherUndergroundAppID != defaultWUAppID {
		out += fmt.Sprintf("WUappId:%s\n", *c.weatherUndergroundAppID)
	}
	if *c.pin != defaultPin {
		out += fmt.Sprintf("HomekitPin:%s\n", *c.pin)
	}
	if *c.zip != defaultZipcode {
		out += fmt.Sprintf("zip:%s\n", *c.zip)
	}
	if *c.target != defaultTarget {
		out += fmt.Sprintf("target:%f\n", *c.target)
	}
	if *c.deltaT != defaultDeltaT {
		out += fmt.Sprintf("deltaT:%f\n", *c.deltaT)
	}
	if *c.tolerance != defaultTolerance {
		out += fmt.Sprintf("tolerance:%f\n", *c.tolerance)
	}
	if *c.pumpAdjustment != defaultPumpAdjustment {
		out += fmt.Sprintf("adj_pump:%f\n", *c.pumpAdjustment)
	}
	if *c.roofAdjustment != defaultRoofAdjustment {
		out += fmt.Sprintf("adj_roof:%f\n", *c.roofAdjustment)
	}
	if *c.disabled {
		out += "disabled:true\n"
	}
	if *c.buttonDisabled {
		out += "button_disabled:true\n"
	}
	if *c.solarDisabled {
		out += "solar_disabled:true\n"
	}
	if len(out) > 0 {
		return ioutil.WriteFile(path, []byte(out), os.FileMode(0644))
	}
	return nil
}
