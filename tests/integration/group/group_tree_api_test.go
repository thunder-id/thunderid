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

package group

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
		Name:        "Test Engineering",
		Handle:      "test-engineering",
		Description: "Test Engineering Unit for Group Tests",
		Parent:      nil,
	}

	pathTestGroup = CreateGroupByPathRequest{
		Name:        "Frontend Team",
		Description: "Frontend development team",
		Members:     []Member{},
	}
)

var pathTestOUID string
var pathTestGroupID string

// CreateOURequest represents the request body for creating an organization unit.
type CreateOURequest struct {
	Handle      string  `json:"handle"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Parent      *string `json:"parent,omitempty"`
}

// OrganizationUnit represents an organization unit.
type OrganizationUnit struct {
	ID          string  `json:"id"`
	Handle      string  `json:"handle"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Parent      *string `json:"parent"`
}

type GroupTreeAPITestSuite struct {
	suite.Suite
}

func TestGroupTreeAPITestSuite(t *testing.T) {
	suite.Run(t, new(GroupTreeAPITestSuite))
}

func (suite *GroupTreeAPITestSuite) SetupSuite() {
	// Create OU for testing
	id, err := createOUForGroupTests(suite, pathTestOU)
	suite.Require().NoError(err, "Failed to create OU during setup: %v", err)
	pathTestOUID = id
}

func (suite *GroupTreeAPITestSuite) TearDownSuite() {
	// Clean up created group if exists
	if pathTestGroupID != "" {
		err := deleteGroupByID(suite, pathTestGroupID)
		if err != nil {
			suite.T().Logf("Failed to delete group during teardown: %v", err)
		}
	}

	// Clean up created OU
	if pathTestOUID != "" {
		err := deleteOUByID(suite, pathTestOUID)
		if err != nil {
			suite.T().Logf("Failed to delete OU during teardown: %v", err)
		}
	}
}

