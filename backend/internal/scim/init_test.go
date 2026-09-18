// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
)

// TestInitialize_RegistersDiscoveryRoutes tests Initialize for Registers Discovery Routes.
func TestInitialize_RegistersDiscoveryRoutes(t *testing.T) {
	mux := http.NewServeMux()
	Initialize(mux, nil, nil, nil, scimconfig.SCIMConfig{})
	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/scim/v2/ServiceProviderConfig"},
		{method: http.MethodGet, path: "/scim/v2/Schemas"},
		{method: http.MethodGet, path: "/scim/v2/ResourceTypes"},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		_, pattern := mux.Handler(req)
		if pattern == "" {
			t.Fatalf("expected route to be registered for %s %s", tc.method, tc.path)
		}
	}

	// Verify OPTIONS endpoints returning 204 No Content
	optionsPaths := []string{
		"/scim/v2/ServiceProviderConfig",
		"/scim/v2/Schemas",
		"/scim/v2/Schemas/some-schema-urn",
		"/scim/v2/ResourceTypes",
		"/scim/v2/ResourceTypes/some-resource-id",
		"/scim/v2/Users",
		"/scim/v2/Users/some-user-id",
		"/scim/v2/Groups",
		"/scim/v2/Groups/some-group-id",
	}
	for _, path := range optionsPaths {
		req := httptest.NewRequest(http.MethodOptions, path, nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, http.StatusNoContent, rr.Code, "expected 204 for OPTIONS on path: %s", path)
	}
}
