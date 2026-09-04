// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// SCIMAuthzTestSuite validates that the SCIM Users and Groups endpoints
// enforce the same OU-scoped authz as the plain management API
// (tests/integration/user's UserAuthzTestSuite), since backend/internal/scim
// delegates every mutation to internal/user and internal/group, which own
// the actual OU boundary check.
//
// The product ships only the root "system" scope by default. This suite creates a custom
// resource server (mirroring backend/internal/system/security/permissions.go's naming) that
// declares the fine-grained scopes actually exercised:
//
//	system:user            → create, replace, delete SCIM users
//	system:user:view       → list, get SCIM users
//	system:usertype:view   → required to resolve a usertype's own schema
//	                          before create/replace can proceed
//	system:group           → create, replace, patch, delete SCIM groups
//	system:group:view      → list, get SCIM groups
//
// Fixture topology:
//
//	OU1 (handle: scim-authz-ou1) ← scim-manager and its own users/groups live here
//	OU2 (handle: scim-authz-ou2) ← sibling OU with its own user/group, out of reach
//
// One asymmetry worth calling out: SCIM Users are OU-bound through the
// usertype's own schema (a create payload names the extension schema, which
// carries its OU), so "create in another OU" is reachable by targeting OU2's
// extension URN. SCIM Groups carry no OU field on the wire at all — Group
// creation always lands in the caller's own OU (security.GetOUID(ctx)), so
// there is no create-in-another-OU case to test; only read/replace/patch/delete
// against a group that already exists in a foreign OU can be denied.
type SCIMAuthzTestSuite struct {
	suite.Suite

	ou1ID string
	ou2ID string

	entityTypeOU1ID   string
	entityTypeOU2ID   string
	entityTypeOU1Name string
	entityTypeOU2Name string
	extensionURNOU1   string
	extensionURNOU2   string

	mgrUserID          string
	targetUserOU1ID    string
	deletableUserOU1ID string
	targetUserOU2ID    string

	scopedRSID string
	roleID     string

	targetGroupOU1ID    string
	deletableGroupOU1ID string
	targetGroupOU2ID    string

	// scopedClient carries the scim-manager's system:user/system:group scoped
	// token, restricted to OU1.
	scopedClient *http.Client
}

const (
	scimAuthzDevelopClientID    = "CONSOLE"
	scimAuthzDevelopRedirectURI = "https://localhost:8095/console"

	scimAuthzMgrUsername = "scim-authz-manager"
	scimAuthzMgrPassword = "ScimAuthzMgr@123"
	scimAuthzMgrRoleName = "SCIM Authz Manager (scim-authz-test)"
)

// TestSCIMAuthzTestSuite tests SCIM Authz Test Suite.
func TestSCIMAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(SCIMAuthzTestSuite))
}

// ---------------------------------------------------------------------------
// Suite setup
// ---------------------------------------------------------------------------

