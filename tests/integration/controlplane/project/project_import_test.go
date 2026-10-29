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

// The bootstrap declares the Default project with the default organization unit as its root.
func (ts *ProjectAPITestSuite) TestBootstrapCreatesTheDefaultProject() {
	var defaultProject project
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects/"+defaultProjectID, &defaultProject))
	ts.Equal(defaultOUID, defaultProject.RootOUID)
	ts.Equal("default", defaultProject.Handle)
	ts.Equal("Default", defaultProject.DisplayName)
}

// A root organization unit imported without a project document is a plain organization unit, and an
// export of organization units alone carries no project.
func (ts *ProjectAPITestSuite) TestRootOUImportedWithoutAProjectDocumentHasNoProject() {
	rootID := "019a0000-0000-7000-8000-0000000001a0"
	_ = testutils.DeleteOrganizationUnit(rootID)
	ts.importContent(fmt.Sprintf("resource_type: organization_unit\nid: %s\nhandle: %s\nname: Imported\n",
		rootID, ts.handle("imported")))
	ts.childOUs = append(ts.childOUs, rootID)
	ts.Nil(ts.projectWithRootOU(rootID), "only a declared project is imported")

	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Exported"}`, ts.handle("exported")))
	body, err := json.Marshal(map[string]any{"organizationUnits": []string{created.RootOUID}})
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

// A declared project keeps the IDs another system gave it: the project ID and its root organization
// unit's ID. Importing it again updates it in place.
func (ts *ProjectAPITestSuite) TestImportedProjectKeepsItsIDs() {
	projectID := "019a0000-0000-7000-8000-0000000002a0"
	rootID := "019a0000-0000-7000-8000-0000000002a1"
	ts.do(http.MethodDelete, "/projects/"+projectID, "")
	content := fmt.Sprintf("resource_type: project\nid: %s\nrootOuId: %s\nhandle: %s\ndisplayName: Declared\n"+
		"description: From the central projects service\n---\nresource_type: organization_unit\nid: %s\n"+
		"handle: %s\nname: Declared\ndescription: From the central projects service\n",
		projectID, rootID, ts.handle("declared"), rootID, ts.handle("declared"))

	ts.importContent(content)
	ts.projects = append(ts.projects, projectID)
	var imported project
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects/"+projectID, &imported))
	ts.Equal(rootID, imported.RootOUID)
	ts.Equal(ts.handle("declared"), imported.Handle)
	ts.Equal("From the central projects service", imported.Description)
	ts.Len(ts.projectsWithRootOU(rootID), 1, "the root organization unit must not get a second project")

	ts.importContent(strings.ReplaceAll(content, "displayName: Declared", "displayName: Renamed"))
	var renamed project
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects/"+projectID, &renamed))
	ts.Equal("Renamed", renamed.DisplayName)
	ts.Equal(imported.CreatedAt, renamed.CreatedAt)
}

// A declared project links to a root organization unit that exists without a project.
func (ts *ProjectAPITestSuite) TestImportedProjectLinksAnExistingRootOU() {
	rootID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.handle("unlinked"), Name: "Unlinked",
	})
	ts.Require().NoError(err)
	ts.Require().Empty(ts.projectsWithRootOU(rootID))
	projectID := "019a0000-0000-7000-8000-0000000002b0"
	ts.do(http.MethodDelete, "/projects/"+projectID, "")

	ts.importContent(fmt.Sprintf("resource_type: project\nid: %s\nrootOuId: %s\nhandle: %s\ndisplayName: Linked\n",
		projectID, rootID, ts.handle("unlinked")))
	ts.projects = append(ts.projects, projectID)

	var linked project
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects/"+projectID, &linked))
	ts.Equal(rootID, linked.RootOUID)
	ts.Equal("Linked", linked.DisplayName)
}

// An exported project carries its ID and its root organization unit's ID, so importing the export
// brings the project back with the same IDs.
func (ts *ProjectAPITestSuite) TestExportedProjectImportsWithTheSameIDs() {
	projectID := "019a0000-0000-7000-8000-0000000002c0"
	ts.do(http.MethodDelete, "/projects/"+projectID, "")
	created := ts.create(fmt.Sprintf(`{"id":%q,"handle":%q,"displayName":"Round trip"}`, projectID,
		ts.handle("roundtrip")))

	body, err := json.Marshal(map[string]any{
		"projects": []string{created.ID}, "organizationUnits": []string{created.RootOUID},
	})
	ts.Require().NoError(err)
	status, resp := ts.do(http.MethodPost, "/export", string(body))
	ts.Require().Equal(http.StatusOK, status, resp)
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(resp), &exported))
	ts.Contains(exported.Resources, "resource_type: project")
	ts.Contains(exported.Resources, "id: "+created.ID)
	ts.Contains(exported.Resources, "rootOuId: "+created.RootOUID)

	status, resp = ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, resp)

	ts.importContent(exported.Resources)
	var imported project
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects/"+created.ID, &imported))
	ts.Equal(created.RootOUID, imported.RootOUID)
	ts.Equal("Round trip", imported.DisplayName)
	ts.Len(ts.projectsWithRootOU(created.RootOUID), 1)
}

