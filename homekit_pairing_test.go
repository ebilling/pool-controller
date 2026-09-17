package main

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func writePairing(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, hex.EncodeToString([]byte(name))+pairingSuffix)
	if err := os.WriteFile(path, []byte(`{"Name":"`+name+`"}`), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeAccessoryIdentity(t *testing.T, dir string) []string {
	t.Helper()
	var paths []string
	for name, content := range map[string]string{
		"uuid":    "AA:BB:CC:DD:EE:FF",
		"keypair": `{"Public":"cHVibGlj","Private":"cHJpdmF0ZQ=="}`,
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

func TestControllerPairingsFindsPairedControllers(t *testing.T) {
	dir := t.TempDir()
	writeAccessoryIdentity(t, dir)
	controller := writePairing(t, dir, "5D0A1B2C-3E4F-5061-7283-94A5B6C7D8E9")

	got, err := controllerPairings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != controller {
		t.Fatalf("controllerPairings() = %v, want only %s", got, controller)
	}
}

func TestResetHomeKitPairingsKeepsAccessoryIdentity(t *testing.T) {
	dir := t.TempDir()
	identity := writeAccessoryIdentity(t, dir)
	writePairing(t, dir, "controller-one")
	writePairing(t, dir, "controller-two")

	removed, err := ResetHomeKitPairings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatalf("removed %d pairings, want 2", removed)
	}
	for _, keep := range identity {
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("accessory identity %s was deleted: %v", filepath.Base(keep), err)
		}
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
	writePairing(t, dir, "controller")

	if _, err := ResetHomeKitPairings(dir); err != nil {
		t.Fatal(err)
	}
	for _, keep := range []string{conf, rrd} {
		if _, err := os.Stat(keep); err != nil {
			t.Fatalf("%s should be preserved: %v", keep, err)
		}
	}
}

// The pairing files hc wrote are unreadable to hap, so they are cleared out at
// startup to keep the pairing state on disk unambiguous.
func TestRemoveLegacyPairingsOnlyTakesEntityFiles(t *testing.T) {
	dir := t.TempDir()
	identity := writeAccessoryIdentity(t, dir)
	current := writePairing(t, dir, "controller")
	legacy := filepath.Join(dir, hex.EncodeToString([]byte("old-controller"))+legacyPairingSuffix)
	if err := os.WriteFile(legacy, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	removed, err := RemoveLegacyPairings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed %d legacy pairings, want 1", removed)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy pairing still present: %v", err)
	}
	for _, keep := range append(identity, current) {
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("%s should be preserved: %v", filepath.Base(keep), err)
		}
	}
}
