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
// before, named both controller pairings and the accessory's own identity. hap
// reads those files once, when it first opens the store, and copies what they
// hold into "keypair" and the pairing files above.
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

// identityFiles are what make this accessory the device HomeKit remembers:
// "uuid" is the id it advertises, "keypair" the long term keys it proves that
// id with, and the rest is hap's own bookkeeping about the published
// accessories.
var identityFiles = []string{"uuid", "keypair", "schema", "configHash", "version"}

// ResetHomeKitIdentity makes the accessory come back as a device HomeKit has
// never seen: a new id and new keys, with every pairing dropped.
//
// Keeping the id while the keys change is the state to avoid. A controller, or
// a home hub acting for one, remembers the id together with the key it was
// first given, so it can go on showing the accessory and its last known values
// while no longer being able to talk to it. Only the configuration and the
// recorded history in the same directory are left alone.
func ResetHomeKitIdentity(dir string) (int, error) {
	removed, err := ResetHomeKitPairings(dir)
	if err != nil {
		return removed, err
	}
	legacy, err := RemoveLegacyPairings(dir)
	removed += legacy
	if err != nil {
		return removed, err
	}
	for _, name := range identityFiles {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// RemoveLegacyPairings deletes the files left behind by hc. Only call it once
// hap has opened the store, because deleting them first throws away the
// accessory's long term keys and every pairing hap would have migrated, which
// forces the accessory to be added again as a new device. Returns the number
// of files removed.
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
