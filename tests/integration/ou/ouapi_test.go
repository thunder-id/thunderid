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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	testServerURL = "https://localhost:8095"
)

var (
	ouToCreate = CreateOURequest{
		Name:            "OU API Test Organization Unit",
		Handle:          "ou-api-test-org-unit",
		Description:     "Test OU for integration testing",
		Parent:          nil,
		LogoURL:         "https://example.com/logo.png",
		TosURI:          "https://example.com/tos",
		PolicyURI:       "https://example.com/privacy",
		CookiePolicyURI: "https://example.com/cookie-policy",
		ThemeID:         "theme-123",
		LayoutID:        "layout-456",
	}

	childOUToCreate = CreateOURequest{
		Name:            "Child Test OU",
		Handle:          "child-test-ou",
		Description:     "Child OU for testing hierarchy",
		Parent:          nil,
		LogoURL:         "https://example.com/child-logo.png",
		TosURI:          "https://example.com/child-tos",
		PolicyURI:       "https://example.com/child-privacy",
		CookiePolicyURI: "https://example.com/child-cookie-policy",
		ThemeID:         "theme-child",
		LayoutID:        "layout-child",
	}
)

var createdOUID string
var createdChildOUID string

type OUAPITestSuite struct {
	suite.Suite
}

func TestOUAPITestSuite(t *testing.T) {
	suite.Run(t, new(OUAPITestSuite))
}

func (suite *OUAPITestSuite) SetupSuite() {
	id, err := createOU(suite, ouToCreate)
	suite.Require().NoError(err, "Failed to create OU during setup: %v", err)

	createdOUID = id
	childOUToCreate.Parent = &createdOUID
	childID, err := createOU(suite, childOUToCreate)
	suite.Require().NoError(err, "Failed to create child OU during setup: %v", err)
	createdChildOUID = childID
}

func (suite *OUAPITestSuite) TearDownSuite() {
	if createdChildOUID != "" {
		err := deleteOU(createdChildOUID)
		if err != nil {
			suite.T().Logf("Failed to delete child OU during teardown: %v", err)
		}
	}

	if createdOUID != "" {
		err := deleteOU(createdOUID)
		if err != nil {
			suite.T().Fatalf("Failed to delete OU during teardown: %v", err)
		}
	}
}

func (suite *OUAPITestSuite) TestGetOrganizationUnit() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID is not available for retrieval")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/"+createdOUID, nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err, "Failed to read response body: %v", err)

	var retrievedOU OrganizationUnit
	err = json.Unmarshal(body, &retrievedOU)
	suite.Require().NoError(err)

	// Verify the retrieved OU
	suite.Equal(createdOUID, retrievedOU.ID)
	suite.Equal(ouToCreate.Name, retrievedOU.Name)
	suite.Equal(ouToCreate.Description, retrievedOU.Description)
	suite.Equal(ouToCreate.Parent, retrievedOU.Parent)
	suite.Equal(ouToCreate.LogoURL, retrievedOU.LogoURL)
	suite.Equal(ouToCreate.TosURI, retrievedOU.TosURI)
	suite.Equal(ouToCreate.PolicyURI, retrievedOU.PolicyURI)
	suite.Equal(ouToCreate.CookiePolicyURI, retrievedOU.CookiePolicyURI)
	suite.Equal(ouToCreate.ThemeID, retrievedOU.ThemeID)
	suite.Equal(ouToCreate.LayoutID, retrievedOU.LayoutID)
	suite.False(retrievedOU.CreatedAt.IsZero(), "createdAt must not be zero")
	suite.False(retrievedOU.UpdatedAt.IsZero(), "updatedAt must not be zero")
	suite.Equal(time.UTC, retrievedOU.CreatedAt.Location(), "createdAt must be UTC")
	suite.Equal(time.UTC, retrievedOU.UpdatedAt.Location(), "updatedAt must be UTC")
}

