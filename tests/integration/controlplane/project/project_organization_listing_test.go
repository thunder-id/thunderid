// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// Without a project, the resource servers list leaves out those of a project unless includeAll is set.
func (ts *ProjectAPITestSuite) TestOrganizationListOmitsProjectResourceServers() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("rs-org-list")))
	inShop := ts.createResourceServer(ts.resourceServerBody("org-list-shop", `"projectId":%q`, shop.ID))
	inOrganization := ts.createResourceServer(ts.resourceServerBody("org-list-org", `"ouId":%q`, defaultOUID))

	listed := ts.listAllIDs("/resource-servers", "resourceServers", "")
	ts.Contains(listed, inOrganization.ID)
	ts.NotContains(listed, inShop.ID)

	listed = ts.listAllIDs("/resource-servers", "resourceServers", "includeAll=true")
	ts.Contains(listed, inOrganization.ID)
	ts.Contains(listed, inShop.ID)

	// A page past the end lists none, but still counts every resource server outside projects.
	var beyond struct {
		TotalResults    int               `json:"totalResults"`
		ResourceServers []json.RawMessage `json:"resourceServers"`
	}
	status, body := ts.do(http.MethodGet, "/resource-servers?limit=10&offset=100000", "")
	ts.Require().Equal(http.StatusOK, status, body)
	ts.Require().NoError(json.Unmarshal([]byte(body), &beyond))
	ts.Empty(beyond.ResourceServers)
	ts.Positive(beyond.TotalResults)
}

// Without a project, the groups list leaves out those of a project unless includeAll is set.
func (ts *ProjectAPITestSuite) TestOrganizationListOmitsProjectGroups() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("grp-org-list")))
	inShop := ts.createGroup(ts.groupBody("org-list-shop", `"projectId":%q`, shop.ID))
	inOrganization := ts.createGroup(ts.groupBody("org-list-org", `"ouId":%q`, defaultOUID))

	listed := ts.listAllIDs("/groups", "groups", "")
	ts.Contains(listed, inOrganization.ID)
	ts.NotContains(listed, inShop.ID)

	listed = ts.listAllIDs("/groups", "groups", "includeAll=true")
	ts.Contains(listed, inOrganization.ID)
	ts.Contains(listed, inShop.ID)
}

// Without a project, the users list leaves out those of a project unless includeAll is set.
func (ts *ProjectAPITestSuite) TestOrganizationListOmitsProjectUsers() {
	userType := ts.createProjectUserType("org-list")
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("user-org-list")))
	inShop := ts.createUser(ts.userBody(userType, "org-list-shop", `"projectId":%q`, shop.ID))
	inOrganization := ts.createUser(ts.userBody(userType, "org-list-org", `"ouId":%q`, defaultOUID))

	listed := ts.listAllIDs("/users", "users", "")
	ts.Contains(listed, inOrganization.ID)
	ts.NotContains(listed, inShop.ID)

	listed = ts.listAllIDs("/users", "users", "includeAll=true")
	ts.Contains(listed, inOrganization.ID)
	ts.Contains(listed, inShop.ID)
}

// Without a project, the roles list leaves out those of a project unless includeAll is set.
func (ts *ProjectAPITestSuite) TestOrganizationListOmitsProjectRoles() {
	shop := ts.create(fmt.Sprintf(`{"handle":%q,"displayName":"Shop"}`, ts.handle("role-org-list")))
	inShop := ts.createRole(ts.roleBody("org-list-shop", `"projectId":%q`, shop.ID))
	inOrganization := ts.createRole(ts.roleBody("org-list-org", `"ouId":%q`, defaultOUID))

	listed := ts.listAllIDs("/roles", "roles", "")
	ts.Contains(listed, inOrganization.ID)
	ts.NotContains(listed, inShop.ID)

	listed = ts.listAllIDs("/roles", "roles", "includeAll=true")
	ts.Contains(listed, inOrganization.ID)
	ts.Contains(listed, inShop.ID)
}

