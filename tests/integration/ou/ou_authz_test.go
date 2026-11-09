// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package ou contains integration tests for the OU API endpoints.
package ou

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

// OUAuthzTestSuite validates the OU authorization model end-to-end.
//
// This suite creates a custom resource server that declares the Data Plane's hierarchical
// permission structure:
//
//	Authz Test RS (ou)
//	├── Resource  "system"      → permission "system"
//	└── Resource  "org"         → permission "org"
//	    └── Resource  "ou"      → permission "org:ou"
//	        └── Action "view"   → permission "org:ou:view"
//
// The suite creates a test user scoped to OU1 and a role carrying the
// org:ou:view permission. It then obtains a view-only access token and
// verifies that:
//
//   - Read operations on OU1 and OU12 (its direct child) are allowed (200)
//   - Read operations on OU2 (sibling) are denied (403)
//   - Listing the root OUs is denied (403): only the system permission lists them
//   - All write operations are denied (403) because org:ou:view does not
//     satisfy org:ou
//
// Fixture topology:
//
//	OU1  (handle: authz-ou1)   ← ou-admin user belongs here
//	└── OU12 (handle: authz-ou12)
//	OU2  (handle: authz-ou2)
type OUAuthzTestSuite struct {
	suite.Suite

	// Users here live outside the default OU, and a sign-in is bounded to its application's OU, so each
	// signs in through an application in its own OU rather than the Console.
	signIn testutils.OUSignInApps

	// Admin-created fixtures
	ou1ID  string
	ou2ID  string
	ou12ID string

	// Test-specific role and OU-admin user
	ouAdminRoleID string
	ouAdminUserID string
	scopedRSID    string

	// HTTP client that carries the OU-admin's view-only access token
	ouViewClient *http.Client
}

const (
	authzOU1Handle  = "authz-ou1"
	authzOU2Handle  = "authz-ou2"
	authzOU12Handle = "authz-ou12"

	ouAdminUsername = "ou-authz-admin"
	ouAdminPassword = "OUAdmin@123"

	// Name of the role created in SetupSuite. Using a unique name avoids
	// collisions when tests run multiple times without a clean DB.
	ouViewRoleName = "OU View Admin (authz-test)"
)

var developRedirectURI = testutils.TestServerURL + "/console"

var authzTestServerURL = testutils.TestServerURL

// authzEntityTypeID persists the entity type ID across SetupSuite/TearDownSuite.
var authzEntityTypeID string

func TestOUAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(OUAuthzTestSuite))
}

// ---------------------------------------------------------------------------
// Suite setup
// ---------------------------------------------------------------------------

