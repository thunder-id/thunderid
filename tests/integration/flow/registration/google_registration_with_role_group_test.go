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

package registration

import (
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

var (
	googleRegGroupRoleFlow = testutils.Flow{
		Name:     "Google Registration with Group and Role Assignment",
		FlowType: "REGISTRATION",
		Handle:   "registration_flow_google_group_role_test",
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
				"onSuccess":    "google_auth",
				"onIncomplete": "prompt_usertype",
			},
			{
				"id":   "prompt_usertype",
				"type": "PROMPT",
				"meta": map[string]interface{}{
					"components": []map[string]interface{}{
						{
							"type":    "TEXT",
							"id":      "heading_usertype",
							"label":   "Sign Up",
							"variant": "HEADING_2",
						},
						{
							"type": "BLOCK",
							"id":   "block_usertype",
							"components": []map[string]interface{}{
								{
									"type":        "SELECT",
									"id":          "usertype_input",
									"ref":         "userType",
									"label":       "User Type",
									"placeholder": "Select your user type",
									"required":    true,
									"options":     []interface{}{},
								},
								{
									"type":      "ACTION",
									"id":        "action_usertype",
									"label":     "Continue",
									"variant":   "PRIMARY",
									"eventType": "SUBMIT",
								},
							},
						},
					},
				},
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "usertype_input",
								"identifier": "userType",
								"type":       "SELECT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_usertype",
							"nextNode": "user_type_resolver",
						},
					},
				},
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
				"onSuccess": "provisioning",
			},
			{
				"id":   "provisioning",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"assignGroup": "placeholder-group-id",
					"assignRole":  "placeholder-role-id",
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

	googleRegGroupRoleTestOU = testutils.OrganizationUnit{
		Handle:      "google-reg-group-role-test-ou",
		Name:        "Google Registration Group/Role Test OU",
		Description: "Organization unit for testing Google registration with group/role assignment",
		Parent:      nil,
	}

	googleRegGroupRoleEntityType = testutils.UserType{
		Name: "google_reg_group_role_user",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
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

	googleRegGroupRoleTestApp = testutils.Application{
		Name:                      "Google Registration Group/Role Test App",
		Description:               "Application for testing Google registration with group/role assignment",
		IsRegistrationFlowEnabled: true,
		ClientID:                  "google_reg_group_role_test_client",
		ClientSecret:              "google_reg_group_role_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{googleRegGroupRoleEntityType.Name},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}
)

var (
	googleRegGroupRoleTestAppID string
	googleRegGroupRoleTestOUID  string
)

const (
	mockGoogleRegGroupRoleFlowPort = 8094
)

type GoogleRegistrationGroupRoleTestSuite struct {
	suite.Suite
	mockGoogleServer *testutils.MockGoogleOIDCServer
	idpID            string
	entityTypeID     string
	groupID          string
	roleID           string
	config           *common.TestSuiteConfig
}

func TestGoogleRegistrationGroupRoleTestSuite(t *testing.T) {
	suite.Run(t, new(GoogleRegistrationGroupRoleTestSuite))
}

func (ts *GoogleRegistrationGroupRoleTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	// Start mock Google server
	mockServer, err := testutils.NewMockGoogleOIDCServer(mockGoogleRegGroupRoleFlowPort,
		"test_google_client", "test_google_secret")
	ts.Require().NoError(err, "Failed to create mock Google server")
	ts.mockGoogleServer = mockServer

	ts.mockGoogleServer.AddUser(&testutils.GoogleUserInfo{
		Sub:           "google-group-role-user-789",
		Email:         "grouproleuser@gmail.com",
		EmailVerified: true,
		Name:          "Group Role User",
		GivenName:     "GroupRole",
		FamilyName:    "User",
		Picture:       "https://example.com/grouprolepicture.jpg",
		Locale:        "en",
	})

	err = ts.mockGoogleServer.Start()
	ts.Require().NoError(err, "Failed to start mock Google server")

	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(googleRegGroupRoleTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	googleRegGroupRoleTestOUID = ouID

	// create user type
	googleRegGroupRoleEntityType.OUID = googleRegGroupRoleTestOUID
	googleRegGroupRoleEntityType.AllowSelfRegistration = true
	schemaID, err := testutils.CreateUserType(googleRegGroupRoleEntityType)
	ts.Require().NoError(err, "Failed to create user type")
	ts.entityTypeID = schemaID

	// Create test group
	testGroup := testutils.Group{
		Name:        "Provisioned Users Group",
		Description: "Group for testing user provisioning with group assignment",
		OUID:        googleRegGroupRoleTestOUID,
	}
	groupID, err := testutils.CreateGroup(testGroup)
	ts.Require().NoError(err, "Failed to create test group")
	ts.groupID = groupID
	ts.config.CreatedGroupIDs = append(ts.config.CreatedGroupIDs, groupID)

	// Create test role
	testRole := testutils.Role{
		Name:        "Provisioned Users Role",
		Description: "Role for testing user provisioning with role assignment",
		OUID:        googleRegGroupRoleTestOUID,
		Permissions: []testutils.ResourcePermissions{},
	}
	roleID, err := testutils.CreateRole(testRole)
	ts.Require().NoError(err, "Failed to create test role")
	ts.roleID = roleID
	ts.config.CreatedRoleIDs = append(ts.config.CreatedRoleIDs, roleID)

	// Create Google IDP
	googleIDP := testutils.IDP{
		Name:        "Google Group/Role Test IDP",
		Description: "Google IDP for testing registration with group/role assignment",
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
				Name:     "redirect_uri",
				Value:    "http://localhost:3000/callback",
				IsSecret: false,
			},
			{
				Name:     "scopes",
				Value:    "openid email profile",
				IsSecret: false,
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
		},
	}

	idpID, err := testutils.CreateIDP(googleIDP)
	ts.Require().NoError(err, "Failed to create Google IDP")
	ts.idpID = idpID
	ts.config.CreatedIdpIDs = append(ts.config.CreatedIdpIDs, idpID)

	// Update flow definition with created IDs
	nodes := googleRegGroupRoleFlow.Nodes.([]map[string]interface{})
	nodes[3]["properties"].(map[string]interface{})["idpId"] = idpID
	nodes[4]["properties"].(map[string]interface{})["assignGroup"] = groupID
	nodes[4]["properties"].(map[string]interface{})["assignRole"] = roleID
	googleRegGroupRoleFlow.Nodes = nodes

	// Create registration flow
	flowID, err := testutils.CreateFlow(googleRegGroupRoleFlow)
	ts.Require().NoError(err, "Failed to create registration flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	googleRegGroupRoleTestApp.RegistrationFlowID = flowID

	// Create test application
	googleRegGroupRoleTestApp.OUID = googleRegGroupRoleTestOUID
	appID, err := testutils.CreateApplication(googleRegGroupRoleTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	googleRegGroupRoleTestAppID = appID
}

func (ts *GoogleRegistrationGroupRoleTestSuite) TearDownTest() {
	// Clean up users created during each test
	if len(ts.config.CreatedUserIDs) > 0 {
		if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
			ts.T().Logf("Failed to cleanup users after test: %v", err)
		}
		// Reset the list for the next test
		ts.config.CreatedUserIDs = []string{}
	}
}

func (ts *GoogleRegistrationGroupRoleTestSuite) TearDownSuite() {
	// Delete test application
	if googleRegGroupRoleTestAppID != "" {
		if err := testutils.DeleteApplication(googleRegGroupRoleTestAppID); err != nil {
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

	// Delete test groups
	for _, groupID := range ts.config.CreatedGroupIDs {
		if err := testutils.DeleteGroup(groupID); err != nil {
			ts.T().Logf("Failed to delete test group during teardown: %v", err)
		}
	}

	// Delete test roles
	for _, roleID := range ts.config.CreatedRoleIDs {
		if err := testutils.DeleteRole(roleID); err != nil {
			ts.T().Logf("Failed to delete test role during teardown: %v", err)
		}
	}

	// Delete test organization unit
	if googleRegGroupRoleTestOUID != "" {
		if err := testutils.DeleteOrganizationUnit(googleRegGroupRoleTestOUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}

	// Clean up any remaining users
	if len(ts.config.CreatedUserIDs) > 0 {
		if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
			ts.T().Logf("Failed to cleanup users during teardown: %v", err)
		}
	}

	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}

	// Stop mock server
	if ts.mockGoogleServer != nil {
		_ = ts.mockGoogleServer.Stop()
		// Wait for port to be released
		time.Sleep(200 * time.Millisecond)
	}
}

func (ts *GoogleRegistrationGroupRoleTestSuite) TestGoogleRegistrationWithGroupAndRoleAssignment() {
	// Step 1: Initiate the flow
	flowStep, err := common.InitiateRegistrationFlow(googleRegGroupRoleTestAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate registration flow")

	// Verify flow status and type
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("REDIRECTION", flowStep.Type, "Expected flow type to be REDIRECT")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	redirectURLStr := flowStep.Data.RedirectURL
	ts.Require().NotEmpty(redirectURLStr, "Redirect URL should not be empty")

	// Step 2: Simulate OAuth flow
	authCode, state, err := testutils.SimulateFederatedOAuthFlow(redirectURLStr)
	ts.Require().NoError(err, "Failed to simulate OAuth flow")
	ts.Require().NotEmpty(authCode, "Authorization code should not be empty")

	// Step 3: Complete the flow
	inputs := map[string]string{"code": authCode, "state": state}
	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete flow")

	// Verify flow completion
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(completeFlowStep.Assertion, "Assertion token should be present")

	// Decode and validate JWT claims
	jwtClaims, err := testutils.DecodeJWT(completeFlowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
	ts.Require().Equal(googleRegGroupRoleEntityType.Name, jwtClaims.UserType, "Expected userType to match")
	ts.Require().Equal(googleRegGroupRoleTestAppID, jwtClaims.Aud, "Expected aud to match application ID")

	// Step 4: Verify user was created
	user, err := testutils.FindUserByAttribute("sub", "google-group-role-user-789")
	ts.Require().NoError(err, "Failed to retrieve user by sub")
	ts.Require().NotNil(user, "User should be found after registration")

	// Store the created user for cleanup
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)

	// Step 5: Verify user was added to the group
	groupMembers, err := testutils.GetGroupMembers(ts.groupID)
	ts.Require().NoError(err, "Failed to retrieve group members")
	ts.Require().NotNil(groupMembers, "Group members should be found")

	// Check if user is in the group members
	hasUser := false
	for _, member := range groupMembers {
		if member.ID == user.ID && member.Type == "user" {
			hasUser = true
			break
		}
	}
	ts.Require().True(hasUser, "User should be a member of the group after provisioning")

	// Step 6: Verify user was assigned the role
	roleAssignments, err := testutils.GetRoleAssignments(ts.roleID)
	ts.Require().NoError(err, "Failed to retrieve role assignments")

	// Check if user has the role assignment
	hasRoleAssignment := false
	for _, assignment := range roleAssignments {
		if assignment.ID == user.ID && assignment.Type == "user" {
			hasRoleAssignment = true
			break
		}
	}
	ts.Require().True(hasRoleAssignment, "User should have the role assigned after provisioning")
}
