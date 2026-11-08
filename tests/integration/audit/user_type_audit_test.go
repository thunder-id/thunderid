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

// auditTypeAttribute is an attribute a type's schema defines, which no line may carry.
const auditTypeAttribute = "audit_badge_number"

// Creating, updating and deleting a user type through the management API writes one line each, with
// the admin as actor and the user type as target and subject. No part of the type, its handle, display
// name or the attributes its schema defines, appears in any line.
func (ts *AuditLogTestSuite) TestAUserTypeLifecycleWritesOneLinePerChange() {
	ouID := ts.createOrganizationUnit("type-lifecycle")
	adminID := ts.adminUserID()
	handle := ts.prefix + "lifecycle-type"
	displayName := "Audit Lifecycle Contractor"

	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/user-types",
		ts.entityTypeBody(handle, displayName, ouID))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	typeID := ts.idOf(body)
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(typeID) })
	updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, "/user-types/"+typeID,
		ts.entityTypeBody(handle, displayName+" Renamed", ouID))
	ts.Require().Equal(http.StatusOK, updated.StatusCode, string(body))
	deleted := ts.send(http.MethodDelete, "/user-types/"+typeID, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	typeRef := testutils.AuditRef{Type: "user_type", ID: typeID}
	lines := ts.linesOf(created, updated, deleted)
	for i, tc := range []struct {
		event  string
		action string
	}{
		{"user_type.create", "create"},
		{"user_type.update", "update"},
		{"user_type.delete", "delete"},
	} {
		line := lines[i]
		ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
		ts.Equal(lineKeys, ts.keys(line.Raw), tc.event)
		ts.Equal(tc.event, line.Event)
		ts.Equal(tc.action, line.Action)
		ts.Equal(typeRef, line.Target, tc.event)
		ts.Equal(typeRef, line.Subject, tc.event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: adminID}, line.Actor, tc.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, tc.event)
		ts.Nil(line.Client, "%s: a loopback peer is not recorded as the client", tc.event)
		ts.requireNoTypeValue(line, handle, displayName)
	}
}

// An update of a user type that does not exist reaches the entity type service, which records the
// failed attempt with the ID asked for and no subject.
func (ts *AuditLogTestSuite) TestAnUpdateOfAMissingUserTypeIsRecordedAsFailed() {
	missing := unusedID()
	ouID := ts.createOrganizationUnit("type-missing")
	handle := ts.prefix + "missing-type"

	updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, "/user-types/"+missing,
		ts.entityTypeBody(handle, "Audit Missing Type", ouID))
	ts.Require().Equal(http.StatusNotFound, updated.StatusCode, string(body))

	line := ts.linesOf(updated)[0]
	ts.Equal("user_type.update", line.Event)
	ts.Equal(testutils.AuditRef{Type: "user_type", ID: missing}, line.Target)
	ts.Equal(testutils.AuditRef{}, line.Subject, "a failed attempt may name no user type, so it has no subject")
	ts.Equal(map[string]string{"result": "failed", "reason": "usrs_1002"}, line.Outcome)
	ts.requireNoTypeValue(line, handle, "Audit Missing Type")
}

