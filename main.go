package main

import (
	"github.com/brutella/hc"
	"os"
	"pool-controller"
)

func main() {
	if len(os.Args) < 2 {
		log.fatal("Usage: pi-go-homekit CONFIG")
	}
	ppc := NewPoolPumpController(os.Args[1])
	ppc.Start()

	transport, err := hc.NewIPTransport(hc.Config{Pin: ppc.pin},
		ppc.thermometer.Accessory,
		ppc.pump.Accessory,
		ppc.sweep.Accessory)

	if err != nil {
		log.fatal(err)
	}

	hc.OnTermination(func() {
		ppc.Stop()
		transport.Stop()
	})

	transport.Start()
	log.info("Exiting")
}
