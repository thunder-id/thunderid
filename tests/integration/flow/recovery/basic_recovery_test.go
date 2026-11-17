// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package recovery

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// emailPatchRemove removes the email config to restore the original state.
var emailPatchRemove = map[string]interface{}{
	"email": map[string]interface{}{},
}

var (
	basicRecoveryOU = testutils.OrganizationUnit{
		Handle:      "basic-recovery-test-ou",
		Name:        "Basic Recovery Flow Test OU",
		Description: "Organization unit for basic (email-link) recovery flow testing",
	}

	basicRecoveryUserSchema = testutils.UserType{
		Name: "basic-recovery-user-type",
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
		AllowSelfRegistration: false,
	}
)

// EmailLinkPasswordRecoveryTestSuite tests the email-link password recovery flow.
// Refer to the email-link recovery flow (handle: default-flow)
// in @backend/cmd/server/bootstrap/01-default-resources.yaml for the flow configuration.
type EmailLinkPasswordRecoveryTestSuite struct {
	suite.Suite
	config         *common.TestSuiteConfig
	mockSMTP       *testutils.MockSMTPServer
	userSchemaID   string
	testOUID       string
	testAppID      string
	recoveryFlowID string
	authFlowID     string
	testUserID     string
	testUsername   string
	testPassword   string
}

func TestEmailLinkPasswordRecoveryTestSuite(t *testing.T) {
	suite.Run(t, new(EmailLinkPasswordRecoveryTestSuite))
}

func (ts *EmailLinkPasswordRecoveryTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}
	ts.testPassword = "OldPassword123!"
	ts.testUsername = common.GenerateUniqueUsername("recoveryuser")

	// Create OU
	ouID, err := testutils.CreateOrganizationUnit(basicRecoveryOU)
	ts.Require().NoError(err, "Failed to create test OU")
	ts.testOUID = ouID

	// Create user schema
	basicRecoveryUserSchema.OUID = ts.testOUID
	schemaID, err := testutils.CreateUserType(basicRecoveryUserSchema)
	ts.Require().NoError(err, "Failed to create user schema")
	ts.userSchemaID = schemaID

	// Create a test user with known credentials
	userID, err := testutils.CreateMultipleUsers(testutils.User{
		OUID: ts.testOUID,
		Type: basicRecoveryUserSchema.Name,
		Attributes: json.RawMessage(`{
			"username": "` + ts.testUsername + `",
			"password": "` + ts.testPassword + `",
			"email":    "` + ts.testUsername + `@example.com"
		}`),
	})
	ts.Require().NoError(err, "Failed to create test user")
	ts.testUserID = userID[0]

	// Start mock SMTP server
	ts.mockSMTP = testutils.NewMockSMTPServer(0)
	ts.Require().NoError(ts.mockSMTP.Start(), "Failed to start mock SMTP server")
	time.Sleep(100 * time.Millisecond)

	emailPatch := map[string]interface{}{
		"email": map[string]interface{}{
			"smtp": map[string]interface{}{
				"host":                  "localhost",
				"port":                  ts.mockSMTP.GetPort(),
				"from_address":          "noreply@thunder.test",
				"enable_start_tls":      false,
				"enable_authentication": false,
			},
		},
	}

	// Patch deployment.yaml to point email at the mock SMTP server and restart
	ts.Require().NoError(testutils.PatchDeploymentConfig(emailPatch), "Failed to patch email config")
	ts.Require().NoError(testutils.RestartServer(), "Failed to restart server with email config")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(), "Failed to re-obtain admin token after restart")

	// Create the email-link recovery flow
	recoveryFlowID, err := testutils.CreateFlow(buildEmailLinkPasswordRecoveryFlow())
	ts.Require().NoError(err, "Failed to create email-link password recovery flow")
	ts.recoveryFlowID = recoveryFlowID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, recoveryFlowID)

	authFlowID, err := testutils.CreateIsolatedAuthFlow("basic-recovery-isolated-auth")
	ts.Require().NoError(err, "Failed to create isolated auth flow")
	ts.authFlowID = authFlowID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, authFlowID)

	// Create test application with both auth and recovery flows enabled
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                      ts.testOUID,
		Name:                      "Basic Recovery Flow Test App",
		Description:               "Application for testing email-link recovery",
		IsRegistrationFlowEnabled: false,
		IsRecoveryFlowEnabled:     true,
		ClientID:                  "basic_recovery_test_client",
		ClientSecret:              "basic_recovery_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{basicRecoveryUserSchema.Name},
		AuthFlowID:                ts.authFlowID,
		RecoveryFlowID:            ts.recoveryFlowID,
	})
	ts.Require().NoError(err, "Failed to create test application")
	ts.testAppID = appID
}

