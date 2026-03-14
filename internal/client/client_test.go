package client

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"zendure-exporter/internal/config"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func testConfig(url string, discovery bool) *config.Config {
	return &config.Config{
		ListenAddr:                  "0.0.0.0",
		ListenPort:                  9854,
		DiscoveryMode:               discovery,
		DeviceRequestTimeoutSeconds: 5,
		Devices: []config.DeviceConfig{
			{ID: "test_device", Model: "SolarFlow800 Pro", BaseURL: url, Enabled: true},
		},
	}
}

func testDevice(url string) config.DeviceConfig {
	return config.DeviceConfig{
		ID:      "test_device",
		Model:   "SolarFlow800 Pro",
		BaseURL: url,
		Enabled: true,
	}
}

// --- Conversion function tests ---

func TestConvertTemperature(t *testing.T) {
	tests := []struct {
		name     string
		raw      float64
		expected float64
	}{
		{"0°C (2731 raw)", 2731, 0.0},
		{"25°C (2981 raw)", 2981, 25.0},
		{"-10°C (2631 raw)", 2631, -10.0},
		{"100°C (3731 raw)", 3731, 100.0},
		{"20.5°C (2936 raw)", 2936, 20.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertTemperature(tt.raw)
			if math.Abs(got-tt.expected) > 0.01 {
				t.Errorf("convertTemperature(%v) = %v, want %v", tt.raw, got, tt.expected)
			}
		})
	}
}

func TestConvertCentiVolts(t *testing.T) {
	tests := []struct {
		raw      float64
		expected float64
	}{
		{5200, 52.0},
		{330, 3.3},
		{0, 0.0},
		{4815, 48.15},
	}
	for _, tt := range tests {
		got := convertCentiVolts(tt.raw)
		if math.Abs(got-tt.expected) > 0.001 {
			t.Errorf("convertCentiVolts(%v) = %v, want %v", tt.raw, got, tt.expected)
		}
	}
}

func TestConvertBatteryCurrent(t *testing.T) {
	tests := []struct {
		name     string
		raw      float64
		expected float64
	}{
		{"positive 5A (50 raw)", 50, 5.0},
		{"zero", 0, 0.0},
		{"negative -5A (65486 raw / 0xFFCE)", 65486, -5.0},
		{"negative -0.1A (65535 raw / 0xFFFF)", 65535, -0.1},
		{"max positive 3276.7A (32767 raw)", 32767, 3276.7},
		{"negative -3276.8A (32768 raw / 0x8000)", 32768, -3276.8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertBatteryCurrent(tt.raw)
			if math.Abs(got-tt.expected) > 0.01 {
				t.Errorf("convertBatteryCurrent(%v) = %v, want %v", tt.raw, got, tt.expected)
			}
		})
	}
}

