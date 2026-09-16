package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

// Handler will handle the http requests
type Handler struct {
	ppc *PoolPumpController
}

// HostType is used to specify how to listen
type HostType uint8

const (
	// LocalHost is 127.0.0.1
	LocalHost HostType = iota
	// AnyHost is 0.0.0.0
	AnyHost
)

func (h HostType) String() string {
	switch h {
	case LocalHost:
		return "127.0.0.1"
	case AnyHost:
		return "0.0.0.0"
	}
	return ""
}

// Server for the pool-controller
type Server struct {
	port    int
	host    HostType
	handler *Handler
	server  http.Server
	done    chan bool
}

// NewServer creates a webserver
func NewServer(host HostType, port int, ppc *PoolPumpController) *Server {
	s := Server{
		port: port,
		host: host,
		handler: &Handler{
			ppc: ppc,
		},
		done: make(chan bool),
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	Info("Creating server on %s", addr)
	s.server = http.Server{
		Addr:    addr,
		Handler: s.handler,
	}
	s.server.ErrorLog = NewHTTPErrorLogger()
	return &s
}

func startServer(s *Server, cert, key string) {
	err := s.server.ListenAndServeTLS(cert, key)
	if err != nil {
		Fatal("Error from Server: %s", err.Error())
	}
	s.done <- true
	Info("Exiting HttpServer")
}

// Start the server
func (s *Server) Start(cert, key string) {
	go startServer(s, cert, key)
	Info("Starting HTTPS on %s:%d", s.host, s.port)
}

// Stop takes down the server
func (s *Server) Stop() {
	interval := time.Second
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := s.server.Shutdown(ctx)
	if err != nil {
		Info("HttpServerShutdown: %s", err.Error())
	}
	for {
		select {
		case <-s.done:
			return
		case <-time.After(interval):
			Info("Waiting for HttpServer to shut down")
		}
	}
}

const (
	// PumpImage is the pump data graph
	PumpImage = 0
	// TempImage is the temperature data graph
	TempImage = 1
)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	Debug("Received: %s", r.URL)
	switch r.URL.Path {
	case "/":
		h.rootHandler(w, r)
		return
	case "/pair":
		h.pairHandler(w, r)
		return
	case "/qr":
		h.qrHandler(w, r)
		return
	case "/pumps":
		h.graphHandler(w, r, PumpImage)
		return
	case "/temps":
		h.graphHandler(w, r, TempImage)
		return
	case "/status":
		h.statusHandler(w, r)
		return
	case "/config":
		h.configHandler(w, r)
		return
	case "/runCalibration":
		h.runCalibrationHandler(w, r)
		return
	case "/calibrate":
		h.calibrateHandler(w, r)
		return
	default:
		http.Error(w, "Unknown request type", 404)
	}
}

func (h *Handler) setRefresh(w http.ResponseWriter, r *http.Request, seconds int) {
	refresh := fmt.Sprintf("%d; url=%s", seconds, r.RequestURI)
	Debug("Setting Refresh to: %s", refresh)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Refresh", refresh)
}

