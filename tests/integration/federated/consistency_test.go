// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// Identity consistency across one flow.
//
// A federated sign-in has to describe the identity the flow already holds: the email the identity
// provider asserts is compared with an email the user entered and with the signed-in user's own email.

// errCodeInvalidFederatedUser is the client error a federated sign-in fails with when it names an
// identity other than the one the flow holds.
const errCodeInvalidFederatedUser = "FET-1014"

// passwordThenSignInFlow signs a local user in with a password and then signs in through the
// connection with the given executor, the shape of a flow that steps up with a federated sign-in.
func passwordThenSignInFlow(handle, idpID, executor string) testutils.Flow {
	return testutils.Flow{
		Name:     "Federated Password Then Sign-In Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
							{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_credentials", "nextNode": "credentials_auth"},
					},
				},
			},
			{
				"id":        "credentials_auth",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor"},
				"onSuccess": "federated_auth",
			},
			{
				"id":         "federated_auth",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"idpId": idpID},
				"executor":   map[string]interface{}{"name": executor},
				"onSuccess":  "auth_assert",
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

// emailThenSignInFlow asks for an email and then signs in through the connection.
func emailThenSignInFlow(handle, idpID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Federated Email Then Sign-In Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
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
						"action": map[string]interface{}{"ref": "action_email", "nextNode": "oidc_auth"},
					},
				},
			},
			{
				"id":         "oidc_auth",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"idpId": idpID},
				"executor":   map[string]interface{}{"name": "OIDCAuthExecutor"},
				"onSuccess":  "auth_assert",
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

// signInAfterPassword signs username in with the password of a linked local user and then signs in at the identity provider
// as sub, returning the step the federated callback leads to.
func (s *FederatedMappingSuite) signInAfterPassword(appID, username, sub string) *common.FlowStep {
	s.T().Helper()
	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	s.Require().NoError(err, "failed to initiate the authentication flow")
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username, "password": linkPassword}, "action_credentials",
		step.ChallengeToken)
	s.Require().NoError(err, "failed to submit the credentials")
	s.Require().Equal("REDIRECTION", step.Type, "expected the federated sign-in to redirect, got %+v %+v",
		step, step.Error)

	s.activeSub = sub
	code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization at the identity provider")
	step, err = common.CompleteFlow(step.ExecutionID, map[string]string{"code": code, "state": state}, "",
		step.ChallengeToken)
	s.Require().NoError(err, "a refused sign-in is still a 200 carrying its error")
	return step
}

// assertRejectedAsAnotherIdentity asserts the flow ended on the inconsistent-identity client error.
func (s *FederatedMappingSuite) assertRejectedAsAnotherIdentity(step *common.FlowStep) {
	s.T().Helper()
	s.Require().Equal("ERROR", step.FlowStatus, "the federated sign-in must not be accepted, got %+v", step)
	s.Require().NotNil(step.Error, "expected the terminal step to carry its error, got %+v", step)
	s.Equal(errCodeInvalidFederatedUser, step.Error.Code, "got %+v", step.Error)
	s.Empty(step.Assertion, "a refused sign-in must not issue an assertion")
}

// A federated sign-in describing the user who already signed in with a password completes.
func (s *FederatedMappingSuite) TestSignInAfterPasswordAsTheSameUserCompletes() {
	appID := s.createScenarioApp(
		passwordThenSignInFlow("auth_flow_fed_pwd_same", s.idpID, "OIDCAuthExecutor"), "fed_pwd_same")
	user := s.knownOIDCIdentity()

	step := s.signInAfterPassword(appID, user.Email, user.Sub)

	s.Equal("COMPLETE", step.FlowStatus, "got %+v %+v", step, step.Error)
	s.NotEmpty(step.Assertion, "a completed authentication should carry an assertion")
}

// A federated sign-in asserting an email other than the signed-in user's is another identity.
func (s *FederatedMappingSuite) TestSignInAfterPasswordWithAnotherEmailIsRejected() {
	appID := s.createScenarioApp(
		passwordThenSignInFlow("auth_flow_fed_pwd_email", s.idpID, "OIDCAuthExecutor"), "fed_pwd_email")
	user := s.knownOIDCIdentity()
	username := user.Email
	user.Email = "other-" + user.Email
	s.mockOIDC.AddUser(user)

	s.assertRejectedAsAnotherIdentity(s.signInAfterPassword(appID, username, user.Sub))
}

// The generic OAuth executor applies the same comparison.
func (s *FederatedMappingSuite) TestOAuthSignInAfterPasswordWithAnotherEmailIsRejected() {
	appID := s.createScenarioApp(
		passwordThenSignInFlow("auth_flow_fed_pwd_oauth", s.oauthIDPID, "OAuthExecutor"), "fed_pwd_oauth")
	user := s.knownOAuthIdentity()
	s.mockOAuth.AddUser(&testutils.OAuthUserInfo{Sub: user.Sub, Email: "other-" + user.Email, Name: "OAuth User"})

	s.assertRejectedAsAnotherIdentity(s.signInAfterPassword(appID, user.Email, user.Sub))
}

// An email the user entered earlier in the flow has to match the email the identity provider asserts.
func (s *FederatedMappingSuite) TestSignInNotMatchingTheEnteredEmailIsRejected() {
	appID := s.createScenarioApp(emailThenSignInFlow("auth_flow_fed_entered_email", s.idpID), "fed_entered_email")
	user := s.knownOIDCIdentity()
	s.activeSub = user.Sub

	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	s.Require().NoError(err, "failed to initiate the authentication flow")
	s.Require().True(common.HasInput(step.Data.Inputs, "email"), "expected the flow to ask for an email")
	step, err = common.CompleteFlow(step.ExecutionID, map[string]string{"email": "entered-" + user.Email},
		"action_email", step.ChallengeToken)
	s.Require().NoError(err, "failed to submit the email")
	s.Require().Equal("REDIRECTION", step.Type, "expected a redirection, got %+v", step)

	code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization at the identity provider")
	step, err = common.CompleteFlow(step.ExecutionID, map[string]string{"code": code, "state": state}, "",
		step.ChallengeToken)
	s.Require().NoError(err, "a refused sign-in is still a 200 carrying its error")

	s.assertRejectedAsAnotherIdentity(step)
}

// An identity provider that asserts no email leaves nothing to compare, so the sign-in completes.
func (s *FederatedMappingSuite) TestSignInWithoutAnEmailClaimCompletes() {
	user := s.knownOIDCIdentity()
	user.Email = ""
	s.mockOIDC.AddUser(user)

	step, err := s.authenticateFlow(s.strictAuthAppID, user.Sub)

	s.Require().NoError(err, "the federated callback should be answered")
	s.Equal("COMPLETE", step.FlowStatus, "got %+v %+v", step, step.Error)
}
