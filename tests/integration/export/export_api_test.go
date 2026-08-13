// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package export

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	testServerURL = "https://localhost:8095"

	vcExportConfigHandle     = "export_test_credential"
	vcExportConfigVCT        = "https://credentials.thunderid.local/ExportTestCredential"
	vcExportDefinitionHandle = "export_test_presentation"

	// Seeded from resources/declarative_resources; declarative resources are
	// excluded from wildcard export because they already live in configuration.
	vcExportDeclarativeConfigID     = "decl-credential-config-1"
	vcExportDeclarativeDefinitionID = "decl-presentation-def-1"
)

// ExportAPITestSuite is a test suite for export API tests.
type ExportAPITestSuite struct {
	suite.Suite
	ouID           string
	vcConfigID     string
	vcDefinitionID string
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

	validity := 3600
	configID, err := testutils.CreateCredentialConfiguration(testutils.CredentialConfiguration{
		Handle:      vcExportConfigHandle,
		OUID:        ouID,
		Name:        "Export Test Credential",
		Description: "Credential configuration for export testing",
		Format:      "dc+sd-jwt",
		VCT:         vcExportConfigVCT,
		Claims: []testutils.ClaimMapping{
			{Name: "given_name", DisplayName: "Given Name"},
			{Name: "family_name", DisplayName: "Family Name"},
		},
		ValiditySeconds: &validity,
	})
	if err != nil {
		ts.T().Fatalf("Failed to create test credential configuration: %v", err)
	}
	ts.vcConfigID = configID

	enforceTrustedIssuer := false
	definitionID, err := testutils.CreatePresentationDefinition(testutils.PresentationDefinition{
		Handle:               vcExportDefinitionHandle,
		OUID:                 ouID,
		Name:                 "Export Test Presentation",
		Description:          "Presentation definition for export testing",
		VCT:                  vcExportConfigVCT,
		Format:               "dc+sd-jwt",
		RequestedClaims:      []string{"given_name", "family_name"},
		MandatoryClaims:      []string{"given_name"},
		EnforceTrustedIssuer: &enforceTrustedIssuer,
	})
	if err != nil {
		ts.T().Fatalf("Failed to create test presentation definition: %v", err)
	}
	ts.vcDefinitionID = definitionID
}

// TearDownSuite tears down the test suite.
func (ts *ExportAPITestSuite) TearDownSuite() {
	if ts.vcDefinitionID != "" {
		if err := testutils.DeletePresentationDefinition(ts.vcDefinitionID); err != nil {
			ts.T().Logf("Failed to delete test presentation definition: %v", err)
		}
	}
	if ts.vcConfigID != "" {
		if err := testutils.DeleteCredentialConfiguration(ts.vcConfigID); err != nil {
			ts.T().Logf("Failed to delete test credential configuration: %v", err)
		}
	}
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
	ts.Assert().Contains(yamlContent, "clientId: {{.EXPORT_TEST_APP_CLIENT_ID}}")
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
		Connections: []string{idpID},
	}

	yamlContent, err := ts.exportResourcesYAML(exportRequest)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	// Verify the exported YAML content
	ts.Assert().Contains(yamlContent, "name: Export Test IDP")
	ts.Assert().Contains(yamlContent, "description: Test identity provider for export functionality")
	ts.Assert().Contains(yamlContent, "type: oauth")
	ts.Assert().Contains(yamlContent, "clientId: export_test_oauth_client")
	ts.Assert().Contains(yamlContent, "clientSecret: {{.EXPORT_TEST_IDP_CLIENT_SECRET}}")
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
		Connections: []string{idpID1, idpID2},
	}

	yamlContent, err := ts.exportResourcesYAML(exportRequest)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	// Verify both IDPs are in the export
	ts.Assert().Contains(yamlContent, "name: GitHub IDP Export")
	ts.Assert().Contains(yamlContent, "name: Google IDP Export")
	ts.Assert().Contains(yamlContent, "type: oauth")
	ts.Assert().Contains(yamlContent, "type: oidc")
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
		Applications: []string{appID},
		Connections:  []string{idpID},
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
		Connections: []string{"*"},
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
		Connections: []string{idpID},
	}

	yamlContent, err := ts.exportResourcesYAML(exportRequest)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	// Verify typed fields: only the secret field (clientSecret) is parameterized.
	ts.Assert().Contains(yamlContent, "clientId: props_test_client")
	ts.Assert().Contains(yamlContent, "clientSecret: {{.PROPERTIES_TEST_IDP_CLIENT_SECRET}}")
	ts.Assert().Contains(yamlContent, "redirectUri: https://localhost:3000/callback")
	ts.Assert().Contains(yamlContent, "- openid")
	ts.Assert().Contains(yamlContent, "- email")
	ts.Assert().Contains(yamlContent, "- profile")
}