func (h *Handler) writeResponse(w http.ResponseWriter, content []byte, ctype string) {
	w.Header().Set("Content-Type", ctype)
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func getscale(r *http.Request) string {
	scale := ""
	cookie, _ := r.Cookie("scale")
	if cookie != nil {
		scale = cookie.Value
	}
	return getFormValue(r, "scale", scale)
}

func duration(r *http.Request) time.Duration {
	var num int
	var let string
	scale := getscale(r)
	day := time.Hour * 24
	if len(scale) > 1 {
		fmt.Sscanf(scale, "%d%s", &num, &let)
		d := time.Duration(num)
		switch let {
		case "m":
			return d * time.Minute
		case "h":
			return d * time.Hour
		case "d":
			return d * day
		case "w":
			return d * 7 * day
		default:
		}
	}
	return day
}

func getFormValue(r *http.Request, name, defaultValue string) string {
	value := r.FormValue(name)
	if value == "" {
		return defaultValue
	}
	return value
}

func (h *Handler) graphHandler(w http.ResponseWriter, r *http.Request, which int) {
	var err error
	var graph []byte
	end := time.Now()
	start := end.Add(-1 * duration(r))
	width, _ := strconv.ParseUint(getFormValue(r, "width", "640"), 10, 32)
	height, _ := strconv.ParseUint(getFormValue(r, "height", "300"), 10, 32)
	if which == PumpImage {
		h.ppc.pumpRrd.Grapher().SetSize(uint(width), uint(height))
		_, graph, err = h.ppc.pumpRrd.Grapher().Graph(start, end)
	} else if which == TempImage {
		h.ppc.tempRrd.Grapher().SetSize(uint(width), uint(height))
		_, graph, err = h.ppc.tempRrd.Grapher().Graph(start, end)
	} else {
		http.Error(w, "Unknown Graph", 404)
		return
	}
	if err != nil {
		Error("Could not produce graph: %s", err.Error())
	}
	w.Header().Set("Cache-Control", "no-store")
	h.writeResponse(w, graph, "image/png")
}

type liveStatus struct {
	Pump       string  `json:"pump"`
	PumpOn     bool    `json:"pump_on"`
	Solar      string  `json:"solar"`
	SolarOn    bool    `json:"solar_on"`
	Thermostat string  `json:"thermostat"`
	Control    string  `json:"control"`
	TargetF    float64 `json:"target_f"`
	PoolF      float64 `json:"pool_f"`
	RoofF      float64 `json:"roof_f"`
	Updated    string  `json:"updated"`
}

func (h *Handler) liveStatus() liveStatus {
	control := "Auto"
	if h.ppc.switches.ManualState(h.ppc.config.cfg.RunTime) {
		control = "Manual"
	}
	mode := thermostatModeName(h.ppc.config.cfg)
	return liveStatus{
		Pump:       h.ppc.switches.State().String(),
		PumpOn:     h.ppc.switches.State() > OFF,
		Solar:      h.ppc.switches.solar.Status(),
		SolarOn:    h.ppc.switches.solar.Status() == "On",
		Thermostat: mode,
		Control:    control,
		TargetF:    toFarenheit(h.ppc.config.cfg.Target),
		PoolF:      toFarenheit(h.ppc.runningTemp.Temperature()),
		RoofF:      toFarenheit(h.ppc.roofTemp.Temperature()),
		Updated:    time.Now().Format("2006-01-02 15:04:05"),
	}
}

func (h *Handler) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	buf, err := json.Marshal(h.liveStatus())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.writeResponse(w, buf, "application/json")
}

func (h *Handler) pin() string {
	var p1, p2, p3 string
	fmt.Sscanf(h.ppc.config.cfg.Pin, "%3s%2s%3s", &p1, &p2, &p3)
	return fmt.Sprintf("%3s-%2s-%3s", p1, p2, p3)
}

func (h *Handler) pairHandler(w http.ResponseWriter, r *http.Request) {
	body := `<div class="card">
<h2>HomeKit pairing</h2>
<p class="pin">` + html.EscapeString(h.pin()) + `</p>
<img class="qr" src="/qr" width="256" height="256" alt="HomeKit pairing QR code">
</div>`
	h.writeResponse(w, []byte(page("HomeKit pairing", body)), "text/html")
}

func (h *Handler) qrHandler(w http.ResponseWriter, r *http.Request) {
	png, _ := qrcode.Encode(h.ppc.config.cfg.Pin, qrcode.Medium, 256)
	h.writeResponse(w, []byte(png), "image/png")
}

