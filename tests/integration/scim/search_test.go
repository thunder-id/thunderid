// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// SCIMSearchTestSuite exercises POST /Users/.search — the RFC 7644 §3.4.3
// alternative to GET /Users?filter= for filter expressions too long for a
// query string. It shares the same filter grammar (parseSCIMFilterForEq) as
// GET /Users?filter=, so this suite is deliberately narrower than
// SCIMFilterTestSuite: one case per distinguishing feature of .search itself
// (envelope schema requirement, request-body pagination, wrong Content-Type,
// empty body) rather than re-proving the filter grammar end to end.
type SCIMSearchTestSuite struct {
	suite.Suite
	ouID           string
	entityTypeID   string
	entityTypeName string
	extensionURN   string
	createdUserIDs []string

	engineeringUserID string // department=search-Engineering, address.locality=search-Colombo
	salesUserID       string // department=search-Sales, address.locality=search-Kandy
}

func TestSCIMSearchTestSuite(t *testing.T) {
	suite.Run(t, new(SCIMSearchTestSuite))
}

// SetupSuite initializes the test suite environment.
func (ts *SCIMSearchTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "scim-it-search-ou",
		Name:        "SCIM Search Integration Test OU",
		Description: "Organization unit for SCIM /Users/.search tests",
	})
	ts.Require().NoError(err, "failed to create test organization unit")
	ts.ouID = ouID

	ts.entityTypeName = "scim-it-search-person"
	entityTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      ts.entityTypeName,
		DisplayName: "Entity Type",
		OUID:        ouID,
		Schema: map[string]interface{}{
			"email":      map[string]interface{}{"type": "string", "required": true, "unique": true},
			"department": map[string]interface{}{"type": "string"},
			"address": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"locality": map[string]interface{}{"type": "string"},
				},
			},
		},
	})
	ts.Require().NoError(err, "failed to create test entity type")
	ts.entityTypeID = entityTypeID

	urn, _, err := discoverExtensionSchema(ts.entityTypeName)
	ts.Require().NoError(err, "failed to discover extension schema via GET /Schemas")
	ts.extensionURN = urn

	ts.engineeringUserID = ts.createSearchFixtureUser(
		"scim.it.search.eng@example.com", "search-Engineering", "search-Colombo")
	ts.salesUserID = ts.createSearchFixtureUser(
		"scim.it.search.sales@example.com", "search-Sales", "search-Kandy")
}

// TearDownSuite cleans up the test suite environment.
func (ts *SCIMSearchTestSuite) TearDownSuite() {
	for _, id := range ts.createdUserIDs {
		_, _, _ = scimRequest(http.MethodDelete, "/Users/"+id, nil, nil)
	}
	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// createSearchFixtureUser handles create search fixture user.
func (ts *SCIMSearchTestSuite) createSearchFixtureUser(email, department, locality string) string {
	payload := map[string]interface{}{
		"schemas": []string{ts.extensionURN},
		ts.extensionURN: map[string]interface{}{
			"email":      email,
			"department": department,
			"address":    map[string]interface{}{"locality": locality},
		},
	}
	body, err := json.Marshal(payload)
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, status, "failed to provision search fixture user: %s", respBody)

	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &created))
	id, _ := created["id"].(string)
	ts.Require().NotEmpty(id)
	ts.createdUserIDs = append(ts.createdUserIDs, id)
	return id
}

// search handles search.
func (ts *SCIMSearchTestSuite) search(req scimSearchRequest) (int, scimUserListResponse, []byte) {
	ts.T().Helper()
	body, err := json.Marshal(req)
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPost, "/Users/.search", body, nil)
	ts.Require().NoError(err)

	var list scimUserListResponse
	if status == http.StatusOK {
		ts.Require().NoError(json.Unmarshal(respBody, &list))
	}
	return status, list, respBody
}

// ---------------------------------------------------------------------------
// Success cases
// ---------------------------------------------------------------------------

// TestSearchNoFilterDefaultPagination verifies .search with no filter uses the default pagination.
func (ts *SCIMSearchTestSuite) TestSearchNoFilterDefaultPagination() {
	status, list, body := ts.search(scimSearchRequest{Schemas: []string{scimSearchSchemaURN}})
	ts.Require().Equal(http.StatusOK, status, "search failed: %s", body)

	ts.Equal(1, list.StartIndex, "startIndex should default to 1")
	ts.GreaterOrEqual(list.TotalResults, 2, "should see at least both fixture users")
	ts.GreaterOrEqual(list.ItemsPerPage, 1)
}

// TestSearchFilterByDepartmentWithPagination verifies .search filters by department and paginates.
func (ts *SCIMSearchTestSuite) TestSearchFilterByDepartmentWithPagination() {
	status, list, body := ts.search(scimSearchRequest{
		Schemas:    []string{scimSearchSchemaURN},
		Filter:     ts.extensionURN + `:department eq "search-Engineering"`,
		StartIndex: 1,
		Count:      10,
	})
	ts.Require().Equal(http.StatusOK, status, "search failed: %s", body)
	ts.Require().Equal(1, list.TotalResults)
	ts.Equal(ts.engineeringUserID, list.Resources[0].ID)
}

