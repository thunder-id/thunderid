// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// credentialMarker starts every credential value these tests send, so a check that no part of a
	// credential is recorded does not depend on the full value.
	credentialMarker = "AuditCredMarker"

	// passkeyRelyingPartyID is the relying party whose origin, the server's own URL, the test deployment's
	// passkey.allowed_origins accepts.
	passkeyRelyingPartyID = "localhost"

	userSuspendFlowHandle = "default-user-suspend-flow"
)

func (ts *AuditLogTestSuite) TestEveryUserCredentialUpdateRecordsItsOutcome() {
	ouID, userType := ts.createUserType("cred")
	userID := ts.createUser(ouID, userType, ts.prefix+"cred", credentialMarker+"Initial1!")
	siblingOU, siblingType := ts.createUserType("cred-sibling")
	siblingID := ts.createUser(siblingOU, siblingType, ts.prefix+"cred-sibling", credentialMarker+"Sibling1!")
	managerID, managerClient := ts.createScopedUserManager("cred-scoped")
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	managerRef := testutils.AuditRef{Type: "user", ID: managerID}
	userRef := testutils.AuditRef{Type: "user", ID: userID}
	siblingRef := testutils.AuditRef{Type: "user", ID: siblingID}
	missing := unusedID()

	cases := []struct {
		name    string
		client  *http.Client
		path    string
		body    string
		status  int
		actor   testutils.AuditRef
		target  testutils.AuditRef
		outcome map[string]string
		subject bool
	}{
		{
			name: "an administrator sets the password of a user", client: testutils.GetHTTPClient(),
			path:   "/users/" + userID + "/update-credentials",
			body:   fmt.Sprintf(`{"credentials":{"password":%q}}`, credentialMarker+"Admin1!"),
			status: http.StatusNoContent, actor: adminRef, target: userRef,
			outcome: map[string]string{"result": "succeeded"}, subject: true,
		},
		{
			name: "a credential value that is not a string", client: testutils.GetHTTPClient(),
			path: "/users/" + userID + "/update-credentials", body: `{"credentials":{"password":4242}}`,
			status: http.StatusBadRequest, actor: adminRef, target: userRef,
			outcome: map[string]string{"result": "failed", "reason": "usr_1001"},
		},
		{
			name: "a credential type the server manages itself", client: testutils.GetHTTPClient(),
			path:   "/users/" + userID + "/update-credentials",
			body:   fmt.Sprintf(`{"credentials":{"passkey":%q}}`, credentialMarker+"Passkey1!"),
			status: http.StatusBadRequest, actor: adminRef, target: userRef,
			outcome: map[string]string{"result": "failed", "reason": "usr_1024"},
		},
		{
			name: "a user that does not exist", client: testutils.GetHTTPClient(),
			path:   "/users/" + missing + "/update-credentials",
			body:   fmt.Sprintf(`{"credentials":{"password":%q}}`, credentialMarker+"Missing1!"),
			status: http.StatusNotFound, actor: adminRef, target: testutils.AuditRef{Type: "user", ID: missing},
			outcome: map[string]string{"result": "failed", "reason": "usr_1003"},
		},
		{
			name: "a user manager sets the password of a user outside its unit", client: managerClient,
			path:   "/users/" + siblingID + "/update-credentials",
			body:   fmt.Sprintf(`{"credentials":{"password":%q}}`, credentialMarker+"Intruder1!"),
			status: http.StatusForbidden, actor: managerRef, target: siblingRef,
			outcome: map[string]string{"result": "denied", "reason": "forbidden"}, subject: true,
		},
		// Last, since replacing a password the caller holds revokes the token the caller sends.
		{
			name: "a user sets their own password", client: managerClient,
			path:   "/users/me/update-credentials",
			body:   fmt.Sprintf(`{"attributes":{"password":%q}}`, credentialMarker+"Self1!"),
			status: http.StatusNoContent, actor: managerRef, target: managerRef,
			outcome: map[string]string{"result": "succeeded"}, subject: true,
		},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		response, body := ts.exchange(tc.client, http.MethodPost, tc.path, tc.body)
		ts.Require().Equal(tc.status, response.StatusCode, "%s: %s", tc.name, string(body))
		responses[i] = response
	}

	for i, written := range ts.linesPerCall(responses...) {
		tc := cases[i]
		line := ts.onlyLineOf(written, "user.credential.update", tc.name)
		ts.Equal("update", line.Action, tc.name)
		ts.Equal(tc.actor, line.Actor, tc.name)
		ts.Equal(tc.target, line.Target, tc.name)
		ts.Equal(tc.outcome, line.Outcome, tc.name)
		if tc.subject {
			ts.Equal(tc.target, line.Subject, tc.name)
		} else {
			ts.Equal(testutils.AuditRef{}, line.Subject, "%s: a failed attempt has no subject", tc.name)
		}
		ts.Nil(line.Client, "%s: a loopback peer is not recorded", tc.name)
		ts.Nil(ts.details(line), "%s: a credential event carries no details", tc.name)
		for _, other := range written {
			ts.requireNoCredentialValue(other, tc.name)
		}
	}
}

