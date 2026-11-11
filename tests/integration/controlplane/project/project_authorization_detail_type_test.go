// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// authorizationDetailTypesPath is the collection of authorization details types these tests create.
const authorizationDetailTypesPath = "/authorization-detail-types"

// projectAuthorizationDetailType is the part of an authorization details type response these tests read.
type projectAuthorizationDetailType struct {
	ID        string `json:"id"`
	Version   int    `json:"version"`
	ProjectID string `json:"projectId"`
}

// On the Control Plane, an authorization details type belongs to a project.
func (ts *ProjectAPITestSuite) TestCreateAuthorizationDetailTypeRequiresProject() {
	status, body := ts.do(http.MethodPost, authorizationDetailTypesPath, ts.authorizationDetailTypeBody("unplaced", ""))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("RAR-1031", codeOf(ts.T(), body))

	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"
	status, body = ts.do(http.MethodPost, authorizationDetailTypesPath,
		ts.authorizationDetailTypeBody("unknown", unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("RAR-1032", codeOf(ts.T(), body))
}

// Each project lists only its own types, and a type is read back with its project.
func (ts *ProjectAPITestSuite) TestCreateAuthorizationDetailTypeInProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("adt-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("adt-other")))

	inShop := ts.createAuthorizationDetailType(ts.authorizationDetailTypeBody("shop", shop.ID))
	ts.Equal(shop.ID, inShop.ProjectID)
	inOther := ts.createAuthorizationDetailType(ts.authorizationDetailTypeBody("other", other.ID))

	ts.Equal([]string{inShop.ID}, ts.authorizationDetailTypeIDs(shop.ID))
	ts.Equal([]string{inOther.ID}, ts.authorizationDetailTypeIDs(other.ID))

	var fetched projectAuthorizationDetailType
	ts.Require().Equal(http.StatusOK, ts.getJSON(authorizationDetailTypesPath+"/"+inShop.ID, &fetched))
	ts.Equal(shop.ID, fetched.ProjectID)
}

// An update keeps a type in its project unless it names another one, moves it without a new version, and refuses
// a project that does not exist.
func (ts *ProjectAPITestSuite) TestUpdateAuthorizationDetailTypeWithProject() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"
	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("adt-upd-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("adt-upd-second")))
	created := ts.createAuthorizationDetailType(ts.authorizationDetailTypeBody("updated", first.ID))

	updated := ts.updateAuthorizationDetailType(created.ID, ts.authorizationDetailTypeBody("updated", ""))
	ts.Equal(first.ID, updated.ProjectID, "an update without a project keeps it")

	updated = ts.updateAuthorizationDetailType(created.ID, ts.authorizationDetailTypeBody("updated", second.ID))
	ts.Equal(second.ID, updated.ProjectID)
	ts.Equal(created.Version, updated.Version, "a move to another project keeps the version")
	ts.Empty(ts.authorizationDetailTypeIDs(first.ID))

	status, body := ts.do(http.MethodPut, authorizationDetailTypesPath+"/"+created.ID,
		ts.authorizationDetailTypeBody("updated", unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("RAR-1032", codeOf(ts.T(), body))
}

// A project that still has an authorization details type is not deleted until the type is gone.
func (ts *ProjectAPITestSuite) TestDeleteProjectWithAuthorizationDetailTypeIsRefused() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Busy"}`, ts.handle("adt-busy")))
	held := ts.createAuthorizationDetailType(ts.authorizationDetailTypeBody("busy", created.ID))

	status, body := ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusConflict, status, body)
	ts.Equal("PRJ-1013", codeOf(ts.T(), body))

	status, body = ts.do(http.MethodDelete, authorizationDetailTypesPath+"/"+held.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, body)
	status, body = ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusNoContent, status, body)
}

// An exported type keeps its project, so a deploy of the project carries it.
func (ts *ProjectAPITestSuite) TestExportedAuthorizationDetailTypeKeepsItsProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("adt-export-shop")))
	created := ts.createAuthorizationDetailType(ts.authorizationDetailTypeBody("export", shop.ID))

	status, resp := ts.do(http.MethodPost, "/export",
		fmt.Sprintf(`{"authorizationDetailTypes":[%q]}`, created.ID))
	ts.Require().Equal(http.StatusOK, status, resp)
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(resp), &exported))
	ts.Contains(exported.Resources, "projectId: "+shop.ID)
}

// authorizationDetailTypeBody builds an authorization details type request, in the given project when one is
// named.
func (ts *ProjectAPITestSuite) authorizationDetailTypeBody(name, projectID string) string {
	project := ""
	if projectID != "" {
		project = fmt.Sprintf(`,"projectId":%q`, projectID)
	}
	return fmt.Sprintf(`{"type":%q,"name":%q,"schema":{"type":"object","properties":{"amount":{"type":"number"}}}%s}`,
		"https://types.example.com/"+ts.handle("adt-"+name), "Type "+name, project)
}

// createAuthorizationDetailType posts an authorization details type, requires a 201, and records it for cleanup.
func (ts *ProjectAPITestSuite) createAuthorizationDetailType(body string) projectAuthorizationDetailType {
	status, resp := ts.do(http.MethodPost, authorizationDetailTypesPath, body)
	ts.Require().Equal(http.StatusCreated, status, resp)
	var created projectAuthorizationDetailType
	ts.Require().NoError(json.Unmarshal([]byte(resp), &created))
	ts.authorizationDetailTypes = append(ts.authorizationDetailTypes, created.ID)
	return created
}

// updateAuthorizationDetailType puts an authorization details type and requires a 200.
func (ts *ProjectAPITestSuite) updateAuthorizationDetailType(id, body string) projectAuthorizationDetailType {
	status, resp := ts.do(http.MethodPut, authorizationDetailTypesPath+"/"+id, body)
	ts.Require().Equal(http.StatusOK, status, resp)
	var updated projectAuthorizationDetailType
	ts.Require().NoError(json.Unmarshal([]byte(resp), &updated))
	return updated
}

// authorizationDetailTypeIDs lists a project's authorization details types and returns their IDs.
func (ts *ProjectAPITestSuite) authorizationDetailTypeIDs(projectID string) []string {
	var list struct {
		Types []projectAuthorizationDetailType `json:"types"`
	}
	ts.Require().Equal(http.StatusOK,
		ts.getJSON(authorizationDetailTypesPath+"?limit=100&projectId="+url.QueryEscape(projectID), &list))
	ids := []string{}
	for _, t := range list.Types {
		ids = append(ids, t.ID)
	}
	return ids
}
