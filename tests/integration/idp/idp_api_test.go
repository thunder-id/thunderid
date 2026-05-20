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

package idp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	testServerURL = testutils.TestServerURL
)

var (
	testGithubIdp = testutils.IDP{
		Name:        "Test Github IDP",
		Description: "Github Identity Provider for testing",
		Type:        "GITHUB",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test_github_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "test_github_secret",
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    "https://localhost:3000/github/callback",
				IsSecret: false,
			},
			{
				Name:     "scopes",
				Value:    "user:email,read:user",
				IsSecret: false,
			},
			// Default endpoints added by server
			{
				Name:     "authorization_endpoint",
				Value:    "https://github.com/login/oauth/authorize",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    "https://github.com/login/oauth/access_token",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    "https://api.github.com/user",
				IsSecret: false,
			},
		},
	}
	testGoogleIdp = testutils.IDP{
		Name:        "Test Google IDP",
		Description: "Google Identity Provider for testing",
		Type:        "GOOGLE",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test_google_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "test_google_secret",
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    "https://localhost:3000/google/callback",
				IsSecret: false,
			},
			{
				Name:     "scopes",
				Value:    "openid,email,profile",
				IsSecret: false,
			},
			// Default endpoints added by server
			{
				Name:     "authorization_endpoint",
				Value:    "https://accounts.google.com/o/oauth2/v2/auth",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    "https://oauth2.googleapis.com/token",
				IsSecret: false,
			},
			{
				Name:     "jwks_endpoint",
				Value:    "https://www.googleapis.com/oauth2/v3/certs",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    "https://openidconnect.googleapis.com/v1/userinfo",
				IsSecret: false,
			},
		},
	}
	idpToCreate = testutils.IDP{
		Name:        "Test OIDC IDP",
		Description: "OIDC test identity provider for CRUD operations",
		Type:        "OAUTH",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test_oidc_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "test_oidc_secret",
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    "https://localhost:3000/oidc/callback",
				IsSecret: false,
			},
			{
				Name:     "authorization_endpoint",
				Value:    "https://test-idp.example.com/authorize",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    "https://test-idp.example.com/token",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    "https://test-idp.example.com/userinfo",
				IsSecret: false,
			},
			{
				Name:     "scopes",
				Value:    "openid,email,profile",
				IsSecret: false,
			},
		},
	}
	idpToUpdate = testutils.IDP{
		Name:        "Test Updated IDP",
		Description: "Updated test identity provider",
		Type:        "OAUTH",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test_updated_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "test_updated_secret",
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    "https://localhost:3000/updated/callback",
				IsSecret: false,
			},
			{
				Name:     "authorization_endpoint",
				Value:    "https://updated-idp.example.com/authorize",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    "https://updated-idp.example.com/token",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    "https://updated-idp.example.com/userinfo",
				IsSecret: false,
			},
			{
				Name:     "scopes",
				Value:    "user:email,read:user",
				IsSecret: false,
			},
		},
	}

	idpWithInvalidType = testutils.IDP{
		Name:        "Invalid Type IDP",
		Description: "IDP with unsupported type",
		Type:        "INVALID_TYPE",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test_client",
				IsSecret: false,
			},
		},
	}
	idpWithEmptyType = testutils.IDP{
		Name:        "Empty Type IDP",
		Description: "IDP with empty type",
		Type:        "",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test_client",
				IsSecret: false,
			},
		},
	}
	idpWithEmptyName = testutils.IDP{
		Name:        "",
		Description: "IDP with empty name",
		Type:        "OAUTH",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test_client",
				IsSecret: false,
			},
		},
	}
	idpWithUnsupportedProperty = testutils.IDP{
		Name:        "Unsupported Property IDP",
		Description: "IDP with unsupported property",
		Type:        "OAUTH",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test_client",
				IsSecret: false,
			},
			{
				Name:     "unsupported_property",
				Value:    "some_value",
				IsSecret: false,
			},
		},
	}
	idpWithEmptyPropertyName = testutils.IDP{
		Name:        "Empty Property Name IDP",
		Description: "IDP with empty property name",
		Type:        "OAUTH",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test_client",
				IsSecret: false,
			},
			{
				Name:     "",
				Value:    "some_value",
				IsSecret: false,
			},
		},
	}
	idpWithEmptyPropertyValue = testutils.IDP{
		Name:        "Empty Property Value IDP",
		Description: "IDP with empty property value",
		Type:        "OAUTH",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "",
				IsSecret: true,
			},
		},
	}

	// Valid IDP with all supported properties
	idpWithAllOAuthProperties = testutils.IDP{
		Name:        "Complete OAuth IDP",
		Description: "OAuth IDP with all supported properties",
		Type:        "OAUTH",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "oauth_client_id",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "oauth_client_secret",
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    "https://localhost:3000/oauth/callback",
				IsSecret: false,
			},
			{
				Name:     "scopes",
				Value:    "openid,profile,email",
				IsSecret: false,
			},
			{
				Name:     "authorization_endpoint",
				Value:    "https://provider.com/oauth/authorize",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    "https://provider.com/oauth/token",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    "https://provider.com/oauth/userinfo",
				IsSecret: false,
			},
			{
				Name:     "logout_endpoint",
				Value:    "https://provider.com/oauth/logout",
				IsSecret: false,
			},
			{
				Name:     "prompt",
				Value:    "consent",
				IsSecret: false,
			},
		},
	}
)