func (ts *EmailLinkPasswordRecoveryTestSuite) TearDownSuite() {
	// Delete test application
	if ts.testAppID != "" {
		if err := testutils.DeleteApplication(ts.testAppID); err != nil {
			ts.T().Logf("teardown: failed to delete test app: %v", err)
		}
	}

	// Delete test user
	if ts.testUserID != "" {
		if err := testutils.CleanupUsers([]string{ts.testUserID}); err != nil {
			ts.T().Logf("teardown: failed to delete test user: %v", err)
		}
	}

	// Delete created flows
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("teardown: failed to delete flow %s: %v", flowID, err)
		}
	}

	// Delete user schema
	if ts.userSchemaID != "" {
		if err := testutils.DeleteUserType(ts.userSchemaID); err != nil {
			ts.T().Logf("teardown: failed to delete user schema: %v", err)
		}
	}

	// Delete OU
	if ts.testOUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.testOUID); err != nil {
			ts.T().Logf("teardown: failed to delete OU: %v", err)
		}
	}

	// Stop mock SMTP server
	if ts.mockSMTP != nil {
		if err := ts.mockSMTP.Stop(); err != nil {
			ts.T().Logf("teardown: failed to stop mock SMTP server: %v", err)
		}
	}

	// Restore email config and restart server
	if err := testutils.PatchDeploymentConfig(emailPatchRemove); err != nil {
		ts.T().Logf("teardown: failed to restore email config: %v", err)
	}
	if err := testutils.RestartServer(); err != nil {
		ts.T().Logf("teardown: server did not restart cleanly after config restore: %v", err)
	}
	if err := testutils.ObtainAdminAccessToken(); err != nil {
		ts.T().Logf("teardown: failed to re-obtain admin token after restore: %v", err)
	}
}

// TestBasicRecoveryFlow_Success tests the full happy-path email-link recovery flow.
// The flow: prompt_username → identify_user → generate_token → send_email →
//
//	email_sent_status → verify_token → prompt_new_password → set_credential → complete
func (ts *EmailLinkPasswordRecoveryTestSuite) TestBasicRecoveryFlow_Success() {
	ts.mockSMTP.ClearEmails()
	newPassword := "NewPassword456!"

	// Step 1: Initiate recovery flow — engine stops at prompt_username.
	flowStep, err := common.InitiateRecoveryFlow(ts.testAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate recovery flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().Equal("VIEW", flowStep.Type)
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "username"),
		"Expected username input at prompt_username")

	// Step 2: Submit username — engine runs identify_user, generate_token, send_email,
	// then stops at email_sent_status (advancing CurrentNode to verify_recovery_token).
	flowStep, err = common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"username": ts.testUsername},
		"action_submit_username",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err, "Failed to submit username")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"Expected INCOMPLETE after username submission (at email_sent_status)")

	// Wait for mock SMTP to receive the email.
	time.Sleep(1 * time.Second)

	email := ts.mockSMTP.GetLastEmail()
	ts.Require().NotNil(email, "Expected recovery email to be captured by mock SMTP server")

	recoveryLink := email.ExtractRecoveryLink()
	ts.Require().NotEmpty(recoveryLink, "Expected recovery link in email body")

	recoveryToken := testutils.ExtractQueryParam(recoveryLink, "inviteToken")
	ts.Require().NotEmpty(recoveryToken, "Expected inviteToken query param in recovery link")

	// Step 3: Submit recovery token — engine resumes at verify_recovery_token,
	// verifies token, then stops at prompt_new_password.
	flowStep, err = common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"inviteToken": recoveryToken},
		"",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err, "Failed to submit recovery token")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"Expected INCOMPLETE after token verification (at prompt_new_password)")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "password"),
		"Expected password input at prompt_new_password")

	// Step 4: Submit new password — engine runs set_credential, then recovery_complete
	// (display-only → end) → COMPLETE.
	flowStep, err = common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"password": newPassword},
		"action_submit_password",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err, "Failed to submit new password")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Expected COMPLETE after setting new password")

	// Step 5: Verify old password no longer works.
	ok, err := testutils.AuthenticateWithCredential("username", ts.testUsername, "password", ts.testPassword)
	ts.Require().NoError(err)
	ts.Require().False(ok, "Old password should be rejected after recovery")

	// Step 6: Verify new password works.
	ok, err = testutils.AuthenticateWithCredential("username", ts.testUsername, "password", newPassword)
	ts.Require().NoError(err)
	ts.Require().True(ok, "New password should authenticate successfully after recovery")

	// Restore password for other tests.
	ts.testPassword = newPassword
}

