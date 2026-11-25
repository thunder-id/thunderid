// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// importEvent, deleteEvent and exportEvent are the events of a request on the configuration as a whole.
	importEvent = "configuration.import"
	deleteEvent = "configuration.delete"
	exportEvent = "configuration.export"

	// managementAPIKey is the key whose digest the integration deployment configures as
	// server.security.management_api_key_hash.
	managementAPIKey = "integration-management-api-key"
)

// configurationRef is the target of every configuration event: the configuration as a whole, with no ID.
var configurationRef = testutils.AuditRef{Type: "configuration"}

// Each import request writes one configuration.import line after the lines of the resources it wrote,
// with the outcome of the request and whether it was a dry run, and nothing the request carried. A dry run
// writes no resource, so its line is the only one.
func (ts *AuditLogTestSuite) TestAnImportIsRecordedOnceWithItsOutcome() {
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	// The imported unit is a child, since the admin's token imports no root unit on a data plane.
	parentID := ts.createOrganizationUnit("cfg-import-parent")
	ouID := unusedID()
	ouHandle := ts.prefix + "cfg-import-ou"
	ts.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(ouID) })
	ouDocument := fmt.Sprintf("resource_type: organization_unit\nid: %s\nhandle: %s\nname: %s\nparent: %s\n",
		ouID, ouHandle, ouHandle, parentID)
	dryRunUserID := unusedID()
	const password = "AuditConfigurationImport123!"
	_, userType := ts.createUserType("cfg-dry")

	cases := []struct {
		name    string
		request testutils.ImportRequest
		status  int
		outcome map[string]string
		dryRun  string
		// only is set when the import line must be the only line the call wrote.
		only bool
	}{
		{
			name:    "an import that wrote every resource",
			request: testutils.ImportRequest{Content: ouDocument},
			status:  http.StatusOK, outcome: map[string]string{"result": "succeeded"}, dryRun: "false",
		},
		{
			name: "a dry run, which writes no resource",
			request: testutils.ImportRequest{DryRun: true, Content: fmt.Sprintf(
				"resource_type: user\nid: %s\ntype: %s\nouId: %s\nattributes:\n  username: %s\n"+
					"credentials:\n  password: %s\n", dryRunUserID, userType, ouID, ts.prefix+"cfg-dry", password)},
			status: http.StatusOK, outcome: map[string]string{"result": "succeeded"}, dryRun: "true", only: true,
		},
		{
			name: "an import with an item that failed",
			request: testutils.ImportRequest{Content: fmt.Sprintf(
				"resource_type: application\nid: %s\nname: %s\ntype: fullstack\nouId: %s\nauthFlowId: %s\n",
				unusedID(), ts.prefix+"cfg-failed-app", ouID, unusedID())},
			status:  http.StatusOK,
			outcome: map[string]string{"result": "failed", "reason": "item_failed"}, dryRun: "false",
		},
		{
			name:    "an import with no content",
			request: testutils.ImportRequest{},
			status:  http.StatusBadRequest,
			outcome: map[string]string{"result": "failed", "reason": "imp_1001"}, dryRun: "false", only: true,
		},
		{
			name:    "an import of content that is not YAML",
			request: testutils.ImportRequest{Content: "resource_type: ["},
			status:  http.StatusBadRequest,
			outcome: map[string]string{"result": "failed", "reason": "imp_1002"}, dryRun: "false", only: true,
		},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		tc.request.Options = testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"}
		body, err := json.Marshal(tc.request)
		ts.Require().NoError(err)
		response, read := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/import", string(body))
		ts.Require().Equal(tc.status, response.StatusCode, "%s: %s", tc.name, string(read))
		responses[i] = response
	}
	stored, _ := ts.exchange(testutils.GetHTTPClient(), http.MethodGet, "/users/"+dryRunUserID, "")
	ts.Require().Equal(http.StatusNotFound, stored.StatusCode, "a dry run must not create the user")

	for i, written := range ts.linesPerCall(responses...) {
		tc := cases[i]
		ts.Require().NotEmpty(written, tc.name)
		if tc.only {
			ts.Require().Len(written, 1, "%s: the import line is the only line", tc.name)
		}
		line := written[len(written)-1]
		ts.Equal(importEvent, line.Event, "%s: the import line comes after the lines of its resources", tc.name)
		ts.Equal(1, countEvent(written, importEvent), "%s: one import line per request", tc.name)
		ts.Equal("import", line.Action, tc.name)
		ts.Equal(configurationRef, line.Target, tc.name)
		ts.Equal(testutils.AuditRef{}, line.Subject, tc.name)
		ts.Equal(adminRef, line.Actor, tc.name)
		ts.Equal(tc.outcome, line.Outcome, tc.name)
		ts.Equal(map[string]string{"dry_run": tc.dryRun}, ts.details(line), tc.name)
		for _, carried := range []string{ouID, ouHandle, parentID, dryRunUserID, password,
			"AuditConfigurationImport", ts.prefix} {
			ts.NotContains(line.Raw, carried, "%s: nothing the import carried is recorded", tc.name)
		}
	}
}

