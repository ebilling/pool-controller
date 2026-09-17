package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Setup flags describing how the accessory can be paired.
const (
	setupFlagNFC  uint64 = 1
	setupFlagIP   uint64 = 2
	setupFlagBTLE uint64 = 4
)

const base36Digits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// setupPayloadURI builds the X-HM:// payload the Home app expects to find in a
// pairing QR code. A QR code holding only the eight digit setup code is not a
// valid payload, so the Home app cannot recognize it at all.
//
// The payload packs 46 bits, from the most significant end: 3 bits of version,
// 4 reserved, 8 of accessory category, 4 of setup flags and 27 of setup code.
// Those are written as 9 base36 digits followed by the 4 character setup id.
// The category has to match the "ci" mDNS record and the setup id has to agree
// with the "sh" setup hash, or the Home app will not find the accessory whose
// code it just scanned.
func setupPayloadURI(pin, setupID string, category byte, flags uint64) (string, error) {
	digits := strings.ReplaceAll(pin, "-", "")
	if len(digits) != 8 {
		return "", fmt.Errorf("setup code %q has %d digits, want 8", pin, len(digits))
	}
	code, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return "", fmt.Errorf("setup code %q is not numeric: %w", pin, err)
	}
	if len(setupID) != 4 {
		return "", fmt.Errorf("setup id %q has %d characters, want 4", setupID, len(setupID))
	}

	const (
		version  uint64 = 0
		reserved uint64 = 0
	)
	payload := ((version & 0x7) << 43) |
		((reserved & 0xf) << 39) |
		(uint64(category) << 31) |
		((flags & 0xf) << 27) |
		(code & 0x7ffffff)

	encoded := make([]byte, 9)
	for i := 8; i >= 0; i-- {
		encoded[i] = base36Digits[payload%36]
		payload /= 36
	}
	return "X-HM://" + string(encoded) + setupID, nil
}

// formatSetupCode renders an eight digit setup code the way Apple prints it on
// a setup label, so it can be typed into the Home app as it is shown.
func formatSetupCode(pin string) string {
	digits := strings.ReplaceAll(pin, "-", "")
	if len(digits) != 8 {
		return pin
	}
	return digits[0:3] + "-" + digits[3:5] + "-" + digits[5:8]
}