// TestExportWithInvalidIdentityProviderID tests export with invalid IDP ID.
func (ts *ExportAPITestSuite) TestExportWithInvalidIdentityProviderID() {
	// Test export with invalid IDP ID
	invalidExportRequest := ExportRequest{
		Connections: []string{"invalid-uuid"},
	}

	_, err := ts.exportResourcesYAML(invalidExportRequest)
	ts.Require().Error(err)
}

// TestCredentialConfigurationExport exports a credential configuration by ID and
// verifies the emitted document carries the fields an import would need.
func (ts *ExportAPITestSuite) TestCredentialConfigurationExport() {
	yamlContent, err := ts.exportResourcesYAML(ExportRequest{
		CredentialConfigurations: []string{ts.vcConfigID},
	})
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	ts.Assert().Contains(yamlContent, "resource_type: credential_configuration")
	ts.Assert().Contains(yamlContent, "handle: "+vcExportConfigHandle)
	ts.Assert().Contains(yamlContent, "vct: "+vcExportConfigVCT)
	ts.Assert().Contains(yamlContent, "format: dc+sd-jwt")
	ts.Assert().Contains(yamlContent, "name: given_name")
	ts.Assert().Contains(yamlContent, "name: family_name")
}

// TestPresentationDefinitionExport exports a presentation definition by ID and
// verifies the emitted document carries its claim sets.
func (ts *ExportAPITestSuite) TestPresentationDefinitionExport() {
	yamlContent, err := ts.exportResourcesYAML(ExportRequest{
		PresentationDefinitions: []string{ts.vcDefinitionID},
	})
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	ts.Assert().Contains(yamlContent, "resource_type: presentation_definition")
	ts.Assert().Contains(yamlContent, "handle: "+vcExportDefinitionHandle)
	ts.Assert().Contains(yamlContent, "vct: "+vcExportConfigVCT)
	ts.Assert().Contains(yamlContent, "given_name")
	ts.Assert().Contains(yamlContent, "family_name")
}

// TestVCResourcesExportYAML exports both VC resource types in a single request.
func (ts *ExportAPITestSuite) TestVCResourcesExportYAML() {
	yamlContent, err := ts.exportResourcesYAML(ExportRequest{
		CredentialConfigurations: []string{ts.vcConfigID},
		PresentationDefinitions:  []string{ts.vcDefinitionID},
	})
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	ts.Assert().Contains(yamlContent, "resource_type: credential_configuration")
	ts.Assert().Contains(yamlContent, "resource_type: presentation_definition")
	ts.Assert().Contains(yamlContent, "handle: "+vcExportConfigHandle)
	ts.Assert().Contains(yamlContent, "handle: "+vcExportDefinitionHandle)
}