// A delete of a user type that does not exist still succeeds, but writes no line, so a caller cannot
// record the deletion of an ID it made up.
func (ts *AuditLogTestSuite) TestADeleteOfAMissingUserTypeWritesNoLine() {
	deleted := ts.send(http.MethodDelete, "/user-types/"+unusedID(), "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	ts.Empty(ts.linesPerCall(deleted)[0], "a delete that removed nothing writes no line")
}

// A user type manager scoped to one organization unit passes the security layer with org:usertype,
// and the entity type service refuses its changes to a user type in a sibling unit, which are recorded
// as denied with the type as subject; a refused create made no type and has none. A create the service
// rejects is recorded as failed.
func (ts *AuditLogTestSuite) TestEveryUserTypeChangeRecordsItsOutcome() {
	siblingOU := ts.createOrganizationUnit("type-sibling")
	siblingHandle := ts.prefix + "sibling-type"
	siblingID, err := testutils.CreateUserType(testutils.UserType{
		Handle: siblingHandle, DisplayName: "Audit Sibling Type", OUID: siblingOU,
		Schema: map[string]interface{}{auditTypeAttribute: map[string]interface{}{"type": "string"}},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(siblingID) })
	manager := ts.createScopedManager("type-scoped", []string{"usertype"}, []string{"org:usertype"})
	managerClient := manager.client
	managerRef := testutils.AuditRef{Type: "user", ID: manager.id}
	siblingRef := testutils.AuditRef{Type: "user_type", ID: siblingID}
	noType := testutils.AuditRef{Type: "user_type"}
	intruderHandle := ts.prefix + "intruder-type"

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
			method: http.MethodPost, path: "/user-types",
			body:   ts.entityTypeBody(intruderHandle, "Audit Intruder Type", siblingOU),
			status: http.StatusForbidden, event: "user_type.create",
			outcome: map[string]string{"result": "denied", "reason": "forbidden"}, target: noType,
		},
		{
			name: "an update of a type in a unit the caller does not manage", client: managerClient,
			method: http.MethodPut, path: "/user-types/" + siblingID,
			body:   ts.entityTypeBody(siblingHandle, "Audit Sibling Type", siblingOU),
			status: http.StatusForbidden, event: "user_type.update",
			outcome: map[string]string{"result": "denied", "reason": "forbidden"}, target: siblingRef,
			subject: true,
		},
		{
			name: "a delete of a type in a unit the caller does not manage", client: managerClient,
			method: http.MethodDelete, path: "/user-types/" + siblingID,
			status: http.StatusForbidden, event: "user_type.delete",
			outcome: map[string]string{"result": "denied", "reason": "forbidden"}, target: siblingRef,
			subject: true,
		},
		{
			name: "a create with a handle already taken", client: testutils.GetHTTPClient(),
			method: http.MethodPost, path: "/user-types",
			body:   ts.entityTypeBody(siblingHandle, "Audit Sibling Type", siblingOU),
			status: http.StatusConflict, event: "user_type.create",
			outcome: map[string]string{"result": "failed", "reason": "usrs_1003"}, target: noType,
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
		ts.requireNoTypeValue(line, siblingHandle, "Audit Sibling Type")
		ts.NotContains(line.Raw, intruderHandle, tc.name)
	}
}

// The agent types are served by the same service under their own events. A change to the default agent
// type is recorded as an agent type update; a second agent type with its handle, or a second agent type in
// its organization unit, is refused and recorded as failed; and deleting an agent type no agent uses is
// recorded as a delete.
func (ts *AuditLogTestSuite) TestEveryAgentTypeChangeRecordsItsOutcome() {
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { ts.NoError(testutils.RestoreAgentType(snapshot)) })
	// An organization unit holds one agent type, so the one to delete gets an organization unit of its own.
	disposableOUID := ts.createOrganizationUnit("agent-type-delete")
	disposableID, err := testutils.CreateNewAgentType(testutils.UserType{
		Handle: ts.prefix + "disposable-agents", DisplayName: "Audit Disposable Agents", OUID: disposableOUID,
		Schema: map[string]interface{}{auditTypeAttribute: map[string]interface{}{"type": "string"}},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteAgentType(disposableID) })
	displayName := "Audit Default Agents"
	body, err := json.Marshal(map[string]interface{}{
		"handle": "default", "displayName": displayName, "ouId": snapshot.OUID,
		"allowSelfRegistration": snapshot.AllowSelfRegistration, "systemAttributes": snapshot.SystemAttributes,
		"schema": snapshot.Schema,
	})
	ts.Require().NoError(err)
	defaultRef := testutils.AuditRef{Type: "agent_type", ID: snapshot.ID}
	noType := testutils.AuditRef{Type: "agent_type"}

	cases := []struct {
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
			method: http.MethodPut, path: "/agent-types/" + snapshot.ID, body: string(body),
			status: http.StatusOK, event: "agent_type.update", outcome: map[string]string{"result": "succeeded"},
			target: defaultRef, subject: true,
		},
		{
			method: http.MethodPost, path: "/agent-types", body: string(body),
			status: http.StatusConflict, event: "agent_type.create",
			outcome: map[string]string{"result": "failed", "reason": "usrs_1003"}, target: noType,
		},
		{
			method: http.MethodPost, path: "/agent-types",
			body:   ts.entityTypeBody(ts.prefix+"tools", displayName, snapshot.OUID),
			status: http.StatusConflict, event: "agent_type.create",
			outcome: map[string]string{"result": "failed", "reason": "usrs_1021"}, target: noType,
		},
		{
			method: http.MethodDelete, path: "/agent-types/" + disposableID,
			status: http.StatusNoContent, event: "agent_type.delete", outcome: map[string]string{"result": "succeeded"},
			target: testutils.AuditRef{Type: "agent_type", ID: disposableID}, subject: true,
		},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		response, body := ts.exchange(testutils.GetHTTPClient(), tc.method, tc.path, tc.body)
		ts.Require().Equal(tc.status, response.StatusCode, "%s %s: %s", tc.method, tc.path, string(body))
		responses[i] = response
	}

	lines := ts.linesOf(responses...)
	for i, tc := range cases {
		line := lines[i]
		ts.Equal(tc.event, line.Event)
		ts.Equal(tc.outcome, line.Outcome, tc.event)
		ts.Equal(tc.target, line.Target, tc.event)
		if tc.subject {
			ts.Equal(tc.target, line.Subject, tc.event)
		} else {
			ts.Equal(testutils.AuditRef{}, line.Subject, tc.event)
		}
		ts.NotContains(ts.keys(line.Raw), "details", "%s: the line carries no part of the agent type", tc.event)
		ts.NotContains(line.Raw, displayName, "no attribute of the agent type is recorded")
		ts.NotContains(line.Raw, auditTypeAttribute, "no attribute definition is recorded")
	}
}

