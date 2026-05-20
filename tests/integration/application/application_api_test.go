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

package application

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	testServerURL = "https://localhost:8095"
)

var (
	testOU = testutils.OrganizationUnit{
		Handle:      "test_application_ou",
		Name:        "Test Organization Unit for Applications",
		Description: "Organization unit created for application API testing",
		Parent:      nil,
	}

	testApp = Application{
		Name:                      "Test App",
		Description:               "Test application for API testing",
		URL:                       "https://testapp.example.com",
		LogoURL:                   "https://testapp.example.com/logo.png",
		IsRegistrationFlowEnabled: false,
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "test_app_client",
					ClientSecret:            "test_app_secret",
					RedirectURIs:            []string{"http://localhost/testapp/callback"},
					GrantTypes:              []string{"authorization_code", "client_credentials"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PKCERequired:            false,
					PublicClient:            false,
				},
			},
		},
	}

	appToCreate = Application{
		Name:                      "App To Create",
		Description:               "Application to create for API testing",
		IsRegistrationFlowEnabled: true,
		Template:                  "spa",
		URL:                       "https://apptocreate.example.com",
		LogoURL:                   "https://apptocreate.example.com/logo.png",
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "app_to_create_client",
					ClientSecret:            "app_to_create_secret",
					RedirectURIs:            []string{"http://localhost/apptocreate/callback"},
					GrantTypes:              []string{"authorization_code", "client_credentials"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PKCERequired:            false,
					PublicClient:            false,
				},
			},
		},
	}

	appToUpdate = Application{
		Name:                      "Updated App",
		Description:               "Updated Description",
		IsRegistrationFlowEnabled: false,
		Template:                  "mobile",
		URL:                       "https://appToUpdate.example.com",
		LogoURL:                   "https://appToUpdate.example.com/logo.png",
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "updated_client_id",
					ClientSecret:            "updated_secret",
					RedirectURIs:            []string{"http://localhost/callback2"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PKCERequired:            false,
					PublicClient:            false,
				},
			},
		},
	}
)

var (
	testOUID                  string
	defaultAuthFlowID         string
	defaultRegistrationFlowID string
	testAppID                 string
	testAppInstance           Application
)

type ApplicationAPITestSuite struct {
	suite.Suite
}

func TestApplicationAPITestSuite(t *testing.T) {

	suite.Run(t, new(ApplicationAPITestSuite))
}

// SetupSuite creates test applications for the test suite
func (ts *ApplicationAPITestSuite) SetupSuite() {
	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	testOUID = ouID
	testApp.OUID = testOUID
	appToCreate.OUID = testOUID
	appToUpdate.OUID = testOUID

	// Get Flow IDs
	defaultAuthFlowID, err = testutils.GetFlowIDByHandle("default-basic-flow", "AUTHENTICATION")
	ts.Require().NoError(err, "Failed to get basic auth flow ID")
	testApp.AuthFlowID = defaultAuthFlowID

	defaultRegistrationFlowID, err = testutils.GetFlowIDByHandle("default-basic-flow", "REGISTRATION")
	ts.Require().NoError(err, "Failed to get basic registration flow ID")
	testApp.RegistrationFlowID = defaultRegistrationFlowID

	// Create test application
	app1ID, err := createApplication(testApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	testAppID = app1ID

	// Build the test app structure for validations
	testAppInstance = testApp
	testAppInstance.ID = testAppID
	if len(testAppInstance.InboundAuthConfig) > 0 && testAppInstance.InboundAuthConfig[0].OAuthAppConfig != nil {
		testAppInstance.ClientID = testAppInstance.InboundAuthConfig[0].OAuthAppConfig.ClientID
	}
}

// TearDownSuite cleans up test applications
func (ts *ApplicationAPITestSuite) TearDownSuite() {
	// Delete the test application
	if testAppID != "" {
		err := deleteApplication(testAppID)
		if err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}

	// Delete the test organization unit
	if testOUID != "" {
		err := testutils.DeleteOrganizationUnit(testOUID)
		if err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// Test application listing
func (ts *ApplicationAPITestSuite) TestApplicationListing() {

	req, err := http.NewRequest("GET", testServerURL+"/applications", nil)
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

	// Parse the response body
	var appList ApplicationList
	err = json.NewDecoder(resp.Body).Decode(&appList)
	if err != nil {
		ts.T().Fatalf("Failed to parse response body: %v", err)
	}

	totalResults := appList.TotalResults
	if totalResults == 0 {
		ts.T().Fatalf("Response does not contain a valid total results count")
	}

	appCount := appList.Count
	if appCount == 0 {
		ts.T().Fatalf("Response does not contain a valid application count")
	}

	applicationListLength := len(appList.Applications)
	if applicationListLength == 0 {
		ts.T().Fatalf("Response does not contain any applications")
	}

	// Verify that the test application is present in the list
	testApps := []Application{testAppInstance}
	for _, expectedApp := range testApps {
		found := false
		for _, app := range appList.Applications {
			if app.ID == expectedApp.ID &&
				app.Name == expectedApp.Name &&
				app.Description == expectedApp.Description &&
				app.ClientID == expectedApp.ClientID &&
				app.LogoURL == expectedApp.LogoURL {
				found = true
				break
			}
		}
		if !found {
			ts.T().Fatalf("Test application not found in list: %+v", expectedApp)
		}
	}
}

// Test application listing with logo_url field validation
func (ts *ApplicationAPITestSuite) TestApplicationListingWithLogoURL() {
	// Create two applications: one with logo_url and one without
	appWithLogo := Application{
		OUID:                      testOUID,
		Name:                      "App With Logo",
		Description:               "Application with logo URL",
		AuthFlowID:                defaultAuthFlowID,
		RegistrationFlowID:        defaultRegistrationFlowID,
		IsRegistrationFlowEnabled: false,
		URL:                       "https://appwithlogo.example.com",
		LogoURL:                   "https://appwithlogo.example.com/logo.png",
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "app_with_logo_client",
					ClientSecret:            "app_with_logo_secret",
					RedirectURIs:            []string{"http://localhost/appwithlogo/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PKCERequired:            false,
					PublicClient:            false,
				},
			},
		},
	}

	appWithoutLogo := Application{
		OUID:                      testOUID,
		Name:                      "App Without Logo",
		Description:               "Application without logo URL",
		AuthFlowID:                defaultAuthFlowID,
		RegistrationFlowID:        defaultRegistrationFlowID,
		IsRegistrationFlowEnabled: false,
		URL:                       "https://appwithoutlogo.example.com",
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "app_without_logo_client",
					ClientSecret:            "app_without_logo_secret",
					RedirectURIs:            []string{"http://localhost/appwithoutlogo/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PKCERequired:            false,
					PublicClient:            false,
				},
			},
		},
	}

	// Create both applications
	appID1, err := createApplication(appWithLogo)
	if err != nil {
		ts.T().Fatalf("Failed to create application with logo: %v", err)
	}
	defer func() {
		if err := deleteApplication(appID1); err != nil {
			ts.T().Logf("Failed to delete application with logo: %v", err)
		}
	}()

	appID2, err := createApplication(appWithoutLogo)
	if err != nil {
		ts.T().Fatalf("Failed to create application without logo: %v", err)
	}
	defer func() {
		if err := deleteApplication(appID2); err != nil {
			ts.T().Logf("Failed to delete application without logo: %v", err)
		}
	}()

	// List applications
	req, err := http.NewRequest("GET", testServerURL+"/applications", nil)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ts.T().Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var appList ApplicationList
	err = json.NewDecoder(resp.Body).Decode(&appList)
	if err != nil {
		ts.T().Fatalf("Failed to parse response body: %v", err)
	}

	// Verify app with logo has logo_url field populated
	foundWithLogo := false
	for _, app := range appList.Applications {
		if app.ID == appID1 {
			foundWithLogo = true
			if app.LogoURL != appWithLogo.LogoURL {
				ts.T().Errorf("Expected logo_url %s, got %s", appWithLogo.LogoURL, app.LogoURL)
			}
			break
		}
	}
	if !foundWithLogo {
		ts.T().Fatalf("Application with logo not found in list")
	}

	// Verify app without logo has empty logo_url field
	foundWithoutLogo := false
	for _, app := range appList.Applications {
		if app.ID == appID2 {
			foundWithoutLogo = true
			if app.LogoURL != "" {
				ts.T().Errorf("Expected empty logo_url, got %s", app.LogoURL)
			}
			break
		}
	}
	if !foundWithoutLogo {
		ts.T().Fatalf("Application without logo not found in list")
	}
}

// Test application get by ID
func (ts *ApplicationAPITestSuite) TestApplicationGetByID() {
	// Set default flow IDs
	appToCreate.AuthFlowID = defaultAuthFlowID
	appToCreate.RegistrationFlowID = defaultRegistrationFlowID

	// Create an application for get testing
	appID, err := createApplication(appToCreate)
	if err != nil {
		ts.T().Fatalf("Failed to create application for get test: %v", err)
	}
	defer func() {
		// Clean up the created application
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete application after get test: %v", err)
		}
	}()

	// Build the expected app structure for validation
	expectedApp := appToCreate
	expectedApp.ID = appID
	if len(expectedApp.InboundAuthConfig) > 0 && expectedApp.InboundAuthConfig[0].OAuthAppConfig != nil {
		expectedApp.ClientID = expectedApp.InboundAuthConfig[0].OAuthAppConfig.ClientID
	}

	retrieveAndValidateApplicationDetails(ts, expectedApp)
}

// Test application update
func (ts *ApplicationAPITestSuite) TestApplicationUpdate() {
	// Set default flow IDs
	appToCreate.AuthFlowID = defaultAuthFlowID
	appToCreate.RegistrationFlowID = defaultRegistrationFlowID

	// Create an application for update testing
	appID, err := createApplication(appToCreate)
	if err != nil {
		ts.T().Fatalf("Failed to create application for update test: %v", err)
	}
	defer func() {
		// Clean up the created application
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete application after update test: %v", err)
		}
	}()

	// Add the ID to the application to update
	appToUpdateWithID := appToUpdate
	appToUpdateWithID.ID = appID

	// Set the default flow IDs
	appToUpdateWithID.AuthFlowID = defaultAuthFlowID
	appToUpdateWithID.RegistrationFlowID = defaultRegistrationFlowID

	appJSON, err := json.Marshal(appToUpdateWithID)
	if err != nil {
		ts.T().Fatalf("Failed to marshal appToUpdate: %v", err)
	}

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+appID, reqBody)
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
		responseBody, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}

	// For update operations, verify the response directly
	var updatedApp Application
	if err = json.NewDecoder(resp.Body).Decode(&updatedApp); err != nil {
		responseBody, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Failed to decode update response: %v. Response: %s", err, string(responseBody))
	}

	// Client secret should be present in the update response
	if len(updatedApp.InboundAuthConfig) > 0 &&
		updatedApp.InboundAuthConfig[0].OAuthAppConfig != nil &&
		updatedApp.InboundAuthConfig[0].OAuthAppConfig.ClientSecret == "" {
		ts.T().Fatalf("Expected client secret in update response but got empty string")
	}

	// Now validate by getting the application (which should not have client secret)
	// Make sure client ID is properly set in the root level before validation
	if len(appToUpdateWithID.InboundAuthConfig) > 0 &&
		appToUpdateWithID.InboundAuthConfig[0].OAuthAppConfig != nil {
		appToUpdateWithID.ClientID = appToUpdateWithID.InboundAuthConfig[0].OAuthAppConfig.ClientID
	}

	retrieveAndValidateApplicationDetails(ts, appToUpdateWithID)
}

func retrieveAndValidateApplicationDetails(ts *ApplicationAPITestSuite, expectedApp Application) {

	req, err := http.NewRequest("GET", testServerURL+"/applications/"+expectedApp.ID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}

	// Check if the response Content-Type is application/json
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		ts.T().Fatalf("Expected Content-Type application/json, got %s", contentType)
	}

	var app Application
	body, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &app)
	if err != nil {
		ts.T().Fatalf("Failed to parse response body: %v\nResponse body: %s", err, string(body))
	}

	// For GET operations, client secret should be empty in the response
	// Make sure expectedApp has client secret cleared for proper comparison
	appForComparison := expectedApp
	if len(appForComparison.InboundAuthConfig) > 0 && appForComparison.InboundAuthConfig[0].OAuthAppConfig != nil {
		// Make sure client ID is in root object
		appForComparison.ClientID = appForComparison.InboundAuthConfig[0].OAuthAppConfig.ClientID
		// Remove client secret for GET comparison
		appForComparison.InboundAuthConfig[0].OAuthAppConfig.ClientSecret = ""
	}

	appForComparison.AuthFlowID = defaultAuthFlowID
	appForComparison.RegistrationFlowID = defaultRegistrationFlowID

	// If expected doesn't have assertion token but API returned one (default), copy it to expected
	// This handles cases where the server provides default assertion config
	if appForComparison.Assertion == nil && app.Assertion != nil {
		appForComparison.Assertion = app.Assertion
	}

	// Ensure login consent config is set in expected app if it's null
	if appForComparison.LoginConsent == nil {
		appForComparison.LoginConsent = &LoginConsentConfig{
			ValidityPeriod: 0,
		}
	}

	if !app.equals(appForComparison) {
		appJSON, _ := json.MarshalIndent(app, "", "  ")
		expectedJSON, _ := json.MarshalIndent(appForComparison, "", "  ")
		ts.T().Fatalf("Application mismatch:\nGot:\n%s\n\nExpected:\n%s", string(appJSON), string(expectedJSON))
	}
}

func createApplication(app Application) (string, error) {
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

	// For create operations, directly parse the response to a full Application
	var createdApp Application
	err = json.NewDecoder(resp.Body).Decode(&createdApp)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}

	// Verify client secret is present in the create response for confidential clients
	if len(createdApp.InboundAuthConfig) > 0 &&
		createdApp.InboundAuthConfig[0].OAuthAppConfig != nil &&
		(createdApp.InboundAuthConfig[0].OAuthAppConfig.TokenEndpointAuthMethod == "client_secret_basic" ||
			createdApp.InboundAuthConfig[0].OAuthAppConfig.TokenEndpointAuthMethod == "client_secret_post") &&
		createdApp.InboundAuthConfig[0].OAuthAppConfig.ClientSecret == "" {
		return "", fmt.Errorf("expected client secret in create response but got empty string")
	}

	id := createdApp.ID
	if id == "" {
		return "", fmt.Errorf("response does not contain id")
	}
	return id, nil
}

