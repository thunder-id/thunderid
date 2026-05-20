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

package usertype

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

// UserUniquenessTestSuite contains tests for user uniqueness validation
type UserUniquenessTestSuite struct {
	suite.Suite
	client         *http.Client
	createdSchemas []string // Track schemas for cleanup
	createdUsers   []string // Track users for cleanup
	oUID           string
}

var testUserUniquenessValidationOU = testutils.OrganizationUnit{
	Handle:      "test-user-uniqueness-validation-ou",
	Name:        "Test Organization Unit for User Uniqueness Validation",
	Description: "Organization unit created for user uniqueness validation testing",
	Parent:      nil,
}

func TestUserUniquenessTestSuite(t *testing.T) {
	suite.Run(t, new(UserUniquenessTestSuite))
}

func (ts *UserUniquenessTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()
	ts.createdSchemas = []string{}
	ts.createdUsers = []string{}

	// Create organization unit for tests
	ouID, err := testutils.CreateOrganizationUnit(testUserUniquenessValidationOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit: %v", err)
	}
	ts.oUID = ouID
}

func (ts *UserUniquenessTestSuite) TearDownSuite() {
	// Clean up created users first
	for _, userID := range ts.createdUsers {
		ts.deleteUser(userID)
	}
	// Then clean up created schemas
	for _, schemaID := range ts.createdSchemas {
		ts.deleteSchema(schemaID)
	}
	// Finally clean up created organization unit
	if ts.oUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.oUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit %s: %v", ts.oUID, err)
		}
	}
}

// TestCreateUserWithUniqueConstraintViolation tests that creating a user with duplicate unique fields fails
func (ts *UserUniquenessTestSuite) TestCreateUserWithUniqueConstraintViolation() {
	// Create a schema with unique constraints
	uniqueSchemaID := ts.createSchemaWithUniqueFields()
	ts.createdSchemas = append(ts.createdSchemas, uniqueSchemaID)

	// Create first user - should succeed
	createUserReq1 := CreateUserRequest{
		OUID: ts.oUID,
		Type: "unique-employee",
		Attributes: json.RawMessage(`{
			"username": "john_doe",
			"email": "john.doe@company.com",
			"employeeId": "EMP001",
			"department": "Engineering"
		}`),
	}

	userID1 := ts.createUserAndExpectSuccess(createUserReq1)
	ts.createdUsers = append(ts.createdUsers, userID1)

	// Try to create second user with same username (unique field) - should fail
	createUserReq2 := CreateUserRequest{
		OUID: ts.oUID,
		Type: "unique-employee",
		Attributes: json.RawMessage(`{
			"username": "john_doe",
			"email": "jane.smith@company.com",
			"employeeId": "EMP002",
			"department": "Marketing"
		}`),
	}

	ts.createUserAndExpectError(createUserReq2, "USR-1014")

	// Try to create third user with same email (unique field) - should fail
	createUserReq3 := CreateUserRequest{
		OUID: ts.oUID,
		Type: "unique-employee",
		Attributes: json.RawMessage(`{
			"username": "alice_brown",
			"email": "john.doe@company.com",
			"employeeId": "EMP003",
			"department": "HR"
		}`),
	}

	ts.createUserAndExpectError(createUserReq3, "USR-1014")

	// Try to create fourth user with same employeeId (unique field) - should fail
	createUserReq4 := CreateUserRequest{
		OUID: ts.oUID,
		Type: "unique-employee",
		Attributes: json.RawMessage(`{
			"username": "bob_wilson",
			"email": "bob.wilson@company.com",
			"employeeId": "EMP001",
			"department": "Finance"
		}`),
	}

	ts.createUserAndExpectError(createUserReq4, "USR-1014")

	// Create user with all different unique values - should succeed
	createUserReq5 := CreateUserRequest{
		OUID: ts.oUID,
		Type: "unique-employee",
		Attributes: json.RawMessage(`{
			"username": "charlie_davis",
			"email": "charlie.davis@company.com",
			"employeeId": "EMP004",
			"department": "IT"
		}`),
	}

	userID5 := ts.createUserAndExpectSuccess(createUserReq5)
	ts.createdUsers = append(ts.createdUsers, userID5)
}

