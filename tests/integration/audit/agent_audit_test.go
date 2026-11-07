// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	auditAgentSecret        = "AuditAgentSecret0123456789"
	auditAgentRotatedSecret = "AuditAgentRotated0123456789"
)

// Creating, updating and deleting an agent through the management API writes one line each, with the
// admin as actor and the agent as target and subject. No part of the agent, its client secret, client
// ID or name, appears in any line, including the update that replaces the client secret.
func (ts *AuditLogTestSuite) TestAnAgentLifecycleWritesOneLinePerChange() {
	ouID := ts.createOrganizationUnit("agent-lifecycle")
	adminID := ts.adminUserID()
	name := ts.prefix + "lifecycle-agent"
	clientID := ts.prefix + "lifecycle-agent-client"

	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/agents",
		ts.agentBody(name, ouID, clientID, auditAgentSecret))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	agentID := ts.idOf(body)
	ts.T().Cleanup(func() { _ = testutils.DeleteAgent(agentID) })
	updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, "/agents/"+agentID,
		ts.agentBody(name+"-renamed", ouID, clientID, auditAgentRotatedSecret))
	ts.Require().Equal(http.StatusOK, updated.StatusCode, string(body))
	deleted := ts.send(http.MethodDelete, "/agents/"+agentID, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	agentRef := testutils.AuditRef{Type: "agent", ID: agentID}
	lines := ts.linesOf(created, updated, deleted)
	for i, tc := range []struct {
		event  string
		action string
	}{
		{"agent.create", "create"},
		{"agent.update", "update"},
		{"agent.delete", "delete"},
	} {
		line := lines[i]
		ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
		ts.Equal(lineKeys, ts.keys(line.Raw), tc.event)
		ts.Equal(tc.event, line.Event)
		ts.Equal(tc.action, line.Action)
		ts.Equal(agentRef, line.Target, tc.event)
		ts.Equal(agentRef, line.Subject, tc.event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: adminID}, line.Actor, tc.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, tc.event)
		ts.Contains([]string{"127.0.0.1", "::1"}, line.Client["address"])
		ts.Equal(testUserAgent, line.Client["user_agent"], tc.event)
		ts.requireNoAgentValue(line, name, clientID)
	}
}

// An update or a delete of an agent that does not exist reaches the agent service, which records the
// failed attempt with the ID asked for and no subject. The agent delete is not idempotent, so a delete
// of a made-up ID is recorded as failed rather than as a deletion.
func (ts *AuditLogTestSuite) TestAChangeOfAMissingAgentIsRecordedAsFailed() {
	missing := unusedID()
	ouID := ts.createOrganizationUnit("agent-missing")

	updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, "/agents/"+missing,
		ts.agentBody(ts.prefix+"missing-agent", ouID, ts.prefix+"missing-agent-client", auditAgentSecret))
	ts.Require().Equal(http.StatusNotFound, updated.StatusCode, string(body))
	deleted := ts.send(http.MethodDelete, "/agents/"+missing, "")
	ts.Require().Equal(http.StatusNotFound, deleted.StatusCode)

	missingRef := testutils.AuditRef{Type: "agent", ID: missing}
	lines := ts.linesOf(updated, deleted)
	for i, event := range []string{"agent.update", "agent.delete"} {
		line := lines[i]
		ts.Equal(event, line.Event)
		ts.Equal(missingRef, line.Target, event)
		ts.Equal(testutils.AuditRef{}, line.Subject, "%s: a failed attempt may name no agent", event)
		ts.Equal(map[string]string{"result": "failed", "reason": "agt_1004"}, line.Outcome, event)
		ts.NotContains(line.Raw, auditAgentSecret, "a client secret is never recorded")
	}
}

