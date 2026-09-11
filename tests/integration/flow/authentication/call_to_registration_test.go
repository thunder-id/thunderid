// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// CallToRegistrationFlowTestSuite tests an AUTHENTICATION flow that presents a login-options
// screen, then invokes a REGISTRATION sub-flow via a CALL node when the user chooses to sign up.
// The callee suspends at a registration PROMPT; on resume, the user is provisioned, control
// returns to the caller, and an auth assertion is produced. An error path verifies that a
// provisioning failure routes to the caller's onFailure target without panicking.
type CallToRegistrationFlowTestSuite struct {
	suite.Suite
	config       *common.TestSuiteConfig
	ouID         string
	entityTypeID string
	appID        string
}

func TestCallToRegistrationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(CallToRegistrationFlowTestSuite))
}

func (ts *CallToRegistrationFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ou := testutils.OrganizationUnit{
		Handle:      "call-to-reg-test-ou",
		Name:        "Call To Registration Test OU",
		Description: "OU for CALL node auth→registration test",
		Parent:      nil,
	}
	ouID, err := testutils.CreateOrganizationUnit(ou)
	ts.Require().NoError(err, "Failed to create OU")
	ts.ouID = ouID

	userType := testutils.UserType{
		Name: "call-reg-user-type",
		OUID: ts.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type":   "string",
				"unique": true,
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			// The regex gives the error-path test a failure the provisioning executor cannot
			// recover from by re-prompting, which is what routes the CALL node to onFailure.
			"email": map[string]interface{}{
				"type":     "string",
				"required": true,
				"regex":    "^[a-zA-Z0-9._%+-]+@example\\.com$",
			},
		},
		AllowSelfRegistration: true,
	}
	entityTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err, "Failed to create user type")
	ts.entityTypeID = entityTypeID

	// Create the REGISTRATION callee flow: START → user_type_resolver → prompt_user_data → provision_user → END
	regFlow := testutils.Flow{
		Name:     "Call-To-Reg: Registration Callee Flow",
		FlowType: "REGISTRATION",
		Handle:   "call_to_reg_callee_flow",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "user_type_resolver",
			},
			{
				"id":   "user_type_resolver",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "UserTypeResolver",
				},
				"onSuccess":    "prompt_user_data",
				"onIncomplete": "prompt_user_data",
			},
			{
				"id":   "prompt_user_data",
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
								"ref":        "input_email",
								"identifier": "email",
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
							"nextNode": "provision_user",
						},
					},
				},
			},
			{
				"id":   "provision_user",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "ProvisioningExecutor",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_username",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_email",
							"identifier": "email",
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
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}
	regFlowID, err := testutils.CreateFlow(regFlow)
	ts.Require().NoError(err, "Failed to create registration callee flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, regFlowID)

	// Create the AUTHENTICATION caller flow:
	//   START → prompt_login_options (action_signup → call_registration)
	//         → call_registration CALL (onSuccess: auth_assert, onFailure: end)
	//         → auth_assert → end
	authFlow := testutils.Flow{
		Name:     "Call-To-Reg: Authentication Caller Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "call_to_reg_caller_auth_flow",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "prompt_login_options",
			},
			{
				"id":   "prompt_login_options",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"action": map[string]interface{}{
							"ref":      "action_signup",
							"nextNode": "call_registration",
						},
					},
				},
			},
			{
				"id":        "call_registration",
				"type":      "CALL",
				"flow":      map[string]interface{}{"ref": regFlowID},
				"onSuccess": "auth_assert",
				"onFailure": "end",
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
	ts.Require().NoError(err, "Failed to create authentication caller flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, authFlowID)

	app := testutils.Application{
		OUID:                      ts.ouID,
		Name:                      "Call-To-Registration Test App",
		Description:               "App for CALL node auth→registration testing",
		IsRegistrationFlowEnabled: true,
		AuthFlowID:                authFlowID,
		RegistrationFlowID:        regFlowID,
		ClientID:                  "call_to_reg_test_client",
		ClientSecret:              "call_to_reg_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{userType.Name},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId"},
		},
	}
	appID, err := testutils.CreateApplication(app)
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID
}

