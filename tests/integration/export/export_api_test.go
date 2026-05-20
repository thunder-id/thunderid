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

package export

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
	testServerURL = "https://localhost:8095"
)

// ExportAPITestSuite is a test suite for export API tests.
type ExportAPITestSuite struct {
	suite.Suite
	ouID string
}

// TestExportAPITestSuite runs the export API test suite.
func TestExportAPITestSuite(t *testing.T) {
	suite.Run(t, new(ExportAPITestSuite))
}

// SetupSuite sets up the test suite.
func (ts *ExportAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "export-test-ou",
		Name:        "Export Test OU",
		Description: "Organization unit for export integration tests",
		Parent:      nil,
	})
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit: %v", err)
	}
	ts.ouID = ouID
}

// TearDownSuite tears down the test suite.
func (ts *ExportAPITestSuite) TearDownSuite() {
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

// TestApplicationExportYAML tests the application export functionality returning YAML.
func (ts *ExportAPITestSuite) TestApplicationExportYAML() {
	// Create a test application first
	app := Application{
		OUID:                      ts.ouID,
		Name:                      "Export Test App",
		Description:               "Test application for export functionality",
		URL:                       "https://exporttest.example.com",
		LogoURL:                   "https://exporttest.example.com/logo.png",
		IsRegistrationFlowEnabled: true,
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "export_test_client",
					ClientSecret:            "export_test_secret",
					RedirectURIs:            []string{"https://exporttest.example.com/callback"},
					GrantTypes:              []string{"authorization_code", "refresh_token"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PKCERequired:            false,
					PublicClient:            false,
				},
			},
		},
	}

	appID, err := ts.createApplication(app)
	ts.Require().NoError(err)
	defer ts.deleteApplication(appID)

	// Test YAML export functionality
	exportRequest := ExportRequest{
		Applications: []string{appID},
	}

	yamlContent, err := ts.exportResourcesYAML(exportRequest)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	// Verify the exported YAML content
	ts.Assert().Contains(yamlContent, "name: Export Test App")
	ts.Assert().Contains(yamlContent, "description: Test application for export functionality")
	ts.Assert().Contains(yamlContent, "client_id: {{.EXPORT_TEST_APP_CLIENT_ID}}")
	ts.Assert().NotContains(yamlContent, "export_test_secret") // Client secret should not be exported
	ts.Assert().Contains(yamlContent, "# File: Export_Test_App.yaml")

	// Test JSON export functionality for backward compatibility
	exportResponse, err := ts.exportResourcesJSON(exportRequest)
	ts.Require().NoError(err)
	ts.Require().NotNil(exportResponse)
	ts.Assert().Len(exportResponse.Files, 1)

	// Verify the exported file
	exportedFile := exportResponse.Files[0]
	ts.Assert().Equal("Export_Test_App.yaml", exportedFile.FileName)
	ts.Assert().Contains(exportedFile.Content, "name: Export Test App")
}

// TestExportWithInvalidApplicationID tests export with invalid application ID.
func (ts *ExportAPITestSuite) TestExportWithInvalidApplicationID() {
	// Test export with invalid application ID
	invalidExportRequest := ExportRequest{
		Applications: []string{"invalid-uuid"},
	}

	_, err := ts.exportResourcesYAML(invalidExportRequest)
	ts.Require().Error(err)
}

// TestExportWithEmptyRequest tests export with empty request.
func (ts *ExportAPITestSuite) TestExportWithEmptyRequest() {
	// Test export with empty request
	emptyExportRequest := ExportRequest{
		Applications: []string{},
	}

	_, err := ts.exportResourcesYAML(emptyExportRequest)
	ts.Require().Error(err)
}