// An agent manager scoped to one organization unit passes the security layer with system:agent, and
// the agent service refuses its changes to an agent in a sibling unit, which are recorded as denied. A
// refused update or delete names an agent that exists, so it is the subject; a refused create made no
// agent and has none. A create the agent service rejects is recorded as failed.
func (ts *AuditLogTestSuite) TestEveryAgentChangeRecordsItsOutcome() {
	siblingOU := ts.createOrganizationUnit("agent-sibling")
	siblingName := ts.prefix + "sibling-agent"
	siblingAgentID, err := testutils.CreateAgent(testutils.Agent{
		OUID: siblingOU, Type: "default", Attributes: map[string]interface{}{"name": siblingName},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteAgent(siblingAgentID) })
	manager := ts.createScopedManager("agent-scoped", []string{"agent"}, []string{"system:agent"})
	managerClient := manager.client
	managerRef := testutils.AuditRef{Type: "user", ID: manager.id}
	siblingRef := testutils.AuditRef{Type: "agent", ID: siblingAgentID}
	noAgent := testutils.AuditRef{Type: "agent"}
	intruderBody := ts.agentBody(ts.prefix+"intruder-agent", siblingOU, ts.prefix+"intruder-client",
		auditAgentSecret)

	cases := []struct {
		name    string
		client  *http.Client
		method  string
		path    string
		body    string
		status  int
		event   string
		outcome map[string]string
		target  testutils.AuditRef
		subject bool
	}{
		{
			name: "a create in a unit the caller does not manage", client: managerClient,
			method: http.MethodPost, path: "/agents", body: intruderBody,
			status: http.StatusForbidden, event: "agent.create",
			outcome: map[string]string{"result": "denied", "reason": "forbidden"}, target: noAgent,
		},
		{
			name: "an update of an agent in a unit the caller does not manage", client: managerClient,
			method: http.MethodPut, path: "/agents/" + siblingAgentID, body: intruderBody,
			status: http.StatusForbidden, event: "agent.update",
			outcome: map[string]string{"result": "denied", "reason": "forbidden"}, target: siblingRef,
			subject: true,
		},
		{
			name: "a delete of an agent in a unit the caller does not manage", client: managerClient,
			method: http.MethodDelete, path: "/agents/" + siblingAgentID,
			status: http.StatusForbidden, event: "agent.delete",
			outcome: map[string]string{"result": "denied", "reason": "forbidden"}, target: siblingRef,
			subject: true,
		},
		{
			name: "a create in an organization unit that does not exist", client: testutils.GetHTTPClient(),
			method: http.MethodPost, path: "/agents",
			body: ts.agentBody(ts.prefix+"orphan-agent", unusedID(), ts.prefix+"orphan-client",
				auditAgentSecret),
			status: http.StatusBadRequest, event: "agent.create",
			outcome: map[string]string{"result": "failed", "reason": "agt_1005"}, target: noAgent,
		},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		response, body := ts.exchange(tc.client, tc.method, tc.path, tc.body)
		ts.Require().Equal(tc.status, response.StatusCode, "%s: %s", tc.name, string(body))
		responses[i] = response
	}

	lines := ts.linesOf(responses...)
	for i, tc := range cases {
		line := lines[i]
		ts.Equal(tc.event, line.Event, tc.name)
		ts.Equal(tc.outcome, line.Outcome, tc.name)
		ts.Equal(tc.target, line.Target, tc.name)
		if tc.subject {
			ts.Equal(tc.target, line.Subject, tc.name)
		} else {
			ts.Equal(testutils.AuditRef{}, line.Subject, tc.name)
		}
		if tc.client == managerClient {
			ts.Equal(managerRef, line.Actor, tc.name)
		}
		ts.requireNoAgentValue(line, siblingName, ts.prefix+"intruder-client")
	}
}

// An import that creates an agent writes the agent service's event for it and nothing else: the lookup
// that decides between update and create writes no line, and the import itself is not audited.
func (ts *AuditLogTestSuite) TestAnImportWritesTheEventOfTheAgentItCreates() {
	ouID := ts.createOrganizationUnit("agent-import")
	agentID := unusedID()
	name := ts.prefix + "imported-agent"
	request, err := json.Marshal(testutils.ImportRequest{
		Content: fmt.Sprintf("resource_type: agent\nid: %s\nouId: %s\ntype: default\nattributes:\n  name: %s\n",
			agentID, ouID, name),
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	ts.Require().NoError(err)

	imported, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/import", string(request))
	ts.Require().Equal(http.StatusOK, imported.StatusCode, string(body))
	ts.T().Cleanup(func() { _ = testutils.DeleteAgent(agentID) })
	var result testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Equal(1, result.Summary.Imported, string(body))

	line := ts.linesOf(imported)[0]
	agentRef := testutils.AuditRef{Type: "agent", ID: agentID}
	ts.Equal("agent.create", line.Event)
	ts.Equal(agentRef, line.Target)
	ts.Equal(agentRef, line.Subject)
	ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome)
	ts.NotContains(line.Raw, name, "no attribute of the agent is recorded")
}

// requireNoAgentValue checks that the line carries no client secret, no client ID and no name of an agent.
func (ts *AuditLogTestSuite) requireNoAgentValue(line testutils.AuditLine, name, clientID string) {
	ts.T().Helper()
	ts.NotContains(line.Raw, auditAgentSecret, "a client secret is never recorded")
	ts.NotContains(line.Raw, auditAgentRotatedSecret, "a replaced client secret is never recorded")
	ts.NotContains(line.Raw, "AuditAgent", "no part of a client secret is recorded")
	ts.NotContains(line.Raw, clientID, "the client ID is not recorded")
	ts.NotContains(line.Raw, name, "no attribute of the agent is recorded")
}

// agentBody is the request body of an agent of the default type with a confidential OAuth client.
func (ts *AuditLogTestSuite) agentBody(name, ouID, clientID, clientSecret string) string {
	ts.T().Helper()
	body, err := json.Marshal(map[string]interface{}{
		"type":       "default",
		"ouId":       ouID,
		"attributes": map[string]interface{}{"name": name},
		"inboundAuthConfig": []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                clientID,
				"clientSecret":            clientSecret,
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	ts.Require().NoError(err)
	return string(body)
}
