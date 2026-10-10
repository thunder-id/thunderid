// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
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
	scopedRSID          string
	targetGroupOU1ID    string
	deletableGroupOU1ID string
	targetGroupOU2ID    string

	// Privileged fixture: a group conferring system:user, which the group-manager does not hold,
	// plus a harmless group nested inside it to exercise the ancestor walk.
	librarianGroupID  string
	librarianRoleID   string
	nestedInLibraryID string

	// Member users created in each OU to test membership authz
	memberUserOU1ID   string
	memberUserOU2ID   string
	memberSchemaOU2ID string

	// An OU holding no groups, plus a manager scoped to it, to verify a listing restricted to an
	// empty scope returns nothing instead of falling back to an unrestricted list.
	emptyOUID        string
	emptyOUSchemaID  string
	emptyOUMgrUserID string
	emptyOUMgrRoleID string
	emptyOUMgrClient *http.Client

	// A manager scoped to the declarative OU, used to verify that an OU-restricted listing reaches
	// the file-based store and not only the database.
	declOUSchemaID  string
	declOUMgrUserID string
	declOUMgrRoleID string
	declOUMgrClient *http.Client

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

	declOUSchemaName  = "authz-decl-ou-schema"
	declOUMgrUsername = "authz-decl-ou-manager"
	declOUMgrPassword = "DeclOUMgr@123"

	emptyOUHandle        = "authz-group-empty-ou"
	emptyOUSchemaName    = "authz-empty-ou-schema"
	emptyOUMgrUsername   = "authz-empty-ou-manager"
	emptyOUMgrPassword   = "EmptyOUMgr@123"
	groupAuthzScopedRSID = "https://authz-test.example.com/group"
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
		Handle:      entityTypeOU1Name,
		DisplayName: "Authz Mgr Schema Ou1",
		OUID:        ts.groupOU1ID,
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
		Type: entityTypeOU1Name,
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
		Type: entityTypeOU1Name,
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
		Handle:      memberSchemaOU2Name,
		DisplayName: "Authz Member Schema Ou2",
		OUID:        ts.groupOU2ID,
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
		Type: memberSchemaOU2Name,
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
		Name:        "authz-target-ou1",
		Description: "Target Group OU1",
		OUID:        ts.groupOU1ID,
	})
	ts.Require().NoError(err, "create target group in OU1")
	ts.targetGroupOU1ID = targetOU1ID

	deletableID, err := testutils.CreateGroup(testutils.Group{
		Name:        "authz-deletable-ou1",
		Description: "Deletable Group OU1",
		OUID:        ts.groupOU1ID,
	})
	ts.Require().NoError(err, "create deletable group in OU1")
	ts.deletableGroupOU1ID = deletableID

	targetOU2ID, err := testutils.CreateGroup(testutils.Group{
		Name:        "authz-target-ou2",
		Description: "Target Group OU2",
		OUID:        ts.groupOU2ID,
	})
	ts.Require().NoError(err, "create target group in OU2")
	ts.targetGroupOU2ID = targetOU2ID

	// ---- 5. Create a custom resource server declaring the fine-grained system scopes ----
	// The product ships only the root "system" scope; this reproduces "system:ou:view",
	// "system:group" and "system:group:view" so the suite can verify resource-level enforcement
	// when configured.
	systemRSID, err := testutils.CreateSystemScopedResourceServer(
		ts.groupOU1ID, "Authz Test RS (group)", groupAuthzScopedRSID, "ou", "group", "user")
	ts.Require().NoError(err, "create scoped resource server")
	ts.scopedRSID = systemRSID

	// ---- 6. Create a role with system:group permission and assign to the user-manager ----
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: groupMgrRoleName,
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
		"",
		groupAuthzScopedRSID,
	)
	ts.Require().NoError(err, "obtain group-manager token")
	ts.Require().NotEmpty(tokenResp.AccessToken, "group-manager token must be non-empty")

	ts.groupAdminClient = testutils.GetHTTPClientWithToken(tokenResp.AccessToken)

	// ---- 8. Create a privileged group the group-manager must not be able to join ----
	// A group carrying no roles of its own, which will be nested inside the privileged group.
	// Joining it confers the privileged permissions transitively, so the guard must walk ancestors.
	nestedID, err := testutils.CreateGroup(testutils.Group{
		Name:        "authz-nested-in-library-ou1",
		Description: "Harmless group nested inside the librarian group",
		OUID:        ts.groupOU1ID,
	})
	ts.Require().NoError(err, "create nested group")
	ts.nestedInLibraryID = nestedID

	// The privileged group, created with the harmless group already nested inside it.
	librarianID, err := testutils.CreateGroup(testutils.Group{
		Name:        "authz-librarian-ou1",
		Description: "Group conferring system:user",
		OUID:        ts.groupOU1ID,
		Members:     []testutils.Member{{Id: nestedID, Type: "group"}},
	})
	ts.Require().NoError(err, "create librarian group")
	ts.librarianGroupID = librarianID

	// Assigned a role conferring "system:user", which the group-manager does not hold. Adding anyone
	// to this group would therefore transfer a permission the caller was never granted.
	librarianRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "authz-librarian-role",
		OUID: ts.groupOU1ID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: systemRSID,
				Permissions:      []string{"system:user"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: librarianID, Type: "group"},
		},
	})
	ts.Require().NoError(err, "create librarian role")
	ts.librarianRoleID = librarianRoleID

	// ---- 9. Create an OU with no groups and a manager scoped to it ----
	emptyOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      emptyOUHandle,
		Name:        "Group Authz Empty OU",
		Description: "OU holding no groups, for empty-scope listing checks",
	})
	ts.Require().NoError(err, "create empty OU")
	ts.emptyOUID = emptyOUID

	emptySchemaID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      emptyOUSchemaName,
		DisplayName: "Authz Empty Ou Schema",
		OUID:        ts.emptyOUID,
		Schema: map[string]interface{}{
			"username":     map[string]interface{}{"type": "string"},
			"password":     map[string]interface{}{"type": "string", "credential": true},
			"display_name": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "create user type for the empty OU")
	ts.emptyOUSchemaID = emptySchemaID

	emptyMgrUserID, err := testutils.CreateUser(testutils.User{
		Type: emptyOUSchemaName,
		OUID: ts.emptyOUID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q, "display_name": "Empty OU Manager"}`,
			emptyOUMgrUsername, emptyOUMgrPassword,
		)),
	})
	ts.Require().NoError(err, "create empty-OU manager user")
	ts.emptyOUMgrUserID = emptyMgrUserID

	emptyMgrRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "authz-empty-ou-manager-role",
		OUID: ts.emptyOUID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: systemRSID,
				Permissions:      []string{"system:ou:view", "system:group", "system:group:view"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.emptyOUMgrUserID, Type: "user"},
		},
	})
	ts.Require().NoError(err, "create empty-OU manager role")
	ts.emptyOUMgrRoleID = emptyMgrRoleID

	emptyMgrToken, err := testutils.ObtainAccessTokenWithPassword(
		groupAuthzDevelopClientID,
		groupAuthzDevelopRedirectURI,
		"system:ou:view system:group system:group:view",
		emptyOUMgrUsername,
		emptyOUMgrPassword,
		true,
		"",
		groupAuthzScopedRSID,
	)
	ts.Require().NoError(err, "obtain empty-OU manager token")
	ts.Require().NotEmpty(emptyMgrToken.AccessToken, "empty-OU manager token must be non-empty")

	ts.emptyOUMgrClient = testutils.GetHTTPClientWithToken(emptyMgrToken.AccessToken)

	// ---- 10. Create a manager scoped to the declarative OU ----
	declSchemaID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      declOUSchemaName,
		DisplayName: "Authz Decl Ou Schema",
		OUID:        declGroupOUID,
		Schema: map[string]interface{}{
			"username":     map[string]interface{}{"type": "string"},
			"password":     map[string]interface{}{"type": "string", "credential": true},
			"display_name": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "create user type for the declarative OU")
	ts.declOUSchemaID = declSchemaID

	declMgrUserID, err := testutils.CreateUser(testutils.User{
		Type: declOUSchemaName,
		OUID: declGroupOUID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q, "display_name": "Declarative OU Manager"}`,
			declOUMgrUsername, declOUMgrPassword,
		)),
	})
	ts.Require().NoError(err, "create declarative-OU manager user")
	ts.declOUMgrUserID = declMgrUserID

	declMgrRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "authz-decl-ou-manager-role",
		OUID: declGroupOUID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: systemRSID,
				Permissions:      []string{"system:ou:view", "system:group", "system:group:view"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.declOUMgrUserID, Type: "user"},
		},
	})
	ts.Require().NoError(err, "create declarative-OU manager role")
	ts.declOUMgrRoleID = declMgrRoleID

	declMgrToken, err := testutils.ObtainAccessTokenWithPassword(
		groupAuthzDevelopClientID,
		groupAuthzDevelopRedirectURI,
		"system:ou:view system:group system:group:view",
		declOUMgrUsername,
		declOUMgrPassword,
		true,
		"",
		groupAuthzScopedRSID,
	)
	ts.Require().NoError(err, "obtain declarative-OU manager token")
	ts.Require().NotEmpty(declMgrToken.AccessToken, "declarative-OU manager token must be non-empty")

	ts.declOUMgrClient = testutils.GetHTTPClientWithToken(declMgrToken.AccessToken)
}

