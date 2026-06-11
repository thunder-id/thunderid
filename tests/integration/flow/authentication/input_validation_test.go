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
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	validationEmailRegexPattern = `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	validationEmailInvalidMsg   = "validation.email.invalid"
	validationPasswordMinMsg    = "validation.password.minLength"
	validationPasswordRegexMsg  = "validation.password.complexity"
	validationPasswordRegex     = `[0-9]`
)

var (
	validationTestOU = testutils.OrganizationUnit{
		Handle:      "input_validation_flow_test_ou",
		Name:        "Test OU for Input Validation Flow",
		Description: "Organization unit created for input validation flow testing",
		Parent:      nil,
	}

	validationFlow = testutils.Flow{
		Name:     "Input Validation Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_input_validation_test",
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
								"ref":        "input_001",
								"identifier": "username",
								"type":       "TEXT_INPUT",
								"required":   true,
								"validation": []map[string]interface{}{
									{
										"type":    "regex",
										"value":   validationEmailRegexPattern,
										"message": validationEmailInvalidMsg,
									},
								},
							},
							{
								"ref":        "input_002",
								"identifier": "password",
								"type":       "PASSWORD_INPUT",
								"required":   true,
								"validation": []map[string]interface{}{
									{
										"type":    "minLength",
										"value":   8,
										"message": validationPasswordMinMsg,
									},
									{
										"type":    "regex",
										"value":   validationPasswordRegex,
										"message": validationPasswordRegexMsg,
									},
								},
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_001",
							"nextNode": "basic_auth",
						},
					},
				},
			},
			{
				"id":   "basic_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "BasicAuthExecutor",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_001",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_002",
							"identifier": "password",
							"type":       "PASSWORD_INPUT",
							"required":   true,
						},
					},
				},
				"onSuccess":    "auth_assert",
				"onIncomplete": "prompt_credentials",
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

	validationTestApp = testutils.Application{
		Name:                      "Input Validation Test Application",
		Description:               "Application for testing input validation flows",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "input_validation_test_client",
		ClientSecret:              "input_validation_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"input_validation_user"},
	}

	validationTestUserType = testutils.UserType{
		Name: "input_validation_user",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	}

	validationTestUser = testutils.User{
		Type: validationTestUserType.Name,
		Attributes: json.RawMessage(`{
			"username": "validuser@example.com",
			"password": "Validpass1"
		}`),
	}
)

type InputValidationFlowTestSuite struct {
	suite.Suite
	config       *common.TestSuiteConfig
	ouID         string
	appID        string
	entityTypeID string
}

func TestInputValidationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(InputValidationFlowTestSuite))
}

func (ts *InputValidationFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(validationTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	validationTestUserType.OUID = ts.ouID
	schemaID, err := testutils.CreateUserType(validationTestUserType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = schemaID

	flowID, err := testutils.CreateFlow(validationFlow)
	ts.Require().NoError(err, "Failed to create input validation test flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	validationTestApp.AuthFlowID = flowID

	validationTestApp.OUID = ts.ouID
	appID, err := testutils.CreateApplication(validationTestApp)
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID

	testUser := validationTestUser
	testUser.OUID = ts.ouID
	userIDs, err := testutils.CreateMultipleUsers(testUser)
	ts.Require().NoError(err, "Failed to create test user")
	ts.config.CreatedUserIDs = userIDs
}

func (ts *InputValidationFlowTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users during teardown: %v", err)
	}
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow %s during teardown: %v", flowID, err)
		}
	}
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// TestInitialPromptReturnsValidationRules tests that validation rules are returned in
// `data.inputs[].validation` in the initial prompt response for API-only clients.
func (ts *InputValidationFlowTestSuite) TestInitialPromptReturnsValidationRules() {
	err := common.UpdateAppConfig(ts.appID, ts.config.CreatedFlowIDs[0], "")
	ts.Require().NoError(err, "App config update should succeed")

	flowStep, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().NotEmpty(flowStep.Data.Inputs)

	usernameInput := findInput(flowStep.Data.Inputs, "username")
	ts.Require().NotNil(usernameInput, "username input should be present")
	ts.Require().Len(usernameInput.Validation, 1, "username should have 1 validation rule")
	ts.Equal("regex", usernameInput.Validation[0].Type)
	ts.Equal(validationEmailInvalidMsg, usernameInput.Validation[0].Message)

	passwordInput := findInput(flowStep.Data.Inputs, "password")
	ts.Require().NotNil(passwordInput, "password input should be present")
	ts.Require().Len(passwordInput.Validation, 2, "password should have 2 validation rules")
	ts.Equal("minLength", passwordInput.Validation[0].Type)
	ts.Equal("regex", passwordInput.Validation[1].Type)
}

// TestInvalidInputReturnsFieldErrors tests that when validation fails, the server returns
// `flowStatus: INCOMPLETE` and `data.fieldErrors` with one entry per failed rule.
func (ts *InputValidationFlowTestSuite) TestInvalidInputReturnsFieldErrors() {
	err := common.UpdateAppConfig(ts.appID, ts.config.CreatedFlowIDs[0], "")
	ts.Require().NoError(err, "App config update should succeed")

	flowStep, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	resp, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"username": "not-an-email",
		"password": "abc",
	}, "action_001", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit invalid inputs")

	ts.Equal("INCOMPLETE", resp.FlowStatus, "Expected INCOMPLETE on validation failure")
	ts.Equal("VIEW", resp.Type, "Expected VIEW for reprompt")
	ts.Empty(resp.Assertion, "No assertion on validation failure")
	ts.Nil(resp.Error, "Error should not be set for input validation failures")

	ts.Require().NotEmpty(resp.Data.FieldErrors, "fieldErrors should be populated")
	usernameErrors := filterFieldErrors(resp.Data.FieldErrors, "username")
	passwordErrors := filterFieldErrors(resp.Data.FieldErrors, "password")
	ts.Len(usernameErrors, 1, "username should have 1 validation error")
	ts.Equal(validationEmailInvalidMsg, usernameErrors[0].Message)
	ts.Len(passwordErrors, 2, "password should have 2 validation errors (minLength and regex)")
}

// TestValidationFailureRePromptsInitialFieldSet tests that on validation failure the
// response re-prompts the same field set the user saw initially (preserving form
// structure) with errors attached only to the failing fields, and returns the action
// so the form can be re-submitted.
func (ts *InputValidationFlowTestSuite) TestValidationFailureRePromptsInitialFieldSet() {
	err := common.UpdateAppConfig(ts.appID, ts.config.CreatedFlowIDs[0], "")
	ts.Require().NoError(err, "App config update should succeed")

	flowStep, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	// Submit with a valid username and an invalid password. The response should
	// re-prompt the same form (both fields) with the error attached to the password.
	resp, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"username": "validuser@example.com",
		"password": "abc",
	}, "action_001", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit invalid inputs")

	ts.Equal("INCOMPLETE", resp.FlowStatus)
	ts.Equal("VIEW", resp.Type)

	// fieldErrors should mention only the failing field.
	usernameErrors := filterFieldErrors(resp.Data.FieldErrors, "username")
	passwordErrors := filterFieldErrors(resp.Data.FieldErrors, "password")
	ts.Empty(usernameErrors, "username passed; should have no field errors")
	ts.NotEmpty(passwordErrors, "password failed; should have field errors")

	// The same field set the user saw initially should be re-prompted.
	ts.NotNil(findInput(resp.Data.Inputs, "username"), "username (in initial prompt) should be re-prompted")
	ts.NotNil(findInput(resp.Data.Inputs, "password"), "password (in initial prompt) should be re-prompted")
	ts.Len(resp.Data.Inputs, 2, "both initially-prompted inputs should be re-prompted")

	// The action should also be re-returned so the submit button can be re-rendered.
	ts.NotEmpty(resp.Data.Actions, "actions should be re-prompted")
}

// TestMultipleRulesPerFieldProduceSeparateEntries tests that when two rules fail on the
// same field, each produces a separate `fieldErrors` entry.
func (ts *InputValidationFlowTestSuite) TestMultipleRulesPerFieldProduceSeparateEntries() {
	err := common.UpdateAppConfig(ts.appID, ts.config.CreatedFlowIDs[0], "")
	ts.Require().NoError(err, "App config update should succeed")

	flowStep, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	resp, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"username": "validuser@example.com",
		"password": "abc",
	}, "action_001", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit invalid inputs")

	ts.Equal("INCOMPLETE", resp.FlowStatus)
	passwordErrors := filterFieldErrors(resp.Data.FieldErrors, "password")
	ts.Require().Len(passwordErrors, 2, "two failing rules should produce two entries")
	ts.Equal(validationPasswordMinMsg, passwordErrors[0].Message)
	ts.Equal(validationPasswordRegexMsg, passwordErrors[1].Message)
}

// TestValidSubmissionAdvancesFlow tests the happy path: valid inputs pass validation,
// the flow advances, and `data.fieldErrors` is not present in the response.
func (ts *InputValidationFlowTestSuite) TestValidSubmissionAdvancesFlow() {
	err := common.UpdateAppConfig(ts.appID, ts.config.CreatedFlowIDs[0], "")
	ts.Require().NoError(err, "App config update should succeed")

	flowStep, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	resp, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"username": "validuser@example.com",
		"password": "Validpass1",
	}, "action_001", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit valid inputs")

	ts.Equal("COMPLETE", resp.FlowStatus, "Expected COMPLETE for valid submission")
	ts.NotEmpty(resp.Assertion, "JWT assertion should be issued on successful auth")
	ts.Empty(resp.Data.FieldErrors, "fieldErrors should be absent when all rules pass")
}

// TestValidationRunsAfterRequiredCheck tests that validation runs only after the
// required-input presence check passes. A submission missing a required field must
// return the existing missing-input response, not a validation error.
func (ts *InputValidationFlowTestSuite) TestValidationRunsAfterRequiredCheck() {
	err := common.UpdateAppConfig(ts.appID, ts.config.CreatedFlowIDs[0], "")
	ts.Require().NoError(err, "App config update should succeed")

	flowStep, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	resp, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"username": "validuser@example.com",
	}, "action_001", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit partial inputs")

	ts.Equal("INCOMPLETE", resp.FlowStatus, "Expected INCOMPLETE for missing required input")
	ts.Empty(resp.Data.FieldErrors, "fieldErrors should NOT be set when a required field is missing")
	ts.NotEmpty(resp.Data.Inputs, "Missing required input should be re-prompted")
}

// findInput returns the first input in inputs matching identifier, or nil if absent.
func findInput(inputs []common.Inputs, identifier string) *common.Inputs {
	for i := range inputs {
		if inputs[i].Identifier == identifier {
			return &inputs[i]
		}
	}
	return nil
}

// filterFieldErrors returns all field errors targeting the given identifier.
func filterFieldErrors(errs []common.FieldError, identifier string) []common.FieldError {
	var out []common.FieldError
	for _, e := range errs {
		if e.Identifier == identifier {
			out = append(out, e)
		}
	}
	return out
}
