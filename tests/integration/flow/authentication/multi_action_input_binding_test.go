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

package authentication

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	mockMultiActionGooglePort = 8099
)

var (
	multiActionInputBindingFlow = testutils.Flow{
		Name:     "Multi Action Input Binding Test Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "multi_action_input_binding_test",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "prompt_choice",
			},
			{
				"id":   "prompt_choice",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_001",
								"identifier": "username",
								"type":       "string",
								"required":   true,
							},
							{
								"ref":        "input_002",
								"identifier": "password",
								"type":       "string",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_basic",
							"nextNode": "basic_auth",
						},
					},
					{
						"action": map[string]interface{}{
							"ref":      "action_google",
							"nextNode": "google_auth",
						},
					},
				},
			},
			{
				"id":   "basic_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "BasicAuthExecutor",
				},
				"onSuccess": "auth_assert",
			},
			{
				"id":   "google_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId": "placeholder-idp-id",
				},
				"executor": map[string]interface{}{
					"name": "GoogleOIDCAuthExecutor",
				},
				"onSuccess": "auth_assert",
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

	multiActionInputBindingTestApp = testutils.Application{
		Name:                      "Multi Action Input Binding Test Application",
		Description:               "Application for testing prompts-based input-action binding",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "multi_action_input_binding_test_client",
		ClientSecret:              "multi_action_input_binding_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"multi_action_input_binding_test_person"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}

	multiActionInputBindingTestOU = testutils.OrganizationUnit{
		Handle:      "multi-action-input-binding-test-ou",
		Name:        "Multi Action Input Binding Test Organization Unit",
		Description: "Organization unit for multi action input binding testing",
		Parent:      nil,
	}

	multiActionInputBindingEntityType = testutils.UserType{
		Name: "multi_action_input_binding_test_person",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"email": map[string]interface{}{
				"type": "string",
			},
			"sub": map[string]interface{}{
				"type": "string",
			},
			"givenName": map[string]interface{}{
				"type": "string",
			},
			"familyName": map[string]interface{}{
				"type": "string",
			},
		},
	}

	testUserMultiActionInputBinding = testutils.User{
		Type: multiActionInputBindingEntityType.Name,
		Attributes: json.RawMessage(`{
			"username": "multiactionuser",
			"password": "testpassword",
			"email": "multiactionuser@example.com",
			"sub": "google-multi-action-user-123",
			"givenName": "Multi",
			"familyName": "Action"
		}`),
	}
)

var (
	multiActionInputBindingTestAppID    string
	multiActionInputBindingTestOUID     string
	multiActionInputBindingEntityTypeID string
)

type MultiActionInputBindingTestSuite struct {
	suite.Suite
	config           *common.TestSuiteConfig
	mockGoogleServer *testutils.MockGoogleOIDCServer
}

func TestMultiActionInputBindingTestSuite(t *testing.T) {
	suite.Run(t, new(MultiActionInputBindingTestSuite))
}

