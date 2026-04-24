# Provider Architecture: Resource Code Patterns

Cross-cutting architectural analysis of how the 7 BCM resource implementations are structured, their consistency, and the design tradeoffs.

**Last updated:** April 2026
**Files analyzed:** `resource_cm*.go` in `internal/provider/`

---

## 1. Resource Style Census

| Resource | Lines | Model Fields | Nested Types | Build Function | Build Returns | Read/Map Pattern | Style |
|---|---|---|---|---|---|---|---|
| `cmnet_network` | 921 | 34 | 0 | Standalone `buildNetworkAPIEntity()` | `(map, error)` | Separate `mapNetworkAPIResponseToState()` | **A** |
| `cmetcd_cluster` | 689 | 8 | 0 | Receiver `r.buildEntity()` | `map` | Separate `r.parseResponseIntoModel()` | **B** |
| `cmkube_cluster` | 1,334 | 22 | 4 lists | Receiver `r.buildEntity()` | `(map, diag)` | Separate `r.parseResponseIntoModel()` | **B+** |
| `cmuser_user` | 891 | 21 | 0 | Receiver `r.buildAPIEntity()` | `map` | Combined `r.readUser()` | **C** |
| `cmpart_softwareimage` | 978 | 21 | 1 list | Receiver `r.buildAPIEntity()` | `map` | Combined `r.readSoftwareImage()` | **C** |
| `cmdevice_device` | 3,174 | ~40 | 4 (slices/set) | Receiver `r.buildDeviceAPIEntityWithExisting()` | `(map, error)` | Combined in Read + `r.parseDeviceFromAPI()` | **C** |
| `cmdevice_category` | 4,085 | ~55 | 12 (Object/List) | Receiver `r.buildAPIEntity()` | `map` | Combined `r.readCategory()` | **C** |

**Result:** 4/7 resources use Style C, 2 use Style B, 1 uses Style A. The codebase is not consistent.

---

## 2. Three Styles Explained

### Style A: Standalone functions, separated concerns (network)

```go
func buildNetworkAPIEntity(ctx, data, uuid) (map[string]interface{}, error)
func mapNetworkAPIResponseToState(ctx, apiData, data)
```

- Build and map are **standalone package functions** -- no receiver, no access to `r.Client`
- Build returns `(map, error)` for proper error propagation
- CRUD methods do the API call, then pass the response to the mapper
- **Advantages:** Unit-testable without mocking a resource struct; pure data transformations are explicit; cleanest separation of concerns
- **Why it works for network:** The network entity is self-contained -- no foreign UUID references, no nested objects, all scalar fields

### Style B: Receiver methods, separated concerns (etcd, kube)

```go
func (r *CMEtcdClusterResource) buildEntity(ctx, data) map[string]interface{}
func (r *CMEtcdClusterResource) parseResponseIntoModel(ctx, data, model)
```

- Build and map are **receiver methods** but still separated
- `Read()` does the API call, then delegates mapping to `parseResponseIntoModel()`
- **Advantages:** Separation of API call from state mapping still exists; mapper is testable if you construct a resource struct
- **Disadvantage vs Style A:** The receiver is unnecessary on both functions (neither uses `r.Client`)

### Style C: Receiver methods, combined read (user, softwareimage, device, category)

```go
func (r *CMUserUserResource) buildAPIEntity(ctx, model, uuid) map[string]interface{}
func (r *CMUserUserResource) readUser(ctx, model, diags)  // does API call + state mapping
```

