// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package project contains integration tests for the project management API and MCP tools. Only the
// Control Plane serves them, so the package runs under make test_integration_cp.
package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

type project struct {
	ID          string    `json:"id"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"displayName"`
	Description string    `json:"description,omitempty"`
	RootOUID    string    `json:"rootOuId"`
	CreatedAt   time.Time `json:"createdAt"`
}

type projectList struct {
	TotalResults int       `json:"totalResults"`
	Count        int       `json:"count"`
	Projects     []project `json:"projects"`
}

type organizationUnit struct {
	ID     string  `json:"id"`
	Handle string  `json:"handle"`
	Name   string  `json:"name"`
	Parent *string `json:"parent"`
}

type errorResponse struct {
	Code string `json:"code"`
}

type ProjectAPITestSuite struct {
	suite.Suite
	// suffix keeps this run's handles apart from leftovers of an interrupted earlier run.
	suffix   string
	projects []string
	childOUs []string
}

func TestProjectAPITestSuite(t *testing.T) {
	suite.Run(t, new(ProjectAPITestSuite))
}

func (ts *ProjectAPITestSuite) SetupSuite() {
	ts.suffix = fmt.Sprintf("%d", time.Now().UnixNano())
}

// TearDownTest removes what a test created, children first so their projects can be deleted.
func (ts *ProjectAPITestSuite) TearDownTest() {
	for _, id := range ts.childOUs {
		_ = testutils.DeleteOrganizationUnit(id)
	}
	for _, id := range ts.projects {
		ts.do(http.MethodDelete, "/projects/"+id, "")
	}
	ts.projects, ts.childOUs = nil, nil
}

func (ts *ProjectAPITestSuite) handle(name string) string {
	return "prj-" + name + "-" + ts.suffix
}

func (ts *ProjectAPITestSuite) TestCreateProjectCreatesItsRootOU() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop","description":"Online shop"}`,
		ts.handle("shop")))

	ts.NotEmpty(created.ID)
	ts.NotEqual(created.ID, created.RootOUID)
	ts.WithinDuration(time.Now(), created.CreatedAt, time.Minute)

	var rootOU organizationUnit
	ts.Equal(http.StatusOK, ts.getJSON("/organization-units/"+created.RootOUID, &rootOU))
	ts.Nil(rootOU.Parent)
	ts.Equal(ts.handle("shop"), rootOU.Handle)
	ts.Equal("Shop", rootOU.Name)

	var fetched project
	ts.Equal(http.StatusOK, ts.getJSON("/projects/"+created.ID, &fetched))
	ts.Equal(created, fetched)
}

func (ts *ProjectAPITestSuite) TestCreateProjectKeepsSuppliedID() {
	const id = "019a0000-0000-7000-8000-00000000c0de"
	ts.do(http.MethodDelete, "/projects/"+id, "")

	created := ts.create(fmt.Sprintf(`{"id":%q,"handle":%q,"displayName":"External"}`, id, ts.handle("ext")))
	ts.Equal(id, created.ID)

	status, body := ts.do(http.MethodPost, "/projects",
		fmt.Sprintf(`{"id":%q,"handle":%q,"displayName":"Again"}`, id, ts.handle("ext-again")))
	ts.Equal(http.StatusConflict, status)
	ts.Equal("PRJ-1006", codeOf(ts.T(), body))
}

func (ts *ProjectAPITestSuite) TestCreateProjectWithUsedHandleConflicts() {
	ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"First"}`, ts.handle("dup")))

	status, body := ts.do(http.MethodPost, "/projects",
		fmt.Sprintf(`{"handle":%q,"displayName":"Second"}`, ts.handle("dup")))

	// The handle is the root organization unit's, so the root OU handle conflict reports it.
	ts.Equal(http.StatusConflict, status)
	ts.Equal("OU-1008", codeOf(ts.T(), body))
}

func (ts *ProjectAPITestSuite) TestCreateProjectValidation() {
	cases := map[string]struct {
		body string
		code string
	}{
		"invalid handle": {`{"handle":"not/valid","displayName":"X"}`, "OU-1001"},
		"missing name":   {fmt.Sprintf(`{"handle":%q}`, ts.handle("noname")), "PRJ-1003"},
		"non uuid id":    {fmt.Sprintf(`{"id":"abc","handle":%q,"displayName":"X"}`, ts.handle("badid")), "PRJ-1005"},
		"malformed body": {`{`, "PRJ-1001"},
	}
	for name, tc := range cases {
		ts.Run(name, func() {
			status, body := ts.do(http.MethodPost, "/projects", tc.body)
			ts.Equal(http.StatusBadRequest, status)
			ts.Equal(tc.code, codeOf(ts.T(), body))
		})
	}

	status, body := ts.do(http.MethodGet, "/projects?limit=0", "")
	ts.Equal(http.StatusBadRequest, status)
	ts.Equal("PRJ-1008", codeOf(ts.T(), body))
}

func (ts *ProjectAPITestSuite) TestListProjects() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Listed"}`, ts.handle("listed")))

	var list projectList
	ts.Equal(http.StatusOK, ts.getJSON("/projects?limit=100", &list))
	ts.GreaterOrEqual(list.TotalResults, 1)

	found := false
	for _, p := range list.Projects {
		if p.ID == created.ID {
			found = true
			ts.Equal(created.RootOUID, p.RootOUID)
		}
	}
	ts.True(found, "created project missing from the list")
}

