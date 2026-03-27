package client

import (
	"encoding/json"
	"fmt"
	"math"
)

// fieldMapping defines how a source JSON field maps to a Prometheus metric.
type fieldMapping struct {
	metric  string
	convert func(float64) float64
}

// identity returns the value unchanged.
func identity(v float64) float64 { return v }

// convertTemperature converts from 0.1K (with 2731 offset) to Celsius.
func convertTemperature(raw float64) float64 {
	return (raw - 2731) / 10.0
}

// convertCentiVolts converts from 0.01V to Volts.
func convertCentiVolts(raw float64) float64 {
	return raw / 100.0
}

// convertBatteryCurrent converts 16-bit two's complement raw value to Amperes.
func convertBatteryCurrent(raw float64) float64 {
	intVal := int64(raw) & 0xFFFF
	if intVal >= 0x8000 {
		intVal -= 0x10000
	}
	return float64(intVal) / 10.0
}

// convertSocSetPercent normalizes SOC target values to percent.
// Some payloads use direct percent (70-100), others use 0.1% encoding (700-1000).
func convertSocSetPercent(raw float64) float64 {
	if raw > 100 {
		return raw / 10.0
	}
	return raw
}

// convertMinSocPercent normalizes minimum SOC values to percent.
// Some payloads use direct percent (0-50), others use 0.1% encoding (0-500).
func convertMinSocPercent(raw float64) float64 {
	if raw > 50 {
		return raw / 10.0
	}
	return raw
}

// toFloat64 converts a JSON value (float64, json.Number, bool) to float64.
func toFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		if math.IsNaN(val) || math.IsInf(val, 0) {
			return 0, fmt.Errorf("non-finite float value: %v", val)
		}
		return val, nil
	case json.Number:
		return val.Float64()
	case bool:
		if val {
			return 1, nil
		}
		return 0, nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}