func (suite *OUAPITestSuite) TestListOrganizationUnits() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID is not available, OU creation failed in setup")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err, "Failed to read response body: %v", err)

	var ouListResponse OrganizationUnitListResponse
	err = json.Unmarshal(body, &ouListResponse)
	suite.Require().NoError(err)

	// Verify response structure
	suite.GreaterOrEqual(ouListResponse.TotalResults, 1)
	suite.Equal(1, ouListResponse.StartIndex)
	suite.Equal(len(ouListResponse.OrganizationUnits), ouListResponse.Count)

	// Verify the list contains our created OUs
	foundParent := false
	for _, ou := range ouListResponse.OrganizationUnits {
		if ou.ID == createdOUID {
			foundParent = true
			suite.Equal(ouToCreate.Name, ou.Name)
			suite.Equal(ouToCreate.Description, ou.Description)
			suite.Equal(ouToCreate.LogoURL, ou.LogoURL)
			suite.False(ou.CreatedAt.IsZero(), "createdAt must not be zero")
			suite.False(ou.UpdatedAt.IsZero(), "updatedAt must not be zero")
			suite.Equal(time.UTC, ou.CreatedAt.Location(), "createdAt must be UTC")
			suite.Equal(time.UTC, ou.UpdatedAt.Location(), "updatedAt must be UTC")
		}
	}
	suite.True(foundParent, "Created parent OU should be in the list")
}

func (suite *OUAPITestSuite) TestListOrganizationUnitsWithPagination() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units?limit=1&offset=0", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var ouListResponse OrganizationUnitListResponse
	err = json.Unmarshal(body, &ouListResponse)
	suite.Require().NoError(err)

	suite.GreaterOrEqual(ouListResponse.TotalResults, 1)
	suite.Equal(1, ouListResponse.StartIndex)
	suite.LessOrEqual(ouListResponse.Count, 1)
	if ouListResponse.TotalResults > 1 {
		suite.NotEmpty(ouListResponse.Links)
	}
}

func (suite *OUAPITestSuite) TestListOrganizationUnitsWithInvalidPagination() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units?limit=-1", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1010", errorResp["code"])

	req, err = http.NewRequest("GET", testServerURL+"/organization-units?offset=-1", nil)
	suite.Require().NoError(err)

	resp, err = client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	body, err = io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1011", errorResp["code"])
}

func (suite *OUAPITestSuite) TestListOrganizationUnitsWithOnlyOffset() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units?offset=0", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var ouListResponse OrganizationUnitListResponse
	err = json.Unmarshal(body, &ouListResponse)
	suite.Require().NoError(err)

	suite.GreaterOrEqual(ouListResponse.TotalResults, 1)
	suite.Equal(1, ouListResponse.StartIndex)
	suite.LessOrEqual(ouListResponse.Count, 30)
}

func (suite *OUAPITestSuite) TestUpdateOrganizationUnit() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID is not available for update")
	}

	ouBefore := suite.fetchOU(createdOUID)
	time.Sleep(10 * time.Millisecond)

	updateRequest := UpdateOURequest{
		Name:            "Updated Test Organization Unit",
		Handle:          "updated-test-org-unit",
		Description:     "Updated description for testing",
		LogoURL:         "https://example.com/updated-logo.png",
		TosURI:          "https://example.com/updated-tos",
		PolicyURI:       "https://example.com/updated-privacy",
		CookiePolicyURI: "https://example.com/updated-cookie-policy",
		ThemeID:         "theme-updated",
		LayoutID:        "layout-updated",
	}

	jsonData, err := json.Marshal(updateRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("PUT", testServerURL+"/organization-units/"+createdOUID, bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var updatedOU OrganizationUnit
	err = json.Unmarshal(body, &updatedOU)
	suite.Require().NoError(err)

	// Verify the update
	suite.Equal(createdOUID, updatedOU.ID)
	suite.Equal("Updated Test Organization Unit", updatedOU.Name)
	suite.Equal("updated-test-org-unit", updatedOU.Handle)
	suite.Equal("Updated description for testing", updatedOU.Description)
	suite.Equal("https://example.com/updated-logo.png", updatedOU.LogoURL)
	suite.Equal("https://example.com/updated-tos", updatedOU.TosURI)
	suite.Equal("https://example.com/updated-privacy", updatedOU.PolicyURI)
	suite.Equal("https://example.com/updated-cookie-policy", updatedOU.CookiePolicyURI)
	suite.Equal("theme-updated", updatedOU.ThemeID)
	suite.Equal("layout-updated", updatedOU.LayoutID)
	suite.False(updatedOU.UpdatedAt.IsZero(), "updatedAt must not be zero after update")
	suite.Equal(time.UTC, updatedOU.UpdatedAt.Location(), "updatedAt must be UTC")
	suite.True(updatedOU.UpdatedAt.After(ouBefore.UpdatedAt),
		"updatedAt must advance after update (before=%v, after=%v)", ouBefore.UpdatedAt, updatedOU.UpdatedAt)
	suite.Equal(ouBefore.CreatedAt, updatedOU.CreatedAt, "createdAt must not change after an update")
}

func (suite *OUAPITestSuite) TestDeleteOrganizationUnit() {
	tempOUToCreate := CreateOURequest{
		Name:        "Temp Test OU",
		Handle:      "temp-test-ou",
		Description: "Temporary OU for deletion test",
		Parent:      nil,
	}

	tempOUID, err := createOU(suite, tempOUToCreate)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	// Delete the temporary OU
	req, err := http.NewRequest("DELETE", testServerURL+"/organization-units/"+tempOUID, nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNoContent, resp.StatusCode)

	getReq, err := http.NewRequest("GET", testServerURL+"/organization-units/"+tempOUID, nil)
	suite.Require().NoError(err)

	getResp, err := client.Do(getReq)
	suite.Require().NoError(err, "Failed to execute GET request: %v", err)
	defer getResp.Body.Close()

	suite.Equal(http.StatusNotFound, getResp.StatusCode)
}

func (suite *OUAPITestSuite) TestGetNonExistentOrganizationUnit() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/non-existent-id", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err, "Failed to execute GET request: %v", err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode)
}

