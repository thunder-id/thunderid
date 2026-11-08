// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// projectGroupSystemOUID is the system organization unit the bootstrap declares.
	projectGroupSystemOUID = "01900000-0000-7000-8000-000000000001"
	// declarativeProjectGroupID is the declarative group in a project, loaded from
	// tests/integration/resources/declarative_resources/groups/group-declarative-project.yaml.
	declarativeProjectGroupID = "decl-group-project"
	declarativeGroupProjectID = "019a0000-0000-7000-8000-0000000d0009"
)

// projectGroupSummary is the part of a group response these tests read.
type projectGroupSummary struct {
	ID         string `json:"id"`
	OUID       string `json:"ouId"`
	ProjectID  string `json:"projectId"`
	IsReadOnly bool   `json:"isReadOnly"`
}

// GroupProjectTestSuite covers project IDs on a server that serves no projects, where a project ID is kept
// without being checked, together with the composite store that merges declarative and runtime groups.
type GroupProjectTestSuite struct {
	suite.Suite
	suffix  string
	created []string
}

func TestGroupProjectTestSuite(t *testing.T) {
	suite.Run(t, new(GroupProjectTestSuite))
}

func (suite *GroupProjectTestSuite) SetupSuite() {
	suite.suffix = fmt.Sprintf("%d", time.Now().UnixNano())
}

func (suite *GroupProjectTestSuite) TearDownTest() {
	for _, id := range suite.created {
		_ = testutils.DeleteGroup(id)
	}
	suite.created = nil
}

func (suite *GroupProjectTestSuite) TestDeclarativeGroupInProject() {
	var group projectGroupSummary
	suite.Require().Equal(http.StatusOK, suite.getJSON("/groups/"+declarativeProjectGroupID, &group))

	suite.Equal(projectGroupSystemOUID, group.OUID)
	suite.Equal(declarativeGroupProjectID, group.ProjectID)
	suite.True(group.IsReadOnly)
}

// A project lists both its declarative and its runtime groups, narrowed by an organization unit filter, and
// pages across both stores.
func (suite *GroupProjectTestSuite) TestListByProjectMergesDeclarativeAndRuntime() {
	inSystem := suite.createGroup(fmt.Sprintf(`{"name":%q,"projectId":%q}`,
		"Project Group "+suite.suffix, declarativeGroupProjectID))
	suite.Equal(projectGroupSystemOUID, inSystem.OUID)
	suite.Equal(declarativeGroupProjectID, inSystem.ProjectID, "a server without projects keeps the project ID")

	project := "?limit=100&projectId=" + url.QueryEscape(declarativeGroupProjectID)
	suite.ElementsMatch([]string{declarativeProjectGroupID, inSystem.ID}, suite.listIDs(project))
	suite.ElementsMatch([]string{declarativeProjectGroupID, inSystem.ID},
		suite.listIDs(project+projectGroupOUFilter(projectGroupSystemOUID)))
	suite.Empty(suite.listIDs(project + projectGroupOUFilter(declGroupOUID)))

	status, body := suite.do(http.MethodGet, "/groups?limit=1&offset=1&projectId="+
		url.QueryEscape(declarativeGroupProjectID), "")
	suite.Require().Equal(http.StatusOK, status, body)
	var page struct {
		TotalResults int                   `json:"totalResults"`
		Groups       []projectGroupSummary `json:"groups"`
	}
	suite.Require().NoError(json.Unmarshal([]byte(body), &page))
	suite.Equal(2, page.TotalResults, "the count spans both stores")
	suite.Len(page.Groups, 1, "the merged list pages across both stores")
}

// An update that names no project keeps it, and one that names another project moves the group there.
func (suite *GroupProjectTestSuite) TestUpdateKeepsOrMovesTheProject() {
	group := suite.createGroup(fmt.Sprintf(`{"name":%q,"ouId":%q,"projectId":%q}`,
		"Kept Group "+suite.suffix, projectGroupSystemOUID, declarativeGroupProjectID))

	updated := suite.updateGroup(group.ID, fmt.Sprintf(`{"name":%q,"ouId":%q}`,
		"Kept Group Renamed "+suite.suffix, projectGroupSystemOUID))
	suite.Equal(declarativeGroupProjectID, updated.ProjectID)

	const otherProjectID = "019a0000-0000-7000-8000-0000000d0010"
	updated = suite.updateGroup(group.ID, fmt.Sprintf(`{"name":%q,"ouId":%q,"projectId":%q}`,
		"Kept Group Renamed "+suite.suffix, projectGroupSystemOUID, otherProjectID))
	suite.Equal(otherProjectID, updated.ProjectID)
}

// A project ID longer than the column is refused, though this server checks no project.
func (suite *GroupProjectTestSuite) TestTooLongProjectIDIsRefused() {
	status, body := suite.do(http.MethodPost, "/groups", fmt.Sprintf(`{"name":%q,"ouId":%q,"projectId":%q}`,
		"Long Project Group "+suite.suffix, projectGroupSystemOUID, strings.Repeat("p", 37)))

	suite.Equal(http.StatusBadRequest, status, body)
	var errResp struct {
		Code string `json:"code"`
	}
	suite.Require().NoError(json.Unmarshal([]byte(body), &errResp))
	suite.Equal("GRP-1021", errResp.Code)
}

func (suite *GroupProjectTestSuite) TestListWithEmptyProjectID() {
	suite.Empty(suite.listIDs("?projectId="))
}

// createGroup posts a group, requires a 201, and records it for cleanup.
func (suite *GroupProjectTestSuite) createGroup(body string) projectGroupSummary {
	status, resp := suite.do(http.MethodPost, "/groups", body)
	suite.Require().Equal(http.StatusCreated, status, resp)
	var created projectGroupSummary
	suite.Require().NoError(json.Unmarshal([]byte(resp), &created))
	suite.created = append(suite.created, created.ID)
	return created
}

// updateGroup puts a group and requires a 200.
func (suite *GroupProjectTestSuite) updateGroup(id, body string) projectGroupSummary {
	status, resp := suite.do(http.MethodPut, "/groups/"+id, body)
	suite.Require().Equal(http.StatusOK, status, resp)
	var updated projectGroupSummary
	suite.Require().NoError(json.Unmarshal([]byte(resp), &updated))
	return updated
}

// listIDs lists groups with the given query, requires a 200, and returns their IDs.
func (suite *GroupProjectTestSuite) listIDs(query string) []string {
	var list struct {
		Groups []projectGroupSummary `json:"groups"`
	}
	suite.Require().Equal(http.StatusOK, suite.getJSON("/groups"+query, &list))
	ids := []string{}
	for _, group := range list.Groups {
		ids = append(ids, group.ID)
	}
	return ids
}

// getJSON reads a path and decodes a 200 response into out.
func (suite *GroupProjectTestSuite) getJSON(path string, out interface{}) int {
	status, body := suite.do(http.MethodGet, path, "")
	if status == http.StatusOK {
		suite.Require().NoError(json.Unmarshal([]byte(body), out))
	}
	return status
}

// do sends a request with a JSON body and returns the status and body.
func (suite *GroupProjectTestSuite) do(method, path, body string) (int, string) {
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

// projectGroupOUFilter builds the query parameter that narrows a group listing to an organization unit.
func projectGroupOUFilter(ouID string) string {
	return "&filter=" + url.QueryEscape(fmt.Sprintf("ouId eq %q", ouID))
}
