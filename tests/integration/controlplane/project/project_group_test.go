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

// projectGroup is the part of a group response these tests read.
type projectGroup struct {
	ID        string `json:"id"`
	OUID      string `json:"ouId"`
	ProjectID string `json:"projectId"`
}

// projectGroupList is the part of a group list response these tests read.
type projectGroupList struct {
	TotalResults int            `json:"totalResults"`
	Groups       []projectGroup `json:"groups"`
}

// Projects share the system organization unit, yet each lists only its own groups. A group created without
// a project belongs to the organization, and one in a project that names no organization unit goes to the
// system organization unit.
func (ts *ProjectAPITestSuite) TestCreateGroupInProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("grp-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("grp-other")))

	inShop := ts.createGroup(ts.groupBody("shop", `"projectId":%q`, shop.ID))
	ts.Equal(defaultOUID, inShop.OUID)
	ts.Equal(shop.ID, inShop.ProjectID)
	inOther := ts.createGroup(ts.groupBody("other", `"projectId":%q,"ouId":%q`, other.ID, defaultOUID))
	inOrganization := ts.createGroup(ts.groupBody("org", `"ouId":%q`, defaultOUID))
	ts.Empty(inOrganization.ProjectID)

	ts.Equal([]string{inShop.ID}, ts.groupIDs(ts.listGroups("?limit=100&projectId="+url.QueryEscape(shop.ID))))
	ts.Equal([]string{inOther.ID}, ts.groupIDs(ts.listGroups("?limit=100&projectId="+url.QueryEscape(other.ID))))

	var fetched projectGroup
	ts.Require().Equal(http.StatusOK, ts.getJSON("/groups/"+inShop.ID, &fetched))
	ts.Equal(shop.ID, fetched.ProjectID)
}

// A group in a project is in the system organization unit; one below it is refused.
func (ts *ProjectAPITestSuite) TestGroupInProjectOrganizationUnit() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("grp-ou-shop")))
	childOUID := ts.createChildOU("grp-ou-child", defaultOUID)

	status, body := ts.do(http.MethodPost, "/groups", ts.groupBody("ou-child", `"projectId":%q,"ouId":%q`,
		shop.ID, childOUID))

	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("GRP-1022", codeOf(ts.T(), body))
}

// An update keeps a group in its project unless it names another, and moves it between projects.
func (ts *ProjectAPITestSuite) TestUpdateGroupWithProject() {
	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("grp-upd-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("grp-upd-second")))
	group := ts.createGroup(ts.groupBody("updated", `"projectId":%q`, first.ID))

	updated := ts.updateGroup(group.ID, ts.groupBody("updated", `"ouId":%q`, defaultOUID))
	ts.Equal(first.ID, updated.ProjectID, "an update without a project keeps it")

	updated = ts.updateGroup(group.ID, ts.groupBody("updated", `"ouId":%q,"projectId":%q`, defaultOUID, second.ID))
	ts.Equal(second.ID, updated.ProjectID)
	ts.Empty(ts.groupIDs(ts.listGroups("?limit=100&projectId=" + url.QueryEscape(first.ID))))
	ts.Equal([]string{group.ID}, ts.groupIDs(ts.listGroups("?limit=100&projectId="+url.QueryEscape(second.ID))))
}

func (ts *ProjectAPITestSuite) TestGroupWithUnknownProject() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"

	status, body := ts.do(http.MethodPost, "/groups", ts.groupBody("unknown", `"projectId":%q`, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("GRP-1021", codeOf(ts.T(), body))

	group := ts.createGroup(ts.groupBody("unknown-upd", `"ouId":%q`, defaultOUID))
	status, body = ts.do(http.MethodPut, "/groups/"+group.ID, ts.groupBody("unknown-upd",
		`"ouId":%q,"projectId":%q`, defaultOUID, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("GRP-1021", codeOf(ts.T(), body))

	list := ts.listGroups("?projectId=" + unknownProjectID)
	ts.Zero(list.TotalResults)
	ts.Empty(list.Groups)
}

// An exported group carries its project, so importing the export brings it back in the same project.
func (ts *ProjectAPITestSuite) TestExportedGroupKeepsItsProject() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Export"}`, ts.handle("grp-export")))
	group := ts.createGroup(ts.groupBody("exported", `"projectId":%q`, created.ID))

	body, err := json.Marshal(map[string]any{"groups": []string{group.ID}})
	ts.Require().NoError(err)
	status, resp := ts.do(http.MethodPost, "/export", string(body))
	ts.Require().Equal(http.StatusOK, status, resp)
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(resp), &exported))
	ts.Equal(1, strings.Count(exported.Resources, "projectId: "+created.ID), exported.Resources)

	status, resp = ts.do(http.MethodDelete, "/groups/"+group.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, resp)

	ts.importContent(exported.Resources)
	var imported projectGroup
	ts.Require().Equal(http.StatusOK, ts.getJSON("/groups/"+group.ID, &imported))
	ts.Equal(created.ID, imported.ProjectID)
}

// A project that still has a group is not deleted until the group is gone.
func (ts *ProjectAPITestSuite) TestDeleteProjectWithGroupIsRefused() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Busy"}`, ts.handle("grp-busy")))
	group := ts.createGroup(ts.groupBody("busy", `"projectId":%q`, created.ID))

	status, body := ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusConflict, status, body)
	ts.Equal("PRJ-1013", codeOf(ts.T(), body))

	status, body = ts.do(http.MethodDelete, "/groups/"+group.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, body)
	status, body = ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusNoContent, status, body)
}