func (suite *OUAPITestSuite) TestCreateOrganizationUnitWithInvalidData() {
	invalidOU := map[string]interface{}{
		"description": "OU without name",
		"parent":      nil,
	}

	jsonData, err := json.Marshal(invalidOU)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/organization-units", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)

	suite.Require().NoError(err, "Failed to execute POST request: %v", err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (suite *OUAPITestSuite) TestCreateOrganizationUnitWithInvalidParent() {
	invalidOU := CreateOURequest{
		Name:        "OU with Invalid Parent",
		Handle:      "ou-with-invalid-parent",
		Description: "Testing invalid parent",
		Parent:      stringPtr("invalid-parent-id-12345"),
	}

	jsonData, err := json.Marshal(invalidOU)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/organization-units", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	// Verify the error response
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1005", errorResp["code"])
	suite.Equal("Parent organization unit not found", errorResp["message"].(map[string]interface{})["defaultValue"])
}

func (suite *OUAPITestSuite) TestCreateOrganizationUnitWithDuplicateName() {
	duplicateOU := CreateOURequest{
		Name:        ouToCreate.Name,
		Handle:      "duplicate-name-test",
		Description: "Duplicate name test",
		Parent:      nil,
	}

	jsonData, err := json.Marshal(duplicateOU)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/organization-units", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusConflict, resp.StatusCode)

	// Verify the error response
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1004", errorResp["code"])
	suite.Equal("Organization unit name conflict", errorResp["message"].(map[string]interface{})["defaultValue"])
}

func (suite *OUAPITestSuite) TestCreateOrganizationUnitWithDuplicateHandle() {
	duplicateHandleOU := CreateOURequest{
		Name:        "OU with Duplicate Handle",
		Handle:      ouToCreate.Handle,
		Description: "Duplicate handle test",
		Parent:      nil,
	}

	jsonData, err := json.Marshal(duplicateHandleOU)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/organization-units", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusConflict, resp.StatusCode)

	// Verify the error response
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1008", errorResp["code"])
	suite.Equal("Organization unit handle conflict", errorResp["message"].(map[string]interface{})["defaultValue"])
}

func (suite *OUAPITestSuite) TestDeleteOrganizationUnitWithChildren() {
	if createdOUID == "" || createdChildOUID == "" {
		suite.T().Fatal("Parent or child OU ID is not available for deletion test")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("DELETE", testServerURL+"/organization-units/"+createdOUID, nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	// Verify the error response
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1006", errorResp["code"])
	suite.Equal("Organization unit has children", errorResp["message"].(map[string]interface{})["defaultValue"])
}

func (suite *OUAPITestSuite) TestUpdateOrganizationUnitWithInvalidParent() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID is not available for update test")
	}

	updateRequest := UpdateOURequest{
		Name:        "Updated OU with Invalid Parent",
		Handle:      "updated-ou-with-invalid-parent",
		Description: "Testing invalid parent update",
		Parent:      stringPtr("invalid-parent-id-12345"),
	}

	jsonData, err := json.Marshal(updateRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("PUT", testServerURL+"/organization-units/"+createdOUID, bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	// Verify the error response
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1005", errorResp["code"])
	suite.Equal("Parent organization unit not found", errorResp["message"].(map[string]interface{})["defaultValue"])
}

func (suite *OUAPITestSuite) TestUpdateOrganizationUnitWithDuplicateName() {
	if createdOUID == "" {
		suite.T().Fatal("Parent OU ID is not available for update test")
	}

	sibling1Request := CreateOURequest{
		Name:        "Sibling OU 1",
		Handle:      "sibling-ou-1",
		Description: "First sibling OU for testing duplicate names",
		Parent:      &createdOUID,
	}

	sibling2Request := CreateOURequest{
		Name:        "Sibling OU 2",
		Handle:      "sibling-ou-2",
		Description: "Second sibling OU for testing duplicate names",
		Parent:      &createdOUID,
	}

	// Create first sibling
	sibling1ID, err := createOU(suite, sibling1Request)
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteOU(sibling1ID); deleteErr != nil {
			suite.T().Logf("Failed to cleanup sibling1 OU: %v", deleteErr)
		}
	}()

	// Create second sibling
	sibling2ID, err := createOU(suite, sibling2Request)
	suite.Require().NoError(err)
	defer func() {
		if deleteErr := deleteOU(sibling2ID); deleteErr != nil {
			suite.T().Logf("Failed to cleanup sibling2 OU: %v", deleteErr)
		}
	}()

	updateRequest := UpdateOURequest{
		Name:        sibling1Request.Name,
		Handle:      "testing-duplicate-name-update",
		Description: "Testing duplicate name update with sibling",
		Parent:      &createdOUID,
	}

	jsonData, err := json.Marshal(updateRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("PUT", testServerURL+"/organization-units/"+sibling2ID, bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusConflict, resp.StatusCode)

	// Verify the error response
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1004", errorResp["code"])
	suite.Equal("Organization unit name conflict", errorResp["message"].(map[string]interface{})["defaultValue"])
}

func (suite *OUAPITestSuite) TestCreateOrganizationUnitWithEmptyName() {
	invalidOU := CreateOURequest{
		Name:        "",
		Handle:      "empty-name-ou",
		Description: "OU with empty name",
		Parent:      nil,
	}

	jsonData, err := json.Marshal(invalidOU)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/organization-units", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (suite *OUAPITestSuite) TestCreateOrganizationUnitWithEmptyHandle() {
	invalidOU := CreateOURequest{
		Name:        "OU with Empty Handle",
		Handle:      "",
		Description: "OU with empty handle",
		Parent:      nil,
	}

	jsonData, err := json.Marshal(invalidOU)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/organization-units", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (suite *OUAPITestSuite) TestUpdateNonExistentOrganizationUnit() {
	updateRequest := UpdateOURequest{
		Name:        "Non-existent OU Update",
		Handle:      "non-existent-ou-update",
		Description: "Testing update of non-existent OU",
	}

	jsonData, err := json.Marshal(updateRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("PUT", testServerURL+"/organization-units/non-existent-id", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode)
}

func (suite *OUAPITestSuite) TestDeleteNonExistentOrganizationUnit() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("DELETE", testServerURL+"/organization-units/non-existent-id", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode)
}

func (suite *OUAPITestSuite) TestGetOrganizationUnitChildren() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID is not available for retrieving children")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/"+createdOUID+"/ous", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err, "Failed to read response body")

	var childrenResponse OrganizationUnitListResponse
	err = json.Unmarshal(body, &childrenResponse)
	suite.Require().NoError(err)

	// Verify response structure
	suite.GreaterOrEqual(childrenResponse.TotalResults, 1)
	suite.Equal(1, childrenResponse.StartIndex)

	// Verify our child OU is in the list
	foundChild := false
	for _, ou := range childrenResponse.OrganizationUnits {
		if ou.ID == createdChildOUID {
			foundChild = true
			suite.Equal(childOUToCreate.Name, ou.Name)
			suite.Equal(childOUToCreate.Description, ou.Description)
			suite.Equal(childOUToCreate.LogoURL, ou.LogoURL)
			suite.False(ou.CreatedAt.IsZero(), "createdAt must not be zero for child OU")
			suite.False(ou.UpdatedAt.IsZero(), "updatedAt must not be zero for child OU")
			suite.Equal(time.UTC, ou.CreatedAt.Location(), "createdAt must be UTC for child OU")
			suite.Equal(time.UTC, ou.UpdatedAt.Location(), "updatedAt must be UTC for child OU")
		}
	}
	suite.True(foundChild, "Created child OU should be in the children list")
}

func (suite *OUAPITestSuite) TestGetOrganizationUnitUsers() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID is not available for retrieving users")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/"+createdOUID+"/users", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err, "Failed to read response body: %v", err)

	var usersResponse testutils.UserListResponse
	err = json.Unmarshal(body, &usersResponse)
	suite.Require().NoError(err)

	// Verify response structure
	suite.GreaterOrEqual(usersResponse.TotalResults, 0)
	suite.Equal(1, usersResponse.StartIndex)
	suite.Equal(len(usersResponse.Users), usersResponse.Count)
}

