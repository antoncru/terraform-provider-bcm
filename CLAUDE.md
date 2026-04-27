# CLAUDE.md

Terraform Provider for Nvidia BCM (Bright Cluster Manager) built on Plugin Framework v1.16.1.

## Quick Reference

```bash
# Build & Install
make install              # Build, lint, install, generate docs

# Testing
make test                 # Unit tests
TF_ACC=1 go test -v -timeout 120m ./internal/provider/ -run TestAccName

# Code Quality
make fmt && make lint     # Format and lint
pre-commit run --all-files

# Example Validation
scripts/test-examples.sh                    # Validate all examples
scripts/test-examples.sh --data-sources     # Data sources only
scripts/test-examples.sh --resources        # Resources only
scripts/test-examples.sh --cleanup-only     # Clean up leftover test resources
scripts/test-examples.sh --verbose          # Verbose output

# Documentation
make generate             # Generate docs (don't edit docs/ manually)
```

**Test Environment:**
```bash
export TF_ACC=1
export BCM_ENDPOINT="https://172.21.15.254:8081"
export BCM_USERNAME="root"
export BCM_PASSWORD="Hashicorp123!"
```

## Project Structure

```
internal/provider/
├── provider.go           # Provider config
├── bcm_client.go         # JSON-RPC client
├── resource_*.go         # Resources (CRUD)
├── data_source_*.go      # Data sources
└── *_test.go             # Tests
examples/                 # Terraform examples
docs/                     # Generated (don't edit)
specs/                    # Speckit specifications
```

## Architecture

### BCM API Pattern

```go
// JSON-RPC call pattern
client.CallJSONRPC(ctx, "cmdevice", "getNodes", args...)

// Data sources: list all, filter client-side
getSoftwareImages() → filter in Go

// Resources: direct lookup with args
getSoftwareImage(name) → single result
```

**Authentication:** Cookie-based (`cm-login-token`), auto-managed.

### Patch-Based API Semantics

BCM uses **patch semantics for scalar fields** and **full replacement for arrays**.

| Payload pattern | BCM behavior |
|-----------------|-------------|
| Key absent from JSON | **Keep existing value** (no change) |
| Key present with value | Set to that value |
| Key present with `""` / `0` / zero UUID | Set to empty/zero (clears the field) |
| Array field (interfaces, roles) | **Full replacement** — send complete array |

**Design decision:** The `Set*Field` helpers in `utils.go` omit null/unknown fields from the entity map. This is correct for both CREATE (BCM uses defaults) and UPDATE (BCM keeps existing values). As a consequence:

- **Removing an optional field from Terraform config does NOT clear it in BCM.** For `Optional+Computed` fields, Terraform uses the prior state value when config is null, so no update is triggered. BCM retains the existing value. This applies to all optional fields including `management_network`.
- **To clear a field**, users must set it to an explicit empty value (e.g., `notes = ""`) if the schema allows it, rather than removing the attribute from config.

**Reference:** `resource_cmetcd_cluster.go` is the model resource — it sends explicit defaults for all nullable fields in the `else` branch, making updates fully deterministic regardless of patch semantics.

### Resource Pattern

1. **Create** - `addResource()`, handle async ops with polling
2. **Read** - Direct lookup with args parameter
3. **Update** - `updateResource()` with entity + UUID (patch: omitted keys preserve existing values)
4. **Delete** - `removeResource()`
5. **Import** - `resource.ImportStatePassthroughID`

**Critical Rules:**
- NEVER propagate `Unknown` values to state
- Preserve plan values for fields BCM resets (e.g., `original_image`)
- Use exponential backoff for eventual consistency

### Provider-Only Fields (Not in BCM Entity Model)

These fields are not part of the BCM entity JSON. The `*_name` fields use the `_name` suffix convention to distinguish them from BCM API fields.

| Field | Resource | Purpose | Notes |
|-------|----------|---------|-------|
| `category_name` | Device | Name→UUID resolver | Resolved via `cmdevice.getCategory(name)` |
| `management_network_name` | Device | Name→UUID resolver | Resolved via `cmnet.getNetwork(name)` |
| `partition_name` | Device | Name→UUID resolver | Resolved via `cmpart.getPartition(name)` |
| `network_name` | Device (interfaces) | Name→UUID resolver | Resolved via `cmnet.getNetwork(name)` |
| `force` | Device, Category, Software Image, User | API call argument | Passed as arg to `add*`/`update*`/`remove*` calls, not part of entity |
| `timeout` | Power Action | Client-side wait limit | Not sent to BCM |

