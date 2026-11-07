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
	auditDesignDescription = "Audited design description"
	auditDesignColor       = "#a1b2c3"
)

// designResource is a theme or a layout, the two design resources the management API serves.
type designResource struct {
	// kind is the target type, the event prefix and the import resource type.
	kind string
	// path is the management API collection.
	path string
	// contentKey is the field that carries the theme or the layout itself.
	contentKey string
	// code is the lower case prefix of the resource's error codes.
	code string
	// handleInUse and handleChanged are the reasons of a create with a handle in use and of an update
	// that changes the handle.
	handleInUse, handleChanged string
}

var designResources = []designResource{
	{kind: "theme", path: "/design/themes", contentKey: "theme", code: "thm",
		handleInUse: "thm_1015", handleChanged: "thm_1017"},
	{kind: "layout", path: "/design/layouts", contentKey: "layout", code: "lay",
		handleInUse: "lay_1016", handleChanged: "lay_1018"},
}

// designContent is the theme or layout every test sends, so a test can check none of it is recorded.
func designContent() map[string]interface{} {
	return map[string]interface{}{"colors": map[string]interface{}{"primary": auditDesignColor}}
}

// designBody is the management API request body of a theme or layout with handle and displayName.
func (ts *AuditLogTestSuite) designBody(resource designResource, handle, displayName string) string {
	ts.T().Helper()
	body := map[string]interface{}{
		"displayName":       displayName,
		"description":       auditDesignDescription,
		resource.contentKey: designContent(),
	}
	if handle != "" {
		body["handle"] = handle
	}
	encoded, err := json.Marshal(body)
	ts.Require().NoError(err)
	return string(encoded)
}

// designDocument is the import document of a theme or layout with id, handle and displayName.
func designDocument(resource designResource, id, handle, displayName string) string {
	return fmt.Sprintf("resource_type: %s\nid: %s\nhandle: %s\ndisplayName: \"%s\"\n"+
		"description: %s\n%s:\n  colors:\n    primary: \"%s\"\n",
		resource.kind, id, handle, displayName, auditDesignDescription, resource.contentKey, auditDesignColor)
}

