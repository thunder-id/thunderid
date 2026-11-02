// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// UserAuthzTestSuite validates that user CRUD operations respect OU-scoped authz.
//
// Permission model:
//
//	system:user      → create, update, delete users
//	system:user:view → list, get users (implied by system:user)
//
// A user-manager living in OU1 holds the system:user permission. The suite
// verifies that:
//
//   - Read operations on users in OU1 are allowed (200)
//   - Read operations on users in OU2 (sibling) are denied (403)
//   - Write operations on users in OU1 are allowed (201/200/204)
//   - Write operations on users in OU2 are denied (403)
//   - Listing users only returns users from the accessible OU
//
// Fixture topology:
//
//	OU1 (handle: authz-user-ou1) ← user-manager and target users belong here
//	OU2 (handle: authz-user-ou2) ← sibling OU with its own target user
type UserAuthzTestSuite struct {
	suite.Suite

	// Admin-created OUs
	userOU1ID string
	userOU2ID string

	// user types (one per OU)
	entityTypeOU1ID string
	entityTypeOU2ID string

	// Test role and users
	userMgrRoleID      string
	userMgrUserID      string
	scopedRSID         string
	targetUserOU1ID    string
	deletableUserOU1ID string
	targetUserOU2ID    string

	// HTTP client carrying the user-manager's system:user scoped token
	userAdminClient *http.Client
}

const (
	userAuthzServerURL = "https://localhost:8095"

	userAuthzOU1Handle = "authz-user-ou1"
	userAuthzOU2Handle = "authz-user-ou2"

	userMgrUsername   = "authz-user-manager"
	userMgrPassword   = "UserMgr@123"
	userMgrRoleName   = "User Admin (user-authz-test)"
	entityTypeOU1Name = "authz-user-type-ou1"
	entityTypeOU2Name = "authz-user-type-ou2"

	userAuthzDevelopClientID    = "CONSOLE"
	userAuthzDevelopRedirectURI = "https://localhost:8095/console"
)

func TestUserAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(UserAuthzTestSuite))
}

// ---------------------------------------------------------------------------
// Suite setup
// ---------------------------------------------------------------------------

