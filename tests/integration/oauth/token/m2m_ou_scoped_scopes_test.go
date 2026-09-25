// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// M2MOUScopedScopesTestSuite proves the division of labour between the two organization units
// involved in an OU-scoped token request:
//
//   - What the application is *entitled* to is resolved from its own owning organization unit,
//     through the role assigned to it there. That answer does not change with the accessing OU.
//   - What the token may actually *carry* is then narrowed to the permissions the accessing
//     organization unit can see, which is decided by the resource-server sharing policies reaching it.
//
// The fixture shares the resource server with a child organization unit while withholding one of its
// two resources, so the same credential and the same requested scopes yield a different scope set
// depending on which organization unit the token is requested for.
type M2MOUScopedScopesTestSuite struct {
	suite.Suite
	rsID         string
	rsIdentifier string
	ordersResID  string
	roleID       string
	policyID     string
}

func TestM2MOUScopedScopesTestSuite(t *testing.T) {
	suite.Run(t, new(M2MOUScopedScopesTestSuite))
}

const (
	scopeBooksRead   = "books:read"
	scopeBooksCreate = "books:create"
	scopeOrdersRead  = "orders:read"

	// The application whose entitlement this fixture builds. Its subtree policy is what admits it to
	// the child organization unit in the first place.
	m2mSubtreeAppID = "decl-m2m-subtree"
)

func (suite *M2MOUScopedScopesTestSuite) SetupSuite() {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	suite.rsIdentifier = "https://m2m-scopes-" + suffix + ".example.com"

	// Owned by the same organization unit that owns the declarative M2M applications.
	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:       "M2M Scope Filter RS " + suffix,
		Identifier: suite.rsIdentifier,
		OUID:       m2mRootOUID,
	}, nil)
	suite.Require().NoError(err, "create resource server")
	suite.rsID = rsID

	booksID := suite.createResource(rsID, "Books", "books")
	suite.createAction(rsID, booksID, "Read", "read")
	suite.createAction(rsID, booksID, "Create", "create")

	// The resource deliberately withheld from the child organization unit.
	suite.ordersResID = suite.createResource(rsID, "Orders", "orders")
	suite.createAction(rsID, suite.ordersResID, "Read", "read")

	// The application's entitlement lives in its own owning organization unit: a role there,
	// carrying every permission, assigned directly to the application.
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "M2M Scope Filter Role " + suffix,
		OUID: m2mRootOUID,
		Permissions: []testutils.ResourcePermissions{{
			ResourceServerID: rsID,
			Permissions:      []string{scopeBooksRead, scopeBooksCreate, scopeOrdersRead},
		}},
		Assignments: []testutils.Assignment{{ID: m2mSubtreeAppID, Type: "app"}},
	})
	suite.Require().NoError(err, "create role")
	suite.roleID = roleID

	// Share the resource server with the child, withholding the orders branch. Naming a path
	// withholds it and everything beneath it, so orders:read goes with it and no per-node
	// enumeration is needed.
	suite.policyID = suite.shareResourceServerWithheldOrders(rsID)
}

// shareResourceServerWithheldOrders records one policy reaching the child organization unit, with
// the orders branch excluded from the resources it may see.
func (suite *M2MOUScopedScopesTestSuite) shareResourceServerWithheldOrders(rsID string) string {
	suite.T().Helper()

	body, err := json.Marshal(map[string]interface{}{
		"targetOuScope": map[string]interface{}{
			"ouIds": []map[string]interface{}{{"ouId": m2mChildAOUID}},
		},
		"overlayRules": map[string]interface{}{
			"resources": map[string]interface{}{
				"editable":       false,
				"excludedValues": []string{"orders"},
			},
		},
	})
	suite.Require().NoError(err)

	return suite.postForID(
		fmt.Sprintf("%s/resource-servers/%s/sharing-policies", testutils.TestServerURL, rsID), body)
}

func (suite *M2MOUScopedScopesTestSuite) TearDownSuite() {
	// The policy goes with the resource server, but removing it first keeps a failed server delete
	// from leaving a policy pointing at a resource that is gone.
	if suite.policyID != "" && suite.rsID != "" {
		suite.deletePolicy(suite.rsID, suite.policyID)
	}
	if suite.roleID != "" {
		if err := testutils.DeleteRole(suite.roleID); err != nil {
			suite.T().Logf("teardown: delete role: %v", err)
		}
	}
	if suite.rsID != "" {
		if err := testutils.DeleteResourceServerWithChildren(suite.rsID); err != nil {
			suite.T().Logf("teardown: delete resource server: %v", err)
		}
	}
}

