// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// auditFlowNodeID names a node of every flow these tests write; no line may carry it.
const auditFlowNodeID = "audit_flow_assert_node"

// Creating, updating, restoring and deleting a flow through the management API writes one line each,
// with the admin as actor and the flow as target and subject. No part of the definition, its handle,
// name or nodes, appears in any line.
func (ts *AuditLogTestSuite) TestAFlowLifecycleWritesOneLinePerChange() {
	adminID := ts.adminUserID()
	handle := ts.prefix + "flow-lifecycle"
	name := ts.prefix + "Lifecycle Flow"

	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/flows",
		ts.flowBody(handle, name, ""))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	var flow struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(body, &flow))
	ts.Require().NotEmpty(flow.ID)
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flow.ID) })
	updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, "/flows/"+flow.ID,
		ts.flowBody(handle, name+" Renamed", ""))
	ts.Require().Equal(http.StatusOK, updated.StatusCode, string(body))
	restored, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/flows/"+flow.ID+"/restore",
		`{"version":1}`)
	ts.Require().Equal(http.StatusOK, restored.StatusCode, string(body))
	deleted := ts.send(http.MethodDelete, "/flows/"+flow.ID, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	flowRef := testutils.AuditRef{Type: "flow", ID: flow.ID}
	lines := ts.linesOf(created, updated, restored, deleted)
	for i, tc := range []struct {
		event  string
		action string
	}{
		{"flow.create", "create"},
		{"flow.update", "update"},
		{"flow.restore", "restore"},
		{"flow.delete", "delete"},
	} {
		line := lines[i]
		ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
		ts.Equal(lineKeys, ts.keys(line.Raw), tc.event)
		ts.Equal(tc.event, line.Event)
		ts.Equal(tc.action, line.Action)
		ts.Equal(flowRef, line.Target, tc.event)
		ts.Equal(flowRef, line.Subject, tc.event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: adminID}, line.Actor, tc.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, tc.event)
		ts.Nil(line.Client, "%s: a loopback peer is not recorded", tc.event)
		ts.requireNoFlowContent(line, handle, name)
	}
}

// A change the flow service refuses writes one failed line naming the flow the caller asked for, or no
// flow when it was not created, and no subject.
func (ts *AuditLogTestSuite) TestAFlowChangeTheServiceRefusesIsRecordedAsFailed() {
	missing := unusedID()
	handle := ts.prefix + "flow-refused"
	name := ts.prefix + "Refused Flow"
	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name: name, FlowType: "AUTHENTICATION", Handle: handle, Nodes: auditFlowNodes(),
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })

	updatedMissing := ts.send(http.MethodPut, "/flows/"+missing, ts.flowBody(handle, name, ""))
	ts.Require().Equal(http.StatusNotFound, updatedMissing.StatusCode)
	duplicate := ts.send(http.MethodPost, "/flows", ts.flowBody(handle, name, ""))
	ts.Require().Equal(http.StatusBadRequest, duplicate.StatusCode)
	restoredMissing := ts.send(http.MethodPost, "/flows/"+flowID+"/restore", `{"version":99}`)
	ts.Require().Equal(http.StatusNotFound, restoredMissing.StatusCode)

	lines := ts.linesOf(updatedMissing, duplicate, restoredMissing)
	for i, tc := range []struct {
		event  string
		action string
		target testutils.AuditRef
		reason string
	}{
		{"flow.update", "update", testutils.AuditRef{Type: "flow", ID: missing}, "flm_1003"},
		{"flow.create", "create", testutils.AuditRef{Type: "flow"}, "flm_1013"},
		{"flow.restore", "restore", testutils.AuditRef{Type: "flow", ID: flowID}, "flm_1008"},
	} {
		line := lines[i]
		ts.Equal(tc.event, line.Event)
		ts.Equal(tc.action, line.Action)
		ts.Equal(tc.target, line.Target, tc.event)
		ts.Equal(testutils.AuditRef{}, line.Subject, "a failed attempt has no subject")
		ts.Equal(map[string]string{"result": "failed", "reason": tc.reason}, line.Outcome, tc.event)
		ts.requireNoFlowContent(line, handle, name)
	}
}

