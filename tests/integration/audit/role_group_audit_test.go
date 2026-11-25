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
	// Role and group names hold at most 50 characters, which PostgreSQL enforces and SQLite does not.
	// A name is unique only within its organization unit, and each test makes its own, so names leave
	// out ts.prefix.
	auditRoleName        = "Audit Librarian"
	auditGroupName       = "Audit Readers"
	auditRoleGroupDetail = "Audit membership description"
)

// expectedLine is what one audit line of a call records: the event, the action and the subject.
type expectedLine struct {
	event   string
	action  string
	subject testutils.AuditRef
}

// Creating a role with an assignee, updating it, assigning and unassigning it, and deleting it through
// the management API writes a line for each change and one more for each assignee the change grants or
// takes the role from, with the admin as actor, the role as target and the assignee as subject. No name,
// description or permission appears in any line.
func (ts *AuditLogTestSuite) TestARoleLifecycleWritesALinePerChangeAndAssignee() {
	ouID, userType := ts.createUserType("role-lifecycle")
	adminID := ts.adminUserID()
	userID := ts.createUser(ouID, userType, ts.prefix+"role-assignee", "AuditRoleAssignee123!")
	groupID := ts.createGroup(ouID, auditGroupName)

	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/roles", ts.jsonBody(map[string]any{
		"name": auditRoleName, "description": auditRoleGroupDetail, "ouId": ouID,
		"assignments": []map[string]string{{"id": userID, "type": "user"}},
	}))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	roleID := ts.idOf(body)
	ts.T().Cleanup(func() { _ = testutils.DeleteRole(roleID) })
	updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, "/roles/"+roleID, ts.jsonBody(
		map[string]any{"name": auditRoleName + " Renamed", "description": auditRoleGroupDetail, "ouId": ouID}))
	ts.Require().Equal(http.StatusOK, updated.StatusCode, string(body))
	added, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/roles/"+roleID+"/assignments/add",
		ts.jsonBody(map[string]any{"assignments": []map[string]string{{"id": groupID, "type": "group"}}}))
	ts.Require().Equal(http.StatusNoContent, added.StatusCode, string(body))
	removed, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost,
		"/roles/"+roleID+"/assignments/remove",
		ts.jsonBody(map[string]any{"assignments": []map[string]string{{"id": userID, "type": "user"}}}))
	ts.Require().Equal(http.StatusNoContent, removed.StatusCode, string(body))
	deleted := ts.send(http.MethodDelete, "/roles/"+roleID, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	roleRef := testutils.AuditRef{Type: "role", ID: roleID}
	userRef := testutils.AuditRef{Type: "user", ID: userID}
	groupRef := testutils.AuditRef{Type: "group", ID: groupID}
	ts.requireLinesPerCall(roleRef, adminID, []*http.Response{created, updated, added, removed, deleted},
		[][]expectedLine{
			{{"role.create", "create", roleRef}, {"role.assignment.add", "add", userRef}},
			{{"role.update", "update", roleRef}},
			{{"role.assignment.add", "add", groupRef}},
			{{"role.assignment.remove", "remove", userRef}},
			{{"role.delete", "delete", roleRef}},
		}, auditRoleName)
}

