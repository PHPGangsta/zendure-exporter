package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// binaryPath returns the path to the built zendure-exporter binary.
// Tests in this file require the binary to be built beforehand:
//
//	go build -o zendure-exporter ./cmd/zendure-exporter
func binaryPath(t *testing.T) string {
	t.Helper()
	// Walk up from the test file to find the project root (where go.mod lives).
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))

	bin := filepath.Join(projectRoot, "zendure-exporter")
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("binary not found at %s — run 'go build -o zendure-exporter ./cmd/zendure-exporter' first", bin)
	}
	return bin
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestCheckConfig_ValidConfig(t *testing.T) {
	bin := binaryPath(t)
	cfgPath := writeConfigFile(t, `
devices:
  - id: dev1
    base_url: http://10.0.0.1
    enabled: true
`)
	cmd := exec.Command(bin, "--config", cfgPath, "--check-config") //nolint:gosec,noctx // test binary path is controlled
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0, got error: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "OK") {
		t.Errorf("expected 'OK' in output, got: %s", out)
	}
}

func TestCheckConfig_InvalidConfig_MissingDevices(t *testing.T) {
	bin := binaryPath(t)
	cfgPath := writeConfigFile(t, `
listen_port: 9854
`)
	cmd := exec.Command(bin, "--config", cfgPath, "--check-config") //nolint:gosec,noctx // test binary path is controlled
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for missing devices")
	}
	if !strings.Contains(string(out), "ERROR") {
		t.Errorf("expected 'ERROR' in output, got: %s", out)
	}
}

func TestCheckConfig_InvalidConfig_BadPort(t *testing.T) {
	bin := binaryPath(t)
	cfgPath := writeConfigFile(t, `
listen_port: 99999
devices:
  - id: dev1
    base_url: http://10.0.0.1
    enabled: true
`)
	cmd := exec.Command(bin, "--config", cfgPath, "--check-config") //nolint:gosec,noctx // test binary path is controlled
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for invalid port")
	}
	if !strings.Contains(string(out), "ERROR") {
		t.Errorf("expected 'ERROR' in output, got: %s", out)
	}
}

func TestCheckConfig_MissingFile(t *testing.T) {
	bin := binaryPath(t)
	cmd := exec.Command(bin, "--config", "/nonexistent/config.yml", "--check-config") //nolint:gosec,noctx // test binary path is controlled
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for missing config file")
	}
	if !strings.Contains(string(out), "ERROR") {
		t.Errorf("expected 'ERROR' in output, got: %s", out)
	}
}

func TestCheckConfig_MultiDeviceConfig(t *testing.T) {
	bin := binaryPath(t)
	cfgPath := writeConfigFile(t, `
listen_port: 9854
discovery_mode: true
device_request_timeout_seconds: 10
devices:
  - id: sf800-living
    model: SolarFlow800
    base_url: http://10.0.0.10
    enabled: true
  - id: sf800pro-garage
    model: SolarFlow800 Pro
    base_url: http://10.0.0.11
    enabled: true
  - id: sf2400-basement
    model: SolarFlow2400 AC
    base_url: http://10.0.0.12
    enabled: false
`)
	cmd := exec.Command(bin, "--config", cfgPath, "--check-config") //nolint:gosec,noctx // test binary path is controlled
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected exit 0 for valid multi-device config, got error: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "OK") {
		t.Errorf("expected 'OK' in output, got: %s", out)
	}
}