// An import that creates a user type writes the entity type service's event for it and nothing else:
// the lookup that decides between update and create writes no line, so no failed update precedes the
// create.
func (ts *AuditLogTestSuite) TestAnImportWritesTheEventOfTheUserTypeItCreates() {
	ouID := ts.createOrganizationUnit("type-import")
	typeID := unusedID()
	handle := ts.prefix + "imported-type"

	imported, result := ts.importUpsert(fmt.Sprintf("resource_type: user_type\nid: %s\nhandle: %s\n"+
		"displayName: Audit Imported Type\nouId: %s\nschema:\n  %s:\n    type: string\n",
		typeID, handle, ouID, auditTypeAttribute))
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(typeID) })
	ts.Require().Equal("create", result.Results[0].Operation)

	line := ts.resourceLineOfImport(imported, map[string]string{"result": "succeeded"})
	typeRef := testutils.AuditRef{Type: "user_type", ID: typeID}
	ts.Equal("user_type.create", line.Event)
	ts.Equal(typeRef, line.Target)
	ts.Equal(typeRef, line.Subject)
	ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome)
	ts.requireNoTypeValue(line, handle, "Audit Imported Type")
}

// An import that changes a user type the management API created writes one succeeded update and no
// failed event: the importer finds the type before it updates it.
func (ts *AuditLogTestSuite) TestAnImportWritesTheEventOfTheUserTypeItUpdates() {
	ouID := ts.createOrganizationUnit("type-reimport")
	handle := ts.prefix + "reimported-type"
	typeID, err := testutils.CreateUserType(testutils.UserType{
		Handle: handle, DisplayName: "Audit Reimported Type", OUID: ouID,
		Schema: map[string]interface{}{auditTypeAttribute: map[string]interface{}{"type": "string"}},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(typeID) })

	imported, result := ts.importUpsert(fmt.Sprintf("resource_type: user_type\nid: %s\nhandle: %s\n"+
		"displayName: Audit Reimported Type\nouId: %s\nschema:\n  %s:\n    type: string\n"+
		"  %s_issued:\n    type: number\n", typeID, handle, ouID, auditTypeAttribute, auditTypeAttribute))
	ts.Require().Equal("update", result.Results[0].Operation)

	written := ts.linesPerCall(imported)[0]
	ts.requireOnlyImportedUpdate(written, testutils.AuditRef{Type: "user_type", ID: typeID}, "user_type.update")
	ts.requireNoTypeValue(written[0], handle, "Audit Reimported Type")
}

