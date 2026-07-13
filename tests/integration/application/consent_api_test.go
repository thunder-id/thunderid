/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// mockConsentServerPort is the port the mock OpenFGC consent server will bind to.
	mockConsentServerPort = 8096
	// mockConsentServerBaseURL is the inspection/test base URL of the mock server.
	mockConsentServerBaseURL = "http://localhost:8096"
	// mockConsentServerAPIBaseURL is the API base URL passed to the server's consent config.
	mockConsentServerAPIBaseURL = "http://localhost:8096/api/v1"
)

// consentEnabledPatch is the deployment.yaml patch applied in SetupSuite to enable the
// consent service and point it at the local mock server.
var consentEnabledPatch = map[string]interface{}{
	"consent": map[string]interface{}{
		"enabled":     true,
		"base_url":    mockConsentServerAPIBaseURL,
		"timeout":     10,
		"max_retries": 3,
	},
}

// consentDisabledPatch is the deployment.yaml patch applied in TearDownSuite to restore
// consent to disabled so subsequent test suites are unaffected.
var consentDisabledPatch = map[string]interface{}{
	"consent": map[string]interface{}{
		"enabled": false,
	},
}

// mockConsentPurposeResponse mirrors the purposeResponseDTO exposed by the mock server's
// test inspection endpoint (/test/purposes).
type mockConsentPurposeResponse struct {
	ID       string                      `json:"purposeId"`
	Name     string                      `json:"name"`
	GroupID  string                      `json:"groupId"`
	Elements []mockConsentPurposeElement `json:"elements"`
}

// mockConsentPurposeElement mirrors the purposeElementDTO used in the mock server's
// internal state and responses.
type mockConsentPurposeElement struct {
	Name        string `json:"name"`
	IsMandatory bool   `json:"mandatory"`
}

type ConsentAPITestSuite struct {
	suite.Suite
	mockConsentServer         *testutils.MockConsentServer
	consentAuthFlowID         string
	consentRegistrationFlowID string
	ouID                      string
}

func TestConsentAPITestSuite(t *testing.T) {
	suite.Run(t, new(ConsentAPITestSuite))
}

func (ts *ConsentAPITestSuite) SetupSuite() {
	// 1. Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "test_consent_ou",
		Name:        "Test Organization Unit for Consent",
		Description: "Organization unit created for consent API testing",
		Parent:      nil,
	})
	ts.Require().NoError(err, "Failed to create consent test organization unit")
	ts.ouID = ouID

	// 2. Start the mock consent server
	ts.mockConsentServer = testutils.NewMockConsentServer(mockConsentServerPort)
	ts.Require().NoError(ts.mockConsentServer.Start(), "failed to start mock consent server")

	// 3. Patch deployment config to enable consent
	ts.Require().NoError(
		testutils.PatchDeploymentConfig(consentEnabledPatch),
		"failed to patch deployment config to enable consent",
	)

	// 4. Restart the server and wait for it to be ready
	ts.Require().NoError(
		testutils.RestartServer(),
		"failed to restart the server with consent-enabled config",
	)

	// 5. Re-obtain admin token after restart
	ts.Require().NoError(
		testutils.ObtainAdminAccessToken(),
		"failed to obtain admin access token after restart",
	)

	// 6. Fetch flow IDs
	ts.consentAuthFlowID, err = testutils.GetFlowIDByHandle("default-flow", "AUTHENTICATION")
	ts.Require().NoError(err, "failed to get default authentication flow ID")

	ts.consentRegistrationFlowID, err = testutils.GetFlowIDByHandle("default-flow", "REGISTRATION")
	ts.Require().NoError(err, "failed to get default registration flow ID")
}

