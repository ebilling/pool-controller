package main

import (
	"github.com/brutella/hc"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		Fatal("Usage: pool-controller CONFIG")
	}
	ppc := NewPoolPumpController(os.Args[1])
	ppc.Update()
	ppc.Start()
	hcConfig := hc.Config{
		Pin: ppc.pin,
		StoragePath: "/var/cache/homekit",
	}
	transport, err := hc.NewIPTransport(
		hcConfig,
		ppc.switches.pump.Accessory(),
		ppc.switches.sweep.Accessory(),
		ppc.switches.solar.Accessory(),
		ppc.waterTemp.Accessory(),
		ppc.runningTemp.Accessory(),
		ppc.roofTemp.Accessory())


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
