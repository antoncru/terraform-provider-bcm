# BCM Sentinel Values & Array Element Safety

**Date:** 2026-04-07

## What are sentinels?

BCM's JSON-RPC API is strongly typed — every field must have a value of the declared type. Where Terraform would use `null` to mean "not set," BCM uses a **type-appropriate placeholder** called a sentinel value. These look like valid data but actually signal "nothing here."

| Sentinel Value | Type | BCM Meaning |
|---|---|---|
| `"0.0.0.0"` | IP address | No gateway configured |
| `"::0"` | IPv6 address | No IPv6 address configured |
| `"00:00:00:00:00:00"` | MAC address | No MAC / auto-detect |
| `"00000000-0000-0000-0000-000000000000"` | UUID | No reference / unset |
| `"CATEGORY"` | String keyword | Inherit value from the device's category |
| `""` | Empty string | Not configured |
| `[]` | Empty array | No entries |

## Why the provider maps sentinels to `null`

If sentinels were stored directly in Terraform state, two problems occur:

1. **False drift on every plan.** A user who doesn't set `default_gateway` expects no changes. But if state holds `"0.0.0.0"` and config has no value, Terraform sees a difference and proposes an update every time.

2. **Confusing state output.** `terraform state show` would display `default_gateway = "0.0.0.0"` or `from_template_node = "00000000-0000-0000-0000-000000000000"` — meaningless noise.

Mapping sentinels to `null` makes state match the absence of config and keeps output clean.

## How sentinels flow through the provider

### Read path (BCM → Terraform state)

Each sentinel is intercepted during parsing and converted to `null`:

```go
// Example: defaultGateway
if defaultGateway, ok := data["defaultGateway"].(string); ok && defaultGateway != "" && defaultGateway != "0.0.0.0" {
    model.DefaultGateway = types.StringValue(defaultGateway)
} else {
    model.DefaultGateway = types.StringNull()
}
```

The helper functions handle the common cases automatically:
- `getStringValue` — returns `null` for `""` and missing keys
- `getBoolValue` — returns `BoolValue(false)` for `false` (bools are never null when present)
- `getInt64Value` — returns `Int64Value(0)` for `0` (ints are never null when present)
- `getJSONValue` — returns `null` for `nil`, `"null"`, and `"[]"`
- `GetStringListValue` — returns `null` for `[]` and missing keys

### Write path (Terraform state → BCM)

**Sentinels are never reconstructed.** When a field is `null`, the provider simply omits the key from the JSON-RPC payload. This works because of BCM's patch semantics for top-level device fields:

| Payload pattern | BCM behavior |
|---|---|
| Key absent from JSON | **Keep existing value** (no change) |
| Key present with value | Set to that value |

The round-trip:
```
BCM stores: "0.0.0.0"
     ↓ read
Provider parses: null  (sentinel detected)
     ↓ write
Provider sends: (key omitted)
     ↓ patch semantics
BCM keeps: "0.0.0.0"
```

The sentinel stays on the BCM side without the provider ever needing to reproduce it.

## Array element safety: the nuance

BCM uses **two different semantics** depending on the field type:

| Field type | BCM behavior |
|---|---|
| Top-level scalar fields | **Patch** — omitted keys preserve existing values |
| Array fields (`interfaces[]`, `services[]`, `roles[]`) | **Full replacement** — the entire array is replaced |

This distinction means that within array elements, **omitting a field could cause BCM to reset it to a default** rather than preserving it. While empirical testing shows BCM may tolerate partial array elements (a working `updateDevice` call succeeded with many fields omitted from interface objects), the omitted fields all had default values — so we cannot distinguish "BCM preserved them" from "BCM reset them to defaults."

### What changed

**Before (omit-when-null pattern for array element fields):**

```go
// Within buildInterfaceAPIEntity:
SetStringField(entity, "gateway", iface.Gateway)       // omitted when null
SetInt64Field(entity, "lanchannel", iface.LanChannel)   // omitted when null
SetStringField(entity, "speed", iface.Speed)            // omitted when null
```

This is safe for top-level device fields (patch semantics), but risky for fields within array elements where the element is fully replaced.

**After (explicit-default pattern for array element fields):**

```go
// Within buildInterfaceAPIEntity:
if !iface.Gateway.IsNull() && !iface.Gateway.IsUnknown() {
    entity["gateway"] = iface.Gateway.ValueString()
} else {
    entity["gateway"] = "0.0.0.0"
}
if !iface.LanChannel.IsNull() && !iface.LanChannel.IsUnknown() {
    entity["lanchannel"] = iface.LanChannel.ValueInt64()
} else {
    entity["lanchannel"] = int64(1)
}
if !iface.Speed.IsNull() && !iface.Speed.IsUnknown() {
    entity["speed"] = iface.Speed.ValueString()
} else {
    entity["speed"] = ""
}
```

Every field is now always sent with an explicit value — either the user's configured value or the BCM default. This matches the pattern already established for the original core fields (`dhcp`, `startIf`, `ipv6Ip`, `bootable`, `bringupduringinstall`).

### The design rule