func (ts *ConsentAPITestSuite) TearDownSuite() {
	// Restore consent config to disabled
	if err := testutils.PatchDeploymentConfig(consentDisabledPatch); err != nil {
		ts.T().Logf("teardown: failed to restore consent config: %v", err)
	}

	// Restart the server with the restored config
	if err := testutils.RestartServer(); err != nil {
		ts.T().Logf("teardown: Server did not come back after config restore: %v", err)
	}

	// Re-obtain admin token
	if err := testutils.ObtainAdminAccessToken(); err != nil {
		ts.T().Logf("teardown: failed to re-obtain admin token after config restore: %v", err)
	}

	// Delete the consent test organization unit
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("teardown: failed to delete consent test organization unit: %v", err)
		}
	}

	// Stop the mock consent server
	if err := ts.mockConsentServer.Stop(); err != nil {
		ts.T().Logf("teardown: failed to stop mock consent server: %v", err)
	}
}

func (ts *ConsentAPITestSuite) TestConsentPurposeCreatedOnApplicationCreate() {
	app := Application{
		OUID:               ts.ouID,
		Name:               "Consent Create Test App",
		Description:        "App to test consent purpose creation",
		AuthFlowID:         ts.consentAuthFlowID,
		RegistrationFlowID: ts.consentRegistrationFlowID,
		LoginConsent: &LoginConsentConfig{
			ValidityPeriod: 3600,
		},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "consent_create_test_client",
					ClientSecret:            "consent_create_test_secret",
					RedirectURIs:            []string{"http://localhost/consent-create-test/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					UserInfo: &UserInfoConfig{
						UserAttributes: []string{"email", "username"},
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err, "failed to create application with login_consent")
	defer func() {
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("teardown: failed to delete application %s: %v", appID, err)
		}
	}()

	purposes, err := getMockPurposesForApp(appID)
	ts.Require().NoError(err, "failed to retrieve purposes from mock consent server")

	ts.Assert().Len(purposes, 1, "expected exactly one consent purpose to be created")
	ts.Assert().Equal(2, len(purposes[0].Elements),
		"expected consent purpose to contain 2 elements (email, username)")
}

func (ts *ConsentAPITestSuite) TestConsentPurposeNotCreatedWhenNoUserAttributes() {
	app := Application{
		OUID:               ts.ouID,
		Name:               "Consent No Attrs Test App",
		Description:        "App to test consent with no user attributes",
		AuthFlowID:         ts.consentAuthFlowID,
		RegistrationFlowID: ts.consentRegistrationFlowID,
		LoginConsent: &LoginConsentConfig{
			ValidityPeriod: 0,
		},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "consent_no_attrs_test_client",
					ClientSecret:            "consent_no_attrs_test_secret",
					RedirectURIs:            []string{"http://localhost/consent-no-attrs/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					// No UserInfo/UserAttributes configured.
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err, "failed to create application with login_consent but no user attributes")
	defer func() {
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("teardown: failed to delete application %s: %v", appID, err)
		}
	}()

	purposes, err := getMockPurposesForApp(appID)
	ts.Require().NoError(err, "failed to retrieve purposes from mock consent server")

	ts.Assert().Empty(purposes,
		"expected no consent purpose when application has no user attributes")
}

func (ts *ConsentAPITestSuite) TestConsentPurposeUpdatedOnApplicationUpdate() {
	// Step 1: Create app with 1 attribute.
	app := Application{
		OUID:               ts.ouID,
		Name:               "Consent Update Test App",
		Description:        "App to test consent purpose update",
		AuthFlowID:         ts.consentAuthFlowID,
		RegistrationFlowID: ts.consentRegistrationFlowID,
		LoginConsent: &LoginConsentConfig{
			ValidityPeriod: 0,
		},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "consent_update_test_client",
					ClientSecret:            "consent_update_test_secret",
					RedirectURIs:            []string{"http://localhost/consent-update/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					UserInfo: &UserInfoConfig{
						UserAttributes: []string{"email"},
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err, "failed to create application")
	defer func() {
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("teardown: failed to delete application %s: %v", appID, err)
		}
	}()

	// Verify initial state: 1 purpose with 1 element.
	purposes, err := getMockPurposesForApp(appID)
	ts.Require().NoError(err)
	ts.Require().Len(purposes, 1, "expected 1 purpose after create")
	ts.Require().Len(purposes[0].Elements, 1, "expected 1 element after create")

	// Step 2: Update app to add another attribute.
	updatedApp := app
	updatedApp.InboundAuthConfig[0].OAuthAppConfig.UserInfo = &UserInfoConfig{
		UserAttributes: []string{"email", "username"},
	}

	err = updateApplication(appID, updatedApp)
	ts.Require().NoError(err, "failed to update application")

	// Verify updated state: still 1 purpose, now with 2 elements.
	purposes, err = getMockPurposesForApp(appID)
	ts.Require().NoError(err)
	ts.Assert().Len(purposes, 1, "expected 1 purpose after update")
	ts.Assert().Len(purposes[0].Elements, 2, "expected 2 elements after attribute was added")
}

