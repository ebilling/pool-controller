package main

import (
	"net/http"
	"testing"
)

func handler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("This is an example server.\n"))
}

func TestStartTLS(t *testing.T) {
	LogTestMode()
	SetGpioProvider(testpin_generator)
	config := NewConfig()
	ppc := NewPoolPumpController(config)
	server := NewServer(8887, ppc)
	server.Start(*config.ssl_cert, *config.ssl_key)

}
