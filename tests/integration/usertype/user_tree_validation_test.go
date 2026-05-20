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
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

type UserTreeValidationTestSuite struct {
	suite.Suite
	client         *http.Client
	createdSchemas []string // Track schemas for cleanup
	createdUsers   []string // Track users for cleanup
	createdOUs     []string // Track organization units for cleanup
	ou1ID          string   // ID of ou1
	ou2ID          string   // ID of ou2 (child of ou1)
}

var testUserTreeValidationOU = testutils.OrganizationUnit{
	Handle:      "test-user-tree-validation-ou",
	Name:        "Test Organization Unit for User Tree Validation",
	Description: "Organization unit created for user tree validation testing",
	Parent:      nil,
}

func TestUserTreeValidationTestSuite(t *testing.T) {
	suite.Run(t, new(UserTreeValidationTestSuite))
}

func (ts *UserTreeValidationTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()
	ts.createdSchemas = []string{}
	ts.createdUsers = []string{}
	ts.createdOUs = []string{}

	// Create organization units for testing
	ts.createOrganizationUnits()
}

func (ts *UserTreeValidationTestSuite) TearDownSuite() {
	// Clean up created users first
	for _, userID := range ts.createdUsers {
		ts.deleteUser(userID)
	}
	// Then clean up created schemas
	for _, schemaID := range ts.createdSchemas {
		ts.deleteSchema(schemaID)
	}
	// Finally clean up created organization units in reverse order (children first)
	for i := len(ts.createdOUs) - 1; i >= 0; i-- {
		if err := testutils.DeleteOrganizationUnit(ts.createdOUs[i]); err != nil {
			ts.T().Logf("Failed to delete OU %s: %v", ts.createdOUs[i], err)
		}
	}
}

// TestCreateUserByPathWithValidSchema tests user creation via tree API with valid schema
func (ts *UserTreeValidationTestSuite) TestCreateUserByPathWithValidSchema() {
	// First create a user type
	_, schemaName := ts.createEmployeeSchema()

	// Create a user that conforms to the schema via tree API
	createUserReq := CreateUserByPathRequest{
		Type: schemaName,
		Attributes: json.RawMessage(`{
			"given_name": "Alice",
			"family_name": "Johnson",
			"email": "alice.johnson@company.com",
			"department": "Marketing",
			"isManager": true
		}`),
	}

	userID := ts.createUserByPathAndExpectSuccess("ou1/ou2", createUserReq)
	ts.createdUsers = append(ts.createdUsers, userID)
}

// TestCreateUserByPathWithInvalidSchema tests user creation via tree API with invalid schema
func (ts *UserTreeValidationTestSuite) TestCreateUserByPathWithInvalidSchema() {
	// Create a user type
	_, schemaName := ts.createEmployeeSchema()

	// Create a user with invalid data (wrong type for given_name)
	createUserReq := CreateUserByPathRequest{
		Type: schemaName,
		Attributes: json.RawMessage(`{
			"given_name": 456,
			"family_name": "Smith",
			"email": "invalid@company.com",
			"department": "HR",
			"isManager": false
		}`),
	}

	ts.createUserByPathAndExpectError("ou1/ou2", createUserReq, "USR-1019")
}

// TestCreateUserByPathWithComplexSchema tests user creation with complex nested schema
func (ts *UserTreeValidationTestSuite) TestCreateUserByPathWithComplexSchema() {
	// Create a complex schema
	_, schemaName := ts.createComplexSchema()

	// Test valid complex data
	createUserReq := CreateUserByPathRequest{
		Type: schemaName,
		Attributes: json.RawMessage(`{
			"name": "Sarah Wilson",
			"profile": {
				"bio": "Experienced manager",
				"skills": ["leadership", "strategy", "communication"],
				"ratings": {
					"performance": 4.8,
					"teamwork": 4.9
				}
			},
			"teams": ["engineering", "product"]
		}`),
	}

	userID := ts.createUserByPathAndExpectSuccess("ou1/ou2", createUserReq)
	ts.createdUsers = append(ts.createdUsers, userID)

	// Test invalid complex data (wrong type in nested array)
	createUserReq2 := CreateUserByPathRequest{
		Type: schemaName,
		Attributes: json.RawMessage(`{
			"name": "Bob Johnson",
			"profile": {
				"bio": "Another manager",
				"skills": ["leadership", 123, "communication"],
				"ratings": {
					"performance": 4.5,
					"teamwork": 4.7
				}
			},
			"teams": ["marketing", "sales"]
		}`),
	}

	ts.createUserByPathAndExpectError("ou1/ou2", createUserReq2, "USR-1019")
}

