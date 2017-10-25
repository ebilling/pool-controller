package main

import (
	"flag"
	"fmt"
	"github.com/brutella/hc"
	"os"
	"runtime"
)

func main() {
	if runtime.GOOS == "darwin" {
		StartTestMode()
	}
	help := flag.Bool("h", default_forceRrd, "Display this usage message")
	config := NewConfig()
	Debug("Found %d flags in: %s", flag.NFlag(), os.Args)
	config.OverwriteWithSaved() // Recover saved values, delete conf to clean them
	if *help {
		flag.Usage()
		fmt.Fprintf(os.Stderr, "Any changes put into the web interface will override these"+
			"flags.\nThe config is stored in %s%s.  It can be carefully edited by hand",
			config.data_dir, server_conf)
		os.Exit(1)
	}

	if err := GpioInit(); err != nil {
		Fatal("Could not initialize GPIO: %s", err.Error())
	}

	PowerLed := NewGpio(5)
	PowerLed.Output(High)
	ppc := NewPoolPumpController(config)
	ppc.Start()

	server := NewServer(9443, ppc)
	server.Start(*config.ssl_cert, *config.ssl_key)

	hcConfig := hc.Config{
		Pin:         *config.pin,
		StoragePath: *config.data_dir,
	}

	transport, err := hc.NewIPTransport(
		hcConfig,
		ppc.runningTemp.Accessory(),
		ppc.pumpTemp.Accessory(),
		ppc.roofTemp.Accessory(),
		ppc.switches.pump.Accessory(),
		ppc.switches.sweep.Accessory(),
		ppc.switches.solar.Accessory())

	if err != nil {
		Fatal("Could not start IP Transport: %s", err.Error())
	}

	hc.OnTermination(func() {
		Info("Stopping Controller")
		ppc.Stop()
		Info("Stopping Transport")
		transport.Stop()
		Info("Stopping Server")
		server.Stop()
		Info("All services sent shutdown signal!")
	})
	Info("Homekit Pin: %s", hcConfig.Pin)
	transport.Start()
	Info("Exiting")
	PowerLed.Output(Low)
}
