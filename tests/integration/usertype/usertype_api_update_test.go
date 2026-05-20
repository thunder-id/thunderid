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

type UpdateUserTypeTestSuite struct {
	suite.Suite
	client          *http.Client
	testSchemaID    string
	anotherSchemaID string
	oUID            string
}

var testUserTypeAPIUpdateOU = testutils.OrganizationUnit{
	Handle:      "test-user-type-api-update-ou",
	Name:        "Test Organization Unit for User Type API Update",
	Description: "Organization unit created for user type API update testing",
	Parent:      nil,
}

func TestUpdateUserTypeTestSuite(t *testing.T) {
	suite.Run(t, new(UpdateUserTypeTestSuite))
}

func (ts *UpdateUserTypeTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	// Create organization unit for tests
	ouID, err := testutils.CreateOrganizationUnit(testUserTypeAPIUpdateOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit: %v", err)
	}
	ts.oUID = ouID

	// Create test schemas for update tests
	schema1 := CreateUserTypeRequest{
		Name: "update-test-schema-1",
		Schema: json.RawMessage(`{
			"originalField": {"type": "string"}
		}`),
	}
	schema1.OUID = ts.oUID

	schema2 := CreateUserTypeRequest{
		Name: "update-test-schema-2",
		Schema: json.RawMessage(`{
			"anotherField": {"type": "string"}
		}`),
	}
	schema2.OUID = ts.oUID

	ts.testSchemaID = ts.createTestSchema(schema1)
	ts.anotherSchemaID = ts.createTestSchema(schema2)
}

func (ts *UpdateUserTypeTestSuite) TearDownSuite() {
	// Clean up test schemas
	if ts.testSchemaID != "" {
		ts.deleteTestSchema(ts.testSchemaID)
	}
	if ts.anotherSchemaID != "" {
		ts.deleteTestSchema(ts.anotherSchemaID)
	}

	// Clean up created organization units
	if ts.oUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.oUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit %s: %v", ts.oUID, err)
		}
	}
}

// TestUpdateUserType tests PUT /user-types/{id} with valid data
func (ts *UpdateUserTypeTestSuite) TestUpdateUserType() {
	updateRequest := UpdateUserTypeRequest{
		Name: "updated-schema-name",
		Schema: json.RawMessage(`{
            "updatedField": {"type": "string", "required": true},
            "newField": {"type": "number"},
            "complexField": {
                "type": "object",
                "properties": {
                    "nestedField": {"type": "boolean", "required": true}
                }
            }
        }`),
	}
	updateRequest.OUID = ts.oUID

	jsonData, err := json.Marshal(updateRequest)
	if err != nil {
		ts.T().Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("PUT", testServerURL+"/user-types/"+ts.testSchemaID, bytes.NewBuffer(jsonData))
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusOK, resp.StatusCode, "Should return 200 OK")

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		ts.T().Fatalf("Failed to read response body: %v", err)
	}

	var updatedSchema UserType
	err = json.Unmarshal(bodyBytes, &updatedSchema)
	if err != nil {
		ts.T().Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify updated schema according to API spec
	ts.Assert().Equal(ts.testSchemaID, updatedSchema.ID, "ID should remain the same")
	ts.Assert().Equal(updateRequest.Name, updatedSchema.Name, "Name should be updated")
	ts.Assert().JSONEq(string(updateRequest.Schema), string(updatedSchema.Schema), "Schema data should be updated")
}

// TestUpdateUserTypeNotFound tests PUT /user-types/{id} with non-existent ID
func (ts *UpdateUserTypeTestSuite) TestUpdateUserTypeNotFound() {
	nonExistentID := "550e8400-e29b-41d4-a716-446655440000"

	updateRequest := UpdateUserTypeRequest{
		Name:   "updated-name",
		Schema: json.RawMessage(`{"field": {"type": "string"}}`),
	}
	updateRequest.OUID = ts.oUID

	jsonData, err := json.Marshal(updateRequest)
	if err != nil {
		ts.T().Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("PUT", testServerURL+"/user-types/"+nonExistentID, bytes.NewBuffer(jsonData))
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusNotFound, resp.StatusCode, "Should return 404 Not Found")

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		ts.T().Fatalf("Failed to read response body: %v", err)
	}

	var errorResp ErrorResponse
	err = json.Unmarshal(bodyBytes, &errorResp)
	if err != nil {
		ts.T().Fatalf("Failed to unmarshal error response: %v", err)
	}

	ts.Assert().NotEmpty(errorResp.Code, "Error should have code")
	ts.Assert().NotEmpty(errorResp.Message.DefaultValue, "Error should have message")
}

// TestUpdateUserTypeWithNameConflict tests PUT /user-types/{id} with conflicting name
func (ts *UpdateUserTypeTestSuite) TestUpdateUserTypeWithNameConflict() {
	// Try to update first schema with the name of the second schema
	updateRequest := UpdateUserTypeRequest{
		Name:   "update-test-schema-2", // Name of another existing schema
		Schema: json.RawMessage(`{"conflictField": {"type": "string"}}`),
	}
	updateRequest.OUID = ts.oUID

	jsonData, err := json.Marshal(updateRequest)
	if err != nil {
		ts.T().Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("PUT", testServerURL+"/user-types/"+ts.testSchemaID, bytes.NewBuffer(jsonData))
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusConflict, resp.StatusCode, "Should return 409 Conflict for name conflict")

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		ts.T().Fatalf("Failed to read response body: %v", err)
	}

	var errorResp ErrorResponse
	err = json.Unmarshal(bodyBytes, &errorResp)
	if err != nil {
		ts.T().Fatalf("Failed to unmarshal error response: %v", err)
	}

	ts.Assert().NotEmpty(errorResp.Code, "Error should have code")
	ts.Assert().NotEmpty(errorResp.Message.DefaultValue, "Error should have message")
}

