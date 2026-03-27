package collector

import (
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/PHPGangsta/zendure-exporter/internal/client"
	"github.com/PHPGangsta/zendure-exporter/internal/config"
)

var deviceLabels = []string{"device_id", "device_model"}
var packLabels = []string{"device_id", "device_model", "pack_sn"}
var channelLabels = []string{"device_id", "device_model", "channel"}
var discoveryLabels = []string{"device_id", "device_model", "field"}

// Fetcher is the interface for fetching device data from a single Zendure device.
// *client.Client satisfies this interface; the indirection decouples the two
// packages and allows test doubles to be injected via WithFetcher.
type Fetcher interface {
	FetchDevice(dev config.DeviceConfig) (*client.DeviceData, error)
}

// Option configures a Collector. Options are applied after the defaults are
// set in New, so each option only overrides specific fields.
type Option func(*Collector)

// WithVersion sets the version string reported in the build_info metric.
// Defaults to "dev" when not provided.
func WithVersion(v string) Option {
	return func(c *Collector) { c.version = v }
}

// WithFetcher overrides the Fetcher used to retrieve device data on each scrape.
// Intended for testing — supply a mock to exercise the collector without real
// HTTP calls. Production code should omit this option and let New create a
// *client.Client automatically.
func WithFetcher(f Fetcher) Option {
	return func(c *Collector) { c.fetcher = f }
}

// Collector implements prometheus.Collector. It fetches metrics from Zendure
// devices on every Prometheus scrape and exposes them as Prometheus metrics.
type Collector struct {
	cfg     *config.Config
	fetcher Fetcher
	logger  *slog.Logger
	version string

	// Device-level metric descriptors (keyed by metric name from client).
	deviceMetrics map[string]*prometheus.Desc
	// Channel-level metric descriptors.
	channelMetrics map[string]*prometheus.Desc
	// Battery pack metric descriptors.
	packMetrics map[string]*prometheus.Desc

	// Discovery mode metric.
	discoveryDesc *prometheus.Desc

	// Self-metrics.
	buildInfo          *prometheus.Desc
	scrapeDuration     *prometheus.Desc
	scrapeSuccess      *prometheus.Desc
	upstreamErrors     *prometheus.Desc
	lastSuccessTS      *prometheus.Desc
	unknownFieldsTotal *prometheus.Desc
	fetchDuration      *prometheus.Desc
	circuitBreakerOpen *prometheus.Desc

	// Mutable state for counters (persisted across scrapes).
	mu                  sync.Mutex
	upstreamErrorCounts map[string]map[string]float64 // device_id -> error_type -> count
	unknownFieldCounts  map[string]float64             // key: device_id
	lastSuccessTimes    map[string]float64             // key: device_id
	hasSucceeded        bool                           // true after at least one successful scrape

	// Per-device circuit-breaker state.
	consecutiveFailures map[string]int       // key: device_id
	circuitOpenUntil    map[string]time.Time // key: device_id; zero value = closed

	// Monotonic counter incremented on every Collect call.
	// The value is attached to the scrape-scoped logger as "scrape_id" so that
	// all log lines from a single scrape cycle share the same identifier.
	scrapeCounter atomic.Uint64
}

// New creates a new Collector. The defaults are:
//   - fetcher: a *client.Client constructed from cfg and logger
//   - version: "dev"
//
// Pass functional options to override either default:
//
//	collector.New(cfg, logger, collector.WithVersion("v1.2.3"))
//	collector.New(cfg, logger, collector.WithFetcher(myMock))
func New(cfg *config.Config, logger *slog.Logger, opts ...Option) *Collector {
	c := &Collector{
		cfg:     cfg,
		fetcher: client.New(cfg, logger),
		logger:  logger,
		version: "dev",

		deviceMetrics:  make(map[string]*prometheus.Desc),
		channelMetrics: make(map[string]*prometheus.Desc),
		packMetrics:    make(map[string]*prometheus.Desc),

		upstreamErrorCounts: make(map[string]map[string]float64),
		unknownFieldCounts:  make(map[string]float64),
		lastSuccessTimes:    make(map[string]float64),
		consecutiveFailures: make(map[string]int),
		circuitOpenUntil:    make(map[string]time.Time),
	}

	for _, opt := range opts {
		opt(c)
	}

	c.registerDeviceMetrics()
	c.registerPackMetrics()
	c.registerChannelMetrics()
	c.registerSelfMetrics()

	if cfg.DiscoveryMode {
		c.discoveryDesc = prometheus.NewDesc(
			"zendure_unknown_property",
			"Unknown property from device API (discovery mode)",
			discoveryLabels, nil,
		)
	}

	return c
}

// Ready returns true after at least one successful device scrape.
func (c *Collector) Ready() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hasSucceeded
}

// Collect implements prometheus.Collector. It fetches device data on every scrape.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()

	// Attach a monotonically-increasing scrape_id to every log line produced
	// during this scrape cycle so concurrent device goroutines can be correlated.
	scrapeID := c.scrapeCounter.Add(1)
	logger := c.logger.With("scrape_id", scrapeID)

	ch <- prometheus.MustNewConstMetric(
		c.buildInfo, prometheus.GaugeValue, 1, c.version,
	)

	var wg sync.WaitGroup
	for _, dev := range c.cfg.Devices {
		if !dev.Enabled {
			continue
		}
		wg.Add(1)
		go func(d config.DeviceConfig) {
			defer wg.Done()
			c.collectDevice(ch, d, logger)
		}(dev)
	}
	wg.Wait()

	ch <- prometheus.MustNewConstMetric(
		c.scrapeDuration, prometheus.GaugeValue, time.Since(start).Seconds(),
	)
}

