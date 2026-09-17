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
// reports as "unable to add accessory". The per-request lines are debug in the
// library, but they are written at info here when debug logging is on: syslog
// debug is dropped by the default journal, which is why a pairing can look like
// silence even though the accessory answered.
func CaptureHomeKitLogs(debug bool) {
	haplog.Info.SetOutput(hapLogWriter(syslogWriter.Info))
	if debug {
		haplog.Debug.SetOutput(hapLogWriter(syslogWriter.Info))
	} else {
		haplog.Debug.Disable()
	}
}

// loggedPairingStore is hap's file store, plus a line whenever a pairing is
// written or forgotten. hap itself ignores the error from that write, which is
// why a pairing can succeed in the Home app and still leave this accessory
// unpaired on disk.
type loggedPairingStore struct {
	hap.Store
}

func (s loggedPairingStore) Set(key string, value []byte) error {
	err := s.Store.Set(key, value)
	if strings.HasSuffix(key, pairingSuffix) {
		if err != nil {
			Error("could not store HomeKit pairing %s: %s", key, err)
		} else {
			Info("stored HomeKit pairing %s", key)
		}
	}
	return err
}

func (s loggedPairingStore) Delete(key string) error {
	err := s.Store.Delete(key)
	if strings.HasSuffix(key, pairingSuffix) {
		if err != nil {
			Error("could not forget HomeKit pairing %s: %s", key, err)
		} else {
			Info("forgot HomeKit pairing %s", key)
		}
	}
	return err
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
	store := loggedPairingStore{Store: hap.NewFsStore(dir)}
	srv, err := hap.NewServer(store, bridge.A, bridged...)
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

// Announcement describes the addresses the accessory offers over mDNS. A phone
// picks one of them, so an address it cannot route to, from a second network or
// a container bridge, is enough to leave the Home app waiting with nothing in
// the log to show for it.
func (h *HomeKitService) Announcement() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "unknown: " + err.Error()
	}
	var out []string
	for _, iface := range ifaces {
		if !announces(iface) || !h.announcesOn(iface.Name) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip, ok := addr.(*net.IPNet)
			if !ok || ip.IP.To4() == nil {
				continue
			}
			out = append(out, iface.Name+"="+ip.IP.String())
		}
	}
	if len(out) == 0 {
		return "no interface to announce on"
	}
	return strings.Join(out, " ")
}

// announces reports whether mDNS can reach anyone through this interface.
func announces(iface net.Interface) bool {
	return iface.Flags&net.FlagUp != 0 &&
		iface.Flags&net.FlagLoopback == 0 &&
		iface.Flags&net.FlagMulticast != 0
}

func (h *HomeKitService) announcesOn(name string) bool {
	if len(h.srv.Ifaces) == 0 {
		return true
	}
	for _, only := range h.srv.Ifaces {
		if only == name {
			return true
		}
	}
	return false
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