func (ts *MultiActionInputBindingTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	// Start mock Google server
	mockServer, err := testutils.NewMockGoogleOIDCServer(mockMultiActionGooglePort,
		"test_multi_action_google_client", "test_multi_action_google_secret")
	ts.Require().NoError(err, "Failed to create mock Google server")
	ts.mockGoogleServer = mockServer

	ts.mockGoogleServer.AddUser(&testutils.GoogleUserInfo{
		Sub:           "google-multi-action-user-123",
		Email:         "multiactionuser@example.com",
		EmailVerified: true,
		Name:          "Multi Action",
		GivenName:     "Multi",
		FamilyName:    "Action",
		Picture:       "https://example.com/picture.jpg",
		Locale:        "en",
	})

	err = ts.mockGoogleServer.Start()
	ts.Require().NoError(err, "Failed to start mock Google server")

	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(multiActionInputBindingTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit: %v", err)
	}
	multiActionInputBindingTestOUID = ouID

	// Create test user type within the OU
	multiActionInputBindingEntityType.OUID = multiActionInputBindingTestOUID
	schemaID, err := testutils.CreateUserType(multiActionInputBindingEntityType)
	if err != nil {
		ts.T().Fatalf("Failed to create test user type: %v", err)
	}
	multiActionInputBindingEntityTypeID = schemaID

	// Create test user with the created OU
	testUser := testUserMultiActionInputBinding
	testUser.OUID = multiActionInputBindingTestOUID
	userID, err := testutils.CreateUser(testUser)
	if err != nil {
		ts.T().Fatalf("Failed to create test user: %v", err)
	}
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, userID)

	// Create Google IDP
	googleIDP := testutils.IDP{
		Name:        "Multi Action Test Google IDP",
		Description: "Google IDP for multi action input binding test",
		Type:        "GOOGLE",
		Properties: []testutils.IDPProperty{
			{Name: "client_id", Value: "test_multi_action_google_client", IsSecret: false},
			{Name: "client_secret", Value: "test_multi_action_google_secret", IsSecret: true},
			{Name: "redirect_uri", Value: "http://localhost:3000/callback", IsSecret: false},
			{Name: "scopes", Value: "openid email profile", IsSecret: false},
			{Name: "authorization_endpoint", Value: ts.mockGoogleServer.GetURL() + "/o/oauth2/v2/auth", IsSecret: false},
			{Name: "token_endpoint", Value: ts.mockGoogleServer.GetURL() + "/token", IsSecret: false},
			{Name: "userinfo_endpoint", Value: ts.mockGoogleServer.GetURL() + "/v1/userinfo", IsSecret: false},
			{Name: "jwks_endpoint", Value: ts.mockGoogleServer.GetURL() + "/oauth2/v3/certs", IsSecret: false},
		},
	}

	idpID, err := testutils.CreateIDP(googleIDP)
	ts.Require().NoError(err, "Failed to create Google IDP")
	ts.config.CreatedIdpIDs = append(ts.config.CreatedIdpIDs, idpID)

	// Update flow definition with created IDP ID
	nodes := multiActionInputBindingFlow.Nodes.([]map[string]interface{})
	nodes[3]["properties"].(map[string]interface{})["idpId"] = idpID
	multiActionInputBindingFlow.Nodes = nodes

	// Create flow
	flowID, err := testutils.CreateFlow(multiActionInputBindingFlow)
	ts.Require().NoError(err, "Failed to create multi action input binding flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	multiActionInputBindingTestApp.AuthFlowID = flowID

	// Create test application
	multiActionInputBindingTestApp.OUID = multiActionInputBindingTestOUID
	appID, err := testutils.CreateApplication(multiActionInputBindingTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application: %v", err)
	}
	multiActionInputBindingTestAppID = appID
}

func (ts *MultiActionInputBindingTestSuite) TearDownSuite() {
	// Delete test users
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users: %v", err)
	}

	// Delete test flows
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete flow %s: %v", flowID, err)
		}
	}

	// Delete test IDPs
	for _, idpID := range ts.config.CreatedIdpIDs {
		if err := testutils.DeleteIDP(idpID); err != nil {
			ts.T().Logf("Failed to delete IDP %s: %v", idpID, err)
		}
	}

	// Delete test application
	if multiActionInputBindingTestAppID != "" {
		if err := testutils.DeleteApplication(multiActionInputBindingTestAppID); err != nil {
			ts.T().Logf("Failed to delete test application: %v", err)
		}
	}

	// Delete test user type
	if multiActionInputBindingEntityTypeID != "" {
		if err := testutils.DeleteUserType(multiActionInputBindingEntityTypeID); err != nil {
			ts.T().Logf("Failed to delete user type: %v", err)
		}
	}

	// Delete test organization unit
	if multiActionInputBindingTestOUID != "" {
		if err := testutils.DeleteOrganizationUnit(multiActionInputBindingTestOUID); err != nil {
			ts.T().Logf("Failed to delete organization unit: %v", err)
		}
	}

	// Stop mock Google server
	if ts.mockGoogleServer != nil {
		_ = ts.mockGoogleServer.Stop()
		time.Sleep(200 * time.Millisecond)
	}
}