// SetupSuite initializes the test suite environment.
func (ts *SCIMAuthzTestSuite) SetupSuite() {
	ou1ID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "scim-authz-ou1",
		Name:        "SCIM Authz Test OU1",
		Description: "Primary OU for SCIM authz integration test",
	})
	ts.Require().NoError(err, "create scim-authz OU1")
	ts.ou1ID = ou1ID

	ou2ID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "scim-authz-ou2",
		Name:        "SCIM Authz Test OU2",
		Description: "Sibling OU for SCIM authz integration test",
	})
	ts.Require().NoError(err, "create scim-authz OU2")
	ts.ou2ID = ou2ID

	ts.entityTypeOU1Name = "scim-authz-type-ou1"
	entityTypeOU1ID, err := testutils.CreateUserType(testutils.UserType{
		Name: ts.entityTypeOU1Name,
		OUID: ts.ou1ID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string", "required": true, "unique": true},
		},
	})
	ts.Require().NoError(err, "create entity type for OU1")
	ts.entityTypeOU1ID = entityTypeOU1ID

	ts.entityTypeOU2Name = "scim-authz-type-ou2"
	entityTypeOU2ID, err := testutils.CreateUserType(testutils.UserType{
		Name: ts.entityTypeOU2Name,
		OUID: ts.ou2ID,
		Schema: map[string]interface{}{
			"email": map[string]interface{}{"type": "string", "required": true, "unique": true},
		},
	})
	ts.Require().NoError(err, "create entity type for OU2")
	ts.entityTypeOU2ID = entityTypeOU2ID

	urn1, _, err := discoverExtensionSchema(ts.entityTypeOU1Name)
	ts.Require().NoError(err, "discover extension schema for OU1 type")
	ts.extensionURNOU1 = urn1

	urn2, _, err := discoverExtensionSchema(ts.entityTypeOU2Name)
	ts.Require().NoError(err, "discover extension schema for OU2 type")
	ts.extensionURNOU2 = urn2

	mgrUserID, err := testutils.CreateUser(testutils.User{
		Type: ts.entityTypeOU1Name,
		OUID: ts.ou1ID,
		Attributes: json.RawMessage(`{"username": "scim-authz-manager", ` +
			`"password": "ScimAuthzMgr@123", "email": "scim-authz-manager@example.com"}`),
	})
	ts.Require().NoError(err, "create scim-manager user")
	ts.mgrUserID = mgrUserID

	targetUserOU1ID, err := testutils.CreateUser(testutils.User{
		Type:       ts.entityTypeOU1Name,
		OUID:       ts.ou1ID,
		Attributes: json.RawMessage(`{"email": "scim-authz-target-ou1@example.com"}`),
	})
	ts.Require().NoError(err, "create target user in OU1")
	ts.targetUserOU1ID = targetUserOU1ID

	deletableUserOU1ID, err := testutils.CreateUser(testutils.User{
		Type:       ts.entityTypeOU1Name,
		OUID:       ts.ou1ID,
		Attributes: json.RawMessage(`{"email": "scim-authz-deletable-ou1@example.com"}`),
	})
	ts.Require().NoError(err, "create deletable user in OU1")
	ts.deletableUserOU1ID = deletableUserOU1ID

	targetUserOU2ID, err := testutils.CreateUser(testutils.User{
		Type:       ts.entityTypeOU2Name,
		OUID:       ts.ou2ID,
		Attributes: json.RawMessage(`{"email": "scim-authz-target-ou2@example.com"}`),
	})
	ts.Require().NoError(err, "create target user in OU2")
	ts.targetUserOU2ID = targetUserOU2ID

	targetGroupOU1ID, err := testutils.CreateGroup(testutils.Group{
		Name: "scim-authz-target-group-ou1",
		OUID: ts.ou1ID,
	})
	ts.Require().NoError(err, "create target group in OU1")
	ts.targetGroupOU1ID = targetGroupOU1ID

	deletableGroupOU1ID, err := testutils.CreateGroup(testutils.Group{
		Name: "scim-authz-deletable-group-ou1",
		OUID: ts.ou1ID,
	})
	ts.Require().NoError(err, "create deletable group in OU1")
	ts.deletableGroupOU1ID = deletableGroupOU1ID

	targetGroupOU2ID, err := testutils.CreateGroup(testutils.Group{
		Name: "scim-authz-target-group-ou2",
		OUID: ts.ou2ID,
	})
	ts.Require().NoError(err, "create target group in OU2")
	ts.targetGroupOU2ID = targetGroupOU2ID

	// The product ships only the root "system" scope; this reproduces the fine-grained
	// system:user/system:group/system:usertype:view scopes so the suite can verify
	// resource-level enforcement when configured (see tests/integration/user's equivalent).
	const scopedRSIdentifier = "https://authz-test.example.com/scim"
	systemRSID, err := testutils.CreateSystemScopedResourceServer(
		ts.ou1ID, "Authz Test RS (scim)", scopedRSIdentifier, "user", "usertype", "group")
	ts.Require().NoError(err, "create scoped resource server")
	ts.scopedRSID = systemRSID

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: scimAuthzMgrRoleName,
		OUID: ts.ou1ID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: systemRSID,
				Permissions: []string{
					"system:user", "system:user:view", "system:usertype:view",
					"system:group", "system:group:view",
				},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.mgrUserID, Type: "user"},
		},
	})
	ts.Require().NoError(err, "create scim-manager role")
	ts.roleID = roleID

	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		scimAuthzDevelopClientID,
		scimAuthzDevelopRedirectURI,
		"system system:user system:user:view system:usertype:view system:group system:group:view",
		scimAuthzMgrUsername,
		scimAuthzMgrPassword,
		true,
		"",
		scopedRSIdentifier,
	)
	ts.Require().NoError(err, "obtain scim-manager token")
	ts.Require().NotEmpty(tokenResp.AccessToken, "scim-manager token must be non-empty")

	ts.scopedClient = testutils.GetHTTPClientWithToken(tokenResp.AccessToken)
}

// ---------------------------------------------------------------------------
// Suite teardown
// ---------------------------------------------------------------------------

