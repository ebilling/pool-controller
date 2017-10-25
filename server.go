package main

import (
	"context"
	"fmt"
	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strconv"
	"time"
)

type Handler struct {
	ppc *PoolPumpController
}

// A TLS server for the pool-controller
type Server struct {
	port    int
	handler *Handler
	server  http.Server
}

func NewServer(port int, ppc *PoolPumpController) *Server {
	s := Server{
		port: port,
		handler: &Handler{
			ppc: ppc,
		},
	}
	s.server = http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: s.handler,
	}
	return &s
}

func startServer(s *Server, cert, key string) {
	err := s.server.ListenAndServeTLS(cert, key)
	if err != nil {
		Error("Error from Server: %s", err.Error())
	}
}

func (s *Server) Start(cert, key string) {
	go startServer(s, cert, key)
	Info("Starting HTTPS on 0.0.0.0:%d", s.port)
}

func (s *Server) Stop() {
	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	err := s.server.Shutdown(ctx)
	if err != nil {
		Info("ServerShutdown: %s", err.Error())
	} else {
		Error("Server did not respond to shutdown request")
	}
}

const (
	PumpImage = 0
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
	fmt.Sscanf(*h.ppc.config.pin, "%3s%2s%3s", &p1, &p2, &p3)
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
	png, _ := qrcode.Encode(*h.ppc.config.pin, qrcode.Medium, 256)
	h.writeResponse(w, []byte(png), "image/png")
}

func nav() string {
	out := "<p><font face=helvetica color=#444444 size=-2>"
	out += "<table cellspacing=5><tr><td><a href=/>graphs</a></td><td>&nbsp;</td>\n"
	out += "<td><a href=/pair>homekit</a></td><td>&nbsp;</td>\n"
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

	html := "<html><head><title>Pool Pump Controller</title></head><body><center>" +
		"<table>\n"
	html += indent(1) + "<tr><td colspan=2 align=center><font face=helvetica color=#444444 " +
		"size=-1><form action=/ method=POST>Time Window:<input name=scale value=\"" +
		scale + "\" size=5> ex. 12h (w, d, h, m)</form></font></td></tr>\n"
	html += indent(1) + "<tr><td>" + image("temps", 640, 300, scale) + "</td>"
	html += "<td align=left nowrap><font face=helvetica color=#444444 size=-1>"
	html += fmt.Sprintf("Target: %0.1f F<br>", toFarenheit(*h.ppc.config.target))
	html += fmt.Sprintf("Pool: %0.1f F<br>", toFarenheit(h.ppc.runningTemp.Temperature()))
	html += fmt.Sprintf("Roof: %0.1f F<br>", toFarenheit(h.ppc.roofTemp.Temperature()))
	html += fmt.Sprintf("Weather: %0.1f F<br>",
		toFarenheit(h.ppc.weather.GetCurrentTempC(h.ppc.zipcode)))
	html += fmt.Sprintf("Solar: %0.1f W/sqm", h.ppc.weather.GetSolarRadiation(h.ppc.zipcode))
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
	html += indent(1) + "<tr><td colspan=2 align=center>" +
		fmt.Sprintf("Updated: %.19s", time.Now().String()) +
		"</td></tr>\n"
	html += "</table></font>"
	html += nav()
	html += "</center></body></html>"
	h.writeResponse(w, []byte(html), "text/html")
}

func (h *Handler) Authenticate(r *http.Request) bool {
	user, password, ok := r.BasicAuth()
	if !ok || user != "admin" {
		Error("Unknown user (%s) attempting to configure server", user)
		return false
	}
	err := bcrypt.CompareHashAndPassword(h.ppc.config.GetAuth(), []byte(password))
	if err == nil {
		Debug("User %s logged in", user)
		return true
	}
	Error("Login for User (%s) failed: %s", user, err.Error())
	return false
}