var (
	testGithubIdpID string
	testGoogleIdpID string
	createdIdpID    string
	testIdps        []testutils.IDP // Track all created IDPs for validation
)

type IdpAPITestSuite struct {
	suite.Suite
}

func TestIdpAPITestSuite(t *testing.T) {

	suite.Run(t, new(IdpAPITestSuite))
}

func (ts *IdpAPITestSuite) SetupSuite() {
	// Create all test IDPs
	githubId, err := testutils.CreateIDP(testGithubIdp)
	if err != nil {
		ts.T().Fatalf("Failed to create test Github IDP during setup: %v", err)
	}
	testGithubIdpID = githubId

	googleId, err := testutils.CreateIDP(testGoogleIdp)
	if err != nil {
		ts.T().Fatalf("Failed to create test Google IDP during setup: %v", err)
	}
	testGoogleIdpID = googleId

	createId, err := testutils.CreateIDP(idpToCreate)
	if err != nil {
		ts.T().Fatalf("Failed to create test OIDC IDP during setup: %v", err)
	}
	createdIdpID = createId

	// Build the list of created IDPs for test validations
	testIdps = []testutils.IDP{
		{ID: testGithubIdpID, Name: testGithubIdp.Name, Description: testGithubIdp.Description,
			Type: testGithubIdp.Type, Properties: testGithubIdp.Properties},
		{ID: testGoogleIdpID, Name: testGoogleIdp.Name, Description: testGoogleIdp.Description,
			Type: testGoogleIdp.Type, Properties: testGoogleIdp.Properties},
		{ID: createdIdpID, Name: idpToCreate.Name, Description: idpToCreate.Description,
			Type: idpToCreate.Type, Properties: idpToCreate.Properties},
	}
}

func (ts *IdpAPITestSuite) TearDownSuite() {
	// Delete all test IDPs
	if testGithubIdpID != "" {
		err := testutils.DeleteIDP(testGithubIdpID)
		if err != nil {
			ts.T().Logf("Failed to delete test Github IDP during teardown: %v", err)
		}
	}

	if testGoogleIdpID != "" {
		err := testutils.DeleteIDP(testGoogleIdpID)
		if err != nil {
			ts.T().Logf("Failed to delete test Google IDP during teardown: %v", err)
		}
	}

	if createdIdpID != "" {
		err := testutils.DeleteIDP(createdIdpID)
		if err != nil {
			ts.T().Logf("Failed to delete test OIDC IDP during teardown: %v", err)
		}
	}
}

