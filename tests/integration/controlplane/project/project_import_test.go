// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

type importOutcome struct {
	Summary struct {
		Failed int `json:"failed"`
	} `json:"summary"`
	Results []struct {
		ResourceType string `json:"resourceType"`
		Operation    string `json:"operation"`
		Status       string `json:"status"`
		Code         string `json:"code"`
		Message      string `json:"message"`
	} `json:"results"`
}

// defaultOUID and defaultProjectID are the default organization unit and the Default project the
// bootstrap declares (backend/cmd/server/bootstrap/01-default-resources.yaml).
const (
	defaultOUID      = "01900000-0000-7000-8000-000000000001"
	defaultProjectID = "01900000-0000-7000-8000-000000000002"
)

// The bootstrap declares the Default project.
func (ts *ProjectAPITestSuite) TestBootstrapCreatesTheDefaultProject() {
	var defaultProject project
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects/"+defaultProjectID, &defaultProject))
	ts.Equal("default", defaultProject.Handle)
	ts.Equal("Default", defaultProject.DisplayName)
}

// A root organization unit imported without a project document creates no project, and an export of
// organization units alone carries no project.
func (ts *ProjectAPITestSuite) TestOUImportCreatesNoProject() {
	rootID := "019a0000-0000-7000-8000-0000000001a0"
	_ = testutils.DeleteOrganizationUnit(rootID)
	ts.importContent(fmt.Sprintf("resource_type: organization_unit\nid: %s\nhandle: %s\nname: Imported\n",
		rootID, ts.handle("imported")))
	ts.ous = append(ts.ous, rootID)
	ts.Nil(ts.projectWithHandle(ts.handle("imported")), "only a declared project is imported")

	body, err := json.Marshal(map[string]any{"organizationUnits": []string{rootID}})
	ts.Require().NoError(err)
	status, resp := ts.do(http.MethodPost, "/export", string(body))
	ts.Require().Equal(http.StatusOK, status, resp)
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(resp), &exported))
	ts.Contains(exported.Resources, "resource_type: organization_unit")
	ts.NotContains(exported.Resources, "resource_type: project")
}

// A declared project keeps the ID another system gave it and creates no organization unit. Importing it
// again updates it in place.
func (ts *ProjectAPITestSuite) TestImportedProjectKeepsItsID() {
	projectID := "019a0000-0000-7000-8000-0000000002a0"
	ts.do(http.MethodDelete, "/projects/"+projectID, "")
	content := fmt.Sprintf("resource_type: project\nid: %s\nhandle: %s\ndisplayName: Declared\n"+
		"description: From the central projects service\n", projectID, ts.handle("declared"))

	ts.importContent(content)
	ts.projects = append(ts.projects, projectID)
	var imported project
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects/"+projectID, &imported))
	ts.Equal(ts.handle("declared"), imported.Handle)
	ts.Equal("Declared", imported.DisplayName)
	ts.Equal("From the central projects service", imported.Description)
	status, _ := ts.do(http.MethodGet, "/organization-units/tree/"+ts.handle("declared"), "")
	ts.Equal(http.StatusNotFound, status, "an imported project creates no organization unit")

	ts.importContent(strings.ReplaceAll(content, "displayName: Declared", "displayName: Renamed"))
	var renamed project
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects/"+projectID, &renamed))
	ts.Equal("Renamed", renamed.DisplayName)
	ts.Equal(imported.CreatedAt, renamed.CreatedAt)
}

// An exported project carries its ID, so importing the export brings the project back with the same ID.
func (ts *ProjectAPITestSuite) TestExportedProjectImportsWithTheSameID() {
	projectID := "019a0000-0000-7000-8000-0000000002c0"
	ts.do(http.MethodDelete, "/projects/"+projectID, "")
	created := ts.create(fmt.Sprintf(`{"id":%q,"handle":%q,"displayName":"Round trip"}`, projectID,
		ts.handle("roundtrip")))

	body, err := json.Marshal(map[string]any{"projects": []string{created.ID}})
	ts.Require().NoError(err)
	status, resp := ts.do(http.MethodPost, "/export", string(body))
	ts.Require().Equal(http.StatusOK, status, resp)
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(resp), &exported))
	ts.Contains(exported.Resources, "resource_type: project")
	ts.Contains(exported.Resources, "id: "+created.ID)
	ts.NotContains(exported.Resources, "rootOuId")

	status, resp = ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, resp)

	ts.importContent(exported.Resources)
	var imported project
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects/"+created.ID, &imported))
	ts.Equal(created.Handle, imported.Handle)
	ts.Equal("Round trip", imported.DisplayName)
}

