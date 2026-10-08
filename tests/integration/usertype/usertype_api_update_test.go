// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package usertype

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
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
		Handle:      "update-test-schema-1",
		DisplayName: "Update Test Schema 1",
		Schema: json.RawMessage(`{
			"originalField": {"type": "string"}
		}`),
	}
	schema1.OUID = ts.oUID

	schema2 := CreateUserTypeRequest{
		Handle:      "update-test-schema-2",
		DisplayName: "Update Test Schema 2",
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
		Handle:      "update-test-schema-1",
		DisplayName: "Updated Schema Name",
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
	ts.Assert().Equal(updateRequest.Handle, updatedSchema.Handle, "Handle should be unchanged")
	ts.Assert().Equal(updateRequest.DisplayName, updatedSchema.DisplayName, "Display name should be updated")
	ts.Assert().JSONEq(string(updateRequest.Schema), string(updatedSchema.Schema), "Schema data should be updated")
}

// TestUpdateUserTypeNotFound tests PUT /user-types/{id} with non-existent ID
func (ts *UpdateUserTypeTestSuite) TestUpdateUserTypeNotFound() {
	nonExistentID := "550e8400-e29b-41d4-a716-446655440000"

	updateRequest := UpdateUserTypeRequest{
		Handle:      "updated-name",
		DisplayName: "Updated Name",
		Schema:      json.RawMessage(`{"field": {"type": "string"}}`),
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

// TestUpdateUserTypeWithHandleChange tests PUT /user-types/{id} with a handle other than the
// stored one. The handle is immutable, so the request is rejected whether or not the new handle
// is already taken.
func (ts *UpdateUserTypeTestSuite) TestUpdateUserTypeWithHandleChange() {
	// Try to update first schema with the handle of the second schema
	updateRequest := UpdateUserTypeRequest{
		Handle:      "update-test-schema-2", // Handle of another existing schema
		DisplayName: "Update Test Schema 2",
		Schema:      json.RawMessage(`{"conflictField": {"type": "string"}}`),
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

	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode, "Should return 400 Bad Request for a handle change")

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		ts.T().Fatalf("Failed to read response body: %v", err)
	}

	var errorResp ErrorResponse
	err = json.Unmarshal(bodyBytes, &errorResp)
	if err != nil {
		ts.T().Fatalf("Failed to unmarshal error response: %v", err)
	}

	ts.Assert().Equal("USRS-1017", errorResp.Code, "Error should be the handle update not allowed code")
	ts.Assert().NotEmpty(errorResp.Message.DefaultValue, "Error should have message")
}

// TestUpdateUserTypeWithoutHandle tests PUT /user-types/{id} without a handle. The handle is
// immutable, so an update that omits it keeps the existing one.
func (ts *UpdateUserTypeTestSuite) TestUpdateUserTypeWithoutHandle() {
	status, body := ts.sendJSON(http.MethodPut, "/user-types/"+ts.anotherSchemaID, UpdateUserTypeRequest{
		DisplayName: "Update Test Schema 2 Renamed",
		OUID:        ts.oUID,
		Schema:      json.RawMessage(`{"anotherField": {"type": "string"}}`),
	})
	ts.Require().Equal(http.StatusOK, status, "Should return 200 OK. Response: %s", string(body))

	var updatedSchema UserType
	ts.Require().NoError(json.Unmarshal(body, &updatedSchema))
	ts.Assert().Equal("update-test-schema-2", updatedSchema.Handle, "Handle should be unchanged")
	ts.Assert().Equal("Update Test Schema 2 Renamed", updatedSchema.DisplayName, "Display name should be updated")
}

// TestUpdateUserTypeTrimsDisplayName tests that surrounding whitespace in the display name is trimmed on update.
func (ts *UpdateUserTypeTestSuite) TestUpdateUserTypeTrimsDisplayName() {
	schemaID := ts.createTestSchema(CreateUserTypeRequest{
		Handle:      "update-test-trim-display-name",
		DisplayName: "Trim Display Name",
		OUID:        ts.oUID,
		Schema:      json.RawMessage(`{"field": {"type": "string"}}`),
	})
	defer ts.deleteTestSchema(schemaID)

	status, body := ts.sendJSON(http.MethodPut, "/user-types/"+schemaID, UpdateUserTypeRequest{
		DisplayName: "  Trimmed Display Name  ",
		OUID:        ts.oUID,
		Schema:      json.RawMessage(`{"field": {"type": "string"}}`),
	})
	ts.Require().Equal(http.StatusOK, status, "Should return 200 OK. Response: %s", string(body))

	var updatedSchema UserType
	ts.Require().NoError(json.Unmarshal(body, &updatedSchema))
	ts.Assert().Equal("Trimmed Display Name", updatedSchema.DisplayName, "Display name should be trimmed")
}

// TestUpdateUserTypeDisplayNameKeepsUsers tests that changing the display name of a user type does
// not affect its existing users, which reference the type by its handle.
func (ts *UpdateUserTypeTestSuite) TestUpdateUserTypeDisplayNameKeepsUsers() {
	const handle = "update-display-name-users"
	schema := json.RawMessage(`{"username": {"type": "string", "unique": true}}`)

	schemaID := ts.createTestSchema(CreateUserTypeRequest{
		Handle:           handle,
		DisplayName:      "Update Display Name Users",
		OUID:             ts.oUID,
		SystemAttributes: &SystemAttributes{Display: "username"},
		Schema:           schema,
	})
	defer ts.deleteTestSchema(schemaID)

	userID, err := testutils.CreateUser(testutils.User{
		Type:       handle,
		OUID:       ts.oUID,
		Attributes: json.RawMessage(`{"username": "display-name-update-user"}`),
	})
	ts.Require().NoError(err, "Failed to create user")
	defer func() {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("Failed to delete user %s: %v", userID, err)
		}
	}()

	status, body := ts.sendJSON(http.MethodPut, "/user-types/"+schemaID, UpdateUserTypeRequest{
		DisplayName:      "Update Display Name Users Renamed",
		OUID:             ts.oUID,
		SystemAttributes: &SystemAttributes{Display: "username"},
		Schema:           schema,
	})
	ts.Require().Equal(http.StatusOK, status, "Should return 200 OK. Response: %s", string(body))

	// The existing user still resolves its type, display attribute and attributes.
	status, body = ts.sendJSON(http.MethodGet, "/users/"+userID+"?include=display", nil)
	ts.Require().Equal(http.StatusOK, status, "Response: %s", string(body))

	var user testutils.User
	ts.Require().NoError(json.Unmarshal(body, &user))
	ts.Assert().Equal(handle, user.Type, "User should still reference the type handle")
	ts.Assert().Equal("display-name-update-user", user.Display, "Display attribute should still resolve")

	// The type is referenced by its handle, not by its display name.
	status, body = ts.sendJSON(http.MethodPost, "/users", CreateUserRequest{
		OUID:       ts.oUID,
		Type:       "Update Display Name Users Renamed",
		Attributes: json.RawMessage(`{"username": "display-name-reference"}`),
	})
	ts.Assert().Equal(http.StatusBadRequest, status, "Display name should not resolve a type. Response: %s",
		string(body))
}

