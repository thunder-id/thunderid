// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

// OUAuthzDPTestSuite validates the OU authorization model end-to-end.
//
// The product ships only the root "system" scope by default. This suite creates a custom
// resource server that declares the following hierarchical permission structure, simulating an
// operator configuring fine-grained scopes:
//
//	Authz Test RS (ou)
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
type OUAuthzDPTestSuite struct {
	suite.Suite

	// Imported fixtures
	ou1ID  string
	ou2ID  string
	ou12ID string

	// HTTP client that carries the OU-admin's view-only access token
	ouViewClient *http.Client
}

const (
	authzTestServerURLDP = testutils.TestServerURL

	authzOU1HandleDP  = "authz-ou1"
	authzOU2HandleDP  = "authz-ou2"
	authzOU12HandleDP = "authz-ou12"

	ouAdminUsernameDP = "ou-authz-admin"
	ouAdminPasswordDP = "OUAdmin@123"

	developClientIDDP    = "CONSOLE"
	developRedirectURIDP = "https://localhost:8095/console"

	// Name of the role created in SetupSuite. Using a unique name avoids
	// collisions when tests run multiple times without a clean DB.
	ouViewRoleNameDP = "OU View Admin (authz-test)"

	scopedRSIdentifierDP = "https://authz-test.example.com/ou"

	// Ids of the imported fixtures.
	authzOU1IDDP        = "019a0000-0000-7000-8000-00000000d001"
	authzOU2IDDP        = "019a0000-0000-7000-8000-00000000d002"
	authzOU12IDDP       = "019a0000-0000-7000-8000-00000000d003"
	ouAdminUserTypeIDDP = "019a0000-0000-7000-8000-00000000d004"
	ouAdminUserIDDP     = "019a0000-0000-7000-8000-00000000d005"
	scopedRSIDDP        = "019a0000-0000-7000-8000-00000000d006"
	ouViewRoleIDDP      = "019a0000-0000-7000-8000-00000000d007"
)

func TestOUAuthzDPTestSuite(t *testing.T) {
	t.Skip(testutils.DataPlaneSuiteSkipReason)
	suite.Run(t, new(OUAuthzDPTestSuite))
}

// ---------------------------------------------------------------------------
// Suite setup
// ---------------------------------------------------------------------------

func (ts *OUAuthzDPTestSuite) SetupSuite() {
	// A data plane accepts root OUs and the fixtures in them only through import, so the suite
	// imports them with the management API key as a deployment pipeline does.
	//
	// The resource server declares the fine-grained system scopes. The product ships only the root
	// "system" scope; this reproduces the "system:ou:view" permission so the suite can verify
	// resource-level enforcement when configured.
	fixtures := fmt.Sprintf(`resource_type: organization_unit
id: %[1]s
handle: %[2]s
name: Authz Test OU1
description: Primary OU for authz integration test
---
resource_type: organization_unit
id: %[3]s
handle: %[4]s
name: Authz Test OU2
description: Sibling OU for authz integration test
---
resource_type: organization_unit
id: %[5]s
handle: %[6]s
name: Authz Test OU12 (child of OU1)
description: Child OU under OU1 for authz integration test
parent: %[1]s
---
resource_type: user_type
id: %[7]s
handle: ou-authz-admin-schema
displayName: Ou Authz Admin Schema
ouId: %[1]s
schema:
  username:
    type: string
    unique: true
  password:
    type: string
    credential: true
---
resource_type: user
id: %[8]s
type: ou-authz-admin-schema
ouId: %[1]s
attributes:
  username: %[9]s
credentials:
  password: %[10]q
---
resource_type: resource_server
id: %[11]s
name: Authz Test RS (ou)
identifier: %[12]s
ouId: %[1]s
delimiter: ":"
resources:
  - name: system
    handle: system
  - name: ou
    handle: ou
    parent: system
    actions:
      - name: view
        handle: view
---
resource_type: role
id: %[13]s
name: %[14]s
ouId: %[1]s
permissions:
  - resourceServerId: %[11]s
    permissions:
      - system:ou:view
assignments:
  - id: %[8]s
    type: user
`,
		authzOU1IDDP, authzOU1HandleDP, authzOU2IDDP, authzOU2HandleDP, authzOU12IDDP, authzOU12HandleDP,
		ouAdminUserTypeIDDP, ouAdminUserIDDP, ouAdminUsernameDP, ouAdminPasswordDP,
		scopedRSIDDP, scopedRSIdentifierDP, ouViewRoleIDDP, ouViewRoleNameDP)
	ts.Require().NoError(testutils.ImportWithManagementAPIKey(fixtures), "import the fixtures")
	ts.ou1ID = authzOU1IDDP
	ts.ou2ID = authzOU2IDDP
	ts.ou12ID = authzOU12IDDP

	// Obtain a scoped access token for the OU-admin user.
	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		developClientIDDP,
		developRedirectURIDP,
		"system system:ou:view",
		ouAdminUsernameDP,
		ouAdminPasswordDP,
		true,
		"",
		scopedRSIdentifierDP,
	)
	ts.Require().NoError(err, "obtain ou-admin scoped token")
	ts.Require().NotEmpty(tokenResp.AccessToken, "ou-admin token must be non-empty")
	ts.Require().Equal("system:ou:view", tokenResp.Scope, "token must carry exactly the requested scope")

	ts.ouViewClient = testutils.GetHTTPClientWithToken(tokenResp.AccessToken)
}

