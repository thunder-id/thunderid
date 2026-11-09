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

const auditOUDescription = "Audited organization unit description"

// Creating, updating and deleting an organization unit through the management API writes one line each,
// with the admin as actor and the unit as target and subject. Neither the unit's name, its handle nor its
// description appears in any line.
func (ts *AuditLogTestSuite) TestAnOrganizationUnitLifecycleWritesOneLinePerChange() {
	adminID := ts.adminUserID()
	// The API creates no root unit on a data plane, so the unit lives under one.
	parent := ts.createOrganizationUnit("ou-lc-parent")
	handle := ts.prefix + "ou-lifecycle"
	name := "Audited Lifecycle Unit " + ts.prefix

	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/organization-units",
		ts.ouBody(handle, name, &parent))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	ouID := ts.idOf(body)
	ts.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(ouID) })
	updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, "/organization-units/"+ouID,
		ts.ouBody(handle, name+" renamed", &parent))
	ts.Require().Equal(http.StatusOK, updated.StatusCode, string(body))
	deleted := ts.send(http.MethodDelete, "/organization-units/"+ouID, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	ouRef := testutils.AuditRef{Type: "ou", ID: ouID}
	lines := ts.linesOf(created, updated, deleted)
	for i, tc := range []struct {
		event  string
		action string
	}{
		{"ou.create", "create"},
		{"ou.update", "update"},
		{"ou.delete", "delete"},
	} {
		line := lines[i]
		ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
		ts.Equal(lineKeys, ts.keys(line.Raw), tc.event)
		ts.Equal(tc.event, line.Event)
		ts.Equal(tc.action, line.Action)
		ts.Equal(ouRef, line.Target, tc.event)
		ts.Equal(ouRef, line.Subject, tc.event)
		ts.Equal(testutils.AuditRef{Type: "user", ID: adminID}, line.Actor, tc.event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, tc.event)
		ts.Nil(line.Client, "%s: a loopback peer is not recorded", tc.event)
		ts.NotContains(line.Raw, handle, "the handle is not recorded")
		ts.NotContains(line.Raw, "Audited Lifecycle Unit", "the name is not recorded")
		ts.NotContains(line.Raw, auditOUDescription, "the description is not recorded")
	}
}

