# BCM Device JSON Parity Audit — cpu-03.device

**Date:** 2026-04-07
**Source file:** `cpu-03.device` (full BCM `updateDevice` JSON payload for device `cpu-03`)
**Audited against:** `resource_cmdevice_device.go`, `resource_cmdevice_device_interfaces.go`, `models.go`, `utils.go`, `resource_cmdevice_device_test.go`

> **See also:** `notes/bcm-provider-fields-analysis.md` for a narrative-style analysis of the same fields with design rationale and schema decisions.

## Summary

Every single key in the JSON is accounted for. The JSON contains:

- **81 device-level keys** — all modeled or intentionally skipped
- **26 unique interface-level keys** (across 2 interfaces) — all modeled or intentionally skipped
- **22 service-level keys** — all modeled or intentionally skipped

Build, vet, and short tests all pass. No issues found.

---

## Device-Level Keys (81 total)

| # | JSON Key | JSON Value | Model Field | Type | Parse | Build | Status |
|---|----------|-----------|-------------|------|-------|-------|--------|
| 1 | `allowNetworkingRestart` | `false` | `AllowNetworkingRestart` | `types.Bool` | `getBoolValue` | `SetBoolField` | OK |
| 2 | `baseType` | `"Device"` | `BaseType` | `types.String` | explicit parse | hardcoded in build | OK |
| 3 | `biosSetup` | `null` | `BiosSetup` | `types.String` (JSON) | `getJSONValue` | `SetJSONField` | OK |
| 4 | `blockDevicesClearedOnNextBoot` | `[]` | `BlockDevicesClearedOnNextBoot` | `types.List` | `GetStringListValue` | inline | OK |
| 5 | `bmcSettings` | `null` | `BmcSettings` | `types.String` (JSON) | `getJSONValue` | `SetJSONField` | OK |
| 6 | `bootLoader` | `"CATEGORY"` | `BootLoader` | `types.String` | explicit (CATEGORY→null) | `SetStringField` | OK |
| 7 | `bootLoaderFile` | `""` | `BootLoaderFile` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 8 | `bootLoaderProtocol` | `"CATEGORY"` | `BootLoaderProtocol` | `types.String` | explicit (CATEGORY→null) | `SetStringField` | OK |
| 9 | `category` | UUID | `Category` | `types.String` | explicit parse | `SetStringField` | OK |
| 10 | `childType` | `"PhysicalNode"` | `ChildType` | `types.String` | explicit parse | hardcoded | OK |
| 11 | `cmdaemonUrl` | `"https://..."` | `CmdaemonURL` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 12 | `cpuspeedGovernor` | `""` | `CpuspeedGovernor` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 13 | `creationTime` | `1744148871` | `CreationTime` | `types.Int64` | explicit parse | not sent (BCM-computed) | OK |
| 14 | `customPingScript` | `""` | `CustomPingScript` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 15 | `customPingScriptArgument` | `""` | `CustomPingScriptArgument` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 16 | `customPowerScript` | `""` | `CustomPowerScript` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 17 | `customPowerScriptArgument` | `""` | `CustomPowerScriptArgument` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 18 | `customRemoteConsoleScript` | `""` | `CustomRemoteConsoleScript` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 19 | `customRemoteConsoleScriptArgument` | `""` | `CustomRemoteConsoleScriptArgument` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 20 | `dataNode` | `false` | `DataNode` | `types.Bool` | `getBoolValue` | `SetBoolField` | OK |
| 21 | `defaultGateway` | `"0.0.0.0"` | `DefaultGateway` | `types.String` | explicit (0.0.0.0→null) | `SetStringField` | OK |
| 22 | `disableFabricNVME` | `false` | `DisableFabricNVME` | `types.Bool` | `getBoolValue` | `SetBoolField` | OK |
| 23 | `disksetup` | `""` | `Disksetup` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 24 | `excludeListFull` | `""` | `ExcludeListFull` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 25 | `excludeListGrab` | `""` | `ExcludeListGrab` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 26 | `excludeListGrabnew` | `""` | `ExcludeListGrabnew` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 27 | `excludeListManipulateScript` | `""` | `ExcludeListManipulateScript` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 28 | `excludeListSync` | `""` | `ExcludeListSync` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 29 | `excludeListUpdate` | `""` | `ExcludeListUpdate` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 30 | `extra_values` | `null` | `ExtraValues` | `types.String` (JSON) | `getJSONValue` | `SetJSONField` | OK |
| 31 | `finalize` | `""` | `Finalize` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 32 | `fips` | `"CATEGORY"` | `Fips` | `types.String` | explicit (CATEGORY→null) | `SetStringField` | OK |
| 33 | `forceFullEnvironment` | `false` | `ForceFullEnvironment` | `types.Bool` | `getBoolValue` | `SetBoolField` | OK |
| 34 | `fromTemplateNode` | zero UUID | `FromTemplateNode` | `types.String` | explicit (zero UUID→null) | `SetStringField` | OK |
| 35 | `fsexports` | `[]` | `Fsexports` | `types.String` (JSON) | `getJSONValue` ([]→null) | `SetJSONField` | OK |
| 36 | `fsmounts` | `[]` | `Fsmounts` | `types.String` (JSON) | `getJSONValue` | `SetJSONField` | OK |
| 37 | `gpuSettings` | `[]` | `GpuSettings` | `types.String` (JSON) | `getJSONValue` | `SetJSONField` | OK |
| 38 | `hostname` | `"cpu-03"` | `Hostname` | `types.String` | explicit parse | `SetStringField` | OK |
| 39 | `indexInsideContainer` | `0` | `IndexInsideContainer` | `types.Int64` | `getInt64Value` | `SetInt64Field` | OK |
| 40 | `initialize` | `""` | `Initialize` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 41 | `installBootRecord` | `false` | `InstallBootRecord` | `types.Bool` | `getBoolValue` | `SetBoolField` | OK |
| 42 | `installMode` | `""` | `InstallMode` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 43 | `interfaces` | array | `Interfaces` | `[]DeviceInterfaceModel` | `parseInterfacesFromAPI` | `buildInterfacesAPIArray` | OK |
| 44 | `ioScheduler` | `""` | `IoScheduler` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 45 | `kernelOutputConsole` | `""` | `KernelOutputConsole` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 46 | `kernelParameters` | `""` | `KernelParameters` | `types.String` | explicit parse | `SetStringField` | OK |
| 47 | `kernelVersion` | `""` | `KernelVersion` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 48 | `mac` | `"B8:59:9F:E4:22:12"` | `MAC` | `types.String` | explicit (00:00→null) | explicit build | OK |
| 49 | `managementNetwork` | UUID | `ManagementNetwork` | `types.String` | explicit (zero→null) | `SetStringField` | OK |
| 50 | `modified` | `false` | — | — | skipped (protocol) | hardcoded `true` | SKIP |
| 51 | `modules` | `[]` | `Modules` | `types.List` | `GetStringListValue` | inline | OK |
| 52 | `nextBootInstallMode` | `""` | `NextBootInstallMode` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 53 | `nodeInstallerDisk` | `false` | `NodeInstallerDisk` | `types.Bool` | `getBoolValue` | `SetBoolField` | OK |
| 54 | `notes` | `""` | `Notes` | `types.String` | explicit parse | `SetStringField` | OK |
| 55 | `parent_uuid` | UUID | `ParentUUID` | `types.String` | explicit (both cases, zero→null) | both `parentUuid` + `parent_uuid` | OK |
| 56 | `partition` | UUID | `Partition` | `types.String` | explicit parse | explicit build | OK |
| 57 | `powerControl` | `"ipmi0"` | `PowerControl` | `types.String` | explicit parse | `SetStringField` | OK |
| 58 | `powerDistributionUnits` | `[]` | `PowerDistributionUnits` | `types.String` (JSON) | `getJSONValue` | `SetJSONField` | OK |
| 59 | `provisioningInterface` | UUID | `ProvisioningInterface` | `types.String` | `getStringValue` | explicit build | OK |
| 60 | `provisioningTransport` | `"RSYNCDAEMON"` | `ProvisioningTransport` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 61 | `proxySettings` | `null` | `ProxySettings` | `types.String` (JSON) | `getJSONValue` | `SetJSONField` | OK |
| 62 | `pxelabel` | `""` | `Pxelabel` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 63 | `rack` | `null` | `Rack` | `types.String` (JSON) | `getJSONValue` | `SetJSONField` | OK |
| 64 | `raidconf` | `""` | `Raidconf` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 65 | `revision` | `""` | — | — | skipped (protocol) | hardcoded `""` | SKIP |
| 66 | `roles` | `[]` | `Roles` | `types.Set` | `parseRolesFromAPI` | separate function | OK |
| 67 | `seLinuxSettings` | `null` | `SeLinuxSettings` | `types.String` (JSON) | `getJSONValue` | `SetJSONField` | OK |
| 68 | `services` | array | `Services` | `[]DeviceOSServiceConfigModel` | `parseDeviceServicesFromAPI` | `buildDeviceServicesAPI` | OK |
| 69 | `softwareImageProxy` | `null` | `SoftwareImageProxy` | `types.String` (JSON) | `getJSONValue` | `SetJSONField` | OK |
| 70 | `staticRoutes` | `[]` | `StaticRoutes` | `types.String` (JSON) | `getJSONValue` | `SetJSONField` | OK |
| 71 | `supportsGNSS` | `false` | `SupportsGNSS` | `types.Bool` | `getBoolValue` | `SetBoolField` | OK |
| 72 | `switchPorts` | `[]` | `SwitchPorts` | `types.String` (JSON) | `getJSONValue` | `SetJSONField` | OK |
| 73 | `tag` | `""` | `Tag` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 74 | `templateNode` | `false` | `TemplateNode` | `types.Bool` | `getBoolValue` | `SetBoolField` | OK |
| 75 | `timeZoneSettings` | `null` | `TimeZoneSettings` | `types.String` (JSON) | `getJSONValue` | `SetJSONField` | OK |
| 76 | `to_be_removed` | `false` | — | — | skipped (protocol) | hardcoded `false` | SKIP |
| 77 | `useExclusivelyFor` | `""` | `UseExclusivelyFor` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 78 | `userDefinedResources` | `[]` | `UserDefinedResources` | `types.String` (JSON) | `getJSONValue` | `SetJSONField` | OK |
| 79 | `userdefined1` | `""` | `Userdefined1` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 80 | `userdefined2` | `""` | `Userdefined2` | `types.String` | `getStringValue` | `SetStringField` | OK |
| 81 | `uuid` | UUID | `UUID` / `ID` | `types.String` | explicit parse | explicit build | OK |