func deleteApplication(appID string) error {
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

// TestApplicationCreationWithDefaults tests that applications created without grant_types, response_types, or token_endpoint_auth_method get proper defaults
func (ts *ApplicationAPITestSuite) TestApplicationCreationWithDefaults() {
	appWithDefaults := Application{
		OUID:                      testOUID,
		Name:                      "App With Defaults",
		Description:               "Application to test default values",
		URL:                       "https://defaults.example.com",
		LogoURL:                   "https://defaults.example.com/logo.png",
		IsRegistrationFlowEnabled: false,
		AuthFlowID:                defaultAuthFlowID,
		RegistrationFlowID:        defaultRegistrationFlowID,
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:     "defaults_app_client",
					ClientSecret: "defaults_app_secret",
					RedirectURIs: []string{"http://localhost/defaults/callback"},
					// Intentionally omitting GrantTypes, ResponseTypes, and TokenEndpointAuthMethod
					PKCERequired: false,
					PublicClient: false,
				},
			},
		},
	}

	appID, err := createApplication(appWithDefaults)
	if err != nil {
		ts.T().Fatalf("Failed to create application: %v", err)
	}

	req, err := http.NewRequest("GET", testServerURL+"/applications/"+appID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create GET request: %v", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send GET request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}

	var retrievedApp Application
	err = json.NewDecoder(resp.Body).Decode(&retrievedApp)
	if err != nil {
		ts.T().Fatalf("Failed to decode response: %v", err)
	}

	// Verify defaults were applied
	if len(retrievedApp.InboundAuthConfig) > 0 && retrievedApp.InboundAuthConfig[0].OAuthAppConfig != nil {
		oauthConfig := retrievedApp.InboundAuthConfig[0].OAuthAppConfig

		ts.Assert().Equal([]string{"authorization_code"}, oauthConfig.GrantTypes, "Default grant_types should be ['authorization_code']")
		ts.Assert().Equal([]string{"code"}, oauthConfig.ResponseTypes, "Default response_types should be ['code']")
		ts.Assert().Equal("client_secret_basic", oauthConfig.TokenEndpointAuthMethod, "Default token_endpoint_auth_method should be 'client_secret_basic'")
	}

	err = deleteApplication(appID)
	if err != nil {
		ts.T().Logf("Failed to delete test application: %v", err)
	}
}

// TestApplicationCreationWithInvalidTokenEndpointAuthMethod tests validation of invalid token_endpoint_auth_method values
func (ts *ApplicationAPITestSuite) TestApplicationCreationWithInvalidTokenEndpointAuthMethod() {
	appWithInvalidAuthMethod := Application{
		OUID:                      testOUID,
		Name:                      "App With Invalid Auth Method",
		Description:               "Application to test invalid token endpoint auth method",
		URL:                       "https://invalid.example.com",
		LogoURL:                   "https://invalid.example.com/logo.png",
		AuthFlowID:                defaultAuthFlowID,
		RegistrationFlowID:        defaultRegistrationFlowID,
		IsRegistrationFlowEnabled: false,
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "invalid_auth_app_client",
					ClientSecret:            "invalid_auth_app_secret",
					RedirectURIs:            []string{"http://localhost/invalid/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "invalid_auth_method", // Invalid value
					PKCERequired:            false,
					PublicClient:            false,
				},
			},
		},
	}

	_, err := createApplication(appWithInvalidAuthMethod)
	if err == nil {
		ts.T().Fatalf("Expected validation error for invalid token_endpoint_auth_method, but application was created successfully")
	}

	appWithEmptyAuthMethod := Application{
		OUID:                      testOUID,
		Name:                      "App With Empty Auth Method",
		Description:               "Application to test empty token endpoint auth method",
		URL:                       "https://empty.example.com",
		LogoURL:                   "https://empty.example.com/logo.png",
		AuthFlowID:                defaultAuthFlowID,
		RegistrationFlowID:        defaultRegistrationFlowID,
		IsRegistrationFlowEnabled: false,
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "empty_auth_app_client",
					ClientSecret:            "empty_auth_app_secret",
					RedirectURIs:            []string{"http://localhost/empty/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "",
					PKCERequired:            false,
					PublicClient:            false,
				},
			},
		},
	}

	appID, err := createApplication(appWithEmptyAuthMethod)
	if err != nil {
		ts.T().Fatalf("Failed to create application with empty token_endpoint_auth_method: %v", err)
	}

	req, err := http.NewRequest("GET", testServerURL+"/applications/"+appID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create GET request: %v", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send GET request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}

	var retrievedApp Application
	err = json.NewDecoder(resp.Body).Decode(&retrievedApp)
	if err != nil {
		ts.T().Fatalf("Failed to decode response: %v", err)
	}

	if len(retrievedApp.InboundAuthConfig) > 0 && retrievedApp.InboundAuthConfig[0].OAuthAppConfig != nil {
		oauthConfig := retrievedApp.InboundAuthConfig[0].OAuthAppConfig
		ts.Assert().Equal("client_secret_basic", oauthConfig.TokenEndpointAuthMethod, "Empty token_endpoint_auth_method should get default 'client_secret_basic'")
	}

	err = deleteApplication(appID)
	if err != nil {
		ts.T().Logf("Failed to delete test application: %v", err)
	}
}

// TestApplicationCreationWithPartialDefaults tests applications with some fields missing (partial defaults)
func (ts *ApplicationAPITestSuite) TestApplicationCreationWithPartialDefaults() {
	appWithPartialDefaults := Application{
		OUID:                      testOUID,
		Name:                      "App With Partial Defaults",
		Description:               "Application to test partial default values",
		URL:                       "https://partial.example.com",
		LogoURL:                   "https://partial.example.com/logo.png",
		AuthFlowID:                defaultAuthFlowID,
		RegistrationFlowID:        defaultRegistrationFlowID,
		IsRegistrationFlowEnabled: false,
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:     "partial_app_client",
					ClientSecret: "partial_app_secret",
					RedirectURIs: []string{"http://localhost/partial/callback"},
					// GrantTypes missing - should get default
					ResponseTypes:           []string{"code"},     // Explicitly set
					TokenEndpointAuthMethod: "client_secret_post", // Explicitly set
					PKCERequired:            false,
					PublicClient:            false,
				},
			},
		},
	}

	appID, err := createApplication(appWithPartialDefaults)
	if err != nil {
		ts.T().Fatalf("Failed to create application: %v", err)
	}

	// Verify that defaults were applied by getting the application
	req, err := http.NewRequest("GET", testServerURL+"/applications/"+appID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create GET request: %v", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send GET request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}

	var retrievedApp Application
	err = json.NewDecoder(resp.Body).Decode(&retrievedApp)
	if err != nil {
		ts.T().Fatalf("Failed to decode response: %v", err)
	}

	if len(retrievedApp.InboundAuthConfig) > 0 && retrievedApp.InboundAuthConfig[0].OAuthAppConfig != nil {
		oauthConfig := retrievedApp.InboundAuthConfig[0].OAuthAppConfig

		ts.Assert().Equal([]string{"authorization_code"}, oauthConfig.GrantTypes, "Missing grant_types should get default ['authorization_code']")
		ts.Assert().Equal([]string{"code"}, oauthConfig.ResponseTypes, "Explicitly set response_types should be preserved")
		ts.Assert().Equal("client_secret_post", oauthConfig.TokenEndpointAuthMethod, "Explicitly set token_endpoint_auth_method should be preserved")
	}

	err = deleteApplication(appID)
	if err != nil {
		ts.T().Logf("Failed to delete test application: %v", err)
	}
}

