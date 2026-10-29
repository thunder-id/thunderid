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

// GroupAuthzTestSuite validates group CRUD authorization under the deployment boundary, where a
// token's scope applies to every OU in the deployment.
//
// Permission model:
//
//	system:group      → create, update, delete groups
//	system:group:view → list, get groups (implied by system:group)
//
// A user-manager living in OU1 holds the system:group permission. The suite
// verifies that:
//
//   - Read operations on groups in OU1 and in OU2 (sibling) are allowed (200)
//   - Write operations on groups in OU1 and in OU2 are allowed (201/200/204)
//   - Listing groups returns groups from every OU in the deployment
//   - Adding a member to a group conferring permissions the caller lacks is denied (403)
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

	// An OU holding no groups, plus a manager living in it, to verify the manager's listing still
	// spans the deployment.
	emptyOUID        string
	emptyOUSchemaID  string
	emptyOUMgrUserID string
	emptyOUMgrRoleID string
	emptyOUMgrClient *http.Client

	// A manager living in the declarative OU, used to verify that the listing reaches the file-based
	// store and not only the database.
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

	// ---- 9. Create an OU with no groups and a manager living in it ----
	emptyOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      emptyOUHandle,
		Name:        "Group Authz Empty OU",
		Description: "OU holding no groups, for listing checks",
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

	// ---- 10. Create a manager living in the declarative OU ----
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

// listGroupsPage fetches one page of GET /groups with the given client, requiring a 200.
func (ts *GroupAuthzTestSuite) listGroupsPage(client *http.Client, limit, offset int) GroupListResponse {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/groups?limit=%d&offset=%d", groupAuthzServerURL, limit, offset), nil)
	ts.Require().NoError(err)

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode, "list groups should succeed")

	var listResp GroupListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))
	return listResp
}

// listAllGroupIDs pages through GET /groups with the given client and returns every group ID, so
// assertions hold however many groups the deployment holds.
func (ts *GroupAuthzTestSuite) listAllGroupIDs(client *http.Client) []string {
	ts.T().Helper()

	var ids []string
	for offset := 0; ; {
		page := ts.listGroupsPage(client, 100, offset)
		for _, g := range page.Groups {
			ids = append(ids, g.Id)
		}
		offset += page.Count
		if page.Count == 0 || offset >= page.TotalResults {
			return ids
		}
	}
}

// ---------------------------------------------------------------------------
// Tests — READ operations (system:group:view implied by system:group)
// ---------------------------------------------------------------------------

// TestListGroups verifies the list contains groups from every OU in the deployment.
func (ts *GroupAuthzTestSuite) TestListGroups() {
	ids := ts.listAllGroupIDs(ts.groupAdminClient)

	ts.Containsf(ids, ts.targetGroupOU1ID,
		"list must include target group in OU1, got IDs: %v", ids)
	ts.Containsf(ids, ts.targetGroupOU2ID,
		"list must include target group in OU2 (sibling), got IDs: %v", ids)
}

// TestListGroupsFilteredOnOwnOU verifies an OU filter on the manager's own OU lists that OU's groups.
func (ts *GroupAuthzTestSuite) TestListGroupsFilteredOnOwnOU() {
	resp := ts.doGroup(http.MethodGet, "/groups?limit=100&"+groupOUFilter(ts.groupOU1ID), nil)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp GroupListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ids := make([]string, 0, len(listResp.Groups))
	for _, g := range listResp.Groups {
		ts.Equal(ts.groupOU1ID, g.OUID, "group %s", g.Id)
		ids = append(ids, g.Id)
	}
	ts.Containsf(ids, ts.targetGroupOU1ID, "filtered list must include the OU1 group, got IDs: %v", ids)
}

// TestListGroupsFilteredOnOtherOU verifies an OU filter on a different OU lists that OU's groups,
// since the manager's group permission reaches every OU in the deployment.
func (ts *GroupAuthzTestSuite) TestListGroupsFilteredOnOtherOU() {
	resp := ts.doGroup(http.MethodGet, "/groups?limit=100&"+groupOUFilter(ts.groupOU2ID), nil)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp GroupListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ids := make([]string, 0, len(listResp.Groups))
	for _, g := range listResp.Groups {
		ts.Equal(ts.groupOU2ID, g.OUID, "group %s", g.Id)
		ids = append(ids, g.Id)
	}
	ts.Containsf(ids, ts.targetGroupOU2ID, "filtered list must include the OU2 group, got IDs: %v", ids)
}

