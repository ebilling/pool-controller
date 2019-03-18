package main

import (
	"context"
	"fmt"
	qrcode "github.com/skip2/go-qrcode"
	"net/http"
	"net/url"
	"strconv"
	"time"
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
	s.server = http.Server{
		Addr:    fmt.Sprintf("%s:%d", host, port),
		Handler: s.handler,
	}
	s.server.ErrorLog = Logger() // Direct errors to common log
	return &s
}

func startServer(s *Server, cert, key string) {
	err := s.server.ListenAndServeTLS(cert, key)
	if err != nil {
		Error("Error from Server: %s", err.Error())
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
	h.setRefresh(w, r, 20) // Refresh image every 20 seconds
	h.writeResponse(w, graph, "image/png")
}

func image(which string, width, height int, scale string) string {
	return fmt.Sprintf("<img src=\"/%s?scale=%s&width=%d&height=%d\" width=%d height=%d "+
		"alt=\"Temperatures and Solar Radiation\" />",
		which, scale, width, height, width, height)
}

func indent(howmany int) string {
	out := ""
	for i := 0; i < howmany; i++ {
		out += "\t"
	}
	return out
}

func (h *Handler) pin() string {
	var p1, p2, p3 string
	fmt.Sscanf(h.ppc.config.cfg.Pin, "%3s%2s%3s", &p1, &p2, &p3)
	return fmt.Sprintf("%3s-%2s-%3s", p1, p2, p3)
}

func (h *Handler) pairHandler(w http.ResponseWriter, r *http.Request) {
	html := "<html><head><title>HomeKit Pairing Codes</title></head><body><center>"
	html += "<table><tr><th>" + h.pin() + "</th></tr>"
	html += "<tr><td><img src=\"/qr\"></td></tr></table>"
	html += nav()
	html += "</center></body></html>"
	h.writeResponse(w, []byte(html), "text/html")
}

func (h *Handler) qrHandler(w http.ResponseWriter, r *http.Request) {
	png, _ := qrcode.Encode(h.ppc.config.cfg.Pin, qrcode.Medium, 256)
	h.writeResponse(w, []byte(png), "image/png")
}

func nav() string {
	out := "<p><font face=helvetica color=#444444 size=-2>"
	out += "<table cellspacing=5><tr><td><a href=/>graphs</a></td><td>&nbsp;</td>\n"
	out += "<td><a href=/pair>homekit</a></td><td>&nbsp;</td>\n"
	out += "<td><a href=/calibrate>calibrate</a></td><td>&nbsp;</td>\n"
	out += "<td><a href=/config>config</a></td></tr></table></font>\n"
	return out
}

func (h *Handler) rootHandler(w http.ResponseWriter, r *http.Request) {
	scale := getscale(r)
	cookie := &http.Cookie{
		Name:   "scale",
		Value:  scale,
		MaxAge: int(365 * 24 * time.Hour / time.Second),
	}
	http.SetCookie(w, cookie)
	h.setRefresh(w, r, 60)
	modeStr := "Auto"
	if h.ppc.switches.ManualState() {
		modeStr = "Manual"
	}

	weatherData, _ := h.ppc.weather.GetWeatherByZip(h.ppc.config.cfg.Zip)

	html := "<html><head><title>Pool Pump Controller</title></head><body><center>" +
		"<table>\n"
	html += indent(1) + "<tr><td colspan=2 align=center><font face=helvetica color=#444444 " +
		"size=-1><form action=/ method=POST>Time Window:<input name=scale value=\"" +
		scale + "\" size=5> ex. 12h (w, d, h, m)</form></font></td></tr>\n"
	html += indent(1) + "<tr><td>" + image("temps", 640, 300, scale) + "</td>"
	html += "<td align=left nowrap><font face=helvetica color=#444444 size=-1>"
	html += fmt.Sprintf("Target: %0.1f F<br>", toFarenheit(h.ppc.config.cfg.Target))
	html += fmt.Sprintf("Pool: %0.1f F<br>", toFarenheit(h.ppc.runningTemp.Temperature()))
	html += fmt.Sprintf("Roof: %0.1f F<br>", toFarenheit(h.ppc.roofTemp.Temperature()))
	html += fmt.Sprintf("Weather: %0.1f F<br>", toFarenheit(weatherData.CurrentTempC))
	html += fmt.Sprintf("Solar: %0.1f W/sqm", weatherData.SolarRadiation)
	html += "</font></td></tr>\n"
	html += indent(1) + "<tr><td colspan=2><br></td></tr>"
	html += indent(1) + "<tr>"
	html += "<td>" + image("pumps", 640, 200, scale) + "</td>"
	html += "<td align=left nowrap><font face=helvetica color=#444444 size=-1>"
	html += fmt.Sprintf("Pump: %s<br>", h.ppc.switches.State())
	html += fmt.Sprintf("Solar: %s<br>", h.ppc.switches.solar.Status())
	html += fmt.Sprintf("Mode: %s", modeStr)
	html += "</font></td></tr>\n"
	html += indent(1) + "<tr><td align=center><font size=-1 color=#aaaaaa>" +
		"4=SolarMixing, 3=SolarHeating, 2=Cleaning, 1=PumpRunning, 0=Off, " +
		"-1=Disabled</font></td><td></td></tr>\n"
	html += "<tr><td colspan=2><br></td></tr>\n"
	html += indent(1) + "<tr><td align=center>" +
		fmt.Sprintf("Updated: %.19s", time.Now().String()) +
		"</td><td></td></tr>\n"
	html += "<tr><td align=center>" + nav() + "</td><td></td></tr>\n"
	html += "</table></font>"
	html += "</center></body></html>"
	h.writeResponse(w, []byte(html), "text/html")
}

func (h *Handler) calibrateHandler(w http.ResponseWriter, r *http.Request) {
	html := "<html><head><title>Thermometer Calibration</title></head><body><center>"
	html += `<font face=helvetica color=#444444 size=-1>To calibrate your system, please
	insert resistors of known value across the terminals for BOTH temperature probes.
	<b>Suggested value is 10,000Ohms.</b>, but you can measure it for increased
	accuracy.</font><br>`
	html += "<table><form action=/runCalibration method=POST>\n"
	html += "<tr><td align=right><font face=helvetica color=#444444 size=-1>Pump Resistor Value</td>"
	html += "<td><input name=pump_res value=10000 size=5></font> ohms</td></tr>\n"
	html += "<tr><td align=right><font face=helvetica color=#444444 size=-1>Roof Resistor Value</td>"
	html += "<td><input name=roof_res value=10000 size=5></font> ohms</td></tr>\n"
	html += "<tr><td colspan=2 align=center><input type=submit name=submit value=Run Calibration></td></tr>\n"
	html += "<tr><td colspan=2 align=center>" + nav() + "</td></tr>\n"
	html += "</form></table></font></center></body></html>"
	h.writeResponse(w, []byte(html), "text/html")
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

	html := "<html><head><title>Thermometer Calibration</title></head><body><center>"
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
	html += "</body></html>"
	h.writeResponse(w, []byte(html), "text/html")
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

func processStringUpdate(r *http.Request, formname string, ptr *string) bool {
	value := getFormValue(r, formname, *ptr)
	if value != *ptr {
		Debug("Updating value for %s from %s to %s", formname, *ptr, value)
		*ptr = value
		return true
	}
	Debug("No update to %s, value(%s) orig(%s)", formname, value, *ptr)
	return false
}

func processBoolUpdate(r *http.Request, formname string, ptr *bool) bool {
	value := false
	strvalue := getFormValue(r, formname, "false")
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
	checkbox := "type=checkbox value=on"
	checked := ""
	if value {
		checked = " checked"
	}
	return h.configRow(name, inputName, "", checkbox+checked)
}

func (h *Handler) configRow(name, inputName, configValue, extraArgs string) string {
	return fmt.Sprintf(
		"<tr><td align=right>%s:</td><td><font size=-1><input name=\"%s\" size=20 %s></font></td><td>%s</td></tr>\n",
		name, inputName, extraArgs, configValue)
}

func (h *Handler) configHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: move this to a form on the page.
	w.Header().Set("WWW-Authenticate", "Basic") //  realm=\"Bonnie Labs\"
	if !h.Authenticate(r) {
		http.Error(w, "Unauthorized", 401)
		return
	}
	foundone := false
	c := h.ppc.config

	pw := getFormValue(r, "passcode", "")
	if pw1 := getFormValue(r, "passcode2", ""); pw != "" && pw1 != "" && pw == pw1 {
		c.SetAuth(pw1)
		foundone = true
	}
	if processStringUpdate(r, "appid", &c.cfg.WeatherUndergroundAppID) {
		foundone = true
	}
	if processStringUpdate(r, "zipcode", &c.cfg.Zip) {
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

	passArgs := " type=\"password\" autocomplete=\"new-password\""

	html := "<html><head><title>Pool Controller Configuration</title></head><body>"
	html += "<center><font face=helvetica color=#444444>Pool Controller Configuration"
	html += "<font size=-1>\n"
	html += "<table border=0 cellpadding=3>\n"
	html += "<form action=/config method=POST>\n"
	html += "<tr><th align=left>Administrator:</th><td colspan=3></td></tr>\n"
	html += h.configRow("Admin Password", "passcode", "", passArgs)
	html += h.configRow("Confirm Password", "passcode2", "", passArgs)
	html += "<tr><td colspan=3><br></td></tr>\n"

	html += "<tr><th align=left>Weather:</th><td colspan=3></td></tr>\n"
	html += h.configRow("Zipcode", "zipcode", c.cfg.Zip, "")
	html += h.configRow("WeatherUnderground ID", "appid", c.cfg.WeatherUndergroundAppID, "")
	html += "<tr><td colspan=3><br></td></tr>\n"

	html += "<tr><th align=left>Temperature Sensor Adjustment:</th><td colspan=3></td></tr>\n"
	html += h.configRow("Pump Tuning", "adj_pump", fmt.Sprintf("%0.2f", c.cfg.PumpAdjustment), "")
	html += h.configRow("Roof Tuning", "adj_roof", fmt.Sprintf("%0.2f", c.cfg.RoofAdjustment), "")
	html += "<tr><td colspan=3><br></td></tr>\n"

	html += "<tr><th align=left>Solar Settings:</th><td colspan=3></td></tr>\n"
	html += h.configRow("Target", "target", fmt.Sprintf("%0.2f&deg;C", c.cfg.Target), "")
	html += h.configRow("Tolerance", "tolerance", fmt.Sprintf("%0.2f&deg;C", c.cfg.Tolerance), "")
	html += h.configRow("MinDelta", "mindelta", fmt.Sprintf("%0.2f&deg;C", c.cfg.DeltaT), "")
	html += "<tr><td colspan=3><br></td></tr>\n"

	html += "<tr><th align=left>Debug Settings:</th><td colspan=3></td></tr>\n"
	html += h.configBoolRow("Debug Logging Enabled", "debug", __debug__)
	html += h.configBoolRow("Disable all pumps", "disabled", c.cfg.Disabled)
	html += h.configBoolRow("Disable button", "button_disabled", c.cfg.ButtonDisabled)
	html += h.configBoolRow("Disable solar", "solar_disabled", c.cfg.SolarDisabled)

	html += "<input type=hidden name=posted value=true>\n"
	html += "</table><input type=submit value=Save></font></font></form>\n"
	html += nav()
	html += "</center></body></html>\n"

	h.writeResponse(w, []byte(html), "text/html")
}
