// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package usertype

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

// UserTypeAuthzTestSuite validates the user type authorization model end-to-end.
//
// The product ships only the root "system" scope by default. This suite creates a custom
// resource server that declares the following hierarchical permission structure, simulating an
// operator configuring fine-grained scopes:
//
//	Authz Test RS (usertype)
//	└── Resource  "system"           → permission "system"
//	    └── Resource  "usertype"   → permission "system:usertype"
//	        └── Action "view"        → permission "system:usertype:view"
//
// The suite creates an OU hierarchy (OU1 → OU12, OU2 as sibling), a test
// user scoped to OU12, and a role carrying system:usertype permission.
// It then obtains a scoped access token and verifies that authorization is
// deployment-bounded: the token's scope applies to every OU in the deployment,
// regardless of the OU the token names.
//
//   - READ operations are allowed on user types in any OU (ancestor OU1 and
//     sibling OU2 alike), and listing returns user types from every OU
//   - WRITE operations are allowed in any OU (create in sibling OU2, update
//     and delete in ancestor OU1)
//   - WRITE operations on OU12 (own OU) are allowed (create, update, delete)
//
// Fixture topology:
//
//	OU1  (handle: schema-authz-ou1)   ← user type created here
//	└── OU12 (handle: schema-authz-ou12) ← user belongs here
//	OU2  (handle: schema-authz-ou2)   ← separate user type created here
type UserTypeAuthzTestSuite struct {
	suite.Suite

	// Admin-created fixture IDs
	ou1ID  string
	ou2ID  string
	ou12ID string

	// user types created via admin in OU1 and OU2
	ou1SchemaID string
	ou2SchemaID string

	// Test-specific role and user
	schemaAdminRoleID string
	schemaAdminUserID string
	scopedRSID        string

	// Schema created by the scoped user during write tests
	ou12SchemaID string

	// HTTP client that carries the scoped user's access token
	schemaClient *http.Client
}

const (
	authzSchemaTestServerURL = testutils.TestServerURL

	schemaAuthzOU1Handle  = "schema-authz-ou1"
	schemaAuthzOU2Handle  = "schema-authz-ou2"
	schemaAuthzOU12Handle = "schema-authz-ou12"

	schemaAdminUsername = "schema-authz-admin"
	schemaAdminPassword = "SchemaAdmin@123"

	schemaAuthzDevelopClientID    = "CONSOLE"
	schemaAuthzDevelopRedirectURI = "https://localhost:8095/console"

	// Unique role name to avoid collisions across test runs.
	schemaAdminRoleName = "User Type Admin (authz-test)"
)

// schemaAuthzUserTypeID persists the user type ID used to create the test
// user across SetupSuite/TearDownSuite.
var schemaAuthzUserTypeID string

func TestUserTypeAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(UserTypeAuthzTestSuite))
}

// ---------------------------------------------------------------------------
// Suite setup
// ---------------------------------------------------------------------------

