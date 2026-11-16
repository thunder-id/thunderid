// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package registration

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

var (
	selfInviteRegistrationFlow = testutils.Flow{
		Name:     "Self Invite Registration Test Flow",
		FlowType: "REGISTRATION",
		Handle:   "reg_flow_self_invite_test",
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
				"onSuccess":    "prompt_email",
				"onIncomplete": "prompt_email",
			},
			{
				"id":   "prompt_email",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_email",
								"identifier": "email",
								"type":       "EMAIL_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_submit_email",
							"nextNode": "invite_generate",
						},
					},
				},
			},
			{
				"id":   "invite_generate",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "InviteExecutor",
					"mode": "generate",
				},
				"onSuccess": "send_registration_email",
			},
			{
				"id":   "send_registration_email",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"emailTemplate": "SELF_REGISTRATION",
				},
				"executor": map[string]interface{}{
					"name": "EmailExecutor",
					"mode": "send",
					"inputs": []map[string]interface{}{
						{
							"identifier": "email",
							"type":       "EMAIL_INPUT",
							"required":   true,
						},
					},
				},
				"onSuccess": "invite_sent",
			},
			{
				"id":      "invite_sent",
				"type":    "PROMPT",
				"message": "Invitation sent",
				"next":    "invite_verify",
			},
			{
				"id":   "invite_verify",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "InviteExecutor",
					"mode": "verify",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_invite_token",
							"identifier": "inviteToken",
							"type":       "HIDDEN",
							"required":   true,
						},
					},
				},
				"onSuccess": "prompt_user_details",
			},
			{
				"id":   "prompt_user_details",
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
							"ref":      "action_submit_details",
							"nextNode": "provisioning",
						},
					},
				},
			},
			{
				"id":   "provisioning",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "ProvisioningExecutor",
				},
				"onSuccess":    "auth_assert",
				"onIncomplete": "prompt_schema_attrs",
			},
			{
				"id":   "prompt_schema_attrs",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{},
						"action": map[string]interface{}{
							"ref":      "action_schema_attrs",
							"nextNode": "provisioning",
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

	selfInviteRegTestUserType = testutils.UserType{
		Name: "self_invite_reg_test_user",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"email": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
		},
		AllowSelfRegistration: true,
	}

	selfInviteRegTestOU = testutils.OrganizationUnit{
		Handle:      "self-invite-reg-test-ou",
		Name:        "Self Invite Reg Test OU",
		Description: "Organization unit for self invite registration tests",
	}
)

// SelfInviteRegistrationTestSuite tests registration where the email is confirmed through an invite link
// before the user details are collected.
type SelfInviteRegistrationTestSuite struct {
	suite.Suite
	config           *common.TestSuiteConfig
	mockSMTP         *testutils.MockSMTPServer
	ouID             string
	appID            string
	userTypeID       string
	originalPatchSet bool
}

func TestSelfInviteRegistrationTestSuite(t *testing.T) {
	suite.Run(t, new(SelfInviteRegistrationTestSuite))
}

