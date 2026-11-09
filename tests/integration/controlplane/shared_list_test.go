// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// sharedListPageSize is the page size the lists are read with.
const sharedListPageSize = 100

// sharedListType is one resource type the Control Plane lists by organization unit: the child permission of
// system that lists it, where it is listed, and the resource each unit of the tree holds.
type sharedListType struct {
	name      string
	scope     string
	list      string
	key       string
	ownedByA  string
	ownedByA1 string
}

// The Control Plane does not bound system or its children by organization unit, so a token needs no unit. A token
// with either lists every unit's resources without a filter, and a filter lists what the unit owns, then what is
// shared into it. A owns one
// resource of each type, shared with its children, and its child A1 owns one. Agents are left out: only their
// resource files share them.
func (ts *ControlPlaneTestSuite) TestListsShowWhatEachUnitOwnsAndIsShared() {
	ouA := ts.sharedListOU("shared-list-a", nil)
	ouA1 := ts.sharedListOU("shared-list-a1", &ouA)
	userType := ts.sharedListUserType(ouA)

	types := []sharedListType{
		{name: "users", scope: "system:user", list: "/users", key: "users",
			ownedByA: ts.sharedListUser(userType, ouA), ownedByA1: ts.sharedListUser(userType, ouA1)},
		{name: "applications", scope: "system:app", list: "/applications", key: "applications",
			ownedByA: ts.sharedListApplication(ouA), ownedByA1: ts.sharedListApplication(ouA1)},
		{name: "roles", scope: "system:role", list: "/roles", key: "roles",
			ownedByA: ts.sharedListRole(ouA), ownedByA1: ts.sharedListRole(ouA1)},
		{name: "flows", scope: "system:flow", list: "/flows", key: "flows",
			ownedByA: ts.sharedListFlow(ouA), ownedByA1: ts.sharedListFlow(ouA1)},
		{name: "connections", scope: "system:connection", list: "/connections", key: "connections",
			ownedByA: ts.sharedListConnection(ouA), ownedByA1: ts.sharedListConnection(ouA1)},
	}
	sharePaths := map[string]string{
		"users": "/users/", "applications": "/applications/", "roles": "/roles/", "flows": "/flows/",
		"connections": "/connections/http/",
	}
	for _, tc := range types {
		ts.sharedListShareWithChildUnits(sharePaths[tc.name] + tc.ownedByA)
	}

	systemToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err)
	for _, tc := range types {
		childToken := os.Getenv(testutils.ControlPlaneScopedTokenEnv(tc.scope))
		ts.Require().NotEmpty(childToken, "the harness exports a %s token", tc.scope)
		for scope, token := range map[string]string{"system": systemToken, tc.scope: childToken} {
			ts.Run(tc.name+" with "+scope, func() {
				claims, err := testutils.DecodeJWT(token)
				ts.Require().NoError(err)
				ts.Require().Empty(claims.OUID, "the token names no organization unit")
				client := testutils.GetHTTPClientWithToken(token)

				every := ts.sharedListRead(client, tc, "")
				ts.Contains(every, tc.ownedByA, "the unfiltered list holds A's resource")
				ts.Contains(every, tc.ownedByA1, "the unfiltered list holds A1's resource")

				inA := ts.sharedListRead(client, tc, ouA)
				ts.Require().Contains(inA, tc.ownedByA)
				ts.True(inA[tc.ownedByA], "A lists its own resource as owned")
				ts.NotContains(inA, tc.ownedByA1, "A does not list its child's resource")

				inA1 := ts.sharedListRead(client, tc, ouA1)
				ts.Require().Contains(inA1, tc.ownedByA1)
				ts.True(inA1[tc.ownedByA1], "A1 lists its own resource as owned")
				ts.Require().Contains(inA1, tc.ownedByA, "A1 lists the resource A shares with it")
				ts.False(inA1[tc.ownedByA], "A1 lists A's resource as shared")
			})
		}
	}
}

// sharedListRead reads every page of a type's list, filtered to ouID when it is set, and returns whether each
// listed resource is listed as owned.
func (ts *ControlPlaneTestSuite) sharedListRead(client *http.Client, tc sharedListType, ouID string) map[string]bool {
	ts.T().Helper()
	query := ""
	if ouID != "" {
		query = "&filter=" + url.QueryEscape(fmt.Sprintf("ouId eq %q", ouID))
	}
	listed := map[string]bool{}
	for offset := 0; ; offset += sharedListPageSize {
		status, raw := ts.send(client, http.MethodGet,
			fmt.Sprintf("%s?limit=%d&offset=%d%s", tc.list, sharedListPageSize, offset, query), "", nil)
		ts.Require().Equal(http.StatusOK, status, string(raw))
		var page map[string]json.RawMessage
		ts.Require().NoError(json.Unmarshal(raw, &page), string(raw))
		var total int
		ts.Require().NoError(json.Unmarshal(page["totalResults"], &total), string(raw))
		var items []struct {
			ID          string `json:"id"`
			IsOwnedByOU *bool  `json:"isOwnedByOU"`
		}
		ts.Require().NoError(json.Unmarshal(page[tc.key], &items), string(raw))
		for _, item := range items {
			listed[item.ID] = item.IsOwnedByOU == nil || *item.IsOwnedByOU
		}
		if len(items) < sharedListPageSize || offset+sharedListPageSize >= total {
			return listed
		}
	}
}

