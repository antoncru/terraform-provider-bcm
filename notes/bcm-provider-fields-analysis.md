# BCM Provider Fields Analysis

How the Terraform provider builds the `addDevice` JSON-RPC payload, field by field, compared against what BCM returns. Based on analysis of `cpu-03.json`, `cpu-05.json`, live API testing, and MySQL database inspection.

**Source code:** `resource_cmdevice_device.go` (`buildDeviceAPIEntityWithExisting`), `resource_cmdevice_device_interfaces.go` (`buildInterfaceAPIEntity`)
**Reference data:** `hashi-poc/demo-provision/raw-json-rpc/cpu-03.json`, `cpu-05.json`

---

## 1. BCM Patch Semantics

BCM uses **patch semantics for scalar device fields** and **full replacement for arrays** (interfaces, services, roles).

| Payload pattern | BCM behavior |
|-----------------|-------------|
| Key absent from JSON | **Keep existing value** (no change) |
| Key present with value | Set to that value |
| Key present with `""` / `0` / zero UUID | Set to empty/zero (clears the field) |
| Array field (interfaces, roles, services) | **Full replacement** — send the complete array |

**Provider strategy:** The `Set*Field` helpers (`SetStringField`, `SetBoolField`, etc.) omit null/unknown fields from the entity map. On create, BCM applies its own defaults for omitted fields. On update, BCM preserves existing values for omitted fields. Interfaces and services always send every field because BCM replaces the entire array.

---

## 2. Device-Level Fields

| BCM JSON Field | BCM Value (cpu-03) | Source | Provider Sends? | How |
|---|---|---|---|---|
| `allowNetworkingRestart` | `false` | Both | Conditional | `SetBoolField` — omitted if null |
| `baseType` | `"Device"` | Both | Always | Hardcoded `"Device"` |
| `biosSetup` | `null` | Both | Conditional | `SetJSONField` — omitted if null |
| `blockDevicesClearedOnNextBoot` | `[]` | Both | Conditional | Set from plan list, omitted if null |
| `bmcSettings` | `null` | Both | Conditional | `SetJSONField` — omitted if null |
| `bootLoader` | `"CATEGORY"` | Both | Conditional | `SetStringField` — omitted if null |
| `bootLoaderFile` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `bootLoaderProtocol` | `"CATEGORY"` | Both | Conditional | `SetStringField` — omitted if null |
| `category` | `"2ef0ee0e-..."` | Both | Always | `SetStringField` from plan |
| `childType` | `"PhysicalNode"` | Both | Always | Hardcoded `"PhysicalNode"` |
| `cmdaemonUrl` | `"https://..."` | Both | Conditional | `SetStringField` — omitted if null |
| `cpuspeedGovernor` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| **`creationTime`** | `1744148871` | **BCM only** | **Never** | BCM-managed timestamp |
| `customPingScript` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `customPingScriptArgument` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `customPowerScript` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `customPowerScriptArgument` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `customRemoteConsoleScript` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `customRemoteConsoleScriptArgument` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `dataNode` | `false` | Both | Conditional | `SetBoolField` — omitted if null |
| `defaultGateway` | `"0.0.0.0"` | Both | Conditional | `SetStringField` — omitted if null |
| **`defaultGatewayMetric`** | *(absent)* | **Provider only** | Conditional | `SetInt64Field` — omitted if null |
| `disableFabricNVME` | `false` | Both | Conditional | `SetBoolField` — omitted if null |
| `disksetup` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `excludeListFull` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `excludeListGrab` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `excludeListGrabnew` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `excludeListManipulateScript` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `excludeListSync` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `excludeListUpdate` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `extra_values` | `null` | Both | Conditional | `SetJSONField` — omitted if null |
| `finalize` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `fips` | `"CATEGORY"` | Both | Conditional | `SetStringField` — omitted if null |
| `forceFullEnvironment` | `false` | Both | Conditional | `SetBoolField` — omitted if null |
| `fromTemplateNode` | `"00000000-..."` | Both | Conditional | `SetStringField` — omitted if null |
| `fsexports` | `[]` | Both | Conditional | `SetJSONField` — omitted if null |
| `fsmounts` | `[]` | Both | Conditional | `SetJSONField` — omitted if null |
| `gpuSettings` | `[]` | Both | Conditional | `SetJSONField` — omitted if null |
| `hostname` | `"cpu-03"` | Both | Always | `SetStringField` from plan |
| `indexInsideContainer` | `0` | Both | Conditional | `SetInt64Field` — omitted if null |
| `initialize` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `installBootRecord` | `false` | Both | Conditional | `SetBoolField` — omitted if null |
| `installMode` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `interfaces` | `[...]` | Both | Always | `buildInterfacesAPIArray` |
| `ioScheduler` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `kernelOutputConsole` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `kernelParameters` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `kernelVersion` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `mac` | `"B8:59:9F:E4:22:12"` | Both | Always | Derived from plan.MAC or first interface |
| `managementNetwork` | `"161d0552-..."` | Both | Conditional | `SetStringField` — omitted if null |
| `modified` | `false` | Both | Always | Hardcoded `true` (BCM returns `false`) |
| `modules` | `[]` | Both | Conditional | Set from plan list, omitted if null |
| `nextBootInstallMode` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `nodeInstallerDisk` | `false` | Both | Conditional | `SetBoolField` — omitted if null |
| `notes` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `parent_uuid` | `"f1ddabcd-..."` | Both | Always on create | Auto-set to deviceUUID on create (self-reference); on update, honored from plan/state |
| `partition` | `"11b3d293-..."` | Both | Always | Resolved from category |
| **`partNumber`** | *(absent)* | **Provider only** | Conditional | `SetStringField` — omitted if null |
| `powerControl` | `"ipmi0"` | Both | Conditional | `SetStringField` — omitted if null |
| `powerDistributionUnits` | `[]` | Both | Conditional | `SetJSONField` — omitted if null |
| `provisioningInterface` | `"e9046e04-..."` | Both | Always | Generated by `deriveProvisioningInterface()` |
| `provisioningTransport` | `"RSYNCDAEMON"` | Both | Conditional | `SetStringField` — omitted if null |
| `proxySettings` | `null` | Both | Conditional | `SetJSONField` — omitted if null |
| `pxelabel` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `rack` | `null` | Both | Conditional | `SetJSONField` — omitted if null |
| `raidconf` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `revision` | `""` | Both | Always | Hardcoded `""` |
| `roles` | `[]` | Both | Always | Built by `lookupAndBuildRolesForEntity` |
| `seLinuxSettings` | `null` | Both | Conditional | `SetJSONField` — omitted if null |
| **`serialNumber`** | *(absent)* | **Provider only** | Conditional | `SetStringField` — omitted if null |
| `services` | `[...]` | Both | Conditional | `buildDeviceServicesAPI(models, deviceUUID)` — omitted if empty; `ref_role_uuid` defaults to deviceUUID |
| `softwareImageProxy` | `null` | Both | Conditional | `SetJSONField` — omitted if null |
| `staticRoutes` | `[]` | Both | Conditional | `SetJSONField` — omitted if null |
| `supportsGNSS` | `false` | Both | Conditional | `SetBoolField` — omitted if null |
| `switchPorts` | `[]` | Both | Conditional | `SetJSONField` — omitted if null |
| `tag` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `templateNode` | `false` | Both | Conditional | `SetBoolField` — omitted if null |
| `timeZoneSettings` | `null` | Both | Conditional | `SetJSONField` — omitted if null |
| `to_be_removed` | `false` | Both | Always | Hardcoded `false` |
| `useExclusivelyFor` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `userDefinedResources` | `[]` | Both | Conditional | `SetJSONField` — omitted if null |
| `userdefined1` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `userdefined2` | `""` | Both | Conditional | `SetStringField` — omitted if null |
| `uuid` | `"f1ddabcd-..."` | Both | Always | Generated by `uuid.New()` on create |
| `versionConfigFiles` | `false` | Both | Conditional | `SetBoolField` — omitted if null |

