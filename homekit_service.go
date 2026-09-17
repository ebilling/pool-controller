package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	haplog "github.com/brutella/hap/log"
)

// setupID names the printed setup label. It has to stay stable across restarts
// because it is hashed into the "sh" mDNS record, which the Home app compares
// against the label it just scanned.
const setupID = "HOME"

// hapLogWriter turns hap's log output into one syslog line per message.
type hapLogWriter func(string) error

func (w hapLogWriter) Write(p []byte) (int, error) {
	if err := w(strings.TrimRight(string(p), "\n")); err != nil {
		return 0, err
	}
	return len(p), nil
}

// CaptureHomeKitLogs routes hap's own messages to syslog. Its info lines carry
// the reason a pairing was refused or failed, which the Home app only ever
// reports as "unable to add accessory". The debug lines follow each pairing
// step and are noisy, so they need debug logging turned on.
func CaptureHomeKitLogs(debug bool) {
	haplog.Info.SetOutput(hapLogWriter(syslogWriter.Info))
	if debug {
		haplog.Debug.SetOutput(hapLogWriter(syslogWriter.Debug))
	} else {
		haplog.Debug.Disable()
	}
}

// HomeKitService owns the HAP server: the mDNS announcement, the pairing
// endpoints, and the accessory database kept in the data directory.
type HomeKitService struct {
	srv      *hap.Server
	dir      string
	category byte

	mu      sync.Mutex
	cancel  context.CancelFunc
	stopped chan struct{}
}

// NewHomeKitService publishes the accessories as a single bridge. Naming the
// bridge as the primary accessory is what lets HomeKit present the thermostat,
// the thermometers, and the relays as one device.
func NewHomeKitService(dir, pin string, bridged ...*accessory.A) (*HomeKitService, error) {
	bridge := accessory.NewBridge(AccessoryInfo("Pool Controller", mftr))
	srv, err := hap.NewServer(hap.NewFsStore(dir), bridge.A, bridged...)
	if err != nil {
		return nil, err
	}
	srv.Pin = pin
	srv.SetupId = setupID
	return &HomeKitService{srv: srv, dir: dir, category: bridge.A.Type}, nil
}

// ListenOn fixes the port HomeKit serves and announces. A port of zero leaves
// the choice to the kernel, which makes the accessory impossible to reach on
// purpose, since nothing outside the mDNS announcement knows where it went.
func (h *HomeKitService) ListenOn(port int) {
	if port > 0 {
		h.srv.Addr = fmt.Sprintf(":%d", port)
	}
}

// AnnounceOn restricts the mDNS announcement to one interface. With every
// interface announced, a phone is free to pick an address it cannot route to,
// and the pairing then stalls with nothing to show for it.
func (h *HomeKitService) AnnounceOn(iface string) error {
	if iface == "" {
		return nil
	}
	if _, err := net.InterfaceByName(iface); err != nil {
		return err
	}
	h.srv.Ifaces = []string{iface}
	return nil
}

// Address is where HomeKit serves, for the log. It is only known ahead of time
// if the port was fixed.
func (h *HomeKitService) Address() string {
	if h.srv.Addr == "" {
		return "an unpredictable port"
	}
	return h.srv.Addr
}

// Start serves HomeKit in the background until Stop is called.
func (h *HomeKitService) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})

	h.mu.Lock()
	h.cancel = cancel
	h.stopped = stopped
	h.mu.Unlock()

	go func() {
		defer close(stopped)
		if err := h.srv.ListenAndServe(ctx); err != nil && ctx.Err() == nil {
			Error("HomeKit server stopped: %s", err)
		}
	}()
}

// Stop shuts the HAP server down and waits for it to finish.
func (h *HomeKitService) Stop() {
	h.mu.Lock()
	cancel, stopped := h.cancel, h.stopped
	h.cancel, h.stopped = nil, nil
	h.mu.Unlock()

	if cancel == nil {
		return
	}
	cancel()
	<-stopped
}

// SetupPayload is the X-HM:// URI to render as a pairing QR code.
func (h *HomeKitService) SetupPayload() (string, error) {
	return setupPayloadURI(h.srv.Pin, h.srv.SetupId, h.category, setupFlagIP)
}

// IsPaired reports whether any controller is paired, which is also what decides
// whether HomeKit advertises this accessory as discoverable.
func (h *HomeKitService) IsPaired() bool {
	return h.srv.IsPaired()
}

// ResetPairings forgets the paired controllers and restarts the daemon so the
// accessory announces itself as discoverable again.
//
// A restart is what makes this take effect: hap decides discoverability while
// it announces the service and refreshes that record only when it adds or
// removes a pairing itself, so pairings removed from disk are otherwise
// invisible to the running announcement.
func (h *HomeKitService) ResetPairings() (int, error) {
	removed, err := ResetHomeKitPairings(h.dir)
	if err != nil {
		return removed, err
	}
	go restartDaemon()
	return removed, nil
}

// restartDaemon replaces this process with a fresh copy of itself, keeping the
// same pid so the init script and the pid file stay valid. The pause gives the
// web response that asked for the restart time to reach the browser.
func restartDaemon() {
	time.Sleep(time.Second)
	exe, err := os.Executable()
	if err != nil {
		Error("Could not locate our own binary to restart: %s", err)
		return
	}
	Info("Restarting %s", exe)
	if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
		Error("Could not restart: %s", err)
	}
}
