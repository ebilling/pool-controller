package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brutella/hap"
)

func testHomeKitService(t *testing.T) *HomeKitService {
	t.Helper()
	homekit, err := NewHomeKitService(t.TempDir(), defaultPin,
		NewTemperatureSensorAccessory("Pool", mftr).A)
	if err != nil {
		t.Fatal(err)
	}
	return homekit
}

func TestHomeKitListensWhereItSaysItDoes(t *testing.T) {
	homekit := testHomeKitService(t)
	if got := homekit.Address(); got != "an unpredictable port" {
		t.Errorf("without a port, Address() = %q", got)
	}

	homekit.ListenOn(defaultHomeKitPort)
	if got, want := homekit.Address(), ":51826"; got != want {
		t.Errorf("Address() = %q, want %q", got, want)
	}
}

func TestHomeKitReportsWhereItIsAnnounced(t *testing.T) {
	homekit := testHomeKitService(t)
	got := homekit.Announcement()
	if strings.HasPrefix(got, "unknown: ") {
		t.Skipf("this host will not list its interfaces: %s", got)
	}
	if strings.Contains(got, "127.0.0.1") {
		t.Errorf("Announcement() offers the loopback address: %q", got)
	}

	homekit.srv.Ifaces = []string{"nope0"}
	if got, want := homekit.Announcement(), "no interface to announce on"; got != want {
		t.Errorf("Announcement() = %q, want %q", got, want)
	}
}

func TestLoggedPairingStoreReportsAWrite(t *testing.T) {
	dir := t.TempDir()
	store := loggedPairingStore{Store: hap.NewFsStore(dir)}
	key := "controller" + pairingSuffix
	if err := store.Set(key, []byte(`{"Name":"controller"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, key)); err != nil {
		t.Fatalf("pairing was not written: %v", err)
	}
}

func TestHomeKitRejectsAnInterfaceThatIsNotThere(t *testing.T) {
	homekit := testHomeKitService(t)
	if err := homekit.AnnounceOn("nope0"); err == nil {
		t.Error("announcing on a missing interface should fail rather than go unnoticed")
	}
	if err := homekit.AnnounceOn(""); err != nil {
		t.Errorf("an empty interface means all of them: %v", err)
	}
}
