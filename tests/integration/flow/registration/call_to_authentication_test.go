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

package registration

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// CallToAuthenticationFlowTestSuite tests a REGISTRATION flow that presents an initial
// options screen, then invokes an AUTHENTICATION sub-flow via a CALL node when the user
// chooses to log in with an existing account. The callee suspends at a credentials PROMPT;
// on resume with valid credentials the callee completes and control returns to the caller.
type CallToAuthenticationFlowTestSuite struct {
	suite.Suite
	config       *common.TestSuiteConfig
	ouID         string
	entityTypeID string
	appID        string
}

func TestCallToAuthenticationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(CallToAuthenticationFlowTestSuite))
}

func (ts *CallToAuthenticationFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ou := testutils.OrganizationUnit{
		Handle:      "call-to-auth-reg-test-ou",
		Name:        "Call To Auth Reg Test OU",
		Description: "OU for CALL node registration→auth test",
		Parent:      nil,
	}
	ouID, err := testutils.CreateOrganizationUnit(ou)
	ts.Require().NoError(err)
	ts.ouID = ouID

	userType := testutils.UserType{
		Name: "call-auth-user-type",
		OUID: ts.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"email": map[string]interface{}{
				"type":     "string",
				"required": true,
			},
		},
		AllowSelfRegistration: true,
	}
	entityTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err)
	ts.entityTypeID = entityTypeID

	// Pre-create a user to authenticate against in the happy-path test
	user := testutils.User{
		Type: userType.Name,
		OUID: ts.ouID,
		Attributes: []byte(`{
			"username": "call_auth_existing_user",
			"password": "Existing@1234",
			"email": "call_auth_existing@example.com"
		}`),
	}
	userIDs, err := testutils.CreateMultipleUsers(user)
	ts.Require().NoError(err)
	ts.config.CreatedUserIDs = userIDs

	// Create the AUTHENTICATION callee flow: START → prompt_credentials → credentials_auth → auth_assert → END.
	authFlow := testutils.Flow{
		Name:     "Call-Auth: Authentication Callee Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "call_auth_callee_flow",
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
							{
								"ref":        "input_username",
								"identifier": "username",
								"type":       "TEXT_INPUT",
								"required":   true,
							},
							{
								"ref":        "input_password",
								"identifier": "password",
								"type":       "PASSWORD_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_submit",
							"nextNode": "credentials_auth",
						},
					},
				},
			},
			{
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialsAuthExecutor",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_username",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_password",
							"identifier": "password",
							"type":       "PASSWORD_INPUT",
							"required":   true,
						},
					},
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
	authFlowID, err := testutils.CreateFlow(authFlow)
	ts.Require().NoError(err, "Failed to create auth callee flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, authFlowID)

	// Create the REGISTRATION caller flow:
	//   START → user_type_resolver → prompt_initial (action_login → call_auth)
	//         → call_auth CALL (onSuccess: end, onFailure: provisioning → end)
	// ProvisioningExecutor is on the failure path so the happy path completes after CALL.
	regFlow := testutils.Flow{
		Name:     "Call-Auth: Registration Caller Flow",
		FlowType: "REGISTRATION",
		Handle:   "call_auth_caller_reg_flow",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "prompt_initial",
			},
			{
				"id":   "prompt_initial",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"action": map[string]interface{}{
							"ref":      "action_login",
							"nextNode": "call_auth",
						},
					},
					{
						"action": map[string]interface{}{
							"ref":      "action_register",
							"nextNode": "user_type_resolver",
						},
					},
				},
			},
			{
				"id":        "call_auth",
				"type":      "CALL",
				"flow":      map[string]interface{}{"ref": authFlowID},
				"onSuccess": "end",
				"onFailure": "end",
			},
			{
				"id":   "user_type_resolver",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "UserTypeResolver",
				},
				"onSuccess":    "prompt_credentials",
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_username",
								"identifier": "username",
								"type":       "TEXT_INPUT",
								"required":   true,
							},
							{
								"ref":        "input_password",
								"identifier": "password",
								"type":       "PASSWORD_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_submit_credentials",
							"nextNode": "credentials_auth",
						},
					},
				},
			},
			{
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialsAuthExecutor",
				},
				"onSuccess": "provisioning",
			},
			{
				"id":   "provisioning",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "ProvisioningExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}
	regFlowID, err := testutils.CreateFlow(regFlow)
	ts.Require().NoError(err, "Failed to create registration caller flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, regFlowID)

	app := testutils.Application{
		OUID:                      ts.ouID,
		Name:                      "Call-To-Auth Registration Test App",
		Description:               "App for CALL node registration→auth testing",
		IsRegistrationFlowEnabled: true,
		AuthFlowID:                authFlowID,
		RegistrationFlowID:        regFlowID,
		ClientID:                  "call_to_auth_reg_test_client",
		ClientSecret:              "call_to_auth_reg_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{userType.Name},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId"},
		},
	}
	appID, err := testutils.CreateApplication(app)
	ts.Require().NoError(err)
	ts.appID = appID
}

func (ts *CallToAuthenticationFlowTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users: %v", err)
	}
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete application: %v", err)
		}
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete flow %s: %v", flowID, err)
		}
	}
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete OU: %v", err)
		}
	}
}

func (ts *CallToAuthenticationFlowTestSuite) TestCallToAuthentication() {
	err := common.UpdateAppConfig(ts.appID, ts.config.CreatedFlowIDs[0], ts.config.CreatedFlowIDs[1])
	ts.Require().NoError(err)

	// Step 1: Initiate registration — caller suspends at the initial options screen
	optionsStep, err := common.InitiateRegistrationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", optionsStep.FlowStatus, "Expected INCOMPLETE at initial options prompt")
	ts.Require().Equal("VIEW", optionsStep.Type)
	ts.Require().NotEmpty(optionsStep.ExecutionID)

	// Step 2: Trigger "Log in" — CALL fires, callee suspends at the credentials PROMPT
	calleeStep, err := common.CompleteFlow(optionsStep.ExecutionID, map[string]string{},
		"action_login", optionsStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to trigger login action")
	ts.Require().Equal("INCOMPLETE", calleeStep.FlowStatus, "Expected INCOMPLETE at callee credentials prompt")
	ts.Require().Equal("VIEW", calleeStep.Type)
	ts.Require().True(common.HasInput(calleeStep.Data.Inputs, "username"))
	ts.Require().True(common.HasInput(calleeStep.Data.Inputs, "password"))

	// Step 3: Submit valid credentials — callee authenticates, hits END, control returns to
	// caller, caller END reached, flow completes
	inputs := map[string]string{
		"username": "call_auth_existing_user",
		"password": "Existing@1234",
	}
	completeStep, err := common.CompleteFlow(calleeStep.ExecutionID, inputs, "action_submit",
		calleeStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", completeStep.FlowStatus, "Expected COMPLETE after callee return")
	ts.Require().Nil(completeStep.Error)
}
