<!--
 * @Author: dav1d wei.liu@zendure.com
 * @Date: 2025-03-05 18:45:36
 * @LastEditors: dav1dBoy 492664938@qq.com
 * @LastEditTime: 2026-01-04 14:40:33
 * @FilePath: /zenSDK/docs/en_properties.md
 * @Description:
 *
 * Copyright (c) 2025 by Zendure, All Rights Reserved.
-->

# SolarFlow Generic

Unified property definition for SolarFlow series devices.

---

## Document Conventions

- Power unit: W
- Voltage unit: V
- Temperature storage: 0.1 Kelvin unless stated otherwise
- Bit fields are defined in dedicated sections
- Ranges are inclusive
- Unless otherwise specified, all `int` values are non-negative

---

## Table Template 

Use this template when adding new property tables.

| Attribute | Type | Access | Unit | Range | Description |
|-----------|------|--------|------|-------|------------|
| example | int | RO/RW | W | 0-100 | Description here |

Access values:

- RO: Read Only
- RW: Read / Write

---

## Battery Pack Data Properties

| Attribute | Type | Access | Unit | Range | Description |
|-----------|------|--------|------|-------|------------|
| sn | string | RO | — | — | Battery pack serial number |
| packType | int | RO | — | — | Reserved |
| socLevel | int | RO | % | 0-100 | State of charge |
| state | int | RO | — | 0-2 | 0 Standby, 1 Charging, 2 Discharging |
| power | int | RO | W | — | Battery pack power |
| maxTemp | int | RO | 0.1K | — | Stored temperature value |
| totalVol | int | RO | V | — | Total voltage |
| batcur | int | RO | A | — | Battery current, see conversion |
| maxVol | int | RO | 0.01V | — | Max cell voltage |
| minVol | int | RO | 0.01V | — | Min cell voltage |
| softVersion | int | RO | — | — | Firmware version |
| heatState | int | RO | — | 0-1 | Heating state |

### Temperature Conversion

float maxTemp_C = (maxTemp - 2731) / 10.0;

### Battery Current Conversion

Raw format: 16-bit two’s complement  
Source container: uint8_t[2]  
Convert to signed int16  

current_A = value / 10.0;

---

## Device Data Properties — Read Only

| Attribute | Type | Access | Unit | Range | Description |
|-----------|------|--------|------|-------|------------|
| heatState | int | RO | — | 0-1 | Heating state |
| packInputPower | int | RO | W | — | Battery discharge power |
| outputPackPower | int | RO | W | — | Battery charge power |
| outputHomePower | int | RO | W | — | Output to home |
| remainOutTime | int | RO | min | — | Remaining discharge time |
| packState | int | RO | — | 0-2 | Pack state |
| packNum | int | RO | — | — | Number of packs |
| electricLevel | int | RO | % | 0-100 | Average SOC |
| gridInputPower | int | RO | W | — | Grid input power |
| solarInputPower | int | RO | W | — | Total PV input |
| solarPower1~6 | int | RO | W | — | PV channel power |
| pass | int | RO | — | 0-1 | Pass-through state |
| reverseState | int | RO | — | 0-1 | Reverse flow state |
| socStatus | int | RO | — | 0-1 | SOC calibration |
| hyperTmp | int | RO | — | — | Enclosure temperature |
| dcStatus | int | RO | — | 0-2 | DC state |
| pvStatus | int | RO | — | 0-1 | PV state |
| acStatus | int | RO | — | 0-2 | AC state |
| dataReady | int | RO | — | 0-1 | Data ready flag |
| gridState | int | RO | — | 0-1 | Grid connection state |
| BatVolt | int | RO | 0.01V | — | Battery voltage |
| FMVolt | int | RO | V | — | Voltage activation value |
| socLimit | int | RO | — | 0-2 | SOC limit state |
| rssi | int | RO | dBm | — | Signal strength |
| gridOffPower | int | RO | W | — | Off-grid power |
| lampSwitch | int | RO | — | 0-1 | Lamp state |
| gridOffMode | int | RO | — | — | Off-grid mode |
| IOTState | int | RO | — | — | IoT connection |
| fanSwitch | int | RO | — | 0-1 | Fan state |
| fanSpeed | int | RO | — | — | Fan level |
| faultLevel | int | RO | — | — | Fault severity |
| bindstate | int | RO | — | — | Bind state |
| VoltWakeup | int | RO | — | — | Voltage wake-up |
| OldMode | int | RO | — | — | Legacy mode |
| OTAState | int | RO | — | — | OTA state |
| LCNState | int | RO | — | — | LCN state |
| factoryModeState | int | RO | — | — | Factory mode |
| timestamp | int | RO | — | — | System timestamp |
| ts | int | RO | — | — | Unix timestamp |
| timeZone | int | RO | — | — | Timezone |
| tsZone | int | RO | — | — | Timezone offset |
| chargeMaxLimit | int | RO | W | — | Max charge power |
| phaseSwitch | int | RO | — | — | Phase switch |
| is_error | int | RO | — | 0-1 | Error flag |
| acCouplingState | int | RO | — | - | AC Coupling State |
| dryNodeState | int | RO | — | - | Dry contact status  1: Connected  0: Connected(May be reversed depending on actual wiring) |

---

## Device Data Properties — Read / Write

| Attribute | Type | Access | Unit | Range | Description |
|-----------|------|--------|------|-------|------------|
| writeRsp | N/A | RW | — | — | Write response |
| acMode | int | RW | — | 1-2 | Charge or discharge mode |
| inputLimit | int | RW | W | — | AC charge limit |
| outputLimit | int | RW | W | — | Output limit |
| socSet | int | RW | % | 70-100 | Target SOC |
| minSoc | int | RW | % | 0-50 | Minimum SOC |
| gridReverse | int | RW | — | 0-2 | Reverse flow control |
| inverseMaxPower | int | RW | W | — | Max inverter output |
| gridStandard | int | RW | — | 0-9 | 0: Germany 1: France 2: Austria 3: Switzerland 4: Netherlands 5: Spain 6: Belgium 7: Greece 8: Denmark 9: Italy|
| smartMode | int | RW | — | 0-1 | Flash write behavior |
| batCalTime | int | RW | — | - | Unit: minutes. Unauthorized modifications are not recommended |
| Fanmode | int | RW | — | 0-1 | 0: Fan off 1: Fan on. Unauthorized modification is not recommended. |
| Fanspeed | int | RW | — | 0-1 | 0: Auto; 1: 1st gear; 2: 2nd gear. Unauthorized modification is not recommended. |
| gridOffMode | int | RW | — | 0-2 | 0: Standard Mode 1: Economic Mode 2: Closure |

### smartMode Behavior

- 1: Parameters are not written to flash
- Device restores previous flash values after reboot
- Recommended for frequent configuration changes

---

## Bit Field Definitions

### AC Coupling State

| Bit | Meaning |
|-----|--------|
| Bit0 | AC-coupled input present, auto-cleared by DSP |
| Bit1 | AC input present flag |
| Bit2 | AC-coupled overload |
| Bit3 | Excess AC input power |

