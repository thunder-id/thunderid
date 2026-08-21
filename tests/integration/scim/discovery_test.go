// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// SCIMDiscoveryTestSuite covers the endpoints a real SCIM client (an IdP
// provisioning connector) calls before ever creating a resource: capability
// discovery, resource types, and schema introspection. It also pins down the
// two known gaps in this server's SCIM support (no Users PATCH, no ETag-less
// ServiceProviderConfig drift) so a regression here is caught immediately.
type SCIMDiscoveryTestSuite struct {
	suite.Suite
	ouID           string
	entityTypeID   string
	entityTypeName string
}

// TestSCIMDiscoveryTestSuite tests SCIM Discovery Test Suite.
func TestSCIMDiscoveryTestSuite(t *testing.T) {
	suite.Run(t, new(SCIMDiscoveryTestSuite))
}

// scimPaginationMaxPageSize mirrors scimconfig.PaginationMaxPageSize
// (backend/internal/scim/config/config.go). Update here whenever that constant
// changes so the advertisement-to-enforcement assertion remains accurate.
const scimPaginationMaxPageSize = 100

// SetupSuite initializes the test suite environment.
func (ts *SCIMDiscoveryTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "scim-it-discovery-ou",
		Name:        "SCIM Discovery Integration Test OU",
		Description: "Organization unit for SCIM discovery endpoint tests",
	})
	ts.Require().NoError(err, "failed to create test organization unit")
	ts.ouID = ouID

	ts.entityTypeName = "scim-it-discovery-person"
	entityTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: ts.entityTypeName,
		OUID: ouID,
		Schema: map[string]interface{}{
			"email":      map[string]interface{}{"type": "string", "required": true},
			"department": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "failed to create test entity type")
	ts.entityTypeID = entityTypeID
}