## 3. Interface-Level Fields

Interfaces use **full array replacement** — every field is sent, including sentinel defaults.

| BCM JSON Field | BCM Value | Source | Provider Sends? | How |
|---|---|---|---|---|
| `additionalHostnames` | `[]` | Both | Always | Default `[]` if null |
| `alternativeHostname` | `""` | Both | Always | Default `""` if null |
| `baseType` | `"NetworkInterface"` | Both | Always | Hardcoded |
| **`bootable`** | *(absent)* | **Provider only** | Always | Default `false` if null — used for provisioning interface derivation |
| `bringupduringinstall` | `"NO"` | Both | Always | Default `"NO"` if null |
| `cardtype` | `"Ethernet"` | Both | Always | Generated from `type` (bmc→BMC, else→Ethernet) |
| `childType` | `"NetworkBmcInterface"` | Both | Always | Generated from `type` via `interfaceTypeToBCMChildType()` |
| `connectedMode` | `false` | Both | Always | Default `false` if null |
| `dhcp` | `false` | Both | Always | Default `false` if null |
| **`extra_values`** | `null` | **BCM only** | **Never** | Not sent by provider |
| `gateway` | `"0.0.0.0"` | Both | Always | Default `"0.0.0.0"` if null |
| `ip` | `"10.229.10.5"` | Both | Conditional | `SetStringField` — omitted if null |
| `ipv6Dhcp` | `false` | Both | Always | Default `false` |
| `ipv6Ip` | `"::0"` | Both | Always | Default `"::0"` if null |
| `lanchannel` | `1` | Both | Always | Default `1` if null |
| `mac` | `"00:00:00:00:00:00"` | Both | Conditional | `SetStringField` — omitted if null |
| `modified` | `false` | Both | Always | Hardcoded `true` (BCM returns `false`) |
| `name` | `"ipmi0"` | Both | Always | `SetStringField` from plan |
| `network` | `"47672401-..."` | Both | Conditional | `SetStringField` — omitted if null |
| `onNetworkPriority` | `10` | Both | Always | Default `0` if null |
| `revision` | `""` | Both | Always | Hardcoded `""` |
| `speed` | `""` | Both | Always | Default `""` if null |
| `startIf` | `"ALWAYS"` | Both | Always | Default `"ALWAYS"` if null |
| **`switchPorts`** | `[]` | **BCM only** | **Never** | Not sent by provider |
| `to_be_removed` | `false` | Both | Always | Hardcoded `false` |
| `uuid` | `"b2997c6b-..."` | Both | Always | Generated or preserved from state |
| `vlanid` | `0` | Both | Always | Default `0` if null |

### Interface type variations

BMC and physical interfaces return slightly different field sets from BCM:

| ipmi0 has (BMC-specific) | BOOTIF has (physical-specific) |
|---|---|
| `gateway` | `cardtype` |
| `lanchannel` | `mac` |
| `vlanid` | `speed` |