func (ts *OUAuthzTestSuite) SetupSuite() {
	// ---- 1. Create test OUs ----
	ou1, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      authzOU1Handle,
		Name:        "Authz Test OU1",
		Description: "Primary OU for authz integration test",
	})
	ts.Require().NoError(err, "create OU1")
	ts.ou1ID = ou1

	ou2, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      authzOU2Handle,
		Name:        "Authz Test OU2",
		Description: "Sibling OU for authz integration test",
	})
	ts.Require().NoError(err, "create OU2")
	ts.ou2ID = ou2

	ou12, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      authzOU12Handle,
		Name:        "Authz Test OU12 (child of OU1)",
		Description: "Child OU under OU1 for authz integration test",
		Parent:      &ts.ou1ID,
	})
	ts.Require().NoError(err, "create OU12 (child of OU1)")
	ts.ou12ID = ou12

	// ---- 2. Create a minimal user type in OU1 ----
	schema := testutils.UserType{
		Handle:                "ou-authz-admin-schema",
		DisplayName:           "Ou Authz Admin Schema",
		OUID:                  ts.ou1ID,
		AllowSelfRegistration: false,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	}
	schemaID, err := testutils.CreateUserType(schema)
	ts.Require().NoError(err, "create ou-admin user type")
	authzEntityTypeID = schemaID

	// ---- 3. Create the OU-admin user in OU1 ----
	userID, err := testutils.CreateUser(testutils.User{
		Type: schema.Handle,
		OUID: ts.ou1ID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q}`,
			ouAdminUsername, ouAdminPassword,
		)),
	})
	ts.Require().NoError(err, "create ou-admin user")
	ts.ouAdminUserID = userID

	// ---- 4. Create a custom resource server declaring the fine-grained org scopes ----
	// This reproduces the Data Plane permission "org:ou:view" so the suite can verify
	// resource-level enforcement.
	const scopedRSIdentifier = "https://authz-test.example.com/ou"
	systemRSID, err := testutils.CreateOrgScopedResourceServer(
		ts.ou1ID, "Authz Test RS (ou)", scopedRSIdentifier, "ou")
	ts.Require().NoError(err, "create scoped resource server")
	ts.scopedRSID = systemRSID

	// ---- 5. Create a role with permission org:ou:view ----
	role := testutils.Role{
		Name: ouViewRoleName,
		OUID: ts.ou1ID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: systemRSID,
				Permissions:      []string{"org:ou:view"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.ouAdminUserID, Type: "user"},
		},
	}
	roleID, err := testutils.CreateRole(role)
	ts.Require().NoError(err, "create ou-view-admin role")
	ts.ouAdminRoleID = roleID

	// ---- 6. Obtain a scoped access token for the OU-admin user ----
	tokenResp, err := ts.signIn.ObtainAccessToken(ts.ou1ID,
		"system org:ou:view", ouAdminUsername, ouAdminPassword, scopedRSIdentifier)
	ts.Require().NoError(err, "obtain ou-admin scoped token")
	ts.Require().NotEmpty(tokenResp.AccessToken, "ou-admin token must be non-empty")
	ts.Require().Equal("org:ou:view", tokenResp.Scope, "token must carry exactly the requested scope")

	ts.ouViewClient = testutils.GetHTTPClientWithToken(tokenResp.AccessToken)
}

// ---------------------------------------------------------------------------
// Suite teardown
// ---------------------------------------------------------------------------

func (ts *OUAuthzTestSuite) TearDownSuite() {
	if err := ts.signIn.DeleteAll(); err != nil {
		ts.T().Logf("teardown: delete sign-in applications: %v", err)
	}
	if ts.ouAdminRoleID != "" {
		if err := testutils.DeleteRole(ts.ouAdminRoleID); err != nil {
			ts.T().Logf("teardown: delete role: %v", err)
		}
	}
	if ts.ouAdminUserID != "" {
		if err := testutils.DeleteUser(ts.ouAdminUserID); err != nil {
			ts.T().Logf("teardown: delete ou-admin user: %v", err)
		}
	}
	if ts.scopedRSID != "" {
		if err := testutils.DeleteResourceServerWithChildren(ts.scopedRSID); err != nil {
			ts.T().Errorf("teardown: delete scoped resource server: %v", err)
		}
	}
	if authzEntityTypeID != "" {
		if err := testutils.DeleteUserType(authzEntityTypeID); err != nil {
			ts.T().Logf("teardown: delete user type: %v", err)
		}
	}
	// Delete child OU before parent.
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
// Helper — issue an HTTP request via the OU-view-only client
// ---------------------------------------------------------------------------

func (ts *OUAuthzTestSuite) do(method, path string, body []byte) *http.Response {
	ts.T().Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, authzTestServerURL+path, bodyReader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := ts.ouViewClient.Do(req)
	ts.Require().NoError(err)
	return resp
}

func closeBody(resp *http.Response) { _ = resp.Body.Close() }

// ---------------------------------------------------------------------------
// Tests — READ operations (org:ou:view is sufficient)
// ---------------------------------------------------------------------------

// TestListOUs verifies that an OU-scoped admin may not list the root OUs: an organization permission
// reaches only the unit its token names, so only the system permission lists them.
func (ts *OUAuthzTestSuite) TestListOUs() {
	resp := ts.do(http.MethodGet, "/organization-units", nil)
	defer closeBody(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode, "an org:ou:view token must not list the root OUs")
}

// TestListOwnChildOUs verifies that the OU-admin lists the direct children of OU1, and not those of OU12.
func (ts *OUAuthzTestSuite) TestListOwnChildOUs() {
	resp := ts.do(http.MethodGet, "/organization-units/"+ts.ou1ID+"/ous", nil)
	defer closeBody(resp)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "list OU1's children should succeed")

	var listResp OrganizationUnitListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))
	ids := make([]string, 0, len(listResp.OrganizationUnits))
	for _, ou := range listResp.OrganizationUnits {
		ids = append(ids, ou.ID)
	}
	ts.Containsf(ids, ts.ou12ID, "OU1's children must include OU12, got IDs: %v", ids)

	childResp := ts.do(http.MethodGet, "/organization-units/"+ts.ou12ID+"/ous", nil)
	defer closeBody(childResp)
	ts.Equal(http.StatusForbidden, childResp.StatusCode, "the children of a child OU are out of reach")
}

// TestGetOwnOUByID verifies that the OU-admin can read their own OU by ID.
func (ts *OUAuthzTestSuite) TestGetOwnOUByID() {
	resp := ts.do(http.MethodGet, "/organization-units/"+ts.ou1ID, nil)
	defer closeBody(resp)

	ts.Equal(http.StatusOK, resp.StatusCode, "OU-admin should be able to read own OU by ID")
}

// TestGetSiblingOUByID verifies the OU-admin is denied access to OU2 by ID.
func (ts *OUAuthzTestSuite) TestGetSiblingOUByID() {
	resp := ts.do(http.MethodGet, "/organization-units/"+ts.ou2ID, nil)
	defer closeBody(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"OU-admin should be denied access to sibling OU (OU2) by ID")
}

// TestGetChildOUByID verifies the OU-admin reads OU12, a direct child of OU1, by ID.
func (ts *OUAuthzTestSuite) TestGetChildOUByID() {
	resp := ts.do(http.MethodGet, "/organization-units/"+ts.ou12ID, nil)
	defer closeBody(resp)

	ts.Equal(http.StatusOK, resp.StatusCode, "OU-admin should read a direct child OU (OU12) by ID")
}

// TestGetOwnOUByPath verifies the OU-admin can read their own OU by handle path.
func (ts *OUAuthzTestSuite) TestGetOwnOUByPath() {
	resp := ts.do(http.MethodGet, "/organization-units/tree/"+authzOU1Handle, nil)
	defer closeBody(resp)

	ts.Equal(http.StatusOK, resp.StatusCode,
		"OU-admin should be able to read own OU by handle path")
}

// TestGetSiblingOUByPath verifies the OU-admin is denied access to OU2 by handle path.
func (ts *OUAuthzTestSuite) TestGetSiblingOUByPath() {
	resp := ts.do(http.MethodGet, "/organization-units/tree/"+authzOU2Handle, nil)
	defer closeBody(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"OU-admin should be denied access to sibling OU (OU2) by path")
}

// ---------------------------------------------------------------------------
// Tests — WRITE operations (require org:ou; view-only token must be denied)
// ---------------------------------------------------------------------------

func (ts *OUAuthzTestSuite) newOUPayload(handle, name string, parent *string) []byte {
	ts.T().Helper()
	req := CreateOURequest{Handle: handle, Name: name}
	if parent != nil {
		req.Parent = parent
	}
	b, err := json.Marshal(req)
	ts.Require().NoError(err)
	return b
}

// TestCreateOUAtRoot verifies the view-only OU-admin cannot create a root OU.
func (ts *OUAuthzTestSuite) TestCreateOUAtRoot() {
	payload := ts.newOUPayload("authz-new-root-ou", "Authz New Root OU", nil)
	resp := ts.do(http.MethodPost, "/organization-units", payload)
	defer closeBody(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"view-only OU-admin must not create a root-level OU")
}

// TestCreateOUUnderOwnOU verifies the view-only OU-admin cannot create a child of OU1.
func (ts *OUAuthzTestSuite) TestCreateOUUnderOwnOU() {
	payload := ts.newOUPayload("authz-new-child-ou", "Authz New Child OU", &ts.ou1ID)
	resp := ts.do(http.MethodPost, "/organization-units", payload)
	defer closeBody(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"view-only OU-admin must not create a child OU (org:ou required)")
}

// TestUpdateOwnOU verifies the view-only OU-admin cannot update OU1.
func (ts *OUAuthzTestSuite) TestUpdateOwnOU() {
	updatePayload, err := json.Marshal(UpdateOURequest{
		Handle: authzOU1Handle,
		Name:   "Authz Test OU1 (modified)",
	})
	ts.Require().NoError(err)

	resp := ts.do(http.MethodPut, "/organization-units/"+ts.ou1ID, updatePayload)
	defer closeBody(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"view-only OU-admin must not update own OU (org:ou required)")
}

// TestDeleteOwnOU verifies the view-only OU-admin cannot delete OU1.
func (ts *OUAuthzTestSuite) TestDeleteOwnOU() {
	resp := ts.do(http.MethodDelete, "/organization-units/"+ts.ou1ID, nil)
	defer closeBody(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"view-only OU-admin must not delete own OU (org:ou required)")
}

// TestListOUsWithFilterMatch verifies that a filter applied to the children of the OU-admin's own OU
// returns the children that match the filter expression.
func (ts *OUAuthzTestSuite) TestListOUsWithFilterMatch() {
	// "Authz Test OU12 (child of OU1)" is the name set in SetupSuite for OU12.
	resp := ts.doWithFilter("/organization-units/"+ts.ou1ID+"/ous", `name eq "Authz Test OU12 (child of OU1)"`)
	defer closeBody(resp)

	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ids := make([]string, 0, len(listResp.OrganizationUnits))
	for _, ou := range listResp.OrganizationUnits {
		ids = append(ids, ou.ID)
	}

	ts.Containsf(ids, ts.ou12ID,
		"filter matching OU12's name should return OU12; got IDs: %v", ids)
}

// TestListOUsWithFilterNoMatch verifies that a filter that matches none of the OU-admin's child OUs
// returns an empty list.
func (ts *OUAuthzTestSuite) TestListOUsWithFilterNoMatch() {
	resp := ts.doWithFilter("/organization-units/"+ts.ou1ID+"/ous", `name eq "__no_match__"`)
	defer closeBody(resp)

	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ts.Equal(0, listResp.TotalResults,
		"filter that matches no child OU should return empty results")
	ts.Empty(listResp.OrganizationUnits)
}

// doWithFilter issues a GET request using the scoped OU-admin client with the
// given filter expression as the "filter" query parameter.
func (ts *OUAuthzTestSuite) doWithFilter(path, filterExpr string) *http.Response {
	ts.T().Helper()

	u, err := url.Parse(authzTestServerURL + path)
	ts.Require().NoError(err)
	q := u.Query()
	q.Set("filter", filterExpr)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	ts.Require().NoError(err)

	resp, err := ts.ouViewClient.Do(req)
	ts.Require().NoError(err)
	return resp
}

// TestListOUsOffsetBeyondAuthorizedSet verifies that paging past the end of the OU-admin's child OUs
// returns an empty page while still reporting the true total.
func (ts *OUAuthzTestSuite) TestListOUsOffsetBeyondAuthorizedSet() {
	resp := ts.do(http.MethodGet, "/organization-units/"+ts.ou1ID+"/ous?limit=10&offset=1000", nil)
	defer closeBody(resp)

	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ts.Empty(listResp.OrganizationUnits,
		"an offset past the child OUs must return no organization units")
	ts.Equal(0, listResp.Count, "count must be zero for an empty page")
	ts.GreaterOrEqual(listResp.TotalResults, 1,
		"total results must still report the number of child OUs")
}

// ---------------------------------------------------------------------------
// Tests — the root "system" permission across the deployment
// ---------------------------------------------------------------------------

// TestSystemScopeReachesEveryOU verifies the administrator's "system" permission reaches an OU
// other than the one its token names.
//
// TODO: Change this to a refusal when the root "system" permission is confined to the token's
// organization unit under OU-bounded authorization.
func (ts *OUAuthzTestSuite) TestSystemScopeReachesEveryOU() {
	req, err := http.NewRequest(http.MethodGet, authzTestServerURL+"/organization-units/"+ts.ou1ID, nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer closeBody(resp)

	ts.Equal(http.StatusOK, resp.StatusCode,
		"the system permission must reach an OU other than the token's")
}

// TestSystemScopeCannotCreateRootOU verifies a root OU is not created through the API, even with the
// "system" permission. A root OU reaches the data plane only through import with the management API
// key.
func (ts *OUAuthzTestSuite) TestSystemScopeCannotCreateRootOU() {
	payload := ts.newOUPayload("authz-admin-root-ou", "Authz Admin Root OU", nil)
	req, err := http.NewRequest(http.MethodPost, authzTestServerURL+"/organization-units",
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer closeBody(resp)

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusForbidden, resp.StatusCode,
		"the system permission must not create a root OU through the API: %s", string(body))

	var errResp testutils.ErrorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp), "error body: %s", string(body))
	ts.Equal("SSE-4030", errResp.Code)
}
