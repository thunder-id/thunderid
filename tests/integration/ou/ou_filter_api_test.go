/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package ou

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// --- Root-level OU list AND/OR multi-expression filter tests ---

// TestListOrganizationUnitsWithFilterAndExpression verifies that an AND filter
// narrows results: both conditions must match.
func (suite *OUAPITestSuite) TestListOrganizationUnitsWithFilterAndExpression() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	// Both name and handle match the created OU — should include it.
	resp := suite.doFilterRequest(
		"/organization-units",
		`name eq "`+ouToCreate.Name+`" AND handle eq "`+ouToCreate.Handle+`"`,
	)
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	found := false
	for _, ou := range listResp.OrganizationUnits {
		if ou.ID == createdOUID {
			found = true
		}
	}
	suite.True(found, "AND filter with both matching conditions should include the OU")

	// Name matches but handle does not — should return empty.
	resp2 := suite.doFilterRequest(
		"/organization-units",
		`name eq "`+ouToCreate.Name+`" AND handle eq "__no_match__"`,
	)
	defer resp2.Body.Close()
	suite.Equal(http.StatusOK, resp2.StatusCode)

	var emptyResp OrganizationUnitListResponse
	suite.decodeBody(resp2, &emptyResp)
	for _, ou := range emptyResp.OrganizationUnits {
		suite.NotEqual(createdOUID, ou.ID, "AND filter with non-matching handle should exclude the OU")
	}
}

// TestListOrganizationUnitsWithFilterOrExpression verifies that an OR filter
// returns OUs matching either condition.
func (suite *OUAPITestSuite) TestListOrganizationUnitsWithFilterOrExpression() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	// Create a second root OU for this test.
	secondOU := CreateOURequest{
		Name:   "Second Filter Test OU",
		Handle: "second-filter-test-ou",
	}
	secondID, err := createOU(suite, secondOU)
	suite.Require().NoError(err, "Failed to create second OU for OR test")
	defer func() {
		if err := deleteOU(secondID); err != nil {
			suite.T().Logf("Failed to delete second OU %s: %v", secondID, err)
		}
	}()

	resp := suite.doFilterRequest(
		"/organization-units",
		`name eq "`+ouToCreate.Name+`" OR name eq "`+secondOU.Name+`"`,
	)
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	foundFirst, foundSecond := false, false
	for _, ou := range listResp.OrganizationUnits {
		if ou.ID == createdOUID {
			foundFirst = true
		}
		if ou.ID == secondID {
			foundSecond = true
		}
	}
	suite.True(foundFirst, "OR filter should include the first OU")
	suite.True(foundSecond, "OR filter should include the second OU")
}

// TestListOrganizationUnitsWithFilterMixedAndOr verifies AND-before-OR precedence:
// (name match AND createdAt match) OR __no_match__ resolves to true for the OU.
func (suite *OUAPITestSuite) TestListOrganizationUnitsWithFilterMixedAndOr() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	resp := suite.doFilterRequest(
		"/organization-units",
		`name eq "`+ouToCreate.Name+`" AND createdAt gt "2000-01-01T00:00:00Z" OR name eq "__no_match__"`,
	)
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	found := false
	for _, ou := range listResp.OrganizationUnits {
		if ou.ID == createdOUID {
			found = true
		}
	}
	suite.True(found, "Mixed AND/OR filter should include the OU (AND group evaluates to true)")
}

// TestListOrganizationUnitsWithInvalidConnector verifies that an unsupported connector
// returns 400 OU-1014.
func (suite *OUAPITestSuite) TestListOrganizationUnitsWithInvalidConnector() {
	resp := suite.doFilterRequest("/organization-units", `name eq "A" XOR name eq "B"`)
	defer resp.Body.Close()
	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	var errResp ErrorResponse
	suite.decodeBody(resp, &errResp)
	suite.Equal("OU-1014", errResp.Code)
}

// --- Children list AND/OR filter tests ---