func (ts *AuditLogTestSuite) TestAnImportThatSetsAUserCredentialRecordsEachStep() {
	ouID, userType := ts.createUserType("cred-import")
	userID := unusedID()
	ts.T().Cleanup(func() { _ = testutils.DeleteUser(userID) })
	refusedID := unusedID()
	ts.T().Cleanup(func() { _ = testutils.DeleteUser(refusedID) })
	document := "resource_type: user\nid: %s\ntype: %s\nouId: %s\nattributes:\n  username: %s\n" +
		"credentials:\n  %s: %s\n"

	imported, _ := ts.importUpsert(fmt.Sprintf(document, userID, userType, ouID, ts.prefix+"cred-imported",
		"password", credentialMarker+"Import1!"), 1)
	// A credential the type declares is set by the create itself, so it writes no credential event. The user
	// type declares no pin, so that credential is written after the create and refused.
	refused, _ := ts.importUpsert(fmt.Sprintf(document, refusedID, userType, ouID, ts.prefix+"cred-refused",
		"pin", credentialMarker+"Pin1!"), 0)

	succeeded := map[string]string{"result": "succeeded"}
	userRef := testutils.AuditRef{Type: "user", ID: userID}
	refusedRef := testutils.AuditRef{Type: "user", ID: refusedID}
	failedImport := map[string]string{"result": "failed", "reason": "item_failed"}
	perCall := ts.linesPerCall(imported, refused)
	for i, want := range []struct {
		name  string
		lines []importStep
	}{
		{"an import that sets the password", []importStep{
			{"user.create", userRef, userRef, succeeded},
			{importEvent, configurationRef, testutils.AuditRef{}, succeeded},
		}},
		{"an import whose credential is refused", []importStep{
			{"user.create", refusedRef, refusedRef, succeeded},
			{"user.credential.update", refusedRef, testutils.AuditRef{},
				map[string]string{"result": "failed", "reason": "usr_1024"}},
			{"user.delete", refusedRef, refusedRef, succeeded},
			{importEvent, configurationRef, testutils.AuditRef{}, failedImport},
		}},
	} {
		ts.requireLines(perCall[i], want.lines, want.name)
	}
	stored, _ := ts.exchange(testutils.GetHTTPClient(), http.MethodGet, "/users/"+refusedID, "")
	ts.Equal(http.StatusNotFound, stored.StatusCode, "the import must roll back the user it could not finish")
}

