package collector

import (
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"zendure-exporter/internal/client"
	"zendure-exporter/internal/config"
)

var deviceLabels = []string{"device_id", "device_model"}
var packLabels = []string{"device_id", "device_model", "pack_sn"}
var channelLabels = []string{"device_id", "device_model", "channel"}
var discoveryLabels = []string{"device_id", "device_model", "field"}

// Collector implements prometheus.Collector. It fetches metrics from Zendure
// devices on every Prometheus scrape and exposes them as Prometheus metrics.
type Collector struct {
	cfg    *config.Config
	client *client.Client
	logger *slog.Logger
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

	// Mutable state for counters (persisted across scrapes).
	mu                  sync.Mutex
	upstreamErrorCounts map[string]float64 // key: device_id
	unknownFieldCounts  map[string]float64 // key: device_id
	lastSuccessTimes    map[string]float64 // key: device_id
}

// New creates a new Collector instance.
func New(cfg *config.Config, logger *slog.Logger, version string) *Collector {
	c := &Collector{
		cfg:     cfg,
		client:  client.New(cfg, logger),
		logger:  logger,
		version: version,

		deviceMetrics:  make(map[string]*prometheus.Desc),
		channelMetrics: make(map[string]*prometheus.Desc),
		packMetrics:    make(map[string]*prometheus.Desc),

		upstreamErrorCounts: make(map[string]float64),
		unknownFieldCounts:  make(map[string]float64),
		lastSuccessTimes:    make(map[string]float64),
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

// NewWithClient creates a Collector with an injected client (for testing).
func NewWithClient(cfg *config.Config, logger *slog.Logger, cl *client.Client, version string) *Collector {
	c := New(cfg, logger, version)
	c.client = cl
	return c
}

func (c *Collector) registerDeviceMetrics() {
	defs := map[string]string{
		// Power metrics
		"zendure_solar_input_power_watts":       "Total PV input power in watts",
		"zendure_pack_input_power_watts":         "Battery discharge power in watts",
		"zendure_output_pack_power_watts":        "Battery charge power in watts",
		"zendure_output_home_power_watts":        "Output power to home in watts",
		"zendure_grid_input_power_watts":         "Grid input power in watts",
		"zendure_grid_off_power_watts":           "Off-grid power in watts",
		// Battery / SOC
		"zendure_electric_level_percent":         "Average state of charge in percent",
		"zendure_battery_voltage_volts":          "Battery voltage in volts",
		"zendure_pack_num":                       "Number of battery packs",
		"zendure_remain_out_time_minutes":        "Remaining discharge time in minutes",
		"zendure_charge_max_limit_watts":         "Max charge power limit in watts",
		// State / Status
		"zendure_pack_state":                     "Pack state: 0=Standby, 1=Charging, 2=Discharging",
		"zendure_pass":                           "Pass-through state (0/1)",
		"zendure_heat_state":                     "Heating state (0/1)",
		"zendure_reverse_state":                  "Reverse flow state (0/1)",
		"zendure_grid_state":                     "Grid connection state (0/1)",
		"zendure_dc_status":                      "DC state (0-2)",
		"zendure_pv_status":                      "PV state (0/1)",
		"zendure_ac_status":                      "AC state (0-2)",
		"zendure_data_ready":                     "Data ready flag (0/1)",
		"zendure_soc_status":                     "SOC calibration state (0/1)",
		"zendure_soc_limit":                      "SOC limit state (0-2)",
		"zendure_is_error":                       "Error flag (0/1)",
		"zendure_fault_level":                    "Fault severity level",
		"zendure_lamp_switch":                    "Lamp state (0/1)",
		"zendure_fan_switch":                     "Fan state (0/1)",
		"zendure_fan_speed":                      "Fan speed level",
		// Environment / Misc
		"zendure_enclosure_temperature_celsius":  "Enclosure temperature in Celsius",
		"zendure_rssi_dbm":                       "WiFi signal strength in dBm",
		"zendure_fm_voltage_volts":               "Voltage activation value in volts",
		"zendure_ac_coupling_state":              "AC coupling state bitfield",
		"zendure_dry_node_state":                 "Dry contact state (0/1)",
		// RW config
		"zendure_ac_mode":                        "AC mode: 1=Charge, 2=Discharge",
		"zendure_input_limit_watts":              "AC charge limit in watts",
		"zendure_output_limit_watts":             "Output limit in watts",
		"zendure_soc_set_percent":                "Target SOC in percent (70-100)",
		"zendure_min_soc_percent":                "Minimum SOC in percent (0-50)",
		"zendure_inverse_max_power_watts":        "Max inverter output in watts",
		"zendure_grid_reverse":                   "Reverse flow control (0-2)",
		"zendure_grid_standard":                  "Grid standard: 0=DE, 1=FR, 2=AT, 3=CH, 4=NL, 5=ES, 6=BE, 7=GR, 8=DK, 9=IT",
		"zendure_grid_off_mode_setting":          "Off-grid mode: 0=Standard, 1=Economic, 2=Closure",
	}

	for name, help := range defs {
		c.deviceMetrics[name] = prometheus.NewDesc(name, help, deviceLabels, nil)
	}
}

func (c *Collector) registerChannelMetrics() {
	c.channelMetrics["zendure_solar_power_channel_watts"] = prometheus.NewDesc(
		"zendure_solar_power_channel_watts",
		"Per-channel PV power in watts",
		channelLabels, nil,
	)
}

func (c *Collector) registerPackMetrics() {
	defs := map[string]string{
		"zendure_pack_soc_level_percent":        "Pack state of charge in percent",
		"zendure_pack_state_enum":               "Pack state: 0=Standby, 1=Charging, 2=Discharging",
		"zendure_pack_power_watts":              "Pack power in watts",
		"zendure_pack_temperature_celsius":      "Pack max temperature in Celsius",
		"zendure_pack_total_voltage_volts":      "Pack total voltage in volts",
		"zendure_pack_current_amperes":          "Pack current in amperes",
		"zendure_pack_max_cell_voltage_volts":   "Max cell voltage in volts",
		"zendure_pack_min_cell_voltage_volts":   "Min cell voltage in volts",
		"zendure_pack_heat_state":               "Pack heating state (0/1)",
	}

	for name, help := range defs {
		c.packMetrics[name] = prometheus.NewDesc(name, help, packLabels, nil)
	}
}

func (c *Collector) registerSelfMetrics() {
	c.buildInfo = prometheus.NewDesc(
		"zendure_exporter_build_info",
		"A metric with a constant '1' value labeled by version from which zendure-exporter was built",
		[]string{"version"}, nil,
	)
	c.scrapeDuration = prometheus.NewDesc(
		"zendure_exporter_scrape_duration_seconds",
		"Total duration of last scrape in seconds",
		nil, nil,
	)
	c.scrapeSuccess = prometheus.NewDesc(
		"zendure_exporter_scrape_success",
		"1 if last scrape of device succeeded, 0 otherwise",
		deviceLabels, nil,
	)
	c.upstreamErrors = prometheus.NewDesc(
		"zendure_exporter_upstream_request_errors_total",
		"Total upstream request errors per device",
		deviceLabels, nil,
	)
	c.lastSuccessTS = prometheus.NewDesc(
		"zendure_last_success_timestamp_seconds",
		"Unix timestamp of last successful scrape per device",
		deviceLabels, nil,
	)
	c.unknownFieldsTotal = prometheus.NewDesc(
		"zendure_exporter_unknown_fields_total",
		"Total count of unknown fields seen per device (discovery mode)",
		deviceLabels, nil,
	)
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.deviceMetrics {
		ch <- d
	}
	for _, d := range c.channelMetrics {
		ch <- d
	}
	for _, d := range c.packMetrics {
		ch <- d
	}
	if c.discoveryDesc != nil {
		ch <- c.discoveryDesc
	}

	ch <- c.buildInfo
	ch <- c.scrapeDuration
	ch <- c.scrapeSuccess
	ch <- c.upstreamErrors
	ch <- c.lastSuccessTS
	ch <- c.unknownFieldsTotal
}

// Collect implements prometheus.Collector. It fetches device data on every scrape.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()

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
			c.collectDevice(ch, d)
		}(dev)
	}
	wg.Wait()

	ch <- prometheus.MustNewConstMetric(
		c.scrapeDuration, prometheus.GaugeValue, time.Since(start).Seconds(),
	)
}

