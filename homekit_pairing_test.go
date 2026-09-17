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

func TestResetHomeKitIdentityMakesANewDevice(t *testing.T) {
	dir := t.TempDir()
	writeAccessoryIdentity(t, dir)
	writePairing(t, dir, "controller")
	conf := filepath.Join(dir, "server.conf")
	if err := os.WriteFile(conf, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	rrd := filepath.Join(dir, "temperature.rrd")
	if err := os.WriteFile(rrd, []byte("rrd"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := ResetHomeKitIdentity(dir); err != nil {
		t.Fatal(err)
	}
	for _, gone := range append(identityFiles, hex.EncodeToString([]byte("controller"))+pairingSuffix) {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("%s survived the identity reset: %v", gone, err)
		}
	}
	for _, keep := range []string{conf, rrd} {
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("%s should be preserved: %v", filepath.Base(keep), err)
		}
	}
}

func TestResetHomeKitIdentityIsSafeToRepeat(t *testing.T) {
	dir := t.TempDir()
	if _, err := ResetHomeKitIdentity(dir); err != nil {
		t.Fatalf("resetting an empty directory should not fail: %v", err)
	}
}

// A controller paired against hc stays paired across the upgrade: hap copies
// the old files into its own when it opens the store, and clearing them out
// afterwards must not undo that.
func TestStartupKeepsAControllerPairedByTheOldLibrary(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, hex.EncodeToString([]byte("controller"))+legacyPairingSuffix)
	entity := `{"Name":"controller","PublicKey":"cHVibGljLWtleQ=="}`
	if err := os.WriteFile(legacy, []byte(entity), 0600); err != nil {
		t.Fatal(err)
	}

	homekit, err := NewHomeKitService(dir, defaultPin,
		NewTemperatureSensorAccessory("Pool", mftr).A)
	if err != nil {
		t.Fatal(err)
	}
	if removed, err := RemoveLegacyPairings(dir); err != nil {
		t.Fatal(err)
	} else if removed != 1 {
		t.Fatalf("removed %d legacy files, want 1", removed)
	}

	if !homekit.IsPaired() {
		t.Error("the migrated pairing was lost, so the accessory has to be added again")
	}
}