// TestCredentialConfigurationExportWithWildcard exports every runtime credential
// configuration and verifies declarative ones are excluded: they are already
// under configuration management and must not be re-exported.
func (ts *ExportAPITestSuite) TestCredentialConfigurationExportWithWildcard() {
	yamlContent, err := ts.exportResourcesYAML(ExportRequest{
		CredentialConfigurations: []string{"*"},
	})
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	ts.Assert().Contains(yamlContent, "handle: "+vcExportConfigHandle)
	ts.Assert().NotContains(yamlContent, vcExportDeclarativeConfigID,
		"declarative credential configurations must be excluded from wildcard export")
}

// TestPresentationDefinitionExportWithWildcard exports every runtime presentation
// definition and verifies declarative ones are excluded.
func (ts *ExportAPITestSuite) TestPresentationDefinitionExportWithWildcard() {
	yamlContent, err := ts.exportResourcesYAML(ExportRequest{
		PresentationDefinitions: []string{"*"},
	})
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	ts.Assert().Contains(yamlContent, "handle: "+vcExportDefinitionHandle)
	ts.Assert().NotContains(yamlContent, vcExportDeclarativeDefinitionID,
		"declarative presentation definitions must be excluded from wildcard export")
}

// TestExportWithInvalidCredentialConfigurationID tests export with an unknown
// credential configuration ID.
func (ts *ExportAPITestSuite) TestExportWithInvalidCredentialConfigurationID() {
	_, err := ts.exportResourcesYAML(ExportRequest{
		CredentialConfigurations: []string{"11111111-2222-3333-4444-555555555555"},
	})
	ts.Require().Error(err)
}

// TestExportWithInvalidPresentationDefinitionID tests export with an unknown
// presentation definition ID.
func (ts *ExportAPITestSuite) TestExportWithInvalidPresentationDefinitionID() {
	_, err := ts.exportResourcesYAML(ExportRequest{
		PresentationDefinitions: []string{"11111111-2222-3333-4444-555555555555"},
	})
	ts.Require().Error(err)
}

// Helper functions

