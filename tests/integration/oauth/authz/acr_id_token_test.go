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
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	acrE2EClientID     = "acr_e2e_test_client"
	acrE2EClientSecret = "acr_e2e_test_secret"
	acrE2ERedirectURI  = "https://localhost:3000/acr-e2e-callback"
)

var acrE2EFlow = testutils.Flow{
	Name:     "ACR E2E ID Token Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_acr_e2e_test",
	Nodes: []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "acr_chooser",
		},
		{
			"id":      "acr_chooser",
			"type":    "PROMPT",
			"variant": "LOGIN_OPTIONS",
			"properties": map[string]interface{}{
				"authMethodMapping": map[string]interface{}{
					"urn:thunder:acr:password":       "pwd_action",
					"urn:thunder:acr:generated-code": "code_action",
				},
			},
			"prompts": []map[string]interface{}{
				{
					"action": map[string]interface{}{
						"ref":      "pwd_action",
						"nextNode": "prompt_pwd",
					},
				},
				{
					"action": map[string]interface{}{
						"ref":      "code_action",
						"nextNode": "prompt_code",
					},
				},
			},
		},
		{
			"id":   "prompt_pwd",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{"ref": "input_u1", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_p1", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
					"action": map[string]interface{}{
						"ref":      "submit_pwd",
						"nextNode": "basic_auth_pwd",
					},
				},
			},
		},
		{
			"id":   "prompt_code",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{"ref": "input_u2", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_p2", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
					"action": map[string]interface{}{
						"ref":      "submit_code",
						"nextNode": "basic_auth_code",
					},
				},
			},
		},
		{
			"id":   "basic_auth_pwd",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "BasicAuthExecutor",
				"inputs": []map[string]interface{}{
					{"ref": "input_u1", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_p1", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
			},
			"onSuccess":    "auth_assert",
			"onIncomplete": "prompt_pwd",
		},
		{
			"id":   "basic_auth_code",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "BasicAuthExecutor",
				"inputs": []map[string]interface{}{
					{"ref": "input_u2", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_p2", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
			},
			"onSuccess":    "auth_assert",
			"onIncomplete": "prompt_code",
		},
		{
			"id":   "auth_assert",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "AuthAssertExecutor",
			},
			"onSuccess": "end",
		},
		{
			"id":   "end",
			"type": "END",
		},
	},
}

var acrE2ETestOU = testutils.OrganizationUnit{
	Handle:      "acr-e2e-test-ou",
	Name:        "ACR E2E Test Organization Unit",
	Description: "Organization unit for ACR E2E ID token testing",
	Parent:      nil,
}

var acrE2EUserSchema = testutils.UserType{
	Name: "acr_e2e_test_person",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
		"email":    map[string]interface{}{"type": "string"},
	},
}

var acrE2ETestUser = testutils.User{
	Type: acrE2EUserSchema.Name,
	Attributes: json.RawMessage(`{
		"username": "acre2euser",
		"password": "testpassword",
		"email": "acre2euser@example.com"
	}`),
}

type AcrIDTokenTestSuite struct {
	suite.Suite
	client        *http.Client
	applicationID string
	authFlowID    string
	ouID          string
	userSchemaID  string
	userIDs       []string
}

func TestAcrIDTokenTestSuite(t *testing.T) {
	suite.Run(t, new(AcrIDTokenTestSuite))
}

func (ts *AcrIDTokenTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(acrE2ETestOU)
	ts.Require().NoError(err, "failed to create OU")
	ts.ouID = ouID

	schema := acrE2EUserSchema
	schema.OUID = ts.ouID
	schemaID, err := testutils.CreateUserType(schema)
	ts.Require().NoError(err, "failed to create user schema")
	ts.userSchemaID = schemaID

	user := acrE2ETestUser
	user.OUID = ts.ouID
	userIDs, err := testutils.CreateMultipleUsers(user)
	ts.Require().NoError(err, "failed to create test user")
	ts.userIDs = userIDs

	flowID, err := testutils.CreateFlow(acrE2EFlow)
	ts.Require().NoError(err, "failed to create ACR E2E flow")
	ts.authFlowID = flowID

	app := testutils.Application{
		Name:                      "ACR E2E ID Token Test App",
		Description:               "Application for ACR E2E ID token tests",
		IsRegistrationFlowEnabled: false,
		OUID:                      ts.ouID,
		AuthFlowID:                ts.authFlowID,
		ClientID:                  acrE2EClientID,
		ClientSecret:              acrE2EClientSecret,
		RedirectURIs:              []string{acrE2ERedirectURI},
		AllowedUserTypes:          []string{acrE2EUserSchema.Name},
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                acrE2EClientID,
					"clientSecret":            acrE2EClientSecret,
					"redirectUris":            []string{acrE2ERedirectURI},
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"acrValues": []string{
						"urn:thunder:acr:password",
						"urn:thunder:acr:generated-code",
					},
				},
			},
		},
	}

	appID, err := testutils.CreateApplication(app)
	ts.Require().NoError(err, "failed to create ACR E2E test application")
	ts.applicationID = appID
}

func (ts *AcrIDTokenTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.userIDs); err != nil {
		ts.T().Logf("failed to cleanup users: %v", err)
	}
	if ts.applicationID != "" {
		if err := testutils.DeleteApplication(ts.applicationID); err != nil {
			ts.T().Logf("failed to delete application: %v", err)
		}
	}
	if ts.authFlowID != "" {
		if err := testutils.DeleteFlow(ts.authFlowID); err != nil {
			ts.T().Logf("failed to delete flow: %v", err)
		}
	}
	if ts.userSchemaID != "" {
		if err := testutils.DeleteUserType(ts.userSchemaID); err != nil {
			ts.T().Logf("failed to delete user schema: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("failed to delete OU: %v", err)
		}
	}
}

