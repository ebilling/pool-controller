package main

import "strings"
import "testing"

func TestPageForcesLightColorScheme(t *testing.T) {
	got := page("Pool controller", "<p>hi</p>")
	if !strings.Contains(got, `name="color-scheme" content="light"`) {
		t.Fatal("layout must pin color-scheme to light so labels stay visible")
	}
	if !strings.Contains(got, "background: var(--bg)") {
		t.Fatal("layout must set an explicit page background")
	}
	if !strings.Contains(got, `href="/config"`) {
		t.Fatal("nav should include config")
	}
}

func TestMetricCardEscapes(t *testing.T) {
	got := metricCard("<script>", "1°F")
	if strings.Contains(got, "<script>") {
		t.Fatal("metric labels must be escaped")
	}
}