// Every path that changes an organization unit records its attempt. An attempt by handle path names the
// unit the path resolved to, and none when it resolved to none. A succeeded or denied attempt on a unit
// that exists has it as subject; a failed attempt, and a create that made no unit, has none. A delete of
// a unit that does not exist fails, so it is recorded.
func (ts *AuditLogTestSuite) TestEveryOrganizationUnitPathRecordsItsOutcome() {
	// createOrganizationUnit gives a unit the handle the prefix, its name and "-ou" make, which is the
	// handle path of a root unit.
	pathHandle := ts.prefix + "ou-paths-ou"
	byPathID := ts.createOrganizationUnit("ou-paths")
	removable := ts.createOrganizationUnit("ou-removable")
	parent := ts.createOrganizationUnit("ou-parent")
	child := ts.createChildOrganizationUnit("ou-child", parent)
	sibling := ts.createOrganizationUnit("ou-sibling")
	manager := ts.createScopedManager("ou-mgr", []string{"ou"}, []string{"org:ou"})
	managerClient := manager.client
	managerRef := testutils.AuditRef{Type: "user", ID: manager.id}
	missing := unusedID()
	noUnit := testutils.AuditRef{Type: "ou"}

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
			name: "an update of a unit that does not exist", client: testutils.GetHTTPClient(),
			method: http.MethodPut, path: "/organization-units/" + missing,
			body:   ts.ouBody(ts.prefix+"ou-missing", "Audited Missing Unit", nil),
			status: http.StatusNotFound, event: "ou.update",
			outcome: map[string]string{"result": "failed", "reason": "ou_1003"},
			target:  testutils.AuditRef{Type: "ou", ID: missing},
		},
		{
			name: "a delete of a unit that does not exist", client: testutils.GetHTTPClient(),
			method: http.MethodDelete, path: "/organization-units/" + missing,
			status: http.StatusNotFound, event: "ou.delete",
			outcome: map[string]string{"result": "failed", "reason": "ou_1003"},
			target:  testutils.AuditRef{Type: "ou", ID: missing},
		},
		{
			name: "a create with a handle a sibling unit has", client: testutils.GetHTTPClient(),
			method: http.MethodPost, path: "/organization-units",
			body:   ts.ouBody(ts.prefix+"ou-child-ou", "Audited Duplicate Unit", &parent),
			status: http.StatusConflict, event: "ou.create",
			outcome: map[string]string{"result": "failed", "reason": "ou_1008"}, target: noUnit,
		},
		{
			// A root unit reaches a data plane only through an import with the management API key, so the
			// service refuses even the admin's token.
			name: "a create of a root unit with a token", client: testutils.GetHTTPClient(),
			method: http.MethodPost, path: "/organization-units",
			body:   ts.ouBody(ts.prefix+"ou-root", "Audited Root Unit", nil),
			status: http.StatusForbidden, event: "ou.create",
			outcome: map[string]string{"result": "denied", "reason": "forbidden"}, target: noUnit,
		},
		{
			name: "an update that moves a unit to the root with a token", client: testutils.GetHTTPClient(),
			method: http.MethodPut, path: "/organization-units/" + child,
			body:   ts.ouBody(ts.prefix+"ou-child-ou", "Audited Moved Unit", nil),
			status: http.StatusForbidden, event: "ou.update",
			outcome: map[string]string{"result": "denied", "reason": "forbidden"},
			target:  testutils.AuditRef{Type: "ou", ID: child}, subject: true,
		},
		{
			name: "an update by handle path", client: testutils.GetHTTPClient(),
			method: http.MethodPut, path: "/organization-units/tree/" + pathHandle,
			body:   ts.ouBody(pathHandle, "Audited Path Unit renamed", nil),
			status: http.StatusOK, event: "ou.update", outcome: map[string]string{"result": "succeeded"},
			target: testutils.AuditRef{Type: "ou", ID: byPathID}, subject: true,
		},
		{
			name: "an update by a handle path that resolves to no unit", client: testutils.GetHTTPClient(),
			method: http.MethodPut, path: "/organization-units/tree/" + ts.prefix + "no-such-ou",
			body:   ts.ouBody(ts.prefix+"no-such-ou", "Audited Missing Unit", nil),
			status: http.StatusNotFound, event: "ou.update",
			outcome: map[string]string{"result": "failed", "reason": "ou_1003"}, target: noUnit,
		},
		{
			name: "a delete by a handle path that resolves to no unit", client: testutils.GetHTTPClient(),
			method: http.MethodDelete, path: "/organization-units/tree/" + ts.prefix + "no-such-ou",
			status: http.StatusNotFound, event: "ou.delete",
			outcome: map[string]string{"result": "failed", "reason": "ou_1003"}, target: noUnit,
		},
		{
			name: "a delete of a unit with a child", client: testutils.GetHTTPClient(),
			method: http.MethodDelete, path: "/organization-units/" + parent,
			status: http.StatusBadRequest, event: "ou.delete",
			outcome: map[string]string{"result": "failed", "reason": "ou_1006"},
			target:  testutils.AuditRef{Type: "ou", ID: parent},
		},
		{
			name: "a delete by handle path", client: testutils.GetHTTPClient(),
			method: http.MethodDelete, path: "/organization-units/tree/" + ts.prefix + "ou-removable-ou",
			status: http.StatusNoContent, event: "ou.delete", outcome: map[string]string{"result": "succeeded"},
			target: testutils.AuditRef{Type: "ou", ID: removable}, subject: true,
		},
		{
			name: "an update of a unit the caller does not manage", client: managerClient,
			method: http.MethodPut, path: "/organization-units/" + sibling,
			body:   ts.ouBody(ts.prefix+"ou-sibling-ou", "Audited Intruded Unit", nil),
			status: http.StatusForbidden, event: "ou.update",
			outcome: map[string]string{"result": "denied", "reason": "forbidden"},
			target:  testutils.AuditRef{Type: "ou", ID: sibling}, subject: true,
		},
		{
			name: "a create under a unit the caller does not manage", client: managerClient,
			method: http.MethodPost, path: "/organization-units",
			body:   ts.ouBody(ts.prefix+"ou-intruder", "Audited Intruder Unit", &sibling),
			status: http.StatusForbidden, event: "ou.create",
			outcome: map[string]string{"result": "denied", "reason": "forbidden"}, target: noUnit,
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
		ts.NotContains(line.Raw, "Audited", "%s: no name is recorded", tc.name)
		ts.NotContains(line.Raw, ts.prefix, "%s: no handle or handle path is recorded", tc.name)
	}
}

