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
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

var (
	authzTestOU = testutils.OrganizationUnit{
		Handle:      "authz-flow-test-ou",
		Name:        "Authorization Flow Test Organization Unit",
		Description: "Organization unit for authorization flow testing",
		Parent:      nil,
	}

	authzTestEntityType = testutils.UserType{
		Name: "authz-test-person",
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
			"given_name": map[string]interface{}{
				"type": "string",
			},
			"family_name": map[string]interface{}{
				"type": "string",
			},
		},
	}

	authzTestFlow = testutils.Flow{
		Name:     "Authorization Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_authz_test",
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
				"onSuccess": "authorization_check",
			},
			{
				"id":   "authorization_check",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthorizationExecutor",
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

	authzTestApp = testutils.Application{
		Name:                      "Authz Flow Test Application",
		Description:               "Application for testing authorization in flows",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "authz_flow_test_client",
		ClientSecret:              "authz_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"authz-test-person"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}

	userWithRole = testutils.User{
		Type: "authz-test-person",
		Attributes: json.RawMessage(`{
			"username": "authorized_user",
			"password": "SecurePass123!",
			"email": "authorized@test.com",
			"given_name": "Authorized",
			"family_name": "User"
		}`),
	}

	userNoRole = testutils.User{
		Type: "authz-test-person",
		Attributes: json.RawMessage(`{
			"username": "unauthorized_user",
			"password": "SecurePass123!",
			"email": "unauthorized@test.com",
			"given_name": "Unauthorized",
			"family_name": "User"
		}`),
	}

	documentEditorRole = testutils.Role{
		Name:        "DocumentEditor",
		Description: "Can read and write documents",
	}
)

var (
	authzTestOUID           string
	authzTestAppID          string
	authzTestRoleID         string
	authzUserWithRole       string
	authzUserNoRole         string
	authzEntityTypeID       string
	authzTestResourceServer string
)

type FlowAuthzTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig
}

func TestFlowAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(FlowAuthzTestSuite))
}

