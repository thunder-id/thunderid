// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// A password reset runs on the server's authority, on a public path that accepts any caller's bearer token.
// The recovery requests here carry the admin's token, so the line naming the reset user rather than the admin
// shows the actor comes from the reset and not from the request's subject.
func (ts *AuditLogTestSuite) TestAPasswordResetThroughRecoveryNamesTheResetUserAsActor() {
	ouID := ts.createOrganizationUnit("cred-recovery")
	userType := testutils.UserType{
		Handle: ts.prefix + "cred-recovery-person", DisplayName: "Audit Recovery Person", OUID: ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string"},
		},
	}
	userTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err, "failed to create the user type of the reset user")
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(userTypeID) })
	username := ts.prefix + "cred-recovery"
	attributes, err := json.Marshal(map[string]interface{}{
		"username": username, "password": credentialMarker + "Before1!", "email": username + "@example.com",
	})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{OUID: ouID, Type: userType.Handle, Attributes: attributes})
	ts.Require().NoError(err, "failed to create the reset user")
	ts.T().Cleanup(func() { _ = testutils.DeleteUser(userID) })

	smtp := testutils.NewMockSMTPServer(0)
	ts.Require().NoError(smtp.Start(), "failed to start the mock SMTP server")
	ts.T().Cleanup(func() { _ = smtp.Stop() })
	senderID, err := testutils.CreateSMTPEmailProvider(ts.prefix+"cred-recovery-sender", "localhost",
		smtp.GetPort(), "noreply@thunderid.test")
	ts.Require().NoError(err, "failed to create the SMTP email provider")
	ts.T().Cleanup(func() { _ = testutils.DeleteNotificationSender(senderID) })
	flowID, err := testutils.CreateFlow(emailLinkRecoveryFlow(ts.prefix+"cred-recovery-flow", senderID))
	ts.Require().NoError(err, "failed to create the recovery flow")
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })
	// The default authentication flow names a recovery flow of its own, which the application must match.
	authFlowID, err := testutils.CreateIsolatedAuthFlow(ts.prefix + "cred-recovery-auth")
	ts.Require().NoError(err, "failed to create the authentication flow of the application")
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(authFlowID) })
	appID, err := testutils.CreateApplication(testutils.Application{
		Name: ts.prefix + "cred-recovery-app", OUID: ouID, AuthFlowID: authFlowID,
		IsRecoveryFlowEnabled: true, RecoveryFlowID: flowID,
		ClientID: ts.prefix + "cred-recovery-client", ClientSecret: credentialMarker + "ClientSecret1!",
		RedirectURIs: []string{"http://localhost:3000/callback"}, AllowedUserTypes: []string{userType.Handle},
	})
	ts.Require().NoError(err, "failed to create the application that runs the recovery flow")
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err)

	_, step := ts.executeFlow(adminToken, appID, map[string]interface{}{
		"applicationId": appID, "flowType": "RECOVERY",
	})
	_, step = ts.executeFlow(adminToken, appID, map[string]interface{}{
		"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
		"inputs": map[string]string{"username": username}, "action": "action_submit_username",
	})
	var email *testutils.EmailMessage
	ts.Require().Eventually(func() bool {
		email = smtp.GetLastEmail()
		return email != nil
	}, 10*time.Second, 100*time.Millisecond, "the recovery email was not sent")
	inviteToken := testutils.ExtractQueryParam(email.ExtractRecoveryLink(), "inviteToken")
	ts.Require().NotEmpty(inviteToken, "the recovery email carries no recovery link")
	_, step = ts.executeFlow(adminToken, appID, map[string]interface{}{
		"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
		"inputs": map[string]string{"inviteToken": inviteToken},
	})
	ts.Require().True(common.HasInput(step.Data.Inputs, "password"), "the flow must prompt for the new password")
	newPassword := credentialMarker + "After1!"
	reset, step := ts.executeFlow(adminToken, appID, map[string]interface{}{
		"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
		"inputs": map[string]string{"password": newPassword}, "action": "action_submit_password",
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the password reset must complete")
	reauthenticated, err := testutils.AuthenticateWithCredential("username", username, "password", newPassword)
	ts.Require().NoError(err)
	ts.Require().True(reauthenticated, "the reset must have set the new password")

	written := ts.linesPerCall(reset)[0]
	line := ts.onlyLineOf(written, "user.credential.update", "a password reset through recovery")
	userRef := testutils.AuditRef{Type: "user", ID: userID}
	ts.Equal("update", line.Action)
	ts.Equal(userRef, line.Actor, "the reset user is the actor, not the owner of the bearer token")
	ts.Equal(userRef, line.Target)
	ts.Equal(userRef, line.Subject)
	ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome)
	for _, other := range written {
		ts.requireNoCredentialValue(other, "a password reset through recovery")
	}
}

