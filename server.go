package main

import (
	"fmt"
	"net/http"
	"time"
)

type Handler struct {
	tempRrd       *Rrd
	pumpRrd       *Rrd
	config        *Config
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
			tempRrd:      ppc.tempRrd,
			pumpRrd:      ppc.pumpRrd,
			config:       ppc.config,
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
	Info("Received: %s", r.URL)
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

func (h *Handler) writeResponse(w http.ResponseWriter, content []byte, ctype string) {
	w.Header().Set("Content-Type", ctype)
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func (h *Handler) configHandler(w http.ResponseWriter, r *http.Request) {
	h.writeResponse(w, []byte("Please implement me"), "text/plain")
}

func duration(r *http.Request) time.Duration {
	var num int
	var let string
	fmt.Sscanf(r.FormValue("scale"), "%d%s", &num, &let)
	d := time.Duration(num)
	day := time.Hour * 24

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
	return day
}

func (h *Handler) graphHandler(w http.ResponseWriter, r *http.Request, which int) {
	var err error
	var graph []byte
	end := time.Now()
	start := end.Add(-1 * duration(r))
	if which == PumpImage {
		_, graph, err = h.pumpRrd.Grapher().Graph(start, end)
	} else if which == TempImage {
		_, graph, err = h.tempRrd.Grapher().Graph(start, end)
	} else {
		http.Error(w, "Unknown Graph", 404)
		return
	}
	if err != nil {
		Error("Could not produce graph: %s", err.Error())
	}
	h.writeResponse(w, graph, "image/png")
}

func (h *Handler) rootHandler(w http.ResponseWriter, r *http.Request) {
	html := `<html>
  <head><title>Pool Pump Controller</title><meta http-equiv="refresh" content="10"></head>
  <body>
    <center>
      <table>
        <tr><td></td><td><IMG src="temps"></td></tr>
        <tr><td></td><td><br></td></tr>
        <tr><td><table>
                <tr><td>4</td><td>Solar Mixing</td></tr>
                <tr><td>3</td><td>Solar</td></tr>
                <tr><td>2</td><td>Sweep</td></tr>
                <tr><td>1</td><td>Pump</td></tr>
                <tr><td>0</td><td>Off</td></tr>
                <tr><td>-1</td><td>Disabled</td></tr>
            </table></td>
            <td><IMG SRC="pumps"></td></tr>
        </table>
    </center>
  </body>
</html>`
	h.writeResponse(w, []byte(html), "text/html")
}
