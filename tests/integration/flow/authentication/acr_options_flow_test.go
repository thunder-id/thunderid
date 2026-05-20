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

package authentication

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

// acrOptionsFlow is a flow that contains a login_options PROMPT node followed by two
// separate password-based authentication paths, each tagged to a distinct ACR value.
//
// Flow graph:
//
//	START → acr_chooser (login_options)
//	  ├─ "pwd_action"  (acr: urn:thunder:acr:password)       → prompt_pwd  → basic_auth_pwd  → auth_assert → END
//	  └─ "code_action" (acr: urn:thunder:acr:generated-code)  → prompt_code → basic_auth_code → auth_assert → END
var acrOptionsFlow = testutils.Flow{
	Name:     "ACR Options Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_acr_options_test",
	Nodes: []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "acr_chooser",
		},
		// login_options prompt node — the ACR chooser
		{
			"id":      "acr_chooser",
			"type":    "PROMPT",
			"variant": "LOGIN_OPTIONS",
			"properties": map[string]interface{}{
				"authMethodMapping": map[string]interface{}{
					"urn:thunder:acr:password":       "pwd_action",
					"urn:thunder:acr:generated-code": "code_action",
					"urn:thunder:acr:biometrics":     "bio_action",
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
				{
					"action": map[string]interface{}{
						"ref":      "bio_action",
						"nextNode": "prompt_bio",
					},
				},
			},
		},
		// Credentials prompt for the password ACR path
		{
			"id":   "prompt_pwd",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_u1",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_p1",
							"identifier": "password",
							"type":       "PASSWORD_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "submit_pwd",
						"nextNode": "basic_auth_pwd",
					},
				},
			},
		},
		// Credentials prompt for the generated-code (OTP-style password) ACR path.
		// For simplicity in tests both paths use the BasicAuthExecutor.
		{
			"id":   "prompt_code",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_u2",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_p2",
							"identifier": "password",
							"type":       "PASSWORD_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "submit_code",
						"nextNode": "basic_auth_code",
					},
				},
			},
		},
		// Credentials prompt for the biometrics ACR path.
		// For simplicity in tests this path also uses the BasicAuthExecutor.
		{
			"id":   "prompt_bio",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_u3",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_p3",
							"identifier": "password",
							"type":       "PASSWORD_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "submit_bio",
						"nextNode": "basic_auth_bio",
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
			"id":   "basic_auth_bio",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "BasicAuthExecutor",
				"inputs": []map[string]interface{}{
					{"ref": "input_u3", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_p3", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
			},
			"onSuccess":    "auth_assert",
			"onIncomplete": "prompt_bio",
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

// acrOptionsTestApp is a minimal OAuth2 application used across ACR options tests.
var acrOptionsTestApp = testutils.Application{
	Name:                      "ACR Options Flow Test Application",
	Description:               "Application for testing login_options flow behaviour",
	IsRegistrationFlowEnabled: false,
	ClientID:                  "acr_options_flow_test_client",
	ClientSecret:              "acr_options_flow_test_secret",
	RedirectURIs:              []string{"https://localhost:3000/acr-options-callback"},
	AllowedUserTypes:          []string{"acr_options_test_person"},
	InboundAuthConfig: []map[string]interface{}{
		{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                "acr_options_flow_test_client",
				"clientSecret":            "acr_options_flow_test_secret",
				"redirectUris":            []string{"https://localhost:3000/acr-options-callback"},
				"grantTypes":              []string{"authorization_code"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"acrValues": []string{
					"urn:thunder:acr:password",
					"urn:thunder:acr:generated-code",
					"urn:thunder:acr:biometrics",
				},
			},
		},
	},
}

var acrOptionsTestOU = testutils.OrganizationUnit{
	Handle:      "acr-options-flow-test-ou",
	Name:        "ACR Options Flow Test Organization Unit",
	Description: "Organization unit for ACR options flow testing",
	Parent:      nil,
}

var acrOptionsUserType = testutils.UserType{
	Name: "acr_options_test_person",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
		"email":    map[string]interface{}{"type": "string"},
	},
}

var acrOptionsTestUser = testutils.User{
	Type: acrOptionsUserType.Name,
	Attributes: json.RawMessage(`{
		"username": "acroptionsuser",
		"password": "testpassword",
		"email": "acroptionsuser@example.com"
	}`),
}

var (
	acrOptionsTestAppID  string
	acrOptionsTestOUID   string
	acrOptionsFlowID     string
	acrOptionsUserTypeID string
)

// AcrOptionsFlowTestSuite tests the login_options PROMPT node filtering, ordering,
// auto-selection, AMR validation, and the ACR claim in the auth assertion JWT.
type AcrOptionsFlowTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig
}