// upsertDesign is an upsert import of the theme or layout document.
func upsertDesign(document string) testutils.ImportRequest {
	return testutils.ImportRequest{Content: document,
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"}}
}

// deleteDesign is an import that removes the theme or layout with id.
func deleteDesign(resource designResource, id string) testutils.ImportRequest {
	return testutils.ImportRequest{
		Deletions: []testutils.ResourceDeletion{{ResourceType: resource.kind, ID: id}},
		Options:   testutils.ImportOptions{ContinueOnError: true, Target: "runtime"}}
}

// createDesign creates a theme or layout through an import with the management API key, the only way a
// data plane takes one, and returns the response and its ID.
func (ts *AuditLogTestSuite) createDesign(resource designResource, handle, displayName string) (
	*http.Response, string) {
	ts.T().Helper()
	id := unusedID()
	created := ts.importDesign(upsertDesign(designDocument(resource, id, handle, displayName)), "create")
	ts.T().Cleanup(func() { _ = testutils.DeleteImportedResource(resource.kind, id) })
	return created, id
}

// requireNothingOfTheDesign checks that no handle, name, description or content of a theme or layout is
// in line.
func (ts *AuditLogTestSuite) requireNothingOfTheDesign(line testutils.AuditLine, handle, displayName string) {
	ts.T().Helper()
	ts.NotContains(line.Raw, handle, "the handle is not recorded")
	ts.NotContains(line.Raw, displayName, "the display name is not recorded")
	ts.NotContains(line.Raw, auditDesignDescription, "the description is not recorded")
	ts.NotContains(line.Raw, auditDesignColor, "no content is recorded")
}

// Creating, updating and deleting a theme or a layout writes one line each, with the theme or layout as
// target and subject. A data plane takes them only from an import with the management API key, so the key
// is the actor, and a token's write over the management API is denied with the admin as actor. No handle,
// name, description or content appears in any line.
func (ts *AuditLogTestSuite) TestAThemeAndALayoutLifecycleWritesOneLinePerChange() {
	adminID := ts.adminUserID()
	admin := testutils.AuditRef{Type: "user", ID: adminID}
	managementAPIKey := testutils.AuditRef{Type: "system", ID: "management-api-key"}
	succeeded := map[string]string{"result": "succeeded"}
	denied := map[string]string{"result": "denied", "reason": "forbidden"}
	for _, resource := range designResources {
		ts.Run(resource.kind, func() {
			handle := ts.prefix + resource.kind + "-lifecycle"
			displayName := ts.prefix + resource.kind + " lifecycle name"
			created, id := ts.createDesign(resource, handle, displayName)
			updated := ts.importDesign(upsertDesign(designDocument(resource, id, handle, displayName+" renamed")),
				"update")
			tokenCreate := ts.send(http.MethodPost, resource.path,
				ts.designBody(resource, handle+"-token", displayName))
			ts.Require().Equal(http.StatusForbidden, tokenCreate.StatusCode)
			tokenUpdate := ts.send(http.MethodPut, resource.path+"/"+id, ts.designBody(resource, handle, displayName))
			ts.Require().Equal(http.StatusForbidden, tokenUpdate.StatusCode)
			tokenDelete := ts.send(http.MethodDelete, resource.path+"/"+id, "")
			ts.Require().Equal(http.StatusForbidden, tokenDelete.StatusCode)
			deleted := ts.importDesign(deleteDesign(resource, id), "delete")

			ref := testutils.AuditRef{Type: resource.kind, ID: id}
			lines := ts.linesOf(created, updated, tokenCreate, tokenUpdate, tokenDelete, deleted)
			for i, tc := range []struct {
				action  string
				target  testutils.AuditRef
				subject testutils.AuditRef
				actor   testutils.AuditRef
				outcome map[string]string
			}{
				{"create", ref, ref, managementAPIKey, succeeded},
				{"update", ref, ref, managementAPIKey, succeeded},
				// A denied create names no theme or layout.
				{"create", testutils.AuditRef{Type: resource.kind}, testutils.AuditRef{}, admin, denied},
				{"update", ref, ref, admin, denied},
				{"delete", ref, ref, admin, denied},
				{"delete", ref, ref, managementAPIKey, succeeded},
			} {
				line := lines[i]
				event := resource.kind + "." + tc.action
				ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
				ts.Equal(lineKeys, ts.keys(line.Raw), event)
				ts.Equal(event, line.Event)
				ts.Equal(tc.action, line.Action)
				ts.Equal(tc.target, line.Target, event)
				ts.Equal(tc.subject, line.Subject, event)
				ts.Equal(tc.actor, line.Actor, event)
				ts.Equal(tc.outcome, line.Outcome, event)
				ts.Nil(line.Client, "%s: a loopback peer is not recorded", event)
				ts.requireNothingOfTheDesign(line, handle, displayName)
			}
		})
	}
}

// A create or an update the theme or layout service refuses writes one failed line with the service's
// reason and no subject. A failed create names no theme or layout; a failed update names the ID the
// import sent.
func (ts *AuditLogTestSuite) TestAFailedThemeOrLayoutChangeIsRecordedAsFailed() {
	for _, resource := range designResources {
		ts.Run(resource.kind, func() {
			handle := ts.prefix + resource.kind + "-failed"
			displayName := ts.prefix + resource.kind + " failed name"
			_, id := ts.createDesign(resource, handle, displayName)

			cases := []struct {
				name      string
				operation string
				document  string
				target    string
				reason    string
			}{
				{"a create without a display name", "create",
					designDocument(resource, unusedID(), handle+"-unnamed", ""), "", resource.code + "_1005"},
				{"a create with a handle in use", "create",
					designDocument(resource, unusedID(), handle, displayName), "", resource.handleInUse},
				{"an update that changes the handle", "update",
					designDocument(resource, id, handle+"-changed", displayName), id, resource.handleChanged},
			}
			responses := make([]*http.Response, 0, len(cases))
			for _, tc := range cases {
				response, result := ts.sendImport(upsertDesign(tc.document))
				ts.Require().Equal("failed", result.Status, tc.name)
				ts.Require().Equal(tc.operation, result.Operation, tc.name)
				responses = append(responses, response)
			}

			lines := ts.linesOf(responses...)
			for i, tc := range cases {
				line := lines[i]
				ts.Equal(resource.kind+"."+tc.operation, line.Event, tc.name)
				ts.Equal(testutils.AuditRef{Type: resource.kind, ID: tc.target}, line.Target, tc.name)
				ts.Equal(testutils.AuditRef{}, line.Subject, "a failed attempt has no subject: "+tc.name)
				ts.Equal(map[string]string{"result": "failed", "reason": tc.reason}, line.Outcome, tc.name)
				ts.requireNothingOfTheDesign(line, handle, displayName)
			}
		})
	}
}

// A deletion of a theme or layout that does not exist still succeeds, but writes no line, so a caller
// cannot record the deletion of an ID it made up.
func (ts *AuditLogTestSuite) TestADeleteOfAMissingThemeOrLayoutWritesNoLine() {
	for _, resource := range designResources {
		ts.Run(resource.kind, func() {
			deleted := ts.importDesign(deleteDesign(resource, unusedID()), "delete")

			ts.Empty(ts.linesPerCall(deleted)[0], "a delete that removed nothing writes no line")
		})
	}
}

// An upsert import writes the theme or layout service's event for each document and nothing else: the
// lookup that decides between update and create writes no line, so a new theme or layout is one create
// and an existing one is one update. A deletion the import names writes the service's delete. A dry run
// changes nothing and writes no line.
func (ts *AuditLogTestSuite) TestAnImportWritesTheEventOfEachThemeOrLayoutItChanges() {
	for _, resource := range designResources {
		ts.Run(resource.kind, func() {
			id := unusedID()
			handle := ts.prefix + resource.kind + "-import"
			displayName := ts.prefix + resource.kind + " import name"
			document := designDocument(resource, id, handle, displayName)
			ts.T().Cleanup(func() { _ = testutils.DeleteImportedResource(resource.kind, id) })
			dryRun := upsertDesign(document)
			dryRun.DryRun = true

			dryCreate := ts.importDesign(dryRun, "create")
			created := ts.importDesign(upsertDesign(document), "create")
			dryUpdate := ts.importDesign(dryRun, "update")
			updated := ts.importDesign(upsertDesign(document), "update")
			deleted := ts.importDesign(deleteDesign(resource, id), "delete")

			ref := testutils.AuditRef{Type: resource.kind, ID: id}
			perCall := ts.linesPerCall(dryCreate, dryUpdate)
			for i, dryRun := range []string{"create", "update"} {
				ts.Require().Len(perCall[i], 1, "a dry run that would %s writes only the import's own line", dryRun)
				ts.Equal(importEvent, perCall[i][0].Event, dryRun)
			}
			succeeded := map[string]string{"result": "succeeded"}
			lines := []testutils.AuditLine{
				ts.resourceLineOfImport(created, succeeded),
				ts.resourceLineOfImport(updated, succeeded),
				ts.resourceLineOfImport(deleted, succeeded),
			}
			for i, action := range []string{"create", "update", "delete"} {
				line := lines[i]
				ts.Equal(resource.kind+"."+action, line.Event)
				ts.Equal(ref, line.Target, action)
				ts.Equal(ref, line.Subject, action)
				ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, action)
				ts.requireNothingOfTheDesign(line, handle, displayName)
			}
		})
	}
}

// importDesign sends request to the import API with the management API key and checks its one result
// succeeded as operation.
func (ts *AuditLogTestSuite) importDesign(request testutils.ImportRequest, operation string) *http.Response {
	ts.T().Helper()
	imported, result := ts.sendImport(request)
	ts.Require().Equal("success", result.Status, result.Message)
	ts.Require().Equal(operation, result.Operation, result.Message)
	return imported
}

// sendImport sends request to the import API with the management API key and returns the response and
// its one result.
func (ts *AuditLogTestSuite) sendImport(request testutils.ImportRequest) (*http.Response, testutils.ImportItem) {
	ts.T().Helper()
	encoded, err := json.Marshal(request)
	ts.Require().NoError(err)
	imported, body := ts.exchange(testutils.GetManagementAPIKeyHTTPClient(), http.MethodPost, "/import",
		string(encoded))
	ts.Require().Equal(http.StatusOK, imported.StatusCode, string(body))
	var result testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Len(result.Results, 1, string(body))
	return imported, result.Results[0]
}
