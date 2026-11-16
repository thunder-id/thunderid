// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The Control Plane writes an audit line for each connection, resource server and action it creates,
// updates and deletes, and for each sharing policy of a connection it creates, updates and deletes, with the
// administrator the trusted issuer's token names as the actor and no client secret or name. A policy's line
// has the connection as subject. It writes its output to the server log only when a test started it, so the
// test restarts it first.
func (ts *ControlPlaneTestSuite) TestConnectionAndResourceServerChangesAreAudited() {
	ts.Require().NoError(testutils.RestartServer(), "the Control Plane must restart so its output is captured")
	reader := testutils.NewAuditLogReader()
	ts.Require().NoError(reader.Reset())

	type created struct {
		ID string `json:"id"`
	}
	connectionName := unique("cp-audit-idp")
	clientSecret := unique("CpAuditIdpSecret")
	connectionBody := map[string]any{"name": connectionName, "ouId": ts.ouID, "clientId": "cp-audit-client",
		"clientSecret": clientSecret, "redirectUri": "https://localhost:3000/google/callback"}
	var connection created
	status, raw := ts.call(http.MethodPost, "/connections/google", connectionBody, &connection)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.T().Cleanup(func() { ts.call(http.MethodDelete, "/connections/google/"+connection.ID, nil, nil) })
	connectionBody["name"] = connectionName + "-renamed"
	status, raw = ts.call(http.MethodPut, "/connections/google/"+connection.ID, connectionBody, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))

	childID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: unique("cp-audit-share-child"), Name: "Control Plane sharing child", Parent: &ts.ouID,
		ProjectID: defaultProjectID,
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(childID) })
	policiesPath := "/connections/google/" + connection.ID + "/sharing-policies"
	var policy created
	status, raw = ts.call(http.MethodPost, policiesPath, map[string]any{"childUnits": map[string]any{"mode": "all"}},
		&policy)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	status, raw = ts.call(http.MethodPut, policiesPath+"/"+policy.ID,
		map[string]any{"version": 1, "childUnits": map[string]any{"mode": "all"}}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	status, raw = ts.call(http.MethodDelete, policiesPath+"/"+policy.ID, nil, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	status, raw = ts.call(http.MethodDelete, "/connections/google/"+connection.ID, nil, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	rsName := unique("cp-audit-rs")
	var server created
	status, raw = ts.call(http.MethodPost, "/resource-servers", map[string]any{
		"name": rsName, "identifier": "https://" + rsName + ".example.com", "ouId": ts.ouID,
	}, &server)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.T().Cleanup(func() { _ = testutils.DeleteResourceServerWithChildren(server.ID) })
	var action created
	status, raw = ts.call(http.MethodPost, "/resource-servers/"+server.ID+"/actions",
		map[string]any{"name": "CP Audit Read", "handle": "cpauditread"}, &action)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	status, raw = ts.call(http.MethodPut, "/resource-servers/"+server.ID,
		map[string]any{"name": rsName + "-renamed", "ouId": ts.ouID}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	status, raw = ts.call(http.MethodDelete, "/resource-servers/"+server.ID+"/actions/"+action.ID, nil, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))
	status, raw = ts.call(http.MethodDelete, "/resource-servers/"+server.ID, nil, nil)
	ts.Require().Equal(http.StatusNoContent, status, string(raw))

	connectionRef := testutils.AuditRef{Type: "connection", ID: connection.ID}
	serverRef := testutils.AuditRef{Type: "resource_server", ID: server.ID}
	actionRef := testutils.AuditRef{Type: "action", ID: action.ID}
	for _, want := range []struct {
		event  string
		target testutils.AuditRef
	}{
		{"connection.create", connectionRef},
		{"connection.update", connectionRef},
		{"connection.delete", connectionRef},
		{"resource_server.create", serverRef},
		{"resource_server.action.create", actionRef},
		{"resource_server.update", serverRef},
		{"resource_server.action.delete", actionRef},
		{"resource_server.delete", serverRef},
	} {
		line, err := reader.Await(func(line testutils.AuditLine) bool {
			return line.Event == want.event && line.Target == want.target
		})
		ts.Require().NoError(err, want.event)
		ts.Equal(want.target, line.Subject, want.event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: testutils.TrustedIssuerAdminSubject}, line.Actor, want.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, want.event)
		ts.NotContains(line.Raw, clientSecret, "a client secret is never recorded")
		ts.NotContains(line.Raw, connectionName, "the name of a connection is not recorded")
		ts.NotContains(line.Raw, rsName, "the name of a resource server is not recorded")
	}

	policyRef := testutils.AuditRef{Type: "connection_sharing_policy", ID: policy.ID}
	for _, action := range []string{"create", "update", "delete"} {
		event := "connection.sharing_policy." + action
		line, err := reader.Await(func(line testutils.AuditLine) bool {
			return line.Event == event && line.Target == policyRef
		})
		ts.Require().NoError(err, event)
		ts.Equal(connectionRef, line.Subject, event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: testutils.TrustedIssuerAdminSubject}, line.Actor, event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, event)
		ts.NotContains(line.Raw, connectionName, "%s: the connection's name is not recorded", event)
		ts.NotContains(line.Raw, childID, "%s: no unit the policy reaches is recorded", event)
	}
}
