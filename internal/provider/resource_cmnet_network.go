// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &CMNetNetworkResource{}
	_ resource.ResourceWithImportState = &CMNetNetworkResource{}
)

// NewCMNetNetworkResource creates a new instance of the network resource.
func NewCMNetNetworkResource() resource.Resource {
	return &CMNetNetworkResource{}
}

// CMNetNetworkResource defines the resource implementation.
type CMNetNetworkResource struct {
	BCMResourceBase
}

// CMNetNetworkResourceModel describes the resource data model.
type CMNetNetworkResourceModel struct {
	ID             types.String `tfsdk:"id"`
	UUID           types.String `tfsdk:"uuid"`
	Name           types.String `tfsdk:"name"`
	Subnet         types.String `tfsdk:"subnet"`
	BaseAddress    types.String `tfsdk:"base_address"`
	NetmaskBits    types.Int64  `tfsdk:"netmask_bits"`
	Gateway        types.String `tfsdk:"gateway"`
	NetworkType    types.String `tfsdk:"network_type"`
	MTU            types.Int64  `tfsdk:"mtu"`
	DomainName     types.String `tfsdk:"domain_name"`
	DHCPEnabled    types.Bool   `tfsdk:"dhcp_enabled"`
	DHCPRangeStart types.String `tfsdk:"dhcp_range_start"`
	DHCPRangeEnd   types.String `tfsdk:"dhcp_range_end"`
	Management     types.Bool   `tfsdk:"management"`
	Bootable       types.Bool   `tfsdk:"bootable"`
	Notes          types.String `tfsdk:"notes"`
	BaseType       types.String `tfsdk:"base_type"`
	ChildType      types.String `tfsdk:"child_type"`
	Revision       types.String `tfsdk:"revision"`
	Modified       types.Bool   `tfsdk:"modified"`
	ToBeRemoved    types.Bool   `tfsdk:"to_be_removed"`

	AllowAutosign           types.String `tfsdk:"allow_autosign"`
	GenerateDNSZone         types.String `tfsdk:"generate_dns_zone"`
	LockDownDhcpd           types.Bool   `tfsdk:"lock_down_dhcpd"`
	DisableAutomaticExports types.Bool   `tfsdk:"disable_automatic_exports"`
	ExcludeFromSearchDomain types.Bool   `tfsdk:"exclude_from_search_domain"`
	SearchDomainIndex       types.Int64  `tfsdk:"search_domain_index"`
	IPv6Enabled             types.Bool   `tfsdk:"ipv6_enabled"`
	IPv6BaseAddress         types.String `tfsdk:"ipv6_base_address"`
	IPv6Gateway             types.String `tfsdk:"ipv6_gateway"`
	IPv6NetmaskBits         types.Int64  `tfsdk:"ipv6_netmask_bits"`
	EC2AvailabilityZone     types.String `tfsdk:"ec2_availability_zone"`
	EC2String               types.String `tfsdk:"ec2_string"`
	CloudSubnetID           types.String `tfsdk:"cloud_subnet_id"`
	ExtraValues             types.String `tfsdk:"extra_values"`
}

// Metadata returns the resource type name.
func (r *CMNetNetworkResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cmnet_network"
}