// ---------------------------------------------------------------------------
// Suite teardown
// ---------------------------------------------------------------------------

func (ts *GroupAuthzTestSuite) TearDownSuite() {
	if ts.scopedRSID != "" {
		if err := testutils.DeleteResourceServerWithChildren(ts.scopedRSID); err != nil {
			ts.T().Errorf("teardown: delete scoped resource server: %v", err)
		}
	}
	if ts.groupMgrRoleID != "" {
		if err := testutils.DeleteRole(ts.groupMgrRoleID); err != nil {
			ts.T().Logf("teardown: delete group-manager role: %v", err)
		}
	}
	if ts.librarianRoleID != "" {
		if err := testutils.DeleteRole(ts.librarianRoleID); err != nil {
			ts.T().Logf("teardown: delete librarian role: %v", err)
		}
	}
	if ts.declOUMgrRoleID != "" {
		if err := testutils.DeleteRole(ts.declOUMgrRoleID); err != nil {
			ts.T().Logf("teardown: delete declarative-OU manager role: %v", err)
		}
	}
	if ts.declOUMgrUserID != "" {
		if err := testutils.DeleteUser(ts.declOUMgrUserID); err != nil {
			ts.T().Logf("teardown: delete declarative-OU manager user: %v", err)
		}
	}
	if ts.declOUSchemaID != "" {
		if err := testutils.DeleteUserType(ts.declOUSchemaID); err != nil {
			ts.T().Logf("teardown: delete declarative-OU user type: %v", err)
		}
	}
	if ts.emptyOUMgrRoleID != "" {
		if err := testutils.DeleteRole(ts.emptyOUMgrRoleID); err != nil {
			ts.T().Logf("teardown: delete empty-OU manager role: %v", err)
		}
	}
	if ts.emptyOUMgrUserID != "" {
		if err := testutils.DeleteUser(ts.emptyOUMgrUserID); err != nil {
			ts.T().Logf("teardown: delete empty-OU manager user: %v", err)
		}
	}
	if ts.emptyOUSchemaID != "" {
		if err := testutils.DeleteUserType(ts.emptyOUSchemaID); err != nil {
			ts.T().Logf("teardown: delete empty-OU user type: %v", err)
		}
	}
	if ts.emptyOUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.emptyOUID); err != nil {
			ts.T().Logf("teardown: delete empty OU: %v", err)
		}
	}
	for _, id := range []string{ts.nestedInLibraryID, ts.librarianGroupID,
		ts.targetGroupOU1ID, ts.deletableGroupOU1ID} {
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
		"ouId":        ts.groupOU1ID,
		"name":        "authz-created-group",
		"description": "Created Group",
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
		"ouId":        ts.groupOU2ID,
		"name":        "authz-denied-group",
		"description": "Denied Group",
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
		"ouId":        ts.groupOU1ID,
		"name":        "authz-target-ou1",
		"description": "Updated Description",
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
		"name": "Should Not Update",
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

// ---------------------------------------------------------------------------
// Tests — OU-scoped denials on the remaining group routes
// ---------------------------------------------------------------------------

// TestListGroupsByPathInOtherOU verifies the path-based listing is denied for a sibling OU. The
// denial is raised by the OU lookup that resolves the handle path, before the group authorization
// check is reached, so a caller outside the OU cannot even enumerate it.
func (ts *GroupAuthzTestSuite) TestListGroupsByPathInOtherOU() {
	resp := ts.doGroup(http.MethodGet, "/groups/tree/"+groupAuthzOU2Handle, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"group-manager must not list groups under an OU it holds no group permission for")
}

// TestListGroupsByPathInOwnOU verifies the same route still works for the manager's own OU.
func (ts *GroupAuthzTestSuite) TestListGroupsByPathInOwnOU() {
	resp := ts.doGroup(http.MethodGet, "/groups/tree/"+groupAuthzOU1Handle, nil)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode,
		"group-manager should be able to list groups under their own OU")

	var listResp GroupListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ids := make([]string, 0, len(listResp.Groups))
	for _, g := range listResp.Groups {
		ids = append(ids, g.Id)
	}
	ts.Containsf(ids, ts.targetGroupOU1ID, "tree listing must include the OU1 target group, got %v", ids)
}