The provider sends all fields regardless of type (array replacement semantics). BCM accepts the extras silently.

## 4. Field Direction Mismatches

| Field | Direction | Notes |
|---|---|---|
| `bootable` (interface) | **Provider → BCM** | Provider always sends `false` by default; BCM doesn't return it. Used internally for provisioning interface derivation. |
| `defaultGatewayMetric` (device) | **Provider → BCM** | Provider supports it; absent from BCM JSON (may appear in newer BCM versions). |
| `serialNumber` (device) | **Provider → BCM** | Provider supports it; absent from this BCM JSON. |
| `partNumber` (device) | **Provider → BCM** | Provider supports it; absent from this BCM JSON. |
| `creationTime` (device) | **BCM → Provider** | BCM-managed; provider reads it but never sends it. |
| `extra_values` (interface) | **BCM → Provider** | BCM returns it; provider doesn't send it at interface level. |
| `switchPorts` (interface) | **BCM → Provider** | BCM returns it; provider doesn't send it at interface level. |

## 5. BCM Lifecycle Fields

Three fields appear on every entity as BCM's internal bookkeeping:

| Field | Value Sent | Purpose |
|---|---|---|
| `modified` | `true` | Signals BCM the entity has pending changes. BCM returns `false` after commit. |
| `to_be_removed` | `false` | Soft-delete flag. `true` marks entity for removal. Provider sends `false` on create/update. |
| `revision` | `""` | Optimistic concurrency control. Empty string = "overwrite regardless" (no revision check). |

Sent on both create and update (shared entity builder). BCM likely applies these defaults automatically on create, but this hasn't been tested with omitted values.

## 6. Omitted Device Fields (47 fields — BCM applies defaults)

When optional fields are null in the HCL config, the `Set*Field` helpers omit them. BCM applies these defaults:

| Default Value | Fields |
|---|---|
| `false` | `allowNetworkingRestart`, `dataNode`, `disableFabricNVME`, `forceFullEnvironment`, `installBootRecord`, `nodeInstallerDisk`, `supportsGNSS`, `templateNode`, `versionConfigFiles` |
| `""` | `bootLoaderFile`, `cpuspeedGovernor`, `customPingScript`, `customPingScriptArgument`, `customPowerScript`, `customPowerScriptArgument`, `customRemoteConsoleScript`, `customRemoteConsoleScriptArgument`, `disksetup`, `excludeListFull`, `excludeListGrab`, `excludeListGrabnew`, `excludeListManipulateScript`, `excludeListSync`, `excludeListUpdate`, `finalize`, `initialize`, `installMode`, `ioScheduler`, `kernelOutputConsole`, `kernelParameters`, `kernelVersion`, `nextBootInstallMode`, `notes`, `pxelabel`, `raidconf`, `tag`, `useExclusivelyFor`, `userdefined1`, `userdefined2` |
| `"CATEGORY"` | `bootLoader`, `bootLoaderProtocol`, `fips` |
| `"0.0.0.0"` | `defaultGateway` |
| Zero UUID | `fromTemplateNode` |
| `0` | `indexInsideContainer` |
| `null` | `biosSetup`, `bmcSettings`, `extra_values`, `proxySettings`, `rack`, `seLinuxSettings`, `softwareImageProxy`, `timeZoneSettings` |
| `[]` | `blockDevicesClearedOnNextBoot`, `fsexports`, `fsmounts`, `gpuSettings`, `modules`, `powerDistributionUnits`, `staticRoutes`, `switchPorts`, `userDefinedResources` |
| BCM-managed | `creationTime` |

## 7. Auto-Derived Fields

These fields are not in the HCL config — the provider sets them automatically on create:

| Field | How Set | Notes |
|---|---|---|
| `provisioning_interface` | `deriveProvisioningInterface()` → BOOTIF UUID | On create, any plan value is ignored (stale); on update, honored from state |
| `parent_uuid` | Auto-set to `deviceUUID` | Self-reference; BCM normalizes this to device UUID regardless of input |
| `ref_role_uuid` (services) | Auto-set to `deviceUUID` | Back-reference to owning device; BCM also normalizes this |

## 8. Provisioning Interface Derivation

`deriveProvisioningInterface()` in `resource_cmdevice_device.go` selects the correct interface UUID for PXE provisioning when the user doesn't set `provisioning_interface` explicitly.

**Priority chain:**

| Priority | Condition | Rationale |
|----------|-----------|-----------|
| 1 | `name == "BOOTIF"` (case-insensitive) | PXE boot interface convention — most reliable signal |
| 2 | `bootable == true` | Explicit flag, but BCM rarely sets it (usually `null`) |
| 3 | First non-BMC interface | BMC (IPMI/iLO/iDRAC) is out-of-band management, cannot PXE |

If no candidate is found (e.g., all interfaces are BMC), the function returns `""` and the caller raises an error — forcing the user to set `provisioning_interface` explicitly. The previous Priority 4 ("first interface" all-BMC fallback) was removed because it silently picked a BMC interface, causing validation errors.

### Create vs Update behavior

`provisioning_interface` is `Optional + Computed` with `UseStateForUnknown()`. The resolution differs by path:

| Stage | What happens |
|-------|-------------|
| **A. Auto-derive** | `deriveProvisioningInterface()` picks the best candidate from the built interfaces. |
| **B. User override** | On update only (`existingInterfaces != nil`): if the plan has a non-empty value, it overrides auto-derive. `UseStateForUnknown` copies prior state into the plan when config omits the field, so this fires on nearly every update. |
| **C. Guard** | If still empty, error before any API call. |

