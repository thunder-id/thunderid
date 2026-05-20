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

type UserValidationEdgeCasesTestSuite struct {
	suite.Suite
	client         *http.Client
	createdSchemas []string // Track schemas for cleanup
	createdUsers   []string // Track users for cleanup
	oUID           string
}

var testUserValidationEdgeCasesOU = testutils.OrganizationUnit{
	Handle:      "test-user-validation-edge-cases-ou",
	Name:        "Test Organization Unit for User Validation Edge Cases",
	Description: "Organization unit created for user validation edge cases testing",
	Parent:      nil,
}

func TestUserValidationEdgeCasesTestSuite(t *testing.T) {
	suite.Run(t, new(UserValidationEdgeCasesTestSuite))
}

func (ts *UserValidationEdgeCasesTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()
	ts.createdSchemas = []string{}
	ts.createdUsers = []string{}

	// Create organization unit for tests
	ouID, err := testutils.CreateOrganizationUnit(testUserValidationEdgeCasesOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit: %v", err)
	}
	ts.oUID = ouID

	ts.createEmployeeSchema()
	ts.createSchemaWithNumbers()
	ts.createSchemaWithStringEnum()
	ts.createSchemaWithMixedEnum()
}

func (ts *UserValidationEdgeCasesTestSuite) TearDownSuite() {
	for _, userID := range ts.createdUsers {
		ts.deleteUser(userID)
	}
	for _, schemaID := range ts.createdSchemas {
		ts.deleteSchema(schemaID)
	}
	if ts.oUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.oUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit %s: %v", ts.oUID, err)
		}
	}
}

// TestCreateUserWithEmptyAttributes tests user creation with empty attributes
func (ts *UserValidationEdgeCasesTestSuite) TestCreateUserWithEmptyAttributes() {
	// Create a user with empty attributes (should succeed - all properties are optional)
	createUserReq := CreateUserRequest{
		OUID:       ts.oUID,
		Type:       "employee",
		Attributes: json.RawMessage(`{}`),
	}

	userID := ts.createUserAndExpectSuccess(createUserReq)
	ts.createdUsers = append(ts.createdUsers, userID)
}

// TestCreateUserWithNullAttributes tests user creation with null attributes
func (ts *UserValidationEdgeCasesTestSuite) TestCreateUserWithNullAttributes() {
	// Create a user with null attributes
	createUserReq := CreateUserRequest{
		OUID: ts.oUID,
		Type: "employee",
		// Attributes is nil/null
	}

	userID := ts.createUserAndExpectSuccess(createUserReq)
	ts.createdUsers = append(ts.createdUsers, userID)
}

// TestCreateUserWithPartialAttributes tests user creation with only some schema attributes
func (ts *UserValidationEdgeCasesTestSuite) TestCreateUserWithPartialAttributes() {
	// Create a user with only some of the schema attributes (should succeed)
	createUserReq := CreateUserRequest{
		OUID: ts.oUID,
		Type: "employee",
		Attributes: json.RawMessage(`{
			"given_name": "John",
			"email": "john@company.com"
		}`),
	}

	userID := ts.createUserAndExpectSuccess(createUserReq)
	ts.createdUsers = append(ts.createdUsers, userID)
}

// TestCreateUserWithExtraAttributes tests user creation with extra attributes not in schema
func (ts *UserValidationEdgeCasesTestSuite) TestCreateUserWithExtraAttributes() {
	// Create a user with extra attributes not defined in schema (should fail - strict validation)
	createUserReq := CreateUserRequest{
		OUID: ts.oUID,
		Type: "employee",
		Attributes: json.RawMessage(`{
			"given_name": "John",
			"family_name": "Doe",
			"email": "john.doe@company.com",
			"department": "Engineering",
			"isManager": false,
			"extraField": "this is not in schema",
			"anotherExtra": 123
		}`),
	}

	ts.createUserAndExpectError(createUserReq, "USR-1019")
}

