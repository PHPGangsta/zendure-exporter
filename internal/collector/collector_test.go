package collector

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus "github.com/prometheus/client_model/go"

	"github.com/PHPGangsta/zendure-exporter/internal/client"
	"github.com/PHPGangsta/zendure-exporter/internal/config"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

// collectMetrics gathers all metrics from the collector into a map keyed by metric name.
func collectMetrics(t *testing.T, col *Collector) map[string]*io_prometheus.MetricFamily {
	t.Helper()
	reg := prometheus.NewRegistry()
	reg.MustRegister(col)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}

	result := make(map[string]*io_prometheus.MetricFamily)
	for _, f := range families {
		result[f.GetName()] = f
	}
	return result
}

// devicePayload returns a realistic JSON payload map used by both unit and integration tests.
func devicePayload() map[string]any {
	return map[string]any{
		"solarInputPower": 450,
		"outputHomePower": 200,
		"electricLevel":   75,
		"packState":       1,
		"hyperTmp":        3011, // (3011-2731)/10 = 28.0
		"BatVolt":         5200, // 52.00 V
		"solarPower1":     150,
		"solarPower2":     300,
		"packData": []any{
			map[string]any{
				"sn":       "PACK001",
				"socLevel": 80,
				"power":    100,
				"maxTemp":  2981, // (2981-2731)/10 = 25.0
				"maxVol":   342,  // 3.42 V
				"minVol":   338,  // 3.38 V
			},
		},
	}
}

// newTestServer starts an httptest.Server that serves the given payload as JSON.
// Used by integration tests and by unit tests that require real HTTP behaviour.
func newTestServer(payload map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}))
}

// newErrorServer starts an httptest.Server that always returns HTTP 500.
func newErrorServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintln(w, "error")
	}))
}

// newTestConfig returns a single-device config pointing at baseURL.
func newTestConfig(baseURL string) *config.Config {
	return &config.Config{
		ListenAddr:                  "127.0.0.1",
		ListenPort:                  9854,
		DeviceRequestTimeoutSeconds: 5,
		Devices: []config.DeviceConfig{
			{ID: "test_device", Model: "SolarFlow800 Pro", BaseURL: baseURL, Enabled: true},
		},
	}
}

// --- Mock fetcher ---

// mockFetcher implements Fetcher for unit tests without a real HTTP server.
// Responses are keyed by device ID; if a device has no entry FetchDevice returns an error.
type mockFetcher struct {
	results map[string]mockFetchResult
}

type mockFetchResult struct {
	data *client.DeviceData
	err  error
}

func newMockFetcher(results map[string]mockFetchResult) *mockFetcher {
	return &mockFetcher{results: results}
}

func (m *mockFetcher) FetchDevice(dev config.DeviceConfig) (*client.DeviceData, error) {
	r, ok := m.results[dev.ID]
	if !ok {
		return nil, fmt.Errorf("mockFetcher: no result configured for device %q", dev.ID)
	}
	return r.data, r.err
}

// mockSuccess returns a successful mockFetchResult with the provided DeviceData.
func mockSuccess(data *client.DeviceData) mockFetchResult {
	return mockFetchResult{data: data}
}

// mockHTTPError returns a failing mockFetchResult using ErrHTTPError (status 500).
// errorTypeOf will classify this as "http_error", matching what the real error server returns.
func mockHTTPError(deviceID string) mockFetchResult {
	return mockFetchResult{err: &client.ErrHTTPError{
		URL:    "http://mock/" + deviceID,
		Status: 500,
		Body:   "error",
	}}
}

// mockDeviceData returns a *client.DeviceData whose values match devicePayload() after
// the client mapper has applied all field mappings and unit conversions.
func mockDeviceData(deviceID, deviceModel string) *client.DeviceData {
	return &client.DeviceData{
		DeviceID:    deviceID,
		DeviceModel: deviceModel,
		Metrics: map[string]float64{
			"zendure_solar_input_power_watts":      450,
			"zendure_output_home_power_watts":      200,
			"zendure_electric_level_percent":       75,
			"zendure_pack_state":                   1,
			"zendure_enclosure_temperature_celsius": 28.0,
			"zendure_battery_voltage_volts":        52.0,
		},
		ChannelMetrics: map[string]map[string]float64{
			"zendure_solar_power_channel_watts": {"1": 150, "2": 300},
		},
		BatteryPacks: []client.BatteryPackData{
			{
				SerialNumber: "PACK001",
				Metrics: map[string]float64{
					"zendure_pack_soc_level_percent":      80,
					"zendure_pack_power_watts":            100,
					"zendure_pack_temperature_celsius":    25.0,
					"zendure_pack_max_cell_voltage_volts": 3.42,
					"zendure_pack_min_cell_voltage_volts": 3.38,
				},
			},
		},
		UnknownFields: map[string]float64{},
	}
}