// TestSearchFilterByHierarchicalCustomAttributeURNQualified verifies .search filters by a
// URN-qualified nested custom attribute.
func (ts *SCIMSearchTestSuite) TestSearchFilterByHierarchicalCustomAttributeURNQualified() {
	status, list, body := ts.search(scimSearchRequest{
		Schemas: []string{scimSearchSchemaURN},
		Filter:  ts.extensionURN + `:address.locality eq "search-Kandy"`,
	})
	ts.Require().Equal(http.StatusOK, status, "search failed: %s", body)
	ts.Require().Equal(1, list.TotalResults)
	ts.Equal(ts.salesUserID, list.Resources[0].ID)
}

// TestSearchCompoundAndFilter verifies .search accepts a compound "and" filter.
func (ts *SCIMSearchTestSuite) TestSearchCompoundAndFilter() {
	status, list, body := ts.search(scimSearchRequest{
		Schemas: []string{scimSearchSchemaURN},
		Filter:  ts.extensionURN + `:department eq "search-Sales" and ` + ts.extensionURN + `:address.locality eq "search-Kandy"`,
	})
	ts.Require().Equal(http.StatusOK, status, "search failed: %s", body)
	ts.Require().Equal(1, list.TotalResults)
	ts.Equal(ts.salesUserID, list.Resources[0].ID)
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

// TestSearchWrongContentTypeRejected verifies .search with the wrong Content-Type is rejected.
func (ts *SCIMSearchTestSuite) TestSearchWrongContentTypeRejected() {
	body, err := json.Marshal(scimSearchRequest{Schemas: []string{scimSearchSchemaURN}})
	ts.Require().NoError(err)

	status, _, err := scimRequest(http.MethodPost, "/Users/.search", body,
		map[string]string{"Content-Type": "text/plain"})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status)
}

// TestSearchMissingSchemasRejected pins that .search requires its own
// envelope schema URN (SearchRequest), distinct from the core User schema
// checked on Create/Replace.
func (ts *SCIMSearchTestSuite) TestSearchMissingSchemasRejected() {
	status, _, body := ts.search(scimSearchRequest{})
	ts.Equal(http.StatusBadRequest, status, "missing schemas: %s", body)
}

// TestSearchEmptyBodyRejected verifies .search with an empty body is rejected.
func (ts *SCIMSearchTestSuite) TestSearchEmptyBodyRejected() {
	status, _, err := scimRequest(http.MethodPost, "/Users/.search", []byte{}, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status)
}

// TestSearchMalformedJSONRejected verifies a non-empty .search body that isn't valid JSON is rejected.
func (ts *SCIMSearchTestSuite) TestSearchMalformedJSONRejected() {
	status, _, err := scimRequest(http.MethodPost, "/Users/.search", []byte(`{not valid json`), nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status)
}

// TestSearchSortByRejected verifies .search rejects sortBy when sorting is not supported.
func (ts *SCIMSearchTestSuite) TestSearchSortByRejected() {
	status, _, body := ts.search(scimSearchRequest{
		Schemas: []string{scimSearchSchemaURN},
		SortBy:  "department",
	})
	ts.Equal(http.StatusBadRequest, status, "sortBy must be rejected: %s", body)
}

// TestSearchConflictingAttributesParamsRejected verifies .search rejects attributes and
// excludedAttributes both being set, same as GET.
func (ts *SCIMSearchTestSuite) TestSearchConflictingAttributesParamsRejected() {
	status, _, body := ts.search(scimSearchRequest{
		Schemas:            []string{scimSearchSchemaURN},
		Attributes:         []string{ts.extensionURN + ":department"},
		ExcludedAttributes: []string{ts.extensionURN + ":department"},
	})
	ts.Equal(http.StatusBadRequest, status, "conflicting attributes params must be rejected: %s", body)
}

// TestSearchOrFilterRejected verifies .search rejects an "or" filter, same as GET.
func (ts *SCIMSearchTestSuite) TestSearchOrFilterRejected() {
	status, _, body := ts.search(scimSearchRequest{
		Schemas: []string{scimSearchSchemaURN},
		Filter:  ts.extensionURN + `:department eq "search-Engineering" or ` + ts.extensionURN + `:department eq "search-Sales"`,
	})
	ts.Equal(http.StatusBadRequest, status, "'or' is not supported: %s", body)
}

// TestSearchUnsupportedOperatorRejected verifies .search rejects an unsupported filter operator.
func (ts *SCIMSearchTestSuite) TestSearchUnsupportedOperatorRejected() {
	status, _, body := ts.search(scimSearchRequest{
		Schemas: []string{scimSearchSchemaURN},
		Filter:  ts.extensionURN + `:department co "search-Engin"`,
	})
	ts.Equal(http.StatusBadRequest, status, "only 'eq' is supported: %s", body)
}