// Creating a group with a member, updating it, adding and removing members, creating one under an
// organization unit path and deleting it through the management API writes a line for each change and
// one more for each member the change adds or removes, with the group as target and the member as
// subject. No name or description appears in any line.
func (ts *AuditLogTestSuite) TestAGroupLifecycleWritesALinePerChangeAndMember() {
	ouID, userType := ts.createUserType("group-lifecycle")
	ouHandle := ts.prefix + "group-lifecycle-ou"
	adminID := ts.adminUserID()
	firstID := ts.createUser(ouID, userType, ts.prefix+"group-first", "AuditGroupFirst123!")
	secondID := ts.createUser(ouID, userType, ts.prefix+"group-second", "AuditGroupSecond123!")
	nestedID := ts.createGroup(ouID, "Audit Nested Readers")

	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/groups", ts.jsonBody(map[string]any{
		"name": auditGroupName, "description": auditRoleGroupDetail, "ouId": ouID,
		"members": []map[string]string{{"id": firstID, "type": "user"}},
	}))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	groupID := ts.idOf(body)
	ts.T().Cleanup(func() { _ = testutils.DeleteGroup(groupID) })
	updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, "/groups/"+groupID, ts.jsonBody(
		map[string]any{"name": auditGroupName + " Renamed", "description": auditRoleGroupDetail, "ouId": ouID}))
	ts.Require().Equal(http.StatusOK, updated.StatusCode, string(body))
	added, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/groups/"+groupID+"/members/add",
		ts.jsonBody(map[string]any{"members": []map[string]string{
			{"id": secondID, "type": "user"}, {"id": nestedID, "type": "group"},
		}}))
	ts.Require().Equal(http.StatusOK, added.StatusCode, string(body))
	removed, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/groups/"+groupID+"/members/remove",
		ts.jsonBody(map[string]any{"members": []map[string]string{{"id": firstID, "type": "user"}}}))
	ts.Require().Equal(http.StatusOK, removed.StatusCode, string(body))
	deleted := ts.send(http.MethodDelete, "/groups/"+groupID, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	groupRef := testutils.AuditRef{Type: "group", ID: groupID}
	ts.requireLinesPerCall(groupRef, adminID, []*http.Response{created, updated, added, removed, deleted},
		[][]expectedLine{
			{{"group.create", "create", groupRef}, {"group.member.add", "add", testutils.AuditRef{Type: "user",
				ID: firstID}}},
			{{"group.update", "update", groupRef}},
			{
				{"group.member.add", "add", testutils.AuditRef{Type: "user", ID: secondID}},
				{"group.member.add", "add", testutils.AuditRef{Type: "group", ID: nestedID}},
			},
			{{"group.member.remove", "remove", testutils.AuditRef{Type: "user", ID: firstID}}},
			{{"group.delete", "delete", groupRef}},
		}, auditGroupName)

	byPath, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/groups/tree/"+ouHandle,
		ts.jsonBody(map[string]any{"name": auditGroupName, "description": auditRoleGroupDetail,
			"members": []map[string]string{{"id": secondID, "type": "user"}}}))
	ts.Require().Equal(http.StatusCreated, byPath.StatusCode, string(body))
	byPathID := ts.idOf(body)
	ts.T().Cleanup(func() { _ = testutils.DeleteGroup(byPathID) })
	byPathRef := testutils.AuditRef{Type: "group", ID: byPathID}
	ts.requireLinesPerCall(byPathRef, adminID, []*http.Response{byPath}, [][]expectedLine{{
		{"group.create", "create", byPathRef},
		{"group.member.add", "add", testutils.AuditRef{Type: "user", ID: secondID}},
	}}, auditGroupName)
}

// A change to a role or a group that does not exist reaches the service, which records the failed
// attempt with the caller's ID as target and no subject, and no member or assignee: the call granted
// nothing.
func (ts *AuditLogTestSuite) TestAChangeOfAMissingRoleOrGroupIsRecordedAsFailed() {
	missing := unusedID()
	ouID := ts.createOrganizationUnit("role-group-missing")
	adminID := ts.adminUserID()
	update := ts.jsonBody(map[string]any{"name": "Audit Nobody", "ouId": ouID})
	assignees := ts.jsonBody(map[string]any{"assignments": []map[string]string{{"id": adminID, "type": "user"}}})
	members := ts.jsonBody(map[string]any{"members": []map[string]string{{"id": adminID, "type": "user"}}})
	missingRole := testutils.AuditRef{Type: "role", ID: missing}
	missingGroup := testutils.AuditRef{Type: "group", ID: missing}

	cases := []struct {
		method, path, body string
		status             int
		event, reason      string
		target             testutils.AuditRef
	}{
		{http.MethodPost, "/roles", ts.jsonBody(map[string]any{"name": "Audit Orphan", "ouId": missing}),
			http.StatusBadRequest, "role.create", "rol_1005", testutils.AuditRef{Type: "role"}},
		{http.MethodPut, "/roles/" + missing, update, http.StatusNotFound, "role.update", "rol_1003", missingRole},
		{http.MethodPost, "/roles/" + missing + "/assignments/add", assignees, http.StatusNotFound,
			"role.assignment.add", "rol_1003", missingRole},
		{http.MethodPost, "/roles/" + missing + "/assignments/remove", assignees, http.StatusNotFound,
			"role.assignment.remove", "rol_1003", missingRole},
		{http.MethodPost, "/groups/tree/" + ts.prefix + "no-such-ou", ts.jsonBody(map[string]any{
			"name": "Audit Orphan"}), http.StatusNotFound, "group.create", "grp_1003", testutils.AuditRef{Type: "group"}},
		{http.MethodPut, "/groups/" + missing, update, http.StatusNotFound, "group.update", "grp_1003", missingGroup},
		{http.MethodPost, "/groups/" + missing + "/members/add", members, http.StatusNotFound,
			"group.member.add", "grp_1003", missingGroup},
		{http.MethodPost, "/groups/" + missing + "/members/remove", members, http.StatusNotFound,
			"group.member.remove", "grp_1003", missingGroup},
		{http.MethodDelete, "/groups/" + missing, "", http.StatusNotFound, "group.delete", "grp_1003", missingGroup},
	}
	responses := make([]*http.Response, 0, len(cases)+1)
	for _, tc := range cases {
		response, body := ts.exchange(testutils.GetHTTPClient(), tc.method, tc.path, tc.body)
		ts.Require().Equal(tc.status, response.StatusCode, "%s %s: %s", tc.method, tc.path, string(body))
		responses = append(responses, response)
	}
	lines := ts.linesOf(responses...)
	for i, tc := range cases {
		line := lines[i]
		ts.Equal(tc.event, line.Event, tc.path)
		ts.Equal(tc.target, line.Target, tc.path)
		ts.Equal(testutils.AuditRef{}, line.Subject, "%s: a failed attempt names no subject", tc.path)
		ts.Equal(map[string]string{"result": "failed", "reason": tc.reason}, line.Outcome, tc.path)
		ts.Equal(testutils.AuditRef{Type: "user", ID: adminID}, line.Actor, tc.path)
	}
}

