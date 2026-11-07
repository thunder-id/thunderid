// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// projectApplication is the part of an application create or update response these tests read.
type projectApplication struct {
	ID        string `json:"id"`
	OUID      string `json:"ouId"`
	ProjectID string `json:"projectId"`
}

// projectApplicationList is the part of an application list response these tests read.
type projectApplicationList struct {
	TotalResults int `json:"totalResults"`
	Applications []struct {
		ID          string `json:"id"`
		ProjectID   string `json:"projectId"`
		IsOwnedByOU *bool  `json:"isOwnedByOU"`
	} `json:"applications"`
}

// Projects share the system organization unit, yet each lists only its own applications.
func (ts *ProjectAPITestSuite) TestCreateApplicationInProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("app-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("app-other")))

	inShop := ts.createApp(ts.appBody("shop", `"projectId":%q`, shop.ID))
	ts.Equal(defaultOUID, inShop.OUID)
	ts.Equal(shop.ID, inShop.ProjectID)
	inOther := ts.createApp(ts.appBody("other", `"projectId":%q`, other.ID))
	ts.Equal(defaultOUID, inOther.OUID)

	shopList := ts.listApps("?projectId=" + url.QueryEscape(shop.ID))
	ts.Equal([]string{inShop.ID}, ts.appIDs(shopList))
	ts.Equal(shop.ID, shopList.Applications[0].ProjectID)
	ts.Equal([]string{inOther.ID}, ts.appIDs(ts.listApps("?projectId="+url.QueryEscape(other.ID))))

	var fetched projectApplication
	ts.Require().Equal(http.StatusOK, ts.getJSON("/applications/"+inShop.ID, &fetched))
	ts.Equal(shop.ID, fetched.ProjectID)
}

// An application is still created without a project while the project scope does not require one.
func (ts *ProjectAPITestSuite) TestCreateApplicationWithoutProject() {
	app := ts.createApp(ts.appBody("unscoped", `"ouId":%q`, defaultOUID))
	ts.Empty(app.ProjectID)
	ts.Equal(defaultOUID, app.OUID)
}

// An update keeps an application in its project and organization unit unless it names others, and
// moves it from one project to another.
func (ts *ProjectAPITestSuite) TestUpdateApplicationWithProject() {
	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("app-upd-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("app-upd-second")))
	otherRoot := ts.createRootOU("app-upd-other")
	app := ts.createApp(ts.appBody("updated", `"projectId":%q`, first.ID))

	updated := ts.updateApp(app.ID, ts.appBody("updated", `"description":%q`, "updated"))
	ts.Equal(first.ID, updated.ProjectID, "an update without a project keeps it")
	ts.Equal(defaultOUID, updated.OUID, "an update without an organization unit keeps it")

	updated = ts.updateApp(app.ID, ts.appBody("updated", `"projectId":%q`, second.ID))
	ts.Equal(second.ID, updated.ProjectID)
	ts.Empty(ts.appIDs(ts.listApps("?projectId=" + url.QueryEscape(first.ID))))
	ts.Equal([]string{app.ID}, ts.appIDs(ts.listApps("?projectId="+url.QueryEscape(second.ID))))

	status, body := ts.do(http.MethodPut, "/applications/"+app.ID, ts.appBody("updated", `"ouId":%q`, otherRoot))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("APP-1064", codeOf(ts.T(), body))

	status, body = ts.do(http.MethodPut, "/applications/"+app.ID,
		ts.appBody("updated", `"projectId":%q`, "019a0000-0000-7000-8000-00000000dead"))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("APP-1063", codeOf(ts.T(), body))

	var fetched projectApplication
	ts.Require().Equal(http.StatusOK, ts.getJSON("/applications/"+app.ID, &fetched))
	ts.Equal(defaultOUID, fetched.OUID)
	ts.Equal(second.ID, fetched.ProjectID)
}

// An application in a project is in the system organization unit, and no other.
func (ts *ProjectAPITestSuite) TestApplicationInProjectOrganizationUnit() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("app-ou-shop")))
	otherRoot := ts.createRootOU("app-ou-other")
	childOUID := ts.createChildOU("app-ou-child", defaultOUID)

	inDefault := ts.createApp(ts.appBody("ou-default", `"projectId":%q,"ouId":%q`, shop.ID, defaultOUID))
	ts.Equal(defaultOUID, inDefault.OUID)

	for name, ouID := range map[string]string{
		"ou-child":   childOUID,
		"ou-outside": otherRoot,
		"ou-unknown": "019a0000-0000-7000-8000-00000000beef",
	} {
		status, body := ts.do(http.MethodPost, "/applications",
			ts.appBody(name, `"projectId":%q,"ouId":%q`, shop.ID, ouID))
		ts.Equal(http.StatusBadRequest, status, body)
		ts.Equal("APP-1064", codeOf(ts.T(), body), name)
	}

	ts.Equal([]string{inDefault.ID}, ts.appIDs(ts.listApps("?projectId="+url.QueryEscape(shop.ID))))

	list := ts.listApps("?projectId=" + url.QueryEscape(shop.ID) + "&filter=" +
		url.QueryEscape(fmt.Sprintf("ouId eq %q", otherRoot)))
	ts.Empty(list.Applications)
}

// A project is not deleted while an application is still in it.
func (ts *ProjectAPITestSuite) TestDeleteProjectWithApplicationIsRefused() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Busy"}`, ts.handle("app-busy")))
	app := ts.createApp(ts.appBody("busy", `"projectId":%q`, created.ID))

	status, body := ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusConflict, status, body)
	ts.Equal("PRJ-1013", codeOf(ts.T(), body))

	ts.Require().NoError(testutils.DeleteApplication(app.ID))
	status, body = ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusNoContent, status, body)
}

