# BCM 10 → BCM 11 Device Entity Compatibility

BCM 11 introduces structural changes to the device entity JSON schema. This document catalogs every field difference, explains why the provider's architecture handles most of them transparently, and describes the two fixes required for full compatibility.

**Source entities analyzed:**
- BCM 11: `dgx-04.json`, `cpu-04.json` (from `bcm11-casper/bcm11-json/`)
- BCM 10: `dgx-03.json`, `cpu-03.json` (from `bcm10-json/`)

---

## How the Provider Handles Version-Specific Fields

The provider's null-safe getter/setter pattern makes most field differences invisible — no code changes required. Two mechanisms work together:

**On Read** — null-safe getters return null for absent keys:

```go
func getStringValue(data map[string]interface{}, key string) types.String {
    if val, ok := data[key]; ok && val != nil {
        if str, ok := val.(string); ok && str != "" {
            return types.StringValue(str)
        }
    }
    return types.StringNull()  // absent key → null, empty string → null
}
```

`getBoolValue` and `getInt64Value` follow the same pattern. When a BCM version omits a field, the getter returns null. For Optional+Computed attributes the user didn't set, Terraform's plan is `Unknown`, and `Unknown → null` is always a valid transition.

**On Write** — null-guarded setters omit unset fields:

```go
func SetStringField(entity map[string]interface{}, key string, value types.String) {
    if !value.IsNull() && !value.IsUnknown() {
        entity[key] = value.ValueString()
    }
}
```

When the user doesn't configure a field, the setter skips it — the key never appears in the API request. BCM's patch semantics preserve the existing value for omitted keys. The unsupporting BCM version never sees the field.

**Combined behavior for any Optional+Computed field that exists in only one BCM version:**

| User config | Write to unsupporting BCM | Read from unsupporting BCM |
|---|---|---|
| Not set | Field omitted from request — safe | Key absent → null — safe |
| Explicitly set | Field sent — BCM ignores it | Key absent → null → **plan/state mismatch error** |

The only failure mode is explicitly configuring a version-specific field against a BCM that doesn't support it.

**Example: setting `tag = "bar"` on BCM 11 (which removed the `tag` field):**

1. Terraform plan resolves `tag` to the concrete value `"bar"` from the user's config
2. Provider Create: `SetStringField(entity, "tag", plan.Tag)` adds `"tag": "bar"` to the API request
3. BCM 11 receives the entity. It ignores the unrecognized `tag` key and creates the device
4. Provider reads back the device. BCM 11's response has no `tag` field. `getStringValue(data, "tag")` returns null
5. Terraform compares plan (`"bar"`) to state (`null`) → **"Provider produced inconsistent result after apply"**
6. The device exists in BCM, but Terraform doesn't save the state. On the next plan, Terraform sees no device in state and tries to create it again — likely hitting a "resource already exists" error from BCM

This is an expected consequence of using a feature the target BCM version doesn't support, not a provider bug.

### String vs Bool/Int: A Null Symmetry Subtlety

Removed **string** fields with empty defaults (`""`) are fully version-transparent because `getStringValue` maps both absent keys and empty strings to null:

- `cpuspeedGovernor` on BCM 10: returns `""` → null. On BCM 11: absent → null. **Identical.**

Removed **bool/int** fields are not symmetric — the zero value is distinguishable from null:

- `disableFabricNVME` on BCM 10: returns `false` → `BoolValue(false)`. On BCM 11: absent → `BoolNull()`. **Different.**

When the user doesn't set the field, both are safe (`Unknown` → anything is valid). The distinction only matters if the user explicitly sets the field, which again triggers the expected version-mismatch error.

---

## Complete Field Difference Inventory

### Fields Added in BCM 11 (absent in BCM 10)

