package main

import (
	"golang.org/x/crypto/bcrypt"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Handler struct {
	ppc    *PoolPumpController
}

// A TLS server for the pool-controller
type Server struct {
	port       int
	handler    *Handler
	server     http.Server
}

func NewServer(port int, ppc *PoolPumpController) (*Server) {
	s := Server {
		port:        port,
		handler:     &Handler{
			ppc:      ppc,
		},
	}
	s.server =  http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      s.handler,
	}
	return &s
}

func startServer(s *Server) {
	err := s.server.ListenAndServeTLS(
		"/etc/ssl/certs/pool-controller.crt",
		"/etc/ssl/private/pool-controller.key")
	if err != nil {
		Error("Error from Server: %s", err.Error())
	}
}

func (s *Server) Start() {
	go startServer(s)
	Info("Starting HTTPS on 0.0.0.0:%d", s.port)
}

func (s *Server) Stop() {
	s.server.Close()
	// TODO: Maybe someday implement http.Server.Shutdown
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
	if cookie != nil { scale = cookie.Value	}
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
		switch let{
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
	if value == "" { return defaultValue }
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
	return fmt.Sprintf("<img src=\"/%s?scale=%s&width=%d&height=%d\" width=%d height=%d " +
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

func (h *Handler) rootHandler(w http.ResponseWriter, r *http.Request) {
	scale := getscale(r)
	cookie := &http.Cookie{
		Name: "scale",
		Value: scale,
		MaxAge: int(365 * 24 * time.Hour/time.Second),
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
		"size=-1><form action=/ method=POST>Time Window:<input name=scale value=" +
		scale + " size=5> ex. 12h (w, d, h, m)</form></font></td></tr>\n"
        html += indent(1) + "<tr><td>" + image("temps", 640, 300, scale) + "</td>"
	html += "<td align=left nowrap><font face=helvetica color=#444444 size=-1>"
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
		fmt.Sprintf("Updated: %s", time.Now().String()) +
		"</td></tr>\n"
        html += "</table></font></center></body></html>"
	h.writeResponse(w, []byte(html), "text/html")
}

func (h *Handler) Authenticate(r *http.Request) bool {
	user, password, ok := r.BasicAuth()
	if !ok || user != "admin" { return false }
	authhash := h.ppc.config.GetString("auth.hash")
	if authhash == "" {
		defPass := h.ppc.config.GetString("homekit.pin")
		defHash, _ := bcrypt.GenerateFromPassword([]byte(defPass), bcrypt.DefaultCost)
		authhash = string(defHash)
	}
	hashed, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	err := bcrypt.CompareHashAndPassword(hashed, []byte(authhash))
	if err == nil { return true }
	return false
}

func configRow(name, inputName, configName string) string {
	return fmt.Sprintf("<tr><td align=right>%s:</td><input name=%s size=20></td>" +
		"<td>%s</td></tr>\n", name, inputName, configName)
}

func setConfigValue(c *Config, configName, formValue string) bool {
	if formValue == "" { return false }
	c.Set(configName, formValue)
	return true
}

func (h *Handler) configHandler(w http.ResponseWriter, r *http.Request) {
	if !h.Authenticate(r) {
		http.Error(w, "Unauthorized", 401)
		return
	}

	c := h.ppc.config
	foundone := false
	if setConfigValue(c, "auth.hash",      "passcode")  { foundone = true }
	if setConfigValue(c, "weather.zip",    "zipcode")   { foundone = true }
	if setConfigValue(c, "weather.appid",  "appid")     { foundone = true }
	if setConfigValue(c, "temp.tolerance", "tolerance") { foundone = true }
	if setConfigValue(c, "temp.minDeltaT", "mindelta")  { foundone = true }
	if setConfigValue(c, "temp.target",    "target")    { foundone = true }

	if foundone { c.Save() }

	html := "<html><head><title>Pool Controller Configuration</title></head><body>"
	html += "<table cellpadding=3>"
	html += "<form <font face=helvetica color=#444444 size=-1>" +
		"<form action=/config method=POST>"
	html += "<tr><td align=left>Administrator:</td><td colspan=2></td></tr>"
	html += configRow("Admin Password", "passcode", "")
	html += "<tr><td colspan=3><br></td></tr>"

	html += "<tr><td align=left>Weather:</td><td colspan=2></td></tr>"
	html += configRow("Zipcode", "zipcode", c.GetString("weather.zip"))
	html += configRow("WeatherUnderground ID", "appid", c.GetString("weather.appid"))
	html += "<tr><td colspan=3><br></td></tr>"

	html += "<tr><td align=left>Solar Settings:</td><td colspan=2></td></tr>"
	html += configRow("Tolerance", "tolerance", c.GetString("temp.tolerance"))
	html += configRow("MinDelta", "mindelta", c.GetString("temp.minDeltaT"))
	html += configRow("Target", "target", c.GetString("temp.target"))
	html += "</form></table></body></html>"

	h.writeResponse(w, []byte(html), "text/html")
}
