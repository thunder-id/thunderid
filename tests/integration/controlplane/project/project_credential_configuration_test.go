// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// configurationsPath is the collection of credential configurations these tests create.
const configurationsPath = "/openid4vci/credential-configurations"

// projectConfiguration is the part of a credential configuration response these tests read.
type projectConfiguration struct {
	ID        string `json:"id"`
	OUID      string `json:"ouId"`
	ProjectID string `json:"projectId"`
}

// On the Control Plane, a credential configuration belongs to a project.
func (ts *ProjectAPITestSuite) TestCreateCredentialConfigurationRequiresProject() {
	status, body := ts.do(http.MethodPost, configurationsPath,
		ts.configurationBody("unplaced", `"ouId":%q`, defaultOUID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("VCI-2015", codeOf(ts.T(), body))
}

// A configuration in a project is in the system organization unit, and each project lists only its own.
func (ts *ProjectAPITestSuite) TestCreateCredentialConfigurationInProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("vci-shop")))
	other := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Other"}`, ts.handle("vci-other")))

	inShop := ts.createConfiguration(ts.configurationBody("shop", `"projectId":%q`, shop.ID))
	ts.Equal(defaultOUID, inShop.OUID)
	ts.Equal(shop.ID, inShop.ProjectID)
	inOther := ts.createConfiguration(ts.configurationBody("other", `"projectId":%q`, other.ID))

	ts.Equal([]string{inShop.ID}, ts.configurationIDs("?projectId="+url.QueryEscape(shop.ID)))
	ts.Equal([]string{inOther.ID}, ts.configurationIDs("?projectId="+url.QueryEscape(other.ID)))

	var fetched projectConfiguration
	ts.Require().Equal(http.StatusOK, ts.getJSON(configurationsPath+"/"+inShop.ID, &fetched))
	ts.Equal(shop.ID, fetched.ProjectID)
}

// A configuration in a project is in the system organization unit, and no other.
func (ts *ProjectAPITestSuite) TestCredentialConfigurationInProjectOrganizationUnit() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("vci-ou-shop")))
	childOUID := ts.createChildOU("vci-ou-child", defaultOUID)

	status, body := ts.do(http.MethodPost, configurationsPath,
		ts.configurationBody("ou-child", `"projectId":%q,"ouId":%q`, shop.ID, childOUID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("VCI-2014", codeOf(ts.T(), body))
}

// An update keeps a configuration in its project unless it names another one, moves it between projects, and
// refuses a project that does not exist.
func (ts *ProjectAPITestSuite) TestUpdateCredentialConfigurationWithProject() {
	const unknownProjectID = "019a0000-0000-7000-8000-00000000dead"
	first := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("vci-upd-first")))
	second := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("vci-upd-second")))
	created := ts.createConfiguration(ts.configurationBody("updated", `"projectId":%q`, first.ID))

	updated := ts.updateConfiguration(created.ID, ts.configurationBody("updated", `"name":%q`, "Renamed"))
	ts.Equal(first.ID, updated.ProjectID, "an update without a project keeps it")

	updated = ts.updateConfiguration(created.ID, ts.configurationBody("updated", `"projectId":%q`, second.ID))
	ts.Equal(second.ID, updated.ProjectID)
	ts.Empty(ts.configurationIDs("?projectId=" + url.QueryEscape(first.ID)))

	status, body := ts.do(http.MethodPut, configurationsPath+"/"+created.ID,
		ts.configurationBody("updated", `"projectId":%q`, unknownProjectID))
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Equal("VCI-2013", codeOf(ts.T(), body))
}

// A project that still has a credential configuration is not deleted until the configuration is gone.
func (ts *ProjectAPITestSuite) TestDeleteProjectWithCredentialConfigurationIsRefused() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Busy"}`, ts.handle("vci-busy")))
	configuration := ts.createConfiguration(ts.configurationBody("busy", `"projectId":%q`, created.ID))

	status, body := ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusConflict, status, body)
	ts.Equal("PRJ-1013", codeOf(ts.T(), body))

	status, body = ts.do(http.MethodDelete, configurationsPath+"/"+configuration.ID, "")
	ts.Require().Equal(http.StatusNoContent, status, body)
	status, body = ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusNoContent, status, body)
}

// An exported configuration keeps its project, so a deploy of the project carries it.
func (ts *ProjectAPITestSuite) TestExportedCredentialConfigurationKeepsItsProject() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("vci-export-shop")))
	created := ts.createConfiguration(ts.configurationBody("export", `"projectId":%q`, shop.ID))

	status, resp := ts.do(http.MethodPost, "/export", fmt.Sprintf(`{"credentialConfigurations":[%q]}`, created.ID))
	ts.Require().Equal(http.StatusOK, status, resp)
	var exported struct {
		Resources string `json:"resources"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(resp), &exported))
	ts.Contains(exported.Resources, "projectId: "+shop.ID)
}

// configurationBody builds a credential configuration request, with the given fields spliced in.
func (ts *ProjectAPITestSuite) configurationBody(name, fields string, args ...interface{}) string {
	return fmt.Sprintf(`{"handle":%q,"vct":"urn:example:pid:1",%s}`, ts.handle("vci-"+name),
		fmt.Sprintf(fields, args...))
}

// createConfiguration posts a credential configuration, requires a 201, and records it for cleanup.
func (ts *ProjectAPITestSuite) createConfiguration(body string) projectConfiguration {
	status, resp := ts.do(http.MethodPost, configurationsPath, body)
	ts.Require().Equal(http.StatusCreated, status, resp)
	var created projectConfiguration
	ts.Require().NoError(json.Unmarshal([]byte(resp), &created))
	ts.credentialConfigurations = append(ts.credentialConfigurations, created.ID)
	return created
}

// updateConfiguration puts a credential configuration and requires a 200.
func (ts *ProjectAPITestSuite) updateConfiguration(id, body string) projectConfiguration {
	status, resp := ts.do(http.MethodPut, configurationsPath+"/"+id, body)
	ts.Require().Equal(http.StatusOK, status, resp)
	var updated projectConfiguration
	ts.Require().NoError(json.Unmarshal([]byte(resp), &updated))
	return updated
}

// configurationIDs lists credential configurations with the given query and returns their IDs.
func (ts *ProjectAPITestSuite) configurationIDs(query string) []string {
	var list []projectConfiguration
	ts.Require().Equal(http.StatusOK, ts.getJSON(configurationsPath+query, &list))
	ids := []string{}
	for _, configuration := range list {
		ids = append(ids, configuration.ID)
	}
	return ids
}
