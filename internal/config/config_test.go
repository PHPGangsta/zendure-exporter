package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	return path
}

func TestLoad_ValidMinimalConfig(t *testing.T) {
	path := writeTestConfig(t, `
devices:
  - id: dev1
    base_url: http://10.0.0.1
    enabled: true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check defaults
	if cfg.ListenAddr != "0.0.0.0" {
		t.Errorf("expected default listen_addr 0.0.0.0, got %s", cfg.ListenAddr)
	}
	if cfg.ListenPort != 9854 {
		t.Errorf("expected default listen_port 9854, got %d", cfg.ListenPort)
	}
	if cfg.DeviceRequestTimeoutSeconds != 5 {
		t.Errorf("expected default timeout 5, got %d", cfg.DeviceRequestTimeoutSeconds)
	}
	if cfg.DiscoveryMode {
		t.Error("expected discovery_mode default false")
	}
	if cfg.Debug {
		t.Error("expected debug default false")
	}
}

func TestLoad_ValidFullConfig(t *testing.T) {
	path := writeTestConfig(t, `
listen_addr: 127.0.0.1
listen_port: 8080
discovery_mode: true
debug: true
device_request_timeout_seconds: 10
devices:
  - id: dev1
    model: SolarFlow800 Pro
    base_url: http://10.0.0.1
    enabled: true
  - id: dev2
    model: SolarFlow2400 AC
    base_url: http://10.0.0.2
    enabled: false
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddr != "127.0.0.1" {
		t.Errorf("expected listen_addr 127.0.0.1, got %s", cfg.ListenAddr)
	}
	if cfg.ListenPort != 8080 {
		t.Errorf("expected listen_port 8080, got %d", cfg.ListenPort)
	}
	if !cfg.DiscoveryMode {
		t.Error("expected discovery_mode true")
	}
	if !cfg.Debug {
		t.Error("expected debug true")
	}
	if cfg.DeviceRequestTimeoutSeconds != 10 {
		t.Errorf("expected timeout 10, got %d", cfg.DeviceRequestTimeoutSeconds)
	}
	if len(cfg.Devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(cfg.Devices))
	}
	if cfg.Devices[0].ID != "dev1" {
		t.Errorf("expected device 0 id dev1, got %s", cfg.Devices[0].ID)
	}
	if cfg.Devices[0].Model != "SolarFlow800 Pro" {
		t.Errorf("expected device 0 model SolarFlow800 Pro, got %s", cfg.Devices[0].Model)
	}
	if cfg.Devices[1].ID != "dev2" {
		t.Errorf("expected device 1 id dev2, got %s", cfg.Devices[1].ID)
	}
	if cfg.Devices[1].Enabled {
		t.Error("expected device 1 to be disabled")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTestConfig(t, `
listen_port: [invalid yaml
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_PortTooLow(t *testing.T) {
	path := writeTestConfig(t, `
listen_port: 0
devices:
  - id: dev1
    base_url: http://10.0.0.1
    enabled: true
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for port 0")
	}
}

func TestLoad_PortTooHigh(t *testing.T) {
	path := writeTestConfig(t, `
listen_port: 70000
devices:
  - id: dev1
    base_url: http://10.0.0.1
    enabled: true
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for port 70000")
	}
}

func TestLoad_TimeoutTooLow(t *testing.T) {
	path := writeTestConfig(t, `
device_request_timeout_seconds: 0
devices:
  - id: dev1
    base_url: http://10.0.0.1
    enabled: true
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for timeout 0")
	}
}

func TestLoad_DeviceMissingID(t *testing.T) {
	path := writeTestConfig(t, `
devices:
  - base_url: http://10.0.0.1
    enabled: true
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing device id")
	}
}

func TestLoad_DeviceMissingBaseURL(t *testing.T) {
	path := writeTestConfig(t, `
devices:
  - id: dev1
    enabled: true
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing base_url")
	}
}

func TestLoad_NoEnabledDevices(t *testing.T) {
	path := writeTestConfig(t, `
devices:
  - id: dev1
    base_url: http://10.0.0.1
    enabled: false
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error when no devices are enabled")
	}
}

func TestLoad_NoDevices(t *testing.T) {
	path := writeTestConfig(t, `
listen_port: 9854
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error when no devices are configured")
	}
}

func TestLoad_EmptyDevicesList(t *testing.T) {
	path := writeTestConfig(t, `
devices: []
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty devices list")
	}
}

func TestLoad_MultipleDevicesOneEnabled(t *testing.T) {
	path := writeTestConfig(t, `
devices:
  - id: dev1
    base_url: http://10.0.0.1
    enabled: false
  - id: dev2
    base_url: http://10.0.0.2
    enabled: true
  - id: dev3
    base_url: http://10.0.0.3
    enabled: false
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Devices) != 3 {
		t.Errorf("expected 3 devices, got %d", len(cfg.Devices))
	}
}

func TestLoad_DeviceModelOptional(t *testing.T) {
	path := writeTestConfig(t, `
devices:
  - id: dev1
    base_url: http://10.0.0.1
    enabled: true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Devices[0].Model != "" {
		t.Errorf("expected empty model for device without model field, got %s", cfg.Devices[0].Model)
	}
}

func TestLoad_PortBoundaryValues(t *testing.T) {
	// Port 1 should be valid
	path := writeTestConfig(t, `
listen_port: 1
devices:
  - id: dev1
    base_url: http://10.0.0.1
    enabled: true
`)
	if _, err := Load(path); err != nil {
		t.Fatalf("port 1 should be valid: %v", err)
	}

	// Port 65535 should be valid
	path = writeTestConfig(t, `
listen_port: 65535
devices:
  - id: dev1
    base_url: http://10.0.0.1
    enabled: true
`)
	if _, err := Load(path); err != nil {
		t.Fatalf("port 65535 should be valid: %v", err)
	}
}

func TestLoad_InvalidBaseURL_NoScheme(t *testing.T) {
	path := writeTestConfig(t, `
devices:
  - id: dev1
    base_url: 192.168.1.1
    enabled: true
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for base_url without scheme")
	}
}

func TestLoad_InvalidBaseURL_FTPScheme(t *testing.T) {
	path := writeTestConfig(t, `
devices:
  - id: dev1
    base_url: ftp://192.168.1.1
    enabled: true
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for ftp scheme")
	}
}

func TestLoad_ValidBaseURL_HTTPS(t *testing.T) {
	path := writeTestConfig(t, `
devices:
  - id: dev1
    base_url: https://192.168.1.1
    enabled: true
`)
	if _, err := Load(path); err != nil {
		t.Fatalf("https base_url should be valid: %v", err)
	}
}

func TestLoad_PerDeviceTimeout(t *testing.T) {
	path := writeTestConfig(t, `
device_request_timeout_seconds: 5
devices:
  - id: dev1
    base_url: http://10.0.0.1
    enabled: true
    timeout_seconds: 10
  - id: dev2
    base_url: http://10.0.0.2
    enabled: true
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.EffectiveTimeout(cfg.Devices[0]) != 10 {
		t.Errorf("expected per-device timeout 10, got %d", cfg.EffectiveTimeout(cfg.Devices[0]))
	}
	if cfg.EffectiveTimeout(cfg.Devices[1]) != 5 {
		t.Errorf("expected global timeout 5, got %d", cfg.EffectiveTimeout(cfg.Devices[1]))
	}
}

func TestLoad_NegativeDeviceTimeout(t *testing.T) {
	path := writeTestConfig(t, `
devices:
  - id: dev1
    base_url: http://10.0.0.1
    enabled: true
    timeout_seconds: -1
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for negative device timeout")
	}
}
