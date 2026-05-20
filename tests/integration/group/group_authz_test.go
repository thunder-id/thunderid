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

package group

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

// GroupAuthzTestSuite validates that group CRUD operations respect OU-scoped authz.
//
// Permission model:
//
//	system:group      → create, update, delete groups
//	system:group:view → list, get groups (implied by system:group)
//
// A user-manager living in OU1 holds the system:group permission. The suite
// verifies that:
//
//   - Read operations on groups in OU1 are allowed (200)
//   - Read operations on groups in OU2 (sibling) are denied (403)
//   - Write operations on groups in OU1 are allowed (201/200/204)
//   - Write operations on groups in OU2 are denied (403)
//   - Listing groups only returns groups from the accessible OU
//
// Fixture topology:
//
//	OU1 (handle: authz-group-ou1) ← group-manager and target groups belong here
//	OU2 (handle: authz-group-ou2) ← sibling OU with its own target group
type GroupAuthzTestSuite struct {
	suite.Suite

	// Admin-created OUs
	groupOU1ID string
	groupOU2ID string

	// user types (one for the user-manager in OU1)
	entityTypeOU1ID string

	// Test role and manager
	groupMgrRoleID      string
	groupMgrUserID      string
	targetGroupOU1ID    string
	deletableGroupOU1ID string
	targetGroupOU2ID    string

	// Member users created in each OU to test membership authz
	memberUserOU1ID   string
	memberUserOU2ID   string
	memberSchemaOU2ID string

	// HTTP client carrying the user-manager's system:group scoped token
	groupAdminClient *http.Client
}

const (
	groupAuthzServerURL = "https://localhost:8095"

	groupAuthzOU1Handle = "authz-group-ou1"
	groupAuthzOU2Handle = "authz-group-ou2"

	groupMgrUsername  = "authz-group-manager"
	groupMgrPassword  = "GroupMgr@123"
	groupMgrRoleName  = "Group Admin (group-authz-test)"
	entityTypeOU1Name = "authz-mgr-schema-ou1"

	groupAuthzDevelopClientID    = "CONSOLE"
	groupAuthzDevelopRedirectURI = "https://localhost:8095/console"

	memberSchemaOU2Name = "authz-member-schema-ou2"

	memberOU1Username = "authz-member-ou1"
	memberOU1Password = "MemberOU1@123"
	memberOU2Username = "authz-member-ou2"
	memberOU2Password = "MemberOU2@123"
)

func TestGroupAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(GroupAuthzTestSuite))
}

// ---------------------------------------------------------------------------
// Suite setup
// ---------------------------------------------------------------------------