// newMockConfig returns a single-device config for use with mockFetcher.
// The BaseURL is irrelevant when the collector is initialised with a mock fetcher.
func newMockConfig() *config.Config {
	return &config.Config{
		ListenAddr:                  "127.0.0.1",
		ListenPort:                  9854,
		DeviceRequestTimeoutSeconds: 5,
		Devices: []config.DeviceConfig{
			{ID: "test_device", Model: "SolarFlow800 Pro", BaseURL: "http://ignored", Enabled: true},
		},
	}
}

// --- Unit tests ---

func TestCollector_BasicDeviceMetrics(t *testing.T) {
	cfg := newMockConfig()
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockSuccess(mockDeviceData("test_device", "SolarFlow800 Pro")),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)

	assertGaugeValue(t, metrics, "zendure_solar_input_power_watts", 450, "device_id", "test_device")
	assertGaugeValue(t, metrics, "zendure_output_home_power_watts", 200, "device_id", "test_device")
	assertGaugeValue(t, metrics, "zendure_electric_level_percent", 75, "device_id", "test_device")
	assertGaugeValue(t, metrics, "zendure_pack_state", 1, "device_id", "test_device")
}

func TestCollector_TemperatureConversion(t *testing.T) {
	cfg := newMockConfig()
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockSuccess(mockDeviceData("test_device", "SolarFlow800 Pro")),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)
	assertGaugeValueApprox(t, metrics, "zendure_enclosure_temperature_celsius", 28.0, 0.1, "device_id", "test_device")
}

func TestCollector_VoltageConversion(t *testing.T) {
	cfg := newMockConfig()
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockSuccess(mockDeviceData("test_device", "SolarFlow800 Pro")),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)
	assertGaugeValueApprox(t, metrics, "zendure_battery_voltage_volts", 52.0, 0.01, "device_id", "test_device")
}

func TestCollector_ChannelMetrics(t *testing.T) {
	cfg := newMockConfig()
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockSuccess(mockDeviceData("test_device", "SolarFlow800 Pro")),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)

	fam, ok := metrics["zendure_solar_power_channel_watts"]
	if !ok {
		t.Fatal("missing metric family zendure_solar_power_channel_watts")
	}

	found := map[string]float64{}
	for _, m := range fam.GetMetric() {
		for _, l := range m.GetLabel() {
			if l.GetName() == "channel" {
				found[l.GetValue()] = m.GetGauge().GetValue()
			}
		}
	}

	if v, ok := found["1"]; !ok || v != 150 {
		t.Errorf("channel 1: got %v, want 150", v)
	}
	if v, ok := found["2"]; !ok || v != 300 {
		t.Errorf("channel 2: got %v, want 300", v)
	}
}

func TestCollector_BatteryPackMetrics(t *testing.T) {
	cfg := newMockConfig()
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockSuccess(mockDeviceData("test_device", "SolarFlow800 Pro")),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)

	assertGaugeWithLabel(t, metrics, "zendure_pack_soc_level_percent", "pack_sn", "PACK001", 80)
	assertGaugeWithLabel(t, metrics, "zendure_pack_power_watts", "pack_sn", "PACK001", 100)
	assertGaugeWithLabelApprox(t, metrics, "zendure_pack_temperature_celsius", "pack_sn", "PACK001", 25.0, 0.1)
	assertGaugeWithLabelApprox(t, metrics, "zendure_pack_max_cell_voltage_volts", "pack_sn", "PACK001", 3.42, 0.01)
	assertGaugeWithLabelApprox(t, metrics, "zendure_pack_min_cell_voltage_volts", "pack_sn", "PACK001", 3.38, 0.01)
}

func TestCollector_ScrapeSuccessOnSuccess(t *testing.T) {
	cfg := newMockConfig()
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockSuccess(mockDeviceData("test_device", "SolarFlow800 Pro")),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)
	assertGaugeValue(t, metrics, "zendure_exporter_scrape_success", 1, "device_id", "test_device")
}

func TestCollector_ScrapeSuccessOnFailure(t *testing.T) {
	cfg := newMockConfig()
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockHTTPError("test_device"),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)

	assertGaugeValue(t, metrics, "zendure_exporter_scrape_success", 0, "device_id", "test_device")
	assertCounterValue(t, metrics, "zendure_exporter_upstream_request_errors_total", 1, "device_id", "test_device")
}

