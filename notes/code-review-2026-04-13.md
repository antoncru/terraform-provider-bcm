# Code Review: terraform-provider-bcm

**Date:** 2026-04-13
**Branch:** device-model-and-power-operation
**Reviewer:** Claude Opus 4.6 (automated)
**Scope:** Full codebase review for correctness, engineering best practices, code quality, and efficiency

---

## Codebase Overview

| Category | Count | Total LOC |
|----------|-------|-----------|
| Provider Core | 5 files | 1,228 |
| Resources | 18 files | 27,222 |
| Data Sources | 20 files | 11,619 |
| Actions | 3 files | 996 |
| Utilities/Helpers | 13 files | 3,277 |
| API Client | 2 files | 1,761 |
| Test Infrastructure | 5 files | 4,630 |
| Scripts | 8 files | 605 |
| **TOTAL** | **75 files** | **~51,300 LOC** |

- Go 1.24 (go.mod), Plugin Framework v1.16.1
- `go build ./...` compiles cleanly, `go vet ./...` passes with no warnings
- 8 direct dependencies, 62 indirect

---

## Critical / Correctness Issues

### 1. Device Read silently removes resource from state on any API error

**File:** `internal/provider/resource_cmdevice_device.go:1710-1717`

```go
body, err := r.Client.CallJSONRPC(ctx, "cmdevice", "getDevice", deviceID)
if err != nil || len(body) == 0 {
    resp.State.RemoveResource(ctx)
    return
}
```

**Problem:** This removes the resource from Terraform state on *any* error (network timeout, auth failure, HTTP 500), not just "not found." A transient network error during `terraform refresh` would silently drop the resource from state, requiring manual reimport.

**Comparison:** Other resources like `resource_cmetcd_cluster.go:352-359` correctly check for specific "not found" error patterns before removing state:

```go
if containsAny(err.Error(), []string{"not found", "does not exist", "404", "null"}) {
    resp.State.RemoveResource(ctx)
    return
}
resp.Diagnostics.AddError("Read Failed", ...)
```

**Fix:** Add "not found" pattern matching before removing state; surface other errors as diagnostics.

**Impact:** High - silent state loss on transient failures.

---

### 2. `time.Sleep` in production CRUD paths ignores context cancellation

**Files:**
- `resource_cmetcd_cluster.go:249, 269, 299` (Create retry loop)
- `resource_cmetcd_cluster.go:489, 509` (Update retry loop)
- `resource_cmdevice_category.go:1289, 1320, 1560, 1591, 1836, 1864`
- `resource_cmuser_user.go:431, 563`

**Problem:** The retry loops in Create/Update use `time.Sleep(sleepDuration)` which blocks the goroutine and ignores context cancellation. If a Terraform operation is cancelled (Ctrl+C), these sleeps continue blocking until they expire.

**Comparison:** The `bcm_client.go:352-357` correctly uses context-aware waiting:

```go
select {
case <-time.After(backoff):
    continue
case <-ctx.Done():
    return nil, fmt.Errorf("call cancelled: %w", ctx.Err())
}
```

**Fix:** Replace all `time.Sleep(duration)` in CRUD methods with the `select`/`time.After`/`ctx.Done` pattern.

**Impact:** Medium - blocks goroutines on cancellation, delays shutdown.

---

### 3. Provider env var handling doesn't distinguish null from empty string

**File:** `internal/provider/provider.go:87-99`

```go
endpoint := data.Endpoint.ValueString()
if endpoint == "" {
    endpoint = os.Getenv("BCM_ENDPOINT")
}
```

**Problem:** `ValueString()` returns `""` for both null and empty-string values. If a user explicitly sets `endpoint = ""` in their provider block, the fallback to env var kicks in silently instead of erroring. The correct pattern checks `IsNull()` before falling back.

**Fix:**
```go
endpoint := ""
if !data.Endpoint.IsNull() {
    endpoint = data.Endpoint.ValueString()
} else {
    endpoint = os.Getenv("BCM_ENDPOINT")
}
```

