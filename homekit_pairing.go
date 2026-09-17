package main

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// HomeKit pairing state lives in the data directory as "<hex(name)>.entity"
// files written by brutella/hc. One of them is this accessory's own identity,
// named after the id recorded in "uuid"; every other one is an iOS controller
// that completed pairing.
//
// hc derives mDNS discoverability from that set: an accessory holding a
// controller pairing advertises sf=0, and hc only re-evaluates the flag when it
// handles a pair or unpair request. Removing the accessory in the Home app
// while this process is stopped or unreachable therefore leaves a pairing
// behind that nothing will ever delete, and the accessory stays undiscoverable
// across restarts.
const (
	entitySuffix    = ".entity"
	accessoryIDFile = "uuid"
)

func accessoryID(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, accessoryIDFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// controllerPairings returns the entity files for paired controllers, excluding
// this accessory's own identity.
func controllerPairings(dir string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*"+entitySuffix))
	if err != nil {
		return nil, err
	}
	self := accessoryID(dir)
	controllers := make([]string, 0, len(paths))
	for _, path := range paths {
		name, ok := entityFileName(path)
		if !ok || (self != "" && name == self) {
			continue
		}
		controllers = append(controllers, path)
	}
	return controllers, nil
}

func entityFileName(path string) (string, bool) {
	raw, err := hex.DecodeString(strings.TrimSuffix(filepath.Base(path), entitySuffix))
	if err != nil {
		return "", false
	}
	return string(raw), true
}

// ResetHomeKitPairings forgets every paired controller while keeping this
// accessory's own key pair, so the next start advertises as discoverable and
// the setup code works again.
func ResetHomeKitPairings(dir string) (int, error) {
	controllers, err := controllerPairings(dir)
	if err != nil {
		return 0, err
	}
	for i, path := range controllers {
		if err := os.Remove(path); err != nil {
			return i, err
		}
	}
	return len(controllers), nil
}