func (ts *MultiActionInputBindingTestSuite) TestInitiateFlow_ShowsAllActions() {
	flowStep, err := common.InitiateAuthenticationFlow(multiActionInputBindingTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	ts.Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Equal("VIEW", flowStep.Type)
	ts.Len(flowStep.Data.Actions, 2, "Should have 2 actions available")
	ts.Len(flowStep.Data.Inputs, 2, "Should have all inputs from prompts available for rendering")

	actionRefs := make([]string, len(flowStep.Data.Actions))
	for i, action := range flowStep.Data.Actions {
		actionRefs[i] = action.Ref
	}
	ts.Contains(actionRefs, "action_basic")
	ts.Contains(actionRefs, "action_google")

	inputIdentifiers := make([]string, len(flowStep.Data.Inputs))
	for i, input := range flowStep.Data.Inputs {
		inputIdentifiers[i] = input.Identifier
	}
	ts.Contains(inputIdentifiers, "username")
	ts.Contains(inputIdentifiers, "password")
}

func (ts *MultiActionInputBindingTestSuite) TestSelectActionWithoutInputs_ShouldRedirectToGoogle() {
	flowStep, err := common.InitiateAuthenticationFlow(multiActionInputBindingTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().NotEmpty(flowStep.ExecutionID)

	// Select action_google which has no inputs - should redirect to Google
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, nil, "action_google", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete flow with action_google")

	// Should redirect to Google OAuth
	ts.Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Equal("REDIRECTION", flowStep.Type)
	ts.NotEmpty(flowStep.Data.RedirectURL, "Should have redirect URL for Google auth")
}

func (ts *MultiActionInputBindingTestSuite) TestSelectActionWithInputs_ShouldPromptForInputs() {
	flowStep, err := common.InitiateAuthenticationFlow(multiActionInputBindingTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().NotEmpty(flowStep.ExecutionID)

	// Select action_basic which requires username and password
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, nil, "action_basic", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete flow with action_basic")

	ts.Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Equal("VIEW", flowStep.Type)
	ts.Len(flowStep.Data.Inputs, 2, "Should prompt for 2 inputs")

	inputIdentifiers := make([]string, len(flowStep.Data.Inputs))
	for i, input := range flowStep.Data.Inputs {
		inputIdentifiers[i] = input.Identifier
	}
	ts.Contains(inputIdentifiers, "username")
	ts.Contains(inputIdentifiers, "password")
}

func (ts *MultiActionInputBindingTestSuite) TestSelectActionWithInputsProvided_ShouldComplete() {
	flowStep, err := common.InitiateAuthenticationFlow(multiActionInputBindingTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().NotEmpty(flowStep.ExecutionID)

	// Select action_basic
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, nil, "action_basic", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to select action_basic")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	// Provide username and password
	inputs := map[string]string{
		"username": "multiactionuser",
		"password": "testpassword",
	}
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete flow with inputs")

	ts.Equal("COMPLETE", flowStep.FlowStatus, "Flow should complete after providing required inputs")
	ts.NotEmpty(flowStep.Assertion, "Should have assertion token")
}

func (ts *MultiActionInputBindingTestSuite) TestGoogleAuthFlowComplete() {
	flowStep, err := common.InitiateAuthenticationFlow(multiActionInputBindingTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	ExecutionID := flowStep.ExecutionID

	// Select action_google
	flowStep, err = common.CompleteFlow(ExecutionID, nil, "action_google", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to select action_google")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().Equal("REDIRECTION", flowStep.Type)

	redirectURL := flowStep.Data.RedirectURL
	ts.Require().NotEmpty(redirectURL)

	// Simulate Google OAuth flow
	authCode, state, err := testutils.SimulateFederatedOAuthFlow(redirectURL)
	ts.Require().NoError(err, "Failed to simulate Google authorization")
	ts.Require().NotEmpty(authCode)

	// Complete flow with authorization code
	inputs := map[string]string{"code": authCode, "state": state}
	flowStep, err = common.CompleteFlow(ExecutionID, inputs, "", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete flow with auth code")

	ts.Equal("COMPLETE", flowStep.FlowStatus)
	ts.NotEmpty(flowStep.Assertion, "Should have assertion token")
}
