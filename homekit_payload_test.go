package main

import "testing"

// The expected payloads come from brutella/hc, which produced the URIs this
// accessory advertised before it built them itself.
func TestSetupPayloadURI(t *testing.T) {
	const (
		typeGarageDoorOpener = 4
		typeLightbulb        = 5
	)
	for _, tc := range []struct {
		name     string
		pin      string
		category byte
		flags    uint64
		want     string
	}{
		{"ip lightbulb", "102-93-847", typeLightbulb, setupFlagIP, "X-HM://00526Q9UFERIC"},
		{"ip and btle", "102-93-847", typeLightbulb, setupFlagIP | setupFlagBTLE, "X-HM://005B2DA3BERIC"},
		{"garage door", "102-93-847", typeGarageDoorOpener, setupFlagIP, "X-HM://0042O68UVERIC"},
		{"undashed code", "10293847", typeLightbulb, setupFlagIP, "X-HM://00526Q9UFERIC"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := setupPayloadURI(tc.pin, "ERIC", tc.category, tc.flags)
			if err != nil {
				t.Fatalf("setupPayloadURI() returned %s", err)
			}
			if got != tc.want {
				t.Errorf("setupPayloadURI() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSetupPayloadURIRejectsUnusableInput(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pin     string
		setupID string
	}{
		{"not numeric", "BADBADBA", "ERIC"},
		{"too few digits", "1029384", "ERIC"},
		{"too many digits", "102938475", "ERIC"},
		{"short setup id", "10293847", "ERI"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uri, err := setupPayloadURI(tc.pin, tc.setupID, 9, setupFlagIP)
			if err == nil {
				t.Fatalf("setupPayloadURI() accepted %q/%q and returned %q",
					tc.pin, tc.setupID, uri)
			}
			if uri != "" {
				t.Errorf("setupPayloadURI() returned %q alongside an error", uri)
			}
		})
	}
}

func TestFormatSetupCode(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"10293847", "102-93-847"},
		{"102-93-847", "102-93-847"},
		{"", ""},
		{"123", "123"},
	} {
		if got := formatSetupCode(tc.in); got != tc.want {
			t.Errorf("formatSetupCode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
