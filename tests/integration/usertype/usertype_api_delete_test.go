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
	"net/url"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

type DeleteUserTypeTestSuite struct {
	suite.Suite
	client *http.Client
	oUID   string
}

var testUserTypeAPIDeleteOU = testutils.OrganizationUnit{
	Handle:      "test-user-type-api-delete-ou",
	Name:        "Test Organization Unit for User Type API Delete",
	Description: "Organization unit created for user type API delete testing",
	Parent:      nil,
}

func TestDeleteUserTypeTestSuite(t *testing.T) {
	suite.Run(t, new(DeleteUserTypeTestSuite))
}

func (ts *DeleteUserTypeTestSuite) TearDownSuite() {
	if ts.oUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.oUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit %s: %v", ts.oUID, err)
		}
	}
}

func (ts *DeleteUserTypeTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	// Create organization unit for tests
	ouID, err := testutils.CreateOrganizationUnit(testUserTypeAPIDeleteOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit: %v", err)
	}
	ts.oUID = ouID
}

// TestDeleteUserType tests DELETE /user-types/{id} with valid ID
func (ts *DeleteUserTypeTestSuite) TestDeleteUserType() {
	// Create a schema to delete
	schema := CreateUserTypeRequest{
		Name: "schema-to-delete",
		Schema: json.RawMessage(`{
            "tempField": {"type": "string", "required": true},
            "description": {"type": "string"}
        }`),
	}
	schema.OUID = ts.oUID

	schemaID := ts.createTestSchema(schema)

	// Delete the schema
	req, err := http.NewRequest("DELETE", testServerURL+"/user-types/"+schemaID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusNoContent, resp.StatusCode, "Should return 204 No Content for successful deletion")

	// Verify schema is deleted by trying to get it
	getReq, err := http.NewRequest("GET", testServerURL+"/user-types/"+schemaID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create get request: %v", err)
	}

	getResp, err := ts.client.Do(getReq)
	if err != nil {
		ts.T().Fatalf("Failed to send get request: %v", err)
	}
	defer getResp.Body.Close()

	ts.Assert().Equal(http.StatusNotFound, getResp.StatusCode, "Schema should not exist after deletion")
}

// TestDeleteUserTypeNotFound tests DELETE /user-types/{id} with non-existent ID
func (ts *DeleteUserTypeTestSuite) TestDeleteUserTypeNotFound() {
	nonExistentID := "550e8400-e29b-41d4-a716-446655440000"

	req, err := http.NewRequest("DELETE", testServerURL+"/user-types/"+nonExistentID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusNoContent, resp.StatusCode, "Should return 204 No Content for non-existent schema (idempotent behavior)")

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		ts.T().Fatalf("Failed to read response body: %v", err)
	}
	ts.Assert().Empty(bodyBytes, "204 response should have no content body")
}

