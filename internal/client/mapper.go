package client

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/PHPGangsta/zendure-exporter/internal/config"
)

// deviceFieldMap maps source JSON field names to Prometheus metric names and conversions.
var deviceFieldMap = map[string]fieldMapping{
	// Power metrics
	"solarInputPower": {"zendure_solar_input_power_watts", identity},
	"packInputPower":  {"zendure_pack_input_power_watts", identity},
	"outputPackPower": {"zendure_output_pack_power_watts", identity},
	"outputHomePower": {"zendure_output_home_power_watts", identity},
	"gridInputPower":  {"zendure_grid_input_power_watts", identity},
	"gridOffPower":    {"zendure_grid_off_power_watts", identity},

	// Battery / SOC
	"electricLevel":  {"zendure_electric_level_percent", identity},
	"BatVolt":        {"zendure_battery_voltage_volts", convertCentiVolts},
	"packNum":        {"zendure_pack_num", identity},
	"remainOutTime":  {"zendure_remain_out_time_minutes", identity},
	"chargeMaxLimit": {"zendure_charge_max_limit_watts", identity},

	// State / Status
	"packState":    {"zendure_pack_state", identity},
	"pass":         {"zendure_pass", identity},
	"heatState":    {"zendure_heat_state", identity},
	"reverseState": {"zendure_reverse_state", identity},
	"gridState":    {"zendure_grid_state", identity},
	"dcStatus":     {"zendure_dc_status", identity},
	"pvStatus":     {"zendure_pv_status", identity},
	"acStatus":     {"zendure_ac_status", identity},
	"dataReady":    {"zendure_data_ready", identity},
	"socStatus":    {"zendure_soc_status", identity},
	"socLimit":     {"zendure_soc_limit", identity},
	"is_error":     {"zendure_is_error", identity},
	"faultLevel":   {"zendure_fault_level", identity},
	"lampSwitch":   {"zendure_lamp_switch", identity},
	"fanSwitch":    {"zendure_fan_switch", identity},
	"fanSpeed":     {"zendure_fan_speed", identity},

	// Environment / Misc
	"hyperTmp":        {"zendure_enclosure_temperature_celsius", convertTemperature},
	"rssi":            {"zendure_rssi_dbm", identity},
	"FMVolt":          {"zendure_fm_voltage_volts", identity},
	"acCouplingState": {"zendure_ac_coupling_state", identity},
	"dryNodeState":    {"zendure_dry_node_state", identity},

	// Read/Write config (exposed as metrics for monitoring)
	"acMode":          {"zendure_ac_mode", identity},
	"inputLimit":      {"zendure_input_limit_watts", identity},
	"outputLimit":     {"zendure_output_limit_watts", identity},
	"socSet":          {"zendure_soc_set_percent", convertSocSetPercent},
	"minSoc":          {"zendure_min_soc_percent", convertMinSocPercent},
	"inverseMaxPower": {"zendure_inverse_max_power_watts", identity},
	"gridReverse":     {"zendure_grid_reverse", identity},
	"gridStandard":    {"zendure_grid_standard", identity},
	"gridOffMode":     {"zendure_grid_off_mode_setting", identity},
}

// batteryPackFieldMap maps battery pack JSON fields to Prometheus metric names.
var batteryPackFieldMap = map[string]fieldMapping{
	"socLevel":  {"zendure_pack_soc_level_percent", identity},
	"state":     {"zendure_pack_state_enum", identity},
	"power":     {"zendure_pack_power_watts", identity},
	"maxTemp":   {"zendure_pack_temperature_celsius", convertTemperature},
	"totalVol":  {"zendure_pack_total_voltage_volts", convertCentiVolts},
	"batcur":    {"zendure_pack_current_amperes", convertBatteryCurrent},
	"maxVol":    {"zendure_pack_max_cell_voltage_volts", convertCentiVolts},
	"minVol":    {"zendure_pack_min_cell_voltage_volts", convertCentiVolts},
	"heatState": {"zendure_pack_heat_state", identity},
}

// ignoredFields are known fields that we deliberately skip (not metrics, not discovery).
var ignoredFields = []string{
	"timestamp", "ts", "timeZone", "tsZone",
	"bindstate", "VoltWakeup", "OldMode", "oldMode", "OTAState",
	"LCNState", "factoryModeState", "IOTState",
	"phaseSwitch",
	// RW fields not exposed
	"writeRsp", "smartMode", "batCalTime",
}