// The import is the only request path that reaches an agent's credential update.
func (ts *AuditLogTestSuite) TestAnImportThatSetsAnAgentCredentialRecordsEachStep() {
	ouID := ts.createOrganizationUnit("cred-agent")
	agentType := ts.createCredentialAgentTypeIn(ouID, "cred-agent")
	agentID := unusedID()
	ts.T().Cleanup(func() { _ = testutils.DeleteAgent(agentID) })
	refusedID := unusedID()
	ts.T().Cleanup(func() { _ = testutils.DeleteAgent(refusedID) })
	document := "resource_type: agent\nid: %s\nouId: %s\ntype: %s\nattributes:\n  name: %s\n" +
		"credentials:\n  %s: %s\n"

	imported, _ := ts.importUpsert(fmt.Sprintf(document, agentID, ouID, agentType, ts.prefix+"cred-agent",
		"password", credentialMarker+"Agent1!"), 1)
	// A credential the type declares is set by the create itself, so it writes no credential event. The agent
	// type declares no pin, so that credential is written after the create and refused.
	refused, _ := ts.importUpsert(fmt.Sprintf(document, refusedID, ouID, agentType, ts.prefix+"cred-agent-pin",
		"pin", credentialMarker+"AgentPin1!"), 0)

	succeeded := map[string]string{"result": "succeeded"}
	agentRef := testutils.AuditRef{Type: "agent", ID: agentID}
	refusedRef := testutils.AuditRef{Type: "agent", ID: refusedID}
	perCall := ts.linesPerCall(imported, refused)
	for i, want := range []struct {
		name  string
		lines []importStep
	}{
		{"an import that sets the password", []importStep{
			{"agent.create", agentRef, agentRef, succeeded},
			{importEvent, configurationRef, testutils.AuditRef{}, succeeded},
		}},
		{"an import whose credential is refused", []importStep{
			{"agent.create", refusedRef, refusedRef, succeeded},
			{"agent.credential.update", refusedRef, testutils.AuditRef{},
				map[string]string{"result": "failed", "reason": "agt_1016"}},
			{"agent.delete", refusedRef, refusedRef, succeeded},
			{importEvent, configurationRef, testutils.AuditRef{}, map[string]string{
				"result": "failed", "reason": "item_failed"}},
		}},
	} {
		ts.requireLines(perCall[i], want.lines, want.name)
	}
}

// A refused registration names no user and is anonymous: the provider refuses before it resolves the
// session to one. A stored credential names its user as the actor, as the endpoint is public.
func (ts *AuditLogTestSuite) TestEveryPasskeyRegistrationRecordsItsOutcome() {
	ouID, userType := ts.createUserType("passkey")
	username := ts.prefix + "passkey"
	password := credentialMarker + "Passkey1!"
	userID := ts.createUser(ouID, userType, username, password)
	suspendedName := ts.prefix + "passkey-held"
	suspendedID := ts.createUser(ouID, userType, suspendedName, password)

	enrolled := ts.startPasskeyRegistration(userID, username, password)
	held := ts.startPasskeyRegistration(suspendedID, suspendedName, password)
	suspended, body := ts.runAdministrationFlow(userSuspendFlowHandle, map[string]string{"subject": suspendedID})
	ts.Require().Equal(http.StatusOK, suspended.StatusCode, string(body))
	ts.Require().Contains(string(body), `"COMPLETE"`, "the suspension must complete")
	unknownSession := ts.startPasskeyRegistration(userID, username, password)
	unknownSession.sessionToken = credentialMarker + "UnknownSession"

	malformed := passkeyFinishRequest{SessionToken: enrolled.sessionToken}
	malformed.PublicKeyCredential.ID = credentialMarker + "CredentialID"
	malformed.PublicKeyCredential.RawID = malformed.PublicKeyCredential.ID
	malformed.PublicKeyCredential.Type = "public-key"
	malformed.PublicKeyCredential.Response.ClientDataJSON = base64.RawURLEncoding.EncodeToString(
		[]byte(`{"type":"webauthn.create"}`))
	malformed.PublicKeyCredential.Response.AttestationObject = base64.RawURLEncoding.EncodeToString(
		[]byte(credentialMarker))

	noUser := testutils.AuditRef{Type: "user"}
	userRef := testutils.AuditRef{Type: "user", ID: userID}
	refused := map[string]string{"result": "failed", "reason": "authn_mgr_1002"}
	cases := []struct {
		name    string
		request passkeyFinishRequest
		status  int
		target  testutils.AuditRef
		outcome map[string]string
		subject bool
	}{
		// Sent before the registration completes, since a completed registration's session is cleared.
		{name: "an attestation that cannot be parsed", request: malformed, status: http.StatusBadRequest,
			target: noUser, outcome: refused},
		{name: "a session the server never issued", request: ts.attest(unknownSession),
			status: http.StatusBadRequest, target: noUser, outcome: refused},
		{name: "a registration for a suspended user", request: ts.attest(held), status: http.StatusUnauthorized,
			target: noUser, outcome: map[string]string{"result": "denied", "reason": "account_suspended"}},
		{name: "a registration that completes", request: ts.attest(enrolled), status: http.StatusOK,
			target: userRef, outcome: map[string]string{"result": "succeeded"}, subject: true},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		response, body := ts.finishPasskeyRegistration(tc.request)
		ts.Require().Equal(tc.status, response.StatusCode, "%s: %s", tc.name, string(body))
		responses[i] = response
	}

	sensitive := []string{username, suspendedName, enrolled.sessionToken, held.sessionToken,
		enrolled.authenticator.CredentialID(), held.authenticator.CredentialID()}
	for i, line := range ts.linesOf(responses...) {
		tc := cases[i]
		ts.Equal("user.credential.enroll", line.Event, tc.name)
		ts.Equal("enroll", line.Action, tc.name)
		ts.Equal(tc.target, line.Target, tc.name)
		ts.Equal(tc.outcome, line.Outcome, tc.name)
		if tc.subject {
			ts.Equal(tc.target, line.Actor, "%s: a stored credential names its user as the actor", tc.name)
			ts.Equal(tc.target, line.Subject, tc.name)
		} else {
			ts.Equal(testutils.AuditRef{Type: "anonymous"}, line.Actor, "%s: a refused attempt is anonymous", tc.name)
			ts.Equal(testutils.AuditRef{}, line.Subject, "%s: an attempt that names no user has no subject", tc.name)
		}
		ts.Nil(line.Client, "%s: a loopback peer is not recorded", tc.name)
		ts.requireNoCredentialValue(line, tc.name)
		ts.Nil(ts.details(line), "%s: an enrolment event carries no details", tc.name)
		ts.NotContains(line.Raw, suspendedID, "%s: a refused registration names no user", tc.name)
		for _, value := range sensitive {
			ts.NotContains(line.Raw, value, "%s: no passkey data or identifier is recorded", tc.name)
		}
	}
}

