/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

// OUAuthzTestSuite validates the OU authorization model end-to-end.
//
// The bootstrap script (01-default-resources.sh/.ps1) seeds the following
// hierarchical permission structure under the "system" resource server:
//
//	system RS  (name: "System")
//	└── Resource  "system"      → permission "system"
//	    └── Resource  "ou"      → permission "system:ou"
//	        └── Action "view"   → permission "system:ou:view"
//
// The suite creates a test user scoped to OU1 and a role carrying the
// system:ou:view permission. It then obtains a view-only access token and
// verifies that:
//
//   - Read operations on OU1 are allowed (200)
//   - Read operations on OU2 (sibling) and OU12 (child) are denied (403)
//   - All write operations are denied (403) because system:ou:view does not
//     satisfy system:ou
//
// Fixture topology:
//
//	OU1  (handle: authz-ou1)   ← ou-admin user belongs here
//	└── OU12 (handle: authz-ou12)
//	OU2  (handle: authz-ou2)
type OUAuthzTestSuite struct {
	suite.Suite

	// Admin-created fixtures
	ou1ID  string
	ou2ID  string
	ou12ID string

	// Test-specific role and OU-admin user
	ouAdminRoleID string
	ouAdminUserID string

	// HTTP client that carries the OU-admin's view-only access token
	ouViewClient *http.Client
}

const (
	authzTestServerURL = testutils.TestServerURL

	authzOU1Handle  = "authz-ou1"
	authzOU2Handle  = "authz-ou2"
	authzOU12Handle = "authz-ou12"

	ouAdminUsername = "ou-authz-admin"
	ouAdminPassword = "OUAdmin@123"

	developClientID    = "CONSOLE"
	developRedirectURI = "https://localhost:8095/console"

	// Name of the role created in SetupSuite. Using a unique name avoids
	// collisions when tests run multiple times without a clean DB.
	ouViewRoleName = "OU View Admin (authz-test)"
)

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
		Name:                  "ou-authz-admin-schema",
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
		Type:             schema.Name,
		OUID:             ts.ou1ID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q}`,
			ouAdminUsername, ouAdminPassword,
		)),
	})
	ts.Require().NoError(err, "create ou-admin user")
	ts.ouAdminUserID = userID

	// ---- 4. Look up the system resource server that was seeded by bootstrap ----
	// We use the system RS ID to attach the correct permission to the test role.
	systemRSID, err := testutils.GetResourceServerByName("System")
	ts.Require().NoError(err, "look up system resource server")

	// ---- 5. Create a role with permission system:ou:view ----
	role := testutils.Role{
		Name:               ouViewRoleName,
		OUID:               ts.ou1ID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: systemRSID,
				Permissions:      []string{"system:ou:view"},
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
	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		developClientID,
		developRedirectURI,
		"system system:ou:view",
		ouAdminUsername,
		ouAdminPassword,
		true,
	)
	ts.Require().NoError(err, "obtain ou-admin scoped token")
	ts.Require().NotEmpty(tokenResp.AccessToken, "ou-admin token must be non-empty")
	ts.Require().Equal("system:ou:view", tokenResp.Scope, "token must carry exactly the requested scope")

	ts.ouViewClient = testutils.GetHTTPClientWithToken(tokenResp.AccessToken)
}

// ---------------------------------------------------------------------------
// Suite teardown
// ---------------------------------------------------------------------------

func (ts *OUAuthzTestSuite) TearDownSuite() {
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
// Tests — READ operations (system:ou:view is sufficient)
// ---------------------------------------------------------------------------

// TestListOUs verifies that an OU-scoped admin's list result contains OU1 but
// not OU2 (sibling root) or OU12 (child, not directly assigned).
func (ts *OUAuthzTestSuite) TestListOUs() {
	resp := ts.do(http.MethodGet, "/organization-units", nil)
	defer closeBody(resp)

	ts.Require().Equal(http.StatusOK, resp.StatusCode, "list OUs should succeed")

	var listResp OrganizationUnitListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ids := make([]string, 0, len(listResp.OrganizationUnits))
	for _, ou := range listResp.OrganizationUnits {
		ids = append(ids, ou.ID)
	}

	ts.Containsf(ids, ts.ou1ID,
		"list must include OU1 (user's home OU), got IDs: %v", ids)
	ts.NotContainsf(ids, ts.ou2ID,
		"list must NOT include OU2 (sibling root), got IDs: %v", ids)
	ts.NotContainsf(ids, ts.ou12ID,
		"list must NOT include OU12 (child, not directly assigned), got IDs: %v", ids)
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

// TestGetChildOUByID verifies the OU-admin is denied access to OU12 by ID.
func (ts *OUAuthzTestSuite) TestGetChildOUByID() {
	resp := ts.do(http.MethodGet, "/organization-units/"+ts.ou12ID, nil)
	defer closeBody(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"OU-admin should be denied access to child OU (OU12) by ID")
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
// Tests — WRITE operations (require system:ou; view-only token must be denied)
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
		"view-only OU-admin must not create a child OU (system:ou required)")
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
		"view-only OU-admin must not update own OU (system:ou required)")
}

// TestDeleteOwnOU verifies the view-only OU-admin cannot delete OU1.
func (ts *OUAuthzTestSuite) TestDeleteOwnOU() {
	resp := ts.do(http.MethodDelete, "/organization-units/"+ts.ou1ID, nil)
	defer closeBody(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"view-only OU-admin must not delete own OU (system:ou required)")
}

// TestListOUsWithFilterMatch verifies that a filter applied over the restricted
// OU set returns only the OUs that both the caller is authorized to see AND
// match the filter expression.
func (ts *OUAuthzTestSuite) TestListOUsWithFilterMatch() {
	// "Authz Test OU1" is the name set in SetupSuite for OU1.
	resp := ts.doWithFilter("/organization-units", `name eq "Authz Test OU1"`)
	defer closeBody(resp)

	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ids := make([]string, 0, len(listResp.OrganizationUnits))
	for _, ou := range listResp.OrganizationUnits {
		ids = append(ids, ou.ID)
	}

	ts.Containsf(ids, ts.ou1ID,
		"filter matching OU1's name should return OU1; got IDs: %v", ids)
}

// TestListOUsWithFilterNoMatch verifies that a filter that matches no authorized
// OU returns an empty list, even though the caller is authorized to see OU1.
func (ts *OUAuthzTestSuite) TestListOUsWithFilterNoMatch() {
	resp := ts.doWithFilter("/organization-units", `name eq "__no_match__"`)
	defer closeBody(resp)

	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ts.Equal(0, listResp.TotalResults,
		"filter that matches no authorized OU should return empty results")
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
