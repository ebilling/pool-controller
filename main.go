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

	// Discoverability is decided when the transport is constructed, so forget
	// stale controller pairings before that happens.
	if *config.resetPairings {
		removed, err := ResetHomeKitPairings(*config.dataDirectory)
		if err != nil {
			Fatal("Could not reset HomeKit pairings: %s", err.Error())
		}
		Info("Forgot %d HomeKit controller pairing(s)", removed)
	}

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

	if paired, err := controllerPairings(*config.dataDirectory); err != nil {
		Error("Could not read HomeKit pairing state: %s", err.Error())
	} else if len(paired) > 0 {
		Alert("HomeKit is paired with %d controller(s), so this accessory is not "+
			"discoverable. If it was removed from the Home app, restart with "+
			"-reset-homekit-pairings.", len(paired))
	} else {
		Info("HomeKit has no controller pairings; accessory is discoverable")
	}

	// The server serves the pairing QR code, so it needs the setup payload that
	// only the transport can produce.
	server := NewServer(AnyHost, *config.httpPort, ppc)
	if uri, err := transport.XHMURI(); err != nil {
		Error("Could not build HomeKit setup payload: %s", err.Error())
	} else {
		server.SetPairingURI(uri)
		Info("HomeKit setup payload: %s", uri)
	}
	server.Start(*config.sslCertificate, *config.sslPrivateKey)

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