func TestCollector_NoDeviceMetricsOnFailure(t *testing.T) {
	cfg := newMockConfig()
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockHTTPError("test_device"),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)

	if _, ok := metrics["zendure_solar_input_power_watts"]; ok {
		t.Error("device metrics should not be emitted on failure")
	}
}

func TestCollector_ScrapeDuration(t *testing.T) {
	cfg := newMockConfig()
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockSuccess(mockDeviceData("test_device", "SolarFlow800 Pro")),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)

	fam, ok := metrics["zendure_exporter_scrape_duration_seconds"]
	if !ok {
		t.Fatal("missing scrape duration metric")
	}
	val := fam.GetMetric()[0].GetGauge().GetValue()
	if val < 0 {
		t.Errorf("scrape duration should be >= 0, got %v", val)
	}
}

func TestCollector_LastSuccessTimestamp(t *testing.T) {
	cfg := newMockConfig()
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockSuccess(mockDeviceData("test_device", "SolarFlow800 Pro")),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)

	fam, ok := metrics["zendure_last_success_timestamp_seconds"]
	if !ok {
		t.Fatal("missing last success timestamp metric")
	}
	val := fam.GetMetric()[0].GetGauge().GetValue()
	if val <= 0 {
		t.Errorf("last success timestamp should be > 0, got %v", val)
	}
}

func TestCollector_NoLastSuccessOnFailure(t *testing.T) {
	cfg := newMockConfig()
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockHTTPError("test_device"),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)

	if fam, ok := metrics["zendure_last_success_timestamp_seconds"]; ok {
		if len(fam.GetMetric()) > 0 {
			t.Error("last success timestamp should not be emitted when device never succeeded")
		}
	}
}

func TestCollector_MultiDevice_OneFailsOtherSucceeds(t *testing.T) {
	cfg := &config.Config{
		ListenAddr:                  "127.0.0.1",
		ListenPort:                  9854,
		DeviceRequestTimeoutSeconds: 5,
		Devices: []config.DeviceConfig{
			{ID: "good_device", Model: "SolarFlow800", BaseURL: "http://ignored", Enabled: true},
			{ID: "bad_device", Model: "SolarFlow800 Pro", BaseURL: "http://ignored", Enabled: true},
		},
	}
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"good_device": mockSuccess(mockDeviceData("good_device", "SolarFlow800")),
		"bad_device":  mockHTTPError("bad_device"),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)

	assertGaugeValue(t, metrics, "zendure_solar_input_power_watts", 450, "device_id", "good_device")

	fam := metrics["zendure_exporter_scrape_success"]
	if fam == nil {
		t.Fatal("missing scrape_success metric")
	}

	successByDevice := map[string]float64{}
	for _, m := range fam.GetMetric() {
		for _, l := range m.GetLabel() {
			if l.GetName() == "device_id" {
				successByDevice[l.GetValue()] = m.GetGauge().GetValue()
			}
		}
	}
	if successByDevice["good_device"] != 1 {
		t.Errorf("good_device scrape_success: got %v, want 1", successByDevice["good_device"])
	}
	if successByDevice["bad_device"] != 0 {
		t.Errorf("bad_device scrape_success: got %v, want 0", successByDevice["bad_device"])
	}
}

func TestCollector_DisabledDeviceSkipped(t *testing.T) {
	cfg := &config.Config{
		ListenAddr:                  "127.0.0.1",
		ListenPort:                  9854,
		DeviceRequestTimeoutSeconds: 5,
		Devices: []config.DeviceConfig{
			{ID: "disabled_device", Model: "SolarFlow800", BaseURL: "http://ignored", Enabled: false},
		},
	}
	// Empty mock: if FetchDevice is called the test will get an error metric, making
	// the "no device metrics" assertion below fail — which is the desired verification.
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)

	if _, ok := metrics["zendure_solar_input_power_watts"]; ok {
		t.Error("disabled device should not produce metrics")
	}
	if _, ok := metrics["zendure_exporter_scrape_duration_seconds"]; !ok {
		t.Error("scrape duration should always be present")
	}
}

func TestCollector_DiscoveryMode(t *testing.T) {
	cfg := newMockConfig()
	cfg.DiscoveryMode = true
	data := mockDeviceData("test_device", "SolarFlow800 Pro")
	data.UnknownFields = map[string]float64{
		"unknownfieldxyz": 42,
		"anothernewfield": 99.5,
	}
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockSuccess(data),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)

	fam, ok := metrics["zendure_unknown_property"]
	if !ok {
		t.Fatal("missing discovery metric zendure_unknown_property")
	}

	found := map[string]float64{}
	for _, m := range fam.GetMetric() {
		for _, l := range m.GetLabel() {
			if l.GetName() == "field" {
				found[l.GetValue()] = m.GetGauge().GetValue()
			}
		}
	}

	if len(found) < 2 {
		t.Errorf("expected at least 2 unknown fields, got %d: %v", len(found), found)
	}
}

