// Package config handles loading and validation of the exporter configuration.
//
// [Load] reads a YAML file, applies defaults, unmarshals the content into
// [Config], and validates all fields before returning. Validation errors
// include the file path or device index so they are immediately actionable.
//
// Defaults applied when a field is omitted from the YAML file:
//
//   - ListenAddr:                  "0.0.0.0"
//   - ListenPort:                  9854
//   - DeviceRequestTimeoutSeconds: 5
//
// [Config.EffectiveTimeout] resolves the per-device timeout, falling back to
// the global DeviceRequestTimeoutSeconds when no per-device override is set.
//
// Use --check-config in the CLI to validate a configuration file without
// starting the exporter.
package config