// Helper methods

func (ts *UserTreeValidationTestSuite) getUniqueName(baseName string) string {
	return fmt.Sprintf("%s_%d", baseName, time.Now().UnixNano())
}

func (ts *UserTreeValidationTestSuite) createEmployeeSchema() (string, string) {
	schemaName := ts.getUniqueName("employee")
	schema := CreateUserTypeRequest{
		Name: schemaName,
		Schema: json.RawMessage(`{
			"given_name": {"type": "string"},
			"family_name": {"type": "string"},
			"email": {"type": "string"},
			"department": {"type": "string"},
			"isManager": {"type": "boolean"}
		}`),
	}
	schema.OUID = ts.ou1ID

	schemaID := ts.createSchema(schema)
	return schemaID, schemaName
}

func (ts *UserTreeValidationTestSuite) createComplexSchema() (string, string) {
	schemaName := ts.getUniqueName("manager")
	schema := CreateUserTypeRequest{
		Name: schemaName,
		Schema: json.RawMessage(`{
			"name": {"type": "string"},
			"profile": {
				"type": "object",
				"properties": {
					"bio": {"type": "string"},
					"skills": {
						"type": "array",
						"items": {"type": "string"}
					},
					"ratings": {
						"type": "object",
						"properties": {
							"performance": {"type": "number"},
							"teamwork": {"type": "number"}
						}
					}
				}
			},
			"teams": {
				"type": "array",
				"items": {"type": "string"}
			}
		}`),
	}
	schema.OUID = ts.ou1ID

	schemaID := ts.createSchema(schema)
	return schemaID, schemaName
}

func (ts *UserTreeValidationTestSuite) createSchema(schema CreateUserTypeRequest) string {
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

func (ts *UserTreeValidationTestSuite) createUserByPathAndExpectSuccess(path string, createUserReq CreateUserByPathRequest) string {
	jsonData, err := json.Marshal(createUserReq)
	ts.Require().NoError(err, "Failed to marshal user request")

	req, err := http.NewRequest("POST", testServerURL+"/users/tree/"+path, bytes.NewBuffer(jsonData))
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

func (ts *UserTreeValidationTestSuite) createUserByPathAndExpectError(path string, createUserReq CreateUserByPathRequest, expectedErrorCode string) {
	jsonData, err := json.Marshal(createUserReq)
	ts.Require().NoError(err, "Failed to marshal user request")

	req, err := http.NewRequest("POST", testServerURL+"/users/tree/"+path, bytes.NewBuffer(jsonData))
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

func (ts *UserTreeValidationTestSuite) deleteUser(userID string) {
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

func (ts *UserTreeValidationTestSuite) deleteSchema(schemaID string) {
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

func (ts *UserTreeValidationTestSuite) createOrganizationUnits() {
	parentOU := testutils.OrganizationUnit{
		Handle:      "ou1",
		Name:        "Organization Unit 1",
		Description: "Test OU 1 for schema validation",
	}

	ou1ID, err := testutils.CreateOrganizationUnit(parentOU)
	ts.Require().NoError(err, "Failed to create parent organization unit")
	ts.ou1ID = ou1ID
	ts.createdOUs = append(ts.createdOUs, ou1ID)

	childOU := testutils.OrganizationUnit{
		Handle:      "ou2",
		Name:        "Organization Unit 2",
		Description: "Test OU 2 for schema validation",
		Parent:      &ts.ou1ID,
	}

	ou2ID, err := testutils.CreateOrganizationUnit(childOU)
	ts.Require().NoError(err, "Failed to create child organization unit")
	ts.ou2ID = ou2ID
	ts.createdOUs = append(ts.createdOUs, ou2ID)
}
