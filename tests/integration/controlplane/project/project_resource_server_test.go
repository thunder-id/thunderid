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

// projectResourceServer is the part of a resource server response these tests read.
type projectResourceServer struct {
	ID        string `json:"id"`
	OUID      string `json:"ouId"`
	ProjectID string `json:"projectId"`
}

// projectResourceServerList is the part of a resource server list response these tests read.
type projectResourceServerList struct {
	TotalResults    int                     `json:"totalResults"`
	ResourceServers []projectResourceServer `json:"resourceServers"`
}

// Projects share the system organization unit, yet each lists only its own resource servers. A resource
// server created without a project belongs to none.
func (ts *ProjectAPITestSuite) TestCreateResourceServerInProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("rs-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("rs-other")))

	inShop := ts.createResourceServer(ts.resourceServerBody("shop", `"projectId":%q`, shop.ID))
	ts.Equal(defaultOUID, inShop.OUID)
	ts.Equal(shop.ID, inShop.ProjectID)
	inOther := ts.createResourceServer(ts.resourceServerBody("other", `"projectId":%q`, other.ID))
	unscoped := ts.createResourceServer(ts.resourceServerBody("unscoped", `"ouId":%q`, defaultOUID))
	ts.Empty(unscoped.ProjectID)

	ts.Equal([]string{inShop.ID},
		ts.resourceServerIDs(ts.listResourceServers("?limit=100&projectId="+url.QueryEscape(shop.ID))))
	ts.Equal([]string{inOther.ID},
		ts.resourceServerIDs(ts.listResourceServers("?limit=100&projectId="+url.QueryEscape(other.ID))))

	var fetched projectResourceServer
	ts.Require().Equal(http.StatusOK, ts.getJSON("/resource-servers/"+inShop.ID, &fetched))
	ts.Equal(shop.ID, fetched.ProjectID)
}

// A resource server in a project is in the system organization unit; any other organization unit is refused.
func (ts *ProjectAPITestSuite) TestResourceServerInProjectOrganizationUnit() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("rs-ou-shop")))
	otherRoot := ts.createRootOU("rs-ou-other")
	childOUID := ts.createChildOU("rs-ou-child", defaultOUID)

	placed := ts.createResourceServer(ts.resourceServerBody("ou-placed", `"projectId":%q`, shop.ID))
	ts.Equal(defaultOUID, placed.OUID)
	named := ts.createResourceServer(
		ts.resourceServerBody("ou-named", `"projectId":%q,"ouId":%q`, shop.ID, defaultOUID))
	ts.Equal(defaultOUID, named.OUID)
	ts.Equal(shop.ID, named.ProjectID)

	for name, ouID := range map[string]string{
		"ou-child": childOUID, "ou-outside": otherRoot, "ou-missing": "019a0000-0000-7000-8000-0000000beef0",
	} {
		status, body := ts.do(http.MethodPost, "/resource-servers",
			ts.resourceServerBody(name, `"projectId":%q,"ouId":%q`, shop.ID, ouID))
		ts.Equal(http.StatusBadRequest, status, body)
		ts.Equal("RES-1026", codeOf(ts.T(), body), name)
	}

	project := "?limit=100&projectId=" + url.QueryEscape(shop.ID)
	ts.ElementsMatch([]string{placed.ID, named.ID}, ts.resourceServerIDs(ts.listResourceServers(project)))
	ts.ElementsMatch([]string{placed.ID, named.ID},
		ts.resourceServerIDs(ts.listResourceServers(project+ouFilter(defaultOUID))))
	ts.Empty(ts.listResourceServers(project + ouFilter(childOUID)).ResourceServers)
}

// A resource server that names no organization unit or project goes to the system organization unit and
// belongs to no project.
func (ts *ProjectAPITestSuite) TestCreateResourceServerWithoutOrganizationUnit() {
	created := ts.createResourceServer(ts.resourceServerBody("no-ou", `"description":"organization level"`))

	ts.Equal(defaultOUID, created.OUID)
	ts.Empty(created.ProjectID)
}