// TestCreateUserWithNumberValidation tests user creation with number validation
func (ts *UserValidationEdgeCasesTestSuite) TestCreateUserWithNumberValidation() {
	// Test valid numbers
	createUserReq := CreateUserRequest{
		OUID: ts.oUID,
		Type: "numeric-user",
		Attributes: json.RawMessage(`{
			"age": 25,
			"salary": 50000.5,
			"rating": 4
		}`),
	}

	userID := ts.createUserAndExpectSuccess(createUserReq)
	ts.createdUsers = append(ts.createdUsers, userID)

	// Test invalid number type (string instead of number)
	createUserReq2 := CreateUserRequest{
		OUID: ts.oUID,
		Type: "numeric-user",
		Attributes: json.RawMessage(`{
			"age": "twenty-five",
			"salary": 50000.5,
			"rating": 4
		}`),
	}

	ts.createUserAndExpectError(createUserReq2, "USR-1019")
}

// TestCreateUserWithStringEnumValidation tests string enum validation
func (ts *UserValidationEdgeCasesTestSuite) TestCreateUserWithStringEnumValidation() {
	// Test valid enum value
	createUserReq := CreateUserRequest{
		OUID: ts.oUID,
		Type: "status-user",
		Attributes: json.RawMessage(`{
			"name": "Alice",
			"status": "active",
			"priority": "high"
		}`),
	}

	userID := ts.createUserAndExpectSuccess(createUserReq)
	ts.createdUsers = append(ts.createdUsers, userID)

	// Test invalid enum value
	createUserReq2 := CreateUserRequest{
		OUID: ts.oUID,
		Type: "status-user",
		Attributes: json.RawMessage(`{
			"name": "Bob",
			"status": "invalid-status",
			"priority": "high"
		}`),
	}

	ts.createUserAndExpectError(createUserReq2, "USR-1019")
}

// TestCreateUserWithMixedEnumValidation tests mixed type enum validation
func (ts *UserValidationEdgeCasesTestSuite) TestCreateUserWithMixedEnumValidation() {
	// Test valid enum values
	createUserReq := CreateUserRequest{
		OUID: ts.oUID,
		Type: "mixed-user",
		Attributes: json.RawMessage(`{
			"name": "Charlie",
			"level": 5,
			"isActive": true
		}`),
	}

	userID := ts.createUserAndExpectSuccess(createUserReq)
	ts.createdUsers = append(ts.createdUsers, userID)

	// Test invalid enum value for number
	createUserReq2 := CreateUserRequest{
		OUID: ts.oUID,
		Type: "mixed-user",
		Attributes: json.RawMessage(`{
			"name": "Dave",
			"level": 99,
			"isActive": true
		}`),
	}

	ts.createUserAndExpectError(createUserReq2, "USR-1019")
}

// TestUpdateUserChangeType tests updating user to different type with different schema
func (ts *UserValidationEdgeCasesTestSuite) TestUpdateUserChangeType() {
	// Create an employee user
	createUserReq := CreateUserRequest{
		OUID: ts.oUID,
		Type: "employee",
		Attributes: json.RawMessage(`{
			"given_name": "John",
			"family_name": "Doe",
			"email": "john.doe@company.com",
			"department": "Engineering",
			"isManager": false
		}`),
	}

	userID := ts.createUserAndExpectSuccess(createUserReq)
	ts.createdUsers = append(ts.createdUsers, userID)

	// Update user to different type with valid attributes for new schema
	updateUserReq := UpdateUserRequest{
		Type: "numeric-user",
		OUID: ts.oUID,
		Attributes: json.RawMessage(`{
			"age": 30,
			"salary": 75000.0,
			"rating": 5
		}`),
	}

	ts.updateUserAndExpectSuccess(userID, updateUserReq)

	// Update user to different type with invalid attributes for new schema
	updateUserReq2 := UpdateUserRequest{
		Type: "numeric-user",
		OUID: ts.oUID,
		Attributes: json.RawMessage(`{
			"age": "thirty",
			"salary": 75000.0,
			"rating": 5
		}`),
	}

	ts.updateUserAndExpectError(userID, updateUserReq2, "USR-1019")
}

// Helper methods to create different schema types

func (ts *UserValidationEdgeCasesTestSuite) createEmployeeSchema() string {
	schema := CreateUserTypeRequest{
		Name: "employee",
		OUID: ts.oUID,
		Schema: json.RawMessage(`{
			"given_name": {"type": "string"},
			"family_name": {"type": "string"},
			"email": {"type": "string"},
			"department": {"type": "string"},
			"isManager": {"type": "boolean"}
		}`),
	}

	return ts.createSchema(schema)
}

