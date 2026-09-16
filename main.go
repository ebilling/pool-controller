package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/brutella/hc"
)

func main() {
	fs := flag.NewFlagSet("pool-controller", flag.PanicOnError)
	help := fs.Bool("h", false, "Display this usage message")
	printVersion := fs.Bool("version", false, "Print the build version and exit")
	config := NewConfig(fs, os.Args[1:]) // Parses flags

	if *printVersion {
		fmt.Println(versionLine())
		os.Exit(0)
	}

	if *help {
		flag.Usage()
		fmt.Fprintf(os.Stderr, "Any changes put into the web interface will override these"+
			"flags.\nThe config is stored in %s%s.  It can be carefully edited by hand",
			*config.dataDirectory, serverConfiguration)
		os.Exit(1)
	}

	// Recover saved values, edit conf to clean them
	Info("%s", versionLine())
	Info("Args: %s", os.Args[1:])

	// Write PID
	err := ioutil.WriteFile(*config.pidfile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
	if err != nil {
		Fatal("Could not write pid file: %s", err.Error())
	}

	switch {
	case *config.simulate:
		EnableSimulation()
		Info("Simulation mode: pump=%0.1fC roof=%0.1fC (no real GPIO)",
			*config.simPumpTemp, *config.simRoofTemp)
	case *config.gpioDriver == "cdev":
		EnableCdevGpio()
	case *config.gpioDriver == "periph":
		Info("GPIO: using periph.io")
	default:
		Fatal("Unknown -gpio-driver %q: want 'cdev' or 'periph'", *config.gpioDriver)
	}

	if err := GpioInit(); err != nil {
		Fatal("Could not initialize GPIO: %s", err.Error())
	}

	PowerLed := NewGpio(5)
	PowerLed.Output(High)
	ppc := NewPoolPumpController(config)
	err = ppc.Start()
	if err != nil {
		Fatal("Could not start pool controller: %s", err.Error())
	}

	server := NewServer(AnyHost, *config.httpPort, ppc)
	server.Start(*config.sslCertificate, *config.sslPrivateKey)

	hcConfig := hc.Config{
		Pin:         config.cfg.Pin,
		StoragePath: *config.dataDirectory,
	}

	transport, err := hc.NewIPTransport(
		hcConfig,
		ppc.thermostat.Accessory(),
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