func (c *Collector) collectDevice(ch chan<- prometheus.Metric, dev config.DeviceConfig) {
	labels := []string{dev.ID, dev.Model}

	data, err := c.client.FetchDevice(dev)
	if err != nil {
		c.logger.Error("device fetch failed",
			"device_id", dev.ID, "base_url", dev.BaseURL, "err", err)

		c.mu.Lock()
		c.upstreamErrorCounts[dev.ID]++
		c.mu.Unlock()

		// Emit failure self-metrics; do NOT emit device metrics.
		ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 0, labels...)
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

	// Record success.
	c.mu.Lock()
	c.lastSuccessTimes[dev.ID] = float64(time.Now().Unix())
	c.mu.Unlock()

	ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 1, labels...)
	c.emitCounters(ch, dev)
}

func (c *Collector) emitCounters(ch chan<- prometheus.Metric, dev config.DeviceConfig) {
	labels := []string{dev.ID, dev.Model}

	c.mu.Lock()
	errCount := c.upstreamErrorCounts[dev.ID]
	unknownCount := c.unknownFieldCounts[dev.ID]
	lastSuccess := c.lastSuccessTimes[dev.ID]
	c.mu.Unlock()

	ch <- prometheus.MustNewConstMetric(c.upstreamErrors, prometheus.CounterValue, errCount, labels...)
	ch <- prometheus.MustNewConstMetric(c.unknownFieldsTotal, prometheus.CounterValue, unknownCount, labels...)

	if lastSuccess > 0 {
		ch <- prometheus.MustNewConstMetric(c.lastSuccessTS, prometheus.GaugeValue, lastSuccess, labels...)
	}
}
