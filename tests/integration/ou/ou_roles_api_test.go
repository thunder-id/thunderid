// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// OURolesAPITestSuite covers the two role-listing endpoints on organization units,
// GET /organization-units/{id}/roles and GET /organization-units/tree/{path...}/roles.
//
// Roles live in a different database from organization units, so the OU package reaches them
// through the OURoleResolver adapter rather than joining the ROLE table. Both endpoints are
// therefore exercised against a real role fixture, and against a sibling OU, to confirm the
// resolver scopes its count and its page to the requested OU instead of returning every role.
//
// Fixture topology:
//
//	roles-parent-ou (2 roles)
//	└── roles-child-ou (1 role)  ← nested so the path form is exercised on a multi-segment path
//	roles-empty-ou  (0 roles)
type OURolesAPITestSuite struct {
	suite.Suite

	parentOUID string
	childOUID  string
	emptyOUID  string

	parentRoleIDs []string
	childRoleID   string
}

const (
	rolesParentOUHandle = "roles-parent-ou"
	rolesChildOUHandle  = "roles-child-ou"
	rolesEmptyOUHandle  = "roles-empty-ou"

	rolesChildOUPath = rolesParentOUHandle + "/" + rolesChildOUHandle
)

func TestOURolesAPITestSuite(t *testing.T) {
	suite.Run(t, new(OURolesAPITestSuite))
}

func (suite *OURolesAPITestSuite) SetupSuite() {
	parentID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      rolesParentOUHandle,
		Name:        "Roles Parent OU",
		Description: "Parent OU for the OU role listing tests",
	})
	suite.Require().NoError(err, "Failed to create parent OU")
	suite.parentOUID = parentID

	childID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      rolesChildOUHandle,
		Name:        "Roles Child OU",
		Description: "Child OU for the OU role listing tests",
		Parent:      &parentID,
	})
	suite.Require().NoError(err, "Failed to create child OU")
	suite.childOUID = childID

	emptyID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      rolesEmptyOUHandle,
		Name:        "Roles Empty OU",
		Description: "OU deliberately left without roles",
	})
	suite.Require().NoError(err, "Failed to create empty OU")
	suite.emptyOUID = emptyID

	for _, name := range []string{"ou-roles-first", "ou-roles-second"} {
		roleID, err := testutils.CreateRole(testutils.Role{
			Name:        name,
			Description: "Role in the parent OU",
			OUID:        suite.parentOUID,
		})
		suite.Require().NoError(err, "Failed to create role %s", name)
		suite.parentRoleIDs = append(suite.parentRoleIDs, roleID)
	}

	childRoleID, err := testutils.CreateRole(testutils.Role{
		Name:        "ou-roles-child",
		Description: "Role in the child OU",
		OUID:        suite.childOUID,
	})
	suite.Require().NoError(err, "Failed to create child OU role")
	suite.childRoleID = childRoleID
}

func (suite *OURolesAPITestSuite) TearDownSuite() {
	for _, id := range append(suite.parentRoleIDs, suite.childRoleID) {
		if id != "" {
			if err := testutils.DeleteRole(id); err != nil {
				suite.T().Logf("Failed to delete role %s: %v", id, err)
			}
		}
	}
	for _, id := range []string{suite.childOUID, suite.parentOUID, suite.emptyOUID} {
		if id != "" {
			if err := testutils.DeleteOrganizationUnit(id); err != nil {
				suite.T().Logf("Failed to delete OU %s: %v", id, err)
			}
		}
	}
}