func TestCollector_DiscoveryModeOff(t *testing.T) {
	cfg := newMockConfig()
	cfg.DiscoveryMode = false
	data := mockDeviceData("test_device", "SolarFlow800 Pro")
	data.UnknownFields = map[string]float64{"unknownfieldxyz": 42}
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockSuccess(data),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)

	if _, ok := metrics["zendure_unknown_property"]; ok {
		t.Error("discovery metrics should not be present when discovery_mode is off")
	}
}

func TestCollector_ErrorCounterIncrements(t *testing.T) {
	cfg := newMockConfig()
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockHTTPError("test_device"),
	})), WithVersion("v1.0.0"))

	// First scrape.
	metrics1 := collectMetrics(t, col)
	assertCounterValue(t, metrics1, "zendure_exporter_upstream_request_errors_total", 1, "device_id", "test_device")

	// Second scrape (re-register since Prometheus registry doesn't allow double registration).
	reg2 := prometheus.NewRegistry()
	reg2.MustRegister(col)
	families2, err := reg2.Gather()
	if err != nil {
		t.Fatalf("failed to gather from registry: %v", err)
	}
	m2 := make(map[string]*io_prometheus.MetricFamily)
	for _, f := range families2 {
		m2[f.GetName()] = f
	}
	assertCounterValue(t, m2, "zendure_exporter_upstream_request_errors_total", 2, "device_id", "test_device")
}

func TestCollector_DeviceLabelsPresent(t *testing.T) {
	cfg := newMockConfig()
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockSuccess(mockDeviceData("test_device", "SolarFlow800 Pro")),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)

	fam, ok := metrics["zendure_solar_input_power_watts"]
	if !ok {
		t.Fatal("missing metric")
	}

	m := fam.GetMetric()[0]
	labels := map[string]string{}
	for _, l := range m.GetLabel() {
		labels[l.GetName()] = l.GetValue()
	}

	if labels["device_id"] != "test_device" {
		t.Errorf("device_id label: got %q, want %q", labels["device_id"], "test_device")
	}
	if labels["device_model"] != "SolarFlow800 Pro" {
		t.Errorf("device_model label: got %q, want %q", labels["device_model"], "SolarFlow800 Pro")
	}
}

func TestCollector_Describe(t *testing.T) {
	cfg := &config.Config{
		ListenAddr:                  "127.0.0.1",
		ListenPort:                  9854,
		DeviceRequestTimeoutSeconds: 5,
		DiscoveryMode:               true,
		Devices: []config.DeviceConfig{
			{ID: "test", Model: "SolarFlow800", BaseURL: "http://localhost", Enabled: true},
		},
	}
	col := New(cfg, testLogger(), WithVersion("v1.0.0"))

	ch := make(chan *prometheus.Desc, 200)
	col.Describe(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}

	// 36 device + 1 channel + 9 pack + 1 discovery + 5 self = 52
	if count < 50 {
		t.Errorf("expected at least 50 descriptors, got %d", count)
	}
}

func TestCollector_RWConfigMetrics(t *testing.T) {
	cfg := newMockConfig()
	data := &client.DeviceData{
		DeviceID:    "test_device",
		DeviceModel: "SolarFlow800 Pro",
		Metrics: map[string]float64{
			"zendure_ac_mode":           2,
			"zendure_input_limit_watts":  800,
			"zendure_output_limit_watts": 600,
			"zendure_soc_set_percent":   90,
			"zendure_min_soc_percent":   10,
		},
		ChannelMetrics: map[string]map[string]float64{},
		UnknownFields:  map[string]float64{},
	}
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockSuccess(data),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)
	assertGaugeValue(t, metrics, "zendure_ac_mode", 2, "device_id", "test_device")
	assertGaugeValue(t, metrics, "zendure_input_limit_watts", 800, "device_id", "test_device")
	assertGaugeValue(t, metrics, "zendure_output_limit_watts", 600, "device_id", "test_device")
	assertGaugeValue(t, metrics, "zendure_soc_set_percent", 90, "device_id", "test_device")
	assertGaugeValue(t, metrics, "zendure_min_soc_percent", 10, "device_id", "test_device")
}