// TestUpdateUserTypeWithInvalidData tests PUT /user-types/{id} with invalid request data
func (ts *UpdateUserTypeTestSuite) TestUpdateUserTypeWithInvalidData() {
	testCases := []struct {
		name        string
		requestBody string
	}{
		{
			name: "empty display name",
			requestBody: `{"handle": "update-test-schema-1", "displayName": "", ` +
				`"schema": {"field": {"type": "string"}}}`,
		},
		{
			name:        "missing display name",
			requestBody: `{"handle": "update-test-schema-1", "schema": {"field": {"type": "string"}}}`,
		},
		{
			name:        "empty schema",
			requestBody: `{"handle": "update-test-schema-1", "displayName": "Updated Name", "schema": {}}`,
		},
		{
			name:        "missing schema",
			requestBody: `{"handle": "update-test-schema-1", "displayName": "Updated Name"}`,
		},
		{
			name:        "invalid JSON",
			requestBody: `{"handle": "update-test-schema-1", "displayName": "Updated Name", "schema": invalid}`,
		},
		{
			name:        "malformed JSON",
			requestBody: `{"handle": "update-test-schema-1", "displayName": "Updated Name"`,
		},
		{
			name: "non-boolean required flag",
			requestBody: `{"handle": "update-test-schema-1", "displayName": "Updated Name", ` +
				`"schema": {"field": {"type": "string", "required": "yes"}}}`,
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
		Handle:      "update-test-schema-1",
		DisplayName: "Complex Updated Schema",
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
	ts.Assert().Equal(updateRequest.Handle, updatedSchema.Handle, "Handle should be unchanged")
	ts.Assert().Equal(updateRequest.DisplayName, updatedSchema.DisplayName, "Display name should be updated")
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

// sendJSON sends a request with an optional JSON body and returns the status code and response body.
func (ts *UpdateUserTypeTestSuite) sendJSON(method, path string, body interface{}) (int, []byte) {
	var reader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			ts.T().Fatalf("Failed to marshal request: %v", err)
		}
		reader = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, testServerURL+path, reader)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		ts.T().Fatalf("Failed to read response body: %v", err)
	}
	return resp.StatusCode, respBody
}