// sharedListOU creates an organization unit of the default project, beneath parent when it is set.
func (ts *ControlPlaneTestSuite) sharedListOU(handle string, parent *string) string {
	ts.T().Helper()
	handle = unique(handle)
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: handle, Name: handle, Parent: parent, ProjectID: defaultProjectID,
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { ts.NoError(testutils.DeleteOrganizationUnit(id)) })
	return id
}

// sharedListUserType creates a user type owned by ouID, which its child units use too, and returns its handle.
func (ts *ControlPlaneTestSuite) sharedListUserType(ouID string) string {
	ts.T().Helper()
	handle := unique("shared-list-person")
	id, err := testutils.CreateUserType(testutils.UserType{
		Handle:      handle,
		DisplayName: handle,
		OUID:        ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "required": true, "unique": true},
		},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { ts.NoError(testutils.DeleteUserType(id)) })
	return handle
}

func (ts *ControlPlaneTestSuite) sharedListUser(userType, ouID string) string {
	ts.T().Helper()
	attributes, err := json.Marshal(map[string]any{"username": unique("shared-list-user")})
	ts.Require().NoError(err)
	id, err := testutils.CreateUser(testutils.User{OUID: ouID, Type: userType, Attributes: attributes})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { ts.NoError(testutils.DeleteUser(id)) })
	return id
}

func (ts *ControlPlaneTestSuite) sharedListApplication(ouID string) string {
	ts.T().Helper()
	id, err := testutils.CreateApplication(testutils.Application{
		Name: unique("Shared List App"), OUID: ouID, ProjectID: defaultProjectID,
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { ts.NoError(testutils.DeleteApplication(id)) })
	return id
}

func (ts *ControlPlaneTestSuite) sharedListRole(ouID string) string {
	ts.T().Helper()
	id, err := testutils.CreateRole(testutils.Role{Name: unique("shared-list-role"), OUID: ouID})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { ts.NoError(testutils.DeleteRole(id)) })
	return id
}

func (ts *ControlPlaneTestSuite) sharedListFlow(ouID string) string {
	ts.T().Helper()
	handle := unique("shared-list-flow")
	id, err := testutils.CreateFlow(testutils.Flow{
		Name:      handle,
		FlowType:  "AUTHENTICATION",
		Handle:    handle,
		OUID:      ouID,
		ProjectID: defaultProjectID,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "auth_assert"},
			{"id": "auth_assert", "type": "TASK_EXECUTION",
				"executor": map[string]interface{}{"name": "AuthAssertExecutor"}, "onSuccess": "end"},
			{"id": "end", "type": "END"},
		},
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { ts.NoError(testutils.DeleteFlow(id)) })
	return id
}

func (ts *ControlPlaneTestSuite) sharedListConnection(ouID string) string {
	ts.T().Helper()
	name := unique("shared-list-http")
	var created struct {
		ID string `json:"id"`
	}
	status, raw := ts.call(http.MethodPost, "/connections/http", map[string]any{
		"name": name, "ouId": ouID, "url": "https://" + name + ".example.com/hook",
	}, &created)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.T().Cleanup(func() {
		status, raw := ts.call(http.MethodDelete, "/connections/http/"+created.ID, nil, nil)
		ts.Less(status, http.StatusBadRequest, string(raw))
	})
	return created.ID
}

// sharedListShareWithChildUnits shares the resource at path with every unit beneath its owner, and removes the
// policy before the resource is deleted.
func (ts *ControlPlaneTestSuite) sharedListShareWithChildUnits(path string) {
	ts.T().Helper()
	var policy struct {
		ID string `json:"id"`
	}
	status, raw := ts.call(http.MethodPost, path+"/sharing-policies",
		map[string]any{"childUnits": map[string]any{"mode": "all"}}, &policy)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.T().Cleanup(func() {
		status, raw := ts.call(http.MethodDelete, path+"/sharing-policies/"+policy.ID, nil, nil)
		ts.Less(status, http.StatusBadRequest, string(raw))
	})
}