**Impact:** Medium - subtle correctness issue in config resolution.

---

## Design / Architecture Issues

### 4. CMEtcdCluster and CMKubeCluster don't use `BCMResourceBase`

**Files:**
- `resource_cmetcd_cluster.go:36-38` - `struct { client *BCMClient }`
- `resource_cmkube_cluster.go:39-41` - `struct { client *BCMClient }`

**Problem:** These resources define their own `client *BCMClient` field and duplicate the Configure method boilerplate, while all other resources (device, category, software image, network, user) use the `BCMResourceBase` embedding pattern. Consequences:
- Changes to the base Configure pattern must be applied in 3 places instead of 1
- The field name differs (`r.client` vs `r.Client`) creating confusion when reading across files

**Resources using `BCMResourceBase`:** device, category, software image, network, user
**Resources NOT using it:** etcd cluster, kube cluster

**Fix:** Refactor both to embed `BCMResourceBase` and use `r.ConfigureResource(req, resp)`.

---

### 5. Repeated UUID regex compilation instead of using existing helpers

**File:** `resource_cmdevice_device.go` (19+ occurrences)

The same UUID regex `^[0-9a-f]{8}-[0-9a-f]{4}-...` is compiled inline via `regexp.MustCompile()` on every `Schema()` call. Meanwhile, `schema_helpers.go` already defines:

```go
var UUIDRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-...`)

