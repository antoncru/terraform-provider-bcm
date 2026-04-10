// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                   = &CMDeviceDeviceResource{}
	_ resource.ResourceWithImportState    = &CMDeviceDeviceResource{}
	_ resource.ResourceWithValidateConfig = &CMDeviceDeviceResource{}
)

// CMDeviceDeviceResource defines the resource implementation.
type CMDeviceDeviceResource struct {
	BCMResourceBase
}

// CMDeviceDeviceResourceModel describes the resource data model.
type CMDeviceDeviceResourceModel struct {
	// Identity fields (required/computed)
	ID       types.String `tfsdk:"id"`       // Computed, same as UUID
	UUID     types.String `tfsdk:"uuid"`     // Computed, BCM-assigned
	Hostname types.String `tfsdk:"hostname"` // Required, RFC 1123 validation
	MAC      types.String `tfsdk:"mac"`      // Optional+Computed, derived from first interface

	// References (required)
	Category          types.String `tfsdk:"category"`           // Required, UUID reference
	ManagementNetwork types.String `tfsdk:"management_network"` // Optional+Computed, UUID reference
	Partition         types.String `tfsdk:"partition"`          // Optional, UUID reference (uses default if not set)

	// Optional configuration
	Notes              types.String `tfsdk:"notes"`                // Optional
	KernelParameters   types.String `tfsdk:"kernel_parameters"`    // Optional
	BootLoader         types.String `tfsdk:"boot_loader"`          // Optional
	BootLoaderProtocol types.String `tfsdk:"boot_loader_protocol"` // Optional
	Force              types.Bool   `tfsdk:"force"`                // Optional, default: false

	// Power control configuration
	PowerControl types.String `tfsdk:"power_control"` // Optional, e.g., "none", "ipmi", "ipdu"

	// Network gateway configuration
	DefaultGateway       types.String `tfsdk:"default_gateway"`        // Optional, IP address
	DefaultGatewayMetric types.Int64  `tfsdk:"default_gateway_metric"` // Optional+Computed, gateway metric/priority

	// Hardware identifiers
	SerialNumber types.String `tfsdk:"serial_number"` // Optional+Computed, hardware serial number
	PartNumber   types.String `tfsdk:"part_number"`   // Optional+Computed, hardware part number

	// Computed fields
	CreationTime types.Int64  `tfsdk:"creation_time"` // Computed
	BaseType     types.String `tfsdk:"base_type"`     // Computed, always "Device"
	ChildType    types.String `tfsdk:"child_type"`    // Computed, BCM-determined

	// Interfaces block - at least one interface is required
	Interfaces []DeviceInterfaceModel `tfsdk:"interfaces"`

	// Roles - set of role UUIDs assigned to this device
	// Used for Kubernetes cluster topology (control-plane, worker, etcd, master)
	Roles types.Set `tfsdk:"roles"`

	// Kubernetes roles - nested blocks for cluster membership
	// These define the device's role in Kubernetes/etcd clusters
	KubeletRoles  []KubeletRoleModel  `tfsdk:"kubelet_role"`
	EtcdHostRoles []EtcdHostRoleModel `tfsdk:"etcd_host_role"`

	// Extended CMDevice API fields (fuller parity with getDevice JSON)
	CmdaemonURL           types.String                 `tfsdk:"cmdaemon_url"`
	Fips                  types.String                 `tfsdk:"fips"`
	FromTemplateNode      types.String                 `tfsdk:"from_template_node"`
	IndexInsideContainer  types.Int64                  `tfsdk:"index_inside_container"`
	ParentUUID            types.String                 `tfsdk:"parent_uuid"`
	ProvisioningInterface types.String                 `tfsdk:"provisioning_interface"`
	ProvisioningTransport types.String                 `tfsdk:"provisioning_transport"`
	Services              []DeviceOSServiceConfigModel `tfsdk:"services"`

	// Provisioning & boot configuration
	AllowNetworkingRestart types.Bool   `tfsdk:"allow_networking_restart"`
	BootLoaderFile         types.String `tfsdk:"boot_loader_file"`
	Disksetup              types.String `tfsdk:"disksetup"`
	InstallBootRecord      types.Bool   `tfsdk:"install_boot_record"`
	InstallMode            types.String `tfsdk:"install_mode"`
	NextBootInstallMode    types.String `tfsdk:"next_boot_install_mode"`
	NodeInstallerDisk      types.Bool   `tfsdk:"node_installer_disk"`
	Pxelabel               types.String `tfsdk:"pxelabel"`
	Raidconf               types.String `tfsdk:"raidconf"`
	VersionConfigFiles     types.Bool   `tfsdk:"version_config_files"`

	// Kernel & performance
	CpuspeedGovernor    types.String `tfsdk:"cpuspeed_governor"`
	IoScheduler         types.String `tfsdk:"io_scheduler"`
	KernelOutputConsole types.String `tfsdk:"kernel_output_console"`
	KernelVersion       types.String `tfsdk:"kernel_version"`

	// Script hooks
	CustomPingScript                  types.String `tfsdk:"custom_ping_script"`
	CustomPingScriptArgument          types.String `tfsdk:"custom_ping_script_argument"`
	CustomPowerScript                 types.String `tfsdk:"custom_power_script"`
	CustomPowerScriptArgument         types.String `tfsdk:"custom_power_script_argument"`
	CustomRemoteConsoleScript         types.String `tfsdk:"custom_remote_console_script"`
	CustomRemoteConsoleScriptArgument types.String `tfsdk:"custom_remote_console_script_argument"`
	Finalize                          types.String `tfsdk:"finalize"`
	Initialize                        types.String `tfsdk:"initialize"`

	// Exclude lists (rsync/provisioning filters)
	ExcludeListFull             types.String `tfsdk:"exclude_list_full"`
	ExcludeListGrab             types.String `tfsdk:"exclude_list_grab"`
	ExcludeListGrabnew          types.String `tfsdk:"exclude_list_grabnew"`
	ExcludeListManipulateScript types.String `tfsdk:"exclude_list_manipulate_script"`
	ExcludeListSync             types.String `tfsdk:"exclude_list_sync"`
	ExcludeListUpdate           types.String `tfsdk:"exclude_list_update"`

	// Device flags
	DataNode             types.Bool `tfsdk:"data_node"`
	DisableFabricNVME    types.Bool `tfsdk:"disable_fabric_nvme"`
	ForceFullEnvironment types.Bool `tfsdk:"force_full_environment"`
	SupportsGNSS         types.Bool `tfsdk:"supports_gnss"`
	TemplateNode         types.Bool `tfsdk:"template_node"`

	// Metadata & user-defined
	Tag               types.String `tfsdk:"tag"`
	UseExclusivelyFor types.String `tfsdk:"use_exclusively_for"`
	Userdefined1      types.String `tfsdk:"userdefined1"`
	Userdefined2      types.String `tfsdk:"userdefined2"`

	// References (nullable UUIDs / simple values)
	Rack               types.String `tfsdk:"rack"`
	SoftwareImageProxy types.String `tfsdk:"software_image_proxy"`

	// String lists
	BlockDevicesClearedOnNextBoot types.List `tfsdk:"block_devices_cleared_on_next_boot"`
	Modules                       types.List `tfsdk:"modules"`

	// Complex objects stored as JSON-encoded strings (null when unset)
	BiosSetup        types.String `tfsdk:"bios_setup"`
	BmcSettings      types.String `tfsdk:"bmc_settings"`
	ExtraValues      types.String `tfsdk:"extra_values"`
	ProxySettings    types.String `tfsdk:"proxy_settings"`
	SeLinuxSettings  types.String `tfsdk:"se_linux_settings"`
	TimeZoneSettings types.String `tfsdk:"time_zone_settings"`

	// Complex lists stored as JSON-encoded strings (null when empty)
	Fsexports              types.String `tfsdk:"fsexports"`
	Fsmounts               types.String `tfsdk:"fsmounts"`
	GpuSettings            types.String `tfsdk:"gpu_settings"`
	PowerDistributionUnits types.String `tfsdk:"power_distribution_units"`
	StaticRoutes           types.String `tfsdk:"static_routes"`
	SwitchPorts            types.String `tfsdk:"switch_ports"`
	UserDefinedResources   types.String `tfsdk:"user_defined_resources"`
}

// DeviceOSServiceConfigModel represents one BCM OSServiceConfig entry on a device
// (device.services[] in the CMDevice API). Field names map to Terraform attributes
// (snake_case); build/parse maps to BCM JSON (camelCase / mixed keys).
type DeviceOSServiceConfigModel struct {
	UUID                       types.String `tfsdk:"uuid"`
	Name                       types.String `tfsdk:"name"`
	BaseType                   types.String `tfsdk:"base_type"`
	AddFromRole                types.Bool   `tfsdk:"add_from_role"`
	Autostart                  types.Bool   `tfsdk:"autostart"`
	BelongsToRole              types.Bool   `tfsdk:"belongs_to_role"`
	Monitored                  types.Bool   `tfsdk:"monitored"`
	RefExtraUUID               types.String `tfsdk:"ref_extra_uuid"`
	RefRoleUUID                types.String `tfsdk:"ref_role_uuid"`
	RunIf                      types.String `tfsdk:"run_if"`
	ScriptTimeout              types.Int64  `tfsdk:"script_timeout"`
	ServiceType                types.Int64  `tfsdk:"service_type"`
	SicknessCheckInterval      types.Int64  `tfsdk:"sickness_check_interval"`
	SicknessCheckScriptTimeout types.Int64  `tfsdk:"sickness_check_script_timeout"`
	ChildType                  types.String `tfsdk:"child_type"`
	FromGenericRole            types.Bool   `tfsdk:"from_generic_role"`
	Internal                   types.Bool   `tfsdk:"internal"`
	SicknessCheckScript        types.String `tfsdk:"sickness_check_script"`
}

// NewCMDeviceDeviceResource creates a new resource instance.
func NewCMDeviceDeviceResource() resource.Resource {
	return &CMDeviceDeviceResource{}
}

// Metadata returns the resource type name.
func (r *CMDeviceDeviceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cmdevice_device"
}

