// Package client fetches and parses device metrics from Zendure local HTTP APIs.
//
// The central type is [Client], which fetches /properties/report from a single
// device and returns [DeviceData] containing Prometheus-ready float64 values.
// All unit conversions are applied during parsing:
//
//   - Temperature: 0.1 K encoding (with 2731 offset) → °C
//   - Voltage:     centi-volt encoding (×0.01) → V
//   - Current:     16-bit two's-complement encoding → A
//   - SOC limits:  0.1% encoding when > threshold → %
//
// Field mapping from the device API JSON to Prometheus metric names is defined
// in deviceFieldMap (device-level) and batteryPackFieldMap (per-pack). Unknown
// fields are always logged at Warn level; when DiscoveryMode is enabled they
// are also returned in [DeviceData.UnknownFields] for exposure as metrics.
//
// The package is split across three source files by responsibility:
//
//   - client.go     — exported types and constructor
//   - fetcher.go    — HTTP transport ([Client.FetchDevice])
//   - mapper.go     — JSON parsing, field maps, and sanitization
//   - converters.go — unit conversion functions and toFloat64 helper
package client
