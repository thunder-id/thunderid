// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The Control Plane writes an audit line for each application it creates, updates and deletes, with
// the administrator the trusted issuer's token names as the actor and no client secret. It writes its
// output to the server log only when a test started it, so the test restarts it first.
func (ts *ControlPlaneTestSuite) TestApplicationChangesAreAudited() {
	ts.Require().NoError(testutils.RestartServer(), "the Control Plane must restart so its output is captured")
	reader := testutils.NewAuditLogReader()
	ts.Require().NoError(reader.Reset())

	name := unique("cp-audit-app")
	clientSecret := unique("CpAuditSecret")
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID: ts.ouID, Name: name, ClientID: unique("cp-audit-client"), ClientSecret: clientSecret,
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })
	status, raw := ts.call(http.MethodPut, "/applications/"+appID, map[string]any{
		"ouId": ts.ouID, "name": name, "description": "Updated on the Control Plane",
	}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Require().NoError(testutils.DeleteApplication(appID))

	appRef := testutils.AuditRef{Type: "application", ID: appID}
	for _, event := range []string{"application.create", "application.update", "application.delete"} {
		line, err := reader.Await(func(line testutils.AuditLine) bool {
			return line.Event == event && line.Target == appRef
		})
		ts.Require().NoError(err, event)
		ts.Equal(appRef, line.Subject, event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: testutils.TrustedIssuerAdminSubject}, line.Actor, event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, event)
		ts.NotContains(line.Raw, clientSecret, "a client secret is never recorded")
		ts.NotContains(line.Raw, name, "no attribute of the application is recorded")
	}
}

// The Control Plane writes an audit line for each role and group it creates, updates and deletes, and for
// each assignee a role change grants or takes the role from, with the administrator the trusted issuer's
// token names as the actor and no name of either.
func (ts *ControlPlaneTestSuite) TestRoleAndGroupChangesAreAudited() {
	ts.Require().NoError(testutils.RestartServer(), "the Control Plane must restart so its output is captured")
	reader := testutils.NewAuditLogReader()
	ts.Require().NoError(reader.Reset())

	groupName := unique("cp-audit-group")
	groupID, err := testutils.CreateGroup(testutils.Group{Name: groupName, OUID: ts.ouID})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteGroup(groupID) })
	status, raw := ts.call(http.MethodPut, "/groups/"+groupID, map[string]any{
		"ouId": ts.ouID, "name": groupName, "description": "Updated on the Control Plane",
	}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	roleName := unique("cp-audit-role")
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: roleName, OUID: ts.ouID, Assignments: []testutils.Assignment{{ID: groupID, Type: "group"}},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteRole(roleID) })
	status, raw = ts.call(http.MethodPut, "/roles/"+roleID, map[string]any{
		"ouId": ts.ouID, "name": roleName, "description": "Updated on the Control Plane",
	}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	// The role is deleted with its assignments taken off it first.
	ts.Require().NoError(testutils.DeleteRole(roleID))
	ts.Require().NoError(testutils.DeleteGroup(groupID))

	groupRef := testutils.AuditRef{Type: "group", ID: groupID}
	roleRef := testutils.AuditRef{Type: "role", ID: roleID}
	for _, want := range []struct {
		event           string
		target, subject testutils.AuditRef
	}{
		{"group.create", groupRef, groupRef},
		{"group.update", groupRef, groupRef},
		{"role.create", roleRef, roleRef},
		{"role.assignment.add", roleRef, groupRef},
		{"role.update", roleRef, roleRef},
		{"role.assignment.remove", roleRef, groupRef},
		{"role.delete", roleRef, roleRef},
		{"group.delete", groupRef, groupRef},
	} {
		line, err := reader.Await(func(line testutils.AuditLine) bool {
			return line.Event == want.event && line.Target == want.target
		})
		ts.Require().NoError(err, want.event)
		ts.Equal(want.subject, line.Subject, want.event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: testutils.TrustedIssuerAdminSubject}, line.Actor, want.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, want.event)
		ts.NotContains(line.Raw, groupName, "no attribute of the group is recorded")
		ts.NotContains(line.Raw, roleName, "no attribute of the role is recorded")
	}
}