**On create:** Stage A always wins (Stage B is skipped because `existingInterfaces == nil` and freshly generated UUIDs make any config value stale).

**On update:** Stage B almost always wins (`UseStateForUnknown` provides prior state). Stage A is the fallback.

| Scenario | Result |
|----------|--------|
| User omits field from config | `UseStateForUnknown` → Stage B → previous value preserved |
| User sets field explicitly | Stage B uses user's UUID |
| User's UUID doesn't match any interface | BCM `validateDevice` rejects with `BAD_VALUE` |
| All interfaces BMC, field omitted | Stage A returns `""`, but state provides → Stage B preserves |
| All interfaces BMC, no prior state | Stage A `""`, Stage B empty → error raised |

## 9. Fixes Applied

| Fix | Before | After | Impact |
|-----|--------|-------|--------|
| `provisioningInterface` | Used stale UUID from plan on create | `deriveProvisioningInterface()` always wins on create | Prevents `BAD_VALUE` validation errors |
| `parent_uuid` | Omitted if null on create | Auto-set to `deviceUUID` | Matches BCM's expected self-reference pattern |
| `ref_role_uuid` | Defaulted to zero UUID | Auto-set to `deviceUUID` | Matches BCM's owning-device back-reference |
| `dhcp` default | `true` | `false` | Correct for static IP environments |

---

## Appendix A: Canonical addDevice Payload

Verified against live BCM instance on 2026-04-15. Device UUID `43a70c99-06e4-4ae0-a17c-23c5bb8a02f6`, created via `terraform apply` with minimal HCL config (no redundant defaults).

**JSON-RPC envelope:** `POST /json` with `{"service": "cmdevice", "call": "addDevice", "args": [entity, force]}` where `args[1]` is the `force` boolean (default `false`).

**API call sequence observed:**
1. `CMPart.getSoftwareImage` — partition commit check
2. `CMDevice.validateDevice` — pre-flight validation (passed)
3. `cmdevice.addDevice` — device creation (succeeded)
4. `cmdevice.getDevice` — read-back to populate state

```json
{
  "service": "cmdevice",
  "call": "addDevice",
  "args": [
    {
      "baseType": "Device",
      "childType": "PhysicalNode",
      "hostname": "cpu-05",
      "category": "b62cb533-e276-4c9b-99c6-2f5f1d0c0609",
      "mac": "B8:CE:F6:D9:47:BC",
      "uuid": "43a70c99-06e4-4ae0-a17c-23c5bb8a02f6",
      "parent_uuid": "43a70c99-06e4-4ae0-a17c-23c5bb8a02f6",
      "partition": "11b3d293-2183-4ecc-a4b1-5ffec6e7a466",
      "managementNetwork": "161d0552-5b1a-44b3-8ba0-abbadd22db73",
      "powerControl": "ipmi0",
      "cmdaemonUrl": "https://10.184.162.104:8081",
      "provisioningInterface": "826e6e85-863a-4b14-aba8-7b3a86de4166",
      "provisioningTransport": "RSYNCDAEMON",
      "modified": true,
      "to_be_removed": false,
      "revision": "",
      "interfaces": [
        {
          "baseType": "NetworkInterface",
          "childType": "NetworkBMCInterface",
          "cardtype": "BMC",
          "name": "ipmi0",
          "uuid": "e78030cb-9628-4dcc-b967-9f057ab077f0",
          "ip": "10.229.10.14",
          "network": "47672401-30e0-4bd2-9c75-9c3bd4ed2cf3",
          "dhcp": false,
          "bootable": false,
          "onNetworkPriority": 10,
          "lanchannel": 1,
          "connectedMode": false,
          "ipv6Dhcp": false,
          "ipv6Ip": "::0",
          "startIf": "ALWAYS",
          "bringupduringinstall": "NO",
          "gateway": "0.0.0.0",
          "vlanid": 0,
          "alternativeHostname": "",
          "additionalHostnames": [],
          "speed": "",
          "modified": true,
          "to_be_removed": false,
          "revision": ""
        },
        {
          "baseType": "NetworkInterface",
          "childType": "NetworkPhysicalInterface",
          "cardtype": "Ethernet",
          "name": "BOOTIF",
          "uuid": "826e6e85-863a-4b14-aba8-7b3a86de4166",
          "ip": "10.184.162.104",
          "network": "161d0552-5b1a-44b3-8ba0-abbadd22db73",
          "dhcp": false,
          "bootable": false,
          "onNetworkPriority": 60,
          "lanchannel": 1,
          "connectedMode": false,
          "ipv6Dhcp": false,
          "ipv6Ip": "::0",
          "startIf": "ALWAYS",
          "bringupduringinstall": "NO",
          "gateway": "0.0.0.0",
          "vlanid": 0,
          "alternativeHostname": "",
          "additionalHostnames": [],
          "speed": "",
          "modified": true,
          "to_be_removed": false,
          "revision": ""
        }
      ],
      "services": [
        {
          "baseType": "OSServiceConfig",
          "name": "nslcd",
          "addFromRole": true,
          "autostart": true,
          "belongsToRole": true,
          "monitored": true,
          "fromGenericRole": false,
          "internal": false,
          "childType": "",
          "ref_extra_uuid": "00000000-0000-0000-0000-000000000000",
          "ref_role_uuid": "43a70c99-06e4-4ae0-a17c-23c5bb8a02f6",
          "runIf": "ALWAYS",
          "scriptTimeout": -1,
          "serviceType": 0,
          "sicknessCheckInterval": 60,
          "sicknessCheckScript": "",
          "sicknessCheckScriptTimeout": 10,
          "modified": true,
          "to_be_removed": false,
          "revision": ""
        }
      ]
    },
    false
  ]
}
```