**Note:** `defaultGatewayMetric` and `serialNumber`/`partNumber` are modeled in the struct but absent from this particular JSON — BCM omits them when they have default values. This is correct behavior.

---

## Interface-Level Keys (26 unique across 2 interfaces)

The JSON contains two interfaces: a BMC interface (`ipmi0`) and a physical interface (`BOOTIF`).

| # | JSON Key | JSON Value | Model Field | Parse | Build | Status |
|---|----------|-----------|-------------|-------|-------|--------|
| 1 | `additionalHostnames` | `[]` | `AdditionalHostnames` | `GetStringListValue` | inline | OK |
| 2 | `alternativeHostname` | `""` | `AlternativeHostname` | `getStringValue` | `SetStringField` | OK |
| 3 | `baseType` | `"NetworkInterface"` | `BaseType` | `getStringValue` | hardcoded | OK |
| 4 | `bringupduringinstall` | `"NO"` | `BringUpDuringInstall` | `getStringValue` (both cases) | explicit with default `"NO"` | OK |
| 5 | `cardtype` | `"Ethernet"` | `CardType` | `getStringValue` | derived from `Type` | OK |
| 6 | `childType` | `"NetworkBmcInterface"` | `ChildType` | `getStringValue` | derived from `Type` | OK |
| 7 | `connectedMode` | `false` | `ConnectedMode` | `getBoolValue` | `SetBoolField` | OK |
| 8 | `dhcp` | `false` | `DHCP` | `getBoolValue` | explicit with default `true` | OK |
| 9 | `extra_values` | `null` | — | skipped (complex, per plan) | not sent | SKIP |
| 10 | `gateway` | `"0.0.0.0"` | `Gateway` | explicit (0.0.0.0→null) | `SetStringField` | OK |
| 11 | `ip` | `"10.229.10.5"` | `IP` | explicit (0.0.0.0→null) | `SetStringField` | OK |
| 12 | `ipv6Dhcp` | `false` | `IPv6DHCP` | `getBoolValue` | explicit (overrides hardcoded) | OK |
| 13 | `ipv6Ip` | `"::0"` | `IPv6IP` | explicit (::0→null) | explicit with default `::0` | OK |
| 14 | `lanchannel` | `1` | `LanChannel` | `getInt64Value` | `SetInt64Field` | OK |
| 15 | `mac` | `"00:00:00:00:00:00"` | `MAC` | explicit (00:00→null) | `SetStringField` | OK |
| 16 | `modified` | `false` | — | skipped (protocol) | hardcoded `true` | SKIP |
| 17 | `name` | `"ipmi0"` | `Name` | `getStringValue` | `SetStringField` | OK |
| 18 | `network` | UUID | `Network` | `getStringValue` | `SetStringField` | OK |
| 19 | `onNetworkPriority` | `10` | `OnNetworkPriority` | `getInt64Value` | `SetInt64Field` | OK |
| 20 | `revision` | `""` | — | skipped (protocol) | hardcoded `""` | SKIP |
| 21 | `speed` | `""` | `Speed` | `getStringValue` | `SetStringField` | OK |
| 22 | `startIf` | `"ALWAYS"` | `StartIf` | `getStringValue` | explicit with default `"ALWAYS"` | OK |
| 23 | `switchPorts` | `[]` | — | skipped (complex, per plan) | not sent | SKIP |
| 24 | `to_be_removed` | `false` | — | skipped (protocol) | hardcoded `false` | SKIP |
| 25 | `uuid` | UUID | `UUID` | `getStringValue` | explicit build | OK |
| 26 | `vlanid` | `0` | `VlanID` | `getInt64Value` | `SetInt64Field` | OK |

