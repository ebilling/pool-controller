package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

// Handler will handle the http requests
type Handler struct {
	ppc *PoolPumpController

	pairingMu    sync.RWMutex
	pairingURI   string
	pairingReset func() (int, error)
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
		Addr:              addr,
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
	s.server.ErrorLog = NewHTTPErrorLogger()
	return &s
}

// SetPairingURI publishes the HomeKit setup payload (X-HM://...) that the Home
// app expects to find in a pairing QR code. The bare setup code is not a valid
// payload, so without this the Home app cannot recognize the image at all.
func (s *Server) SetPairingURI(uri string) {
	s.handler.pairingMu.Lock()
	defer s.handler.pairingMu.Unlock()
	s.handler.pairingURI = uri
}

func (h *Handler) setupPayload() string {
	h.pairingMu.RLock()
	defer h.pairingMu.RUnlock()
	return h.pairingURI
}

// SetPairingReset installs the action behind the pairing page's reset button.
// It must forget stored controller pairings and make the running transport
// re-advertise itself as discoverable.
func (s *Server) SetPairingReset(reset func() (int, error)) {
	s.handler.pairingMu.Lock()
	defer s.handler.pairingMu.Unlock()
	s.handler.pairingReset = reset
}

func (h *Handler) pairingResetter() func() (int, error) {
	h.pairingMu.RLock()
	defer h.pairingMu.RUnlock()
	return h.pairingReset
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
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src 'self' data:")
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	Debug("Received: %s", r.URL)
	switch r.URL.Path {
	case "/":
		if !allowMethods(w, r, http.MethodGet, http.MethodPost) {
			return
		}
		h.rootHandler(w, r)
		return
	case "/pair":
		if !allowMethods(w, r, http.MethodGet) || !h.requireAuth(w, r) {
			return
		}
		h.pairHandler(w, r)
		return
	case "/qr":
		if !allowMethods(w, r, http.MethodGet) || !h.requireAuth(w, r) {
			return
		}
		h.qrHandler(w, r)
		return
	case "/qr.svg":
		if !allowMethods(w, r, http.MethodGet) || !h.requireAuth(w, r) {
			return
		}
		h.qrSVGHandler(w, r)
		return
	case "/pumps":
		if !allowMethods(w, r, http.MethodGet) {
			return
		}
		h.graphHandler(w, r, PumpImage)
		return
	case "/temps":
		if !allowMethods(w, r, http.MethodGet) {
			return
		}
		h.graphHandler(w, r, TempImage)
		return
	case "/status":
		if !allowMethods(w, r, http.MethodGet) {
			return
		}
		h.statusHandler(w, r)
		return
	case "/config":
		if !allowMethods(w, r, http.MethodGet, http.MethodPost) || !h.requireAuth(w, r) {
			return
		}
		h.configHandler(w, r)
		return
	case "/runCalibration":
		if !allowMethods(w, r, http.MethodPost) || !h.requireAuth(w, r) {
			return
		}
		h.runCalibrationHandler(w, r)
		return
	case "/resetPairings":
		if !allowMethods(w, r, http.MethodPost) || !h.requireAuth(w, r) {
			return
		}
		h.resetPairingsHandler(w, r)
		return
	case "/calibrate":
		if !allowMethods(w, r, http.MethodGet) || !h.requireAuth(w, r) {
			return
		}
		h.calibrateHandler(w, r)
		return
	default:
		http.Error(w, "Unknown request type", 404)
	}
}

func allowMethods(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	for _, method := range methods {
		if r.Method == method {
			return true
		}
	}
	for _, method := range methods {
		w.Header().Add("Allow", method)
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	return false
}

func (h *Handler) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if h.Authenticate(r) {
		return true
	}
	w.Header().Set("WWW-Authenticate", `Basic realm="pool-controller", charset="UTF-8"`)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	return false
}

func (h *Handler) setRefresh(w http.ResponseWriter, r *http.Request, seconds int) {
	refresh := fmt.Sprintf("%d; url=%s", seconds, r.RequestURI)
	Debug("Setting Refresh to: %s", refresh)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Refresh", refresh)
}