func (ts *ProjectAPITestSuite) TestApplicationWithUnknownProject() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"

	status, body := ts.do(http.MethodPost, "/applications",
		ts.appBody("unknown", `"projectId":%q`, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("APP-1063", codeOf(ts.T(), body))

	list := ts.listApps("?projectId=" + unknownProjectID)
	ts.Zero(list.TotalResults)
	ts.Empty(list.Applications)

	list = ts.listApps("?projectId=")
	ts.Zero(list.TotalResults)
	ts.Empty(list.Applications)
}

func (ts *ProjectAPITestSuite) TestListApplicationsInProjectWithOtherFilter() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Filtered"}`, ts.handle("app-filtered")))

	status, body := ts.do(http.MethodGet, "/applications?projectId="+url.QueryEscape(created.ID)+"&filter="+
		url.QueryEscape(`name eq "app"`), "")

	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("APP-1062", codeOf(ts.T(), body))
}

// The list_applications and create_application MCP tools scope applications to a project the same way
// the REST API does.
func (ts *ProjectAPITestSuite) TestApplicationMCPToolsInProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("app-mcp-shop")))
	childOUID := ts.createChildOU("app-mcp-child", defaultOUID)

	inDefault := ts.createMCPApp(map[string]any{"name": ts.handle("app-mcp-default"), "projectId": shop.ID})
	ts.Equal(defaultOUID, inDefault["ouId"])

	unscoped := ts.createMCPApp(map[string]any{"name": ts.handle("app-mcp-unscoped"), "ouId": defaultOUID})
	ts.Empty(unscoped["projectId"])

	args := map[string]any{
		"name": ts.handle("app-mcp-child"), "projectId": shop.ID, "ouId": childOUID,
		"type": "fullstack", "isRegistrationFlowEnabled": false, "isRecoveryFlowEnabled": false,
	}
	created, raw := ts.callMCPTool("create_application", args)
	ts.Truef(created.Result.IsError, "an application in another organization unit was created: %s", raw)

	ts.Equal([]string{inDefault["id"].(string)}, ts.listMCPApps(map[string]any{"projectId": shop.ID}))
	ts.Empty(ts.listMCPApps(map[string]any{"projectId": "019a0000-0000-7000-8000-00000000dead"}))
}

// createMCPApp creates an application through the create_application MCP tool, requires it to succeed,
// and records it for cleanup.
func (ts *ProjectAPITestSuite) createMCPApp(args map[string]any) map[string]any {
	args["type"] = "fullstack"
	args["isRegistrationFlowEnabled"] = false
	args["isRecoveryFlowEnabled"] = false
	created, raw := ts.callMCPTool("create_application", args)
	ts.Require().Falsef(created.Result.IsError, "create failed: %s", raw)
	id, _ := created.Result.StructuredContent["id"].(string)
	ts.Require().NotEmpty(id)
	ts.apps = append(ts.apps, id)
	return created.Result.StructuredContent
}

// listMCPApps lists applications through the list_applications MCP tool and returns their IDs.
func (ts *ProjectAPITestSuite) listMCPApps(args map[string]any) []string {
	listed, raw := ts.callMCPTool("list_applications", args)
	ts.Require().Falsef(listed.Result.IsError, "list failed: %s", raw)
	apps, _ := listed.Result.StructuredContent["applications"].([]any)
	ids := []string{}
	for _, app := range apps {
		id, _ := app.(map[string]any)["id"].(string)
		ids = append(ids, id)
	}
	return ids
}

// appBody builds an application request without inbound authentication, with the given placement
// fields spliced in.
func (ts *ProjectAPITestSuite) appBody(name, placement string, args ...interface{}) string {
	return fmt.Sprintf(`{"name":%q,"type":"fullstack",%s}`, ts.handle("app-"+name), fmt.Sprintf(placement, args...))
}

// createApp posts an application, requires a 201, and records it for cleanup.
func (ts *ProjectAPITestSuite) createApp(body string) projectApplication {
	status, resp := ts.do(http.MethodPost, "/applications", body)
	ts.Require().Equal(http.StatusCreated, status, resp)
	var created projectApplication
	ts.Require().NoError(json.Unmarshal([]byte(resp), &created))
	ts.apps = append(ts.apps, created.ID)
	return created
}

// updateApp puts an application and requires a 200.
func (ts *ProjectAPITestSuite) updateApp(id, body string) projectApplication {
	status, resp := ts.do(http.MethodPut, "/applications/"+id, body)
	ts.Require().Equal(http.StatusOK, status, resp)
	var updated projectApplication
	ts.Require().NoError(json.Unmarshal([]byte(resp), &updated))
	return updated
}

// listApps lists applications with the given query and requires a 200.
func (ts *ProjectAPITestSuite) listApps(query string) projectApplicationList {
	var list projectApplicationList
	ts.Require().Equal(http.StatusOK, ts.getJSON("/applications"+query, &list))
	return list
}

// appIDs returns the IDs of the listed applications.
func (ts *ProjectAPITestSuite) appIDs(list projectApplicationList) []string {
	ids := []string{}
	for _, app := range list.Applications {
		ids = append(ids, app.ID)
	}
	return ids
}

// createRootOU creates a root organization unit outside every project and records it for cleanup.
func (ts *ProjectAPITestSuite) createRootOU(name string) string {
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{Handle: ts.handle(name), Name: name})
	ts.Require().NoError(err)
	ts.ous = append(ts.ous, id)
	return id
}

// createChildOU creates an organization unit below the given parent and records it for cleanup.
func (ts *ProjectAPITestSuite) createChildOU(name, parentID string) string {
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.handle(name), Name: name, Parent: &parentID,
	})
	ts.Require().NoError(err)
	ts.ous = append(ts.ous, id)
	return id
}