func (suite *OUAPITestSuite) TestGetOrganizationUnitGroups() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID is not available for retrieving groups")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/"+createdOUID+"/groups", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err, "Failed to read response body: %v", err)

	var groupsResponse GroupListResponse
	err = json.Unmarshal(body, &groupsResponse)
	suite.Require().NoError(err)

	// Verify response structure
	suite.GreaterOrEqual(groupsResponse.TotalResults, 0)
	suite.Equal(1, groupsResponse.StartIndex)
	suite.Equal(len(groupsResponse.Groups), groupsResponse.Count)
}

func (suite *OUAPITestSuite) TestGetNonExistentOrganizationUnitChildren() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/non-existent-id/ous", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1003", errorResp["code"])
}

func (suite *OUAPITestSuite) TestGetNonExistentOrganizationUnitUsers() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/non-existent-id/users", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1003", errorResp["code"])
}

func (suite *OUAPITestSuite) TestGetNonExistentOrganizationUnitGroups() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/non-existent-id/groups", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp map[string]interface{}
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1003", errorResp["code"])
}

func (suite *OUAPITestSuite) TestGetOrganizationUnitChildrenWithPagination() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID is not available for retrieving children with pagination")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/"+createdOUID+"/ous?limit=1&offset=0", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var childrenResponse OrganizationUnitListResponse
	err = json.Unmarshal(body, &childrenResponse)
	suite.Require().NoError(err)

	suite.GreaterOrEqual(childrenResponse.TotalResults, 0)
	suite.Equal(1, childrenResponse.StartIndex)
	suite.LessOrEqual(childrenResponse.Count, 1)
}