// TestIdentityProviderExportYAML tests the identity provider export functionality returning YAML.
func (ts *ExportAPITestSuite) TestIdentityProviderExportYAML() {
	// Create a test IDP first
	idp := IDP{
		Name:        "Export Test IDP",
		Description: "Test identity provider for export functionality",
		Type:        "OAUTH",
		Properties: []IDPProperty{
			{
				Name:     "client_id",
				Value:    "export_test_oauth_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "export_test_oauth_secret",
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    "https://localhost:3000/oauth/callback",
				IsSecret: false,
			},
			{
				Name:     "authorization_endpoint",
				Value:    "https://export-test-idp.example.com/authorize",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    "https://export-test-idp.example.com/token",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    "https://export-test-idp.example.com/userinfo",
				IsSecret: false,
			},
		},
	}

	idpID, err := ts.createIDP(idp)
	ts.Require().NoError(err)
	defer ts.deleteIDP(idpID)

	// Test YAML export functionality
	exportRequest := ExportRequest{
		IdentityProviders: []string{idpID},
	}

	yamlContent, err := ts.exportResourcesYAML(exportRequest)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	// Verify the exported YAML content
	ts.Assert().Contains(yamlContent, "name: Export Test IDP")
	ts.Assert().Contains(yamlContent, "description: Test identity provider for export functionality")
	ts.Assert().Contains(yamlContent, "type: OAUTH")
	ts.Assert().Contains(yamlContent, "properties:")
	ts.Assert().Contains(yamlContent, "name: client_id")
	ts.Assert().Contains(yamlContent, "value: {{.EXPORT_TEST_IDP_CLIENT_ID}}")
	ts.Assert().Contains(yamlContent, "name: client_secret")
	ts.Assert().Contains(yamlContent, "value: {{.EXPORT_TEST_IDP_CLIENT_SECRET}}")
	ts.Assert().Contains(yamlContent, "is_secret: true")
	ts.Assert().Contains(yamlContent, "# File: Export_Test_IDP.yaml")
}

// TestMultipleIdentityProvidersExportYAML tests exporting multiple identity providers.
func (ts *ExportAPITestSuite) TestMultipleIdentityProvidersExportYAML() {
	// Create first IDP
	idp1 := IDP{
		Name:        "GitHub IDP Export",
		Description: "GitHub identity provider for export",
		Type:        "OAUTH",
		Properties: []IDPProperty{
			{
				Name:     "client_id",
				Value:    "github_export_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "github_export_secret",
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    "https://localhost:3000/github/callback",
				IsSecret: false,
			},
			{
				Name:     "authorization_endpoint",
				Value:    "https://github-export.example.com/authorize",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    "https://github-export.example.com/token",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    "https://github-export.example.com/userinfo",
				IsSecret: false,
			},
		},
	}

	idpID1, err := ts.createIDP(idp1)
	ts.Require().NoError(err)
	defer ts.deleteIDP(idpID1)

	// Create second IDP
	idp2 := IDP{
		Name:        "Google IDP Export",
		Description: "Google identity provider for export",
		Type:        "OIDC",
		Properties: []IDPProperty{
			{
				Name:     "client_id",
				Value:    "google_export_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "google_export_secret",
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    "https://localhost:3000/google/callback",
				IsSecret: false,
			},
			{
				Name:     "authorization_endpoint",
				Value:    "https://google-export.example.com/authorize",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    "https://google-export.example.com/token",
				IsSecret: false,
			},
		},
	}

	idpID2, err := ts.createIDP(idp2)
	ts.Require().NoError(err)
	defer ts.deleteIDP(idpID2)

	// Test exporting multiple IDPs
	exportRequest := ExportRequest{
		IdentityProviders: []string{idpID1, idpID2},
	}

	yamlContent, err := ts.exportResourcesYAML(exportRequest)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	// Verify both IDPs are in the export
	ts.Assert().Contains(yamlContent, "name: GitHub IDP Export")
	ts.Assert().Contains(yamlContent, "name: Google IDP Export")
	ts.Assert().Contains(yamlContent, "type: OAUTH")
	ts.Assert().Contains(yamlContent, "type: OIDC")
	ts.Assert().Contains(yamlContent, "# File: GitHub_IDP_Export.yaml")
	ts.Assert().Contains(yamlContent, "# File: Google_IDP_Export.yaml")
}

