/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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

	engineeringUserID string // department=search-Engineering, address.locality=Colombo
	salesUserID       string // department=search-Sales, address.locality=Kandy
}

func TestSCIMSearchTestSuite(t *testing.T) {
	suite.Run(t, new(SCIMSearchTestSuite))
}

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
		Name: ts.entityTypeName,
		OUID: ouID,
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
		"scim.it.search.eng@example.com", "search-Engineering", "Colombo")
	ts.salesUserID = ts.createSearchFixtureUser(
		"scim.it.search.sales@example.com", "search-Sales", "Kandy")
}

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

func (ts *SCIMSearchTestSuite) createSearchFixtureUser(email, department, locality string) string {
	payload := map[string]interface{}{
		"schemas": []string{scimCoreUserSchemaURN, ts.extensionURN},
		"emails": []map[string]interface{}{
			{"value": email, "type": "work"},
		},
		ts.extensionURN: map[string]interface{}{
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

func (ts *SCIMSearchTestSuite) TestSearchNoFilterDefaultPagination() {
	status, list, body := ts.search(scimSearchRequest{Schemas: []string{scimSearchSchemaURN}})
	ts.Require().Equal(http.StatusOK, status, "search failed: %s", body)

	ts.Equal(1, list.StartIndex, "startIndex should default to 1")
	ts.GreaterOrEqual(list.TotalResults, 2, "should see at least both fixture users")
	ts.GreaterOrEqual(list.ItemsPerPage, 1)
}

func (ts *SCIMSearchTestSuite) TestSearchFilterByDepartmentWithPagination() {
	status, list, body := ts.search(scimSearchRequest{
		Schemas:    []string{scimSearchSchemaURN},
		Filter:     `department eq "search-Engineering"`,
		StartIndex: 1,
		Count:      10,
	})
	ts.Require().Equal(http.StatusOK, status, "search failed: %s", body)
	ts.Require().Equal(1, list.TotalResults)
	ts.Equal(ts.engineeringUserID, list.Resources[0].ID)
}

func (ts *SCIMSearchTestSuite) TestSearchFilterByHierarchicalCustomAttributeURNQualified() {
	status, list, body := ts.search(scimSearchRequest{
		Schemas: []string{scimSearchSchemaURN},
		Filter:  ts.extensionURN + `:address.locality eq "Kandy"`,
	})
	ts.Require().Equal(http.StatusOK, status, "search failed: %s", body)
	ts.Require().Equal(1, list.TotalResults)
	ts.Equal(ts.salesUserID, list.Resources[0].ID)
}

func (ts *SCIMSearchTestSuite) TestSearchCompoundAndFilter() {
	status, list, body := ts.search(scimSearchRequest{
		Schemas: []string{scimSearchSchemaURN},
		Filter:  `department eq "search-Sales" and address.locality eq "Kandy"`,
	})
	ts.Require().Equal(http.StatusOK, status, "search failed: %s", body)
	ts.Require().Equal(1, list.TotalResults)
	ts.Equal(ts.salesUserID, list.Resources[0].ID)
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

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

func (ts *SCIMSearchTestSuite) TestSearchEmptyBodyRejected() {
	status, _, err := scimRequest(http.MethodPost, "/Users/.search", []byte{}, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status)
}

func (ts *SCIMSearchTestSuite) TestSearchOrFilterRejected() {
	status, _, body := ts.search(scimSearchRequest{
		Schemas: []string{scimSearchSchemaURN},
		Filter:  `department eq "search-Engineering" or department eq "search-Sales"`,
	})
	ts.Equal(http.StatusBadRequest, status, "'or' is not supported: %s", body)
}

func (ts *SCIMSearchTestSuite) TestSearchUnsupportedOperatorRejected() {
	status, _, body := ts.search(scimSearchRequest{
		Schemas: []string{scimSearchSchemaURN},
		Filter:  `department co "search-Engin"`,
	})
	ts.Equal(http.StatusBadRequest, status, "only 'eq' is supported: %s", body)
}
