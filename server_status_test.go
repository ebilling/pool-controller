package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStatusPageTemperatureUnitSelector(t *testing.T) {
	SetGpioProvider(NewTestPin)
	trp := NewTestRunPumps()
	trp.setConditions(30, 20, 40, 0, OFF)
	trp.ppc.config.cfg.TemperatureUnit = TemperatureCelsius
	h := &Handler{ppc: trp.ppc}

	rec := httptest.NewRecorder()
	h.rootHandler(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	for _, want := range []string{
		`id="status-temperature-unit"`,
		`<option value="C" selected>°C</option>`,
		`id="metric-target">30.0 °C`,
		`id="metric-pool">20.0 °C`,
		`id="metric-roof">40.0 °C`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("status page is missing %q", want)
		}
	}

	status := h.liveStatus()
	if status.TargetC != 30 || status.PoolC != 20 || status.RoofC != 40 {
		t.Errorf("Celsius status = target %g pool %g roof %g", status.TargetC, status.PoolC, status.RoofC)
	}
}