// TestGetOrganizationUnitChildrenWithFilterAndExpression verifies AND filtering on children.
func (suite *OUAPITestSuite) TestGetOrganizationUnitChildrenWithFilterAndExpression() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	// Both name and handle match the child OU.
	resp := suite.doFilterRequest(
		"/organization-units/"+createdOUID+"/ous",
		`name eq "`+childOUToCreate.Name+`" AND handle eq "`+childOUToCreate.Handle+`"`,
	)
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	suite.GreaterOrEqual(listResp.TotalResults, 1)
	found := false
	for _, ou := range listResp.OrganizationUnits {
		if ou.ID == createdChildOUID {
			found = true
		}
	}
	suite.True(found, "AND filter should include the matching child OU")

	// Name matches but handle does not — should return empty.
	resp2 := suite.doFilterRequest(
		"/organization-units/"+createdOUID+"/ous",
		`name eq "`+childOUToCreate.Name+`" AND handle eq "__no_match__"`,
	)
	defer resp2.Body.Close()
	suite.Equal(http.StatusOK, resp2.StatusCode)

	var emptyResp OrganizationUnitListResponse
	suite.decodeBody(resp2, &emptyResp)
	suite.Equal(0, emptyResp.TotalResults)
}

// TestGetOrganizationUnitChildrenWithFilterOrExpression verifies OR filtering on children.
func (suite *OUAPITestSuite) TestGetOrganizationUnitChildrenWithFilterOrExpression() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	resp := suite.doFilterRequest(
		"/organization-units/"+createdOUID+"/ous",
		`name eq "`+childOUToCreate.Name+`" OR name eq "__no_match__"`,
	)
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	found := false
	for _, ou := range listResp.OrganizationUnits {
		if ou.ID == createdChildOUID {
			found = true
		}
	}
	suite.True(found, "OR filter should include the matching child OU")
}

// --- Path-based children AND filter test ---

// TestGetOrganizationUnitChildrenByPathWithFilterAndExpression verifies AND filtering
// via the path-based endpoint.
func (suite *OUAPITestSuite) TestGetOrganizationUnitChildrenByPathWithFilterAndExpression() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	resp := suite.doFilterRequest(
		"/organization-units/tree/"+ouToCreate.Handle+"/ous",
		`name eq "`+childOUToCreate.Name+`" AND handle eq "`+childOUToCreate.Handle+`"`,
	)
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	found := false
	for _, ou := range listResp.OrganizationUnits {
		if ou.ID == createdChildOUID {
			found = true
		}
	}
	suite.True(found, "path-based AND filter should return the matching child OU")
}

// --- Root-level OU list filter tests ---

// TestListOrganizationUnitsWithFilterEqMatch verifies that filtering by exact name returns only matching OUs.
func (suite *OUAPITestSuite) TestListOrganizationUnitsWithFilterEqMatch() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	resp := suite.doFilterRequest("/organization-units", "name eq \""+ouToCreate.Name+"\"")
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	suite.GreaterOrEqual(listResp.TotalResults, 1)
	found := false
	for _, ou := range listResp.OrganizationUnits {
		if ou.ID == createdOUID {
			found = true
			suite.Equal(ouToCreate.Name, ou.Name)
		}
	}
	suite.True(found, "filter eq should include the matching OU")
}

// TestListOrganizationUnitsWithFilterEqNoMatch verifies that filtering with a non-matching name returns empty.
func (suite *OUAPITestSuite) TestListOrganizationUnitsWithFilterEqNoMatch() {
	resp := suite.doFilterRequest("/organization-units", "name eq \"__no_such_ou_xyz__\"")
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	suite.Equal(0, listResp.TotalResults)
	suite.Empty(listResp.OrganizationUnits)
}

// TestListOrganizationUnitsWithFilterGtMatch verifies that gt filtering by name returns OUs whose name
// sorts after the threshold.
func (suite *OUAPITestSuite) TestListOrganizationUnitsWithFilterGtMatch() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	// "OU API Test Organization Unit" starts with "O" which is > "L"
	resp := suite.doFilterRequest("/organization-units", "name gt \"L\"")
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	found := false
	for _, ou := range listResp.OrganizationUnits {
		if ou.ID == createdOUID {
			found = true
		}
	}
	suite.True(found, "filter gt should include OU whose name sorts after the threshold")
}

// TestListOrganizationUnitsWithFilterGtNoMatch verifies that gt filtering with a threshold above all
// names returns an empty list.
func (suite *OUAPITestSuite) TestListOrganizationUnitsWithFilterGtNoMatch() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	// "OU API Test Organization Unit" starts with "O" which is NOT > "P"
	resp := suite.doFilterRequest("/organization-units", "name gt \"P\"")
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	for _, ou := range listResp.OrganizationUnits {
		suite.NotEqual(createdOUID, ou.ID, "filter gt should exclude OU whose name sorts at or before the threshold")
	}
}

