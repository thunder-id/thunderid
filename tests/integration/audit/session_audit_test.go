// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	sessionPassword          = "AuditSession123!"
	sessionCredentialsAction = "action_submit_credentials"
)

// systemRef is the actor of a session the server ends on its own authority.
var systemRef = testutils.AuditRef{Type: "system"}

// sessionFixture is a user who signs in through an application that starts a session and signs out of it,
// and a second application on the same sign-in flow that joins the session.
type sessionFixture struct {
	userID       string
	username     string
	signInAppID  string
	joiningAppID string
}

// Every flow request here carries the admin's bearer token, so a sign-in or a single sign-on line naming the
// user rather than the admin shows its actor comes from the session and not from the request. The session
// handle and the token family ID are credentials, so no line of any of the calls may carry them.
func (ts *AuditLogTestSuite) TestASignInItsSingleSignOnAndItsSignOutWriteOneSessionLineEach() {
	f := ts.createSessionFixture("session-lifecycle")
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	userRef := testutils.AuditRef{Type: "user", ID: f.userID}
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err)

	prompted, submitted, signedIn := ts.signIn(f)
	held := signedIn.SessionData[0].FlowSessionData
	sid, signInFamily := ts.sessionClaims(signedIn.Assertion)

	reused, joined := ts.executeSessionStep(f.joiningAppID, map[string]interface{}{
		"applicationId": f.joiningAppID, "flowType": "AUTHENTICATION",
		"sessionData": []testutils.FlowSessionData{held},
	})
	ts.Require().Equal("COMPLETE", joined.FlowStatus, "the joining application must be signed in by the session")
	joinedSid, joinedFamily := ts.sessionClaims(joined.Assertion)
	ts.Require().Equal(sid, joinedSid, "the joining application must get the same session")

	ended, signedOut := ts.executeSessionStep(f.signInAppID, map[string]interface{}{
		"applicationId": f.signInAppID, "flowType": "SIGNOUT",
		"sessionData": []testutils.FlowSessionData{held},
	})
	ts.Require().Equal("COMPLETE", signedOut.FlowStatus, "the sign-out must complete")
	ts.Require().Equal([]string{held.FlowIdentifier}, signedOut.ClearedSessions, "the sign-out must end the session")

	written := ts.linesPerCall(prompted, submitted, reused, ended)
	ts.Empty(sessionEvents(written[0]), "the credential prompt starts no session")
	ts.Equal([]string{"session.create"}, sessionEvents(written[1]), "the verified credentials start one session")
	ts.Equal([]string{"session.reuse"}, sessionEvents(written[2]), "the single sign-on joins and starts nothing")
	ts.Equal([]string{"session.end"}, sessionEvents(written[3]), "the sign-out ends one session")

	created := ts.onlyLineOf(written[1], "session.create", "a sign-in")
	ts.Equal(append(append([]string{}, lineKeys...), "application", "session_ref", "flow_ref"), ts.keys(created.Raw),
		"a sign-in")
	ts.Equal("create", created.Action)
	ts.Equal(userRef, created.Actor, "the user who signed in is the actor, not the owner of the bearer token")
	ts.NotEqual(adminRef, created.Actor, "the owner of the bearer token did not sign in")
	ts.Equal(userRef, created.Target)
	ts.Equal(userRef, created.Subject)
	ts.Equal(map[string]string{"result": "succeeded"}, created.Outcome)
	ts.Equal(sid, created.SessionRef, "the session is named by the sid its tokens carry")
	ts.Equal(map[string]string{"id": f.signInAppID}, created.Application, "the application signed in to is named")

	reuse := ts.onlyLineOf(written[2], "session.reuse", "a single sign-on")
	ts.Equal(append(append([]string{}, lineKeys...), "application", "session_ref", "flow_ref"), ts.keys(reuse.Raw),
		"a single sign-on")
	ts.Equal("reuse", reuse.Action)
	ts.Equal(userRef, reuse.Actor, "the session's user is the actor, not the owner of the bearer token")
	ts.NotEqual(adminRef, reuse.Actor, "the owner of the bearer token did not sign in")
	ts.Equal(userRef, reuse.Target)
	ts.Equal(userRef, reuse.Subject)
	ts.Equal(map[string]string{"result": "succeeded"}, reuse.Outcome)
	ts.Equal(sid, reuse.SessionRef, "the joined session is the one the sign-in started")
	ts.Equal(map[string]string{"id": f.joiningAppID}, reuse.Application, "the joining application is named")

	end := ts.onlyLineOf(written[3], "session.end", "a sign-out")
	ts.Equal(append(append([]string{}, lineKeys...), "application", "session_ref", "flow_ref", "details"),
		ts.keys(end.Raw),
		"a sign-out")
	ts.Equal("end", end.Action)
	ts.Equal(userRef, end.Actor, "the user signs out of the session, not the owner of the bearer token")
	ts.Equal(userRef, end.Target)
	ts.Equal(userRef, end.Subject)
	ts.Equal(map[string]string{"result": "succeeded"}, end.Outcome)
	ts.Equal(sid, end.SessionRef, "the ended session is the one the sign-in started")
	ts.Equal(map[string]string{"end_reason": "sign_out"}, ts.details(end))
	ts.Equal(map[string]string{"id": f.signInAppID}, end.Application, "the flow's application is named")

	ts.NotEmpty(created.FlowRef, "a sign-in names the flow it ran in")
	ts.NotEqual(created.FlowRef, reuse.FlowRef, "each flow has its own reference")
	ts.NotEqual(created.FlowRef, end.FlowRef, "each flow has its own reference")
	for _, call := range written {
		for _, line := range call {
			ts.requireNoSessionCredential(line, held.SessionHandle,
				signInFamily, joinedFamily, signedIn.Assertion, joined.Assertion, adminToken)
			for _, executionID := range []string{signedIn.ExecutionID, joined.ExecutionID, signedOut.ExecutionID} {
				ts.NotEmpty(executionID)
				ts.NotContains(line.Raw, executionID, "the execution ID continues the flow and is never recorded")
				ts.NotEqual(executionID, line.FlowRef, "the flow reference is not the execution ID")
			}
		}
	}
}