| API Field | Provider Attribute | In Schema? | Behavior on BCM 10 |
|---|---|---|---|
| `accessSettings` | `access_settings` | Yes (Optional+Computed) | Safe if unset; absent on BCM 10 → null |
| `authenticationService` | `authentication_service` | Yes (Optional+Computed) | Safe if unset; absent on BCM 10 → null |
| `chassisPosition` | `chassis_position` | Yes (Optional+Computed) | Safe if unset; absent on BCM 10 → null |
| `defaultGatewayMetric` | `default_gateway_metric` | Yes (Optional+Computed) | Safe if unset; errors if explicitly set |
| `partNumber` | `part_number` | Yes (Optional+Computed) | Safe if unset; BCM 10 absent and BCM 11 `""` both map to null |
| `prometheusMetricForwarders` | `prometheus_metric_forwarders` | Yes (Optional+Computed) | Safe if unset; absent on BCM 10 → null |
| `rackPosition` | `rack` (dual-key fix) | Yes | **Fixed** — see [Fix 2](#fix-2-rack--rackposition-rename) |
| `serialNumber` | `serial_number` | Yes (Optional+Computed) | Safe if unset; same null mapping as `partNumber` |

### Fields Removed in BCM 11 (present in BCM 10)

| API Field | Provider Attribute | In Schema? | Behavior on BCM 11 |
|---|---|---|---|
| `cpuspeedGovernor` | `cpuspeed_governor` | Yes (Optional+Computed) | Safe if unset; BCM 10 `""` and BCM 11 absent both map to null |
| `disableFabricNVME` | `disable_fabric_nvme` | Yes (Optional+Computed) | Safe if unset; errors if explicitly set (bool null asymmetry) |
| `indexInsideContainer` | `index_inside_container` | Yes (Optional+Computed) | Safe if unset; errors if explicitly set (int null asymmetry) |
| `rack` | `rack` (dual-key fix) | Yes | **Fixed** — see [Fix 2](#fix-2-rack--rackposition-rename) |
| `tag` | `tag` | Yes (Optional+Computed) | Safe if unset; BCM 10 `""` and BCM 11 absent both map to null |

### Interface-Level Differences

| API Field | BCM 10 | BCM 11 | In Schema? | Impact |
|---|---|---|---|---|
| `bootable` | Absent | `false` | Yes (Optional+Computed) | Safe; both are valid transitions from Unknown |
| `excludeFromDhcpd` | Absent | `false` | Yes (Optional+Computed) | Safe; `getBoolValue` returns null on BCM 10 (absent), `false` on BCM 11 |
| `mac` on BMC | Absent | `"00:00:00:00:00:00"` | Yes | Both map to null; provider explicitly normalizes `"00:00:00:00:00:00"` |
| `cardtype` | `"Ethernet"` | `""` | Yes (Computed-only) | User can't set it; no error possible |

---

## Service Fields Removed in BCM 11

### The Error

Creating devices with services on BCM 11 initially produced:

```
Error: Provider produced inconsistent result after apply

.services[0].belongs_to_role: was cty.True, but now null.
```

### Root Cause

`belongsToRole` is a **removed field** in BCM 11 — it no longer appears in service objects returned by the API. This is the same class of change as `tag`, `cpuspeedGovernor`, and other removed device-level fields.

The error occurred because the HCL explicitly set `belongs_to_role = true`. The provider sent it, BCM 11 ignored it, the read-back returned null, and Terraform flagged the plan/state mismatch. This is the exact failure mode described in the [version-specific fields section](#how-the-provider-handles-version-specific-fields) — explicitly configuring a removed field produces an informative error.

### Resolution

No provider code change needed. The fix is in the HCL: **don't set `belongs_to_role` (or other removed service fields) when targeting BCM 11.** Since `belongs_to_role` is Optional+Computed, omitting it from the service block results in an `Unknown` plan value, and `Unknown → null` is a valid transition. The same applies to any other service fields BCM 11 may have removed (`addFromRole`, `fromGenericRole`, etc.).

This follows the same principle as all other removed fields: the provider's null-safe getter/setter pattern handles them transparently when unset, and correctly errors when explicitly set against an unsupporting version.

---

## Fix 2: `rack` → `rackPosition` Rename

### The Problem

BCM 10 uses `rack`. BCM 11 renamed it to `rackPosition`. The provider only read/wrote `rack`:
- **On BCM 11**: writes `"rack"` → ignored. Reads `"rack"` → absent → null. Rack assignments silently fail.
- **On BCM 10**: works as before.

### The Solution

**Write** — send both keys (each BCM version uses the one it recognizes):
```go
SetJSONField(entity, "rack", plan.Rack)
SetJSONField(entity, "rackPosition", plan.Rack)
```

**Read** — try both keys with fallback:
```go
model.Rack = getJSONValue(data, "rack")
if model.Rack.IsNull() {
    model.Rack = getJSONValue(data, "rackPosition")
}
```

No schema changes needed — the Terraform attribute remains `rack`. Both keys are null-safe (`SetJSONField` skips null values). Fully backward and forward compatible.

---

## BCM 11 Fields Now in Schema

All BCM 11 device-level fields are now modeled in the provider schema as Optional+Computed. On BCM 10 (where they're absent), the null-safe getter/setter pattern keeps them invisible:

| API Field | Provider Attribute | Type |
|---|---|---|
| `accessSettings` | `access_settings` | JSON-encoded object |
| `authenticationService` | `authentication_service` | String (e.g., `"CATEGORY"`) |
| `chassisPosition` | `chassis_position` | JSON-encoded object |
| `prometheusMetricForwarders` | `prometheus_metric_forwarders` | JSON-encoded array |
| `excludeFromDhcpd` (interface) | `exclude_from_dhcpd` | Bool |

---

## Operational Note: `data_node`

`dataNode` exists in both BCM 10 and BCM 11 — it is not a version difference. It is a per-device boolean that marks a node for storage/data workloads (e.g., `cpu-04` has `dataNode: true`, `dgx-04` has `false`).

If the HCL does not set `data_node`, the provider omits it from the request and BCM defaults to `false`. Devices that need `data_node = true` must set it explicitly. For CSV-driven configs, add a `data_node` column and reference it via `each.value.data_node`.

---

## Testing Considerations

- Unit tests pass with both fixes (no behavioral change for BCM 10 code paths)
- Acceptance tests against BCM 11 should verify:
  - Device creation with explicit services (e.g., `nslcd`) — the former blocker
  - Rack assignment round-trips correctly
  - Removed fields (`cpuspeedGovernor`, `disableFabricNVME`, `indexInsideContainer`, `tag`) read as null without errors when not configured