func (ts *IdpAPITestSuite) TestIdpListing() {
	req, err := http.NewRequest("GET", testServerURL+"/identity-providers", nil)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}

	client := testutils.GetHTTPClient()

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Validate the response
	if resp.StatusCode != http.StatusOK {
		ts.T().Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse the response body - list endpoint returns BasicIdpResponse objects
	var basicIdps []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	err = json.NewDecoder(resp.Body).Decode(&basicIdps)
	if err != nil {
		ts.T().Fatalf("Failed to parse response body: %v", err)
	}

	idpListLength := len(basicIdps)
	if idpListLength == 0 {
		ts.T().Fatalf("Response does not contain any identity providers")
	}

	// Verify that all test IDPs we created are present in the list (basic info only)
	for _, expectedIdp := range testIdps {
		found := false
		for _, idp := range basicIdps {
			if idp.ID == expectedIdp.ID && idp.Name == expectedIdp.Name &&
				idp.Description == expectedIdp.Description {
				found = true
				break
			}
		}
		if !found {
			ts.T().Fatalf("Test IDP not found in list: ID=%s, Name=%s", expectedIdp.ID, expectedIdp.Name)
		}
	}
}

func (ts *IdpAPITestSuite) TestIdpGetByID() {
	if createdIdpID == "" {
		ts.T().Fatal("IdP ID is not available for retrieval")
	}
	idp := testIdps[2]
	retrieveAndValidateIdpDetails(ts, idp)
}

func (ts *IdpAPITestSuite) TestCreateIdpSuccess() {
	successTestCases := []struct {
		name        string
		idp         testutils.IDP
		description string
	}{
		{
			name: "BasicOAuthIDP",
			idp: testutils.IDP{
				Name:        "Test Success OAuth IDP",
				Description: "Basic OAuth IDP for success testing",
				Type:        "OAUTH",
				Properties: []testutils.IDPProperty{
					{
						Name:     "client_id",
						Value:    "test_success_client",
						IsSecret: false,
					},
					{
						Name:     "client_secret",
						Value:    "test_success_secret",
						IsSecret: true,
					},
					{
						Name:     "redirect_uri",
						Value:    "https://localhost:3000/oauth/callback",
						IsSecret: false,
					},
					{
						Name:     "authorization_endpoint",
						Value:    "https://test-oauth.example.com/authorize",
						IsSecret: false,
					},
					{
						Name:     "token_endpoint",
						Value:    "https://test-oauth.example.com/token",
						IsSecret: false,
					},
					{
						Name:     "userinfo_endpoint",
						Value:    "https://test-oauth.example.com/userinfo",
						IsSecret: false,
					},
				},
			},
			description: "Should successfully create basic OAuth IDP",
		},
		{
			name: "BasicGoogleIDP",
			idp: testutils.IDP{
				Name:        "Test Success Google IDP",
				Description: "Basic Google IDP for success testing",
				Type:        "GOOGLE",
				Properties: []testutils.IDPProperty{
					{
						Name:     "client_id",
						Value:    "test_google_success_client",
						IsSecret: false,
					},
					{
						Name:     "client_secret",
						Value:    "test_google_success_secret",
						IsSecret: true,
					},
					{
						Name:     "redirect_uri",
						Value:    "https://localhost:3000/google/callback",
						IsSecret: false,
					},
				},
			},
			description: "Should successfully create basic Google IDP",
		},
		{
			name: "BasicGithubIDP",
			idp: testutils.IDP{
				Name:        "Test Success Github IDP",
				Description: "Basic Github IDP for success testing",
				Type:        "GITHUB",
				Properties: []testutils.IDPProperty{
					{
						Name:     "client_id",
						Value:    "test_github_success_client",
						IsSecret: false,
					},
					{
						Name:     "client_secret",
						Value:    "test_github_success_secret",
						IsSecret: true,
					},
					{
						Name:     "redirect_uri",
						Value:    "https://localhost:3000/github/callback",
						IsSecret: false,
					},
				},
			},
			description: "Should successfully create basic Github IDP",
		},
	}

	for _, tc := range successTestCases {
		ts.T().Run(tc.name, func(t *testing.T) {
			// Create the IDP
			idpID, err := testutils.CreateIDP(tc.idp)
			if err != nil {
				t.Fatalf("Failed to create IDP for test case %s: %v", tc.name, err)
			}

			// For Google and GitHub types, add expected default properties
			expectedProps := tc.idp.Properties
			if tc.idp.Type == "GOOGLE" {
				expectedProps = append(expectedProps,
					testutils.IDPProperty{Name: "authorization_endpoint", Value: "https://accounts.google.com/o/oauth2/v2/auth", IsSecret: false},
					testutils.IDPProperty{Name: "token_endpoint", Value: "https://oauth2.googleapis.com/token", IsSecret: false},
					testutils.IDPProperty{Name: "jwks_endpoint", Value: "https://www.googleapis.com/oauth2/v3/certs", IsSecret: false},
					testutils.IDPProperty{Name: "userinfo_endpoint", Value: "https://openidconnect.googleapis.com/v1/userinfo", IsSecret: false},
					testutils.IDPProperty{Name: "scopes", Value: "openid", IsSecret: false},
				)
			} else if tc.idp.Type == "GITHUB" {
				expectedProps = append(expectedProps,
					testutils.IDPProperty{Name: "authorization_endpoint", Value: "https://github.com/login/oauth/authorize", IsSecret: false},
					testutils.IDPProperty{Name: "token_endpoint", Value: "https://github.com/login/oauth/access_token", IsSecret: false},
					testutils.IDPProperty{Name: "userinfo_endpoint", Value: "https://api.github.com/user", IsSecret: false},
					testutils.IDPProperty{Name: "user_email_endpoint", Value: "https://api.github.com/user/emails", IsSecret: false},
				)
			}

			// Validate that the IDP was created correctly
			retrieveAndValidateIdpDetails(ts, testutils.IDP{
				ID:          idpID,
				Name:        tc.idp.Name,
				Description: tc.idp.Description,
				Type:        tc.idp.Type,
				Properties:  expectedProps,
			})

			// Clean up
			err = testutils.DeleteIDP(idpID)
			if err != nil {
				t.Logf("Failed to clean up test IDP for test case %s: %v", tc.name, err)
			}

			t.Logf("Test case %s passed: %s", tc.name, tc.description)
		})
	}
}