**Key observations:**
- `parent_uuid` and `uuid` are identical — BCM's self-reference pattern
- `ref_role_uuid` on nslcd matches device UUID — BCM's "owning entity" back-reference
- `provisioningInterface` matches BOOTIF interface UUID — correctly derived
- `dhcp: false` on both interfaces — static IP environment
- No redundant default fields (`dataNode`, `supportsGNSS`, etc.) — minimal config

**Field counts:**

| Category | Count | Description |
|---|---|---|
| Provider-generated (always sent) | 8 device + 8/interface | `uuid`, `baseType`, `childType`, `modified`, `revision`, `to_be_removed`, `partition`, `provisioningInterface` |
| User-provided (required) | 3 | `hostname`, `mac`, `category` |
| User-provided (optional, set here) | 4 | `managementNetwork`, `powerControl`, `cmdaemonUrl`, `provisioningTransport` |
| Auto-derived (not in HCL) | 2 device + 1/service | `parent_uuid`, `provisioningInterface`, `ref_role_uuid` |
| Sentinel defaults (interfaces only) | ~12/interface | `dhcp`, `gateway`, `vlanid`, `startIf`, etc. |
| Omitted (BCM defaults) | 47 | All empty strings, false, null, empty arrays |

---

## Appendix B: Attribute-by-Attribute Diff

Side-by-side comparison of every field in BCM's API response (`cpu-05.json`) against the provider's `addDevice` payload (minimal config, post all fixes).

**Legend:** 🟢 = identical, 🟡 ~ omitted default (semantically equivalent), 🔴 ! different value, 🔴 + provider-only, 🔴 - BCM-only.

### Device-Level Fields

| # | Field | BCM Returns | Provider Sends | Match | Notes |
|---|-------|-------------|----------------|-------|-------|
| 1 🟡 | `allowNetworkingRestart` | `false` | *(omitted)* | 🟡 ~ | BCM defaults to `false` |
| 2 🟢 | `baseType` | `"Device"` | `"Device"` | 🟢 = | |
| 3 🟡 | `biosSetup` | `null` | *(omitted)* | 🟡 ~ | |
| 4 🟡 | `blockDevicesClearedOnNextBoot` | `[]` | *(omitted)* | 🟡 ~ | |
| 5 🟡 | `bmcSettings` | `null` | *(omitted)* | 🟡 ~ | |
| 6 🟡 | `bootLoader` | `"CATEGORY"` | *(omitted)* | 🟡 ~ | |
| 7 🟡 | `bootLoaderFile` | `""` | *(omitted)* | 🟡 ~ | |
| 8 🟡 | `bootLoaderProtocol` | `"CATEGORY"` | *(omitted)* | 🟡 ~ | |
| 9 🟢 | `category` | `"b62cb533-..."` | `"b62cb533-..."` | 🟢 = | |
| 10 🟢 | `childType` | `"PhysicalNode"` | `"PhysicalNode"` | 🟢 = | |
| 11 🟢 | `cmdaemonUrl` | `"https://10.184.162.104:8081"` | `"https://10.184.162.104:8081"` | 🟢 = | |
| 12 🟡 | `cpuspeedGovernor` | `""` | *(omitted)* | 🟡 ~ | |
| 13 🔴 | `creationTime` | `1743107704` | *(omitted)* | 🔴 - | BCM-managed timestamp |
| 14–19 🟡 | `custom*Script*` (6 fields) | `""` | *(omitted)* | 🟡 ~ | |
| 20 🟡 | `dataNode` | `false` | *(omitted)* | 🟡 ~ | |
| 21 🟡 | `defaultGateway` | `"0.0.0.0"` | *(omitted)* | 🟡 ~ | |
| 22 🟡 | `disableFabricNVME` | `false` | *(omitted)* | 🟡 ~ | |
| 23–29 🟡 | `disksetup`, `excludeList*` (7 fields) | `""` | *(omitted)* | 🟡 ~ | |
| 30 🟡 | `extra_values` | `null` | *(omitted)* | 🟡 ~ | |
| 31–32 🟡 | `finalize`, `fips` | `""` / `"CATEGORY"` | *(omitted)* | 🟡 ~ | |
| 33 🟡 | `forceFullEnvironment` | `false` | *(omitted)* | 🟡 ~ | |
| 34–37 🟡 | `fromTemplateNode`, `fsexports`, `fsmounts`, `gpuSettings` | zero/`[]` | *(omitted)* | 🟡 ~ | |
| 38 🟢 | `hostname` | `"cpu-05"` | `"cpu-05"` | 🟢 = | |
| 39 🟡 | `indexInsideContainer` | `0` | *(omitted)* | 🟡 ~ | |
| 40–43 🟡 | `initialize`, `installBootRecord`, `installMode`, `ioScheduler` | `""` / `false` | *(omitted)* | 🟡 ~ | |
| 44–46 🟡 | `kernelOutputConsole`, `kernelParameters`, `kernelVersion` | `""` | *(omitted)* | 🟡 ~ | |
| 47 🟢 | `mac` | `"B8:CE:F6:D9:47:BC"` | `"B8:CE:F6:D9:47:BC"` | 🟢 = | |
| 48 🟢 | `managementNetwork` | `"161d0552-..."` | `"161d0552-..."` | 🟢 = | |
| 49 🔴 | `modified` | `false` | `true` | 🔴 ! | Provider signals pending changes |
| 50–53 🟡 | `modules`, `nextBootInstallMode`, `nodeInstallerDisk`, `notes` | `[]` / `""` / `false` | *(omitted)* | 🟡 ~ | |
| 54 🟢 | `parent_uuid` | `"86e28980-..."` | `"<deviceUUID>"` | 🟢 = | Auto-set to deviceUUID on create |
| 55 🟢 | `partition` | `"11b3d293-..."` | `"11b3d293-..."` | 🟢 = | |
| 56 🟢 | `powerControl` | `"ipmi0"` | `"ipmi0"` | 🟢 = | |
| 57 🟡 | `powerDistributionUnits` | `[]` | *(omitted)* | 🟡 ~ | |
| 58 🔴 | `provisioningInterface` | `"3faeb4b3-..."` | `"<BOOTIF-uuid>"` | 🔴 ! | Fresh UUID matching BOOTIF |
| 59 🟢 | `provisioningTransport` | `"RSYNCDAEMON"` | `"RSYNCDAEMON"` | 🟢 = | |
| 60–63 🟡 | `proxySettings`, `pxelabel`, `rack`, `raidconf` | `null` / `""` | *(omitted)* | 🟡 ~ | |
| 64 🟢 | `revision` | `""` | `""` | 🟢 = | |
| 65 🟢 | `roles` | `[]` | `[]` | 🟢 = | |
| 66–69 🟡 | `seLinuxSettings`, `services`, `softwareImageProxy`, `staticRoutes` | various | *(omitted or see below)* | 🟡 ~ | |
| 70 🟡 | `supportsGNSS` | `false` | *(omitted)* | 🟡 ~ | |
| 71–74 🟡 | `switchPorts`, `tag`, `templateNode`, `timeZoneSettings` | `[]` / `""` / `false` / `null` | *(omitted)* | 🟡 ~ | |
| 75 🟢 | `to_be_removed` | `false` | `false` | 🟢 = | |
| 76–79 🟡 | `useExclusivelyFor`, `userDefinedResources`, `userdefined1`, `userdefined2` | `""` / `[]` | *(omitted)* | 🟡 ~ | |
| 80 🔴 | `uuid` | `"86e28980-..."` | `"<generated>"` | 🔴 ! | Fresh UUID on create |
| 81 🟡 | `versionConfigFiles` | `false` | *(omitted)* | 🟡 ~ | |