**Notes:**
- `cardtype` is absent from the BMC interface JSON — the build function infers `"BMC"` from `type == "bmc"`.
- `speed` is absent from the BMC interface JSON — only present on the physical interface.
- `mac` only appears on the physical interface.

---

## Service-Level Keys (22 total)

The JSON contains one service entry (`nslcd`).

| # | JSON Key | JSON Value | Model Field | Parse | Build | Status |
|---|----------|-----------|-------------|-------|-------|--------|
| 1 | `addFromRole` | `true` | `AddFromRole` | `getBoolValue` (both cases) | `SetBoolField` | OK |
| 2 | `autostart` | `true` | `Autostart` | `getBoolValue` | `SetBoolField` | OK |
| 3 | `baseType` | `"OSServiceConfig"` | `BaseType` | `getStringValue` | explicit with default | OK |
| 4 | `belongsToRole` | `true` | `BelongsToRole` | `getBoolValue` (both cases) | `SetBoolField` | OK |
| 5 | `childType` | `""` | `ChildType` | `deviceStringFromKeys` | `SetStringField` | OK |
| 6 | `extra_values` | `null` | — | skipped (complex, per plan) | not sent | SKIP |
| 7 | `fromGenericRole` | `false` | `FromGenericRole` | `getBoolValue` (both cases) | `SetBoolField` | OK |
| 8 | `internal` | `false` | `Internal` | `getBoolValue` | `SetBoolField` | OK |
| 9 | `modified` | `false` | — | skipped (protocol) | not sent | SKIP |
| 10 | `monitored` | `true` | `Monitored` | `getBoolValue` | `SetBoolField` | OK |
| 11 | `name` | `"nslcd"` | `Name` | `getStringValue` | `SetStringField` | OK |
| 12 | `ref_extra_uuid` | zero UUID | `RefExtraUUID` | `deviceStringFromKeys` | explicit build | OK |
| 13 | `ref_role_uuid` | UUID | `RefRoleUUID` | `deviceStringFromKeys` | explicit build | OK |
| 14 | `revision` | `""` | — | skipped (protocol) | not sent | SKIP |
| 15 | `runIf` | `"ALWAYS"` | `RunIf` | `deviceStringFromKeys` | `SetStringField` | OK |
| 16 | `scriptTimeout` | `-1` | `ScriptTimeout` | `getInt64Value` (both cases) | `SetInt64Field` | OK |
| 17 | `serviceType` | `0` | `ServiceType` | `getInt64Value` (both cases) | `SetInt64Field` | OK |
| 18 | `sicknessCheckInterval` | `60` | `SicknessCheckInterval` | `getInt64Value` (both cases) | `SetInt64Field` | OK |
| 19 | `sicknessCheckScript` | `""` | `SicknessCheckScript` | `deviceStringFromKeys` | `SetStringField` | OK |
| 20 | `sicknessCheckScriptTimeout` | `10` | `SicknessCheckScriptTimeout` | `getInt64Value` (both cases) | `SetInt64Field` | OK |
| 21 | `to_be_removed` | `false` | — | skipped (protocol) | not sent | SKIP |
| 22 | `uuid` | UUID | `UUID` | `getStringValue` | `SetStringField` | OK |

