// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// projectOU is the part of an organization unit response these tests read.
type projectOU struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
}

// projectOUList is the part of an organization unit list response these tests read.
type projectOUList struct {
	TotalResults      int         `json:"totalResults"`
	OrganizationUnits []projectOU `json:"organizationUnits"`
}

// The Control Plane serves projects, so an organization unit must name an existing one.
func (ts *ProjectAPITestSuite) TestCreateOURequiresProject() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"

	status, body := ts.do(http.MethodPost, "/organization-units", ts.ouBody("ou-no-project", nil, ""))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("OU-1023", codeOf(ts.T(), body))

	status, body = ts.do(http.MethodPost, "/organization-units", ts.ouBody("ou-unknown", nil, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("OU-1022", codeOf(ts.T(), body))
}

// Each project lists only its own organization units: its roots and the units it placed under the system
// organization unit, with their children listed under them.
func (ts *ProjectAPITestSuite) TestCreateOUInProject() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"

	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("ou-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("ou-other")))
	systemOUID := defaultOUID

	root := ts.createOU(ts.ouBody("ou-shop-root", nil, shop.ID))
	ts.Equal(shop.ID, root.ProjectID)
	underSystem := ts.createOU(ts.ouBody("ou-shop-system", &systemOUID, shop.ID))
	child := ts.createOU(ts.ouBody("ou-shop-child", &root.ID, shop.ID))
	inOther := ts.createOU(ts.ouBody("ou-other-root", nil, other.ID))

	ts.ElementsMatch([]string{root.ID, underSystem.ID}, ts.ouIDs(ts.listOUs("?limit=100&projectId="+
		url.QueryEscape(shop.ID))))
	ts.Equal([]string{inOther.ID}, ts.ouIDs(ts.listOUs("?limit=100&projectId="+url.QueryEscape(other.ID))))
	ts.Empty(ts.listOUs("?projectId=" + unknownProjectID).OrganizationUnits)

	var fetched projectOU
	ts.Require().Equal(http.StatusOK, ts.getJSON("/organization-units/"+child.ID, &fetched))
	ts.Equal(shop.ID, fetched.ProjectID)
}

// An organization unit is in its parent's project, unless its parent is the system organization unit.
func (ts *ProjectAPITestSuite) TestOUParentInAnotherProjectIsRefused() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("ou-parent-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("ou-parent-other")))
	root := ts.createOU(ts.ouBody("ou-parent-root", nil, shop.ID))

	status, body := ts.do(http.MethodPost, "/organization-units", ts.ouBody("ou-parent-child", &root.ID, other.ID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("OU-1024", codeOf(ts.T(), body))
}

// An update keeps an organization unit in its project unless it names another one, which moves the unit and its
// descendants there.
func (ts *ProjectAPITestSuite) TestUpdateOUWithProject() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"

	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("ou-upd-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("ou-upd-second")))
	root := ts.createOU(ts.ouBody("ou-upd-root", nil, first.ID))

	updated := ts.updateOU(root.ID, ts.ouBody("ou-upd-root", nil, ""))
	ts.Equal(first.ID, updated.ProjectID, "an update without a project keeps it")

	updated = ts.updateOU(root.ID, ts.ouBody("ou-upd-root", nil, second.ID))
	ts.Equal(second.ID, updated.ProjectID)
	ts.Empty(ts.listOUs("?limit=100&projectId=" + url.QueryEscape(first.ID)).OrganizationUnits)

	child := ts.createOU(ts.ouBody("ou-upd-child", &root.ID, second.ID))
	updated = ts.updateOU(root.ID, ts.ouBody("ou-upd-root", nil, first.ID))
	ts.Equal(first.ID, updated.ProjectID)
	var movedChild projectOU
	ts.Require().Equal(http.StatusOK, ts.getJSON("/organization-units/"+child.ID, &movedChild))
	ts.Equal(first.ID, movedChild.ProjectID, "the child moves with its parent")

	status, body := ts.do(http.MethodPut, "/organization-units/"+root.ID,
		ts.ouBody("ou-upd-root", nil, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("OU-1022", codeOf(ts.T(), body))
}

// The system organization unit is shared by every project, so it belongs to none.
func (ts *ProjectAPITestSuite) TestSystemOUIsInNoProject() {
	var system projectOU
	ts.Require().Equal(http.StatusOK, ts.getJSON("/organization-units/"+defaultOUID, &system))
	ts.Empty(system.ProjectID)

	status, body := ts.do(http.MethodPut, "/organization-units/"+defaultOUID,
		fmt.Sprintf(`{"handle":"default","name":"Default","projectId":%q}`, defaultProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("OU-1025", codeOf(ts.T(), body))
}

// A project that still has an organization unit is not deleted until the unit is gone.
func (ts *ProjectAPITestSuite) TestDeleteProjectWithOUIsRefused() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Busy"}`, ts.handle("ou-busy")))
	unit := ts.createOU(ts.ouBody("ou-busy-root", nil, created.ID))

	status, body := ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusConflict, status, body)
	ts.Equal("PRJ-1013", codeOf(ts.T(), body))

	status, body = ts.do(http.MethodDelete, "/organization-units/"+unit.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, body)
	status, body = ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusNoContent, status, body)
}

// An exported organization unit keeps its project, so a deploy of the project carries it.
func (ts *ProjectAPITestSuite) TestExportedOUKeepsItsProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("ou-export-shop")))
	unit := ts.createOU(ts.ouBody("ou-export-root", nil, shop.ID))

	status, resp := ts.do(http.MethodPost, "/export", fmt.Sprintf(`{"organizationUnits":[%q]}`, unit.ID))
	ts.Require().Equal(http.StatusOK, status, resp)
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(resp), &exported))
	ts.Contains(exported.Resources, "projectId: "+shop.ID)
}

// ouBody builds an organization unit request with the given parent and project, either of which may be left out.
func (ts *ProjectAPITestSuite) ouBody(name string, parent *string, projectID string) string {
	body := map[string]any{"handle": ts.handle(name), "name": name}
	if parent != nil {
		body["parent"] = *parent
	}
	if projectID != "" {
		body["projectId"] = projectID
	}
	encoded, err := json.Marshal(body)
	ts.Require().NoError(err)
	return string(encoded)
}

// createOU posts an organization unit, requires a 201, and records it for cleanup.
func (ts *ProjectAPITestSuite) createOU(body string) projectOU {
	status, resp := ts.do(http.MethodPost, "/organization-units", body)
	ts.Require().Equal(http.StatusCreated, status, resp)
	var created projectOU
	ts.Require().NoError(json.Unmarshal([]byte(resp), &created))
	ts.ous = append(ts.ous, created.ID)
	return created
}

// updateOU puts an organization unit and requires a 200.
func (ts *ProjectAPITestSuite) updateOU(id, body string) projectOU {
	status, resp := ts.do(http.MethodPut, "/organization-units/"+id, body)
	ts.Require().Equal(http.StatusOK, status, resp)
	var updated projectOU
	ts.Require().NoError(json.Unmarshal([]byte(resp), &updated))
	return updated
}

// listOUs lists organization units with the given query and requires a 200.
func (ts *ProjectAPITestSuite) listOUs(query string) projectOUList {
	var list projectOUList
	ts.Require().Equal(http.StatusOK, ts.getJSON("/organization-units"+query, &list))
	return list
}

// ouIDs returns the IDs of the listed organization units.
func (ts *ProjectAPITestSuite) ouIDs(list projectOUList) []string {
	ids := []string{}
	for _, unit := range list.OrganizationUnits {
		ids = append(ids, unit.ID)
	}
	return ids
}