func TestAcrOptionsFlowTestSuite(t *testing.T) {
	suite.Run(t, new(AcrOptionsFlowTestSuite))
}

func (ts *AcrOptionsFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(acrOptionsTestOU)
	ts.Require().NoError(err, "failed to create OU for ACR options tests")
	acrOptionsTestOUID = ouID

	acrOptionsUserType.OUID = acrOptionsTestOUID
	schemaID, err := testutils.CreateUserType(acrOptionsUserType)
	ts.Require().NoError(err, "failed to create user schema for ACR options tests")
	acrOptionsUserTypeID = schemaID

	user := acrOptionsTestUser
	user.OUID = acrOptionsTestOUID
	userIDs, err := testutils.CreateMultipleUsers(user)
	ts.Require().NoError(err, "failed to create test user for ACR options tests")
	ts.config.CreatedUserIDs = userIDs

	flowID, err := testutils.CreateFlow(acrOptionsFlow)
	ts.Require().NoError(err, "failed to create ACR options flow")
	acrOptionsFlowID = flowID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	acrOptionsTestApp.AuthFlowID = flowID
	acrOptionsTestApp.OUID = acrOptionsTestOUID
	appID, err := testutils.CreateApplication(acrOptionsTestApp)
	ts.Require().NoError(err, "failed to create ACR options test application")
	acrOptionsTestAppID = appID
}

func (ts *AcrOptionsFlowTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("failed to cleanup users: %v", err)
	}
	if acrOptionsTestAppID != "" {
		if err := testutils.DeleteApplication(acrOptionsTestAppID); err != nil {
			ts.T().Logf("failed to delete test application: %v", err)
		}
	}
	for _, id := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(id); err != nil {
			ts.T().Logf("failed to delete flow %s: %v", id, err)
		}
	}
	if acrOptionsUserTypeID != "" {
		if err := testutils.DeleteUserType(acrOptionsUserTypeID); err != nil {
			ts.T().Logf("failed to delete user schema: %v", err)
		}
	}
	if acrOptionsTestOUID != "" {
		if err := testutils.DeleteOrganizationUnit(acrOptionsTestOUID); err != nil {
			ts.T().Logf("failed to delete OU: %v", err)
		}
	}
}

func (ts *AcrOptionsFlowTestSuite) TestAcrOptions_NodeValidInFlowGraph() {
	flowStep, err := common.InitiateAuthenticationFlow(acrOptionsTestAppID, false, nil, "")
	ts.Require().NoError(err, "flow initiation should succeed")

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().Equal("VIEW", flowStep.Type)
	ts.Require().NotEmpty(flowStep.ExecutionID)

	ts.Require().NotEmpty(flowStep.Data.Actions, "login_options node should return actions")
	ts.Require().True(
		common.HasAction(flowStep.Data.Actions, "pwd_action"),
		"pwd_action should be present")
	ts.Require().True(
		common.HasAction(flowStep.Data.Actions, "code_action"),
		"code_action should be present")
}

func (ts *AcrOptionsFlowTestSuite) TestAcrOptions_FilteredToRequestedACR() {
	authID, flowID, err := ts.initiateAuthorizeAndExtract("urn:thunder:acr:password urn:thunder:acr:biometrics")
	ts.Require().NoError(err, "authorization initiation should succeed")
	ts.Require().NotEmpty(authID)
	ts.Require().NotEmpty(flowID)

	flowStep, err := common.ResumeFlow(flowID)
	ts.Require().NoError(err, "flow resumption should succeed")

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	actionRefs := ts.actionRefs(flowStep.Data.Actions)
	ts.Require().Contains(actionRefs, "pwd_action", "pwd_action should be present")
	ts.Require().Contains(actionRefs, "bio_action", "bio_action should be present")
	ts.Require().NotContains(actionRefs, "code_action", "code_action should be filtered out")
}

func (ts *AcrOptionsFlowTestSuite) TestAcrOptions_OrderedByPreference() {
	authID, flowID, err := ts.initiateAuthorizeAndExtract("urn:thunder:acr:generated-code urn:thunder:acr:password")
	ts.Require().NoError(err)
	ts.Require().NotEmpty(authID)
	ts.Require().NotEmpty(flowID)

	flowStep, err := common.ResumeFlow(flowID)
	ts.Require().NoError(err)

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().Len(flowStep.Data.Actions, 2, "both ACR actions should be present")

	ts.Require().Equal("code_action", flowStep.Data.Actions[0].Ref,
		"generated-code action should be first")
	ts.Require().Equal("pwd_action", flowStep.Data.Actions[1].Ref,
		"password action should be second")
}

