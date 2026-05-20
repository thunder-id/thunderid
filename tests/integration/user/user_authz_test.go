/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package user

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
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
		Name:               entityTypeOU1Name,
		OUID: ts.userOU1ID,
		Schema: map[string]interface{}{
			"username":     map[string]interface{}{"type": "string"},
			"password":     map[string]interface{}{"type": "string", "credential": true},
			"display_name": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "create user type for OU1")
	ts.entityTypeOU1ID = schemaOU1ID

	schemaOU2ID, err := testutils.CreateUserType(testutils.UserType{
		Name:               entityTypeOU2Name,
		OUID: ts.userOU2ID,
		Schema: map[string]interface{}{
			"display_name": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "create user type for OU2")
	ts.entityTypeOU2ID = schemaOU2ID

	// ---- 3. Create the user-manager in OU1 (needs username+password for token grant) ----
	userMgrID, err := testutils.CreateUser(testutils.User{
		Type:             entityTypeOU1Name,
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
		Type:             entityTypeOU1Name,
		OUID: ts.userOU1ID,
		Attributes:       json.RawMessage(`{"username": "authz-target-ou1", "display_name": "Target User OU1"}`),
	})
	ts.Require().NoError(err, "create target user in OU1")
	ts.targetUserOU1ID = targetOU1ID

	deletableID, err := testutils.CreateUser(testutils.User{
		Type:             entityTypeOU1Name,
		OUID: ts.userOU1ID,
		Attributes:       json.RawMessage(`{"username": "authz-deletable-ou1", "display_name": "Deletable User OU1"}`),
	})
	ts.Require().NoError(err, "create deletable user in OU1")
	ts.deletableUserOU1ID = deletableID

	targetOU2ID, err := testutils.CreateUser(testutils.User{
		Type:             entityTypeOU2Name,
		OUID: ts.userOU2ID,
		Attributes:       json.RawMessage(`{"display_name": "Target User OU2"}`),
	})
	ts.Require().NoError(err, "create target user in OU2")
	ts.targetUserOU2ID = targetOU2ID

	// ---- 5. Look up the system resource server seeded by bootstrap ----
	systemRSID, err := testutils.GetResourceServerByName("System")
	ts.Require().NoError(err, "look up system resource server")

	// ---- 6. Create a role with system:user permission and assign to the user-manager ----
	roleID, err := testutils.CreateRole(testutils.Role{
		Name:               userMgrRoleName,
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
		"type":             entityTypeOU1Name,
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
		"type":             entityTypeOU2Name,
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
		"type":             entityTypeOU1Name,
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
		"type":             entityTypeOU2Name,
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