// TearDownSuite cleans up the test suite environment.
func (ts *SCIMAuthzTestSuite) TearDownSuite() {
	if ts.roleID != "" {
		if err := testutils.DeleteRole(ts.roleID); err != nil {
			ts.T().Logf("teardown: delete scim-manager role: %v", err)
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
	for _, id := range []string{ts.targetGroupOU1ID, ts.deletableGroupOU1ID, ts.targetGroupOU2ID} {
		if id != "" {
			if err := testutils.DeleteGroup(id); err != nil {
				ts.T().Logf("teardown: delete group %s: %v", id, err)
			}
		}
	}
	for _, id := range []string{ts.targetUserOU1ID, ts.deletableUserOU1ID, ts.mgrUserID, ts.targetUserOU2ID} {
		if id != "" {
			if err := testutils.DeleteUser(id); err != nil {
				ts.T().Logf("teardown: delete user %s: %v", id, err)
			}
		}
	}
	if ts.entityTypeOU1ID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeOU1ID); err != nil {
			ts.T().Logf("teardown: delete entity type OU1: %v", err)
		}
	}
	if ts.entityTypeOU2ID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeOU2ID); err != nil {
			ts.T().Logf("teardown: delete entity type OU2: %v", err)
		}
	}
	if ts.ou2ID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ou2ID); err != nil {
			ts.T().Logf("teardown: delete scim-authz OU2: %v", err)
		}
	}
	if ts.ou1ID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ou1ID); err != nil {
			ts.T().Logf("teardown: delete scim-authz OU1: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// doSCIM issues a request against /scim/v2 via the scim-manager's scoped
// client, mirroring scimRequest but with a caller-supplied, permission-
// restricted client instead of the suite-wide admin one.
// doSCIM handles do scim.
func (ts *SCIMAuthzTestSuite) doSCIM(method, path string, body []byte) (int, []byte) {
	ts.T().Helper()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, scimBaseURL+path, reader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/scim+json")
	}

	resp, err := ts.scopedClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, respBody
}

// buildUserPayload handles build user payload.
func (ts *SCIMAuthzTestSuite) buildUserPayload(extensionURN, email string) []byte {
	payload := map[string]interface{}{
		"schemas":    []string{scimCoreUserSchemaURN, extensionURN},
		"emails":     []map[string]interface{}{{"value": email, "type": "work"}},
		extensionURN: map[string]interface{}{},
	}
	b, err := json.Marshal(payload)
	ts.Require().NoError(err)
	return b
}

// ---------------------------------------------------------------------------
// Tests — SCIM Users, READ (system:user:view)
// ---------------------------------------------------------------------------

// TestListUsersOnlyReturnsOwnOU tests List Users Only Returns Own OU.
func (ts *SCIMAuthzTestSuite) TestListUsersOnlyReturnsOwnOU() {
	status, body := ts.doSCIM(http.MethodGet, "/Users", nil)
	ts.Require().Equal(http.StatusOK, status, "list users should succeed: %s", body)

	var list scimUserListResponse
	ts.Require().NoError(json.Unmarshal(body, &list))

	ids := make([]string, 0, len(list.Resources))
	for _, u := range list.Resources {
		ids = append(ids, u.ID)
	}
	ts.Contains(ids, ts.targetUserOU1ID, "list must include the target user in OU1")
	ts.NotContains(ids, ts.targetUserOU2ID, "list must NOT include the sibling OU2 user")
}

// TestGetUserInOwnOUAllowed tests Get User In Own OU Allowed.
func (ts *SCIMAuthzTestSuite) TestGetUserInOwnOUAllowed() {
	status, body := ts.doSCIM(http.MethodGet, "/Users/"+ts.targetUserOU1ID, nil)
	ts.Equal(http.StatusOK, status, "scim-manager should read a user in its own OU: %s", body)
}

// TestGetUserInOtherOUForbidden tests Get User In Other OU Forbidden.
func (ts *SCIMAuthzTestSuite) TestGetUserInOtherOUForbidden() {
	status, _ := ts.doSCIM(http.MethodGet, "/Users/"+ts.targetUserOU2ID, nil)
	ts.Equal(http.StatusForbidden, status, "scim-manager must be denied a user in a different OU")
}

// ---------------------------------------------------------------------------
// Tests — SCIM Users, WRITE (system:user)
// ---------------------------------------------------------------------------

// TestCreateUserInOwnOUAllowed tests Create User In Own OU Allowed.
func (ts *SCIMAuthzTestSuite) TestCreateUserInOwnOUAllowed() {
	body := ts.buildUserPayload(ts.extensionURNOU1, "scim-authz-created-ou1@example.com")
	status, respBody := ts.doSCIM(http.MethodPost, "/Users", body)
	ts.Require().Equal(http.StatusCreated, status, "scim-manager should create a user in its own OU: %s", respBody)

	var created map[string]interface{}
	if err := json.Unmarshal(respBody, &created); err == nil {
		if id, _ := created["id"].(string); id != "" {
			_ = testutils.DeleteUser(id)
		}
	}
}

// TestCreateUserInOtherOUForbidden tests Create User In Other OU Forbidden.
func (ts *SCIMAuthzTestSuite) TestCreateUserInOtherOUForbidden() {
	body := ts.buildUserPayload(ts.extensionURNOU2, "scim-authz-created-ou2@example.com")
	status, _ := ts.doSCIM(http.MethodPost, "/Users", body)
	ts.Equal(http.StatusForbidden, status, "scim-manager must not create a user in a different OU")
}

// TestReplaceUserInOwnOUAllowed tests Replace User In Own OU Allowed.
func (ts *SCIMAuthzTestSuite) TestReplaceUserInOwnOUAllowed() {
	body := ts.buildUserPayload(ts.extensionURNOU1, "scim-authz-target-ou1@example.com")
	status, respBody := ts.doSCIM(http.MethodPut, "/Users/"+ts.targetUserOU1ID, body)
	ts.Equal(http.StatusOK, status, "scim-manager should replace a user in its own OU: %s", respBody)
}

// TestReplaceUserInOtherOUForbidden tests Replace User In Other OU Forbidden.
func (ts *SCIMAuthzTestSuite) TestReplaceUserInOtherOUForbidden() {
	body := ts.buildUserPayload(ts.extensionURNOU2, "scim-authz-target-ou2@example.com")
	status, _ := ts.doSCIM(http.MethodPut, "/Users/"+ts.targetUserOU2ID, body)
	ts.Equal(http.StatusForbidden, status, "scim-manager must not replace a user in a different OU")
}

// TestDeleteUserInOwnOUAllowed tests Delete User In Own OU Allowed.
func (ts *SCIMAuthzTestSuite) TestDeleteUserInOwnOUAllowed() {
	status, body := ts.doSCIM(http.MethodDelete, "/Users/"+ts.deletableUserOU1ID, nil)
	ts.Require().Equal(http.StatusNoContent, status, "scim-manager should delete a user in its own OU: %s", body)
	ts.deletableUserOU1ID = ""
}

// TestDeleteUserInOtherOUForbidden tests Delete User In Other OU Forbidden.
func (ts *SCIMAuthzTestSuite) TestDeleteUserInOtherOUForbidden() {
	status, _ := ts.doSCIM(http.MethodDelete, "/Users/"+ts.targetUserOU2ID, nil)
	ts.Equal(http.StatusForbidden, status, "scim-manager must not delete a user in a different OU")
}

// ---------------------------------------------------------------------------
// Tests — SCIM Groups, READ (system:group:view)
// ---------------------------------------------------------------------------

// TestListGroupsOnlyReturnsOwnOU tests List Groups Only Returns Own OU.
func (ts *SCIMAuthzTestSuite) TestListGroupsOnlyReturnsOwnOU() {
	status, body := ts.doSCIM(http.MethodGet, "/Groups", nil)
	ts.Require().Equal(http.StatusOK, status, "list groups should succeed: %s", body)

	var list scimGroupListResponse
	ts.Require().NoError(json.Unmarshal(body, &list))

	ids := make([]string, 0, len(list.Resources))
	for _, g := range list.Resources {
		ids = append(ids, g.ID)
	}
	ts.Contains(ids, ts.targetGroupOU1ID, "list must include the target group in OU1")
	ts.NotContains(ids, ts.targetGroupOU2ID, "list must NOT include the sibling OU2 group")
}

// TestGetGroupInOwnOUAllowed tests Get Group In Own OU Allowed.
func (ts *SCIMAuthzTestSuite) TestGetGroupInOwnOUAllowed() {
	status, body := ts.doSCIM(http.MethodGet, "/Groups/"+ts.targetGroupOU1ID, nil)
	ts.Equal(http.StatusOK, status, "scim-manager should read a group in its own OU: %s", body)
}

// TestGetGroupInOtherOUForbidden tests Get Group In Other OU Forbidden.
func (ts *SCIMAuthzTestSuite) TestGetGroupInOtherOUForbidden() {
	status, _ := ts.doSCIM(http.MethodGet, "/Groups/"+ts.targetGroupOU2ID, nil)
	ts.Equal(http.StatusForbidden, status, "scim-manager must be denied a group in a different OU")
}

// ---------------------------------------------------------------------------
// Tests — SCIM Groups, WRITE (system:group)
// ---------------------------------------------------------------------------

// TestCreateGroupAlwaysLandsInOwnOU documents that SCIM Group create carries
// no OU field on the wire — the group always lands in the caller's own OU
// (security.GetOUID(ctx)), so there is no cross-OU create case to deny here.
// Immediate GET-back with the same scoped client is the observable proof the
// group landed in an OU the manager can access.
// TestCreateGroupAlwaysLandsInOwnOU tests Create Group Always Lands In Own OU.
func (ts *SCIMAuthzTestSuite) TestCreateGroupAlwaysLandsInOwnOU() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":     []string{scimCoreGroupSchemaURN},
		"displayName": "scim-authz-created-group",
	})
	ts.Require().NoError(err)

	status, respBody := ts.doSCIM(http.MethodPost, "/Groups", body)
	ts.Require().Equal(http.StatusCreated, status, "scim-manager should create a group: %s", respBody)

	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &created))
	id, _ := created["id"].(string)
	ts.Require().NotEmpty(id)
	defer func() { _ = testutils.DeleteGroup(id) }()

	status, getBody := ts.doSCIM(http.MethodGet, "/Groups/"+id, nil)
	ts.Equal(http.StatusOK, status, "the just-created group must be readable by its own creator: %s", getBody)
}

