// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

var (
	ctTokenTestOU = testutils.OrganizationUnit{
		Handle:      "ct_token_test_ou",
		Name:        "Test OU for Challenge Token Flow",
		Description: "Organization unit created for challenge token flow testing",
		Parent:      nil,
	}

	ctTokenTestFlow = testutils.Flow{
		Name:     "Challenge Token Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_ct_token_test",
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
							},
							{
								"ref":        "input_002",
								"identifier": "password",
								"type":       "PASSWORD_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_001",
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

	ctTokenEntityType = testutils.UserType{
		Handle:      "ct_token_test_user",
		DisplayName: "Ct Token Test User",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
		},
	}

	ctTokenTestUser = testutils.User{
		Type: ctTokenEntityType.Handle,
		Attributes: json.RawMessage(`{
			"username": "ct_token_testuser",
			"password": "testpassword"
		}`),
	}
)

var (
	ctTokenTestAppID    string
	ctTokenEntityTypeID string
)

type ChallengeTokenTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig
	ouID   string
}

func TestChallengeTokenTestSuite(t *testing.T) {
	suite.Run(t, new(ChallengeTokenTestSuite))
}

func (ts *ChallengeTokenTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(ctTokenTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	ctTokenEntityType.OUID = ts.ouID
	schemaID, err := testutils.CreateUserType(ctTokenEntityType)
	ts.Require().NoError(err, "Failed to create test user type")
	ctTokenEntityTypeID = schemaID

	flowID, err := testutils.CreateFlow(ctTokenTestFlow)
	ts.Require().NoError(err, "Failed to create challenge token test flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	app := testutils.Application{
		Name:                      "Challenge Token Test App",
		Description:               "Application for testing challenge token flows",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "ct_token_test_client",
		ClientSecret:              "ct_token_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{ctTokenEntityType.Handle},
		AuthFlowID:                flowID,
		OUID:                      ts.ouID,
	}
	appID, err := testutils.CreateApplication(app)
	ts.Require().NoError(err, "Failed to create test application")
	ctTokenTestAppID = appID

	user := ctTokenTestUser
	user.OUID = ts.ouID
	userIDs, err := testutils.CreateMultipleUsers(user)
	ts.Require().NoError(err, "Failed to create test user")
	ts.config.CreatedUserIDs = userIDs
}

func (ts *ChallengeTokenTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users: %v", err)
	}
	if ctTokenTestAppID != "" {
		if err := testutils.DeleteApplication(ctTokenTestAppID); err != nil {
			ts.T().Logf("Failed to delete test application: %v", err)
		}
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow %s: %v", flowID, err)
		}
	}
	if ctTokenEntityTypeID != "" {
		if err := testutils.DeleteUserType(ctTokenEntityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

func (ts *ChallengeTokenTestSuite) TestNewFlowReturnsChallengeToken() {
	flowStep, err := common.InitiateAuthenticationFlow(ctTokenTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected INCOMPLETE flow status")
	ts.Require().NotEmpty(flowStep.ChallengeToken, "Challenge token must be returned for incomplete flow")
}

func (ts *ChallengeTokenTestSuite) TestValidTokenAllowsContinuation() {
	flowStep, err := common.InitiateAuthenticationFlow(ctTokenTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().NotEmpty(flowStep.ChallengeToken, "Challenge token must be returned")

	inputs := map[string]string{
		"username": "ct_token_testuser",
		"password": "testpassword",
	}
	completeStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Flow should complete with valid challenge token and credentials")
	ts.Require().Equal("COMPLETE", completeStep.FlowStatus, "Expected COMPLETE flow status")
	ts.Require().NotEmpty(completeStep.Assertion, "Assertion token should be present")
}

func (ts *ChallengeTokenTestSuite) TestTokenIsRotatedOnEachStep() {
	flowStep, err := common.InitiateAuthenticationFlow(ctTokenTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	firstToken := flowStep.ChallengeToken
	ts.Require().NotEmpty(firstToken, "Challenge token must be returned on initiation")

	// Submit only the username to trigger a retry step (password missing)
	inputs := map[string]string{
		"username": "ct_token_testuser",
	}
	retryStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001", firstToken)

	ts.Require().NoError(err, "Flow step should succeed with valid challenge token")
	ts.Require().Equal("INCOMPLETE", retryStep.FlowStatus, "Expected retry step for missing password")
	ts.Require().NotEmpty(retryStep.ChallengeToken, "Rotated challenge token must be returned")
	ts.Require().NotEqual(firstToken, retryStep.ChallengeToken, "Challenge token must be rotated on each step")
}

func (ts *ChallengeTokenTestSuite) TestStaleTokenIsRejectedAfterRotation() {
	flowStep, err := common.InitiateAuthenticationFlow(ctTokenTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	firstToken := flowStep.ChallengeToken
	ts.Require().NotEmpty(firstToken, "Challenge token must be returned on initiation")

	// Submit only the username to trigger a retry step and rotate the token
	inputs := map[string]string{
		"username": "ct_token_testuser",
	}
	retryStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001", firstToken)
	ts.Require().NoError(err, "Flow step should succeed with valid challenge token")
	ts.Require().Equal("INCOMPLETE", retryStep.FlowStatus, "Expected retry step for missing password")
	ts.Require().NotEmpty(retryStep.ChallengeToken, "Rotated challenge token must be returned")

	// Attempt to use the old (stale) token after rotation
	fullInputs := map[string]string{
		"username": "ct_token_testuser",
		"password": "testpassword",
	}
	errorStep, err := common.CompleteFlow(flowStep.ExecutionID, fullInputs, "action_001", firstToken)
	ts.Require().NoError(err, "Flow step should be returned for stale challenge token")
	ts.Require().Equal("ERROR", errorStep.FlowStatus, "Expected ERROR flow status for stale token")
	ts.Require().NotNil(errorStep.Error, "Error should be present in flow step")
	ts.Require().Equal("ICS-1002", errorStep.Error.Code, "Expected invalid challenge token error code for stale token")
}

func (ts *ChallengeTokenTestSuite) TestInvalidTokenRejectsRequest() {
	flowStep, err := common.InitiateAuthenticationFlow(ctTokenTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().NotEmpty(flowStep.ChallengeToken, "Challenge token must be returned")

	inputs := map[string]string{
		"username": "ct_token_testuser",
		"password": "testpassword",
	}
	errorStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001", "wrong-challenge-token")
	ts.Require().NoError(err, "Flow step should be returned for invalid challenge token")
	ts.Require().Equal("ERROR", errorStep.FlowStatus, "Expected ERROR flow status")
	ts.Require().NotNil(errorStep.Error, "Error should be present in flow step")
	ts.Require().Equal("ICS-1002", errorStep.Error.Code, "Expected invalid challenge token error code")
}

func (ts *ChallengeTokenTestSuite) TestMissingTokenRejectsRequest() {
	flowStep, err := common.InitiateAuthenticationFlow(ctTokenTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().NotEmpty(flowStep.ChallengeToken, "Challenge token must be returned")

	inputs := map[string]string{
		"username": "ct_token_testuser",
		"password": "testpassword",
	}
	errorStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001", "")
	ts.Require().NoError(err, "Flow step should be returned for missing challenge token")
	ts.Require().Equal("ERROR", errorStep.FlowStatus, "Expected ERROR flow status")
	ts.Require().NotNil(errorStep.Error, "Error should be present in flow step")
	ts.Require().Equal("ICS-1002", errorStep.Error.Code, "Expected invalid challenge token error code")
}

func (ts *ChallengeTokenTestSuite) TestFlowCanBeRetriedAfterInvalidToken() {
	flowStep, err := common.InitiateAuthenticationFlow(ctTokenTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().NotEmpty(flowStep.ChallengeToken, "Challenge token must be returned on initiation")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID must be returned on initiation")

	inputs := map[string]string{
		"username": "ct_token_testuser",
		"password": "testpassword",
	}

	// Submit with a wrong token
	errorStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001", "wrong-challenge-token")
	ts.Require().NoError(err, "Flow step should be returned for invalid challenge token")
	ts.Require().Equal("ERROR", errorStep.FlowStatus, "Expected ERROR flow status")
	ts.Require().NotNil(errorStep.Error, "Error should be present in flow step")
	ts.Require().Equal("ICS-1002", errorStep.Error.Code, "Expected invalid challenge token error code")

	// Retry the same execution with the correct token
	completeStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Flow should complete when retried with the correct challenge token")
	ts.Require().Equal("COMPLETE", completeStep.FlowStatus, "Expected COMPLETE flow status on retry")
	ts.Require().NotEmpty(completeStep.Assertion, "Assertion token should be present after successful retry")
}