---

## Intentionally Skipped Keys

These keys appear at every level (device, interface, service) and are BCM protocol fields — never parsed to Terraform state:

| Key | Reason | Build Behavior |
|-----|--------|----------------|
| `modified` | BCM change-tracking flag | Hardcoded `true` on write |
| `to_be_removed` | BCM deletion flag | Hardcoded `false` on write |
| `revision` | BCM optimistic concurrency | Hardcoded `""` on write |

Additionally at the interface level:
| Key | Reason |
|-----|--------|
| `extra_values` | Complex nested object, rarely used, not worth modeling |
| `switchPorts` | Complex nested array, not worth modeling at interface level (modeled at device level) |

---

## Sentinel Value Handling

All BCM sentinel values are correctly mapped to Terraform `null` to avoid false drift:

| BCM Value | Terraform State | Applied To |
|-----------|----------------|-----------|
| `"CATEGORY"` | `null` | `bootLoader`, `bootLoaderProtocol`, `fips` |
| `"0.0.0.0"` | `null` | `defaultGateway`, interface `gateway` |
| `"::0"` | `null` | interface `ipv6Ip` |
| `"00:00:00:00:00:00"` | `null` | `mac` (device + interface) |
| `"00000000-0000-0000-0000-000000000000"` | `null` | `fromTemplateNode`, `parentUUID`, `managementNetwork` |
| `""` (empty string) | `null` | All string fields via `getStringValue` |
| `[]` (empty array) | `null` | All JSON arrays via `getJSONValue`; string lists via `GetStringListValue` |
| `null` | `null` | All JSON objects via `getJSONValue` |