// TestUpdateUserTypeWithInvalidData tests PUT /user-types/{id} with invalid request data
func (ts *UpdateUserTypeTestSuite) TestUpdateUserTypeWithInvalidData() {
	testCases := []struct {
		name        string
		requestBody string
	}{
		{
			name:        "empty name",
			requestBody: `{"name": "", "schema": {"field": {"type": "string"}}}`,
		},
		{
			name:        "missing name",
			requestBody: `{"schema": {"field": {"type": "string"}}}`,
		},
		{
			name:        "empty schema",
			requestBody: `{"name": "updated-name", "schema": {}}`,
		},
		{
			name:        "missing schema",
			requestBody: `{"name": "updated-name"}`,
		},
		{
			name:        "invalid JSON",
			requestBody: `{"name": "updated-name", "schema": invalid}`,
		},
		{
			name:        "malformed JSON",
			requestBody: `{"name": "updated-name"`,
		},
		{
			name:        "non-boolean required flag",
			requestBody: `{"name": "updated-name", "schema": {"field": {"type": "string", "required": "yes"}}}`,
		},
	}

	for _, tc := range testCases {
		ts.T().Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest("PUT", testServerURL+"/user-types/"+ts.testSchemaID, bytes.NewBufferString(tc.requestBody))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := ts.client.Do(req)
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode, "Should return 400 Bad Request for: %s", tc.name)

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			var errorResp ErrorResponse
			err = json.Unmarshal(bodyBytes, &errorResp)
			if err != nil {
				t.Fatalf("Failed to unmarshal error response: %v", err)
			}

			ts.Assert().NotEmpty(errorResp.Code, "Error should have code")
			ts.Assert().NotEmpty(errorResp.Message.DefaultValue, "Error should have message")
		})
	}
}

// TestUpdateUserTypeWithComplexData tests PUT /user-types/{id} with complex schema
func (ts *UpdateUserTypeTestSuite) TestUpdateUserTypeWithComplexData() {
	updateRequest := UpdateUserTypeRequest{
		Name: "complex-updated-schema",
		Schema: json.RawMessage(`{
			"user": {
				"type": "object",
				"properties": {
					"profile": {
						"type": "object",
						"properties": {
							"personalInfo": {
								"type": "object",
								"properties": {
									"given_name": {"type": "string"},
									"family_name": {"type": "string"},
									"dateOfBirth": {"type": "string", "regex": "^\\d{4}-\\d{2}-\\d{2}$"}
								}
							},
							"contactInfo": {
								"type": "object",
								"properties": {
									"email": {
										"type": "string",
										"regex": "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
									},
									"phone": {"type": "string"}
								}
							}
						}
					},
					"preferences": {
						"type": "array",
						"items": {
							"type": "object",
							"properties": {
								"category": {"type": "string"},
								"enabled": {"type": "boolean"}
							}
						}
					}
				}
			}
		}`),
	}
	updateRequest.OUID = ts.oUID

	jsonData, err := json.Marshal(updateRequest)
	if err != nil {
		ts.T().Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("PUT", testServerURL+"/user-types/"+ts.testSchemaID, bytes.NewBuffer(jsonData))
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusOK, resp.StatusCode, "Should return 200 OK")

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		ts.T().Fatalf("Failed to read response body: %v", err)
	}

	var updatedSchema UserType
	err = json.Unmarshal(bodyBytes, &updatedSchema)
	if err != nil {
		ts.T().Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify complex schema was updated correctly
	ts.Assert().Equal(ts.testSchemaID, updatedSchema.ID, "ID should remain the same")
	ts.Assert().Equal(updateRequest.Name, updatedSchema.Name, "Name should be updated")
	ts.Assert().JSONEq(string(updateRequest.Schema), string(updatedSchema.Schema), "Complex schema data should be updated")
}

// Helper function to create a test schema
func (ts *UpdateUserTypeTestSuite) createTestSchema(schema CreateUserTypeRequest) string {
	if schema.OUID == "" {
		schema.OUID = ts.oUID
	}

	jsonData, err := json.Marshal(schema)
	if err != nil {
		ts.T().Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", testServerURL+"/user-types", bytes.NewBuffer(jsonData))
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Expected status 201, got %d. Response: %s", resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		ts.T().Fatalf("Failed to read response body: %v", err)
	}

	var createdSchema UserType
	err = json.Unmarshal(bodyBytes, &createdSchema)
	if err != nil {
		ts.T().Fatalf("Failed to unmarshal response: %v", err)
	}

	return createdSchema.ID
}

// Helper function to delete a test schema
func (ts *UpdateUserTypeTestSuite) deleteTestSchema(schemaID string) {
	req, err := http.NewRequest("DELETE", testServerURL+"/user-types/"+schemaID, nil)
	if err != nil {
		return
	}

	resp, err := ts.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}