// TestCreateGroupByPathInOtherOU verifies the group-manager cannot create a group under OU2 through
// the path-based route, closing it as an alternative to the denied POST /groups route.
func (ts *GroupAuthzTestSuite) TestCreateGroupByPathInOtherOU() {
	payload := ts.mustMarshal(map[string]any{
		"name":        "authz-denied-tree-group",
		"description": "Denied Group via tree route",
	})

	resp := ts.doGroup(http.MethodPost, "/groups/tree/"+groupAuthzOU2Handle, payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"group-manager must not create a group under an OU it holds no group permission for")
}

// TestGetGroupMembersInOtherOU verifies the members of a group in OU2 are not readable.
func (ts *GroupAuthzTestSuite) TestGetGroupMembersInOtherOU() {
	resp := ts.doGroup(http.MethodGet, "/groups/"+ts.targetGroupOU2ID+"/members", nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"group-manager must not read the members of a group in a different OU")
}

// TestAddMemberToGroupInOtherOU verifies members cannot be added to a group in OU2.
func (ts *GroupAuthzTestSuite) TestAddMemberToGroupInOtherOU() {
	payload := ts.mustMarshal(MembersRequest{
		Members: []Member{
			{Id: ts.memberUserOU2ID, Type: MemberTypeUser},
		},
	})

	resp := ts.doGroup(http.MethodPost, "/groups/"+ts.targetGroupOU2ID+"/members/add", payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"group-manager must not add members to a group in a different OU")
}

