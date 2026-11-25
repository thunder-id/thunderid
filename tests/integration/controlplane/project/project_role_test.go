// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// projectRole is the part of a role response these tests read.
type projectRole struct {
	ID        string `json:"id"`
	OUID      string `json:"ouId"`
	ProjectID string `json:"projectId"`
}

// projectRoleList is the part of a role list response these tests read.
type projectRoleList struct {
	TotalResults int           `json:"totalResults"`
	Roles        []projectRole `json:"roles"`
}

// Projects share the system organization unit, yet each lists only its own roles. A role created without a
// project or an organization unit belongs to no project and goes to the system organization unit.
func (ts *ProjectAPITestSuite) TestCreateRoleInProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("role-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("role-other")))

	inShop := ts.createRole(ts.roleBody("shop", `"projectId":%q`, shop.ID))
	ts.Equal(defaultOUID, inShop.OUID)
	ts.Equal(shop.ID, inShop.ProjectID)
	inOther := ts.createRole(ts.roleBody("other", `"projectId":%q`, other.ID))
	unscoped := ts.createRole(fmt.Sprintf(`{"name":%q,"permissions":[]}`, ts.handle("role-unscoped")))
	ts.Equal(defaultOUID, unscoped.OUID)
	ts.Empty(unscoped.ProjectID)

	ts.Equal([]string{inShop.ID}, ts.roleIDs(ts.listRoles("?limit=100&projectId="+url.QueryEscape(shop.ID))))
	ts.Equal([]string{inOther.ID}, ts.roleIDs(ts.listRoles("?limit=100&projectId="+url.QueryEscape(other.ID))))

	var fetched projectRole
	ts.Require().Equal(http.StatusOK, ts.getJSON("/roles/"+inShop.ID, &fetched))
	ts.Equal(shop.ID, fetched.ProjectID)
}

// A role in a project is in the system organization unit; one below it is refused.
func (ts *ProjectAPITestSuite) TestRoleInProjectOrganizationUnit() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("role-ou-shop")))
	childOUID := ts.createChildOU("role-ou-child", defaultOUID)
	inShop := ts.createRole(ts.roleBody("ou-default", `"projectId":%q,"ouId":%q`, shop.ID, defaultOUID))

	status, body := ts.do(http.MethodPost, "/roles", ts.roleBody("ou-child", `"projectId":%q,"ouId":%q`,
		shop.ID, childOUID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("ROL-1041", codeOf(ts.T(), body))

	ts.Equal([]string{inShop.ID}, ts.roleIDs(ts.listRoles(
		"?limit=100&projectId="+url.QueryEscape(shop.ID)+roleOUFilter(defaultOUID))))
	ts.Empty(ts.listRoles("?limit=100&projectId=" + url.QueryEscape(shop.ID) + roleOUFilter(childOUID)).Roles)
}

// An update keeps a role in its organization unit and project unless it names others, and moves it between
// projects.
func (ts *ProjectAPITestSuite) TestUpdateRoleWithProject() {
	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("role-upd-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("role-upd-second")))
	role := ts.createRole(ts.roleBody("updated", `"projectId":%q`, first.ID))

	updated := ts.updateRole(role.ID, fmt.Sprintf(`{"name":%q,"permissions":[]}`, ts.handle("role-updated")))
	ts.Equal(defaultOUID, updated.OUID)
	ts.Equal(first.ID, updated.ProjectID, "an update without a project keeps it")

	updated = ts.updateRole(role.ID, ts.roleBody("updated", `"projectId":%q`, second.ID))
	ts.Equal(second.ID, updated.ProjectID)
	ts.Empty(ts.roleIDs(ts.listRoles("?limit=100&projectId=" + url.QueryEscape(first.ID))))
	ts.Equal([]string{role.ID}, ts.roleIDs(ts.listRoles("?limit=100&projectId="+url.QueryEscape(second.ID))))
}

// An organization-level role moves into a project when an update names one.
func (ts *ProjectAPITestSuite) TestUpdateMovesRoleIntoProject() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Target"}`, ts.handle("role-into")))
	role := ts.createRole(fmt.Sprintf(`{"name":%q,"permissions":[]}`, ts.handle("role-into")))

	updated := ts.updateRole(role.ID, ts.roleBody("into", `"projectId":%q`, created.ID))

	ts.Equal(created.ID, updated.ProjectID)
	ts.Equal(defaultOUID, updated.OUID)
}

// A shared role stays in its project: its sharing policies were issued while it was there.
func (ts *ProjectAPITestSuite) TestSharedRoleCannotChangeProject() {
	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("role-shared-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("role-shared-second")))
	ts.createChildOU("role-shared-child", defaultOUID)
	role := ts.createRole(ts.roleBody("shared", `"projectId":%q`, first.ID))
	status, body := ts.do(http.MethodPost, "/roles/"+role.ID+"/sharing-policies", `{"childUnits":{"mode":"all"}}`)
	ts.Require().Equal(http.StatusCreated, status, body)

	status, body = ts.do(http.MethodPut, "/roles/"+role.ID, ts.roleBody("shared", `"projectId":%q`, second.ID))

	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("ROL-1029", codeOf(ts.T(), body))
}

func (ts *ProjectAPITestSuite) TestRoleWithUnknownProject() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"

	status, body := ts.do(http.MethodPost, "/roles", ts.roleBody("unknown", `"projectId":%q`, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("ROL-1040", codeOf(ts.T(), body))

	role := ts.createRole(fmt.Sprintf(`{"name":%q,"permissions":[]}`, ts.handle("role-unknown-upd")))
	status, body = ts.do(http.MethodPut, "/roles/"+role.ID,
		ts.roleBody("unknown-upd", `"projectId":%q`, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("ROL-1040", codeOf(ts.T(), body))

	list := ts.listRoles("?projectId=" + unknownProjectID)
	ts.Zero(list.TotalResults)
	ts.Empty(list.Roles)
}

// An exported role carries its project, so importing the export brings it back in the same project.
func (ts *ProjectAPITestSuite) TestExportedRoleKeepsItsProject() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Export"}`, ts.handle("role-export")))
	role := ts.createRole(ts.roleBody("exported", `"projectId":%q`, created.ID))

	body, err := json.Marshal(map[string]any{"roles": []string{role.ID}})
	ts.Require().NoError(err)
	status, resp := ts.do(http.MethodPost, "/export", string(body))
	ts.Require().Equal(http.StatusOK, status, resp)
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(resp), &exported))
	ts.Equal(1, strings.Count(exported.Resources, "projectId: "+created.ID), exported.Resources)

	status, resp = ts.do(http.MethodDelete, "/roles/"+role.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, resp)

	ts.importContent(exported.Resources)
	var imported projectRole
	ts.Require().Equal(http.StatusOK, ts.getJSON("/roles/"+role.ID, &imported))
	ts.Equal(created.ID, imported.ProjectID)
}

// A project that still has a role is not deleted until the role is gone.
func (ts *ProjectAPITestSuite) TestDeleteProjectWithRoleIsRefused() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Busy"}`, ts.handle("role-busy")))
	role := ts.createRole(ts.roleBody("busy", `"projectId":%q`, created.ID))

	status, body := ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusConflict, status, body)
	ts.Equal("PRJ-1013", codeOf(ts.T(), body))

	status, body = ts.do(http.MethodDelete, "/roles/"+role.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, body)
	status, body = ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusNoContent, status, body)
}