func (ts *ConsentAPITestSuite) TestConsentPurposePreservedOnAttributeRemoval() {
	// Step 1: Create app with login_consent.
	app := Application{
		OUID:               ts.ouID,
		Name:               "Consent Disable Test App",
		Description:        "App to test consent purpose preservation on attribute removal",
		AuthFlowID:         ts.consentAuthFlowID,
		RegistrationFlowID: ts.consentRegistrationFlowID,
		LoginConsent: &LoginConsentConfig{
			ValidityPeriod: 0,
		},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "consent_disable_test_client",
					ClientSecret:            "consent_disable_test_secret",
					RedirectURIs:            []string{"http://localhost/consent-disable/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					UserInfo: &UserInfoConfig{
						UserAttributes: []string{"email"},
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err, "failed to create application")
	defer func() {
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("teardown: failed to delete application %s: %v", appID, err)
		}
	}()

	// Verify initial state: 1 purpose with the original element.
	purposes, err := getMockPurposesForApp(appID)
	ts.Require().NoError(err)
	ts.Require().Len(purposes, 1, "expected 1 purpose after create")
	originalPurposeID := purposes[0].ID

	// Step 2: Remove the configured attribute.
	updatedApp := app
	updatedApp.InboundAuthConfig[0].OAuthAppConfig.UserInfo = &UserInfoConfig{
		UserAttributes: []string{},
	}

	err = updateApplication(appID, updatedApp)
	ts.Require().NoError(err, "failed to update application to remove user attributes")

	// Verify that the purpose is preserved untouched — purposes outlive their owning app's
	// request set so prior consents remain interpretable.
	purposes, err = getMockPurposesForApp(appID)
	ts.Require().NoError(err)
	ts.Require().Len(purposes, 1, "expected attribute purpose to be preserved after attribute removal")
	ts.Assert().Equal(originalPurposeID, purposes[0].ID, "purpose ID should be unchanged")
	ts.Assert().Len(purposes[0].Elements, 1, "purpose elements should remain unchanged")
}