// TestGetGroupInOwnOU verifies the group-manager can read a group in their own OU.
func (ts *GroupAuthzTestSuite) TestGetGroupInOwnOU() {
	resp := ts.doGroup(http.MethodGet, "/groups/"+ts.targetGroupOU1ID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"group-manager should be able to read a group in their own OU")
}

// TestGetGroupInOtherOU verifies the group-manager can read a group in OU2.
func (ts *GroupAuthzTestSuite) TestGetGroupInOtherOU() {
	resp := ts.doGroup(http.MethodGet, "/groups/"+ts.targetGroupOU2ID, nil)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode,
		"group-manager should be able to read a group in a different OU")

	var group Group
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&group))
	ts.Equal(ts.groupOU2ID, group.OUID)
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

// TestCreateGroupInOtherOU verifies the group-manager can create a group in OU2.
func (ts *GroupAuthzTestSuite) TestCreateGroupInOtherOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"ouId":        ts.groupOU2ID,
		"name":        "authz-created-group-ou2",
		"description": "Created Group in OU2",
	})
	ts.Require().NoError(err)

	resp := ts.doGroup(http.MethodPost, "/groups", payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusCreated, resp.StatusCode,
		"group-manager should be able to create a group in a different OU")

	var created Group
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&created))
	if created.Id != "" {
		defer func() {
			if delErr := testutils.DeleteGroup(created.Id); delErr != nil {
				ts.T().Logf("cleanup: failed to delete created group %s: %v", created.Id, delErr)
			}
		}()
	}
	ts.Equal(ts.groupOU2ID, created.OUID, "the group must be created in OU2")
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

// TestUpdateGroupInOtherOU verifies the group-manager can update a group in OU2.
func (ts *GroupAuthzTestSuite) TestUpdateGroupInOtherOU() {
	updatableID, err := testutils.CreateGroup(testutils.Group{
		Name:        "authz-updatable-ou2",
		Description: "Group used to verify cross-OU updates",
		OUID:        ts.groupOU2ID,
	})
	ts.Require().NoError(err, "create updatable group in OU2")
	defer func() {
		if delErr := testutils.DeleteGroup(updatableID); delErr != nil {
			ts.T().Logf("cleanup: delete updatable group %s: %v", updatableID, delErr)
		}
	}()

	payload, err := json.Marshal(map[string]interface{}{
		"ouId":        ts.groupOU2ID,
		"name":        "authz-updatable-ou2",
		"description": "Updated Description",
	})
	ts.Require().NoError(err)

	resp := ts.doGroup(http.MethodPut, "/groups/"+updatableID, payload)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode,
		"group-manager should be able to update a group in a different OU")

	var updated Group
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&updated))
	ts.Equal("Updated Description", updated.Description)
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

