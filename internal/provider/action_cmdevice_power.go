// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure interface compliance at compile time.
var (
	_ action.Action              = &CMDevicePowerAction{}
	_ action.ActionWithConfigure = &CMDevicePowerAction{}
)

// CMDevicePowerAction implements power operations for BCM devices.
// This action allows Terraform practitioners to execute power operations
// (power_on, power_off, reboot, power_cycle) on BCM-managed devices.
type CMDevicePowerAction struct {
	client *BCMClient
}

// CMDevicePowerActionModel describes the action configuration from HCL.
type CMDevicePowerActionModel struct {
	DeviceID          types.String `tfsdk:"device_id"`
	PowerAction       types.String `tfsdk:"power_action"`
	Force             types.Bool   `tfsdk:"force"`
	WaitForCompletion types.Bool   `tfsdk:"wait_for_completion"`
	Timeout           types.String `tfsdk:"timeout"`
}

// powerOperationMapping maps Terraform power_action values to BCM PowerOperation operation strings.
// BCM uses cmdevice.powerOperation with a structured args payload, not individual methods.
var powerOperationMapping = map[string]string{
	"power_on":    "ON",
	"power_off":   "OFF",
	"reset":       "RESET",
	"power_cycle": "CYCLE",
}

// powerOperationPayload matches BCM's PowerOperation request structure.
type powerOperationPayload struct {
	BaseType  string   `json:"baseType"`
	Devices   []string `json:"devices"`
	Operation string   `json:"operation"`
	Force     bool     `json:"force"`
	Wait      bool     `json:"wait"`
}

// NewCMDevicePowerAction creates a new action instance.
// This function is registered with the provider via the Actions method.
func NewCMDevicePowerAction() action.Action {
	return &CMDevicePowerAction{}
}

// Metadata returns the action type name.
// The full type name will be "bcm_cmdevice_power".
func (a *CMDevicePowerAction) Metadata(ctx context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cmdevice_power"
}

// Schema defines the action schema with input attributes.
// Actions do not have computed or state attributes - they only define inputs.
func (a *CMDevicePowerAction) Schema(ctx context.Context, req action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Execute power operations on BCM devices via `cmdevice.powerOperation`.\n\n" +
			"This action supports power on, power off, reset, and power cycle operations " +
			"via BMC/IPMI. It can be invoked directly or triggered via resource lifecycle events.\n\n" +
			"**Requires Terraform 1.14 or later.**",

		Attributes: map[string]schema.Attribute{
			"device_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "BCM device UUID. Use `bcm_cmdevice_device.name.uuid` for managed devices.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"power_action": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Power operation to execute (maps to BCM `PowerOperation` operation):\n" +
					"  - `power_on`: Power on device via BMC (ON)\n" +
					"  - `power_off`: Power off device via BMC (OFF)\n" +
					"  - `reset`: Graceful device reset via BMC (RESET)\n" +
					"  - `power_cycle`: Hard power cycle off/on (CYCLE)",
				Validators: []validator.String{
					stringvalidator.OneOf("power_on", "power_off", "reset", "power_cycle"),
				},
			},
			"force": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Force the power operation. Default: `true`.",
			},
			"wait_for_completion": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Wait for power state change to complete before returning. Default: `false`.",
			},
			"timeout": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Timeout duration when `wait_for_completion` is enabled. Uses Go duration format (e.g., `5m`, `30s`). Default: `5m`. Range: 10s-30m.",
			},
		},
	}
}