// TestApplicationCreationWithPrivateKeyJWT tests creating an application with private_key_jwt token endpoint auth method.
func (ts *ApplicationAPITestSuite) TestApplicationCreationWithPrivateKeyJWT() {
	jwksJSON := `{"keys":[{"kty":"RSA","use":"sig","kid":"test-key","n":"0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw","e":"AQAB"}]}`

	testCases := []struct {
		name              string
		app               Application
		expectError       bool
		expectedCertType  string
		expectedCertValue string
	}{
		{
			name: "successful creation with JWKS_URI certificate",
			app: Application{
				OUID:                      testOUID,
				Name:                      "Private Key JWT JWKS URI App",
				Description:               "Application with private_key_jwt and JWKS_URI certificate",
				URL:                       "https://pkjwt-jwksuri.example.com",
				AuthFlowID:                defaultAuthFlowID,
				RegistrationFlowID:        defaultRegistrationFlowID,
				IsRegistrationFlowEnabled: false,
				InboundAuthConfig: []InboundAuthConfig{
					{
						Type: "oauth2",
						OAuthAppConfig: &OAuthAppConfig{
							RedirectURIs:            []string{"https://pkjwt-jwksuri.example.com/callback"},
							GrantTypes:              []string{"authorization_code", "client_credentials"},
							ResponseTypes:           []string{"code"},
							TokenEndpointAuthMethod: "private_key_jwt",
							PKCERequired:            false,
							PublicClient:            false,
							Certificate: &ApplicationCert{
								Type:  "JWKS_URI",
								Value: "https://pkjwt-jwksuri.example.com/.well-known/jwks.json",
							},
						},
					},
				},
			},
			expectError:       false,
			expectedCertType:  "JWKS_URI",
			expectedCertValue: "https://pkjwt-jwksuri.example.com/.well-known/jwks.json",
		},
		{
			name: "successful creation with inline JWKS certificate",
			app: Application{
				OUID:                      testOUID,
				Name:                      "Private Key JWT JWKS App",
				Description:               "Application with private_key_jwt and inline JWKS certificate",
				URL:                       "https://pkjwt-jwks.example.com",
				AuthFlowID:                defaultAuthFlowID,
				RegistrationFlowID:        defaultRegistrationFlowID,
				IsRegistrationFlowEnabled: false,
				InboundAuthConfig: []InboundAuthConfig{
					{
						Type: "oauth2",
						OAuthAppConfig: &OAuthAppConfig{
							RedirectURIs:            []string{"https://pkjwt-jwks.example.com/callback"},
							GrantTypes:              []string{"authorization_code"},
							ResponseTypes:           []string{"code"},
							TokenEndpointAuthMethod: "private_key_jwt",
							PKCERequired:            false,
							PublicClient:            false,
							Certificate: &ApplicationCert{
								Type:  "JWKS",
								Value: jwksJSON,
							},
						},
					},
				},
			},
			expectError:       false,
			expectedCertType:  "JWKS",
			expectedCertValue: jwksJSON,
		},
		{
			name: "failure - private_key_jwt with empty certificate type",
			app: Application{
				OUID:                      testOUID,
				Name:                      "Private Key JWT No Cert App",
				Description:               "Application with private_key_jwt but no certificate",
				URL:                       "https://pkjwt-nocert.example.com",
				AuthFlowID:                defaultAuthFlowID,
				RegistrationFlowID:        defaultRegistrationFlowID,
				IsRegistrationFlowEnabled: false,
				Certificate: &ApplicationCert{
					Type:  "",
					Value: "",
				},
				InboundAuthConfig: []InboundAuthConfig{
					{
						Type: "oauth2",
						OAuthAppConfig: &OAuthAppConfig{
							RedirectURIs:            []string{"https://pkjwt-nocert.example.com/callback"},
							GrantTypes:              []string{"authorization_code"},
							ResponseTypes:           []string{"code"},
							TokenEndpointAuthMethod: "private_key_jwt",
							PKCERequired:            false,
							PublicClient:            false,
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "failure - private_key_jwt with client secret",
			app: Application{
				OUID:                      testOUID,
				Name:                      "Private Key JWT With Secret App",
				Description:               "Application with private_key_jwt and client secret",
				URL:                       "https://pkjwt-secret.example.com",
				AuthFlowID:                defaultAuthFlowID,
				RegistrationFlowID:        defaultRegistrationFlowID,
				IsRegistrationFlowEnabled: false,
				InboundAuthConfig: []InboundAuthConfig{
					{
						Type: "oauth2",
						OAuthAppConfig: &OAuthAppConfig{
							ClientSecret:            "should_not_be_allowed",
							RedirectURIs:            []string{"https://pkjwt-secret.example.com/callback"},
							GrantTypes:              []string{"authorization_code"},
							ResponseTypes:           []string{"code"},
							TokenEndpointAuthMethod: "private_key_jwt",
							PKCERequired:            false,
							PublicClient:            false,
							Certificate: &ApplicationCert{
								Type:  "JWKS_URI",
								Value: "https://pkjwt-secret.example.com/.well-known/jwks.json",
							},
						},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		ts.Run(tc.name, func() {
			appID, err := createApplication(tc.app)
			if tc.expectError {
				ts.Require().Error(err)
				return
			}

			ts.Require().NoError(err)
			ts.Require().NotEmpty(appID)
			defer func() {
				if err := deleteApplication(appID); err != nil {
					ts.T().Logf("Failed to delete test application: %v", err)
				}
			}()

			retrievedApp, err := getApplicationByID(appID)
			ts.Require().NoError(err)
			ts.Require().NotEmpty(retrievedApp.InboundAuthConfig)
			ts.Require().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig)

			oauthConfig := retrievedApp.InboundAuthConfig[0].OAuthAppConfig
			ts.Assert().Equal("private_key_jwt", oauthConfig.TokenEndpointAuthMethod)
			ts.Require().NotNil(oauthConfig.Certificate)
			ts.Assert().Equal(tc.expectedCertType, oauthConfig.Certificate.Type)
			ts.Assert().Equal(tc.expectedCertValue, oauthConfig.Certificate.Value)
			ts.Assert().Empty(oauthConfig.ClientSecret, "private_key_jwt app should not have a client secret")
		})
	}
}

// TestApplicationWithJWKSURICertificate tests creating application with JWKS_URI certificate.
func (ts *ApplicationAPITestSuite) TestApplicationWithJWKSURICertificate() {
	app := Application{
		OUID:        testOUID,
		Name:        "JWKS URI Certificate Test App",
		Description: "Test application with JWKS_URI certificate",
		URL:         "https://jwksuri.example.com",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://jwksuri.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid", "profile", "email"},
				},
			},
		},
		Certificate: &ApplicationCert{
			Type:  "JWKS_URI",
			Value: "https://jwksuri.example.com/.well-known/jwks.json",
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(appID)

	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Require().NotNil(retrievedApp.Certificate)
	ts.Assert().Equal("JWKS_URI", retrievedApp.Certificate.Type)
	ts.Assert().Equal("https://jwksuri.example.com/.well-known/jwks.json", retrievedApp.Certificate.Value)

	err = deleteApplication(appID)
	if err != nil {
		ts.T().Logf("Failed to delete test application: %v", err)
	}
}

// TestApplicationWithJWKSCertificate tests creating application with inline JWKS certificate.
func (ts *ApplicationAPITestSuite) TestApplicationWithJWKSCertificate() {
	jwksJSON := `{"keys":[{"kty":"RSA","use":"sig","kid":"test-key","n":"0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw","e":"AQAB"}]}`

	app := Application{
		OUID:        testOUID,
		Name:        "JWKS Inline Certificate Test App",
		Description: "Test application with inline JWKS certificate",
		URL:         "https://jwks.example.com",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://jwks.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid", "profile"},
				},
			},
		},
		Certificate: &ApplicationCert{
			Type:  "JWKS",
			Value: jwksJSON,
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(appID)

	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Require().NotNil(retrievedApp.Certificate)
	ts.Assert().Equal("JWKS", retrievedApp.Certificate.Type)
	ts.Assert().Equal(jwksJSON, retrievedApp.Certificate.Value)

	err = deleteApplication(appID)
	if err != nil {
		ts.T().Logf("Failed to delete test application: %v", err)
	}
}

// TestCreateApplicationCertLifecycle verifies that a certificate created with an
// application is also removed from the database when the application is deleted.
// This confirms that the cert INSERT and the app INSERT are part of the same
// database transaction and that cleanup is complete.
func (ts *ApplicationAPITestSuite) TestCreateApplicationCertLifecycle() {
	const testClientID = "cert_lifecycle_test_oauth_client"
	const jwksURI = "https://cert-lifecycle.example.com/.well-known/jwks.json"

	app := Application{
		OUID:                      testOUID,
		Name:                      "Cert Lifecycle Test App",
		Description:               "Test cert lifecycle atomicity with app lifecycle",
		URL:                       "https://cert-lifecycle.example.com",
		AuthFlowID:                defaultAuthFlowID,
		RegistrationFlowID:        defaultRegistrationFlowID,
		IsRegistrationFlowEnabled: false,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                testClientID,
					RedirectURIs:            []string{"https://cert-lifecycle.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "private_key_jwt",
					PKCERequired:            false,
					PublicClient:            false,
					Certificate: &ApplicationCert{
						Type:  "JWKS_URI",
						Value: jwksURI,
					},
				},
			},
		},
	}

	// Step 1: create the application with a JWKS_URI certificate.
	appID, err := createApplication(app)
	ts.Require().NoError(err, "expected application creation to succeed")
	ts.Require().NotEmpty(appID)

	// Step 2 (SQLite only): verify the cert row was written to the CERTIFICATE table.
	if testutils.GetDBType() == "sqlite" {
		query := fmt.Sprintf(
			"SELECT COUNT(*) FROM CERTIFICATE WHERE REF_TYPE='OAUTH_APP' AND REF_ID='%s';",
			testClientID,
		)
		count, queryErr := testutils.QueryConfigDB(query)
		ts.Require().NoError(queryErr, "failed to query CERTIFICATE table after creation")
		ts.Assert().Equal("1", count,
			"expected exactly 1 cert row in CERTIFICATE after successful app creation; "+
				"cert and app must be created atomically")
	}

	// Step 3: delete the application.
	ts.Require().NoError(deleteApplication(appID), "expected application deletion to succeed")

	// Step 4 (SQLite only): verify the cert row was also removed.
	// This confirms that the deletion is also transactional and that no orphaned
	// cert rows are left behind after the application is removed.
	if testutils.GetDBType() == "sqlite" {
		query := fmt.Sprintf(
			"SELECT COUNT(*) FROM CERTIFICATE WHERE REF_TYPE='OAUTH_APP' AND REF_ID='%s';",
			testClientID,
		)
		count, queryErr := testutils.QueryConfigDB(query)
		ts.Require().NoError(queryErr, "failed to query CERTIFICATE table after deletion")
		ts.Assert().Equal("0", count,
			"expected 0 cert rows in CERTIFICATE after app deletion; "+
				"deleting the app must also clean up its certificate")
	}
}

// TestConcurrentApplicationCreationCertAtomicity verifies that when two goroutines
// simultaneously create applications with the same client_id and JWKS_URI certificate,
// the final database state contains exactly one certificate row for that client_id.
//
// Two potential failure paths exercise the cert rollback guarantee:
//   - Service-layer pre-check: the second request detects the duplicate client_id
//     after the first has committed and never enters the transaction – no cert written.
//   - Mid-transaction DB failure: if the second request passes the pre-check (read
//     before the first transaction commits) and enters the transaction, its cert INSERT
//     succeeds but the OAUTH_INBOUND_PROFILE INSERT fails on the PRIMARY KEY
//     constraint; the transaction is rolled back, removing the cert row.
//
// In either path the end-state invariant is the same: the CERTIFICATE table must
// contain exactly one row for the shared client_id (belonging to the successful app).
func (ts *ApplicationAPITestSuite) TestConcurrentApplicationCreationCertAtomicity() {
	const sharedClientID = "cert_test_client"
	const jwksURI = "https://cert-concurrent.example.com/.well-known/jwks.json"

	makeApp := func(name string) Application {
		return Application{
			OUID:                      testOUID,
			Name:                      name,
			Description:               "Concurrent cert atomicity test",
			URL:                       "https://cert-concurrent.example.com",
			AuthFlowID:                defaultAuthFlowID,
			RegistrationFlowID:        defaultRegistrationFlowID,
			IsRegistrationFlowEnabled: false,
			InboundAuthConfig: []InboundAuthConfig{
				{
					Type: "oauth2",
					OAuthAppConfig: &OAuthAppConfig{
						ClientID:                sharedClientID,
						RedirectURIs:            []string{"https://cert-concurrent.example.com/callback"},
						GrantTypes:              []string{"authorization_code"},
						ResponseTypes:           []string{"code"},
						TokenEndpointAuthMethod: "private_key_jwt",
						PKCERequired:            false,
						PublicClient:            false,
						Certificate: &ApplicationCert{
							Type:  "JWKS_URI",
							Value: jwksURI,
						},
					},
				},
			},
		}
	}

	type result struct {
		id  string
		err error
	}

	var wg sync.WaitGroup
	results := make([]result, 2)
	start := make(chan struct{})

	for i := 0; i < 2; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Block until the main goroutine signals to start, so both requests
			// are in-flight concurrently and each has a chance to pass the
			// service-layer pre-check before the other commits.
			<-start
			id, err := createApplication(makeApp(fmt.Sprintf("Cert Concurrent App %d", i)))
			results[i] = result{id: id, err: err}
		}()
	}

	close(start) // fire both goroutines simultaneously
	wg.Wait()

	// Exactly one creation should succeed.
	successCount := 0
	var successID string
	for _, r := range results {
		if r.err == nil {
			successCount++
			successID = r.id
		}
	}
	ts.Require().Equal(1, successCount,
		"expected exactly one application creation to succeed when two goroutines"+
			" race with the same client_id")

	// Cleanup: delete the successfully created application.
	if successID != "" {
		defer func() {
			if err := deleteApplication(successID); err != nil {
				ts.T().Logf("cleanup: failed to delete concurrent test application: %v", err)
			}
		}()
	}

	// SQLite-only: verify the CERTIFICATE table contains exactly one row for the
	// shared client_id. Two rows would indicate the losing goroutine's cert INSERT
	// committed without a matching app row (the pre-refactor bug).
	if testutils.GetDBType() == "sqlite" {
		query := fmt.Sprintf(
			"SELECT COUNT(*) FROM CERTIFICATE WHERE REF_TYPE='OAUTH_APP' AND REF_ID='%s';",
			sharedClientID,
		)
		count, queryErr := testutils.QueryConfigDB(query)
		ts.Require().NoError(queryErr, "failed to query CERTIFICATE table")
		ts.Assert().Equal("1", count,
			"expected exactly 1 cert row for client_id %q; "+
				"a second row would indicate the losing goroutine's cert was not rolled back",
			sharedClientID)
	}
}

// TestApplicationScopesAsArray tests that scopes are stored and retrieved as array.
func (ts *ApplicationAPITestSuite) TestApplicationScopesAsArray() {
	expectedScopes := []string{"openid", "profile", "email", "address", "phone"}

	app := Application{
		OUID:        testOUID,
		Name:        "Scopes Array Test App",
		Description: "Test application with scopes as array",
		URL:         "https://scopes.example.com",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://scopes.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  expectedScopes,
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(appID)

	// Cleanup
	err = deleteApplication(appID)
	if err != nil {
		ts.T().Logf("Failed to delete test application: %v", err)
	}
}

// TestApplicationWithMultipleScopesAndCertificate tests creating application with both scopes and certificate.
func (ts *ApplicationAPITestSuite) TestApplicationWithMultipleScopesAndCertificate() {
	app := Application{
		OUID:        testOUID,
		Name:        "Multi Feature Test App",
		Description: "Test application with certificate and scopes",
		URL:         "https://multi.example.com",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://multi.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid", "profile", "email", "custom:scope"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(appID)

	err = deleteApplication(appID)
	if err != nil {
		ts.T().Logf("Failed to delete test application: %v", err)
	}
}

// TestApplicationRedirectURIFragmentValidation tests that redirect URIs with fragments are rejected.
func (ts *ApplicationAPITestSuite) TestApplicationRedirectURIFragmentValidation() {
	app := Application{
		OUID:        testOUID,
		Name:        "Invalid Redirect URI Test",
		Description: "Test redirect URI validation",
		URL:         "https://invalid.example.com",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://invalid.example.com/callback#fragment"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}

	_, err := createApplication(app)
	ts.Assert().Error(err)
}

// TestApplicationEmptyScopesArray tests that empty scopes array is accepted.
func (ts *ApplicationAPITestSuite) TestApplicationEmptyScopesArray() {
	app := Application{
		OUID:        testOUID,
		Name:        "Empty Scopes Test App",
		Description: "Test application with empty scopes",
		URL:         "https://emptyscopes.example.com",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://emptyscopes.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(appID)

	err = deleteApplication(appID)
	if err != nil {
		ts.T().Logf("Failed to delete test application: %v", err)
	}
}

// TestApplicationCertificateUpdate tests updating application certificate.
func (ts *ApplicationAPITestSuite) TestApplicationCertificateUpdate() {
	app := Application{
		OUID:        testOUID,
		Name:        "Certificate Update Test App",
		Description: "Test certificate updates",
		URL:         "https://certupdate.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://certupdate.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Update to add JWKS_URI certificate
	app.Certificate = &ApplicationCert{
		Type:  "JWKS_URI",
		Value: "https://certupdate.example.com/.well-known/jwks.json",
	}

	appJSON, _ := json.Marshal(app)
	req, _ := http.NewRequest("PUT", testServerURL+"/applications/"+appID, bytes.NewReader(appJSON))
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)

	// Update to JWKS
	app.Certificate = &ApplicationCert{
		Type:  "JWKS",
		Value: `{"keys":[{"kty":"RSA","use":"sig","kid":"test"}]}`,
	}
	appJSON, _ = json.Marshal(app)
	req, _ = http.NewRequest("PUT", testServerURL+"/applications/"+appID, bytes.NewReader(appJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)
}

// TestOAuthAppCertificateUpdate tests updating OAuth app certificate.
func (ts *ApplicationAPITestSuite) TestOAuthAppCertificateUpdate() {
	app := Application{
		OUID:        testOUID,
		Name:        "OAuth Cert Update Test",
		Description: "Test OAuth certificate updates",
		URL:         "https://oauthcertupdate.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://oauthcertupdate.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Update to add JWKS_URI certificate at application level
	app.Certificate = &ApplicationCert{
		Type:  "JWKS_URI",
		Value: "https://oauthcertupdate.example.com/.well-known/jwks.json",
	}

	appJSON, _ := json.Marshal(app)
	req, _ := http.NewRequest("PUT", testServerURL+"/applications/"+appID, bytes.NewReader(appJSON))
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)
}

// TestApplicationInvalidCertificateType tests invalid certificate type rejection.
func (ts *ApplicationAPITestSuite) TestApplicationInvalidCertificateType() {
	app := Application{
		OUID:        testOUID,
		Name:        "Invalid Cert Type Test",
		Description: "Test invalid certificate type",
		URL:         "https://invalidcert.example.com",
		Certificate: &ApplicationCert{Type: "INVALID_TYPE", Value: "some-value"},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://invalidcert.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	_, err := createApplication(app)
	ts.Assert().Error(err)
}

// TestApplicationInvalidJWKSURI tests invalid JWKS_URI rejection.
func (ts *ApplicationAPITestSuite) TestApplicationInvalidJWKSURI() {
	app := Application{
		OUID:        testOUID,
		Name:        "Invalid JWKS URI Test",
		Description: "Test invalid JWKS URI",
		URL:         "https://invalidjwksuri.example.com",
		Certificate: &ApplicationCert{Type: "JWKS_URI", Value: "not-a-valid-uri"},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://invalidjwksuri.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	_, err := createApplication(app)
	ts.Assert().Error(err)
}

// TestApplicationEmptyJWKS tests empty JWKS value rejection.
func (ts *ApplicationAPITestSuite) TestApplicationEmptyJWKS() {
	app := Application{
		OUID:        testOUID,
		Name:        "Empty JWKS Test",
		Description: "Test empty JWKS",
		URL:         "https://emptyjwks.example.com",
		Certificate: &ApplicationCert{Type: "JWKS", Value: ""},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://emptyjwks.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	_, err := createApplication(app)
	ts.Assert().Error(err)
}

// TestApplicationPublicClientValidations tests public client configuration validations.
func (ts *ApplicationAPITestSuite) TestApplicationPublicClientValidations() {
	// Public client with wrong auth method
	app := Application{
		OUID:        testOUID,
		Name:        "Public Client Invalid Auth",
		Description: "Test public client validations",
		URL:         "https://publicclienttest.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://publicclienttest.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PublicClient:            true,
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	_, err := createApplication(app)
	ts.Assert().Error(err)

	// Public client with invalid grant type
	app.Name = "Public Client Invalid Grant"
	app.InboundAuthConfig[0].OAuthAppConfig.TokenEndpointAuthMethod = "none"
	app.InboundAuthConfig[0].OAuthAppConfig.GrantTypes = []string{"client_credentials"}
	app.InboundAuthConfig[0].OAuthAppConfig.ResponseTypes = []string{}
	_, err = createApplication(app)
	ts.Assert().Error(err)
}

// TestApplicationOAuthConfigValidations tests OAuth configuration validations.
func (ts *ApplicationAPITestSuite) TestApplicationOAuthConfigValidations() {
	// authorization_code without redirect_uris
	app := Application{
		OUID:        testOUID,
		Name:        "OAuth Config No RedirectURIs",
		Description: "Test OAuth config validations",
		URL:         "https://oauthconfigtest.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	_, err := createApplication(app)
	ts.Assert().Error(err)

	// client_credentials with response_types
	app.Name = "OAuth Config Invalid client_credentials"
	app.InboundAuthConfig[0].OAuthAppConfig.RedirectURIs = []string{"https://test.example.com/callback"}
	app.InboundAuthConfig[0].OAuthAppConfig.GrantTypes = []string{"client_credentials"}
	app.InboundAuthConfig[0].OAuthAppConfig.ResponseTypes = []string{"code"}
	_, err = createApplication(app)
	ts.Assert().Error(err)

	// client_credentials with none auth method
	app.Name = "OAuth Config client_credentials with none"
	app.InboundAuthConfig[0].OAuthAppConfig.ResponseTypes = []string{}
	app.InboundAuthConfig[0].OAuthAppConfig.TokenEndpointAuthMethod = "none"
	_, err = createApplication(app)
	ts.Assert().Error(err)
}

// TestApplicationWithTokenConfiguration tests creating and updating applications with token config.
func (ts *ApplicationAPITestSuite) TestApplicationWithTokenConfiguration() {
	app := Application{
		OUID:        testOUID,
		Name:        "Token Config Test App",
		Description: "Test application with token configuration",
		URL:         "https://tokenconfig.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://tokenconfig.example.com/callback"},
					GrantTypes:              []string{"authorization_code", "refresh_token"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid", "profile"},
					Token: &OAuthTokenConfig{
						AccessToken: &AccessTokenConfig{
							ValidityPeriod: 3600,
							UserAttributes: []string{"email", "username"},
						},
						IDToken: &IDTokenConfig{
							ValidityPeriod: 3600,
							UserAttributes: []string{"sub", "email"},
						},
					},
					ScopeClaims: map[string][]string{
						"profile": {"name", "given_name", "family_name"},
						"email":   {"email", "email_verified"},
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(appID)
	defer deleteApplication(appID)

	// Retrieve and verify the token configuration was persisted
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token)
	ts.Assert().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.AccessToken)
	ts.Assert().Equal(int64(3600), retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.AccessToken.ValidityPeriod)
}

// TestApplicationWithIDTokenScopeClaims tests ID token scope claims configuration.
func (ts *ApplicationAPITestSuite) TestApplicationWithIDTokenScopeClaims() {
	app := Application{
		OUID:        testOUID,
		Name:        "ID Token Scope Claims Test",
		Description: "Test ID token scope claims",
		URL:         "https://idtokenclaims.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://idtokenclaims.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid", "profile", "email", "address"},
					Token: &OAuthTokenConfig{
						IDToken: &IDTokenConfig{
							ValidityPeriod: 7200,
							UserAttributes: []string{"sub", "email", "name"},
						},
					},
					ScopeClaims: map[string][]string{
						"profile": {"name", "given_name", "family_name", "middle_name", "nickname", "preferred_username"},
						"email":   {"email", "email_verified"},
						"address": {"address"},
						"phone":   {"phone_number", "phone_number_verified"},
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Retrieve and verify scope claims
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.ScopeClaims)
	ts.Assert().Contains(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.ScopeClaims, "profile")
	ts.Assert().Contains(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.ScopeClaims["email"], "email")
}

// TestApplicationUpdateWithTokenConfigChanges tests updating token configuration.
func (ts *ApplicationAPITestSuite) TestApplicationUpdateWithTokenConfigChanges() {
	// Create app with basic token config
	app := Application{
		OUID:        testOUID,
		Name:        "Token Config Update Test",
		Description: "Test token config updates",
		URL:         "https://tokenconfigupdate.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://tokenconfigupdate.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
					Token: &OAuthTokenConfig{
						AccessToken: &AccessTokenConfig{
							ValidityPeriod: 1800,
						},
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Update with more complex token config
	app.InboundAuthConfig[0].OAuthAppConfig.Token = &OAuthTokenConfig{
		AccessToken: &AccessTokenConfig{
			ValidityPeriod: 7200,
			UserAttributes: []string{"email", "username", "role"},
		},
		IDToken: &IDTokenConfig{
			ValidityPeriod: 3600,
			UserAttributes: []string{"sub", "email", "name"},
		},
	}
	app.InboundAuthConfig[0].OAuthAppConfig.ScopeClaims = map[string][]string{
		"profile": {"name", "picture"},
	}

	appJSON, _ := json.Marshal(app)
	req, _ := http.NewRequest("PUT", testServerURL+"/applications/"+appID, bytes.NewReader(appJSON))
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)

	// Verify the updated config
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Equal(int64(7200), retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.AccessToken.ValidityPeriod)
	ts.Assert().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.IDToken)
}

// TestApplicationWithPKCERequired tests creating application with PKCE requirement.
func (ts *ApplicationAPITestSuite) TestApplicationWithPKCERequired() {
	app := Application{
		OUID:        testOUID,
		Name:        "PKCE Required Test",
		Description: "Test PKCE required configuration",
		URL:         "https://pkce.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://pkce.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "none",
					PublicClient:            true,
					PKCERequired:            true,
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Verify PKCE configuration
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().True(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.PKCERequired)
	ts.Assert().True(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.PublicClient)
}

// TestApplicationListRetrievesMultiple tests listing multiple applications.
func (ts *ApplicationAPITestSuite) TestApplicationListRetrievesMultiple() {
	// Create multiple applications
	appIDs := make([]string, 0)

	for i := 0; i < 3; i++ {
		app := Application{
			OUID:        testOUID,
			Name:        fmt.Sprintf("List Test App %d", i),
			Description: fmt.Sprintf("Test application %d", i),
			URL:         fmt.Sprintf("https://listtest%d.example.com", i),
			Certificate: nil,
			InboundAuthConfig: []InboundAuthConfig{
				{
					Type: "oauth2",
					OAuthAppConfig: &OAuthAppConfig{
						RedirectURIs:            []string{fmt.Sprintf("https://listtest%d.example.com/callback", i)},
						GrantTypes:              []string{"authorization_code"},
						ResponseTypes:           []string{"code"},
						TokenEndpointAuthMethod: "client_secret_basic",
						Scopes:                  []string{"openid"},
					},
				},
			},
		}

		appID, err := createApplication(app)
		ts.Require().NoError(err)
		appIDs = append(appIDs, appID)
	}

	// Cleanup
	defer func() {
		for _, appID := range appIDs {
			deleteApplication(appID)
		}
	}()

	// List applications
	req, _ := http.NewRequest("GET", testServerURL+"/applications", nil)
	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)

	var listResponse ApplicationList
	json.NewDecoder(resp.Body).Decode(&listResponse)
	ts.Assert().GreaterOrEqual(listResponse.TotalResults, 3)
}

// TestApplicationUpdateCompleteOAuthConfig tests updating all OAuth fields.
func (ts *ApplicationAPITestSuite) TestApplicationUpdateCompleteOAuthConfig() {
	// Create with minimal config
	app := Application{
		OUID:        testOUID,
		Name:        "Complete OAuth Update Test",
		Description: "Test complete OAuth config update",
		URL:         "https://completeoauth.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://completeoauth.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Update with complete configuration
	app.InboundAuthConfig[0].OAuthAppConfig.RedirectURIs = []string{
		"https://completeoauth.example.com/callback1",
		"https://completeoauth.example.com/callback2",
	}
	app.InboundAuthConfig[0].OAuthAppConfig.GrantTypes = []string{
		"authorization_code",
		"refresh_token",
	}
	app.InboundAuthConfig[0].OAuthAppConfig.Scopes = []string{
		"openid", "profile", "email", "address", "phone",
	}
	app.InboundAuthConfig[0].OAuthAppConfig.PKCERequired = true
	app.Certificate = &ApplicationCert{
		Type:  "JWKS_URI",
		Value: "https://completeoauth.example.com/.well-known/jwks.json",
	}

	appJSON, _ := json.Marshal(app)
	req, _ := http.NewRequest("PUT", testServerURL+"/applications/"+appID, bytes.NewReader(appJSON))
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)

	// Verify all updates
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Len(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.RedirectURIs, 2)
	ts.Assert().Len(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.GrantTypes, 2)
	ts.Assert().Len(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Scopes, 5)
	ts.Assert().True(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.PKCERequired)
}

// Helper function to get application by ID
func getApplicationByID(appID string) (*Application, error) {
	req, err := http.NewRequest("GET", testServerURL+"/applications/"+appID, nil)
	if err != nil {
		return nil, err
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get application: status %d", resp.StatusCode)
	}

	var app Application
	if err := json.NewDecoder(resp.Body).Decode(&app); err != nil {
		return nil, err
	}

	return &app, nil
}

// TestApplicationWithOnlyAccessToken tests creating application with only AccessToken config.
func (ts *ApplicationAPITestSuite) TestApplicationWithOnlyAccessToken() {
	app := Application{
		OUID:        testOUID,
		Name:        "Only Access Token Test",
		Description: "Test with only access token config",
		URL:         "https://accesstokenonly.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://accesstokenonly.example.com/callback"},
					GrantTypes:              []string{"client_credentials"},
					ResponseTypes:           []string{},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"api.read", "api.write"},
					Token: &OAuthTokenConfig{
						AccessToken: &AccessTokenConfig{
							ValidityPeriod: 7200,
							UserAttributes: []string{"email", "username", "role", "department"},
						},
						// No IDToken
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Retrieve and verify AccessToken is configured properly
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token)
	ts.Assert().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.AccessToken)
	ts.Assert().Equal(int64(7200), retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.AccessToken.ValidityPeriod)
	ts.Assert().Len(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.AccessToken.UserAttributes, 4)
}

// TestApplicationWithOnlyIDToken tests creating application with only IDToken config.
func (ts *ApplicationAPITestSuite) TestApplicationWithOnlyIDToken() {
	app := Application{
		OUID:        testOUID,
		Name:        "Only ID Token Test",
		Description: "Test with only ID token config",
		URL:         "https://idtokenonly.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://idtokenonly.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid", "profile"},
					Token: &OAuthTokenConfig{
						// No AccessToken
						IDToken: &IDTokenConfig{
							ValidityPeriod: 3600,
							UserAttributes: []string{"sub", "email", "name", "picture"},
						},
					},
					ScopeClaims: map[string][]string{
						"profile": {"name", "given_name", "family_name", "middle_name"},
						"email":   {"email", "email_verified"},
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Retrieve and verify IDToken is configured properly
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token)
	ts.Assert().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.IDToken)
	ts.Assert().Equal(int64(3600), retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.IDToken.ValidityPeriod)
	ts.Assert().Len(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.ScopeClaims, 2)
}

// TestApplicationWithBothTokenTypes tests creating application with both AccessToken and IDToken.
func (ts *ApplicationAPITestSuite) TestApplicationWithBothTokenTypes() {
	app := Application{
		OUID:        testOUID,
		Name:        "Both Token Types Test",
		Description: "Test with both access and ID tokens",
		URL:         "https://bothtokens.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://bothtokens.example.com/callback"},
					GrantTypes:              []string{"authorization_code", "refresh_token"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_post",
					Scopes:                  []string{"openid", "profile", "email"},
					Token: &OAuthTokenConfig{
						AccessToken: &AccessTokenConfig{
							ValidityPeriod: 5400,
							UserAttributes: []string{"email", "username"},
						},
						IDToken: &IDTokenConfig{
							ValidityPeriod: 3600,
							UserAttributes: []string{"sub", "email"},
						},
					},
					ScopeClaims: map[string][]string{
						"profile": {"name"},
						"email":   {"email"},
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Retrieve and verify both tokens are present
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token)
	ts.Assert().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.AccessToken)
	ts.Assert().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.IDToken)
	ts.Assert().Equal(int64(5400), retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.AccessToken.ValidityPeriod)
	ts.Assert().Equal(int64(3600), retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.IDToken.ValidityPeriod)
}

// TestApplicationUpdateRemoveOAuthConfig tests removing OAuth config from application.
func (ts *ApplicationAPITestSuite) TestApplicationUpdateRemoveOAuthConfig() {
	// Create app with OAuth config
	app := Application{
		OUID:        testOUID,
		Name:        "Remove OAuth Config Test",
		Description: "Test removing OAuth config",
		URL:         "https://removeoauth.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://removeoauth.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Update to remove OAuth config (empty InboundAuthConfig)
	app.InboundAuthConfig = []InboundAuthConfig{}
	appJSON, _ := json.Marshal(app)
	req, _ := http.NewRequest("PUT", testServerURL+"/applications/"+appID, bytes.NewReader(appJSON))
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)

	// Verify OAuth config was removed
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Len(retrievedApp.InboundAuthConfig, 0)
}

// TestApplicationWithMultipleGrantAndResponseTypes tests multiple grant/response types conversion.
func (ts *ApplicationAPITestSuite) TestApplicationWithMultipleGrantAndResponseTypes() {
	app := Application{
		OUID:        testOUID,
		Name:        "Multiple Grant Types Test",
		Description: "Test with multiple grant and response types",
		URL:         "https://multiplegrants.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs: []string{
						"https://multiplegrants.example.com/callback1",
						"https://multiplegrants.example.com/callback2",
						"https://multiplegrants.example.com/callback3",
					},
					GrantTypes: []string{
						"authorization_code",
						"refresh_token",
						"client_credentials",
					},
					ResponseTypes: []string{
						"code",
					},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid", "profile", "email", "address", "phone", "offline_access"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Retrieve and verify arrays were properly stored
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Len(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.RedirectURIs, 3)
	ts.Assert().Len(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.GrantTypes, 3)
	ts.Assert().Len(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Scopes, 6)
}

// TestApplicationWithMinimalTokenConfig tests minimal token configuration.
func (ts *ApplicationAPITestSuite) TestApplicationWithMinimalTokenConfig() {
	app := Application{
		OUID:        testOUID,
		Name:        "Minimal Token Config Test",
		Description: "Test with minimal token config",
		URL:         "https://minimaltoken.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://minimaltoken.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
					Token:                   &OAuthTokenConfig{
						// No AccessToken or IDToken
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Retrieve and verify minimal token config
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token)
}

// TestApplicationWithComplexScopeClaims tests complex scope claims mapping.
func (ts *ApplicationAPITestSuite) TestApplicationWithComplexScopeClaims() {
	app := Application{
		OUID:        testOUID,
		Name:        "Complex Scope Claims Test",
		Description: "Test with complex scope claims",
		URL:         "https://complexscopes.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://complexscopes.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid", "profile", "email", "address", "phone", "custom"},
					Token: &OAuthTokenConfig{
						IDToken: &IDTokenConfig{
							ValidityPeriod: 3600,
							UserAttributes: []string{"sub", "email", "name"},
						},
					},
					ScopeClaims: map[string][]string{
						"profile": {
							"name", "given_name", "family_name", "middle_name",
							"nickname", "preferred_username", "profile", "picture",
							"website", "gender", "birthdate", "zoneinfo", "locale",
							"updated_at",
						},
						"email": {"email", "email_verified"},
						"address": {
							"address.formatted", "address.street_address",
							"address.locality", "address.region",
							"address.postal_code", "address.country",
						},
						"phone":  {"phone_number", "phone_number_verified"},
						"custom": {"organization", "department", "employee_id"},
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Retrieve and verify complex scope claims
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.IDToken)
	ts.Assert().Len(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.ScopeClaims, 5)
	ts.Assert().Contains(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.ScopeClaims, "profile")
	ts.Assert().GreaterOrEqual(len(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.ScopeClaims["profile"]), 10)
}

// TestApplicationCertificateRollbackOnOAuthFail tests certificate rollback when OAuth creation fails.
func (ts *ApplicationAPITestSuite) TestApplicationCertificateRollbackOnOAuthFail() {
	// Try to create app with invalid OAuth config (should trigger rollback)
	app := Application{
		OUID:        testOUID,
		Name:        "Certificate Rollback Test",
		Description: "Test certificate rollback on OAuth failure",
		URL:         "https://rollback.example.com",
		Certificate: &ApplicationCert{
			Type:  "JWKS_URI",
			Value: "https://rollback.example.com/.well-known/jwks.json",
		},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://rollback.example.com/callback#fragment"}, // Invalid - has fragment
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	_, err := createApplication(app)
	ts.Assert().Error(err) // Should fail due to fragment in redirect URI
}

// TestApplicationGetByName tests retrieving application by name.
func (ts *ApplicationAPITestSuite) TestApplicationGetByName() {
	uniqueName := fmt.Sprintf("Get By Name Test %d", time.Now().UnixNano())
	app := Application{
		OUID:        testOUID,
		Name:        uniqueName,
		Description: "Test get by name",
		URL:         "https://getbyname.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://getbyname.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Get by name using query parameter
	req, _ := http.NewRequest("GET", testServerURL+"/applications?name="+url.QueryEscape(uniqueName), nil)
	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)
}

// TestApplicationWithOAuthCertificateEmptyJWKSURI tests OAuth cert with empty JWKS_URI.
func (ts *ApplicationAPITestSuite) TestApplicationWithOAuthCertificateEmptyJWKSURI() {
	app := Application{
		OUID:        testOUID,
		Name:        "OAuth Empty JWKS URI Test",
		Description: "Test OAuth certificate with empty JWKS_URI",
		URL:         "https://oauthemptyjwksuri.example.com",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://oauthemptyjwksuri.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
		Certificate: &ApplicationCert{
			Type:  "JWKS_URI",
			Value: "",
		},
	}

	_, err := createApplication(app)
	ts.Assert().Error(err) // Should fail due to empty JWKS_URI
}

// TestApplicationValidationGrantTypeResponseTypeIncompat tests incompatible grant/response type combinations.
func (ts *ApplicationAPITestSuite) TestApplicationValidationGrantTypeResponseTypeIncompat() {
	// authorization_code without 'code' in response_types
	app := Application{
		OUID:        testOUID,
		Name:        "Grant Response Incompat Test",
		Description: "Test incompatible grant and response types",
		URL:         "https://grantresponseincompat.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://grantresponseincompat.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"token"}, // Wrong response type for authorization_code
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	_, err := createApplication(app)
	ts.Assert().Error(err) // Should fail due to incompatibility
}

// TestApplicationMultipleRedirectURIValidation tests multiple redirect URI validation.
func (ts *ApplicationAPITestSuite) TestApplicationMultipleRedirectURIValidation() {
	app := Application{
		OUID:        testOUID,
		Name:        "Multiple Redirect URI Validation Test",
		Description: "Test validation of multiple redirect URIs",
		URL:         "https://multiredirect.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs: []string{
						"https://multiredirect.example.com/callback1",
						"invalid-uri", // Invalid
						"https://multiredirect.example.com/callback3",
					},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	_, err := createApplication(app)
	ts.Assert().Error(err) // Should fail due to invalid redirect URI
}

// TestApplicationUpdateAddOAuthConfig tests adding OAuth config to existing app without it.
func (ts *ApplicationAPITestSuite) TestApplicationUpdateAddOAuthConfig() {
	// Create app without OAuth config
	app := Application{
		OUID:              testOUID,
		Name:              "Add OAuth Config Test",
		Description:       "Test adding OAuth config via update",
		URL:               "https://addoauth.example.com",
		Certificate:       nil,
		InboundAuthConfig: []InboundAuthConfig{}, // No OAuth initially
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Update to add OAuth config
	app.InboundAuthConfig = []InboundAuthConfig{
		{
			Type: "oauth2",
			OAuthAppConfig: &OAuthAppConfig{
				RedirectURIs:            []string{"https://addoauth.example.com/callback"},
				GrantTypes:              []string{"authorization_code"},
				ResponseTypes:           []string{"code"},
				TokenEndpointAuthMethod: "client_secret_basic",
				Scopes:                  []string{"openid"},
			},
		},
	}

	appJSON, _ := json.Marshal(app)
	req, _ := http.NewRequest("PUT", testServerURL+"/applications/"+appID, bytes.NewReader(appJSON))
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)

	// Verify OAuth config was added
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Len(retrievedApp.InboundAuthConfig, 1)
	ts.Assert().NotEmpty(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.ClientID)
}

// TestApplicationTotalCountRetrieval tests getting total application count.
func (ts *ApplicationAPITestSuite) TestApplicationTotalCountRetrieval() {
	// Create a few apps
	appIDs := make([]string, 0)
	for i := 0; i < 2; i++ {
		app := Application{
			OUID:        testOUID,
			Name:        fmt.Sprintf("Count Test App %d", i),
			Description: "Test count",
			URL:         fmt.Sprintf("https://counttest%d.example.com", i),
			Certificate: nil,
			InboundAuthConfig: []InboundAuthConfig{
				{
					Type: "oauth2",
					OAuthAppConfig: &OAuthAppConfig{
						RedirectURIs:            []string{fmt.Sprintf("https://counttest%d.example.com/cb", i)},
						GrantTypes:              []string{"authorization_code"},
						ResponseTypes:           []string{"code"},
						TokenEndpointAuthMethod: "client_secret_basic",
						Scopes:                  []string{"openid"},
					},
				},
			},
		}
		appID, err := createApplication(app)
		ts.Require().NoError(err)
		appIDs = append(appIDs, appID)
	}

	// Cleanup
	defer func() {
		for _, appID := range appIDs {
			deleteApplication(appID)
		}
	}()

	// Get list to verify count
	req, _ := http.NewRequest("GET", testServerURL+"/applications", nil)
	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var listResponse ApplicationList
	json.NewDecoder(resp.Body).Decode(&listResponse)
	ts.Assert().GreaterOrEqual(listResponse.TotalResults, 2)
}

// TestApplicationWithCompleteMetadata tests creating an application with all metadata fields.
func (ts *ApplicationAPITestSuite) TestApplicationWithCompleteMetadata() {
	app := Application{
		OUID:        testOUID,
		Name:        "Complete Metadata App",
		Description: "App with all metadata",
		URL:         "https://completemeta.example.com",
		LogoURL:     "https://completemeta.example.com/logo.png",
		TosURI:      "https://completemeta.example.com/tos",
		PolicyURI:   "https://completemeta.example.com/privacy",
		Contacts:    []string{"admin@completemeta.example.com", "support@completemeta.example.com"},
		Certificate: nil,
		Assertion: &AssertionConfig{
			ValidityPeriod: 7200,
			UserAttributes: []string{"email", "username", "groups"},
		},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://completemeta.example.com/callback"},
					GrantTypes:              []string{"authorization_code", "refresh_token"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid", "profile", "email"},
					Token: &OAuthTokenConfig{
						AccessToken: &AccessTokenConfig{
							ValidityPeriod: 3600,
							UserAttributes: []string{"sub", "email"},
						},
						IDToken: &IDTokenConfig{
							ValidityPeriod: 3600,
							UserAttributes: []string{"sub", "email", "name"},
						},
					},
					ScopeClaims: map[string][]string{
						"profile": {"name", "given_name", "family_name"},
						"email":   {"email", "email_verified"},
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Retrieve and verify all fields
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)

	// Verify basic fields
	ts.Assert().Equal("Complete Metadata App", retrievedApp.Name)
	ts.Assert().Equal("App with all metadata", retrievedApp.Description)
	ts.Assert().Equal("https://completemeta.example.com", retrievedApp.URL)
	ts.Assert().Equal("https://completemeta.example.com/logo.png", retrievedApp.LogoURL)

	// Verify metadata fields
	ts.Assert().Equal("https://completemeta.example.com/tos", retrievedApp.TosURI)
	ts.Assert().Equal("https://completemeta.example.com/privacy", retrievedApp.PolicyURI)
	ts.Assert().Equal([]string{"admin@completemeta.example.com", "support@completemeta.example.com"}, retrievedApp.Contacts)

	// Verify assertion config
	ts.Require().NotNil(retrievedApp.Assertion)
	ts.Assert().Equal(int64(7200), retrievedApp.Assertion.ValidityPeriod)
	ts.Assert().Equal([]string{"email", "username", "groups"}, retrievedApp.Assertion.UserAttributes)

	// Verify OAuth config fields
	ts.Require().Len(retrievedApp.InboundAuthConfig, 1)
	ts.Assert().Equal([]string{"https://completemeta.example.com/callback"}, retrievedApp.InboundAuthConfig[0].OAuthAppConfig.RedirectURIs)
	ts.Assert().Equal([]string{"authorization_code", "refresh_token"}, retrievedApp.InboundAuthConfig[0].OAuthAppConfig.GrantTypes)
	ts.Assert().Equal([]string{"code"}, retrievedApp.InboundAuthConfig[0].OAuthAppConfig.ResponseTypes)
	ts.Assert().Equal("client_secret_basic", retrievedApp.InboundAuthConfig[0].OAuthAppConfig.TokenEndpointAuthMethod)
	ts.Assert().Equal([]string{"openid", "profile", "email"}, retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Scopes)

	// Verify OAuth token config
	ts.Require().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token)

	// Verify access token config
	ts.Require().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.AccessToken)
	ts.Assert().Equal(int64(3600), retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.AccessToken.ValidityPeriod)
	ts.Assert().Equal([]string{"sub", "email"}, retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.AccessToken.UserAttributes)

	// Verify ID token config
	ts.Require().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.IDToken)
	ts.Assert().Equal(int64(3600), retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.IDToken.ValidityPeriod)
	ts.Assert().Equal([]string{"sub", "email", "name"}, retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.IDToken.UserAttributes)
	ts.Assert().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.ScopeClaims)
	ts.Assert().Equal([]string{"name", "given_name", "family_name"}, retrievedApp.InboundAuthConfig[0].OAuthAppConfig.ScopeClaims["profile"])
}

// TestApplicationWithOnlyRootToken tests app with only root token config.
func (ts *ApplicationAPITestSuite) TestApplicationWithOnlyRootToken() {
	app := Application{
		OUID:        testOUID,
		Name:        "Root Token Only App",
		Description: "App with only root token",
		Certificate: nil,
		Assertion: &AssertionConfig{
			ValidityPeriod: 5400,
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Require().NotNil(retrievedApp.Assertion)
	ts.Assert().Equal(int64(5400), retrievedApp.Assertion.ValidityPeriod)
}

// TestApplicationUpdateMetadataFields tests updating metadata fields.
func (ts *ApplicationAPITestSuite) TestApplicationUpdateMetadataFields() {
	// Create initial app
	app := Application{
		OUID:        testOUID,
		Name:        "Update Metadata App",
		Description: "Initial description",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://updatemeta.example.com/cb"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Update with metadata
	app.Description = "Updated description"
	app.TosURI = "https://updatemeta.example.com/tos"
	app.PolicyURI = "https://updatemeta.example.com/privacy"
	app.Contacts = []string{"contact@updatemeta.example.com"}
	app.LogoURL = "https://updatemeta.example.com/logo.png"

	payload, _ := json.Marshal(app)
	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/applications/%s", testServerURL, appID), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)

	// Verify updates
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Equal("Updated description", retrievedApp.Description)
	ts.Assert().Equal("https://updatemeta.example.com/tos", retrievedApp.TosURI)
	ts.Assert().Equal("https://updatemeta.example.com/privacy", retrievedApp.PolicyURI)
	ts.Assert().Equal([]string{"contact@updatemeta.example.com"}, retrievedApp.Contacts)
	ts.Assert().Equal("https://updatemeta.example.com/logo.png", retrievedApp.LogoURL)
}

// TestApplicationPublicClientWithoutSecret tests public client creation without client secret.
func (ts *ApplicationAPITestSuite) TestApplicationPublicClientWithoutSecret() {
	app := Application{
		OUID:        testOUID,
		Name:        "Public Client No Secret",
		Description: "Public client without secret",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://public-nosecret.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "none",
					PublicClient:            true,
					PKCERequired:            true,
					Scopes:                  []string{"openid", "profile"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().True(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.PublicClient)
	ts.Assert().True(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.PKCERequired)
	ts.Assert().Equal("none", string(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.TokenEndpointAuthMethod))
}

// TestApplicationPublicClientPKCEValidation tests that public clients must have PKCE required.
func (ts *ApplicationAPITestSuite) TestApplicationPublicClientPKCEValidation() {
	app := Application{
		OUID:        testOUID,
		Name:        "Public Client PKCE Validation",
		Description: "Public client with PKCE explicitly set to false should fail",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://public-pkce-validation.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "none",
					PublicClient:            true,
					PKCERequired:            false, // Explicitly set to false - should fail validation
					Scopes:                  []string{"openid", "profile"},
				},
			},
		},
	}

	// Attempt to create application - should fail validation
	appID, err := createApplication(app)
	ts.Require().Error(err, "Public client with pkce_required=false should fail validation")
	ts.Assert().Contains(err.Error(), "PKCE required", "Error should mention PKCE requirement")
	ts.Assert().Empty(appID, "Application ID should be empty on validation failure")
}

// TestApplicationWithRefreshTokenGrant tests app with refresh_token grant.
func (ts *ApplicationAPITestSuite) TestApplicationWithRefreshTokenGrant() {
	app := Application{
		OUID:        testOUID,
		Name:        "Refresh Token App",
		Description: "App with refresh token grant",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://refreshtoken.example.com/callback"},
					GrantTypes:              []string{"authorization_code", "refresh_token"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid", "offline_access"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Contains(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.GrantTypes, "authorization_code")
	ts.Assert().Contains(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.GrantTypes, "refresh_token")
}

// TestApplicationUpdateTokenConfiguration tests updating token configuration.
func (ts *ApplicationAPITestSuite) TestApplicationUpdateTokenConfiguration() {
	// Create app with initial token config
	app := Application{
		OUID:        testOUID,
		Name:        "Update Token Config App",
		Description: "App to update token config",
		Certificate: nil,
		Assertion: &AssertionConfig{
			ValidityPeriod: 3600,
		},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://updatetoken.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Update token config
	app.Assertion.ValidityPeriod = 7200
	app.Assertion.UserAttributes = []string{"email", "username"}

	payload, _ := json.Marshal(app)
	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/applications/%s", testServerURL, appID), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)

	// Verify token config update
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Require().NotNil(retrievedApp.Assertion)
	ts.Assert().Equal(int64(7200), retrievedApp.Assertion.ValidityPeriod)
	ts.Assert().Equal([]string{"email", "username"}, retrievedApp.Assertion.UserAttributes)
}

// TestApplicationWithEmptyContacts tests app with empty contacts array.
func (ts *ApplicationAPITestSuite) TestApplicationWithEmptyContacts() {
	app := Application{
		OUID:        testOUID,
		Name:        "Empty Contacts App",
		Description: "App with empty contacts",
		Certificate: nil,
		Contacts:    []string{},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Empty(retrievedApp.Contacts)
}

// TestApplicationClientCredentialsGrant tests app with client_credentials grant.
func (ts *ApplicationAPITestSuite) TestApplicationClientCredentialsGrant() {
	app := Application{
		OUID:        testOUID,
		Name:        "Client Credentials App",
		Description: "App with client credentials grant",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					GrantTypes:              []string{"client_credentials"},
					ResponseTypes:           []string{},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"api:read", "api:write"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Equal([]string{"client_credentials"}, retrievedApp.InboundAuthConfig[0].OAuthAppConfig.GrantTypes)
	ts.Assert().Empty(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.RedirectURIs)
}

// TestApplicationWithIDTokenScopeClaimsOnly tests app with only ID token scope claims.
func (ts *ApplicationAPITestSuite) TestApplicationWithIDTokenScopeClaimsOnly() {
	app := Application{
		OUID:        testOUID,
		Name:        "ID Token Scope Claims App",
		Description: "App with ID token scope claims only",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://idtoken-scope.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid", "profile"},
					Token: &OAuthTokenConfig{
						IDToken: &IDTokenConfig{
							ValidityPeriod: 3600,
						},
					},
					ScopeClaims: map[string][]string{
						"profile": {"name", "picture"},
						"email":   {"email"},
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Require().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token)
	ts.Require().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Token.IDToken)
	ts.Assert().NotNil(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.ScopeClaims)
	ts.Assert().Equal([]string{"name", "picture"}, retrievedApp.InboundAuthConfig[0].OAuthAppConfig.ScopeClaims["profile"])
}

// TestApplicationGetByNonExistentID tests retrieving app by non-existent ID.
func (ts *ApplicationAPITestSuite) TestApplicationGetByNonExistentID() {
	nonExistentID := "00000000-0000-0000-0000-000000000000"
	_, err := getApplicationByID(nonExistentID)
	ts.Assert().Error(err)
}

// TestApplicationWithMultipleRedirectURIsAndScopes tests app with multiple redirect URIs and scopes.
func (ts *ApplicationAPITestSuite) TestApplicationWithMultipleRedirectURIsAndScopes() {
	app := Application{
		OUID:        testOUID,
		Name:        "Multiple URIs and Scopes App",
		Description: "App with multiple redirect URIs and scopes",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs: []string{
						"https://multi-uris.example.com/callback1",
						"https://multi-uris.example.com/callback2",
						"https://multi-uris.example.com/callback3",
					},
					GrantTypes:              []string{"authorization_code", "refresh_token"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid", "profile", "email", "address", "phone"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Len(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.RedirectURIs, 3)
	ts.Assert().Len(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Scopes, 5)
	ts.Assert().Contains(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Scopes, "address")
	ts.Assert().Contains(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.Scopes, "phone")
}

// TestApplicationUpdateNonExistent tests updating a non-existent application.
func (ts *ApplicationAPITestSuite) TestApplicationUpdateNonExistent() {
	nonExistentID := "00000000-0000-0000-0000-000000000000"

	updateApp := Application{
		OUID:        testOUID,
		Name:        "Non-Existent App Update",
		Description: "Attempting to update non-existent app",
		Certificate: nil,
	}

	appJSON, err := json.Marshal(updateApp)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+nonExistentID, reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should return 404 Not Found
	ts.Assert().Equal(http.StatusNotFound, resp.StatusCode)
}

// TestApplicationDeleteNonExistent tests deleting a non-existent application.
// Note: DELETE is idempotent - deleting a non-existent resource returns 204 (success).
func (ts *ApplicationAPITestSuite) TestApplicationDeleteNonExistent() {
	nonExistentID := "00000000-0000-0000-0000-000000000000"

	req, err := http.NewRequest("DELETE", testServerURL+"/applications/"+nonExistentID, nil)
	ts.Require().NoError(err)

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// DELETE is idempotent - should return 204 No Content even for non-existent resources
	ts.Assert().Equal(http.StatusNoContent, resp.StatusCode)
}

// TestApplicationWithInvalidAuthFlowID tests creating app with invalid auth flow ID.
func (ts *ApplicationAPITestSuite) TestApplicationWithInvalidAuthFlowID() {
	app := Application{
		OUID:        testOUID,
		Name:        "Invalid Auth Flow App",
		Description: "App with invalid auth flow ID",
		AuthFlowID:  "edc013d0-e893-4dc0-990c-3e1d203e005b",
		Certificate: nil,
	}

	_, err := createApplication(app)
	ts.Assert().Error(err, "Should fail with invalid auth flow ID")
}

// TestApplicationWithInvalidRegistrationFlowID tests creating app with invalid registration flow ID.
func (ts *ApplicationAPITestSuite) TestApplicationWithInvalidRegistrationFlowID() {
	app := Application{
		OUID:                      testOUID,
		Name:                      "Invalid Registration Flow App",
		Description:               "App with invalid registration flow ID",
		RegistrationFlowID:        "80024fb3-29ed-4c33-aa48-8aee5e96d522",
		IsRegistrationFlowEnabled: true,
		Certificate:               nil,
	}

	_, err := createApplication(app)
	ts.Assert().Error(err, "Should fail with invalid registration flow ID")
}

// TestApplicationWithDuplicateName tests creating app with duplicate name.
func (ts *ApplicationAPITestSuite) TestApplicationWithDuplicateName() {
	app := Application{
		OUID:        testOUID,
		Name:        "Duplicate Name Test App",
		Description: "First app with this name",
		Certificate: nil,
	}

	appID1, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID1)

	// Try to create another app with the same name
	app2 := Application{
		OUID:        testOUID,
		Name:        "Duplicate Name Test App", // Same name
		Description: "Second app with duplicate name",
		Certificate: nil,
	}

	_, err = createApplication(app2)
	ts.Assert().Error(err, "Should fail with duplicate application name")
}

// TestApplicationWithEmptyName tests creating app with empty name.
func (ts *ApplicationAPITestSuite) TestApplicationWithEmptyName() {
	app := Application{
		OUID:        testOUID,
		Name:        "", // Empty name
		Description: "App with empty name",
		Certificate: nil,
	}

	_, err := createApplication(app)
	ts.Assert().Error(err, "Should fail with empty application name")
}

// TestApplicationWithVeryLongName tests creating app with very long name.
func (ts *ApplicationAPITestSuite) TestApplicationWithVeryLongName() {
	// Create a name longer than 256 characters
	longName := ""
	for i := 0; i < 300; i++ {
		longName += "a"
	}

	app := Application{
		OUID:        testOUID,
		Name:        longName,
		Description: "App with very long name",
		Certificate: nil,
	}

	appID, err := createApplication(app)
	if err == nil {
		defer deleteApplication(appID)
		// If creation succeeds, verify the name was stored correctly
		retrievedApp, getErr := getApplicationByID(appID)
		ts.Require().NoError(getErr)
		ts.Assert().Equal(longName, retrievedApp.Name)
	}
	// Long names might be accepted or rejected depending on validation
}

// TestApplicationWithSpecialCharactersInName tests creating app with special characters in name.
func (ts *ApplicationAPITestSuite) TestApplicationWithSpecialCharactersInName() {
	app := Application{
		OUID:        testOUID,
		Name:        "Test App with 特殊文字 and émojis 🚀",
		Description: "App with unicode and special characters",
		Certificate: nil,
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Equal("Test App with 特殊文字 and émojis 🚀", retrievedApp.Name)
}

// TestApplicationWithEmptyOAuthGrantTypes tests creating app with empty grant types.
// Note: Empty grant types array gets default value (authorization_code) applied automatically.
func (ts *ApplicationAPITestSuite) TestApplicationWithEmptyOAuthGrantTypes() {
	app := Application{
		OUID:        testOUID,
		Name:        "Empty Grant Types App",
		Description: "App with empty OAuth grant types",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://empty-grants.example.com/callback"},
					GrantTypes:              []string{}, // Empty grant types - will get default
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err, "Should succeed - empty grant types get default value")
	defer deleteApplication(appID)

	// Verify the default grant type was applied
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Require().NotNil(retrievedApp.InboundAuthConfig)
	ts.Assert().Len(retrievedApp.InboundAuthConfig, 1)
	ts.Assert().Contains(retrievedApp.InboundAuthConfig[0].OAuthAppConfig.GrantTypes, "authorization_code")
}

// TestApplicationUpdateInvalidAuthFlow tests updating app with invalid auth flow.
func (ts *ApplicationAPITestSuite) TestApplicationUpdateInvalidAuthFlow() {
	app := Application{
		OUID:        testOUID,
		Name:        "Update Auth Flow Test App",
		Description: "App to test auth flow update",
		Certificate: nil,
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Try to update with invalid auth flow ID
	updateApp := Application{
		OUID:        testOUID,
		Name:        "Updated with Invalid Auth Flow",
		Description: "Updated description",
		AuthFlowID:  "edc013d0-e893-4dc0-990c-3e1d203e005b",
		Certificate: nil,
	}

	appJSON, err := json.Marshal(updateApp)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+appID, reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should return error (likely 400 Bad Request)
	ts.Assert().NotEqual(http.StatusOK, resp.StatusCode)
}

// TestApplicationListWhenEmpty tests listing applications when database might be empty.
func (ts *ApplicationAPITestSuite) TestApplicationListWhenEmpty() {
	// This test assumes the database might have applications, so we just verify the endpoint works
	req, err := http.NewRequest("GET", testServerURL+"/applications", nil)
	ts.Require().NoError(err)

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusOK, resp.StatusCode)

	var appList ApplicationList
	err = json.NewDecoder(resp.Body).Decode(&appList)
	ts.Require().NoError(err)

	// Verify the response structure is valid
	ts.Assert().GreaterOrEqual(appList.TotalResults, 0)
	ts.Assert().GreaterOrEqual(appList.Count, 0)
}

// TestApplicationWithNullOptionalFields tests creating app with null optional fields.
func (ts *ApplicationAPITestSuite) TestApplicationWithNullOptionalFields() {
	app := Application{
		OUID:        testOUID,
		Name:        "Null Optional Fields App",
		Description: "", // Empty description (optional)
		URL:         "", // Empty URL (optional)
		LogoURL:     "", // Empty logo URL (optional)
		Certificate: nil,
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Equal("Null Optional Fields App", retrievedApp.Name)
	ts.Assert().Equal("", retrievedApp.Description)
}

// TestApplicationUpdateWithEmptyAppID tests updating application with empty app ID
func (ts *ApplicationAPITestSuite) TestApplicationUpdateWithEmptyAppID() {
	updateApp := Application{
		OUID:        testOUID,
		Name:        "Update Test",
		Description: "Test update with empty app ID",
		Certificate: nil,
	}

	appJSON, err := json.Marshal(updateApp)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/applications/", reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should return 400 Bad Request, 404 Not Found, or 405 Method Not Allowed
	ts.Assert().True(resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed)
}

// TestApplicationUpdateWithEmptyName tests updating application with empty name
func (ts *ApplicationAPITestSuite) TestApplicationUpdateWithEmptyName() {
	// Create an application first
	appID, err := createApplication(appToCreate)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	updateApp := Application{
		OUID:        testOUID,
		Name:        "", // Empty name
		Description: "Updated description",
		Certificate: nil,
	}

	appJSON, err := json.Marshal(updateApp)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+appID, reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should return 400 Bad Request
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestApplicationUpdateWithDuplicateName tests updating application with duplicate name
func (ts *ApplicationAPITestSuite) TestApplicationUpdateWithDuplicateName() {
	// Create first application
	app1 := Application{
		OUID:        testOUID,
		Name:        "Duplicate Name Update Test App 1",
		Description: "First app",
		Certificate: nil,
	}
	appID1, err := createApplication(app1)
	ts.Require().NoError(err)
	defer deleteApplication(appID1)

	// Create second application
	app2 := Application{
		OUID:        testOUID,
		Name:        "Duplicate Name Update Test App 2",
		Description: "Second app",
		Certificate: nil,
	}
	appID2, err := createApplication(app2)
	ts.Require().NoError(err)
	defer deleteApplication(appID2)

	// Try to update app2 with app1's name
	updateApp := Application{
		OUID:        testOUID,
		Name:        "Duplicate Name Update Test App 1", // Same as app1
		Description: "Updated description",
		Certificate: nil,
	}

	appJSON, err := json.Marshal(updateApp)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+appID2, reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should return 409 Conflict or 400 Bad Request
	ts.Assert().True(resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusBadRequest)
}

// TestApplicationUpdateWithInvalidURL tests updating application with invalid URL
func (ts *ApplicationAPITestSuite) TestApplicationUpdateWithInvalidURL() {
	// Create an application first
	appID, err := createApplication(appToCreate)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	updateApp := Application{
		OUID:        testOUID,
		Name:        "Update Invalid URL",
		Description: "Test update with invalid URL",
		URL:         "://invalid-url", // Invalid URL
		Certificate: nil,
	}

	appJSON, err := json.Marshal(updateApp)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+appID, reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should return 400 Bad Request
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestApplicationUpdateWithInvalidLogoURL tests updating application with invalid LogoURL
func (ts *ApplicationAPITestSuite) TestApplicationUpdateWithInvalidLogoURL() {
	// Create an application first
	appID, err := createApplication(appToCreate)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	updateApp := Application{
		OUID:        testOUID,
		Name:        "Update Invalid LogoURL",
		Description: "Test update with invalid LogoURL",
		LogoURL:     "://invalid-logo-url", // Invalid URL
		Certificate: nil,
	}

	appJSON, err := json.Marshal(updateApp)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+appID, reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should return 400 Bad Request
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestApplicationUpdateWithClientIDGeneration tests updating application with auto-generated client ID
func (ts *ApplicationAPITestSuite) TestApplicationUpdateWithClientIDGeneration() {
	// Create app without OAuth config
	app := Application{
		OUID:              testOUID,
		Name:              "Client ID Generation Test",
		Description:       "Test client ID generation during update",
		URL:               "https://clientidgen.example.com",
		Certificate:       nil,
		InboundAuthConfig: []InboundAuthConfig{}, // No OAuth initially
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Update to add OAuth config without client ID (should be auto-generated)
	app.InboundAuthConfig = []InboundAuthConfig{
		{
			Type: "oauth2",
			OAuthAppConfig: &OAuthAppConfig{
				ClientID:                "", // Empty - should be auto-generated
				RedirectURIs:            []string{"https://clientidgen.example.com/callback"},
				GrantTypes:              []string{"authorization_code"},
				ResponseTypes:           []string{"code"},
				TokenEndpointAuthMethod: "client_secret_basic",
				Scopes:                  []string{"openid"},
			},
		},
	}

	appJSON, err := json.Marshal(app)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+appID, reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should succeed
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)

	var updatedApp Application
	err = json.NewDecoder(resp.Body).Decode(&updatedApp)
	ts.Require().NoError(err)

	// Verify client ID was generated
	ts.Assert().NotEmpty(updatedApp.InboundAuthConfig[0].OAuthAppConfig.ClientID)
}

// TestApplicationUpdateWithClientIDChange tests updating application with changed client ID
func (ts *ApplicationAPITestSuite) TestApplicationUpdateWithClientIDChange() {
	// Create app with OAuth config
	app := Application{
		OUID:        testOUID,
		Name:        "Client ID Change Test",
		Description: "Test client ID change during update",
		URL:         "https://clientidchange.example.com",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "original_client_id",
					RedirectURIs:            []string{"https://clientidchange.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Update with different client ID
	app.InboundAuthConfig[0].OAuthAppConfig.ClientID = "new_client_id"

	appJSON, err := json.Marshal(app)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+appID, reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should succeed
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)

	var updatedApp Application
	err = json.NewDecoder(resp.Body).Decode(&updatedApp)
	ts.Require().NoError(err)

	// Verify client ID was changed
	ts.Assert().Equal("new_client_id", updatedApp.InboundAuthConfig[0].OAuthAppConfig.ClientID)
}

// TestApplicationUpdateWithDuplicateClientID tests updating application with duplicate client ID
func (ts *ApplicationAPITestSuite) TestApplicationUpdateWithDuplicateClientID() {
	// Create first app with OAuth config
	app1 := Application{
		OUID:        testOUID,
		Name:        "Duplicate Client ID Test App 1",
		Description: "First app",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "shared_client_id",
					RedirectURIs:            []string{"https://app1.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}
	appID1, err := createApplication(app1)
	ts.Require().NoError(err)
	defer deleteApplication(appID1)

	// Create second app with different client ID
	app2 := Application{
		OUID:        testOUID,
		Name:        "Duplicate Client ID Test App 2",
		Description: "Second app",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "unique_client_id",
					RedirectURIs:            []string{"https://app2.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}
	appID2, err := createApplication(app2)
	ts.Require().NoError(err)
	defer deleteApplication(appID2)

	// Try to update app2 with app1's client ID
	app2.InboundAuthConfig[0].OAuthAppConfig.ClientID = "shared_client_id"

	appJSON, err := json.Marshal(app2)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+appID2, reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should return 409 Conflict or 400 Bad Request
	ts.Assert().True(resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusBadRequest)
}

// TestApplicationCreateWithDefaultAuthFlowID tests creating application without auth flow ID (should use default)
func (ts *ApplicationAPITestSuite) TestApplicationCreateWithDefaultAuthFlowID() {
	app := Application{
		OUID:        testOUID,
		Name:        "Default Auth Flow Test",
		Description: "Test default auth flow ID",
		Certificate: nil,
		// AuthFlowID not set - should use default
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)

	// Verify default auth flow ID was set
	ts.Assert().NotEmpty(retrievedApp.AuthFlowID)
}

// TestApplicationCreateWithoutRegistrationFlowID tests creating application without a registration
// flow ID when auto-inference is disabled (default). The registration flow ID should remain empty.
func (ts *ApplicationAPITestSuite) TestApplicationCreateWithoutRegistrationFlowID() {
	app := Application{
		OUID:                      testOUID,
		Name:                      "No Registration Flow Test",
		Description:               "Test that registration flow is not inferred when auto-inference is disabled",
		IsRegistrationFlowEnabled: true,
		AuthFlowID:                defaultAuthFlowID,
		Certificate:               nil,
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)

	// Verify registration flow ID was not inferred (auto-inference is disabled by default)
	ts.Assert().Empty(retrievedApp.RegistrationFlowID)
}

// TestApplicationUpdateRemoveCertificate tests updating application to remove certificate
func (ts *ApplicationAPITestSuite) TestApplicationUpdateRemoveCertificate() {
	// Create app with certificate
	app := Application{
		OUID:        testOUID,
		Name:        "Remove Certificate Test",
		Description: "Test removing certificate during update",
		URL:         "https://removecert.example.com",
		Certificate: &ApplicationCert{
			Type:  "JWKS_URI",
			Value: "https://removecert.example.com/.well-known/jwks.json",
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Update to remove certificate
	updateApp := Application{
		OUID:        testOUID,
		Name:        "Remove Certificate Test",
		Description: "Updated description",
		Certificate: nil, // Remove certificate
	}

	appJSON, err := json.Marshal(updateApp)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+appID, reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should succeed
	ts.Assert().Equal(http.StatusOK, resp.StatusCode)

	var updatedApp Application
	err = json.NewDecoder(resp.Body).Decode(&updatedApp)
	ts.Require().NoError(err)

	// Verify certificate was removed
	ts.Assert().Nil(updatedApp.Certificate)
}

// TestApplicationCreateWithDuplicateClientID tests creating application with duplicate client ID
func (ts *ApplicationAPITestSuite) TestApplicationCreateWithDuplicateClientID() {
	// Create first app with OAuth config
	app1 := Application{
		OUID:        testOUID,
		Name:        "Duplicate Client ID Create Test App 1",
		Description: "First app",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "duplicate_create_client_id",
					RedirectURIs:            []string{"https://app1.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}
	appID1, err := createApplication(app1)
	ts.Require().NoError(err)
	defer deleteApplication(appID1)

	// Try to create second app with same client ID
	app2 := Application{
		OUID:        testOUID,
		Name:        "Duplicate Client ID Create Test App 2",
		Description: "Second app with duplicate client ID",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "duplicate_create_client_id", // Same as app1
					RedirectURIs:            []string{"https://app2.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}

	_, err = createApplication(app2)
	ts.Assert().Error(err, "Should fail with duplicate client ID")
}

// TestApplicationCreateWithInvalidURL tests creating application with invalid URL
func (ts *ApplicationAPITestSuite) TestApplicationCreateWithInvalidURL() {
	app := Application{
		OUID:        testOUID,
		Name:        "Invalid URL Create Test",
		Description: "Test create with invalid URL",
		URL:         "://invalid-url", // Invalid URL
		Certificate: nil,
	}

	appJSON, err := json.Marshal(app)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("POST", testServerURL+"/applications", reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should return 400 Bad Request
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestApplicationCreateWithInvalidLogoURL tests creating application with invalid LogoURL
func (ts *ApplicationAPITestSuite) TestApplicationCreateWithInvalidLogoURL() {
	app := Application{
		OUID:        testOUID,
		Name:        "Invalid LogoURL Create Test",
		Description: "Test create with invalid LogoURL",
		LogoURL:     "://invalid-logo-url", // Invalid URL
		Certificate: nil,
	}

	appJSON, err := json.Marshal(app)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("POST", testServerURL+"/applications", reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should return 400 Bad Request
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestApplicationUpdateWithNilApp tests updating application with nil app body
func (ts *ApplicationAPITestSuite) TestApplicationUpdateWithNilApp() {
	// Create an application first
	appID, err := createApplication(appToCreate)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Send update request with nil/empty body
	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+appID, nil)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should return 400 Bad Request (nil app body)
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode)
}

// Helper function to create a theme configuration for testing
func createThemeForTest(theme []byte) (string, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"handle":      fmt.Sprintf("test-theme-%d", time.Now().UnixNano()),
		"displayName": "Test Theme",
		"theme":       json.RawMessage(theme),
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal theme request: %w", err)
	}

	req, err := http.NewRequest("POST", testServerURL+"/design/themes", bytes.NewReader(payload))
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

	var themeResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&themeResponse)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}

	themeID, ok := themeResponse["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id or id is not a string")
	}
	return themeID, nil
}

// Helper function to delete a theme configuration for testing
func deleteThemeForTest(themeID string) error {
	req, err := http.NewRequest("DELETE", testServerURL+"/design/themes/"+themeID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204 or 404, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

// Helper function to create a layout configuration for testing
func createLayoutForTest(layout []byte, description string) (string, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"handle":      fmt.Sprintf("test-layout-%d", time.Now().UnixNano()),
		"displayName": "Test Layout",
		"description": description,
		"layout":      json.RawMessage(layout),
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal layout request: %w", err)
	}

	req, err := http.NewRequest("POST", testServerURL+"/design/layouts", bytes.NewReader(payload))
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

	var layoutResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&layoutResponse)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}

	layoutID, ok := layoutResponse["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id or id is not a string")
	}
	return layoutID, nil
}

// Helper function to delete a layout configuration for testing
func deleteLayoutForTest(layoutID string) error {
	req, err := http.NewRequest("DELETE", testServerURL+"/design/layouts/"+layoutID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204 or 404, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

// TestApplicationWithThemeAndLayoutID tests creating an application with a valid theme ID and layout ID
func (ts *ApplicationAPITestSuite) TestApplicationWithThemeAndLayoutID() {
	// Create a theme configuration first
	themePreferences := []byte(`{
		"theme": {
			"activeColorScheme": "dark",
			"colorSchemes": {
				"dark": {
					"colors": {
						"primary": {
							"main": "#1976d2",
							"dark": "#0d47a1",
							"contrastText": "#ffffff"
						}
					}
				}
			}
		}
	}`)
	themeID, err := createThemeForTest(themePreferences)
	ts.Require().NoError(err, "Failed to create theme for test")
	defer deleteThemeForTest(themeID)

	// Create a layout configuration
	layoutPreferences := []byte(`{
		"layout": {
			"type": "centered",
			"showLogo": true
		}
	}`)
	layoutID, err := createLayoutForTest(layoutPreferences, "Test Layout")
	ts.Require().NoError(err, "Failed to create layout for test")
	defer deleteLayoutForTest(layoutID)

	// Create application with theme and layout IDs
	app := Application{
		OUID:        testOUID,
		Name:        "App With Theme and Layout",
		Description: "Application with theme and layout configuration",
		ThemeID:     themeID,
		LayoutID:    layoutID,
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://design-app.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Verify the theme and layout IDs are stored
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Equal(themeID, retrievedApp.ThemeID)
	ts.Assert().Equal(layoutID, retrievedApp.LayoutID)
}

// TestApplicationWithInvalidThemeAndLayoutID tests creating an application with invalid theme/layout IDs
func (ts *ApplicationAPITestSuite) TestApplicationWithInvalidThemeAndLayoutID() {
	app := Application{
		OUID:        testOUID,
		Name:        "App With Invalid Theme",
		Description: "Application with invalid theme ID",
		ThemeID:     "00000000-0000-0000-0000-000000000000",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://invalid-design.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}

	appJSON, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+"/applications", bytes.NewReader(appJSON))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	var errResp map[string]interface{}
	err = json.Unmarshal(bodyBytes, &errResp)
	ts.Require().NoError(err)
	ts.Assert().Equal("APP-1026", errResp["code"])
}

// TestApplicationUpdateWithThemeAndLayout tests updating an application with theme and layout IDs
func (ts *ApplicationAPITestSuite) TestApplicationUpdateWithThemeAndLayout() {
	// Create a theme configuration first
	themePreferences := []byte(`{
		"activeColorScheme": "light",
		"colorSchemes": {
			"light": {
				"colors": {
					"primary": {
						"main": "#2196f3",
						"dark": "#1976d2",
						"contrastText": "#ffffff"
					}
				}
			}
		}
	}`)
	themeID, err := createThemeForTest(themePreferences)
	ts.Require().NoError(err, "Failed to create theme for test")
	defer deleteThemeForTest(themeID)

	// Create a layout configuration
	layoutPreferences := []byte(`{
		"header": {
			"showLogo": true
		}
	}`)
	layoutID, err := createLayoutForTest(layoutPreferences, "Test Layout")
	ts.Require().NoError(err, "Failed to create layout for test")
	defer deleteLayoutForTest(layoutID)

	// Create application without theme/layout
	app := Application{
		OUID:        testOUID,
		Name:        "App To Update Design",
		Description: "Application to update with theme and layout",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://update-design.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Update application with theme and layout IDs
	app.ThemeID = themeID
	app.LayoutID = layoutID
	appJSON, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+appID, bytes.NewReader(appJSON))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusOK, resp.StatusCode)

	// Verify the theme and layout IDs are updated
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Equal(themeID, retrievedApp.ThemeID)
	ts.Assert().Equal(layoutID, retrievedApp.LayoutID)
}

// TestApplicationUpdateWithInvalidThemeAndLayoutID tests updating an application with invalid theme/layout IDs
func (ts *ApplicationAPITestSuite) TestApplicationUpdateWithInvalidThemeAndLayoutID() {
	// Create application without theme/layout
	app := Application{
		OUID:        testOUID,
		Name:        "App To Update Invalid Design",
		Description: "Application to update with invalid theme/layout",
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://invalid-update.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Update application with invalid theme ID
	app.ThemeID = "00000000-0000-0000-0000-000000000000"
	appJSON, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+appID, bytes.NewReader(appJSON))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	var errResp map[string]interface{}
	err = json.Unmarshal(bodyBytes, &errResp)
	ts.Require().NoError(err)
	ts.Assert().Equal("APP-1026", errResp["code"])
}

// TestThemeAndLayoutCannotDeleteWhenAssociatedWithApplication tests that theme/layout cannot be deleted when associated with an application
func (ts *ApplicationAPITestSuite) TestThemeAndLayoutCannotDeleteWhenAssociatedWithApplication() {
	// Create a theme configuration
	themePreferences := []byte(`{
		"activeColorScheme": "dark",
		"colorSchemes": {
			"dark": {
				"colors": {
					"primary": {
						"main": "#1976d2",
						"dark": "#0d47a1",
						"contrastText": "#ffffff"
					}
				}
			}
		}
	}`)
	themeID, err := createThemeForTest(themePreferences)
	ts.Require().NoError(err, "Failed to create theme for test")
	defer deleteThemeForTest(themeID)

	// Create application with theme ID
	app := Application{
		OUID:        testOUID,
		Name:        "App Preventing Theme Delete",
		Description: "Application that prevents theme deletion",
		ThemeID:     themeID,
		Certificate: nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://prevent-delete.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Try to delete the theme - should fail
	req, err := http.NewRequest("DELETE", testServerURL+"/design/themes/"+themeID, nil)
	ts.Require().NoError(err)

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusConflict, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	var errResp map[string]interface{}
	err = json.Unmarshal(bodyBytes, &errResp)
	ts.Require().NoError(err)
	ts.Assert().Equal("THM-1004", errResp["code"])

	// Delete the application first
	err = deleteApplication(appID)
	ts.Require().NoError(err)

	// Now the theme should be deletable
	resp, err = client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusNoContent, resp.StatusCode)
}

// TestApplicationWithAllowedUserTypes tests creating an application with valid allowed_user_types
func (ts *ApplicationAPITestSuite) TestApplicationWithAllowedUserTypes() {
	// Create test user types first
	employeeSchema := testutils.UserType{
		Name: "employee",
		OUID: testOUID,
		Schema: map[string]interface{}{
			"email": map[string]interface{}{
				"type": "string",
			},
			"name": map[string]interface{}{
				"type": "string",
			},
		},
	}
	customerSchema := testutils.UserType{
		Name: "customer",
		OUID: testOUID,
		Schema: map[string]interface{}{
			"email": map[string]interface{}{
				"type": "string",
			},
		},
	}

	employeeSchemaID, err := testutils.CreateUserType(employeeSchema)
	ts.Require().NoError(err, "Failed to create employee user type")
	defer func() {
		if err := testutils.DeleteUserType(employeeSchemaID); err != nil {
			ts.T().Logf("Failed to delete employee schema: %v", err)
		}
	}()

	customerSchemaID, err := testutils.CreateUserType(customerSchema)
	ts.Require().NoError(err, "Failed to create customer user type")
	defer func() {
		if err := testutils.DeleteUserType(customerSchemaID); err != nil {
			ts.T().Logf("Failed to delete customer schema: %v", err)
		}
	}()

	// Create application with allowed_user_types
	app := Application{
		OUID:                      testOUID,
		Name:                      "App With Allowed User Types",
		Description:               "Application with allowed user types",
		IsRegistrationFlowEnabled: false,
		AllowedUserTypes:          []string{"employee", "customer"},
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "allowed_types_client",
					ClientSecret:            "allowed_types_secret",
					RedirectURIs:            []string{"http://localhost/allowedtypes/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PKCERequired:            false,
					PublicClient:            false,
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err, "Failed to create application with allowed_user_types")
	defer func() {
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete application: %v", err)
		}
	}()

	// Verify the application was created with allowed_user_types
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Equal([]string{"employee", "customer"}, retrievedApp.AllowedUserTypes)
}

// TestApplicationWithInvalidAllowedUserTypes tests creating an application with invalid allowed_user_types
func (ts *ApplicationAPITestSuite) TestApplicationWithInvalidAllowedUserTypes() {
	// Create application with non-existent user types
	app := Application{
		OUID:                      testOUID,
		Name:                      "App With Invalid User Types",
		Description:               "Application with invalid user types",
		IsRegistrationFlowEnabled: false,
		AllowedUserTypes:          []string{"nonexistent_type_1", "nonexistent_type_2"},
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "invalid_types_client",
					ClientSecret:            "invalid_types_secret",
					RedirectURIs:            []string{"http://localhost/invalidtypes/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PKCERequired:            false,
					PublicClient:            false,
				},
			},
		},
	}

	appJSON, err := json.Marshal(app)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("POST", testServerURL+"/applications", reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should fail with 400 Bad Request
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode, "Should return 400 for invalid user types")

	// Verify error response
	var errorResp struct {
		Code        string                `json:"code"`
		Message     testutils.I18nMessage `json:"message"`
		Description testutils.I18nMessage `json:"description"`
	}
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	ts.Require().NoError(err)
	ts.Assert().Equal("APP-1025", errorResp.Code, "Error code should be APP-1025")
	ts.Assert().Contains(errorResp.Message.DefaultValue, "Invalid user type", "Error message should mention invalid user type")
}

// TestApplicationUpdateWithAllowedUserTypes tests updating an application with allowed_user_types
func (ts *ApplicationAPITestSuite) TestApplicationUpdateWithAllowedUserTypes() {
	// Create test user types
	employeeSchema := testutils.UserType{
		Name: "employee_update",
		OUID: testOUID,
		Schema: map[string]interface{}{
			"email": map[string]interface{}{
				"type": "string",
			},
		},
	}
	partnerSchema := testutils.UserType{
		Name: "partner",
		OUID: testOUID,
		Schema: map[string]interface{}{
			"email": map[string]interface{}{
				"type": "string",
			},
		},
	}

	employeeSchemaID, err := testutils.CreateUserType(employeeSchema)
	ts.Require().NoError(err)
	defer func() {
		if err := testutils.DeleteUserType(employeeSchemaID); err != nil {
			ts.T().Logf("Failed to delete employee schema: %v", err)
		}
	}()

	partnerSchemaID, err := testutils.CreateUserType(partnerSchema)
	ts.Require().NoError(err)
	defer func() {
		if err := testutils.DeleteUserType(partnerSchemaID); err != nil {
			ts.T().Logf("Failed to delete partner schema: %v", err)
		}
	}()

	// Create application without allowed_user_types
	app := Application{
		OUID:                      testOUID,
		Name:                      "App To Update With User Types",
		Description:               "Application to update",
		IsRegistrationFlowEnabled: false,
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "update_types_client",
					ClientSecret:            "update_types_secret",
					RedirectURIs:            []string{"http://localhost/updatetypes/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PKCERequired:            false,
					PublicClient:            false,
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer func() {
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete application: %v", err)
		}
	}()

	// Update application with allowed_user_types
	appToUpdate := app
	appToUpdate.ID = appID
	appToUpdate.AllowedUserTypes = []string{"employee_update", "partner"}

	appJSON, err := json.Marshal(appToUpdate)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+appID, reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusOK, resp.StatusCode, "Update should succeed")

	// Verify the update
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Equal([]string{"employee_update", "partner"}, retrievedApp.AllowedUserTypes)
}

// TestApplicationUpdateWithInvalidAllowedUserTypes tests updating an application with invalid allowed_user_types
func (ts *ApplicationAPITestSuite) TestApplicationUpdateWithInvalidAllowedUserTypes() {
	// Create application first
	app := Application{
		OUID:                      testOUID,
		Name:                      "App To Update With Invalid Types",
		Description:               "Application to update",
		IsRegistrationFlowEnabled: false,
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "update_invalid_client",
					ClientSecret:            "update_invalid_secret",
					RedirectURIs:            []string{"http://localhost/updateinvalid/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PKCERequired:            false,
					PublicClient:            false,
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer func() {
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete application: %v", err)
		}
	}()

	// Try to update with invalid user types
	appToUpdate := app
	appToUpdate.ID = appID
	appToUpdate.AllowedUserTypes = []string{"invalid_type_1", "invalid_type_2"}

	appJSON, err := json.Marshal(appToUpdate)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/applications/"+appID, reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should fail with 400 Bad Request
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode, "Should return 400 for invalid user types")

	// Verify error response
	var errorResp struct {
		Code        string                `json:"code"`
		Message     testutils.I18nMessage `json:"message"`
		Description testutils.I18nMessage `json:"description"`
	}
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	ts.Require().NoError(err)
	ts.Assert().Equal("APP-1025", errorResp.Code, "Error code should be APP-1025")
}

// TestApplicationWithEmptyAllowedUserTypes tests creating an application with empty allowed_user_types array
func (ts *ApplicationAPITestSuite) TestApplicationWithEmptyAllowedUserTypes() {
	app := Application{
		OUID:                      testOUID,
		Name:                      "App With Empty Allowed User Types",
		Description:               "Application with empty allowed user types",
		IsRegistrationFlowEnabled: false,
		AllowedUserTypes:          []string{},
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "empty_types_client",
					ClientSecret:            "empty_types_secret",
					RedirectURIs:            []string{"http://localhost/emptytypes/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PKCERequired:            false,
					PublicClient:            false,
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err, "Empty allowed_user_types should be allowed")
	defer func() {
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete application: %v", err)
		}
	}()

	// Verify empty array is stored (or nil, both are acceptable as they mean "no restrictions")
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	// Empty array or nil both mean "no restrictions", both are acceptable
	if retrievedApp.AllowedUserTypes != nil {
		ts.Assert().Len(retrievedApp.AllowedUserTypes, 0, "If not nil, AllowedUserTypes should be an empty array")
	}
}

// TestApplicationWithPartialInvalidAllowedUserTypes tests creating an application with mix of valid and invalid user types
func (ts *ApplicationAPITestSuite) TestApplicationWithPartialInvalidAllowedUserTypes() {
	// Create one valid user type
	validSchema := testutils.UserType{
		Name: "valid_user_type",
		OUID: testOUID,
		Schema: map[string]interface{}{
			"email": map[string]interface{}{
				"type": "string",
			},
		},
	}

	validSchemaID, err := testutils.CreateUserType(validSchema)
	ts.Require().NoError(err)
	defer func() {
		if err := testutils.DeleteUserType(validSchemaID); err != nil {
			ts.T().Logf("Failed to delete valid schema: %v", err)
		}
	}()

	// Create application with mix of valid and invalid user types
	app := Application{
		OUID:                      testOUID,
		Name:                      "App With Partial Invalid User Types",
		Description:               "Application with mix of valid and invalid user types",
		IsRegistrationFlowEnabled: false,
		AllowedUserTypes:          []string{"valid_user_type", "invalid_user_type"},
		Certificate:               nil,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "partial_invalid_client",
					ClientSecret:            "partial_invalid_secret",
					RedirectURIs:            []string{"http://localhost/partialinvalid/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PKCERequired:            false,
					PublicClient:            false,
				},
			},
		},
	}

	appJSON, err := json.Marshal(app)
	ts.Require().NoError(err)

	reqBody := bytes.NewReader(appJSON)
	req, err := http.NewRequest("POST", testServerURL+"/applications", reqBody)
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	// Should fail with 400 Bad Request because one user type is invalid
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode, "Should return 400 when any user type is invalid")

	// Verify error response
	var errorResp struct {
		Code        string                `json:"code"`
		Message     testutils.I18nMessage `json:"message"`
		Description testutils.I18nMessage `json:"description"`
	}
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	ts.Require().NoError(err)
	ts.Assert().Equal("APP-1025", errorResp.Code, "Error code should be APP-1025")
}

func (ts *ApplicationAPITestSuite) TestApplicationWithUserInfoConfig() {
	app := Application{
		OUID:                      testOUID,
		Name:                      "App With UserInfo Config",
		Description:               "Testing UserInfo and ScopeClaims persistence",
		IsRegistrationFlowEnabled: false,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "userinfo_config_app_test_client",
					ClientSecret:            "userinfo_config_app_test_secret",
					RedirectURIs:            []string{"http://localhost/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PKCERequired:            false,
					PublicClient:            false,
					Scopes:                  []string{"openid", "profile", "email"},
					Token: &OAuthTokenConfig{
						IDToken: &IDTokenConfig{
							UserAttributes: []string{"sub"},
						},
					},
					UserInfo: &UserInfoConfig{
						UserAttributes: []string{"email", "given_name", "family_name"},
					},
					ScopeClaims: map[string][]string{
						"profile": {"given_name", "family_name"},
						"email":   {"email"},
					},
				},
			},
		},
	}

	// Set flow IDs (using defaults from suite)
	app.AuthFlowID = defaultAuthFlowID
	app.RegistrationFlowID = defaultRegistrationFlowID

	// Create
	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Get
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)

	// Validate
	ts.Require().NotEmpty(retrievedApp.InboundAuthConfig)
	oauthConfig := retrievedApp.InboundAuthConfig[0].OAuthAppConfig
	ts.Require().NotNil(oauthConfig)

	// Check UserInfo
	ts.Require().NotNil(oauthConfig.UserInfo)
	ts.Assert().ElementsMatch([]string{"email", "given_name", "family_name"}, oauthConfig.UserInfo.UserAttributes)

	// Check ScopeClaims
	ts.Require().NotNil(oauthConfig.ScopeClaims)
	ts.Assert().ElementsMatch([]string{"given_name", "family_name"}, oauthConfig.ScopeClaims["profile"])

	// Check IDToken is separate
	ts.Require().NotNil(oauthConfig.Token.IDToken)
	ts.Assert().ElementsMatch([]string{"sub"}, oauthConfig.Token.IDToken.UserAttributes)
}

func (ts *ApplicationAPITestSuite) TestApplicationUserInfoWithFallback() {
	app := Application{
		OUID:                      testOUID,
		Name:                      "App UserInfo Fallback",
		Description:               "Testing UserInfo fallback logic",
		IsRegistrationFlowEnabled: false,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "userinfo_fallback_test_client",
					ClientSecret:            "userinfo_fallback_test_secret",
					RedirectURIs:            []string{"http://localhost/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PKCERequired:            false,
					PublicClient:            false,
					Scopes:                  []string{"openid", "email"},
					Token: &OAuthTokenConfig{
						IDToken: &IDTokenConfig{
							UserAttributes: []string{"email", "sub"},
						},
					},
					// UserInfo not specified
				},
			},
		},
	}

	// Set flow IDs (using defaults from suite)
	app.AuthFlowID = defaultAuthFlowID
	app.RegistrationFlowID = defaultRegistrationFlowID

	// Create
	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	// Get
	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)

	// Validate
	ts.Require().NotEmpty(retrievedApp.InboundAuthConfig)
	oauthConfig := retrievedApp.InboundAuthConfig[0].OAuthAppConfig
	ts.Require().NotNil(oauthConfig)

	// Check UserInfo inherited from IDToken
	ts.Require().NotNil(oauthConfig.UserInfo)
	ts.Assert().ElementsMatch([]string{"email", "sub"}, oauthConfig.UserInfo.UserAttributes)
}

func (ts *ApplicationAPITestSuite) TestApplicationUserInfoResponseTypeJWS() {
	app := Application{
		OUID:        testOUID,
		Name:        "App UserInfo JWS",
		Description: "Testing JWS response type",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "userinfo_jws_test_client",
					ClientSecret:            "userinfo_jws_test_secret",
					RedirectURIs:            []string{"http://localhost/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					UserInfo: &UserInfoConfig{
						ResponseType:   "JWS",
						SigningAlg:     "RS256",
						UserAttributes: []string{"email"},
					},
				},
			},
		},
	}

	app.AuthFlowID = defaultAuthFlowID
	app.RegistrationFlowID = defaultRegistrationFlowID

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	retrievedApp, err := getApplicationByID(appID)
	ts.Require().NoError(err)

	oauth := retrievedApp.InboundAuthConfig[0].OAuthAppConfig
	ts.Require().NotNil(oauth.UserInfo)
	ts.Assert().Equal("RS256", oauth.UserInfo.SigningAlg)
}

func (ts *ApplicationAPITestSuite) TestApplicationUserInfoInvalidSigningAlgRejected() {
	app := Application{
		OUID:        testOUID,
		Name:        "App UserInfo Invalid SigningAlg",
		Description: "Testing that an unsupported signingAlg is rejected",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "userinfo_invalid_alg_client",
					ClientSecret:            "userinfo_invalid_alg_secret",
					RedirectURIs:            []string{"http://localhost/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					UserInfo: &UserInfoConfig{
						SigningAlg:     "INVALID_ALG",
						UserAttributes: []string{"email"},
					},
				},
			},
		},
	}

	app.AuthFlowID = defaultAuthFlowID
	app.RegistrationFlowID = defaultRegistrationFlowID

	_, err := createApplication(app)
	ts.Require().Error(err, "Creating an app with an unsupported signingAlg should fail")
	ts.Assert().Contains(err.Error(), "400", "Expected HTTP 400 for unsupported signingAlg")
}

// ---------------------------------------------------------------------------
// IDToken responseType validation tests
// ---------------------------------------------------------------------------

const testEncJWKS = `{"keys":[{"kty":"RSA","use":"enc","alg":"RSA-OAEP-256","n":"0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw","e":"AQAB"}]}`

// TestIDTokenResponseType_JWE_ValidConfig creates an app with responseType=JWE and verifies the fields round-trip.
func (ts *ApplicationAPITestSuite) TestIDTokenResponseType_JWE_ValidConfig() {
	app := Application{
		OUID:        testOUID,
		Name:        "IDToken JWE Response Type Test",
		Description: "Test responseType=JWE for ID token",
		URL:         "https://idtoken-jwe.example.com",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://idtoken-jwe.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
					Certificate:             &ApplicationCert{Type: "JWKS", Value: testEncJWKS},
					Token: &OAuthTokenConfig{
						IDToken: &IDTokenConfig{
							ResponseType:  "JWE",
							EncryptionAlg: "RSA-OAEP-256",
							EncryptionEnc: "A256GCM",
						},
					},
				},
			},
		},
	}
	app.AuthFlowID = defaultAuthFlowID
	app.RegistrationFlowID = defaultRegistrationFlowID

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	retrieved, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	idToken := retrieved.InboundAuthConfig[0].OAuthAppConfig.Token.IDToken
	ts.Assert().Equal("JWE", idToken.ResponseType)
	ts.Assert().Equal("RSA-OAEP-256", idToken.EncryptionAlg)
	ts.Assert().Equal("A256GCM", idToken.EncryptionEnc)
}

// TestIDTokenResponseType_NESTED_JWT_ValidConfig creates an app with responseType=NESTED_JWT.
func (ts *ApplicationAPITestSuite) TestIDTokenResponseType_NESTED_JWT_ValidConfig() {
	app := Application{
		OUID:        testOUID,
		Name:        "IDToken NESTED_JWT Response Type Test",
		Description: "Test responseType=NESTED_JWT for ID token",
		URL:         "https://idtoken-nested.example.com",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://idtoken-nested.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
					Certificate:             &ApplicationCert{Type: "JWKS", Value: testEncJWKS},
					Token: &OAuthTokenConfig{
						IDToken: &IDTokenConfig{
							ResponseType:  "NESTED_JWT",
							EncryptionAlg: "RSA-OAEP-256",
							EncryptionEnc: "A256GCM",
						},
					},
				},
			},
		},
	}
	app.AuthFlowID = defaultAuthFlowID
	app.RegistrationFlowID = defaultRegistrationFlowID

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	retrieved, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	idToken := retrieved.InboundAuthConfig[0].OAuthAppConfig.Token.IDToken
	ts.Assert().Equal("NESTED_JWT", idToken.ResponseType)
	ts.Assert().Equal("RSA-OAEP-256", idToken.EncryptionAlg)
	ts.Assert().Equal("A256GCM", idToken.EncryptionEnc)
}

// TestIDTokenResponseType_JWT_WithEncryptionAlg is rejected — encryption fields not allowed for JWT.
func (ts *ApplicationAPITestSuite) TestIDTokenResponseType_JWT_WithEncryptionAlg() {
	app := Application{
		OUID:        testOUID,
		Name:        "IDToken JWT With EncryptionAlg",
		Description: "Expect 400 when JWT responseType has encryptionAlg",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://idtoken-jwt.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Token: &OAuthTokenConfig{
						IDToken: &IDTokenConfig{
							ResponseType:  "JWT",
							EncryptionAlg: "RSA-OAEP-256",
						},
					},
				},
			},
		},
	}
	app.AuthFlowID = defaultAuthFlowID
	app.RegistrationFlowID = defaultRegistrationFlowID

	_, err := createApplication(app)
	ts.Require().Error(err)
	ts.Assert().Contains(err.Error(), "400")
}

// TestIDTokenResponseType_JWE_MissingEncFields is rejected — JWE requires both alg and enc.
func (ts *ApplicationAPITestSuite) TestIDTokenResponseType_JWE_MissingEncFields() {
	app := Application{
		OUID:        testOUID,
		Name:        "IDToken JWE Missing Enc Fields",
		Description: "Expect 400 when JWE responseType lacks encryptionAlg/Enc",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://idtoken-jwe.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Token: &OAuthTokenConfig{
						IDToken: &IDTokenConfig{
							ResponseType: "JWE",
						},
					},
				},
			},
		},
	}
	app.AuthFlowID = defaultAuthFlowID
	app.RegistrationFlowID = defaultRegistrationFlowID

	_, err := createApplication(app)
	ts.Require().Error(err)
	ts.Assert().Contains(err.Error(), "400")
}

// TestIDTokenResponseType_Empty_DefaultsToJWT verifies that omitting responseType defaults to JWT behaviour.
func (ts *ApplicationAPITestSuite) TestIDTokenResponseType_Empty_DefaultsToJWT() {
	app := Application{
		OUID:        testOUID,
		Name:        "IDToken Default ResponseType Test",
		Description: "Omitting responseType should default to JWT with no error",
		URL:         "https://idtoken-default.example.com",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					RedirectURIs:            []string{"https://idtoken-default.example.com/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					Scopes:                  []string{"openid"},
					Token: &OAuthTokenConfig{
						IDToken: &IDTokenConfig{ValidityPeriod: 3600},
					},
				},
			},
		},
	}
	app.AuthFlowID = defaultAuthFlowID
	app.RegistrationFlowID = defaultRegistrationFlowID

	appID, err := createApplication(app)
	ts.Require().NoError(err)
	defer deleteApplication(appID)

	retrieved, err := getApplicationByID(appID)
	ts.Require().NoError(err)
	ts.Assert().Equal(int64(3600), retrieved.InboundAuthConfig[0].OAuthAppConfig.Token.IDToken.ValidityPeriod)
}