func (suite *M2MOUScopedScopesTestSuite) deletePolicy(rsID, policyID string) {
	suite.T().Helper()
	req, err := http.NewRequest("DELETE",
		fmt.Sprintf("%s/resource-servers/%s/sharing-policies/%s", testutils.TestServerURL, rsID, policyID), nil)
	if err != nil {
		suite.T().Logf("teardown: build delete policy request: %v", err)
		return
	}
	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		suite.T().Logf("teardown: delete sharing policy: %v", err)
		return
	}
	defer resp.Body.Close()
}

func (suite *M2MOUScopedScopesTestSuite) createResource(rsID, name, handle string) string {
	suite.T().Helper()
	body, _ := json.Marshal(map[string]interface{}{"name": name, "handle": handle})
	return suite.postForID(fmt.Sprintf("%s/resource-servers/%s/resources", testutils.TestServerURL, rsID), body)
}

func (suite *M2MOUScopedScopesTestSuite) createAction(rsID, resourceID, name, handle string) string {
	suite.T().Helper()
	body, _ := json.Marshal(map[string]interface{}{"name": name, "handle": handle})
	return suite.postForID(
		fmt.Sprintf("%s/resource-servers/%s/resources/%s/actions", testutils.TestServerURL, rsID, resourceID), body)
}

func (suite *M2MOUScopedScopesTestSuite) postForID(url string, body []byte) string {
	suite.T().Helper()
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusCreated, resp.StatusCode, "body: %s", string(raw))

	var created struct {
		ID string `json:"id"`
	}
	suite.Require().NoError(json.Unmarshal(raw, &created))
	suite.Require().NotEmpty(created.ID)
	return created.ID
}

// requestScopedToken asks for a token bound to the fixture's resource server, optionally against an
// accessing organization unit, and returns the scopes the server actually granted.
func (suite *M2MOUScopedScopesTestSuite) requestScopedToken(ouID string) (int, []string) {
	suite.T().Helper()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", strings.Join([]string{scopeBooksRead, scopeBooksCreate, scopeOrdersRead}, " "))
	form.Set("resource", suite.rsIdentifier)

	endpoint := testutils.TestServerURL + "/oauth2/token"
	if ouID != "" {
		endpoint = testutils.TestServerURL + "/ou/" + ouID + "/oauth2/token"
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(m2mSubtreeClientID, m2mSubtreeSecret)

	// Raw client: this test authenticates as an OAuth client and owns its Authorization header.
	resp, err := testutils.GetRawHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	var parsed struct {
		Scope string `json:"scope"`
	}
	_ = json.Unmarshal(raw, &parsed)
	if resp.StatusCode != http.StatusOK {
		suite.T().Logf("token request for ou=%q returned %d: %s", ouID, resp.StatusCode, string(raw))
		return resp.StatusCode, nil
	}

	scopes := strings.Fields(parsed.Scope)
	sort.Strings(scopes)
	return resp.StatusCode, scopes
}

// TestOwningOUGetsEveryEntitledScope establishes the baseline: against its own organization unit the
// application carries everything its role grants, since the owner sees its own resource server whole.
func (suite *M2MOUScopedScopesTestSuite) TestOwningOUGetsEveryEntitledScope() {
	status, scopes := suite.requestScopedToken(m2mRootOUID)

	suite.Equal(http.StatusOK, status)
	suite.Equal([]string{scopeBooksCreate, scopeBooksRead, scopeOrdersRead}, scopes)
}

// TestAccessingOUOnlyGetsSharedScopes is the point of the feature: the same credential, the same
// role, and the same requested scopes yield a narrower token for the child, because the orders
// branch was withheld from it when the resource server was shared.
func (suite *M2MOUScopedScopesTestSuite) TestAccessingOUOnlyGetsSharedScopes() {
	status, scopes := suite.requestScopedToken(m2mChildAOUID)

	suite.Equal(http.StatusOK, status)
	suite.Equal([]string{scopeBooksCreate, scopeBooksRead}, scopes,
		"orders:read must be filtered out: the orders branch was withheld from the child")
	suite.NotContains(scopes, scopeOrdersRead)
}

// TestBareEndpointIsUnaffected pins that scope filtering only engages when an accessing organization
// unit is named; without one, issuance resolves against the application's own organization unit
// exactly as it did before the prefix existed.
func (suite *M2MOUScopedScopesTestSuite) TestBareEndpointIsUnaffected() {
	status, scopes := suite.requestScopedToken("")

	suite.Equal(http.StatusOK, status)
	suite.Equal([]string{scopeBooksCreate, scopeBooksRead, scopeOrdersRead}, scopes)
}