// A delete of a role that does not exist still succeeds, but writes no line, so a caller cannot record
// the deletion of an ID it made up.
func (ts *AuditLogTestSuite) TestADeleteOfAMissingRoleWritesNoLine() {
	deleted := ts.send(http.MethodDelete, "/roles/"+unusedID(), "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

	ts.Empty(ts.linesPerCall(deleted)[0], "a delete that removed nothing writes no line")
}

// A group manager scoped to one organization unit passes the security layer with org:group, and the
// group service refuses its changes to a group in a sibling unit, and its addition of a member to a group
// in its own unit that confers a permission the manager does not hold. Each refusal is recorded as
// denied. A refused change to a group names the group it found as subject; a refused membership change
// names none, since it granted nothing.
func (ts *AuditLogTestSuite) TestAGroupChangeTheGroupServiceRefusesIsRecordedAsDenied() {
	manager := ts.createScopedManager("group-scoped", []string{"group", "user"},
		[]string{"org:group", "org:group:view"})
	siblingOU := ts.createOrganizationUnit("group-sibling")
	siblingID := ts.createGroup(siblingOU, auditGroupName)
	privilegedID := ts.createGroup(manager.ouID, "Audit Privileged Readers")
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "Audit User Admin",
		OUID: manager.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: manager.resourceServerID, Permissions: []string{"org:user"}},
		},
		Assignments: []testutils.Assignment{{ID: privilegedID, Type: "group"}},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteRole(roleID) })
	self := ts.jsonBody(map[string]any{"members": []map[string]string{{"id": manager.id, "type": "user"}}})
	siblingRef := testutils.AuditRef{Type: "group", ID: siblingID}

	cases := []struct {
		method, path, body, event string
		target, subject           testutils.AuditRef
	}{
		{http.MethodPost, "/groups", ts.jsonBody(map[string]any{"name": auditGroupName, "ouId": siblingOU}),
			"group.create", testutils.AuditRef{Type: "group"}, testutils.AuditRef{}},
		{http.MethodPut, "/groups/" + siblingID, ts.jsonBody(map[string]any{
			"name": auditGroupName + " Renamed", "ouId": siblingOU}), "group.update", siblingRef, siblingRef},
		{http.MethodPost, "/groups/" + siblingID + "/members/add", self, "group.member.add", siblingRef,
			testutils.AuditRef{}},
		{http.MethodPost, "/groups/" + privilegedID + "/members/add", self, "group.member.add",
			testutils.AuditRef{Type: "group", ID: privilegedID}, testutils.AuditRef{}},
		{http.MethodDelete, "/groups/" + siblingID, "", "group.delete", siblingRef, siblingRef},
	}
	responses := make([]*http.Response, 0, len(cases))
	for _, tc := range cases {
		response, body := ts.exchange(manager.client, tc.method, tc.path, tc.body)
		ts.Require().Equal(http.StatusForbidden, response.StatusCode, "%s %s: %s", tc.method, tc.path, string(body))
		responses = append(responses, response)
	}

	lines := ts.linesOf(responses...)
	for i, tc := range cases {
		line := lines[i]
		ts.Equal(tc.event, line.Event, tc.path)
		ts.Equal(testutils.AuditRef{Type: "user", ID: manager.id}, line.Actor, tc.path)
		ts.Equal(tc.target, line.Target, tc.path)
		ts.Equal(tc.subject, line.Subject, tc.path)
		ts.Equal(map[string]string{"result": "denied", "reason": "forbidden"}, line.Outcome, tc.path)
		ts.NotContains(line.Raw, auditGroupName, "%s: no name is recorded", tc.path)
	}
}