// A delete of a flow that does not exist still succeeds, but writes no line, so a caller cannot record
// the deletion of an ID it made up.
func (ts *AuditLogTestSuite) TestADeleteOfAMissingFlowWritesNoLine() {
	deleted := ts.send(http.MethodDelete, "/flows/"+unusedID(), "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	ts.Empty(ts.linesPerCall(deleted)[0], "a delete that removed nothing writes no line")
}

// An upsert import writes the flow service's event for the flow it creates or updates and nothing else:
// the lookups that decide between update and create, by ID and then by handle, write no line.
func (ts *AuditLogTestSuite) TestAnImportWritesTheEventOfTheFlowItWrites() {
	ouID, err := testutils.DefaultOUID()
	ts.Require().NoError(err)
	handle := ts.prefix + "flow-import"
	name := ts.prefix + "Imported Flow"
	flowID := unusedID()

	createdByImport := ts.importFlow(flowID, handle, name, ouID)
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })
	// Another ID with the same handle finds the flow by its handle, and updates it.
	updatedByImport := ts.importFlow(unusedID(), handle, name+" Again", ouID)

	succeeded := map[string]string{"result": "succeeded"}
	lines := []testutils.AuditLine{
		ts.resourceLineOfImport(createdByImport, succeeded),
		ts.resourceLineOfImport(updatedByImport, succeeded),
	}
	flowRef := testutils.AuditRef{Type: "flow", ID: flowID}
	for i, event := range []string{"flow.create", "flow.update"} {
		ts.Equal(event, lines[i].Event)
		ts.Equal(flowRef, lines[i].Target, event)
		ts.Equal(flowRef, lines[i].Subject, event)
		ts.Equal(succeeded, lines[i].Outcome, event)
		ts.requireNoFlowContent(lines[i], handle, name)
	}
}

// importFlow imports, with upsert, the authentication flow flowID with handle and name, owned by ouID,
// and returns the response after checking the flow was written.
func (ts *AuditLogTestSuite) importFlow(flowID, handle, name, ouID string) *http.Response {
	ts.T().Helper()
	document, err := json.Marshal(map[string]interface{}{
		"resource_type": "flow",
		"id":            flowID,
		"handle":        handle,
		"name":          name,
		"flowType":      "AUTHENTICATION",
		"ouId":          ouID,
		"nodes":         auditFlowNodes(),
	})
	ts.Require().NoError(err)
	request, err := json.Marshal(testutils.ImportRequest{
		Content: string(document),
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	ts.Require().NoError(err)

	imported, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/import", string(request))
	ts.Require().Equal(http.StatusOK, imported.StatusCode, string(body))
	var result testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Equal(1, result.Summary.Imported, string(body))
	return imported
}

// flowBody is the request body of an authentication flow with handle and name, owned by ouID or, when
// ouID is empty, by the default organization unit.
func (ts *AuditLogTestSuite) flowBody(handle, name, ouID string) string {
	ts.T().Helper()
	if ouID == "" {
		var err error
		ouID, err = testutils.DefaultOUID()
		ts.Require().NoError(err)
	}
	body, err := json.Marshal(testutils.Flow{
		Name: name, FlowType: "AUTHENTICATION", Handle: handle, Nodes: auditFlowNodes(), OUID: ouID,
	})
	ts.Require().NoError(err)
	return string(body)
}

// auditFlowNodes are the nodes of the smallest authentication flow.
func auditFlowNodes() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": auditFlowNodeID},
		{
			"id":        auditFlowNodeID,
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
			"onSuccess": "end",
		},
		{"id": "end", "type": "END"},
	}
}

// requireNoFlowContent checks that line carries no part of the flow definition.
func (ts *AuditLogTestSuite) requireNoFlowContent(line testutils.AuditLine, handle, name string) {
	ts.T().Helper()
	ts.NotContains(line.Raw, handle, "the handle of the flow is not recorded")
	ts.NotContains(line.Raw, name, "the name of the flow is not recorded")
	ts.NotContains(line.Raw, auditFlowNodeID, "no node of the flow is recorded")
	ts.NotContains(line.Raw, "AuthAssertExecutor", "no executor of the flow is recorded")
}