func (h *Handler) writeResponse(w http.ResponseWriter, content []byte, ctype string) {
	w.Header().Set("Content-Type", ctype)
	if w.Header().Get("Cache-Control") == "" {
		// Every page here reports live state, down to whether HomeKit is
		// paired, so a copy the browser kept is a wrong answer.
		w.Header().Set("Cache-Control", "no-store")
	}
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
		h.ppc.pumpRrd.mu.Lock()
		defer h.ppc.pumpRrd.mu.Unlock()
		h.ppc.pumpRrd.Grapher().SetSize(uint(width), uint(height))
		_, graph, err = h.ppc.pumpRrd.Grapher().Graph(start, end)
	} else if which == TempImage {
		h.ppc.tempRrd.mu.Lock()
		defer h.ppc.tempRrd.mu.Unlock()
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
	TargetC    float64 `json:"target_c"`
	PoolC      float64 `json:"pool_c"`
	RoofC      float64 `json:"roof_c"`
	TargetF    float64 `json:"target_f"`
	PoolF      float64 `json:"pool_f"`
	RoofF      float64 `json:"roof_f"`
	SensorsOK  bool    `json:"sensors_ok"`
	SensorInfo string  `json:"sensor_info"`
	Updated    string  `json:"updated"`
}

// sensorStatus describes a probe for the status page. It flags a stale reading
// sooner than the control policy acts on one, so a probe going bad shows up
// here before it changes what the pumps do.
func sensorStatus(t Thermometer, now time.Time) (bool, string) {
	updated, err := t.ReadingStatus()
	if err != nil {
		return false, err.Error()
	}
	if now.Sub(updated) > time.Minute {
		return false, "reading is stale"
	}
	return true, "OK"
}