// A group document that names a project and no organization unit imports into the system organization unit,
// and importing it again keeps the group there.
func (ts *ProjectAPITestSuite) TestReimportedProjectGroupWithoutOUKeepsItsOU() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Import"}`, ts.handle("grp-reimport")))
	id := "019a0000-0000-7000-8000-" + ts.suffix[len(ts.suffix)-12:]
	ts.groups = append(ts.groups, id)
	document := fmt.Sprintf("resource_type: group\nid: %s\nname: %s\nprojectId: %s\n",
		id, ts.handle("g-reimport"), created.ID)

	ts.importContent(document)
	ts.importContent(document)

	var imported projectGroup
	ts.Require().Equal(http.StatusOK, ts.getJSON("/groups/"+id, &imported))
	ts.Equal(defaultOUID, imported.OUID)
	ts.Equal(created.ID, imported.ProjectID)
}

// A project's group list narrows to an organization unit, resolves organization unit handles, and refuses
// any filter other than the organization unit. An empty project lists nothing.
func (ts *ProjectAPITestSuite) TestListGroupsInProjectQueries() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("grp-list-shop")))
	group := ts.createGroup(ts.groupBody("listed", `"projectId":%q`, shop.ID))
	inProject := "?limit=100&projectId=" + url.QueryEscape(shop.ID)

	ts.Equal([]string{group.ID}, ts.groupIDs(ts.listGroups(inProject+groupOUFilter(defaultOUID))))
	ts.Empty(ts.listGroups(inProject + groupOUFilter(ts.createChildOU("grp-list-child", defaultOUID))).Groups)

	var displayed struct {
		Groups []struct {
			ID       string `json:"id"`
			OUHandle string `json:"ouHandle"`
		} `json:"groups"`
	}
	ts.Require().Equal(http.StatusOK, ts.getJSON("/groups"+inProject+"&include=display", &displayed))
	ts.Require().Len(displayed.Groups, 1)
	ts.NotEmpty(displayed.Groups[0].OUHandle)

	status, body := ts.do(http.MethodGet, "/groups"+inProject+"&filter="+url.QueryEscape(`name eq "Shoppers"`), "")
	ts.Equal(http.StatusBadRequest, status, body)

	list := ts.listGroups("?projectId=")
	ts.Zero(list.TotalResults)
	ts.Empty(list.Groups)
}

// The group tools create a group in a project and list the project's groups.
func (ts *ProjectAPITestSuite) TestGroupMCPToolsWithProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("grp-mcp-shop")))

	created, raw := ts.callMCPTool("create_group", map[string]any{
		"name": ts.handle("g-mcp"), "projectId": shop.ID,
	})
	ts.Require().Falsef(created.Result.IsError, "create failed: %s", raw)
	id, _ := created.Result.StructuredContent["id"].(string)
	ts.Require().NotEmpty(id)
	ts.groups = append(ts.groups, id)
	ts.Equal(shop.ID, created.Result.StructuredContent["projectId"])
	ts.Equal(defaultOUID, created.Result.StructuredContent["ouId"])

	listed, raw := ts.callMCPTool("list_groups", map[string]any{"projectId": shop.ID})
	ts.Require().Falsef(listed.Result.IsError, "list failed: %s", raw)
	ts.EqualValues(1, listed.Result.StructuredContent["totalResults"])
	ts.Contains(raw, id)

	refused, raw := ts.callMCPTool("list_groups", map[string]any{"projectId": shop.ID, "name": "Shoppers"})
	ts.Truef(refused.Result.IsError, "a name next to a project must be refused: %s", raw)
}

// groupOUFilter builds the query parameter that narrows a group listing to an organization unit.
func groupOUFilter(ouID string) string {
	return "&filter=" + url.QueryEscape(fmt.Sprintf("ouId eq %q", ouID))
}

// groupBody builds a group request with a unique name, with the given placement fields spliced in.
func (ts *ProjectAPITestSuite) groupBody(name, placement string, args ...interface{}) string {
	return fmt.Sprintf(`{"name":%q,%s}`, ts.handle("g-"+name), fmt.Sprintf(placement, args...))
}

// createGroup posts a group, requires a 201, and records it for cleanup.
func (ts *ProjectAPITestSuite) createGroup(body string) projectGroup {
	status, resp := ts.do(http.MethodPost, "/groups", body)
	ts.Require().Equal(http.StatusCreated, status, resp)
	var created projectGroup
	ts.Require().NoError(json.Unmarshal([]byte(resp), &created))
	ts.groups = append(ts.groups, created.ID)
	return created
}

// updateGroup puts a group and requires a 200.
func (ts *ProjectAPITestSuite) updateGroup(id, body string) projectGroup {
	status, resp := ts.do(http.MethodPut, "/groups/"+id, body)
	ts.Require().Equal(http.StatusOK, status, resp)
	var updated projectGroup
	ts.Require().NoError(json.Unmarshal([]byte(resp), &updated))
	return updated
}

// listGroups lists groups with the given query and requires a 200.
func (ts *ProjectAPITestSuite) listGroups(query string) projectGroupList {
	var list projectGroupList
	ts.Require().Equal(http.StatusOK, ts.getJSON("/groups"+query, &list))
	return list
}

// groupIDs returns the IDs of the listed groups.
func (ts *ProjectAPITestSuite) groupIDs(list projectGroupList) []string {
	ids := []string{}
	for _, group := range list.Groups {
		ids = append(ids, group.ID)
	}
	return ids
}
