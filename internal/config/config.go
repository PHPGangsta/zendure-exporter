package config

import (
	"fmt"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

type DeviceConfig struct {
	ID             string `yaml:"id"`
	Model          string `yaml:"model"`
	BaseURL        string `yaml:"base_url"`
	Enabled        bool   `yaml:"enabled"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type Config struct {
	ListenAddr                  string         `yaml:"listen_addr"`
	ListenPort                  int            `yaml:"listen_port"`
	DiscoveryMode               bool           `yaml:"discovery_mode"`
	Debug                       bool           `yaml:"debug"`
	DeviceRequestTimeoutSeconds int            `yaml:"device_request_timeout_seconds"`
	Devices                     []DeviceConfig `yaml:"devices"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	cfg := &Config{
		ListenAddr:                  "0.0.0.0",
		ListenPort:                  9854,
		DeviceRequestTimeoutSeconds: 5,
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// EffectiveTimeout returns the per-device timeout if set, otherwise the global timeout.
func (c *Config) EffectiveTimeout(dev DeviceConfig) int {
	if dev.TimeoutSeconds > 0 {
		return dev.TimeoutSeconds
	}
	return c.DeviceRequestTimeoutSeconds
}

func (c *Config) validate() error {
	if c.ListenPort < 1 || c.ListenPort > 65535 {
		return fmt.Errorf("listen_port must be between 1 and 65535, got %d", c.ListenPort)
	}

	if c.DeviceRequestTimeoutSeconds < 1 {
		return fmt.Errorf("device_request_timeout_seconds must be >= 1, got %d", c.DeviceRequestTimeoutSeconds)
	}

	enabledCount := 0
	for i, d := range c.Devices {
		if d.ID == "" {
			return fmt.Errorf("devices[%d]: id is required", i)
		}
		if d.BaseURL == "" {
			return fmt.Errorf("devices[%d] (%s): base_url is required", i, d.ID)
		}
		u, err := url.Parse(d.BaseURL)
		if err != nil {
			return fmt.Errorf("devices[%d] (%s): invalid base_url %q: %w", i, d.ID, d.BaseURL, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("devices[%d] (%s): base_url must use http or https scheme, got %q", i, d.ID, u.Scheme)
		}
		if u.Host == "" {
			return fmt.Errorf("devices[%d] (%s): base_url must include a host", i, d.ID)
		}
		if d.TimeoutSeconds < 0 {
			return fmt.Errorf("devices[%d] (%s): timeout_seconds must be >= 0, got %d", i, d.ID, d.TimeoutSeconds)
		}
		if d.Enabled {
			enabledCount++
		}
	}

	if enabledCount == 0 {
		return fmt.Errorf("at least one device must be enabled")
	}

	return nil
}