// TestReplaceGroupInOwnOUAllowed tests Replace Group In Own OU Allowed.
func (ts *SCIMAuthzTestSuite) TestReplaceGroupInOwnOUAllowed() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":     []string{scimCoreGroupSchemaURN},
		"displayName": "scim-authz-target-group-ou1",
	})
	ts.Require().NoError(err)

	status, respBody := ts.doSCIM(http.MethodPut, "/Groups/"+ts.targetGroupOU1ID, body)
	ts.Equal(http.StatusOK, status, "scim-manager should replace a group in its own OU: %s", respBody)
}

// TestReplaceGroupInOtherOUForbidden tests Replace Group In Other OU Forbidden.
func (ts *SCIMAuthzTestSuite) TestReplaceGroupInOtherOUForbidden() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":     []string{scimCoreGroupSchemaURN},
		"displayName": "scim-authz-target-group-ou2",
	})
	ts.Require().NoError(err)

	status, _ := ts.doSCIM(http.MethodPut, "/Groups/"+ts.targetGroupOU2ID, body)
	ts.Equal(http.StatusForbidden, status, "scim-manager must not replace a group in a different OU")
}

// TestPatchGroupInOwnOUAllowed tests Patch Group In Own OU Allowed.
func (ts *SCIMAuthzTestSuite) TestPatchGroupInOwnOUAllowed() {
	body, err := json.Marshal(scimPatchRequest{
		Schemas: []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{
			Op:    "replace",
			Path:  "displayName",
			Value: "scim-authz-target-group-ou1-renamed",
		}},
	})
	ts.Require().NoError(err)

	status, respBody := ts.doSCIM(http.MethodPatch, "/Groups/"+ts.targetGroupOU1ID, body)
	ts.Equal(http.StatusOK, status, "scim-manager should patch a group in its own OU: %s", respBody)
}