// Configure receives the provider-configured BCM client.
// This method is called by the framework after the provider's Configure method.
func (a *CMDevicePowerAction) Configure(ctx context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*BCMClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Action Configure Type",
			fmt.Sprintf("Expected *BCMClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	a.client = client
}

// Invoke executes the power operation on the specified device.
// This is the main entry point for action execution.
func (a *CMDevicePowerAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	// Verify client was configured - prevents nil pointer dereference
	if a.client == nil {
		resp.Diagnostics.AddError(
			"Action Not Configured",
			"The BCM client was not configured. Please ensure the provider block is properly configured before using this action.",
		)
		return
	}

	var config CMDevicePowerActionModel

	// Read configuration from request
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Extract values from config
	deviceID := config.DeviceID.ValueString()
	powerAction := config.PowerAction.ValueString()

	// Map Terraform power_action to BCM PowerOperation operation string
	bcmOperation, ok := powerOperationMapping[powerAction]
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid Power Action",
			fmt.Sprintf("Unknown power action: %s. Valid values are: power_on, power_off, reset, power_cycle.", powerAction),
		)
		return
	}

	// Resolve force (default true)
	force := true
	if !config.Force.IsNull() {
		force = config.Force.ValueBool()
	}

	// Resolve wait_for_completion
	wait := false
	if !config.WaitForCompletion.IsNull() {
		wait = config.WaitForCompletion.ValueBool()
	}

	// SAFETY CHECK: Look up the device via getNode to verify it exists,
	// block head-node operations, and resolve hostname → UUID.
	// powerOperation requires UUIDs in the devices array.
	resp.SendProgress(action.InvokeProgressEvent{
		Message: fmt.Sprintf("Looking up device %s...", deviceID),
	})

	nodeData, err := a.client.CallJSONRPC(ctx, "cmdevice", "getNode", deviceID)
	if err != nil {
		tflog.Error(ctx, "POWER ACTION: getNode lookup failed — aborting", map[string]interface{}{
			"device_id": deviceID,
			"error":     err.Error(),
		})
		resp.Diagnostics.AddError(
			"Device Lookup Failed",
			fmt.Sprintf("Could not find device %q via cmdevice.getNode: %s\n\n"+
				"The power operation has been aborted to prevent unintended behavior.\n"+
				"Verify that device_id is a valid BCM device UUID or hostname.",
				deviceID, err.Error()),
		)
		return
	}

	var node map[string]interface{}
	if err := json.Unmarshal(nodeData, &node); err != nil {
		resp.Diagnostics.AddError(
			"Device Lookup Failed",
			fmt.Sprintf("Could not parse getNode response for device %q: %s\n\n"+
				"The power operation has been aborted to prevent unintended behavior.",
				deviceID, err.Error()),
		)
		return
	}

	childType, _ := node["childType"].(string)
	hostname, _ := node["hostname"].(string)
	nodeUUID, _ := node["uuid"].(string)

	tflog.Info(ctx, "POWER ACTION: device identity from BCM", map[string]interface{}{
		"device_id_sent": deviceID,
		"bcm_hostname":  hostname,
		"bcm_uuid":      nodeUUID,
		"bcm_child_type": childType,
	})

	if childType == "HeadNode" {
		resp.Diagnostics.AddError(
			"Safety Check Failed",
			fmt.Sprintf("Cannot execute power operations on HeadNode devices.\n\n"+
				"Device: %s (UUID: %s)\n"+
				"Type: %s\n\n"+
				"Head nodes manage the BCM cluster and should not be power cycled. "+
				"This operation has been blocked to prevent cluster disruption.",
				hostname, nodeUUID, childType),
		)
		return
	}

	if nodeUUID == "" {
		resp.Diagnostics.AddError(
			"Device UUID Not Found",
			fmt.Sprintf("getNode for %q returned no UUID. Cannot call powerOperation without a device UUID.", deviceID),
		)
		return
	}

	// Build the PowerOperation payload
	payload := powerOperationPayload{
		BaseType:  "PowerOperation",
		Devices:   []string{nodeUUID},
		Operation: bcmOperation,
		Force:     force,
		Wait:      wait,
	}

	payloadJSON, _ := json.Marshal(payload)
	tflog.Info(ctx, "POWER ACTION: powerOperation payload", map[string]interface{}{
		"json_body":      string(payloadJSON),
		"device_id_sent": deviceID,
		"resolved_uuid":  nodeUUID,
		"operation":      bcmOperation,
		"bcm_endpoint":   a.client.Endpoint + "/json",
	})

	resp.SendProgress(action.InvokeProgressEvent{
		Message: fmt.Sprintf("Executing %s (%s) on %s (UUID: %s)...", powerAction, bcmOperation, hostname, nodeUUID),
	})

	// Execute via cmdevice.powerOperation with args array
	_, err = a.client.CallJSONRPC(ctx, "cmdevice", "powerOperation", payload)
	if err != nil {
		resp.Diagnostics.AddError(
			"Power Operation Failed",
			fmt.Sprintf("Failed to execute %s on device %s (UUID: %s): %s\n\n"+
				"Note: Power operations require:\n"+
				"1. BMC/IPMI configured on the target device (powerControl != 'none')\n"+
				"2. BCM head node able to reach the device's BMC\n"+
				"3. Correct device UUID",
				powerAction, hostname, nodeUUID, err.Error()),
		)
		return
	}

	resp.SendProgress(action.InvokeProgressEvent{
		Message: fmt.Sprintf("Power operation '%s' completed successfully on %s (UUID: %s)", powerAction, hostname, nodeUUID),
	})

	tflog.Info(ctx, "Power operation completed", map[string]interface{}{
		"device_id":    deviceID,
		"hostname":     hostname,
		"uuid":         nodeUUID,
		"power_action": powerAction,
		"operation":    bcmOperation,
	})
}
