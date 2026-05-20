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
	"io"
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

var (
	pathTestOU = CreateOURequest{
		Name:        "Engineering",
		Handle:      "engineering",
		Description: "Engineering Unit",
		Parent:      nil,
		LogoURL:     "https://example.com/path-logo.png",
		ThemeID:     "theme-path",
		LayoutID:    "layout-path",
	}
)

var pathTestOUID string

type OUPathAPITestSuite struct {
	suite.Suite
}

func TestOUPathAPITestSuite(t *testing.T) {
	suite.Run(t, new(OUPathAPITestSuite))
}

func (suite *OUPathAPITestSuite) SetupSuite() {
	id, err := createOUForPath(suite, pathTestOU)
	suite.Require().NoError(err, "Failed to create OU during setup: %v", err)
	pathTestOUID = id
}

func (suite *OUPathAPITestSuite) TearDownSuite() {
	if pathTestOUID != "" {
		err := deleteOUForPath(suite, pathTestOU.Handle)
		if err != nil {
			suite.T().Logf("Failed to delete OU during teardown: %v", err)
		}
	}
}

// TestGetOrganizationUnitByPath tests retrieving an organization unit by handle path
func (suite *OUPathAPITestSuite) TestGetOrganizationUnitByPath() {
	if pathTestOUID == "" {
		suite.T().Fatal("OU ID is not available for path-based retrieval")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/tree/"+pathTestOU.Handle, nil)
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
	suite.Equal(pathTestOUID, retrievedOU.ID)
	suite.Equal(pathTestOU.Name, retrievedOU.Name)
	suite.Equal(pathTestOU.Handle, retrievedOU.Handle)
	suite.Equal(pathTestOU.Description, retrievedOU.Description)
	suite.Equal(pathTestOU.Parent, retrievedOU.Parent)
	suite.Equal(pathTestOU.LogoURL, retrievedOU.LogoURL)
	suite.Equal(pathTestOU.ThemeID, retrievedOU.ThemeID)
	suite.Equal(pathTestOU.LayoutID, retrievedOU.LayoutID)
}

// TestGetOrganizationUnitByInvalidPath tests retrieving an organization unit by invalid handle path
func (suite *OUPathAPITestSuite) TestGetOrganizationUnitByInvalidPath() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/tree/nonexistent", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusNotFound, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1003", errorResp.Code)
	suite.Equal("Organization unit not found", errorResp.Message.DefaultValue)
}

