package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testSecureHandler(t *testing.T) *Handler {
	t.Helper()
	persist := false
	dir := t.TempDir()
	cfg := &Config{cfg: &PersistedConfig{}, persist: &persist, dataDirectory: &dir}
	cfg.SetAuth("secret")
	return &Handler{ppc: &PoolPumpController{config: cfg}}
}

func TestMutatingRoutesRequireAuthentication(t *testing.T) {
	h := testSecureHandler(t)
	for _, path := range []string{"/config", "/calibrate", "/runCalibration", "/pair", "/qr", "/resetPairings"} {
		method := http.MethodGet
		if path == "/runCalibration" || path == "/resetPairings" {
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

func TestPairingPageOffersResetWhileStillPaired(t *testing.T) {
	h := testSecureHandler(t)
	dir := *h.ppc.config.dataDirectory
	if err := os.WriteFile(filepath.Join(dir, accessoryIDFile), []byte("AA:BB:CC:DD:EE:FF"), 0600); err != nil {
		t.Fatal(err)
	}
	writeEntity(t, dir, "AA:BB:CC:DD:EE:FF")
	writeEntity(t, dir, "stale-controller")

	body := h.pairBody("")
	if !strings.Contains(body, `action="/resetPairings"`) {
		t.Fatal("pairing page should offer a reset while a stale pairing blocks discovery")
	}

	if _, err := ResetHomeKitPairings(dir); err != nil {
		t.Fatal(err)
	}
	if body := h.pairBody(""); strings.Contains(body, `action="/resetPairings"`) {
		t.Fatal("reset should not be offered once no controllers are paired")
	}
}

func TestResetPairingsHandlerRunsResetAction(t *testing.T) {
	h := testSecureHandler(t)
	called := 0
	h.pairingReset = func() (int, error) {
		called++
		return 2, nil
	}

	rec := httptest.NewRecorder()
	h.resetPairingsHandler(rec, httptest.NewRequest(http.MethodPost, "/resetPairings", nil))
	if called != 1 {
		t.Fatalf("reset action called %d times, want 1", called)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("reset returned %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Forgot 2 pairing(s)") {
		t.Fatalf("reset page did not report the result: %s", rec.Body.String())
	}
}

func TestResetPairingsHandlerReportsMissingTransport(t *testing.T) {
	h := testSecureHandler(t)
	rec := httptest.NewRecorder()
	h.resetPairingsHandler(rec, httptest.NewRequest(http.MethodPost, "/resetPairings", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("reset returned %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "transport is not running") {
		t.Fatalf("missing transport not reported: %s", rec.Body.String())
	}
}

func TestPairingQRUsesHomeKitSetupPayload(t *testing.T) {
	server := NewServer(LocalHost, 0, &PoolPumpController{})
	req := httptest.NewRequest(http.MethodGet, "/qr", nil)

	rec := httptest.NewRecorder()
	server.handler.qrHandler(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("qr without a setup payload returned %d, wanted 503 rather than an unscannable code", rec.Code)
	}

	server.SetPairingURI("X-HM://0024K0Y3WHOME")
	rec = httptest.NewRecorder()
	server.handler.qrHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("qr returned %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("qr content type %q, want image/png", got)
	}
}

func TestServerHasDefensiveTimeouts(t *testing.T) {
	server := NewServer(LocalHost, 0, &PoolPumpController{})
	if server.server.ReadHeaderTimeout <= 0 ||
		server.server.ReadTimeout <= 0 ||
		server.server.WriteTimeout <= 0 ||
		server.server.IdleTimeout <= 0 {
		t.Fatal("server timeouts not configured")
	}
	if server.server.TLSConfig == nil ||
		server.server.TLSConfig.MinVersion == 0 {
		t.Fatal("minimum TLS version not configured")
	}
	if server.server.ReadHeaderTimeout > 10*time.Second {
		t.Fatalf("ReadHeaderTimeout too permissive: %s", server.server.ReadHeaderTimeout)
	}
}
