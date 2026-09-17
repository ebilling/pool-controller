package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
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

	if *config.simulate {
		EnableSimulation()
		Info("Simulation mode: pump=%0.1fC roof=%0.1fC (no real GPIO)",
			*config.simPumpTemp, *config.simRoofTemp)
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

	if *config.debug {
		EnableDebug()
		Info("Debug logging is on; HomeKit pairing requests will appear at info")
	}
	// HomeKit refuses a pairing for reasons only the library knows about, so
	// let it log next to everything else.
	CaptureHomeKitLogs(doDebug)

	homekit, err := NewHomeKitService(
		*config.dataDirectory,
		config.cfg.Pin,
		ppc.thermostat.Accessory(),
		ppc.pumpTemp.Accessory(),
		ppc.roofTemp.Accessory(),
		ppc.switches.pump.Accessory(),
		ppc.switches.sweep.Accessory(),
		ppc.switches.solar.Accessory())
	if err != nil {
		Fatal("Could not start HomeKit: %s", err.Error())
	}
	homekit.ListenOn(*config.homekitPort)
	if err := homekit.AnnounceOn(*config.homekitIface); err != nil {
		Fatal("Could not announce HomeKit on %s: %s", *config.homekitIface, err.Error())
	}

	// Opening the store above carried the accessory keys and any pairings out
	// of the files the previous HomeKit library wrote, so they can go now.
	if removed, err := RemoveLegacyPairings(*config.dataDirectory); err != nil {
		Error("Could not clear the files left by the previous HomeKit library: %s", err.Error())
	} else if removed > 0 {
		Info("Removed %d migrated file(s) written by the previous HomeKit library", removed)
	}

	if *config.resetPairings {
		removed, err := ResetHomeKitPairings(*config.dataDirectory)
		if err != nil {
			Fatal("Could not reset HomeKit pairings: %s", err.Error())
		}
		Info("Forgot %d HomeKit controller pairing(s)", removed)
	}

	if homekit.IsPaired() {
		Info("HomeKit is paired, so this accessory is not discoverable. Forget the " +
			"pairing from the web interface, or restart with -reset-homekit-pairings, " +
			"if it was already removed from the Home app.")
	} else {
		Info("HomeKit has no pairing; accessory is discoverable")
	}

	// The web interface shows the pairing code, so it needs the setup payload.
	server := NewServer(AnyHost, *config.httpPort, ppc)
	if uri, err := homekit.SetupPayload(); err != nil {
		Error("Could not build HomeKit setup payload: %s", err.Error())
	} else {
		server.SetPairingURI(uri)
		Info("HomeKit setup payload: %s", uri)
	}
	server.SetPairingReset(homekit.ResetPairings)
	server.Start(*config.sslCertificate, *config.sslPrivateKey)

	homekit.Start()
	Info("HomeKit listening on %s, setup code: %s, announced on %s",
		homekit.Address(), formatSetupCode(config.cfg.Pin), homekit.Announcement())

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	Info("Received signal %s, shutting down", <-signals)

	Debug("Stopping Controller")
	ppc.Stop()
	Debug("Stopping Server")
	server.Stop()
	Debug("Stopping HomeKit")
	homekit.Stop()

	PowerLed.Output(Low)
	Info("Exiting")
}
