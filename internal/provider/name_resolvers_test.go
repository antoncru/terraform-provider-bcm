// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// ========================================
// Name Resolver Unit Tests
// ========================================

func newMockBCMClient(serverURL string) *BCMClient {
	jar, _ := cookiejar.New(nil)
	return &BCMClient{
		HTTPClient: &http.Client{
			Jar: jar,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		},
		Endpoint:   serverURL,
		MaxRetries: 0,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   500 * time.Millisecond,
	}
}

func createNameResolverMockServer(responses map[string]interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Service string        `json:"service"`
			Call    string        `json:"call"`
			Args    []interface{} `json:"args,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if req.Service == "login" {
			w.Header().Set("Set-Cookie", "cm-login-token=test-token; Path=/")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(true)
			return
		}

		key := fmt.Sprintf("%s.%s", req.Service, req.Call)
		resp, ok := responses[key]
		if !ok {
			http.Error(w, fmt.Sprintf("unexpected call: %s", key), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestResolveNetworkUUID_Success(t *testing.T) {
	server := createNameResolverMockServer(map[string]interface{}{
		"cmnet.getNetwork": map[string]interface{}{
			"uuid": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			"name": "internalnet",
		},
	})
	defer server.Close()

	client := newMockBCMClient(server.URL)
	uuid, err := resolveNetworkUUID(context.Background(), client, "internalnet")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if uuid != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Fatalf("expected UUID aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee, got: %s", uuid)
	}
}

func TestResolveNetworkUUID_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Service string `json:"service"`
			Call    string `json:"call"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req.Service == "login" {
			w.Header().Set("Set-Cookie", "cm-login-token=test-token; Path=/")
			_ = json.NewEncoder(w).Encode(true)
			return
		}

		http.Error(w, "Network not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := newMockBCMClient(server.URL)
	_, err := resolveNetworkUUID(context.Background(), client, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent network, got nil")
	}
}

func TestResolveCategoryUUID_Success(t *testing.T) {
	server := createNameResolverMockServer(map[string]interface{}{
		"cmdevice.getCategory": map[string]interface{}{
			"uuid": "11111111-2222-3333-4444-555555555555",
			"name": "default",
		},
	})
	defer server.Close()

	client := newMockBCMClient(server.URL)
	uuid, err := resolveCategoryUUID(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if uuid != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("expected UUID 11111111-2222-3333-4444-555555555555, got: %s", uuid)
	}
}

func TestResolveCategoryUUID_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Service string `json:"service"`
			Call    string `json:"call"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req.Service == "login" {
			w.Header().Set("Set-Cookie", "cm-login-token=test-token; Path=/")
			_ = json.NewEncoder(w).Encode(true)
			return
		}

		http.Error(w, "Category not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := newMockBCMClient(server.URL)
	_, err := resolveCategoryUUID(context.Background(), client, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent category, got nil")
	}
}

func TestResolvePartitionUUID_Success(t *testing.T) {
	server := createNameResolverMockServer(map[string]interface{}{
		"cmpart.getPartition": map[string]interface{}{
			"uuid": "99999999-8888-7777-6666-555555555555",
			"name": "default",
		},
	})
	defer server.Close()

	client := newMockBCMClient(server.URL)
	uuid, err := resolvePartitionUUID(context.Background(), client, "default")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if uuid != "99999999-8888-7777-6666-555555555555" {
		t.Fatalf("expected UUID 99999999-8888-7777-6666-555555555555, got: %s", uuid)
	}
}

func TestResolvePartitionUUID_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Service string `json:"service"`
			Call    string `json:"call"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req.Service == "login" {
			w.Header().Set("Set-Cookie", "cm-login-token=test-token; Path=/")
			_ = json.NewEncoder(w).Encode(true)
			return
		}

		http.Error(w, "Partition not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := newMockBCMClient(server.URL)
	_, err := resolvePartitionUUID(context.Background(), client, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent partition, got nil")
	}
}

func TestResolveNetworkUUID_EmptyUUID(t *testing.T) {
	server := createNameResolverMockServer(map[string]interface{}{
		"cmnet.getNetwork": map[string]interface{}{
			"name": "broken-net",
			"uuid": "",
		},
	})
	defer server.Close()

	client := newMockBCMClient(server.URL)
	_, err := resolveNetworkUUID(context.Background(), client, "broken-net")
	if err == nil {
		t.Fatal("expected error for empty UUID, got nil")
	}
}

// ========================================
// Schema Validation Tests (XOR constraints)
// ========================================

func TestAccCMDeviceDevice_ValidationBothCategoryAndCategoryName(t *testing.T) {
	cleanup := clearBCMEnvVars()
	defer cleanup()

	mockServer := createNameResolverMockServer(map[string]interface{}{})
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "bcm" {
  endpoint             = %[1]q
  username             = "mock-user"
  password             = "mock-pass"
  insecure_skip_verify = true
}

resource "bcm_cmdevice_device" "test" {
  hostname              = "test-both-cat"
  category              = "12345678-1234-1234-1234-123456789012"
  category_name         = "default"
  management_network    = "12345678-1234-1234-1234-123456789012"

  interfaces {
    name    = "eth0"
    type    = "physical"
    network = "12345678-1234-1234-1234-123456789012"
  }
}
`, mockServer.URL),
				ExpectError: regexp.MustCompile(`(?i)cannot be specified when`),
			},
		},
	})
}

func TestAccCMDeviceDevice_ValidationBothNetworkAndNetworkName(t *testing.T) {
	cleanup := clearBCMEnvVars()
	defer cleanup()

	mockServer := createNameResolverMockServer(map[string]interface{}{})
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "bcm" {
  endpoint             = %[1]q
  username             = "mock-user"
  password             = "mock-pass"
  insecure_skip_verify = true
}

resource "bcm_cmdevice_device" "test" {
  hostname              = "test-both-net"
  category              = "12345678-1234-1234-1234-123456789012"
  management_network    = "12345678-1234-1234-1234-123456789012"

  interfaces {
    name         = "eth0"
    type         = "physical"
    network      = "12345678-1234-1234-1234-123456789012"
    network_name = "internalnet"
  }
}
`, mockServer.URL),
				ExpectError: regexp.MustCompile(`(?i)cannot be specified when`),
			},
		},
	})
}