// A regeneration of a missing application is refused, and the refusal is recorded as a failed update.
func (ts *AuditLogTestSuite) TestASecretRegenerationIsRecorded() {
	ouID := ts.createOrganizationUnit("cred-secret")
	clientID := ts.prefix + "cred-secret-client"
	oldSecret := credentialMarker + "ClientSecret0123456789"
	appID, err := testutils.CreateApplication(testutils.Application{
		Name: ts.prefix + "cred-secret-app", OUID: ouID, Type: "fullstack",
		ClientID: clientID, ClientSecret: oldSecret,
	})
	ts.Require().NoError(err, "failed to create the application whose secret is regenerated")
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })

	rotated, body := ts.post(testutils.GetHTTPClient(), "/applications/"+appID+"/client-secret/regenerate", nil, nil)
	ts.Require().Equal(http.StatusOK, rotated.StatusCode, string(body))
	var regenerated struct {
		ClientSecret string `json:"clientSecret"`
	}
	ts.Require().NoError(json.Unmarshal(body, &regenerated))
	newSecret := regenerated.ClientSecret
	ts.Require().NotEmpty(newSecret, "the response must carry the new secret: %s", string(body))
	missing := unusedID()
	refused, body := ts.post(testutils.GetHTTPClient(), "/applications/"+missing+"/client-secret/regenerate", nil,
		nil)
	ts.Require().Equal(http.StatusNotFound, refused.StatusCode, "a missing application must not be regenerated: %s",
		string(body))

	perCall := ts.linesPerCall(rotated, refused)
	line := ts.onlyLineOf(perCall[0], "application.update", "a regenerated secret")
	appRef := testutils.AuditRef{Type: "application", ID: appID}
	ts.Equal("update", line.Action)
	ts.Equal(testutils.AuditRef{Type: "user", ID: ts.adminUserID()}, line.Actor)
	ts.Equal(appRef, line.Target)
	ts.Equal(appRef, line.Subject)
	ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome)
	ts.Nil(line.Client, "a loopback peer is not recorded")
	refusal := ts.onlyLineOf(perCall[1], "application.update", "a refused regeneration")
	ts.Equal(testutils.AuditRef{Type: "application", ID: missing}, refusal.Target)
	ts.Equal(map[string]string{"result": "failed", "reason": "app_1001"}, refusal.Outcome)
	for _, written := range perCall {
		for _, other := range written {
			ts.NotContains(other.Raw, newSecret, "the new client secret is never recorded")
			ts.NotContains(other.Raw, credentialMarker, "the old client secret is never recorded")
			ts.NotContains(other.Raw, clientID, "the client ID is not recorded")
		}
	}
}

