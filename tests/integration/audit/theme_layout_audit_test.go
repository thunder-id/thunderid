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

// designBody is the request body of a theme or layout with handle and displayName.
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

// createDesign creates a theme or layout over the management API and returns the response and its ID.
func (ts *AuditLogTestSuite) createDesign(resource designResource, handle, displayName string) (
	*http.Response, string) {
	ts.T().Helper()
	created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, resource.path,
		ts.designBody(resource, handle, displayName))
	ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
	var design struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(body, &design))
	ts.Require().NotEmpty(design.ID)
	ts.T().Cleanup(func() { ts.send(http.MethodDelete, resource.path+"/"+design.ID, "") })
	return created, design.ID
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

// Creating, updating and deleting a theme or a layout through the management API writes one line each,
// with the admin as actor and the theme or layout as target and subject. No handle, name, description or
// content appears in any line.
func (ts *AuditLogTestSuite) TestAThemeAndALayoutLifecycleWritesOneLinePerChange() {
	adminID := ts.adminUserID()
	for _, resource := range designResources {
		ts.Run(resource.kind, func() {
			handle := ts.prefix + resource.kind + "-lifecycle"
			displayName := ts.prefix + resource.kind + " lifecycle name"
			created, id := ts.createDesign(resource, handle, displayName)
			updated, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, resource.path+"/"+id,
				ts.designBody(resource, handle, displayName+" renamed"))
			ts.Require().Equal(http.StatusOK, updated.StatusCode, string(body))
			deleted := ts.send(http.MethodDelete, resource.path+"/"+id, "")
			ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

			ref := testutils.AuditRef{Type: resource.kind, ID: id}
			lines := ts.linesOf(created, updated, deleted)
			for i, action := range []string{"create", "update", "delete"} {
				line := lines[i]
				event := resource.kind + "." + action
				ts.True(strings.HasPrefix(line.Raw, auditLinePrefix), line.Raw)
				ts.Equal(lineKeys, ts.keys(line.Raw), event)
				ts.Equal(event, line.Event)
				ts.Equal(action, line.Action)
				ts.Equal(ref, line.Target, event)
				ts.Equal(ref, line.Subject, event)
				ts.Equal(testutils.AuditRef{Type: "user", ID: adminID}, line.Actor, event)
				ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, event)
				ts.Contains([]string{"127.0.0.1", "::1"}, line.Client["address"])
				ts.requireNothingOfTheDesign(line, handle, displayName)
			}
		})
	}
}

// A create or an update the theme or layout service refuses writes one failed line with the service's
// reason and no subject. A failed create names no theme or layout; a failed update names the ID the
// caller sent, even one that matches nothing.
func (ts *AuditLogTestSuite) TestAFailedThemeOrLayoutChangeIsRecordedAsFailed() {
	for _, resource := range designResources {
		ts.Run(resource.kind, func() {
			handle := ts.prefix + resource.kind + "-failed"
			displayName := ts.prefix + resource.kind + " failed name"
			_, id := ts.createDesign(resource, handle, displayName)
			missing := unusedID()

			cases := []struct {
				name   string
				method string
				path   string
				body   string
				status int
				target string
				reason string
			}{
				{"a create without a display name", http.MethodPost, resource.path,
					ts.designBody(resource, handle+"-unnamed", ""), http.StatusBadRequest, "",
					resource.code + "_1005"},
				{"a create with a handle in use", http.MethodPost, resource.path,
					ts.designBody(resource, handle, displayName), http.StatusBadRequest, "", resource.handleInUse},
				{"an update of a missing ID", http.MethodPut, resource.path + "/" + missing,
					ts.designBody(resource, "", displayName), http.StatusNotFound, missing,
					resource.code + "_1003"},
				{"an update that changes the handle", http.MethodPut, resource.path + "/" + id,
					ts.designBody(resource, handle+"-changed", displayName), http.StatusBadRequest, id,
					resource.handleChanged},
			}
			responses := make([]*http.Response, 0, len(cases))
			for _, tc := range cases {
				response, body := ts.exchange(testutils.GetHTTPClient(), tc.method, tc.path, tc.body)
				ts.Require().Equal(tc.status, response.StatusCode, tc.name+": "+string(body))
				responses = append(responses, response)
			}

			lines := ts.linesOf(responses...)
			for i, tc := range cases {
				line := lines[i]
				ts.Equal(resource.kind+"."+map[string]string{http.MethodPost: "create",
					http.MethodPut: "update"}[tc.method], line.Event, tc.name)
				ts.Equal(testutils.AuditRef{Type: resource.kind, ID: tc.target}, line.Target, tc.name)
				ts.Equal(testutils.AuditRef{}, line.Subject, "a failed attempt has no subject: "+tc.name)
				ts.Equal(map[string]string{"result": "failed", "reason": tc.reason}, line.Outcome, tc.name)
				ts.requireNothingOfTheDesign(line, handle, displayName)
			}
		})
	}
}

// A delete of a theme or layout that does not exist still succeeds, but writes no line, so a caller
// cannot record the deletion of an ID it made up.
func (ts *AuditLogTestSuite) TestADeleteOfAMissingThemeOrLayoutWritesNoLine() {
	for _, resource := range designResources {
		ts.Run(resource.kind, func() {
			deleted := ts.send(http.MethodDelete, resource.path+"/"+unusedID(), "")
			ts.Require().Equal(http.StatusNoContent, deleted.StatusCode)

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
			document := fmt.Sprintf("resource_type: %s\nid: %s\nhandle: %s\ndisplayName: %s\n"+
				"description: %s\n%s:\n  colors:\n    primary: \"%s\"\n",
				resource.kind, id, handle, displayName, auditDesignDescription, resource.contentKey,
				auditDesignColor)
			ts.T().Cleanup(func() { ts.send(http.MethodDelete, resource.path+"/"+id, "") })
			upsert := testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"}

			dryCreate := ts.importDesign(testutils.ImportRequest{Content: document, DryRun: true,
				Options: upsert}, "create")
			created := ts.importDesign(testutils.ImportRequest{Content: document, Options: upsert}, "create")
			dryUpdate := ts.importDesign(testutils.ImportRequest{Content: document, DryRun: true,
				Options: upsert}, "update")
			updated := ts.importDesign(testutils.ImportRequest{Content: document, Options: upsert}, "update")
			deleted := ts.importDesign(testutils.ImportRequest{
				Deletions: []testutils.ResourceDeletion{{ResourceType: resource.kind, ID: id}},
				Options:   testutils.ImportOptions{ContinueOnError: true, Target: "runtime"}}, "delete")

			ref := testutils.AuditRef{Type: resource.kind, ID: id}
			perCall := ts.linesPerCall(dryCreate, dryUpdate)
			ts.Empty(perCall[0], "a dry run that would create writes no line")
			ts.Empty(perCall[1], "a dry run that would update writes no line")
			lines := ts.linesOf(created, updated, deleted)
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

// importDesign sends request to the import API and checks its one result succeeded as operation.
func (ts *AuditLogTestSuite) importDesign(request testutils.ImportRequest, operation string) *http.Response {
	ts.T().Helper()
	encoded, err := json.Marshal(request)
	ts.Require().NoError(err)
	imported, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/import", string(encoded))
	ts.Require().Equal(http.StatusOK, imported.StatusCode, string(body))
	var result testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Len(result.Results, 1, string(body))
	ts.Require().Equal("success", result.Results[0].Status, string(body))
	ts.Require().Equal(operation, result.Results[0].Operation, string(body))
	return imported
}