// TestListOrganizationUnitsWithFilterLtMatch verifies that lt filtering by name returns OUs whose name
// sorts before the threshold.
func (suite *OUAPITestSuite) TestListOrganizationUnitsWithFilterLtMatch() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	// "OU API Test Organization Unit" starts with "O" which is < "P"
	resp := suite.doFilterRequest("/organization-units", "name lt \"P\"")
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	found := false
	for _, ou := range listResp.OrganizationUnits {
		if ou.ID == createdOUID {
			found = true
		}
	}
	suite.True(found, "filter lt should include OU whose name sorts before the threshold")
}

// TestListOrganizationUnitsWithFilterLtNoMatch verifies that lt filtering with a threshold below all
// names returns an empty list.
func (suite *OUAPITestSuite) TestListOrganizationUnitsWithFilterLtNoMatch() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	// "OU API Test Organization Unit" starts with "O" which is NOT < "L"
	resp := suite.doFilterRequest("/organization-units", "name lt \"L\"")
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	for _, ou := range listResp.OrganizationUnits {
		suite.NotEqual(createdOUID, ou.ID, "filter lt should exclude OU whose name sorts at or after the threshold")
	}
}

// TestListOrganizationUnitsWithFilterByHandle verifies eq filtering on the handle attribute.
func (suite *OUAPITestSuite) TestListOrganizationUnitsWithFilterByHandle() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	resp := suite.doFilterRequest("/organization-units", "handle eq \""+ouToCreate.Handle+"\"")
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	suite.GreaterOrEqual(listResp.TotalResults, 1)
	found := false
	for _, ou := range listResp.OrganizationUnits {
		if ou.ID == createdOUID {
			found = true
		}
	}
	suite.True(found, "filter by handle eq should return the matching OU")
}

// TestListOrganizationUnitsWithInvalidFilter verifies that a malformed filter expression returns 400.
func (suite *OUAPITestSuite) TestListOrganizationUnitsWithInvalidFilter() {
	resp := suite.doFilterRequest("/organization-units", "not a valid filter")
	defer resp.Body.Close()
	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	var errResp ErrorResponse
	suite.decodeBody(resp, &errResp)
	suite.Equal("OU-1014", errResp.Code)
}

// TestListOrganizationUnitsWithUnsupportedFilterAttribute verifies that filtering on an unknown attribute
// returns 400.
func (suite *OUAPITestSuite) TestListOrganizationUnitsWithUnsupportedFilterAttribute() {
	resp := suite.doFilterRequest("/organization-units", "unknownField eq \"foo\"")
	defer resp.Body.Close()
	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	var errResp ErrorResponse
	suite.decodeBody(resp, &errResp)
	suite.Equal("OU-1014", errResp.Code)
}

// --- Children list filter tests ---

// TestGetOrganizationUnitChildrenWithFilterEqMatch verifies that filtering children by name returns
// only the matching child OU.
func (suite *OUAPITestSuite) TestGetOrganizationUnitChildrenWithFilterEqMatch() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	resp := suite.doFilterRequest(
		"/organization-units/"+createdOUID+"/ous",
		"name eq \""+childOUToCreate.Name+"\"",
	)
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	suite.GreaterOrEqual(listResp.TotalResults, 1)
	found := false
	for _, ou := range listResp.OrganizationUnits {
		if ou.ID == createdChildOUID {
			found = true
			suite.Equal(childOUToCreate.Name, ou.Name)
		}
	}
	suite.True(found, "filter eq should include the matching child OU")
}

// TestGetOrganizationUnitChildrenWithFilterEqNoMatch verifies that a non-matching filter returns an
// empty children list.
func (suite *OUAPITestSuite) TestGetOrganizationUnitChildrenWithFilterEqNoMatch() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	resp := suite.doFilterRequest(
		"/organization-units/"+createdOUID+"/ous",
		"name eq \"__no_such_child__\"",
	)
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	suite.Equal(0, listResp.TotalResults)
	suite.Empty(listResp.OrganizationUnits)
}

// TestGetOrganizationUnitChildrenWithFilterGtMatch verifies that gt filtering on children returns
// children whose name sorts after the threshold.
func (suite *OUAPITestSuite) TestGetOrganizationUnitChildrenWithFilterGtMatch() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	// "Child Test OU" starts with "C" which is > "B"
	resp := suite.doFilterRequest(
		"/organization-units/"+createdOUID+"/ous",
		"name gt \"B\"",
	)
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	found := false
	for _, ou := range listResp.OrganizationUnits {
		if ou.ID == createdChildOUID {
			found = true
		}
	}
	suite.True(found, "filter gt should include child OU whose name sorts after the threshold")
}

