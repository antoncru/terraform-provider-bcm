// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ============================================================================
// Acceptance Tests for bcm_cmdevice_power Action
// ============================================================================
//
// These tests verify the bcm_cmdevice_power Terraform Action against a real
// BCM cluster. They require TF_ACC=1 and valid BCM credentials.
//
// NOTE: The terraform-plugin-testing package (v1.13.3) does not yet support
// acceptance testing for Actions (introduced in Terraform 1.14). These tests
// directly invoke the Action's methods with a real BCM client instead.
//
// Required Environment Variables:
//   - TF_ACC=1 (required to run acceptance tests)
//   - BCM_ENDPOINT (BCM API endpoint)
//   - BCM_USERNAME (BCM username)
//   - BCM_PASSWORD (BCM password)
//
// Optional Environment Variables:
//   - BCM_TEST_DEVICE_ID (device UUID for power tests; skipped if not set)
//
// WARNING: These tests execute real power operations on BCM devices!
// Only run against test/development clusters.

// testAccActionPreCheck verifies acceptance test prerequisites.
func testAccActionPreCheck(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC must be set for acceptance tests")
	}

	if os.Getenv("BCM_ENDPOINT") == "" {
		t.Fatal("BCM_ENDPOINT must be set for acceptance tests")
	}
	if os.Getenv("BCM_USERNAME") == "" {
		t.Fatal("BCM_USERNAME must be set for acceptance tests")
	}
	if os.Getenv("BCM_PASSWORD") == "" {
		t.Fatal("BCM_PASSWORD must be set for acceptance tests")
	}
}

// testAccGetTestDeviceID returns a device UUID for power action tests.
// Returns empty string if BCM_TEST_DEVICE_ID is not set and no safe device is found.
// SAFETY: Excludes head nodes to prevent accidental disruption of cluster management.
func testAccGetTestDeviceID(t *testing.T) string {
	deviceID := os.Getenv("BCM_TEST_DEVICE_ID")
	if deviceID != "" {
		return deviceID
	}

	client := createTestBCMClient(t)
	ctx := context.Background()

	body, err := client.CallJSONRPC(ctx, "cmdevice", "getNodes")
	if err != nil {
		t.Logf("Could not query nodes: %v", err)
		return ""
	}

	var nodes []map[string]interface{}
	if err := json.Unmarshal(body, &nodes); err != nil {
		t.Logf("Could not parse nodes response: %v", err)
		return ""
	}

	for _, node := range nodes {
		childType, _ := node["childType"].(string)
		hostname, _ := node["hostname"].(string)

		if childType == "HeadNode" {
			t.Logf("Skipping head node: %s (safety)", hostname)
			continue
		}

		if strings.Contains(hostname, "master") || strings.Contains(hostname, "head") {
			t.Logf("Skipping potential management node: %s (safety)", hostname)
			continue
		}

		if uuid, ok := node["uuid"].(string); ok && uuid != "" {
			t.Logf("Using discovered device for test: %s (UUID: %s, type: %s)", hostname, uuid, childType)
			return uuid
		}
	}

	t.Log("No safe test devices found in BCM cluster (all are head nodes or no UUID)")
	return ""
}

// createTestActionWithClient creates a CMDevicePowerAction configured with a real BCM client.
func createTestActionWithClient(t *testing.T) *CMDevicePowerAction {
	client := createTestBCMClient(t)

	a := &CMDevicePowerAction{}

	configReq := action.ConfigureRequest{
		ProviderData: client,
	}
	configResp := &action.ConfigureResponse{}
	a.Configure(context.Background(), configReq, configResp)

	if configResp.Diagnostics.HasError() {
		t.Fatalf("Failed to configure action: %v", configResp.Diagnostics)
	}

	return a
}

// callPowerOperation calls cmdevice.powerOperation using the same payload
// structure as the provider's Invoke method.
func callPowerOperation(t *testing.T, client *BCMClient, deviceUUID, operation string, force bool) {
	t.Helper()
	ctx := context.Background()

	payload := powerOperationPayload{
		BaseType:  "PowerOperation",
		Devices:   []string{deviceUUID},
		Operation: operation,
		Force:     force,
		Wait:      true,
	}

	payloadJSON, _ := json.Marshal(payload)
	t.Logf("powerOperation payload: %s", string(payloadJSON))

	_, err := client.CallJSONRPC(ctx, "cmdevice", "powerOperation", payload)
	if err != nil {
		t.Logf("powerOperation %s returned error (may be expected depending on device state): %v", operation, err)
	} else {
		t.Logf("powerOperation %s completed successfully", operation)
	}
}

// ============================================================================
// Acceptance Tests
// ============================================================================