### Interface: ipmi0 (BMC)

| # | Field | BCM Returns | Provider Sends | Match | Notes |
|---|-------|-------------|----------------|-------|-------|
| 1 🟢 | `additionalHostnames` | `[]` | `[]` | 🟢 = | |
| 2 🟢 | `alternativeHostname` | `""` | `""` | 🟢 = | |
| 3 🟢 | `baseType` | `"NetworkInterface"` | `"NetworkInterface"` | 🟢 = | |
| 4 🔴 | `bootable` | *(absent)* | `false` | 🔴 + | Provider-only |
| 5 🟢 | `bringupduringinstall` | `"NO"` | `"NO"` | 🟢 = | |
| 6 🔴 | `cardtype` | *(absent)* | `"BMC"` | 🔴 + | Provider-only; BCM omits for BMC |
| 7 🟢 | `childType` | `"NetworkBmcInterface"` | `"NetworkBmcInterface"` | 🟢 = | |
| 8 🟢 | `connectedMode` | `false` | `false` | 🟢 = | |
| 9 🟢 | `dhcp` | `false` | `false` | 🟢 = | |
| 10 🔴 | `extra_values` | `null` | *(omitted)* | 🔴 - | BCM-only |
| 11 🟢 | `gateway` | `"0.0.0.0"` | `"0.0.0.0"` | 🟢 = | |
| 12 🟢 | `ip` | `"10.229.10.14"` | `"10.229.10.14"` | 🟢 = | |
| 13 🟢 | `ipv6Dhcp` | `false` | `false` | 🟢 = | |
| 14 🟢 | `ipv6Ip` | `"::0"` | `"::0"` | 🟢 = | |
| 15 🟢 | `lanchannel` | `1` | `1` | 🟢 = | |
| 16 🔴 | `modified` | `false` | `true` | 🔴 ! | |
| 17 🟢 | `name` | `"ipmi0"` | `"ipmi0"` | 🟢 = | |
| 18 🟢 | `network` | `"47672401-..."` | `"47672401-..."` | 🟢 = | |
| 19 🟢 | `onNetworkPriority` | `10` | `10` | 🟢 = | |
| 20 🟢 | `revision` | `""` | `""` | 🟢 = | |
| 21 🔴 | `speed` | *(absent)* | `""` | 🔴 + | Provider sends default |
| 22 🟢 | `startIf` | `"ALWAYS"` | `"ALWAYS"` | 🟢 = | |
| 23 🔴 | `switchPorts` | `[]` | *(omitted)* | 🔴 - | BCM-only |
| 24 🟢 | `to_be_removed` | `false` | `false` | 🟢 = | |
| 25 🔴 | `uuid` | `"6a79ec10-..."` | `"<generated>"` | 🔴 ! | |
| 26 🟢 | `vlanid` | `0` | `0` | 🟢 = | |

### Interface: BOOTIF (Physical)