func (ts *ConsentAPITestSuite) TestConsentPurposePreservedOnApplicationDelete() {
	app := Application{
		OUID:               ts.ouID,
		Name:               "Consent Delete Test App",
		Description:        "App to test consent purpose preservation on app delete",
		AuthFlowID:         ts.consentAuthFlowID,
		RegistrationFlowID: ts.consentRegistrationFlowID,
		LoginConsent: &LoginConsentConfig{
			ValidityPeriod: 0,
		},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "consent_delete_test_client",
					ClientSecret:            "consent_delete_test_secret",
					RedirectURIs:            []string{"http://localhost/consent-delete/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					UserInfo: &UserInfoConfig{
						UserAttributes: []string{"email"},
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err, "failed to create application")

	// Verify a purpose was created.
	purposes, err := getMockPurposesForApp(appID)
	ts.Require().NoError(err)
	ts.Require().Len(purposes, 1, "expected 1 purpose before delete")
	originalPurposeID := purposes[0].ID

	// Delete the application.
	err = deleteApplication(appID)
	ts.Require().NoError(err, "failed to delete application")

	// Verify the purpose survives — purposes outlive the app so prior consents remain
	// interpretable.
	purposes, err = getMockPurposesForApp(appID)
	ts.Require().NoError(err)
	ts.Require().Len(purposes, 1, "expected consent purpose to be preserved when application is deleted")
	ts.Assert().Equal(originalPurposeID, purposes[0].ID, "purpose ID should be unchanged")
}

// TestPurposesPreservedOnAttributeOnlyUpdate verifies that updating an application to
// have zero user attributes leaves every existing purpose untouched. Both the attribute
// purpose (managed by the inbound client) and any permission purpose (lazily created by
// the consent enforcer on auth flow runs, simulated here via direct mock injection) must
// survive, because purposes outlive their owning app's request set.
func (ts *ConsentAPITestSuite) TestPurposesPreservedOnAttributeOnlyUpdate() {
	app := Application{
		OUID:               ts.ouID,
		Name:               "Consent Permission Preserved Test App",
		Description:        "App to verify permission purpose survives attribute removal",
		AuthFlowID:         ts.consentAuthFlowID,
		RegistrationFlowID: ts.consentRegistrationFlowID,
		LoginConsent:       &LoginConsentConfig{ValidityPeriod: 0},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "consent_perm_preserved_test_client",
					ClientSecret:            "consent_perm_preserved_test_secret",
					RedirectURIs:            []string{"http://localhost/consent-perm-preserved/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					UserInfo:                &UserInfoConfig{UserAttributes: []string{"email"}},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err, "failed to create application")
	defer func() {
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("teardown: failed to delete application %s: %v", appID, err)
		}
	}()

	// Verify the attribute purpose was created by app creation.
	purposes, err := getMockPurposesForApp(appID)
	ts.Require().NoError(err)
	ts.Require().Len(purposes, 1, "expected 1 attribute purpose after create")
	attrPurposeID := purposes[0].ID

	// Inject a permission purpose into the mock to simulate a prior auth-flow run having
	// lazily created it via applyPermissionsPurpose. The mock does not know about purpose
	// types — it just stores whatever name+elements we give it.
	permPurposeID, err := createMockPurpose(appID, "permissions:"+appID, []mockConsentPurposeElement{
		{Name: "booking:reservations:read"},
		{Name: "booking:reservations:create"},
	})
	ts.Require().NoError(err, "failed to seed permission purpose into mock")

	// Update the application to remove all attributes.
	updatedApp := app
	updatedApp.InboundAuthConfig[0].OAuthAppConfig.UserInfo = &UserInfoConfig{UserAttributes: []string{}}
	ts.Require().NoError(updateApplication(appID, updatedApp), "failed to update application")

	// Verify: both purposes are preserved — purposes outlive their owning app's request set.
	purposes, err = getMockPurposesForApp(appID)
	ts.Require().NoError(err)
	ts.Require().Len(purposes, 2, "expected both attribute and permission purposes to be preserved")
	purposeIDs := map[string]bool{purposes[0].ID: true, purposes[1].ID: true}
	ts.Assert().True(purposeIDs[attrPurposeID], "attribute purpose should be preserved")
	ts.Assert().True(purposeIDs[permPurposeID], "permission purpose should be preserved")
}

// TestBothPurposesPreservedOnApplicationDelete verifies that deleting an application leaves
// every purpose owned by it intact — both the attribute purpose (managed by the inbound
// client) and the permission purpose (lazily created by the consent enforcer, simulated
// here via direct mock injection). Purposes outlive the app so prior consents remain
// interpretable.
func (ts *ConsentAPITestSuite) TestBothPurposesPreservedOnApplicationDelete() {
	app := Application{
		OUID:               ts.ouID,
		Name:               "Consent Delete Both Test App",
		Description:        "App to verify both attribute and permission purposes are deleted",
		AuthFlowID:         ts.consentAuthFlowID,
		RegistrationFlowID: ts.consentRegistrationFlowID,
		LoginConsent:       &LoginConsentConfig{ValidityPeriod: 0},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "consent_delete_both_test_client",
					ClientSecret:            "consent_delete_both_test_secret",
					RedirectURIs:            []string{"http://localhost/consent-delete-both/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					UserInfo:                &UserInfoConfig{UserAttributes: []string{"email"}},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err, "failed to create application")

	// Inject a permission purpose to simulate one created earlier by the consent enforcer.
	permPurposeID, err := createMockPurpose(appID, "permissions:"+appID, []mockConsentPurposeElement{
		{Name: "booking:reservations:cancel"},
	})
	ts.Require().NoError(err, "failed to seed permission purpose into mock")

	// Verify pre-state: both purposes exist.
	purposes, err := getMockPurposesForApp(appID)
	ts.Require().NoError(err)
	ts.Require().Len(purposes, 2, "expected both attribute and permission purposes before delete")
	attrPurposeID := ""
	for _, p := range purposes {
		if p.ID != permPurposeID {
			attrPurposeID = p.ID
			break
		}
	}

	// Delete the application.
	ts.Require().NoError(deleteApplication(appID), "failed to delete application")

	// Verify: both purposes survive — app deletion no longer touches consent purposes.
	purposes, err = getMockPurposesForApp(appID)
	ts.Require().NoError(err)
	ts.Require().Len(purposes, 2, "expected both purposes to be preserved when application is deleted")
	purposeIDs := map[string]bool{purposes[0].ID: true, purposes[1].ID: true}
	ts.Assert().True(purposeIDs[attrPurposeID], "attribute purpose should be preserved")
	ts.Assert().True(purposeIDs[permPurposeID], "permission purpose should be preserved")
}

func (ts *ConsentAPITestSuite) TestLoginConsentEnabledFieldPersistedCorrectly() {
	app := Application{
		OUID:               ts.ouID,
		Name:               "Consent Persist Test App",
		Description:        "App to test login_consent field persistence",
		AuthFlowID:         ts.consentAuthFlowID,
		RegistrationFlowID: ts.consentRegistrationFlowID,
		LoginConsent: &LoginConsentConfig{
			ValidityPeriod: 7200,
		},
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "consent_persist_test_client",
					ClientSecret:            "consent_persist_test_secret",
					RedirectURIs:            []string{"http://localhost/consent-persist/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					UserInfo: &UserInfoConfig{
						UserAttributes: []string{"email"},
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err, "failed to create application")
	defer func() {
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("teardown: failed to delete application %s: %v", appID, err)
		}
	}()

	retrieved, err := getApplicationByID(appID)
	ts.Require().NoError(err, "failed to retrieve application")
	ts.Require().NotNil(retrieved.LoginConsent, "login_consent should be present in response")

	ts.Assert().Equal(int64(7200), retrieved.LoginConsent.ValidityPeriod,
		"login_consent.validity_period should be 7200")
}

// updateApplication sends a PUT /applications/{id} request and returns any error.
func updateApplication(appID string, app Application) error {
	appJSON, err := json.Marshal(app)
	if err != nil {
		return fmt.Errorf("failed to marshal application for update: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, testServerURL+"/applications/"+appID, bytes.NewReader(appJSON))
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send PUT request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, string(body))
	}

	return nil
}

// createMockPurpose seeds a consent purpose directly into the mock server, simulating one
// that would have been lazily created by the consent enforcer during an auth flow run.
// Returns the ID assigned by the mock.
func createMockPurpose(appID, name string, elements []mockConsentPurposeElement) (string, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"name":     name,
		"elements": elements,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal purpose payload: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, mockConsentServerAPIBaseURL+"/consent-purposes", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("group-id", appID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("mock returned status %d: %s", resp.StatusCode, string(body))
	}
	var created struct {
		ID string `json:"purposeId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		return "", fmt.Errorf("failed to decode purpose response: %w", err)
	}
	return created.ID, nil
}

// getMockPurposesForApp queries the mock consent server's test inspection endpoint
// and returns the consent purposes currently stored for the given application ID.
func getMockPurposesForApp(appID string) ([]mockConsentPurposeResponse, error) {
	url := fmt.Sprintf("%s/test/purposes?groupIds=%s", mockConsentServerBaseURL, appID)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to query mock consent server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mock consent server returned status %d: %s", resp.StatusCode, string(body))
	}

	var purposes []mockConsentPurposeResponse
	if err := json.NewDecoder(resp.Body).Decode(&purposes); err != nil {
		return nil, fmt.Errorf("failed to decode purposes response: %w", err)
	}

	return purposes, nil
}