| Context | Pattern | Rationale |
|---|---|---|
| **Top-level device fields** | `Set*Field` (omit when null) | BCM patch semantics preserve existing values |
| **Fields within array elements** (`interfaces[]`, `services[]`) | Explicit default when null | Array is fully replaced — every field on every element must be present |

### Which arrays are affected

| Array | Element construction | Needs explicit defaults? |
|---|---|---|
| `interfaces[]` | Built element-by-element in `buildInterfaceAPIEntity` | **Yes** — fixed |
| `services[]` | Built element-by-element in `buildDeviceServicesAPI` | **Yes** — fixed |
| `roles[]` | Built by `lookupAndBuildRolesForEntity` / `buildKubernetesRolesForEntity` | Yes, but those already build complete objects |
| `blockDevicesClearedOnNextBoot[]` | Flat string list, sent as complete `[]string` | No — no element-level fields |
| `modules[]` | Flat string list, sent as complete `[]string` | No — no element-level fields |
| `fsexports[]`, `fsmounts[]`, etc. | JSON-encoded string, deserialized whole via `SetJSONField` | No — sent as-is from user's JSON |

### Default values used in array element builds

**Interface element defaults:**

| Field | Default | Source |
|---|---|---|
| `ipv6Ip` | `"::0"` | BCM IPv6 sentinel |
| `dhcp` | `true` | BCM default for new interfaces |
| `bootable` | `false` | BCM default |
| `startIf` | `"ALWAYS"` | BCM default |
| `ipv6Dhcp` | `false` | BCM default |
| `bringupduringinstall` | `"NO"` | BCM default |
| `gateway` | `"0.0.0.0"` | BCM sentinel for "no gateway" |
| `lanchannel` | `1` | BCM default IPMI LAN channel |
| `onNetworkPriority` | `0` | BCM default priority |
| `vlanid` | `0` | BCM default (no VLAN) |
| `alternativeHostname` | `""` | BCM default |
| `connectedMode` | `false` | BCM default |
| `speed` | `""` | BCM default |
| `additionalHostnames` | `[]` | BCM default |
| `cardtype` | `"Ethernet"` / `"BMC"` | Derived from interface type |

**Service element defaults:**

| Field | Default | Source |
|---|---|---|
| `addFromRole` | `false` | BCM default |
| `autostart` | `false` | BCM default |
| `belongsToRole` | `false` | BCM default |
| `monitored` | `false` | BCM default |
| `ref_extra_uuid` | `"00000000-..."` | BCM zero UUID sentinel |
| `ref_role_uuid` | `"00000000-..."` | BCM zero UUID sentinel |
| `runIf` | `"ALWAYS"` | BCM default |
| `scriptTimeout` | `-1` | BCM default (no timeout) |
| `serviceType` | `0` | BCM default |
| `sicknessCheckInterval` | `60` | BCM default (seconds) |
| `sicknessCheckScriptTimeout` | `10` | BCM default (seconds) |
| `childType` | `""` | BCM default |
| `fromGenericRole` | `false` | BCM default |
| `internal` | `false` | BCM default |
| `sicknessCheckScript` | `""` | BCM default |

## Exception: `services[]` UUID fields

`ref_extra_uuid` and `ref_role_uuid` in the `services` block are parsed with `deviceStringFromKeys` → `getStringValue`. When BCM returns the zero UUID, that value is a non-empty string, so state keeps the literal `"00000000-0000-0000-0000-000000000000"` rather than mapping to null. On write, null in Terraform is converted back to the zero UUID explicitly (see §"Array element safety" above, service element defaults table).

## Code references

Device-level sentinel parsing (`resource_cmdevice_device.go`):

- `mac`: `"00:00:00:00:00:00"` → null
- `managementNetwork`: zero UUID → null
- `bootLoader` / `bootLoaderProtocol` / `fips`: `"CATEGORY"` → null
- `fromTemplateNode`: zero UUID → null
- `parentUuid` / `parent_uuid`: zero UUID → null
- `defaultGateway`: `"0.0.0.0"` → null
- `defaultGatewayMetric`: `0` → null

Interface-level sentinel parsing (`resource_cmdevice_device_interfaces.go`):

- `ip`: `"0.0.0.0"` → null
- `ipv6Ip`: `"::0"` → null
- `gateway`: `"0.0.0.0"` → null
- `mac`: `"00:00:00:00:00:00"` → null

## Known limitation: clearing fields

Removing an optional field from Terraform config does **not** clear it in BCM. For `Optional+Computed` fields, Terraform propagates the prior state value when config is null, so no update is triggered and BCM retains whatever it has. To clear a field, users must set an explicit empty value (e.g., `notes = ""`), not simply remove the attribute.

## Evidence

- **Patch semantics verified** via `sampleRest/investigate_management_network_create.py` which tested all permutations for `managementNetwork` against the live API.
- **Array replacement confirmed** by comparing the working `update-cpu-03-3.json` (minimal interface objects) against the full `cpu-03.device` JSON — BCM accepted both, but since all omitted fields had default values, we chose the safer explicit-default approach.
- **Full field audit** documented in `ai_reports/cpu-03-device-json-audit.md`.
- **Provider field analysis:** `notes/bcm-provider-fields-analysis.md` (device), `notes/bcm-provider-network-analysis.md` (network), `notes/bcm-provider-category-analysis.md` (category).