func (h *Handler) rootHandler(w http.ResponseWriter, r *http.Request) {
	scale := getscale(r)
	cookie := &http.Cookie{
		Name:   "scale",
		Value:  scale,
		MaxAge: int(365 * 24 * time.Hour / time.Second),
	}
	http.SetCookie(w, cookie)
	controlStr := "Auto"
	if h.ppc.switches.ManualState(h.ppc.config.cfg.RunTime) {
		controlStr = "Manual"
	}
	thermostatMode := thermostatModeName(h.ppc.config.cfg)
	pumpOn := h.ppc.switches.State() > OFF
	solarOn := h.ppc.switches.solar.Status() == "On"

	body := `<div class="pills">` +
		statusPill("pill-pump", "Pump", h.ppc.switches.State().String(), pumpOn) +
		statusPill("pill-solar", "Solar", h.ppc.switches.solar.Status(), solarOn) +
		statusPill("pill-thermostat", "Thermostat", thermostatMode, thermostatMode != "Off") +
		statusPill("pill-control", "Control", controlStr, controlStr == "Manual") +
		`</div>
<div class="metrics">` +
		metricCard("metric-target", "Target", fmt.Sprintf("%0.1f °F", toFarenheit(h.ppc.config.cfg.Target))) +
		metricCard("metric-pool", "Pool", fmt.Sprintf("%0.1f °F", toFarenheit(h.ppc.runningTemp.Temperature()))) +
		metricCard("metric-roof", "Roof", fmt.Sprintf("%0.1f °F", toFarenheit(h.ppc.roofTemp.Temperature()))) +
		`</div>
<form class="toolbar" action="/" method="POST">
<label>Window <input name="scale" value="` + html.EscapeString(scale) + `" placeholder="12h"></label>
<span>Examples: 12h, 2d, 1w</span>
<input type="submit" value="Show">
</form>
<div class="card chart">` + image("temps", 900, 320, scale) + `</div>
<div class="card chart">` + image("pumps", 900, 220, scale) + `</div>
<p class="legend">4 solar mixing · 3 solar heating · 2 cleaning · 1 pump · 0 off · −1 disabled</p>
<p class="updated" id="updated">Updated ` + html.EscapeString(time.Now().Format("2006-01-02 15:04:05")) + `</p>
` + liveRefreshScript

	h.writeResponse(w, []byte(page("Pool controller", body)), "text/html")
}

func (h *Handler) calibrateHandler(w http.ResponseWriter, r *http.Request) {
	body := `<div class="card">
<h2>Thermometer calibration</h2>
<p class="lead">Put known resistors across both probe terminals. 10,000 ohms is the usual value; measure yours if you can.</p>
<form class="stack" action="/runCalibration" method="POST">
<label>Pump resistor (ohms)<input name="pump_res" value="10000" inputmode="decimal"></label>
<label>Roof resistor (ohms)<input name="roof_res" value="10000" inputmode="decimal"></label>
<input type="submit" name="submit" value="Run calibration">
</form>
</div>`
	h.writeResponse(w, []byte(page("Calibration", body)), "text/html")
}

// Calibrate runs a routine to calibrate the thermometers using measured resistors
func (h *Handler) Calibrate(html *string, t Thermometer, resStr, name string) error {
	r, err := strconv.ParseFloat(resStr, 64)
	if err != nil {
		*html += "<h2>Could not parse " + resStr + "for: " + name
		*html += ", please correct the value.</h2><br>(" + err.Error() + ")"
		return err
	}
	err = t.Calibrate(r)
	if err != nil {
		*html += "<h2>Calibration failed, please try again.</h2><br>(" + err.Error() + ")"
		return err
	}
	return nil
}

