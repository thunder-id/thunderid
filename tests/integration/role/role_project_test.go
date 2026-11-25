// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// projectSystemOUID is the system organization unit the bootstrap declares.
	projectSystemOUID = "01900000-0000-7000-8000-000000000001"
	// declarativeProjectRoleID is the declarative role in a project, loaded from
	// tests/integration/resources/declarative_resources/roles/role-declarative-project.yaml.
	declarativeProjectRoleID = "decl-role-project"
	declarativeRoleProjectID = "019a0000-0000-7000-8000-0000000d0008"
	// projectRoleDeclarativeOUID is the declarative organization unit a runtime role of the project is created in.
	projectRoleDeclarativeOUID = "decl-ou-1"
)

// projectRoleSummary is the part of a role response these tests read.
type projectRoleSummary struct {
	ID         string `json:"id"`
	OUID       string `json:"ouId"`
	ProjectID  string `json:"projectId"`
	IsReadOnly bool   `json:"isReadOnly"`
}

// RoleProjectTestSuite covers project IDs on a server that serves no projects, where a project ID is kept
// without being checked, together with the composite store that merges declarative and runtime roles.
type RoleProjectTestSuite struct {
	suite.Suite
	suffix  string
	created []string
}

func TestRoleProjectTestSuite(t *testing.T) {
	suite.Run(t, new(RoleProjectTestSuite))
}

func (suite *RoleProjectTestSuite) SetupSuite() {
	suite.suffix = fmt.Sprintf("%d", time.Now().UnixNano())
}

func (suite *RoleProjectTestSuite) TearDownTest() {
	for _, id := range suite.created {
		_ = testutils.DeleteRole(id)
	}
	suite.created = nil
}

func (suite *RoleProjectTestSuite) TestDeclarativeRoleInProject() {
	var role projectRoleSummary
	suite.Require().Equal(http.StatusOK, suite.getJSON("/roles/"+declarativeProjectRoleID, &role))

	suite.Equal(projectSystemOUID, role.OUID)
	suite.Equal(declarativeRoleProjectID, role.ProjectID)
}

// A project lists both its declarative and its runtime roles, narrowed by an organization unit filter.
func (suite *RoleProjectTestSuite) TestListByProjectMergesDeclarativeAndRuntime() {
	inSystem := suite.createRole(fmt.Sprintf(`{"name":%q,"projectId":%q,"permissions":[]}`,
		"Project Role "+suite.suffix, declarativeRoleProjectID))
	suite.Equal(projectSystemOUID, inSystem.OUID)
	suite.Equal(declarativeRoleProjectID, inSystem.ProjectID, "a server without projects keeps the project ID")
	inDeclOU := suite.createRole(fmt.Sprintf(`{"name":%q,"ouId":%q,"projectId":%q,"permissions":[]}`,
		"Project Role In OU "+suite.suffix, projectRoleDeclarativeOUID, declarativeRoleProjectID))
	suite.Equal(projectRoleDeclarativeOUID, inDeclOU.OUID)

	project := "?limit=100&projectId=" + url.QueryEscape(declarativeRoleProjectID)
	suite.ElementsMatch([]string{declarativeProjectRoleID, inSystem.ID, inDeclOU.ID}, suite.listIDs(project))
	suite.ElementsMatch([]string{declarativeProjectRoleID, inSystem.ID},
		suite.listIDs(project+projectRoleOUFilter(projectSystemOUID)))
	suite.Equal([]string{inDeclOU.ID}, suite.listIDs(project+projectRoleOUFilter(projectRoleDeclarativeOUID)))

	status, body := suite.do(http.MethodGet, "/roles?limit=1&offset=2&projectId="+
		url.QueryEscape(declarativeRoleProjectID), "")
	suite.Require().Equal(http.StatusOK, status, body)
	var page struct {
		TotalResults int                  `json:"totalResults"`
		Roles        []projectRoleSummary `json:"roles"`
	}
	suite.Require().NoError(json.Unmarshal([]byte(body), &page))
	suite.Equal(3, page.TotalResults, "the count spans both stores")
	suite.Len(page.Roles, 1, "the merged list pages across both stores")
}

// An update that names no project or organization unit keeps both.
func (suite *RoleProjectTestSuite) TestUpdateKeepsProjectAndOrganizationUnit() {
	role := suite.createRole(fmt.Sprintf(`{"name":%q,"projectId":%q,"permissions":[]}`,
		"Kept Role "+suite.suffix, declarativeRoleProjectID))

	status, body := suite.do(http.MethodPut, "/roles/"+role.ID,
		fmt.Sprintf(`{"name":%q,"permissions":[]}`, "Kept Role Renamed "+suite.suffix))
	suite.Require().Equal(http.StatusOK, status, body)
	var updated projectRoleSummary
	suite.Require().NoError(json.Unmarshal([]byte(body), &updated))

	suite.Equal(declarativeRoleProjectID, updated.ProjectID)
	suite.Equal(projectSystemOUID, updated.OUID)
}

func (suite *RoleProjectTestSuite) TestListWithEmptyProjectID() {
	suite.Empty(suite.listIDs("?projectId="))
}

func (suite *RoleProjectTestSuite) TestListByProjectWithOtherFilter() {
	status, body := suite.do(http.MethodGet, "/roles?projectId="+url.QueryEscape(declarativeRoleProjectID)+
		"&filter="+url.QueryEscape(`name eq "admin"`), "")

	suite.Equal(http.StatusBadRequest, status, body)
	var errResp struct {
		Code string `json:"code"`
	}
	suite.Require().NoError(json.Unmarshal([]byte(body), &errResp))
	suite.Equal("ROL-1033", errResp.Code)
}

// createRole posts a role, requires a 201, and records it for cleanup.
func (suite *RoleProjectTestSuite) createRole(body string) projectRoleSummary {
	status, resp := suite.do(http.MethodPost, "/roles", body)
	suite.Require().Equal(http.StatusCreated, status, resp)
	var created projectRoleSummary
	suite.Require().NoError(json.Unmarshal([]byte(resp), &created))
	suite.created = append(suite.created, created.ID)
	return created
}

// listIDs lists roles with the given query, requires a 200, and returns their IDs.
func (suite *RoleProjectTestSuite) listIDs(query string) []string {
	var list struct {
		Roles []projectRoleSummary `json:"roles"`
	}
	suite.Require().Equal(http.StatusOK, suite.getJSON("/roles"+query, &list))
	ids := []string{}
	for _, role := range list.Roles {
		ids = append(ids, role.ID)
	}
	return ids
}

// getJSON reads a path and decodes a 200 response into out.
func (suite *RoleProjectTestSuite) getJSON(path string, out interface{}) int {
	status, body := suite.do(http.MethodGet, path, "")
	if status == http.StatusOK {
		suite.Require().NoError(json.Unmarshal([]byte(body), out))
	}
	return status
}

// do sends a request with a JSON body and returns the status and body.
func (suite *RoleProjectTestSuite) do(method, path, body string) (int, string) {
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, string(respBody)
}

// projectRoleOUFilter builds the query parameter that narrows a role listing to an organization unit.
func projectRoleOUFilter(ouID string) string {
	return "&filter=" + url.QueryEscape(fmt.Sprintf("ouId eq %q", ouID))
}