func (ts *IdpAPITestSuite) TestCreateIdpWithError() {
	testCases := []struct {
		name              string
		idp               testutils.IDP
		expectedStatus    int
		expectedErrorCode string
		description       string
	}{
		{
			name:              "InvalidType",
			idp:               idpWithInvalidType,
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: "IDP-1004",
			description:       "Should return error for unsupported IDP type",
		},
		{
			name:              "EmptyType",
			idp:               idpWithEmptyType,
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: "IDP-1004",
			description:       "Should return error for empty IDP type",
		},
		{
			name:              "EmptyName",
			idp:               idpWithEmptyName,
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: "IDP-1003",
			description:       "Should return error for empty IDP name",
		},
		{
			name:              "UnsupportedProperty",
			idp:               idpWithUnsupportedProperty,
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: "IDP-1007",
			description:       "Should return error for unsupported property",
		},
		{
			name:              "EmptyPropertyName",
			idp:               idpWithEmptyPropertyName,
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: "IDP-1006",
			description:       "Should return error for empty property name",
		},
		{
			name:              "EmptyPropertyValue",
			idp:               idpWithEmptyPropertyValue,
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: "IDP-1006",
			description:       "Should return error for empty property value",
		},
	}

	for _, tc := range testCases {
		ts.T().Run(tc.name, func(t *testing.T) {
			idpJSON, err := json.Marshal(tc.idp)
			if err != nil {
				t.Fatalf("Failed to marshal IDP for test case %s: %v", tc.name, err)
			}

			resp, err := makeIDPAPIRequest("POST", "/identity-providers", bytes.NewReader(idpJSON))
			if err != nil {
				t.Fatalf("Failed to send request for test case %s: %v", tc.name, err)
			}
			defer resp.Body.Close()

			validateErrorResponse(ts, resp, tc.expectedStatus, tc.expectedErrorCode)
			t.Logf("Test case %s passed: %s", tc.name, tc.description)
		})
	}
}