// TestGetOrganizationUnitChildrenWithFilterLtMatch verifies that lt filtering on children returns
// children whose name sorts before the threshold.
func (suite *OUAPITestSuite) TestGetOrganizationUnitChildrenWithFilterLtMatch() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	// "Child Test OU" starts with "C" which is < "D"
	resp := suite.doFilterRequest(
		"/organization-units/"+createdOUID+"/ous",
		"name lt \"D\"",
	)
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	found := false
	for _, ou := range listResp.OrganizationUnits {
		if ou.ID == createdChildOUID {
			found = true
		}
	}
	suite.True(found, "filter lt should include child OU whose name sorts before the threshold")
}

// TestGetOrganizationUnitChildrenWithInvalidFilter verifies that a malformed filter on the children
// endpoint returns 400.
func (suite *OUAPITestSuite) TestGetOrganizationUnitChildrenWithInvalidFilter() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	resp := suite.doFilterRequest("/organization-units/"+createdOUID+"/ous", "bad filter")
	defer resp.Body.Close()
	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	var errResp ErrorResponse
	suite.decodeBody(resp, &errResp)
	suite.Equal("OU-1014", errResp.Code)
}

// TestGetOrganizationUnitChildrenWithUnsupportedFilterAttribute verifies that filtering children on
// an unknown attribute returns 400.
func (suite *OUAPITestSuite) TestGetOrganizationUnitChildrenWithUnsupportedFilterAttribute() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	resp := suite.doFilterRequest("/organization-units/"+createdOUID+"/ous", "unknownAttr eq \"foo\"")
	defer resp.Body.Close()
	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	var errResp ErrorResponse
	suite.decodeBody(resp, &errResp)
	suite.Equal("OU-1014", errResp.Code)
}

// --- Path-based children filter tests ---

// TestGetOrganizationUnitChildrenByPathWithFilterEqMatch verifies eq filtering on the path-based
// children endpoint.
func (suite *OUAPITestSuite) TestGetOrganizationUnitChildrenByPathWithFilterEqMatch() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	resp := suite.doFilterRequest(
		"/organization-units/tree/"+ouToCreate.Handle+"/ous",
		"name eq \""+childOUToCreate.Name+"\"",
	)
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	suite.GreaterOrEqual(listResp.TotalResults, 1)
	found := false
	for _, ou := range listResp.OrganizationUnits {
		if ou.ID == createdChildOUID {
			found = true
		}
	}
	suite.True(found, "path filter eq should return the matching child OU")
}

// TestGetOrganizationUnitChildrenByPathWithFilterEqNoMatch verifies that a non-matching filter on
// the path-based endpoint returns an empty list.
func (suite *OUAPITestSuite) TestGetOrganizationUnitChildrenByPathWithFilterEqNoMatch() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	resp := suite.doFilterRequest(
		"/organization-units/tree/"+ouToCreate.Handle+"/ous",
		"name eq \"__no_match__\"",
	)
	defer resp.Body.Close()
	suite.Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	suite.decodeBody(resp, &listResp)

	suite.Equal(0, listResp.TotalResults)
}

// TestGetOrganizationUnitChildrenByPathWithInvalidFilter verifies that a malformed filter on the
// path-based children endpoint returns 400.
func (suite *OUAPITestSuite) TestGetOrganizationUnitChildrenByPathWithInvalidFilter() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID not available")
	}

	resp := suite.doFilterRequest(
		"/organization-units/tree/"+ouToCreate.Handle+"/ous",
		"not valid",
	)
	defer resp.Body.Close()
	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	var errResp ErrorResponse
	suite.decodeBody(resp, &errResp)
	suite.Equal("OU-1014", errResp.Code)
}

// --- Helpers ---

// doFilterRequest issues a GET request to path with the given filter expression as the "filter"
// query parameter.
func (suite *OUAPITestSuite) doFilterRequest(path, filterExpr string) *http.Response {
	client := testutils.GetHTTPClient()
	u, err := url.Parse(testServerURL + path)
	suite.Require().NoError(err)
	q := u.Query()
	q.Set("filter", filterExpr)
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	suite.Require().NoError(err)
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	return resp
}

// decodeBody reads and JSON-decodes the response body into dst.
func (suite *OUAPITestSuite) decodeBody(resp *http.Response, dst interface{}) {
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().NoError(json.Unmarshal(body, dst))
}