// --- toFloat64 tests ---

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    float64
		wantErr bool
	}{
		{"float64", 42.5, 42.5, false},
		{"float64 zero", 0.0, 0.0, false},
		{"float64 negative", -10.5, -10.5, false},
		{"bool true", true, 1.0, false},
		{"bool false", false, 0.0, false},
		{"int", int(42), 42.0, false},
		{"int64", int64(999), 999.0, false},
		{"string", "hello", 0, true},
		{"nil", nil, 0, true},
		{"slice", []int{1, 2}, 0, true},
		{"NaN", math.NaN(), 0, true},
		{"Inf", math.Inf(1), 0, true},
		{"-Inf", math.Inf(-1), 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toFloat64(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("toFloat64(%v) expected error, got %v", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("toFloat64(%v) unexpected error: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("toFloat64(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// --- sanitizeFieldName tests ---

func TestSanitizeFieldName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simpleField", "simplefield"},
		{"CamelCase", "camelcase"},
		{"with spaces", "with_spaces"},
		{"special!@#chars", "special_chars"},
		{"multiple___underscores", "multiple_underscores"},
		{"_leading_trailing_", "leading_trailing"},
		{"123numeric", "123numeric"},
		{"ALLCAPS", "allcaps"},
		{"a.b.c", "a_b_c"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFieldName(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeFieldName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// --- parsePayload tests ---

func TestParsePayload_BasicDeviceMetrics(t *testing.T) {
	payload := `{
		"solarInputPower": 1500,
		"outputHomePower": 800,
		"electricLevel": 75,
		"packState": 1,
		"pass": 0,
		"rssi": -65
	}`

	c := New(testConfig("http://unused", false), testLogger())
	dev := testDevice("http://unused")

	data, err := c.parsePayload(dev, []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertMetric(t, data.Metrics, "zendure_solar_input_power_watts", 1500)
	assertMetric(t, data.Metrics, "zendure_output_home_power_watts", 800)
	assertMetric(t, data.Metrics, "zendure_electric_level_percent", 75)
	assertMetric(t, data.Metrics, "zendure_pack_state", 1)
	assertMetric(t, data.Metrics, "zendure_pass", 0)
	assertMetric(t, data.Metrics, "zendure_rssi_dbm", -65)

	if data.DeviceID != "test_device" {
		t.Errorf("DeviceID = %q, want %q", data.DeviceID, "test_device")
	}
	if data.DeviceModel != "SolarFlow800 Pro" {
		t.Errorf("DeviceModel = %q, want %q", data.DeviceModel, "SolarFlow800 Pro")
	}
}

func TestParsePayload_TemperatureConversion(t *testing.T) {
	payload := `{"hyperTmp": 2981}`

	c := New(testConfig("http://unused", false), testLogger())
	data, err := c.parsePayload(testDevice("http://unused"), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertMetricApprox(t, data.Metrics, "zendure_enclosure_temperature_celsius", 25.0, 0.01)
}

func TestParsePayload_VoltageConversion(t *testing.T) {
	payload := `{
		"BatVolt": 5200,
		"packData": [{
			"sn": "PACK001",
			"totalVol": 4860
		}]
	}`

	c := New(testConfig("http://unused", false), testLogger())
	data, err := c.parsePayload(testDevice("http://unused"), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertMetricApprox(t, data.Metrics, "zendure_battery_voltage_volts", 52.0, 0.01)

	if len(data.BatteryPacks) != 1 {
		t.Fatalf("expected 1 battery pack, got %d", len(data.BatteryPacks))
	}
	assertMetricApprox(t, data.BatteryPacks[0].Metrics, "zendure_pack_total_voltage_volts", 48.60, 0.01)
}

func TestParsePayload_SolarChannels(t *testing.T) {
	payload := `{
		"solarPower1": 300,
		"solarPower2": 250,
		"solarPower4": 100
	}`

	c := New(testConfig("http://unused", false), testLogger())
	data, err := c.parsePayload(testDevice("http://unused"), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	channels := data.ChannelMetrics["zendure_solar_power_channel_watts"]
	if channels == nil {
		t.Fatal("expected channel metrics for zendure_solar_power_channel_watts")
	}

	assertMapValue(t, channels, "1", 300)
	assertMapValue(t, channels, "2", 250)
	assertMapValue(t, channels, "4", 100)

	if _, ok := channels["3"]; ok {
		t.Error("channel 3 should not be present")
	}
}

func TestParsePayload_BatteryPacks(t *testing.T) {
	payload := `{
		"packData": [
			{
				"sn": "EB04A001",
				"socLevel": 80,
				"state": 1,
				"power": 500,
				"maxTemp": 2981,
				"totalVol": 4860,
				"batcur": 50,
				"maxVol": 330,
				"minVol": 320
			},
			{
				"sn": "EB04A002",
				"socLevel": 60,
				"state": 2,
				"batcur": 65486
			}
		]
	}`

	c := New(testConfig("http://unused", false), testLogger())
	data, err := c.parsePayload(testDevice("http://unused"), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data.BatteryPacks) != 2 {
		t.Fatalf("expected 2 battery packs, got %d", len(data.BatteryPacks))
	}

	pack1 := data.BatteryPacks[0]
	if pack1.SerialNumber != "EB04A001" {
		t.Errorf("pack1 SN = %q, want %q", pack1.SerialNumber, "EB04A001")
	}
	assertMetric(t, pack1.Metrics, "zendure_pack_soc_level_percent", 80)
	assertMetric(t, pack1.Metrics, "zendure_pack_state_enum", 1)
	assertMetric(t, pack1.Metrics, "zendure_pack_power_watts", 500)
	assertMetricApprox(t, pack1.Metrics, "zendure_pack_temperature_celsius", 25.0, 0.01)
	assertMetricApprox(t, pack1.Metrics, "zendure_pack_current_amperes", 5.0, 0.01)
	assertMetricApprox(t, pack1.Metrics, "zendure_pack_total_voltage_volts", 48.60, 0.01)
	assertMetricApprox(t, pack1.Metrics, "zendure_pack_max_cell_voltage_volts", 3.3, 0.01)
	assertMetricApprox(t, pack1.Metrics, "zendure_pack_min_cell_voltage_volts", 3.2, 0.01)

	pack2 := data.BatteryPacks[1]
	if pack2.SerialNumber != "EB04A002" {
		t.Errorf("pack2 SN = %q, want %q", pack2.SerialNumber, "EB04A002")
	}
	assertMetric(t, pack2.Metrics, "zendure_pack_soc_level_percent", 60)
	assertMetricApprox(t, pack2.Metrics, "zendure_pack_current_amperes", -5.0, 0.01)
}

func TestParsePayload_BatteryPackMissingSN(t *testing.T) {
	payload := `{
		"packData": [
			{"socLevel": 80, "state": 1}
		]
	}`

	c := New(testConfig("http://unused", false), testLogger())
	data, err := c.parsePayload(testDevice("http://unused"), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data.BatteryPacks) != 0 {
		t.Errorf("expected 0 packs (missing SN), got %d", len(data.BatteryPacks))
	}
}

func TestParsePayload_DiscoveryMode(t *testing.T) {
	payload := `{
		"solarInputPower": 1500,
		"unknownNumeric": 42,
		"unknownString": "hello",
		"timestamp": 123456
	}`

	c := New(testConfig("http://unused", true), testLogger())
	data, err := c.parsePayload(testDevice("http://unused"), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Known field parsed normally
	assertMetric(t, data.Metrics, "zendure_solar_input_power_watts", 1500)

	// Unknown numeric exposed
	if val, ok := data.UnknownFields["unknownnumeric"]; !ok {
		t.Error("expected unknownnumeric in UnknownFields")
	} else if val != 42 {
		t.Errorf("unknownnumeric = %v, want 42", val)
	}

	// Unknown string not exposed
	if _, ok := data.UnknownFields["unknownstring"]; ok {
		t.Error("non-numeric unknown field should not be in UnknownFields")
	}

	// Ignored field not in discovery
	if _, ok := data.UnknownFields["timestamp"]; ok {
		t.Error("ignored field 'timestamp' should not appear in UnknownFields")
	}
}

func TestParsePayload_DiscoveryModeOff(t *testing.T) {
	payload := `{
		"solarInputPower": 1500,
		"unknownNumeric": 42
	}`

	c := New(testConfig("http://unused", false), testLogger())
	data, err := c.parsePayload(testDevice("http://unused"), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data.UnknownFields) != 0 {
		t.Errorf("discovery mode off: expected no unknown fields, got %d", len(data.UnknownFields))
	}
}

func TestParsePayload_InvalidJSON(t *testing.T) {
	c := New(testConfig("http://unused", false), testLogger())
	_, err := c.parsePayload(testDevice("http://unused"), []byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParsePayload_EmptyObject(t *testing.T) {
	c := New(testConfig("http://unused", false), testLogger())
	data, err := c.parsePayload(testDevice("http://unused"), []byte("{}"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data.Metrics) != 0 {
		t.Errorf("expected no metrics from empty object, got %d", len(data.Metrics))
	}
}

func TestParsePayload_RWConfigFields(t *testing.T) {
	payload := `{
		"acMode": 2,
		"inputLimit": 800,
		"outputLimit": 600,
		"socSet": 90,
		"minSoc": 10,
		"inverseMaxPower": 1200,
		"gridReverse": 1,
		"gridStandard": 0,
		"gridOffMode": 2
	}`

	c := New(testConfig("http://unused", false), testLogger())
	data, err := c.parsePayload(testDevice("http://unused"), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertMetric(t, data.Metrics, "zendure_ac_mode", 2)
	assertMetric(t, data.Metrics, "zendure_input_limit_watts", 800)
	assertMetric(t, data.Metrics, "zendure_output_limit_watts", 600)
	assertMetric(t, data.Metrics, "zendure_soc_set_percent", 90)
	assertMetric(t, data.Metrics, "zendure_min_soc_percent", 10)
	assertMetric(t, data.Metrics, "zendure_inverse_max_power_watts", 1200)
	assertMetric(t, data.Metrics, "zendure_grid_reverse", 1)
	assertMetric(t, data.Metrics, "zendure_grid_standard", 0)
	assertMetric(t, data.Metrics, "zendure_grid_off_mode_setting", 2)
}

func TestParsePayload_RWConfigFields_TenthsPercentEncodings(t *testing.T) {
	payload := `{
		"socSet": 1000,
		"minSoc": 80
	}`

	c := New(testConfig("http://unused", false), testLogger())
	data, err := c.parsePayload(testDevice("http://unused"), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertMetric(t, data.Metrics, "zendure_soc_set_percent", 100)
	assertMetric(t, data.Metrics, "zendure_min_soc_percent", 8)
}

func TestParsePayload_FullRealisticPayload(t *testing.T) {
	payload := `{
		"solarInputPower": 2100,
		"solarPower1": 1050,
		"solarPower2": 1050,
		"packInputPower": 0,
		"outputPackPower": 500,
		"outputHomePower": 1600,
		"gridInputPower": 0,
		"electricLevel": 85,
		"BatVolt": 5180,
		"packNum": 2,
		"packState": 1,
		"pass": 1,
		"heatState": 0,
		"hyperTmp": 2981,
		"rssi": -52,
		"acMode": 1,
		"outputLimit": 800,
		"socSet": 100,
		"minSoc": 10,
		"timestamp": 1709910000,
		"ts": 1709910000,
		"packData": [
			{
				"sn": "PACK001",
				"socLevel": 90,
				"state": 1,
				"power": 250,
				"maxTemp": 2971,
				"totalVol": 5200,
				"batcur": 50,
				"maxVol": 335,
				"minVol": 320,
				"heatState": 0
			},
			{
				"sn": "PACK002",
				"socLevel": 80,
				"state": 1,
				"power": 250,
				"maxTemp": 2961,
				"totalVol": 5100,
				"batcur": 48,
				"maxVol": 333,
				"minVol": 318,
				"heatState": 0
			}
		]
	}`

	c := New(testConfig("http://unused", false), testLogger())
	data, err := c.parsePayload(testDevice("http://unused"), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Device-level metrics
	assertMetric(t, data.Metrics, "zendure_solar_input_power_watts", 2100)
	assertMetric(t, data.Metrics, "zendure_output_home_power_watts", 1600)
	assertMetricApprox(t, data.Metrics, "zendure_battery_voltage_volts", 51.80, 0.01)
	assertMetricApprox(t, data.Metrics, "zendure_enclosure_temperature_celsius", 25.0, 0.1)

	// Channel metrics
	channels := data.ChannelMetrics["zendure_solar_power_channel_watts"]
	assertMapValue(t, channels, "1", 1050)
	assertMapValue(t, channels, "2", 1050)

	// Battery packs
	if len(data.BatteryPacks) != 2 {
		t.Fatalf("expected 2 battery packs, got %d", len(data.BatteryPacks))
	}

	pack1 := data.BatteryPacks[0]
	assertMetric(t, pack1.Metrics, "zendure_pack_soc_level_percent", 90)
	assertMetricApprox(t, pack1.Metrics, "zendure_pack_temperature_celsius", 24.0, 0.1)
	assertMetricApprox(t, pack1.Metrics, "zendure_pack_total_voltage_volts", 52.00, 0.01)
	assertMetricApprox(t, pack1.Metrics, "zendure_pack_max_cell_voltage_volts", 3.35, 0.01)

	// Ignored fields should not appear
	if _, ok := data.Metrics["timestamp"]; ok {
		t.Error("timestamp should be ignored")
	}
}

func TestParsePayload_WrappedPayload_RealDeviceStyle(t *testing.T) {
	payload := `{
		"timestamp": 1773342560,
		"messageId": 64,
		"sn": "EEA1NEN9N381331",
		"version": 2,
		"product": "solarFlow800Pro",
		"properties": {
			"packInputPower": 189,
			"outputHomePower": 189,
			"electricLevel": 64,
			"BatVolt": 4875,
			"socSet": 1000,
			"minSoc": 80,
			"fanSwitch": 1,
			"Fanspeed": 0,
			"oldMode": 0,
			"solarPower1": 0,
			"solarPower2": 0,
			"rssi": -76
		},
		"packData": [{
			"sn": "CO4EENJJN380725",
			"socLevel": 64,
			"state": 2,
			"power": 194,
			"maxTemp": 2911,
			"totalVol": 4860,
			"batcur": 65496,
			"maxVol": 324,
			"minVol": 323
		}]
	}`

	c := New(testConfig("http://unused", true), testLogger())
	data, err := c.parsePayload(testDevice("http://unused"), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertMetric(t, data.Metrics, "zendure_pack_input_power_watts", 189)
	assertMetric(t, data.Metrics, "zendure_output_home_power_watts", 189)
	assertMetricApprox(t, data.Metrics, "zendure_battery_voltage_volts", 48.75, 0.01)
	assertMetric(t, data.Metrics, "zendure_soc_set_percent", 100)
	assertMetric(t, data.Metrics, "zendure_min_soc_percent", 8)
	assertMetric(t, data.Metrics, "zendure_fan_switch", 1)
	assertMetric(t, data.Metrics, "zendure_fan_speed", 0)

	channels := data.ChannelMetrics["zendure_solar_power_channel_watts"]
	assertMapValue(t, channels, "1", 0)
	assertMapValue(t, channels, "2", 0)

	if len(data.BatteryPacks) != 1 {
		t.Fatalf("expected 1 battery pack, got %d", len(data.BatteryPacks))
	}
	pack := data.BatteryPacks[0]
	assertMetricApprox(t, pack.Metrics, "zendure_pack_total_voltage_volts", 48.60, 0.01)
	assertMetricApprox(t, pack.Metrics, "zendure_pack_current_amperes", -4.0, 0.01)
	assertMetricApprox(t, pack.Metrics, "zendure_pack_max_cell_voltage_volts", 3.24, 0.01)
	assertMetricApprox(t, pack.Metrics, "zendure_pack_min_cell_voltage_volts", 3.23, 0.01)

	if len(data.UnknownFields) != 0 {
		t.Fatalf("expected no unknown fields for wrapped payload, got %d", len(data.UnknownFields))
	}
}

// --- HTTP integration tests ---

func TestFetchDevice_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/properties/report" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"solarInputPower": 1000, "electricLevel": 50}`)
	}))
	defer server.Close()

	c := New(testConfig(server.URL, false), testLogger())
	data, err := c.FetchDevice(testDevice(server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertMetric(t, data.Metrics, "zendure_solar_input_power_watts", 1000)
	assertMetric(t, data.Metrics, "zendure_electric_level_percent", 50)
}

func TestFetchDevice_HTTP500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	}))
	defer server.Close()

	c := New(testConfig(server.URL, false), testLogger())
	_, err := c.FetchDevice(testDevice(server.URL))
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
}

func TestFetchDevice_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not json at all")
	}))
	defer server.Close()

	c := New(testConfig(server.URL, false), testLogger())
	_, err := c.FetchDevice(testDevice(server.URL))
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestFetchDevice_ConnectionRefused(t *testing.T) {
	c := New(testConfig("http://127.0.0.1:1", false), testLogger())
	_, err := c.FetchDevice(testDevice("http://127.0.0.1:1"))
	if err == nil {
		t.Error("expected error for connection refused")
	}
}

func TestFetchDevice_TrailingSlashInBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/properties/report" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"solarInputPower": 42}`)
	}))
	defer server.Close()

	c := New(testConfig(server.URL+"/", false), testLogger())
	data, err := c.FetchDevice(testDevice(server.URL + "/"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertMetric(t, data.Metrics, "zendure_solar_input_power_watts", 42)
}

// --- Field mapping completeness ---

func TestDeviceFieldMapCoversAllSpecMetrics(t *testing.T) {
	// Verify no duplicate metric names in device field map
	seen := make(map[string]string)
	for field, mapping := range deviceFieldMap {
		if prev, ok := seen[mapping.metric]; ok {
			t.Errorf("duplicate metric %q: fields %q and %q", mapping.metric, prev, field)
		}
		seen[mapping.metric] = field
	}
}

func TestBatteryPackFieldMapCoversAllSpecMetrics(t *testing.T) {
	seen := make(map[string]string)
	for field, mapping := range batteryPackFieldMap {
		if prev, ok := seen[mapping.metric]; ok {
			t.Errorf("duplicate metric %q: fields %q and %q", mapping.metric, prev, field)
		}
		seen[mapping.metric] = field
	}
}

func TestParsePayload_FloatValues(t *testing.T) {
	// JSON numbers are parsed as float64 by encoding/json
	payload := `{"solarInputPower": 1500.5, "electricLevel": 75.3}`

	c := New(testConfig("http://unused", false), testLogger())
	data, err := c.parsePayload(testDevice("http://unused"), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertMetricApprox(t, data.Metrics, "zendure_solar_input_power_watts", 1500.5, 0.01)
	assertMetricApprox(t, data.Metrics, "zendure_electric_level_percent", 75.3, 0.01)
}

func TestParsePayload_NullFieldsIgnored(t *testing.T) {
	payload := `{"solarInputPower": null, "electricLevel": 50}`

	c := New(testConfig("http://unused", false), testLogger())
	data, err := c.parsePayload(testDevice("http://unused"), []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// null should not produce a metric (toFloat64 returns error for nil)
	if _, ok := data.Metrics["zendure_solar_input_power_watts"]; ok {
		t.Error("null field should not produce a metric")
	}
	assertMetric(t, data.Metrics, "zendure_electric_level_percent", 50)
}

func TestParsePayload_IgnoredFieldsNotInMetrics(t *testing.T) {
	fields := make(map[string]any)
	for _, f := range ignoredFields {
		fields[f] = 42
	}
	body, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("failed to marshal fields: %v", err)
	}

	c := New(testConfig("http://unused", true), testLogger())
	data, err := c.parsePayload(testDevice("http://unused"), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data.Metrics) != 0 {
		t.Errorf("ignored fields should not produce metrics, got %d", len(data.Metrics))
	}
	if len(data.UnknownFields) != 0 {
		t.Errorf("ignored fields should not appear in discovery, got %d", len(data.UnknownFields))
	}
}

// --- Helpers ---

func assertMetric(t *testing.T, metrics map[string]float64, name string, expected float64) {
	t.Helper()
	val, ok := metrics[name]
	if !ok {
		t.Errorf("metric %q not found", name)
		return
	}
	if val != expected {
		t.Errorf("metric %q = %v, want %v", name, val, expected)
	}
}

func assertMetricApprox(t *testing.T, metrics map[string]float64, name string, expected, tolerance float64) {
	t.Helper()
	val, ok := metrics[name]
	if !ok {
		t.Errorf("metric %q not found", name)
		return
	}
	if math.Abs(val-expected) > tolerance {
		t.Errorf("metric %q = %v, want ~%v (±%v)", name, val, expected, tolerance)
	}
}

func assertMapValue(t *testing.T, m map[string]float64, key string, expected float64) {
	t.Helper()
	val, ok := m[key]
	if !ok {
		t.Errorf("key %q not found in map", key)
		return
	}
	if val != expected {
		t.Errorf("map[%q] = %v, want %v", key, val, expected)
	}
}
