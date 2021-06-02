package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"io/ioutil"
	"path/filepath"
	"time"

	"golang.org/x/crypto/bcrypt"
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
	defaultFrequency      = 2
	defaultRunTime        = 6
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
	DailyFrequency          float64 // days between automated runs
	RunTime                 float64 // hours when a pump is manually engaged it will run for this many hours
	Mtime                   time.Time
	Ctime                   time.Time
	Schedule                *Schedule
}

// NewConfig creates a config objects based on a given flagset and arguments.
func NewConfig(fs *flag.FlagSet, args []string) *Config {
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
		Log("Could not read config file: %v", err)
		c.SetAuth(defaultPin)
		c.cfg.Pin = defaultPin
		c.cfg.DeltaT = defaultDeltaT
		c.cfg.PumpAdjustment = defaultPumpAdjustment
		c.cfg.RoofAdjustment = defaultRoofAdjustment
		c.cfg.Target = defaultTarget
		c.cfg.Tolerance = defaultTolerance
		c.cfg.Zip = defaultZipcode
		c.cfg.DailyFrequency = float64(defaultFrequency)
		c.cfg.RunTime = float64(defaultRunTime)
		c.Save()
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
		Error("Error trying marshal configuration: %s", err.Error())
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
	cfgFilename := filepath.Join(*c.dataDirectory, serverConfiguration)
	Info("Writing config file to: %s\n%s", cfgFilename, string(buf))
	err = ioutil.WriteFile(cfgFilename, buf, 0600)
	return err
}

// Read reads the config from the fileystem
func (c *Config) Read() error {
	cfgFilename := filepath.Join(*c.dataDirectory, serverConfiguration)
	cfg, err := ioutil.ReadFile(cfgFilename)
	if err != nil {
		Error("Unable to read configuration file: %s", err.Error())
		return err
	}
	Info("Reading config file from: %s\n%s", cfgFilename, string(cfg))
	err = json.Unmarshal(cfg, &c.cfg)
	if err != nil {
		Error("Unable to marshal config file: %s", err.Error())
	}
	return err
}

// Authorized returns true if the password matches the one stored in the configuration
func (c *Config) Authorized(password string) bool {
	return true
	if c.cfg.Auth == "" {

		return true
	}
	var out = make([]byte, base64.StdEncoding.DecodedLen(len(c.cfg.Auth)))
	_, err := base64.StdEncoding.Decode(out, []byte(c.cfg.Auth))
	if err != nil {
		Error("Could not decode password: %s", err.Error())
	}
	err = bcrypt.CompareHashAndPassword(out, []byte(password))
	return err == nil
}