func (ts *GroupAuthzTestSuite) SetupSuite() {
	// ---- 1. Create the two OUs ----
	ou1ID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      groupAuthzOU1Handle,
		Name:        "Group Authz Test OU1",
		Description: "Primary OU for group authz integration test",
	})
	ts.Require().NoError(err, "create group-authz OU1")
	ts.groupOU1ID = ou1ID

	ou2ID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      groupAuthzOU2Handle,
		Name:        "Group Authz Test OU2",
		Description: "Sibling OU for group authz integration test",
	})
	ts.Require().NoError(err, "create group-authz OU2")
	ts.groupOU2ID = ou2ID

	// ---- 2. Create user type for user-manager in OU1 ----
	schemaOU1ID, err := testutils.CreateUserType(testutils.UserType{
		Name:               entityTypeOU1Name,
		OUID: ts.groupOU1ID,
		Schema: map[string]interface{}{
			"username":     map[string]interface{}{"type": "string"},
			"password":     map[string]interface{}{"type": "string", "credential": true},
			"display_name": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "create user type for OU1")
	ts.entityTypeOU1ID = schemaOU1ID

	// ---- 3. Create the user-manager in OU1 ----
	userMgrID, err := testutils.CreateUser(testutils.User{
		Type:             entityTypeOU1Name,
		OUID: ts.groupOU1ID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q, "display_name": "Group Manager"}`,
			groupMgrUsername, groupMgrPassword,
		)),
	})
	ts.Require().NoError(err, "create group-manager user")
	ts.groupMgrUserID = userMgrID

	// ---- 3b. Create a plain member user in OU1 (used in membership authz tests) ----
	memberOU1ID, err := testutils.CreateUser(testutils.User{
		Type:             entityTypeOU1Name,
		OUID: ts.groupOU1ID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q, "display_name": "Member OU1"}`,
			memberOU1Username, memberOU1Password,
		)),
	})
	ts.Require().NoError(err, "create member user in OU1")
	ts.memberUserOU1ID = memberOU1ID

	// ---- 3c. Create a user type for OU2 ----
	schemaOU2ID, err := testutils.CreateUserType(testutils.UserType{
		Name:               memberSchemaOU2Name,
		OUID: ts.groupOU2ID,
		Schema: map[string]interface{}{
			"username":     map[string]interface{}{"type": "string"},
			"password":     map[string]interface{}{"type": "string", "credential": true},
			"display_name": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "create user type for OU2")
	ts.memberSchemaOU2ID = schemaOU2ID

	// ---- 3d. Create a plain member user in OU2 (used in membership authz tests) ----
	memberOU2ID, err := testutils.CreateUser(testutils.User{
		Type:             memberSchemaOU2Name,
		OUID: ts.groupOU2ID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q, "display_name": "Member OU2"}`,
			memberOU2Username, memberOU2Password,
		)),
	})
	ts.Require().NoError(err, "create member user in OU2")
	ts.memberUserOU2ID = memberOU2ID

	// ---- 4. Create target groups ----
	targetOU1ID, err := testutils.CreateGroup(testutils.Group{
		Name:               "authz-target-ou1",
		Description:        "Target Group OU1",
		OUID: ts.groupOU1ID,
	})
	ts.Require().NoError(err, "create target group in OU1")
	ts.targetGroupOU1ID = targetOU1ID

	deletableID, err := testutils.CreateGroup(testutils.Group{
		Name:               "authz-deletable-ou1",
		Description:        "Deletable Group OU1",
		OUID: ts.groupOU1ID,
	})
	ts.Require().NoError(err, "create deletable group in OU1")
	ts.deletableGroupOU1ID = deletableID

	targetOU2ID, err := testutils.CreateGroup(testutils.Group{
		Name:               "authz-target-ou2",
		Description:        "Target Group OU2",
		OUID: ts.groupOU2ID,
	})
	ts.Require().NoError(err, "create target group in OU2")
	ts.targetGroupOU2ID = targetOU2ID

	// ---- 5. Look up the system resource server seeded by bootstrap ----
	systemRSID, err := testutils.GetResourceServerByName("System")
	ts.Require().NoError(err, "look up system resource server")

	// ---- 6. Create a role with system:group permission and assign to the user-manager ----
	roleID, err := testutils.CreateRole(testutils.Role{
		Name:               groupMgrRoleName,
		OUID: ts.groupOU1ID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: systemRSID,
				Permissions:      []string{"system:ou:view", "system:group", "system:group:view"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.groupMgrUserID, Type: "user"},
		},
	})
	ts.Require().NoError(err, "create group-manager role")
	ts.groupMgrRoleID = roleID

	// ---- 7. Obtain a scoped access token for the user-manager ----
	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		groupAuthzDevelopClientID,
		groupAuthzDevelopRedirectURI,
		"system:ou:view system:group system:group:view",
		groupMgrUsername,
		groupMgrPassword,
		true,
	)
	ts.Require().NoError(err, "obtain group-manager token")
	ts.Require().NotEmpty(tokenResp.AccessToken, "group-manager token must be non-empty")

	ts.groupAdminClient = testutils.GetHTTPClientWithToken(tokenResp.AccessToken)
}

// ---------------------------------------------------------------------------
// Suite teardown
// ---------------------------------------------------------------------------

func (ts *GroupAuthzTestSuite) TearDownSuite() {
	if ts.groupMgrRoleID != "" {
		if err := testutils.DeleteRole(ts.groupMgrRoleID); err != nil {
			ts.T().Logf("teardown: delete group-manager role: %v", err)
		}
	}
	for _, id := range []string{ts.targetGroupOU1ID, ts.deletableGroupOU1ID} {
		if id != "" {
			if err := testutils.DeleteGroup(id); err != nil {
				ts.T().Logf("teardown: delete group %s: %v", id, err)
			}
		}
	}
	if ts.groupMgrUserID != "" {
		if err := testutils.DeleteUser(ts.groupMgrUserID); err != nil {
			ts.T().Logf("teardown: delete user manager: %v", err)
		}
	}
	if ts.memberUserOU1ID != "" {
		if err := testutils.DeleteUser(ts.memberUserOU1ID); err != nil {
			ts.T().Logf("teardown: delete member user OU1: %v", err)
		}
	}
	if ts.memberUserOU2ID != "" {
		if err := testutils.DeleteUser(ts.memberUserOU2ID); err != nil {
			ts.T().Logf("teardown: delete member user OU2: %v", err)
		}
	}
	if ts.targetGroupOU2ID != "" {
		if err := testutils.DeleteGroup(ts.targetGroupOU2ID); err != nil {
			ts.T().Logf("teardown: delete target group in OU2: %v", err)
		}
	}
	if ts.memberSchemaOU2ID != "" {
		if err := testutils.DeleteUserType(ts.memberSchemaOU2ID); err != nil {
			ts.T().Logf("teardown: delete member schema OU2: %v", err)
		}
	}
	if ts.entityTypeOU1ID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeOU1ID); err != nil {
			ts.T().Logf("teardown: delete user type OU1: %v", err)
		}
	}
	if ts.groupOU2ID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.groupOU2ID); err != nil {
			ts.T().Logf("teardown: delete group-authz OU2: %v", err)
		}
	}
	if ts.groupOU1ID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.groupOU1ID); err != nil {
			ts.T().Logf("teardown: delete group-authz OU1: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// Helper — issue a request via the user-manager's admin client
// ---------------------------------------------------------------------------

func (ts *GroupAuthzTestSuite) doGroup(method, path string, body []byte) *http.Response {
	ts.T().Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, groupAuthzServerURL+path, bodyReader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := ts.groupAdminClient.Do(req)
	ts.Require().NoError(err)
	return resp
}

// ---------------------------------------------------------------------------
// Tests — READ operations (system:group:view implied by system:group)
// ---------------------------------------------------------------------------

// TestListGroups verifies the list contains groups from the accessible OU only.
func (ts *GroupAuthzTestSuite) TestListGroups() {
	resp := ts.doGroup(http.MethodGet, "/groups", nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode, "list groups should succeed")

	var listResp GroupListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ids := make([]string, 0, len(listResp.Groups))
	for _, g := range listResp.Groups {
		ids = append(ids, g.Id) // Id (uppercase I lowercase d) based on the testutils structure.
	}

	ts.Containsf(ids, ts.targetGroupOU1ID,
		"list must include target group in OU1, got IDs: %v", ids)
	ts.NotContainsf(ids, ts.targetGroupOU2ID,
		"list must NOT include target group in OU2 (sibling), got IDs: %v", ids)
}

// TestGetGroupInOwnOU verifies the group-manager can read a group in their own OU.
func (ts *GroupAuthzTestSuite) TestGetGroupInOwnOU() {
	resp := ts.doGroup(http.MethodGet, "/groups/"+ts.targetGroupOU1ID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"group-manager should be able to read a group in their own OU")
}

// TestGetGroupInOtherOU verifies the group-manager is denied reading a group in OU2.
func (ts *GroupAuthzTestSuite) TestGetGroupInOtherOU() {
	resp := ts.doGroup(http.MethodGet, "/groups/"+ts.targetGroupOU2ID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"group-manager must be denied access to a group in a different OU")
}

// ---------------------------------------------------------------------------
// Tests — WRITE operations (system:group)
// ---------------------------------------------------------------------------

// TestCreateGroupInOwnOU verifies the group-manager can create a group in their own OU.
func (ts *GroupAuthzTestSuite) TestCreateGroupInOwnOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"ouId": ts.groupOU1ID,
		"name":               "authz-created-group",
		"description":        "Created Group",
	})
	ts.Require().NoError(err)

	resp := ts.doGroup(http.MethodPost, "/groups", payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusCreated, resp.StatusCode,
		"group-manager should be able to create a group in their own OU")

	// Parse the created group ID and clean it up via the admin client.
	var created testutils.Group
	if decodeErr := json.NewDecoder(resp.Body).Decode(&created); decodeErr == nil && created.ID != "" {
		if delErr := testutils.DeleteGroup(created.ID); delErr != nil {
			ts.T().Logf("cleanup: failed to delete created group %s: %v", created.ID, delErr)
		}
	}
}

// TestCreateGroupInOtherOU verifies the group-manager is denied creating a group in OU2.
func (ts *GroupAuthzTestSuite) TestCreateGroupInOtherOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"ouId": ts.groupOU2ID,
		"name":               "authz-denied-group",
		"description":        "Denied Group",
	})
	ts.Require().NoError(err)

	resp := ts.doGroup(http.MethodPost, "/groups", payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"group-manager must not create a group in a different OU")
}

// TestUpdateGroupInOwnOU verifies the group-manager can update a group in their own OU.
func (ts *GroupAuthzTestSuite) TestUpdateGroupInOwnOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"ouId": ts.groupOU1ID,
		"name":               "authz-target-ou1",
		"description":        "Updated Description",
	})
	ts.Require().NoError(err)

	resp := ts.doGroup(http.MethodPut, "/groups/"+ts.targetGroupOU1ID, payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"group-manager should be able to update a group in their own OU")
}

// TestUpdateGroupInOtherOU verifies the group-manager is denied updating a group in OU2.
func (ts *GroupAuthzTestSuite) TestUpdateGroupInOtherOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"ouId": ts.groupOU2ID,
		"name":               "Should Not Update",
	})
	ts.Require().NoError(err)

	resp := ts.doGroup(http.MethodPut, "/groups/"+ts.targetGroupOU2ID, payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"group-manager must not update a group in a different OU")
}

// TestDeleteGroupInOwnOU verifies the group-manager can delete a group in their own OU.
func (ts *GroupAuthzTestSuite) TestDeleteGroupInOwnOU() {
	resp := ts.doGroup(http.MethodDelete, "/groups/"+ts.deletableGroupOU1ID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusNoContent, resp.StatusCode,
		"group-manager should be able to delete a group in their own OU")

	// Clear so TearDownSuite does not attempt a double-delete.
	ts.deletableGroupOU1ID = ""
}

// TestDeleteGroupInOtherOU verifies the group-manager is denied deleting a group in OU2.
func (ts *GroupAuthzTestSuite) TestDeleteGroupInOtherOU() {
	resp := ts.doGroup(http.MethodDelete, "/groups/"+ts.targetGroupOU2ID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"group-manager must not delete a group in a different OU")
}

// ---------------------------------------------------------------------------
// Tests — MEMBER operations (system:group)
// ---------------------------------------------------------------------------

// TestAddMemberFromOtherOU verifies the group-manager is denied adding a user from OU2 to a group in OU1.
// Run before TestAddMemberFromOwnOU / TestRemoveMemberFromGroupInOwnOU (alphabetical order).
func (ts *GroupAuthzTestSuite) TestAddMemberFromOtherOU() {
	payload, err := json.Marshal(MembersRequest{
		Members: []Member{
			{Id: ts.memberUserOU2ID, Type: MemberTypeUser},
		},
	})
	ts.Require().NoError(err)

	resp := ts.doGroup(http.MethodPost, "/groups/"+ts.targetGroupOU1ID+"/members/add", payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"group-manager must not add a user from OU2 to a group in OU1")
}

// TestAddMemberFromOwnOU verifies the group-manager can add a user from OU1 to a group in OU1.
func (ts *GroupAuthzTestSuite) TestAddMemberFromOwnOU() {
	payload, err := json.Marshal(MembersRequest{
		Members: []Member{
			{Id: ts.memberUserOU1ID, Type: MemberTypeUser},
		},
	})
	ts.Require().NoError(err)

	resp := ts.doGroup(http.MethodPost, "/groups/"+ts.targetGroupOU1ID+"/members/add", payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"group-manager should be able to add a user from their own OU to a group in their own OU")
}

// TestRemoveMemberFromGroupInOwnOU verifies the group-manager can remove a user from a group in OU1.
// Depends on TestAddMemberFromOwnOU having already added memberUserOU1 to targetGroupOU1.
func (ts *GroupAuthzTestSuite) TestRemoveMemberFromGroupInOwnOU() {
	payload, err := json.Marshal(MembersRequest{
		Members: []Member{
			{Id: ts.memberUserOU1ID, Type: MemberTypeUser},
		},
	})
	ts.Require().NoError(err)

	resp := ts.doGroup(http.MethodPost, "/groups/"+ts.targetGroupOU1ID+"/members/remove", payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"group-manager should be able to remove a user from a group in their own OU")
}
