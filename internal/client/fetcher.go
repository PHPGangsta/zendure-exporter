package client

import (
	"context"
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
		// NewRequestWithContext only fails for malformed URLs, which config validation prevents.
		return nil, &ErrUnreachable{URL: url, Err: err}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ErrUnreachable{URL: url, Err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		rawBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if readErr != nil {
			return nil, &ErrHTTPError{URL: url, Status: resp.StatusCode}
		}
		return nil, &ErrHTTPError{URL: url, Status: resp.StatusCode, Body: strings.TrimSpace(string(rawBody))}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ErrUnreachable{URL: url, Err: err}
	}

	c.logger.Debug("raw payload", "device_id", dev.ID, "body", string(body))

	data, err := c.parsePayload(dev, body)
	if err != nil {
		return nil, &ErrParse{DeviceID: dev.ID, Err: err}
	}
	return data, nil
}