// TestMoveGroupToOtherOU verifies a group cannot be relocated into an OU the caller does not
// administer, even though the caller may update the group in its current OU.
func (ts *GroupAuthzTestSuite) TestMoveGroupToOtherOU() {
	movableID, err := testutils.CreateGroup(testutils.Group{
		Name:        "authz-movable-ou1",
		Description: "Group used to verify cross-OU moves are denied",
		OUID:        ts.groupOU1ID,
	})
	ts.Require().NoError(err, "create movable group in OU1")
	defer func() {
		if delErr := testutils.DeleteGroup(movableID); delErr != nil {
			ts.T().Logf("cleanup: delete movable group %s: %v", movableID, delErr)
		}
	}()

	payload := ts.mustMarshal(map[string]any{
		"ouId":        ts.groupOU2ID,
		"name":        "authz-movable-ou1",
		"description": "Attempted move into OU2",
	})

	resp := ts.doGroup(http.MethodPut, "/groups/"+movableID, payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"group-manager must not move a group into an OU they do not administer")

	// The group must remain in OU1 after the rejected move.
	getResp := ts.doGroup(http.MethodGet, "/groups/"+movableID, nil)
	defer getResp.Body.Close()

	ts.Require().Equal(http.StatusOK, getResp.StatusCode)

	var group Group
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&group))
	ts.Equal(ts.groupOU1ID, group.OUID, "the rejected move must not have changed the group's OU")
}

// TestListGroupsWithEmptyScopeReturnsNothing verifies a manager whose scope holds no groups gets an
// empty listing rather than falling back to an unrestricted one.
func (ts *GroupAuthzTestSuite) TestListGroupsWithEmptyScopeReturnsNothing() {
	req, err := http.NewRequest(http.MethodGet, groupAuthzServerURL+"/groups?limit=100", nil)
	ts.Require().NoError(err)

	resp, err := ts.emptyOUMgrClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode, "listing should succeed for an empty scope")

	var listResp GroupListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ts.Equal(0, listResp.TotalResults, "an OU holding no groups must report no results")
	ts.Empty(listResp.Groups, "an OU holding no groups must return no groups")
	ts.Equal(1, listResp.StartIndex)
	ts.Empty(listResp.Links, "an empty listing carries no pagination links")
}

