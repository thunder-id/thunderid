// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// definitionsPath is the collection of presentation definitions these tests create.
const definitionsPath = "/openid4vp/presentation-definitions"

// projectDefinition is the part of a presentation definition response these tests read.
type projectDefinition struct {
	ID        string `json:"id"`
	OUID      string `json:"ouId"`
	ProjectID string `json:"projectId"`
}

// On the Control Plane, a presentation definition belongs to a project.
func (ts *ProjectAPITestSuite) TestCreatePresentationDefinitionRequiresProject() {
	status, body := ts.do(http.MethodPost, definitionsPath,
		ts.definitionBody("unplaced", `"ouId":%q`, defaultOUID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("VP-2020", codeOf(ts.T(), body))
}

// A definition in a project is in the system organization unit, and each project lists only its own.
func (ts *ProjectAPITestSuite) TestCreatePresentationDefinitionInProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("vp-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("vp-other")))

	inShop := ts.createDefinition(ts.definitionBody("shop", `"projectId":%q`, shop.ID))
	ts.Equal(defaultOUID, inShop.OUID)
	ts.Equal(shop.ID, inShop.ProjectID)
	inOther := ts.createDefinition(ts.definitionBody("other", `"projectId":%q`, other.ID))

	ts.Equal([]string{inShop.ID}, ts.definitionIDs("?projectId="+url.QueryEscape(shop.ID)))
	ts.Equal([]string{inOther.ID}, ts.definitionIDs("?projectId="+url.QueryEscape(other.ID)))

	var fetched projectDefinition
	ts.Require().Equal(http.StatusOK, ts.getJSON(definitionsPath+"/"+inShop.ID, &fetched))
	ts.Equal(shop.ID, fetched.ProjectID)
}

// A definition in a project is in the system organization unit, and no other.
func (ts *ProjectAPITestSuite) TestPresentationDefinitionInProjectOrganizationUnit() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("vp-ou-shop")))
	childOUID := ts.createChildOU("vp-ou-child", defaultOUID)

	status, body := ts.do(http.MethodPost, definitionsPath,
		ts.definitionBody("ou-child", `"projectId":%q,"ouId":%q`, shop.ID, childOUID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("VP-2019", codeOf(ts.T(), body))
}

// An update keeps a definition in its project unless it names another one, moves it between projects, and
// refuses a project that does not exist.
func (ts *ProjectAPITestSuite) TestUpdatePresentationDefinitionWithProject() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"
	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("vp-upd-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("vp-upd-second")))
	created := ts.createDefinition(ts.definitionBody("updated", `"projectId":%q`, first.ID))

	updated := ts.updateDefinition(created.ID, ts.definitionBody("updated", `"name":%q`, "Renamed"))
	ts.Equal(first.ID, updated.ProjectID, "an update without a project keeps it")

	updated = ts.updateDefinition(created.ID, ts.definitionBody("updated", `"projectId":%q`, second.ID))
	ts.Equal(second.ID, updated.ProjectID)
	ts.Empty(ts.definitionIDs("?projectId=" + url.QueryEscape(first.ID)))

	status, body := ts.do(http.MethodPut, definitionsPath+"/"+created.ID,
		ts.definitionBody("updated", `"projectId":%q`, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("VP-2018", codeOf(ts.T(), body))
}

// A project that still has a presentation definition is not deleted until the definition is gone.
func (ts *ProjectAPITestSuite) TestDeleteProjectWithPresentationDefinitionIsRefused() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Busy"}`, ts.handle("vp-busy")))
	definition := ts.createDefinition(ts.definitionBody("busy", `"projectId":%q`, created.ID))

	status, body := ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusConflict, status, body)
	ts.Equal("PRJ-1013", codeOf(ts.T(), body))

	status, body = ts.do(http.MethodDelete, definitionsPath+"/"+definition.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, body)
	status, body = ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusNoContent, status, body)
}

// An exported definition keeps its project, so a deploy of the project carries it.
func (ts *ProjectAPITestSuite) TestExportedPresentationDefinitionKeepsItsProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("vp-export-shop")))
	created := ts.createDefinition(ts.definitionBody("export", `"projectId":%q`, shop.ID))

	status, resp := ts.do(http.MethodPost, "/export", fmt.Sprintf(`{"presentationDefinitions":[%q]}`, created.ID))
	ts.Require().Equal(http.StatusOK, status, resp)
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(resp), &exported))
	ts.Contains(exported.Resources, "projectId: "+shop.ID)
}

// definitionBody builds a presentation definition request, with the given fields spliced in.
func (ts *ProjectAPITestSuite) definitionBody(name, fields string, args ...interface{}) string {
	return fmt.Sprintf(`{"handle":%q,"vct":"urn:example:pid:1",%s}`, ts.handle("vp-"+name),
		fmt.Sprintf(fields, args...))
}

// createDefinition posts a presentation definition, requires a 201, and records it for cleanup.
func (ts *ProjectAPITestSuite) createDefinition(body string) projectDefinition {
	status, resp := ts.do(http.MethodPost, definitionsPath, body)
	ts.Require().Equal(http.StatusCreated, status, resp)
	var created projectDefinition
	ts.Require().NoError(json.Unmarshal([]byte(resp), &created))
	ts.presentationDefinitions = append(ts.presentationDefinitions, created.ID)
	return created
}

// updateDefinition puts a presentation definition and requires a 200.
func (ts *ProjectAPITestSuite) updateDefinition(id, body string) projectDefinition {
	status, resp := ts.do(http.MethodPut, definitionsPath+"/"+id, body)
	ts.Require().Equal(http.StatusOK, status, resp)
	var updated projectDefinition
	ts.Require().NoError(json.Unmarshal([]byte(resp), &updated))
	return updated
}

// definitionIDs lists presentation definitions with the given query and returns their IDs.
func (ts *ProjectAPITestSuite) definitionIDs(query string) []string {
	var list []projectDefinition
	ts.Require().Equal(http.StatusOK, ts.getJSON(definitionsPath+query, &list))
	ids := []string{}
	for _, definition := range list {
		ids = append(ids, definition.ID)
	}
	return ids
}