// An administrator-initiated reset records its invalidation, as the administrator, and its completion, both
// targeting the user, so the completion can be traced to the administrator who started it by the user and the
// time. The completion names the user when it is sent from the link without a token, and the administrator when
// the administrator sends it with their own token. Neither line carries the link token or a credential.
func (ts *AuditLogTestSuite) TestAnAdministratorInitiatedResetRecordsBothStepsForTheUser() {
	ouID := ts.createOrganizationUnit("cred-admin-reset")
	userType := testutils.UserType{
		Handle: ts.prefix + "cred-admin-reset-person", DisplayName: "Audit Admin Reset Person", OUID: ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "email"},
		},
	}
	userTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err, "failed to create the user type of the reset user")
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(userTypeID) })

	smtp := testutils.NewMockSMTPServer(0)
	ts.Require().NoError(smtp.Start(), "failed to start the mock SMTP server")
	ts.T().Cleanup(func() { _ = smtp.Stop() })
	senderID, err := testutils.CreateSMTPEmailProvider(ts.prefix+"cred-admin-reset-sender", "localhost",
		smtp.GetPort(), "noreply@thunderid.test")
	ts.Require().NoError(err, "failed to create the SMTP email provider")
	ts.T().Cleanup(func() { _ = testutils.DeleteNotificationSender(senderID) })
	handle := ts.prefix + "cred-admin-reset-flow"
	flowID, err := testutils.CopyFlowWithSenders("default-admin-user-credential-reset-flow", "ADMIN_INITIATED", handle,
		map[string]string{"send_link": senderID}, nil)
	ts.Require().NoError(err, "failed to copy the shipped administrator credential reset flow")
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err)
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}

	for _, test := range []struct {
		name, slug, completingToken string
		adminCompletes              bool
	}{
		{name: "from the link", slug: "link"},
		{name: "with the administrator's token", slug: "admin", completingToken: adminToken, adminCompletes: true},
	} {
		ts.Run(test.name, func() {
			smtp.ClearEmails()
			username := ts.prefix + "cred-admin-reset-" + test.slug
			attributes, err := json.Marshal(map[string]interface{}{
				"username": username, "password": credentialMarker + "Before1!", "email": username + "@example.com",
			})
			ts.Require().NoError(err)
			userID, err := testutils.CreateUser(testutils.User{OUID: ouID, Type: userType.Handle,
				Attributes: attributes})
			ts.Require().NoError(err, "failed to create the reset user")
			ts.T().Cleanup(func() { _ = testutils.DeleteUser(userID) })

			started, step := ts.executeAdminReset(adminToken, map[string]interface{}{
				"flowType": "ADMIN_INITIATED", "handle": handle,
				"inputs": map[string]string{"subject": userID, "credential": "password"},
			})
			ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the flow must wait once the link is sent")
			var email *testutils.EmailMessage
			ts.Require().Eventually(func() bool {
				email = smtp.GetLastEmail()
				return email != nil
			}, 10*time.Second, 100*time.Millisecond, "the reset email was not sent")
			link := email.ExtractRecoveryLink()
			inviteToken := testutils.ExtractQueryParam(link, "inviteToken")
			ts.Require().NotEmpty(inviteToken, "the reset email carries no link")
			_, step = ts.executeAdminReset(test.completingToken, map[string]interface{}{
				"executionId": testutils.ExtractQueryParam(link, "executionId"),
				"inputs":      map[string]string{"inviteToken": inviteToken},
			})
			ts.Require().True(common.HasInput(step.Data.Inputs, "password"),
				"the flow must prompt for the new password")
			completed, step := ts.executeAdminReset(test.completingToken, map[string]interface{}{
				"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
				"inputs": map[string]string{"password": credentialMarker + "After1!"},
				"action": "action_prompt_new_credential",
			})
			ts.Require().Equal("COMPLETE", step.FlowStatus, "the reset must complete")

			perCall := ts.linesPerCall(started, completed)
			userRef := testutils.AuditRef{Type: "user", ID: userID}
			initiated := ts.onlyLineOf(perCall[0], "user.credential.invalidate", "the reset's initiation")
			ts.Equal(adminRef, initiated.Actor, "the initiation names the administrator")
			ts.Equal(userRef, initiated.Target)
			ts.Equal(map[string]string{"result": "succeeded"}, initiated.Outcome)
			ts.Equal(map[string]string{"credential_type": "password"}, ts.details(initiated),
				"the initiation names the credential type, nothing else")
			finished := ts.onlyLineOf(perCall[1], "user.credential.update", "the reset's completion")
			if test.adminCompletes {
				ts.Equal(adminRef, finished.Actor, "a completion sent with the administrator's token names them")
			} else {
				ts.Equal(userRef, finished.Actor, "a completion from the link names the user, not the administrator")
			}
			ts.Equal(userRef, finished.Target)
			ts.Equal(map[string]string{"result": "succeeded"}, finished.Outcome)
			ts.Empty(ts.details(finished), "the completion carries no details")
			for _, written := range perCall {
				for _, line := range written {
					ts.NotContains(line.Raw, inviteToken, "the link token is never recorded")
					ts.NotContains(line.Raw, credentialMarker, "no part of a credential is recorded")
				}
			}
		})
	}
}