func (ts *CallToRegistrationFlowTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users: %v", err)
	}
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete test application: %v", err)
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

func (ts *CallToRegistrationFlowTestSuite) TestCallToRegistration() {
	err := common.UpdateAppConfig(ts.appID, ts.config.CreatedFlowIDs[1], ts.config.CreatedFlowIDs[0])
	ts.Require().NoError(err)

	// Step 1: Initiate — caller suspends at the login-options screen
	optionsStep, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().Equal("INCOMPLETE", optionsStep.FlowStatus)
	ts.Require().Equal("VIEW", optionsStep.Type)
	ts.Require().NotEmpty(optionsStep.ExecutionID)

	// Step 2: Trigger "Sign up" — CALL fires, callee suspends at the registration data PROMPT
	calleeStep, err := common.CompleteFlow(optionsStep.ExecutionID, map[string]string{},
		"action_signup", optionsStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to trigger sign-up action")
	ts.Require().Equal("INCOMPLETE", calleeStep.FlowStatus, "Expected INCOMPLETE at callee registration prompt")
	ts.Require().Equal("VIEW", calleeStep.Type)
	ts.Require().True(common.HasInput(calleeStep.Data.Inputs, "username"), "username input expected")
	ts.Require().True(common.HasInput(calleeStep.Data.Inputs, "email"), "email input expected")
	ts.Require().True(common.HasInput(calleeStep.Data.Inputs, "password"), "password input expected")

	// Step 3: Submit registration data — callee provisions user, hits END, caller resumes,
	// AuthAssertExecutor runs, flow completes with assertion
	inputs := map[string]string{
		"username": "call_reg_new_user_001",
		"email":    "call_reg_new_user_001@example.com",
		"password": "Secure@1234",
	}
	completeStep, err := common.CompleteFlow(calleeStep.ExecutionID, inputs, "action_submit",
		calleeStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete registration flow")
	ts.Require().Equal("COMPLETE", completeStep.FlowStatus, "Expected COMPLETE after callee return")
	ts.Require().NotEmpty(completeStep.Assertion, "JWT assertion expected on completion")
	ts.Require().Nil(completeStep.Error)

	// Track the user provisioned by the callee so TearDownSuite can clean it up
	provisioned, err := testutils.FindUserByAttribute("username", "call_reg_new_user_001")
	if err == nil && provisioned != nil && provisioned.ID != "" {
		ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, provisioned.ID)
	}
}

func (ts *CallToRegistrationFlowTestSuite) TestCallToRegistration_CalleeFails_RoutesToOnFailure() {
	err := common.UpdateAppConfig(ts.appID, ts.config.CreatedFlowIDs[1], ts.config.CreatedFlowIDs[0])
	ts.Require().NoError(err)

	// Step 1: Initiate — caller suspends at the login-options screen
	optionsStep, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", optionsStep.FlowStatus)

	// Step 2: Trigger "Sign up" — callee suspends at the registration data PROMPT
	calleeStep, err := common.CompleteFlow(optionsStep.ExecutionID, map[string]string{},
		"action_signup", optionsStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", calleeStep.FlowStatus)

	// Step 3: Submit an email the schema regex rejects — provisioning executor fails, engine pops
	// the frame and routes to caller CALL node's onFailure (END). Flow ends without auth assertion.
	// A schema violation is used rather than a duplicate unique attribute because the provisioning
	// executor reports a duplicate as recoverable and re-prompts for it instead of failing.
	inputs := map[string]string{
		"username": "call_reg_invalid_email_user",
		"email":    "not-a-valid-email",
		"password": "Secure@1234",
	}
	failStep, err := common.CompleteFlow(calleeStep.ExecutionID, inputs, "action_submit",
		calleeStep.ChallengeToken)
	ts.Require().NoError(err, "Expected graceful handling of callee failure, not a transport error")
	ts.Require().Equal("COMPLETE", failStep.FlowStatus, "Flow should complete via onFailure path")
	ts.Require().Empty(failStep.Assertion, "No assertion expected — auth_assert was not reached")
}