// An organization unit filter without a project lists the resource servers the unit owns, and any other
// filter is refused.
func (ts *ProjectAPITestSuite) TestListResourceServersByOrganizationUnit() {
	childOUID := ts.createChildOU("rs-filter-child", defaultOUID)
	inChild := ts.createResourceServer(ts.resourceServerBody("filter-child", `"ouId":%q`, childOUID))
	ts.createResourceServer(ts.resourceServerBody("filter-default", `"ouId":%q`, defaultOUID))

	ts.Equal([]string{inChild.ID},
		ts.resourceServerIDs(ts.listResourceServers("?limit=100"+ouFilter(childOUID))))

	status, body := ts.do(http.MethodGet,
		"/resource-servers?filter="+url.QueryEscape(`name eq "orders"`), "")
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("RES-1027", codeOf(ts.T(), body))
}

// An update keeps a resource server in its organization unit and project unless it names others, refuses an
// organization unit outside the project, and moves it between projects.
func (ts *ProjectAPITestSuite) TestUpdateResourceServerWithProject() {
	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("rs-upd-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("rs-upd-second")))
	otherRoot := ts.createRootOU("rs-upd-other")
	childOUID := ts.createChildOU("rs-upd-child", defaultOUID)
	rs := ts.createResourceServer(ts.resourceServerBody("updated", `"projectId":%q`, first.ID))

	updated := ts.updateResourceServer(rs.ID, ts.resourceServerBody("updated", `"description":"kept"`))
	ts.Equal(defaultOUID, updated.OUID, "an update without an organization unit keeps it")
	ts.Equal(first.ID, updated.ProjectID, "an update without a project keeps it")

	for _, ouID := range []string{childOUID, otherRoot} {
		status, body := ts.do(http.MethodPut, "/resource-servers/"+rs.ID,
			ts.resourceServerBody("updated", `"ouId":%q`, ouID))
		ts.Equal(http.StatusBadRequest, status, body)
		ts.Equal("RES-1026", codeOf(ts.T(), body))
	}

	updated = ts.updateResourceServer(rs.ID, ts.resourceServerBody("updated", `"projectId":%q`, second.ID))
	ts.Equal(second.ID, updated.ProjectID)
	ts.Empty(ts.resourceServerIDs(ts.listResourceServers("?limit=100&projectId=" + url.QueryEscape(first.ID))))
	ts.Equal([]string{rs.ID},
		ts.resourceServerIDs(ts.listResourceServers("?limit=100&projectId="+url.QueryEscape(second.ID))))
}

// A resource server stays editable after its project is deleted: an update that changes neither the project
// nor the organization unit does not check the project again.
func (ts *ProjectAPITestSuite) TestUpdateResourceServerOfDeletedProject() {
	gone := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Gone"}`, ts.handle("rs-gone")))
	rs := ts.createResourceServer(ts.resourceServerBody("gone", `"projectId":%q`, gone.ID))
	status, body := ts.do(http.MethodDelete, "/projects/"+gone.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, body)

	updated := ts.updateResourceServer(rs.ID, ts.resourceServerBody("gone", `"description":"still editable"`))

	ts.Equal(gone.ID, updated.ProjectID)
	ts.Equal(defaultOUID, updated.OUID)
}

// An organization-level resource server moves into a project when an update names one.
func (ts *ProjectAPITestSuite) TestUpdateMovesResourceServerIntoProject() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Target"}`, ts.handle("rs-into")))
	rs := ts.createResourceServer(ts.resourceServerBody("into", `"ouId":%q`, defaultOUID))

	updated := ts.updateResourceServer(rs.ID, ts.resourceServerBody("into", `"projectId":%q`, created.ID))

	ts.Equal(created.ID, updated.ProjectID)
	ts.Equal(defaultOUID, updated.OUID)
}