func UUIDValidator() validator.String {
    return stringvalidator.RegexMatches(UUIDRegex, "must be a valid RFC 4122 UUID...")
}
```

**Affected files:** `resource_cmdevice_device.go`, `resource_cmdevice_category.go` (partial)

**Fix:** Replace all inline UUID regex validators with `UUIDValidator()`.

---

### 6. `minInt` custom helper when Go 1.24 provides builtin `min()`

**File:** `internal/provider/error_messages.go:132-138`

```go
// minInt returns the minimum of two integers (Go 1.21+ has this in stdlib, but for compatibility).
func minInt(a, b int) int {
```

The comment acknowledges Go 1.21+ has the builtin, and `go.mod` specifies `go 1.24`. The test file (`error_messages_test.go:171`) already uses the builtin `min()`, confirming the project targets Go 1.21+.

**Fix:** Remove `minInt`, replace usages with builtin `min()`.

---

### 7. Duplicated validation response parsing in etcd resource

**File:** `resource_cmetcd_cluster.go:189-217` (Create) and `:449-477` (Update)

The pattern of parsing `{"success": bool, "validation": [...]}` responses is copy-pasted between Create and Update. This 30-line block should be extracted to a helper method.

**Fix:** Extract to a `parseValidationResponse(body []byte) error` helper.

---

### 8. Large resource files

| File | Lines |
|------|-------|
| `resource_cmdevice_category.go` | 4,023 |
| `resource_cmdevice_device.go` | 3,080 |
| `resource_cmkube_cluster.go` | 1,322 |
| `resource_cmpart_softwareimage.go` | 972 |

The device and category resources mix schema definition, CRUD operations, entity building, API parsing, interface normalization, and role management in single files. Some extraction has been done (`role_builders.go`, `resource_cmdevice_device_interfaces.go`), but the main files are still large.

**Suggestion:** Consider extracting `buildDeviceAPIEntity*` and `parseDeviceFromAPI` into a `resource_cmdevice_device_builders.go` file.

---

## Code Quality / Efficiency Issues

### 9. Inconsistent error handling style across resources

| Resource | Client access | Error titles | Error helpers used |
|----------|--------------|-------------|-------------------|
| Device | `r.Client` (BCMResourceBase) | Inline strings | No |
| Category | `r.Client` (BCMResourceBase) | `ErrorTitle()` | Yes (partial) |
| Software Image | `r.Client` (BCMResourceBase) | Inline strings | No |
| Network | `r.Client` (BCMResourceBase) | Inline strings | No |
| User | `r.Client` (BCMResourceBase) | Inline strings | No |
| Etcd Cluster | `r.client` (own field) | Inline strings | No |
| Kube Cluster | `r.client` (own field) | Inline strings | No |

The `error_messages.go` helpers (`ErrorTitle`, `BuildAPIError`, `BuildNotFoundError`, `BuildParseError`, `BuildValidationAPIError`) are defined but only partially adopted.

**Suggestion:** Adopt the error helpers consistently across all resources.

---

### 10. `math/rand` usage in bcm_client.go

**File:** `internal/provider/bcm_client.go:14`

Uses `math/rand` (not `crypto/rand`) with the deprecated global `rand.Float64()` function. Since Go 1.20+, the global source is auto-seeded, so this works correctly. However, `math/rand/v2` is the recommended replacement.

**Risk:** Low (correct behavior with Go 1.24), but the deprecated API may trigger linter warnings.

---

### 11. Defensive logging may leak sensitive data at Trace level

**File:** `internal/provider/bcm_client.go:392-395`

```go
tflog.Trace(ctx, fmt.Sprintf("%s response", logPrefix), map[string]interface{}{
    "body": string(body),
})
```

Full response bodies are logged at Trace level. While Trace is typically only enabled during debugging, responses could include sensitive data (user details, credential hashes, internal hostnames).

**Comparison:** The login debug log correctly limits what it logs:
```go
tflog.Debug(ctx, "Attempting BCM login", map[string]interface{}{
    "endpoint": endpoint + "/json",
    "username": username,
})
```

**Suggestion:** Consider using `limitString(string(body), 500)` for Trace logging, or use `tflog.MaskFieldValuesWithFieldKeys` for sensitive fields.

---

### 12. `GetStringListValue` silently drops non-string items

**File:** `internal/provider/utils.go:353-358`

```go
for _, item := range slice {
    if str, ok := item.(string); ok && str != "" {
        elements = append(elements, types.StringValue(str))
    }
}
```

If BCM returns a mixed-type array (e.g., `["a", 123, "b"]`), the non-string items are silently dropped without logging. This could mask API compatibility issues.

**Fix:** Add a `tflog.Warn` when a non-string item is encountered.

---

## Testing Assessment

### Strengths

- **Comprehensive test patterns:** Acceptance tests, unit tests, mock tests, and idempotency tests
- **Drift detection:** Tests use `PreConfig` to modify resources externally via BCM API, then verify Terraform detects the drift
- **CheckDestroy:** Cleanup verification with exponential backoff retry
- **Shared test infrastructure:** `test_helpers.go` provides `createTestBCMClient`, `verifyResourceDeleted`, `getResourceUUIDByName` with proper documentation
- **Field mapping documentation:** `test_helpers.go` documents snake_case to camelCase mappings needed for drift tests

### Weaknesses

- **Hardcoded eventual consistency delays:** `TestEventualConsistencyDelay = 2 * time.Second` used throughout. In CI environments with variable load, this can cause flaky tests.
- **`time.Sleep` in PreConfig functions:** Same context-cancellation issue as production code, but less impactful in tests.

---

## What's Done Well

1. **BCM patch semantics handled correctly** - The `Set*Field` helpers in `utils.go` correctly omit null/unknown fields, and the CLAUDE.md documentation explains the design decision thoroughly.

2. **Cookie-based auth with proper retry logic** - The `BCMClient` has well-implemented exponential backoff with jitter, proper response body cleanup in retry loops (not deferred), and context cancellation support.

3. **Import support** - The Read methods correctly distinguish import vs normal read paths, handling BCM's "CATEGORY" inheritance values appropriately. The `useStateForUnknownUnlessNull` plan modifier solves a real edge case.

4. **Pre-flight validation** - The `ValidateEntity` / `ProcessValidationErrors` pattern catches issues before CRUD operations hit BCM, with proper zero-UUID filtering for creates.

5. **Dependency checking before delete** - `dependency_helpers.go` prevents orphaned references with clear, actionable error messages and resolution options.

6. **Head node safety check** - The power action correctly blocks operations on head nodes to prevent cluster disruption.

7. **Schema helpers reduce boilerplate** - `schema_helpers.go` and `resource_base.go` provide reusable building blocks (though adoption isn't universal).

8. **Comprehensive CLAUDE.md** - Excellent documentation of BCM quirks, field mappings, patch semantics, and design decisions. This is a model for provider documentation.

9. **Multi-layer error parsing** - `parseErrorResponse()` handles HTTP status, JSON error objects, validation arrays, empty arrays, booleans, and null responses — covering BCM's inconsistent API response formats.

10. **Interface compile-time checks** - All resources use `var _ resource.Resource = &Type{}` to verify interface compliance at compile time.

---

## Priority Recommendations

| # | Issue | Effort | Impact | Section | Status (Apr 2026) |
|---|-------|--------|--------|---------|-----|
| 1 | Fix device Read to distinguish "not found" from transient errors | Small | **High** | Critical #1 | **Fixed** — uses `containsAny` pattern |
| 2 | Replace `time.Sleep` with context-aware `select` in CRUD retry loops | Small | **Medium** | Critical #2 | **Fixed** — removed from production CRUD paths |
| 3 | Fix provider env var null vs empty handling | Small | **Medium** | Critical #3 | **Fixed** — `provider.go` uses `IsNull()` check |
| 4 | Migrate etcd/kube resources to `BCMResourceBase` | Medium | **Medium** | Design #4 | **Fixed** — both embed `BCMResourceBase` |
| 5 | Use `UUIDValidator()` instead of inline regex compilation | Small | **Low** | Design #5 | Open — 11 inline compilations in device resource |
| 6 | Remove `minInt`, use builtin `min()` | Trivial | **Low** | Design #6 | Open |
| 7 | Extract duplicated validation response parsing | Small | **Low** | Design #7 | Open |
| 8 | Adopt `error_messages.go` helpers consistently | Medium | **Low** | Quality #9 | Open |
| 9 | Add logging for silently dropped non-string list items | Trivial | **Low** | Quality #12 | Open |

---

## Files Reviewed

### Core (read in full)
- `provider.go`, `bcm_client.go`, `resource_base.go`, `models.go`, `utils.go`
- `error_messages.go`, `schema_helpers.go`, `plan_modifiers.go`
- `dependency_helpers.go`, `role_builders.go`
- `action_cmdevice_power.go`
- `resource_cmetcd_cluster.go` (model resource, read in full)

### Resources (read schema + CRUD methods)
- `resource_cmdevice_device.go` (schema, Read, Update, Delete)
- `resource_cmdevice_category.go` (schema, model)
- `resource_cmpart_softwareimage.go` (schema)
- `resource_cmuser_user.go` (schema)
- `resource_cmkube_cluster.go` (struct, Configure)

### Test infrastructure
- `test_helpers.go` (first 200 lines)

### Automated checks
- `go build ./...` - clean
- `go vet ./...` - clean
- Pattern searches for: `math/rand`, `regexp.MustCompile`, `time.Sleep`, `resp.Body.Close`, `BCMResourceBase`, `minInt`

---

## Appendix A: Fix Applied — Device Read Error Handling (Critical #1)

### Problem in Detail

At `resource_cmdevice_device.go:1710-1716`, the device Read method treated **any** API error as "resource not found" and silently removed it from Terraform state:

```go
// BEFORE (broken)
body, err := r.Client.CallJSONRPC(ctx, "cmdevice", "getDevice", deviceID)
if err != nil || len(body) == 0 {
    tflog.Warn(ctx, "Device not found in BCM, removing from state", ...)
    resp.State.RemoveResource(ctx)
    return
}
```

The condition `err != nil || len(body) == 0` does not distinguish between a genuine "not found" response and a transient failure. Every error type triggers `RemoveResource`:

| Error type | What happened | What *should* happen |
|---|---|---|
| Device genuinely deleted in BCM | Removed from state | Removed from state (correct) |
| Network timeout | Removed from state | Return error diagnostic |
| TLS handshake failure | Removed from state | Return error diagnostic |
| BCM returns HTTP 500 | Removed from state | Return error diagnostic |
| Auth cookie expired | Removed from state | Return error diagnostic |
| DNS resolution failure | Removed from state | Return error diagnostic |

### Why This Matters

When Terraform runs `Read` (during `plan`, `apply`, or `refresh`), it uses the result to reconcile state with reality. If Read removes the resource from state, Terraform concludes the resource no longer exists and will either:

1. **During `terraform plan/apply`:** Propose to **recreate** the resource from scratch — even though the real resource is still there in BCM. This could cause a duplicate or a conflict on the next `addDevice` call.
2. **During `terraform refresh`:** Silently drop the resource from the state file. The user loses track of a resource that still exists. Recovering requires `terraform import`.

This is especially dangerous in environments with flaky network connectivity to the BCM head node, or during BCM maintenance windows where the API may temporarily return 500s.

### How Every Other Resource Handles It

All 6 other resources in the provider handle this correctly with a two-branch pattern. They check for specific "not found" indicators before removing state, and surface all other errors as diagnostics:

**Etcd Cluster** (`resource_cmetcd_cluster.go:351-361`):
```go
if err != nil {
    if containsAny(err.Error(), []string{"not found", "does not exist", "404", "null"}) {
        resp.State.RemoveResource(ctx)  // genuinely gone
        return
    }
    resp.Diagnostics.AddError("Read Failed", ...)  // transient error — surface it
    return
}
```

**Network** (`resource_cmnet_network.go:360-377`) — same pattern with resource-specific error strings.

**Software Image** and **User** — delegate to a `readXxx()` helper that returns a `found bool`, cleanly separating "not found" from "error."

**Kube Cluster** (`resource_cmkube_cluster.go:586-597`) — same `containsAny` pattern.

**Category** (`resource_cmdevice_category.go:1535-1539`) — handles it inside a retry loop.

The device resource was the **only one** that treated all errors as "not found."

### Fix Applied

**File:** `internal/provider/resource_cmdevice_device.go` (Read method)

```go
// AFTER (fixed)
body, err := r.Client.CallJSONRPC(ctx, "cmdevice", "getDevice", deviceID)
if err != nil {
    // Only remove from state if BCM confirms the device no longer exists.
    // Transient errors (network timeout, auth failure, 500) must surface as
    // diagnostics so Terraform retries instead of silently dropping the resource.
    if containsAny(err.Error(), []string{"not found", "does not exist", "404", "null"}) {
        tflog.Warn(ctx, "Device not found in BCM, removing from state", map[string]interface{}{
            "uuid": state.UUID.ValueString(),
        })
        resp.State.RemoveResource(ctx)
        return
    }
    resp.Diagnostics.AddError(
        "Error Reading Device",
        fmt.Sprintf("Could not read device '%s' (UUID: %s): %s",
            state.Hostname.ValueString(), state.UUID.ValueString(), err.Error()),
    )
    return
}
if len(body) == 0 {
    // BCM returned an empty body — treat as deleted
    tflog.Warn(ctx, "Device returned empty response, removing from state", map[string]interface{}{
        "uuid": state.UUID.ValueString(),
    })
    resp.State.RemoveResource(ctx)
    return
}
```

**What changed:**
1. The single `if err != nil || len(body) == 0` branch is now two separate checks.
2. When `err != nil`, the error message is inspected for "not found" indicators using the same `containsAny` helper every other resource uses. Only confirmed "not found" triggers state removal.
3. All other errors (network, auth, server errors) are surfaced as Terraform diagnostics so the user sees the failure and can retry.
4. The `len(body) == 0` check is handled independently — an empty body with no error is a legitimate "deleted" signal from BCM.

**Verification:** `go build ./...` and `go vet ./...` both pass cleanly after the change.