// Deleting a user ends every session the user holds, each on its own line. The sessions end in a system flow
// the deletion runs on the server's authority, so the system is their actor, and the administrator who caused
// them is the actor of the deletion line under the same trace.
func (ts *AuditLogTestSuite) TestDeletingAUserEndsEachOfItsSessions() {
	f := ts.createSessionFixture("session-subject")
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	userRef := testutils.AuditRef{Type: "user", ID: f.userID}

	// A sign-in that presents no session starts a new one, so the user holds two.
	_, _, first := ts.signIn(f)
	_, _, second := ts.signIn(f)
	firstSid, firstFamily := ts.sessionClaims(first.Assertion)
	secondSid, secondFamily := ts.sessionClaims(second.Assertion)
	ts.Require().NotEqual(firstSid, secondSid, "the user must hold two sessions")

	deleted := ts.send(http.MethodDelete, "/users/"+f.userID, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	written := ts.linesPerCall(deleted)[0]
	ts.Equal([]string{"session.end", "session.end"}, sessionEvents(written), "one line per session the user held")
	deletion := ts.onlyLineOf(written, "user.delete", "the deletion of a user with sessions")
	ts.Equal(adminRef, deletion.Actor, "the administrator deleted the user")
	ts.Equal(userRef, deletion.Target)
	var ended []string
	for _, line := range written {
		ts.requireNoSessionCredential(line, first.SessionData[0].SessionHandle, second.SessionData[0].SessionHandle,
			firstFamily, secondFamily)
		if line.Event != "session.end" {
			continue
		}
		ended = append(ended, line.SessionRef)
		ts.Equal("end", line.Action)
		ts.Equal(systemRef, line.Actor, "a session ended by the deletion's system flow is ended by the system")
		ts.Equal(deletion.TraceID, line.TraceID, "the end is recorded under the deletion that caused it")
		ts.Equal(userRef, line.Target)
		ts.Equal(userRef, line.Subject)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome)
		ts.Equal(map[string]string{"end_reason": "subject_revocation"}, ts.details(line))
		ts.Nil(line.Application)
	}
	ts.ElementsMatch([]string{firstSid, secondSid}, ended, "each line names one of the user's sessions")
}

