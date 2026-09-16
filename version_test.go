package main

import "testing"

func TestVersionFromInjectedLdflag(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "abc1234"
	if got := buildVersion(); got != "abc1234" {
		t.Errorf("buildVersion() = %q, want injected ldflag", got)
	}
}

func TestVersionLineIncludesName(t *testing.T) {
	line := versionLine()
	if line[:len("pool-controller ")] != "pool-controller " {
		t.Errorf("versionLine() = %q", line)
	}
	if len(line) < len("pool-controller unknown go=go") {
		t.Errorf("versionLine() too short: %q", line)
	}
}
