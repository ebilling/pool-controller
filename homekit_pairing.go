package main

import (
	"os"
	"path/filepath"
)

// HomeKit pairing state lives in the data directory as "<hex(name)>.pairing"
// files, one per paired iOS controller. This accessory's own identity is kept
// apart from those, in "uuid" and "keypair", so every pairing file can be
// removed without the accessory becoming a different device.
//
// An accessory that holds a pairing advertises itself as not discoverable, and
// hap refreshes that mDNS flag when it adds or removes a pairing itself. A
// pairing removed behind its back, as the reset below does, therefore only
// takes effect once the server announces itself again.
const pairingSuffix = ".pairing"

// legacyPairingSuffix is how brutella/hc, the library this controller used
// before, named both controller pairings and the accessory's own identity.
const legacyPairingSuffix = ".entity"

// controllerPairings returns the pairing file of every paired controller.
func controllerPairings(dir string) ([]string, error) {
	return filepath.Glob(filepath.Join(dir, "*"+pairingSuffix))
}

// ResetHomeKitPairings forgets every paired controller while keeping this
// accessory's own identity, so it advertises as discoverable again and the
// setup code pairs from scratch.
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

// RemoveLegacyPairings deletes the pairing files left behind by hc. They are
// unreadable to hap, so they neither keep a controller paired nor stop the
// accessory from being discoverable; they only make the pairing state on disk
// ambiguous. Returns the number of files removed.
func RemoveLegacyPairings(dir string) (int, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*"+legacyPairingSuffix))
	if err != nil {
		return 0, err
	}
	for i, path := range paths {
		if err := os.Remove(path); err != nil {
			return i, err
		}
	}
	return len(paths), nil
}
