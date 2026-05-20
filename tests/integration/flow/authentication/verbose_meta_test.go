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

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

var (
	basicAuthFlowWithPrompt = testutils.Flow{
		Name:     "Basic Auth Flow with Prompt Test",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_basic_with_prompt_test",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "prompt_credentials",
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"meta": map[string]interface{}{
					"components": []map[string]interface{}{
						{
							"type":    "TEXT",
							"id":      "text_001",
							"label":   "{{ t(signin:heading.label) }}",
							"variant": "HEADING_01",
						},
						{
							"type": "BLOCK",
							"id":   "block_001",
							"components": []map[string]interface{}{
								{
									"type":        "TEXT_INPUT",
									"id":          "input_001",
									"label":       "{{ t(signin:fields.username.label) }}",
									"required":    true,
									"placeholder": "{{ t(signin:fields.username.placeholder) }}",
								},
								{
									"type":        "TEXT_INPUT",
									"id":          "input_002",
									"label":       "{{ t(signin:fields.password.label) }}",
									"required":    true,
									"placeholder": "{{ t(signin:fields.password.placeholder) }}",
								},
								{
									"type":  "ACTION",
									"id":    "action_001",
									"label": "{{ t(signin:buttons.submit.label) }}",
								},
							},
						},
					},
				},
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

	basicAuthFlow = testutils.Flow{
		Name:     "Basic Auth Flow Test",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_basic_test",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "basic_auth",
			},
			{
				"id":   "basic_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "BasicAuthExecutor",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_001",
							"type":       "TEXT_INPUT",
							"identifier": "username",
							"required":   true,
						},
						{
							"ref":        "input_002",
							"type":       "PASSWORD_INPUT",
							"identifier": "password",
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
)

var (
	verboseTestOU = testutils.OrganizationUnit{
		Handle:      "verbose_test_ou",
		Name:        "Test Organization Unit for Verbose Mode",
		Description: "Organization unit created for verbose mode testing",
		Parent:      nil,
	}

	verboseTestEntityType = testutils.UserType{
		Name: "verbose_test_schema",
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
		},
	}

	verboseTestApp = testutils.Application{
		Name:                      "Verbose Test Application",
		Description:               "Application for testing verbose mode and meta",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "verbose_test_client",
		ClientSecret:              "verbose_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{verboseTestEntityType.Name},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}

	verboseTestUser = testutils.User{
		Type: verboseTestEntityType.Name,
		Attributes: json.RawMessage(`{
			"username": "verboseuser",
			"password": "testpassword123",
			"email": "verbose@example.com"
		}`),
	}
)

var (
	verboseTestAppID        string
	verboseEntityTypeID     string
	verboseFlowWithPromptID string
	verboseBasicFlowID      string
)

type VerboseMetaTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig
	ouID   string
}

func TestVerboseMetaTestSuite(t *testing.T) {
	suite.Run(t, new(VerboseMetaTestSuite))
}

func (ts *VerboseMetaTestSuite) SetupSuite() {
	// Initialize config
	ts.config = &common.TestSuiteConfig{}

	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(verboseTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	// create user type
	verboseTestEntityType.OUID = ouID
	schemaID, err := testutils.CreateUserType(verboseTestEntityType)
	ts.Require().NoError(err, "Failed to create user type")
	verboseEntityTypeID = schemaID

	// Create flows
	flowWithPromptID, err := testutils.CreateFlow(basicAuthFlowWithPrompt)
	ts.Require().NoError(err, "Failed to create basic auth flow with prompt")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowWithPromptID)
	verboseFlowWithPromptID = flowWithPromptID
	verboseTestApp.AuthFlowID = flowWithPromptID

	basicFlowID, err := testutils.CreateFlow(basicAuthFlow)
	ts.Require().NoError(err, "Failed to create basic auth flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, basicFlowID)
	verboseBasicFlowID = basicFlowID

	// Create test application with the flow with prompt
	verboseTestApp.OUID = ts.ouID
	appID, err := testutils.CreateApplication(verboseTestApp)
	ts.Require().NoError(err, "Failed to create test application")
	verboseTestAppID = appID

	// Create test user
	verboseTestUser.OUID = ouID
	userID, err := testutils.CreateUser(verboseTestUser)
	ts.Require().NoError(err, "Failed to create test user")
	ts.config.CreatedUserIDs = []string{userID}
}

func (ts *VerboseMetaTestSuite) TearDownSuite() {
	// Clean up test users
	for _, userID := range ts.config.CreatedUserIDs {
		testutils.DeleteUser(userID)
	}

	// Clean up test application
	if verboseTestAppID != "" {
		err := testutils.DeleteApplication(verboseTestAppID)
		ts.Require().NoError(err, "Failed to delete test application")
	}

	// Clean up test flows
	for _, flowID := range ts.config.CreatedFlowIDs {
		err := testutils.DeleteFlow(flowID)
		ts.Require().NoError(err, "Failed to delete test flow")
	}

	// Clean up test user type
	if verboseEntityTypeID != "" {
		err := testutils.DeleteUserType(verboseEntityTypeID)
		ts.Require().NoError(err, "Failed to delete user type")
	}

	// Clean up test organization unit
	if ts.ouID != "" {
		err := testutils.DeleteOrganizationUnit(ts.ouID)
		ts.Require().NoError(err, "Failed to delete test organization unit")
	}
}

func (ts *VerboseMetaTestSuite) TestVerboseModeEnabled() {
	// Step 1: Initiate flow with verbose=true
	flowStep, err := common.InitiateAuthenticationFlow(verboseTestAppID, true, nil, "")
	ts.Require().NoError(err, "Failed to initiate auth flow")
	ts.Require().NotNil(flowStep, "Flow step should not be nil")

	// Verify flow is incomplete and waiting for user input
	ts.Equal("INCOMPLETE", flowStep.FlowStatus, "Flow status should be INCOMPLETE")
	ts.Equal("VIEW", flowStep.Type, "Flow type should be VIEW")

	// Verify meta object is present in the response
	ts.Require().NotNil(flowStep.Data.Meta, "Meta should be present when verbose=true")

	// Verify meta structure contains expected components
	metaMap, ok := flowStep.Data.Meta.(map[string]interface{})
	ts.Require().True(ok, "Meta should be a map")
	ts.Contains(metaMap, "components", "Meta should contain components")

	components, ok := metaMap["components"].([]interface{})
	ts.Require().True(ok, "Components should be an array")
	ts.Greater(len(components), 0, "Components should not be empty")

	// Step 2: Continue flow with credentials (verbose flag should persist)
	inputs := map[string]string{
		"username": "verboseuser",
		"password": "testpassword123",
	}
	action := "action_001"
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, action,
		flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to continue auth flow")
	ts.Require().NotNil(flowStep, "Flow step should not be nil")

	// Verify flow completes successfully
	ts.Equal("COMPLETE", flowStep.FlowStatus, "Flow status should be COMPLETE")
	ts.NotEmpty(flowStep.Assertion, "Assertion should be present")
}

func (ts *VerboseMetaTestSuite) TestVerboseModeDisabled() {
	// Step 1: Initiate flow with verbose=false (default)
	flowStep, err := common.InitiateAuthenticationFlow(verboseTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate auth flow")
	ts.Require().NotNil(flowStep, "Flow step should not be nil")

	// Verify flow is incomplete and waiting for user input
	ts.Equal("INCOMPLETE", flowStep.FlowStatus, "Flow status should be INCOMPLETE")
	ts.Equal("VIEW", flowStep.Type, "Flow type should be VIEW")

	// Verify meta object is NOT present in the response
	ts.Nil(flowStep.Data.Meta, "Meta should NOT be present when verbose=false")

	// Verify inputs are still present (only meta is excluded)
	ts.NotEmpty(flowStep.Data.Inputs, "Inputs should still be present")
	ts.NotEmpty(flowStep.Data.Actions, "Actions should still be present")

	// Step 2: Continue flow with credentials
	inputs := map[string]string{
		"username": "verboseuser",
		"password": "testpassword123",
	}
	action := "action_001"
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, action,
		flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to continue auth flow")
	ts.Require().NotNil(flowStep, "Flow step should not be nil")

	// Verify flow completes successfully
	ts.Equal("COMPLETE", flowStep.FlowStatus, "Flow status should be COMPLETE")
	ts.NotEmpty(flowStep.Assertion, "Assertion should be present")
}

func (ts *VerboseMetaTestSuite) TestVerboseModeWithGraphWithoutMeta() {
	// Create a new app with a graph that doesn't have meta defined
	appWithoutMeta := testutils.Application{
		Name:                      "No Meta Test Application",
		OUID:                      ts.ouID,
		Description:               "Application for testing verbose mode without meta",
		IsRegistrationFlowEnabled: false,
		AuthFlowID:                verboseBasicFlowID,
		ClientID:                  "no_meta_test_client",
		ClientSecret:              "no_meta_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{verboseTestEntityType.Name},
	}

	appID, err := testutils.CreateApplication(appWithoutMeta)
	ts.Require().NoError(err, "Failed to create test application")
	defer func() {
		_ = testutils.DeleteApplication(appID)
	}()

	// Step 1: Initiate flow with verbose=true on a graph without meta
	flowStep, err := common.InitiateAuthenticationFlow(appID, true, nil, "")
	ts.Require().NoError(err, "Failed to initiate auth flow")
	ts.Require().NotNil(flowStep, "Flow step should not be nil")

	// Verify flow is incomplete and waiting for user input
	ts.Equal("INCOMPLETE", flowStep.FlowStatus, "Flow status should be INCOMPLETE")

	// Verify meta is nil when not defined in graph (even with verbose=true)
	ts.Nil(flowStep.Data.Meta, "Meta should be nil when not defined in graph")

	// Verify inputs are present
	ts.NotEmpty(flowStep.Data.Inputs, "Inputs should be present")

	// Step 2: Continue flow with credentials
	inputs := map[string]string{
		"username": "verboseuser",
		"password": "testpassword123",
	}
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "",
		flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to continue auth flow")
	ts.Require().NotNil(flowStep, "Flow step should not be nil")

	// Verify flow completes successfully
	ts.Equal("COMPLETE", flowStep.FlowStatus, "Flow status should be COMPLETE")
	ts.NotEmpty(flowStep.Assertion, "Assertion should be present")
}

func (ts *VerboseMetaTestSuite) TestVerbosePersistsAcrossRequests() {
	// Step 1: Initiate flow with verbose=true
	flowStep, err := common.InitiateAuthenticationFlow(verboseTestAppID, true, nil, "")
	ts.Require().NoError(err, "Failed to initiate auth flow")
	ts.Require().NotNil(flowStep, "Flow step should not be nil")
	ts.NotNil(flowStep.Data.Meta, "Meta should be present in first step")

	// Step 2: Continue flow WITHOUT sending verbose flag
	// Verbose should persist from initial request
	inputs := map[string]string{
		"username": "verboseuser",
		"password": "testpassword123",
	}
	action := "action_001"

	// Note: We're not sending verbose flag here, it should be retrieved from stored context
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, action,
		flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to continue auth flow")
	ts.Require().NotNil(flowStep, "Flow step should not be nil")

	// Verify flow completes successfully
	ts.Equal("COMPLETE", flowStep.FlowStatus, "Flow status should be COMPLETE")
}
