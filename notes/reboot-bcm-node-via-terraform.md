# Reset a BCM node with Terraform

This guide explains how to trigger a **graceful reset** of a Bright Cluster Manager (BCM)–managed node using the **HashiCorp BCM Terraform provider**. It is based on the provider implementation in this repository (`bcm_cmdevice_power` action) which calls `cmdevice.powerOperation`.

## Prerequisites

| Requirement | Notes |
|-------------|--------|
| **Terraform** | **1.14 or later** — reset uses Terraform **Actions**, which were introduced in 1.14. |
| **Provider** | `hashi-demo-lab/bcm` (or a local build from this repo via `dev_overrides` / `make install`). |
| **BCM API** | Reachable endpoint; credentials with permission to call `cmdevice` methods. |
| **Target node** | Must **not** be a BCM **head node** — the provider blocks power operations on `HeadNode` to avoid cluster disruption. |
| **Power management** | BMC/IPMI must be configured on the target device (`powerControl != 'none'`). The BCM head node must be able to reach the device's BMC. |

## Terraform 1.14 action syntax (`config`)

Provider-specific arguments for an action **must** be nested in a **`config { }` block**, not placed directly under `action`. If you put `device_id` / `power_action` at the top level, `terraform validate` fails with *Unsupported argument*. See the [action block reference](https://developer.hashicorp.com/terraform/language/block/action).

## What you use (not a resource)

Reset is **not** a managed resource with state. It is an **imperative Terraform Action**:

- **Action type name:** `bcm_cmdevice_power` (provider type prefix + `_cmdevice_power`).
- **Attribute for reset:** `power_action = "reset"`.
- **Target:** `device_id` — BCM device **UUID**. Use `bcm_cmdevice_device.name.uuid` for managed devices.

**Preferred:** set `device_id` from a **`bcm_cmdevice_device`** resource in the same configuration (e.g. `.uuid`) so the reset target stays aligned with what Terraform manages. See [Recommended configuration](#recommended-configuration-device-from-resource) below.

Declaring an `action` block does **not** reset the node by itself. You run the reset by **invoking** that action with the `-invoke` CLI flag.

## How it works (end-to-end)

The following matches the provider code path in `internal/provider/action_cmdevice_power.go` and registration in `internal/provider/provider.go`.

1. **Registration** — The provider exposes actions via `ProviderWithActions`; `NewCMDevicePowerAction` is registered so Terraform knows the `bcm_cmdevice_power` action type.

2. **Configure** — When Terraform prepares the action, the BCM JSON-RPC client from the provider configuration is injected (`ActionWithConfigure`).

3. **Invoke** — When you run `apply` with `-invoke`:
   - Terraform loads the action's **`config`** body (`device_id`, `power_action`, optional `force`, `wait_for_completion`, `timeout`).
   - `power_action` `"reset"` is mapped to BCM operation **`RESET`** via `powerOperationMapping`.
   - The provider calls **`cmdevice.getNode`** with your `device_id` to verify the device exists, read `childType`, and resolve the device UUID. If `getNode` fails, the invoke aborts with an error.
   - If `childType` is **`HeadNode`**, the invoke fails with a safety error and **does not** call `powerOperation`.
   - Otherwise it builds a `PowerOperation` payload and calls **`cmdevice.powerOperation`** via `CallJSONRPC` (structured `args` array).
   - On RPC success, Terraform reports the action as completed. **No resource state is written** for the reset itself.

4. **BCM API payload** — The JSON-RPC call looks like this:

   ```json
   {
     "service": "cmdevice",
     "call": "powerOperation",
     "args": [{
       "baseType": "PowerOperation",
       "devices": ["<device-uuid>"],
       "operation": "RESET",
       "force": true,
       "wait": true
     }]
   }
   ```

## Recommended configuration (device from resource)

When you already manage the node with **`resource "bcm_cmdevice_device" "..."`**, wire the power action to that resource using its **UUID**:

```hcl
resource "bcm_cmdevice_device" "worker" {
  hostname = "my-worker-01"
  # ... category, interfaces, etc.
}

action "bcm_cmdevice_power" "reset_worker" {
  config {
    device_id    = bcm_cmdevice_device.worker.uuid
    power_action = "reset"
  }
}
```

The resource must live in the **same root module** as the `action` block (or be exposed via outputs / remote state). You **do not** need a `variable` for the target when you use `bcm_cmdevice_device.*.uuid`.

## Alternative: input variables

The standalone example under `examples/actions/bcm_cmdevice_power/` uses **`var.device_uuid`** so you can run power actions **without** defining a `bcm_cmdevice_device`. That pattern is fine for demos, but **prefer referencing `bcm_cmdevice_device.*.uuid`** when the node is already in your stack.

```hcl
variable "target_device" {
  type        = string
  description = "Device UUID"
}

action "bcm_cmdevice_power" "reset_worker" {
  config {
    device_id    = var.target_device
    power_action = "reset"
  }
}
```

Set the variable with `TF_VAR_target_device`, `-var`, or a `.tfvars` file as usual.

## Minimal root module layout

Use a single `terraform` block and one `provider "bcm"` configuration per directory. If you merge ideas from `examples/actions/bcm_cmdevice_power/main.tf` and `lifecycle.tf`, deduplicate those blocks—Terraform allows only one `terraform` / provider setup at the root.

## Step-by-step: plan and apply

Replace the action address if your block label differs (`action.<type>.<name>`).

### 1. Environment

```bash
export BCM_ENDPOINT="https://<head-or-api-host>:8081"
export BCM_USERNAME="root"
export BCM_PASSWORD="your-secret"
```

If you use the **variable-based** alternative, also set e.g. `export TF_VAR_target_device="<uuid>"`.

### 2. Initialize

```bash
terraform init
```

This downloads the provider (or uses your local override if configured).

### 3. Preview the action invocation

```bash
terraform plan -invoke=action.bcm_cmdevice_power.reset_worker
```

Terraform plans **only** this action invocation (not a full resource apply). Review the output for the intended target and any diagnostics.

### 4. Execute the reset

```bash
terraform apply -invoke=action.bcm_cmdevice_power.reset_worker
```

Confirm when prompted, or use `-auto-approve` in automation (use with care — this resets a real node).

### 5. How you know it worked

- **Terraform:** Exit code `0` and no error diagnostics from the provider after the invoke step.
- **BCM / node:** The node should begin a graceful reset shortly after a successful `powerOperation` RPC (subject to BCM and BMC behavior). Confirm via BCM UI, `ping`, serial/BMC, or workload recovery as you normally would.
- **State:** Your Terraform state file is **unchanged** by the reset action alone (no new resource instances for the reset).

If something fails, read the provider error text — it often mentions BMC/IPMI configuration (`powerControl`, device UUID, etc.).

**`Unsupported argument` on `device_id` / `power_action`:** Ensure those attributes are inside **`config { }`**, not at the root of the `action` block (this is a Terraform language rule, not a provider bug).

## Action address reference

| Your HCL | Invoke address |
|----------|----------------|
| `action "bcm_cmdevice_power" "reset_worker" { ... }` | `action.bcm_cmdevice_power.reset_worker` |

In a child module, prefix with `module.<name>.` per Terraform's usual addressing rules.

## All power actions

Same action type supports all operations via `cmdevice.powerOperation`:

| `power_action` | BCM Operation | Description |
|----------------|---------------|-------------|
| `power_on`     | `ON`          | Power on device via BMC/IPMI |
| `power_off`    | `OFF`         | Power off device via BMC/IPMI |
| `reset`        | `RESET`       | Graceful device reset via BMC |
| `power_cycle`  | `CYCLE`       | Hard power cycle off/on |

### Additional attributes

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| `force` | bool | `true` | Force the power operation |
| `wait_for_completion` | bool | `false` | Wait for power state change |
| `timeout` | string | `"5m"` | Timeout when `wait_for_completion` is enabled |

## Further reading in this repo

- Example HCL: `examples/actions/bcm_cmdevice_power/main.tf` and `examples/actions/bcm_cmdevice_power/README.md`
- Device + action in one module: `examples/actions/bcm_cmdevice_power/lifecycle.tf`
- Generated action docs: `docs/actions/cmdevice_power.md` (from `make generate`)
- Implementation: `internal/provider/action_cmdevice_power.go`