// ---------------------------------------------------------------------------
// Suite teardown
// ---------------------------------------------------------------------------

func (ts *OUAuthzDPTestSuite) TearDownSuite() {
	// Import cannot remove a resource server that still has resources, so the resource server and
	// its resources are removed through the management API first.
	if err := testutils.DeleteResourceServerWithChildren(scopedRSIDDP); err != nil {
		ts.T().Errorf("teardown: delete scoped resource server: %v", err)
	}

	// The import removes the rest in reverse dependency order; the child OU is listed before its
	// parent.
	err := testutils.ImportWithManagementAPIKey("",
		testutils.ImportDeletion{ResourceType: "role", ID: ouViewRoleIDDP},
		testutils.ImportDeletion{ResourceType: "user", ID: ouAdminUserIDDP},
		testutils.ImportDeletion{ResourceType: "user_type", ID: ouAdminUserTypeIDDP},
		testutils.ImportDeletion{ResourceType: "organization_unit", ID: authzOU12IDDP},
		testutils.ImportDeletion{ResourceType: "organization_unit", ID: authzOU2IDDP},
		testutils.ImportDeletion{ResourceType: "organization_unit", ID: authzOU1IDDP},
	)
	if err != nil {
		ts.T().Errorf("teardown: remove the fixtures: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helper — issue an HTTP request via the OU-view-only client
// ---------------------------------------------------------------------------

func (ts *OUAuthzDPTestSuite) do(method, path string, body []byte) *http.Response {
	ts.T().Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, authzTestServerURLDP+path, bodyReader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := ts.ouViewClient.Do(req)
	ts.Require().NoError(err)
	return resp
}

func closeBodyDP(resp *http.Response) { _ = resp.Body.Close() }

// ---------------------------------------------------------------------------
// Tests — READ operations (system:ou:view is sufficient)
// ---------------------------------------------------------------------------

// TestListOUs verifies that an OU-scoped admin's list result contains OU1 but
// not OU2 (sibling root) or OU12 (child, not directly assigned).
func (ts *OUAuthzDPTestSuite) TestListOUs() {
	resp := ts.do(http.MethodGet, "/organization-units", nil)
	defer closeBodyDP(resp)

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
func (ts *OUAuthzDPTestSuite) TestGetOwnOUByID() {
	resp := ts.do(http.MethodGet, "/organization-units/"+ts.ou1ID, nil)
	defer closeBodyDP(resp)

	ts.Equal(http.StatusOK, resp.StatusCode, "OU-admin should be able to read own OU by ID")
}

// TestGetSiblingOUByID verifies the OU-admin is denied access to OU2 by ID.
func (ts *OUAuthzDPTestSuite) TestGetSiblingOUByID() {
	resp := ts.do(http.MethodGet, "/organization-units/"+ts.ou2ID, nil)
	defer closeBodyDP(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"OU-admin should be denied access to sibling OU (OU2) by ID")
}

// TestGetChildOUByID verifies the OU-admin is denied access to OU12 by ID.
func (ts *OUAuthzDPTestSuite) TestGetChildOUByID() {
	resp := ts.do(http.MethodGet, "/organization-units/"+ts.ou12ID, nil)
	defer closeBodyDP(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"OU-admin should be denied access to child OU (OU12) by ID")
}

// TestGetOwnOUByPath verifies the OU-admin can read their own OU by handle path.
func (ts *OUAuthzDPTestSuite) TestGetOwnOUByPath() {
	resp := ts.do(http.MethodGet, "/organization-units/tree/"+authzOU1HandleDP, nil)
	defer closeBodyDP(resp)

	ts.Equal(http.StatusOK, resp.StatusCode,
		"OU-admin should be able to read own OU by handle path")
}

// TestGetSiblingOUByPath verifies the OU-admin is denied access to OU2 by handle path.
func (ts *OUAuthzDPTestSuite) TestGetSiblingOUByPath() {
	resp := ts.do(http.MethodGet, "/organization-units/tree/"+authzOU2HandleDP, nil)
	defer closeBodyDP(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"OU-admin should be denied access to sibling OU (OU2) by path")
}

// ---------------------------------------------------------------------------
// Tests — WRITE operations (require system:ou; view-only token must be denied)
// ---------------------------------------------------------------------------

func (ts *OUAuthzDPTestSuite) newOUPayload(handle, name string, parent *string) []byte {
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
func (ts *OUAuthzDPTestSuite) TestCreateOUAtRoot() {
	payload := ts.newOUPayload("authz-new-root-ou", "Authz New Root OU", nil)
	resp := ts.do(http.MethodPost, "/organization-units", payload)
	defer closeBodyDP(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"view-only OU-admin must not create a root-level OU")
}

// TestCreateOUUnderOwnOU verifies the view-only OU-admin cannot create a child of OU1.
func (ts *OUAuthzDPTestSuite) TestCreateOUUnderOwnOU() {
	payload := ts.newOUPayload("authz-new-child-ou", "Authz New Child OU", &ts.ou1ID)
	resp := ts.do(http.MethodPost, "/organization-units", payload)
	defer closeBodyDP(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"view-only OU-admin must not create a child OU (system:ou required)")
}

// TestUpdateOwnOU verifies the view-only OU-admin cannot update OU1.
func (ts *OUAuthzDPTestSuite) TestUpdateOwnOU() {
	updatePayload, err := json.Marshal(UpdateOURequest{
		Handle: authzOU1HandleDP,
		Name:   "Authz Test OU1 (modified)",
	})
	ts.Require().NoError(err)

	resp := ts.do(http.MethodPut, "/organization-units/"+ts.ou1ID, updatePayload)
	defer closeBodyDP(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"view-only OU-admin must not update own OU (system:ou required)")
}

// TestDeleteOwnOU verifies the view-only OU-admin cannot delete OU1.
func (ts *OUAuthzDPTestSuite) TestDeleteOwnOU() {
	resp := ts.do(http.MethodDelete, "/organization-units/"+ts.ou1ID, nil)
	defer closeBodyDP(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"view-only OU-admin must not delete own OU (system:ou required)")
}

// TestListOUsWithFilterMatch verifies that a filter applied over the restricted
// OU set returns only the OUs that both the caller is authorized to see AND
// match the filter expression.
func (ts *OUAuthzDPTestSuite) TestListOUsWithFilterMatch() {
	// "Authz Test OU1" is the name set in SetupSuite for OU1.
	resp := ts.doWithFilter("/organization-units", `name eq "Authz Test OU1"`)
	defer closeBodyDP(resp)

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
func (ts *OUAuthzDPTestSuite) TestListOUsWithFilterNoMatch() {
	resp := ts.doWithFilter("/organization-units", `name eq "__no_match__"`)
	defer closeBodyDP(resp)

	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ts.Equal(0, listResp.TotalResults,
		"filter that matches no authorized OU should return empty results")
	ts.Empty(listResp.OrganizationUnits)
}

// doWithFilter issues a GET request using the scoped OU-admin client with the
// given filter expression as the "filter" query parameter.
func (ts *OUAuthzDPTestSuite) doWithFilter(path, filterExpr string) *http.Response {
	ts.T().Helper()

	u, err := url.Parse(authzTestServerURLDP + path)
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

// TestListOUsOffsetBeyondAuthorizedSet verifies that paging past the end of the caller's authorized
// organization units returns an empty page while still reporting the true total.
func (ts *OUAuthzDPTestSuite) TestListOUsOffsetBeyondAuthorizedSet() {
	resp := ts.do(http.MethodGet, "/organization-units?limit=10&offset=1000", nil)
	defer closeBodyDP(resp)

	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var listResp OrganizationUnitListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ts.Empty(listResp.OrganizationUnits,
		"an offset past the authorized set must return no organization units")
	ts.Equal(0, listResp.Count, "count must be zero for an empty page")
	ts.GreaterOrEqual(listResp.TotalResults, 1,
		"total results must still report the authorized set size")
}

// ---------------------------------------------------------------------------
// Tests — the root "system" permission on a data plane
// ---------------------------------------------------------------------------

// TestSystemScopeConfinedToOwnOU verifies the administrator's "system" permission reaches only the
// OU its token names.
func (ts *OUAuthzDPTestSuite) TestSystemScopeConfinedToOwnOU() {
	req, err := http.NewRequest(http.MethodGet, authzTestServerURLDP+"/organization-units/"+ts.ou1ID, nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer closeBodyDP(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"the system permission must not reach an OU other than the token's")
}

// TestSystemScopeCannotCreateRootOU verifies root OUs are not created through the API on a data
// plane, even with the "system" permission.
func (ts *OUAuthzDPTestSuite) TestSystemScopeCannotCreateRootOU() {
	payload := ts.newOUPayload("authz-admin-root-ou", "Authz Admin Root OU", nil)
	req, err := http.NewRequest(http.MethodPost, authzTestServerURLDP+"/organization-units",
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer closeBodyDP(resp)

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"root OUs must not be created through the API on a data plane")
}
