# BCM Device Resource Reference (`bcm_cmdevice_device`)

## How to Add a Device

The `bcm_cmdevice_device` resource manages a physical or virtual node in a BCM cluster. This document covers the full attribute schema and a practical walkthrough for (re-)adding a device.

### Scenario: Remove cpu-03 from BCM, then re-add it via Terraform

Given the existing imported resource (generated via `terraform plan -generate-config-out`), here is what needs to change to create the device fresh rather than manage an imported one.

#### Fields to remove (Computed by provider/BCM)

These are auto-assigned on create. Hardcoding them from a previous import will either be ignored or cause errors:

| Field | Why remove it |
|-------|--------------|
| `parent_uuid` | BCM assigns this after creation |
| `provisioning_interface` | BCM assigns the interface UUID |
| `cmdaemon_url` | BCM generates based on the device's IP |
| `tag` | BCM auto-generates (e.g., `00000000a000`) |

`uuid` and `id` don't appear in config — they are purely Computed and the provider generates a new UUID internally on create.

#### Fields to remove (unnecessary null/default values)

Lines like `= null` or `= false` that match the default can be removed for cleanliness:

- `default_gateway = null`
- `force = null`
- `kernel_parameters = null`
- `notes = null`
- `roles = []`
- All `= false` boolean flags (unless you want to override a category default)

#### Fields to keep

| Field | Notes |
|-------|-------|
| `hostname = "cpu-03"` | **Required** — the device name |
| `category = "2ef0ee0e-..."` | **Required** — must be valid UUID of an existing category |
| `mac = "B8:59:9F:E4:22:12"` | Important — the physical MAC of the node |
| `management_network = "161d0552-..."` | Optional+Computed, but keep it to get the right network |
| `partition = "11b3d293-..."` | Optional — if omitted, resolved from category |
| `interfaces { ... }` | **At least one required** — define the network interfaces |
| `power_control = "ipmi0"` | Keep if you want IPMI power management |

#### Cleaned-up resource definition

```hcl
resource "bcm_cmdevice_device" "node" {
  hostname           = "cpu-03"
  category           = "2ef0ee0e-3ac3-46e5-9130-daa6417ae43a"
  mac                = "B8:59:9F:E4:22:12"
  management_network = "161d0552-5b1a-44b3-8ba0-abbadd22db73"
  partition          = "11b3d293-2183-4ecc-a4b1-5ffec6e7a466"
  power_control      = "ipmi0"

  interfaces {
    name                = "ipmi0"
    type                = "bmc"
    ip                  = "10.229.10.5"
    network             = "47672401-30e0-4bd2-9c75-9c3bd4ed2cf3"
    on_network_priority = 10
    start_if            = "ALWAYS"
    lanchannel          = 1
  }

  interfaces {
    name                = "BOOTIF"
    type                = "physical"
    ip                  = "10.184.162.102"
    network             = "161d0552-5b1a-44b3-8ba0-abbadd22db73"
    on_network_priority = 60
    start_if            = "ALWAYS"
  }
}
```

The `action "bcm_cmdevice_power" "reset_node"` block can stay — it references `bcm_cmdevice_device.node.uuid` which resolves to the new device's UUID after creation.

---

## Attribute Classification Definitions

In Terraform's Plugin Framework, every attribute has one or more of these flags:

| Flag | What it means for you (the HCL author) |
|------|----------------------------------------|
| **Required** | You **must** provide this value in your `.tf` config. Terraform will error at plan time if it's missing. |
| **Optional** | You **may** provide this value. If omitted, BCM uses a default (often inherited from the category) or the field is left unset. |
| **Computed** | The provider or BCM **determines** this value. You **cannot** set it in config (if Computed-only). If combined with Optional (`Optional+Computed`), you *can* set it, but if you don't, the provider/BCM fills it in and stores it in state. |

### Key combination: Optional+Computed

This is the most common pattern in the device resource. You can provide a value to override the default, or leave it out and BCM/the provider will fill it in. Once set in state, removing the attribute from config does **not** clear it — BCM's patch semantics preserve the existing value. To clear a field, set it to an explicit empty value (e.g., `notes = ""`).

---

## Full Attribute Reference

### Top-Level Attributes

