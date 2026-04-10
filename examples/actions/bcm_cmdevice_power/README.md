# BCM CMDevice Power Action Example

This example demonstrates how to use the `bcm_cmdevice_power` action to execute power operations on BCM-managed devices via `cmdevice.powerOperation`.

## Requirements

- Terraform 1.14 or later (actions are a new Terraform 1.14 feature; use a nested **`config { }`** block for provider attributes)
- BCM API endpoint and credentials
- Target device UUID (`powerOperation` requires UUIDs — the provider resolves hostnames to UUIDs via `getNode` but UUIDs are preferred)

## Usage

### 1. Set Environment Variables

```bash
export BCM_ENDPOINT="https://172.21.15.254:8081"
export BCM_USERNAME="root"
export BCM_PASSWORD="your-password"
export TF_VAR_device_uuid="2870c0b0-6fda-4026-9b8f-28be4c372fee"
```

### 2. Initialize Terraform

```bash
terraform init
```

### 3. Plan and Validate

```bash
terraform plan
```

### 4. Invoke Action Directly

Actions can be invoked directly using the `-invoke` flag:

```bash
# Power on a device
terraform apply -invoke="action.bcm_cmdevice_power.power_on_by_uuid"

# Reset (graceful reboot) a device
terraform apply -invoke="action.bcm_cmdevice_power.reset_by_uuid"

# Power cycle a device
terraform apply -invoke="action.bcm_cmdevice_power.power_cycle"
```

## Power Actions

All operations use `cmdevice.powerOperation` with a structured `PowerOperation` payload.

| `power_action` | BCM Operation | Description |
|----------------|---------------|-------------|
| `power_on`     | `ON`          | Power on device via BMC/IPMI |
| `power_off`    | `OFF`         | Power off device via BMC/IPMI |
| `reset`        | `RESET`       | Graceful device reset via BMC |
| `power_cycle`  | `CYCLE`       | Hard power cycle off/on |

## Additional Attributes

| Attribute | Type | Default | Description |
|-----------|------|---------|-------------|
| `force` | bool | `true` | Force the power operation |
| `wait_for_completion` | bool | `false` | Wait for power state change to complete |
| `timeout` | string | `"5m"` | Timeout when `wait_for_completion` is enabled |

## Device Identification

The `device_id` attribute accepts a BCM device **UUID**. Use `bcm_cmdevice_device.name.uuid` for managed devices:

```hcl
action "bcm_cmdevice_power" "reset_node" {
  config {
    device_id    = bcm_cmdevice_device.worker.uuid
    power_action = "reset"
  }
}
```

## Safety

The provider blocks power operations on **HeadNode** devices to prevent cluster management disruption.

## Lifecycle Triggers

Actions can be triggered by resource lifecycle events:

```hcl
resource "bcm_cmdevice_device" "worker" {
  hostname = "worker-01"
  # ... other configuration

  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.bcm_cmdevice_power.boot_worker]
    }
  }
}

action "bcm_cmdevice_power" "boot_worker" {
  config {
    device_id    = bcm_cmdevice_device.worker.uuid
    power_action = "power_on"
  }
}
```

## Notes

- Actions do not maintain state — they execute side effects
- Each invocation is independent
- BMC/IPMI must be configured on the target device (`powerControl != 'none'`) for power operations to succeed
- The BCM head node must be able to reach the device's BMC