// TestAcrClaimInIDToken runs the full authorization_code flow and asserts the acr claim.
func (ts *AcrIDTokenTestSuite) TestAcrClaimInIDToken() {
	resp, err := ts.initiateAuthorize("urn:thunder:acr:password")
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusFound, resp.StatusCode)
	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location)

	authID, flowID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(authID)
	ts.Require().NotEmpty(flowID)

	flowStep, err := testutils.ExecuteAuthenticationFlow(flowID, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	flowStep, err = testutils.ExecuteAuthenticationFlow(flowID, map[string]string{
		"username": "acre2euser",
		"password": "testpassword",
	}, "submit_pwd", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "flow should complete after valid credentials")
	ts.Require().NotEmpty(flowStep.Assertion, "assertion must be present")

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(authzResp.RedirectURI)

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err, "authorization code must be in the redirect")
	ts.Require().NotEmpty(code)

	tokenResult, err := testutils.RequestToken(
		acrE2EClientID, acrE2EClientSecret, code, acrE2ERedirectURI, "authorization_code")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, "token exchange should succeed")
	ts.Require().NotNil(tokenResult.Token)
	ts.Require().NotEmpty(tokenResult.Token.IDToken, "ID token must be present in token response")

	idTokenClaims, err := testutils.DecodeJWT(tokenResult.Token.IDToken)
	ts.Require().NoError(err, "ID token must be decodable")
	ts.Require().NotNil(idTokenClaims)

	acrClaim, ok := idTokenClaims.Additional["acr"]
	ts.Require().True(ok, "acr claim must be present in the ID token")
	ts.Require().Equal("urn:thunder:acr:password", acrClaim,
		"acr claim in ID token must reflect the ACR the user satisfied")
}

// TestAcrClaimInIDToken_WithManualSelection runs the flow with user-selected ACR.
func (ts *AcrIDTokenTestSuite) TestAcrClaimInIDToken_WithManualSelection() {
	resp, err := ts.initiateAuthorize("urn:thunder:acr:generated-code urn:thunder:acr:password")
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusFound, resp.StatusCode)
	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location)

	authID, flowID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err)

	flowStep, err := testutils.ExecuteAuthenticationFlow(flowID, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	flowStep, err = testutils.ExecuteAuthenticationFlow(flowID, nil, "pwd_action", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	flowStep, err = testutils.ExecuteAuthenticationFlow(flowID, map[string]string{
		"username": "acre2euser",
		"password": "testpassword",
	}, "submit_pwd", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)
	ts.Require().NotEmpty(flowStep.Assertion)

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err)

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)

	tokenResult, err := testutils.RequestToken(
		acrE2EClientID, acrE2EClientSecret, code, acrE2ERedirectURI, "authorization_code")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode)
	ts.Require().NotNil(tokenResult.Token)
	ts.Require().NotEmpty(tokenResult.Token.IDToken)

	idTokenClaims, err := testutils.DecodeJWT(tokenResult.Token.IDToken)
	ts.Require().NoError(err)

	acrClaim, ok := idTokenClaims.Additional["acr"]
	ts.Require().True(ok, "acr claim must be present in the ID token")
	ts.Require().Equal("urn:thunder:acr:password", acrClaim,
		"acr claim must reflect the manually selected authentication class")
}

// TestAcrClaimPresentWithAcrValues verifies the fallback to app-configured acrValues
// when the request omits acr_values.
func (ts *AcrIDTokenTestSuite) TestAcrClaimPresentWithAcrValues() {
	resp, err := ts.initiateAuthorize("")
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusFound, resp.StatusCode)
	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location)

	authID, flowID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err)

	flowStep, err := testutils.ExecuteAuthenticationFlow(flowID, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	flowStep, err = testutils.ExecuteAuthenticationFlow(flowID, nil, "pwd_action", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	flowStep, err = testutils.ExecuteAuthenticationFlow(flowID, map[string]string{
		"username": "acre2euser",
		"password": "testpassword",
	}, "submit_pwd", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)
	ts.Require().NotEmpty(flowStep.Assertion)

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err)

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)

	tokenResult, err := testutils.RequestToken(
		acrE2EClientID, acrE2EClientSecret, code, acrE2ERedirectURI, "authorization_code")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode)
	ts.Require().NotNil(tokenResult.Token)
	ts.Require().NotEmpty(tokenResult.Token.IDToken)

	idTokenClaims, err := testutils.DecodeJWT(tokenResult.Token.IDToken)
	ts.Require().NoError(err)

	acrClaim, ok := idTokenClaims.Additional["acr"]
	ts.Require().True(ok, "acr claim must be present when acrValues are configured")
	ts.Require().Equal("urn:thunder:acr:password", acrClaim,
		"acr claim should reflect the selected authentication class")
}

// initiateAuthorize sends GET /oauth2/authorize with the given acr_values parameter.
func (ts *AcrIDTokenTestSuite) initiateAuthorize(acrValues string) (*http.Response, error) {
	params := url.Values{}
	params.Set("client_id", acrE2EClientID)
	params.Set("redirect_uri", acrE2ERedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid")
	params.Set("state", "acr_e2e_state")
	if acrValues != "" {
		params.Set("acr_values", acrValues)
	}

	req, err := http.NewRequest(http.MethodGet,
		testutils.TestServerURL+"/oauth2/authorize?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	return testutils.GetNoRedirectHTTPClient().Do(req)
}