func TestAccCMDeviceDevice_ValidationNeitherCategoryNorCategoryName(t *testing.T) {
	cleanup := clearBCMEnvVars()
	defer cleanup()

	mockServer := createNameResolverMockServer(map[string]interface{}{})
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "bcm" {
  endpoint             = %[1]q
  username             = "mock-user"
  password             = "mock-pass"
  insecure_skip_verify = true
}

resource "bcm_cmdevice_device" "test" {
  hostname              = "test-no-cat"
  management_network    = "12345678-1234-1234-1234-123456789012"

  interfaces {
    name    = "eth0"
    type    = "physical"
    network = "12345678-1234-1234-1234-123456789012"
  }
}
`, mockServer.URL),
				ExpectError: regexp.MustCompile(`(?i)at least one`),
			},
		},
	})
}

func TestAccCMDeviceDevice_ValidationNeitherNetworkNorNetworkName(t *testing.T) {
	cleanup := clearBCMEnvVars()
	defer cleanup()

	mockServer := createNameResolverMockServer(map[string]interface{}{})
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "bcm" {
  endpoint             = %[1]q
  username             = "mock-user"
  password             = "mock-pass"
  insecure_skip_verify = true
}

resource "bcm_cmdevice_device" "test" {
  hostname              = "test-no-net"
  category              = "12345678-1234-1234-1234-123456789012"
  management_network    = "12345678-1234-1234-1234-123456789012"

  interfaces {
    name = "eth0"
    type = "physical"
  }
}
`, mockServer.URL),
				ExpectError: regexp.MustCompile(`(?i)at least one`),
			},
		},
	})
}

func TestAccCMDeviceDevice_ValidationBothMgmtNetworkAndMgmtNetworkName(t *testing.T) {
	cleanup := clearBCMEnvVars()
	defer cleanup()

	mockServer := createNameResolverMockServer(map[string]interface{}{})
	defer mockServer.Close()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "bcm" {
  endpoint             = %[1]q
  username             = "mock-user"
  password             = "mock-pass"
  insecure_skip_verify = true
}

resource "bcm_cmdevice_device" "test" {
  hostname                = "test-both-mgmt"
  category                = "12345678-1234-1234-1234-123456789012"
  management_network      = "12345678-1234-1234-1234-123456789012"
  management_network_name = "internalnet"

  interfaces {
    name    = "eth0"
    type    = "physical"
    network = "12345678-1234-1234-1234-123456789012"
  }
}
`, mockServer.URL),
				ExpectError: regexp.MustCompile(`(?i)cannot be specified when`),
			},
		},
	})
}
