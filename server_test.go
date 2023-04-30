package main

import (
	"flag"
	"testing"
)

func TestStartTLS(t *testing.T) {
	SetGpioProvider(NewTestPin)
	flags := flag.NewFlagSet("ServerTest", flag.PanicOnError)
	args := []string{}
	config := NewConfig(flags, args)
	ppc := NewPoolPumpController(config)
	server := NewServer(LocalHost, 8887, ppc)
	server.Start(*config.sslCertificate, *config.sslPrivateKey)
}