// listRoles issues a role listing request and returns the decoded response.
func (suite *OURolesAPITestSuite) listRoles(path string) RoleListResponse {
	suite.T().Helper()

	req, err := http.NewRequest("GET", testServerURL+path, nil)
	suite.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			suite.T().Logf("Failed to close response body: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equalf(http.StatusOK, resp.StatusCode, "unexpected status, body: %s", body)

	var rolesResponse RoleListResponse
	suite.Require().NoError(json.Unmarshal(body, &rolesResponse))
	return rolesResponse
}

// roleIDs collects the IDs from a listing so membership can be asserted independently of order.
func roleIDs(response RoleListResponse) []string {
	ids := make([]string, 0, len(response.Roles))
	for _, role := range response.Roles {
		ids = append(ids, role.ID)
	}
	return ids
}

// TestGetOrganizationUnitRoles verifies the ID-based endpoint lists exactly the OU's own roles.
func (suite *OURolesAPITestSuite) TestGetOrganizationUnitRoles() {
	response := suite.listRoles("/organization-units/" + suite.parentOUID + "/roles")

	suite.Equal(2, response.TotalResults)
	suite.Equal(1, response.StartIndex)
	suite.Equal(len(response.Roles), response.Count)

	ids := roleIDs(response)
	for _, expected := range suite.parentRoleIDs {
		suite.Containsf(ids, expected, "parent OU role %s missing, got %v", expected, ids)
	}
	// The child OU's role must not leak into the parent's listing; the endpoint lists the OU's own
	// roles, not the subtree's.
	suite.NotContainsf(ids, suite.childRoleID,
		"child OU role must not appear in the parent listing, got %v", ids)

	for _, role := range response.Roles {
		suite.NotEmpty(role.Name, "role name must be populated")
		suite.False(role.IsReadOnly, "API-created roles are mutable")
	}
}

// TestGetOrganizationUnitRolesPagination verifies limit and offset are honoured, since the count and
// the page are resolved by two separate calls into the role store.
func (suite *OURolesAPITestSuite) TestGetOrganizationUnitRolesPagination() {
	first := suite.listRoles("/organization-units/" + suite.parentOUID + "/roles?limit=1&offset=0")
	suite.Equal(2, first.TotalResults, "total must report every role, not just the page")
	suite.Equal(1, first.Count)
	suite.Equal(1, first.StartIndex)

	second := suite.listRoles("/organization-units/" + suite.parentOUID + "/roles?limit=1&offset=1")
	suite.Equal(2, second.TotalResults)
	suite.Equal(1, second.Count)
	suite.Equal(2, second.StartIndex)

	suite.NotEqual(roleIDs(first)[0], roleIDs(second)[0],
		"the second page must not repeat the first page's role")
}

// TestGetOrganizationUnitRolesEmpty verifies an OU with no roles returns an empty listing rather
// than every role in the deployment.
func (suite *OURolesAPITestSuite) TestGetOrganizationUnitRolesEmpty() {
	response := suite.listRoles("/organization-units/" + suite.emptyOUID + "/roles")

	suite.Equal(0, response.TotalResults)
	suite.Equal(0, response.Count)
	suite.Empty(response.Roles)
}

// TestGetNonExistentOrganizationUnitRoles verifies the OU is resolved before its roles are listed.
func (suite *OURolesAPITestSuite) TestGetNonExistentOrganizationUnitRoles() {
	req, err := http.NewRequest("GET", testServerURL+"/organization-units/non-existent-id/roles", nil)
	suite.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode)

	var errorResp ErrorResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&errorResp))
	suite.Equal("OU-1003", errorResp.Code)
}

// TestGetOrganizationUnitRolesByPath verifies the handle-path form resolves the same listing,
// exercised on a nested path so the multi-segment case is covered.
func (suite *OURolesAPITestSuite) TestGetOrganizationUnitRolesByPath() {
	response := suite.listRoles("/organization-units/tree/" + rolesChildOUPath + "/roles")

	suite.Equal(1, response.TotalResults)
	suite.Equal(1, response.Count)
	suite.Equal(1, response.StartIndex)
	suite.Equal([]string{suite.childRoleID}, roleIDs(response))
}

// TestGetOrganizationUnitRolesByPathRootHandle verifies the single-segment path form, where the
// "/roles" suffix has to be stripped from a path with nothing preceding the handle.
func (suite *OURolesAPITestSuite) TestGetOrganizationUnitRolesByPathRootHandle() {
	response := suite.listRoles("/organization-units/tree/" + rolesParentOUHandle + "/roles")

	suite.Equal(2, response.TotalResults)
	ids := roleIDs(response)
	for _, expected := range suite.parentRoleIDs {
		suite.Containsf(ids, expected, "parent OU role %s missing, got %v", expected, ids)
	}
}

// TestGetOrganizationUnitRolesByInvalidPath verifies an unresolvable handle path is a 404 rather
// than an empty listing, which would read as "this OU has no roles".
func (suite *OURolesAPITestSuite) TestGetOrganizationUnitRolesByInvalidPath() {
	req, err := http.NewRequest("GET", testServerURL+"/organization-units/tree/nonexistent/roles", nil)
	suite.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusNotFound, resp.StatusCode)

	var errorResp ErrorResponse
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&errorResp))
	suite.Equal("OU-1003", errorResp.Code)
}