// The management API key authenticates an import, which is recorded with the deployment's own system
// actor rather than a user.
func (ts *AuditLogTestSuite) TestAnImportWithTheManagementAPIKeyNamesTheSystemActor() {
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/import",
		bytes.NewReader([]byte(`{"content":"","dryRun":true}`)))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", testUserAgent)
	req.Header.Set("API-Key", managementAPIKey)
	response, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	_, err = io.Copy(io.Discard, response.Body)
	ts.Require().NoError(err)
	ts.Require().NoError(response.Body.Close())
	ts.Require().Equal(http.StatusBadRequest, response.StatusCode)

	line := ts.linesOf(response)[0]
	ts.Equal(importEvent, line.Event)
	ts.Equal(testutils.AuditRef{Type: "system", ID: "management-api-key"}, line.Actor)
	ts.Equal(configurationRef, line.Target)
	ts.Equal(map[string]string{"result": "failed", "reason": "imp_1001"}, line.Outcome)
	ts.Equal(map[string]string{"dry_run": "true"}, ts.details(line))
	ts.NotContains(line.Raw, managementAPIKey, "the key is never recorded")
}

// A request to delete a declarative resource file writes one configuration.delete line naming neither the
// resource type nor the key.
func (ts *AuditLogTestSuite) TestADeclarativeFileDeleteIsRecordedOnce() {
	missingKey := ts.prefix + "cfg-missing-file"
	cases := []struct {
		name string
		body string
	}{
		{"a delete without a key", `{"resourceType":"application"}`},
		{"a delete of a file that does not exist", fmt.Sprintf(`{"resourceType":"application","resourceKey":%q}`,
			missingKey)},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		response, read := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/import/delete", tc.body)
		ts.Require().Equal(http.StatusBadRequest, response.StatusCode, "%s: %s", tc.name, string(read))
		responses[i] = response
	}

	for i, line := range ts.linesOf(responses...) {
		name := cases[i].name
		ts.Equal(deleteEvent, line.Event, name)
		ts.Equal("delete", line.Action, name)
		ts.Equal(configurationRef, line.Target, name)
		ts.Equal(testutils.AuditRef{}, line.Subject, name)
		ts.Equal(map[string]string{"result": "failed", "reason": "imp_1001"}, line.Outcome, name)
		ts.Empty(ts.details(line), name)
		ts.NotContains(line.Raw, missingKey, "%s: the key is not recorded", name)
		ts.NotContains(line.Raw, `"application"`, "%s: the resource type is not recorded", name)
	}
}

// Each export request writes one configuration.export line: succeeded when every resource it named was
// exported, failed with item_failed when one was not, and failed with the code when none was. Nothing the
// request named or the export returned is recorded.
func (ts *AuditLogTestSuite) TestAnExportIsRecordedOnceWithItsOutcome() {
	ouID := ts.createOrganizationUnit("cfg-export")
	missingID := unusedID()
	cases := []struct {
		name    string
		ids     []string
		status  int
		outcome map[string]string
	}{
		{"an export of every resource named", []string{ouID}, http.StatusOK,
			map[string]string{"result": "succeeded"}},
		{"an export of a resource that does not exist alongside one that does", []string{ouID, missingID},
			http.StatusOK, map[string]string{"result": "failed", "reason": "item_failed"}},
		{"an export of nothing that exists", []string{missingID}, http.StatusBadRequest,
			map[string]string{"result": "failed", "reason": "exp_1002"}},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		body, err := json.Marshal(map[string][]string{"organizationUnits": tc.ids})
		ts.Require().NoError(err)
		response, read := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/export", string(body))
		ts.Require().Equal(tc.status, response.StatusCode, "%s: %s", tc.name, string(read))
		responses[i] = response
	}

	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	for i, line := range ts.linesOf(responses...) {
		tc := cases[i]
		ts.Equal(exportEvent, line.Event, tc.name)
		ts.Equal("export", line.Action, tc.name)
		ts.Equal(configurationRef, line.Target, tc.name)
		ts.Equal(testutils.AuditRef{}, line.Subject, tc.name)
		ts.Equal(adminRef, line.Actor, tc.name)
		ts.Equal(tc.outcome, line.Outcome, tc.name)
		ts.Empty(ts.details(line), tc.name)
		for _, named := range []string{ouID, missingID, ts.prefix} {
			ts.NotContains(line.Raw, named, "%s: nothing the export named is recorded", tc.name)
		}
	}
}

// resourceLineOfImport returns the line of the one resource an import wrote, after checking the call
// wrote that line followed by its own configuration.import line with outcome.
func (ts *AuditLogTestSuite) resourceLineOfImport(imported *http.Response,
	outcome map[string]string) testutils.AuditLine {
	ts.T().Helper()
	written := ts.linesPerCall(imported)[0]
	ts.Require().Len(written, 2, "the import wrote its resource's line and its own")
	ts.Equal(importEvent, written[1].Event)
	ts.Equal(outcome, written[1].Outcome)
	return written[0]
}

// details returns the details of an audit line, which the line type does not carry.
func (ts *AuditLogTestSuite) details(line testutils.AuditLine) map[string]string {
	ts.T().Helper()
	var parsed struct {
		Details map[string]string `json:"details"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(line.Raw), &parsed))
	return parsed.Details
}

// countEvent counts the lines of event among lines.
func countEvent(lines []testutils.AuditLine, event string) int {
	count := 0
	for _, line := range lines {
		if line.Event == event {
			count++
		}
	}
	return count
}