func TestCollector_CircuitBreaker_OpensAfterThresholdFailures(t *testing.T) {
	cfg := newMockConfig()
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockHTTPError("test_device"),
	})), WithVersion("v1.0.0"))

	// Exhaust the threshold.
	for i := range circuitBreakerThreshold {
		reg := prometheus.NewRegistry()
		reg.MustRegister(col)
		_, err := reg.Gather()
		if err != nil {
			t.Fatalf("gather %d failed: %v", i, err)
		}
	}

	// Next scrape: circuit should be open (no fetch attempted, circuit_breaker_open=1).
	metrics := collectMetrics(t, col)
	assertGaugeValue(t, metrics, "zendure_exporter_circuit_breaker_open", 1, "device_id", "test_device")
	assertGaugeValue(t, metrics, "zendure_exporter_scrape_success", 0, "device_id", "test_device")

	// fetchDuration should NOT be present when circuit is open (no fetch happened).
	if fam, ok := metrics["zendure_exporter_device_fetch_duration_seconds"]; ok {
		for _, m := range fam.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "device_id" && l.GetValue() == "test_device" {
					t.Error("fetch duration should not be emitted when circuit is open")
				}
			}
		}
	}
}

func TestCollector_CircuitBreaker_ClosedOnSuccess(t *testing.T) {
	cfg := newMockConfig()
	col := New(cfg, testLogger(), WithFetcher(newMockFetcher(map[string]mockFetchResult{
		"test_device": mockSuccess(mockDeviceData("test_device", "SolarFlow800 Pro")),
	})), WithVersion("v1.0.0"))

	metrics := collectMetrics(t, col)
	assertGaugeValue(t, metrics, "zendure_exporter_circuit_breaker_open", 0, "device_id", "test_device")
	assertGaugeValue(t, metrics, "zendure_exporter_scrape_success", 1, "device_id", "test_device")
}

// --- Helpers ---

func assertGaugeValue(t *testing.T, metrics map[string]*io_prometheus.MetricFamily, name string, expected float64, labelName, labelValue string) {
	t.Helper()
	fam, ok := metrics[name]
	if !ok {
		t.Errorf("missing metric %s", name)
		return
	}
	for _, m := range fam.GetMetric() {
		for _, l := range m.GetLabel() {
			if l.GetName() == labelName && l.GetValue() == labelValue {
				got := m.GetGauge().GetValue()
				if got != expected {
					t.Errorf("%s{%s=%q}: got %v, want %v", name, labelName, labelValue, got, expected)
				}
				return
			}
		}
	}
	t.Errorf("%s: no metric with %s=%q", name, labelName, labelValue)
}

func assertGaugeValueApprox(t *testing.T, metrics map[string]*io_prometheus.MetricFamily, name string, expected, tolerance float64, labelName, labelValue string) {
	t.Helper()
	fam, ok := metrics[name]
	if !ok {
		t.Errorf("missing metric %s", name)
		return
	}
	for _, m := range fam.GetMetric() {
		for _, l := range m.GetLabel() {
			if l.GetName() == labelName && l.GetValue() == labelValue {
				got := m.GetGauge().GetValue()
				if diff := got - expected; diff > tolerance || diff < -tolerance {
					t.Errorf("%s{%s=%q}: got %v, want %v (±%v)", name, labelName, labelValue, got, expected, tolerance)
				}
				return
			}
		}
	}
	t.Errorf("%s: no metric with %s=%q", name, labelName, labelValue)
}

func assertGaugeWithLabel(t *testing.T, metrics map[string]*io_prometheus.MetricFamily, name, labelName, labelValue string, expected float64) {
	t.Helper()
	assertGaugeValue(t, metrics, name, expected, labelName, labelValue)
}

func assertGaugeWithLabelApprox(t *testing.T, metrics map[string]*io_prometheus.MetricFamily, name, labelName, labelValue string, expected, tolerance float64) {
	t.Helper()
	assertGaugeValueApprox(t, metrics, name, expected, tolerance, labelName, labelValue)
}

func assertCounterValue(t *testing.T, metrics map[string]*io_prometheus.MetricFamily, name string, expected float64, labelName, labelValue string) {
	t.Helper()
	fam, ok := metrics[name]
	if !ok {
		t.Errorf("missing metric %s", name)
		return
	}
	for _, m := range fam.GetMetric() {
		for _, l := range m.GetLabel() {
			if l.GetName() == labelName && l.GetValue() == labelValue {
				got := m.GetCounter().GetValue()
				if got != expected {
					t.Errorf("%s{%s=%q}: got %v, want %v", name, labelName, labelValue, got, expected)
				}
				return
			}
		}
	}
	t.Errorf("%s: no metric with %s=%q", name, labelName, labelValue)
}
