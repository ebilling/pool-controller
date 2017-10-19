package main

import (
	"github.com/brutella/hc"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		Fatal("Usage: pool-controller CONFIG")
	}
	if err := GpioInit(); err != nil {
		Fatal("Could not initialize GPIO: %s", err.Error())
	}
	config := NewConfig(os.Args[1])
	ppc := NewPoolPumpController(config)
	ppc.Start()

	hcConfig := hc.Config{
		Pin: 	     config.GetString("homekit.pin"),
		StoragePath: config.GetString("homekit.data"),
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

	Info("Homekit Pin: %s", hcConfig.Pin)
	transport.Start()
	Info("Exiting")
}