func (h *Handler) runCalibrationHandler(w http.ResponseWriter, r *http.Request) {
	pumpResistance := getFormValue(r, "pump_res", "")
	roofResistance := getFormValue(r, "roof_res", "")

	html := "<div class=\"card\"><h2>Thermometer calibration</h2>"
	retry := http.Request{
		URL: &url.URL{
			RawPath: "/calibrate",
		},
	}

	success := http.Request{
		URL: &url.URL{
			RawPath: "/",
		},
	}

	if pumpResistance == "" || roofResistance == "" { // No values submitted
		h.setRefresh(w, &retry, 10)
		html += "<h2>Please provide valid resistance for each resistor.</h2> Redirecting..."
	} else {
		if h.Calibrate(&html, h.ppc.pumpTemp, pumpResistance, "Pump Probe") == nil &&
			h.Calibrate(&html, h.ppc.roofTemp, roofResistance, "Roof Probe") == nil {
			h.setRefresh(w, &success, 10)
			h.ppc.PersistCalibration()
			html += "<h2>Success</h2><br>"
			p, ok := h.ppc.pumpTemp.(*GpioThermometer)
			if ok {
				html += fmt.Sprintf("<br>Pool Value: %0.3f", p.adjust)
			}
			p, ok = h.ppc.roofTemp.(*GpioThermometer)
			if ok {
				html += fmt.Sprintf("<br>Roof Value: %0.3f", p.adjust)
			}
		} else {
			html += "<p>Redirecting...."
			h.setRefresh(w, &retry, 10)
		}
	}
	html += "</div>"
	h.writeResponse(w, []byte(page("Calibration", html)), "text/html")
}

// Authenticate the user
func (h *Handler) Authenticate(r *http.Request) bool {
	user, password, ok := r.BasicAuth()
	if !ok || user != "admin" {
		Error("Unknown user (%s) attempting to configure server", user)
		return false
	}
	if h.ppc.config.Authorized(password) {
		Debug("User %s logged in", user)
		return true
	}
	Error("Login for User (%s) failed", user)
	return false
}

func processBoolUpdate(r *http.Request, formname string, ptr *bool) bool {
	value := false
	strvalue := getFormValue(r, formname, "false")
	Log("Update to boolean: %q = %q, was %t", formname, strvalue, *ptr)
	if strvalue == "true" {
		value = true
	}
	if value != *ptr {
		Debug("Updating value for %s from %t to %t", formname, *ptr, value)
		*ptr = value
		return true
	}
	Debug("No update to %s, value(%t) orig(%t)", formname, value, *ptr)
	return false
}

func processFloatUpdate(r *http.Request, formname string, ptr *float64) bool {
	curvalue := fmt.Sprintf("%0.2f", *ptr)
	value := getFormValue(r, formname, "")
	if value != curvalue {
		flt, err := strconv.ParseFloat(value, 64)
		if err == nil {
			Debug("Updating value for %s from %s to %s", formname, curvalue, value)
			*ptr = flt
			return true
		}
	}
	Debug("No update to %s, value(%s) orig(%s)", formname, value, curvalue)
	return false
}

func (h *Handler) configBoolRow(name, inputName string, value bool) string {
	checked := ""
	if value {
		checked = " checked"
	}
	return fmt.Sprintf(
		`<label class="check"><input type="checkbox" name="%s" value="true"%s> %s</label>`+"\n",
		html.EscapeString(inputName), checked, html.EscapeString(name))
}

func (h *Handler) configRow(name, inputName, configValue, extraArgs string) string {
	return fmt.Sprintf(
		`<label>%s<input name="%s" value="%s" %s></label>`+"\n",
		html.EscapeString(name), html.EscapeString(inputName), configValue, extraArgs)
}