func (ts *IdpAPITestSuite) TestCreateIdpWithAllSupportedProperties() {
	idpID, err := testutils.CreateIDP(idpWithAllOAuthProperties)
	if err != nil {
		ts.T().Fatalf("Failed to create IDP with all properties: %v", err)
	}

	retrieveAndValidateIdpDetails(ts, testutils.IDP{
		ID:          idpID,
		Name:        idpWithAllOAuthProperties.Name,
		Description: idpWithAllOAuthProperties.Description,
		Type:        idpWithAllOAuthProperties.Type,
		Properties:  idpWithAllOAuthProperties.Properties,
	})

	err = testutils.DeleteIDP(idpID)
	if err != nil {
		ts.T().Logf("Failed to clean up test IDP: %v", err)
	}
}

func (ts *IdpAPITestSuite) TestUpdateIdp() {
	if createdIdpID == "" {
		ts.T().Fatal("IdP ID is not available for update")
	}

	idpJSON, err := json.Marshal(idpToUpdate)
	if err != nil {
		ts.T().Fatalf("Failed to marshal idPToUpdate: %v", err)
	}

	reqBody := bytes.NewReader(idpJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/identity-providers/"+createdIdpID, reqBody)
	if err != nil {
		ts.T().Fatalf("Failed to create update request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send update request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ts.T().Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Validate the update by retrieving the idP
	retrieveAndValidateIdpDetails(ts, testutils.IDP{
		ID:          createdIdpID,
		Name:        idpToUpdate.Name,
		Description: idpToUpdate.Description,
		Properties:  idpToUpdate.Properties,
	})
}

func (ts *IdpAPITestSuite) TestUpdateIdpWithError() {
	if createdIdpID == "" {
		ts.T().Fatal("IdP ID is not available for update test")
	}

	testCases := []struct {
		name              string
		idp               testutils.IDP
		expectedStatus    int
		expectedErrorCode string
		description       string
	}{
		{
			name:              "InvalidType",
			idp:               idpWithInvalidType,
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: "IDP-1004",
			description:       "Should return error for unsupported IDP type during update",
		},
		{
			name:              "UnsupportedProperty",
			idp:               idpWithUnsupportedProperty,
			expectedStatus:    http.StatusBadRequest,
			expectedErrorCode: "IDP-1007",
			description:       "Should return error for unsupported property during update",
		},
	}

	for _, tc := range testCases {
		ts.T().Run(tc.name, func(t *testing.T) {
			idpJSON, err := json.Marshal(tc.idp)
			if err != nil {
				t.Fatalf("Failed to marshal IDP for test case %s: %v", tc.name, err)
			}

			resp, err := makeIDPAPIRequest("PUT", "/identity-providers/"+createdIdpID, bytes.NewReader(idpJSON))
			if err != nil {
				t.Fatalf("Failed to send request for test case %s: %v", tc.name, err)
			}
			defer resp.Body.Close()

			validateErrorResponse(ts, resp, tc.expectedStatus, tc.expectedErrorCode)
			t.Logf("Test case %s passed: %s", tc.name, tc.description)
		})
	}
}

func (ts *IdpAPITestSuite) TestSupportedIdpTypes() {
	supportedTypes := []string{"OAUTH", "GOOGLE", "GITHUB"}

	for _, idpType := range supportedTypes {
		properties := []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    fmt.Sprintf("test_%s_client", strings.ToLower(idpType)),
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    fmt.Sprintf("test_%s_secret", strings.ToLower(idpType)),
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    fmt.Sprintf("https://localhost:3000/%s/callback", strings.ToLower(idpType)),
				IsSecret: false,
			},
		}

		// Add required endpoints for OAuth type
		if idpType == "OAUTH" {
			properties = append(properties,
				testutils.IDPProperty{
					Name:     "authorization_endpoint",
					Value:    "https://test-oauth.example.com/authorize",
					IsSecret: false,
				},
				testutils.IDPProperty{
					Name:     "token_endpoint",
					Value:    "https://test-oauth.example.com/token",
					IsSecret: false,
				},
				testutils.IDPProperty{
					Name:     "userinfo_endpoint",
					Value:    "https://test-oauth.example.com/userinfo",
					IsSecret: false,
				},
			)
		}

		testIDP := testutils.IDP{
			Name:        fmt.Sprintf("Test %s IDP", idpType),
			Description: fmt.Sprintf("%s identity provider for testing", idpType),
			Type:        idpType,
			Properties:  properties,
		}

		// Create IDP with the supported type
		idpID, err := testutils.CreateIDP(testIDP)
		if err != nil {
			ts.T().Fatalf("Failed to create IDP with type %s: %v", idpType, err)
		}

		// For Google and GitHub types, add expected default properties
		expectedProps := testIDP.Properties
		if idpType == "GOOGLE" {
			expectedProps = append(expectedProps,
				testutils.IDPProperty{Name: "authorization_endpoint", Value: "https://accounts.google.com/o/oauth2/v2/auth", IsSecret: false},
				testutils.IDPProperty{Name: "token_endpoint", Value: "https://oauth2.googleapis.com/token", IsSecret: false},
				testutils.IDPProperty{Name: "jwks_endpoint", Value: "https://www.googleapis.com/oauth2/v3/certs", IsSecret: false},
				testutils.IDPProperty{Name: "userinfo_endpoint", Value: "https://openidconnect.googleapis.com/v1/userinfo", IsSecret: false},
				testutils.IDPProperty{Name: "scopes", Value: "openid", IsSecret: false},
			)
		} else if idpType == "GITHUB" {
			expectedProps = append(expectedProps,
				testutils.IDPProperty{Name: "authorization_endpoint", Value: "https://github.com/login/oauth/authorize", IsSecret: false},
				testutils.IDPProperty{Name: "token_endpoint", Value: "https://github.com/login/oauth/access_token", IsSecret: false},
				testutils.IDPProperty{Name: "userinfo_endpoint", Value: "https://api.github.com/user", IsSecret: false},
				testutils.IDPProperty{Name: "user_email_endpoint", Value: "https://api.github.com/user/emails", IsSecret: false},
			)
		}

		// Validate type is correctly stored
		retrieveAndValidateIdpDetails(ts, testutils.IDP{
			ID:          idpID,
			Name:        testIDP.Name,
			Description: testIDP.Description,
			Type:        testIDP.Type,
			Properties:  expectedProps,
		})

		// Clean up
		err = testutils.DeleteIDP(idpID)
		if err != nil {
			ts.T().Logf("Failed to clean up test IDP: %v", err)
		}
	}
}

func (ts *IdpAPITestSuite) TestSupportedPropertyNames() {
	// Test with OIDC type since it supports more optional properties
	supportedProperties := []string{
		"scopes", "logout_endpoint", "jwks_endpoint", "prompt",
	}

	for _, propertyName := range supportedProperties {
		// Start with required OIDC properties
		properties := []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "test_secret",
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    "https://localhost:3000/callback",
				IsSecret: false,
			},
			{
				Name:     "authorization_endpoint",
				Value:    "https://test.example.com/authorize",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    "https://test.example.com/token",
				IsSecret: false,
			},
			{
				Name:     propertyName,
				Value:    fmt.Sprintf("test_%s_value", strings.ReplaceAll(propertyName, "_", "")),
				IsSecret: false,
			},
		}

		testIDP := testutils.IDP{
			Name:        fmt.Sprintf("Test IDP with %s", propertyName),
			Description: fmt.Sprintf("Testing %s property", propertyName),
			Type:        "OIDC",
			Properties:  properties,
		}

		// Create IDP with the supported property
		idpID, err := testutils.CreateIDP(testIDP)
		if err != nil {
			ts.T().Fatalf("Failed to create IDP with property %s: %v", propertyName, err)
		}

		// Clean up
		err = testutils.DeleteIDP(idpID)
		if err != nil {
			ts.T().Logf("Failed to clean up test IDP: %v", err)
		}
	}
}

