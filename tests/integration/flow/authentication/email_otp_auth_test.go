// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// buildEmailOTPFlow assembles an OTP authentication flow delivered over email. It mirrors the SMS
// OTP flow, with the send half swapped for the EmailExecutor, which exercises the email side of the
// channel agnostic OTPExecutor.
func buildEmailOTPFlow() testutils.Flow {
	return testutils.Flow{
		Name:     "Email OTP Auth Flow Test",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_email_otp_test",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_email"},
			{
				"id":   "prompt_email",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_email", "identifier": "email", "type": "EMAIL_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_email", "nextNode": "generate_otp"},
					},
				},
			},
			{
				"id":   "generate_otp",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "OTPExecutor",
					"mode": "generate",
					"inputs": []map[string]interface{}{
						{"ref": "input_email", "identifier": "email", "type": "EMAIL_INPUT", "required": true},
					},
				},
				"onSuccess": "email_send",
			},
			{
				"id":         "email_send",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"emailTemplate": "OTP"},
				"executor": map[string]interface{}{
					"name": "EmailExecutor",
					"mode": "send",
					"inputs": []map[string]interface{}{
						{"ref": "input_email", "identifier": "email", "type": "EMAIL_INPUT", "required": true},
					},
				},
				"onSuccess": "prompt_otp",
			},
			{
				"id":   "prompt_otp",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_otp", "identifier": "otp", "type": "OTP_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_otp", "nextNode": "verify_otp"},
					},
				},
			},
			{
				"id":        "verify_otp",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "OTPExecutor", "mode": "verify"},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}

var (
	emailOTPTestOU = testutils.OrganizationUnit{
		Handle:      "email-otp-auth-test-ou",
		Name:        "Email OTP Auth Test OU",
		Description: "Organization unit for email OTP authentication flow tests",
	}

	emailOTPEntityType = testutils.UserType{
		Name: "email_otp_user",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"email":    map[string]interface{}{"type": "string"},
		},
	}
)

type EmailOTPAuthFlowTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig

	mockSMTP      *testutils.MockSMTPServer
	appID         string
	entityTypeID  string
	testEmail     string
	originalEmail interface{}
}

func TestEmailOTPAuthFlowTestSuite(t *testing.T) {
	suite.Run(t, new(EmailOTPAuthFlowTestSuite))
}

func (ts *EmailOTPAuthFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}
	ts.testEmail = common.GenerateUniqueUsername("emailotpuser") + "@example.com"

	ouID, err := testutils.CreateOrganizationUnit(emailOTPTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	emailOTPTestOU.ID = ouID

	emailOTPEntityType.OUID = ouID
	entityTypeID, err := testutils.CreateUserType(emailOTPEntityType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = entityTypeID

	userIDs, err := testutils.CreateMultipleUsers(testutils.User{
		OUID: ouID,
		Type: emailOTPEntityType.Name,
		Attributes: json.RawMessage(`{
			"username": "emailotpuser",
			"email": "` + ts.testEmail + `"
		}`),
	})
	ts.Require().NoError(err, "Failed to create test user")
	ts.config.CreatedUserIDs = userIDs

	// Point the server at a mock SMTP server so the delivered OTP can be read back. Start binds the
	// listener before returning, so the port is reachable as soon as it succeeds.
	ts.mockSMTP = testutils.NewMockSMTPServer(0)
	ts.Require().NoError(ts.mockSMTP.Start(), "Failed to start mock SMTP server")

	// The distribution ships a populated email section and a patch replaces the whole key rather than
	// merging into it, so keep the original to restore in teardown.
	originalEmail, err := testutils.ReadDeploymentConfigKey("email")
	ts.Require().NoError(err, "Failed to read the existing email config")
	ts.originalEmail = originalEmail

	ts.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		"email": map[string]interface{}{
			"smtp": map[string]interface{}{
				"host":                  "localhost",
				"port":                  ts.mockSMTP.GetPort(),
				"from_address":          "noreply@thunderid.test",
				"enable_start_tls":      false,
				"enable_authentication": false,
			},
		},
	}), "Failed to patch email config")
	ts.Require().NoError(testutils.RestartServer(), "Failed to restart server with email config")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(), "Failed to re-obtain admin token after restart")

	flowID, err := testutils.CreateFlow(buildEmailOTPFlow())
	ts.Require().NoError(err, "Failed to create email OTP flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                      ouID,
		Name:                      "Email OTP Auth Flow Test Application",
		Description:               "Application for testing email OTP authentication flows",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "email_otp_auth_flow_test_client",
		ClientSecret:              "email_otp_auth_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{emailOTPEntityType.Name},
		AuthFlowID:                flowID,
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	})
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID
}

func (ts *EmailOTPAuthFlowTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users during teardown: %v", err)
	}

	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete application during teardown: %v", err)
		}
	}

	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete flow %s during teardown: %v", flowID, err)
		}
	}

	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}

	if emailOTPTestOU.ID != "" {
		if err := testutils.DeleteOrganizationUnit(emailOTPTestOU.ID); err != nil {
			ts.T().Logf("Failed to delete organization unit during teardown: %v", err)
		}
	}

	if ts.mockSMTP != nil {
		if err := ts.mockSMTP.Stop(); err != nil {
			ts.T().Logf("Failed to stop mock SMTP server during teardown: %v", err)
		}
	}

	if err := testutils.PatchDeploymentConfig(map[string]interface{}{
		"email": ts.originalEmail,
	}); err != nil {
		ts.T().Logf("Failed to restore email config during teardown: %v", err)
	}
	if err := testutils.RestartServer(); err != nil {
		ts.T().Logf("Server did not restart cleanly after config restore: %v", err)
	}
	if err := testutils.ObtainAdminAccessToken(); err != nil {
		ts.T().Logf("Failed to re-obtain admin token after restore: %v", err)
	}
}

