// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// projectTheme is the part of a theme response these tests read.
type projectTheme struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
}

// projectThemeList is the part of a theme list response these tests read.
type projectThemeList struct {
	TotalResults int            `json:"totalResults"`
	Themes       []projectTheme `json:"themes"`
}

// A theme created in a project carries it, and each project lists only its own themes.
func (ts *ProjectAPITestSuite) TestCreateThemeInProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("theme-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("theme-other")))

	inShop := ts.createTheme(ts.themeBody("shop", fmt.Sprintf(`,"projectId":%q`, shop.ID)))
	ts.Equal(shop.ID, inShop.ProjectID)
	inOther := ts.createTheme(ts.themeBody("other", fmt.Sprintf(`,"projectId":%q`, other.ID)))
	unscoped := ts.createTheme(ts.themeBody("unscoped", ""))
	ts.Empty(unscoped.ProjectID)

	shopList := ts.listThemes("?limit=100&projectId=" + url.QueryEscape(shop.ID))
	ts.Equal(1, shopList.TotalResults)
	ts.Equal([]projectTheme{{ID: inShop.ID, ProjectID: shop.ID}}, shopList.Themes)
	ts.Equal([]string{inOther.ID}, themeIDs(ts.listThemes("?limit=100&projectId="+url.QueryEscape(other.ID))))
	ts.Contains(themeIDs(ts.listThemes("?limit=100")), unscoped.ID, "a list without a project lists every theme")

	var fetched projectTheme
	ts.Require().Equal(http.StatusOK, ts.getJSON("/design/themes/"+inShop.ID, &fetched))
	ts.Equal(shop.ID, fetched.ProjectID)
}

// An update keeps a theme in its project unless it names another one, and moves it between projects.
func (ts *ProjectAPITestSuite) TestUpdateThemeWithProject() {
	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("theme-upd-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("theme-upd-second")))
	theme := ts.createTheme(ts.themeBody("updated", fmt.Sprintf(`,"projectId":%q`, first.ID)))

	updated := ts.updateTheme(theme.ID, ts.themeBody("updated", ""))
	ts.Equal(first.ID, updated.ProjectID, "an update without a project keeps it")

	updated = ts.updateTheme(theme.ID, ts.themeBody("updated", fmt.Sprintf(`,"projectId":%q`, second.ID)))
	ts.Equal(second.ID, updated.ProjectID)
	ts.Empty(themeIDs(ts.listThemes("?limit=100&projectId=" + url.QueryEscape(first.ID))))
	ts.Equal([]string{theme.ID}, themeIDs(ts.listThemes("?limit=100&projectId="+url.QueryEscape(second.ID))))
}

// A project that does not exist fails a create and an update, and lists no themes.
func (ts *ProjectAPITestSuite) TestThemeWithUnknownProject() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"
	body := ts.themeBody("unknown", fmt.Sprintf(`,"projectId":%q`, unknownProjectID))

	status, resp := ts.do(http.MethodPost, "/design/themes", body)
	ts.Equal(http.StatusBadRequest, status, resp)
	ts.Equal("THM-1018", codeOf(ts.T(), resp))

	theme := ts.createTheme(ts.themeBody("unknown", ""))
	status, resp = ts.do(http.MethodPut, "/design/themes/"+theme.ID, body)
	ts.Equal(http.StatusBadRequest, status, resp)
	ts.Equal("THM-1018", codeOf(ts.T(), resp))

	list := ts.listThemes("?projectId=" + unknownProjectID)
	ts.Zero(list.TotalResults)
	ts.Empty(list.Themes)
}

// list_themes takes a project and lists only its themes, and get_theme_by_id carries the project.
func (ts *ProjectAPITestSuite) TestThemeMCPToolsWithProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("theme-mcp-shop")))
	inShop := ts.createTheme(ts.themeBody("mcp", fmt.Sprintf(`,"projectId":%q`, shop.ID)))
	ts.createTheme(ts.themeBody("mcp-unscoped", ""))

	listed, raw := ts.callMCPTool("list_themes", map[string]any{"projectId": shop.ID})
	ts.Require().Falsef(listed.Result.IsError, "list failed: %s", raw)
	ts.EqualValues(1, listed.Result.StructuredContent["totalResults"])
	ts.Contains(raw, inShop.ID)

	fetched, raw := ts.callMCPTool("get_theme_by_id", map[string]any{"id": inShop.ID})
	ts.Require().Falsef(fetched.Result.IsError, "get failed: %s", raw)
	ts.Equal(shop.ID, fetched.Result.StructuredContent["projectId"])
}

// themeBody builds a theme request, with the given fields appended.
func (ts *ProjectAPITestSuite) themeBody(name, fields string) string {
	return fmt.Sprintf(`{"handle":%q,"displayName":%q,"theme":{"colorSchemes":{"light":{}}}%s}`,
		ts.handle("theme-"+name), "Theme "+name, fields)
}

// createTheme posts a theme, requires a 201, and records it for cleanup.
func (ts *ProjectAPITestSuite) createTheme(body string) projectTheme {
	status, resp := ts.do(http.MethodPost, "/design/themes", body)
	ts.Require().Equal(http.StatusCreated, status, resp)
	var created projectTheme
	ts.Require().NoError(json.Unmarshal([]byte(resp), &created))
	ts.themes = append(ts.themes, created.ID)
	return created
}

// updateTheme puts a theme and requires a 200.
func (ts *ProjectAPITestSuite) updateTheme(id, body string) projectTheme {
	status, resp := ts.do(http.MethodPut, "/design/themes/"+id, body)
	ts.Require().Equal(http.StatusOK, status, resp)
	var updated projectTheme
	ts.Require().NoError(json.Unmarshal([]byte(resp), &updated))
	return updated
}

// listThemes lists themes with the given query and requires a 200.
func (ts *ProjectAPITestSuite) listThemes(query string) projectThemeList {
	var list projectThemeList
	ts.Require().Equal(http.StatusOK, ts.getJSON("/design/themes"+query, &list))
	return list
}

// themeIDs returns the IDs of the listed themes.
func themeIDs(list projectThemeList) []string {
	ids := []string{}
	for _, theme := range list.Themes {
		ids = append(ids, theme.ID)
	}
	return ids
}
