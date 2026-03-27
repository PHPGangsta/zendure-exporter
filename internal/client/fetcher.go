package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PHPGangsta/zendure-exporter/internal/config"
)

// FetchDevice fetches and parses /properties/report from a single device.
// Returns parsed DeviceData or an error. Callers handle per-device isolation.
func (c *Client) FetchDevice(dev config.DeviceConfig) (*DeviceData, error) {
	url := strings.TrimRight(dev.BaseURL, "/") + "/properties/report"

	timeout := time.Duration(c.cfg.EffectiveTimeout(dev)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for %s: %w", url, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request to %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if err != nil {
			return nil, fmt.Errorf("HTTP %d from %s (failed to read body: %w)", resp.StatusCode, url, err)
		}
		return nil, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, url, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body from %s: %w", url, err)
	}

	c.logger.Debug("raw payload", "device_id", dev.ID, "body", string(body))

	return c.parsePayload(dev, body)
}
