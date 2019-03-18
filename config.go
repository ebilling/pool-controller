package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"golang.org/x/crypto/bcrypt"
	"io/ioutil"
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
	defaultPumpAdjustment = 2.5
	defaultRoofAdjustment = 2.5
	serverConfiguration   = "/server.conf"
	defaultCfg            = PersistedConfig{
		Disabled: false,
	}
)

// Config holds various configuration entries for the system.
type Config struct {
	// Commandline only
	sslCertificate *string
	sslPrivateKey  *string
	dataDirectory  *string
	forceRrd       *bool
	persist        *bool

	// Internal
	pidfile *string

	// Persisted
	cfg *PersistedConfig
}

// PersistedConfig is the portion of the configuration that can be altered and saved from the UI
type PersistedConfig struct {
	// Updatable
	Disabled                bool
	ButtonDisabled          bool
	SolarDisabled           bool
	Auth                    string
	WeatherUndergroundAppID string
	Zip                     string
	Pin                     string
	Target                  float64
	DeltaT                  float64
	Tolerance               float64
	PumpAdjustment          float64
	RoofAdjustment          float64
	Mtime                   time.Time
	Ctime                   time.Time
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
		cfg: &PersistedConfig{},
	}

	c.sslCertificate = fs.String("ssl_cert", defaultSslCert,
		"SSL cert to use for web server and homekit server")
	c.sslPrivateKey = fs.String("ssl_key", defaultSslKey,
		"SSL private key to use for web server and homekit server")
	c.dataDirectory = fs.String("data_dir", defaultDataDir,
		"Directory for homekit data")
	c.pidfile = fs.String("pid", defaultPidFile,
		"File to write the process id into.")
	c.forceRrd = fs.Bool("f", false,
		"force creation of new RRD files if present")
	c.persist = fs.Bool("p", false,
		"If true, any parameter values changed via web interface are saved to a file and read on "+
			"startup.  If false, any saved values will be ignored on start.  Saved changes "+
			"supercede all flags.")
	fs.Parse(args)
	err := c.Read()
	if err != nil {
		c.SetAuth(defaultPin)
		c.cfg.Pin = defaultPin
		c.cfg.DeltaT = defaultDeltaT
		c.cfg.PumpAdjustment = defaultPumpAdjustment
		c.cfg.RoofAdjustment = defaultRoofAdjustment
		c.cfg.Target = defaultTarget
		c.cfg.Tolerance = defaultTolerance
		c.cfg.Zip = defaultZipcode
	}
	return &c
}

func crypt(s string) []byte {
	hash, _ := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
	return hash
}

// SetAuth saves the given password in a crypt form.
func (c *Config) SetAuth(password string) {
	str := crypt(password)
	buf := bytes.NewBuffer(nil)
	baseEncoder := base64.NewEncoder(base64.StdEncoding, buf)
	baseEncoder.Write([]byte(str))
	c.cfg.Auth = buf.String()
}

// GetAuth returns a password that has been stored in a base64 encoded crypt form.
func (c *Config) GetAuth() string {
	return c.cfg.Auth
}

func (c *Config) String() string {
	str, err := json.Marshal(c.cfg)
	if err != nil {
		Log("Error trying marshal configuration: %s", err.Error())
	}
	return "Config: " + string(str)
}

// Save commits the current configuration settings to the configuration file so they aren't lost on restart.
func (c *Config) Save() error {
	if !*c.persist {
		return nil
	}
	var timeZero time.Time
	if c.cfg.Ctime == timeZero {
		c.cfg.Ctime = time.Now()
	}
	c.cfg.Mtime = time.Now()

	buf, err := json.Marshal(c.cfg)
	if err != nil {
		return err
	}
	Log("Config:\n%s", string(buf))

	err = ioutil.WriteFile(*c.dataDirectory+serverConfiguration, buf, 0600)
	return err
}

// Read reads the config from the fileystem
func (c *Config) Read() error {
	cfg, err := ioutil.ReadFile(*c.dataDirectory + serverConfiguration)
	if err != nil {
		Log("Unable to read configuration file: %s", err.Error())
		return err
	}
	err = json.Unmarshal(cfg, &c.cfg)
	if err != nil {
		Log("Unable to marshal config file: %s", err.Error())
		return err
	}
	return nil
}

// Authorized returns true if the password matches the one stored in the configuration
func (c *Config) Authorized(password string) bool {
	var out = make([]byte, base64.StdEncoding.DecodedLen(len(c.cfg.Auth)))
	_, err := base64.StdEncoding.Decode(out, []byte(c.cfg.Auth))
	if err != nil {
		Error("Could not decode password: %s", err.Error())
	}
	err = bcrypt.CompareHashAndPassword(out, []byte(password))
	if err == nil {
		return true
	}
	return false
}
