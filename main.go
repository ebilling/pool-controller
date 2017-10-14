package main

import (
	"github.com/brutella/hc"
	"time"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		Fatal("Usage: pi-go-homekit CONFIG")
	}
	ppc := NewPoolPumpController(os.Args[1])
	ppc.Start()
	hcConfig := hc.Config{
		Pin: ppc.pin,
		StoragePath: "/var/cache/homekit",
	}
	for ppc.waterTemp == nil || ppc.roofTemp == nil {
		Info("Waiting for temperatures to be recorded")
		time.Sleep(5 * time.Second)
	}
	transport, err := hc.NewIPTransport(
		hcConfig,
		ppc.pump.Accessory,
		ppc.sweep.Accessory,
		ppc.waterTemp.acc.Accessory,
		ppc.roofTemp.acc.Accessory)


	if err != nil {
		Fatal("Could not start IP Transport: %s", err.Error())
	}

	hc.OnTermination(func() {
		ppc.Stop()
		transport.Stop()
	})

	transport.Start()
	Info("Exiting")
}