// An import that creates a group with a member and a role assigned to that group writes the events of
// the group service and the role service for them: each create, then each grant once the group and role
// exist, under the import's trace ID, then the import's own line. The lookups that decide between update
// and create write no line.
func (ts *AuditLogTestSuite) TestAnImportWritesTheEventsOfTheRolesAndGroupsItCreates() {
	ouID, userType := ts.createUserType("role-group-import")
	userID := ts.createUser(ouID, userType, ts.prefix+"imported-member", "AuditImportedMember123!")
	groupID := unusedID()
	roleID := unusedID()
	request, err := json.Marshal(testutils.ImportRequest{
		Content: fmt.Sprintf("resource_type: group\nid: %s\nname: %s\nouId: %s\nmembers:\n  - id: %s\n"+
			"    type: user\n---\nresource_type: role\nid: %s\nname: %s\nouId: %s\nassignments:\n"+
			"  - id: %s\n    type: group\n", groupID, auditGroupName, ouID, userID, roleID, auditRoleName, ouID,
			groupID),
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	ts.Require().NoError(err)

	imported, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/import", string(request))
	ts.Require().Equal(http.StatusOK, imported.StatusCode, string(body))
	ts.T().Cleanup(func() {
		_ = testutils.DeleteRole(roleID)
		_ = testutils.DeleteGroup(groupID)
	})
	var result testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Equal(2, result.Summary.Imported, string(body))

	groupRef := testutils.AuditRef{Type: "group", ID: groupID}
	roleRef := testutils.AuditRef{Type: "role", ID: roleID}
	expected := []struct {
		event           string
		target, subject testutils.AuditRef
	}{
		{"group.create", groupRef, groupRef},
		{"role.create", roleRef, roleRef},
		{"group.member.add", groupRef, testutils.AuditRef{Type: "user", ID: userID}},
		{"role.assignment.add", roleRef, groupRef},
		{importEvent, configurationRef, testutils.AuditRef{}},
	}
	lines := ts.linesPerCall(imported)[0]
	ts.Require().Len(lines, len(expected))
	for i, want := range expected {
		ts.Equal(want.event, lines[i].Event)
		ts.Equal(want.target, lines[i].Target, want.event)
		ts.Equal(want.subject, lines[i].Subject, want.event)
		ts.Equal(map[string]string{"result": "succeeded"}, lines[i].Outcome, want.event)
		ts.NotContains(lines[i].Raw, auditGroupName, "%s: no name is recorded", want.event)
		ts.NotContains(lines[i].Raw, auditRoleName, "%s: no name is recorded", want.event)
	}
}

// requireLinesPerCall checks that each call wrote the lines want lists for it, in order, each a
// succeeded attempt by the admin on target, and that no line carries name or the description.
func (ts *AuditLogTestSuite) requireLinesPerCall(target testutils.AuditRef, adminID string,
	responses []*http.Response, want [][]expectedLine, name string) {
	ts.T().Helper()
	for i, written := range ts.linesPerCall(responses...) {
		ts.Require().Len(written, len(want[i]), "call %d", i)
		for j, line := range written {
			expected := want[i][j]
			ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
			ts.Equal(lineKeys, ts.keys(line.Raw), expected.event)
			ts.Equal(expected.event, line.Event)
			ts.Equal(expected.action, line.Action, expected.event)
			ts.Equal(target, line.Target, expected.event)
			ts.Equal(expected.subject, line.Subject, expected.event)
			ts.Equal(testutils.AuditRef{Type: "user", ID: adminID}, line.Actor, expected.event)
			ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, expected.event)
			ts.Nil(line.Client, "%s: a loopback peer is not recorded", expected.event)
			ts.NotContains(line.Raw, name, "%s: no name is recorded", expected.event)
			ts.NotContains(line.Raw, auditRoleGroupDetail, "%s: no description is recorded", expected.event)
		}
	}
}

// createGroup creates a group named name in ouID, removed when the test ends, and returns its ID.
func (ts *AuditLogTestSuite) createGroup(ouID, name string) string {
	ts.T().Helper()
	groupID, err := testutils.CreateGroup(testutils.Group{Name: name, OUID: ouID})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteGroup(groupID) })
	return groupID
}

// jsonBody returns the JSON of a request body.
func (ts *AuditLogTestSuite) jsonBody(body map[string]any) string {
	ts.T().Helper()
	encoded, err := json.Marshal(body)
	ts.Require().NoError(err)
	return string(encoded)
}

// idOf returns the ID in the body of a create response.
func (ts *AuditLogTestSuite) idOf(body []byte) string {
	ts.T().Helper()
	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(body, &created))
	ts.Require().NotEmpty(created.ID)
	return created.ID
}