// Schema defines the resource schema.
func (r *CMDeviceDeviceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a BCM device (compute node) in the cluster.\n\n" +
			"Devices represent physical or virtual nodes that can be provisioned and managed by BCM. " +
			"Each device requires a unique hostname, category assignment, and at least one network interface.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Device identifier (same as UUID)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"uuid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Device UUID assigned by BCM",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"hostname": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Device hostname (RFC 1123 DNS label: lowercase alphanumeric and hyphens, 1-63 chars)",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`),
						"hostname must be RFC 1123 DNS label (lowercase alphanumeric and hyphens, 1-63 chars)",
					),
				},
			},
			"mac": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Device MAC address, computed from the first interface. Can be set explicitly to override.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`),
						"mac must be six groups of two hexadecimal digits separated by colons (e.g., 00:11:22:33:44:55)",
					),
				},
			},
			"category": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Category UUID reference",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`),
						"must be valid UUID (RFC 4122)",
					),
				},
			},
			"management_network": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Management network UUID reference. Optional — if not specified, the device has no management network set.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`),
						"must be valid UUID (RFC 4122)",
					),
				},
			},
			"partition": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Partition UUID reference (uses category default if not specified)",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`),
						"must be valid UUID (RFC 4122)",
					),
				},
			},
			"notes": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Device notes/description",
			},
			"kernel_parameters": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Kernel boot parameters",
			},
			"boot_loader": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Boot loader type (e.g., SYSLINUX, GRUB) - defaults to category value",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("SYSLINUX", "GRUB"),
				},
			},
			"boot_loader_protocol": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Boot loader protocol (e.g., HTTP, TFTP) - defaults to category value",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("HTTP", "TFTP"),
				},
			},
			"force": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Force operation (override BCM validation warnings)",
			},
			"power_control": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Power control method: 'none', 'ipmi', 'pdu', 'redfish', 'custom', or an IPMI interface " +
					"reference such as 'ipmi0' (BCM returns the BMC interface name when using IPMI).",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^(none|ipmi|pdu|redfish|custom|ipmi[0-9]+)$`),
						"must be none, ipmi, pdu, redfish, custom, or ipmi followed by digits (e.g. ipmi0)",
					),
				},
			},
			"default_gateway": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Default gateway IP address for the device",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`),
						"must be a valid IPv4 address",
					),
				},
			},
			"default_gateway_metric": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Default gateway metric/priority (lower is preferred)",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"serial_number": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Hardware serial number",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"part_number": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Hardware part number",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"creation_time": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Device creation timestamp (Unix epoch)",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"base_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Entity base type (always 'Device')",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"child_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Device type (HeadNode, ComputeNode, PhysicalNode, etc.)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cmdaemon_url": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "CMDaemon URL for this device (returned by BCM).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"fips": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "FIPS mode; BCM may return CATEGORY when inherited from the category.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"from_template_node": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Template node UUID reference (zero UUID means not from a template).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`),
						"must be valid UUID (RFC 4122)",
					),
				},
			},
			"index_inside_container": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Index of this device inside its container entity.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"parent_uuid": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Parent entity UUID when applicable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`),
						"must be valid UUID (RFC 4122)",
					),
				},
			},
			"provisioning_interface": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "UUID of the network interface used for provisioning (PXE/rsync). If unset in config, the provider derives this from the first bootable interface (or first interface).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`),
						"must be valid UUID (RFC 4122)",
					),
				},
			},
			"provisioning_transport": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Provisioning transport (e.g. RSYNCDAEMON) as returned by BCM.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			// ── Provisioning & boot ────────────────────────────────
			"allow_networking_restart": schema.BoolAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Whether networking restarts are allowed on this device.",
			},
			"boot_loader_file": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Boot loader file path override.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"disksetup": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Disk partitioning XML (root element must be <diskSetup>).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"install_boot_record": schema.BoolAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Whether to install a boot record during provisioning.",
			},
			"install_mode": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Installation mode for the device.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"next_boot_install_mode": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Installation mode to use on next boot only.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"node_installer_disk": schema.BoolAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Whether to use disk-based node installer.",
			},
			"pxelabel": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "PXE label for network boot.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"raidconf": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "RAID configuration.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"version_config_files": schema.BoolAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Whether to version configuration files.",
			},

			// ── Kernel & performance ───────────────────────────────
			"cpuspeed_governor": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "CPU frequency scaling governor.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"io_scheduler": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "I/O scheduler (e.g. noop, deadline, cfq).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"kernel_output_console": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Kernel console output device.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"kernel_version": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Kernel version override.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},

			// ── Script hooks ───────────────────────────────────────
			"custom_ping_script": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Custom ping health-check script.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"custom_ping_script_argument": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Argument passed to the custom ping script.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"custom_power_script": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Custom power management script.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"custom_power_script_argument": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Argument passed to the custom power script.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"custom_remote_console_script": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Custom remote console script.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"custom_remote_console_script_argument": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Argument passed to the custom remote console script.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"finalize": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Finalize script run at the end of provisioning.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"initialize": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Initialize script run at the start of provisioning.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},

			// ── Exclude lists ──────────────────────────────────────
			"exclude_list_full": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Exclude list for full provisioning.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"exclude_list_grab": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Exclude list for image grab.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"exclude_list_grabnew": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Exclude list for new image grab.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"exclude_list_manipulate_script": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Script for exclude list manipulation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"exclude_list_sync": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Exclude list for sync provisioning.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"exclude_list_update": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Exclude list for update provisioning.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},

			// ── Device flags ───────────────────────────────────────
			"data_node": schema.BoolAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Whether this device is a data node.",
			},
			"disable_fabric_nvme": schema.BoolAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Whether to disable NVMe over Fabrics.",
			},
			"force_full_environment": schema.BoolAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Whether to force a full environment during provisioning.",
			},
			"supports_gnss": schema.BoolAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Whether this device supports GNSS.",
			},
			"template_node": schema.BoolAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Whether this device is a template node.",
			},

			// ── Metadata & user-defined ────────────────────────────
			"tag": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Device tag.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"use_exclusively_for": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Restrict device usage to a specific purpose.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"userdefined1": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "User-defined field 1.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"userdefined2": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "User-defined field 2.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},

			// ── References ─────────────────────────────────────────
			"rack": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Rack UUID reference (null if not assigned to a rack).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"software_image_proxy": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Software image proxy UUID reference.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},

			// ── String lists ───────────────────────────────────────
			"block_devices_cleared_on_next_boot": schema.ListAttribute{
				Optional: true, Computed: true, ElementType: types.StringType,
				MarkdownDescription: "Block devices to clear on next boot.",
			},
			"modules": schema.ListAttribute{
				Optional: true, Computed: true, ElementType: types.StringType,
				MarkdownDescription: "Kernel modules to load.",
			},

			// ── Complex objects (JSON-encoded) ─────────────────────
			"bios_setup": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "BIOS setup configuration (JSON-encoded when non-null).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"bmc_settings": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "BMC settings configuration (JSON-encoded when non-null).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"extra_values": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "BCM extension key-values (JSON-encoded when non-null).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"proxy_settings": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Proxy settings (JSON-encoded when non-null).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"se_linux_settings": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "SELinux settings (JSON-encoded when non-null).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"time_zone_settings": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Timezone settings (JSON-encoded when non-null).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},

			// ── Complex lists (JSON-encoded) ───────────────────────
			"fsexports": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "NFS exports (JSON-encoded array when non-empty).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"fsmounts": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Filesystem mounts (JSON-encoded array when non-empty).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"gpu_settings": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "GPU settings (JSON-encoded array when non-empty).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"power_distribution_units": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Power distribution units (JSON-encoded array when non-empty).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"static_routes": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Static routes (JSON-encoded array when non-empty).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"switch_ports": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "Switch port assignments (JSON-encoded array when non-empty).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"user_defined_resources": schema.StringAttribute{
				Optional: true, Computed: true,
				MarkdownDescription: "User-defined resources (JSON-encoded array when non-empty).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},

			"roles": schema.SetAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				MarkdownDescription: "Set of role names assigned to this device. Roles define the device's function " +
					"in the cluster (e.g., \"backup\", \"provisioning\", \"boot\"). Use the `bcm_cmdevice_roles` " +
					"data source to discover available roles. **Only role names are accepted** (not UUIDs). " +
					"Role names are case-sensitive.\n\n" +
					"Example usage:\n\n" +
					"```hcl\n" +
					"# Discover available roles\n" +
					"data \"bcm_cmdevice_roles\" \"all\" {}\n\n" +
					"resource \"bcm_cmdevice_device\" \"node\" {\n" +
					"  # ... other configuration ...\n" +
					"  roles = [data.bcm_cmdevice_roles.all.roles[0].name]\n" +
					"}\n" +
					"```",
			},
		},
		Blocks: map[string]schema.Block{
			"interfaces": schema.ListNestedBlock{
				MarkdownDescription: "Network interface configurations for the device. " +
					"At least one interface is required. " +
					"Each interface can be physical, bond, or BMC type.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Interface name (e.g., 'eth0', 'bond0', 'ipmi'). Must be unique within the device.",
							Validators: []validator.String{
								stringvalidator.LengthBetween(1, 63),
								stringvalidator.RegexMatches(
									regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`),
									"must start with letter, contain only alphanumeric, underscore, or hyphen",
								),
							},
						},
						"type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Interface type: 'physical', 'bond', or 'bmc'.",
							Validators: []validator.String{
								stringvalidator.OneOf("physical", "bond", "bmc"),
							},
						},
						"network": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Network UUID reference for interface assignment.",
							Validators: []validator.String{
								stringvalidator.RegexMatches(
									regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`),
									"must be valid UUID (RFC 4122)",
								),
							},
						},
						"mac": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "MAC address (format: 00:11:22:33:44:55). Required for physical interfaces on create.",
							Validators: []validator.String{
								stringvalidator.RegexMatches(
									regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`),
									"must be six groups of two hexadecimal digits separated by colons",
								),
							},
						},
						"ip": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Static IPv4 address.",
							Validators: []validator.String{
								stringvalidator.RegexMatches(
									regexp.MustCompile(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`),
									"must be a valid IPv4 address",
								),
							},
						},
						"ipv6_ip": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Static IPv6 address.",
						},
						"dhcp": schema.BoolAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Enable DHCP for IP assignment. Default: true.",
						},
						"bootable": schema.BoolAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Enable PXE boot capability. Default: false. First bootable interface becomes provisioning interface.",
						},
						"start_if": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Interface startup condition: 'ALWAYS', 'NEVER', 'HOTPLUG'. Default: 'ALWAYS'.",
							Validators: []validator.String{
								stringvalidator.OneOf("ALWAYS", "NEVER", "HOTPLUG"),
							},
						},
						"members": schema.ListAttribute{
							Optional:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Member interface names for bond type. Required when type is 'bond'.",
						},
						"bond_mode": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Bond mode (e.g., '802.3ad', 'active-backup', 'balance-rr'). Only applicable when type is 'bond'.",
							Validators: []validator.String{
								stringvalidator.OneOf(
									"802.3ad",
									"active-backup",
									"balance-rr",
									"balance-xor",
									"broadcast",
									"balance-tlb",
									"balance-alb",
								),
							},
						},
						"uuid": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "BCM-assigned interface UUID.",
							PlanModifiers: []planmodifier.String{
								useStateForUnknownUnlessNull(),
							},
						},
						"base_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Entity base type (always 'NetworkInterface').",
							PlanModifiers: []planmodifier.String{
								useStateForUnknownUnlessNull(),
							},
						},
						"child_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Interface type (NetworkPhysicalInterface, NetworkBondInterface, NetworkBMCInterface).",
							PlanModifiers: []planmodifier.String{
								useStateForUnknownUnlessNull(),
							},
						},
						"cardtype": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Hardware card type (Ethernet, InfiniBand, BMC).",
							PlanModifiers: []planmodifier.String{
								useStateForUnknownUnlessNull(),
							},
						},
						"bring_up_during_install": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Whether to bring the interface up during install (BCM: bringupduringinstall, e.g. NO).",
							PlanModifiers: []planmodifier.String{
								useStateForUnknownUnlessNull(),
							},
						},
						"gateway": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Per-interface IPv4 gateway; BCM may return 0.0.0.0 when unset.",
							PlanModifiers: []planmodifier.String{
								useStateForUnknownUnlessNull(),
							},
							Validators: []validator.String{
								stringvalidator.RegexMatches(
									regexp.MustCompile(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`),
									"must be a valid IPv4 address",
								),
							},
						},
						"lanchannel": schema.Int64Attribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "IPMI LAN channel index when applicable.",
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
						"on_network_priority": schema.Int64Attribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Priority of this interface on its network.",
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
						"vlanid": schema.Int64Attribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "VLAN id for the interface (0 if none).",
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
						"alternative_hostname": schema.StringAttribute{
							Optional: true, Computed: true,
							MarkdownDescription: "Alternative hostname for this interface.",
							PlanModifiers:       []planmodifier.String{useStateForUnknownUnlessNull()},
						},
						"connected_mode": schema.BoolAttribute{
							Optional: true, Computed: true,
							MarkdownDescription: "Whether interface operates in connected mode (InfiniBand).",
						},
						"ipv6_dhcp": schema.BoolAttribute{
							Optional: true, Computed: true,
							MarkdownDescription: "Enable IPv6 DHCP for this interface.",
						},
						"speed": schema.StringAttribute{
							Optional: true, Computed: true,
							MarkdownDescription: "Link speed setting.",
							PlanModifiers:       []planmodifier.String{useStateForUnknownUnlessNull()},
						},
						"additional_hostnames": schema.ListAttribute{
							Optional: true, Computed: true, ElementType: types.StringType,
							MarkdownDescription: "Additional hostnames associated with this interface.",
						},
					},
				},
			},
			"kubelet_role": schema.ListNestedBlock{
				MarkdownDescription: "Kubernetes kubelet role configuration. Defines this device as a member of a KubeCluster. " +
					"Each kubelet_role block associates the device with one KubeCluster as a control plane node, worker node, or both.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"uuid": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "BCM-assigned role UUID.",
						},
						"kube_cluster": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "UUID of the KubeCluster this device belongs to.",
							Validators: []validator.String{
								stringvalidator.RegexMatches(
									regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`),
									"must be valid UUID (RFC 4122)",
								),
							},
						},
						"control_plane": schema.BoolAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Whether this node runs control plane components (API server, controller manager, scheduler). Default: true.",
						},
						"worker": schema.BoolAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Whether this node can schedule workload pods. Default: true.",
						},
						"container_runtime_service": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Container runtime service name (e.g., 'docker.service', 'containerd.service'). Default: 'docker.service'.",
						},
						"max_pods": schema.Int64Attribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Maximum number of pods that can run on this node. Default: 110.",
						},
						"options": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Additional kubelet options as JSON string.",
						},
						"custom_yaml": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Custom kubelet configuration YAML.",
						},
					},
				},
			},
			"etcd_host_role": schema.ListNestedBlock{
				MarkdownDescription: "Etcd host role configuration. Defines this device as a member of an EtcdCluster. " +
					"Each etcd_host_role block associates the device with one EtcdCluster as an etcd node.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"uuid": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "BCM-assigned role UUID.",
						},
						"etcd_cluster": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "UUID of the EtcdCluster this device belongs to.",
							Validators: []validator.String{
								stringvalidator.RegexMatches(
									regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`),
									"must be valid UUID (RFC 4122)",
								),
							},
						},
						"member_name": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Etcd member name. Default: '$hostname' (uses device hostname).",
						},
						"spool": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Etcd data directory path. Default: '/var/lib/etcd'.",
						},
						"listen_client_urls": schema.ListAttribute{
							Optional:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "URLs etcd listens on for client traffic.",
						},
						"listen_peer_urls": schema.ListAttribute{
							Optional:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "URLs etcd listens on for peer traffic.",
						},
						"advertise_client_urls": schema.ListAttribute{
							Optional:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "URLs to advertise to clients for connecting to this member.",
						},
						"advertise_peer_urls": schema.ListAttribute{
							Optional:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "URLs to advertise to peers for connecting to this member.",
						},
						"snapshot_count": schema.Int64Attribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Number of committed transactions to trigger a snapshot. Default: 100000.",
						},
						"max_snapshots": schema.Int64Attribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Maximum number of snapshot files to retain. Default: 5.",
						},
					},
				},
			},
			"services": schema.ListNestedBlock{
				MarkdownDescription: "OS service configurations (BCM OSServiceConfig) attached to this device.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"uuid": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "BCM-assigned service config UUID.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Service name (e.g. nslcd).",
						},
						"base_type": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "BCM base type (typically OSServiceConfig).",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"add_from_role": schema.BoolAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Whether this service entry was added from a role.",
						},
						"autostart": schema.BoolAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Whether CMDaemon should restart a failed service.",
						},
						"belongs_to_role": schema.BoolAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Whether this service belongs to a role assignment.",
						},
						"monitored": schema.BoolAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Whether CMDaemon monitors the service.",
						},
						"ref_extra_uuid": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Extra reference UUID (often zero UUID).",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
							Validators: []validator.String{
								stringvalidator.RegexMatches(
									regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`),
									"must be valid UUID (RFC 4122)",
								),
							},
						},
						"ref_role_uuid": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Owning role UUID reference.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
							Validators: []validator.String{
								stringvalidator.RegexMatches(
									regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`),
									"must be valid UUID (RFC 4122)",
								),
							},
						},
						"run_if": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "When to run the service (e.g. ALWAYS).",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"script_timeout": schema.Int64Attribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Service script timeout (-1 often means unlimited).",
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
						"service_type": schema.Int64Attribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "BCM service type enum value.",
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
						"sickness_check_interval": schema.Int64Attribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Interval between sickness checks (seconds).",
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
						"sickness_check_script_timeout": schema.Int64Attribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Timeout for sickness check script (seconds).",
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
						"child_type": schema.StringAttribute{
							Optional: true, Computed: true,
							MarkdownDescription: "Service child type (e.g. OSServiceConfig).",
							PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
						},
						"from_generic_role": schema.BoolAttribute{
							Optional: true, Computed: true,
							MarkdownDescription: "Whether the service was inherited from a generic role.",
						},
						"internal": schema.BoolAttribute{
							Optional: true, Computed: true,
							MarkdownDescription: "Whether the service is an internal (system) service.",
						},
						"sickness_check_script": schema.StringAttribute{
							Optional: true, Computed: true,
							MarkdownDescription: "Custom script for sickness checking.",
							PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *CMDeviceDeviceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.ConfigureResource(req, resp)
}

// ValidateConfig ensures at least one interface is defined.
// Block-level validators on ListNestedBlock may not fire when the block is absent,
// so we enforce the requirement here.
func (r *CMDeviceDeviceResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config CMDeviceDeviceResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(config.Interfaces) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("interfaces"),
			"At least one interface is required",
			"The interfaces block must contain at least one network interface configuration. "+
				"Each device needs at least one interface for network connectivity.",
		)
	}
}

// Create creates the device resource.
func (r *CMDeviceDeviceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.Client == nil {
		resp.Diagnostics.AddError(
			"Provider Not Configured",
			"The provider has not been configured. Please ensure the provider block is properly configured.",
		)
		return
	}

	var plan CMDeviceDeviceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate UUID for new device (BCM requires UUID before creation)
	newUUID := uuid.New().String()

	tflog.Debug(ctx, "Creating BCM device", map[string]interface{}{
		"hostname": plan.Hostname.ValueString(),
		"mac":      plan.MAC.ValueString(),
		"uuid":     newUUID,
	})

	// Resolve partition UUID (either from plan or from category's default)
	partitionResult, err := r.resolvePartitionFromCategory(ctx, plan.Category.ValueString(), plan.Partition)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Resolving Partition",
			fmt.Sprintf("Could not resolve partition for device '%s': %s\n\n"+
				"Please specify partition explicitly or ensure the category has a valid "+
				"partition or software image proxy configuration.", plan.Hostname.ValueString(), err.Error()),
		)
		return
	}

	partitionUUID := partitionResult.PartitionUUID

	// When using softwareImageProxy, add delay for BCM to process the proxy configuration
	if partitionResult.UsesProxy {
		tflog.Debug(ctx, "Waiting for category proxy to stabilize", nil)
		time.Sleep(5 * time.Second)
	}

	// Only wait for partition commit if NOT using softwareImageProxy
	// When softwareImageProxy is used, the partition is derived from category's proxy configuration
	// and we only need to ensure the software image is committed (not the partition)
	if !partitionResult.UsesProxy {
		tflog.Debug(ctx, "Waiting for partition to be committed", map[string]interface{}{
			"partition_uuid": partitionUUID,
			"uses_proxy":     partitionResult.UsesProxy,
		})
		if err := r.waitForPartitionCommit(ctx, r.Client, partitionUUID); err != nil {
			resp.Diagnostics.AddError(
				"Partition Not Ready",
				fmt.Sprintf("Partition %s is not committed/available after waiting: %s\n\n"+
					"This typically happens when a newly created software image hasn't finished "+
					"its async commit process. The provider will automatically retry, but if this "+
					"persists, the software image may need to be created separately and given time "+
					"to commit before creating devices.", partitionUUID, err.Error()),
			)
			return
		}
	} else {
		// When using softwareImageProxy, no need to wait for partition
		// The partition is the cluster's existing base partition, not a newly created one
		tflog.Debug(ctx, "Category uses softwareImageProxy with base partition", map[string]interface{}{
			"partition_uuid": partitionUUID,
			"uses_proxy":     partitionResult.UsesProxy,
		})
	}

	// Build device entity for BCM API (with generated UUID and resolved partition)
	// Partition field is always included as BCM requires it
	// nil for existingInterfaces since this is a new device
	deviceEntity := r.buildDeviceAPIEntityWithExisting(plan, newUUID, partitionUUID, nil)

	// Lookup and add roles to the entity (requires BCM API access to get full role objects)
	if err := r.lookupAndBuildRolesForEntity(ctx, plan, deviceEntity); err != nil {
		resp.Diagnostics.AddError(
			"Error Looking Up Roles",
			fmt.Sprintf("Could not resolve roles for device '%s': %s\n\n"+
				"Ensure the role names exist in the cluster. You can find available roles using "+
				"the bcm_cmdevice_roles data source.", plan.Hostname.ValueString(), err.Error()),
		)
		return
	}

	// Build Kubernetes roles (kubelet_role, etcd_host_role) and merge with existing roles
	// Pass nil for existingRoles since this is a new device
	if err := r.buildKubernetesRolesForEntity(ctx, plan, deviceEntity, nil); err != nil {
		resp.Diagnostics.AddError(
			"Error Building Kubernetes Roles",
			fmt.Sprintf("Could not build Kubernetes roles for device '%s': %s",
				plan.Hostname.ValueString(), err.Error()),
		)
		return
	}

	// Pre-flight validation: Call validateDevice before CREATE
	validationErrors, err := r.Client.ValidateEntity(ctx, "CMDevice", "validateDevice", deviceEntity, true)
	if err != nil {
		resp.Diagnostics.AddError(
			"Validation API Error",
			fmt.Sprintf("Could not validate device '%s': %s", plan.Hostname.ValueString(), err.Error()),
		)
		return
	}

	// Process validation results - halt if errors found
	if ProcessValidationErrors(validationErrors, &resp.Diagnostics) {
		return
	}

	// Get force parameter value
	forceValue := false
	if !plan.Force.IsNull() {
		forceValue = plan.Force.ValueBool()
	}

	// Call BCM API to create device
	body, err := r.Client.CallJSONRPC(ctx, "cmdevice", "addDevice", deviceEntity, forceValue)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Device",
			fmt.Sprintf("Could not create device '%s': %s", plan.Hostname.ValueString(), err.Error()),
		)
		return
	}

	// Parse validation response
	var validationResp struct {
		Success    bool                     `json:"success"`
		Validation []map[string]interface{} `json:"validation"`
	}

	if err := json.Unmarshal(body, &validationResp); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Create Response",
			fmt.Sprintf("Could not parse device creation response: %s", err.Error()),
		)
		return
	}

	// Check validation response
	if !validationResp.Success {
		// Collect validation errors
		var errorMsgs []string
		for _, v := range validationResp.Validation {
			if field, ok := v["field"].(string); ok {
				if msg, ok := v["message"].(string); ok {
					errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", field, msg))
				}
			}
		}
		resp.Diagnostics.AddError(
			"Error Creating Device",
			fmt.Sprintf("Failed to create device '%s': validation errors: %v", plan.Hostname.ValueString(), errorMsgs),
		)
		return
	}

	tflog.Debug(ctx, "Device created successfully", map[string]interface{}{
		"uuid":     newUUID,
		"hostname": plan.Hostname.ValueString(),
	})

	// Read back the created device to populate computed fields
	// Wait a moment for BCM to process the device creation
	time.Sleep(2 * time.Second)

	readBody, err := r.Client.CallJSONRPC(ctx, "cmdevice", "getDevice", newUUID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Created Device",
			fmt.Sprintf("Device created but could not read back: %s", err.Error()),
		)
		return
	}

	// Parse device data
	var deviceData map[string]interface{}
	if err := json.Unmarshal(readBody, &deviceData); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Device Data",
			fmt.Sprintf("Could not parse device data: %s", err.Error()),
		)
		return
	}

	// Map device data to state
	state := r.parseDeviceFromAPI(deviceData)

	// Preserve write-only fields that BCM doesn't return
	state.Force = plan.Force

	// Handle partition - BCM may not return it, use what was resolved during create
	if state.Partition.IsNull() || state.Partition.ValueString() == "" {
		state.Partition = types.StringValue(partitionUUID)
	}

	// Compute mac from first interface if not explicitly set or BCM returned empty
	if (state.MAC.IsNull() || state.MAC.ValueString() == "") && len(state.Interfaces) > 0 {
		if !state.Interfaces[0].MAC.IsNull() {
			state.MAC = state.Interfaces[0].MAC
		}
	}

	// management_network: parseDeviceFromAPI maps zero UUID to null.
	// If user didn't set it (plan is null) but BCM returns a non-zero value,
	// preserve null to match the plan.
	if plan.ManagementNetwork.IsNull() && !state.ManagementNetwork.IsNull() {
		state.ManagementNetwork = types.StringNull()
	}

	// BCM returns default values for Optional+Computed fields when not explicitly set
	// Preserve null from plan to avoid drift
	if plan.PowerControl.IsNull() && !state.PowerControl.IsNull() {
		state.PowerControl = types.StringNull()
	}

	if plan.DefaultGateway.IsNull() && !state.DefaultGateway.IsNull() {
		state.DefaultGateway = types.StringNull()
	}

	if plan.DefaultGatewayMetric.IsNull() && !state.DefaultGatewayMetric.IsNull() {
		state.DefaultGatewayMetric = types.Int64Null()
	}

	if plan.SerialNumber.IsNull() && !state.SerialNumber.IsNull() {
		state.SerialNumber = types.StringNull()
	}

	if plan.PartNumber.IsNull() && !state.PartNumber.IsNull() {
		state.PartNumber = types.StringNull()
	}

	// Preserve the distinction between omitted roles (null) and explicit empty roles ([]).
	if plan.Roles.IsNull() {
		state.Roles = types.SetNull(types.StringType)
	} else if !plan.Roles.IsUnknown() {
		var plannedRoles []string
		if diags := plan.Roles.ElementsAs(ctx, &plannedRoles, false); !diags.HasError() && len(plannedRoles) == 0 {
			state.Roles = emptyStringSet()
		}
	}

	// Normalize interface order to match plan order (prevents spurious diffs)
	state.Interfaces = normalizeInterfaceOrder(state.Interfaces, plan.Interfaces)

	state.Services = filterServicesToMatchConfig(state.Services, plan.Services)

	// Handle Kubernetes roles - preserve state with computed defaults merged
	// Only include roles if user defined them in plan
	if len(plan.KubeletRoles) == 0 {
		state.KubeletRoles = nil
	} else {
		state.KubeletRoles = mergeKubeletRolesWithDefaults(state.KubeletRoles, plan.KubeletRoles)
	}
	if len(plan.EtcdHostRoles) == 0 {
		state.EtcdHostRoles = nil
	} else {
		state.EtcdHostRoles = mergeEtcdHostRolesWithDefaults(state.EtcdHostRoles, plan.EtcdHostRoles)
	}

	// Set state - use what BCM returns for all other fields
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// waitForPartitionCommit polls BCM to verify a software image partition is committed and available
// This is necessary because BCM software image creation is asynchronous - the API returns success
// before the image is fully committed and ready to be used as a device partition.
func (r *CMDeviceDeviceResource) waitForPartitionCommit(ctx context.Context, client *BCMClient, partitionUUID string) error {
	// Increased from 10 to 20 retries to handle BCM's async partition commit timing
	// BCM can take 60-120 seconds for partition commits in some environments
	maxRetries := 20
	baseDelay := 2 * time.Second
	maxDelay := 10 * time.Second

	totalWait := 0 * time.Second

	for i := 0; i < maxRetries; i++ {
		tflog.Debug(ctx, "Checking partition commit status", map[string]interface{}{
			"partition_uuid":  partitionUUID,
			"attempt":         i + 1,
			"max_retries":     maxRetries,
			"total_wait_secs": totalWait.Seconds(),
		})

		// Try to query the partition - if it's committed, this will succeed
		_, err := client.CallJSONRPC(ctx, "CMPart", "getSoftwareImage", partitionUUID)
		if err == nil {
			tflog.Info(ctx, "Partition is committed and available", map[string]interface{}{
				"partition_uuid":  partitionUUID,
				"attempts":        i + 1,
				"total_wait_secs": totalWait.Seconds(),
			})
			return nil // Partition is accessible
		}

		// If not the last retry, wait with linear backoff (capped at maxDelay)
		if i < maxRetries-1 {
			// Linear backoff: 2s, 4s, 6s, 8s, 10s, 10s, ...
			delay := baseDelay * time.Duration(i+1)
			if delay > maxDelay {
				delay = maxDelay
			}
			totalWait += delay

			tflog.Debug(ctx, "Partition not ready, waiting before retry", map[string]interface{}{
				"partition_uuid":  partitionUUID,
				"delay_seconds":   delay.Seconds(),
				"attempt":         i + 1,
				"total_wait_secs": totalWait.Seconds(),
				"error":           err.Error(),
			})
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("partition not committed after %d retries (waited up to %.0f seconds)",
		maxRetries, totalWait.Seconds())
}

// PartitionResolutionResult contains the result of resolving a partition from a category.
type PartitionResolutionResult struct {
	PartitionUUID string
	UsesProxy     bool
}

func emptyStringSet() types.Set {
	rolesSet, diags := types.SetValue(types.StringType, []attr.Value{})
	if diags.HasError() {
		return types.SetNull(types.StringType)
	}

	return rolesSet
}

// resolvePartitionFromCategory resolves the partition UUID for a device.
// If partitionValue is set, it uses that directly. Otherwise, it queries the category
// to get its default partition or resolve it from softwareImageProxy.
//
// Returns:
//   - PartitionResolutionResult with the partition UUID and whether it uses a proxy
//   - error if the resolution fails
func (r *CMDeviceDeviceResource) resolvePartitionFromCategory(ctx context.Context, categoryName string, partitionValue types.String) (PartitionResolutionResult, error) {
	result := PartitionResolutionResult{}

	// If partition is explicitly provided, use it directly
	if !partitionValue.IsNull() && !partitionValue.IsUnknown() {
		result.PartitionUUID = partitionValue.ValueString()
		return result, nil
	}

	categoryLookupName := categoryName
	if matchedCategoryName, err := r.lookupCategoryNameByUUID(ctx, categoryName); err == nil && matchedCategoryName != "" {
		categoryLookupName = matchedCategoryName
	}

	// Query category to get its default partition
	categoryBody, err := r.Client.CallJSONRPC(ctx, "cmdevice", "getCategory", categoryLookupName)
	if err != nil {
		return result, fmt.Errorf("could not query category '%s' to get default partition: %w", categoryLookupName, err)
	}

	var categoryData map[string]interface{}
	if err := json.Unmarshal(categoryBody, &categoryData); err != nil {
		return result, fmt.Errorf("could not parse category data: %w", err)
	}

	// Try direct partition field first
	if partition, ok := categoryData["partition"].(string); ok && partition != "" {
		result.PartitionUUID = partition
		tflog.Debug(ctx, "Using category's direct partition", map[string]interface{}{
			"partition": result.PartitionUUID,
		})
		return result, nil
	}

	// Check if category uses softwareImageProxy instead
	proxyData, ok := categoryData["softwareImageProxy"].(map[string]interface{})
	if !ok || proxyData == nil {
		return result, fmt.Errorf("category does not have a default partition or software image proxy")
	}

	parentImage, ok := proxyData["parentSoftwareImage"].(string)
	if !ok || parentImage == "" {
		return result, fmt.Errorf("category has softwareImageProxy but no parentSoftwareImage")
	}

	result.UsesProxy = true
	tflog.Debug(ctx, "Category uses softwareImageProxy - will use cluster's base partition", map[string]interface{}{
		"parent_software_image": parentImage,
	})

	// Query for the base partition
	partitionsBody, err := r.Client.CallJSONRPC(ctx, "CMPart", "getPartitions")
	if err != nil {
		return result, fmt.Errorf("could not query partitions: %w", err)
	}

	var partitions []map[string]interface{}
	if err := json.Unmarshal(partitionsBody, &partitions); err != nil {
		return result, fmt.Errorf("could not parse partitions response: %w", err)
	}

	// Find the base partition
	for _, part := range partitions {
		if name, ok := part["name"].(string); ok && name == "base" {
			if uuid, ok := part["uuid"].(string); ok && uuid != "" {
				result.PartitionUUID = uuid
				tflog.Debug(ctx, "Found base partition for softwareImageProxy", map[string]interface{}{
					"partition_uuid": result.PartitionUUID,
				})
				return result, nil
			}
		}
	}

	return result, fmt.Errorf("category uses softwareImageProxy but no 'base' partition found in cluster")
}

func (r *CMDeviceDeviceResource) lookupCategoryNameByUUID(ctx context.Context, categoryUUID string) (string, error) {
	body, err := r.Client.CallJSONRPC(ctx, "cmdevice", "getCategories")
	if err != nil {
		return "", err
	}

	var categories []map[string]interface{}
	if err := json.Unmarshal(body, &categories); err != nil {
		return "", err
	}

	for _, category := range categories {
		uuid, ok := category["uuid"].(string)
		if !ok || uuid != categoryUUID {
			continue
		}

		name, _ := category["name"].(string)
		return name, nil
	}

	return "", nil
}

// Read reads the device resource.
func (r *CMDeviceDeviceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.Client == nil {
		resp.Diagnostics.AddError(
			"Provider Not Configured",
			"The provider has not been configured. Please ensure the provider block is properly configured.",
		)
		return
	}

	var state CMDeviceDeviceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use ID if UUID is not set (happens during import)
	deviceID := state.UUID.ValueString()
	if deviceID == "" {
		deviceID = state.ID.ValueString()
	}

	tflog.Debug(ctx, "Reading BCM device", map[string]interface{}{
		"id": deviceID,
	})

	// Call BCM API to get device (efficient direct lookup)
	body, err := r.Client.CallJSONRPC(ctx, "cmdevice", "getDevice", deviceID)
	if err != nil || len(body) == 0 {
		tflog.Warn(ctx, "Device not found in BCM, removing from state", map[string]interface{}{
			"uuid": state.UUID.ValueString(),
		})
		resp.State.RemoveResource(ctx)
		return
	}

	// Parse device data
	var deviceData map[string]interface{}
	if err := json.Unmarshal(body, &deviceData); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Device Data",
			fmt.Sprintf("Could not parse device data: %s", err.Error()),
		)
		return
	}

	// Map device data to state
	newState := r.parseDeviceFromAPI(deviceData)

	// Preserve write-only fields that BCM doesn't return
	newState.Force = state.Force

	// Compute mac from first interface if BCM returned empty
	if (newState.MAC.IsNull() || newState.MAC.ValueString() == "") && len(newState.Interfaces) > 0 {
		if !newState.Interfaces[0].MAC.IsNull() {
			newState.MAC = newState.Interfaces[0].MAC
		}
	}

	// CRITICAL FIX FOR IMPORT: Handle optional+computed fields differently for import vs normal Read
	// During import (state is empty), we should NOT preserve null values - instead use what BCM returns
	// During normal Read (state has values), preserve null if user didn't explicitly set the field

	// Detect import: during import via ImportStatePassthroughID, only "id" is set.
	// Required fields like Hostname and Category will be null. In normal Read after
	// Create/Update, these are always populated.
	isImport := state.Hostname.IsNull() || state.Category.IsNull()

	if !isImport {
		// Normal Read path: Preserve null values from state to avoid drift

		// management_network: BCM stores exactly what was sent. If state is null
		// (user didn't set it) but BCM returns a non-zero value, preserve null
		// to avoid false drift. If state has a real UUID and BCM returns zero UUID
		// (mapped to null by parseDeviceFromAPI), that IS drift — detected automatically.
		if state.ManagementNetwork.IsNull() && !newState.ManagementNetwork.IsNull() {
			newState.ManagementNetwork = types.StringNull()
		}

		// BCM returns "CATEGORY" for boot_loader/boot_loader_protocol when inheriting from category
		// Preserve null/empty plan values to avoid drift
		if state.BootLoader.IsNull() && strings.EqualFold(newState.BootLoader.ValueString(), "CATEGORY") {
			newState.BootLoader = types.StringNull()
		}
		if state.BootLoaderProtocol.IsNull() && strings.EqualFold(newState.BootLoaderProtocol.ValueString(), "CATEGORY") {
			newState.BootLoaderProtocol = types.StringNull()
		}

		if state.Fips.IsNull() && !newState.Fips.IsNull() && strings.EqualFold(newState.Fips.ValueString(), "CATEGORY") {
			newState.Fips = types.StringNull()
		}

		// BCM may add partition if not explicitly set - preserve null from plan to avoid drift
		if state.Partition.IsNull() && !newState.Partition.IsNull() {
			newState.Partition = types.StringNull()
		}

		// BCM returns default values for Optional+Computed fields when not explicitly set
		// Preserve null from plan to avoid drift
		if state.PowerControl.IsNull() && !newState.PowerControl.IsNull() {
			newState.PowerControl = types.StringNull()
		}

		if state.DefaultGateway.IsNull() && !newState.DefaultGateway.IsNull() {
			newState.DefaultGateway = types.StringNull()
		}

		if state.DefaultGatewayMetric.IsNull() && !newState.DefaultGatewayMetric.IsNull() {
			newState.DefaultGatewayMetric = types.Int64Null()
		}

		if state.SerialNumber.IsNull() && !newState.SerialNumber.IsNull() {
			newState.SerialNumber = types.StringNull()
		}

		if state.PartNumber.IsNull() && !newState.PartNumber.IsNull() {
			newState.PartNumber = types.StringNull()
		}
	} else {
		// Import path: Use all values from BCM, don't set to null
		// BCM returns "CATEGORY" for fields inheriting from category - this is valid during import
		tflog.Debug(ctx, "Import detected - using all BCM values", map[string]interface{}{
			"management_network":     newState.ManagementNetwork.ValueString(),
			"boot_loader":            newState.BootLoader.ValueString(),
			"boot_loader_protocol":   newState.BootLoaderProtocol.ValueString(),
			"partition":              newState.Partition.ValueString(),
			"default_gateway_metric": newState.DefaultGatewayMetric.ValueInt64(),
		})
	}

	// Normalize interface order to match state (prevents spurious diffs)
	// During import, keep interfaces from BCM as-is; during normal read, merge with state
	if !isImport && len(state.Interfaces) > 0 && len(newState.Interfaces) > 0 {
		newState.Interfaces = normalizeInterfaceOrder(newState.Interfaces, state.Interfaces)
	}

	if !isImport {
		newState.Services = filterServicesToMatchConfig(newState.Services, state.Services)
	}

	// Preserve the distinction between omitted roles (null) and explicit empty roles ([]).
	if !isImport {
		if state.Roles.IsNull() {
			newState.Roles = types.SetNull(types.StringType)
		} else {
			var existingRoles []string
			if diags := state.Roles.ElementsAs(ctx, &existingRoles, false); !diags.HasError() && len(existingRoles) == 0 {
				newState.Roles = emptyStringSet()
			}
		}
	}

	// Handle Kubernetes roles based on mode
	// During import, use all roles from BCM; during normal read, preserve state-based handling
	if isImport {
		// Import: Keep Kubernetes roles from BCM API response (already parsed in newState)
		tflog.Debug(ctx, "Import path - keeping Kubernetes roles from BCM API", map[string]interface{}{
			"kubelet_role_count":   len(newState.KubeletRoles),
			"etcd_host_role_count": len(newState.EtcdHostRoles),
		})
	} else {
		// Normal read: Only include roles if user defined them in state
		if len(state.KubeletRoles) == 0 {
			newState.KubeletRoles = nil
		} else {
			newState.KubeletRoles = mergeKubeletRolesWithDefaults(newState.KubeletRoles, state.KubeletRoles)
		}
		if len(state.EtcdHostRoles) == 0 {
			newState.EtcdHostRoles = nil
		} else {
			newState.EtcdHostRoles = mergeEtcdHostRolesWithDefaults(newState.EtcdHostRoles, state.EtcdHostRoles)
		}
	}

	// Set state - use what BCM returns (with preserved fields)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// Update updates the device resource.
func (r *CMDeviceDeviceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.Client == nil {
		resp.Diagnostics.AddError(
			"Provider Not Configured",
			"The provider has not been configured. Please ensure the provider block is properly configured.",
		)
		return
	}

	var plan CMDeviceDeviceResourceModel
	var state CMDeviceDeviceResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating BCM device", map[string]interface{}{
		"uuid":     state.UUID.ValueString(),
		"hostname": plan.Hostname.ValueString(),
	})

	// Resolve partition UUID (either from plan or from category's default)
	partitionResult, err := r.resolvePartitionFromCategory(ctx, plan.Category.ValueString(), plan.Partition)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Resolving Partition",
			fmt.Sprintf("Could not resolve partition for device '%s': %s\n\n"+
				"Please specify partition explicitly or ensure the category has a valid "+
				"partition or software image proxy configuration.", plan.Hostname.ValueString(), err.Error()),
		)
		return
	}

	partitionUUID := partitionResult.PartitionUUID

	// Build device entity for BCM API (include UUID for update)
	// Partition field is always included as BCM requires it
	// Pass existing interfaces from state to preserve UUIDs
	deviceEntity := r.buildDeviceAPIEntityWithExisting(plan, state.UUID.ValueString(), partitionUUID, state.Interfaces)

	// Lookup and add roles to the entity (requires BCM API access to get full role objects)
	if err := r.lookupAndBuildRolesForEntity(ctx, plan, deviceEntity); err != nil {
		resp.Diagnostics.AddError(
			"Error Looking Up Roles",
			fmt.Sprintf("Could not resolve roles for device '%s': %s\n\n"+
				"Ensure the role names exist in the cluster. You can find available roles using "+
				"the bcm_cmdevice_roles data source.", plan.Hostname.ValueString(), err.Error()),
		)
		return
	}

	// Get existing roles from current device state for merging
	// This preserves non-Kubernetes roles that aren't being explicitly managed
	var existingKubeRoles []map[string]any
	if len(state.KubeletRoles) > 0 || len(state.EtcdHostRoles) > 0 {
		// Read current device to get existing roles
		existingBody, existingErr := r.Client.CallJSONRPC(ctx, "cmdevice", "getDevice", state.UUID.ValueString())
		if existingErr == nil {
			var existingData map[string]interface{}
			if json.Unmarshal(existingBody, &existingData) == nil {
				if rolesData, ok := existingData["roles"].([]interface{}); ok {
					for _, r := range rolesData {
						if roleMap, ok := r.(map[string]interface{}); ok {
							existingKubeRoles = append(existingKubeRoles, roleMap)
						}
					}
				}
			}
		}
	}

	// Build Kubernetes roles (kubelet_role, etcd_host_role) and merge with existing roles
	if err := r.buildKubernetesRolesForEntity(ctx, plan, deviceEntity, existingKubeRoles); err != nil {
		resp.Diagnostics.AddError(
			"Error Building Kubernetes Roles",
			fmt.Sprintf("Could not build Kubernetes roles for device '%s': %s",
				plan.Hostname.ValueString(), err.Error()),
		)
		return
	}

	// Pre-flight validation: Call validateDevice before UPDATE
	validationErrors, err := r.Client.ValidateEntity(ctx, "CMDevice", "validateDevice", deviceEntity, false)
	if err != nil {
		resp.Diagnostics.AddError(
			"Validation API Error",
			fmt.Sprintf("Could not validate device '%s': %s", plan.Hostname.ValueString(), err.Error()),
		)
		return
	}

	// Process validation results - halt if errors found
	if ProcessValidationErrors(validationErrors, &resp.Diagnostics) {
		return
	}

	// Get force parameter value
	forceValue := false
	if !plan.Force.IsNull() {
		forceValue = plan.Force.ValueBool()
	}

	// Call BCM API to update device
	_, err = r.Client.CallJSONRPC(ctx, "cmdevice", "updateDevice", deviceEntity, forceValue)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Device",
			fmt.Sprintf("Could not update device '%s': %s", plan.Hostname.ValueString(), err.Error()),
		)
		return
	}

	tflog.Debug(ctx, "Device updated successfully", map[string]interface{}{
		"uuid":     state.UUID.ValueString(),
		"hostname": plan.Hostname.ValueString(),
	})

	// Read back the updated device
	readBody, err := r.Client.CallJSONRPC(ctx, "cmdevice", "getDevice", state.UUID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Updated Device",
			fmt.Sprintf("Device updated but could not read back: %s", err.Error()),
		)
		return
	}

	// Parse device data
	var deviceData map[string]interface{}
	if err := json.Unmarshal(readBody, &deviceData); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Device Data",
			fmt.Sprintf("Could not parse device data: %s", err.Error()),
		)
		return
	}

	// Map device data to state
	newState := r.parseDeviceFromAPI(deviceData)

	// Preserve plan values for fields not persisted by BCM or modified during updates
	newState.Force = plan.Force

	// Compute mac from first interface if not explicitly set or BCM returned empty
	if (newState.MAC.IsNull() || newState.MAC.ValueString() == "") && len(newState.Interfaces) > 0 {
		if !newState.Interfaces[0].MAC.IsNull() {
			newState.MAC = newState.Interfaces[0].MAC
		}
	}

	// management_network: parseDeviceFromAPI already maps zero UUID to null.
	// If user removed it from config (plan is null) but BCM still returns a value,
	// preserve null to match the plan — the next read will reconcile.
	if plan.ManagementNetwork.IsNull() && !newState.ManagementNetwork.IsNull() {
		newState.ManagementNetwork = types.StringNull()
	}

	// Set partition to the resolved value (either from plan or resolved from category)
	if partitionUUID != "" {
		newState.Partition = types.StringValue(partitionUUID)
	} else if !plan.Partition.IsNull() {
		newState.Partition = plan.Partition
	} else {
		newState.Partition = types.StringNull()
	}

	// Preserve null values for optional fields that BCM populates from category
	if plan.BootLoader.IsNull() {
		newState.BootLoader = types.StringNull() // Keep null if not explicitly set
	}
	if plan.BootLoaderProtocol.IsNull() {
		newState.BootLoaderProtocol = types.StringNull() // Keep null if not explicitly set
	}

	// BCM returns default values for Optional+Computed fields when not explicitly set
	// Preserve null from plan to avoid drift
	if plan.PowerControl.IsNull() && !newState.PowerControl.IsNull() {
		newState.PowerControl = types.StringNull()
	}

	if plan.DefaultGateway.IsNull() && !newState.DefaultGateway.IsNull() {
		newState.DefaultGateway = types.StringNull()
	}

	if plan.DefaultGatewayMetric.IsNull() && !newState.DefaultGatewayMetric.IsNull() {
		newState.DefaultGatewayMetric = types.Int64Null()
	}

	if plan.SerialNumber.IsNull() && !newState.SerialNumber.IsNull() {
		newState.SerialNumber = types.StringNull()
	}

	if plan.PartNumber.IsNull() && !newState.PartNumber.IsNull() {
		newState.PartNumber = types.StringNull()
	}

	// Preserve the distinction between omitted roles (null) and explicit empty roles ([]).
	if plan.Roles.IsNull() {
		newState.Roles = types.SetNull(types.StringType)
	} else if !plan.Roles.IsUnknown() {
		var plannedRoles []string
		if diags := plan.Roles.ElementsAs(ctx, &plannedRoles, false); !diags.HasError() && len(plannedRoles) == 0 {
			newState.Roles = emptyStringSet()
		}
	}

	// Normalize interface order to match plan order (prevents spurious diffs)
	newState.Interfaces = normalizeInterfaceOrder(newState.Interfaces, plan.Interfaces)

	newState.Services = filterServicesToMatchConfig(newState.Services, plan.Services)

	// Handle Kubernetes roles - preserve state with computed defaults merged
	// Only include roles if user defined them in plan
	if len(plan.KubeletRoles) == 0 {
		newState.KubeletRoles = nil
	} else {
		newState.KubeletRoles = mergeKubeletRolesWithDefaults(newState.KubeletRoles, plan.KubeletRoles)
	}
	if len(plan.EtcdHostRoles) == 0 {
		newState.EtcdHostRoles = nil
	} else {
		newState.EtcdHostRoles = mergeEtcdHostRolesWithDefaults(newState.EtcdHostRoles, plan.EtcdHostRoles)
	}

	// Set state
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// Delete deletes the device resource.
func (r *CMDeviceDeviceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.Client == nil {
		resp.Diagnostics.AddError(
			"Provider Not Configured",
			"The provider has not been configured. Please ensure the provider block is properly configured.",
		)
		return
	}

	var state CMDeviceDeviceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting BCM device", map[string]interface{}{
		"uuid":     state.UUID.ValueString(),
		"hostname": state.Hostname.ValueString(),
	})

	// Clear roles before deletion to prevent orphaned NodeRoles entries.
	// BCM does not cascade-delete role assignments when a device is removed,
	// leaving references to non-existent devices that crash the daemon on startup.
	// See: https://github.com/hashi-demo-lab/terraform-provider-bcm/issues/125
	deviceBody, readErr := r.Client.CallJSONRPC(ctx, "cmdevice", "getDevice", state.UUID.ValueString())
	if readErr == nil && len(deviceBody) > 0 {
		var deviceData map[string]interface{}
		if json.Unmarshal(deviceBody, &deviceData) == nil {
			if rolesData, ok := deviceData["roles"].([]interface{}); ok && len(rolesData) > 0 {
				tflog.Debug(ctx, "Clearing roles before device deletion", map[string]interface{}{
					"uuid":       state.UUID.ValueString(),
					"role_count": len(rolesData),
				})
				deviceData["roles"] = []interface{}{}
				deviceData["modified"] = true
				if _, updateErr := r.Client.CallJSONRPC(ctx, "cmdevice", "updateDevice", deviceData, true); updateErr != nil {
					tflog.Warn(ctx, "Failed to clear roles before deletion, proceeding anyway", map[string]interface{}{
						"uuid":  state.UUID.ValueString(),
						"error": updateErr.Error(),
					})
				}
			}
		}
	}

	// Get force parameter value
	forceValue := false
	if !state.Force.IsNull() {
		forceValue = state.Force.ValueBool()
	}

	// Call BCM API to delete device
	_, err := r.Client.CallJSONRPC(ctx, "cmdevice", "removeDevice", state.UUID.ValueString(), forceValue)
	if err != nil {
		// Check if device was already deleted externally (idempotent delete)
		errStr := err.Error()
		if containsAny(errStr, []string{"No such device", "not found", "does not exist"}) {
			tflog.Info(ctx, "Device already deleted (idempotent)", map[string]interface{}{
				"hostname": state.Hostname.ValueString(),
				"uuid":     state.UUID.ValueString(),
			})
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting Device",
			fmt.Sprintf("Could not delete device '%s' (UUID: %s): %s",
				state.Hostname.ValueString(), state.UUID.ValueString(), err.Error()),
		)
		return
	}

	tflog.Debug(ctx, "Device deleted successfully", map[string]interface{}{
		"uuid":     state.UUID.ValueString(),
		"hostname": state.Hostname.ValueString(),
	})
}

// ImportState imports an existing device by UUID.
func (r *CMDeviceDeviceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)

	tflog.Debug(ctx, "Importing BCM device", map[string]interface{}{
		"id": req.ID,
	})
}

// buildDeviceAPIEntityWithExisting constructs BCM API entity, preserving interface UUIDs from existing state.
func (r *CMDeviceDeviceResource) buildDeviceAPIEntityWithExisting(plan CMDeviceDeviceResourceModel, deviceUUID string, partitionUUID string, existingInterfaces []DeviceInterfaceModel) map[string]interface{} {
	// Build interfaces from the interfaces block
	interfaces := buildInterfacesAPIArray(plan.Interfaces, existingInterfaces)

	// Derive provisioningInterface from built interfaces (which have correct UUIDs).
	// Find the first bootable interface; fall back to interfaces[0] if none is bootable.
	provisioningInterfaceUUID := ""
	for _, iface := range interfaces {
		if ifaceMap, ok := iface.(map[string]interface{}); ok {
			if bootable, ok := ifaceMap["bootable"].(bool); ok && bootable {
				if ifaceUUID, ok := ifaceMap["uuid"].(string); ok {
					provisioningInterfaceUUID = ifaceUUID
					break
				}
			}
		}
	}
	if provisioningInterfaceUUID == "" && len(interfaces) > 0 {
		if firstIface, ok := interfaces[0].(map[string]interface{}); ok {
			if ifaceUUID, ok := firstIface["uuid"].(string); ok {
				provisioningInterfaceUUID = ifaceUUID
			}
		}
	}

	// Device MAC: explicit top-level mac if set, otherwise first interface MAC
	deviceMAC := ""
	if len(plan.Interfaces) > 0 && !plan.Interfaces[0].MAC.IsNull() {
		deviceMAC = plan.Interfaces[0].MAC.ValueString()
	}
	if !plan.MAC.IsNull() && !plan.MAC.IsUnknown() {
		deviceMAC = plan.MAC.ValueString()
	}

	provisioningIfaceFinal := provisioningInterfaceUUID
	if !plan.ProvisioningInterface.IsNull() && !plan.ProvisioningInterface.IsUnknown() && plan.ProvisioningInterface.ValueString() != "" {
		provisioningIfaceFinal = plan.ProvisioningInterface.ValueString()
	}

	entity := map[string]interface{}{
		"baseType":              "Device",
		"childType":             "PhysicalNode",
		"mac":                   deviceMAC,
		"modified":              true,
		"to_be_removed":         false,
		"revision":              "",
		"uuid":                  deviceUUID,
		"provisioningInterface": provisioningIfaceFinal,
		"interfaces":            interfaces,
	}

	// Required fields - set via helpers for defensive null/unknown guards
	SetStringField(entity, "hostname", plan.Hostname)
	SetStringField(entity, "category", plan.Category)

	// Only include managementNetwork when explicitly set by the user.
	// BCM treats omitted and zero UUID identically (both default to zero UUID on read).
	SetStringField(entity, "managementNetwork", plan.ManagementNetwork)

	// Always include partition field as BCM requires it
	// When usesProxy is true, partitionUUID contains the software image UUID from parentSoftwareImage
	// BCM will use this in combination with the category's softwareImageProxy configuration
	entity["partition"] = partitionUUID

	// Add optional fields if present
	SetStringField(entity, "notes", plan.Notes)
	SetStringField(entity, "kernelParameters", plan.KernelParameters)
	SetStringField(entity, "bootLoader", plan.BootLoader)
	SetStringField(entity, "bootLoaderProtocol", plan.BootLoaderProtocol)

	// Power control configuration
	SetStringField(entity, "powerControl", plan.PowerControl)

	// Network gateway configuration
	SetStringField(entity, "defaultGateway", plan.DefaultGateway)
	SetInt64Field(entity, "defaultGatewayMetric", plan.DefaultGatewayMetric)

	// Hardware identifiers
	SetStringField(entity, "serialNumber", plan.SerialNumber)
	SetStringField(entity, "partNumber", plan.PartNumber)

	SetStringField(entity, "cmdaemonUrl", plan.CmdaemonURL)
	SetStringField(entity, "fips", plan.Fips)
	SetStringField(entity, "fromTemplateNode", plan.FromTemplateNode)
	SetInt64Field(entity, "indexInsideContainer", plan.IndexInsideContainer)
	SetStringField(entity, "parent_uuid", plan.ParentUUID)
	SetStringField(entity, "provisioningTransport", plan.ProvisioningTransport)

	// Provisioning & boot
	SetBoolField(entity, "allowNetworkingRestart", plan.AllowNetworkingRestart)
	SetStringField(entity, "bootLoaderFile", plan.BootLoaderFile)
	SetStringField(entity, "disksetup", plan.Disksetup)
	SetBoolField(entity, "installBootRecord", plan.InstallBootRecord)
	SetStringField(entity, "installMode", plan.InstallMode)
	SetStringField(entity, "nextBootInstallMode", plan.NextBootInstallMode)
	SetBoolField(entity, "nodeInstallerDisk", plan.NodeInstallerDisk)
	SetStringField(entity, "pxelabel", plan.Pxelabel)
	SetStringField(entity, "raidconf", plan.Raidconf)
	SetBoolField(entity, "versionConfigFiles", plan.VersionConfigFiles)

	// Kernel & performance
	SetStringField(entity, "cpuspeedGovernor", plan.CpuspeedGovernor)
	SetStringField(entity, "ioScheduler", plan.IoScheduler)
	SetStringField(entity, "kernelOutputConsole", plan.KernelOutputConsole)
	SetStringField(entity, "kernelVersion", plan.KernelVersion)

	// Script hooks
	SetStringField(entity, "customPingScript", plan.CustomPingScript)
	SetStringField(entity, "customPingScriptArgument", plan.CustomPingScriptArgument)
	SetStringField(entity, "customPowerScript", plan.CustomPowerScript)
	SetStringField(entity, "customPowerScriptArgument", plan.CustomPowerScriptArgument)
	SetStringField(entity, "customRemoteConsoleScript", plan.CustomRemoteConsoleScript)
	SetStringField(entity, "customRemoteConsoleScriptArgument", plan.CustomRemoteConsoleScriptArgument)
	SetStringField(entity, "finalize", plan.Finalize)
	SetStringField(entity, "initialize", plan.Initialize)

	// Exclude lists
	SetStringField(entity, "excludeListFull", plan.ExcludeListFull)
	SetStringField(entity, "excludeListGrab", plan.ExcludeListGrab)
	SetStringField(entity, "excludeListGrabnew", plan.ExcludeListGrabnew)
	SetStringField(entity, "excludeListManipulateScript", plan.ExcludeListManipulateScript)
	SetStringField(entity, "excludeListSync", plan.ExcludeListSync)
	SetStringField(entity, "excludeListUpdate", plan.ExcludeListUpdate)

	// Device flags
	SetBoolField(entity, "dataNode", plan.DataNode)
	SetBoolField(entity, "disableFabricNVME", plan.DisableFabricNVME)
	SetBoolField(entity, "forceFullEnvironment", plan.ForceFullEnvironment)
	SetBoolField(entity, "supportsGNSS", plan.SupportsGNSS)
	SetBoolField(entity, "templateNode", plan.TemplateNode)

	// Metadata & user-defined
	SetStringField(entity, "tag", plan.Tag)
	SetStringField(entity, "useExclusivelyFor", plan.UseExclusivelyFor)
	SetStringField(entity, "userdefined1", plan.Userdefined1)
	SetStringField(entity, "userdefined2", plan.Userdefined2)

	// References
	SetJSONField(entity, "rack", plan.Rack)
	SetJSONField(entity, "softwareImageProxy", plan.SoftwareImageProxy)

	// String lists
	if !plan.BlockDevicesClearedOnNextBoot.IsNull() && !plan.BlockDevicesClearedOnNextBoot.IsUnknown() {
		var vals []string
		plan.BlockDevicesClearedOnNextBoot.ElementsAs(context.Background(), &vals, false)
		entity["blockDevicesClearedOnNextBoot"] = vals
	}
	if !plan.Modules.IsNull() && !plan.Modules.IsUnknown() {
		var vals []string
		plan.Modules.ElementsAs(context.Background(), &vals, false)
		entity["modules"] = vals
	}

	// Complex objects
	SetJSONField(entity, "biosSetup", plan.BiosSetup)
	SetJSONField(entity, "bmcSettings", plan.BmcSettings)
	SetJSONField(entity, "extra_values", plan.ExtraValues)
	SetJSONField(entity, "proxySettings", plan.ProxySettings)
	SetJSONField(entity, "seLinuxSettings", plan.SeLinuxSettings)
	SetJSONField(entity, "timeZoneSettings", plan.TimeZoneSettings)

	// Complex lists
	SetJSONField(entity, "fsexports", plan.Fsexports)
	SetJSONField(entity, "fsmounts", plan.Fsmounts)
	SetJSONField(entity, "gpuSettings", plan.GpuSettings)
	SetJSONField(entity, "powerDistributionUnits", plan.PowerDistributionUnits)
	SetJSONField(entity, "staticRoutes", plan.StaticRoutes)
	SetJSONField(entity, "switchPorts", plan.SwitchPorts)
	SetJSONField(entity, "userDefinedResources", plan.UserDefinedResources)

	if svc := buildDeviceServicesAPI(plan.Services); len(svc) > 0 {
		entity["services"] = svc
	}

	// Legacy roles handling is done separately via lookupAndBuildRolesForEntity
	// because it requires BCM API access to get full role objects

	// Build Kubernetes roles from kubelet_role and etcd_host_role blocks
	// These are added to the entity's roles array by buildKubernetesRolesForEntity

	return entity
}

// buildKubernetesRolesForEntity builds KubeletRole and EtcdHostRole entities
// and merges them with existing roles on the device entity.
// This must be called after lookupAndBuildRolesForEntity to properly merge with legacy roles.
func (r *CMDeviceDeviceResource) buildKubernetesRolesForEntity(ctx context.Context, plan CMDeviceDeviceResourceModel, entity map[string]interface{}, existingRoles []map[string]any) error {
	// Build KubeletRole entities from plan
	var kubeletRoles []map[string]any
	if len(plan.KubeletRoles) > 0 {
		kubeletRoles = make([]map[string]any, 0, len(plan.KubeletRoles))
		for _, roleModel := range plan.KubeletRoles {
			roleEntity, err := buildKubeletRoleEntity(ctx, roleModel)
			if err != nil {
				return fmt.Errorf("failed to build kubelet role: %w", err)
			}
			kubeletRoles = append(kubeletRoles, roleEntity)
		}
	}

	// Build EtcdHostRole entities from plan
	var etcdHostRoles []map[string]any
	if len(plan.EtcdHostRoles) > 0 {
		etcdHostRoles = make([]map[string]any, 0, len(plan.EtcdHostRoles))
		for _, roleModel := range plan.EtcdHostRoles {
			roleEntity, err := buildEtcdHostRoleEntity(ctx, roleModel)
			if err != nil {
				return fmt.Errorf("failed to build etcd host role: %w", err)
			}
			etcdHostRoles = append(etcdHostRoles, roleEntity)
		}
	}

	// If the plan explicitly clears all legacy and Kubernetes roles, preserve that empty assignment.
	legacyRolesExplicitlyCleared := false
	if !plan.Roles.IsNull() && !plan.Roles.IsUnknown() {
		var plannedRoles []string
		if diags := plan.Roles.ElementsAs(ctx, &plannedRoles, false); !diags.HasError() && len(plannedRoles) == 0 && len(kubeletRoles) == 0 && len(etcdHostRoles) == 0 {
			legacyRolesExplicitlyCleared = true
			entity["roles"] = []interface{}{}
		}
	}

	// If no Kubernetes roles defined and no existing roles, nothing to do
	if !legacyRolesExplicitlyCleared && len(kubeletRoles) == 0 && len(etcdHostRoles) == 0 && len(existingRoles) == 0 {
		return nil
	}

	// Get existing roles from entity if already set (from legacy roles lookup)
	var currentRoles []map[string]any
	if rolesData, ok := entity["roles"].([]interface{}); ok {
		for _, r := range rolesData {
			if roleMap, ok := r.(map[string]interface{}); ok {
				currentRoles = append(currentRoles, roleMap)
			}
		}
	}

	// Merge: existing legacy roles + new Kubernetes roles
	// mergeDeviceRoles preserves non-Kubernetes roles and replaces Kubernetes roles
	mergedRoles := []map[string]any{}
	if !legacyRolesExplicitlyCleared {
		mergedRoles = mergeDeviceRoles(
			append(existingRoles, currentRoles...),
			kubeletRoles,
			etcdHostRoles,
		)
	} else {
		mergedRoles = append(mergedRoles, kubeletRoles...)
		mergedRoles = append(mergedRoles, etcdHostRoles...)
	}

	// Convert to interface{} slice for JSON marshaling
	rolesInterface := make([]interface{}, len(mergedRoles))
	for i, r := range mergedRoles {
		rolesInterface[i] = r
	}
	entity["roles"] = rolesInterface

	return nil
}

// lookupAndBuildRolesForEntity looks up role objects by name and adds them to the device entity.
// BCM requires full role objects (not just names) when assigning roles to devices.
// This function queries all nodes to extract available role objects, then matches them by name.
// Only role names are accepted - UUIDs are NOT supported (use role names for clarity).
func (r *CMDeviceDeviceResource) lookupAndBuildRolesForEntity(ctx context.Context, plan CMDeviceDeviceResourceModel, entity map[string]interface{}) error {
	if plan.Roles.IsNull() || plan.Roles.IsUnknown() {
		return nil
	}

	var roleIdentifiers []string
	diags := plan.Roles.ElementsAs(ctx, &roleIdentifiers, false)
	if diags.HasError() {
		return fmt.Errorf("failed to extract role identifiers from plan")
	}

	// If empty list, set empty roles (explicit removal)
	if len(roleIdentifiers) == 0 {
		entity["roles"] = []interface{}{}
		return nil
	}

	// Deduplicate role identifiers and validate non-empty
	roleSet := make(map[string]struct{})
	var invalidIdentifiers []string
	for _, id := range roleIdentifiers {
		if id == "" {
			invalidIdentifiers = append(invalidIdentifiers, "(empty string)")
		} else {
			roleSet[id] = struct{}{}
		}
	}

	// Return error if any empty identifiers were found
	if len(invalidIdentifiers) > 0 {
		return fmt.Errorf("invalid role identifiers found: %v - role identifiers must be non-empty strings", invalidIdentifiers)
	}

	// Query all nodes to extract available role objects
	// Roles are embedded in node objects and must be extracted
	body, err := r.Client.CallJSONRPC(ctx, "cmdevice", "getNodes")
	if err != nil {
		return fmt.Errorf("failed to query nodes for role lookup: %w", err)
	}

	var nodes []map[string]interface{}
	if err := json.Unmarshal(body, &nodes); err != nil {
		return fmt.Errorf("failed to parse nodes response: %w", err)
	}

	// Build map of all available role objects by name
	rolesByName := make(map[string]map[string]interface{})
	for _, node := range nodes {
		if rolesData, ok := node["roles"].([]interface{}); ok {
			for _, roleData := range rolesData {
				if role, ok := roleData.(map[string]interface{}); ok {
					if name, ok := role["name"].(string); ok && name != "" {
						rolesByName[name] = role
					}
				}
			}
		}
	}

	// Match requested role identifiers to full role objects
	roleObjects := make([]interface{}, 0, len(roleSet))
	var missingRoles []string

	for identifier := range roleSet {
		// Only lookup by name - UUIDs are NOT supported
		role, found := rolesByName[identifier]

		if found {
			// Create a copy of the role object for this assignment
			roleCopy := make(map[string]interface{})
			for k, v := range role {
				roleCopy[k] = v
			}
			roleObjects = append(roleObjects, roleCopy)
		} else {
			missingRoles = append(missingRoles, identifier)
		}
	}

	if len(missingRoles) > 0 {
		// Build list of available role names for helpful error message
		availableNames := make([]string, 0, len(rolesByName))
		for name := range rolesByName {
			availableNames = append(availableNames, name)
		}
		sort.Strings(availableNames)
		sort.Strings(missingRoles)

		return fmt.Errorf(
			"roles not found in cluster: %s\navailable roles: %s\nuse the `bcm_cmdevice_roles` data source to discover available roles",
			strings.Join(missingRoles, ", "),
			strings.Join(availableNames, ", "),
		)
	}

	// Sort role objects by UUID for consistent ordering
	// IMPORTANT: Must match the sorting in parseRolesFromAPI which sorts by name
	sort.Slice(roleObjects, func(i, j int) bool {
		nameI, _ := roleObjects[i].(map[string]interface{})["name"].(string)
		nameJ, _ := roleObjects[j].(map[string]interface{})["name"].(string)
		return nameI < nameJ
	})

	entity["roles"] = roleObjects
	return nil
}

// parseDeviceFromAPI parses BCM API response into Terraform model.
func (r *CMDeviceDeviceResource) parseDeviceFromAPI(data map[string]interface{}) CMDeviceDeviceResourceModel {
	model := CMDeviceDeviceResourceModel{}

	// Required fields
	if uuid, ok := data["uuid"].(string); ok {
		model.UUID = types.StringValue(uuid)
		model.ID = types.StringValue(uuid) // ID same as UUID
	}

	if hostname, ok := data["hostname"].(string); ok {
		model.Hostname = types.StringValue(hostname)
	}

	// BCM returns "00:00:00:00:00:00" as default MAC — treat as "not set".
	if mac, ok := data["mac"].(string); ok && mac != "" && mac != "00:00:00:00:00:00" {
		model.MAC = types.StringValue(mac)
	} else {
		model.MAC = types.StringNull()
	}

	if category, ok := data["category"].(string); ok {
		model.Category = types.StringValue(category)
	}

	// BCM returns exactly what was sent for managementNetwork.
	// Zero UUID means "not set" — map it to null in Terraform state.
	if managementNetwork, ok := data["managementNetwork"].(string); ok && managementNetwork != "" && managementNetwork != "00000000-0000-0000-0000-000000000000" {
		model.ManagementNetwork = types.StringValue(managementNetwork)
	} else {
		model.ManagementNetwork = types.StringNull()
	}

	// Partition (optional, may not be returned)
	if partition, ok := data["partition"].(string); ok && partition != "" {
		model.Partition = types.StringValue(partition)
	} else {
		model.Partition = types.StringNull()
	}

	// Optional fields with null safety
	if notes, ok := data["notes"].(string); ok && notes != "" {
		model.Notes = types.StringValue(notes)
	} else {
		model.Notes = types.StringNull()
	}

	if kernelParams, ok := data["kernelParameters"].(string); ok && kernelParams != "" {
		model.KernelParameters = types.StringValue(kernelParams)
	} else {
		model.KernelParameters = types.StringNull()
	}

	// BCM returns "CATEGORY" for devices inheriting boot_loader from their category.
	// Map to null — the user never sets this value; omitting it preserves inheritance.
	if bootLoader, ok := data["bootLoader"].(string); ok && bootLoader != "" && !strings.EqualFold(bootLoader, "CATEGORY") {
		model.BootLoader = types.StringValue(bootLoader)
	} else {
		model.BootLoader = types.StringNull()
	}

	// Same CATEGORY handling for boot_loader_protocol.
	if bootLoaderProtocol, ok := data["bootLoaderProtocol"].(string); ok && bootLoaderProtocol != "" && !strings.EqualFold(bootLoaderProtocol, "CATEGORY") {
		model.BootLoaderProtocol = types.StringValue(bootLoaderProtocol)
	} else {
		model.BootLoaderProtocol = types.StringNull()
	}

	// Computed fields
	if creationTime, ok := data["creationTime"].(float64); ok {
		model.CreationTime = types.Int64Value(int64(creationTime))
	}

	if baseType, ok := data["baseType"].(string); ok {
		model.BaseType = types.StringValue(baseType)
	}

	if childType, ok := data["childType"].(string); ok && childType != "" {
		model.ChildType = types.StringValue(childType)
	} else {
		model.ChildType = types.StringNull()
	}

	model.CmdaemonURL = getStringValue(data, "cmdaemonUrl")

	if fips, ok := data["fips"].(string); ok && fips != "" && !strings.EqualFold(fips, "CATEGORY") {
		model.Fips = types.StringValue(fips)
	} else {
		model.Fips = types.StringNull()
	}

	if ft, ok := data["fromTemplateNode"].(string); ok && ft != "" && ft != "00000000-0000-0000-0000-000000000000" {
		model.FromTemplateNode = types.StringValue(ft)
	} else {
		model.FromTemplateNode = types.StringNull()
	}

	model.IndexInsideContainer = getInt64Value(data, "indexInsideContainer")

	parentStr := getStringValue(data, "parentUuid")
	if parentStr.IsNull() {
		parentStr = getStringValue(data, "parent_uuid")
	}
	if !parentStr.IsNull() && parentStr.ValueString() == "00000000-0000-0000-0000-000000000000" {
		model.ParentUUID = types.StringNull()
	} else {
		model.ParentUUID = parentStr
	}

	model.ProvisioningInterface = getStringValue(data, "provisioningInterface")
	model.ProvisioningTransport = getStringValue(data, "provisioningTransport")

	model.Services = parseDeviceServicesFromAPI(data["services"])

	// Power control configuration
	if powerControl, ok := data["powerControl"].(string); ok && powerControl != "" {
		model.PowerControl = types.StringValue(powerControl)
	} else {
		model.PowerControl = types.StringNull()
	}

	// Network gateway configuration
	// BCM returns "0.0.0.0" as the default — treat as "not set" to avoid noise in state.
	if defaultGateway, ok := data["defaultGateway"].(string); ok && defaultGateway != "" && defaultGateway != "0.0.0.0" {
		model.DefaultGateway = types.StringValue(defaultGateway)
	} else {
		model.DefaultGateway = types.StringNull()
	}

	// BCM returns 0 as the default metric — treat as "not set".
	if defaultGatewayMetric, ok := data["defaultGatewayMetric"].(float64); ok && defaultGatewayMetric != 0 {
		model.DefaultGatewayMetric = types.Int64Value(int64(defaultGatewayMetric))
	} else if defaultGatewayMetricInt, ok := data["defaultGatewayMetric"].(int64); ok && defaultGatewayMetricInt != 0 {
		model.DefaultGatewayMetric = types.Int64Value(defaultGatewayMetricInt)
	} else {
		model.DefaultGatewayMetric = types.Int64Null()
	}

	// Hardware identifiers
	if serialNumber, ok := data["serialNumber"].(string); ok && serialNumber != "" {
		model.SerialNumber = types.StringValue(serialNumber)
	} else {
		model.SerialNumber = types.StringNull()
	}

	if partNumber, ok := data["partNumber"].(string); ok && partNumber != "" {
		model.PartNumber = types.StringValue(partNumber)
	} else {
		model.PartNumber = types.StringNull()
	}

	// Provisioning & boot
	model.AllowNetworkingRestart = getBoolValue(data, "allowNetworkingRestart")
	model.BootLoaderFile = getStringValue(data, "bootLoaderFile")
	model.Disksetup = getStringValue(data, "disksetup")
	model.InstallBootRecord = getBoolValue(data, "installBootRecord")
	model.InstallMode = getStringValue(data, "installMode")
	model.NextBootInstallMode = getStringValue(data, "nextBootInstallMode")
	model.NodeInstallerDisk = getBoolValue(data, "nodeInstallerDisk")
	model.Pxelabel = getStringValue(data, "pxelabel")
	model.Raidconf = getStringValue(data, "raidconf")
	model.VersionConfigFiles = getBoolValue(data, "versionConfigFiles")

	// Kernel & performance
	model.CpuspeedGovernor = getStringValue(data, "cpuspeedGovernor")
	model.IoScheduler = getStringValue(data, "ioScheduler")
	model.KernelOutputConsole = getStringValue(data, "kernelOutputConsole")
	model.KernelVersion = getStringValue(data, "kernelVersion")

	// Script hooks
	model.CustomPingScript = getStringValue(data, "customPingScript")
	model.CustomPingScriptArgument = getStringValue(data, "customPingScriptArgument")
	model.CustomPowerScript = getStringValue(data, "customPowerScript")
	model.CustomPowerScriptArgument = getStringValue(data, "customPowerScriptArgument")
	model.CustomRemoteConsoleScript = getStringValue(data, "customRemoteConsoleScript")
	model.CustomRemoteConsoleScriptArgument = getStringValue(data, "customRemoteConsoleScriptArgument")
	model.Finalize = getStringValue(data, "finalize")
	model.Initialize = getStringValue(data, "initialize")

	// Exclude lists
	model.ExcludeListFull = getStringValue(data, "excludeListFull")
	model.ExcludeListGrab = getStringValue(data, "excludeListGrab")
	model.ExcludeListGrabnew = getStringValue(data, "excludeListGrabnew")
	model.ExcludeListManipulateScript = getStringValue(data, "excludeListManipulateScript")
	model.ExcludeListSync = getStringValue(data, "excludeListSync")
	model.ExcludeListUpdate = getStringValue(data, "excludeListUpdate")

	// Device flags
	model.DataNode = getBoolValue(data, "dataNode")
	model.DisableFabricNVME = getBoolValue(data, "disableFabricNVME")
	model.ForceFullEnvironment = getBoolValue(data, "forceFullEnvironment")
	model.SupportsGNSS = getBoolValue(data, "supportsGNSS")
	model.TemplateNode = getBoolValue(data, "templateNode")

	// Metadata & user-defined
	model.Tag = getStringValue(data, "tag")
	model.UseExclusivelyFor = getStringValue(data, "useExclusivelyFor")
	model.Userdefined1 = getStringValue(data, "userdefined1")
	model.Userdefined2 = getStringValue(data, "userdefined2")

	// References
	model.Rack = getJSONValue(data, "rack")
	model.SoftwareImageProxy = getJSONValue(data, "softwareImageProxy")

	// String lists
	model.BlockDevicesClearedOnNextBoot = GetStringListValue(data, "blockDevicesClearedOnNextBoot")
	model.Modules = GetStringListValue(data, "modules")

	// Complex objects (JSON-encoded)
	model.BiosSetup = getJSONValue(data, "biosSetup")
	model.BmcSettings = getJSONValue(data, "bmcSettings")
	model.ExtraValues = getJSONValue(data, "extra_values")
	model.ProxySettings = getJSONValue(data, "proxySettings")
	model.SeLinuxSettings = getJSONValue(data, "seLinuxSettings")
	model.TimeZoneSettings = getJSONValue(data, "timeZoneSettings")

	// Complex lists (JSON-encoded)
	model.Fsexports = getJSONValue(data, "fsexports")
	model.Fsmounts = getJSONValue(data, "fsmounts")
	model.GpuSettings = getJSONValue(data, "gpuSettings")
	model.PowerDistributionUnits = getJSONValue(data, "powerDistributionUnits")
	model.StaticRoutes = getJSONValue(data, "staticRoutes")
	model.SwitchPorts = getJSONValue(data, "switchPorts")
	model.UserDefinedResources = getJSONValue(data, "userDefinedResources")

	// Force is not persisted by BCM, will be preserved from plan/state

	// Parse interfaces from BCM response
	model.Interfaces = parseInterfacesFromAPI(data["interfaces"])

	// Parse roles from BCM response
	// BCM returns roles as an array of role objects with "name" field, or as an array of strings
	model.Roles = parseRolesFromAPI(data["roles"])

	// Parse Kubernetes roles (kubelet_role, etcd_host_role) from BCM response
	model.KubeletRoles, model.EtcdHostRoles = parseKubernetesRolesFromAPI(data["roles"])

	return model
}

// deviceStringFromKeys returns the first non-empty string found under alternate JSON keys.
func deviceStringFromKeys(m map[string]interface{}, keys ...string) types.String {
	for _, k := range keys {
		if v := getStringValue(m, k); !v.IsNull() {
			return v
		}
	}
	return types.StringNull()
}

// parseDeviceServicesFromAPI parses device.services[] from BCM into Terraform models.
func parseDeviceServicesFromAPI(raw interface{}) []DeviceOSServiceConfigModel {
	arr, ok := raw.([]interface{})
	if !ok || len(arr) == 0 {
		return nil
	}
	out := make([]DeviceOSServiceConfigModel, 0, len(arr))
	for _, item := range arr {
		sm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, parseDeviceOSServiceFromAPI(sm))
	}
	return out
}

func parseDeviceOSServiceFromAPI(m map[string]interface{}) DeviceOSServiceConfigModel {
	var out DeviceOSServiceConfigModel
	out.UUID = getStringValue(m, "uuid")
	out.Name = getStringValue(m, "name")
	out.BaseType = getStringValue(m, "baseType")
	out.AddFromRole = getBoolValue(m, "addFromRole")
	if out.AddFromRole.IsNull() {
		out.AddFromRole = getBoolValue(m, "add_from_role")
	}
	out.Autostart = getBoolValue(m, "autostart")
	out.BelongsToRole = getBoolValue(m, "belongsToRole")
	if out.BelongsToRole.IsNull() {
		out.BelongsToRole = getBoolValue(m, "belongs_to_role")
	}
	out.Monitored = getBoolValue(m, "monitored")
	out.RefExtraUUID = deviceStringFromKeys(m, "refExtraUuid", "ref_extra_uuid")
	out.RefRoleUUID = deviceStringFromKeys(m, "refRoleUuid", "ref_role_uuid")
	out.RunIf = deviceStringFromKeys(m, "runIf", "run_if")
	out.ScriptTimeout = getInt64Value(m, "scriptTimeout")
	if out.ScriptTimeout.IsNull() {
		out.ScriptTimeout = getInt64Value(m, "script_timeout")
	}
	out.ServiceType = getInt64Value(m, "serviceType")
	if out.ServiceType.IsNull() {
		out.ServiceType = getInt64Value(m, "service_type")
	}
	out.SicknessCheckInterval = getInt64Value(m, "sicknessCheckInterval")
	if out.SicknessCheckInterval.IsNull() {
		out.SicknessCheckInterval = getInt64Value(m, "sickness_check_interval")
	}
	out.SicknessCheckScriptTimeout = getInt64Value(m, "sicknessCheckScriptTimeout")
	if out.SicknessCheckScriptTimeout.IsNull() {
		out.SicknessCheckScriptTimeout = getInt64Value(m, "sickness_check_script_timeout")
	}
	out.ChildType = deviceStringFromKeys(m, "childType", "child_type")
	out.FromGenericRole = getBoolValue(m, "fromGenericRole")
	if out.FromGenericRole.IsNull() {
		out.FromGenericRole = getBoolValue(m, "from_generic_role")
	}
	out.Internal = getBoolValue(m, "internal")
	out.SicknessCheckScript = deviceStringFromKeys(m, "sicknessCheckScript", "sickness_check_script")
	return out
}

// filterServicesToMatchConfig returns only the BCM-returned services whose names
// match a service in the configured list. This filters out role-injected services
// (belongsToRole=true, addFromRole=true) that the user didn't explicitly configure,
// preventing perpetual drift caused by BCM automatically adding services from roles.
func filterServicesToMatchConfig(bcmServices, configuredServices []DeviceOSServiceConfigModel) []DeviceOSServiceConfigModel {
	if len(configuredServices) == 0 {
		return nil
	}

	configuredNames := make(map[string]struct{}, len(configuredServices))
	for _, s := range configuredServices {
		if !s.Name.IsNull() && !s.Name.IsUnknown() {
			configuredNames[s.Name.ValueString()] = struct{}{}
		}
	}

	if len(bcmServices) == 0 {
		return configuredServices
	}

	var result []DeviceOSServiceConfigModel
	for _, s := range bcmServices {
		if !s.Name.IsNull() {
			if _, ok := configuredNames[s.Name.ValueString()]; ok {
				result = append(result, s)
			}
		}
	}

	if len(result) == 0 {
		return configuredServices
	}
	return result
}

// buildDeviceServicesAPI builds BCM JSON for device.services from Terraform models.
func buildDeviceServicesAPI(models []DeviceOSServiceConfigModel) []interface{} {
	if len(models) == 0 {
		return nil
	}
	out := make([]interface{}, 0, len(models))
	for _, s := range models {
		sm := map[string]interface{}{
			"baseType":      "OSServiceConfig",
			"modified":      true,
			"to_be_removed": false,
			"revision":      "",
		}
		if !s.BaseType.IsNull() && !s.BaseType.IsUnknown() && s.BaseType.ValueString() != "" {
			sm["baseType"] = s.BaseType.ValueString()
		}
		SetStringField(sm, "uuid", s.UUID)
		SetStringField(sm, "name", s.Name)

		// Bools: always send explicit value (array elements are full-replacement)
		if !s.AddFromRole.IsNull() && !s.AddFromRole.IsUnknown() {
			sm["addFromRole"] = s.AddFromRole.ValueBool()
		} else {
			sm["addFromRole"] = false
		}
		if !s.Autostart.IsNull() && !s.Autostart.IsUnknown() {
			sm["autostart"] = s.Autostart.ValueBool()
		} else {
			sm["autostart"] = false
		}
		if !s.BelongsToRole.IsNull() && !s.BelongsToRole.IsUnknown() {
			sm["belongsToRole"] = s.BelongsToRole.ValueBool()
		} else {
			sm["belongsToRole"] = false
		}
		if !s.Monitored.IsNull() && !s.Monitored.IsUnknown() {
			sm["monitored"] = s.Monitored.ValueBool()
		} else {
			sm["monitored"] = false
		}

		if !s.RefExtraUUID.IsNull() && !s.RefExtraUUID.IsUnknown() {
			sm["ref_extra_uuid"] = s.RefExtraUUID.ValueString()
		} else {
			sm["ref_extra_uuid"] = "00000000-0000-0000-0000-000000000000"
		}
		if !s.RefRoleUUID.IsNull() && !s.RefRoleUUID.IsUnknown() {
			sm["ref_role_uuid"] = s.RefRoleUUID.ValueString()
		} else {
			sm["ref_role_uuid"] = "00000000-0000-0000-0000-000000000000"
		}

		// Strings: send explicit default for array elements
		if !s.RunIf.IsNull() && !s.RunIf.IsUnknown() {
			sm["runIf"] = s.RunIf.ValueString()
		} else {
			sm["runIf"] = "ALWAYS"
		}

		// Ints: always send explicit value
		if !s.ScriptTimeout.IsNull() && !s.ScriptTimeout.IsUnknown() {
			sm["scriptTimeout"] = s.ScriptTimeout.ValueInt64()
		} else {
			sm["scriptTimeout"] = int64(-1)
		}
		if !s.ServiceType.IsNull() && !s.ServiceType.IsUnknown() {
			sm["serviceType"] = s.ServiceType.ValueInt64()
		} else {
			sm["serviceType"] = int64(0)
		}
		if !s.SicknessCheckInterval.IsNull() && !s.SicknessCheckInterval.IsUnknown() {
			sm["sicknessCheckInterval"] = s.SicknessCheckInterval.ValueInt64()
		} else {
			sm["sicknessCheckInterval"] = int64(60)
		}
		if !s.SicknessCheckScriptTimeout.IsNull() && !s.SicknessCheckScriptTimeout.IsUnknown() {
			sm["sicknessCheckScriptTimeout"] = s.SicknessCheckScriptTimeout.ValueInt64()
		} else {
			sm["sicknessCheckScriptTimeout"] = int64(10)
		}

		if !s.ChildType.IsNull() && !s.ChildType.IsUnknown() {
			sm["childType"] = s.ChildType.ValueString()
		} else {
			sm["childType"] = ""
		}
		if !s.FromGenericRole.IsNull() && !s.FromGenericRole.IsUnknown() {
			sm["fromGenericRole"] = s.FromGenericRole.ValueBool()
		} else {
			sm["fromGenericRole"] = false
		}
		if !s.Internal.IsNull() && !s.Internal.IsUnknown() {
			sm["internal"] = s.Internal.ValueBool()
		} else {
			sm["internal"] = false
		}
		if !s.SicknessCheckScript.IsNull() && !s.SicknessCheckScript.IsUnknown() {
			sm["sicknessCheckScript"] = s.SicknessCheckScript.ValueString()
		} else {
			sm["sicknessCheckScript"] = ""
		}
		out = append(out, sm)
	}
	return out
}

// parseKubernetesRolesFromAPI extracts KubeletRole and EtcdHostRole from BCM device roles array.
// Returns slices of models for each role type.
func parseKubernetesRolesFromAPI(rolesData interface{}) ([]KubeletRoleModel, []EtcdHostRoleModel) {
	if rolesData == nil {
		return nil, nil
	}

	rolesArray, ok := rolesData.([]interface{})
	if !ok || len(rolesArray) == 0 {
		return nil, nil
	}

	var kubeletRoles []KubeletRoleModel
	var etcdHostRoles []EtcdHostRoleModel

	for _, roleItem := range rolesArray {
		roleMap, ok := roleItem.(map[string]interface{})
		if !ok {
			continue
		}

		childType, _ := roleMap["childType"].(string)
		switch strings.ToLower(childType) {
		case "kubeletrole":
			kubeletRoles = append(kubeletRoles, parseKubeletRoleFromAPI(roleMap))
		case "etcdhostrole":
			etcdHostRoles = append(etcdHostRoles, parseEtcdHostRoleFromAPI(roleMap))
		}
	}

	return kubeletRoles, etcdHostRoles
}

// parseRolesFromAPI parses BCM API roles response into a Terraform set of role names.
// BCM returns roles as an array of role objects: [{"uuid": "...", "name": "backup", ...}]
// We extract names because users configure roles by name (the recommended approach).
// Returns a Set since order of roles is not significant.
func parseRolesFromAPI(rolesData interface{}) types.Set {
	if rolesData == nil {
		return types.SetNull(types.StringType)
	}

	rolesArray, ok := rolesData.([]interface{})
	if !ok || len(rolesArray) == 0 {
		return emptyStringSet()
	}

	// Extract role names from the array
	roleNames := make([]string, 0, len(rolesArray))
	for _, roleItem := range rolesArray {
		if role, ok := roleItem.(map[string]interface{}); ok {
			// Role object with "name" field
			if name, ok := role["name"].(string); ok && name != "" {
				roleNames = append(roleNames, name)
			}
		}
	}

	if len(roleNames) == 0 {
		return emptyStringSet()
	}

	// Convert to Terraform set (order doesn't matter for sets)
	roleValues := make([]attr.Value, len(roleNames))
	for i, name := range roleNames {
		roleValues[i] = types.StringValue(name)
	}

	rolesSet, diags := types.SetValue(types.StringType, roleValues)
	if diags.HasError() {
		// Fall back to null set if construction fails (should not happen with valid string elements)
		return types.SetNull(types.StringType)
	}
	return rolesSet
}
