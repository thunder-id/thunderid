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

package authz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	acrClientID     = "acr_authz_test_client"
	acrClientSecret = "acr_authz_test_secret"
	acrAppName      = "ACRAuthzTestApp"
	acrRedirectURI  = "https://localhost:3000/acr-callback"
)

var acrValuesAuthzTestOU = testutils.OrganizationUnit{
	Handle:      "acr-values-authz-test-ou",
	Name:        "ACR Values Authorization Test Organization Unit",
	Description: "Organization unit for ACR values authorization testing",
	Parent:      nil,
}

var acrValuesAuthzFlow = testutils.Flow{
	Name:     "ACR Values Authorization Test Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_acr_values_authz_test",
	Nodes: []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "prompt_credentials",
		},
		{
			"id":   "prompt_credentials",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
					"action": map[string]interface{}{"ref": "action_001", "nextNode": "basic_auth"},
				},
			},
		},
		{
			"id":   "basic_auth",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "BasicAuthExecutor",
				"inputs": []map[string]interface{}{
					{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
			},
			"onSuccess": "auth_assert",
		},
		{
			"id":   "auth_assert",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{"name": "AuthAssertExecutor"},
			"onSuccess": "end",
		},
		{
			"id":   "end",
			"type": "END",
		},
	},
}

// AcrValuesAuthzTestSuite tests acr_values behaviour in the authorization endpoint.
type AcrValuesAuthzTestSuite struct {
	suite.Suite
	client        *http.Client
	applicationID string
	authFlowID    string
	ouID          string
}

func TestAcrValuesAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(AcrValuesAuthzTestSuite))
}

func (ts *AcrValuesAuthzTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(acrValuesAuthzTestOU)
	ts.Require().NoError(err, "failed to create test organization unit")
	ts.ouID = ouID

	flowID, err := testutils.CreateFlow(acrValuesAuthzFlow)
	ts.Require().NoError(err, "failed to create auth flow for ACR values test")
	ts.authFlowID = flowID

	app := map[string]interface{}{
		"name":                      acrAppName,
		"description":               "Application for acr_values authorization integration tests",
		"ouId":                      ts.ouID,
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":     acrClientID,
					"clientSecret": acrClientSecret,
					"redirectUris": []string{acrRedirectURI},
					"grantTypes":   []string{"authorization_code"},
					"responseTypes": []string{
						"code",
					},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"acrValues": []string{
						"urn:thunder:acr:password",
						"urn:thunder:acr:generated-code",
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("failed to create ACR test application: status=%d body=%s", resp.StatusCode, body)
	}

	var respData map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&respData))
	ts.applicationID = respData["id"].(string)
}

func (ts *AcrValuesAuthzTestSuite) TearDownSuite() {
	if ts.applicationID != "" {
		req, err := http.NewRequest(http.MethodDelete,
			fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, ts.applicationID), nil)
		if err != nil {
			ts.T().Logf("failed to build delete request: %v", err)
			return
		}
		resp, err := ts.client.Do(req)
		if err != nil {
			ts.T().Logf("failed to delete ACR test application: %v", err)
			return
		}
		resp.Body.Close()
	}

	if ts.authFlowID != "" {
		if err := testutils.DeleteFlow(ts.authFlowID); err != nil {
			ts.T().Logf("failed to delete ACR test auth flow: %v", err)
		}
	}

	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("failed to delete test organization unit: %v", err)
		}
	}
}

// initiateAuthorizeWithAcrValues sends a GET /oauth2/authorize request with acr_values.
func (ts *AcrValuesAuthzTestSuite) initiateAuthorizeWithAcrValues(acrValues string) *http.Response {
	params := url.Values{}
	params.Set("client_id", acrClientID)
	params.Set("redirect_uri", acrRedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid")
	params.Set("state", "acr_test_state")
	if acrValues != "" {
		params.Set("acr_values", acrValues)
	}

	req, err := http.NewRequest(http.MethodGet,
		testutils.TestServerURL+"/oauth2/authorize?"+params.Encode(), nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetNoRedirectHTTPClient().Do(req)
	ts.Require().NoError(err)
	return resp
}

// TestAcrValues_WithMatchingValues verifies that an acr_values param whose values are all
// present in the app's acr_values list results in a valid authorization redirect.
func (ts *AcrValuesAuthzTestSuite) TestAcrValues_WithMatchingValues() {
	resp := ts.initiateAuthorizeWithAcrValues("urn:thunder:acr:password")
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusFound, resp.StatusCode)
	location := resp.Header.Get("Location")
	ts.Assert().NotEmpty(location, "expected redirect to login page")

	authID, flowID, err := testutils.ExtractAuthData(location)
	ts.Assert().NoError(err, "expected valid flow redirect for matching acr_values")
	ts.Assert().NotEmpty(authID)
	ts.Assert().NotEmpty(flowID)
}

// TestAcrValues_WithNoDefaults_PassThrough verifies that when no acr_values are requested
// the authorization request succeeds for an app with acr_values configured.
func (ts *AcrValuesAuthzTestSuite) TestAcrValues_WithNoDefaults_PassThrough() {
	resp := ts.initiateAuthorizeWithAcrValues("")
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusFound, resp.StatusCode)
	location := resp.Header.Get("Location")
	ts.Assert().NotEmpty(location)

	authID, flowID, err := testutils.ExtractAuthData(location)
	ts.Assert().NoError(err, "expected valid flow redirect")
	ts.Assert().NotEmpty(authID)
	ts.Assert().NotEmpty(flowID)
}

// TestAcrValues_WithNoneInDefaults_FallsBackToDefaults verifies that when none of the
// requested acr_values match the app's acr_values list, the authorization
// request still succeeds (falls back to all defaults).
func (ts *AcrValuesAuthzTestSuite) TestAcrValues_WithNoneInDefaults_FallsBackToDefaults() {
	// "urn:thunder:acr:biometrics" is not in the app's acr_values list.
	resp := ts.initiateAuthorizeWithAcrValues("urn:thunder:acr:biometrics")
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusFound, resp.StatusCode)
	location := resp.Header.Get("Location")
	ts.Assert().NotEmpty(location)

	authID, flowID, err := testutils.ExtractAuthData(location)
	ts.Assert().NoError(err, "expected valid flow redirect even when requested ACR not in defaults")
	ts.Assert().NotEmpty(authID)
	ts.Assert().NotEmpty(flowID)
}

// TestAcrValues_PartialMatchWithDefaults verifies that when only some of the requested
// acr_values are in the app's acr_values list, the authorization succeeds.
func (ts *AcrValuesAuthzTestSuite) TestAcrValues_PartialMatchWithDefaults() {
	// "urn:thunder:acr:password" is in the list; "urn:thunder:acr:biometrics" is not.
	resp := ts.initiateAuthorizeWithAcrValues("urn:thunder:acr:password urn:thunder:acr:biometrics")
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusFound, resp.StatusCode)
	location := resp.Header.Get("Location")
	ts.Assert().NotEmpty(location)

	authID, flowID, err := testutils.ExtractAuthData(location)
	ts.Assert().NoError(err, "expected valid flow redirect for partial ACR match")
	ts.Assert().NotEmpty(authID)
	ts.Assert().NotEmpty(flowID)
}