| Attribute | Required | Optional | Computed | Description |
|-----------|:--------:|:--------:|:--------:|-------------|
| `id` | | | **C** | Device identifier (same as UUID) |
| `uuid` | | | **C** | BCM-assigned device UUID |
| `hostname` | **R** | | | Device hostname (RFC 1123 DNS label) |
| `mac` | | O | **C** | MAC address, derived from first interface if not set |
| `category` | **R** | | | Category UUID reference |
| `management_network` | | O | **C** | Management network UUID |
| `partition` | | O | **C** | Partition UUID (uses category default if omitted) |
| `notes` | | O | | Device notes/description |
| `kernel_parameters` | | O | | Kernel boot parameters |
| `boot_loader` | | O | **C** | Boot loader type (SYSLINUX, GRUB) |
| `boot_loader_protocol` | | O | **C** | Boot loader protocol (HTTP, TFTP) |
| `force` | | O | | Force operation (override validation warnings) |
| `power_control` | | O | | Power control method (none, ipmi, pdu, redfish, custom, ipmi0) |
| `default_gateway` | | O | | Default gateway IPv4 address |
| `default_gateway_metric` | | O | **C** | Gateway metric/priority |
| `serial_number` | | O | **C** | Hardware serial number |
| `part_number` | | O | **C** | Hardware part number |
| `creation_time` | | | **C** | Creation timestamp (Unix epoch) |
| `base_type` | | | **C** | Entity base type (always "Device") |
| `child_type` | | | **C** | Device type (HeadNode, ComputeNode, etc.) |
| `cmdaemon_url` | | O | **C** | CMDaemon URL (generated by BCM) |
| `fips` | | O | **C** | FIPS mode |
| `from_template_node` | | O | **C** | Template node UUID reference |
| `index_inside_container` | | O | **C** | Index inside container entity |
| `parent_uuid` | | O | **C** | Parent entity UUID |
| `provisioning_interface` | | O | **C** | Provisioning interface UUID (derived from first bootable interface) |
| `provisioning_transport` | | O | **C** | Provisioning transport (e.g., RSYNCDAEMON) |
| `allow_networking_restart` | | O | **C** | Allow networking restarts |
| `boot_loader_file` | | O | **C** | Boot loader file path override |
| `disksetup` | | O | **C** | Disk partitioning XML |
| `install_boot_record` | | O | **C** | Install a boot record during provisioning |
| `install_mode` | | O | **C** | Installation mode |
| `next_boot_install_mode` | | O | **C** | Install mode for next boot only |
| `node_installer_disk` | | O | **C** | Use disk-based node installer |
| `pxelabel` | | O | **C** | PXE label for network boot |
| `raidconf` | | O | **C** | RAID configuration |
| `version_config_files` | | O | **C** | Version configuration files |
| `cpuspeed_governor` | | O | **C** | CPU frequency scaling governor |
| `io_scheduler` | | O | **C** | I/O scheduler (noop, deadline, cfq) |
| `kernel_output_console` | | O | **C** | Kernel console output device |
| `kernel_version` | | O | **C** | Kernel version override |
| `custom_ping_script` | | O | **C** | Custom ping health-check script |
| `custom_ping_script_argument` | | O | **C** | Argument for custom ping script |
| `custom_power_script` | | O | **C** | Custom power management script |
| `custom_power_script_argument` | | O | **C** | Argument for custom power script |
| `custom_remote_console_script` | | O | **C** | Custom remote console script |
| `custom_remote_console_script_argument` | | O | **C** | Argument for custom remote console script |
| `finalize` | | O | **C** | Finalize script (end of provisioning) |
| `initialize` | | O | **C** | Initialize script (start of provisioning) |
| `exclude_list_full` | | O | **C** | Exclude list for full provisioning |
| `exclude_list_grab` | | O | **C** | Exclude list for image grab |
| `exclude_list_grabnew` | | O | **C** | Exclude list for new image grab |
| `exclude_list_manipulate_script` | | O | **C** | Script for exclude list manipulation |
| `exclude_list_sync` | | O | **C** | Exclude list for sync provisioning |
| `exclude_list_update` | | O | **C** | Exclude list for update provisioning |
| `data_node` | | O | **C** | Is a data node |
| `disable_fabric_nvme` | | O | **C** | Disable NVMe over Fabrics |
| `force_full_environment` | | O | **C** | Force full environment during provisioning |
| `supports_gnss` | | O | **C** | Supports GNSS |
| `template_node` | | O | **C** | Is a template node |
| `tag` | | O | **C** | Device tag |
| `use_exclusively_for` | | O | **C** | Restrict device to specific purpose |
| `userdefined1` | | O | **C** | User-defined field 1 |
| `userdefined2` | | O | **C** | User-defined field 2 |
| `rack` | | O | **C** | Rack UUID reference |
| `software_image_proxy` | | O | **C** | Software image proxy UUID |
| `block_devices_cleared_on_next_boot` | | O | **C** | Block devices to clear on next boot (list) |
| `modules` | | O | **C** | Kernel modules to load (list) |
| `bios_setup` | | O | **C** | BIOS setup configuration (JSON) |
| `bmc_settings` | | O | **C** | BMC settings (JSON) |
| `extra_values` | | O | **C** | BCM extension key-values (JSON) |
| `proxy_settings` | | O | **C** | Proxy settings (JSON) |
| `se_linux_settings` | | O | **C** | SELinux settings (JSON) |
| `time_zone_settings` | | O | **C** | Timezone settings (JSON) |
| `fsexports` | | O | **C** | NFS exports (JSON) |
| `fsmounts` | | O | **C** | Filesystem mounts (JSON) |
| `gpu_settings` | | O | **C** | GPU settings (JSON) |
| `power_distribution_units` | | O | **C** | PDU assignments (JSON) |
| `static_routes` | | O | **C** | Static routes (JSON) |
| `switch_ports` | | O | **C** | Switch port assignments (JSON) |
| `user_defined_resources` | | O | **C** | User-defined resources (JSON) |
| `roles` | | O | **C** | Set of role **names** assigned to device |