// declaredListingResources declares a user, a group, a role and a resource server in the Default project,
// and one of each outside any project.
const declaredListingResources = `resource_type: user_type
id: cp-decl-list-user-type
handle: cp-decl-list-user-type
displayName: Declared Listing Users
ouId: ` + defaultOUID + `
schema: |
  {"username": {"type": "string", "unique": true}}
---
resource_type: user
id: cp-decl-list-user-org
type: cp-decl-list-user-type
ouId: ` + defaultOUID + `
attributes:
  username: cp-decl-list-user-org
---
resource_type: user
id: cp-decl-list-user-project
type: cp-decl-list-user-type
ouId: ` + defaultOUID + `
projectId: ` + defaultProjectID + `
attributes:
  username: cp-decl-list-user-project
---
resource_type: group
id: cp-decl-list-group-org
name: Declared Listing Group
ouId: ` + defaultOUID + `
---
resource_type: group
id: cp-decl-list-group-project
name: Declared Listing Project Group
ouId: ` + defaultOUID + `
projectId: ` + defaultProjectID + `
---
resource_type: role
id: cp-decl-list-role-org
name: Declared Listing Role
ouId: ` + defaultOUID + `
permissions: []
---
resource_type: role
id: cp-decl-list-role-project
name: Declared Listing Project Role
ouId: ` + defaultOUID + `
projectId: ` + defaultProjectID + `
permissions: []
---
resource_type: resource_server
id: cp-decl-list-rs-org
name: Declared Listing Resource Server
identifier: https://cp-decl-list-rs-org.example.com
ouId: ` + defaultOUID + `
---
resource_type: resource_server
id: cp-decl-list-rs-project
name: Declared Listing Project Resource Server
identifier: https://cp-decl-list-rs-project.example.com
ouId: ` + defaultOUID + `
projectId: ` + defaultProjectID + `
`

// Declared resources are listed the same way: the organization lists leave out those of a project unless
// includeAll is set.
func (ts *ProjectAPITestSuite) TestOrganizationListOmitsDeclaredProjectResources() {
	dir := ts.T().TempDir()
	deployment, err := os.ReadFile(filepath.Join(testutils.GetExtractedProductHome(), "deployment.yaml"))
	ts.Require().NoError(err)
	snapshot := filepath.Join(dir, "deployment.yaml")
	ts.Require().NoError(os.WriteFile(snapshot, deployment, 0o600))
	resourcesPath := filepath.Join(dir, "resources.yaml")
	ts.Require().NoError(os.WriteFile(resourcesPath, []byte(declaredListingResources), 0o600))
	defer func() {
		ts.Assert().NoError(testutils.UpdateDeploymentConfig(snapshot), "the deployment must be put back")
		ts.Assert().NoError(testutils.RestartServer(), "the Control Plane must start again on the deployment it had")
	}()
	ts.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{
		"declarative_resources": map[string]interface{}{"enabled": true},
	}))
	ts.Require().NoError(testutils.RestartServerWithResourcesFile(resourcesPath))

	for _, list := range []struct{ path, itemsKey, prefix string }{
		{"/users", "users", "cp-decl-list-user-"},
		{"/groups", "groups", "cp-decl-list-group-"},
		{"/roles", "roles", "cp-decl-list-role-"},
		{"/resource-servers", "resourceServers", "cp-decl-list-rs-"},
	} {
		listed := ts.listAllIDs(list.path, list.itemsKey, "")
		ts.Contains(listed, list.prefix+"org", list.path)
		ts.NotContains(listed, list.prefix+"project", list.path)

		listed = ts.listAllIDs(list.path, list.itemsKey, "includeAll=true")
		ts.Contains(listed, list.prefix+"org", list.path)
		ts.Contains(listed, list.prefix+"project", list.path)
	}

	// A filter keeps to the users outside projects too.
	for username, want := range map[string][]string{
		"cp-decl-list-user-org":     {"cp-decl-list-user-org"},
		"cp-decl-list-user-project": {},
	} {
		filter := "filter=" + url.QueryEscape(fmt.Sprintf("username eq %q", username))
		ts.ElementsMatch(want, ts.listAllIDs("/users", "users", filter), username)
	}
}

// listAllIDs pages through a list endpoint and returns the IDs of every item under itemsKey. Each page
// with more to list must have a next link that carries the extra query, so following the links keeps the
// same listing. Small pages make sure the links are checked.
func (ts *ProjectAPITestSuite) listAllIDs(path, itemsKey, extraQuery string) []string {
	const pageSize = 10
	ids := []string{}
	for offset := 0; ; offset += pageSize {
		query := fmt.Sprintf("?limit=%d&offset=%d", pageSize, offset)
		if extraQuery != "" {
			query += "&" + extraQuery
		}
		var page struct {
			TotalResults int                          `json:"totalResults"`
			Links        []struct{ Href, Rel string } `json:"links"`
		}
		status, body := ts.do(http.MethodGet, path+query, "")
		ts.Require().Equal(http.StatusOK, status, body)
		ts.Require().NoError(json.Unmarshal([]byte(body), &page))
		var items map[string]json.RawMessage
		ts.Require().NoError(json.Unmarshal([]byte(body), &items))
		var listed []struct {
			ID string `json:"id"`
		}
		ts.Require().NoError(json.Unmarshal(items[itemsKey], &listed))
		for _, item := range listed {
			ids = append(ids, item.ID)
		}
		if len(listed) == 0 || offset+pageSize >= page.TotalResults {
			return ids
		}
		next := ""
		for _, link := range page.Links {
			if link.Rel == "next" {
				next = link.Href
			}
		}
		ts.Require().NotEmpty(next, "a page with more to list must link to the next one")
		ts.True(strings.Contains(next, extraQuery), next)
	}
}