// TestDeleteGroupInOtherOU verifies the group-manager can delete a group in OU2.
func (ts *GroupAuthzTestSuite) TestDeleteGroupInOtherOU() {
	deletableID, err := testutils.CreateGroup(testutils.Group{
		Name:        "authz-deletable-ou2",
		Description: "Group used to verify cross-OU deletes",
		OUID:        ts.groupOU2ID,
	})
	ts.Require().NoError(err, "create deletable group in OU2")

	resp := ts.doGroup(http.MethodDelete, "/groups/"+deletableID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusNoContent, resp.StatusCode,
		"group-manager should be able to delete a group in a different OU")

	if resp.StatusCode != http.StatusNoContent {
		if delErr := testutils.DeleteGroup(deletableID); delErr != nil {
			ts.T().Logf("cleanup: delete deletable group %s: %v", deletableID, delErr)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests — MEMBER operations (system:group)
// ---------------------------------------------------------------------------

// TestAddMemberFromOtherOU verifies the group-manager can add a user from OU2 to a group in OU1.
func (ts *GroupAuthzTestSuite) TestAddMemberFromOtherOU() {
	payload, err := json.Marshal(MembersRequest{
		Members: []Member{
			{Id: ts.memberUserOU2ID, Type: MemberTypeUser},
		},
	})
	ts.Require().NoError(err)

	resp := ts.doGroup(http.MethodPost, "/groups/"+ts.targetGroupOU1ID+"/members/add", payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"group-manager should be able to add a user from OU2 to a group in OU1")

	// Remove the member again so the shared OU1 target group keeps its original membership.
	removeResp := ts.doGroup(http.MethodPost, "/groups/"+ts.targetGroupOU1ID+"/members/remove", payload)
	defer removeResp.Body.Close()
	ts.Equal(http.StatusOK, removeResp.StatusCode, "cleanup: remove the OU2 member from the OU1 group")
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

	addResp := ts.doGroup(http.MethodPost, "/groups/"+ts.targetGroupOU1ID+"/members/add", payload)
	defer addResp.Body.Close()
	ts.Require().Equal(http.StatusOK, addResp.StatusCode,
		"group-manager should be able to add a user before removing it")

	resp := ts.doGroup(http.MethodPost, "/groups/"+ts.targetGroupOU1ID+"/members/remove", payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"group-manager should be able to remove a user from a group in their own OU")
}

// ---------------------------------------------------------------------------
// Tests: cross-OU access on the remaining group routes
// ---------------------------------------------------------------------------

// TestListGroupsByPathInOtherOU verifies the path-based listing works for a sibling OU, since the
// manager's scope covers every OU in the deployment.
func (ts *GroupAuthzTestSuite) TestListGroupsByPathInOtherOU() {
	resp := ts.doGroup(http.MethodGet, "/groups/tree/"+groupAuthzOU2Handle, nil)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode,
		"group-manager should be able to list groups under a different OU")

	var listResp GroupListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ids := make([]string, 0, len(listResp.Groups))
	for _, g := range listResp.Groups {
		ids = append(ids, g.Id)
	}
	ts.Containsf(ids, ts.targetGroupOU2ID, "tree listing must include the OU2 target group, got %v", ids)
	ts.NotContainsf(ids, ts.targetGroupOU1ID, "tree listing must only hold groups under OU2, got %v", ids)
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

// TestCreateGroupByPathInOtherOU verifies the group-manager can create a group under OU2 through
// the path-based route.
func (ts *GroupAuthzTestSuite) TestCreateGroupByPathInOtherOU() {
	payload := ts.mustMarshal(map[string]any{
		"name":        "authz-created-tree-group-ou2",
		"description": "Created Group via tree route",
	})

	resp := ts.doGroup(http.MethodPost, "/groups/tree/"+groupAuthzOU2Handle, payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusCreated, resp.StatusCode,
		"group-manager should be able to create a group under a different OU")

	var created Group
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&created))
	if created.Id != "" {
		defer func() {
			if delErr := testutils.DeleteGroup(created.Id); delErr != nil {
				ts.T().Logf("cleanup: failed to delete created group %s: %v", created.Id, delErr)
			}
		}()
	}
	ts.Equal(ts.groupOU2ID, created.OUID, "the group must be created under OU2")
}

// TestGetGroupMembersInOtherOU verifies the members of a group in OU2 are readable.
func (ts *GroupAuthzTestSuite) TestGetGroupMembersInOtherOU() {
	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:        "authz-members-ou2",
		Description: "Group used to verify cross-OU member reads",
		OUID:        ts.groupOU2ID,
		Members:     []testutils.Member{{Id: ts.memberUserOU2ID, Type: "user"}},
	})
	ts.Require().NoError(err, "create group with a member in OU2")
	defer func() {
		if delErr := testutils.DeleteGroup(groupID); delErr != nil {
			ts.T().Logf("cleanup: delete group %s: %v", groupID, delErr)
		}
	}()

	resp := ts.doGroup(http.MethodGet, "/groups/"+groupID+"/members", nil)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode,
		"group-manager should be able to read the members of a group in a different OU")

	var membersResp MemberListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&membersResp))
	ts.Require().Len(membersResp.Members, 1)
	ts.Equal(ts.memberUserOU2ID, membersResp.Members[0].Id)
}

// TestAddMemberToGroupInOtherOU verifies members can be added to a group in OU2.
func (ts *GroupAuthzTestSuite) TestAddMemberToGroupInOtherOU() {
	payload := ts.mustMarshal(MembersRequest{
		Members: []Member{
			{Id: ts.memberUserOU2ID, Type: MemberTypeUser},
		},
	})

	resp := ts.doGroup(http.MethodPost, "/groups/"+ts.targetGroupOU2ID+"/members/add", payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"group-manager should be able to add members to a group in a different OU")

	// Remove the member again so the shared OU2 target group keeps its original membership.
	removeResp := ts.doGroup(http.MethodPost, "/groups/"+ts.targetGroupOU2ID+"/members/remove", payload)
	defer removeResp.Body.Close()
	ts.Equal(http.StatusOK, removeResp.StatusCode, "cleanup: remove the member from the OU2 group")
}