// Deleting an application ends a session only when the application was the last one in it. Deleting the
// joining application first ends nothing, since the session still serves the application that started it,
// which shows the line follows the session's end and not the deletion. As with a user deletion, the system
// ends the session and the administrator is the actor of the deletion line under the same trace.
func (ts *AuditLogTestSuite) TestDeletingTheLastApplicationOfASessionEndsIt() {
	f := ts.createSessionFixture("session-detach")
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	userRef := testutils.AuditRef{Type: "user", ID: f.userID}

	_, _, signedIn := ts.signIn(f)
	held := signedIn.SessionData[0].FlowSessionData
	sid, signInFamily := ts.sessionClaims(signedIn.Assertion)
	_, joined := ts.executeSessionStep(f.joiningAppID, map[string]interface{}{
		"applicationId": f.joiningAppID, "flowType": "AUTHENTICATION",
		"sessionData": []testutils.FlowSessionData{held},
	})
	ts.Require().Equal("COMPLETE", joined.FlowStatus, "the joining application must be signed in by the session")
	_, joinedFamily := ts.sessionClaims(joined.Assertion)

	detached := ts.send(http.MethodDelete, "/applications/"+f.joiningAppID, "")
	ts.Require().Equal(http.StatusNoContent, detached.StatusCode)
	removed := ts.send(http.MethodDelete, "/applications/"+f.signInAppID, "")
	ts.Require().Equal(http.StatusNoContent, removed.StatusCode)

	written := ts.linesPerCall(detached, removed)
	ts.Empty(sessionEvents(written[0]), "a session another application is still in does not end")
	ts.Equal([]string{"session.end"}, sessionEvents(written[1]), "the session ends with its last application")

	end := ts.onlyLineOf(written[1], "session.end", "the deletion of the last application")
	ts.Equal("end", end.Action)
	ts.Equal(systemRef, end.Actor, "a session ended by the deletion's system flow is ended by the system")
	deletion := ts.onlyLineOf(written[1], "application.delete", "the deletion of the last application")
	ts.Equal(adminRef, deletion.Actor, "the administrator deleted the application")
	ts.Equal(deletion.TraceID, end.TraceID, "the end is recorded under the deletion that caused it")
	ts.Equal(userRef, end.Target)
	ts.Equal(userRef, end.Subject)
	ts.Equal(map[string]string{"result": "succeeded"}, end.Outcome)
	ts.Equal(sid, end.SessionRef)
	ts.Equal(map[string]string{"end_reason": "application_detached"}, ts.details(end))
	ts.Equal(map[string]string{"id": f.signInAppID}, end.Application, "the detached application is named")
	for _, call := range written {
		for _, line := range call {
			ts.requireNoSessionCredential(line, held.SessionHandle, signInFamily, joinedFamily)
		}
	}
}

// createSessionFixture creates the user, the flows and the two applications of a session scenario, removed
// when the test ends. name keeps the handles of two scenarios in one run apart.
func (ts *AuditLogTestSuite) createSessionFixture(name string) sessionFixture {
	ts.T().Helper()
	ouID, userType := ts.createUserType(name)
	username := ts.prefix + name
	userID := ts.createUser(ouID, userType, username, sessionPassword)

	signInFlowID, err := testutils.CreateFlow(sessionSignInFlow(ts.prefix + name + "-signin"))
	ts.Require().NoError(err, "failed to create the sign-in flow")
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(signInFlowID) })
	signOutFlowID, err := testutils.CreateFlow(sessionSignOutFlow(ts.prefix + name + "-signout"))
	ts.Require().NoError(err, "failed to create the sign-out flow")
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(signOutFlowID) })

	createApp := func(suffix, signOut string) string {
		appID, err := testutils.CreateApplication(testutils.Application{
			Name: ts.prefix + name + "-" + suffix, OUID: ouID, AuthFlowID: signInFlowID, SignOutFlowID: signOut,
			AllowedUserTypes: []string{userType},
		})
		ts.Require().NoError(err, "failed to create the %s application", suffix)
		ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })
		ts.Require().NotEmpty(testutils.GetFlowSecret(appID), "the %s application must get a Flow Secret", suffix)
		return appID
	}
	return sessionFixture{
		userID: userID, username: username,
		signInAppID: createApp("signin-app", signOutFlowID), joiningAppID: createApp("joining-app", ""),
	}
}