// TearDownSuite cleans up the test suite environment.
func (ts *SCIMDiscoveryTestSuite) TearDownSuite() {
	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// TestServiceProviderConfig pins the capability flags a provisioning
// connector reads during setup. Patch is false because only Groups PATCH is
// implemented (Users PATCH is 501) — ServiceProviderConfig reports the
// conservative, accurate value rather than "true because Groups supports it".
// TestServiceProviderConfig tests Service Provider Config.
func (ts *SCIMDiscoveryTestSuite) TestServiceProviderConfig() {
	status, body, err := scimRequest(http.MethodGet, "/ServiceProviderConfig", nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var cfg scimServiceProviderConfig
	ts.Require().NoError(json.Unmarshal(body, &cfg))
	ts.False(cfg.Patch.Supported, "Patch should report unsupported: only Groups PATCH is implemented")
	ts.True(cfg.Filter.Supported, "Filter should report supported: Users list supports single eq")
	ts.True(cfg.ETag.Supported, "ETag should report supported: If-Match/If-None-Match are implemented")
	ts.False(cfg.Pagination.Cursor, "Pagination.cursor should be false: only index-based pagination is implemented")
	ts.True(cfg.Pagination.Index, "Pagination.index should be true: startIndex/count pagination is supported")
	ts.Equal(scimPaginationMaxPageSize, cfg.Pagination.MaxPageSize,
		"Pagination.maxPageSize must equal the PaginationMaxPageSize constant")
}

// TestServiceProviderConfigPaginationClampsCount asserts that requesting a count
// above the advertised pagination.maxPageSize results in the response's
// itemsPerPage being clamped to maxPageSize. This ties the capability
// advertisement to the enforcement in the handler so that any future drift
// between the two constants is caught here.
// TestServiceProviderConfigPaginationClampsCount tests Service Provider Config Pagination Clamps Count.
func (ts *SCIMDiscoveryTestSuite) TestServiceProviderConfigPaginationClampsCount() {
	// First confirm what maxPageSize is advertised.
	status, body, err := scimRequest(http.MethodGet, "/ServiceProviderConfig", nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var cfg scimServiceProviderConfig
	ts.Require().NoError(json.Unmarshal(body, &cfg))
	maxPageSize := cfg.Pagination.MaxPageSize
	ts.Require().Greater(maxPageSize, 0, "maxPageSize must be a positive integer")

	// Request a count strictly greater than the advertised maxPageSize.
	overCount := maxPageSize + 1
	listURL := fmt.Sprintf("/Users?count=%d", overCount)
	status, body, err = scimRequest(http.MethodGet, listURL, nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var listResp scimUserListResponse
	ts.Require().NoError(json.Unmarshal(body, &listResp))
	ts.LessOrEqual(listResp.ItemsPerPage, maxPageSize,
		"itemsPerPage must not exceed the advertised pagination.maxPageSize")
}

// TestResourceTypesListsUsersAndGroups tests Resource Types Lists Users And Groups.
func (ts *SCIMDiscoveryTestSuite) TestResourceTypesListsUsersAndGroups() {
	status, body, err := scimRequest(http.MethodGet, "/ResourceTypes", nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var list scimResourceTypeListResponse
	ts.Require().NoError(json.Unmarshal(body, &list))

	endpoints := make(map[string]bool)
	for _, rt := range list.Resources {
		endpoints[rt.Endpoint] = true
	}
	ts.True(endpoints["/Users"], "ResourceTypes should list the /Users endpoint")
	ts.True(endpoints["/Groups"], "ResourceTypes should list the /Groups endpoint")
}

// TestSchemasListReflectsEntityTypeRequiredness is the discovery-side
// counterpart of the missing-required-attribute fix: the extension schema for
// our test entity type must report "email" as required, dynamically, purely
// from the entity type definition — not from any hardcoded core schema list.
// TestSchemasListReflectsEntityTypeRequiredness tests Schemas List Reflects Entity Type Requiredness.
func (ts *SCIMDiscoveryTestSuite) TestSchemasListReflectsEntityTypeRequiredness() {
	status, body, err := scimRequest(http.MethodGet, "/Schemas", nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var list scimSchemaListResponse
	ts.Require().NoError(json.Unmarshal(body, &list))

	ids := make(map[string]bool)
	var extSchema *scimSchema
	for i := range list.Resources {
		s := list.Resources[i]
		ids[s.ID] = true
		if strings.EqualFold(s.Name, ts.entityTypeName) {
			extSchema = &list.Resources[i]
		}
	}
	ts.True(ids[scimCoreUserSchemaURN], "Schemas list should include the core User schema")
	ts.True(ids[scimCoreGroupSchemaURN], "Schemas list should include the core Group schema")
	ts.Require().NotNil(extSchema, "Schemas list should include our test entity type's extension schema")

	required := map[string]bool{}
	for _, attr := range extSchema.Attributes {
		required[attr.Name] = attr.Required
	}
	ts.True(required["email"], "email should be reported required, matching the entity type definition")
	ts.Contains(required, "department", "department should be present in the extension schema")
	ts.False(required["department"], "department was not declared required in the entity type")
}

// TestSchemaGetByURN tests Schema Get By URN.
func (ts *SCIMDiscoveryTestSuite) TestSchemaGetByURN() {
	urn, _, err := discoverExtensionSchema(ts.entityTypeName)
	ts.Require().NoError(err)

	status, body, err := scimRequest(http.MethodGet, "/Schemas/"+urn, nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var s scimSchema
	ts.Require().NoError(json.Unmarshal(body, &s))
	ts.Equal(urn, s.ID)
	ts.True(strings.EqualFold(s.Name, ts.entityTypeName))
}

// TestSchemaGetUnknownURN tests Schema Get Unknown URN.
func (ts *SCIMDiscoveryTestSuite) TestSchemaGetUnknownURN() {
	status, _, err := scimRequest(http.MethodGet, "/Schemas/urn:thunderid:params:scim:schemas:does-not-exist:2.0:User", nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusNotFound, status)
}

// TestUsersPatchUnsupported documents the real, current contract: unlike
// Groups, Users PATCH is unimplemented and always returns 501, regardless of
// whether the target ID exists. A provisioning connector that defaults to
// PATCH for attribute updates must fall back to PUT against this server.
// TestUsersPatchUnsupported tests Users Patch Unsupported.
func (ts *SCIMDiscoveryTestSuite) TestUsersPatchUnsupported() {
	status, _, err := scimRequest(http.MethodPatch, "/Users/any-id", []byte(`{}`), nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusNotImplemented, status)
}

// TestListUsersNoTokenUnauthorized documents that /Users, unlike
// ServiceProviderConfig/ResourceTypes, requires authentication: no
// Authorization header at all must be rejected before any authz check runs.
// TestListUsersNoTokenUnauthorized tests List Users No Token Unauthorized.
func (ts *SCIMDiscoveryTestSuite) TestListUsersNoTokenUnauthorized() {
	status, _, err := scimRequestUnauthenticated(http.MethodGet, "/Users", nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, status)
}

// TestListUsersInvalidTokenUnauthorized tests List Users Invalid Token Unauthorized.
func (ts *SCIMDiscoveryTestSuite) TestListUsersInvalidTokenUnauthorized() {
	status, _, err := scimRequestUnauthenticated(http.MethodGet, "/Users",
		map[string]string{"Authorization": "Bearer this-is-not-a-real-token"})
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, status)
}

// TestBulkUnimplemented documents that /Bulk carries no explicit permission
// entry in permissions.go and so falls back to the root "system" permission —
// an authenticated admin passes that authz check and reaches
// handleUnsupportedRequest, which always returns 501, regardless of method
// or body.
// TestBulkUnimplemented tests Bulk Unimplemented.
func (ts *SCIMDiscoveryTestSuite) TestBulkUnimplemented() {
	status, _, err := scimRequest(http.MethodPost, "/Bulk", []byte(`{}`), nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusNotImplemented, status)
}

// TestBulkNoTokenUnauthorized is the companion to TestBulkUnimplemented: with
// no token at all, authentication fails before the root-permission fallback
// is ever evaluated, so the response is 401, not 501.
// TestBulkNoTokenUnauthorized tests Bulk No Token Unauthorized.
func (ts *SCIMDiscoveryTestSuite) TestBulkNoTokenUnauthorized() {
	status, _, err := scimRequestUnauthenticated(http.MethodPost, "/Bulk", nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, status)
}

// TestTopLevelSearchUnimplemented is the top-level (non-Users-scoped)
// ".search" endpoint's counterpart to TestBulkUnimplemented — same
// root-permission fallback, same unconditional 501 from
// handleUnsupportedRequest.
// TestTopLevelSearchUnimplemented tests Top Level Search Unimplemented.
func (ts *SCIMDiscoveryTestSuite) TestTopLevelSearchUnimplemented() {
	status, _, err := scimRequest(http.MethodPost, "/.search", []byte(`{}`), nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusNotImplemented, status)
}

// TestTopLevelSearchNoTokenUnauthorized tests Top Level Search No Token Unauthorized.
func (ts *SCIMDiscoveryTestSuite) TestTopLevelSearchNoTokenUnauthorized() {
	status, _, err := scimRequestUnauthenticated(http.MethodPost, "/.search", nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, status)
}