func (ts *ProjectAPITestSuite) TestResourceServerWithUnknownProject() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"

	status, body := ts.do(http.MethodPost, "/resource-servers",
		ts.resourceServerBody("unknown", `"projectId":%q`, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("RES-1025", codeOf(ts.T(), body))

	rs := ts.createResourceServer(ts.resourceServerBody("unknown-upd", `"ouId":%q`, defaultOUID))
	status, body = ts.do(http.MethodPut, "/resource-servers/"+rs.ID,
		ts.resourceServerBody("unknown-upd", `"projectId":%q`, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("RES-1025", codeOf(ts.T(), body))

	list := ts.listResourceServers("?projectId=" + unknownProjectID)
	ts.Zero(list.TotalResults)
	ts.Empty(list.ResourceServers)
}

// An exported resource server carries its project, so importing the export brings it back in the same
// project.
func (ts *ProjectAPITestSuite) TestExportedResourceServerKeepsItsProject() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Export"}`, ts.handle("rs-export")))
	rs := ts.createResourceServer(ts.resourceServerBody("exported", `"projectId":%q`, created.ID))

	body, err := json.Marshal(map[string]any{"resourceServers": []string{rs.ID}})
	ts.Require().NoError(err)
	status, resp := ts.do(http.MethodPost, "/export", string(body))
	ts.Require().Equal(http.StatusOK, status, resp)
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(resp), &exported))
	ts.Equal(1, strings.Count(exported.Resources, "projectId: "+created.ID), exported.Resources)

	status, resp = ts.do(http.MethodDelete, "/resource-servers/"+rs.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, resp)

	ts.importContent(exported.Resources)
	var imported projectResourceServer
	ts.Require().Equal(http.StatusOK, ts.getJSON("/resource-servers/"+rs.ID, &imported))
	ts.Equal(created.ID, imported.ProjectID)
}

// resourceServerBody builds a resource server request with a unique name and identifier, with the given
// placement fields spliced in.
func (ts *ProjectAPITestSuite) resourceServerBody(name, placement string, args ...interface{}) string {
	handle := ts.handle("rs-" + name)
	return fmt.Sprintf(`{"name":%q,"identifier":%q,%s}`,
		handle, "https://"+handle+".example.com", fmt.Sprintf(placement, args...))
}

// createResourceServer posts a resource server, requires a 201, and records it for cleanup.
func (ts *ProjectAPITestSuite) createResourceServer(body string) projectResourceServer {
	status, resp := ts.do(http.MethodPost, "/resource-servers", body)
	ts.Require().Equal(http.StatusCreated, status, resp)
	var created projectResourceServer
	ts.Require().NoError(json.Unmarshal([]byte(resp), &created))
	ts.resourceServers = append(ts.resourceServers, created.ID)
	return created
}

// updateResourceServer puts a resource server and requires a 200.
func (ts *ProjectAPITestSuite) updateResourceServer(id, body string) projectResourceServer {
	status, resp := ts.do(http.MethodPut, "/resource-servers/"+id, body)
	ts.Require().Equal(http.StatusOK, status, resp)
	var updated projectResourceServer
	ts.Require().NoError(json.Unmarshal([]byte(resp), &updated))
	return updated
}

// listResourceServers lists resource servers with the given query and requires a 200.
func (ts *ProjectAPITestSuite) listResourceServers(query string) projectResourceServerList {
	var list projectResourceServerList
	ts.Require().Equal(http.StatusOK, ts.getJSON("/resource-servers"+query, &list))
	return list
}

// resourceServerIDs returns the IDs of the listed resource servers.
func (ts *ProjectAPITestSuite) resourceServerIDs(list projectResourceServerList) []string {
	ids := []string{}
	for _, rs := range list.ResourceServers {
		ids = append(ids, rs.ID)
	}
	return ids
}

// ouFilter builds the query parameter that narrows a listing to an organization unit.
func ouFilter(ouID string) string {
	return "&filter=" + url.QueryEscape(fmt.Sprintf("ouId eq %q", ouID))
}