// signIn signs the user in through the sign-in application with no session to reuse, and returns the
// response that prompted for the credentials, the response to the credentials and the completed step.
func (ts *AuditLogTestSuite) signIn(f sessionFixture) (*http.Response, *http.Response, *testutils.FlowStep) {
	ts.T().Helper()
	prompted, step := ts.executeSessionStep(f.signInAppID, map[string]interface{}{
		"applicationId": f.signInAppID, "flowType": "AUTHENTICATION",
	})
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "a sign-in with no session must prompt for credentials")
	submitted, step := ts.executeSessionStep(f.signInAppID, map[string]interface{}{
		"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
		"inputs": map[string]string{"username": f.username, "password": sessionPassword},
		"action": sessionCredentialsAction,
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the credentials must sign the user in")
	ts.Require().Len(step.SessionData, 1, "the sign-in must start a session")
	ts.Require().NotEmpty(step.SessionData[0].SessionHandle)
	return prompted, submitted, step
}

// executeSessionStep posts one flow request for appID with the admin's bearer token and the application's
// Flow Secret, and returns the response and the step it got after checking the request was accepted.
func (ts *AuditLogTestSuite) executeSessionStep(appID string, request map[string]interface{}) (
	*http.Response, *testutils.FlowStep) {
	ts.T().Helper()
	token, err := testutils.GetAccessToken()
	ts.Require().NoError(err)
	payload, err := json.Marshal(request)
	ts.Require().NoError(err)
	header := http.Header{
		"Authorization":                {"Bearer " + token},
		testutils.FlowSecretHeaderName: {testutils.GetFlowSecret(appID)},
	}
	response, body := ts.post(testutils.GetRawHTTPClient(), "/flow/execute", payload, header)
	ts.Require().Equal(http.StatusOK, response.StatusCode, string(body))
	var step testutils.FlowStep
	ts.Require().NoError(json.Unmarshal(body, &step))
	ts.Require().NotEqual("ERROR", step.FlowStatus, string(body))
	return response, &step
}

// sessionClaims returns the session ID and the token family ID the assertion carries.
func (ts *AuditLogTestSuite) sessionClaims(assertion string) (string, string) {
	ts.T().Helper()
	claims, err := testutils.DecodeJWTPayloadMap(assertion)
	ts.Require().NoError(err)
	sid, _ := claims["sid"].(string)
	family, _ := claims["tfid"].(string)
	ts.Require().NotEmpty(sid, "the assertion must name its session")
	ts.Require().NotEmpty(family, "the assertion must carry a token family ID")
	return sid, family
}

// requireNoSessionCredential checks that line carries none of secrets.
func (ts *AuditLogTestSuite) requireNoSessionCredential(line testutils.AuditLine, secrets ...string) {
	ts.T().Helper()
	for _, secret := range secrets {
		ts.Require().NotEmpty(secret)
		ts.NotContains(line.Raw, secret, "%s: a session credential is never recorded", line.Event)
	}
}

// sessionEvents returns the session events among lines, in the order written.
func sessionEvents(lines []testutils.AuditLine) []string {
	var events []string
	for _, line := range lines {
		if strings.HasPrefix(line.Event, "session.") {
			events = append(events, line.Event)
		}
	}
	return events
}

// sessionSignInFlow is a sign-in flow that reuses a live session and otherwise asks for a username and a
// password and starts one.
func sessionSignInFlow(handle string) testutils.Flow {
	credentials := []map[string]interface{}{
		{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	return testutils.Flow{
		Name: "Audit Session Sign In " + handle, Handle: handle, FlowType: "AUTHENTICATION",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "sso_check"},
			{"id": "sso_check", "type": "TASK_EXECUTION",
				"executor":   map[string]interface{}{"name": "SSOCheckExecutor"},
				"properties": map[string]interface{}{"checkpointRef": "session_main"},
				"onSuccess":  "session_main", "onFailure": "prompt_credentials"},
			{"id": "prompt_credentials", "type": "PROMPT", "prompts": []map[string]interface{}{{
				"inputs": credentials,
				"action": map[string]interface{}{"ref": sessionCredentialsAction, "nextNode": "credentials_auth"},
			}}},
			{"id": "credentials_auth", "type": "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": credentials},
				"onSuccess": "session_main", "onIncomplete": "prompt_credentials"},
			{"id": "session_main", "type": "TASK_EXECUTION",
				"executor": map[string]interface{}{"name": "SessionExecutor"}, "onSuccess": "auth_assert"},
			{"id": "auth_assert", "type": "TASK_EXECUTION",
				"executor": map[string]interface{}{"name": "AuthAssertExecutor"}, "onSuccess": "end"},
			{"id": "end", "type": "END"},
		},
	}
}

// sessionSignOutFlow is a sign-out flow that ends the session the request presents.
func sessionSignOutFlow(handle string) testutils.Flow {
	return testutils.Flow{
		Name: "Audit Session Sign Out " + handle, Handle: handle, FlowType: "SIGNOUT",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "session_signout"},
			{"id": "session_signout", "type": "TASK_EXECUTION",
				"executor": map[string]interface{}{"name": "SessionSignOutExecutor"}, "onSuccess": "end"},
			{"id": "end", "type": "END"},
		},
	}
}
