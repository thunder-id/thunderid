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
	ID   string `json:"id"`
	OUID string `json:"ouId"`
}

// projectApplicationList is the part of an application list response these tests read.
type projectApplicationList struct {
	TotalResults int `json:"totalResults"`
	Applications []struct {
		ID          string `json:"id"`
		IsOwnedByOU *bool  `json:"isOwnedByOU"`
	} `json:"applications"`
}

func (ts *ProjectAPITestSuite) TestCreateApplicationInProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("app-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("app-other")))

	inShop := ts.createApp(ts.appBody("shop", `"projectId":%q`, shop.ID))
	ts.Equal(shop.RootOUID, inShop.OUID)
	inOther := ts.createApp(ts.appBody("other", `"ouId":%q`, other.RootOUID))

	list := ts.listApps("?projectId=" + url.QueryEscape(shop.ID))

	ts.Equal(1, list.TotalResults)
	ts.Require().Len(list.Applications, 1)
	ts.Equal(inShop.ID, list.Applications[0].ID)
	ts.Equal(new(true), list.Applications[0].IsOwnedByOU)

	list = ts.listApps("?projectId=" + url.QueryEscape(other.ID))
	ts.Require().Len(list.Applications, 1)
	ts.Equal(inOther.ID, list.Applications[0].ID)
}

func (ts *ProjectAPITestSuite) TestUpdateApplicationWithProject() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Updated"}`, ts.handle("app-updated")))
	app := ts.createApp(ts.appBody("updated", `"ouId":%q`, created.RootOUID))

	status, body := ts.do(http.MethodPut, "/applications/"+app.ID,
		ts.appBody("renamed", `"projectId":%q`, created.ID))

	ts.Require().Equal(http.StatusOK, status, body)
	var updated projectApplication
	ts.Require().NoError(json.Unmarshal([]byte(body), &updated))
	ts.Equal(created.RootOUID, updated.OUID)
	list := ts.listApps("?projectId=" + url.QueryEscape(created.ID))
	ts.Require().Len(list.Applications, 1)
	ts.Equal(app.ID, list.Applications[0].ID)

	status, body = ts.do(http.MethodPut, "/applications/"+app.ID,
		ts.appBody("renamed", `"projectId":%q`, "019a0000-0000-7000-8000-00000000dead"))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("APP-1063", codeOf(ts.T(), body))
}

func (ts *ProjectAPITestSuite) TestApplicationInProjectOrganizationUnit() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("app-ou-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("app-ou-other")))
	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.handle("app-ou-child"), Name: "Child", Parent: &shop.RootOUID,
	})
	ts.Require().NoError(err)
	ts.childOUs = append(ts.childOUs, childOUID)

	inRoot := ts.createApp(ts.appBody("ou-root", `"projectId":%q,"ouId":%q`, shop.ID, shop.RootOUID))
	ts.Equal(shop.RootOUID, inRoot.OUID)
	inChild := ts.createApp(ts.appBody("ou-child", `"projectId":%q,"ouId":%q`, shop.ID, childOUID))
	ts.Equal(childOUID, inChild.OUID)

	status, body := ts.do(http.MethodPost, "/applications",
		ts.appBody("ou-outside", `"projectId":%q,"ouId":%q`, shop.ID, other.RootOUID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("APP-1064", codeOf(ts.T(), body))

	status, body = ts.do(http.MethodPost, "/applications",
		ts.appBody("ou-unknown", `"projectId":%q,"ouId":%q`, shop.ID, "019a0000-0000-7000-8000-00000000beef"))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("APP-1018", codeOf(ts.T(), body))

	list := ts.listApps("?projectId=" + url.QueryEscape(shop.ID))
	ts.Require().Len(list.Applications, 1)
	ts.Equal(inRoot.ID, list.Applications[0].ID)

	list = ts.listApps("?projectId=" + url.QueryEscape(shop.ID) + "&filter=" +
		url.QueryEscape(fmt.Sprintf("ouId eq %q", childOUID)))
	ts.Require().Len(list.Applications, 1)
	ts.Equal(inChild.ID, list.Applications[0].ID)

	list = ts.listApps("?projectId=" + url.QueryEscape(shop.ID) + "&filter=" +
		url.QueryEscape(fmt.Sprintf("ouId eq %q", other.RootOUID)))
	ts.Empty(list.Applications)
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
	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.handle("app-mcp-child"), Name: "Child", Parent: &shop.RootOUID,
	})
	ts.Require().NoError(err)
	ts.childOUs = append(ts.childOUs, childOUID)

	inRoot := ts.createMCPApp(map[string]any{"name": ts.handle("app-mcp-root"), "projectId": shop.ID})
	ts.Equal(shop.RootOUID, inRoot["ouId"])
	inChild := ts.createMCPApp(map[string]any{
		"name": ts.handle("app-mcp-child"), "projectId": shop.ID, "ouId": childOUID,
	})
	ts.Equal(childOUID, inChild["ouId"])

	ts.Equal([]string{inRoot["id"].(string)}, ts.listMCPApps(map[string]any{"projectId": shop.ID}))
	ts.Equal([]string{inChild["id"].(string)},
		ts.listMCPApps(map[string]any{"projectId": shop.ID, "ouId": childOUID}))
	ts.Equal([]string{inChild["id"].(string)}, ts.listMCPApps(map[string]any{"ouId": childOUID}))
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

// listApps lists applications with the given query and requires a 200.
func (ts *ProjectAPITestSuite) listApps(query string) projectApplicationList {
	var list projectApplicationList
	ts.Require().Equal(http.StatusOK, ts.getJSON("/applications"+query, &list))
	return list
}
