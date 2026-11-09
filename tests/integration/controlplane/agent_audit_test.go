// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The Control Plane writes an audit line for each user type and agent it creates, updates and deletes,
// with the administrator the trusted issuer's token names as the actor. No line carries the type's
// display name or attribute definitions, or the agent's name or client secret. It writes its output to
// the server log only when a test started it, so the test restarts it first.
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
		ProjectID: defaultProjectID, Type: "default", Attributes: map[string]interface{}{"name": name},
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
}