- Build is a receiver method (but doesn't use `r.Client`)
- `readUser`/`readCategory`/`readSoftwareImage` combines the API call AND state mapping in one function
- **Advantage:** Fewer functions, less indirection for complex entities with many nested objects
- **Disadvantage:** Harder to unit test the mapping logic in isolation; the function is large (500+ lines for category)

---

## 3. Why the Styles Diverge

### Complexity drives the pattern

The network entity has **34 scalar fields, 0 nested objects, 0 foreign UUIDs**. It's a self-contained entity that maps cleanly to a flat struct. A standalone build function works because there's nothing to look up.

The category entity has **55+ fields, 12 nested object/list types, and foreign UUID references** (managementNetwork, softwareImageProxy.parentSoftwareImage). It also has BCM quirks like non-persisted fields (roles, gpu_settings, static_routes) that require plan-value preservation after read. This complexity pushed the implementation toward combined read functions that handle all the edge cases in one place.

### Authorship timeline

All resources were initially written by the same author (Simon Lynch). The network resource was refactored later (the `Set*Field` helpers and drift detection tests came in a later commit), giving it the cleanest pattern. The category and device resources grew incrementally as BCM quirks were discovered, accumulating workarounds on top of the original Style C pattern.

### The receiver is never necessary on build functions

None of the 7 `buildAPIEntity`/`buildEntity` functions access `r.Client` or any other receiver field. They are all pure data transformations from model struct to `map[string]interface{}`. The receiver is stylistic, not functional. Making them standalone functions would:

1. Make it explicit that they're pure data transformations
2. Allow unit testing without constructing a resource struct
3. Signal to future developers that no API call happens here

---

## 4. Naming Inconsistencies

### Build function names

| Resource | Function Name |
|---|---|
| network | `buildNetworkAPIEntity` |
| etcd | `buildEntity` |
| kube | `buildEntity` |
| user | `buildAPIEntity` |
| softwareimage | `buildAPIEntity` |
| device | `buildDeviceAPIEntityWithExisting` |
| category | `buildAPIEntity` |

Three naming conventions: `build<Entity>APIEntity`, `buildEntity`, `buildAPIEntity`.

### Map/read function names

| Resource | Function Name | Pattern |
|---|---|---|
| network | `mapNetworkAPIResponseToState` | Standalone mapper |
| etcd | `parseResponseIntoModel` | Receiver mapper |
| kube | `parseResponseIntoModel` | Receiver mapper |
| user | `readUser` | Combined API+map |
| softwareimage | `readSoftwareImage` | Combined API+map |
| device | `parseDeviceFromAPI` | Receiver mapper (called from Read) |
| category | `readCategory` | Combined API+map |

Four naming conventions: `map*ResponseToState`, `parseResponseIntoModel`, `read<Entity>`, `parse<Entity>FromAPI`.

---

## 5. Recommended Target Architecture

If harmonizing the codebase, the ideal target combines the best of each style:

```go
// Standalone build function (pure data transformation, no receiver needed)
func buildCategoryAPIEntity(ctx context.Context, model *CMDeviceCategoryResourceModel, uuid string) (map[string]interface{}, error) {
    // ... build entity map from model ...
}

// Standalone map function (pure data transformation, no receiver needed)
func mapCategoryAPIResponseToState(ctx context.Context, apiData map[string]interface{}, model *CMDeviceCategoryResourceModel) {
    // ... populate model from API response ...
}

// CRUD methods on receiver (need r.Client for API calls)
func (r *CMDeviceCategoryResource) Create(ctx, req, resp) {
    entity, err := buildCategoryAPIEntity(ctx, &plan, "")
    // ... API call using r.Client ...
    mapCategoryAPIResponseToState(ctx, responseData, &plan)
}
```

### Naming convention

- Build: `build<Entity>APIEntity` (e.g., `buildCategoryAPIEntity`, `buildNetworkAPIEntity`)
- Map: `map<Entity>APIResponseToState` (e.g., `mapCategoryAPIResponseToState`)
- Both standalone, both include the entity name for grep-ability

### Error returns

- Build functions return `(map[string]interface{}, error)` -- CIDR parsing, UUID validation, nested object errors can all fail
- Map functions return nothing (populate model in-place, errors added to diags inline)

### Migration priority

| Resource | Effort | Impact |
|---|---|---|
| `cmetcd_cluster` | Low (rename + drop receiver) | Low (small file) |
| `cmuser_user` | Medium (split readUser) | Medium |
| `cmpart_softwareimage` | Medium (split readSoftwareImage) | Medium |
| `cmkube_cluster` | Medium (rename + drop receiver) | Medium |
| `cmdevice_device` | High (3k lines, complex nested objects) | High |
| `cmdevice_category` | High (4k lines, 12 nested types) | High |
| `cmnet_network` | None (already target pattern) | N/A |

This refactor is not urgent -- the current code works correctly. But it would improve testability, consistency, and onboarding for new contributors.
