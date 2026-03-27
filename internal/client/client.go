// Package client fetches and parses metrics from Zendure device HTTP APIs.
package client

import (
	"log/slog"

	"github.com/PHPGangsta/zendure-exporter/internal/config"
)

// DeviceData holds the parsed and converted metrics from a single device scrape.
type DeviceData struct {
	// DeviceID and DeviceModel from config (used as Prometheus labels).
	DeviceID    string
	DeviceModel string

	// Metrics maps Prometheus metric names to their converted float64 values.
	Metrics map[string]float64

	// ChannelMetrics holds per-channel metrics (e.g. solar power per PV channel).
	// Key: metric name, Value: map of channel label to value.
	ChannelMetrics map[string]map[string]float64

	// BatteryPacks holds per-pack metrics keyed by pack serial number.
	BatteryPacks []BatteryPackData

	// UnknownFields holds field names not in the curated mapping (discovery mode).
	UnknownFields map[string]float64
}

// BatteryPackData holds parsed metrics for a single battery pack.
type BatteryPackData struct {
	SerialNumber string
	Metrics      map[string]float64
}

// Client fetches and parses metrics from Zendure device HTTP APIs.
type Client struct {
	cfg    *config.Config
	logger *slog.Logger
}

// New creates a new Client with the given config and logger.
func New(cfg *config.Config, logger *slog.Logger) *Client {
	return &Client{
		cfg:    cfg,
		logger: logger,
	}
}