// importStep is a line an import must write, in order.
type importStep struct {
	event   string
	target  testutils.AuditRef
	subject testutils.AuditRef
	outcome map[string]string
}

// requireLines checks that a call wrote exactly the expected lines in order, each naming the admin as
// actor and carrying no credential.
func (ts *AuditLogTestSuite) requireLines(written []testutils.AuditLine, want []importStep, name string) {
	ts.T().Helper()
	events := make([]string, 0, len(written))
	for _, line := range written {
		events = append(events, line.Event)
	}
	ts.Require().Len(written, len(want), "%s: the call wrote %v", name, events)
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	for i, line := range written {
		ts.Equal(want[i].event, line.Event, name)
		ts.Equal(want[i].target, line.Target, "%s: %s", name, line.Event)
		ts.Equal(want[i].subject, line.Subject, "%s: %s", name, line.Event)
		ts.Equal(want[i].outcome, line.Outcome, "%s: %s", name, line.Event)
		ts.Equal(adminRef, line.Actor, "%s: %s", name, line.Event)
		ts.requireNoCredentialValue(line, name)
	}
}

// onlyLineOf returns the one line of event a call wrote. A credential replacement may write lines of
// other events under the same trace, such as the revocation that follows it.
func (ts *AuditLogTestSuite) onlyLineOf(written []testutils.AuditLine, event, name string) testutils.AuditLine {
	ts.T().Helper()
	var found []testutils.AuditLine
	for _, line := range written {
		if line.Event == event {
			found = append(found, line)
		}
	}
	ts.Require().Len(found, 1, "%s: the call did not write exactly one %s line", name, event)
	return found[0]
}

// requireNoCredentialValue checks that a line carries no credential value and no credential type.
func (ts *AuditLogTestSuite) requireNoCredentialValue(line testutils.AuditLine, name string) {
	ts.T().Helper()
	ts.NotContains(line.Raw, credentialMarker, "%s: no part of a credential is recorded", name)
	ts.NotContains(line.Raw, "password", "%s: the credential type is not recorded", name)
	ts.NotContains(ts.details(line), "pin", "%s: the credential type is not recorded", name)
}

