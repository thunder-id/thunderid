// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package ouresource contains integration tests for managing resources that belong to an
// organization unit with permissions held in one organization unit.
package ouresource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// OUResourceTestSuite verifies that an administrator holding resource permissions in one
// organization unit manages that unit's resources only.
//
// The product ships only the root "system" scope, so the suite declares the fine-grained system
// permissions on a resource server of its own and grants them to an administrator in OU1:
//
//	Parent (handle: ou-resource-parent)
//	└── OU1 (handle: ou-resource-ou1)  ← administrator belongs here
//	OU2 (handle: ou-resource-ou2)      ← another tree
type OUResourceTestSuite struct {
	suite.Suite

	// Users here live outside the default OU, and a sign-in is bounded to its application's OU, so
	// the administrator signs in through an application in its own OU.
	signIn testutils.OUSignInApps

	parentID   string
	ou1ID      string
	ou2ID      string
	userTypeID string
	adminID    string
	adminRole  string
	rsID       string

	rootClient  *http.Client
	adminClient *http.Client

	cleanups []func()
}

const (
	parentHandle = "ou-resource-parent"
	ou1Handle    = "ou-resource-ou1"
	ou2Handle    = "ou-resource-ou2"

	adminUsername = "ou-resource-admin"
	adminPassword = "OUResource@123"

	rsIdentifier = "https://ou-resource.example.com/system"
)

// adminPermissions are the permissions the administrator holds in OU1, one per resource type the
// suite exercises.
var adminPermissions = []string{
	"system:notificationtemplate",
	"system:app",
	"system:role",
	"system:flow",
	"system:connection",
	"system:rs",
	"system:vc",
	"system:agenttype",
	"system:ou:view",
}

func TestOUResourceTestSuite(t *testing.T) {
	suite.Run(t, new(OUResourceTestSuite))
}