// Schema defines the resource schema.
func (r *CMNetNetworkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a BCM network configuration with full CRUD lifecycle support.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Network unique identifier (same as uuid).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"uuid": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "BCM-assigned UUID for the network.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Unique network name within the BCM cluster.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
				},
			},
			"subnet": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Network subnet in CIDR notation (e.g., '10.0.1.0/24'). When set, base_address and netmask_bits are automatically computed.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						cidrRegex(),
						"must be valid CIDR notation (e.g., '10.0.1.0/24')",
					),
				},
			},
			"base_address": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Network base IP address (parsed from subnet or assigned by BCM).",
			},
			"netmask_bits": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Network mask bits (parsed from subnet or assigned by BCM).",
			},
			"gateway": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Gateway IP address for the network.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`),
						"must be a valid IPv4 address",
					),
				},
			},
			"network_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Network type (e.g., INTERNAL, GLOBAL). BCM defaults to INTERNAL.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mtu": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Maximum transmission unit (MTU) for the network. Default: 1500.",
			},
			"domain_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "DNS domain name for the network. Default: 'cluster.local' if not specified.",
			},
			"dhcp_enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether DHCP is enabled (derived from dhcp_range_start and dhcp_range_end).",
			},
			"dhcp_range_start": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "DHCP pool start IP address. Setting both dhcp_range_start and dhcp_range_end enables DHCP.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`),
						"must be a valid IPv4 address",
					),
				},
			},
			"dhcp_range_end": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "DHCP pool end IP address. Setting both dhcp_range_start and dhcp_range_end enables DHCP.",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`),
						"must be a valid IPv4 address",
					),
				},
			},
			"management": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether this is a management network.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"bootable": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether this network supports PXE boot.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"notes": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "User notes or description for the network.",
			},
			"base_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "BCM entity base type (always 'Network').",
			},
			"child_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "BCM entity child type.",
			},
			"revision": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "BCM revision identifier for concurrency control.",
			},
			"modified": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "BCM modification flag.",
			},
			"to_be_removed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "BCM removal flag.",
			},
			"allow_autosign": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Certificate auto-signing policy (AUTOMATIC, YES, NO). Default: AUTOMATIC.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"generate_dns_zone": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "DNS zone generation mode (BOTH, FORWARD, REVERSE, NONE). Default: BOTH.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"lock_down_dhcpd": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether DHCP server is locked down for this network.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"disable_automatic_exports": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether automatic NFS exports are disabled for this network.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"exclude_from_search_domain": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether this network is excluded from DNS search domains.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"search_domain_index": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "DNS search domain priority index. Default: 0.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"ipv6_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether IPv6 is enabled on this network.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"ipv6_base_address": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "IPv6 base address for the network. Default: ::0.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ipv6_gateway": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "IPv6 gateway address for the network. Default: ::0.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ipv6_netmask_bits": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "IPv6 network mask bits. Default: 0.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"ec2_availability_zone": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "AWS EC2 availability zone for cloud-integrated networks.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ec2_string": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "AWS EC2 configuration string for cloud-integrated networks.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cloud_subnet_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Cloud subnet identifier for cloud-integrated networks.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"extra_values": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "BCM internal extra values (JSON). Read-only.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *CMNetNetworkResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.ConfigureResource(req, resp)
}

// Create creates a new network resource.
func (r *CMNetNetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.Client == nil {
		resp.Diagnostics.AddError(
			"Provider Not Configured",
			"The provider has not been configured. Please ensure the provider block is properly configured.",
		)
		return
	}

	var plan CMNetNetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build API entity (with generated UUID for create - Networks REQUIRE UUID)
	entity, err := buildNetworkAPIEntity(ctx, &plan, "")
	if err != nil {
		resp.Diagnostics.AddError("Invalid Network Configuration", err.Error())
		return
	}

	// Capture the generated UUID from the entity (Networks require UUID even for create).
	generatedUUID, ok := entity["uuid"].(string)
	if !ok {
		resp.Diagnostics.AddError(
			"Internal Error",
			"Failed to extract UUID from created entity",
		)
		return
	}

	// Pre-flight validation: Call validateNetwork before CREATE
	validationErrors, err := r.Client.ValidateEntity(ctx, "CMNet", "validateNetwork", entity, true)
	if err != nil {
		resp.Diagnostics.AddError(
			"Validation API Error",
			fmt.Sprintf("Could not validate network '%s': %s", plan.Name.ValueString(), err.Error()),
		)
		return
	}

	// Process validation results - halt if errors found
	if ProcessValidationErrors(validationErrors, &resp.Diagnostics) {
		return
	}

	tflog.Debug(ctx, "Creating network via BCM API", map[string]interface{}{
		"name": plan.Name.ValueString(),
		"uuid": generatedUUID,
	})

	// Call BCM API to create network
	body, err := r.Client.CallJSONRPC(ctx, "cmnet", "addNetwork", entity)
	if err != nil {
		resp.Diagnostics.AddError(
			"Network Creation Failed",
			fmt.Sprintf("Failed to create network '%s': %s", plan.Name.ValueString(), err.Error()),
		)
		return
	}

	// Parse response - BCM returns success/validation structure, not the entity
	var validationResp struct {
		Success       bool                     `json:"success"`
		UpdatedEntity map[string]interface{}   `json:"updated_entity"`
		Validation    []map[string]interface{} `json:"validation"`
	}

	if err := json.Unmarshal(body, &validationResp); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Create Response",
			fmt.Sprintf("Could not parse BCM API response: %s", err.Error()),
		)
		return
	}

	// Check for validation errors
	if !validationResp.Success {
		var errorMsgs []string
		for _, v := range validationResp.Validation {
			if severity, ok := v["severity"].(string); ok && severity == "ERROR" {
				if msg, ok := v["message"].(string); ok {
					errorMsgs = append(errorMsgs, msg)
				}
			}
		}
		if len(errorMsgs) > 0 {
			resp.Diagnostics.AddError(
				"Network Creation Validation Failed",
				fmt.Sprintf("BCM API validation errors: %v", errorMsgs),
			)
			return
		}
	}

	// Since BCM returns null updated_entity, do a follow-up Read to get full network data
	readBody, err := r.Client.CallJSONRPC(ctx, "cmnet", "getNetwork", generatedUUID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Read Created Network",
			fmt.Sprintf("Network created but failed to read back: %s", err.Error()),
		)
		return
	}

	var createdNetwork map[string]interface{}
	if err := json.Unmarshal(readBody, &createdNetwork); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Read Response",
			fmt.Sprintf("Could not parse BCM API response: %s", err.Error()),
		)
		return
	}

	plannedMTU := plan.MTU
	plannedDomainName := plan.DomainName
	mapNetworkAPIResponseToState(ctx, createdNetwork, &plan)
	if !plannedMTU.IsNull() && !plannedMTU.IsUnknown() && plannedMTU.ValueInt64() == 1500 && plan.MTU.IsNull() {
		plan.MTU = plannedMTU
	}
	if plannedDomainName.IsNull() && !plan.DomainName.IsNull() && plan.DomainName.ValueString() == "cluster.local" {
		plan.DomainName = types.StringNull()
	}

	tflog.Trace(ctx, "Created network resource", map[string]interface{}{
		"uuid": plan.UUID.ValueString(),
		"name": plan.Name.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the resource state from BCM.
func (r *CMNetNetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.Client == nil {
		resp.Diagnostics.AddError(
			"Provider Not Configured",
			"The provider has not been configured. Please ensure the provider block is properly configured.",
		)
		return
	}

	var state CMNetNetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get UUID for lookup - use ID if UUID not set (happens during import)
	lookupID := state.UUID.ValueString()
	if lookupID == "" {
		lookupID = state.ID.ValueString()
	}

	// Call BCM API to get network by UUID (efficient direct lookup)
	body, err := r.Client.CallJSONRPC(ctx, "cmnet", "getNetwork", lookupID)
	if err != nil {
		errorMsg := err.Error()
		if containsAny(errorMsg, []string{"not found", "does not exist", "Unable to use map_key_to_string"}) {
			// Network was deleted outside Terraform - remove from state
			tflog.Info(ctx, "Network not found during refresh - removing from state", map[string]interface{}{
				"uuid": state.UUID.ValueString(),
				"name": state.Name.ValueString(),
			})
			resp.State.RemoveResource(ctx)
			return
		}
		// Other errors should be reported to the user
		resp.Diagnostics.AddError(
			"Network Read Failed",
			fmt.Sprintf("Failed to read network '%s' (UUID: %s): %s", state.Name.ValueString(), state.UUID.ValueString(), err.Error()),
		)
		return
	}

	// Parse response
	var network map[string]interface{}
	if err := json.Unmarshal(body, &network); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Read Response",
			fmt.Sprintf("Could not parse BCM API response: %s", err.Error()),
		)
		return
	}

	// Map response to state
	priorMTU := state.MTU
	priorDomainName := state.DomainName
	mapNetworkAPIResponseToState(ctx, network, &state)
	if !priorMTU.IsNull() && !priorMTU.IsUnknown() && priorMTU.ValueInt64() == 1500 && state.MTU.IsNull() {
		state.MTU = priorMTU
	}
	if priorDomainName.IsNull() && !state.DomainName.IsNull() && state.DomainName.ValueString() == "cluster.local" {
		state.DomainName = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the network resource.
func (r *CMNetNetworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.Client == nil {
		resp.Diagnostics.AddError(
			"Provider Not Configured",
			"The provider has not been configured. Please ensure the provider block is properly configured.",
		)
		return
	}

	var plan CMNetNetworkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state for revision (computed field comes from state, not plan)
	var state CMNetNetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build API entity (includes UUID and revision for update)
	entity, err := buildNetworkAPIEntity(ctx, &plan, plan.UUID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Network Configuration", err.Error())
		return
	}

	// Include revision from state for concurrency control (revision is a computed field)
	entity["revision"] = state.Revision.ValueString()

	// Pre-flight validation: Call validateNetwork before UPDATE
	validationErrors, err := r.Client.ValidateEntity(ctx, "CMNet", "validateNetwork", entity, false)
	if err != nil {
		resp.Diagnostics.AddError(
			"Validation API Error",
			fmt.Sprintf("Could not validate network '%s': %s", plan.Name.ValueString(), err.Error()),
		)
		return
	}

	// Process validation results - halt if errors found
	if ProcessValidationErrors(validationErrors, &resp.Diagnostics) {
		return
	}

	tflog.Debug(ctx, "Updating network via BCM API", map[string]interface{}{
		"uuid": plan.UUID.ValueString(),
		"name": plan.Name.ValueString(),
	})

	// Call BCM API to update network (force=false for safety)
	_, err = r.Client.CallJSONRPC(ctx, "cmnet", "updateNetwork", entity, false)
	if err != nil {
		resp.Diagnostics.AddError(
			"Network Update Failed",
			fmt.Sprintf("Failed to update network '%s': %s", plan.Name.ValueString(), err.Error()),
		)
		return
	}

	// BCM updateNetwork returns the updated entity directly (not wrapped in validation response)
	// Unlike addNetwork which may return validation errors, updateNetwork performs validation
	// before the update and returns the entity on success. We do a follow-up read for consistency.
	readBody, err := r.Client.CallJSONRPC(ctx, "cmnet", "getNetwork", plan.UUID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Read Updated Network",
			fmt.Sprintf("Network updated but failed to read back: %s", err.Error()),
		)
		return
	}

	var updatedNetwork map[string]interface{}
	if err := json.Unmarshal(readBody, &updatedNetwork); err != nil {
		resp.Diagnostics.AddError(
			"Error Parsing Read Response",
			fmt.Sprintf("Could not parse BCM API response: %s", err.Error()),
		)
		return
	}

	// Map response to state
	plannedMTU := plan.MTU
	plannedDomainName := plan.DomainName
	mapNetworkAPIResponseToState(ctx, updatedNetwork, &plan)
	if !plannedMTU.IsNull() && !plannedMTU.IsUnknown() && plannedMTU.ValueInt64() == 1500 && plan.MTU.IsNull() {
		plan.MTU = plannedMTU
	}
	if plannedDomainName.IsNull() && !plan.DomainName.IsNull() && plan.DomainName.ValueString() == "cluster.local" {
		plan.DomainName = types.StringNull()
	}

	tflog.Trace(ctx, "Updated network resource", map[string]interface{}{
		"uuid": plan.UUID.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the network resource.
func (r *CMNetNetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.Client == nil {
		resp.Diagnostics.AddError(
			"Provider Not Configured",
			"The provider has not been configured. Please ensure the provider block is properly configured.",
		)
		return
	}

	var state CMNetNetworkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting network via BCM API", map[string]interface{}{
		"uuid": state.UUID.ValueString(),
		"name": state.Name.ValueString(),
	})

	// Call BCM API to delete network
	_, err := r.Client.CallJSONRPC(ctx, "cmnet", "removeNetwork", state.UUID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Network Deletion Failed",
			fmt.Sprintf("Failed to delete network '%s': %s\nIf the network has active node assignments, manual cleanup may be required.", state.Name.ValueString(), err.Error()),
		)
		return
	}

	tflog.Trace(ctx, "Deleted network resource", map[string]interface{}{
		"uuid": state.UUID.ValueString(),
	})
}

// ImportState imports an existing network by UUID.
func (r *CMNetNetworkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helper functions

// buildNetworkAPIEntity converts Terraform model to BCM API entity.
func buildNetworkAPIEntity(ctx context.Context, data *CMNetNetworkResourceModel, uuid string) (map[string]interface{}, error) {
	_ = ctx // ctx is unused but required by signature
	entity := map[string]interface{}{
		"baseType":      "Network",
		"childType":     "",
		"modified":      true,
		"to_be_removed": false,
		"revision":      "",
	}

	// BCM REQUIRES a UUID for networks (like categories, unlike software images)
	// For create: generate new UUID, for update: use existing UUID
	if uuid != "" {
		entity["uuid"] = uuid
	} else {
		// Generate UUID for new network
		entity["uuid"] = generateUUID()
	}

	// Required fields
	SetStringField(entity, "name", data.Name)

	// Domain name (required by BCM, use default if not provided)
	if !data.DomainName.IsNull() && !data.DomainName.IsUnknown() && data.DomainName.ValueString() != "" {
		entity["domainName"] = data.DomainName.ValueString()
	} else {
		entity["domainName"] = "cluster.local"
	}

	// Parse subnet if provided
	if !data.Subnet.IsNull() && !data.Subnet.IsUnknown() {
		baseAddr, maskBits, err := parseCIDR(data.Subnet.ValueString())
		if err != nil {
			return nil, err
		}
		entity["baseAddress"] = baseAddr
		entity["netmaskBits"] = maskBits
	}

	// Optional fields
	SetStringField(entity, "gateway", data.Gateway)
	SetInt64Field(entity, "mtu", data.MTU)
	SetStringField(entity, "dynamicRangeStart", data.DHCPRangeStart)
	SetStringField(entity, "dynamicRangeEnd", data.DHCPRangeEnd)
	SetStringField(entity, "notes", data.Notes)

	// Network classification
	SetStringField(entity, "type", data.NetworkType)
	SetBoolField(entity, "management", data.Management)
	SetBoolField(entity, "bootable", data.Bootable)

	// DNS/DHCP settings
	SetStringField(entity, "allowAutosign", data.AllowAutosign)
	SetStringField(entity, "generateDNSZone", data.GenerateDNSZone)
	SetBoolField(entity, "lockDownDhcpd", data.LockDownDhcpd)
	SetBoolField(entity, "disableAutomaticExports", data.DisableAutomaticExports)
	SetBoolField(entity, "excludeFromSearchDomain", data.ExcludeFromSearchDomain)
	SetInt64Field(entity, "searchDomainIndex", data.SearchDomainIndex)

	// IPv6 settings
	SetBoolField(entity, "IPv6", data.IPv6Enabled)
	SetStringField(entity, "ipv6BaseAddress", data.IPv6BaseAddress)
	SetStringField(entity, "ipv6Gateway", data.IPv6Gateway)
	SetInt64Field(entity, "ipv6NetmaskBits", data.IPv6NetmaskBits)

	// Cloud settings
	SetStringField(entity, "EC2AvailabilityZone", data.EC2AvailabilityZone)
	SetStringField(entity, "EC2String", data.EC2String)
	SetStringField(entity, "cloudSubnetID", data.CloudSubnetID)

	return entity, nil
}

// mapNetworkAPIResponseToState maps BCM API response to Terraform state.
func mapNetworkAPIResponseToState(ctx context.Context, apiData map[string]interface{}, data *CMNetNetworkResourceModel) {
	_ = ctx // ctx is unused but required by signature
	// Identity fields
	if uuid, ok := apiData["uuid"].(string); ok {
		data.UUID = types.StringValue(uuid)
		data.ID = types.StringValue(uuid)
	}
	if name, ok := apiData["name"].(string); ok {
		data.Name = types.StringValue(name)
	}

	// Network addressing - only set if not BCM defaults
	baseAddr := getStringValue(apiData, "baseAddress")
	netmaskBits := getInt64Value(apiData, "netmaskBits")

	// Only populate subnet if baseAddress is meaningful (not 0.0.0.0)
	if !baseAddr.IsNull() && baseAddr.ValueString() != "0.0.0.0" && baseAddr.ValueString() != "" {
		data.BaseAddress = baseAddr
		data.NetmaskBits = netmaskBits
		subnet := formatCIDR(baseAddr.ValueString(), netmaskBits.ValueInt64())
		data.Subnet = types.StringValue(subnet)
	} else {
		// BCM returned default/empty value - reset to null to reflect actual API state
		// This handles both: 1) plan was null, and 2) BCM externally cleared the subnet
		data.BaseAddress = types.StringNull()
		data.NetmaskBits = types.Int64Null()
		data.Subnet = types.StringNull()
	}

	// DHCP configuration - only set if non-default
	rangeStart := getStringValue(apiData, "dynamicRangeStart")
	rangeEnd := getStringValue(apiData, "dynamicRangeEnd")

	if !rangeStart.IsNull() && rangeStart.ValueString() != "0.0.0.0" && rangeStart.ValueString() != "" {
		data.DHCPRangeStart = rangeStart
	} else {
		// BCM returned default/empty - reset to null to reflect actual API state
		data.DHCPRangeStart = types.StringNull()
	}

	if !rangeEnd.IsNull() && rangeEnd.ValueString() != "0.0.0.0" && rangeEnd.ValueString() != "" {
		data.DHCPRangeEnd = rangeEnd
	} else {
		// BCM returned default/empty - reset to null to reflect actual API state
		data.DHCPRangeEnd = types.StringNull()
	}

	// Derive DHCP enabled
	dhcpEnabled := isDHCPEnabled(
		data.DHCPRangeStart.ValueString(),
		data.DHCPRangeEnd.ValueString(),
	)
	data.DHCPEnabled = types.BoolValue(dhcpEnabled)

	// Optional fields - only set if non-default
	gateway := getStringValue(apiData, "gateway")
	if !gateway.IsNull() && gateway.ValueString() != "0.0.0.0" && gateway.ValueString() != "" {
		data.Gateway = gateway
	} else {
		// BCM returned default/empty - reset to null to reflect actual API state
		data.Gateway = types.StringNull()
	}

	mtu := getInt64Value(apiData, "mtu")
	if !mtu.IsNull() && mtu.ValueInt64() != 1500 {
		// Only set MTU if different from BCM default (1500)
		data.MTU = mtu
	} else {
		// BCM returned default (1500) - reset to null to reflect actual API state
		data.MTU = types.Int64Null()
	}

	// Domain name and notes - reset to null if BCM returns empty
	domainName := getStringValue(apiData, "domainName")
	if !domainName.IsNull() && domainName.ValueString() != "" {
		data.DomainName = domainName
	} else {
		// BCM returned empty - reset to null to reflect actual API state
		data.DomainName = types.StringNull()
	}

	notes := getStringValue(apiData, "notes")
	if !notes.IsNull() && notes.ValueString() != "" {
		data.Notes = notes
	} else {
		// BCM returned empty - reset to null to reflect actual API state
		data.Notes = types.StringNull()
	}

	// Network classification (Optional+Computed)
	data.NetworkType = getStringValue(apiData, "type")
	data.Management = getBoolValue(apiData, "management")
	data.Bootable = getBoolValue(apiData, "bootable")

	// BCM lifecycle fields (Computed)
	data.BaseType = getStringValue(apiData, "baseType")
	data.ChildType = getStringValue(apiData, "childType")
	data.Revision = getStringValue(apiData, "revision")
	data.Modified = getBoolValue(apiData, "modified")
	data.ToBeRemoved = getBoolValue(apiData, "to_be_removed")

	// DNS/DHCP settings (Optional+Computed)
	data.AllowAutosign = getStringValue(apiData, "allowAutosign")
	data.GenerateDNSZone = getStringValue(apiData, "generateDNSZone")
	data.LockDownDhcpd = getBoolValue(apiData, "lockDownDhcpd")
	data.DisableAutomaticExports = getBoolValue(apiData, "disableAutomaticExports")
	data.ExcludeFromSearchDomain = getBoolValue(apiData, "excludeFromSearchDomain")
	data.SearchDomainIndex = getInt64Value(apiData, "searchDomainIndex")

	// IPv6 settings (Optional+Computed)
	data.IPv6Enabled = getBoolValue(apiData, "IPv6")
	data.IPv6BaseAddress = getStringValue(apiData, "ipv6BaseAddress")
	data.IPv6Gateway = getStringValue(apiData, "ipv6Gateway")
	data.IPv6NetmaskBits = getInt64Value(apiData, "ipv6NetmaskBits")

	// Cloud settings (Optional+Computed)
	data.EC2AvailabilityZone = getStringValue(apiData, "EC2AvailabilityZone")
	data.EC2String = getStringValue(apiData, "EC2String")
	data.CloudSubnetID = getStringValue(apiData, "cloudSubnetID")

	// BCM internal (Computed-only, never sent)
	if ev, ok := apiData["extra_values"]; ok && ev != nil {
		if evBytes, err := json.Marshal(ev); err == nil {
			data.ExtraValues = types.StringValue(string(evBytes))
		} else {
			data.ExtraValues = types.StringNull()
		}
	} else {
		data.ExtraValues = types.StringNull()
	}
}

// parseCIDR converts CIDR notation to baseAddress and netmaskBits.
func parseCIDR(cidr string) (baseAddress string, netmaskBits int, err error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid CIDR notation: %w", err)
	}

	baseAddress = ip.String()
	maskBits, _ := ipnet.Mask.Size()
	return baseAddress, maskBits, nil
}

// formatCIDR reconstructs CIDR from baseAddress and netmaskBits.
func formatCIDR(baseAddress string, netmaskBits int64) string {
	return fmt.Sprintf("%s/%d", baseAddress, netmaskBits)
}

// isDHCPEnabled derives DHCP status from range configuration.
func isDHCPEnabled(rangeStart, rangeEnd string) bool {
	return rangeStart != "" && rangeStart != "0.0.0.0" &&
		rangeEnd != "" && rangeEnd != "0.0.0.0"
}

// cidrRegex returns a compiled regex for CIDR notation validation.
func cidrRegex() *regexp.Regexp {
	return regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}/\d{1,2}$`)
}