func (ts *UserValidationEdgeCasesTestSuite) createSchemaWithNumbers() string {
	schema := CreateUserTypeRequest{
		Name: "numeric-user",
		OUID: ts.oUID,
		Schema: json.RawMessage(`{
			"age": {"type": "number"},
			"salary": {"type": "number"},
			"rating": {"type": "number", "enum": [1, 2, 3, 4, 5]}
		}`),
	}

	return ts.createSchema(schema)
}

func (ts *UserValidationEdgeCasesTestSuite) createSchemaWithStringEnum() string {
	schema := CreateUserTypeRequest{
		Name: "status-user",
		OUID: ts.oUID,
		Schema: json.RawMessage(`{
			"name": {"type": "string"},
			"status": {"type": "string", "enum": ["active", "inactive", "pending"]},
			"priority": {"type": "string", "enum": ["low", "medium", "high"]}
		}`),
	}

	return ts.createSchema(schema)
}

func (ts *UserValidationEdgeCasesTestSuite) createSchemaWithMixedEnum() string {
	schema := CreateUserTypeRequest{
		Name: "mixed-user",
		OUID: ts.oUID,
		Schema: json.RawMessage(`{
			"name": {"type": "string"},
			"level": {"type": "number", "enum": [1, 2, 3, 4, 5]},
			"isActive": {"type": "boolean"}
		}`),
	}

	return ts.createSchema(schema)
}

func (ts *UserValidationEdgeCasesTestSuite) createSchema(schema CreateUserTypeRequest) string {
	if schema.OUID == "" {
		schema.OUID = ts.oUID
	}

	jsonData, err := json.Marshal(schema)
	ts.Require().NoError(err, "Failed to marshal schema request")

	req, err := http.NewRequest("POST", testServerURL+"/user-types", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err, "Failed to create schema request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send schema request")
	defer resp.Body.Close()

	ts.Require().Equal(201, resp.StatusCode, "Schema creation should succeed")

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read schema response body")

	var createdSchema UserType
	err = json.Unmarshal(body, &createdSchema)
	ts.Require().NoError(err, "Failed to unmarshal schema response")

	ts.createdSchemas = append(ts.createdSchemas, createdSchema.ID)
	return createdSchema.ID
}

func (ts *UserValidationEdgeCasesTestSuite) createUserAndExpectSuccess(createUserReq CreateUserRequest) string {
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

func (ts *UserValidationEdgeCasesTestSuite) createUserAndExpectError(createUserReq CreateUserRequest, expectedErrorCode string) {
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

	ts.Require().Equal(400, resp.StatusCode, "User creation should fail with validation error")

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	ts.Require().NoError(err, "Failed to unmarshal error response")
	ts.Require().Equal(expectedErrorCode, errorResp.Code, "Error code should match expected")
}

func (ts *UserValidationEdgeCasesTestSuite) updateUserAndExpectSuccess(userID string, updateUserReq UpdateUserRequest) {
	jsonData, err := json.Marshal(updateUserReq)
	ts.Require().NoError(err, "Failed to marshal update user request")

	req, err := http.NewRequest("PUT", testServerURL+"/users/"+userID, bytes.NewBuffer(jsonData))
	ts.Require().NoError(err, "Failed to create update user request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send update user request")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read update user response body")

	if resp.StatusCode != 200 {
		ts.T().Logf("User update failed with status %d: %s", resp.StatusCode, string(body))
	}
	ts.Require().Equal(200, resp.StatusCode, "User update should succeed")
}

func (ts *UserValidationEdgeCasesTestSuite) updateUserAndExpectError(userID string, updateUserReq UpdateUserRequest, expectedErrorCode string) {
	jsonData, err := json.Marshal(updateUserReq)
	ts.Require().NoError(err, "Failed to marshal update user request")

	req, err := http.NewRequest("PUT", testServerURL+"/users/"+userID, bytes.NewBuffer(jsonData))
	ts.Require().NoError(err, "Failed to create update user request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send update user request")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read update user response body")

	ts.Require().Equal(400, resp.StatusCode, "User update should fail with validation error")

	var errorResp ErrorResponse
	err = json.Unmarshal(body, &errorResp)
	ts.Require().NoError(err, "Failed to unmarshal error response")
	ts.Require().Equal(expectedErrorCode, errorResp.Code, "Error code should match expected")
}

func (ts *UserValidationEdgeCasesTestSuite) deleteUser(userID string) {
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

func (ts *UserValidationEdgeCasesTestSuite) deleteSchema(schemaID string) {
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