func (ts *OUResourceTestSuite) SetupSuite() {
	ts.rootClient = testutils.GetHTTPClient()

	parent, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: parentHandle, Name: "OU Resource Parent", Description: "Parent of the administrator's OU",
	})
	ts.Require().NoError(err, "create the parent OU")
	ts.parentID = parent

	ou1, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ou1Handle, Name: "OU Resource OU1", Description: "Administrator's OU", Parent: &ts.parentID,
	})
	ts.Require().NoError(err, "create OU1")
	ts.ou1ID = ou1

	ou2, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ou2Handle, Name: "OU Resource OU2", Description: "Sibling OU",
	})
	ts.Require().NoError(err, "create OU2")
	ts.ou2ID = ou2

	userType := testutils.UserType{
		Handle:      "ou-resource-admin-type",
		DisplayName: "OU Resource Admin Type",
		OUID:        ts.ou1ID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	}
	userTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err, "create user type")
	ts.userTypeID = userTypeID

	adminID, err := testutils.CreateUser(testutils.User{
		Type: userType.Handle,
		OUID: ts.ou1ID,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q, "password": %q}`,
			adminUsername, adminPassword)),
	})
	ts.Require().NoError(err, "create administrator")
	ts.adminID = adminID

	children := make([]string, 0, len(adminPermissions))
	for _, p := range adminPermissions {
		children = append(children, strings.SplitN(p, ":", 3)[1])
	}
	rsID, err := testutils.CreateSystemScopedResourceServer(ts.ou1ID, "OU Resource RS", rsIdentifier,
		children...)
	ts.Require().NoError(err, "create resource server")
	ts.rsID = rsID

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "ou-resource-admin-role",
		OUID: ts.ou1ID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: adminPermissions},
		},
		Assignments: []testutils.Assignment{{ID: ts.adminID, Type: "user"}},
	})
	ts.Require().NoError(err, "create administrator role")
	ts.adminRole = roleID

	scope := "system"
	for _, p := range adminPermissions {
		scope += " " + p
	}
	token, err := ts.signIn.ObtainAccessToken(ts.ou1ID, scope, adminUsername, adminPassword, rsIdentifier)
	ts.Require().NoError(err, "obtain administrator token")
	ts.Require().NotEmpty(token.AccessToken)
	ts.adminClient = testutils.GetHTTPClientWithToken(token.AccessToken)
}

func (ts *OUResourceTestSuite) TearDownSuite() {
	for i := len(ts.cleanups) - 1; i >= 0; i-- {
		ts.cleanups[i]()
	}
	if err := ts.signIn.DeleteAll(); err != nil {
		ts.T().Logf("teardown: delete sign-in applications: %v", err)
	}
	if ts.adminRole != "" {
		if err := testutils.DeleteRole(ts.adminRole); err != nil {
			ts.T().Logf("teardown: delete role: %v", err)
		}
	}
	if ts.adminID != "" {
		if err := testutils.DeleteUser(ts.adminID); err != nil {
			ts.T().Logf("teardown: delete administrator: %v", err)
		}
	}
	if ts.rsID != "" {
		if err := testutils.DeleteResourceServerWithChildren(ts.rsID); err != nil {
			ts.T().Logf("teardown: delete resource server: %v", err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("teardown: delete user type: %v", err)
		}
	}
	for _, ouID := range []string{ts.ou2ID, ts.ou1ID, ts.parentID} {
		if ouID == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			ts.T().Logf("teardown: delete OU %s: %v", ouID, err)
		}
	}
}

// cleanup registers a root-client delete of path, run at teardown.
func (ts *OUResourceTestSuite) cleanup(path string) {
	ts.cleanups = append(ts.cleanups, func() {
		status, body := ts.send(ts.rootClient, http.MethodDelete, path, nil)
		if status >= http.StatusBadRequest && status != http.StatusNotFound {
			ts.T().Logf("teardown: delete %s: %d %s", path, status, body)
		}
	})
}

// send issues a request with the given client and returns the status and body.
func (ts *OUResourceTestSuite) send(client *http.Client, method, path string, body interface{}) (int, []byte) {
	ts.T().Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		ts.Require().NoError(err)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, respBody
}

func (ts *OUResourceTestSuite) root(method, path string, body interface{}) (int, []byte) {
	return ts.send(ts.rootClient, method, path, body)
}

func (ts *OUResourceTestSuite) admin(method, path string, body interface{}) (int, []byte) {
	return ts.send(ts.adminClient, method, path, body)
}

// createAsRoot creates a resource with the root client, registers its cleanup and returns its ID.
func (ts *OUResourceTestSuite) createAsRoot(collection string, body interface{}) string {
	ts.T().Helper()
	status, resp := ts.root(http.MethodPost, collection, body)
	ts.Require().Equalf(http.StatusCreated, status, "create %s: %s", collection, resp)
	id := idOf(resp)
	ts.Require().NotEmpty(id)
	ts.cleanup(collection + "/" + id)
	return id
}

// requireStatus asserts the status of a request made with the administrator's token.
func (ts *OUResourceTestSuite) requireStatus(want int, method, path string, body interface{}) []byte {
	ts.T().Helper()
	status, resp := ts.admin(method, path, body)
	ts.Require().Equalf(want, status, "%s %s: %s", method, path, resp)
	return resp
}

func idOf(body []byte) string {
	var resp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &resp)
	return resp.ID
}

// listedIDs collects every "id" found in the array under key in a list response.
func listedIDs(body []byte, key string) map[string]bool {
	var resp map[string]json.RawMessage
	_ = json.Unmarshal(body, &resp)
	var items []struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(resp[key], &items)
	ids := make(map[string]bool, len(items))
	for _, item := range items {
		ids[item.ID] = true
	}
	return ids
}

// requireConfinedToOwnOU checks the administrator reads its own OU's resource, and is refused the other
// OU's resource on read and delete, which leaves that resource in place.
func (ts *OUResourceTestSuite) requireConfinedToOwnOU(itemPath, ownID, otherID string) {
	ts.T().Helper()
	ts.requireStatus(http.StatusOK, http.MethodGet, itemPath+"/"+ownID, nil)
	ts.requireStatus(http.StatusForbidden, http.MethodGet, itemPath+"/"+otherID, nil)
	ts.requireStatus(http.StatusForbidden, http.MethodDelete, itemPath+"/"+otherID, nil)

	status, body := ts.root(http.MethodGet, itemPath+"/"+otherID, nil)
	ts.Equalf(http.StatusOK, status, "the resource must survive the refused delete: %s", body)
}

// requireListedOnlyOwn checks a list response names the own OU's resource and not the other's.
func (ts *OUResourceTestSuite) requireListedOnlyOwn(ids map[string]bool, ownID, otherID string) {
	ts.T().Helper()
	ts.True(ids[ownID], "the list must include the resource in the administrator's OU")
	ts.False(ids[otherID], "the list must leave out the resource in another OU")
}

// listedArrayIDs collects every "id" of a list response that is a bare array.
func listedArrayIDs(body []byte) map[string]bool {
	var items []struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &items)
	ids := make(map[string]bool, len(items))
	for _, item := range items {
		ids[item.ID] = true
	}
	return ids
}