// The Control Plane writes an audit line for each gateway it registers, applies to, changes and removes,
// with the administrator the trusted issuer's token names as the actor and the gateway as the target. A
// failed apply or removal keeps the gateway ID the caller sent and has no subject. Neither the gateway's
// name, address or key nor the version applied is recorded.
func (ts *ControlPlaneTestSuite) TestGatewayChangesAreAudited() {
	ts.Require().NoError(testutils.RestartServer(), "the Control Plane must restart so its output is captured")
	reader := testutils.NewAuditLogReader()
	ts.Require().NoError(reader.Reset())
	g := newApplyGateway()
	ts.T().Cleanup(g.close)
	gatewayID := ts.registerGateway(g.server)
	var captured struct {
		Version string `json:"version"`
	}
	status, raw := ts.call(http.MethodPost, "/configuration-versions", map[string]any{}, &captured)
	ts.Require().Contains([]int{http.StatusCreated, http.StatusOK}, status, string(raw))
	missing := unique("cp-audit-missing-gateway")

	status, raw = ts.call(http.MethodPost, "/gateways/"+gatewayID+"/apply",
		map[string]any{"version": captured.Version}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	status, raw = ts.call(http.MethodPost, "/gateways/"+missing+"/apply", map[string]any{}, nil)
	ts.Require().Equal(http.StatusNotFound, status, string(raw))
	renamed := unique("cp-audit-gateway")
	status, raw = ts.call(http.MethodPut, "/gateways/"+gatewayID, map[string]any{"name": renamed}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	status, raw = ts.call(http.MethodDelete, "/gateways/"+gatewayID, nil, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	status, raw = ts.call(http.MethodDelete, "/gateways/"+gatewayID, nil, nil)
	ts.Require().Equal(http.StatusNotFound, status, string(raw))

	admin := testutils.AuditRef{Type: "user", ID: testutils.TrustedIssuerAdminSubject}
	succeeded := map[string]string{"result": "succeeded"}
	notFound := map[string]string{"result": "failed", "reason": "gtw_1002"}
	for _, tc := range []struct {
		event   string
		target  string
		subject bool
		outcome map[string]string
	}{
		{"gateway.create", gatewayID, true, succeeded},
		{"gateway.apply", gatewayID, true, succeeded},
		{"gateway.apply", missing, false, notFound},
		{"gateway.update", gatewayID, true, succeeded},
		{"gateway.delete", gatewayID, true, succeeded},
		{"gateway.delete", gatewayID, false, notFound},
	} {
		name := tc.event + " " + tc.outcome["result"]
		gatewayRef := testutils.AuditRef{Type: "gateway", ID: tc.target}
		line, err := reader.Await(func(line testutils.AuditLine) bool {
			return line.Event == tc.event && line.Target == gatewayRef && line.Actor == admin &&
				line.Outcome["result"] == tc.outcome["result"]
		})
		ts.Require().NoError(err, name)
		if tc.subject {
			ts.Equal(gatewayRef, line.Subject, name)
		} else {
			ts.Equal(testutils.AuditRef{}, line.Subject, "%s: a failed attempt may name no gateway", name)
		}
		ts.Equal(tc.outcome, line.Outcome, name)
		ts.Equal(strings.TrimPrefix(tc.event, "gateway."), line.Action, name)
		ts.NotContains(line.Raw, captureGatewayKey, "the gateway's key is never recorded")
		ts.NotContains(line.Raw, g.server.URL, "the gateway's address is not recorded")
		ts.NotContains(line.Raw, captured.Version, "the version applied is not recorded")
		// registerGateway names the gateway apply-gateway-<nanos>.
		ts.NotContains(line.Raw, "apply-gateway", "the gateway's registered name is not recorded")
		ts.NotContains(line.Raw, renamed, "the gateway's new name is not recorded")
	}
}