func (ts *UserAuthzTestSuite) SetupSuite() {
	// ---- 1. Create the two OUs ----
	ou1ID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      userAuthzOU1Handle,
		Name:        "User Authz Test OU1",
		Description: "Primary OU for user authz integration test",
	})
	ts.Require().NoError(err, "create user-authz OU1")
	ts.userOU1ID = ou1ID

	ou2ID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      userAuthzOU2Handle,
		Name:        "User Authz Test OU2",
		Description: "Sibling OU for user authz integration test",
	})
	ts.Require().NoError(err, "create user-authz OU2")
	ts.userOU2ID = ou2ID

	// ---- 2. Create user types (one per OU) ----
	schemaOU1ID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      entityTypeOU1Name,
		DisplayName: "Authz User Type Ou1",
		OUID:        ts.userOU1ID,
		Schema: map[string]interface{}{
			"username":     map[string]interface{}{"type": "string"},
			"password":     map[string]interface{}{"type": "string", "credential": true},
			"display_name": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "create user type for OU1")
	ts.entityTypeOU1ID = schemaOU1ID

	schemaOU2ID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      entityTypeOU2Name,
		DisplayName: "Authz User Type Ou2",
		OUID:        ts.userOU2ID,
		Schema: map[string]interface{}{
			"display_name": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "create user type for OU2")
	ts.entityTypeOU2ID = schemaOU2ID

	// ---- 3. Create the user-manager in OU1 (needs username+password for token grant) ----
	userMgrID, err := testutils.CreateUser(testutils.User{
		Type: entityTypeOU1Name,
		OUID: ts.userOU1ID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q, "display_name": "User Manager"}`,
			userMgrUsername, userMgrPassword,
		)),
	})
	ts.Require().NoError(err, "create user-manager user")
	ts.userMgrUserID = userMgrID

	// ---- 4. Create target users ----
	targetOU1ID, err := testutils.CreateUser(testutils.User{
		Type:       entityTypeOU1Name,
		OUID:       ts.userOU1ID,
		Attributes: json.RawMessage(`{"username": "authz-target-ou1", "display_name": "Target User OU1"}`),
	})
	ts.Require().NoError(err, "create target user in OU1")
	ts.targetUserOU1ID = targetOU1ID

	deletableID, err := testutils.CreateUser(testutils.User{
		Type:       entityTypeOU1Name,
		OUID:       ts.userOU1ID,
		Attributes: json.RawMessage(`{"username": "authz-deletable-ou1", "display_name": "Deletable User OU1"}`),
	})
	ts.Require().NoError(err, "create deletable user in OU1")
	ts.deletableUserOU1ID = deletableID

	targetOU2ID, err := testutils.CreateUser(testutils.User{
		Type:       entityTypeOU2Name,
		OUID:       ts.userOU2ID,
		Attributes: json.RawMessage(`{"display_name": "Target User OU2"}`),
	})
	ts.Require().NoError(err, "create target user in OU2")
	ts.targetUserOU2ID = targetOU2ID

	// ---- 5. Create a custom resource server declaring the fine-grained system scopes ----
	// The product ships only the root "system" scope; this reproduces "system:user" and
	// "system:usertype:view" so the suite can verify resource-level enforcement when configured.
	const scopedRSIdentifier = "https://authz-test.example.com/user"
	systemRSID, err := testutils.CreateSystemScopedResourceServer(
		ts.userOU1ID, "Authz Test RS (user)", scopedRSIdentifier, "user", "usertype")
	ts.Require().NoError(err, "create scoped resource server")
	ts.scopedRSID = systemRSID

	// ---- 6. Create a role with system:user permission and assign to the user-manager ----
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: userMgrRoleName,
		OUID: ts.userOU1ID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: systemRSID,
				Permissions:      []string{"system:user", "system:usertype:view"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.userMgrUserID, Type: "user"},
		},
	})
	ts.Require().NoError(err, "create user-manager role")
	ts.userMgrRoleID = roleID

	// ---- 7. Obtain a scoped access token for the user-manager ----
	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		userAuthzDevelopClientID,
		userAuthzDevelopRedirectURI,
		"system system:user system:usertype:view",
		userMgrUsername,
		userMgrPassword,
		true,
		"",
		scopedRSIdentifier,
	)
	ts.Require().NoError(err, "obtain user-manager token")
	ts.Require().NotEmpty(tokenResp.AccessToken, "user-manager token must be non-empty")

	ts.userAdminClient = testutils.GetHTTPClientWithToken(tokenResp.AccessToken)
}

// ---------------------------------------------------------------------------
// Suite teardown
// ---------------------------------------------------------------------------

func (ts *UserAuthzTestSuite) TearDownSuite() {
	if ts.userMgrRoleID != "" {
		if err := testutils.DeleteRole(ts.userMgrRoleID); err != nil {
			ts.T().Logf("teardown: delete user-manager role: %v", err)
		}
	}
	if ts.scopedRSID != "" {
		// The scoped resource server owns a nested resource tree, and a plain delete is refused with
		// RES-1006 while those resources exist. Logging that failure left the tree behind in the
		// shared database on every run.
		if err := testutils.DeleteResourceServerWithChildren(ts.scopedRSID); err != nil {
			ts.T().Errorf("teardown: delete scoped resource server: %v", err)
		}
	}
	for _, id := range []string{ts.targetUserOU1ID, ts.deletableUserOU1ID, ts.userMgrUserID} {
		if id != "" {
			if err := testutils.DeleteUser(id); err != nil {
				ts.T().Logf("teardown: delete user %s: %v", id, err)
			}
		}
	}
	if ts.targetUserOU2ID != "" {
		if err := testutils.DeleteUser(ts.targetUserOU2ID); err != nil {
			ts.T().Logf("teardown: delete target user in OU2: %v", err)
		}
	}
	if ts.entityTypeOU1ID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeOU1ID); err != nil {
			ts.T().Logf("teardown: delete user type OU1: %v", err)
		}
	}
	if ts.entityTypeOU2ID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeOU2ID); err != nil {
			ts.T().Logf("teardown: delete user type OU2: %v", err)
		}
	}
	if ts.userOU2ID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.userOU2ID); err != nil {
			ts.T().Logf("teardown: delete user-authz OU2: %v", err)
		}
	}
	if ts.userOU1ID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.userOU1ID); err != nil {
			ts.T().Logf("teardown: delete user-authz OU1: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// Helper — issue a request via the user-manager's admin client
// ---------------------------------------------------------------------------

func (ts *UserAuthzTestSuite) doUser(method, path string, body []byte) *http.Response {
	ts.T().Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, userAuthzServerURL+path, bodyReader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := ts.userAdminClient.Do(req)
	ts.Require().NoError(err)
	return resp
}

// ---------------------------------------------------------------------------
// Tests — READ operations (system:user:view implied by system:user)
// ---------------------------------------------------------------------------

// TestListUsers verifies the list contains users from the accessible OU only.
func (ts *UserAuthzTestSuite) TestListUsers() {
	resp := ts.doUser(http.MethodGet, "/users", nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode, "list users should succeed")

	var listResp testutils.UserListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ids := make([]string, 0, len(listResp.Users))
	for _, u := range listResp.Users {
		ids = append(ids, u.ID)
	}

	ts.Containsf(ids, ts.targetUserOU1ID,
		"list must include target user in OU1, got IDs: %v", ids)
	ts.NotContainsf(ids, ts.targetUserOU2ID,
		"list must NOT include target user in OU2 (sibling), got IDs: %v", ids)
}

// TestGetUserInOwnOU verifies the user-manager can read a user in their own OU.
func (ts *UserAuthzTestSuite) TestGetUserInOwnOU() {
	resp := ts.doUser(http.MethodGet, "/users/"+ts.targetUserOU1ID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"user-manager should be able to read a user in their own OU")
}

// TestGetUserInOtherOU verifies the user-manager is denied reading a user in OU2.
func (ts *UserAuthzTestSuite) TestGetUserInOtherOU() {
	resp := ts.doUser(http.MethodGet, "/users/"+ts.targetUserOU2ID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"user-manager must be denied access to a user in a different OU")
}

// ---------------------------------------------------------------------------
// Tests — WRITE operations (system:user)
// ---------------------------------------------------------------------------

// TestCreateUserInOwnOU verifies the user-manager can create a user in their own OU.
func (ts *UserAuthzTestSuite) TestCreateUserInOwnOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"ouId": ts.userOU1ID,
		"type": entityTypeOU1Name,
		"attributes": map[string]interface{}{
			"username":     "authz-created-user",
			"display_name": "Created User",
		},
	})
	ts.Require().NoError(err)

	resp := ts.doUser(http.MethodPost, "/users", payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusCreated, resp.StatusCode,
		"user-manager should be able to create a user in their own OU")

	// Parse the created user ID and clean it up via the admin client.
	var created testutils.User
	if decodeErr := json.NewDecoder(resp.Body).Decode(&created); decodeErr == nil && created.ID != "" {
		if delErr := testutils.DeleteUser(created.ID); delErr != nil {
			ts.T().Logf("cleanup: failed to delete created user %s: %v", created.ID, delErr)
		}
	}
}

// TestCreateUserInOtherOU verifies the user-manager is denied creating a user in OU2.
func (ts *UserAuthzTestSuite) TestCreateUserInOtherOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"ouId": ts.userOU2ID,
		"type": entityTypeOU2Name,
		"attributes": map[string]interface{}{
			"display_name": "Denied User",
		},
	})
	ts.Require().NoError(err)

	resp := ts.doUser(http.MethodPost, "/users", payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"user-manager must not create a user in a different OU")
}

// TestUpdateUserInOwnOU verifies the user-manager can update a user in their own OU.
func (ts *UserAuthzTestSuite) TestUpdateUserInOwnOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"type": entityTypeOU1Name,
		"ouId": ts.userOU1ID,
		"attributes": map[string]interface{}{
			"username":     "authz-target-ou1",
			"display_name": "Updated Display Name",
		},
	})
	ts.Require().NoError(err)

	resp := ts.doUser(http.MethodPut, "/users/"+ts.targetUserOU1ID, payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"user-manager should be able to update a user in their own OU")
}

// TestUpdateUserInOtherOU verifies the user-manager is denied updating a user in OU2.
func (ts *UserAuthzTestSuite) TestUpdateUserInOtherOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"type": entityTypeOU2Name,
		"ouId": ts.userOU2ID,
		"attributes": map[string]interface{}{
			"display_name": "Should Not Update",
		},
	})
	ts.Require().NoError(err)

	resp := ts.doUser(http.MethodPut, "/users/"+ts.targetUserOU2ID, payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"user-manager must not update a user in a different OU")
}

// TestDeleteUserInOwnOU verifies the user-manager can delete a user in their own OU.
func (ts *UserAuthzTestSuite) TestDeleteUserInOwnOU() {
	resp := ts.doUser(http.MethodDelete, "/users/"+ts.deletableUserOU1ID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusNoContent, resp.StatusCode,
		"user-manager should be able to delete a user in their own OU")

	// Clear so TearDownSuite does not attempt a double-delete.
	ts.deletableUserOU1ID = ""
}

// TestDeleteUserInOtherOU verifies the user-manager is denied deleting a user in OU2.
func (ts *UserAuthzTestSuite) TestDeleteUserInOtherOU() {
	resp := ts.doUser(http.MethodDelete, "/users/"+ts.targetUserOU2ID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"user-manager must not delete a user in a different OU")
}

// TestCreateUserByPathRequiresRootPermission pins the authorization boundary of the by-path create
// route, which differs from the direct create above.
//
// `POST /users` is gated by `system:user`, so the user-manager can create in their own OU
// (TestCreateUserInOwnOU). `POST /users/tree/{path...}` has **no entry** in the API permission table
// (`system/security/permissions.go:244-249` lists GET, PUT and DELETE under `/users/**` but no
// POST), and unmatched routes fall back to the **root** system permission
// (`security/service.go:159-166`). The user-manager is therefore refused on this route for their own
// OU as well as for OU2 — the refusal is the root-permission gate, not OU scoping.
//
// Both OUs are asserted deliberately. Testing only OU2 would look like a subtree-scoping test and
// pass for the wrong reason, since the same caller is refused inside their own subtree.
func (ts *UserAuthzTestSuite) TestCreateUserByPathRequiresRootPermission() {
	for _, tc := range []struct {
		name     string
		ouHandle string
		typeName string
		username string
	}{
		{name: "own OU", ouHandle: userAuthzOU1Handle, typeName: entityTypeOU1Name,
			username: "authz-bypath-own-ou"},
		{name: "other OU", ouHandle: userAuthzOU2Handle, typeName: entityTypeOU2Name,
			username: "authz-bypath-other-ou"},
	} {
		ts.Run(tc.name, func() {
			payload, err := json.Marshal(map[string]interface{}{
				"type":       tc.typeName,
				"attributes": map[string]interface{}{"username": tc.username},
			})
			ts.Require().NoError(err)

			resp := ts.doUser(http.MethodPost, "/users/tree/"+tc.ouHandle, payload)
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			ts.Require().NoError(err)
			ts.Require().Equal(http.StatusForbidden, resp.StatusCode, "error body: %s", string(body))

			var errResp testutils.ErrorResponse
			ts.Require().NoError(json.Unmarshal(body, &errResp), "error body: %s", string(body))
			ts.Equal("AUTH-4030", errResp.Code,
				"refusal must come from the route permission gate, not from OU scoping")

			ts.Equal(0, ts.countUsersByUsername(tc.username),
				"a refused by-path create must not persist a user")
		})
	}
}

// countUsersByUsername counts users with the given username using the unrestricted admin client, so
// the check is not itself subject to the scoped caller's visibility.
func (ts *UserAuthzTestSuite) countUsersByUsername(username string) int {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodGet,
		userAuthzServerURL+"/users?filter="+url.QueryEscape(`username eq "`+username+`"`), nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp testutils.UserListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))
	return listResp.TotalResults
}