func (ts *AcrOptionsFlowTestSuite) TestAcrOptions_AutoSelectsWhenSingleACR() {
	authID, flowID, err := ts.initiateAuthorizeAndExtract("urn:thunder:acr:password")
	ts.Require().NoError(err)
	ts.Require().NotEmpty(authID)
	ts.Require().NotEmpty(flowID)

	flowStep, err := common.ResumeFlow(flowID)
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	ts.Require().NotEmpty(flowStep.Data.Inputs, "credential inputs should be returned after auto-selection")
	ts.Require().True(
		common.HasInput(flowStep.Data.Inputs, "username"),
		"username input should be returned for the password path")
	ts.Require().True(
		common.HasInput(flowStep.Data.Inputs, "password"),
		"password input should be returned for the password path")
	ts.Require().False(
		common.HasAction(flowStep.Data.Actions, "pwd_action"),
		"chooser action must not be present after auto-selection")
	ts.Require().False(
		common.HasAction(flowStep.Data.Actions, "code_action"),
		"chooser action must not be present after auto-selection")
	ts.Require().False(
		common.HasAction(flowStep.Data.Actions, "bio_action"),
		"chooser action must not be present after auto-selection")
}

func (ts *AcrOptionsFlowTestSuite) TestAcrOptions_AcrInAuthAssertionJWT() {
	// Step 1: Start the flow without acr filtering so the chooser is shown.
	flowStep, err := common.InitiateAuthenticationFlow(acrOptionsTestAppID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().NotEmpty(flowStep.Data.Actions)

	// Step 2: Select the password ACR action.
	credStep, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{}, "pwd_action", flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", credStep.FlowStatus)
	ts.Require().NotEmpty(credStep.Data.Inputs, "credential inputs should be requested")

	// Step 3: Submit valid credentials.
	var userAttrs map[string]interface{}
	ts.Require().NoError(json.Unmarshal(acrOptionsTestUser.Attributes, &userAttrs))
	inputs := map[string]string{
		"username": userAttrs["username"].(string),
		"password": userAttrs["password"].(string),
	}
	completeStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "submit_pwd", credStep.ChallengeToken)
	ts.Require().NoError(err)

	ts.Require().Equal("COMPLETE", completeStep.FlowStatus, "flow should complete successfully")
	ts.Require().NotEmpty(completeStep.Assertion, "auth assertion JWT must be present")

	// Decode the assertion and verify the completed_auth_class claim is set.
	jwtClaims, err := testutils.DecodeJWT(completeStep.Assertion)
	ts.Require().NoError(err, "assertion JWT must be decodable")
	ts.Require().NotNil(jwtClaims)

	acrClaim, ok := jwtClaims.Additional["completed_auth_class"]
	ts.Require().True(ok, "completed_auth_class claim must be present in the auth assertion JWT")
	ts.Require().Equal("urn:thunder:acr:password", acrClaim,
		"completed_auth_class claim must reflect the selected authentication class")
}

func (ts *AcrOptionsFlowTestSuite) actionRefs(actions []common.Action) []string {
	refs := make([]string, len(actions))
	for i, a := range actions {
		refs[i] = a.Ref
	}
	return refs
}

// initiateAuthorizeAndExtract sends GET /oauth2/authorize with the given acr_values and
// returns the authID and flowID extracted from the redirect location.
func (ts *AcrOptionsFlowTestSuite) initiateAuthorizeAndExtract(acrValues string) (string, string, error) {
	params := url.Values{}
	params.Set("client_id", "acr_options_flow_test_client")
	params.Set("redirect_uri", "https://localhost:3000/acr-options-callback")
	params.Set("response_type", "code")
	params.Set("scope", "openid")
	params.Set("state", "acr_state")
	if acrValues != "" {
		params.Set("acr_values", acrValues)
	}

	resp, err := testutils.GetNoRedirectHTTPClient().Get(testutils.TestServerURL + "/oauth2/authorize?" + params.Encode())
	if err != nil {
		return "", "", fmt.Errorf("authorize request failed: %w", err)
	}
	defer resp.Body.Close()

	location := resp.Header.Get("Location")
	if location == "" {
		return "", "", fmt.Errorf("no Location header in authorize response (status %d)", resp.StatusCode)
	}

	authID, flowID, err := testutils.ExtractAuthData(location)
	if err != nil {
		return "", "", fmt.Errorf("failed to extract auth data from location %q: %w", location, err)
	}
	return authID, flowID, nil
}