Name fields are resolved to UUIDs at Create/Update time via `resolveNameReferences()` in `name_resolvers.go`. Each `*_name` / UUID pair is mutually exclusive (XOR via `ConflictsWith` + `AtLeastOneOf` validators). The resolved UUID is stored in state; the name is preserved from plan/prior state since BCM doesn't return it.

### Validation

```go
// Pre-flight validation before CREATE/UPDATE
validationErrors, _ := r.client.ValidateEntity(ctx, "CMDevice", "validateCategory", entity, isCreate)
```

| Resource | Service | Method |
|----------|---------|--------|
| Software Images | CMPart | validateSoftwareImage |
| Categories | CMDevice | validateCategory |
| Devices | CMDevice | validateDevice |
| Networks | CMNet | validateNetwork |
| Kubernetes | **cmkube** (lowercase) | validateKubeCluster |

## BCM Quirks

### Non-Persisted Category Fields

BCM API accepts but doesn't store these fields:

| Field | Workaround |
|-------|------------|
| `static_routes` | Preserve plan values |
| `fsexports` | Preserve plan values |
| `roles` | Generate UUIDs locally |
| `gpu_settings` | Preserve plan values |
| `services` | Preserve plan values |

**Reference:** [Issue #73](https://github.com/hashi-demo-lab/terraform-provider-bcm/issues/73)

### Device `managementNetwork` (API-verified)

BCM stores `managementNetwork` on devices exactly as sent — it does NOT auto-inherit from the category.

| Sent to API | BCM Returns on Read |
|-------------|---------------------|
| Valid network UUID | Same UUID (stored) |
| Zero UUID (`00000000-...`) | Zero UUID |
| Field omitted | Zero UUID (default) |
| Non-existent UUID | `BAD_VALUE` validation error |

- Zero UUID and omitted are equivalent — both mean "not set"
- Updates are fully round-trippable (set, clear, change all persist)
- Categories always have a real `managementNetwork` UUID; devices default to zero
- On devices, `management_network` is Optional+Computed — removing from config preserves the existing value (patch semantics)
- On categories, `management_network` is Required
- Provider maps zero UUID to `types.StringNull()` on read to avoid false drift

**Investigation script:** `sampleRest/investigate_management_network_create.py`

### disksetup XML

Root element must be `<diskSetup>` (camelCase). See [Issue #48](https://github.com/hashi-demo-lab/terraform-provider-bcm/issues/48).

### Field Mappings (snake_case → camelCase)

Key mappings for drift detection:
- `kernel_parameters` → `kernelParameters`
- `home_directory` → `homeDirectory`
- `login_shell` → `loginShell`
- `authorized_ssh_keys` → `authorizedSshKeys`

## TDD Workflow

See **AGENTS.md** for complete TDD patterns including:
- RED-GREEN-REFACTOR cycles
- Drift detection tests
- Modern testing patterns (statecheck, plancheck)
- CheckDestroy patterns

### Unit Test Map

| Test file | What it tests |
|-----------|---------------|
| `bcm_client_test.go` | JSON-RPC client logic |
| `utils_test.go` | Helper functions (field setters, UUID handling) |
| `schema_helpers_test.go` | Schema builder utilities |
| `error_messages_test.go` | Error message formatting |
| `dependency_helpers_test.go` | Resource dependency resolution |
| `validate_category_test.go` | Category validation logic |
| `resource_base_test.go` | Shared resource base behavior |
| `action_cmdevice_power_test.go` | Power action schema and mapping |
| `provider_config_test.go` | Provider configuration parsing |
| `provider_optional_config_unit_test.go` | Optional config field handling |
| `data_source_cmdevice_categories_unit_test.go` | Category data source unit logic |
| `*_mock_test.go` | Mock-based resource CRUD (device, kube cluster) |

### Acceptance Test Coverage

| Resource/Data Source | Test file | What's tested |
|---------------------|-----------|---------------|
| Devices | `resource_cmdevice_device_test.go` | CRUD, roles, interfaces, management network, drift, import, validation, disappears |
| Device Idempotency | `resource_cmdevice_device_idempotency_test.go` | Create/update/import/drift idempotency |
| Device Interfaces | `resource_cmdevice_device_interfaces_test.go` | Interface configuration |
| Device Roles | `resource_cmdevice_device_roles_test.go` | Role association/dissociation |
| Device Mock Errors | `resource_cmdevice_device_mock_test.go` | Error paths with mock server |
| Categories | `resource_cmdevice_category_test.go` | CRUD, import, force delete, drift |
| Etcd Clusters | `resource_cmetcd_cluster_test.go` | CRUD, import, drift, validation, disappears |
| Kube Clusters | `resource_cmkube_cluster_test.go` | CRUD, drift, validation, worker nodes, etcd |
| Kube Clusters (aligned) | `resource_cmkube_cluster_aligned_test.go` | API-aligned CRUD, networks, app groups, drift |
| Power Actions | `action_cmdevice_power_acc_test.go` | Power on/off/reset/cycle, force, validation |
| Categories (data) | `data_source_cmdevice_categories_test.go` | List, filter, nested attributes, disk setup |
| Nodes (data) | `data_source_cmdevice_nodes_test.go` | List, filter by type/hostname/multiple |
| Networks (data) | `data_source_cmnet_networks_test.go` | List, filter by name/DHCP, no match |
| Software Images (data) | `data_source_cmpart_softwareimages_test.go` | List, filter, nested modules |
| Partitions (data) | `data_source_cmpart_partitions_test.go` | List, filter, computed fields, case insensitive |
| Entity Info (data) | `data_source_cmpart_entity_info_test.go` | List, filter by type/name/UUID, combined |
| Kube Clusters (data) | `data_source_cmkube_clusters_test.go` | List, filter by name/version/etcd, null fields |
| Users (data) | `data_source_cmuser_users_test.go` | List, filter by username/group/ID, nested |
| Provider Config | `provider_optional_config_test.go` | Insecure skip verify, timeout, combined |

### Speckit Commands

```bash
/speckit.specify   # Create spec
/speckit.clarify   # Ask clarifications
/speckit.plan      # Generate plan
/speckit.tasks     # Generate tasks
/speckit.implement # Execute tasks
```

## Troubleshooting

**Unknown values error:** Resolve to actual value or `null`, never propagate `Unknown`.

**Read strategy:** Data sources list+filter, resources direct lookup with args.

**Auth failures:** Check `cm-login-token` cookie, use `insecure_skip_verify = true`.

## References

- **TDD Patterns:** `./AGENTS.md`
- **BCM API Docs:** `sampleRest/CMDevice_Complete_Documentation.md`
- **Skills:** `terraform-provider-tests`, `terraform-provider-design`
- **Resource Code Patterns:** `notes/provider-architecture.md` (style census, receiver vs standalone, naming inconsistencies)
- **Field Analysis:** `notes/bcm-provider-fields-analysis.md` (device), `notes/bcm-provider-network-analysis.md` (network), `notes/bcm-provider-category-analysis.md` (category)

## Appendix

### Provisioning Interface Derivation

`deriveProvisioningInterface()` in `resource_cmdevice_device.go` selects the correct interface UUID for PXE provisioning when the user doesn't set `provisioning_interface` explicitly.

**Priority chain:**

| Priority | Condition | Rationale |
|----------|-----------|-----------|
| 1 | `name == "BOOTIF"` (case-insensitive) | PXE boot interface convention — most reliable signal |
| 2 | `bootable == true` | Explicit flag, but BCM rarely sets it (usually `null`) |
| 3 | `childType == "NetworkBondInterface"` (case-insensitive) | Bond interfaces are preferred for provisioning over single physical links |
| 4 | First non-BMC interface | BMC (IPMI/iLO/iDRAC) is out-of-band management, cannot PXE |
| 5 | First interface | All-BMC edge case fallback |

**Error handling:** If no interface can be derived and the user didn't set `provisioning_interface`, the provider returns an error before any API call rather than sending an empty string to BCM.

**Background:** BCM's `validateDevice` rejects `provisioningInterface` values that don't reference an interface in the same entity. Prior to this fix, the fallback blindly picked `interfaces[0]`, which could be a BMC interface — causing validation errors or incorrect provisioning configuration.