// TestBasicRecoveryFlow_PasswordSentWithUsername tests that a password sent along with the username
// is not used for the new password step. Opening the recovery link still prompts for the new password.
func (ts *EmailLinkPasswordRecoveryTestSuite) TestBasicRecoveryFlow_PasswordSentWithUsername() {
	ts.mockSMTP.ClearEmails()
	earlyPassword := "SentWithUsername789!"
	newPassword := "NewPassword789!"

	flowStep, err := common.InitiateRecoveryFlow(ts.testAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate recovery flow")

	flowStep, err = common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"username": ts.testUsername, "password": earlyPassword},
		"action_submit_username",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err, "Failed to submit username")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	email := common.AwaitEmail(ts.Require(), ts.mockSMTP)
	recoveryToken := testutils.ExtractQueryParam(email.ExtractRecoveryLink(), "inviteToken")
	ts.Require().NotEmpty(recoveryToken, "Expected inviteToken query param in recovery link")

	// Open the link the way the gate does: the execution ID and the token, nothing else.
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{"inviteToken": recoveryToken}, "", "")
	ts.Require().NoError(err, "Failed to submit recovery token")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Opening the link stops at the new password prompt")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "password"),
		"Expected password input at prompt_new_password")

	ok, err := testutils.AuthenticateWithCredential("username", ts.testUsername, "password", earlyPassword)
	ts.Require().NoError(err)
	ts.Require().False(ok, "The password sent with the username is not set")
	ok, err = testutils.AuthenticateWithCredential("username", ts.testUsername, "password", ts.testPassword)
	ts.Require().NoError(err)
	ts.Require().True(ok, "The current password still works until a new one is submitted")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{"password": newPassword},
		"action_submit_password", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit new password")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Expected COMPLETE after setting new password")

	ok, err = testutils.AuthenticateWithCredential("username", ts.testUsername, "password", newPassword)
	ts.Require().NoError(err)
	ts.Require().True(ok, "The password submitted at the prompt is set")
	ts.testPassword = newPassword
}

// TestBasicRecoveryFlow_PasswordSentWithFirstRequest tests that a password sent with the request that starts
// recovery is not used for the new password step.
func (ts *EmailLinkPasswordRecoveryTestSuite) TestBasicRecoveryFlow_PasswordSentWithFirstRequest() {
	ts.mockSMTP.ClearEmails()
	flowStep, err := common.InitiateRecoveryFlow(ts.testAppID, false,
		map[string]string{"username": ts.testUsername, "password": "SentOnStart789!"}, "action_submit_username")
	ts.Require().NoError(err, "Failed to initiate recovery flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	ts.requireLinkAsksForNewPassword(flowStep, "SentOnStart789!")
}

// TestBasicRecoveryFlow_PasswordSentThroughSignInForgotPassword tests that a password sent with the sign-in
// flow's forgot password action, which calls the recovery flow, is not used for the new password step.
func (ts *EmailLinkPasswordRecoveryTestSuite) TestBasicRecoveryFlow_PasswordSentThroughSignInForgotPassword() {
	ts.mockSMTP.ClearEmails()
	authFlowID, err := testutils.CreateFlow(buildSignInFlowCallingRecovery(ts.recoveryFlowID))
	ts.Require().NoError(err, "Failed to create the sign-in flow")
	defer func() {
		if err := testutils.DeleteFlow(authFlowID); err != nil {
			ts.T().Logf("Failed to delete the sign-in flow: %v", err)
		}
	}()
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                  ts.testOUID,
		Name:                  "Basic Recovery Forgot Password Test App",
		IsRecoveryFlowEnabled: true,
		ClientID:              "basic_recovery_forgot_password_client",
		ClientSecret:          "basic_recovery_forgot_password_secret",
		RedirectURIs:          []string{"http://localhost:3000/callback"},
		AllowedUserTypes:      []string{basicRecoveryUserSchema.Name},
		AuthFlowID:            authFlowID,
		RecoveryFlowID:        ts.recoveryFlowID,
	})
	ts.Require().NoError(err, "Failed to create the application")
	defer func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete the application: %v", err)
		}
	}()

	flowStep, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate the sign-in flow")
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"username": ts.testUsername, "password": "SentWithForgot789!"}, "action_forgot_password",
		flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to choose forgot password")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	ts.requireLinkAsksForNewPassword(flowStep, "SentWithForgot789!")
}