// An administrator-initiated reset of an agent's credential records its invalidation, as the administrator, and its
// completion, both targeting the agent. The owner completes it from the link without a token, so the completion
// names the owner. Neither line carries the link token or a credential.
func (ts *AuditLogTestSuite) TestAnAdministratorInitiatedAgentResetRecordsBothStepsForTheAgent() {
	ouID := ts.createOrganizationUnit("cred-agent-reset")
	ownerType := testutils.UserType{
		Handle: ts.prefix + "cred-agent-reset-owner", DisplayName: "Audit Agent Reset Owner", OUID: ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"email":    map[string]interface{}{"type": "email"},
		},
	}
	ownerTypeID, err := testutils.CreateUserType(ownerType)
	ts.Require().NoError(err, "failed to create the owner's user type")
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(ownerTypeID) })
	agentType := ts.prefix + "cred-agent-reset-agent"
	agentTypeID, err := testutils.CreateNewAgentType(testutils.UserType{
		Handle: agentType, DisplayName: "Audit Agent Reset Agent", OUID: ouID,
		Schema: map[string]interface{}{
			"name":     map[string]interface{}{"type": "string", "required": true, "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "failed to create the agent type")
	ts.T().Cleanup(func() { _ = testutils.DeleteAgentType(agentTypeID) })

	ownerEmail := ts.prefix + "cred-agent-reset-owner@example.com"
	attributes, err := json.Marshal(map[string]interface{}{"username": ts.prefix + "cred-agent-reset-owner",
		"email": ownerEmail})
	ts.Require().NoError(err)
	ownerID, err := testutils.CreateUser(testutils.User{OUID: ouID, Type: ownerType.Handle, Attributes: attributes})
	ts.Require().NoError(err, "failed to create the owner")
	ts.T().Cleanup(func() { _ = testutils.DeleteUser(ownerID) })
	agentID, err := testutils.CreateAgent(testutils.Agent{OUID: ouID, Type: agentType,
		Owner: testutils.LocalAgentOwner(ownerID),
		Attributes: map[string]interface{}{"name": ts.prefix + "cred-agent-reset-agent",
			"password": credentialMarker + "Before1!"}})
	ts.Require().NoError(err, "failed to create the agent")
	ts.T().Cleanup(func() { _ = testutils.DeleteAgent(agentID) })

	smtp := testutils.NewMockSMTPServer(0)
	ts.Require().NoError(smtp.Start(), "failed to start the mock SMTP server")
	ts.T().Cleanup(func() { _ = smtp.Stop() })
	senderID, err := testutils.CreateSMTPEmailProvider(ts.prefix+"cred-agent-reset-sender", "localhost",
		smtp.GetPort(), "noreply@thunderid.test")
	ts.Require().NoError(err, "failed to create the SMTP email provider")
	ts.T().Cleanup(func() { _ = testutils.DeleteNotificationSender(senderID) })
	handle := ts.prefix + "cred-agent-reset-flow"
	flowID, err := testutils.CopyFlowWithSenders("default-admin-agent-credential-reset-flow", "ADMIN_INITIATED",
		handle, map[string]string{"send_link": senderID}, nil)
	ts.Require().NoError(err, "failed to copy the shipped administrator agent credential reset flow")
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err)

	started, step := ts.executeAdminReset(adminToken, map[string]interface{}{
		"flowType": "ADMIN_INITIATED", "handle": handle,
		"inputs": map[string]string{"subject": agentID, "credential": "password"},
	})
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the flow must wait once the link is sent")
	var email *testutils.EmailMessage
	ts.Require().Eventually(func() bool {
		email = smtp.GetLastEmail()
		return email != nil
	}, 10*time.Second, 100*time.Millisecond, "the reset email was not sent")
	ts.Require().Equal([]string{ownerEmail}, email.To, "the link goes to the owner")
	link := email.ExtractRecoveryLink()
	inviteToken := testutils.ExtractQueryParam(link, "inviteToken")
	ts.Require().NotEmpty(inviteToken, "the reset email carries no link")
	_, step = ts.executeAdminReset("", map[string]interface{}{
		"executionId": testutils.ExtractQueryParam(link, "executionId"),
		"inputs":      map[string]string{"inviteToken": inviteToken},
	})
	completed, step := ts.executeAdminReset("", map[string]interface{}{
		"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
		"inputs": map[string]string{"password": credentialMarker + "After1!"},
		"action": "action_prompt_new_credential",
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the reset must complete")

	perCall := ts.linesPerCall(started, completed)
	agentRef := testutils.AuditRef{Type: "agent", ID: agentID}
	initiated := ts.onlyLineOf(perCall[0], "agent.credential.invalidate", "the reset's initiation")
	ts.Equal(testutils.AuditRef{Type: "user", ID: ts.adminUserID()}, initiated.Actor,
		"the initiation names the administrator")
	ts.Equal(agentRef, initiated.Target)
	ts.Equal(map[string]string{"result": "succeeded"}, initiated.Outcome)
	ts.Equal(map[string]string{"credential_type": "password"}, ts.details(initiated),
		"the initiation names the credential type, nothing else")
	finished := ts.onlyLineOf(perCall[1], "agent.credential.update", "the reset's completion")
	ts.Equal(testutils.AuditRef{Type: "user", ID: ownerID}, finished.Actor,
		"a completion from the link names the owner, not the administrator")
	ts.Equal(agentRef, finished.Target)
	ts.Equal(map[string]string{"result": "succeeded"}, finished.Outcome)
	for _, written := range perCall {
		for _, line := range written {
			ts.NotContains(line.Raw, inviteToken, "the link token is never recorded")
			ts.NotContains(line.Raw, credentialMarker, "no part of a credential is recorded")
			ts.NotContains(line.Raw, ownerEmail, "the owner's address is never recorded")
		}
	}
}

// executeAdminReset posts one administrator-initiated flow request, with the bearer token when it is not empty,
// and returns the response and the step it got after checking the request was accepted.
func (ts *AuditLogTestSuite) executeAdminReset(token string, request map[string]interface{}) (
	*http.Response, *common.FlowStep) {
	ts.T().Helper()
	payload, err := json.Marshal(request)
	ts.Require().NoError(err)
	header := http.Header{}
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
	response, body := ts.post(testutils.GetRawHTTPClient(), "/flow/execute", payload, header)
	ts.Require().Equal(http.StatusOK, response.StatusCode, string(body))
	var step common.FlowStep
	ts.Require().NoError(json.Unmarshal(body, &step))
	ts.Require().NotEqual("ERROR", step.FlowStatus, string(body))
	return response, &step
}

// executeFlow posts one flow request with the bearer token and the application's Flow Secret, and
// returns the response and the step it got after checking the request was accepted.
func (ts *AuditLogTestSuite) executeFlow(token, appID string, request map[string]interface{}) (
	*http.Response, *common.FlowStep) {
	ts.T().Helper()
	payload, err := json.Marshal(request)
	ts.Require().NoError(err)
	header := http.Header{"Authorization": {"Bearer " + token}}
	if secret := testutils.GetFlowSecret(appID); secret != "" {
		header.Set(testutils.FlowSecretHeaderName, secret)
	}
	response, body := ts.post(testutils.GetRawHTTPClient(), "/flow/execute", payload, header)
	ts.Require().Equal(http.StatusOK, response.StatusCode, string(body))
	var step common.FlowStep
	ts.Require().NoError(json.Unmarshal(body, &step))
	ts.Require().NotEqual("ERROR", step.FlowStatus, string(body))
	return response, &step
}

// emailLinkRecoveryFlow is a recovery flow that identifies the user by username, emails a recovery link,
// verifies the token from it and sets the password the user submits.
func emailLinkRecoveryFlow(handle, senderID string) testutils.Flow {
	prompt := func(ref, identifier, inputType, action, next string) map[string]interface{} {
		return map[string]interface{}{"prompts": []map[string]interface{}{{
			"inputs": []map[string]interface{}{
				{"ref": ref, "identifier": identifier, "type": inputType, "required": true},
			},
			"action": map[string]interface{}{"ref": action, "nextNode": next},
		}}}
	}
	usernamePrompt := prompt("input_username", "username", "TEXT_INPUT", "action_submit_username", "identify_user")
	usernamePrompt["id"], usernamePrompt["type"] = "prompt_username", "PROMPT"
	passwordPrompt := prompt("input_new_password", "password", "PASSWORD_INPUT", "action_submit_password",
		"set_credential")
	passwordPrompt["id"], passwordPrompt["type"] = "prompt_new_password", "PROMPT"
	return testutils.Flow{
		Name: "Audit Email Link Recovery " + handle, Handle: handle, FlowType: "RECOVERY",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_username"},
			usernamePrompt,
			{"id": "identify_user", "type": "TASK_EXECUTION", "executor": map[string]interface{}{
				"name": "IdentifyingExecutor", "mode": "identify", "inputs": []map[string]interface{}{
					{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
				},
			}, "onSuccess": "generate_recovery_token", "onFailure": "email_sent_status"},
			{"id": "generate_recovery_token", "type": "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "InviteExecutor", "mode": "generate"},
				"onSuccess": "send_recovery_email"},
			{"id": "send_recovery_email", "type": "TASK_EXECUTION",
				"properties": map[string]interface{}{"emailTemplate": "password-recovery", "senderId": senderID},
				"executor": map[string]interface{}{"name": "EmailExecutor", "mode": "send",
					"inputs": []map[string]interface{}{
						{"identifier": "email", "type": "EMAIL_INPUT", "required": true},
					}},
				"onSuccess": "email_sent_status", "onFailure": "email_sent_status"},
			{"id": "email_sent_status", "type": "PROMPT", "next": "verify_recovery_token"},
			{"id": "verify_recovery_token", "type": "TASK_EXECUTION", "executor": map[string]interface{}{
				"name": "InviteExecutor", "mode": "verify", "inputs": []map[string]interface{}{
					{"ref": "input_recovery_token", "identifier": "inviteToken", "type": "HIDDEN", "required": true},
				},
			}, "onSuccess": "prompt_new_password"},
			passwordPrompt,
			{"id": "set_credential", "type": "TASK_EXECUTION",
				"executor": map[string]interface{}{"name": "CredentialSetter"}, "onSuccess": "recovery_complete"},
			{"id": "recovery_complete", "type": "PROMPT", "next": "end"},
			{"id": "end", "type": "END"},
		},
	}
}
