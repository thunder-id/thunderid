// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	managedImportPath = "/import/managed"

	// detailFailedCount and detailFailedItems name the failed items of a managed import.
	detailFailedCount = "failed_count"
	detailFailedItems = "failed_items"

	applicationCreateEvent = "application.create"
)

// A managed import writes only its own configuration.import line, since the control plane already recorded
// each resource it deploys, and that line names the items that failed by type and ID. A plain import of the
// same kind of content writes each resource's line and a configuration.import line that names no item.
func (ts *AuditLogTestSuite) TestAManagedImportRecordsOnlyTheRequestWithItsFailedItems() {
	ouID := ts.createOrganizationUnit("managed-import")
	authFlowID := ts.authFlowID()
	failedOutcome := map[string]string{"result": "failed", "reason": "item_failed"}

	managedOK, managedFailed := unusedID(), unusedID()
	ts.T().Cleanup(func() { ts.undeployApplication(managedOK) })
	managed, results := ts.importApplications(testutils.GetManagementAPIKeyHTTPClient(), managedImportPath,
		ts.applicationDocument(managedOK, "managed-ok", ouID, authFlowID),
		ts.applicationDocument(managedFailed, "managed-failed", ouID, unusedID()))
	ts.Require().Equal(map[string]string{managedOK: "success", managedFailed: "failed"}, results,
		"an unknown authentication flow must fail only its own application")
	stored, body := ts.exchange(testutils.GetHTTPClient(), http.MethodGet, "/applications/"+managedOK, "")
	ts.Require().Equal(http.StatusOK, stored.StatusCode,
		"the managed import must have written the application whose line it suppresses: %s", string(body))

	plainOK, plainFailed := unusedID(), unusedID()
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(plainOK) })
	plain, results := ts.importApplications(testutils.GetHTTPClient(), "/import",
		ts.applicationDocument(plainOK, "plain-ok", ouID, authFlowID),
		ts.applicationDocument(plainFailed, "plain-failed", ouID, unusedID()))
	ts.Require().Equal(map[string]string{plainOK: "success", plainFailed: "failed"}, results,
		"an unknown authentication flow must fail only its own application")

	perCall := ts.linesPerCall(managed, plain)

	managedLines := perCall[0]
	ts.Require().Len(managedLines, 1, "a managed import writes its own line and no line per resource")
	line := managedLines[0]
	ts.Equal(importEvent, line.Event)
	ts.Equal(configurationRef, line.Target)
	ts.Equal(testutils.AuditRef{Type: "system", ID: "management-api-key"}, line.Actor)
	ts.Equal(failedOutcome, line.Outcome)
	ts.Equal(map[string]string{
		"dry_run":         "false",
		detailFailedCount: "1",
		detailFailedItems: "application:" + managedFailed,
	}, ts.details(line), "a managed import names the item that failed, and only that item")
	ts.NotContains(line.Raw, managedOK, "an item that succeeded is not named")

	plainLines := perCall[1]
	ts.Require().NotEmpty(plainLines)
	ts.Equal(1, countEvent(plainLines, importEvent), "a plain import writes one import line")
	importLine := plainLines[len(plainLines)-1]
	ts.Equal(importEvent, importLine.Event, "the import line comes after the lines of its resources")
	ts.Equal(failedOutcome, importLine.Outcome)
	ts.Equal(map[string]string{"dry_run": "false"}, ts.details(importLine),
		"a plain import names no failed item, since each resource's line records its own outcome")
	created := lineFor(plainLines, applicationCreateEvent, testutils.AuditRef{Type: "application", ID: plainOK})
	ts.Require().NotNil(created, "a plain import writes the line of the application it created")
	ts.Equal(map[string]string{"result": "succeeded"}, created.Outcome)
	// A create that made no application names none, so the refused one is found by its type alone.
	refused := lineFor(plainLines, applicationCreateEvent, testutils.AuditRef{Type: "application"})
	ts.Require().NotNil(refused, "a plain import writes the line of the application it failed to create")
	ts.Equal(map[string]string{"result": "failed", "reason": "app_1007"}, refused.Outcome)
	ts.Equal(2, countEvent(plainLines, applicationCreateEvent), "one line per application the import carried")
}

// importApplications imports documents through path with client and returns the response and the status of
// each result by resource ID.
func (ts *AuditLogTestSuite) importApplications(client *http.Client, path string, documents ...string) (
	*http.Response, map[string]string) {
	ts.T().Helper()
	content := ""
	for i, document := range documents {
		if i > 0 {
			content += "---\n"
		}
		content += document
	}
	request, err := json.Marshal(testutils.ImportRequest{
		Content: content,
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	ts.Require().NoError(err)
	imported, body := ts.exchange(client, http.MethodPost, path, string(request))
	ts.Require().Equal(http.StatusOK, imported.StatusCode, string(body))
	var result testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	statuses := make(map[string]string, len(result.Results))
	for _, item := range result.Results {
		statuses[item.ResourceID] = item.Status
	}
	return imported, statuses
}

// applicationDocument is the import document of an application.
func (ts *AuditLogTestSuite) applicationDocument(id, name, ouID, authFlowID string) string {
	return fmt.Sprintf("resource_type: application\nid: %s\nname: %s\ntype: fullstack\nouId: %s\nauthFlowId: %s\n",
		id, ts.prefix+name, ouID, authFlowID)
}

// undeployApplication removes an application a managed import deployed, which only another managed import
// may do.
func (ts *AuditLogTestSuite) undeployApplication(id string) {
	request, err := json.Marshal(testutils.ImportRequest{
		Deletions: []testutils.ResourceDeletion{{ResourceType: "application", ID: id}},
	})
	if err != nil {
		ts.T().Logf("cleanup: failed to encode the deletion of application %s: %v", id, err)
		return
	}
	response, body := ts.exchange(testutils.GetManagementAPIKeyHTTPClient(), http.MethodPost, managedImportPath,
		string(request))
	if response.StatusCode != http.StatusOK {
		ts.T().Logf("cleanup: failed to remove application %s: %d %s", id, response.StatusCode, string(body))
	}
}

// lineFor returns the line of event on target, or nil when there is none.
func lineFor(lines []testutils.AuditLine, event string, target testutils.AuditRef) *testutils.AuditLine {
	for i := range lines {
		if lines[i].Event == event && lines[i].Target == target {
			return &lines[i]
		}
	}
	return nil
}