func (suite *OUAPITestSuite) TestGetOrganizationUnitUsersWithPagination() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID is not available for retrieving users with pagination")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/"+createdOUID+"/users?limit=10&offset=0", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var usersResponse testutils.UserListResponse
	err = json.Unmarshal(body, &usersResponse)
	suite.Require().NoError(err)

	suite.GreaterOrEqual(usersResponse.TotalResults, 0)
	suite.Equal(1, usersResponse.StartIndex)
	suite.LessOrEqual(usersResponse.Count, 10)
}

func (suite *OUAPITestSuite) TestGetOrganizationUnitGroupsWithPagination() {
	if createdOUID == "" {
		suite.T().Fatal("OU ID is not available for retrieving groups with pagination")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/"+createdOUID+"/groups?limit=10&offset=0", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var groupsResponse GroupListResponse
	err = json.Unmarshal(body, &groupsResponse)
	suite.Require().NoError(err)

	suite.GreaterOrEqual(groupsResponse.TotalResults, 0)
	suite.Equal(1, groupsResponse.StartIndex)
	suite.LessOrEqual(groupsResponse.Count, 10)
}

func (suite *OUAPITestSuite) fetchOU(id string) OrganizationUnit {
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("GET", testServerURL+"/organization-units/"+id, nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var ou OrganizationUnit
	suite.Require().NoError(json.Unmarshal(body, &ou))
	return ou
}

func createOU(ts *OUAPITestSuite, ouRequest CreateOURequest) (string, error) {
	jsonData, err := json.Marshal(ouRequest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OU request: %w", err)
	}

	req, err := http.NewRequest("POST", testServerURL+"/organization-units", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("expected status 201, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var createdOU OrganizationUnit
	err = json.NewDecoder(resp.Body).Decode(&createdOU)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}

	return createdOU.ID, nil
}

func deleteOU(ouID string) error {
	req, err := http.NewRequest("DELETE", testServerURL+"/organization-units/"+ouID, nil)
	if err != nil {
		return err
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("expected status 204, got %d", resp.StatusCode)
	}
	return nil
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
