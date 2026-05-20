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

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	conditionalExecMockGooglePort    = 8093
	conditionalExecNewUserSub        = "conditional-exec-new-user-sub"
	conditionalExecNewUserEmail      = "newuser@conditional-exec-test.com"
	conditionalExecExistingUserSub   = "conditional-exec-existing-user-sub"
	conditionalExecExistingUserEmail = "existinguser@conditional-exec-test.com"
	conditionalExecNewOUHandle       = "conditional_exec_ou"
)

var (
	conditionalExecFlow = testutils.Flow{
		Name:     "Conditional Exec Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_google_conditional_ou",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "google_auth",
			},
			{
				"id":   "google_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId":                               "test-google-idp-id",
					"allowAuthenticationWithoutLocalUser": true,
				},
				"executor": map[string]interface{}{
					"name": "GoogleOIDCAuthExecutor",
				},
				"onSuccess": "conditional_ou_creation",
			},
			{
				"id":   "conditional_ou_creation",
				"type": "TASK_EXECUTION",
				"condition": map[string]interface{}{
					"key":    "{{ context.userEligibleForProvisioning }}",
					"value":  "true",
					"onSkip": "auth_assert",
				},
				"executor": map[string]interface{}{
					"name": "OUExecutor",
				},
				"onSuccess": "provisioning",
			},
			{
				"id":   "provisioning",
				"type": "TASK_EXECUTION",
				"condition": map[string]interface{}{
					"key":    "{{ context.userEligibleForProvisioning }}",
					"value":  "true",
					"onSkip": "auth_assert",
				},
				"executor": map[string]interface{}{
					"name": "ProvisioningExecutor",
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

	conditionalExecTestApp = testutils.Application{
		Name:                      "Conditional Exec Auth Flow Test Application",
		Description:               "Application for testing conditional node execution in authentication flows",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "conditional_exec_auth_flow_test_client",
		ClientSecret:              "conditional_exec_auth_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"conditional_exec_flow_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}

	conditionalExecTestOU = testutils.OrganizationUnit{
		Handle:      "conditional-exec-auth-flow-test-ou",
		Name:        "Conditional Exec Auth Flow Test OU",
		Description: "Organization unit for conditional execution authentication flow tests",
	}

	conditionalExecEntityType = testutils.UserType{
		Name:                  "conditional_exec_flow_user",
		AllowSelfRegistration: true,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"sub": map[string]interface{}{
				"type": "string",
			},
			"email": map[string]interface{}{
				"type": "string",
			},
			"email_verified": map[string]interface{}{
				"type": "string",
			},
			"name": map[string]interface{}{
				"type": "string",
			},
			"given_name": map[string]interface{}{
				"type": "string",
			},
			"family_name": map[string]interface{}{
				"type": "string",
			},
			"givenName": map[string]interface{}{
				"type": "string",
			},
			"familyName": map[string]interface{}{
				"type": "string",
			},
			"picture": map[string]interface{}{
				"type": "string",
			},
			"locale": map[string]interface{}{
				"type": "string",
			},
		},
	}
)

var (
	conditionalExecTestAppID      string
	conditionalExecEntityTypeID   string
	conditionalExecPreCreatedOUID string
)

type ConditionalExecAuthFlowTestSuite struct {
	suite.Suite
	config           *common.TestSuiteConfig
	mockGoogleServer *testutils.MockGoogleOIDCServer
	existingUserID   string
}

func TestConditionalExecAuthFlowTestSuite(t *testing.T) {
	suite.Run(t, new(ConditionalExecAuthFlowTestSuite))
}

