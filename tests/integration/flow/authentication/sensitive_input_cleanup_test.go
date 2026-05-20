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

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

// sensitiveInputCleanupFlow defines an authentication flow with two prompt nodes for password.
var sensitiveInputCleanupFlow = testutils.Flow{
	Name:     "Sensitive Input Cleanup Test Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_sensitive_input_cleanup_test",
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
			"onSuccess": "prompt_password_again",
		},
		{
			"id":   "prompt_password_again",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_003",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_004",
							"identifier": "password",
							"type":       "PASSWORD_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "action_002",
						"nextNode": "auth_assert",
					},
				},
			},
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

var (
	sensitiveCleanupTestOU = testutils.OrganizationUnit{
		Handle:      "sensitive_cleanup_test_ou",
		Name:        "Test OU for Sensitive Input Cleanup",
		Description: "Organization unit created for sensitive input cleanup testing",
		Parent:      nil,
	}

	sensitiveCleanupTestApp = testutils.Application{
		Name:                      "Sensitive Input Cleanup Test App",
		Description:               "Application for testing sensitive input cleanup in auth flows",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "sensitive_cleanup_test_client",
		ClientSecret:              "sensitive_cleanup_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"sensitive_cleanup_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}

	sensitiveCleanupEntityType = testutils.UserType{
		Name: "sensitive_cleanup_user",
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

	sensitiveCleanupTestUser = testutils.User{
		Type: sensitiveCleanupEntityType.Name,
		Attributes: json.RawMessage(`{
			"username": "sensitiveuser",
			"password": "sensitivepassword"
		}`),
	}
)

var (
	sensitiveCleanupAppID    string
	sensitiveCleanupSchemaID string
)

type SensitiveInputCleanupTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig
	ouID   string
}

func TestSensitiveInputCleanupTestSuite(t *testing.T) {
	suite.Run(t, new(SensitiveInputCleanupTestSuite))
}

func (ts *SensitiveInputCleanupTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(sensitiveCleanupTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	// Create test user type
	sensitiveCleanupEntityType.OUID = ts.ouID
	schemaID, err := testutils.CreateUserType(sensitiveCleanupEntityType)
	if err != nil {
		ts.T().Fatalf("Failed to create test user type during setup: %v", err)
	}
	sensitiveCleanupSchemaID = schemaID

	// Create flow
	flowID, err := testutils.CreateFlow(sensitiveInputCleanupFlow)
	ts.Require().NoError(err, "Failed to create sensitive input cleanup test flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	sensitiveCleanupTestApp.AuthFlowID = flowID

	// Create test application
	sensitiveCleanupTestApp.OUID = ts.ouID
	appID, err := testutils.CreateApplication(sensitiveCleanupTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	sensitiveCleanupAppID = appID

	// Create test user with the created OU
	testUser := sensitiveCleanupTestUser
	testUser.OUID = ts.ouID
	userIDs, err := testutils.CreateMultipleUsers(testUser)
	if err != nil {
		ts.T().Fatalf("Failed to create test user during setup: %v", err)
	}
	ts.config.CreatedUserIDs = userIDs
}

func (ts *SensitiveInputCleanupTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users during teardown: %v", err)
	}

	if sensitiveCleanupAppID != "" {
		if err := testutils.DeleteApplication(sensitiveCleanupAppID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}

	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow %s during teardown: %v", flowID, err)
		}
	}

	if sensitiveCleanupSchemaID != "" {
		if err := testutils.DeleteUserType(sensitiveCleanupSchemaID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}

	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

func (ts *SensitiveInputCleanupTestSuite) TestPasswordClearedAfterAuthExecution() {
	// Update application with the sensitive cleanup flow
	err := common.UpdateAppConfig(sensitiveCleanupAppID, ts.config.CreatedFlowIDs[0], "")
	ts.Require().NoError(err, "App config update should succeed")

	// Step 1: Initiate the flow - should ask for username and password
	flowStep, err := common.InitiateAuthenticationFlow(sensitiveCleanupAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", flowStep.Type, "Expected flow type to be VIEW")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	// Verify both username and password are requested
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "username"),
		"Username input should be present in first prompt")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "password"),
		"Password input should be present in first prompt")

	// Step 2: Submit username + password - basic_auth should validate, then password is cleared
	var userAttrs map[string]interface{}
	err = json.Unmarshal(sensitiveCleanupTestUser.Attributes, &userAttrs)
	ts.Require().NoError(err, "Failed to unmarshal user attributes")

	inputs := map[string]string{
		"username": userAttrs["username"].(string),
		"password": userAttrs["password"].(string),
	}

	// After basic_auth completes, password should be cleared from context.
	// The second prompt node (prompt_password_again) should detect the missing password
	// and return INCOMPLETE, asking for it again.
	step2, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete flow step 2")

	ts.Require().Equal("INCOMPLETE", step2.FlowStatus,
		"Expected INCOMPLETE because password was cleared and second prompt should re-request it")
	ts.Require().Equal("VIEW", step2.Type, "Expected flow type to be VIEW")

	// Verify the second prompt asks for password again but NOT username
	ts.Require().NotEmpty(step2.Data.Inputs, "Second prompt should require inputs")
	ts.Require().True(common.HasInput(step2.Data.Inputs, "password"),
		"Password should be re-requested because it was cleared from context after auth execution")
	ts.Require().False(common.HasInput(step2.Data.Inputs, "username"),
		"Username should NOT be re-requested because it was retained in context")

	// Step 3: Submit password again - should complete the flow
	inputs = map[string]string{
		"password": userAttrs["password"].(string),
	}

	step3, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_002", step2.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete flow step 3")

	ts.Require().Equal("COMPLETE", step3.FlowStatus, "Expected flow to complete after re-submitting password")
	ts.Require().NotEmpty(step3.Assertion, "JWT assertion should be returned after successful authentication")
	ts.Require().Empty(step3.FailureReason, "Failure reason should be empty for successful authentication")
}