func (ts *ProjectAPITestSuite) TestUpdateProject() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Before"}`, ts.handle("upd")))

	status, body := ts.do(http.MethodPut, "/projects/"+created.ID, `{"displayName":"After","description":"New"}`)
	ts.Require().Equal(http.StatusOK, status, body)
	var updated project
	ts.Require().NoError(json.Unmarshal([]byte(body), &updated))
	ts.Equal("After", updated.DisplayName)
	ts.Equal(created.Handle, updated.Handle)

	// The display name and description are the root organization unit's, so the OU shows the update.
	var rootOU organizationUnit
	ts.Require().Equal(http.StatusOK, ts.getJSON("/organization-units/"+created.RootOUID, &rootOU))
	ts.Equal("After", rootOU.Name)

	status, body = ts.do(http.MethodPut, "/projects/"+created.ID,
		fmt.Sprintf(`{"handle":%q,"displayName":"After"}`, ts.handle("other")))
	ts.Equal(http.StatusBadRequest, status)
	ts.Equal("PRJ-1007", codeOf(ts.T(), body))

	status, _ = ts.do(http.MethodPut, "/projects/019a0000-0000-7000-8000-00000000dead", `{"displayName":"X"}`)
	ts.Equal(http.StatusNotFound, status)
}

func (ts *ProjectAPITestSuite) TestDeleteProjectRemovesRootOU() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Deleted"}`, ts.handle("del")))

	status, _ := ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusNoContent, status)

	ts.Equal(http.StatusNotFound, ts.getJSON("/projects/"+created.ID, &project{}))
	ts.Equal(http.StatusNotFound, ts.getJSON("/organization-units/"+created.RootOUID, &organizationUnit{}))

	status, _ = ts.do(http.MethodDelete, "/projects/"+created.ID, "")
	ts.Equal(http.StatusNoContent, status)
}

func (ts *ProjectAPITestSuite) TestRootOUCannotBeDeletedOutsideItsProject() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Guarded"}`, ts.handle("guard")))

	status, body := ts.do(http.MethodDelete, "/organization-units/"+created.RootOUID, "")

	ts.Equal(http.StatusBadRequest, status)
	ts.Equal("OU-1006", codeOf(ts.T(), body))
	ts.Equal(http.StatusOK, ts.getJSON("/projects/"+created.ID, &project{}))
}

func (ts *ProjectAPITestSuite) TestDeleteProjectWithChildOUIsRefused() {
	created := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Parent"}`, ts.handle("parent")))
	childID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.handle("child"), Name: "Child", Parent: &created.RootOUID,
	})
	ts.Require().NoError(err)
	ts.childOUs = append(ts.childOUs, childID)

	status, body := ts.do(http.MethodDelete, "/projects/"+created.ID, "")

	ts.Equal(http.StatusBadRequest, status)
	ts.Equal("OU-1006", codeOf(ts.T(), body))
	ts.Equal(http.StatusOK, ts.getJSON("/projects/"+created.ID, &project{}))
	ts.Equal(http.StatusOK, ts.getJSON("/organization-units/"+created.RootOUID, &organizationUnit{}))
}

// create posts a project, requires a 201, and records it for cleanup.
func (ts *ProjectAPITestSuite) create(body string) project {
	status, resp := ts.do(http.MethodPost, "/projects", body)
	ts.Require().Equal(http.StatusCreated, status, resp)
	var created project
	ts.Require().NoError(json.Unmarshal([]byte(resp), &created))
	ts.projects = append(ts.projects, created.ID)
	return created
}

// getJSON reads a resource into out and returns the status.
func (ts *ProjectAPITestSuite) getJSON(path string, out interface{}) int {
	status, body := ts.do(http.MethodGet, path, "")
	if status == http.StatusOK {
		ts.Require().NoError(json.Unmarshal([]byte(body), out))
	}
	return status
}

// do sends a request with the admin client and returns the status and body.
func (ts *ProjectAPITestSuite) do(method, path, body string) (int, string) {
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	ts.Require().NoError(err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, string(respBody)
}

// codeOf returns the error code of an error response body.
func codeOf(t *testing.T, body string) string {
	t.Helper()
	var e errorResponse
	if err := json.Unmarshal([]byte(body), &e); err != nil {
		t.Fatalf("not an error response: %s", body)
	}
	return e.Code
}