func (ts *ExportAPITestSuite) createApplication(app Application) (string, error) {
	if app.Type == "" {
		app.Type = "fullstack"
	}
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

// idpVendorRegistryMu guards idpVendorRegistry.
var idpVendorRegistryMu sync.RWMutex

// idpVendorRegistry maps an IDP ID (as returned by createIDP) to the /connections vendor path
// it was created under, so deleteIDP (which only takes an ID) can address the right vendor route.
var idpVendorRegistry = map[string]string{}

// idpVendorPath maps a legacy IDP Type value (e.g. "OAUTH") to its /connections vendor path.
func idpVendorPath(idpType string) (string, error) {
	switch strings.ToUpper(idpType) {
	case "GOOGLE":
		return "google", nil
	case "GITHUB":
		return "github", nil
	case "OIDC":
		return "oidc", nil
	case "OAUTH":
		return "oauth", nil
	default:
		return "", fmt.Errorf("unsupported IDP type for /connections: %s", idpType)
	}
}

// idpToConnectionBody converts a legacy IDP{Properties: [...]} fixture into the typed camelCase
// body /connections/{vendor} expects. Every IdP-backed vendor shares the same property key set;
// a vendor's request struct simply ignores fields it doesn't declare.
func idpToConnectionBody(idp IDP) map[string]interface{} {
	fieldByProp := map[string]string{
		"client_id":              "clientId",
		"client_secret":          "clientSecret",
		"redirect_uri":           "redirectUri",
		"authorization_endpoint": "authorizationEndpoint",
		"token_endpoint":         "tokenEndpoint",
		"userinfo_endpoint":      "userInfoEndpoint",
		"jwks_endpoint":          "jwksEndpoint",
		"issuer":                 "issuer",
	}
	body := map[string]interface{}{"name": idp.Name, "description": idp.Description}
	for _, prop := range idp.Properties {
		if prop.Name == "scopes" {
			body["scopes"] = strings.Split(prop.Value, ",")
			continue
		}
		if field, ok := fieldByProp[prop.Name]; ok {
			body[field] = prop.Value
		}
	}
	return body
}

func (ts *ExportAPITestSuite) createIDP(idp IDP) (string, error) {
	vendor, err := idpVendorPath(idp.Type)
	if err != nil {
		return "", err
	}

	idpJSON, err := json.Marshal(idpToConnectionBody(idp))
	if err != nil {
		return "", fmt.Errorf("failed to marshal IDP: %w", err)
	}

	reqBody := bytes.NewReader(idpJSON)
	req, err := http.NewRequest("POST", testServerURL+"/connections/"+vendor, reqBody)
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

	idpVendorRegistryMu.Lock()
	idpVendorRegistry[id] = vendor
	idpVendorRegistryMu.Unlock()

	return id, nil
}

func (ts *ExportAPITestSuite) deleteIDP(idpID string) error {
	idpVendorRegistryMu.RLock()
	vendor, ok := idpVendorRegistry[idpID]
	idpVendorRegistryMu.RUnlock()
	if !ok {
		return fmt.Errorf("no /connections vendor registered for IDP ID %q", idpID)
	}

	req, err := http.NewRequest("DELETE", testServerURL+"/connections/"+vendor+"/"+idpID, nil)
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

	idpVendorRegistryMu.Lock()
	delete(idpVendorRegistry, idpID)
	idpVendorRegistryMu.Unlock()
	return nil
}

// yamlValue returns the first of names present in node. The exported nested keys are snake_case today
// while the declarative contract calls for camelCase (G13), so navigating tolerantly keeps this a
// round-trip check rather than a characterization of the casing bug: it passes now and keeps passing once
// G13 is fixed. Asserting on the spelling would be a tripwire test, which this plan does not use.
func yamlValue(node map[string]interface{}, names ...string) (interface{}, bool) {
	for _, name := range names {
		if value, ok := node[name]; ok {
			return value, true
		}
	}
	return nil, false
}

// yamlMapping is yamlValue narrowed to a nested mapping.
func yamlMapping(node map[string]interface{}, names ...string) (map[string]interface{}, bool) {
	value, ok := yamlValue(node, names...)
	if !ok {
		return nil, false
	}
	mapping, ok := value.(map[string]interface{})
	return mapping, ok
}

// exportPlaceholder matches the {{.VAR}} tokens the exporter substitutes for secrets, and the
// {{ t(...) }} expressions it preserves verbatim.
var exportPlaceholder = regexp.MustCompile(`{{[^}]*}}`)

// findExportedConnection decodes every YAML document in an export bundle and returns the one whose id
// matches. The bundle carries "# File:" comments and separates documents with ---, so it decodes as a
// stream; but an exported secret is rendered as a bare {{.VAR}} token, which YAML reads as the start of
// a flow mapping and rejects. The tokens are replaced with a plain scalar first, since this test is
// about the attribute configuration rather than the placeholder syntax.
func (ts *ExportAPITestSuite) findExportedConnection(bundle, id string) map[string]interface{} {
	ts.T().Helper()
	parseable := exportPlaceholder.ReplaceAllString(bundle, "__redacted__")
	decoder := yaml.NewDecoder(strings.NewReader(parseable))
	for {
		var document map[string]interface{}
		err := decoder.Decode(&document)
		if errors.Is(err, io.EOF) {
			break
		}
		ts.Require().NoError(err, "failed to decode exported YAML: %s", parseable)
		if document == nil {
			continue
		}
		if documentID, ok := document["id"].(string); ok && documentID == id {
			return document
		}
	}
	ts.Require().Fail("exported bundle did not contain connection "+id, parseable)
	return nil
}

// AD2 verifies an attributeConfiguration survives export intact. The exported document is what a
// declarative deployment consumes, so a section dropped or mangled here would silently un-configure a
// connection on re-import. The whole structure is compared after parsing, because substring checks would
// pass on a document whose values had been reordered, truncated or emptied.
func (ts *ExportAPITestSuite) TestIdentityProviderExportPreservesAttributeConfiguration() {
	// Targets resolve against the bootstrapped Person user type, so no fixture is needed here.
	idpID, err := testutils.CreateIDP(testutils.IDP{
		Name:        "Export Attr Config IDP",
		Description: "Identity provider carrying an attribute configuration",
		Type:        "GOOGLE",
		Properties: []testutils.IDPProperty{
			{Name: "client_id", Value: "export_attr_client"},
			{Name: "client_secret", Value: "export_attr_secret", IsSecret: true},
			{Name: "redirect_uri", Value: "https://localhost:8095/callback"},
		},
		AttributeConfiguration: &testutils.AttributeConfiguration{
			UserTypeResolution: &testutils.UserTypeResolution{Default: "Person"},
			UserTypeAttributeMappings: []testutils.UserTypeAttributeMapping{{
				UserType: "Person",
				Attributes: []testutils.AttributeMapping{
					{ExternalAttribute: "given_name", LocalAttribute: "given_name"},
					{ExternalAttribute: "family_name", LocalAttribute: "family_name"},
				},
			}},
			AccountLinking: &testutils.AccountLinking{Attributes: []string{"email"}},
		},
	})
	ts.Require().NoError(err)
	defer func() {
		ts.Require().NoError(testutils.DeleteIDP(idpID))
	}()

	yamlContent, err := ts.exportResourcesYAML(ExportRequest{Connections: []string{idpID}})
	ts.Require().NoError(err)
	ts.Require().NotEmpty(yamlContent)

	document := ts.findExportedConnection(yamlContent, idpID)
	config, ok := yamlMapping(document, "attributeConfiguration")
	ts.Require().True(ok, "exported connection should carry an attributeConfiguration: %v", document)

	resolution, ok := yamlMapping(config, "user_type_resolution", "userTypeResolution")
	ts.Require().True(ok, "exported configuration should carry a resolution section: %v", config)
	ts.Equal("Person", resolution["default"])

	linking, ok := yamlMapping(config, "accountLinking", "account_linking")
	ts.Require().True(ok, "exported configuration should carry an account-linking section: %v", config)
	ts.Equal([]interface{}{"email"}, linking["attributes"],
		"the account-linking list must survive export exactly")

	rawMappings, ok := yamlValue(config, "user_type_attribute_mappings", "userTypeAttributeMappings")
	ts.Require().True(ok, "exported configuration should carry mapping entries: %v", config)
	mappings, ok := rawMappings.([]interface{})
	ts.Require().True(ok)
	ts.Require().Len(mappings, 1, "exactly the one configured mapping entry should be exported")

	entry, ok := mappings[0].(map[string]interface{})
	ts.Require().True(ok)
	entryUserType, ok := yamlValue(entry, "user_type", "userType")
	ts.Require().True(ok, "mapping entry should name its user type: %v", entry)
	ts.Equal("Person", entryUserType)

	rawAttributes, ok := entry["attributes"]
	ts.Require().True(ok, "mapping entry should carry its attributes: %v", entry)
	attributes, ok := rawAttributes.([]interface{})
	ts.Require().True(ok)
	ts.Require().Len(attributes, 2, "both configured pairs should be exported, in order")

	expectedPairs := [][2]string{{"given_name", "given_name"}, {"family_name", "family_name"}}
	for i, expected := range expectedPairs {
		pair, ok := attributes[i].(map[string]interface{})
		ts.Require().True(ok)
		external, ok := yamlValue(pair, "external_attribute", "externalAttribute")
		ts.Require().True(ok, "pair %d should name its external attribute: %v", i, pair)
		local, ok := yamlValue(pair, "local_attribute", "localAttribute")
		ts.Require().True(ok, "pair %d should name its local attribute: %v", i, pair)
		ts.Equal(expected[0], external)
		ts.Equal(expected[1], local)
	}
}
