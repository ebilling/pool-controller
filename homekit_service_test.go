package main

import (
	"strings"
	"testing"
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

func TestHomeKitRejectsAnInterfaceThatIsNotThere(t *testing.T) {
	homekit := testHomeKitService(t)
	if err := homekit.AnnounceOn("nope0"); err == nil {
		t.Error("announcing on a missing interface should fail rather than go unnoticed")
	}
	if err := homekit.AnnounceOn(""); err != nil {
		t.Errorf("an empty interface means all of them: %v", err)
	}
}