// parsePayload parses the JSON response body into DeviceData.
func (c *Client) parsePayload(dev config.DeviceConfig, body []byte) (*DeviceData, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing JSON from device %s: %w", dev.ID, err)
	}

	// Some firmware versions wrap device properties in a top-level "properties" object.
	// Support both wrapped and flat payload shapes.
	deviceFields := raw
	wrappedPayload := false
	if propsRaw, ok := raw["properties"]; ok {
		if props, ok := propsRaw.(map[string]any); ok {
			deviceFields = props
			wrappedPayload = true
		} else {
			c.logger.Warn("top-level properties field is not an object",
				"device_id", dev.ID, "type", fmt.Sprintf("%T", propsRaw))
		}
	}

	data := &DeviceData{
		DeviceID:       dev.ID,
		DeviceModel:    dev.Model,
		Metrics:        make(map[string]float64),
		ChannelMetrics: make(map[string]map[string]float64),
		UnknownFields:  make(map[string]float64),
	}

	consumed := make(map[string]bool)

	// Parse known device-level fields.
	for field, mapping := range deviceFieldMap {
		if val, ok := deviceFields[field]; ok {
			consumed[field] = true
			if f, err := toFloat64(val); err == nil {
				data.Metrics[mapping.metric] = mapping.convert(f)
			} else {
				c.logger.Warn("field conversion failed",
					"device_id", dev.ID, "field", field, "value", val, "err", err)
			}
		}
	}

	// Parse per-channel solar power (solarPower1 through solarPower6).
	for ch := 1; ch <= 6; ch++ {
		field := fmt.Sprintf("solarPower%d", ch)
		if val, ok := deviceFields[field]; ok {
			consumed[field] = true
			if f, err := toFloat64(val); err == nil {
				if data.ChannelMetrics["zendure_solar_power_channel_watts"] == nil {
					data.ChannelMetrics["zendure_solar_power_channel_watts"] = make(map[string]float64)
				}
				data.ChannelMetrics["zendure_solar_power_channel_watts"][strconv.Itoa(ch)] = f
			}
		}
	}

	// Some payload variants expose fan settings as RW aliases only.
	if _, exists := data.Metrics["zendure_fan_switch"]; !exists {
		if val, ok := deviceFields["Fanmode"]; ok {
			consumed["Fanmode"] = true
			if f, err := toFloat64(val); err == nil {
				data.Metrics["zendure_fan_switch"] = f
			}
		}
	}
	if _, exists := data.Metrics["zendure_fan_speed"]; !exists {
		if val, ok := deviceFields["Fanspeed"]; ok {
			consumed["Fanspeed"] = true
			if f, err := toFloat64(val); err == nil {
				data.Metrics["zendure_fan_speed"] = f
			}
		}
	}

	// Parse battery packs (expected as a JSON array under "packData" or "batteries" key).
	parsedPacks := false
	for _, packKey := range []string{"packData", "batteries"} {
		if packRaw, ok := raw[packKey]; ok {
			parsedPacks = true
			if !wrappedPayload {
				consumed[packKey] = true
			}
			if packs, ok := packRaw.([]any); ok {
				data.BatteryPacks = c.parseBatteryPacks(dev.ID, packs)
			}
			break
		}
	}
	if !parsedPacks && wrappedPayload {
		for _, packKey := range []string{"packData", "batteries"} {
			if packRaw, ok := deviceFields[packKey]; ok {
				consumed[packKey] = true
				if packs, ok := packRaw.([]any); ok {
					data.BatteryPacks = c.parseBatteryPacks(dev.ID, packs)
				}
				break
			}
		}
	}

	// Mark ignored fields as consumed so they don't appear in discovery.
	for _, field := range ignoredFields {
		consumed[field] = true
	}

	// Handle unknown/unexpected fields from device API responses.
	// In discovery mode: expose as metrics. Always: log at warn level to help detect firmware changes.
	for field, val := range deviceFields {
		if consumed[field] {
			continue
		}
		if f, err := toFloat64(val); err == nil {
			if c.cfg.DiscoveryMode {
				sanitized := sanitizeFieldName(field)
				data.UnknownFields[sanitized] = f
				c.logger.Info("discovery: unknown numeric field",
					"device_id", dev.ID, "field", field, "sanitized", sanitized, "value", f)
			} else {
				c.logger.Warn("unknown field in device response (possible firmware change)",
					"device_id", dev.ID, "field", field, "value", f)
			}
		} else {
			c.logger.Warn("unknown non-numeric field in device response (ignored)",
				"device_id", dev.ID, "field", field, "type", fmt.Sprintf("%T", val))
		}
	}

	return data, nil
}

// parseBatteryPacks parses the battery pack array from the API response.
func (c *Client) parseBatteryPacks(deviceID string, packs []any) []BatteryPackData {
	var result []BatteryPackData
	for i, p := range packs {
		packMap, ok := p.(map[string]any)
		if !ok {
			c.logger.Warn("battery pack is not an object",
				"device_id", deviceID, "index", i)
			continue
		}

		sn := ""
		if snVal, ok := packMap["sn"]; ok {
			if s, ok := snVal.(string); ok {
				sn = s
			}
		}
		if sn == "" {
			c.logger.Warn("battery pack missing serial number",
				"device_id", deviceID, "index", i)
			continue
		}

		packData := BatteryPackData{
			SerialNumber: sn,
			Metrics:      make(map[string]float64),
		}

		for field, mapping := range batteryPackFieldMap {
			if val, ok := packMap[field]; ok {
				if f, err := toFloat64(val); err == nil {
					packData.Metrics[mapping.metric] = mapping.convert(f)
				} else {
					c.logger.Warn("pack field conversion failed",
						"device_id", deviceID, "pack_sn", sn,
						"field", field, "value", val, "err", err)
				}
			}
		}

		result = append(result, packData)
	}
	return result
}

// sanitizeFieldName converts a raw field name to a safe Prometheus-compatible label value.
// Lowercase, replace non-alphanumeric with _, collapse consecutive _.
func sanitizeFieldName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	prevUnderscore := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevUnderscore = false
		} else if !prevUnderscore {
			b.WriteRune('_')
			prevUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

// KnownDeviceFields returns the set of known device-level source field names.
// Useful for testing and documentation.
func KnownDeviceFields() []string {
	fields := make([]string, 0, len(deviceFieldMap))
	for f := range deviceFieldMap {
		fields = append(fields, f)
	}
	return fields
}

// KnownBatteryPackFields returns the set of known battery pack source field names.
func KnownBatteryPackFields() []string {
	fields := make([]string, 0, len(batteryPackFieldMap))
	for f := range batteryPackFieldMap {
		fields = append(fields, f)
	}
	return fields
}
