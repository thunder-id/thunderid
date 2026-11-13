// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The Control Plane writes an audit line for each user type and agent it creates, updates and deletes,
// and for each credential update of a user, with the administrator the trusted issuer's token names as
// the actor. No line carries the type's display name or attribute definitions, the agent's name or client
// secret, or a user's credential. It writes its output to the server log only when a test started it, so
// the test restarts it first.
func (ts *ControlPlaneTestSuite) TestAgentAndUserTypeChangesAreAudited() {
	ts.Require().NoError(testutils.RestartServer(), "the Control Plane must restart so its output is captured")
	reader := testutils.NewAuditLogReader()
	ts.Require().NoError(reader.Reset())

	displayName := unique("CP Audit Type")
	attribute := "cp_audit_badge"
	userType := testutils.UserType{
		Handle: unique("cp-audit-type"), DisplayName: displayName, OUID: ts.ouID,
		Schema: map[string]interface{}{attribute: map[string]interface{}{"type": "string"}},
	}
	typeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(typeID) })
	userType.DisplayName = displayName + " Renamed"
	status, raw := ts.call(http.MethodPut, "/user-types/"+typeID, userType, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().NoError(testutils.DeleteUserType(typeID))

	name := unique("cp-audit-agent")
	clientSecret := unique("CpAuditAgentSecret")
	agentID, err := testutils.CreateAgent(testutils.Agent{
		OUID: ts.ouID, Type: "default", Attributes: map[string]interface{}{"name": name},
		InboundAuthConfig: []testutils.AgentInboundAuthConfig{{
			Type: "oauth2",
			Config: &testutils.AgentOAuthConfig{
				ClientSecret:            clientSecret,
				GrantTypes:              []string{"client_credentials"},
				TokenEndpointAuthMethod: "client_secret_basic",
			},
		}},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteAgent(agentID) })
	ts.Require().NoError(testutils.DeleteAgent(agentID))

	typeRef := testutils.AuditRef{Type: "user_type", ID: typeID}
	agentRef := testutils.AuditRef{Type: "agent", ID: agentID}
	for _, tc := range []struct {
		event  string
		target testutils.AuditRef
	}{
		{"user_type.create", typeRef},
		{"user_type.update", typeRef},
		{"user_type.delete", typeRef},
		{"agent.create", agentRef},
		{"agent.delete", agentRef},
	} {
		line, err := reader.Await(func(line testutils.AuditLine) bool {
			return line.Event == tc.event && line.Target == tc.target
		})
		ts.Require().NoError(err, tc.event)
		ts.Equal(tc.target, line.Subject, tc.event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: testutils.TrustedIssuerAdminSubject}, line.Actor, tc.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, tc.event)
		ts.NotContains(line.Raw, displayName, "no attribute of the user type is recorded")
		ts.NotContains(line.Raw, attribute, "no attribute definition is recorded")
		ts.NotContains(line.Raw, name, "no attribute of the agent is recorded")
		ts.NotContains(line.Raw, clientSecret, "a client secret is never recorded")
	}

	ts.assertUserCredentialUpdatesAreAudited(reader)
}

// assertUserCredentialUpdatesAreAudited sets the credentials of a user, and of a user that does not exist,
// and checks the line each writes. A credential, its type and the user's attributes are never recorded.
func (ts *ControlPlaneTestSuite) assertUserCredentialUpdatesAreAudited(reader *testutils.AuditLogReader) {
	userType := testutils.UserType{
		Handle: unique("cp-cred-type"), DisplayName: unique("CP Credential Type"), OUID: ts.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	}
	typeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(typeID) })
	username := unique("cp-cred-user")
	attributes, err := json.Marshal(map[string]interface{}{"username": username})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{OUID: ts.ouID, Type: userType.Handle, Attributes: attributes})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteUser(userID) })
	password := unique("CpCredMarker")
	missing := fmt.Sprintf("01900000-0000-7000-8000-%012x", time.Now().UnixNano()&0xffffffffffff)

	credentials := map[string]any{"credentials": map[string]any{"password": password}}
	status, raw := ts.call(http.MethodPost, "/users/"+userID+"/update-credentials", credentials, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	status, raw = ts.call(http.MethodPost, "/users/"+missing+"/update-credentials", credentials, nil)
	ts.Require().Equal(http.StatusNotFound, status, string(raw))

	admin := testutils.AuditRef{Type: "user", ID: testutils.TrustedIssuerAdminSubject}
	for _, tc := range []struct {
		target  string
		subject bool
		outcome map[string]string
	}{
		{userID, true, map[string]string{"result": "succeeded"}},
		{missing, false, map[string]string{"result": "failed", "reason": "usr_1003"}},
	} {
		userRef := testutils.AuditRef{Type: "user", ID: tc.target}
		line, err := reader.Await(func(line testutils.AuditLine) bool {
			return line.Event == "user.credential.update" && line.Target == userRef
		})
		ts.Require().NoError(err, tc.target)
		if tc.subject {
			ts.Equal(userRef, line.Subject, tc.target)
		} else {
			ts.Equal(testutils.AuditRef{}, line.Subject, tc.target)
		}
		ts.Equal(admin, line.Actor, tc.target)
		ts.Equal("update", line.Action, tc.target)
		ts.Equal(tc.outcome, line.Outcome, tc.target)
		ts.NotContains(line.Raw, password, "a credential is never recorded")
		ts.NotContains(line.Raw, "CpCredMarker", "no part of a credential is recorded")
		ts.NotContains(line.Raw, "password", "the credential type is not recorded")
		ts.NotContains(line.Raw, username, "no attribute of the user is recorded")
	}
}