| # | Field | BCM Returns | Provider Sends | Match | Notes |
|---|-------|-------------|----------------|-------|-------|
| 1 🟢 | `additionalHostnames` | `[]` | `[]` | 🟢 = | |
| 2 🟢 | `alternativeHostname` | `""` | `""` | 🟢 = | |
| 3 🟢 | `baseType` | `"NetworkInterface"` | `"NetworkInterface"` | 🟢 = | |
| 4 🔴 | `bootable` | *(absent)* | `false` | 🔴 + | Provider-only |
| 5 🟢 | `bringupduringinstall` | `"NO"` | `"NO"` | 🟢 = | |
| 6 🟢 | `cardtype` | `"Ethernet"` | `"Ethernet"` | 🟢 = | |
| 7 🟢 | `childType` | `"NetworkPhysicalInterface"` | `"NetworkPhysicalInterface"` | 🟢 = | |
| 8 🟢 | `connectedMode` | `false` | `false` | 🟢 = | |
| 9 🟢 | `dhcp` | `false` | `false` | 🟢 = | |
| 10 🔴 | `extra_values` | `null` | *(omitted)* | 🔴 - | BCM-only |
| 11 🔴 | `gateway` | *(absent)* | `"0.0.0.0"` | 🔴 + | Provider sends default |
| 12 🟢 | `ip` | `"10.184.162.104"` | `"10.184.162.104"` | 🟢 = | |
| 13 🟢 | `ipv6Dhcp` | `false` | `false` | 🟢 = | |
| 14 🟢 | `ipv6Ip` | `"::0"` | `"::0"` | 🟢 = | |
| 15 🔴 | `lanchannel` | *(absent)* | `1` | 🔴 + | Provider sends default |
| 16 🟡 | `mac` | `"00:00:00:00:00:00"` | *(omitted)* | 🟡 ~ | |
| 17 🔴 | `modified` | `false` | `true` | 🔴 ! | |
| 18 🟢 | `name` | `"BOOTIF"` | `"BOOTIF"` | 🟢 = | |
| 19 🟢 | `network` | `"161d0552-..."` | `"161d0552-..."` | 🟢 = | |
| 20 🟢 | `onNetworkPriority` | `60` | `60` | 🟢 = | |
| 21 🟢 | `revision` | `""` | `""` | 🟢 = | |
| 22 🟢 | `speed` | `""` | `""` | 🟢 = | |
| 23 🟢 | `startIf` | `"ALWAYS"` | `"ALWAYS"` | 🟢 = | |
| 24 🔴 | `switchPorts` | `[]` | *(omitted)* | 🔴 - | BCM-only |
| 25 🟢 | `to_be_removed` | `false` | `false` | 🟢 = | |
| 26 🔴 | `uuid` | `"3faeb4b3-..."` | `"<BOOTIF-uuid>"` | 🔴 ! | Also used as `provisioningInterface` |
| 27 🔴 | `vlanid` | *(absent)* | `0` | 🔴 + | Provider sends default |

### Service: nslcd

| # | Field | BCM Returns | Provider Sends | Match | Notes |
|---|-------|-------------|----------------|-------|-------|
| 1 🟢 | `addFromRole` | `true` | `true` | 🟢 = | |
| 2 🟢 | `autostart` | `true` | `true` | 🟢 = | |
| 3 🟢 | `baseType` | `"OSServiceConfig"` | `"OSServiceConfig"` | 🟢 = | |
| 4 🟢 | `belongsToRole` | `true` | `true` | 🟢 = | |
| 5 🟢 | `childType` | `""` | `""` | 🟢 = | |
| 6 🔴 | `extra_values` | `null` | *(omitted)* | 🔴 - | BCM-only |
| 7 🟢 | `fromGenericRole` | `false` | `false` | 🟢 = | |
| 8 🟢 | `internal` | `false` | `false` | 🟢 = | |
| 9 🔴 | `modified` | `false` | `true` | 🔴 ! | |
| 10 🟢 | `monitored` | `true` | `true` | 🟢 = | |
| 11 🟢 | `name` | `"nslcd"` | `"nslcd"` | 🟢 = | |
| 12 🟢 | `ref_extra_uuid` | `"00000000-..."` | `"00000000-..."` | 🟢 = | |
| 13 🟢 | `ref_role_uuid` | `"86e28980-..."` | `"<deviceUUID>"` | 🟢 = | Auto-set to deviceUUID |
| 14 🟢 | `revision` | `""` | `""` | 🟢 = | |
| 15 🟢 | `runIf` | `"ALWAYS"` | `"ALWAYS"` | 🟢 = | |
| 16 🟢 | `scriptTimeout` | `-1` | `-1` | 🟢 = | |
| 17 🟢 | `serviceType` | `0` | `0` | 🟢 = | |
| 18 🟢 | `sicknessCheckInterval` | `60` | `60` | 🟢 = | |
| 19 🟢 | `sicknessCheckScript` | `""` | `""` | 🟢 = | |
| 20 🟢 | `sicknessCheckScriptTimeout` | `10` | `10` | 🟢 = | |
| 21 🟢 | `to_be_removed` | `false` | `false` | 🟢 = | |
| 22 🔴 | `uuid` | `"eec3d0ed-..."` | `"<generated>"` | 🔴 ! | |

### Cross-Entity Summary