// TestOUScopedListingIncludesDeclarativeGroups verifies an OU-restricted listing draws from the
// file-based store as well as the database, so a manager scoped to the declarative OU sees the
// YAML-declared groups alongside any runtime group in the same OU.
func (ts *GroupAuthzTestSuite) TestOUScopedListingIncludesDeclarativeGroups() {
	runtimeGroupID, err := testutils.CreateGroup(testutils.Group{
		Name:        "authz-runtime-in-decl-ou",
		Description: "Runtime group in the declarative OU",
		OUID:        declGroupOUID,
	})
	ts.Require().NoError(err, "create runtime group in the declarative OU")
	defer func() {
		if delErr := testutils.DeleteGroup(runtimeGroupID); delErr != nil {
			ts.T().Logf("cleanup: delete runtime group %s: %v", runtimeGroupID, delErr)
		}
	}()

	req, err := http.NewRequest(http.MethodGet, groupAuthzServerURL+"/groups?limit=100", nil)
	ts.Require().NoError(err)

	resp, err := ts.declOUMgrClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp GroupListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ids := make([]string, 0, len(listResp.Groups))
	for _, g := range listResp.Groups {
		ids = append(ids, g.Id)
	}

	ts.Containsf(ids, declGroupID, "scoped listing must include the declarative group, got %v", ids)
	ts.Containsf(ids, declNestedGroupID, "scoped listing must include the nested declarative group, got %v", ids)
	ts.Containsf(ids, runtimeGroupID, "scoped listing must include the runtime group, got %v", ids)
	ts.NotContainsf(ids, ts.targetGroupOU1ID, "scoped listing must exclude groups from other OUs, got %v", ids)
}

// TestOUScopedListingPaginatesAcrossBothStores verifies pagination applies to the merged result of
// the database and file-based stores rather than to either store alone.
func (ts *GroupAuthzTestSuite) TestOUScopedListingPaginatesAcrossBothStores() {
	// The declarative OU holds two YAML groups; a runtime group is needed as well, otherwise the
	// paging below never crosses a store boundary.
	runtimeGroupID, err := testutils.CreateGroup(testutils.Group{
		Name:        "authz-paginated-in-decl-ou",
		Description: "Runtime group used to page across both stores",
		OUID:        declGroupOUID,
	})
	ts.Require().NoError(err, "create runtime group in the declarative OU")
	defer func() {
		if delErr := testutils.DeleteGroup(runtimeGroupID); delErr != nil {
			ts.T().Logf("cleanup: delete runtime group %s: %v", runtimeGroupID, delErr)
		}
	}()

	seen := make(map[string]bool)
	for offset := 0; offset < 3; offset++ {
		req, err := http.NewRequest(http.MethodGet,
			fmt.Sprintf("%s/groups?limit=1&offset=%d", groupAuthzServerURL, offset), nil)
		ts.Require().NoError(err)

		resp, err := ts.declOUMgrClient.Do(req)
		ts.Require().NoError(err)

		ts.Require().Equal(http.StatusOK, resp.StatusCode)

		var listResp GroupListResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&listResp)
		resp.Body.Close()
		ts.Require().NoError(decodeErr)

		ts.Require().Equal(3, listResp.TotalResults,
			"the declarative OU holds the two declared groups plus the runtime one")
		ts.LessOrEqual(listResp.Count, 1, "a page must not exceed the requested limit")
		ts.Equal(offset+1, listResp.StartIndex)

		for _, g := range listResp.Groups {
			ts.Falsef(seen[g.Id], "group %s must not repeat across pages", g.Id)
			seen[g.Id] = true
		}
	}

	ts.Len(seen, 3, "paging through the scoped listing must yield each group exactly once")
	ts.Truef(seen[runtimeGroupID], "paging must surface the runtime group, got %v", seen)
	ts.Truef(seen[declGroupID], "paging must surface the declarative group, got %v", seen)
}

// mustMarshal encodes a JSON request body, failing the test on error.
func (ts *GroupAuthzTestSuite) mustMarshal(v any) []byte {
	ts.T().Helper()
	payload, err := json.Marshal(v)
	ts.Require().NoError(err)
	return payload
}

