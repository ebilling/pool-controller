package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func testSecureHandler(t *testing.T) *Handler {
	t.Helper()
	persist := false
	cfg := &Config{cfg: &PersistedConfig{}, persist: &persist}
	cfg.SetAuth("secret")
	return &Handler{ppc: &PoolPumpController{config: cfg}}
}

func TestMutatingRoutesRequireAuthentication(t *testing.T) {
	h := testSecureHandler(t)
	for _, path := range []string{"/config", "/calibrate", "/runCalibration", "/pair", "/qr"} {
		method := http.MethodGet
		if path == "/runCalibration" {
			method = http.MethodPost
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s returned %d, want 401", method, path, rec.Code)
		}
		if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, "pool-controller") {
			t.Errorf("%s did not return an authentication challenge", path)
		}
	}
}

func TestCalibrationRejectsGetSubmission(t *testing.T) {
	h := testSecureHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/runCalibration", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /runCalibration returned %d, want 405", rec.Code)
	}
}

func TestPartialConfigPostDoesNotClearBooleans(t *testing.T) {
	h := testSecureHandler(t)
	h.ppc.config.cfg.Disabled = true
	h.ppc.config.cfg.SolarDisabled = true
	form := url.Values{"posted": {"true"}, "target": {"29.5"}}
	req := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}

	h.processForm(req, h.ppc.config)
	if !h.ppc.config.cfg.Disabled || !h.ppc.config.cfg.SolarDisabled {
		t.Fatal("partial form cleared an omitted boolean setting")
	}
}

func TestServerHasDefensiveTimeouts(t *testing.T) {
	server := NewServer(LocalHost, 0, &PoolPumpController{})
	if server.server.ReadHeaderTimeout <= 0 ||
		server.server.ReadTimeout <= 0 ||
		server.server.WriteTimeout <= 0 ||
		server.server.IdleTimeout <= 0 {
		t.Fatalf("server timeouts not configured: %+v", server.server)
	}
	if server.server.TLSConfig == nil ||
		server.server.TLSConfig.MinVersion == 0 {
		t.Fatal("minimum TLS version not configured")
	}
	if server.server.ReadHeaderTimeout > 10*time.Second {
		t.Fatalf("ReadHeaderTimeout too permissive: %s", server.server.ReadHeaderTimeout)
	}
}
