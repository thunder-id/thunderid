// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// resources/declarative_resources/resource_servers/resource-declarative-shared.yaml: owned by
	// decl-m2m-root and shared with child A (orders and reports:view), child B (everything except
	// audit and orders:delete) and, through child A's reshare, grandchild A (orders:read).
	m2mSharedRSID         = "decl-shared-rs"
	m2mSharedRSIdentifier = "https://localhost:8090/decl-shared-rs"
	m2mGrandchildBOUID    = "decl-m2m-grandchild-b"

	m2mSubtreeAppID = "decl-m2m-subtree"
	m2mAllOUsAppID  = "decl-m2m-all-ous"
)

// m2mRequestedScopes is everything the applications hold on the shared resource server, through a
// role in the owning organization unit.
var m2mRequestedScopes = []string{"orders:read", "orders:delete", "reports:view", "reports:export", "audit:read"}

// M2MSharedResourceServerTestSuite covers what a token issued for an organization unit may carry
// from a shared resource server: the permissions the application holds, bounded by the ones that
// organization unit may use.
type M2MSharedResourceServerTestSuite struct {
	suite.Suite
	roleID string
}

func TestM2MSharedResourceServerTestSuite(t *testing.T) {
	suite.Run(t, new(M2MSharedResourceServerTestSuite))
}

// SetupSuite grants the shared M2M applications every permission they will ask for, through a role
// in the organization unit that owns both them and the resource server.
func (suite *M2MSharedResourceServerTestSuite) SetupSuite() {
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "m2m-shared-rs-role",
		OUID: m2mRootOUID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: m2mSharedRSID, Permissions: m2mRequestedScopes},
		},
		Assignments: []testutils.Assignment{
			{ID: m2mSubtreeAppID, Type: "app"},
			{ID: m2mAllOUsAppID, Type: "app"},
		},
	})
	suite.Require().NoError(err, "create the role granting the shared resource server's permissions")
	suite.roleID = roleID
}

func (suite *M2MSharedResourceServerTestSuite) TearDownSuite() {
	if suite.roleID != "" {
		_ = testutils.DeleteRole(suite.roleID)
	}
}

// requestScopes issues a client_credentials request for every scope against the shared resource
// server and returns the scopes granted. When ouID is non-empty the request names it as the
// accessing organization unit.
func (suite *M2MSharedResourceServerTestSuite) requestScopes(ouID, clientID, clientSecret string) []string {
	suite.T().Helper()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", strings.Join(m2mRequestedScopes, " "))
	form.Set("resource", m2mSharedRSIdentifier)

	endpoint := testutils.TestServerURL + "/oauth2/token"
	if ouID != "" {
		endpoint = testutils.TestServerURL + "/ou/" + ouID + "/oauth2/token"
	}
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	suite.Require().Equal(http.StatusOK, resp.StatusCode, "body: %s", string(body))

	parsed := map[string]interface{}{}
	suite.Require().NoError(json.Unmarshal(body, &parsed))
	suite.NotEmpty(parsed["access_token"])
	scope, _ := parsed["scope"].(string)
	return strings.Fields(scope)
}

// Without an accessing organization unit the token answers for the application's own, which owns
// the resource server, so every permission the application holds is granted.
func (suite *M2MSharedResourceServerTestSuite) TestBareEndpointGrantsEverythingTheAppHolds() {
	scopes := suite.requestScopes("", m2mSubtreeClientID, m2mSubtreeSecret)

	suite.ElementsMatch(m2mRequestedScopes, scopes)
}

// The owning organization unit may use every permission, so naming it changes nothing.
func (suite *M2MSharedResourceServerTestSuite) TestOwnerOUGrantsEverythingTheAppHolds() {
	scopes := suite.requestScopes(m2mRootOUID, m2mSubtreeClientID, m2mSubtreeSecret)

	suite.ElementsMatch(m2mRequestedScopes, scopes)
}

// A token for child A carries only the permissions child A was given: the orders resource and
// reports:view.
func (suite *M2MSharedResourceServerTestSuite) TestValueBoundsTheScopes() {
	scopes := suite.requestScopes(m2mChildAOUID, m2mSubtreeClientID, m2mSubtreeSecret)

	suite.ElementsMatch([]string{"orders:read", "orders:delete", "reports:view"}, scopes)
}

// A token for child B carries everything except what was withheld from it.
func (suite *M2MSharedResourceServerTestSuite) TestExcludedValuesBoundTheScopes() {
	scopes := suite.requestScopes(m2mChildBOUID, m2mSubtreeClientID, m2mSubtreeSecret)

	suite.ElementsMatch([]string{"orders:read", "reports:view", "reports:export"}, scopes)
}

// A token for grandchild A carries only what child A passed on.
func (suite *M2MSharedResourceServerTestSuite) TestReshareBoundsTheScopes() {
	scopes := suite.requestScopes(m2mGrandchildAOUID, m2mSubtreeClientID, m2mSubtreeSecret)

	suite.ElementsMatch([]string{"orders:read"}, scopes)
}

// A token for grandchild B, reached by child B's reshare that names no rule, carries child B's bound
// rather than the whole server.
func (suite *M2MSharedResourceServerTestSuite) TestReshareWithoutRuleKeepsTheResharersBound() {
	scopes := suite.requestScopes(m2mGrandchildBOUID, m2mSubtreeClientID, m2mSubtreeSecret)

	suite.ElementsMatch([]string{"orders:read", "reports:view", "reports:export"}, scopes)
}

// The application may act for an organization unit the resource server was never shared with, but
// a token for it carries none of the server's permissions.
func (suite *M2MSharedResourceServerTestSuite) TestUnsharedOUGetsNoScopes() {
	scopes := suite.requestScopes(unrelatedDeclOUHandle, m2mAllOUsClientID, m2mAllOUsSecret)

	suite.Empty(scopes)
}