// requireLinkAsksForNewPassword opens the recovery link the way the gate does, checks that it asks for a new
// password and that sentPassword was not set, then completes recovery.
func (ts *EmailLinkPasswordRecoveryTestSuite) requireLinkAsksForNewPassword(flowStep *common.FlowStep,
	sentPassword string) {
	email := common.AwaitEmail(ts.Require(), ts.mockSMTP)
	recoveryToken := testutils.ExtractQueryParam(email.ExtractRecoveryLink(), "inviteToken")
	ts.Require().NotEmpty(recoveryToken, "Expected inviteToken query param in recovery link")

	flowStep, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"inviteToken": recoveryToken}, "", "")
	ts.Require().NoError(err, "Failed to submit recovery token")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Opening the link stops at the new password prompt")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "password"),
		"Expected password input at prompt_new_password")

	ok, err := testutils.AuthenticateWithCredential("username", ts.testUsername, "password", sentPassword)
	ts.Require().NoError(err)
	ts.Require().False(ok, "The password sent before the link was opened is not set")

	newPassword := "Chosen" + sentPassword
	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{"password": newPassword},
		"action_submit_password", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit new password")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Expected COMPLETE after setting new password")
	ts.testPassword = newPassword
}

// buildSignInFlowCallingRecovery returns a sign-in flow whose forgot password action calls the given recovery
// flow.
func buildSignInFlowCallingRecovery(recoveryFlowID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Basic Recovery Forgot Password Sign-in Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "basic-recovery-forgot-password-signin",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
							{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT",
								"required": true},
						},
						"action": map[string]interface{}{"ref": "action_sign_in", "nextNode": "credentials_auth"},
					},
					{
						"action": map[string]interface{}{"ref": "action_forgot_password", "nextNode": "call_recovery"},
					},
				},
			},
			{
				"id":        "credentials_auth",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor"},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{
				"id":        "call_recovery",
				"type":      "CALL",
				"flow":      map[string]interface{}{"ref": recoveryFlowID},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}

// TestBasicRecoveryFlow_UnknownUsername tests that an unknown username still reaches
// email_sent_status (anti-enumeration: same status regardless of username validity).
func (ts *EmailLinkPasswordRecoveryTestSuite) TestBasicRecoveryFlow_UnknownUsername() {
	ts.mockSMTP.ClearEmails()

	// Initiate
	flowStep, err := common.InitiateRecoveryFlow(ts.testAppID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	// Submit a username that does not exist
	flowStep, err = common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"username": "nonexistent_user_xyz_12345"},
		"action_submit_username",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err)

	// The flow must still return INCOMPLETE (email_sent_status) — not an error.
	// This prevents username enumeration.
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"Unknown username must still show email_sent_status (anti-enumeration)")

	// No email should have been sent (user does not exist).
	time.Sleep(500 * time.Millisecond)
	ts.Require().Nil(ts.mockSMTP.GetLastEmail(),
		"No email should be sent for a non-existent username")
}

func (ts *EmailLinkPasswordRecoveryTestSuite) TestBasicRecoveryFlow_SendsToStoredEmail() {
	ts.mockSMTP.ClearEmails()

	flowStep := ts.submitUsernameWithEmail(ts.testUsername, "other@example.com")

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	email := common.AwaitEmail(ts.Require(), ts.mockSMTP)
	ts.Equal([]string{ts.testUsername + "@example.com"}, email.To, "The recovery email goes to the stored email")
}

func (ts *EmailLinkPasswordRecoveryTestSuite) TestBasicRecoveryFlow_UserWithoutEmail() {
	ts.mockSMTP.ClearEmails()
	username := common.GenerateUniqueUsername("recoverynoemail")
	userIDs, err := testutils.CreateMultipleUsers(testutils.User{
		OUID: ts.testOUID,
		Type: basicRecoveryUserSchema.Name,
		Attributes: json.RawMessage(`{
			"username": "` + username + `",
			"password": "NoEmail123!"
		}`),
	})
	ts.Require().NoError(err, "Failed to create the user without an email")
	defer func() {
		if err := testutils.CleanupUsers(userIDs); err != nil {
			ts.T().Logf("Failed to delete the user without an email: %v", err)
		}
	}()

	flowStep := ts.submitUsernameWithEmail(username, "other@example.com")

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	time.Sleep(500 * time.Millisecond)
	ts.Nil(ts.mockSMTP.GetLastEmail(), "No email should be sent")
}