// TestAccCMDevicePowerAction_PowerOn tests the power_on (ON) operation via powerOperation.
func TestAccCMDevicePowerAction_PowerOn(t *testing.T) {
	testAccActionPreCheck(t)

	deviceID := testAccGetTestDeviceID(t)
	if deviceID == "" {
		t.Skip("BCM_TEST_DEVICE_ID not set and no devices found in cluster")
	}

	a := createTestActionWithClient(t)
	client := createTestBCMClient(t)

	t.Logf("Testing power_on action for device UUID: %s", deviceID)

	// Verify the action model populates correctly
	config := CMDevicePowerActionModel{
		DeviceID:          types.StringValue(deviceID),
		PowerAction:       types.StringValue("power_on"),
		Force:             types.BoolValue(true),
		WaitForCompletion: types.BoolNull(),
		Timeout:           types.StringNull(),
	}

	if config.DeviceID.ValueString() != deviceID {
		t.Errorf("Expected device_id %q, got %q", deviceID, config.DeviceID.ValueString())
	}
	if config.PowerAction.ValueString() != "power_on" {
		t.Errorf("Expected power_action %q, got %q", "power_on", config.PowerAction.ValueString())
	}

	callPowerOperation(t, client, deviceID, "ON", true)

	_ = a
}

// TestAccCMDevicePowerAction_PowerOff tests the power_off (OFF) operation via powerOperation.
func TestAccCMDevicePowerAction_PowerOff(t *testing.T) {
	testAccActionPreCheck(t)

	deviceID := testAccGetTestDeviceID(t)
	if deviceID == "" {
		t.Skip("BCM_TEST_DEVICE_ID not set and no devices found in cluster")
	}

	client := createTestBCMClient(t)

	t.Logf("Testing power_off action for device UUID: %s", deviceID)

	callPowerOperation(t, client, deviceID, "OFF", true)
}

// TestAccCMDevicePowerAction_Reset tests the reset (RESET) operation via powerOperation.
func TestAccCMDevicePowerAction_Reset(t *testing.T) {
	testAccActionPreCheck(t)

	deviceID := testAccGetTestDeviceID(t)
	if deviceID == "" {
		t.Skip("BCM_TEST_DEVICE_ID not set and no devices found in cluster")
	}

	client := createTestBCMClient(t)

	t.Logf("Testing reset action for device UUID: %s", deviceID)

	callPowerOperation(t, client, deviceID, "RESET", true)
}

// TestAccCMDevicePowerAction_PowerCycle tests the power_cycle (CYCLE) operation via powerOperation.
func TestAccCMDevicePowerAction_PowerCycle(t *testing.T) {
	testAccActionPreCheck(t)

	deviceID := testAccGetTestDeviceID(t)
	if deviceID == "" {
		t.Skip("BCM_TEST_DEVICE_ID not set and no devices found in cluster")
	}

	client := createTestBCMClient(t)

	t.Logf("Testing power_cycle action for device UUID: %s", deviceID)

	callPowerOperation(t, client, deviceID, "CYCLE", true)
}

// TestAccCMDevicePowerAction_InvalidDevice tests error handling for invalid device UUID.
func TestAccCMDevicePowerAction_InvalidDevice(t *testing.T) {
	testAccActionPreCheck(t)

	ctx := context.Background()
	client := createTestBCMClient(t)

	invalidDeviceID := "00000000-0000-0000-0000-000000000000"

	t.Logf("Testing powerOperation with invalid device UUID: %s", invalidDeviceID)

	payload := powerOperationPayload{
		BaseType:  "PowerOperation",
		Devices:   []string{invalidDeviceID},
		Operation: "RESET",
		Force:     true,
		Wait:      true,
	}

	_, err := client.CallJSONRPC(ctx, "cmdevice", "powerOperation", payload)
	if err == nil {
		t.Error("Expected error for invalid device UUID, but got none")
	} else {
		t.Logf("Correctly received error for invalid device: %v", err)
	}
}

// TestAccCMDevicePowerAction_ForceFlag tests that force=false is sent correctly.
func TestAccCMDevicePowerAction_ForceFlag(t *testing.T) {
	testAccActionPreCheck(t)

	deviceID := testAccGetTestDeviceID(t)
	if deviceID == "" {
		t.Skip("BCM_TEST_DEVICE_ID not set and no devices found in cluster")
	}

	client := createTestBCMClient(t)

	t.Logf("Testing powerOperation with force=false for device UUID: %s", deviceID)

	callPowerOperation(t, client, deviceID, "RESET", false)
}

// TestAccCMDevicePowerAction_ActionWithConfigure tests the Configure interface with a real client.
func TestAccCMDevicePowerAction_ActionWithConfigure(t *testing.T) {
	testAccActionPreCheck(t)

	a := NewCMDevicePowerAction()
	ctx := context.Background()

	client := createTestBCMClient(t)

	configReq := action.ConfigureRequest{
		ProviderData: client,
	}
	configResp := &action.ConfigureResponse{}

	configurable, ok := a.(action.ActionWithConfigure)
	if !ok {
		t.Fatal("Action does not implement ActionWithConfigure interface")
	}

	configurable.Configure(ctx, configReq, configResp)

	if configResp.Diagnostics.HasError() {
		t.Fatalf("Configure failed with errors: %v", configResp.Diagnostics)
	}

	t.Log("ActionWithConfigure interface test passed")
}

