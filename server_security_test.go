package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	qrcode "github.com/skip2/go-qrcode"
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

// The Debug checkbox submits the value the form renders for it, so the handler
// has to recognise that one rather than the browser's default for a valueless
// checkbox.
func TestDebugCheckboxTurnsDebugLoggingOn(t *testing.T) {
	h := testSecureHandler(t)
	DisableDebug()
	t.Cleanup(DisableDebug)

	form := url.Values{"posted": {"true"}, "_present_debug": {"true"}, "debug": {"true"}}
	req := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}
	h.processForm(req, h.ppc.config)
	if !doDebug {
		t.Fatal("ticking Debug left debug logging off")
	}

	form = url.Values{"posted": {"true"}, "_present_debug": {"true"}}
	req = httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}
	h.processForm(req, h.ppc.config)
	if doDebug {
		t.Fatal("clearing Debug left debug logging on")
	}
}

// The pairing page reports whether HomeKit is paired, which changes without
// the page being asked to, so it must not be served from the browser's cache.
func TestPairingPageIsNeverServedFromCache(t *testing.T) {
	h := testSecureHandler(t)
	rec := httptest.NewRecorder()
	h.pairHandler(rec, httptest.NewRequest(http.MethodGet, "/pair", nil))
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

func TestCleaningSettingsHaveTheirOwnSection(t *testing.T) {
	h := testSecureHandler(t)
	rec := httptest.NewRecorder()
	h.configHandler(rec, httptest.NewRequest(http.MethodGet, "/config", nil))
	body := rec.Body.String()

	solar := strings.Index(body, "<legend>Solar</legend>")
	cleaning := strings.Index(body, "<legend>Cleaning / Sweep</legend>")
	debug := strings.Index(body, "<legend>Debug and disables</legend>")
	if solar < 0 || cleaning <= solar || debug <= cleaning {
		t.Fatalf("configuration sections are missing or out of order")
	}
	if strings.Contains(body[solar:cleaning], `name="daily_freq"`) ||
		strings.Contains(body[solar:cleaning], `name="run_time"`) {
		t.Fatal("cleaning settings are still rendered under Solar")
	}
	if !strings.Contains(body[cleaning:debug], `name="daily_freq"`) ||
		!strings.Contains(body[cleaning:debug], `name="run_time"`) {
		t.Fatal("Cleaning / Sweep section is missing cadence settings")
	}
}

func TestConfigurationCanDisplayFahrenheitWhileStoringCelsius(t *testing.T) {
	h := testSecureHandler(t)
	h.ppc.config.cfg.TemperatureUnit = TemperatureFahrenheit
	h.ppc.config.cfg.Target = 30
	h.ppc.config.cfg.Tolerance = 0.5
	h.ppc.config.cfg.DeltaT = 12

	rec := httptest.NewRecorder()
	h.configHandler(rec, httptest.NewRequest(http.MethodGet, "/config", nil))
	body := rec.Body.String()
	for _, want := range []string{
		`<option value="F" selected>Fahrenheit (°F)</option>`,
		`Target (°F)<input name="target" value="86.00"`,
		`Tolerance (°F)<input name="tolerance" value="0.90"`,
		`Min delta (°F)<input name="mindelta" value="21.60"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Fahrenheit configuration is missing %q", want)
		}
	}
}

func TestFahrenheitConfigurationPostConvertsValuesToCelsius(t *testing.T) {
	h := testSecureHandler(t)
	form := url.Values{
		"posted":           {"true"},
		"temperature_unit": {"F"},
		"target":           {"86"},
		"tolerance":        {"1.8"},
		"mindelta":         {"18"},
	}
	req := httptest.NewRequest(http.MethodPost, "/config", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := req.ParseForm(); err != nil {
		t.Fatal(err)
	}

	h.processForm(req, h.ppc.config)
	cfg := h.ppc.config.cfg
	if cfg.TemperatureUnit != TemperatureFahrenheit {
		t.Errorf("temperature unit = %q, want F", cfg.TemperatureUnit)
	}
	if cfg.Target != 30 || cfg.Tolerance != 1 || cfg.DeltaT != 10 {
		t.Errorf("stored temperatures = target %g, tolerance %g, delta %g; want 30, 1, 10 °C",
			cfg.Target, cfg.Tolerance, cfg.DeltaT)
	}
}

func TestPairingPageOffersResetWhileStillPaired(t *testing.T) {
	h := testSecureHandler(t)
	dir := *h.ppc.config.dataDirectory
	writeAccessoryIdentity(t, dir)
	writePairing(t, dir, "stale-controller")

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

func TestPairingQRServesScalableSetupCode(t *testing.T) {
	server := NewServer(LocalHost, 0, &PoolPumpController{})
	req := httptest.NewRequest(http.MethodGet, "/qr.svg", nil)

	rec := httptest.NewRecorder()
	server.handler.qrSVGHandler(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("svg without a setup payload returned %d, want 503", rec.Code)
	}

	server.SetPairingURI("X-HM://0024K0Y3WHOME")
	rec = httptest.NewRecorder()
	server.handler.qrSVGHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("svg returned %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Fatalf("svg content type %q, want image/svg+xml", got)
	}

	body := rec.Body.String()
	// A viewBox sized in modules, with no pixel size on the svg element, is
	// what lets the page scale the code without blurring it.
	code, err := qrcode.New("X-HM://0024K0Y3WHOME", qrcode.Medium)
	if err != nil {
		t.Fatal(err)
	}
	modules := len(code.Bitmap())
	if want := fmt.Sprintf(`viewBox="0 0 %d %d"`, modules, modules); !strings.Contains(body, want) {
		t.Errorf("svg is not sized in QR modules, wanted %s in:\n%s", want, body)
	}
	if openTag, _, _ := strings.Cut(body, ">"); strings.Contains(openTag, "width=") {
		t.Errorf("svg pins a pixel size instead of scaling: %s", openTag)
	}
	if !strings.Contains(body, `<path fill="#000000" d="M`) {
		t.Errorf("svg draws no QR modules: %s", body)
	}
}

// The code has to differ with the payload, or every accessory would show the
// same unusable label.
func TestQRCodeSVGEncodesItsPayload(t *testing.T) {
	first, err := qrCodeSVG("X-HM://0024K0Y3WHOME")
	if err != nil {
		t.Fatal(err)
	}
	second, err := qrCodeSVG("X-HM://00526Q9UFERIC")
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(second) {
		t.Fatal("different setup payloads produced the same code")
	}
}

func TestPairingPageShowsModernSetupLabel(t *testing.T) {
	h := testSecureHandler(t)
	h.ppc.config.cfg.Pin = "10293847"
	h.pairingURI = "X-HM://00526Q9UFERIC"

	body := h.pairBody("")
	for _, want := range []string{
		`src="/qr.svg"`,                        // scannable code
		`<p class="setup-code">102-93-847</p>`, // digits, as printed on a label
		`href="X-HM://00526Q9UFERIC"`,          // tapping it opens the Home app
	} {
		if !strings.Contains(body, want) {
			t.Errorf("pairing page is missing %s:\n%s", want, body)
		}
	}
}

func TestPairingPageWithoutPayloadStillShowsTheCode(t *testing.T) {
	h := testSecureHandler(t)
	h.ppc.config.cfg.Pin = "10293847"

	body := h.pairBody("")
	if !strings.Contains(body, "102-93-847") {
		t.Errorf("pairing page hides the setup code when no payload is known:\n%s", body)
	}
	if strings.Contains(body, "/qr.svg") {
		t.Errorf("pairing page offers an empty code image:\n%s", body)
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
