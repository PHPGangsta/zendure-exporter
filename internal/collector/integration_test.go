package collector

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"zendure-exporter/internal/client"
	"zendure-exporter/internal/config"
)

// --- Mock server helpers ---

func newDelayedServer(delay time.Duration, payload map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
}

func newMalformedJSONServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"solarInputPower": 100, INVALID`)
	}))
}

func newPartialPayloadServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"electricLevel": 42}`)
	}))
}

// scrapeMetricsHTTP performs a full HTTP scrape through promhttp, returning the text body.
func scrapeMetricsHTTP(t *testing.T, col *Collector) string {
	t.Helper()
	reg := prometheus.NewRegistry()
	reg.MustRegister(col)

	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics returned status %d", rec.Code)
	}
	return rec.Body.String()
}

// --- Integration tests ---

func TestIntegration_MetricsEndpoint_ContainsExpectedMetrics(t *testing.T) {
	srv := newTestServer(devicePayload())
	defer srv.Close()

	cfg := newTestConfig(srv.URL)
	col := NewWithClient(cfg, testLogger(), client.New(cfg, testLogger()))

	body := scrapeMetricsHTTP(t, col)

	// Device metrics.
	expectedMetrics := []string{
		"zendure_solar_input_power_watts",
		"zendure_output_home_power_watts",
		"zendure_electric_level_percent",
		"zendure_pack_state",
		"zendure_enclosure_temperature_celsius",
		"zendure_battery_voltage_volts",
	}
	for _, m := range expectedMetrics {
		if !strings.Contains(body, m) {
			t.Errorf("/metrics output missing expected metric %q", m)
		}
	}

	// Channel metrics.
	if !strings.Contains(body, "zendure_solar_power_channel_watts") {
		t.Error("/metrics output missing channel metric")
	}

	// Pack metrics.
	if !strings.Contains(body, "zendure_pack_soc_level_percent") {
		t.Error("/metrics output missing pack metric")
	}

	// Self-metrics.
	selfMetrics := []string{
		"zendure_exporter_scrape_duration_seconds",
		"zendure_exporter_scrape_success",
		"zendure_last_success_timestamp_seconds",
	}
	for _, m := range selfMetrics {
		if !strings.Contains(body, m) {
			t.Errorf("/metrics output missing self-metric %q", m)
		}
	}

	// Labels.
	if !strings.Contains(body, `device_id="test_device"`) {
		t.Error("/metrics output missing device_id label")
	}
	if !strings.Contains(body, `device_model="SolarFlow800 Pro"`) {
		t.Error("/metrics output missing device_model label")
	}
	if !strings.Contains(body, `pack_sn="PACK001"`) {
		t.Error("/metrics output missing pack_sn label")
	}
}

func TestIntegration_MetricsEndpoint_Values(t *testing.T) {
	srv := newTestServer(devicePayload())
	defer srv.Close()

	cfg := newTestConfig(srv.URL)
	col := NewWithClient(cfg, testLogger(), client.New(cfg, testLogger()))

	body := scrapeMetricsHTTP(t, col)

	// Verify specific metric values appear in text output.
	if !strings.Contains(body, `zendure_solar_input_power_watts{device_id="test_device",device_model="SolarFlow800 Pro"} 450`) {
		t.Error("expected solar_input_power_watts=450 in /metrics output")
	}
	if !strings.Contains(body, `zendure_electric_level_percent{device_id="test_device",device_model="SolarFlow800 Pro"} 75`) {
		t.Error("expected electric_level_percent=75 in /metrics output")
	}
}

func TestIntegration_MultiDevice_SuccessTimeoutMalformed(t *testing.T) {
	// Device 1: success.
	goodPayload := map[string]any{
		"solarInputPower": 500,
		"electricLevel":   90,
	}
	goodSrv := newTestServer(goodPayload)
	defer goodSrv.Close()

	// Device 2: timeout (1s timeout config, 3s server delay).
	slowSrv := newDelayedServer(3*time.Second, map[string]any{"solarInputPower": 1})
	defer slowSrv.Close()

	// Device 3: malformed JSON.
	badSrv := newMalformedJSONServer()
	defer badSrv.Close()

	cfg := &config.Config{
		ListenAddr:                  "127.0.0.1",
		ListenPort:                  9854,
		DeviceRequestTimeoutSeconds: 1,
		Devices: []config.DeviceConfig{
			{ID: "good", Model: "SolarFlow800", BaseURL: goodSrv.URL, Enabled: true},
			{ID: "slow", Model: "SolarFlow800 Pro", BaseURL: slowSrv.URL, Enabled: true},
			{ID: "broken", Model: "SolarFlow2400 AC", BaseURL: badSrv.URL, Enabled: true},
		},
	}
	col := NewWithClient(cfg, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})), client.New(cfg, testLogger()))

	body := scrapeMetricsHTTP(t, col)

	// Good device should have metrics.
	if !strings.Contains(body, `zendure_solar_input_power_watts{device_id="good",device_model="SolarFlow800"} 500`) {
		t.Error("good device metric not found in output")
	}

	// Good device scrape_success = 1.
	if !strings.Contains(body, `zendure_exporter_scrape_success{device_id="good",device_model="SolarFlow800"} 1`) {
		t.Error("good device scrape_success=1 not found")
	}

	// Slow device scrape_success = 0 (timeout).
	if !strings.Contains(body, `zendure_exporter_scrape_success{device_id="slow",device_model="SolarFlow800 Pro"} 0`) {
		t.Error("slow device scrape_success=0 not found")
	}

	// Broken device scrape_success = 0 (malformed JSON).
	if !strings.Contains(body, `zendure_exporter_scrape_success{device_id="broken",device_model="SolarFlow2400 AC"} 0`) {
		t.Error("broken device scrape_success=0 not found")
	}

	// Slow/broken devices should NOT have device metrics.
	if strings.Contains(body, `zendure_solar_input_power_watts{device_id="slow"`) {
		t.Error("slow device should not have device metrics")
	}

	// Error counters should be present for failing devices.
	if !strings.Contains(body, `zendure_exporter_upstream_request_errors_total{device_id="slow"`) {
		t.Error("slow device error counter not found")
	}
	if !strings.Contains(body, `zendure_exporter_upstream_request_errors_total{device_id="broken"`) {
		t.Error("broken device error counter not found")
	}
}