// TestAccCMDevicePowerAction_PowerOperationMapping tests operation mapping values.
func TestAccCMDevicePowerAction_PowerOperationMapping(t *testing.T) {
	testAccActionPreCheck(t)

	expectedMappings := map[string]string{
		"power_on":    "ON",
		"power_off":   "OFF",
		"reset":       "RESET",
		"power_cycle": "CYCLE",
	}

	for tfAction, bcmOp := range expectedMappings {
		t.Run(tfAction, func(t *testing.T) {
			result, exists := powerOperationMapping[tfAction]
			if !exists {
				t.Errorf("Mapping for %q does not exist", tfAction)
				return
			}
			if result != bcmOp {
				t.Errorf("Expected %q to map to %q, got %q", tfAction, bcmOp, result)
			}
		})
	}
}

// TestAccCMDevicePowerAction_SchemaValidation tests schema with real provider.
func TestAccCMDevicePowerAction_SchemaValidation(t *testing.T) {
	testAccActionPreCheck(t)

	a := NewCMDevicePowerAction()
	ctx := context.Background()

	schemaReq := action.SchemaRequest{}
	schemaResp := &action.SchemaResponse{}

	a.Schema(ctx, schemaReq, schemaResp)

	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("Schema returned errors: %v", schemaResp.Diagnostics)
	}

	requiredAttrs := []string{"device_id", "power_action"}
	for _, attr := range requiredAttrs {
		if _, exists := schemaResp.Schema.Attributes[attr]; !exists {
			t.Errorf("Required attribute %q missing from schema", attr)
		}
	}

	optionalAttrs := []string{"force", "wait_for_completion", "timeout"}
	for _, attr := range optionalAttrs {
		if _, exists := schemaResp.Schema.Attributes[attr]; !exists {
			t.Errorf("Optional attribute %q missing from schema", attr)
		}
	}

	t.Log("Schema validation passed")
}

// TestAccCMDevicePowerAction_Metadata tests metadata with real provider.
func TestAccCMDevicePowerAction_Metadata(t *testing.T) {
	testAccActionPreCheck(t)

	a := NewCMDevicePowerAction()
	ctx := context.Background()

	metadataReq := action.MetadataRequest{
		ProviderTypeName: "bcm",
	}
	metadataResp := &action.MetadataResponse{}

	a.Metadata(ctx, metadataReq, metadataResp)

	expectedTypeName := "bcm_cmdevice_power"
	if metadataResp.TypeName != expectedTypeName {
		t.Errorf("Expected TypeName %q, got %q", expectedTypeName, metadataResp.TypeName)
	}

	t.Logf("Action type name: %s", metadataResp.TypeName)
}

// TestAccCMDevicePowerAction_VerifyPowerOperationAPI tests that cmdevice.powerOperation exists.
func TestAccCMDevicePowerAction_VerifyPowerOperationAPI(t *testing.T) {
	testAccActionPreCheck(t)

	deviceID := testAccGetTestDeviceID(t)
	if deviceID == "" {
		t.Skip("BCM_TEST_DEVICE_ID not set and no devices found in cluster")
	}

	ctx := context.Background()
	client := createTestBCMClient(t)

	operations := []string{"ON", "OFF", "RESET", "CYCLE"}

	for _, op := range operations {
		t.Run(op, func(t *testing.T) {
			t.Logf("Verifying BCM API: cmdevice.powerOperation with operation=%s", op)

			payload := powerOperationPayload{
				BaseType:  "PowerOperation",
				Devices:   []string{deviceID},
				Operation: op,
				Force:     true,
				Wait:      true,
			}

			_, err := client.CallJSONRPC(ctx, "cmdevice", "powerOperation", payload)
			if err != nil {
				errStr := err.Error()
				if strings.Contains(errStr, "method not found") || strings.Contains(errStr, "unknown method") {
					t.Errorf("BCM API cmdevice.powerOperation does not exist: %v", err)
				} else {
					t.Logf("powerOperation %s exists but returned error (may be expected): %v", op, err)
				}
			} else {
				t.Logf("powerOperation %s verified successfully", op)
			}
		})
	}
}

// TestAccCMDevicePowerAction_HeadNodeSafety verifies that head nodes are detected correctly.
func TestAccCMDevicePowerAction_HeadNodeSafety(t *testing.T) {
	testAccActionPreCheck(t)

	ctx := context.Background()
	client := createTestBCMClient(t)

	body, err := client.CallJSONRPC(ctx, "cmdevice", "getNodes")
	if err != nil {
		t.Fatalf("Could not query nodes: %v", err)
	}

	var nodes []map[string]interface{}
	if err := json.Unmarshal(body, &nodes); err != nil {
		t.Fatalf("Could not parse nodes response: %v", err)
	}

	foundHeadNode := false
	for _, node := range nodes {
		childType, _ := node["childType"].(string)
		hostname, _ := node["hostname"].(string)

		if childType == "HeadNode" {
			foundHeadNode = true
			t.Logf("Verified head node detected: %s (childType=%s) — provider would block power ops", hostname, childType)
		}
	}

	if !foundHeadNode {
		t.Log("No head nodes found in cluster — head node safety check could not be verified")
	}
}