func (ts *UserTypeAuthzTestSuite) SetupSuite() {
	// ---- 1. Create test OUs ----
	ou1, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      schemaAuthzOU1Handle,
		Name:        "Schema Authz Test OU1",
		Description: "Primary OU for user type authz integration test",
	})
	ts.Require().NoError(err, "create OU1")
	ts.ou1ID = ou1

	ou2, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      schemaAuthzOU2Handle,
		Name:        "Schema Authz Test OU2",
		Description: "Sibling OU for user type authz integration test",
	})
	ts.Require().NoError(err, "create OU2")
	ts.ou2ID = ou2

	ou12, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      schemaAuthzOU12Handle,
		Name:        "Schema Authz Test OU12 (child of OU1)",
		Description: "Child OU under OU1 for user type authz integration test",
		Parent:      &ts.ou1ID,
	})
	ts.Require().NoError(err, "create OU12 (child of OU1)")
	ts.ou12ID = ou12

	// ---- 2. Create user types in OU1 and OU2 ----
	ou1Schema := testutils.UserType{
		Handle:                "schema-authz-ou1-schema",
		DisplayName:           "Schema Authz Ou1 Schema",
		OUID:                  ts.ou1ID,
		AllowSelfRegistration: false,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	}
	ou1SchemaID, err := testutils.CreateUserType(ou1Schema)
	ts.Require().NoError(err, "create user type in OU1")
	ts.ou1SchemaID = ou1SchemaID
	schemaAuthzUserTypeID = ou1SchemaID

	ou2Schema := testutils.UserType{
		Handle:                "schema-authz-ou2-schema",
		DisplayName:           "Schema Authz Ou2 Schema",
		OUID:                  ts.ou2ID,
		AllowSelfRegistration: false,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	}
	ou2SchemaID, err := testutils.CreateUserType(ou2Schema)
	ts.Require().NoError(err, "create user type in OU2")
	ts.ou2SchemaID = ou2SchemaID

	// ---- 3. Create the test user in OU12 (uses OU1's schema via inheritance) ----
	userID, err := testutils.CreateUser(testutils.User{
		Type: ou1Schema.Handle,
		OUID: ts.ou12ID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q}`,
			schemaAdminUsername, schemaAdminPassword,
		)),
	})
	ts.Require().NoError(err, "create schema-admin user in OU12")
	ts.schemaAdminUserID = userID

	// ---- 4. Create a custom resource server declaring the fine-grained system scopes ----
	// The product ships only the root "system" scope; this reproduces "system:usertype" so the
	// suite can verify resource-level enforcement when configured.
	const scopedRSIdentifier = "https://authz-test.example.com/usertype"
	systemRSID, err := testutils.CreateSystemScopedResourceServer(
		ts.ou12ID, "Authz Test RS (usertype)", scopedRSIdentifier, "usertype")
	ts.Require().NoError(err, "create scoped resource server")
	ts.scopedRSID = systemRSID

	// ---- 5. Create a role with system:usertype permission ----
	role := testutils.Role{
		Name: schemaAdminRoleName,
		OUID: ts.ou12ID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: systemRSID,
				Permissions:      []string{"system:usertype"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.schemaAdminUserID, Type: "user"},
		},
	}
	roleID, err := testutils.CreateRole(role)
	ts.Require().NoError(err, "create schema-admin role")
	ts.schemaAdminRoleID = roleID

	// ---- 6. Obtain a scoped access token for the test user ----
	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		schemaAuthzDevelopClientID,
		schemaAuthzDevelopRedirectURI,
		"system system:usertype",
		schemaAdminUsername,
		schemaAdminPassword,
		true,
		"",
		scopedRSIdentifier,
	)
	ts.Require().NoError(err, "obtain schema-admin scoped token")
	ts.Require().NotEmpty(tokenResp.AccessToken, "schema-admin token must be non-empty")
	grantedScopes := strings.Fields(tokenResp.Scope)
	ts.Require().Contains(grantedScopes, "system:usertype", "token must carry usertype scope")
	ts.schemaClient = testutils.GetHTTPClientWithToken(tokenResp.AccessToken)
}

// ---------------------------------------------------------------------------
// Suite teardown
// ---------------------------------------------------------------------------

func (ts *UserTypeAuthzTestSuite) TearDownSuite() {
	// Delete role first (has assignment to user).
	if ts.schemaAdminRoleID != "" {
		if err := testutils.DeleteRole(ts.schemaAdminRoleID); err != nil {
			ts.T().Logf("teardown: delete role: %v", err)
		}
	}
	if ts.scopedRSID != "" {
		if err := testutils.DeleteResourceServerWithChildren(ts.scopedRSID); err != nil {
			ts.T().Errorf("teardown: delete scoped resource server: %v", err)
		}
	}
	// Delete the test user.
	if ts.schemaAdminUserID != "" {
		if err := testutils.DeleteUser(ts.schemaAdminUserID); err != nil {
			ts.T().Logf("teardown: delete schema-admin user: %v", err)
		}
	}
	// Safety cleanup: delete OU12 schema if the delete test didn't run or failed.
	if ts.ou12SchemaID != "" {
		if err := testutils.DeleteUserType(ts.ou12SchemaID); err != nil {
			ts.T().Logf("teardown: delete OU12 schema (safety): %v", err)
		}
	}
	// Delete user types.
	if ts.ou2SchemaID != "" {
		if err := testutils.DeleteUserType(ts.ou2SchemaID); err != nil {
			ts.T().Logf("teardown: delete OU2 schema: %v", err)
		}
	}
	if schemaAuthzUserTypeID != "" {
		if err := testutils.DeleteUserType(schemaAuthzUserTypeID); err != nil {
			ts.T().Logf("teardown: delete OU1 schema: %v", err)
		}
	}
	// Delete child OU before parents.
	if ts.ou12ID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ou12ID); err != nil {
			ts.T().Logf("teardown: delete OU12: %v", err)
		}
	}
	if ts.ou2ID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ou2ID); err != nil {
			ts.T().Logf("teardown: delete OU2: %v", err)
		}
	}
	if ts.ou1ID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ou1ID); err != nil {
			ts.T().Logf("teardown: delete OU1: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// Helper — issue an HTTP request via the scoped client
// ---------------------------------------------------------------------------

func (ts *UserTypeAuthzTestSuite) do(method, path string, body []byte) *http.Response {
	ts.T().Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, authzSchemaTestServerURL+path, bodyReader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := ts.schemaClient.Do(req)
	ts.Require().NoError(err)
	return resp
}

func closeBodyAuthz(resp *http.Response) { _ = resp.Body.Close() }

// createAdminUserType creates a user type in the given OU with the admin client
// and returns its ID.
func (ts *UserTypeAuthzTestSuite) createAdminUserType(handle, ouID string) string {
	ts.T().Helper()

	id, err := testutils.CreateUserType(testutils.UserType{
		Handle:      handle,
		DisplayName: handle,
		OUID:        ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
		},
	})
	ts.Require().NoError(err, "create user type %s", handle)
	return id
}

// ---------------------------------------------------------------------------
// Tests — READ operations (the scope applies to every OU)
// ---------------------------------------------------------------------------

// TestListUserTypes verifies that the scoped user can list user types and
// that the result includes user types from every OU, not only its own
// hierarchy.
func (ts *UserTypeAuthzTestSuite) TestListUserTypes() {
	resp := ts.do(http.MethodGet, "/user-types?limit=100", nil)
	defer closeBodyAuthz(resp)

	ts.Require().Equal(http.StatusOK, resp.StatusCode, "list user types should succeed")

	var listResp UserTypeListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ids := make([]string, 0, len(listResp.Types))
	for _, s := range listResp.Types {
		ids = append(ids, s.ID)
	}

	ts.Containsf(ids, ts.ou1SchemaID,
		"list must include OU1 schema (ancestor OU), got IDs: %v", ids)
	ts.Containsf(ids, ts.ou2SchemaID,
		"list must include OU2 schema (sibling OU), got IDs: %v", ids)
}

// TestGetAncestorOUSchema verifies the scoped user can read the parent OU's
// schema by ID.
func (ts *UserTypeAuthzTestSuite) TestGetAncestorOUSchema() {
	resp := ts.do(http.MethodGet, "/user-types/"+ts.ou1SchemaID, nil)
	defer closeBodyAuthz(resp)

	ts.Equal(http.StatusOK, resp.StatusCode,
		"scoped user should be able to read ancestor OU1's schema")
}

// TestGetSiblingOUSchema verifies the scoped user can read OU2's schema even
// though OU2 is outside the user's OU hierarchy.
func (ts *UserTypeAuthzTestSuite) TestGetSiblingOUSchema() {
	resp := ts.do(http.MethodGet, "/user-types/"+ts.ou2SchemaID, nil)
	defer closeBodyAuthz(resp)

	ts.Equal(http.StatusOK, resp.StatusCode,
		"scoped user should be able to read sibling OU2's schema")
}

// ---------------------------------------------------------------------------
// Tests — WRITE operations allowed outside own OU (the scope applies to every OU)
// ---------------------------------------------------------------------------

// TestUpdateAncestorOUSchema verifies that the scoped user can update a schema
// in OU1. A dedicated user type is used so the shared fixtures stay intact.
func (ts *UserTypeAuthzTestSuite) TestUpdateAncestorOUSchema() {
	schemaID := ts.createAdminUserType("schema-authz-ou1-update", ts.ou1ID)
	defer func() {
		if err := testutils.DeleteUserType(schemaID); err != nil {
			ts.T().Logf("cleanup: delete OU1 update schema: %v", err)
		}
	}()

	payload, err := json.Marshal(UpdateUserTypeRequest{
		Handle:      "schema-authz-ou1-update",
		DisplayName: "Schema Authz Ou1 Update Updated",
		OUID:        ts.ou1ID,
		Schema:      json.RawMessage(`{"username": {"type": "string", "unique": true}}`),
	})
	ts.Require().NoError(err)

	resp := ts.do(http.MethodPut, "/user-types/"+schemaID, payload)
	defer closeBodyAuthz(resp)

	ts.Equal(http.StatusOK, resp.StatusCode,
		"scoped user should be able to update ancestor OU1's schema")
}

// TestDeleteAncestorOUSchema verifies that the scoped user can delete a schema
// in OU1. A dedicated user type is used so the shared fixtures stay intact.
func (ts *UserTypeAuthzTestSuite) TestDeleteAncestorOUSchema() {
	schemaID := ts.createAdminUserType("schema-authz-ou1-delete", ts.ou1ID)

	resp := ts.do(http.MethodDelete, "/user-types/"+schemaID, nil)
	defer closeBodyAuthz(resp)

	if !ts.Equal(http.StatusNoContent, resp.StatusCode,
		"scoped user should be able to delete ancestor OU1's schema") {
		if err := testutils.DeleteUserType(schemaID); err != nil {
			ts.T().Logf("cleanup: delete OU1 delete schema: %v", err)
		}
	}
}

// TestCreateSchemaInSiblingOU verifies that the scoped user can create a
// schema in OU2 even though OU2 is outside the user's OU hierarchy.
func (ts *UserTypeAuthzTestSuite) TestCreateSchemaInSiblingOU() {
	payload, err := json.Marshal(CreateUserTypeRequest{
		Handle:      "schema-authz-ou2-created",
		DisplayName: "Schema Authz Ou2 Created",
		OUID:        ts.ou2ID,
		Schema:      json.RawMessage(`{"username": {"type": "string", "unique": true}}`),
	})
	ts.Require().NoError(err)

	resp := ts.do(http.MethodPost, "/user-types", payload)
	defer closeBodyAuthz(resp)

	ts.Require().Equal(http.StatusCreated, resp.StatusCode,
		"scoped user should be able to create a schema in sibling OU2")

	var created UserType
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&created))
	ts.Require().NotEmpty(created.ID, "created schema must have an ID")

	if err := testutils.DeleteUserType(created.ID); err != nil {
		ts.T().Errorf("cleanup: delete OU2 created schema: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests — WRITE operations allowed on own OU (OU12)
// ---------------------------------------------------------------------------

// TestOwnOUSchemaLifecycle exercises the full create → get → update → delete
// lifecycle on the user's own OU (OU12). These steps must run sequentially so
// they are combined into a single test method.
func (ts *UserTypeAuthzTestSuite) TestOwnOUSchemaLifecycle() {
	// ---- Create ----
	createPayload, err := json.Marshal(CreateUserTypeRequest{
		Handle:      "schema-authz-ou12-schema",
		DisplayName: "Schema Authz Ou12 Schema",
		OUID:        ts.ou12ID,
		Schema: json.RawMessage(`{
			"username": {"type": "string", "unique": true},
			"password": {"type": "string", "credential": true},
			"email":    {"type": "string"}
		}`),
	})
	ts.Require().NoError(err)

	createResp := ts.do(http.MethodPost, "/user-types", createPayload)
	defer closeBodyAuthz(createResp)

	ts.Require().Equal(http.StatusCreated, createResp.StatusCode,
		"scoped user should be able to create a schema in own OU12")

	var created UserType
	bodyBytes, err := io.ReadAll(createResp.Body)
	ts.Require().NoError(err)
	ts.Require().NoError(json.Unmarshal(bodyBytes, &created))
	ts.Require().NotEmpty(created.ID, "created schema must have an ID")

	ou12SchemaID := created.ID
	ts.ou12SchemaID = ou12SchemaID // keep for safety cleanup in TearDownSuite

	// ---- Get ----
	getResp := ts.do(http.MethodGet, "/user-types/"+ou12SchemaID, nil)
	defer closeBodyAuthz(getResp)

	ts.Equal(http.StatusOK, getResp.StatusCode,
		"scoped user should be able to read own OU12's schema")

	// ---- Update ----
	updatePayload, err := json.Marshal(UpdateUserTypeRequest{
		Handle:      "schema-authz-ou12-schema",
		DisplayName: "Schema Authz Ou12 Schema Updated",
		OUID:        ts.ou12ID,
		Schema: json.RawMessage(`{
			"username":  {"type": "string", "unique": true},
			"password":  {"type": "string", "credential": true},
			"email":     {"type": "string"},
			"given_name": {"type": "string"}
		}`),
	})
	ts.Require().NoError(err)

	updateResp := ts.do(http.MethodPut, "/user-types/"+ou12SchemaID, updatePayload)
	defer closeBodyAuthz(updateResp)

	ts.Equal(http.StatusOK, updateResp.StatusCode,
		"scoped user should be able to update own OU12's schema")

	// ---- Delete ----
	deleteResp := ts.do(http.MethodDelete, "/user-types/"+ou12SchemaID, nil)
	defer closeBodyAuthz(deleteResp)

	ts.Equal(http.StatusNoContent, deleteResp.StatusCode,
		"scoped user should be able to delete own OU12's schema")

	// Clear so TearDownSuite doesn't try to double-delete.
	ts.ou12SchemaID = ""
}