### `interfaces` Block (at least 1 required)

| Attribute | Required | Optional | Computed | Description |
|-----------|:--------:|:--------:|:--------:|-------------|
| `name` | **R** | | | Interface name (e.g., eth0, bond0, ipmi0) |
| `type` | **R** | | | Interface type: physical, bond, or bmc |
| `network` | | O | | Network UUID reference |
| `mac` | | O | | MAC address (XX:XX:XX:XX:XX:XX) |
| `ip` | | O | | Static IPv4 address |
| `ipv6_ip` | | O | | Static IPv6 address |
| `dhcp` | | O | **C** | Enable DHCP (default: true) |
| `bootable` | | O | **C** | Enable PXE boot |
| `start_if` | | O | **C** | Startup condition: ALWAYS, NEVER, HOTPLUG |
| `members` | | O | | Bond member interface names (list) |
| `bond_mode` | | O | | Bond mode (802.3ad, active-backup, etc.) |
| `uuid` | | | **C** | BCM-assigned interface UUID |
| `base_type` | | | **C** | Always "NetworkInterface" |
| `child_type` | | | **C** | NetworkPhysicalInterface, NetworkBondInterface, etc. |
| `cardtype` | | | **C** | Hardware card type (Ethernet, InfiniBand, BMC) |
| `bring_up_during_install` | | O | **C** | Bring up during install (e.g., NO) |
| `gateway` | | O | **C** | Per-interface IPv4 gateway |
| `lanchannel` | | O | **C** | IPMI LAN channel index |
| `on_network_priority` | | O | **C** | Priority on its network |
| `vlanid` | | O | **C** | VLAN ID (0 if none) |
| `alternative_hostname` | | O | **C** | Alternative hostname |
| `connected_mode` | | O | **C** | InfiniBand connected mode |
| `ipv6_dhcp` | | O | **C** | Enable IPv6 DHCP |
| `speed` | | O | **C** | Link speed setting |
| `additional_hostnames` | | O | **C** | Additional hostnames (list) |

### `kubelet_role` Block (optional, for Kubernetes membership)

| Attribute | Required | Optional | Computed | Description |
|-----------|:--------:|:--------:|:--------:|-------------|
| `uuid` | | | **C** | BCM-assigned role UUID |
| `kube_cluster` | **R** | | | KubeCluster UUID |
| `control_plane` | | O | **C** | Runs control plane components (default: true) |
| `worker` | | O | **C** | Can schedule workload pods (default: true) |
| `container_runtime_service` | | O | **C** | Runtime service name (default: docker.service) |
| `max_pods` | | O | **C** | Max pods on this node (default: 110) |
| `options` | | O | | Additional kubelet options (JSON) |
| `custom_yaml` | | O | | Custom kubelet configuration YAML |

### `etcd_host_role` Block (optional, for etcd cluster membership)