// submitEmail drives the flow to the OTP prompt for the given address.
func (ts *EmailOTPAuthFlowTestSuite) submitEmail(email string) *common.FlowStep {
	ts.mockSMTP.ClearEmails()

	step, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().True(common.HasInput(step.Data.Inputs, "email"), "Email input should be required")

	step, err = common.CompleteFlow(step.ExecutionID, map[string]string{"email": email},
		"action_email", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the email address")

	return step
}

// waitForOTP polls the mock SMTP server until an OTP email arrives.
func (ts *EmailOTPAuthFlowTestSuite) waitForOTP() string {
	for i := 0; i < 20; i++ {
		if email := ts.mockSMTP.GetLastEmail(); email != nil {
			if otp := email.ExtractOTP(); otp != "" {
				return otp
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return ""
}

// wrongOTP derives a code of the same length as the delivered one, differing in every position, so
// an invalid-OTP test can never accidentally submit the real code.
func wrongOTP(otp string) string {
	wrong := []rune(otp)
	for i, r := range wrong {
		if r == '0' {
			wrong[i] = '1'
		} else {
			wrong[i] = '0'
		}
	}
	return string(wrong)
}

// TestEmailOTPAuthFlow_Success authenticates end to end with a code delivered by email.
func (ts *EmailOTPAuthFlowTestSuite) TestEmailOTPAuthFlow_Success() {
	step := ts.submitEmail(ts.testEmail)
	ts.Require().True(common.HasInput(step.Data.Inputs, "otp"), "OTP input should be requested")

	otp := ts.waitForOTP()
	ts.Require().NotEmpty(otp, "An OTP should have been delivered by email")

	finalStep, err := common.CompleteFlow(step.ExecutionID, map[string]string{"otp": otp},
		"action_otp", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OTP")
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().Nil(finalStep.Error, "Error should be nil for a successful authentication")
	ts.Require().NotEmpty(finalStep.Assertion, "A JWT assertion should be returned")

	claims, err := testutils.ValidateJWTAssertionFields(finalStep.Assertion, ts.appID,
		emailOTPEntityType.Name, emailOTPTestOU.ID, emailOTPTestOU.Name, emailOTPTestOU.Handle)
	ts.Require().NoError(err, "Failed to validate JWT assertion fields")
	ts.Require().NotNil(claims, "JWT claims should not be nil")
}

// TestEmailOTPAuthFlow_InvalidOTP rejects a wrong code and re-prompts.
func (ts *EmailOTPAuthFlowTestSuite) TestEmailOTPAuthFlow_InvalidOTP() {
	step := ts.submitEmail(ts.testEmail)
	otp := ts.waitForOTP()
	ts.Require().NotEmpty(otp, "An OTP should have been delivered by email")

	finalStep, err := common.CompleteFlow(step.ExecutionID, map[string]string{"otp": wrongOTP(otp)},
		"action_otp", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit an invalid OTP")
	ts.Require().Equal("INCOMPLETE", finalStep.FlowStatus, "An invalid OTP should remain retryable")
	ts.Require().NotNil(finalStep.Error, "An invalid OTP should report an error")
	ts.Require().Empty(finalStep.Assertion, "No assertion should be issued for an invalid OTP")
	ts.Require().True(common.HasInput(finalStep.Data.Inputs, "otp"), "The OTP input should be re-prompted")
}

// TestEmailOTPAuthFlow_RetryAfterInvalidOTP confirms the correct code still works after a wrong one.
func (ts *EmailOTPAuthFlowTestSuite) TestEmailOTPAuthFlow_RetryAfterInvalidOTP() {
	step := ts.submitEmail(ts.testEmail)
	otp := ts.waitForOTP()
	ts.Require().NotEmpty(otp, "An OTP should have been delivered by email")

	retryStep, err := common.CompleteFlow(step.ExecutionID, map[string]string{"otp": wrongOTP(otp)},
		"action_otp", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit an invalid OTP")
	ts.Require().Equal("INCOMPLETE", retryStep.FlowStatus, "An invalid OTP should remain retryable")

	finalStep, err := common.CompleteFlow(step.ExecutionID, map[string]string{"otp": otp},
		"action_otp", retryStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to retry with the valid OTP")
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected COMPLETE after retrying with the valid OTP")
	ts.Require().NotEmpty(finalStep.Assertion, "A JWT assertion should be returned on a successful retry")
}

// TestEmailOTPAuthFlow_NonExistentEmail confirms an unknown address does not authenticate and does
// not receive a code.
func (ts *EmailOTPAuthFlowTestSuite) TestEmailOTPAuthFlow_NonExistentEmail() {
	ts.mockSMTP.ClearEmails()

	step, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	finalStep, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"email": "no-such-user@example.com"}, "action_email", step.ChallengeToken)
	ts.Require().NoError(err, "An unknown address should be reported in band, not as a failed request")
	ts.Require().Equal("ERROR", finalStep.FlowStatus, "An unknown email address must not authenticate")
	ts.Require().NotNil(finalStep.Error, "The flow should report why it stopped")
	ts.Require().Empty(finalStep.Assertion, "No assertion should be issued for an unknown address")

	// Absence cannot be signalled, so allow the window in which a send would have happened.
	time.Sleep(500 * time.Millisecond)
	ts.Require().Nil(ts.mockSMTP.GetLastEmail(), "No email should be sent for an unknown address")
}