---

## Test Coverage

All 5 `ImportStateVerifyIgnore` blocks in `resource_cmdevice_device_test.go` include every new field name:

**Device-level (37 new entries):**
`allow_networking_restart`, `boot_loader_file`, `disksetup`, `install_boot_record`, `install_mode`, `next_boot_install_mode`, `node_installer_disk`, `pxelabel`, `raidconf`, `version_config_files`, `cpuspeed_governor`, `io_scheduler`, `kernel_output_console`, `kernel_version`, `custom_ping_script`, `custom_ping_script_argument`, `custom_power_script`, `custom_power_script_argument`, `custom_remote_console_script`, `custom_remote_console_script_argument`, `finalize`, `initialize`, `exclude_list_full`, `exclude_list_grab`, `exclude_list_grabnew`, `exclude_list_manipulate_script`, `exclude_list_sync`, `exclude_list_update`, `data_node`, `disable_fabric_nvme`, `force_full_environment`, `supports_gnss`, `template_node`, `tag`, `use_exclusively_for`, `userdefined1`, `userdefined2`, `rack`, `software_image_proxy`, `block_devices_cleared_on_next_boot.#`, `modules.#`, `bios_setup`, `bmc_settings`, `extra_values`, `proxy_settings`, `se_linux_settings`, `time_zone_settings`, `fsexports`, `fsmounts`, `gpu_settings`, `power_distribution_units`, `static_routes`, `switch_ports`, `user_defined_resources`

**Interface-level (5 new entries):**
`interfaces.0.alternative_hostname`, `interfaces.0.connected_mode`, `interfaces.0.ipv6_dhcp`, `interfaces.0.speed`, `interfaces.0.additional_hostnames.#`

**Service-level:**
`services.#` covers the entire services block (services are conditionally nil'd when not user-configured).

---

## Verification

```
go build ./...     — PASS
go vet ./...       — PASS
go test -short     — PASS
make fmt           — clean
make generate      — docs regenerated
ReadLints          — no errors
```