// TestGetGroupsByPath tests retrieving groups by organization unit handle path
func (suite *GroupTreeAPITestSuite) TestGetGroupsByPath() {
	if pathTestOUID == "" {
		suite.T().Fatal("OU ID is not available for path-based group retrieval")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups/tree/"+pathTestOU.Handle, nil)
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

// TestGetGroupsByInvalidPath tests retrieving groups by invalid organization unit handle path
func (suite *GroupTreeAPITestSuite) TestGetGroupsByInvalidPath() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups/tree/nonexistent-ou", nil)
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

	suite.Equal("GRP-1003", errorResp.Code)
	suite.Equal("Group not found", errorResp.Message.DefaultValue)
}

// TestGetGroupsByPathWithPagination tests retrieving groups by path with pagination parameters
func (suite *GroupTreeAPITestSuite) TestGetGroupsByPathWithPagination() {
	if pathTestOUID == "" {
		suite.T().Fatal("OU ID is not available for pagination test")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/groups/tree/"+pathTestOU.Handle+"?limit=5&offset=0", nil)
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

	var groupListResponse GroupListResponse
	err = json.Unmarshal(body, &groupListResponse)
	suite.Require().NoError(err)

	// Verify pagination parameters
	suite.GreaterOrEqual(groupListResponse.TotalResults, 0)
	suite.Equal(groupListResponse.StartIndex, 1)
	suite.LessOrEqual(groupListResponse.Count, 5)
}

// TestCreateGroupByPath tests creating a group by organization unit handle path
func (suite *GroupTreeAPITestSuite) TestCreateGroupByPath() {
	if pathTestOUID == "" {
		suite.T().Fatal("OU ID is not available for group creation by path")
	}

	jsonData, err := json.Marshal(pathTestGroup)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups/tree/"+pathTestOU.Handle, bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusCreated, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var createdGroup Group
	err = json.Unmarshal(body, &createdGroup)
	suite.Require().NoError(err)

	// Verify the created group
	suite.Equal(pathTestGroup.Name, createdGroup.Name)
	suite.Equal(pathTestGroup.Description, createdGroup.Description)
	suite.Equal(pathTestOUID, createdGroup.OUID)
	suite.Equal(len(pathTestGroup.Members), len(createdGroup.Members))

	// Store the group ID for cleanup
	pathTestGroupID = createdGroup.Id

	suite.T().Logf("Created group with ID: %s", pathTestGroupID)
}

// TestCreateGroupByInvalidPath tests creating a group by invalid organization unit handle path
func (suite *GroupTreeAPITestSuite) TestCreateGroupByInvalidPath() {
	createRequest := CreateGroupByPathRequest{
		Name:        "Invalid Path Group",
		Description: "Group created with invalid path",
		Members:     []Member{},
	}

	jsonData, err := json.Marshal(createRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups/tree/nonexistent-ou", bytes.NewBuffer(jsonData))
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

	suite.Equal("GRP-1003", errorResp.Code)
	suite.Equal("Group not found", errorResp.Message.DefaultValue)
}

// TestCreateGroupByPathWithInvalidData tests creating a group by path with invalid data
func (suite *GroupTreeAPITestSuite) TestCreateGroupByPathWithInvalidData() {
	if pathTestOUID == "" {
		suite.T().Fatal("OU ID is not available for invalid data test")
	}

	// Test with empty name
	createRequest := CreateGroupByPathRequest{
		Name:        "",
		Description: "Group with empty name",
		Members:     []Member{},
	}

	jsonData, err := json.Marshal(createRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups/tree/"+pathTestOU.Handle, bytes.NewBuffer(jsonData))
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

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("GRP-1001", errorResp.Code)
	suite.Equal("Invalid request format", errorResp.Message.DefaultValue)
}

// TestCreateGroupByPathWithInvalidMemberType tests creating a group by path with invalid member type
func (suite *GroupTreeAPITestSuite) TestCreateGroupByPathWithInvalidMemberType() {
	if pathTestOUID == "" {
		suite.T().Fatal("OU ID is not available for invalid member type test")
	}

	// Create a request with invalid member type
	requestBody := map[string]interface{}{
		"name":        "Group with Invalid Member",
		"description": "Group with invalid member type",
		"members": []map[string]interface{}{
			{
				"id":   "00000000-0000-0000-0000-000000000000",
				"type": "invalid_type",
			},
		},
	}

	jsonData, err := json.Marshal(requestBody)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups/tree/"+pathTestOU.Handle, bytes.NewBuffer(jsonData))
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

// TestCreateGroupByPathWithMalformedJSON tests creating a group by path with malformed JSON
func (suite *GroupTreeAPITestSuite) TestCreateGroupByPathWithMalformedJSON() {
	if pathTestOUID == "" {
		suite.T().Fatal("OU ID is not available for malformed JSON test")
	}

	malformedJSON := `{"name": "Test Group", "description": "Malformed JSON",`

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups/tree/"+pathTestOU.Handle, bytes.NewBufferString(malformedJSON))
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

// TestCreateGroupByPathWithEmptyPath tests creating a group with empty path
func (suite *GroupTreeAPITestSuite) TestCreateGroupByPathWithEmptyPath() {
	createRequest := CreateGroupByPathRequest{
		Name:        "Group with Empty Path",
		Description: "Group created with empty path",
		Members:     []Member{},
	}

	jsonData, err := json.Marshal(createRequest)
	suite.Require().NoError(err)

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/groups/tree/", bytes.NewBuffer(jsonData))
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

// TestGroupTreeEndpointsWithNestedPaths tests the endpoints with nested organization unit paths
func (suite *GroupTreeAPITestSuite) TestGroupTreeEndpointsWithNestedPaths() {
	if pathTestOUID == "" {
		suite.T().Fatal("OU ID is not available for nested path test")
	}

	// Create a child OU for testing nested paths
	childOU := CreateOURequest{
		Name:        "Backend Team",
		Handle:      "backend",
		Description: "Backend development team",
		Parent:      &pathTestOUID,
	}

	childOUID, err := createOUForGroupTests(suite, childOU)
	suite.Require().NoError(err, "Failed to create child OU for nested path testing")
	defer func() {
		if childOUID != "" {
			if err := deleteOUByID(suite, childOUID); err != nil {
				suite.T().Logf("Failed to delete child OU with ID %s: %v", childOUID, err)
			}
		}
	}()

	client := testutils.GetHTTPClient()

	// Test GET with nested path
	nestedPath := pathTestOU.Handle + "/" + childOU.Handle
	req, err := http.NewRequest("GET", testServerURL+"/groups/tree/"+nestedPath, nil)
	suite.Require().NoError(err)

	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusOK, resp.StatusCode)

	// Test POST with nested path
	nestedGroupRequest := CreateGroupByPathRequest{
		Name:        "Backend API Team",
		Description: "Team responsible for backend APIs",
		Members:     []Member{},
	}

	jsonData, err := json.Marshal(nestedGroupRequest)
	suite.Require().NoError(err)

	postReq, err := http.NewRequest("POST", testServerURL+"/groups/tree/"+nestedPath, bytes.NewBuffer(jsonData))
	suite.Require().NoError(err)
	postReq.Header.Set("Content-Type", "application/json")

	postResp, err := client.Do(postReq)
	suite.Require().NoError(err)
	defer func() {
		if err := postResp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	suite.Equal(http.StatusCreated, postResp.StatusCode)

	// Clean up the created group
	postBody, err := io.ReadAll(postResp.Body)
	suite.Require().NoError(err)

	var createdGroup Group
	err = json.Unmarshal(postBody, &createdGroup)
	suite.Require().NoError(err)

	err = deleteGroupByID(suite, createdGroup.Id)
	if err != nil {
		suite.T().Logf("Failed to delete nested group: %v", err)
	}
}

// Helper functions

// createOUForGroupTests creates an organization unit for group testing
func createOUForGroupTests(suite *GroupTreeAPITestSuite, ouRequest CreateOURequest) (string, error) {
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

// deleteOUByID deletes an organization unit by ID
func deleteOUByID(suite *GroupTreeAPITestSuite, id string) error {
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

// deleteGroupByID deletes a group by ID
func deleteGroupByID(suite *GroupTreeAPITestSuite, id string) error {
	req, err := http.NewRequest("DELETE", testServerURL+"/groups/"+id, nil)
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