// roleBody builds a role request with a unique name and no permissions, with the given placement fields
// spliced in.
func (ts *ProjectAPITestSuite) roleBody(name, placement string, args ...interface{}) string {
	return fmt.Sprintf(`{"name":%q,"permissions":[],%s}`, ts.handle("role-"+name), fmt.Sprintf(placement, args...))
}

// createRole posts a role, requires a 201, and records it for cleanup.
func (ts *ProjectAPITestSuite) createRole(body string) projectRole {
	status, resp := ts.do(http.MethodPost, "/roles", body)
	ts.Require().Equal(http.StatusCreated, status, resp)
	var created projectRole
	ts.Require().NoError(json.Unmarshal([]byte(resp), &created))
	ts.roles = append(ts.roles, created.ID)
	return created
}

// updateRole puts a role and requires a 200.
func (ts *ProjectAPITestSuite) updateRole(id, body string) projectRole {
	status, resp := ts.do(http.MethodPut, "/roles/"+id, body)
	ts.Require().Equal(http.StatusOK, status, resp)
	var updated projectRole
	ts.Require().NoError(json.Unmarshal([]byte(resp), &updated))
	return updated
}

// listRoles lists roles with the given query and requires a 200.
func (ts *ProjectAPITestSuite) listRoles(query string) projectRoleList {
	var list projectRoleList
	ts.Require().Equal(http.StatusOK, ts.getJSON("/roles"+query, &list))
	return list
}

// roleIDs returns the IDs of the listed roles.
func (ts *ProjectAPITestSuite) roleIDs(list projectRoleList) []string {
	ids := []string{}
	for _, role := range list.Roles {
		ids = append(ids, role.ID)
	}
	return ids
}

// roleOUFilter builds the query parameter that narrows a role listing to an organization unit.
func roleOUFilter(ouID string) string {
	return "&filter=" + url.QueryEscape(fmt.Sprintf("ouId eq %q", ouID))
}