func (h *Handler) configRow(name, inputName, configValue, extraArgs string) string {
	return fmt.Sprintf(
		"<tr><td align=right>%s:</td><td><font size=-1><input name=\"%s\" size=20 %s></font></td><td>%s</td></tr>\n",
		name, inputName, extraArgs, configValue)
}

func processStringUpdate(r *http.Request, formname string, ptr **string) bool {
	if value := getFormValue(r, formname, **ptr); value != "" {
		Debug("Updating value for %s from %s to %s", formname, **ptr, value)
		*ptr = &value
		return true
	}
	return false
}

func processFloatUpdate(r *http.Request, formname string, ptr **float64) bool {
	curvalue := fmt.Sprintf("%0.2f", **ptr)
	value := getFormValue(r, formname, "")
	if value != curvalue {
		flt, err := strconv.ParseFloat(value, 64)
		if err == nil {
			Debug("Updating value for %s from %s to %s", formname, curvalue, value)
			*ptr = &flt
			return true
		}
	}
	Debug("No update to %s, value(%s) orig(%s)", formname, value, curvalue)
	return false
}

func (h *Handler) configHandler(w http.ResponseWriter, r *http.Request) {
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
	if processStringUpdate(r, "appid", &c.WUappId) {
		foundone = true
	}
	if processStringUpdate(r, "zipcode", &c.zip) {
		foundone = true
	}
	if processFloatUpdate(r, "cap_pump", &c.cap_pump) {
		foundone = true
	}
	if processFloatUpdate(r, "cap_roof", &c.cap_roof) {
		foundone = true
	}
	if processFloatUpdate(r, "target", &c.target) {
		foundone = true
	}
	if processFloatUpdate(r, "tolerance", &c.tolerance) {
		foundone = true
	}
	if processFloatUpdate(r, "mindelta", &c.deltaT) {
		foundone = true
	}
	if foundone {
		c.Save()
	}

	passArgs := " type=\"password\" autocomplete=\"new-password\""

	html := "<html><head><title>Pool Controller Configuration</title></head><body>"
	html += "<center><font face=helvetica color=#444444>Pool Controller Configuration"
	html += "<font size=-1>\n"
	html += "<table border=0 cellpadding=3>\n"
	html += "<form action=/config method=POST>\n"
	html += "<tr><td align=left>Administrator:</td><td colspan=3></td></tr>\n"
	html += h.configRow("Admin Password", "passcode", "", passArgs)
	html += h.configRow("Confirm Password", "passcode2", "", passArgs)
	html += "<tr><td colspan=3><br></td></tr>\n"

	html += "<tr><td align=left>Weather:</td><td colspan=3></td></tr>\n"
	html += h.configRow("Zipcode", "zipcode", *c.zip, "")
	html += h.configRow("WeatherUnderground ID", "appid", *c.WUappId, "")
	html += "<tr><td colspan=3><br></td></tr>\n"

	html += "<tr><td align=left>Temperature Sensor Adjustment:</td><td colspan=3></td></tr>\n"
	html += h.configRow("Pump Capacitance (uF)", "cap_pump", fmt.Sprintf("%0.2f&micro;F", *c.cap_pump), "")
	html += h.configRow("Roof Capacitance (uF)", "cap_roof", fmt.Sprintf("%0.2f&micro;F", *c.cap_roof), "")
	html += "<tr><td colspan=3><br></td></tr>\n"

	html += "<tr><td align=left>Solar Settings:</td><td colspan=3></td></tr>\n"
	html += h.configRow("Target", "target", fmt.Sprintf("%0.2f&deg;C", *c.target), "")
	html += h.configRow("Tolerance", "tolerance", fmt.Sprintf("%0.2f&deg;C", *c.tolerance), "")
	html += h.configRow("MinDelta", "mindelta", fmt.Sprintf("%0.2f&deg;C", *c.deltaT), "")
	html += "</table><input type=submit value=Save></font></font></form>\n"
	html += nav()
	html += "</center></body></html>\n"

	h.writeResponse(w, []byte(html), "text/html")
}