// A dry run reports whether each declared project would be created or updated, and changes nothing.
func (ts *ProjectAPITestSuite) TestProjectImportDryRun() {
	var defaultProject project
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects/"+defaultProjectID, &defaultProject))
	newID := "019a0000-0000-7000-8000-0000000002e0"
	ts.do(http.MethodDelete, "/projects/"+newID, "")
	content := fmt.Sprintf("resource_type: project\nid: %s\nhandle: default\n"+
		"displayName: Renamed in a dry run\n---\nresource_type: project\nid: %s\nhandle: %s\n"+
		"displayName: Dry run\n", defaultProject.ID, newID, ts.handle("dryrun"))

	outcome := ts.importWith(content, true, true)
	ts.Require().Len(outcome.Results, 2)
	ts.Equal("update", outcome.Results[0].Operation)
	ts.Equal("create", outcome.Results[1].Operation)

	outcome = ts.importWith(content, false, true)
	ts.Require().Len(outcome.Results, 2)
	ts.Equal("create", outcome.Results[0].Operation)

	var unchanged project
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects/"+defaultProject.ID, &unchanged))
	ts.Equal(defaultProject.DisplayName, unchanged.DisplayName)
	status, _ := ts.do(http.MethodGet, "/projects/"+newID, "")
	ts.Equal(http.StatusNotFound, status, "a dry run must not create the project")
}

// A declared project that conflicts with what exists is reported as failed.
func (ts *ProjectAPITestSuite) TestImportedProjectConflicts() {
	existing := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Existing"}`, ts.handle("existing")))
	newID := "019a0000-0000-7000-8000-0000000002f0"
	ts.do(http.MethodDelete, "/projects/"+newID, "")
	doc := func(id, handle, displayName string) string {
		return fmt.Sprintf("resource_type: project\nid: %s\nhandle: %s\ndisplayName: %s\n", id, handle, displayName)
	}

	cases := map[string]struct {
		content string
		upsert  bool
		code    string
	}{
		"invalid ID":              {doc("not-a-uuid", ts.handle("bad"), "Bad"), true, "PRJ-1005"},
		"invalid handle":          {doc(newID, "not/valid", "Bad"), true, "PRJ-1011"},
		"missing display name":    {doc(newID, ts.handle("bad"), `""`), true, "PRJ-1003"},
		"existing without upsert": {doc(existing.ID, existing.Handle, "Again"), false, "PRJ-1006"},
		"handle change":           {doc(existing.ID, ts.handle("other"), "Moved"), true, "PRJ-1007"},
		"handle of another":       {doc(newID, existing.Handle, "Taken"), true, "PRJ-1012"},
		"undecodable fields":      {"resource_type: project\nid: [a, b]\n", true, "IMP-1002"},
	}
	for name, tc := range cases {
		ts.Run(name, func() {
			ts.Equal(tc.code, ts.importFailure(tc.content, tc.upsert))
		})
	}
	status, _ := ts.do(http.MethodGet, "/projects/"+newID, "")
	ts.Equal(http.StatusNotFound, status)
}

// Exporting every project includes the Default project.
func (ts *ProjectAPITestSuite) TestExportAllProjects() {
	status, resp := ts.do(http.MethodPost, "/export", `{"projects":["*"]}`)
	ts.Require().Equal(http.StatusOK, status, resp)
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(resp), &exported))
	ts.Contains(exported.Resources, "id: "+defaultProjectID)
	ts.NotContains(exported.Resources, "rootOuId")
}

// importContent imports YAML resources with upsert, as the bootstrap does, and requires every one to
// succeed.
func (ts *ProjectAPITestSuite) importContent(content string) {
	outcome := ts.importWith(content, true, false)
	ts.Require().Equal(0, outcome.Summary.Failed, outcome.Results)
}

// importFailure imports a single YAML resource and returns the error code it failed with.
func (ts *ProjectAPITestSuite) importFailure(content string, upsert bool) string {
	outcome := ts.importWith(content, upsert, false)
	ts.Require().Len(outcome.Results, 1)
	ts.Require().Equal("failed", outcome.Results[0].Status, outcome.Results[0].Message)
	return outcome.Results[0].Code
}

// importWith imports YAML resources with the given options and returns the outcome.
func (ts *ProjectAPITestSuite) importWith(content string, upsert, dryRun bool) importOutcome {
	body, err := json.Marshal(map[string]any{
		"content": content,
		"dryRun":  dryRun,
		"options": map[string]any{"target": "runtime", "upsert": upsert},
	})
	ts.Require().NoError(err)

	status, resp := ts.do(http.MethodPost, "/import", string(body))
	ts.Require().Equal(http.StatusOK, status, resp)
	var outcome importOutcome
	ts.Require().NoError(json.Unmarshal([]byte(resp), &outcome))
	return outcome
}

// projectWithHandle returns the project with the given handle, or nil.
func (ts *ProjectAPITestSuite) projectWithHandle(handle string) *project {
	var list projectList
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects?limit=100", &list))
	ts.Require().LessOrEqual(list.TotalResults, len(list.Projects), "more projects than one page holds")
	for _, p := range list.Projects {
		if p.Handle == handle {
			return &p
		}
	}
	return nil
}