func (ts *IdpAPITestSuite) TestInvalidIdpId() {
	// Test GET with invalid ID
	resp, err := makeIDPAPIRequest("GET", "/identity-providers/invalid-id", nil)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()
	validateErrorResponse(ts, resp, http.StatusNotFound, "IDP-1001")

	// Test PUT with invalid ID
	idpJSON, err := json.Marshal(idpToUpdate)
	if err != nil {
		ts.T().Fatalf("Failed to marshal IDP: %v", err)
	}
	resp, err = makeIDPAPIRequest("PUT", "/identity-providers/invalid-id", bytes.NewReader(idpJSON))
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()
	validateErrorResponse(ts, resp, http.StatusNotFound, "IDP-1001")

	// Test DELETE with invalid ID - should not return error for idempotency
	resp, err = makeIDPAPIRequest("DELETE", "/identity-providers/invalid-id", nil)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()
	// DELETE typically returns 204 No Content even for non-existent resources for idempotency
	if resp.StatusCode != http.StatusNoContent {
		ts.T().Fatalf("Expected status 204, got %d", resp.StatusCode)
	}
}

func (ts *IdpAPITestSuite) TestMalformedJsonRequest() {
	malformedJSON := `{"name": "test", "type": "OAUTH", "properties":`

	resp, err := makeIDPAPIRequest("POST", "/identity-providers", strings.NewReader(malformedJSON))
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	validateErrorResponse(ts, resp, http.StatusBadRequest, "IDP-1009")
}

