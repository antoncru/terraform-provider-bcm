// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
)

// resolveNetworkUUID resolves a BCM network name to its UUID via cmnet.getNetwork.
func resolveNetworkUUID(ctx context.Context, client *BCMClient, name string) (string, error) {
	body, err := client.CallJSONRPC(ctx, "cmnet", "getNetwork", name)
	if err != nil {
		return "", fmt.Errorf("failed to look up network %q: %w", name, err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to parse network response for %q: %w", name, err)
	}

	uuid, ok := data["uuid"].(string)
	if !ok || uuid == "" {
		return "", fmt.Errorf("network %q exists but has no UUID in BCM response", name)
	}

	return uuid, nil
}

// resolveCategoryUUID resolves a BCM category name to its UUID via cmdevice.getCategory.
func resolveCategoryUUID(ctx context.Context, client *BCMClient, name string) (string, error) {
	body, err := client.CallJSONRPC(ctx, "cmdevice", "getCategory", name)
	if err != nil {
		return "", fmt.Errorf("failed to look up category %q: %w", name, err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to parse category response for %q: %w", name, err)
	}

	uuid, ok := data["uuid"].(string)
	if !ok || uuid == "" {
		return "", fmt.Errorf("category %q exists but has no UUID in BCM response", name)
	}

	return uuid, nil
}

// resolvePartitionUUID resolves a BCM partition name to its UUID via cmpart.getPartition.
func resolvePartitionUUID(ctx context.Context, client *BCMClient, name string) (string, error) {
	body, err := client.CallJSONRPC(ctx, "cmpart", "getPartition", name)
	if err != nil {
		return "", fmt.Errorf("failed to look up partition %q: %w", name, err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to parse partition response for %q: %w", name, err)
	}

	uuid, ok := data["uuid"].(string)
	if !ok || uuid == "" {
		return "", fmt.Errorf("partition %q exists but has no UUID in BCM response", name)
	}

	return uuid, nil
}