// submitUsernameWithEmail starts a recovery flow and submits the username along with an email.
func (ts *EmailLinkPasswordRecoveryTestSuite) submitUsernameWithEmail(username, email string) *common.FlowStep {
	flowStep, err := common.InitiateRecoveryFlow(ts.testAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate recovery flow")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"username": username, "email": email}, "action_submit_username", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit username")

	return flowStep
}

// TestBasicRecoveryFlow_InvalidToken tests that submitting a wrong recovery token
// causes the token verification to fail and the flow to return an error/incomplete status.
func (ts *EmailLinkPasswordRecoveryTestSuite) TestBasicRecoveryFlow_InvalidToken() {
	ts.mockSMTP.ClearEmails()

	// Initiate + submit valid username to reach email_sent_status.
	flowStep, err := common.InitiateRecoveryFlow(ts.testAppID, false, nil, "")
	ts.Require().NoError(err)

	flowStep, err = common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"username": ts.testUsername},
		"action_submit_username",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	// Wait briefly; we won't use the real token.
	time.Sleep(500 * time.Millisecond)
	ts.mockSMTP.ClearEmails()

	// Submit a wrong recovery token.
	flowStep, err = common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"inviteToken": "invalid-token-abcdef"},
		"",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err)

	// Token mismatch must not complete the flow.
	ts.Require().NotEqual("COMPLETE", flowStep.FlowStatus,
		"Invalid token must not complete the recovery flow")
}

// TestBasicRecoveryFlow_RecoveryDisabledApp tests that initiating recovery on an app
// without recovery enabled returns an error.
func (ts *EmailLinkPasswordRecoveryTestSuite) TestBasicRecoveryFlow_RecoveryDisabledApp() {
	// Create a temporary app with recovery disabled.
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                  ts.testOUID,
		Name:                  "No-Recovery App",
		IsRecoveryFlowEnabled: false,
		ClientID:              "no_recovery_client",
		ClientSecret:          "no_recovery_secret",
		RedirectURIs:          []string{"http://localhost:3000/callback"},
		AllowedUserTypes:      []string{basicRecoveryUserSchema.Name},
		AuthFlowID:            ts.authFlowID,
	})
	ts.Require().NoError(err, "Failed to create no-recovery app")
	defer func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete no-recovery app: %v", err)
		}
	}()

	// Attempting to start a RECOVERY flow on this app must fail.
	_, err = common.InitiateRecoveryFlow(appID, false, nil, "")
	ts.Require().Error(err, "Expected error when recovery is disabled for the app")
}

// buildEmailLinkPasswordRecoveryFlow returns the email-link password recovery flow definition.
// Flow: prompt_username → identify_user → generate_recovery_token → send_recovery_email →
//
//	email_sent_status → verify_recovery_token → prompt_new_password → set_credential → recovery_complete → end
func buildEmailLinkPasswordRecoveryFlow() testutils.Flow {
	return testutils.Flow{
		Name:     "Email Link Password Recovery Flow Test",
		Handle:   "email-link-based-password-recovery-test",
		FlowType: "RECOVERY",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "prompt_username",
			},
			{
				"id":   "prompt_username",
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
						},
						"action": map[string]interface{}{
							"ref":      "action_submit_username",
							"nextNode": "identify_user",
						},
					},
				},
			},
			{
				"id":   "identify_user",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "IdentifyingExecutor",
					"mode": "identify",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_username",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
					},
				},
				"onSuccess": "generate_recovery_token",
				"onFailure": "email_sent_status",
			},
			{
				"id":   "generate_recovery_token",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "InviteExecutor",
					"mode": "generate",
				},
				"onSuccess": "send_recovery_email",
			},
			{
				"id":   "send_recovery_email",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"emailTemplate": "PASSWORD_RECOVERY",
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
				"onSuccess": "email_sent_status",
				"onFailure": "email_sent_status",
			},
			{
				"id":   "email_sent_status",
				"type": "PROMPT",
				"next": "verify_recovery_token",
			},
			{
				"id":   "verify_recovery_token",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "InviteExecutor",
					"mode": "verify",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_recovery_token",
							"identifier": "inviteToken",
							"type":       "HIDDEN",
							"required":   true,
						},
					},
				},
				"onSuccess": "prompt_new_password",
			},
			{
				"id":   "prompt_new_password",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_new_password",
								"identifier": "password",
								"type":       "PASSWORD_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_submit_password",
							"nextNode": "set_credential",
						},
					},
				},
			},
			{
				"id":   "set_credential",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialSetter",
				},
				"onSuccess": "recovery_complete",
			},
			{
				"id":   "recovery_complete",
				"type": "PROMPT",
				"next": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}
}