// The default agent type is imported through the same adapter, so an import that changes it writes one
// succeeded agent type update and no failed event.
func (ts *AuditLogTestSuite) TestAnImportWritesTheEventOfTheAgentTypeItUpdates() {
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { ts.NoError(testutils.RestoreAgentType(snapshot)) })
	schema := make(map[string]interface{}, len(snapshot.Schema)+1)
	for name, definition := range snapshot.Schema {
		schema[name] = definition
	}
	schema[auditTypeAttribute] = map[string]interface{}{"type": "string"}
	displayName := "Audit Imported Agents"
	document, err := json.Marshal(map[string]interface{}{
		"resource_type": "agent_type", "id": snapshot.ID, "handle": "default", "displayName": displayName,
		"ouId": snapshot.OUID, "allowSelfRegistration": snapshot.AllowSelfRegistration,
		"systemAttributes": snapshot.SystemAttributes, "schema": schema,
	})
	ts.Require().NoError(err)

	imported, result := ts.importUpsert(string(document))
	ts.Require().Equal("update", result.Results[0].Operation)

	written := ts.linesPerCall(imported)[0]
	ts.requireOnlyImportedUpdate(written, testutils.AuditRef{Type: "agent_type", ID: snapshot.ID},
		"agent_type.update")
	ts.NotContains(written[0].Raw, displayName, "no attribute of the agent type is recorded")
	ts.NotContains(written[0].Raw, auditTypeAttribute, "no attribute definition is recorded")
}

// importUpsert imports one document into the runtime with upsert and returns the response and its
// result, which must report the document imported.
func (ts *AuditLogTestSuite) importUpsert(content string) (*http.Response, testutils.ImportResponse) {
	ts.T().Helper()
	request, err := json.Marshal(testutils.ImportRequest{
		Content: content,
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	ts.Require().NoError(err)
	imported, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/import", string(request))
	ts.Require().Equal(http.StatusOK, imported.StatusCode, string(body))
	var result testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Equal(1, result.Summary.Imported, string(body))
	ts.Require().Len(result.Results, 1, string(body))
	return imported, result
}

// requireOnlyImportedUpdate checks that an import wrote exactly one resource line, a succeeded update of
// typeRef named event by the admin, and no failed event. The import's own configuration.import line is
// not a resource line.
func (ts *AuditLogTestSuite) requireOnlyImportedUpdate(written []testutils.AuditLine, typeRef testutils.AuditRef,
	event string) {
	ts.T().Helper()
	var resourceLines []testutils.AuditLine
	for _, line := range written {
		ts.NotEqual("failed", line.Outcome["result"], "the import wrote a failed event: %s", line.Raw)
		if line.Event != importEvent {
			resourceLines = append(resourceLines, line)
		}
	}
	ts.Require().Len(resourceLines, 1, "the import did not write exactly one resource line")
	line := resourceLines[0]
	ts.Equal(event, line.Event)
	ts.Equal("update", line.Action)
	ts.Equal(typeRef, line.Target)
	ts.Equal(typeRef, line.Subject)
	ts.Equal(testutils.AuditRef{Type: "user", ID: ts.adminUserID()}, line.Actor)
	ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome)
}

// requireNoTypeValue checks that the line carries no handle, no display name and no attribute definition
// of an entity type.
func (ts *AuditLogTestSuite) requireNoTypeValue(line testutils.AuditLine, handle, displayName string) {
	ts.T().Helper()
	ts.NotContains(line.Raw, handle, "the handle is not recorded")
	ts.NotContains(line.Raw, displayName, "the display name is not recorded")
	ts.NotContains(line.Raw, auditTypeAttribute, "no attribute definition is recorded")
}

// entityTypeBody is the request body of a user type or agent type whose schema defines one attribute.
func (ts *AuditLogTestSuite) entityTypeBody(handle, displayName, ouID string) string {
	ts.T().Helper()
	body, err := json.Marshal(testutils.UserType{
		Handle: handle, DisplayName: displayName, OUID: ouID,
		Schema: map[string]interface{}{auditTypeAttribute: map[string]interface{}{"type": "string"}},
	})
	ts.Require().NoError(err)
	return string(body)
}