func (h *Handler) processForm(r *http.Request, c *Config) {
	var foundone bool
	pw := getFormValue(r, "passcode", "")
	if pw1 := getFormValue(r, "passcode2", ""); pw != "" && pw1 != "" && pw == pw1 {
		c.SetAuth(pw1)
		foundone = true
	}
	if processFloatUpdate(r, "adj_pump", &c.cfg.PumpAdjustment) {
		h.ppc.SyncAdjustments()
		foundone = true
	}
	if processFloatUpdate(r, "adj_roof", &c.cfg.RoofAdjustment) {
		h.ppc.SyncAdjustments()
		foundone = true
	}
	if processFloatUpdate(r, "target", &c.cfg.Target) {
		foundone = true
	}
	if processFloatUpdate(r, "tolerance", &c.cfg.Tolerance) {
		foundone = true
	}
	if processFloatUpdate(r, "mindelta", &c.cfg.DeltaT) {
		foundone = true
	}
	if processBoolUpdate(r, "disabled", &c.cfg.Disabled) {
		foundone = true
	}
	if processBoolUpdate(r, "button_disabled", &c.cfg.ButtonDisabled) {
		foundone = true
	}
	if processBoolUpdate(r, "solar_disabled", &c.cfg.SolarDisabled) {
		foundone = true
	}
	if processBoolUpdate(r, "heat_disabled", &c.cfg.HeatDisabled) {
		foundone = true
	}
	if processBoolUpdate(r, "cool_disabled", &c.cfg.CoolDisabled) {
		foundone = true
	}
	if processFloatUpdate(r, "daily_freq", &c.cfg.DailyFrequency) {
		foundone = true
	}
	if processFloatUpdate(r, "run_time", &c.cfg.RunTime) {
		foundone = true
	}
	if foundone {
		c.Save()
	}

	// Don't persist this one
	posted := getFormValue(r, "posted", "")
	if posted == "true" { // only change on form submission
		value := getFormValue(r, "debug", "")
		if value == "on" {
			EnableDebug()
			Debug("Enabling Debug: value(%s) posted(%s)", value, posted)
		} else {
			Debug("Disabling Debug: value(%s) posted(%s)", value, posted)
			DisableDebug()
		}
	}
}

func (h *Handler) configHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: move this to a form on the page.
	w.Header().Set("WWW-Authenticate", "Basic") //  realm=\"Bonnie Labs\"
	if !h.Authenticate(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	c := h.ppc.config
	Debug("Config: %+v", c.cfg)

	posted := getFormValue(r, "posted", "")
	if posted == "true" {
		h.processForm(r, c)
		if h.ppc.thermostat != nil {
			h.ppc.thermostat.Sync()
		}
	}

	passArgs := `type="password" autocomplete="new-password"`

	body := `<form class="stack" action="/config" method="POST">
<fieldset>
<legend>Administrator</legend>
` + h.configRow("Admin password", "passcode", "", passArgs) + `
` + h.configRow("Confirm password", "passcode2", "", passArgs) + `
</fieldset>
<fieldset>
<legend>Sensor tuning</legend>
` + h.configRow("Pump adjustment", "adj_pump", fmt.Sprintf("%0.2f", c.cfg.PumpAdjustment), "") + `
` + h.configRow("Roof adjustment", "adj_roof", fmt.Sprintf("%0.2f", c.cfg.RoofAdjustment), "") + `
</fieldset>
<fieldset>
<legend>Solar</legend>
` + h.configRow("Target (°C)", "target", fmt.Sprintf("%0.2f", c.cfg.Target), "") + `
` + h.configRow("Tolerance (°C)", "tolerance", fmt.Sprintf("%0.2f", c.cfg.Tolerance), "") + `
` + h.configRow("Min delta (°C)", "mindelta", fmt.Sprintf("%0.2f", c.cfg.DeltaT), "") + `
` + h.configRow("Daily run frequency (days)", "daily_freq", fmt.Sprintf("%0.2f", c.cfg.DailyFrequency), "") + `
` + h.configRow("Run period (hours)", "run_time", fmt.Sprintf("%0.2f", c.cfg.RunTime), "") + `
</fieldset>
<fieldset>
<legend>Debug and disables</legend>
` + h.configBoolRow("Debug logging", "debug", doDebug) + `
` + h.configBoolRow("Disable all pumps", "disabled", c.cfg.Disabled) + `
` + h.configBoolRow("Disable button", "button_disabled", c.cfg.ButtonDisabled) + `
` + h.configBoolRow("Disable solar", "solar_disabled", c.cfg.SolarDisabled) + `
` + h.configBoolRow("Disable heating (HomeKit Cool)", "heat_disabled", c.cfg.HeatDisabled) + `
` + h.configBoolRow("Disable cooling (HomeKit Heat)", "cool_disabled", c.cfg.CoolDisabled) + `
</fieldset>
<input type="hidden" name="posted" value="true">
<input type="submit" value="Save">
</form>`

	h.writeResponse(w, []byte(page("Configuration", body)), "text/html")
}
