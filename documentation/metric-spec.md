# Zendure Exporter — Metric Specification

## Source

All properties are based on the [Zendure zenSDK](https://github.com/Zendure/zenSDK) documentation:
- [README.md](https://github.com/Zendure/zenSDK/blob/main/README.md) — supported products, API overview
- [docs/en_properties.md](https://github.com/Zendure/zenSDK/blob/main/docs/en_properties.md) — property reference

Endpoint: `GET /properties/report` on each device's local HTTP server.

Live payloads are now available and reflected here. The exporter supports both observed shapes:
- Flat payloads (fields at top level)
- Wrapped payloads with device fields under top-level `properties`

---

## Supported Products

| Model             |
|-------------------|
| SolarFlow800      |
| SolarFlow800 Plus |
| SolarFlow800 Pro  |
| SolarFlow1600 AC+ |
| SolarFlow2400 AC  |
| SolarFlow2400 AC+ |
| SolarFlow2400 Pro |

---

## Label Strategy

Every Zendure metric carries these labels:

| Label          | Source                                  | Example                  |
|----------------|-----------------------------------------|--------------------------|
| `device_id`    | Config file `devices[].id`              | `sf800pro_basement`      |
| `device_model` | Config file `devices[].model`           | `SolarFlow800 Pro`       |

Battery-pack metrics additionally carry:

| Label      | Source                 | Example              |
|------------|------------------------|----------------------|
| `pack_sn`  | Battery pack `sn` field | `EB04A...`          |

This keeps cardinality bounded (one label set per configured device, plus one per battery pack).

---

## Conventions

- Metric prefix: `zendure_`
- Prometheus naming: `snake_case`, SI base units where applicable
- Gauges for instantaneous values; counters only where a monotonically increasing semantic is clear (none identified yet — all power/energy values appear instantaneous)
- Boolean/flag fields → gauge with value 0 or 1
- Enum fields → gauge with integer value; meaning documented in HELP string
- Missing/null fields → metric not emitted for that scrape (no stale values)
- Temperature conversion: `(raw - 2731) / 10.0` → Celsius (Prometheus convention: use base units, but Celsius is standard for temperature metrics per [Prometheus best practices](https://prometheus.io/docs/practices/naming/))
- Voltage conversion (`raw / 100`) applies to `BatVolt`, `totalVol`, `maxVol`, and `minVol`
- Battery current: 16-bit two's complement, then `/10.0` → Amperes

---

## Device Data Properties — Read Only

### Power Metrics (stable)

| Prometheus Metric                        | Type  | Unit | Source Field       | Conversion       | Description                    |
|------------------------------------------|-------|------|--------------------|------------------|--------------------------------|
| `zendure_solar_input_power_watts`        | gauge | W    | `solarInputPower`  | none             | Total PV input power           |
| `zendure_solar_power_channel_watts`      | gauge | W    | `solarPower1`–`6`  | none; label `channel="1"`–`"6"` | Per-channel PV power |
| `zendure_pack_input_power_watts`         | gauge | W    | `packInputPower`   | none             | Battery discharge power        |
| `zendure_output_pack_power_watts`        | gauge | W    | `outputPackPower`  | none             | Battery charge power           |
| `zendure_output_home_power_watts`        | gauge | W    | `outputHomePower`  | none             | Output power to home           |
| `zendure_grid_input_power_watts`         | gauge | W    | `gridInputPower`   | none             | Grid input power               |
| `zendure_grid_off_power_watts`           | gauge | W    | `gridOffPower`     | none             | Off-grid power                 |

### Battery / SOC Metrics (stable)

| Prometheus Metric                        | Type  | Unit | Source Field       | Conversion       | Description                    |
|------------------------------------------|-------|------|--------------------|------------------|--------------------------------|
| `zendure_electric_level_percent`         | gauge | %    | `electricLevel`    | none             | Average state of charge        |
| `zendure_battery_voltage_volts`          | gauge | V    | `BatVolt`          | raw / 100        | Battery voltage (0.01V units)  |
| `zendure_pack_num`                       | gauge | —    | `packNum`          | none             | Number of battery packs        |
| `zendure_remain_out_time_minutes`        | gauge | min  | `remainOutTime`    | none             | Remaining discharge time       |
| `zendure_charge_max_limit_watts`         | gauge | W    | `chargeMaxLimit`   | none             | Max charge power limit         |

### State / Status Metrics (stable)

| Prometheus Metric                        | Type  | Unit | Source Field       | Conversion       | Description                    |
|------------------------------------------|-------|------|--------------------|------------------|--------------------------------|
| `zendure_pack_state`                     | gauge | —    | `packState`        | none             | 0=Standby, 1=Charging, 2=Discharging |
| `zendure_pass`                           | gauge | —    | `pass`             | none             | Pass-through state (0/1)       |
| `zendure_heat_state`                     | gauge | —    | `heatState`        | none             | Heating state (0/1)            |
| `zendure_reverse_state`                  | gauge | —    | `reverseState`     | none             | Reverse flow state (0/1)       |
| `zendure_grid_state`                     | gauge | —    | `gridState`        | none             | Grid connection state (0/1)    |
| `zendure_dc_status`                      | gauge | —    | `dcStatus`         | none             | DC state (0–2)                 |
| `zendure_pv_status`                      | gauge | —    | `pvStatus`         | none             | PV state (0/1)                 |
| `zendure_ac_status`                      | gauge | —    | `acStatus`         | none             | AC state (0–2)                 |
| `zendure_data_ready`                     | gauge | —    | `dataReady`        | none             | Data ready flag (0/1)          |
| `zendure_soc_status`                     | gauge | —    | `socStatus`        | none             | SOC calibration state (0/1)    |
| `zendure_soc_limit`                      | gauge | —    | `socLimit`         | none             | SOC limit state (0–2)          |
| `zendure_is_error`                       | gauge | —    | `is_error`         | none             | Error flag (0/1)               |
| `zendure_fault_level`                    | gauge | —    | `faultLevel`       | none             | Fault severity level           |
| `zendure_lamp_switch`                    | gauge | —    | `lampSwitch`       | none             | Lamp state (0/1)               |
| `zendure_fan_switch`                     | gauge | —    | `fanSwitch` / `Fanmode` | none         | Fan state (0/1)                |
| `zendure_fan_speed`                      | gauge | —    | `fanSpeed` / `Fanspeed` | none         | Fan speed level                |

### Environment / Misc Metrics (stable)

| Prometheus Metric                        | Type  | Unit | Source Field       | Conversion       | Description                    |
|------------------------------------------|-------|------|--------------------|------------------|--------------------------------|
| `zendure_enclosure_temperature_celsius`  | gauge | °C   | `hyperTmp`         | (raw - 2731) / 10.0 | Enclosure temperature      |
| `zendure_rssi_dbm`                       | gauge | dBm  | `rssi`             | none             | WiFi signal strength           |
| `zendure_fm_voltage_volts`               | gauge | V    | `FMVolt`           | none             | Voltage activation value       |
| `zendure_ac_coupling_state`              | gauge | —    | `acCouplingState`  | none             | AC coupling state (bitfield)   |
| `zendure_dry_node_state`                 | gauge | —    | `dryNodeState`     | none             | Dry contact state (0/1)        |

### Informational / Low-value (ignored)

These fields are available in the API but deliberately not exposed as metrics.
They provide no monitoring value or are internal/operational.

| Source Field       | Reason for ignoring                                    |
|--------------------|--------------------------------------------------------|
| `timestamp`        | Device internal timestamp, not useful for monitoring   |
| `ts`               | Unix timestamp, redundant with Prometheus scrape time  |
| `timeZone`         | Static config, not a metric                            |
| `tsZone`           | Static config, not a metric                            |
| `bindstate`        | Internal binding state                                 |
| `VoltWakeup`       | Internal parameter                                     |
| `OldMode`          | Legacy internal mode                                   |
| `OTAState`         | OTA update state — transient, not useful for dashboards|
| `LCNState`         | Internal LCN state                                     |
| `factoryModeState` | Factory mode — should never be active in production    |
| `IOTState`         | IoT connection state — covered by exporter reachability|
| `gridOffMode` (RO) | Duplicate of RW field, use RW version                  |
| `phaseSwitch`      | Internal phase switch parameter                        |

---

## Device Data Properties — Read / Write (Configuration)

These are writable settings. Exposed as metrics for monitoring current config state.

| Prometheus Metric                          | Type  | Unit | Source Field       | Conversion | Description                           |
|--------------------------------------------|-------|------|--------------------|------------|---------------------------------------|
| `zendure_ac_mode`                          | gauge | —    | `acMode`           | none       | 1=Charge, 2=Discharge                 |
| `zendure_input_limit_watts`                | gauge | W    | `inputLimit`       | none       | AC charge limit                       |
| `zendure_output_limit_watts`               | gauge | W    | `outputLimit`      | none       | Output limit                          |
| `zendure_soc_set_percent`                  | gauge | %    | `socSet`           | if raw > 100, raw / 10; else raw | Target SOC (70–100) |
| `zendure_min_soc_percent`                  | gauge | %    | `minSoc`           | if raw > 50, raw / 10; else raw | Minimum SOC (0–50)        |
| `zendure_inverse_max_power_watts`          | gauge | W    | `inverseMaxPower`  | none       | Max inverter output                   |
| `zendure_grid_reverse`                     | gauge | —    | `gridReverse`      | none       | Reverse flow control (0–2)            |
| `zendure_grid_standard`                    | gauge | —    | `gridStandard`     | none       | Grid standard (0–9, see enum)         |
| `zendure_grid_off_mode_setting`            | gauge | —    | `gridOffMode`      | none       | Off-grid mode setting (0–2)           |

### RW fields not exposed

| Source Field   | Reason                                                    |
|----------------|-----------------------------------------------------------|
| `writeRsp`     | Write response artifact, not a property                   |
| `smartMode`    | Flash-write behavior toggle, internal                     |
| `batCalTime`   | Battery calibration timer, vendor warns against changes   |

Note: `Fanmode` and `Fanspeed` are accepted as source aliases for the exported RO-style metrics
`zendure_fan_switch` and `zendure_fan_speed` when `fanSwitch`/`fanSpeed` are absent.

---

## Battery Pack Properties

All battery pack metrics carry the additional `pack_sn` label.

| Prometheus Metric                              | Type  | Unit | Source Field   | Conversion             | Description                  |
|------------------------------------------------|-------|------|----------------|------------------------|------------------------------|
| `zendure_pack_soc_level_percent`               | gauge | %    | `socLevel`     | none                   | Pack state of charge         |
| `zendure_pack_state_enum`                      | gauge | —    | `state`        | none                   | 0=Standby, 1=Charging, 2=Discharging |
| `zendure_pack_power_watts`                     | gauge | W    | `power`        | none                   | Pack power                   |
| `zendure_pack_temperature_celsius`             | gauge | °C   | `maxTemp`      | (raw - 2731) / 10.0    | Pack max temperature         |
| `zendure_pack_total_voltage_volts`             | gauge | V    | `totalVol`     | raw / 100              | Pack total voltage           |
| `zendure_pack_current_amperes`                 | gauge | A    | `batcur`       | int16 two's complement / 10.0 | Pack current          |
| `zendure_pack_max_cell_voltage_volts`          | gauge | V    | `maxVol`       | raw / 100              | Max cell voltage             |
| `zendure_pack_min_cell_voltage_volts`          | gauge | V    | `minVol`       | raw / 100              | Min cell voltage             |
| `zendure_pack_heat_state`                      | gauge | —    | `heatState`    | none                   | Pack heating state (0/1)     |

### Battery pack fields not exposed

| Source Field    | Reason                                    |
|-----------------|-------------------------------------------|
| `sn`            | Used as `pack_sn` label, not a metric     |
| `packType`      | Reserved / undocumented                   |
| `softVersion`   | Firmware version — static, not a metric   |

---

## Exporter Self-Metrics

| Prometheus Metric                              | Type    | Labels                        | Description                              |
|------------------------------------------------|---------|-------------------------------|------------------------------------------|
| `zendure_exporter_scrape_duration_seconds`     | gauge   | —                             | Total duration of last scrape            |
| `zendure_exporter_scrape_success`              | gauge   | `device_id`, `device_model`   | 1 if last scrape succeeded, 0 otherwise  |
| `zendure_exporter_upstream_request_errors_total`| counter | `device_id`, `device_model`  | Total upstream request errors            |
| `zendure_last_success_timestamp_seconds`       | gauge   | `device_id`, `device_model`   | Unix timestamp of last successful scrape |
| `zendure_exporter_unknown_fields_total`        | counter | `device_id`, `device_model`   | Count of unknown fields seen (discovery) |

---

## Discovery Mode

When `discovery_mode: true` in config:
- Any field in the API response that is **not** in the curated mapping above is exposed as:
  `zendure_unknown_property{device_id="...", device_model="...", field="<sanitized_field_name>"}` (gauge)
- Field name sanitization: lowercase, replace non-alphanumeric with `_`, collapse consecutive `_`
- Only numeric values are exposed; non-numeric unknown fields are logged but not exported
- `zendure_exporter_unknown_fields_total` is incremented for each unknown field encountered

Discovery mode is **off by default** in production to prevent cardinality explosion.

---

## Enum Reference

For documentation in HELP strings and dashboards:

### packState / state
| Value | Meaning      |
|-------|--------------|
| 0     | Standby      |
| 1     | Charging     |
| 2     | Discharging  |

### acMode
| Value | Meaning      |
|-------|--------------|
| 1     | Charge       |
| 2     | Discharge    |

### gridStandard
| Value | Country       |
|-------|---------------|
| 0     | Germany       |
| 1     | France        |
| 2     | Austria       |
| 3     | Switzerland   |
| 4     | Netherlands   |
| 5     | Spain         |
| 6     | Belgium       |
| 7     | Greece        |
| 8     | Denmark       |
| 9     | Italy         |

### gridOffMode
| Value | Meaning        |
|-------|----------------|
| 0     | Standard Mode  |
| 1     | Economic Mode  |
| 2     | Closure        |

### acCouplingState (bitfield)
| Bit  | Meaning                                    |
|------|--------------------------------------------|
| Bit0 | AC-coupled input present (auto-cleared)    |
| Bit1 | AC input present flag                      |
| Bit2 | AC-coupled overload                        |
| Bit3 | Excess AC input power                      |

---

## Open Items

- [ ] Capture real sample payloads from SolarFlow800 Pro and SolarFlow2400 AC to validate field presence per model.
- [ ] Confirm whether `solarPower1`–`6` are always present or only on models with multiple PV inputs.
- [ ] Confirm battery pack data structure in `/properties/report` response (nested array? separate key?).
- [ ] Verify `hyperTmp` uses same 0.1K encoding as battery `maxTemp`.
- [ ] Check if any fields are monotonically increasing (candidates for counter type instead of gauge).
