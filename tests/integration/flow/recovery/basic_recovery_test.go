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

package recovery

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	mockSMTPPort = 2525
)

// emailPatch configures Thunder to use the mock SMTP server.
var emailPatch = map[string]interface{}{
	"email": map[string]interface{}{
		"smtp": map[string]interface{}{
			"host":                  "localhost",
			"port":                  mockSMTPPort,
			"from_address":          "noreply@thunder.test",
			"enable_start_tls":      false,
			"enable_authentication": false,
		},
	},
}

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
// Refer to @backend/cmd/server/bootstrap/flows/recovery/recovery_flow_email.json for the flow configuration.
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
	ts.mockSMTP = testutils.NewMockSMTPServer(mockSMTPPort)
	ts.Require().NoError(ts.mockSMTP.Start(), "Failed to start mock SMTP server")
	time.Sleep(100 * time.Millisecond)

	// Patch deployment.yaml to point email at the mock SMTP server and restart
	ts.Require().NoError(testutils.PatchDeploymentConfig(emailPatch), "Failed to patch email config")
	ts.Require().NoError(testutils.RestartServer(), "Failed to restart server with email config")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(), "Failed to re-obtain admin token after restart")

	// Create the email-link recovery flow
	recoveryFlowID, err := testutils.CreateFlow(buildEmailLinkPasswordRecoveryFlow())
	ts.Require().NoError(err, "Failed to create email-link password recovery flow")
	ts.recoveryFlowID = recoveryFlowID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, recoveryFlowID)

	authFlowID, err := testutils.GetFlowIDByHandle("default-basic-flow", "AUTHENTICATION")
	ts.Require().NoError(err, "Failed to get default auth flow ID")
	ts.authFlowID = authFlowID

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