func (ts *SelfInviteRegistrationTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ts.mockSMTP = testutils.NewMockSMTPServer(0)
	ts.Require().NoError(ts.mockSMTP.Start(), "Failed to start mock SMTP server")

	patch := map[string]interface{}{
		"email": map[string]interface{}{
			"smtp": map[string]interface{}{
				"host":                  "localhost",
				"port":                  ts.mockSMTP.GetPort(),
				"username":              "",
				"password":              "",
				"from_address":          "no-reply@example.com",
				"enable_start_tls":      false,
				"enable_authentication": false,
			},
		},
	}
	if err := testutils.PatchDeploymentConfig(patch); err != nil {
		ts.T().Fatalf("Failed to patch deployment config: %v", err)
	}
	ts.originalPatchSet = true
	if err := testutils.RestartServer(); err != nil {
		ts.T().Fatalf("Failed to restart server with SMTP configuration: %v", err)
	}
	if err := testutils.ObtainAdminAccessToken(); err != nil {
		ts.T().Fatalf("Failed to re-obtain admin token after restart: %v", err)
	}

	ouID, err := testutils.CreateOrganizationUnit(selfInviteRegTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	selfInviteRegTestUserType.OUID = ts.ouID
	userTypeID, err := testutils.CreateUserType(selfInviteRegTestUserType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.userTypeID = userTypeID

	flowID, err := testutils.CreateFlow(selfInviteRegistrationFlow)
	ts.Require().NoError(err, "Failed to create self invite registration flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	authFlowID, err := testutils.CreateIsolatedAuthFlow("self-invite-registration-isolated-auth")
	ts.Require().NoError(err, "Failed to create isolated auth flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, authFlowID)

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                      ts.ouID,
		Name:                      "Self Invite Registration Test Application",
		Description:               "Application for testing self invite registration",
		IsRegistrationFlowEnabled: true,
		ClientID:                  "self_invite_reg_test_client",
		ClientSecret:              "self_invite_reg_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{selfInviteRegTestUserType.Name},
		AuthFlowID:                authFlowID,
		RegistrationFlowID:        flowID,
	})
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID
}

func (ts *SelfInviteRegistrationTestSuite) TearDownSuite() {
	if ts.appID != "" {
		_ = testutils.DeleteApplication(ts.appID)
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		_ = testutils.DeleteFlow(flowID)
	}
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to clean up users: %v", err)
	}
	if ts.userTypeID != "" {
		_ = testutils.DeleteUserType(ts.userTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
	if ts.mockSMTP != nil {
		_ = ts.mockSMTP.Stop()
	}
	if ts.originalPatchSet {
		if err := testutils.UpdateDeploymentConfig("../../resources/deployment.yaml"); err != nil {
			ts.T().Logf("teardown: failed to restore deployment config: %v", err)
		}
		if err := testutils.RestartServer(); err != nil {
			ts.T().Logf("teardown: server did not restart cleanly after config restore: %v", err)
		}
		if err := testutils.ObtainAdminAccessToken(); err != nil {
			ts.T().Logf("teardown: failed to re-obtain admin token after restore: %v", err)
		}
	}
}

// TestSelfInviteRegistration_PasswordSentWithEmail tests that user details sent along with the email are not
// used for the user details step. Opening the invite link still prompts for them.
func (ts *SelfInviteRegistrationTestSuite) TestSelfInviteRegistration_PasswordSentWithEmail() {
	ts.mockSMTP.ClearEmails()
	emailAddr := common.GenerateUniqueUsername("selfinvite") + "@example.com"
	username := common.GenerateUniqueUsername("selfinvite")

	flowStep, err := common.InitiateRegistrationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err)

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"email": emailAddr, "username": username, "password": "SentWithEmail789!"},
		"action_submit_email", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	ts.requireLinkAsksForDetails(flowStep, emailAddr, username, "SentWithEmail789!")
}

// TestSelfInviteRegistration_PasswordSentWithFirstRequest tests that user details sent with the request that
// starts registration are not used for the user details step.
func (ts *SelfInviteRegistrationTestSuite) TestSelfInviteRegistration_PasswordSentWithFirstRequest() {
	ts.mockSMTP.ClearEmails()
	emailAddr := common.GenerateUniqueUsername("selfinvite") + "@example.com"
	username := common.GenerateUniqueUsername("selfinvite")

	flowStep, err := common.InitiateRegistrationFlow(ts.appID, false,
		map[string]string{"email": emailAddr, "username": username, "password": "SentOnStart789!"},
		"action_submit_email")
	ts.Require().NoError(err)

	ts.requireLinkAsksForDetails(flowStep, emailAddr, username, "SentOnStart789!")
}

// requireLinkAsksForDetails opens the invite link of a registration paused after the email step, and requires
// it to ask for the user details. The user is created with the password entered then, not earlyPassword.
func (ts *SelfInviteRegistrationTestSuite) requireLinkAsksForDetails(flowStep *common.FlowStep, emailAddr,
	username, earlyPassword string) {
	ts.T().Helper()
	newPassword := "ChosenAfterLink789!"
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	inviteToken := testutils.ExtractQueryParam(
		common.AwaitEmail(ts.Require(), ts.mockSMTP).ExtractRecoveryLink(), "inviteToken")
	ts.Require().NotEmpty(inviteToken, "Expected inviteToken query param in the invite link")

	// Open the link the way the gate does: the execution ID and the token, nothing else.
	flowStep, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"inviteToken": inviteToken}, "", "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Opening the link stops at the user details prompt")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "password"), "Expected the password to be asked for")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "username"), "Expected the username to be asked for")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"username": username, "password": newPassword}, "action_submit_details",
		flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)

	user, err := testutils.FindUserByAttribute("email", emailAddr)
	ts.Require().NoError(err)
	ts.Require().NotNil(user)
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)

	ok, err := testutils.AuthenticateWithCredential("username", username, "password", earlyPassword)
	ts.Require().NoError(err)
	ts.False(ok, "The password sent before the link is not set")
	ok, err = testutils.AuthenticateWithCredential("username", username, "password", newPassword)
	ts.Require().NoError(err)
	ts.True(ok, "The password entered after opening the link is set")
}