| Attribute | Required | Optional | Computed | Description |
|-----------|:--------:|:--------:|:--------:|-------------|
| `uuid` | | | **C** | BCM-assigned role UUID |
| `etcd_cluster` | **R** | | | EtcdCluster UUID |
| `member_name` | | O | **C** | Etcd member name (default: $hostname) |
| `spool` | | O | **C** | Data directory (default: /var/lib/etcd) |
| `listen_client_urls` | | O | | Client listen URLs (list) |
| `listen_peer_urls` | | O | | Peer listen URLs (list) |
| `advertise_client_urls` | | O | | Client advertise URLs (list) |
| `advertise_peer_urls` | | O | | Peer advertise URLs (list) |
| `snapshot_count` | | O | **C** | Commits per snapshot (default: 100000) |
| `max_snapshots` | | O | **C** | Max snapshot files (default: 5) |

### `services` Block (optional, OS service configurations)

| Attribute | Required | Optional | Computed | Description |
|-----------|:--------:|:--------:|:--------:|-------------|
| `uuid` | | | **C** | BCM-assigned service UUID |
| `name` | **R** | | | Service name (e.g., nslcd) |
| `base_type` | | O | **C** | BCM base type (typically OSServiceConfig) |
| `add_from_role` | | O | **C** | Added from a role |
| `autostart` | | O | **C** | Auto-restart on failure |
| `belongs_to_role` | | O | **C** | Belongs to a role assignment |
| `monitored` | | O | **C** | CMDaemon monitors service |
| `ref_extra_uuid` | | O | **C** | Extra reference UUID |
| `ref_role_uuid` | | O | **C** | Owning role UUID |
| `run_if` | | O | **C** | When to run (e.g., ALWAYS) |
| `script_timeout` | | O | **C** | Script timeout (-1 = unlimited) |
| `service_type` | | O | **C** | BCM service type enum |
| `sickness_check_interval` | | O | **C** | Sickness check interval (seconds) |
| `sickness_check_script_timeout` | | O | **C** | Sickness check script timeout (seconds) |
| `child_type` | | O | **C** | Service child type |
| `from_generic_role` | | O | **C** | Inherited from generic role |
| `internal` | | O | **C** | Internal (system) service |
| `sickness_check_script` | | O | **C** | Custom sickness check script |

---

## Summary Counts

| Classification | Count | What it means |
|---|---|---|
| **Required** (R) | 2 top-level (`hostname`, `category`) + per-block | You must always provide these |
| **Optional only** (O) | 5 top-level (`notes`, `kernel_parameters`, `force`, `power_control`, `default_gateway`) | You can provide these; if omitted they're simply unset |
| **Optional+Computed** (O+C) | ~60 top-level + many per-block | You can override, or let BCM fill in the default |
| **Computed only** (C) | 5 top-level (`id`, `uuid`, `creation_time`, `base_type`, `child_type`) | Provider/BCM generates these; you cannot set them |

---

## Original Imported Resource (for reference)

This is the full resource as generated by `terraform plan -generate-config-out` for `cpu-03`. It contains every field BCM returns, including computed values that should be removed for a clean create:

```hcl
resource "bcm_cmdevice_device" "node" {
  allow_networking_restart = false
  category                 = "2ef0ee0e-3ac3-46e5-9130-daa6417ae43a"
  cmdaemon_url             = "https://10.184.162.102:8081"
  data_node                = false
  default_gateway          = null
  disable_fabric_nvme      = false
  force                    = null
  force_full_environment   = false
  hostname                 = "cpu-03"
  index_inside_container   = 0
  install_boot_record      = false
  kernel_parameters        = null
  mac                      = "B8:59:9F:E4:22:12"
  management_network       = "161d0552-5b1a-44b3-8ba0-abbadd22db73"
  node_installer_disk      = false
  notes                    = null
  parent_uuid              = "f1ddabcd-818c-4f91-b93a-28b7b381c36b"
  partition                = "11b3d293-2183-4ecc-a4b1-5ffec6e7a466"
  power_control            = "ipmi0"
  provisioning_interface   = "e9046e04-4b55-45fc-b4c8-bd3502af41a7"
  provisioning_transport   = "RSYNCDAEMON"
  roles                    = []
  supports_gnss            = false
  tag                      = "00000000a000"
  template_node            = false
  version_config_files     = false
  interfaces {
    bond_mode               = null
    bring_up_during_install = "NO"
    connected_mode          = false
    dhcp                    = false
    ip                      = "10.229.10.5"
    ipv6_dhcp               = false
    ipv6_ip                 = null
    lanchannel              = 1
    mac                     = null
    members                 = null
    name                    = "ipmi0"
    network                 = "47672401-30e0-4bd2-9c75-9c3bd4ed2cf3"
    on_network_priority     = 10
    start_if                = "ALWAYS"
    type                    = "bmc"
    vlanid                  = 0
  }
  interfaces {
    bond_mode               = null
    bring_up_during_install = "NO"
    connected_mode          = false
    dhcp                    = false
    ip                      = "10.184.162.102"
    ipv6_dhcp               = false
    ipv6_ip                 = null
    mac                     = null
    members                 = null
    name                    = "BOOTIF"
    network                 = "161d0552-5b1a-44b3-8ba0-abbadd22db73"
    on_network_priority     = 60
    start_if                = "ALWAYS"
    type                    = "physical"
  }
  services {
    add_from_role                 = true
    autostart                     = true
    base_type                     = "OSServiceConfig"
    belongs_to_role               = true
    from_generic_role             = false
    internal                      = false
    monitored                     = true
    name                          = "nslcd"
    ref_extra_uuid                = "00000000-0000-0000-0000-000000000000"
    ref_role_uuid                 = "f1ddabcd-818c-4f91-b93a-28b7b381c36b"
    run_if                        = "ALWAYS"
    script_timeout                = -1
    service_type                  = 0
    sickness_check_interval       = 60
    sickness_check_script_timeout = 10
  }
}
```

---

## Questions for BCM Team

### Who generates UUIDs — the client or BCM?

**Current provider behavior:** The Terraform provider generates all UUIDs client-side using Go's `github.com/google/uuid` library (`uuid.New().String()`) and sends them to BCM as part of the entity payload in `addDevice`, `addCategory`, `addNetwork`, etc.

**Empirical findings:** A test script (`sampleRest/test_uuid_requirements_all_types.py`) was written to determine whether BCM requires UUIDs from the client or generates them server-side. The test sends entities **without** a UUID and checks whether BCM rejects them:

| Resource Type | Requires Client-Supplied UUID? |
|---|---|
| Network | Yes — BCM returns validation error on zero UUID |
| Category | Yes — BCM returns validation error on zero UUID |
| Software Image | Tested by script (same pattern expected) |

The test creates entities with no `uuid` field (which defaults to the zero UUID `00000000-0000-0000-0000-000000000000`) and checks BCM's validation response:

```python
# From sampleRest/test_uuid_requirements_all_types.py
entity = {
    "name": name,
    "baseType": entity_type,
    "childType": "",
    "modified": True,
    "to_be_removed": False,
    "revision": "",
    # NOTE: no "uuid" field — tests whether BCM requires it
    **fields
}
```

BCM returns a validation error with severity `ERROR` on the `uuid` field, confirming the client must supply it.

**What this means for the provider:**

- On every `Create`, the provider calls `uuid.New().String()` to generate UUIDs for the device, each interface, and any role objects
- The `provisioningInterface` field is a UUID reference to one of the device's interfaces — since interface UUIDs are generated client-side in the same request, the provider builds the interfaces first, then derives `provisioningInterface` from the first bootable interface's UUID
- On `Import`, the provider reads the existing UUID from BCM and stores it in state

**Questions:**