// ---------------------------------------------------------------------------
// Privilege escalation via group membership
// ---------------------------------------------------------------------------

// The reported vulnerability. The group-manager holds system:group but not system:user. The
// librarian group confers system:user through its assigned role, so adding anyone to it — including
// the caller — would grant a permission the caller was never given.
func (ts *GroupAuthzTestSuite) TestAddSelfToPrivilegedGroupIsForbidden() {
	payload := ts.mustMarshal(map[string]any{
		"members": []map[string]string{{"id": ts.groupMgrUserID, "type": "user"}},
	})

	resp := ts.doGroup(http.MethodPost, "/groups/"+ts.librarianGroupID+"/members/add", payload)
	defer resp.Body.Close()

	ts.Equalf(http.StatusForbidden, resp.StatusCode,
		"adding self to a group conferring system:user must be forbidden, got %d", resp.StatusCode)
}

// Escalation on behalf of someone else is the same defect; the guard is actor-relative.
func (ts *GroupAuthzTestSuite) TestAddAnotherUserToPrivilegedGroupIsForbidden() {
	payload := ts.mustMarshal(map[string]any{
		"members": []map[string]string{{"id": ts.memberUserOU1ID, "type": "user"}},
	})

	resp := ts.doGroup(http.MethodPost, "/groups/"+ts.librarianGroupID+"/members/add", payload)
	defer resp.Body.Close()

	ts.Equalf(http.StatusForbidden, resp.StatusCode,
		"adding another user to a privileged group must be forbidden, got %d", resp.StatusCode)
}

// Joining a group nested inside the privileged one confers its permissions transitively. This fails
// unless the guard walks the ancestor chain rather than only the target group's own roles.
func (ts *GroupAuthzTestSuite) TestAddSelfToGroupNestedInPrivilegedGroupIsForbidden() {
	payload := ts.mustMarshal(map[string]any{
		"members": []map[string]string{{"id": ts.groupMgrUserID, "type": "user"}},
	})

	resp := ts.doGroup(http.MethodPost, "/groups/"+ts.nestedInLibraryID+"/members/add", payload)
	defer resp.Body.Close()

	ts.Equalf(http.StatusForbidden, resp.StatusCode,
		"joining a group nested inside a privileged group must be forbidden, got %d", resp.StatusCode)
}

// Nesting a group the caller controls into the privileged group makes that group's members
// transitive members of it, so group-typed members are guarded too.
func (ts *GroupAuthzTestSuite) TestNestingControlledGroupIntoPrivilegedGroupIsForbidden() {
	payload := ts.mustMarshal(map[string]any{
		"members": []map[string]string{{"id": ts.targetGroupOU1ID, "type": "group"}},
	})

	resp := ts.doGroup(http.MethodPost, "/groups/"+ts.librarianGroupID+"/members/add", payload)
	defer resp.Body.Close()

	ts.Equalf(http.StatusForbidden, resp.StatusCode,
		"nesting a controlled group into a privileged group must be forbidden, got %d", resp.StatusCode)
}

// Removal carries the same standing requirement, so a limited administrator cannot strip members
// from a group more powerful than itself.
func (ts *GroupAuthzTestSuite) TestRemoveMemberFromPrivilegedGroupIsForbidden() {
	payload := ts.mustMarshal(map[string]any{
		"members": []map[string]string{{"id": ts.nestedInLibraryID, "type": "group"}},
	})

	resp := ts.doGroup(http.MethodPost, "/groups/"+ts.librarianGroupID+"/members/remove", payload)
	defer resp.Body.Close()

	ts.Equalf(http.StatusForbidden, resp.StatusCode,
		"removing a member from a privileged group must be forbidden, got %d", resp.StatusCode)
}

// The guard must not disturb ordinary delegation: a group conferring nothing stays manageable.
func (ts *GroupAuthzTestSuite) TestAddMemberToUnprivilegedGroupStillSucceeds() {
	payload := ts.mustMarshal(map[string]any{
		"members": []map[string]string{{"id": ts.memberUserOU1ID, "type": "user"}},
	})

	resp := ts.doGroup(http.MethodPost, "/groups/"+ts.targetGroupOU1ID+"/members/add", payload)
	defer resp.Body.Close()

	ts.Equalf(http.StatusOK, resp.StatusCode,
		"managing a group that confers nothing must still succeed, got %d", resp.StatusCode)
}