func retrieveAndValidateIdpDetails(ts *IdpAPITestSuite, expectedIdp testutils.IDP) {
	req, err := http.NewRequest("GET", testServerURL+"/identity-providers/"+expectedIdp.ID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create get request: %v", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send get request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ts.T().Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check if the response Content-Type is application/json
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		rawBody, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Unexpected Content-Type: %s. Raw body: %s", contentType, string(rawBody))
	}

	var idp testutils.IDP
	err = json.NewDecoder(resp.Body).Decode(&idp)
	if err != nil {
		ts.T().Fatalf("Failed to parse response body: %v", err)
	}

	if !compareIDPs(idp, expectedIdp) {
		ts.T().Fatalf("IdP mismatch, expected %+v, got %+v", expectedIdp, idp)
	}
}

// compareIDPs compares two IDP instances for equality, accounting for secret property masking
func compareIDPs(idp, expectedIdp testutils.IDP) bool {
	if idp.ID != expectedIdp.ID || idp.Name != expectedIdp.Name || idp.Description != expectedIdp.Description {
		return false
	}

	if len(idp.Properties) != len(expectedIdp.Properties) {
		return false
	}

	for _, expProp := range expectedIdp.Properties {
		propFound := false
		for _, p := range idp.Properties {
			if p.Name == expProp.Name {
				propFound = true
				// For secret properties, the API returns "******" so we don't compare values
				if !expProp.IsSecret && p.Value != expProp.Value {
					return false
				}
				// For secret properties, just check that it's masked
				if expProp.IsSecret && p.Value != "******" {
					return false
				}
				// Ensure IsSecret flag matches
				if p.IsSecret != expProp.IsSecret {
					return false
				}
				break
			}
		}
		if !propFound {
			return false
		}
	}

	return true
}

// makeIDPAPIRequest makes an HTTP request and returns the response, useful for testing error cases
func makeIDPAPIRequest(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, testServerURL+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := testutils.GetHTTPClient()

	return client.Do(req)
}

// validateErrorResponse validates that the response contains the expected error code
func validateErrorResponse(ts *IdpAPITestSuite, resp *http.Response,
	expectedStatusCode int, expectedErrorCode string) {
	if resp.StatusCode != expectedStatusCode {
		ts.T().Fatalf("Expected status %d, got %d", expectedStatusCode, resp.StatusCode)
	}

	var errorResp testutils.ErrorResponse
	err := json.NewDecoder(resp.Body).Decode(&errorResp)
	if err != nil {
		ts.T().Fatalf("Failed to parse error response: %v", err)
	}

	if errorResp.Code != expectedErrorCode {
		ts.T().Fatalf("Expected error code %s, got %s", expectedErrorCode, errorResp.Code)
	}
}