| Entity | Total | 🟢 Exact | 🔴 Different | 🟡 Equivalent | 🔴 Provider-Only | 🔴 BCM-Only |
|--------|-------|---------|-------------|--------------|-----------------|------------|
| Device | 81 | 14 | 3 (`modified`, `uuid`, `provisioningInterface`) | 63 | 0 | 1 (`creationTime`) |
| ipmi0 | 26 | 17 | 2 (`modified`, `uuid`) | 0 | 3 (`bootable`, `cardtype`, `speed`) | 2 (`extra_values`, `switchPorts`) |
| BOOTIF | 27 | 16 | 2 (`modified`, `uuid`) | 1 (`mac`) | 4 (`bootable`, `gateway`, `lanchannel`, `vlanid`) | 2 (`extra_values`, `switchPorts`) |
| nslcd | 22 | 18 | 2 (`modified`, `uuid`) | 0 | 0 | 1 (`extra_values`) |
| **Total** | **156** | **65** | **9** | **64** | **7** | **6** |

---

## Appendix C: API & Database Verification (2026-04-14/15)

Direct testing of `addDevice` against BCM's API and MySQL database on a live cluster.

### Duplicate Device Detection

Attempting `addDevice` when cpu-05 already exists:

```json
{
  "success": false,
  "validation": [
    { "error_code": "DUPLICATE_FIELD", "field": "mac",
      "message": "A device with MAC B8:CE:F6:D9:47:BC already exists: cpu-05",
      "ref_entity_uuid": "b612c197-9475-42da-9fb6-2117ba8658e3" },
    { "error_code": "DUPLICATE_FIELD", "field": "hostname",
      "message": "A device with hostname cpu-05 already exists",
      "ref_entity_uuid": "b612c197-9475-42da-9fb6-2117ba8658e3" }
  ]
}
```

BCM validates uniqueness on both `mac` and `hostname` independently. `ref_entity_uuid` points to the existing device.

### Database Schema — UUID is Client-Generated

The `Devices` table has `uuid` as PRI with no auto-generation:

```
| uuid | varchar(36) | NO | PRI | | |
```

The **Default** and **Extra** columns are both empty — the database does nothing, the caller must provide the UUID. Compare with DB-generated patterns:

```
-- MySQL 8+ UUID generation:  Default = (uuid()),  Extra = DEFAULT_GENERATED
-- Auto-increment:            Default = NULL,       Extra = auto_increment
```

This confirms the provider's `uuid.New()` is the correct approach — BCM stores client-generated UUIDs as-is.

### Devices and Nodes Share UUID

```sql
select n.uuid, d.uuid, d.hostname from Nodes n join Devices d on n.uuid = d.uuid
where d.hostname="cpu-05";
-- Both return: 86e28980-674d-4e1b-93c9-2ae780cbfe6c
```

`PhysicalNode` = a `Nodes` row linked 1:1 with a `Devices` row via shared UUID.

### NetworkInterfaces — No Visible FK to Device

```sql
select * from NetworkInterfaces where uuid="3faeb4b3-80c7-42e1-8786-9cd9beb758f2";
-- Returns BOOTIF row; no column references the parent device
```

The device-to-interface relationship is managed internally by BCM (likely a join/mapping table or the JSON entity tree). After device deletion, the old interface UUID returns `Empty set`.

### Remove & Re-Add Cycle

Removed cpu-05 via cmsh, re-added via API with provider-generated payload:

```json
{ "success": true, "task_uuid": "00000000-...", "validation": [] }
```

Diff between original and re-added cpu-05 (only UUIDs and timestamps differ):

| Field | Original | Re-added |
|-------|----------|----------|
| `creationTime` | `1743107704` | `1776285009` |
| `uuid` (device) | `86e28980-...` | `b612c197-...` |
| `parent_uuid` | `86e28980-...` | `b612c197-...` |
| `provisioningInterface` | `3faeb4b3-...` | `6e1b2649-...` |
| `ref_role_uuid` | `86e28980-...` | `b612c197-...` |
| ipmi0/BOOTIF/nslcd `uuid` | *(original UUIDs)* | *(fresh UUIDs)* |
| `tag` | `""` | `"00000000a000"` |

All functional fields (hostname, mac, IPs, networks, service config) identical — the payload produces a functionally equivalent device.

### BCM Normalizes parent_uuid and ref_role_uuid

**Critical discovery:** BCM sets `parent_uuid` and `ref_role_uuid` to the device's UUID regardless of what the client sends. Verified by comparing sent vs returned values — all three match the device UUID.

**Implication:** Even incorrect values would be corrected by BCM. Sending correct values is still best practice to avoid relying on undocumented normalization and to keep `validateDevice` pre-flight checks consistent.

### Second Add Cycle — Consistent Behavior

A second remove/re-add (2026-04-15, post all provider fixes) produced identical behavior. Only UUIDs, timestamps, and `tag` differed. Payload is deterministic and correct.

### Summary of Database Findings

| Finding | Detail |
|---------|--------|
| UUID source | Client-generated (`uuid.New()`), not DB auto-increment |
| Duplicate detection | BCM validates uniqueness on `mac` and `hostname` independently |
| Device/Node relationship | `Devices` and `Nodes` joined on shared UUID |
| Interface-to-device FK | Not visible in `NetworkInterfaces` schema; managed internally |
| `parent_uuid` normalization | BCM sets to device UUID regardless of input |
| `ref_role_uuid` normalization | BCM sets to device UUID regardless of input |
| `tag` default | BCM applies `"00000000a000"` if empty string sent |
| Remove/re-add | Produces functionally identical device with fresh UUIDs |