// TestUpdateOrganizationUnitByPath tests updating an organization unit by handle path
func (suite *OUPathAPITestSuite) TestUpdateOrganizationUnitByPath() {
	if pathTestOUID == "" {
		suite.T().Fatal("OU ID is not available for path-based update test")
	}

	updateRequest := UpdateOURequest{
		Handle:      "engineering",
		Name:        "Updated OU via Path",
		Description: "Updated description via path",
		LogoURL:     "https://example.com/updated-path-logo.png",
		ThemeID:     "theme-updated-path",
		LayoutID:    "layout-updated-path",
	}

	jsonData, err := json.Marshal(updateRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("PUT", testServerURL+"/organization-units/tree/"+pathTestOU.Handle, bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log the error instead of returning it
			suite.T().Logf("Failed to close response body: %v", closeErr)
		}
	}()

	suite.Equal(http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var updatedOU OrganizationUnit
	err = json.Unmarshal(body, &updatedOU)
	suite.Require().NoError(err)

	// Verify the update
	suite.Equal(pathTestOUID, updatedOU.ID)
	suite.Equal(updateRequest.Name, updatedOU.Name)
	suite.Equal(updateRequest.Description, updatedOU.Description)
	suite.Equal(updateRequest.Handle, updatedOU.Handle)
	suite.Equal(updateRequest.LogoURL, updatedOU.LogoURL)
	suite.Equal(updateRequest.ThemeID, updatedOU.ThemeID)
	suite.Equal(updateRequest.LayoutID, updatedOU.LayoutID)

	// Update the global test data for subsequent tests
	pathTestOU.Name = updateRequest.Name
	pathTestOU.Description = updateRequest.Description
}

// TestUpdateOrganizationUnitByInvalidPath tests updating an organization unit by invalid handle path
func (suite *OUPathAPITestSuite) TestUpdateOrganizationUnitByInvalidPath() {
	updateRequest := UpdateOURequest{
		Name:        "Updated OU via Invalid Path",
		Description: "Updated description via invalid path",
	}

	jsonData, err := json.Marshal(updateRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("PUT", testServerURL+"/organization-units/tree/nonexistent", bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusNotFound, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1003", errorResp.Code)
	suite.Equal("Organization unit not found", errorResp.Message.DefaultValue)
}

// TestUpdateOrganizationUnitByPathWithInvalidData tests updating an organization unit by path with invalid data
func (suite *OUPathAPITestSuite) TestUpdateOrganizationUnitByPathWithInvalidData() {
	if pathTestOUID == "" {
		suite.T().Fatal("OU ID is not available for path-based update test")
	}

	// Test with empty name
	updateRequest := UpdateOURequest{
		Name:        "",
		Description: "Testing empty name",
	}

	jsonData, err := json.Marshal(updateRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("PUT", testServerURL+"/organization-units/tree/"+pathTestOU.Handle, bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestDeleteOrganizationUnitByInvalidPath tests deleting an organization unit by invalid handle path
func (suite *OUPathAPITestSuite) TestDeleteOrganizationUnitByInvalidPath() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("DELETE", testServerURL+"/organization-units/tree/nonexistent", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusNotFound, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1003", errorResp.Code)
	suite.Equal("Organization unit not found", errorResp.Message.DefaultValue)
}

// TestGetOrganizationUnitChildrenByPath tests retrieving child organization units by handle path
func (suite *OUPathAPITestSuite) TestGetOrganizationUnitChildrenByPath() {
	if pathTestOUID == "" {
		suite.T().Fatal("OU ID is not available for children path-based retrieval")
	}

	// First create a child OU for testing
	childOU := CreateOURequest{
		Name:        "Child Engineering",
		Handle:      "child-engineering",
		Description: "Child of Engineering Unit",
		Parent:      &pathTestOUID,
		LogoURL:     "https://example.com/child-engineering-logo.png",
	}

	childID, err := createOUForPath(suite, childOU)
	suite.Require().NoError(err, "Failed to create child OU for testing")
	defer func() {
		if childID != "" {
			if err := deleteOUByID(suite, childID); err != nil {
				suite.T().Logf("Failed to delete child OU with ID %s: %v", childID, err)
			}
		}
	}()

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/tree/"+pathTestOU.Handle+"/ous", nil)
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

	var ouListResponse OrganizationUnitListResponse
	err = json.Unmarshal(body, &ouListResponse)
	suite.Require().NoError(err)

	// Verify the response structure
	suite.GreaterOrEqual(ouListResponse.TotalResults, 1)
	suite.GreaterOrEqual(ouListResponse.Count, 1)
	suite.GreaterOrEqual(len(ouListResponse.OrganizationUnits), 1)

	// Check if our child OU is in the list
	found := false
	for _, ou := range ouListResponse.OrganizationUnits {
		if ou.ID == childID {
			found = true
			suite.Equal(childOU.Name, ou.Name)
			suite.Equal(childOU.Handle, ou.Handle)
			suite.Equal(childOU.Description, ou.Description)
			suite.Equal(childOU.LogoURL, ou.LogoURL)
			break
		}
	}
	suite.True(found, "Child OU should be found in the children list")
}

// TestGetOrganizationUnitChildrenByInvalidPath tests retrieving child organization units by invalid handle path
func (suite *OUPathAPITestSuite) TestGetOrganizationUnitChildrenByInvalidPath() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/tree/nonexistent/ous", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusNotFound, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1003", errorResp.Code)
	suite.Equal("Organization unit not found", errorResp.Message.DefaultValue)
}

// TestGetOrganizationUnitUsersByPath tests retrieving users by handle path
func (suite *OUPathAPITestSuite) TestGetOrganizationUnitUsersByPath() {
	if pathTestOUID == "" {
		suite.T().Fatal("OU ID is not available for users path-based retrieval")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/tree/"+pathTestOU.Handle+"/users", nil)
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

	var userListResponse testutils.UserListResponse
	err = json.Unmarshal(body, &userListResponse)
	suite.Require().NoError(err)

	// Verify the response structure
	suite.GreaterOrEqual(userListResponse.TotalResults, 0)
	suite.Equal(userListResponse.StartIndex, 1)
	suite.Equal(userListResponse.Count, len(userListResponse.Users))
}

// TestGetOrganizationUnitUsersByInvalidPath tests retrieving users by invalid handle path
func (suite *OUPathAPITestSuite) TestGetOrganizationUnitUsersByInvalidPath() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/tree/nonexistent/users", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusNotFound, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1003", errorResp.Code)
	suite.Equal("Organization unit not found", errorResp.Message.DefaultValue)
}

// TestGetOrganizationUnitGroupsByPath tests retrieving groups by handle path
func (suite *OUPathAPITestSuite) TestGetOrganizationUnitGroupsByPath() {
	if pathTestOUID == "" {
		suite.T().Fatal("OU ID is not available for groups path-based retrieval")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/tree/"+pathTestOU.Handle+"/groups", nil)
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

	var groupListResponse GroupListResponse
	err = json.Unmarshal(body, &groupListResponse)
	suite.Require().NoError(err)

	// Verify the response structure
	suite.GreaterOrEqual(groupListResponse.TotalResults, 0)
	suite.Equal(groupListResponse.StartIndex, 1)
	suite.Equal(groupListResponse.Count, len(groupListResponse.Groups))
}

// TestGetOrganizationUnitGroupsByInvalidPath tests retrieving groups by invalid handle path
func (suite *OUPathAPITestSuite) TestGetOrganizationUnitGroupsByInvalidPath() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/tree/nonexistent/groups", nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusNotFound, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("OU-1003", errorResp.Code)
	suite.Equal("Organization unit not found", errorResp.Message.DefaultValue)
}

// TestGetOrganizationUnitChildrenByPathWithPagination tests pagination for child organization units by path
func (suite *OUPathAPITestSuite) TestGetOrganizationUnitChildrenByPathWithPagination() {
	if pathTestOUID == "" {
		suite.T().Fatal("OU ID is not available for pagination test")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/organization-units/tree/"+pathTestOU.Handle+"/ous?limit=5&offset=0", nil)
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
	suite.Require().NoError(err)

	var ouListResponse OrganizationUnitListResponse
	err = json.Unmarshal(body, &ouListResponse)
	suite.Require().NoError(err)

	// Verify pagination parameters
	suite.GreaterOrEqual(ouListResponse.TotalResults, 0)
	suite.Equal(ouListResponse.StartIndex, 1)
	suite.LessOrEqual(ouListResponse.Count, 5)
}

// deleteOUByID is a helper function to delete an OU by ID
func deleteOUByID(suite *OUPathAPITestSuite, id string) error {
	req, err := http.NewRequest("DELETE", testServerURL+"/organization-units/"+id, nil)
	if err != nil {
		return err
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	return nil
}

// createOUForPath is a helper function to create an OU for path tests
func createOUForPath(suite *OUPathAPITestSuite, ouRequest CreateOURequest) (string, error) {
	jsonData, err := json.Marshal(ouRequest)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", testServerURL+"/organization-units", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusCreated {
		return "", err
	}

	var createdOU OrganizationUnit
	err = json.NewDecoder(resp.Body).Decode(&createdOU)
	if err != nil {
		return "", err
	}

	return createdOU.ID, nil
}

// deleteOUForPath is a helper function to delete an OU for path tests
func deleteOUForPath(suite *OUPathAPITestSuite, path string) error {
	req, err := http.NewRequest("DELETE", testServerURL+"/organization-units/tree/"+path, nil)
	if err != nil {
		return err
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	return nil
}