// errorTypeOf maps a FetchDevice error to a stable string label value.
// The three values correspond to the typed errors in the client package.
func errorTypeOf(err error) string {
	var u *client.ErrUnreachable
	var h *client.ErrHTTPError
	var p *client.ErrParse
	switch {
	case errors.As(err, &u):
		return "unreachable"
	case errors.As(err, &h):
		return "http_error"
	case errors.As(err, &p):
		return "parse_error"
	default:
		return "unknown"
	}
}

func (c *Collector) collectDevice(ch chan<- prometheus.Metric, dev config.DeviceConfig, logger *slog.Logger) {
	labels := []string{dev.ID, dev.Model}

	c.mu.Lock()
	circuitOpen := c.isCircuitOpen(dev.ID)
	c.mu.Unlock()

	if circuitOpen {
		logger.Warn("circuit open, skipping device fetch",
			"device_id", dev.ID, "base_url", dev.BaseURL)
		ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 0, labels...)
		ch <- prometheus.MustNewConstMetric(c.circuitBreakerOpen, prometheus.GaugeValue, 1, labels...)
		c.emitCounters(ch, dev)
		return
	}

	fetchStart := time.Now()
	data, err := c.fetcher.FetchDevice(dev)
	fetchDuration := time.Since(fetchStart).Seconds()

	ch <- prometheus.MustNewConstMetric(c.fetchDuration, prometheus.GaugeValue, fetchDuration, labels...)

	if err != nil {
		errType := errorTypeOf(err)
		logger.Error("device fetch failed",
			"device_id", dev.ID, "base_url", dev.BaseURL, "error_type", errType, "err", err)

		c.mu.Lock()
		if c.upstreamErrorCounts[dev.ID] == nil {
			c.upstreamErrorCounts[dev.ID] = make(map[string]float64)
		}
		c.upstreamErrorCounts[dev.ID][errType]++
		c.consecutiveFailures[dev.ID]++
		if c.consecutiveFailures[dev.ID] >= circuitBreakerThreshold {
			c.circuitOpenUntil[dev.ID] = time.Now().Add(circuitBreakerBackoff)
			logger.Warn("circuit breaker opened",
				"device_id", dev.ID,
				"consecutive_failures", c.consecutiveFailures[dev.ID],
				"backoff_seconds", int(circuitBreakerBackoff.Seconds()))
		}
		c.mu.Unlock()

		// Emit failure self-metrics; do NOT emit device metrics.
		ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 0, labels...)
		ch <- prometheus.MustNewConstMetric(c.circuitBreakerOpen, prometheus.GaugeValue, 0, labels...)
		c.emitCounters(ch, dev)
		return
	}

	// Emit device-level metrics.
	for name, val := range data.Metrics {
		if desc, ok := c.deviceMetrics[name]; ok {
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, val, labels...)
		}
	}

	// Emit channel metrics.
	for name, channels := range data.ChannelMetrics {
		if desc, ok := c.channelMetrics[name]; ok {
			for chLabel, val := range channels {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, val, dev.ID, dev.Model, chLabel)
			}
		}
	}

	// Emit battery pack metrics.
	for _, pack := range data.BatteryPacks {
		packLabels := []string{dev.ID, dev.Model, pack.SerialNumber}
		for name, val := range pack.Metrics {
			if desc, ok := c.packMetrics[name]; ok {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, val, packLabels...)
			}
		}
	}

	// Discovery mode: emit unknown fields.
	if c.discoveryDesc != nil && len(data.UnknownFields) > 0 {
		c.mu.Lock()
		c.unknownFieldCounts[dev.ID] += float64(len(data.UnknownFields))
		c.mu.Unlock()

		for field, val := range data.UnknownFields {
			ch <- prometheus.MustNewConstMetric(
				c.discoveryDesc, prometheus.GaugeValue, val, dev.ID, dev.Model, field,
			)
		}
	}

	// Record success: reset circuit-breaker state and update timestamps.
	c.mu.Lock()
	c.consecutiveFailures[dev.ID] = 0
	c.lastSuccessTimes[dev.ID] = float64(time.Now().Unix())
	c.hasSucceeded = true
	c.mu.Unlock()

	ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 1, labels...)
	ch <- prometheus.MustNewConstMetric(c.circuitBreakerOpen, prometheus.GaugeValue, 0, labels...)
	c.emitCounters(ch, dev)
}

func (c *Collector) emitCounters(ch chan<- prometheus.Metric, dev config.DeviceConfig) {
	labels := []string{dev.ID, dev.Model}

	c.mu.Lock()
	errorCounts := c.upstreamErrorCounts[dev.ID]
	unknownCount := c.unknownFieldCounts[dev.ID]
	lastSuccess := c.lastSuccessTimes[dev.ID]
	c.mu.Unlock()

	// Emit one error counter series per error type seen for this device.
	for errType, count := range errorCounts {
		ch <- prometheus.MustNewConstMetric(
			c.upstreamErrors, prometheus.CounterValue, count,
			dev.ID, dev.Model, errType,
		)
	}
	ch <- prometheus.MustNewConstMetric(c.unknownFieldsTotal, prometheus.CounterValue, unknownCount, labels...)

	if lastSuccess > 0 {
		ch <- prometheus.MustNewConstMetric(c.lastSuccessTS, prometheus.GaugeValue, lastSuccess, labels...)
	}
}
