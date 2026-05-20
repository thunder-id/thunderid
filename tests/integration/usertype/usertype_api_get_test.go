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
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

type GetUserTypeTestSuite struct {
	suite.Suite
	client         *http.Client
	testSchemaID   string
	testSchemaName string
	testSchemaData json.RawMessage
	oUID           string
}

var testUserTypeAPIGetOU = testutils.OrganizationUnit{
	Handle:      "test-user-type-api-get-ou",
	Name:        "Test Organization Unit for User Type API Get",
	Description: "Organization unit created for user type API get testing",
	Parent:      nil,
}

func TestGetUserTypeTestSuite(t *testing.T) {
	suite.Run(t, new(GetUserTypeTestSuite))
}

func (ts *GetUserTypeTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	// Create organization unit for tests
	ouID, err := testutils.CreateOrganizationUnit(testUserTypeAPIGetOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit: %v", err)
	}
	ts.oUID = ouID

	// Create a test schema for retrieval tests
	ts.testSchemaName = "retrieval-test-schema"
	ts.testSchemaData = json.RawMessage(`{
        "username": {"type": "string", "required": true},
        "email": {"type": "string", "regex": "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"},
        "profile": {
            "type": "object",
            "properties": {
                "given_name": {"type": "string"},
                "family_name": {"type": "string"},
                "age": {"type": "number"}
            }
        }
    }`)

	schema := CreateUserTypeRequest{
		Name:   ts.testSchemaName,
		Schema: ts.testSchemaData,
	}

	ts.testSchemaID = ts.createTestSchema(schema)
}

func (ts *GetUserTypeTestSuite) TearDownSuite() {
	// Clean up test schema
	if ts.testSchemaID != "" {
		ts.deleteTestSchema(ts.testSchemaID)
	}

	// Clean up created organization units
	if ts.oUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.oUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit %s: %v", ts.oUID, err)
		}
	}
}

// TestGetUserTypeByID tests GET /user-types/{id} with valid ID
func (ts *GetUserTypeTestSuite) TestGetUserTypeByID() {
	req, err := http.NewRequest("GET", testServerURL+"/user-types/"+ts.testSchemaID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}

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

	var schema UserType
	err = json.Unmarshal(bodyBytes, &schema)
	if err != nil {
		ts.T().Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify retrieved schema according to API spec
	ts.Assert().Equal(ts.testSchemaID, schema.ID, "ID should match")
	ts.Assert().Equal(ts.testSchemaName, schema.Name, "Name should match")
	ts.Assert().JSONEq(string(ts.testSchemaData), string(schema.Schema), "Schema data should match")
}

// TestGetUserTypeNotFound tests GET /user-types/{id} with non-existent ID
func (ts *GetUserTypeTestSuite) TestGetUserTypeNotFound() {
	nonExistentID := "550e8400-e29b-41d4-a716-446655440000"

	req, err := http.NewRequest("GET", testServerURL+"/user-types/"+nonExistentID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}

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

// TestGetUserTypeWithInvalidID tests GET /user-types/{id} with invalid ID formats
func (ts *GetUserTypeTestSuite) TestGetUserTypeWithInvalidID() {
	testCases := []struct {
		name           string
		schemaID       string
		expectedStatus int
	}{
		{
			name:           "empty ID",
			schemaID:       "",
			expectedStatus: http.StatusNotFound, // Empty path segment may result in 404
		},
		{
			name:           "invalid UUID format",
			schemaID:       "invalid-uuid-format",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "special characters in ID",
			schemaID:       "schema@#$%^&*()",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "very long ID",
			schemaID:       "very-long-id-that-exceeds-normal-uuid-length-and-should-be-handled-properly",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range testCases {
		ts.T().Run(tc.name, func(t *testing.T) {
			var requestURL string
			if tc.schemaID == "" {
				requestURL = testServerURL + "/user-types/"
			} else {
				// URL-encode the schema ID to handle special characters
				encodedSchemaID := url.PathEscape(tc.schemaID)
				requestURL = testServerURL + "/user-types/" + encodedSchemaID
			}

			req, err := http.NewRequest("GET", requestURL, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			resp, err := ts.client.Do(req)
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			// Should handle invalid IDs gracefully
			ts.Assert().True(resp.StatusCode == http.StatusBadRequest ||
				resp.StatusCode == http.StatusNotFound,
				"Should handle invalid ID appropriately for case: %s, got status: %d", tc.name, resp.StatusCode)

			// For error responses, verify error structure
			if resp.StatusCode >= 400 {
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("Failed to read response body: %v", err)
				}

				// Only try to unmarshal as JSON if the content type indicates JSON
				contentType := resp.Header.Get("Content-Type")
				if len(bodyBytes) > 0 && (contentType != "" && !strings.Contains(contentType, "application/json")) {
					// Non-JSON response (likely HTML), just verify we got an error status
					t.Logf("Received non-JSON error response with content type: %s", contentType)
					return
				}

				var errorResp ErrorResponse
				err = json.Unmarshal(bodyBytes, &errorResp)
				if err != nil {
					t.Fatalf("Failed to unmarshal error response: %v", err)
				}

				ts.Assert().NotEmpty(errorResp.Code, "Error should have code")
				ts.Assert().NotEmpty(errorResp.Message.DefaultValue, "Error should have message")
			}
		})
	}
}

// TestGetUserTypeResponseHeaders tests response headers for GET /user-types/{id}
func (ts *GetUserTypeTestSuite) TestGetUserTypeResponseHeaders() {
	req, err := http.NewRequest("GET", testServerURL+"/user-types/"+ts.testSchemaID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusOK, resp.StatusCode, "Should return 200 OK")

	// Verify Content-Type header
	contentType := resp.Header.Get("Content-Type")
	ts.Assert().Contains(contentType, "application/json", "Should return JSON content type")
}

// Helper function to create a test schema
func (ts *GetUserTypeTestSuite) createTestSchema(schema CreateUserTypeRequest) string {
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
func (ts *GetUserTypeTestSuite) deleteTestSchema(schemaID string) {
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