// An import writes the organization unit service's event for each unit and nothing else: the lookup that
// decides between update and create writes no line, so a new unit is not preceded by a failed update.
func (ts *AuditLogTestSuite) TestAnImportWritesTheEventOfEachOrganizationUnit() {
	existing := ts.createOrganizationUnit("ou-import-existing")
	newID := unusedID()
	// The new unit is a child, since the admin's token imports no root unit on a data plane.
	request, err := json.Marshal(testutils.ImportRequest{
		Content: fmt.Sprintf("resource_type: organization_unit\nid: %s\nhandle: %s\nname: %s\nparent: %s\n---\n"+
			"resource_type: organization_unit\nid: %s\nhandle: %s\nname: %s\n",
			newID, ts.prefix+"ou-imported", "Audited Imported Unit", existing,
			existing, ts.prefix+"ou-import-existing-ou", "Audited Reimported Unit"),
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	ts.Require().NoError(err)

	imported, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/import", string(request))
	ts.Require().Equal(http.StatusOK, imported.StatusCode, string(body))
	ts.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(newID) })
	var result testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Equal(2, result.Summary.Imported, string(body))

	// The import may also write its own configuration.import line, which is not what this test is about.
	var lines []testutils.AuditLine
	for _, line := range ts.linesPerCall(imported)[0] {
		if line.Event != "configuration.import" {
			lines = append(lines, line)
		}
	}
	ts.Require().Len(lines, 2, "one line for each unit, and none for the lookups")
	for i, tc := range []struct {
		event string
		id    string
	}{
		{"ou.create", newID},
		{"ou.update", existing},
	} {
		ouRef := testutils.AuditRef{Type: "ou", ID: tc.id}
		ts.Equal(tc.event, lines[i].Event)
		ts.Equal(ouRef, lines[i].Target, tc.event)
		ts.Equal(ouRef, lines[i].Subject, tc.event)
		ts.Equal(map[string]string{"result": "succeeded"}, lines[i].Outcome, tc.event)
		ts.NotContains(lines[i].Raw, "Audited", "no name is recorded")
	}
}

// ouBody is the request body of an organization unit with a description, under parent when one is given.
func (ts *AuditLogTestSuite) ouBody(handle, name string, parent *string) string {
	ts.T().Helper()
	body, err := json.Marshal(testutils.OrganizationUnit{
		Handle: handle, Name: name, Description: auditOUDescription, Parent: parent,
	})
	ts.Require().NoError(err)
	return string(body)
}

// createChildOrganizationUnit creates an organization unit under parent, removed when the test ends, and
// returns its ID.
func (ts *AuditLogTestSuite) createChildOrganizationUnit(name, parent string) string {
	ts.T().Helper()
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.prefix + name + "-ou",
		Name:   "Audit Log Test OU " + name,
		Parent: &parent,
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(ouID) })
	return ouID
}
