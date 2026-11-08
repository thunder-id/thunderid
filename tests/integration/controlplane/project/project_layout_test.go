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

// builtInLayoutID is the Centered layout the bootstrap declares in the Default project.
const builtInLayoutID = "01900000-0000-7000-8000-000000000078"

// projectLayout is the part of a layout response these tests read.
type projectLayout struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
}

// projectLayoutList is the part of a layout list response these tests read.
type projectLayoutList struct {
	TotalResults int             `json:"totalResults"`
	Layouts      []projectLayout `json:"layouts"`
}

// The Control Plane serves projects, so a layout must name one.
func (ts *ProjectAPITestSuite) TestCreateLayoutRequiresProject() {
	status, body := ts.do(http.MethodPost, "/design/layouts", ts.layoutBody("no-project", ""))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("LAY-1020", codeOf(ts.T(), body))

	status, body = ts.do(http.MethodPost, "/design/layouts",
		ts.layoutBody("unknown-project", `"projectId":"019a0000-0000-7000-8000-00000000dead",`))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("LAY-1019", codeOf(ts.T(), body))
}

// Each project lists only its own layouts.
func (ts *ProjectAPITestSuite) TestCreateLayoutInProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("layout-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("layout-other")))

	inShop := ts.createLayout(ts.layoutBody("shop", fmt.Sprintf(`"projectId":%q,`, shop.ID)))
	ts.Equal(shop.ID, inShop.ProjectID)
	inOther := ts.createLayout(ts.layoutBody("other", fmt.Sprintf(`"projectId":%q,`, other.ID)))

	ts.Equal([]string{inShop.ID}, ts.layoutIDs(ts.listLayouts("?limit=100&projectId="+url.QueryEscape(shop.ID))))
	ts.Equal([]string{inOther.ID}, ts.layoutIDs(ts.listLayouts("?limit=100&projectId="+url.QueryEscape(other.ID))))
	ts.Empty(ts.listLayouts("?projectId=019a0000-0000-7000-8000-00000000dead").Layouts)

	var fetched projectLayout
	ts.Require().Equal(http.StatusOK, ts.getJSON("/design/layouts/"+inShop.ID, &fetched))
	ts.Equal(shop.ID, fetched.ProjectID)
}

// An update keeps a layout in its project unless it names another, which moves the layout there.
func (ts *ProjectAPITestSuite) TestUpdateLayoutWithProject() {
	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("layout-upd-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("layout-upd-second")))
	created := ts.createLayout(ts.layoutBody("updated", fmt.Sprintf(`"projectId":%q,`, first.ID)))

	updated := ts.updateLayout(created.ID, ts.layoutBody("updated", ""))
	ts.Equal(first.ID, updated.ProjectID, "an update without a project keeps it")

	updated = ts.updateLayout(created.ID, ts.layoutBody("updated", fmt.Sprintf(`"projectId":%q,`, second.ID)))
	ts.Equal(second.ID, updated.ProjectID)
	ts.Empty(ts.listLayouts("?limit=100&projectId=" + url.QueryEscape(first.ID)).Layouts)

	status, body := ts.do(http.MethodPut, "/design/layouts/"+created.ID,
		ts.layoutBody("updated", `"projectId":"019a0000-0000-7000-8000-00000000dead",`))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("LAY-1019", codeOf(ts.T(), body))
}

// The bootstrap's Centered layout belongs to the Default project.
func (ts *ProjectAPITestSuite) TestBuiltInLayoutIsInTheDefaultProject() {
	var layout projectLayout
	ts.Require().Equal(http.StatusOK, ts.getJSON("/design/layouts/"+builtInLayoutID, &layout))
	ts.Equal(defaultProjectID, layout.ProjectID)
	ts.Contains(ts.layoutIDs(ts.listLayouts("?limit=100&projectId="+url.QueryEscape(defaultProjectID))),
		builtInLayoutID)
}

// An exported layout carries its project, so importing the export brings it back in the same project.
func (ts *ProjectAPITestSuite) TestExportedLayoutKeepsItsProject() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Export"}`, ts.handle("layout-export")))
	layout := ts.createLayout(ts.layoutBody("exported", fmt.Sprintf(`"projectId":%q,`, created.ID)))

	body, err := json.Marshal(map[string]any{"layouts": []string{layout.ID}})
	ts.Require().NoError(err)
	status, resp := ts.do(http.MethodPost, "/export", string(body))
	ts.Require().Equal(http.StatusOK, status, resp)
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(resp), &exported))
	ts.Equal(1, strings.Count(exported.Resources, "projectId: "+created.ID), exported.Resources)

	status, resp = ts.do(http.MethodDelete, "/design/layouts/"+layout.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, resp)

	ts.importContent(exported.Resources)
	var imported projectLayout
	ts.Require().Equal(http.StatusOK, ts.getJSON("/design/layouts/"+layout.ID, &imported))
	ts.Equal(created.ID, imported.ProjectID)
}

// A project that still has a layout is not deleted until the layout is gone.
func (ts *ProjectAPITestSuite) TestDeleteProjectWithLayoutIsRefused() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Busy"}`, ts.handle("layout-busy")))
	layout := ts.createLayout(ts.layoutBody("busy", fmt.Sprintf(`"projectId":%q,`, created.ID)))

	status, body := ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusConflict, status, body)
	ts.Equal("PRJ-1013", codeOf(ts.T(), body))

	status, body = ts.do(http.MethodDelete, "/design/layouts/"+layout.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, body)
	status, body = ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusNoContent, status, body)
}

// layoutBody builds a layout request with a unique handle, with the given project field spliced in.
func (ts *ProjectAPITestSuite) layoutBody(name, project string) string {
	return fmt.Sprintf(`{"handle":%q,"displayName":%q,%s"layout":{"structure":"centered"}}`,
		ts.handle("layout-"+name), "Layout "+name, project)
}

// createLayout posts a layout, requires a 201, and records it for cleanup.
func (ts *ProjectAPITestSuite) createLayout(body string) projectLayout {
	status, resp := ts.do(http.MethodPost, "/design/layouts", body)
	ts.Require().Equal(http.StatusCreated, status, resp)
	var created projectLayout
	ts.Require().NoError(json.Unmarshal([]byte(resp), &created))
	ts.layouts = append(ts.layouts, created.ID)
	return created
}

// updateLayout puts a layout and requires a 200.
func (ts *ProjectAPITestSuite) updateLayout(id, body string) projectLayout {
	status, resp := ts.do(http.MethodPut, "/design/layouts/"+id, body)
	ts.Require().Equal(http.StatusOK, status, resp)
	var updated projectLayout
	ts.Require().NoError(json.Unmarshal([]byte(resp), &updated))
	return updated
}

// listLayouts lists layouts with the given query and requires a 200.
func (ts *ProjectAPITestSuite) listLayouts(query string) projectLayoutList {
	var list projectLayoutList
	ts.Require().Equal(http.StatusOK, ts.getJSON("/design/layouts"+query, &list))
	return list
}

// layoutIDs returns the IDs of the listed layouts.
func (ts *ProjectAPITestSuite) layoutIDs(list projectLayoutList) []string {
	ids := []string{}
	for _, layout := range list.Layouts {
		ids = append(ids, layout.ID)
	}
	return ids
}
