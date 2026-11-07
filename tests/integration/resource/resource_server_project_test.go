// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// systemOUID is the system organization unit the bootstrap declares.
	systemOUID = "01900000-0000-7000-8000-000000000001"
	// declarativeProjectServerID is the declarative resource server in a project, loaded from
	// tests/integration/resources/declarative_resources/resource_servers/resource-declarative-7.yaml.
	declarativeProjectServerID = "decl-rs-7"
	declarativeProjectID       = "019a0000-0000-7000-8000-0000000d0007"
	declarativeOUID            = "decl-ou-1"
)

// ResourceServerProjectTestSuite covers project IDs on a server that serves no projects, where a project ID
// is kept without being checked, together with the composite store that merges declarative and runtime
// resource servers.
type ResourceServerProjectTestSuite struct {
	suite.Suite
	created []string
}

func TestResourceServerProjectTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceServerProjectTestSuite))
}

func (suite *ResourceServerProjectTestSuite) TearDownTest() {
	for _, id := range suite.created {
		_ = deleteResourceServer(id)
	}
	suite.created = nil
}

// A declarative resource server keeps the project and organization unit its file names.
func (suite *ResourceServerProjectTestSuite) TestDeclarativeResourceServerInProject() {
	rs, err := getResourceServer(declarativeProjectServerID)

	suite.Require().NoError(err)
	suite.Equal(systemOUID, rs.OUID)
	suite.Equal(declarativeProjectID, rs.ProjectID)
	suite.True(rs.IsReadOnly)
}

// A project lists both its declarative and its runtime resource servers, narrowed by an organization unit
// filter.
func (suite *ResourceServerProjectTestSuite) TestListByProjectMergesDeclarativeAndRuntime() {
	inSystem := suite.create(CreateResourceServerRequest{
		Name: "Project Runtime Server", ProjectID: declarativeProjectID,
	})
	inDeclOU := suite.create(CreateResourceServerRequest{
		Name: "Project Runtime Server In OU", OUID: declarativeOUID, ProjectID: declarativeProjectID,
	})

	rs, err := getResourceServer(inSystem)
	suite.Require().NoError(err)
	suite.Equal(systemOUID, rs.OUID)
	suite.Equal(declarativeProjectID, rs.ProjectID, "a server without projects keeps the project ID")
	rs, err = getResourceServer(inDeclOU)
	suite.Require().NoError(err)
	suite.Equal(declarativeOUID, rs.OUID)

	project := "?limit=100&projectId=" + url.QueryEscape(declarativeProjectID)
	suite.ElementsMatch([]string{declarativeProjectServerID, inSystem, inDeclOU}, suite.listIDs(project))
	suite.ElementsMatch([]string{declarativeProjectServerID, inSystem}, suite.listIDs(project+ouFilter(systemOUID)))
	suite.Equal([]string{inDeclOU}, suite.listIDs(project+ouFilter(declarativeOUID)))

	status, body := suite.get("/resource-servers?limit=1&offset=2&projectId=" + url.QueryEscape(declarativeProjectID))
	suite.Require().Equal(http.StatusOK, status, body)
	var page ResourceServerListResponse
	suite.Require().NoError(json.Unmarshal([]byte(body), &page))
	suite.Equal(3, page.TotalResults, "the count spans both stores")
	suite.Len(page.ResourceServers, 1, "the merged list pages across both stores")
}

func (suite *ResourceServerProjectTestSuite) TestListWithEmptyProjectID() {
	suite.Empty(suite.listIDs("?projectId="))
}

func (suite *ResourceServerProjectTestSuite) TestListWithMalformedFilter() {
	status, body := suite.get("/resource-servers?filter=" + url.QueryEscape("ouId eq"))

	suite.Equal(http.StatusBadRequest, status, body)
	var errResp struct {
		Code string `json:"code"`
	}
	suite.Require().NoError(json.Unmarshal([]byte(body), &errResp))
	suite.Equal("RES-1027", errResp.Code)
}

// create creates a resource server and records it for cleanup.
func (suite *ResourceServerProjectTestSuite) create(req CreateResourceServerRequest) string {
	id, err := createResourceServer(req)
	suite.Require().NoError(err)
	suite.created = append(suite.created, id)
	return id
}

// listIDs lists resource servers with the given query, requires a 200, and returns their IDs.
func (suite *ResourceServerProjectTestSuite) listIDs(query string) []string {
	status, body := suite.get("/resource-servers" + query)
	suite.Require().Equal(http.StatusOK, status, body)
	var list ResourceServerListResponse
	suite.Require().NoError(json.Unmarshal([]byte(body), &list))
	ids := []string{}
	for _, rs := range list.ResourceServers {
		ids = append(ids, rs.ID)
	}
	return ids
}

// get sends a GET request and returns the status and body.
func (suite *ResourceServerProjectTestSuite) get(path string) (int, string) {
	resp, err := testutils.GetHTTPClient().Get(testServerURL + path)
	suite.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, string(body)
}

// ouFilter builds the query parameter that narrows a listing to an organization unit.
func ouFilter(ouID string) string {
	return "&filter=" + url.QueryEscape(fmt.Sprintf("ouId eq %q", ouID))
}
