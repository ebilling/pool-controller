package main

import (
	"flag"
	"net/http"
	"testing"
)

func handler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("This is an example server.\n"))
}

func TestStartTLS(t *testing.T) {
	SetGpioProvider(NewTestPin)
	flags := flag.NewFlagSet("ServerTest", flag.PanicOnError)
	args := []string{}
	config := NewConfig(flags, args)
	ppc := NewPoolPumpController(config)
	server := NewServer(LocalHost, 8887, ppc)
	server.Start(*config.sslCertificate, *config.sslPrivateKey)
}