// TestPatchGroupInOtherOUForbidden tests Patch Group In Other OU Forbidden.
func (ts *SCIMAuthzTestSuite) TestPatchGroupInOtherOUForbidden() {
	body, err := json.Marshal(scimPatchRequest{
		Schemas: []string{scimPatchOpSchemaURN},
		Operations: []scimPatchOp{{
			Op:    "replace",
			Path:  "displayName",
			Value: "should-not-apply",
		}},
	})
	ts.Require().NoError(err)

	status, _ := ts.doSCIM(http.MethodPatch, "/Groups/"+ts.targetGroupOU2ID, body)
	ts.Equal(http.StatusForbidden, status, "scim-manager must not patch a group in a different OU")
}

// TestDeleteGroupInOwnOUAllowed tests Delete Group In Own OU Allowed.
func (ts *SCIMAuthzTestSuite) TestDeleteGroupInOwnOUAllowed() {
	status, body := ts.doSCIM(http.MethodDelete, "/Groups/"+ts.deletableGroupOU1ID, nil)
	ts.Require().Equal(http.StatusNoContent, status, "scim-manager should delete a group in its own OU: %s", body)
	ts.deletableGroupOU1ID = ""
}

// TestDeleteGroupInOtherOUForbidden tests Delete Group In Other OU Forbidden.
func (ts *SCIMAuthzTestSuite) TestDeleteGroupInOtherOUForbidden() {
	status, _ := ts.doSCIM(http.MethodDelete, "/Groups/"+ts.targetGroupOU2ID, nil)
	ts.Equal(http.StatusForbidden, status, "scim-manager must not delete a group in a different OU")
}
