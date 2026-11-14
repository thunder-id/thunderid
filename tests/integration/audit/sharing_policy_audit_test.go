// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// sharingPolicyBody is a policy reaching every unit beneath its issuer. An update restates it with the
// version it changes.
func sharingPolicyBody(version int) string {
	if version == 0 {
		return `{"childUnits":{"mode":"all"}}`
	}
	return fmt.Sprintf(`{"version":%d,"childUnits":{"mode":"all"}}`, version)
}

// Creating, updating and deleting a sharing policy of a user writes one line each, with the policy as
// target and the user as subject. A delete of a policy the user does not hold still succeeds and writes
// no line. No organization unit the policy names appears in any line.
func (ts *AuditLogTestSuite) TestAUserSharingPolicyLifecycleWritesOneLinePerChange() {
	ouID, userType := ts.createUserType("share-user")
	ts.createChildOrganizationUnit("share-user-child", ouID)
	userID := ts.createUser(ouID, userType, ts.prefix+"shared", "AuditShared123!")
	base := "/users/" + userID + "/sharing-policies"

	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, base, sharingPolicyBody(0))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	policyID := ts.idOf(body)
	ts.T().Cleanup(func() { ts.send(http.MethodDelete, base+"/"+policyID, "") })
	updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, base+"/"+policyID,
		sharingPolicyBody(1))
	ts.Require().Equal(http.StatusOK, updated.StatusCode, string(body))
	deleted := ts.send(http.MethodDelete, base+"/"+policyID, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)
	deletedAgain := ts.send(http.MethodDelete, base+"/"+policyID, "")
	ts.Require().Equal(http.StatusNoContent, deletedAgain.StatusCode)

	ts.requireSharingPolicyLifecycle(ts.linesPerCall(created, updated, deleted, deletedAgain), "user",
		testutils.AuditRef{Type: "user", ID: userID}, policyID, ouID)
}

// Creating, updating and deleting a sharing policy of an application writes one line each, with the
// policy as target and the application as subject. A delete of a policy the application does not hold
// still succeeds and writes no line.
func (ts *AuditLogTestSuite) TestAnApplicationSharingPolicyLifecycleWritesOneLinePerChange() {
	ouID := ts.createOrganizationUnit("share-app")
	ts.createChildOrganizationUnit("share-app-child", ouID)
	appID := ts.createApplication(ouID, "share-app")
	base := "/applications/" + appID + "/sharing-policies"

	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, base, sharingPolicyBody(0))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	policyID := ts.idOf(body)
	ts.T().Cleanup(func() { ts.send(http.MethodDelete, base+"/"+policyID, "") })
	updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, base+"/"+policyID,
		sharingPolicyBody(1))
	ts.Require().Equal(http.StatusOK, updated.StatusCode, string(body))
	deleted := ts.send(http.MethodDelete, base+"/"+policyID, "")
	ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)
	deletedAgain := ts.send(http.MethodDelete, base+"/"+policyID, "")
	ts.Require().Equal(http.StatusNoContent, deletedAgain.StatusCode)

	ts.requireSharingPolicyLifecycle(ts.linesPerCall(created, updated, deleted, deletedAgain), "application",
		testutils.AuditRef{Type: "application", ID: appID}, policyID, ouID)
}

