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
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

var acrValuesTestOU = testutils.OrganizationUnit{
	Handle:      "test-acr-values-ou",
	Name:        "Test Organization Unit for ACR Values",
	Description: "Organization unit for ACR values application testing",
	Parent:      nil,
}

var acrValuesTestOUID string

type AcrValuesAPITestSuite struct {
	suite.Suite
}

func TestAcrValuesAPITestSuite(t *testing.T) {
	suite.Run(t, new(AcrValuesAPITestSuite))
}

func (ts *AcrValuesAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(acrValuesTestOU)
	ts.Require().NoError(err, "failed to create test organization unit")
	acrValuesTestOUID = ouID

	if defaultAuthFlowID == "" {
		defaultAuthFlowID, err = testutils.GetFlowIDByHandle("default-basic-flow", "AUTHENTICATION")
		ts.Require().NoError(err, "failed to get default auth flow ID")
	}
}

func (ts *AcrValuesAPITestSuite) TearDownSuite() {
	if acrValuesTestOUID != "" {
		if err := testutils.DeleteOrganizationUnit(acrValuesTestOUID); err != nil {
			ts.T().Logf("failed to delete test organization unit: %v", err)
		}
	}
}

func (ts *AcrValuesAPITestSuite) TestCreateApplicationWithValidAcrValues() {
	app := Application{
		Name:                      "ACR Values Test App",
		Description:               "Application for testing acr_values",
		IsRegistrationFlowEnabled: false,
		OUID:                      acrValuesTestOUID,
		AuthFlowID:                defaultAuthFlowID,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "acr_values_test_client",
					ClientSecret:            "acr_values_test_secret",
					RedirectURIs:            []string{"http://localhost/acr_values_test/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					AcrValues: []string{
						"urn:thunder:acr:password",
						"urn:thunder:acr:generated-code",
					},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err, "failed to create application with acr_values")
	defer func() {
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete test application: %v", err)
		}
	}()

	retrieved, err := getApplicationByID(appID)
	ts.Require().NoError(err, "failed to retrieve application")
	ts.Require().NotNil(retrieved.InboundAuthConfig)
	ts.Require().Len(retrieved.InboundAuthConfig, 1)
	ts.Require().NotNil(retrieved.InboundAuthConfig[0].OAuthAppConfig)

	ts.Assert().ElementsMatch(
		[]string{"urn:thunder:acr:password", "urn:thunder:acr:generated-code"},
		retrieved.InboundAuthConfig[0].OAuthAppConfig.AcrValues,
		"acr_values should be persisted and returned",
	)
}

func (ts *AcrValuesAPITestSuite) TestCreateApplicationWithInvalidAcrValues() {
	app := Application{
		Name:                      "Invalid ACR App",
		Description:               "Application with invalid acr_values",
		IsRegistrationFlowEnabled: false,
		OUID:                      acrValuesTestOUID,
		AuthFlowID:                defaultAuthFlowID,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "invalid_acr_client",
					ClientSecret:            "invalid_acr_secret",
					RedirectURIs:            []string{"http://localhost/invalid_acr/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					AcrValues:        []string{"urn:unknown:acr:value"},
				},
			},
		},
	}

	appJSON, err := json.Marshal(app)
	ts.Require().NoError(err)

	client := testutils.GetHTTPClient()
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/applications", bytes.NewReader(appJSON))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode,
		"unrecognised ACR value should cause 400 Bad Request")
}

func (ts *AcrValuesAPITestSuite) TestUpdateApplicationAcrValues() {
	app := Application{
		Name:                      "ACR Update Test App",
		Description:               "Application for testing acr_values update",
		IsRegistrationFlowEnabled: false,
		OUID:                      acrValuesTestOUID,
		AuthFlowID:                defaultAuthFlowID,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "acr_update_test_client",
					ClientSecret:            "acr_update_test_secret",
					RedirectURIs:            []string{"http://localhost/acr_update/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err, "failed to create test application")
	defer func() {
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete test application: %v", err)
		}
	}()

	updated := app
	updated.InboundAuthConfig = []InboundAuthConfig{
		{
			Type: "oauth2",
			OAuthAppConfig: &OAuthAppConfig{
				ClientID:                "acr_update_test_client",
				RedirectURIs:            []string{"http://localhost/acr_update/callback"},
				GrantTypes:              []string{"authorization_code"},
				ResponseTypes:           []string{"code"},
				TokenEndpointAuthMethod: "client_secret_basic",
				AcrValues:        []string{"urn:thunder:acr:biometrics"},
			},
		},
	}

	err = updateApplication(appID, updated)
	ts.Require().NoError(err, "failed to update application")

	retrieved, err := getApplicationByID(appID)
	ts.Require().NoError(err, "failed to retrieve updated application")
	ts.Require().NotNil(retrieved.InboundAuthConfig[0].OAuthAppConfig)

	ts.Assert().Equal(
		[]string{"urn:thunder:acr:biometrics"},
		retrieved.InboundAuthConfig[0].OAuthAppConfig.AcrValues,
		"acr_values should reflect the updated value",
	)
}

func (ts *AcrValuesAPITestSuite) TestClearApplicationAcrValues() {
	app := Application{
		Name:                      "ACR Clear Test App",
		Description:               "Application for testing clearing of acr_values",
		IsRegistrationFlowEnabled: false,
		OUID:                      acrValuesTestOUID,
		AuthFlowID:                defaultAuthFlowID,
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				OAuthAppConfig: &OAuthAppConfig{
					ClientID:                "acr_clear_test_client",
					ClientSecret:            "acr_clear_test_secret",
					RedirectURIs:            []string{"http://localhost/acr_clear/callback"},
					GrantTypes:              []string{"authorization_code"},
					ResponseTypes:           []string{"code"},
					TokenEndpointAuthMethod: "client_secret_basic",
					AcrValues:        []string{"urn:thunder:acr:password"},
				},
			},
		},
	}

	appID, err := createApplication(app)
	ts.Require().NoError(err, "failed to create test application")
	defer func() {
		if err := deleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete test application: %v", err)
		}
	}()

	cleared := app
	cleared.InboundAuthConfig = []InboundAuthConfig{
		{
			Type: "oauth2",
			OAuthAppConfig: &OAuthAppConfig{
				ClientID:                "acr_clear_test_client",
				RedirectURIs:            []string{"http://localhost/acr_clear/callback"},
				GrantTypes:              []string{"authorization_code"},
				ResponseTypes:           []string{"code"},
				TokenEndpointAuthMethod: "client_secret_basic",
			},
		},
	}

	err = updateApplication(appID, cleared)
	ts.Require().NoError(err, "failed to update application to clear acr_values")

	retrieved, err := getApplicationByID(appID)
	ts.Require().NoError(err, "failed to retrieve application after clearing")
	ts.Require().NotNil(retrieved.InboundAuthConfig[0].OAuthAppConfig)

	ts.Assert().Empty(retrieved.InboundAuthConfig[0].OAuthAppConfig.AcrValues,
		"acr_values should be empty after clearing")
}