func (ts *ConditionalExecAuthFlowTestSuite) SetupSuite() {
	// Initialize config
	ts.config = &common.TestSuiteConfig{}

	// Start mock Google server
	mockGoogleServer, err := testutils.NewMockGoogleOIDCServer(conditionalExecMockGooglePort,
		"test_google_client", "test_google_secret")
	ts.Require().NoError(err, "Failed to create mock Google server")
	ts.mockGoogleServer = mockGoogleServer

	// Add test users
	ts.mockGoogleServer.AddUser(&testutils.GoogleUserInfo{
		Sub:           conditionalExecNewUserSub,
		Email:         conditionalExecNewUserEmail,
		EmailVerified: true,
		Name:          "New Conditional Exec User",
		GivenName:     "New",
		FamilyName:    "User",
		Picture:       "https://example.com/picture.jpg",
		Locale:        "en",
	})
	ts.mockGoogleServer.AddUser(&testutils.GoogleUserInfo{
		Sub:           conditionalExecExistingUserSub,
		Email:         conditionalExecExistingUserEmail,
		EmailVerified: true,
		Name:          "Existing Conditional Exec User",
		GivenName:     "Existing",
		FamilyName:    "User",
		Picture:       "https://example.com/picture2.jpg",
		Locale:        "en",
	})

	err = ts.mockGoogleServer.Start()
	ts.Require().NoError(err, "Failed to start mock Google server")

	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(conditionalExecTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	conditionalExecPreCreatedOUID = ouID

	// create user type with self-registration enabled
	conditionalExecEntityType.OUID = conditionalExecPreCreatedOUID
	schemaID, err := testutils.CreateUserType(conditionalExecEntityType)
	ts.Require().NoError(err, "Failed to create conditional exec user type")
	conditionalExecEntityTypeID = schemaID

	// Create an existing user
	existingUserAttributes := map[string]interface{}{
		"username":   "existingconditionalexecuser",
		"password":   "Test@1234",
		"sub":        conditionalExecExistingUserSub,
		"email":      conditionalExecExistingUserEmail,
		"givenName":  "Existing",
		"familyName": "User",
	}
	attributesJSON, err := json.Marshal(existingUserAttributes)
	ts.Require().NoError(err)

	existingUser := testutils.User{
		Type:       conditionalExecEntityType.Name,
		OUID:       conditionalExecPreCreatedOUID,
		Attributes: json.RawMessage(attributesJSON),
	}
	existingUserID, err := testutils.CreateUser(existingUser)
	ts.Require().NoError(err, "Failed to create existing test user")
	ts.existingUserID = existingUserID

	// Create Google IDP
	googleIDP := testutils.IDP{
		Name:        "Conditional Exec Google IDP",
		Description: "Google IDP for conditional execution flow test",
		Type:        "GOOGLE",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test_google_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "test_google_secret",
				IsSecret: true,
			},
			{
				Name:     "authorization_endpoint",
				Value:    ts.mockGoogleServer.GetURL() + "/o/oauth2/v2/auth",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    ts.mockGoogleServer.GetURL() + "/token",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    ts.mockGoogleServer.GetURL() + "/v1/userinfo",
				IsSecret: false,
			},
			{
				Name:     "jwks_endpoint",
				Value:    ts.mockGoogleServer.GetURL() + "/oauth2/v3/certs",
				IsSecret: false,
			},
			{
				Name:     "scopes",
				Value:    "openid email profile",
				IsSecret: false,
			},
			{
				Name:     "redirect_uri",
				Value:    "http://localhost:3000/callback",
				IsSecret: false,
			},
		},
	}

	idpID, err := testutils.CreateIDP(googleIDP)
	ts.Require().NoError(err, "Failed to create Google IDP")
	ts.config.CreatedIdpIDs = append(ts.config.CreatedIdpIDs, idpID)

	// Update flow definition with created IDP ID
	nodes := conditionalExecFlow.Nodes.([]map[string]interface{})
	nodes[1]["properties"].(map[string]interface{})["idpId"] = idpID
	conditionalExecFlow.Nodes = nodes

	// Create flow
	flowID, err := testutils.CreateFlow(conditionalExecFlow)
	ts.Require().NoError(err, "Failed to create conditional exec flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	conditionalExecTestApp.AuthFlowID = flowID

	// Create test application
	conditionalExecTestApp.OUID = conditionalExecPreCreatedOUID
	appID, err := testutils.CreateApplication(conditionalExecTestApp)
	ts.Require().NoError(err, "Failed to create test application")
	conditionalExecTestAppID = appID
}

func (ts *ConditionalExecAuthFlowTestSuite) TearDownSuite() {
	// Delete test application
	if conditionalExecTestAppID != "" {
		if err := testutils.DeleteApplication(conditionalExecTestAppID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}

	// Delete test flows
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow during teardown: %v", err)
		}
	}

	// Delete test IDPs
	for _, idpID := range ts.config.CreatedIdpIDs {
		if err := testutils.DeleteIDP(idpID); err != nil {
			ts.T().Logf("Failed to delete test IDP during teardown: %v", err)
		}
	}

	// Delete existing user
	if ts.existingUserID != "" {
		_ = testutils.DeleteUser(ts.existingUserID)
	}

	// delete user type
	if conditionalExecEntityTypeID != "" {
		_ = testutils.DeleteUserType(conditionalExecEntityTypeID)
	}

	// Delete test organization units
	childOUHandlePath := conditionalExecTestOU.Handle + "/" + conditionalExecNewOUHandle
	_ = testutils.DeleteOrganizationUnitByHandlePath(childOUHandlePath)

	if conditionalExecPreCreatedOUID != "" {
		_ = testutils.DeleteOrganizationUnit(conditionalExecPreCreatedOUID)
	}

	// Stop mock server
	if ts.mockGoogleServer != nil {
		_ = ts.mockGoogleServer.Stop()
		time.Sleep(200 * time.Millisecond)
	}
}