// TestMoveGroupToOtherOU verifies a group can be relocated into a sibling OU, since the manager's
// scope covers both the source and the target OU.
func (ts *GroupAuthzTestSuite) TestMoveGroupToOtherOU() {
	movableID, err := testutils.CreateGroup(testutils.Group{
		Name:        "authz-movable-ou1",
		Description: "Group used to verify cross-OU moves",
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
		"description": "Moved into OU2",
	})

	resp := ts.doGroup(http.MethodPut, "/groups/"+movableID, payload)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode,
		"group-manager should be able to move a group into a different OU")

	// The group must now live in OU2.
	getResp := ts.doGroup(http.MethodGet, "/groups/"+movableID, nil)
	defer getResp.Body.Close()

	ts.Require().Equal(http.StatusOK, getResp.StatusCode)

	var group Group
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&group))
	ts.Equal(ts.groupOU2ID, group.OUID, "the move must have changed the group's OU")
}

// TestListGroupsByManagerInEmptyOU verifies a manager whose own OU holds no groups still lists the
// groups of every other OU, while the path-based listing of that OU stays empty.
func (ts *GroupAuthzTestSuite) TestListGroupsByManagerInEmptyOU() {
	ids := ts.listAllGroupIDs(ts.emptyOUMgrClient)

	ts.Containsf(ids, ts.targetGroupOU1ID, "listing must include the OU1 target group, got %v", ids)
	ts.Containsf(ids, ts.targetGroupOU2ID, "listing must include the OU2 target group, got %v", ids)

	req, err := http.NewRequest(http.MethodGet, groupAuthzServerURL+"/groups/tree/"+emptyOUHandle, nil)
	ts.Require().NoError(err)

	resp, err := ts.emptyOUMgrClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode, "listing the empty OU by path should succeed")

	var listResp GroupListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ts.Equal(0, listResp.TotalResults, "an OU holding no groups must report no results")
	ts.Empty(listResp.Groups, "an OU holding no groups must return no groups")
	ts.Equal(1, listResp.StartIndex)
	ts.Empty(listResp.Links, "an empty listing carries no pagination links")
}

// TestListingIncludesDeclarativeGroups verifies the listing draws from the file-based store as well
// as the database, so the YAML-declared groups appear alongside runtime groups from every OU.
func (ts *GroupAuthzTestSuite) TestListingIncludesDeclarativeGroups() {
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

	ids := ts.listAllGroupIDs(ts.declOUMgrClient)

	ts.Containsf(ids, declGroupID, "listing must include the declarative group, got %v", ids)
	ts.Containsf(ids, declNestedGroupID, "listing must include the nested declarative group, got %v", ids)
	ts.Containsf(ids, runtimeGroupID, "listing must include the runtime group, got %v", ids)
	ts.Containsf(ids, ts.targetGroupOU1ID, "listing must include groups from other OUs, got %v", ids)
}

// TestListingPaginatesAcrossBothStores verifies pagination applies to the merged result of the
// database and file-based stores rather than to either store alone.
func (ts *GroupAuthzTestSuite) TestListingPaginatesAcrossBothStores() {
	const pageSize = 3

	total := ts.listGroupsPage(ts.declOUMgrClient, 1, 0).TotalResults
	ts.Require().Positive(total)

	seen := make(map[string]bool)
	for offset := 0; offset < total; offset += pageSize {
		listResp := ts.listGroupsPage(ts.declOUMgrClient, pageSize, offset)

		ts.Require().Equal(total, listResp.TotalResults, "the total must not change across pages")
		ts.LessOrEqual(listResp.Count, pageSize, "a page must not exceed the requested limit")
		ts.Equal(offset+1, listResp.StartIndex)

		for _, g := range listResp.Groups {
			ts.Falsef(seen[g.Id], "group %s must not repeat across pages", g.Id)
			seen[g.Id] = true
		}
	}

	ts.Len(seen, total, "paging through the listing must yield each group exactly once")
	ts.Truef(seen[ts.targetGroupOU1ID], "paging must surface the runtime group, got %v", seen)
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