func (h *Handler) liveStatus() liveStatus {
	h.ppc.mu.RLock()
	defer h.ppc.mu.RUnlock()
	control := "Auto"
	if h.ppc.switches.ManualState(h.ppc.config.cfg.ManualRunTime) {
		control = "Manual"
	}
	mode := thermostatModeName(h.ppc.config.cfg)
	now := time.Now()
	pumpOK, pumpInfo := sensorStatus(h.ppc.pumpTemp, now)
	roofOK, roofInfo := sensorStatus(h.ppc.roofTemp, now)
	sensorInfo := "OK"
	if !pumpOK {
		sensorInfo = "Pump: " + pumpInfo
	} else if !roofOK {
		sensorInfo = "Roof: " + roofInfo
	}
	return liveStatus{
		Pump:       h.ppc.switches.State().String(),
		PumpOn:     h.ppc.switches.State() > OFF,
		Solar:      h.ppc.switches.solar.Status(),
		SolarOn:    h.ppc.switches.solar.Status() == "On",
		Thermostat: mode,
		Control:    control,
		TargetC:    h.ppc.config.cfg.Target,
		PoolC:      h.ppc.runningTemp.Temperature(),
		RoofC:      h.ppc.roofTemp.Temperature(),
		TargetF:    toFarenheit(h.ppc.config.cfg.Target),
		PoolF:      toFarenheit(h.ppc.runningTemp.Temperature()),
		RoofF:      toFarenheit(h.ppc.roofTemp.Temperature()),
		SensorsOK:  pumpOK && roofOK,
		SensorInfo: sensorInfo,
		Updated:    now.Format("2006-01-02 15:04:05"),
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
	return formatSetupCode(h.ppc.config.cfg.Pin)
}

func (h *Handler) pairHandler(w http.ResponseWriter, r *http.Request) {
	h.ppc.mu.RLock()
	defer h.ppc.mu.RUnlock()
	h.writeResponse(w, []byte(page("HomeKit pairing", h.pairBody(""))), "text/html")
}

func (h *Handler) pairBody(notice string) string {
	body := `<div class="card">
<h2>HomeKit pairing</h2>`
	if payload := h.setupPayload(); payload != "" {
		body += `
<p class="lead">Scan this code in the Home app, or tap Add Accessory and enter it by hand.</p>` +
			setupLabelHTML(h.pin(), payload)
	} else {
		body += `
<p class="pin">` + html.EscapeString(h.pin()) + `</p>
<p class="legend">HomeKit has not reported a setup payload yet, so there is no code
to scan. Enter the digits above by hand in the Home app.</p>`
	}
	if notice != "" {
		body += "\n" + notice
	}
	return body + h.pairingStateHTML() + `
</div>`
}

// setupLabelHTML renders the setup code the way Apple prints it: the scannable
// code above the digits, so either one can be used from the same label.
func setupLabelHTML(code, payload string) string {
	return `
<div class="setup-label">
<svg class="setup-glyph" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3 2.5 10.6V21h7v-5.5h5V21h7V10.6z"/></svg>
<img class="setup-qr" src="/qr.svg" width="220" height="220" alt="HomeKit setup code">
<p class="setup-code">` + html.EscapeString(code) + `</p>
</div>
<p class="legend"><a class="setup-link" href="` + html.EscapeString(payload) + `">Open in the Home app</a>
if you are reading this page on the iPhone you want to pair.</p>
<p class="legend">Setup payload: <code>` + html.EscapeString(payload) + `</code></p>`
}

func (h *Handler) pairingStateHTML() string {
	controllers, err := controllerPairings(*h.ppc.config.dataDirectory)
	if err != nil {
		return `
<p class="legend">Could not read pairing state: ` + html.EscapeString(err.Error()) + `</p>`
	}
	if len(controllers) == 0 {
		return `
<p class="legend">No controllers are paired, so this accessory is discoverable.</p>`
	}
	return fmt.Sprintf(`
<p class="legend">Paired with %d controller(s), so HomeKit advertises this accessory as
not discoverable and new setup attempts will hang. If you already removed it from the
Home app, forget the pairings to make it discoverable again. That restarts the
controller, which briefly stops the pumps.</p>
<form class="stack" action="/resetPairings" method="POST">
<input type="submit" value="Forget paired controllers">
</form>`, len(controllers))
}

func (h *Handler) resetPairingsHandler(w http.ResponseWriter, r *http.Request) {
	var notice string
	switch reset := h.pairingResetter(); {
	case reset == nil:
		notice = `<p class="legend">Pairing reset is unavailable: the HomeKit transport is not running.</p>`
	default:
		removed, err := reset()
		if err != nil {
			Error("Could not reset HomeKit pairings: %s", err.Error())
			notice = `<p class="legend">Could not forget pairings: ` +
				html.EscapeString(err.Error()) + `</p>`
		} else {
			Alert("Forgot %d HomeKit controller pairing(s) requested from the web interface", removed)
			notice = fmt.Sprintf(`<p class="legend">Forgot %d pairing(s) and restarted the
controller so it advertises itself again. Give it a few seconds, then add it in the
Home app using the code above.</p>`, removed)
		}
	}
	h.ppc.mu.RLock()
	defer h.ppc.mu.RUnlock()
	h.writeResponse(w, []byte(page("HomeKit pairing", h.pairBody(notice))), "text/html")
}

func (h *Handler) qrHandler(w http.ResponseWriter, r *http.Request) {
	payload := h.setupPayload()
	if payload == "" {
		http.Error(w, "Pairing payload unavailable", http.StatusServiceUnavailable)
		return
	}
	png, err := qrcode.Encode(payload, qrcode.Medium, 512)
	if err != nil {
		http.Error(w, "Could not generate pairing code", http.StatusInternalServerError)
		return
	}
	h.writeResponse(w, []byte(png), "image/png")
}

// qrSVGHandler serves the setup code as vector art, which stays sharp at the
// size the pairing page draws it and when a phone camera zooms into it.
func (h *Handler) qrSVGHandler(w http.ResponseWriter, r *http.Request) {
	payload := h.setupPayload()
	if payload == "" {
		http.Error(w, "Pairing payload unavailable", http.StatusServiceUnavailable)
		return
	}
	svg, err := qrCodeSVG(payload)
	if err != nil {
		http.Error(w, "Could not generate pairing code", http.StatusInternalServerError)
		return
	}
	h.writeResponse(w, svg, "image/svg+xml")
}

// qrCodeSVG draws the QR modules as a single path, one unit per module, and
// leaves scaling to the viewBox.
func qrCodeSVG(payload string) ([]byte, error) {
	code, err := qrcode.New(payload, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	bitmap := code.Bitmap()
	var svg strings.Builder
	fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" `+
		`shape-rendering="crispEdges" role="img" aria-label="HomeKit setup code">`,
		len(bitmap), len(bitmap))
	svg.WriteString(`<rect width="100%" height="100%" fill="#ffffff"/><path fill="#000000" d="`)
	for y, row := range bitmap {
		for x, dark := range row {
			if dark {
				fmt.Fprintf(&svg, "M%d %dh1v1h-1z", x, y)
			}
		}
	}
	svg.WriteString(`"/></svg>`)
	return []byte(svg.String()), nil
}

func (h *Handler) rootHandler(w http.ResponseWriter, r *http.Request) {
	h.ppc.mu.RLock()
	defer h.ppc.mu.RUnlock()
	scale := getscale(r)
	cookie := &http.Cookie{
		Name:   "scale",
		Value:  scale,
		MaxAge: int(365 * 24 * time.Hour / time.Second),
	}
	http.SetCookie(w, cookie)
	controlStr := "Auto"
	if h.ppc.switches.ManualState(h.ppc.config.cfg.ManualRunTime) {
		controlStr = "Manual"
	}
	thermostatMode := thermostatModeName(h.ppc.config.cfg)
	pumpOn := h.ppc.switches.State() > OFF
	solarOn := h.ppc.switches.solar.Status() == "On"
	pumpSensorOK, pumpSensorInfo := sensorStatus(h.ppc.pumpTemp, time.Now())
	roofSensorOK, roofSensorInfo := sensorStatus(h.ppc.roofTemp, time.Now())
	sensorInfo := "OK"
	if !pumpSensorOK {
		sensorInfo = "Pump: " + pumpSensorInfo
	} else if !roofSensorOK {
		sensorInfo = "Roof: " + roofSensorInfo
	}
	unit := configuredTemperatureUnit(h.ppc.config.cfg)
	celsiusSelected, fahrenheitSelected := "", ""
	if unit == TemperatureFahrenheit {
		fahrenheitSelected = " selected"
	} else {
		celsiusSelected = " selected"
	}

	body := `<div class="pills">` +
		statusPill("pill-pump", "Pump", h.ppc.switches.State().String(), pumpOn) +
		statusPill("pill-solar", "Solar", h.ppc.switches.solar.Status(), solarOn) +
		statusPill("pill-thermostat", "Thermostat", thermostatMode, thermostatMode != "Off") +
		statusPill("pill-control", "Control", controlStr, controlStr == "Manual") +
		statusPill("pill-sensors", "Sensors", sensorInfo, pumpSensorOK && roofSensorOK) +
		`</div>
<div class="metrics">` +
		metricCard("metric-target", "Target", fmt.Sprintf("%0.1f °%s",
			temperatureForDisplay(h.ppc.config.cfg.Target, unit, false), unit)) +
		metricCard("metric-pool", "Pool", fmt.Sprintf("%0.1f °%s",
			temperatureForDisplay(h.ppc.runningTemp.Temperature(), unit, false), unit)) +
		metricCard("metric-roof", "Roof", fmt.Sprintf("%0.1f °%s",
			temperatureForDisplay(h.ppc.roofTemp.Temperature(), unit, false), unit)) +
		`</div>
<form class="toolbar" action="/" method="POST">
<label>Window <input name="scale" value="` + html.EscapeString(scale) + `" placeholder="12h"></label>
<label>Readouts <select id="status-temperature-unit" aria-label="Temperature unit">
<option value="C"` + celsiusSelected + `>°C</option>
<option value="F"` + fahrenheitSelected + `>°F</option>
</select></label>
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
	h.ppc.mu.Lock()
	defer h.ppc.mu.Unlock()
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
				html += fmt.Sprintf("<br>Pool Value: %0.3f", p.Adjustment())
			}
			p, ok = h.ppc.roofTemp.(*GpioThermometer)
			if ok {
				html += fmt.Sprintf("<br>Roof Value: %0.3f", p.Adjustment())
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

func processThermostatModeUpdate(r *http.Request, cfg *PersistedConfig) bool {
	value := ThermostatMode(getFormValue(r, "thermostat_mode", ""))
	switch value {
	case ThermostatOff, ThermostatHeat, ThermostatCool, ThermostatAuto:
	default:
		return false
	}
	if cfg.ThermostatMode == value {
		return false
	}
	cfg.ThermostatMode = value
	return true
}

func processTemperatureUnitUpdate(r *http.Request, cfg *PersistedConfig) bool {
	value := TemperatureUnit(getFormValue(r, "temperature_unit", string(configuredTemperatureUnit(cfg))))
	switch value {
	case TemperatureCelsius, TemperatureFahrenheit:
	default:
		return false
	}
	if cfg.TemperatureUnit == value {
		return false
	}
	cfg.TemperatureUnit = value
	return true
}

func temperatureForDisplay(celsius float64, unit TemperatureUnit, delta bool) float64 {
	if unit != TemperatureFahrenheit {
		return celsius
	}
	if delta {
		return celsius * 9 / 5
	}
	return toFarenheit(celsius)
}

func processTemperatureUpdate(r *http.Request, formname string, ptr *float64, unit TemperatureUnit, delta bool) bool {
	value := getFormValue(r, formname, "")
	if value == fmt.Sprintf("%0.2f", temperatureForDisplay(*ptr, unit, delta)) {
		return false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return false
	}
	if unit == TemperatureFahrenheit {
		if delta {
			parsed *= 5.0 / 9.0
		} else {
			parsed = (parsed - 32) * 5.0 / 9.0
		}
	}
	*ptr = parsed
	return true
}

func (h *Handler) configBoolRow(name, inputName string, value bool) string {
	checked := ""
	if value {
		checked = " checked"
	}
	return fmt.Sprintf(
		`<input type="hidden" name="_present_%s" value="true"><label class="check"><input type="checkbox" name="%s" value="true"%s> %s</label>`+"\n",
		html.EscapeString(inputName), html.EscapeString(inputName), checked, html.EscapeString(name))
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
	if processTemperatureUnitUpdate(r, c.cfg) {
		foundone = true
	}
	if processTemperatureUpdate(r, "target", &c.cfg.Target, c.cfg.TemperatureUnit, false) {
		foundone = true
	}
	if processTemperatureUpdate(r, "tolerance", &c.cfg.Tolerance, c.cfg.TemperatureUnit, true) {
		foundone = true
	}
	if processTemperatureUpdate(r, "mindelta", &c.cfg.DeltaT, c.cfg.TemperatureUnit, true) {
		foundone = true
	}
	if getFormValue(r, "_present_disabled", "") == "true" &&
		processBoolUpdate(r, "disabled", &c.cfg.Disabled) {
		foundone = true
	}
	if getFormValue(r, "_present_button_disabled", "") == "true" &&
		processBoolUpdate(r, "button_disabled", &c.cfg.ButtonDisabled) {
		foundone = true
	}
	if getFormValue(r, "_present_solar_disabled", "") == "true" &&
		processBoolUpdate(r, "solar_disabled", &c.cfg.SolarDisabled) {
		foundone = true
	}
	if processThermostatModeUpdate(r, c.cfg) {
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

	// Don't persist this one. The checkbox submits the value the form gives
	// it, "true", and not the "on" a browser sends for a checkbox with no
	// value of its own.
	posted := getFormValue(r, "posted", "")
	if posted == "true" && getFormValue(r, "_present_debug", "") == "true" {
		if getFormValue(r, "debug", "false") == "true" {
			EnableDebug()
			Info("Debug logging is on")
		} else {
			Info("Debug logging is off")
			DisableDebug()
		}
	}
}

func (h *Handler) configHandler(w http.ResponseWriter, r *http.Request) {
	h.ppc.mu.Lock()
	defer h.ppc.mu.Unlock()
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
	mode := configuredThermostatMode(c.cfg)
	modeOption := func(value ThermostatMode, label string) string {
		selected := ""
		if mode == value {
			selected = " selected"
		}
		return fmt.Sprintf(`<option value="%s"%s>%s</option>`, value, selected, label)
	}
	lastCleaning := "No qualifying sweep recorded yet"
	if !c.cfg.LastCleaningCompleted.IsZero() {
		lastCleaning = c.cfg.LastCleaningCompleted.Format("2006-01-02 15:04 MST")
	}
	unit := configuredTemperatureUnit(c.cfg)
	unitOption := func(value TemperatureUnit, label string) string {
		selected := ""
		if unit == value {
			selected = " selected"
		}
		return fmt.Sprintf(`<option value="%s"%s>%s</option>`, value, selected, label)
	}
	unitLabel := string(unit)

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
<label>Thermostat mode<select name="thermostat_mode">
` + modeOption(ThermostatOff, "Off") +
		modeOption(ThermostatHeat, "Heat only") +
		modeOption(ThermostatCool, "Cool only") +
		modeOption(ThermostatAuto, "Auto") + `
</select></label>
<label>Temperature unit<select name="temperature_unit" id="temperature-unit">
` + unitOption(TemperatureCelsius, "Celsius (°C)") +
		unitOption(TemperatureFahrenheit, "Fahrenheit (°F)") + `
</select></label>
` + h.configRow("Target (°"+unitLabel+")", "target", fmt.Sprintf("%0.2f", temperatureForDisplay(c.cfg.Target, unit, false)), "") + `
` + h.configRow("Tolerance (°"+unitLabel+")", "tolerance", fmt.Sprintf("%0.2f", temperatureForDisplay(c.cfg.Tolerance, unit, true)), "") + `
` + h.configRow("Min delta (°"+unitLabel+")", "mindelta", fmt.Sprintf("%0.2f", temperatureForDisplay(c.cfg.DeltaT, unit, true)), "") + `
</fieldset>
<fieldset>
<legend>Cleaning / Sweep</legend>
` + h.configRow("Cleaning frequency (days)", "daily_freq", fmt.Sprintf("%0.2f", c.cfg.DailyFrequency), "") + `
` + h.configRow("Required continuous sweep runtime (hours)", "run_time", fmt.Sprintf("%0.2f", c.cfg.RunTime), "") + `
<p>Last qualifying sweep: ` + html.EscapeString(lastCleaning) + `</p>
</fieldset>
<fieldset>
<legend>Debug and disables</legend>
` + h.configBoolRow("Debug logging", "debug", doDebug) + `
` + h.configBoolRow("Disable all pumps", "disabled", c.cfg.Disabled) + `
` + h.configBoolRow("Disable button", "button_disabled", c.cfg.ButtonDisabled) + `
` + h.configBoolRow("Disable solar", "solar_disabled", c.cfg.SolarDisabled) + `
</fieldset>
<input type="hidden" name="posted" value="true">
<input type="submit" value="Save">
</form>
<script>
(function () {
  var unit = document.getElementById("temperature-unit");
  var previous = unit.value;
  unit.addEventListener("change", function () {
    ["target", "tolerance", "mindelta"].forEach(function (name) {
      var input = document.querySelector('input[name="' + name + '"]');
      var value = parseFloat(input.value);
      if (!Number.isFinite(value)) return;
      var delta = name !== "target";
      if (previous === "C" && unit.value === "F") {
        value = delta ? value * 9 / 5 : value * 9 / 5 + 32;
      } else if (previous === "F" && unit.value === "C") {
        value = delta ? value * 5 / 9 : (value - 32) * 5 / 9;
      }
      input.value = value.toFixed(2);
      input.parentElement.firstChild.textContent =
        input.parentElement.firstChild.textContent.replace(/°[CF]/, "°" + unit.value);
    });
    previous = unit.value;
  });
})();
</script>`

	h.writeResponse(w, []byte(page("Configuration", body)), "text/html")
}