func (ts *ConditionalExecAuthFlowTestSuite) TestSkipConditionalNodes() {
	// Set the mock server to return the existing user
	ts.mockGoogleServer.SetAuthorizeFunc(func(email string) (string, error) {
		return conditionalExecExistingUserEmail, nil
	})

	// Step 1: Initialize the flow
	flowStep, err := common.InitiateAuthenticationFlow(conditionalExecTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("REDIRECTION", flowStep.Type, "Expected flow type to be REDIRECTION")

	ExecutionID := flowStep.ExecutionID
	redirectURLStr := flowStep.Data.RedirectURL

	// Step 2: Simulate user authorization at Google
	authCode, state, err := testutils.SimulateFederatedOAuthFlow(redirectURLStr)
	ts.Require().NoError(err, "Failed to simulate Google authorization")

	// Step 3: Complete the flow with the authorization code
	inputs := map[string]string{
		"code":  authCode,
		"state": state,
	}
	flowStep, err = common.CompleteFlow(ExecutionID, inputs, "", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete authentication flow")

	// For existing user, flow should complete directly
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"Expected flow status to be COMPLETE (conditional nodes should be skipped for existing user)")
	ts.Require().NotEmpty(flowStep.Assertion, "Assertion token should be present")
	ts.Empty(flowStep.FailureReason, "Failure reason should be empty")

	// Validate JWT assertion
	jwtClaims, err := testutils.ValidateJWTAssertionFields(
		flowStep.Assertion,
		conditionalExecTestAppID,
		conditionalExecEntityType.Name,
		conditionalExecPreCreatedOUID,
		conditionalExecTestOU.Name,
		conditionalExecTestOU.Handle,
	)
	ts.Require().NoError(err, "Failed to validate JWT assertion fields")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
}

func (ts *ConditionalExecAuthFlowTestSuite) TestExecuteConditionalNodes() {
	// Set the mock server to return the new user
	ts.mockGoogleServer.SetAuthorizeFunc(func(email string) (string, error) {
		return conditionalExecNewUserEmail, nil
	})

	// Step 1: Initialize the flow
	flowStep, err := common.InitiateAuthenticationFlow(conditionalExecTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("REDIRECTION", flowStep.Type, "Expected flow type to be REDIRECTION")

	ExecutionID := flowStep.ExecutionID
	redirectURLStr := flowStep.Data.RedirectURL
	ts.Require().NotEmpty(redirectURLStr, "Redirect URL should not be empty")

	// Step 2: Simulate user authorization at Google
	authCode, state, err := testutils.SimulateFederatedOAuthFlow(redirectURLStr)
	ts.Require().NoError(err, "Failed to simulate Google authorization")
	ts.Require().NotEmpty(authCode, "Authorization code should not be empty")

	// Step 3: Continue the flow with the authorization code
	inputs := map[string]string{
		"code":  authCode,
		"state": state,
	}
	flowStep, err = common.CompleteFlow(ExecutionID, inputs, "", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete authentication flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", flowStep.Type, "Expected flow type to be VIEW")

	// Step 4: Complete the flow with OU details
	ouInputs := map[string]string{
		"ouName":        "Conditional Exec OU",
		"ouHandle":      conditionalExecNewOUHandle,
		"ouDescription": "Organization Unit created during conditional exec auth flow test",
	}
	flowStep, err = common.CompleteFlow(ExecutionID, ouInputs, "", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete authentication flow after OU details")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(flowStep.Assertion, "Assertion token should be present")

	// Find the created user to get their details
	user, err := testutils.FindUserByAttribute("sub", conditionalExecNewUserSub)
	ts.Require().NoError(err, "Failed to find created user")
	ts.Require().NotNil(user, "User should be found after provisioning")

	// Validate JWT assertion - user should be in the newly created OU
	jwtClaims, err := testutils.DecodeJWT(flowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
	ts.Require().Equal(conditionalExecTestAppID, jwtClaims.Aud, "JWT aud should match app ID")
	ts.Require().Equal(conditionalExecEntityType.Name, jwtClaims.UserType, "JWT userType should match schema")
	ts.Require().NotEmpty(jwtClaims.OUID, "JWT ouId should not be empty")

	// Verify the created OU
	createdOU, err := testutils.GetOrganizationUnit(jwtClaims.OUID)
	ts.Require().NoError(err, "Failed to get created OU")
	ts.Require().Equal("Conditional Exec OU", createdOU.Name, "Created OU name should match")
	ts.Require().Equal(conditionalExecNewOUHandle, createdOU.Handle, "Created OU handle should match")
	ts.Require().NotNil(createdOU.Parent, "Created OU should have a parent")
	ts.Require().Equal(conditionalExecPreCreatedOUID, *createdOU.Parent, "Created OU parent should match")
}
