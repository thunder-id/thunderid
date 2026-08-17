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
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// SCIMFilterTestSuite exercises GET /Users?filter= against the grammar
// backend/internal/scim's parseSCIMFilterForEq actually implements: one or
// more "eq" clauses joined by "and" (case-insensitive), no "or", "not", or
// grouping, and exactly one occurrence per attribute. It covers both a
// core-mapped hierarchical attribute (name.givenName, reverse-mapped through
// a schema property this usertype declares) and a plain custom nested
// attribute (address.locality, declared only in the extension schema, so a
// URN-qualified path is needed to disambiguate it).
//
// One shared set of users is provisioned once in SetupSuite and every test
// filters against that same fixture, mirroring tests/integration/user's
// UserFilterTestSuite.
type SCIMFilterTestSuite struct {
	suite.Suite
	ouID           string
	entityTypeID   string
	entityTypeName string
	extensionURN   string

	// Fixture users, keyed by what makes each one distinct for filtering.
	engineeringUserID string // department=filter-Engineering, address.locality=Colombo, name.givenName=Jane
	marketingUserID   string // department=filter-Sales and Marketing (tests literal "and" inside a quoted value)
	salesUserID       string // department=filter-Sales, address.locality=Kandy, name.givenName=John

	createdUserIDs []string
}

func TestSCIMFilterTestSuite(t *testing.T) {
	suite.Run(t, new(SCIMFilterTestSuite))
}

func (ts *SCIMFilterTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "scim-it-filter-ou",
		Name:        "SCIM Filter Integration Test OU",
		Description: "Organization unit for SCIM filter grammar tests",
	})
	ts.Require().NoError(err, "failed to create test organization unit")
	ts.ouID = ouID

	ts.entityTypeName = "scim-it-filter-person"
	entityTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: ts.entityTypeName,
		OUID: ouID,
		Schema: map[string]interface{}{
			"email":      map[string]interface{}{"type": "string", "required": true, "unique": true},
			"given_name": map[string]interface{}{"type": "string"},
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

	ts.engineeringUserID = ts.createFilterFixtureUser(
		"scim.it.filter.jane@example.com", "Jane", "filter-Engineering", "Colombo")
	ts.marketingUserID = ts.createFilterFixtureUser(
		"scim.it.filter.marketing@example.com", "Alex", "filter-Sales and Marketing", "Galle")
	ts.salesUserID = ts.createFilterFixtureUser(
		"scim.it.filter.john@example.com", "John", "filter-Sales", "Kandy")
}

func (ts *SCIMFilterTestSuite) TearDownSuite() {
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

// createFilterFixtureUser provisions one fixture user with email (core-mapped,
// required/unique) and three plain extension attributes: given_name (filtered
// via the core-mapped path name.givenName), department, and a nested address
// object with a "locality" sub-property.
func (ts *SCIMFilterTestSuite) createFilterFixtureUser(email, givenName, department, locality string) string {
	payload := map[string]interface{}{
		"schemas": []string{scimCoreUserSchemaURN, ts.extensionURN},
		"emails": []map[string]interface{}{
			{"value": email, "type": "work"},
		},
		ts.extensionURN: map[string]interface{}{
			"given_name": givenName,
			"department": department,
			"address":    map[string]interface{}{"locality": locality},
		},
	}
	body, err := json.Marshal(payload)
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, status, "failed to provision filter fixture user: %s", respBody)

	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &created))
	id, _ := created["id"].(string)
	ts.Require().NotEmpty(id)
	ts.createdUserIDs = append(ts.createdUserIDs, id)
	return id
}

// listWithFilter issues GET /Users?filter=<filter>, escaped exactly as a real
// client would send it, and returns the decoded status and body.
func (ts *SCIMFilterTestSuite) listWithFilter(filter string) (int, scimUserListResponse) {
	ts.T().Helper()
	status, body, err := scimRequest(http.MethodGet, "/Users?filter="+url.QueryEscape(filter), nil, nil)
	ts.Require().NoError(err)

	var list scimUserListResponse
	if status == http.StatusOK {
		ts.Require().NoError(json.Unmarshal(body, &list))
	}
	return status, list
}

// ---------------------------------------------------------------------------
// Success cases
// ---------------------------------------------------------------------------

func (ts *SCIMFilterTestSuite) TestFilterByPlainCustomAttribute() {
	status, list := ts.listWithFilter(`department eq "filter-Engineering"`)
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().Equal(1, list.TotalResults)
	ts.Equal(ts.engineeringUserID, list.Resources[0].ID)
}

