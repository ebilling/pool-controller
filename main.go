package main

import (
	"flag"
	"fmt"
	"github.com/brutella/hc"
	"io/ioutil"
	"os"
)

func main() {
	fs := flag.NewFlagSet("pool-controller", flag.PanicOnError)
	help := fs.Bool("h", false, "Display this usage message")
	config := NewConfig(fs, os.Args[1:]) // Parses flags

	if *help {
		flag.Usage()
		fmt.Fprintf(os.Stderr, "Any changes put into the web interface will override these"+
			"flags.\nThe config is stored in %s%s.  It can be carefully edited by hand",
			*config.dataDirectory, serverConfiguration)
		os.Exit(1)
	}

	// Recover saved values, edit conf to clean them
	Info("Args: %s", os.Args[1:])

	// Write PID
	ioutil.WriteFile(*config.pidfile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)

	if err := GpioInit(); err != nil {
		Fatal("Could not initialize GPIO: %s", err.Error())
	}

	PowerLed := NewGpio(5)
	PowerLed.Output(High)
	ppc := NewPoolPumpController(config)
	ppc.Start()

	server := NewServer(AnyHost, 9443, ppc)
	server.Start(*config.sslCertificate, *config.sslPrivateKey)

	hcConfig := hc.Config{
		Pin:         config.cfg.Pin,
		StoragePath: *config.dataDirectory,
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
		Debug("Stopping Controller")
		ppc.Stop()
		Debug("Stopping Server")
		server.Stop()
		Debug("Stopping Transport")
		<-transport.Stop()
		Debug("All services sent shutdown signal!")
	})
	Info("Homekit Pin: %s", hcConfig.Pin)

	// Starting transport blocks until the daemon is killed
	transport.Start()

	PowerLed.Output(Low)
	Info("Exiting")
}