func TestIntegration_PartialPayload(t *testing.T) {
	srv := newPartialPayloadServer()
	defer srv.Close()

	cfg := newTestConfig(srv.URL)
	col := NewWithClient(cfg, testLogger(), client.New(cfg, testLogger()))

	body := scrapeMetricsHTTP(t, col)

	// The single field present should be exposed.
	if !strings.Contains(body, `zendure_electric_level_percent{device_id="test_device",device_model="SolarFlow800 Pro"} 42`) {
		t.Error("expected electric_level_percent=42 from partial payload")
	}

	// Scrape should still be considered successful.
	if !strings.Contains(body, `zendure_exporter_scrape_success{device_id="test_device",device_model="SolarFlow800 Pro"} 1`) {
		t.Error("partial payload should still be a successful scrape")
	}

	// Fields not in the payload should not appear.
	if strings.Contains(body, "zendure_solar_input_power_watts") {
		t.Error("missing fields from partial payload should not produce metrics")
	}
}

func TestIntegration_SelfMetrics_AllPresent(t *testing.T) {
	srv := newTestServer(devicePayload())
	defer srv.Close()

	cfg := newTestConfig(srv.URL)
	col := NewWithClient(cfg, testLogger(), client.New(cfg, testLogger()))

	body := scrapeMetricsHTTP(t, col)

	requiredSelfMetrics := []string{
		"zendure_exporter_scrape_duration_seconds",
		"zendure_exporter_scrape_success",
		"zendure_exporter_upstream_request_errors_total",
		"zendure_last_success_timestamp_seconds",
		"zendure_exporter_unknown_fields_total",
	}
	for _, m := range requiredSelfMetrics {
		if !strings.Contains(body, m) {
			t.Errorf("missing self-metric %q in /metrics output", m)
		}
	}
}

func TestIntegration_SelfMetrics_OnError(t *testing.T) {
	srv := newErrorServer(500)
	defer srv.Close()

	cfg := newTestConfig(srv.URL)
	col := NewWithClient(cfg, testLogger(), client.New(cfg, testLogger()))

	body := scrapeMetricsHTTP(t, col)

	// scrape_success should be 0.
	if !strings.Contains(body, `zendure_exporter_scrape_success{device_id="test_device",device_model="SolarFlow800 Pro"} 0`) {
		t.Error("scrape_success should be 0 on error")
	}

	// Error counter should be 1.
	if !strings.Contains(body, `zendure_exporter_upstream_request_errors_total{device_id="test_device",device_model="SolarFlow800 Pro"} 1`) {
		t.Error("error counter should be 1")
	}

	// Last success timestamp should NOT be present (never succeeded).
	if strings.Contains(body, `zendure_last_success_timestamp_seconds{device_id="test_device"`) {
		t.Error("last success timestamp should not be present when device never succeeded")
	}
}

func TestIntegration_DiscoveryMode_UnknownFieldsInOutput(t *testing.T) {
	payload := map[string]any{
		"solarInputPower": 100,
		"mysteryField":    77,
		"anotherNew":      99.5,
	}
	srv := newTestServer(payload)
	defer srv.Close()

	cfg := newTestConfig(srv.URL)
	cfg.DiscoveryMode = true
	col := NewWithClient(cfg, testLogger(), client.New(cfg, testLogger()))

	body := scrapeMetricsHTTP(t, col)

	if !strings.Contains(body, "zendure_unknown_property") {
		t.Error("discovery mode should expose zendure_unknown_property metric")
	}
	if !strings.Contains(body, `field="mysteryfield"`) {
		t.Error("unknown field 'mysteryfield' not found in discovery output")
	}
}