// TestFilterByHierarchicalCoreAttribute filters on name.givenName, a
// core-mapped sub-attribute reverse-mapped through this usertype's own
// "given_name" schema property.
func (ts *SCIMFilterTestSuite) TestFilterByHierarchicalCoreAttribute() {
	status, list := ts.listWithFilter(`name.givenName eq "Jane"`)
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().Equal(1, list.TotalResults)
	ts.Equal(ts.engineeringUserID, list.Resources[0].ID)
}

// TestFilterByHierarchicalCustomAttributeURNQualified filters on a nested
// extension-only attribute (address.locality). "address" has no core SCIM
// mapping of its own name (the core-mapped multi-complex field is plural,
// "addresses"), so this exercises the URN-qualified path a real client sends
// to disambiguate a custom nested attribute unambiguously.
func (ts *SCIMFilterTestSuite) TestFilterByHierarchicalCustomAttributeURNQualified() {
	filter := ts.extensionURN + `:address.locality eq "Colombo"`
	status, list := ts.listWithFilter(filter)
	ts.Require().Equal(http.StatusOK, status, "URN-qualified nested custom attribute filter should be accepted")
	ts.Require().Equal(1, list.TotalResults)
	ts.Equal(ts.engineeringUserID, list.Resources[0].ID)
}

func (ts *SCIMFilterTestSuite) TestFilterCompoundAndTwoClauses() {
	status, list := ts.listWithFilter(`department eq "filter-Sales" and address.locality eq "Kandy"`)
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().Equal(1, list.TotalResults)
	ts.Equal(ts.salesUserID, list.Resources[0].ID)
}

// TestFilterQuotedValueContainingLiteralAndIsNotSplit pins the specific
// splitSCIMAndClauses behavior of masking quoted spans before splitting on
// " and ": the literal word "and" inside this department value must not be
// mistaken for the compound-filter join keyword.
func (ts *SCIMFilterTestSuite) TestFilterQuotedValueContainingLiteralAndIsNotSplit() {
	status, list := ts.listWithFilter(`department eq "filter-Sales and Marketing"`)
	ts.Require().Equal(http.StatusOK, status, "a literal 'and' inside a quoted value must not be split")
	ts.Require().Equal(1, list.TotalResults)
	ts.Equal(ts.marketingUserID, list.Resources[0].ID)
}

// ---------------------------------------------------------------------------
// Rejected cases
// ---------------------------------------------------------------------------

func (ts *SCIMFilterTestSuite) TestFilterDuplicateAttributeInAndRejected() {
	status, _ := ts.listWithFilter(`department eq "filter-Engineering" and department eq "filter-Sales"`)
	ts.Equal(http.StatusBadRequest, status, "repeating the same attribute in a compound filter must be rejected")
}

func (ts *SCIMFilterTestSuite) TestFilterOrRejected() {
	status, _ := ts.listWithFilter(`department eq "filter-Engineering" or department eq "filter-Sales"`)
	ts.Equal(http.StatusBadRequest, status, "'or' is not a supported filter join")
}

func (ts *SCIMFilterTestSuite) TestFilterNotRejected() {
	status, _ := ts.listWithFilter(`not (department eq "filter-Engineering")`)
	ts.Equal(http.StatusBadRequest, status, "'not' is not supported")
}

func (ts *SCIMFilterTestSuite) TestFilterGroupingRejected() {
	status, _ := ts.listWithFilter(`(department eq "filter-Engineering") and department eq "filter-Sales"`)
	ts.Equal(http.StatusBadRequest, status, "grouping with parentheses is not supported")
}

func (ts *SCIMFilterTestSuite) TestFilterUnsupportedOperatorRejected() {
	status, _ := ts.listWithFilter(`department co "filter-Engin"`)
	ts.Equal(http.StatusBadRequest, status, "only 'eq' is a supported filter operator")
}

// TestFilterUnsupportedMultiValueSubAttrRejected pins that filtering on a
// sub-attribute of a multi-valued core field with no flat equivalent to
// compare against (emails.type) is rejected rather than silently matching
// nothing.
func (ts *SCIMFilterTestSuite) TestFilterUnsupportedMultiValueSubAttrRejected() {
	status, _ := ts.listWithFilter(`emails.type eq "work"`)
	ts.Equal(http.StatusBadRequest, status, "filtering on emails.type has no flat equivalent and must be rejected")
}