1. Is client-side UUID generation the intended/supported pattern, or does BCM have an alternate API that returns server-generated UUIDs?
2. Is there any risk of UUID collision? (Statistically negligible with UUIDv4, but worth confirming BCM doesn't do any uniqueness validation beyond the zero-check)
3. Does BCM use these UUIDs as database primary keys, or are they application-layer identifiers with a separate internal ID?
4. If a device is deleted and re-created with the **same** UUID, does BCM handle that correctly, or could stale references cause issues?

**Test script location:** `sampleRest/test_uuid_requirements_all_types.py`

```bash
# Run the test
BCM_ENDPOINT="https://172.21.15.254:8081" \
BCM_USERNAME="root" \
BCM_PASSWORD="Hashicorp123!" \
python3 sampleRest/test_uuid_requirements_all_types.py
```

---

## Appendix: How the Cleaned-Up Resource Definition Was Derived

The cleaned-up resource definition was produced by starting from the full imported resource (generated by `terraform plan -generate-config-out` for the existing `cpu-03` device) and systematically removing everything that isn't needed for a fresh create.

### Step 1: Remove Computed-only fields

These can't be set in config. The import generator included them because it captured every field BCM returned, but they would be ignored or wrong on a fresh create:

- **`parent_uuid = "f1ddabcd-..."`** — This was the *old* device's UUID from the previous BCM record. On a fresh create BCM assigns a new one. Keeping the old value would be wrong.
- **`provisioning_interface = "e9046e04-..."`** — This UUID pointed to the old interface object. The provider generates new interface UUIDs on create and derives `provisioningInterface` from the first bootable interface automatically.
- **`cmdaemon_url = "https://10.184.162.102:8081"`** — BCM computes this from the device's IP after creation.

(`uuid` and `id` are purely Computed and never appear in HCL config — Terraform handles them internally.)

### Step 2: Remove explicit null/default values

These are noise from the import generator. Omitting an Optional field is identical to setting it to `null`:

- **Null values:** `default_gateway = null`, `force = null`, `kernel_parameters = null`, `notes = null`
- **Empty collections:** `roles = []` (empty set is the default)
- **Boolean defaults:** `allow_networking_restart = false`, `data_node = false`, `disable_fabric_nvme = false`, `force_full_environment = false`, `install_boot_record = false`, `node_installer_disk = false`, `supports_gnss = false`, `template_node = false`, `version_config_files = false` — all Optional+Computed; omitting lets BCM use category defaults (typically false)
- **Other defaults:** `provisioning_transport = "RSYNCDAEMON"`, `index_inside_container = 0`, `tag = "00000000a000"` — Optional+Computed values BCM fills in

### Step 3: Clean up interface blocks

Same principle inside the nested blocks — remove null/default noise:

- **Null fields:** `bond_mode = null`, `mac = null`, `members = null`, `ipv6_ip = null`
- **Default booleans:** `connected_mode = false`, `dhcp = false`, `ipv6_dhcp = false`
- **Default integers:** `vlanid = 0`
- **Computed defaults:** `bring_up_during_install = "NO"` — Optional+Computed, BCM fills it in

### Step 4: Remove services block

The `nslcd` service in the import had `belongs_to_role = true` and `add_from_role = true`, meaning it was inherited from a role assignment, not user-configured. BCM re-applies role-based services automatically on create, so declaring it explicitly is unnecessary.

### Step 5: Keep what's essential

What remained after the above removals:

| Field | Why it was kept |
|-------|----------------|
| `hostname = "cpu-03"` | **Required** — the device name |
| `category = "2ef0ee0e-..."` | **Required** — identifies which category this device belongs to |
| `mac = "B8:59:9F:E4:22:12"` | Not required (provider derives from first interface), but identifies the physical hardware explicitly |
| `management_network = "161d0552-..."` | Optional+Computed, but kept to ensure the specific network rather than whatever BCM defaults to |
| `partition = "11b3d293-..."` | Optional (resolved from category if omitted), but kept for explicitness |
| `power_control = "ipmi0"` | Optional, but needed for IPMI power management (the `action` block references it) |
| Two `interfaces` blocks | At least one required; kept with only their meaningful fields (`name`, `type`, `ip`, `network`, `on_network_priority`, `start_if`, `lanchannel` for BMC) |

### Result

The 80-line imported resource became a ~25-line clean definition. The guiding principle: **keep only what you'd need to tell BCM to reconstruct this device from scratch, and let everything else be computed or defaulted.**

### What happens when you run `terraform apply` on the cleaned-up resource

Assuming `cpu-03` does **not** currently exist in BCM (you deleted it and are re-creating):

#### `terraform plan`

Terraform sees `bcm_cmdevice_device.node` in config but not in state. It proposes:

```
bcm_cmdevice_device.node will be created
```

Known values (`hostname`, `category`, `mac`, etc.) are shown. Computed fields are marked `(known after apply)` — things like `uuid`, `id`, `creation_time`, `cmdaemon_url`, `provisioning_interface`. **No state change yet.**

#### `terraform apply` — step by step

| Step | What happens | State written? |
|------|-------------|:--------------:|
| 1. Resolve partition | Provider checks the supplied `partition` UUID (or falls back to category default). Waits for partition to be committed. | No |
| 2. Generate UUIDs | Provider calls `uuid.New()` to mint a fresh UUID for the device and for each interface (ipmi0, BOOTIF). | No |
| 3. Build entity | Assembles the JSON-RPC payload with config values + generated UUIDs. Derives `provisioningInterface` from the first bootable interface's UUID. | No |
| 4. Validate | Calls `cmdevice.validateDevice` against BCM to pre-flight check the entity. If this fails (e.g., duplicate hostname, bad network UUID), Terraform errors out. | No |
| 5. Create | Calls `cmdevice.addDevice` sending the full entity to BCM. BCM persists the device. If this fails, Terraform errors out. | No |
| 6. Wait | Provider sleeps 2 seconds for BCM to process the creation. | No |
| 7. Read back | Calls `cmdevice.getDevice(newUUID)` to get the full device as BCM now stores it, including all computed fields (`cmdaemon_url`, `creation_time`, `child_type`, `base_type`, etc.). | No |
| 8. **Write to state** | Provider maps the BCM response into the Terraform state model and calls `resp.State.Set()`. Terraform serializes this to `terraform.tfstate`. | **Yes** |

**Step 8 is the moment `terraform.tfstate` is updated.** After this, `cpu-03` exists in both BCM and Terraform state. The `uuid` in state is the one the provider generated in step 2.

#### After apply

Running `terraform plan` again reads `cpu-03` from BCM (via `getDevice`), compares to state, compares state to config, and should show:

```
No changes. Your infrastructure matches the configuration.
```

#### Failure scenarios

**Create fails (step 5):** BCM rejects the device. Nothing is written to state. The device doesn't exist in BCM or `.tfstate`. Fix the config and run `apply` again.

**Read-back fails (step 7):** The device **exists in BCM** (step 5 succeeded) but Terraform couldn't read it back. Terraform errors out **without writing state**. Now you have a device in BCM that Terraform doesn't know about. To recover, import it:

```hcl
import {
  to = bcm_cmdevice_device.node
  id = "<the uuid from the error log>"
}
```

The provider logs the UUID at step 5 (`TF_LOG=DEBUG`) so you can find it if the read-back fails.

### Why does the Terraform provider generate UUIDs instead of the database?

Normally the database generates primary keys. In a typical architecture:

- **Postgres:** `id UUID DEFAULT gen_random_uuid()`
- **MySQL:** `id BIGINT AUTO_INCREMENT PRIMARY KEY` or `id CHAR(36) DEFAULT (UUID())` (MySQL 8+)
- **REST APIs:** `POST /api/devices` returns `201 Created` with a server-assigned UUID

BCM's pattern is the opposite — the client supplies the UUID and BCM stores it as-is. Confirmed by the MySQL schema for both `PhysicalNodes` and `NetworkInterfaces`:

```
| uuid | varchar(36) | NO | PRI | | |
```

The **Default** and **Extra** columns are both empty, meaning the database has no auto-generation. If the DB were generating the key, those columns would be populated:

**DB-generated UUID (MySQL 8+):**

```
+-------+-------------+------+-----+-------------------------------+-------------------+
| Field | Type        | Null | Key | Default                       | Extra             |
+-------+-------------+------+-----+-------------------------------+-------------------+
| uuid  | varchar(36) | NO   | PRI | (uuid())                      | DEFAULT_GENERATED |
+-------+-------------+------+-----+-------------------------------+-------------------+
```

**Auto-increment integer (most common pattern):**

```
+-------+------------+------+-----+---------+----------------+
| Field | Type       | Null | Key | Default | Extra          |
+-------+------------+------+-----+---------+----------------+
| id    | bigint     | NO   | PRI | NULL    | auto_increment |
+-------+------------+------+-----+---------+----------------+
```

The telltale sign is the **Extra** column: empty means "the database does nothing — the caller must provide the value." Any generation mechanism shows up there.

**Possible reasons BCM uses client-generated UUIDs:**

- **Offline/distributed creation** — multiple BCM head nodes or clients can create objects without coordinating with a central DB first
- **Replicated/clustered databases** — client-generated UUIDs avoid primary key conflicts between masters in multi-master replication
- **Legacy Java architecture** — BCM has been around for a long time; some older Java middleware layers generate entity UUIDs in the application tier before persisting
- **API designed for their own GUI** — BCM's web frontend likely generates UUIDs client-side in JavaScript before calling the same `addDevice` API; the Terraform provider is just a different client doing the same thing

The tradeoff is that uniqueness and format correctness are pushed onto every client instead of being centralized in the DB layer. UUIDv4 collision probability (~1 in 2^122) makes this safe in practice.