// A sharing policy change that fails records the policy it named, or none on a create, and no subject.
// A user manager scoped to one unit is refused a policy for a user of another unit, which is recorded as
// denied with the user, who exists, as subject.
func (ts *AuditLogTestSuite) TestEverySharingPolicyPathRecordsItsOutcome() {
	siblingOU, siblingType := ts.createUserType("share-sibling")
	siblingUserID := ts.createUser(siblingOU, siblingType, ts.prefix+"share-sibling", "AuditSibling123!")
	appOU := ts.createOrganizationUnit("share-paths-app")
	appID := ts.createApplication(appOU, "share-paths-app")
	managerID, managerClient := ts.createScopedUserManager("share-mgr")
	missing := unusedID()

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
		subject testutils.AuditRef
	}{
		{
			name: "a policy for a user that does not exist", client: testutils.GetHTTPClient(),
			method: http.MethodPost, path: "/users/" + missing + "/sharing-policies", body: sharingPolicyBody(0),
			status: http.StatusNotFound, event: "user.sharing_policy.create",
			outcome: map[string]string{"result": "failed", "reason": "usr_1003"},
			target:  testutils.AuditRef{Type: "user_sharing_policy"},
		},
		{
			name: "an update of a policy the user does not hold", client: testutils.GetHTTPClient(),
			method: http.MethodPut, path: "/users/" + siblingUserID + "/sharing-policies/" + missing,
			body: sharingPolicyBody(1), status: http.StatusNotFound, event: "user.sharing_policy.update",
			outcome: map[string]string{"result": "failed", "reason": "usr_1030"},
			target:  testutils.AuditRef{Type: "user_sharing_policy", ID: missing},
		},
		{
			name: "a policy for a user of a unit the caller does not manage", client: managerClient,
			method: http.MethodPost, path: "/users/" + siblingUserID + "/sharing-policies",
			body: sharingPolicyBody(0), status: http.StatusForbidden, event: "user.sharing_policy.create",
			outcome: map[string]string{"result": "denied", "reason": "forbidden"},
			target:  testutils.AuditRef{Type: "user_sharing_policy"},
			subject: testutils.AuditRef{Type: "user", ID: siblingUserID},
		},
		{
			name: "an update of a policy the application does not hold", client: testutils.GetHTTPClient(),
			method: http.MethodPut, path: "/applications/" + appID + "/sharing-policies/" + missing,
			body: sharingPolicyBody(1), status: http.StatusNotFound, event: "application.sharing_policy.update",
			outcome: map[string]string{"result": "failed", "reason": "app_1052"},
			target:  testutils.AuditRef{Type: "application_sharing_policy", ID: missing},
		},
		{
			name: "a policy for an application that does not exist", client: testutils.GetHTTPClient(),
			method: http.MethodPost, path: "/applications/" + missing + "/sharing-policies",
			body: sharingPolicyBody(0), status: http.StatusNotFound, event: "application.sharing_policy.create",
			outcome: map[string]string{"result": "failed", "reason": "app_1001"},
			target:  testutils.AuditRef{Type: "application_sharing_policy"},
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
		ts.Equal(tc.subject, line.Subject, tc.name)
		if tc.client == managerClient {
			ts.Equal(testutils.AuditRef{Type: "user", ID: managerID}, line.Actor, tc.name)
		}
	}
}

// requireSharingPolicyLifecycle checks the lines of a create, an update, a delete and a repeated delete of
// the policy with policyID of a resource of resourceType: one line for each change, with subject as its
// subject, and none for the repeated delete. None names ouID, the unit the policy shares from.
func (ts *AuditLogTestSuite) requireSharingPolicyLifecycle(perCall [][]testutils.AuditLine, resourceType string,
	subject testutils.AuditRef, policyID, ouID string) {
	ts.T().Helper()
	ts.Require().Len(perCall, 4)
	ts.Empty(perCall[3], "a delete that removed nothing writes no line")
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	policyRef := testutils.AuditRef{Type: resourceType + "_sharing_policy", ID: policyID}
	for i, action := range []string{"create", "update", "delete"} {
		event := resourceType + ".sharing_policy." + action
		ts.Require().Len(perCall[i], 1, event)
		line := perCall[i][0]
		ts.Equal(lineKeys, ts.keys(line.Raw), event)
		ts.Equal(event, line.Event)
		ts.Equal(action, line.Action)
		ts.Equal(policyRef, line.Target, event)
		ts.Equal(subject, line.Subject, event)
		ts.Equal(adminRef, line.Actor, event)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, event)
		ts.NotContains(line.Raw, ouID, "%s: no organization unit of the policy is recorded", event)
	}
}

// createApplication creates an application in ouID, removed when the test ends, and returns its ID.
func (ts *AuditLogTestSuite) createApplication(ouID, name string) string {
	ts.T().Helper()
	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/applications",
		ts.applicationBody(ts.prefix+name, ouID, ts.authFlowID(), ts.prefix+name+"-client"))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	var app struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(body, &app))
	ts.Require().NotEmpty(app.ID)
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(app.ID) })
	return app.ID
}