// TestDeleteUserTypeWithInvalidID tests DELETE /user-types/{id} with invalid ID formats
func (ts *DeleteUserTypeTestSuite) TestDeleteUserTypeWithInvalidID() {
	testCases := []struct {
		name           string
		schemaID       string
		expectedStatus int
	}{
		{
			name:           "invalid UUID format",
			schemaID:       "invalid-uuid-format",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "special characters in ID",
			schemaID:       "schema@#$%^&*()",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "very long ID",
			schemaID:       "very-long-id-that-exceeds-normal-uuid-length-and-should-be-handled-properly",
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tc := range testCases {
		ts.T().Run(tc.name, func(t *testing.T) {
			// URL-encode the schema ID to handle special characters
			encodedSchemaID := url.PathEscape(tc.schemaID)
			req, err := http.NewRequest("DELETE", testServerURL+"/user-types/"+encodedSchemaID, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := ts.client.Do(req)
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			// DELETE should be idempotent and return 204 even for invalid IDs
			ts.Assert().Equal(http.StatusNoContent, resp.StatusCode,
				"Should return 204 No Content for invalid ID (idempotent behavior) for case: %s", tc.name)

			// Verify response has no content body for 204
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}
			ts.Assert().Empty(bodyBytes, "204 response should have no content body")
		})
	}
}

// TestDeleteUserTypeIdempotency tests DELETE /user-types/{id} idempotency
func (ts *DeleteUserTypeTestSuite) TestDeleteUserTypeIdempotency() {
	// Create a schema to delete
	schema := CreateUserTypeRequest{
		Name: "idempotency-test-schema",
		Schema: json.RawMessage(`{
			"field": {"type": "string"}
		}`),
	}

	schemaID := ts.createTestSchema(schema)

	// Delete the schema first time
	req1, err := http.NewRequest("DELETE", testServerURL+"/user-types/"+schemaID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create first request: %v", err)
	}

	resp1, err := ts.client.Do(req1)
	if err != nil {
		ts.T().Fatalf("Failed to send first request: %v", err)
	}
	defer resp1.Body.Close()

	ts.Assert().Equal(http.StatusNoContent, resp1.StatusCode, "First deletion should return 204 No Content")

	// Delete the schema second time (should be idempotent)
	req2, err := http.NewRequest("DELETE", testServerURL+"/user-types/"+schemaID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create second request: %v", err)
	}

	resp2, err := ts.client.Do(req2)
	if err != nil {
		ts.T().Fatalf("Failed to send second request: %v", err)
	}
	defer resp2.Body.Close()

	ts.Assert().Equal(http.StatusNoContent, resp2.StatusCode, "Second deletion should return 204 No Content (idempotent behavior)")
}

// TestDeleteUserTypeMultiple tests deleting multiple schemas
func (ts *DeleteUserTypeTestSuite) TestDeleteUserTypeMultiple() {
	// Create multiple schemas
	schemas := []CreateUserTypeRequest{
		{
			Name:   "multi-delete-schema-1",
			Schema: json.RawMessage(`{"field1": {"type": "string"}}`),
		},
		{
			Name:   "multi-delete-schema-2",
			Schema: json.RawMessage(`{"field2": {"type": "number"}}`),
		},
		{
			Name:   "multi-delete-schema-3",
			Schema: json.RawMessage(`{"field3": {"type": "boolean"}}`),
		},
	}

	schemaIDs := make([]string, len(schemas))
	for i, schema := range schemas {
		schemaIDs[i] = ts.createTestSchema(schema)
	}

	// Delete all schemas
	for i, schemaID := range schemaIDs {
		req, err := http.NewRequest("DELETE", testServerURL+"/user-types/"+schemaID, nil)
		if err != nil {
			ts.T().Fatalf("Failed to create request for schema %d: %v", i+1, err)
		}

		resp, err := ts.client.Do(req)
		if err != nil {
			ts.T().Fatalf("Failed to send request for schema %d: %v", i+1, err)
		}
		defer resp.Body.Close()

		ts.Assert().Equal(http.StatusNoContent, resp.StatusCode, "Schema %d deletion should return 204 No Content", i+1)
	}

	// Verify all schemas are deleted
	for i, schemaID := range schemaIDs {
		getReq, err := http.NewRequest("GET", testServerURL+"/user-types/"+schemaID, nil)
		if err != nil {
			ts.T().Fatalf("Failed to create get request for schema %d: %v", i+1, err)
		}

		getResp, err := ts.client.Do(getReq)
		if err != nil {
			ts.T().Fatalf("Failed to send get request for schema %d: %v", i+1, err)
		}
		defer getResp.Body.Close()

		ts.Assert().Equal(http.StatusNotFound, getResp.StatusCode, "Schema %d should not exist after deletion", i+1)
	}
}

// TestDeleteUserTypeResponseHeaders tests response headers for DELETE /user-types/{id}
func (ts *DeleteUserTypeTestSuite) TestDeleteUserTypeResponseHeaders() {
	// Create a schema to delete
	schema := CreateUserTypeRequest{
		Name: "headers-test-schema",
		Schema: json.RawMessage(`{
			"field": {"type": "string"}
		}`),
	}

	schemaID := ts.createTestSchema(schema)

	// Delete the schema
	req, err := http.NewRequest("DELETE", testServerURL+"/user-types/"+schemaID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusNoContent, resp.StatusCode, "Should return 204 No Content")

	// Verify response has no content body for 204
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		ts.T().Fatalf("Failed to read response body: %v", err)
	}
	ts.Assert().Empty(bodyBytes, "204 response should have no content body")
}

// Helper function to create a test schema
func (ts *DeleteUserTypeTestSuite) createTestSchema(schema CreateUserTypeRequest) string {
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

func (ts *DeleteUserTypeTestSuite) deleteSchema(schemaID string) {
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