// createCredentialAgentTypeIn creates an agent type in ouID that declares a password credential, removed
// when the test ends, and returns its handle.
func (ts *AuditLogTestSuite) createCredentialAgentTypeIn(ouID, name string) string {
	ts.T().Helper()
	handle := ts.prefix + name + "-agents"
	agentTypeID, err := testutils.CreateNewAgentType(testutils.UserType{
		Handle: handle, DisplayName: "Audit Credential Agents " + name, OUID: ouID,
		Schema: map[string]interface{}{
			"name":     map[string]interface{}{"type": "string", "required": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteAgentType(agentTypeID) })
	return handle
}

// runAdministrationFlow executes the shipped administration flow handle as the admin and returns the
// response and its body, whatever they are. The admin token is set by hand, since the shared clients send none to
// /flow/execute.
func (ts *AuditLogTestSuite) runAdministrationFlow(handle string, inputs map[string]string) (
	*http.Response, []byte) {
	ts.T().Helper()
	flowID, err := testutils.GetFlowIDByHandle(handle, "ADMINISTRATION")
	ts.Require().NoError(err, "the %s flow must ship", handle)
	token, err := testutils.GetAccessToken()
	ts.Require().NoError(err)
	payload, err := json.Marshal(map[string]interface{}{"flowId": flowID, "inputs": inputs})
	ts.Require().NoError(err)
	return ts.post(testutils.GetRawHTTPClient(), "/flow/execute", payload,
		http.Header{"Authorization": {"Bearer " + token}})
}

// passkeyRegistration is a passkey registration started and not yet finished.
type passkeyRegistration struct {
	authenticator *testutils.VirtualAuthenticator
	sessionToken  string
	challenge     string
}

// passkeyFinishRequest is the body of a passkey registration finish request.
type passkeyFinishRequest struct {
	PublicKeyCredential struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		RawID    string `json:"rawId"`
		Response struct {
			ClientDataJSON    string `json:"clientDataJSON"`
			AttestationObject string `json:"attestationObject"`
		} `json:"response"`
	} `json:"publicKeyCredential"`
	SessionToken string `json:"sessionToken"`
}

// startPasskeyRegistration signs the user in with the password, starts a passkey registration for the
// user with the assertion it got, and returns the registration with a fresh authenticator.
func (ts *AuditLogTestSuite) startPasskeyRegistration(userID, username, password string) passkeyRegistration {
	ts.T().Helper()
	assertion, err := testutils.ObtainAuthAssertion(username, password)
	ts.Require().NoError(err, "the user must sign in to start a passkey registration")
	authenticator, err := testutils.NewVirtualAuthenticator(passkeyRelyingPartyID, testutils.TestServerURL)
	ts.Require().NoError(err)
	status, body, err := testutils.StartPasskeyRegistrationRaw(userID, passkeyRelyingPartyID, "ThunderID Audit",
		assertion)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, string(body))
	var started struct {
		SessionToken string `json:"sessionToken"`
		Options      struct {
			Challenge string `json:"challenge"`
		} `json:"publicKeyCredentialCreationOptions"`
	}
	ts.Require().NoError(json.Unmarshal(body, &started))
	ts.Require().NotEmpty(started.SessionToken)
	return passkeyRegistration{
		authenticator: authenticator, sessionToken: started.SessionToken, challenge: started.Options.Challenge,
	}
}

// attest answers the registration's challenge with a signed attestation from its authenticator.
func (ts *AuditLogTestSuite) attest(registration passkeyRegistration) passkeyFinishRequest {
	ts.T().Helper()
	credentialID, clientDataJSON, attestationObject, err := registration.authenticator.CreateAttestationResponse(
		registration.challenge, true)
	ts.Require().NoError(err)
	request := passkeyFinishRequest{SessionToken: registration.sessionToken}
	request.PublicKeyCredential.ID = credentialID
	request.PublicKeyCredential.RawID = credentialID
	request.PublicKeyCredential.Type = "public-key"
	request.PublicKeyCredential.Response.ClientDataJSON = clientDataJSON
	request.PublicKeyCredential.Response.AttestationObject = attestationObject
	return request
}

// finishPasskeyRegistration sends the finish request as a browser would: with the Direct API secret and
// no bearer token.
func (ts *AuditLogTestSuite) finishPasskeyRegistration(request passkeyFinishRequest) (*http.Response, []byte) {
	ts.T().Helper()
	payload, err := json.Marshal(request)
	ts.Require().NoError(err)
	return ts.post(testutils.GetRawHTTPClient(), "/register/passkey/finish", payload,
		http.Header{testutils.DirectAuthHeaderName: {testutils.DirectAuthHeaderValue}})
}

// post sends a JSON body with the given headers and returns the response and its body.
func (ts *AuditLogTestSuite) post(client *http.Client, path string, payload []byte, header http.Header) (
	*http.Response, []byte) {
	ts.T().Helper()
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+path, bytes.NewReader(payload))
	ts.Require().NoError(err)
	for key := range header {
		req.Header.Set(key, header.Get(key))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", testUserAgent)
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	read, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().NoError(resp.Body.Close())
	return resp, read
}