// TestMixedResourcesExportYAML tests exporting both applications and identity providers.
func (ts *ExportAPITestSuite) TestMixedResourcesExportYAML() {
	// Create a test application
	app := Application{
		OUID:                      ts.ouID,
		Name:                      "Mixed Export App",
		Description:               "Test application for mixed export",
		URL:                       "https://mixedexport.example.com",
		IsRegistrationFlowEnabled: true,
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "mixed_export_client",
					ClientSecret:            "mixed_export_secret",
					RedirectURIs:            []string{"https://mixedexport.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}

	appID, err := ts.createApplication(app)
	ts.Require().NoError(err)
	defer ts.deleteApplication(appID)

	// Create a test IDP
	idp := IDP{
		Name:        "Mixed Export IDP",
		Description: "Test IDP for mixed export",
		Type:        "OAUTH",
		Properties: []IDPProperty{
			{
				Name:     "client_id",
				Value:    "mixed_idp_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "mixed_idp_secret",
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    "https://localhost:3000/mixed/callback",
				IsSecret: false,
			},
			{
				Name:     "authorization_endpoint",
				Value:    "https://mixed-export.example.com/authorize",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    "https://mixed-export.example.com/token",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    "https://mixed-export.example.com/userinfo",
				IsSecret: false,
			},
		},
	}

	idpID, err := ts.createIDP(idp)
	ts.Require().NoError(err)
	defer ts.deleteIDP(idpID)

	// Test exporting both application and IDP
	exportRequest := ExportRequest{
		Applications:      []string{appID},
		IdentityProviders: []string{idpID},
	}

	yamlContent, err := ts.exportResourcesYAML(exportRequest)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	// Verify both resources are in the export
	ts.Assert().Contains(yamlContent, "name: Mixed Export App")
	ts.Assert().Contains(yamlContent, "name: Mixed Export IDP")
	ts.Assert().Contains(yamlContent, "# File: Mixed_Export_App.yaml")
	ts.Assert().Contains(yamlContent, "# File: Mixed_Export_IDP.yaml")
}

// TestIdentityProviderExportWithWildcard tests exporting all identity providers using wildcard.
func (ts *ExportAPITestSuite) TestIdentityProviderExportWithWildcard() {
	// Create a test IDP
	idp := IDP{
		Name:        "Wildcard Test IDP",
		Description: "Test IDP for wildcard export",
		Type:        "OAUTH",
		Properties: []IDPProperty{
			{
				Name:     "client_id",
				Value:    "wildcard_test_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "wildcard_test_secret",
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    "https://localhost:3000/wildcard/callback",
				IsSecret: false,
			},
			{
				Name:     "authorization_endpoint",
				Value:    "https://wildcard-test.example.com/authorize",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    "https://wildcard-test.example.com/token",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    "https://wildcard-test.example.com/userinfo",
				IsSecret: false,
			},
		},
	}

	idpID, err := ts.createIDP(idp)
	ts.Require().NoError(err)
	defer ts.deleteIDP(idpID)

	// Test wildcard export
	exportRequest := ExportRequest{
		IdentityProviders: []string{"*"},
	}

	yamlContent, err := ts.exportResourcesYAML(exportRequest)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	// Verify the test IDP is included in wildcard export
	ts.Assert().Contains(yamlContent, "name: Wildcard Test IDP")
}

// TestIdentityProviderExportWithProperties tests exporting IDP with various property types.
func (ts *ExportAPITestSuite) TestIdentityProviderExportWithProperties() {
	// Create IDP with multiple property types
	idp := IDP{
		Name:        "Properties Test IDP",
		Description: "Test IDP with various properties",
		Type:        "OIDC",
		Properties: []IDPProperty{
			{
				Name:     "client_id",
				Value:    "props_test_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "props_test_secret",
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    "https://localhost:3000/callback",
				IsSecret: false,
			},
			{
				Name:     "authorization_endpoint",
				Value:    "https://props-test.example.com/authorize",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    "https://props-test.example.com/token",
				IsSecret: false,
			},
			{
				Name:     "scopes",
				Value:    "openid,email,profile",
				IsSecret: false,
			},
		},
	}

	idpID, err := ts.createIDP(idp)
	ts.Require().NoError(err)
	defer ts.deleteIDP(idpID)

	// Export the IDP
	exportRequest := ExportRequest{
		IdentityProviders: []string{idpID},
	}

	yamlContent, err := ts.exportResourcesYAML(exportRequest)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	// Verify all properties are properly parameterized
	ts.Assert().Contains(yamlContent, "name: client_id")
	ts.Assert().Contains(yamlContent, "value: {{.PROPERTIES_TEST_IDP_CLIENT_ID}}")
	ts.Assert().Contains(yamlContent, "name: client_secret")
	ts.Assert().Contains(yamlContent, "value: {{.PROPERTIES_TEST_IDP_CLIENT_SECRET}}")
	ts.Assert().Contains(yamlContent, "name: redirect_uri")
	ts.Assert().Contains(yamlContent, "value: {{.PROPERTIES_TEST_IDP_REDIRECT_URI}}")
	ts.Assert().Contains(yamlContent, "name: scopes")
	ts.Assert().Contains(yamlContent, "value: {{.PROPERTIES_TEST_IDP_SCOPES}}")
	// Verify is_secret flag is preserved
	ts.Assert().Contains(yamlContent, "is_secret: true")
}

// TestExportWithInvalidIdentityProviderID tests export with invalid IDP ID.
func (ts *ExportAPITestSuite) TestExportWithInvalidIdentityProviderID() {
	// Test export with invalid IDP ID
	invalidExportRequest := ExportRequest{
		IdentityProviders: []string{"invalid-uuid"},
	}

	_, err := ts.exportResourcesYAML(invalidExportRequest)
	ts.Require().Error(err)
}

// Helper functions

func (ts *ExportAPITestSuite) createApplication(app Application) (string, error) {
	appJSON, err := json.Marshal(app)
	if err != nil {
		return "", fmt.Errorf("failed to marshal application: %w", err)
	}

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("POST", testServerURL+"/applications", reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		responseBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}

	var createdApp Application
	err = json.NewDecoder(resp.Body).Decode(&createdApp)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}

	id := createdApp.ID
	if id == "" {
		return "", fmt.Errorf("response does not contain id")
	}
	return id, nil
}

func (ts *ExportAPITestSuite) deleteApplication(appID string) error {
	req, err := http.NewRequest("DELETE", testServerURL+"/applications/"+appID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

func (ts *ExportAPITestSuite) exportResourcesYAML(exportRequest ExportRequest) (string, error) {
	reqJSON, err := json.Marshal(exportRequest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal export request: %w", err)
	}

	reqBody := bytes.NewReader(reqJSON)
	req, err := http.NewRequest("POST", testServerURL+"/export", reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to create export request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send export request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}

	// Parse JSON response
	var jsonResponse JSONExportResponse
	err = json.NewDecoder(resp.Body).Decode(&jsonResponse)
	if err != nil {
		return "", fmt.Errorf("failed to parse JSON export response: %w", err)
	}

	return jsonResponse.Resources, nil
}

func (ts *ExportAPITestSuite) exportResourcesJSON(exportRequest ExportRequest) (*ExportResponse, error) {
	reqJSON, err := json.Marshal(exportRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal export request: %w", err)
	}

	reqBody := bytes.NewReader(reqJSON)
	req, err := http.NewRequest("POST", testServerURL+"/export", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create export request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send export request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}

	// Parse the new JSON response format
	var jsonResponse JSONExportResponse
	err = json.NewDecoder(resp.Body).Decode(&jsonResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse export response: %w", err)
	}
	exportResponse := parseResourcesIntoExportResponse(jsonResponse.Resources)
	return exportResponse, nil
}

// parseResourcesIntoExportResponse parses the combined YAML resources string into individual ExportFile entries.
func parseResourcesIntoExportResponse(resources string) *ExportResponse {
	files := []ExportFile{}

	// Split by YAML document separator
	parts := strings.Split(resources, "\n---\n")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Extract filename from the "# File: " comment
		lines := strings.Split(part, "\n")
		fileName := ""
		contentStart := 0

		for i, line := range lines {
			if strings.HasPrefix(line, "# File:") {
				fileName = strings.TrimSpace(strings.TrimPrefix(line, "# File:"))
				contentStart = i + 1
				break
			}
		}

		if fileName == "" {
			continue
		}

		// Join remaining lines as content
		content := strings.Join(lines[contentStart:], "\n")
		content = strings.TrimSpace(content)

		files = append(files, ExportFile{
			FileName: fileName,
			Content:  content,
		})
	}

	return &ExportResponse{Files: files}
}

func (ts *ExportAPITestSuite) createIDP(idp IDP) (string, error) {
	idpJSON, err := json.Marshal(idp)
	if err != nil {
		return "", fmt.Errorf("failed to marshal IDP: %w", err)
	}

	reqBody := bytes.NewReader(idpJSON)
	req, err := http.NewRequest("POST", testServerURL+"/identity-providers", reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		responseBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}

	var createdIDP IDP
	err = json.NewDecoder(resp.Body).Decode(&createdIDP)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}

	id := createdIDP.ID
	if id == "" {
		return "", fmt.Errorf("response does not contain id")
	}
	return id, nil
}

func (ts *ExportAPITestSuite) deleteIDP(idpID string) error {
	req, err := http.NewRequest("DELETE", testServerURL+"/identity-providers/"+idpID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}
