package main

import (
	"github.com/brutella/hc"
	"flag"
)

func main() {
	forceRrd := flag.Bool("f", false, "force creation of new RRD files if present")
	flag.Parse()
	
	if len(flag.Args()) < 1 {
		Fatal("Usage: pool-controller [-f] CONFIG")
	}

	if err := GpioInit(); err != nil {
		Fatal("Could not initialize GPIO: %s", err.Error())
	}
	config := NewConfig(flag.Args()[0])
	ppc := NewPoolPumpController(config)
	ppc.Start(*forceRrd)

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