// Helper methods

func (ts *UserUniquenessTestSuite) createSchemaWithUniqueFields() string {
	schema := CreateUserTypeRequest{
		Name: "unique-employee",
		OUID: ts.oUID,
		Schema: json.RawMessage(`{
			"username": {"type": "string", "unique": true},
			"email": {"type": "string", "unique": true},
			"employeeId": {"type": "string", "unique": true},
			"department": {"type": "string"}
		}`),
	}

	return ts.createSchema(schema)
}

func (ts *UserUniquenessTestSuite) createSchema(schema CreateUserTypeRequest) string {
	jsonData, err := json.Marshal(schema)
	ts.Require().NoError(err, "Failed to marshal schema request")

	req, err := http.NewRequest("POST", testServerURL+"/user-types", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err, "Failed to create schema request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send schema request")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read schema response body")

	if resp.StatusCode != 201 {
		ts.T().Logf("Schema creation failed with status %d: %s", resp.StatusCode, string(body))
	}
	ts.Require().Equal(201, resp.StatusCode, "Schema creation should succeed")

	var createdSchema UserType
	err = json.Unmarshal(body, &createdSchema)
	ts.Require().NoError(err, "Failed to unmarshal schema response")

	return createdSchema.ID
}

func (ts *UserUniquenessTestSuite) createUserAndExpectSuccess(createUserReq CreateUserRequest) string {
	jsonData, err := json.Marshal(createUserReq)
	ts.Require().NoError(err, "Failed to marshal user request")

	req, err := http.NewRequest("POST", testServerURL+"/users", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err, "Failed to create user request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send user request")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read user response body")

	if resp.StatusCode != 201 {
		ts.T().Logf("User creation failed with status %d: %s", resp.StatusCode, string(body))
	}
	ts.Require().Equal(201, resp.StatusCode, "User creation should succeed")

	var createdUser testutils.User
	err = json.Unmarshal(body, &createdUser)
	ts.Require().NoError(err, "Failed to unmarshal user response")

	return createdUser.ID
}

func (ts *UserUniquenessTestSuite) createUserAndExpectError(createUserReq CreateUserRequest, expectedErrorCode string) {
	jsonData, err := json.Marshal(createUserReq)
	ts.Require().NoError(err, "Failed to marshal user request")

	req, err := http.NewRequest("POST", testServerURL+"/users", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err, "Failed to create user request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send user request")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read user response body")

	ts.Require().Equal(409, resp.StatusCode, "User creation should fail with validation error")

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	ts.Require().NoError(err, "Failed to unmarshal error response")
	ts.Require().Equal(expectedErrorCode, errorResp.Code, "Error code should match expected")
}

func (ts *UserUniquenessTestSuite) deleteUser(userID string) {
	req, err := http.NewRequest("DELETE", testServerURL+"/users/"+userID, nil)
	if err != nil {
		ts.T().Logf("Failed to create delete user request: %v", err)
		return
	}

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Logf("Failed to send delete user request: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		ts.T().Logf("Failed to delete user %s: status %d, body: %s", userID, resp.StatusCode, string(body))
	}
}

func (ts *UserUniquenessTestSuite) deleteSchema(schemaID string) {
	req, err := http.NewRequest("DELETE", testServerURL+"/user-types/"+schemaID, nil)
	if err != nil {
		ts.T().Logf("Failed to create delete schema request: %v", err)
		return
	}

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Logf("Failed to send delete schema request: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		ts.T().Logf("Failed to delete schema %s: status %d, body: %s", schemaID, resp.StatusCode, string(body))
	}
}