func (ts *FlowAuthzTestSuite) SetupSuite() {
	// Initialize config
	ts.config = &common.TestSuiteConfig{}
	var err error

	// Create test organization unit
	authzTestOUID, err = testutils.CreateOrganizationUnit(authzTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}

	// create user type within the test organization unit
	authzTestEntityType.OUID = authzTestOUID
	authzEntityTypeID, err = testutils.CreateUserType(authzTestEntityType)
	if err != nil {
		ts.T().Fatalf("Failed to create user type during setup: %v", err)
	}

	// Create flow
	flowID, err := testutils.CreateFlow(authzTestFlow)
	ts.Require().NoError(err, "Failed to create authorization test flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	authzTestApp.AuthFlowID = flowID

	// Create test application
	authzTestApp.OUID = authzTestOUID
	authzTestAppID, err = testutils.CreateApplication(authzTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}

	// Create user with role
	userWithRoleCopy := userWithRole
	userWithRoleCopy.OUID = authzTestOUID
	authzUserWithRole, err = testutils.CreateUser(userWithRoleCopy)
	if err != nil {
		ts.T().Fatalf("Failed to create user with role during setup: %v", err)
	}

	// Create user without role
	userNoRoleCopy := userNoRole
	userNoRoleCopy.OUID = authzTestOUID
	authzUserNoRole, err = testutils.CreateUser(userNoRoleCopy)
	if err != nil {
		ts.T().Fatalf("Failed to create user without role during setup: %v", err)
	}

	// Create resource server with actions for permissions
	resourceServer := testutils.ResourceServer{
		Name:        "Document Management System",
		Description: "System for managing documents",
		Identifier:  "document-mgmt",
		OUID:        authzTestOUID,
	}
	actions := []testutils.Action{
		{
			Name:        "Read Documents",
			Handle:      "read",
			Description: "Permission to read documents",
		},
		{
			Name:        "Write Documents",
			Handle:      "write",
			Description: "Permission to write documents",
		},
	}
	authzTestResourceServer, err := testutils.CreateResourceServerWithActions(resourceServer, actions)
	if err != nil {
		ts.T().Fatalf("Failed to create resource server with actions during setup: %v", err)
	}

	// Create role with user assignment
	roleToCreate := documentEditorRole
	roleToCreate.OUID = authzTestOUID
	roleToCreate.Permissions = []testutils.ResourcePermissions{
		{
			ResourceServerID: authzTestResourceServer,
			Permissions:      []string{"read", "write"},
		},
	}
	roleToCreate.Assignments = []testutils.Assignment{
		{ID: authzUserWithRole, Type: "user"},
	}
	authzTestRoleID, err = testutils.CreateRole(roleToCreate)
	if err != nil {
		ts.T().Fatalf("Failed to create test role during setup: %v", err)
	}
}

func (ts *FlowAuthzTestSuite) TearDownSuite() {
	// Delete in reverse order of creation
	if authzTestRoleID != "" {
		if err := testutils.DeleteRole(authzTestRoleID); err != nil {
			ts.T().Logf("Failed to delete test role during teardown: %v", err)
		}
	}

	if authzTestResourceServer != "" {
		if err := testutils.DeleteResourceServer(authzTestResourceServer); err != nil {
			ts.T().Logf("Failed to delete test resource server during teardown: %v", err)
		}
	}

	if authzUserNoRole != "" {
		if err := testutils.DeleteUser(authzUserNoRole); err != nil {
			ts.T().Logf("Failed to delete user without role during teardown: %v", err)
		}
	}

	if authzUserWithRole != "" {
		if err := testutils.DeleteUser(authzUserWithRole); err != nil {
			ts.T().Logf("Failed to delete user with role during teardown: %v", err)
		}
	}

	if len(ts.config.CreatedFlowIDs) > 0 {
		for _, flowID := range ts.config.CreatedFlowIDs {
			if err := testutils.DeleteFlow(flowID); err != nil {
				ts.T().Logf("Failed to delete created flow (%s) during teardown: %v", flowID, err)
			}
		}
	}

	if authzTestAppID != "" {
		if err := testutils.DeleteApplication(authzTestAppID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}

	if authzTestOUID != "" {
		if err := testutils.DeleteOrganizationUnit(authzTestOUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}

	if authzEntityTypeID != "" {
		if err := testutils.DeleteUserType(authzEntityTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}
}

// TestAuthorizationFlow_UserWithDirectRoleAssignment tests authorization when user has all requested permissions
func (ts *FlowAuthzTestSuite) TestAuthorizationFlow_UserWithDirectRoleAssignment() {
	// Initiate authentication flow with requested permissions
	inputs := map[string]string{
		"applicationId":         authzTestAppID,
		"requested_permissions": "read write",
	}

	flowStep, err := common.InitiateAuthenticationFlow(authzTestAppID, false, inputs, "")
	ts.Require().NoError(err, "Failed to initiate flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	// Execute basic auth step with authorized user credentials
	authInputs := map[string]string{
		"username": "authorized_user",
		"password": "SecurePass123!",
	}

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, authInputs, "action_001",
		flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete authentication")
	ts.Require().NotNil(flowStep, "Flow step should not be nil")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Flow should be complete")
	ts.Require().NotEmpty(flowStep.Assertion, "Assertion should not be empty")

	// Decode the JWT assertion
	claims, err := testutils.DecodeJWT(flowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT")
	ts.Require().NotNil(claims, "Claims should not be nil")

	// Verify authorized_permissions claim
	authorizedPermsRaw, ok := claims.Additional["authorized_permissions"]
	ts.Require().True(ok, "authorized_permissions claim should be present")

	authorizedPermsStr, ok := authorizedPermsRaw.(string)
	ts.Require().True(ok, "authorized_permissions should be a string")

	// Parse space-separated permissions
	authorizedPerms := strings.Split(strings.TrimSpace(authorizedPermsStr), " ")
	ts.Require().Len(authorizedPerms, 2, "Should have 2 authorized permissions")
	ts.Require().Contains(authorizedPerms, "read", "Should contain read")
	ts.Require().Contains(authorizedPerms, "write", "Should contain write")
}

// TestAuthorizationFlow_UserWithNoRole tests authorization when user has no role/permissions
func (ts *FlowAuthzTestSuite) TestAuthorizationFlow_UserWithNoRole() {
	// Initiate authentication flow with requested permissions
	inputs := map[string]string{
		"applicationId":         authzTestAppID,
		"requested_permissions": "read write",
	}

	flowStep, err := common.InitiateAuthenticationFlow(authzTestAppID, false, inputs, "")
	ts.Require().NoError(err, "Failed to initiate flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	// Execute basic auth step with unauthorized user credentials
	authInputs := map[string]string{
		"username": "unauthorized_user",
		"password": "SecurePass123!",
	}

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, authInputs, "action_001",
		flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete authentication")
	ts.Require().NotNil(flowStep, "Flow step should not be nil")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Flow should be complete")
	ts.Require().NotEmpty(flowStep.Assertion, "Assertion should not be empty")

	// Decode the JWT assertion
	claims, err := testutils.DecodeJWT(flowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT")
	ts.Require().NotNil(claims, "Claims should not be nil")

	// Verify authorized_permissions claim - should not be present
	_, ok := claims.Additional["authorized_permissions"]
	ts.Require().False(ok, "authorized_permissions claim should not be present")
}

// TestAuthorizationFlow_UserWithPartialPermissions tests authorization when user has
// only subset of requested permissions
func (ts *FlowAuthzTestSuite) TestAuthorizationFlow_UserWithPartialPermissions() {
	// Initiate authentication flow requesting 3 permissions (user only has 2)
	inputs := map[string]string{
		"applicationId":         authzTestAppID,
		"requested_permissions": "read write delete",
	}

	flowStep, err := common.InitiateAuthenticationFlow(authzTestAppID, false, inputs, "")
	ts.Require().NoError(err, "Failed to initiate flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	// Execute basic auth step with authorized user credentials
	authInputs := map[string]string{
		"username": "authorized_user",
		"password": "SecurePass123!",
	}

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, authInputs, "action_001",
		flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete authentication")
	ts.Require().NotNil(flowStep, "Flow step should not be nil")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Flow should be complete")
	ts.Require().NotEmpty(flowStep.Assertion, "Assertion should not be empty")

	// Decode the JWT assertion
	claims, err := testutils.DecodeJWT(flowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT")
	ts.Require().NotNil(claims, "Claims should not be nil")

	// Verify authorized_permissions claim - should only have read and write, not delete
	authorizedPermsRaw, ok := claims.Additional["authorized_permissions"]
	ts.Require().True(ok, "authorized_permissions claim should be present")

	authorizedPermsStr, ok := authorizedPermsRaw.(string)
	ts.Require().True(ok, "authorized_permissions should be a string")

	// Parse space-separated permissions
	authorizedPerms := strings.Split(strings.TrimSpace(authorizedPermsStr), " ")
	ts.Require().Len(authorizedPerms, 2, "Should have 2 authorized permissions (not 3)")
	ts.Require().Contains(authorizedPerms, "read", "Should contain read")
	ts.Require().Contains(authorizedPerms, "write", "Should contain write")
	ts.Require().NotContains(authorizedPerms, "delete", "Should NOT contain delete")
}
