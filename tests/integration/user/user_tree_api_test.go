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

package user

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
	pathTestOU = testutils.OrganizationUnit{
		Handle:      "test-ou-for-users",
		Name:        "Test OU for Users",
		Description: "Test organization unit for user path-based operations",
	}

	testUserType = testutils.UserType{
		Name: "employee",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type":     "string",
				"unique":   true,
				"required": true,
			},
			"email": map[string]interface{}{
				"type":     "string",
				"required": true,
			},
			"department": map[string]interface{}{
				"type": "string",
			},
		},
	}
	employeeEntityTypeID string
)

// CreateUserByPathRequest represents the request body for creating a user by path.
type CreateUserByPathRequest struct {
	Type       string          `json:"type"`
	Groups     []string        `json:"groups,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
}

type UserTreeAPITestSuite struct {
	suite.Suite
	testOUID string
}

func TestUserTreeAPITestSuite(t *testing.T) {
	suite.Run(t, new(UserTreeAPITestSuite))
}

func (suite *UserTreeAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(pathTestOU)
	if err != nil {
		suite.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}

	testUserType.OUID = ouID
	schemaID, err := testutils.CreateUserType(testUserType)
	if err != nil {
		suite.T().Fatalf("Failed to create employee user type during setup: %v", err)
	}

	employeeEntityTypeID = schemaID

	suite.testOUID = ouID
	suite.T().Logf("Created test OU with ID: %s and handle: %s", suite.testOUID, pathTestOU.Handle)
}

func (suite *UserTreeAPITestSuite) TearDownSuite() {
	if employeeEntityTypeID != "" {
		if err := testutils.DeleteUserType(employeeEntityTypeID); err != nil {
			suite.T().Logf("Failed to delete employee user type during teardown: %v", err)
		}
	}

	if suite.testOUID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.testOUID); err != nil {
			suite.T().Logf("Failed to delete test OU during teardown: %v", err)
		}
	}
}

// TestGetUsersByPath tests retrieving users by organization unit handle path
func (suite *UserTreeAPITestSuite) TestGetUsersByPath() {
	if suite.testOUID == "" {
		suite.T().Fatal("OU ID is not available for path-based user retrieval")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/users/tree/"+pathTestOU.Handle, nil)
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

// TestCreateUserByPath tests creating a user by organization unit handle path
func (suite *UserTreeAPITestSuite) TestCreateUserByPath() {
	if suite.testOUID == "" {
		suite.T().Fatal("OU ID is not available for path-based user creation")
	}

	client := testutils.GetHTTPClient()

	createRequest := CreateUserByPathRequest{
		Type:       "employee",
		Attributes: json.RawMessage(`{"username": "test.user", "email": "test.user@example.com", "department": "Engineering"}`),
	}

	requestJSON, err := json.Marshal(createRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+"/users/tree/"+pathTestOU.Handle, bytes.NewBuffer(requestJSON))
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

	var createdUser testutils.User
	err = json.Unmarshal(body, &createdUser)
	suite.Require().NoError(err)

	// Verify the created user
	suite.NotEmpty(createdUser.ID)
	suite.Equal(suite.testOUID, createdUser.OUID)
	suite.Equal("employee", createdUser.Type)
	suite.NotEmpty(createdUser.Attributes)

	// Clean up: delete the created user
	if err := testutils.DeleteUser(createdUser.ID); err != nil {
		suite.T().Logf("Failed to delete created user: %v", err)
	}
}

// TestGetUsersByInvalidPath tests retrieving users by invalid organization unit handle path
func (suite *UserTreeAPITestSuite) TestGetUsersByInvalidPath() {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/users/tree/nonexistent-ou", nil)
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

	var errorResp testutils.ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	suite.Require().NoError(err)

	suite.Equal("USR-1005", errorResp.Code)
	suite.Equal("Organization unit not found", errorResp.Message.DefaultValue)
}

// TestGetUsersByPathWithPagination tests retrieving users by path with pagination parameters
func (suite *UserTreeAPITestSuite) TestGetUsersByPathWithPagination() {
	if suite.testOUID == "" {
		suite.T().Fatal("OU ID is not available for pagination test")
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("GET", testServerURL+"/users/tree/"+pathTestOU.Handle+"?limit=5&offset=0", nil)
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

	var userListResponse testutils.UserListResponse
	err = json.Unmarshal(body, &userListResponse)
	suite.Require().NoError(err)

	// Verify pagination parameters
	suite.GreaterOrEqual(userListResponse.TotalResults, 0)
	suite.Equal(userListResponse.StartIndex, 1)
	suite.LessOrEqual(userListResponse.Count, 5)
}
