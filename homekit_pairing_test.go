package main

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func writeEntity(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, hex.EncodeToString([]byte(name))+entitySuffix)
	if err := os.WriteFile(path, []byte(`{"Name":"`+name+`"}`), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestControllerPairingsExcludesAccessoryIdentity(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, accessoryIDFile), []byte("AA:BB:CC:DD:EE:FF"), 0600); err != nil {
		t.Fatal(err)
	}
	self := writeEntity(t, dir, "AA:BB:CC:DD:EE:FF")
	controller := writeEntity(t, dir, "5D0A1B2C-3E4F-5061-7283-94A5B6C7D8E9")

	got, err := controllerPairings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != controller {
		t.Fatalf("controllerPairings() = %v, want only %s", got, controller)
	}
	if _, err := os.Stat(self); err != nil {
		t.Fatalf("accessory identity should be untouched: %v", err)
	}
}

func TestResetHomeKitPairingsKeepsAccessoryIdentity(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, accessoryIDFile), []byte("AA:BB:CC:DD:EE:FF\n"), 0600); err != nil {
		t.Fatal(err)
	}
	self := writeEntity(t, dir, "AA:BB:CC:DD:EE:FF")
	writeEntity(t, dir, "controller-one")
	writeEntity(t, dir, "controller-two")

	removed, err := ResetHomeKitPairings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatalf("removed %d pairings, want 2", removed)
	}
	if _, err := os.Stat(self); err != nil {
		t.Fatalf("accessory identity was deleted: %v", err)
	}

	remaining, err := controllerPairings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("pairings still present: %v", remaining)
	}
}

func TestResetHomeKitPairingsLeavesUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "server.conf")
	if err := os.WriteFile(conf, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	rrd := filepath.Join(dir, "temperature.rrd")
	if err := os.WriteFile(rrd, []byte("rrd"), 0600); err != nil {
		t.Fatal(err)
	}
	writeEntity(t, dir, "controller")

	if _, err := ResetHomeKitPairings(dir); err != nil {
		t.Fatal(err)
	}
	for _, keep := range []string{conf, rrd} {
		if _, err := os.Stat(keep); err != nil {
			t.Fatalf("%s should be preserved: %v", keep, err)
		}
	}
}