// A declared project whose root organization unit does not exist is created with that organization
// unit, under the declared ID, or under a generated one when none is declared.
func (ts *ProjectAPITestSuite) TestImportedProjectCreatesItsRootOU() {
	projectID := "019a0000-0000-7000-8000-0000000002d0"
	rootID := "019a0000-0000-7000-8000-0000000002d1"
	ts.do(http.MethodDelete, "/projects/"+projectID, "")
	ts.importContent(fmt.Sprintf("resource_type: project\nid: %s\nrootOuId: %s\nhandle: %s\n"+
		"displayName: Created\n", projectID, rootID, ts.handle("created")))
	ts.projects = append(ts.projects, projectID)

	var created project
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects/"+projectID, &created))
	ts.Equal(rootID, created.RootOUID)
	var rootOU organizationUnit
	ts.Require().Equal(http.StatusOK, ts.getJSON("/organization-units/"+rootID, &rootOU))
	ts.Equal(ts.handle("created"), rootOU.Handle)
	ts.Equal("Created", rootOU.Name)

	generatedID := "019a0000-0000-7000-8000-0000000002d2"
	ts.do(http.MethodDelete, "/projects/"+generatedID, "")
	ts.importContent(fmt.Sprintf("resource_type: project\nid: %s\nhandle: %s\ndisplayName: Generated\n",
		generatedID, ts.handle("generated")))
	ts.projects = append(ts.projects, generatedID)

	var generated project
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects/"+generatedID, &generated))
	ts.NotEmpty(generated.RootOUID)
	ts.NotEqual(generatedID, generated.RootOUID)
}

// A dry run reports whether each declared project would be created or updated, and changes nothing.
func (ts *ProjectAPITestSuite) TestProjectImportDryRun() {
	var defaultProject project
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects/"+defaultProjectID, &defaultProject))
	newID := "019a0000-0000-7000-8000-0000000002e0"
	ts.do(http.MethodDelete, "/projects/"+newID, "")
	content := fmt.Sprintf("resource_type: project\nid: %s\nrootOuId: %s\nhandle: default\n"+
		"displayName: Renamed in a dry run\n---\nresource_type: project\nid: %s\nhandle: %s\n"+
		"displayName: Dry run\n", defaultProject.ID, defaultOUID, newID, ts.handle("dryrun"))

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
	childID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.handle("existing-child"), Name: "Child", Parent: &existing.RootOUID,
	})
	ts.Require().NoError(err)
	ts.childOUs = append(ts.childOUs, childID)
	unlinkedID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.handle("conflict-unlinked"), Name: "Unlinked",
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(unlinkedID) })
	newID := "019a0000-0000-7000-8000-0000000002f0"
	ts.do(http.MethodDelete, "/projects/"+newID, "")
	doc := func(id, rootOUID, handle, displayName string) string {
		return fmt.Sprintf("resource_type: project\nid: %s\nrootOuId: %s\nhandle: %s\ndisplayName: %s\n",
			id, rootOUID, handle, displayName)
	}

	cases := map[string]struct {
		content string
		upsert  bool
		code    string
	}{
		"invalid ID":           {doc("not-a-uuid", "", ts.handle("bad"), "Bad"), true, "PRJ-1005"},
		"missing display name": {doc(newID, "", ts.handle("bad"), `""`), true, "PRJ-1003"},
		"existing without upsert": {doc(existing.ID, existing.RootOUID, existing.Handle, "Again"), false,
			"PRJ-1006"},
		"another root":       {doc(existing.ID, unlinkedID, existing.Handle, "Moved"), true, "PRJ-1010"},
		"child as root":      {doc(newID, childID, ts.handle("existing-child"), "Child"), true, "PRJ-1011"},
		"handle mismatch":    {doc(newID, unlinkedID, ts.handle("other"), "Unlinked"), true, "PRJ-1007"},
		"root of another":    {doc(newID, existing.RootOUID, existing.Handle, "Taken"), true, "PRJ-1012"},
		"undecodable fields": {"resource_type: project\nid: [a, b]\n", true, "IMP-1002"},
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
	ts.Contains(exported.Resources, "rootOuId: "+defaultOUID)
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

// projectWithRootOU returns the project whose root organization unit is the given one, or nil.
func (ts *ProjectAPITestSuite) projectWithRootOU(rootOUID string) *project {
	projects := ts.projectsWithRootOU(rootOUID)
	if len(projects) == 0 {
		return nil
	}
	return &projects[0]
}

// projectsWithRootOU returns every project whose root organization unit is the given one.
func (ts *ProjectAPITestSuite) projectsWithRootOU(rootOUID string) []project {
	var list projectList
	ts.Require().Equal(http.StatusOK, ts.getJSON("/projects?limit=100", &list))
	ts.Require().LessOrEqual(list.TotalResults, len(list.Projects), "more projects than one page holds")
	var matches []project
	for _, p := range list.Projects {
		if p.RootOUID == rootOUID {
			matches = append(matches, p)
		}
	}
	return matches
}
