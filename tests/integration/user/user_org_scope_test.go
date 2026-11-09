// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// orgScopeSystemResourceServerID is the ID of the System resource server the server bootstraps.
const orgScopeSystemResourceServerID = "01900000-0000-7000-8000-000000000020"

// orgScopePageSize is the page size user lists are read with.
const orgScopePageSize = 100

// UserOrgScopeTestSuite checks how far a service application's token reaches into users, depending on the
// permission it carries and the organization unit it names.
//
//	A (handle: org-scope-a)        ← the application, its role, a user shared with A's children
//	├── A1 (handle: org-scope-a1)  ← a user A1 owns
//	└── A2 (handle: org-scope-a2)
//	B (handle: org-scope-b)        ← a user
//	C (handle: org-scope-c)        ← a user
//
// The role grants the application system, org and org:user:view. An application holds what it is assigned in
// the organization unit that owns it, so a token for A1 carries the same permissions as one for A.
type UserOrgScopeTestSuite struct {
	suite.Suite

	ouA, ouA1, ouA2, ouB, ouC   string
	userA, userA1, userB, userC string
	userTypeIDs                 []string
	appID, roleID               string
	clientID, clientSecret      string
	rootClient                  *http.Client
}

func TestUserOrgScopeTestSuite(t *testing.T) {
	suite.Run(t, new(UserOrgScopeTestSuite))
}

func (ts *UserOrgScopeTestSuite) SetupSuite() {
	ts.rootClient = testutils.GetHTTPClient()
	ts.ouA = ts.createOU("org-scope-a", nil)
	ts.ouB = ts.createOU("org-scope-b", nil)
	ts.ouC = ts.createOU("org-scope-c", nil)
	ts.ouA1 = ts.createOU("org-scope-a1", &ts.ouA)
	ts.ouA2 = ts.createOU("org-scope-a2", &ts.ouA)

	typeA := ts.createUserType("org-scope-person-a", ts.ouA)
	ts.userA = ts.createUser(typeA, ts.ouA, "org-scope-user-a")
	ts.userA1 = ts.createUser(typeA, ts.ouA1, "org-scope-user-a1")
	ts.userB = ts.createUser(ts.createUserType("org-scope-person-b", ts.ouB), ts.ouB, "org-scope-user-b")
	ts.userC = ts.createUser(ts.createUserType("org-scope-person-c", ts.ouC), ts.ouC, "org-scope-user-c")
	ts.shareWithChildUnits("/users/" + ts.userA)

	ts.clientID = "org-scope-client"
	ts.clientSecret = "org-scope-secret"
	appID, err := testutils.CreateApplication(testutils.Application{
		Name: "Org Scope Service",
		OUID: ts.ouA,
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                ts.clientID,
				"clientSecret":            ts.clientSecret,
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"pkceRequired":            false,
				"publicClient":            false,
				// The token names the unit it was issued for, which is what bounds an org permission.
				"token": map[string]interface{}{"accessToken": map[string]interface{}{
					"clientConfig": map[string]interface{}{"attributes": []string{"ouId"}},
				}},
			},
		}},
	})
	ts.Require().NoError(err, "create the service application")
	ts.appID = appID
	ts.Require().NoError(testutils.ShareApplicationWithChildUnits(ts.appID))

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "org-scope-service-role",
		OUID: ts.ouA,
		Permissions: []testutils.ResourcePermissions{{
			ResourceServerID: orgScopeSystemResourceServerID,
			Permissions:      []string{"system", "org", "org:user:view"},
		}},
		Assignments: []testutils.Assignment{{ID: ts.appID, Type: "app"}},
	})
	ts.Require().NoError(err, "create the service role")
	ts.roleID = roleID
	ts.shareWithChildUnits("/roles/" + ts.roleID)
}

func (ts *UserOrgScopeTestSuite) TearDownSuite() {
	if ts.roleID != "" {
		ts.NoError(testutils.DeleteRole(ts.roleID))
	}
	if ts.appID != "" {
		ts.NoError(testutils.DeleteApplication(ts.appID))
	}
	for _, id := range []string{ts.userA, ts.userA1, ts.userB, ts.userC} {
		if id != "" {
			ts.NoError(testutils.DeleteUser(id))
		}
	}
	for _, id := range ts.userTypeIDs {
		ts.NoError(testutils.DeleteUserType(id))
	}
	for _, id := range []string{ts.ouA1, ts.ouA2, ts.ouA, ts.ouB, ts.ouC} {
		if id != "" {
			ts.NoError(testutils.DeleteOrganizationUnit(id))
		}
	}
}

// system reaches every unit. org and org:user:view are bounded to the unit the token names: a list with no
// query parameters holds what that unit owns, then what is shared into it, and nothing of its children,
// its parent or other trees.
func (ts *UserOrgScopeTestSuite) TestTheUserListFollowsTheTokensScopeAndUnit() {
	cases := []struct {
		name    string
		ouID    string
		scope   string
		tokenOU string
		owned   []string
		shared  []string
		hidden  []string
	}{
		{name: "system at the token endpoint", scope: "system", tokenOU: ts.ouA,
			owned: []string{ts.userA, ts.userA1, ts.userB, ts.userC}},
		{name: "org at the token endpoint", scope: "org", tokenOU: ts.ouA,
			owned: []string{ts.userA}, hidden: []string{ts.userA1, ts.userB, ts.userC}},
		{name: "org at A1's token endpoint", ouID: ts.ouA1, scope: "org", tokenOU: ts.ouA1,
			owned: []string{ts.userA1}, shared: []string{ts.userA}, hidden: []string{ts.userB, ts.userC}},
		{name: "org:user:view at the token endpoint", scope: "org:user:view", tokenOU: ts.ouA,
			owned: []string{ts.userA}, hidden: []string{ts.userA1, ts.userB, ts.userC}},
		{name: "org:user:view at A1's token endpoint", ouID: ts.ouA1, scope: "org:user:view", tokenOU: ts.ouA1,
			owned: []string{ts.userA1}, shared: []string{ts.userA}, hidden: []string{ts.userB, ts.userC}},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			token := ts.token(tc.ouID, tc.scope)
			claims, err := testutils.DecodeJWT(token)
			ts.Require().NoError(err)
			ts.Require().Equal(tc.tokenOU, claims.OUID, "the token names the wrong organization unit")

			listed := ts.listUsers(token)
			for _, id := range tc.owned {
				ts.Require().Contains(listed, id, "user %s is not listed", id)
				ts.True(listed[id], "user %s is not listed as owned", id)
			}
			for _, id := range tc.shared {
				ts.Require().Contains(listed, id, "user %s is not listed", id)
				ts.False(listed[id], "user %s is not listed as shared", id)
			}
			for _, id := range tc.hidden {
				ts.NotContains(listed, id, "user %s is listed", id)
			}
		})
	}
}

// A read without a unit reads in the unit the token names: a user it owns and one shared into it are read,
// marked, and anything else is refused.
func (ts *UserOrgScopeTestSuite) TestAUserIsReadInTheTokensUnit() {
	token := ts.token(ts.ouA1, "org:user:view")
	cases := []struct {
		name   string
		userID string
		status int
		owned  bool
	}{
		{name: "owned by A1", userID: ts.userA1, status: http.StatusOK, owned: true},
		{name: "shared into A1", userID: ts.userA, status: http.StatusOK},
		{name: "in another tree", userID: ts.userB, status: http.StatusForbidden},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			status, raw := ts.do(token, http.MethodGet, "/users/"+tc.userID, nil)
			ts.Require().Equal(tc.status, status, string(raw))
			if tc.status != http.StatusOK {
				return
			}
			var user testutils.User
			ts.Require().NoError(json.Unmarshal(raw, &user))
			ts.Require().NotNil(user.IsOwnedByOU)
			ts.Equal(tc.owned, *user.IsOwnedByOU)
			ts.Equal(ts.ouA1, user.OUID, "the user is read under the unit the token names")
		})
	}
}

// A user shared into A1 stays A's to change: A1 may neither update nor delete it, even holding org.
func (ts *UserOrgScopeTestSuite) TestASharedUserIsReadOnlyInTheTokensUnit() {
	token := ts.token(ts.ouA1, "org")

	status, raw := ts.do(token, http.MethodPut, "/users/"+ts.userA, map[string]interface{}{
		"ouId": ts.ouA, "type": "org-scope-person-a", "attributes": map[string]string{"username": "renamed"},
	})
	ts.Equal(http.StatusForbidden, status, string(raw))
	ts.Contains(string(raw), "USR-1045")

	status, raw = ts.do(token, http.MethodDelete, "/users/"+ts.userA, nil)
	ts.Equal(http.StatusForbidden, status, string(raw))
	ts.Contains(string(raw), "USR-1045")
}

// A token that names no organization unit reaches users only with system. org and org:user:view act in the unit a
// token names, so without one the Data Plane refuses the token before reading anything.
func (ts *UserOrgScopeTestSuite) TestATokenNamingNoUnitNeedsSystem() {
	const clientID, clientSecret = "org-scope-no-unit-client", "org-scope-no-unit-secret"
	appID, err := testutils.CreateApplication(testutils.Application{
		Name: "Org Scope Service Without A Unit",
		OUID: ts.ouA,
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                clientID,
				"clientSecret":            clientSecret,
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"pkceRequired":            false,
				"publicClient":            false,
			},
		}},
	})
	ts.Require().NoError(err)
	defer func() { ts.NoError(testutils.DeleteApplication(appID)) }()
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "org-scope-no-unit-role",
		OUID: ts.ouA,
		Permissions: []testutils.ResourcePermissions{{
			ResourceServerID: orgScopeSystemResourceServerID,
			Permissions:      []string{"system", "org", "org:user:view"},
		}},
		Assignments: []testutils.Assignment{{ID: appID, Type: "app"}},
	})
	ts.Require().NoError(err)
	defer func() { ts.NoError(testutils.DeleteRole(roleID)) }()

	for _, tc := range []struct {
		scope  string
		status int
	}{
		{scope: "system", status: http.StatusOK},
		{scope: "org", status: http.StatusForbidden},
		{scope: "org:user:view", status: http.StatusForbidden},
	} {
		ts.Run(tc.scope, func() {
			token := ts.tokenFor(clientID, clientSecret, "", tc.scope)
			claims, err := testutils.DecodeJWT(token)
			ts.Require().NoError(err)
			ts.Require().Empty(claims.OUID, "the token must name no organization unit")

			status, raw := ts.do(token, http.MethodGet, "/users?limit=10", nil)
			ts.Equal(tc.status, status, string(raw))
			status, raw = ts.do(token, http.MethodGet, "/users/"+ts.userA, nil)
			ts.Equal(tc.status, status, string(raw))
		})
	}
}

func (ts *UserOrgScopeTestSuite) createOU(handle string, parent *string) string {
	ts.T().Helper()
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{Handle: handle, Name: handle, Parent: parent})
	ts.Require().NoError(err, "create organization unit %s", handle)
	return id
}

// createUserType creates a user type owned by ouID and returns its handle.
func (ts *UserOrgScopeTestSuite) createUserType(handle, ouID string) string {
	ts.T().Helper()
	id, err := testutils.CreateUserType(testutils.UserType{
		Handle:      handle,
		DisplayName: handle,
		OUID:        ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "required": true, "unique": true},
		},
	})
	ts.Require().NoError(err, "create user type %s", handle)
	ts.userTypeIDs = append(ts.userTypeIDs, id)
	return handle
}

func (ts *UserOrgScopeTestSuite) createUser(userType, ouID, username string) string {
	ts.T().Helper()
	id, err := testutils.CreateUser(testutils.User{
		Type: userType, OUID: ouID, Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q}`, username)),
	})
	ts.Require().NoError(err, "create user %s", username)
	return id
}

// shareWithChildUnits shares the resource at path with every unit beneath its owner.
func (ts *UserOrgScopeTestSuite) shareWithChildUnits(path string) {
	ts.T().Helper()
	status, raw := sendRequest(ts.T(), ts.rootClient, http.MethodPost, path+"/sharing-policies",
		map[string]interface{}{"childUnits": map[string]interface{}{"mode": "all"}})
	ts.Require().Equal(http.StatusCreated, status, string(raw))
}

// token requests a client credentials token for the System resource server carrying exactly scope, at the
// token endpoint of ouID when it is set.
func (ts *UserOrgScopeTestSuite) token(ouID, scope string) string {
	ts.T().Helper()
	return ts.tokenFor(ts.clientID, ts.clientSecret, ouID, scope)
}

// tokenFor requests a client credentials token as the client, as token does.
func (ts *UserOrgScopeTestSuite) tokenFor(clientID, clientSecret, ouID, scope string) string {
	ts.T().Helper()
	path := "/oauth2/token"
	if ouID != "" {
		path = "/ou/" + ouID + path
	}
	form := url.Values{
		"grant_type": {"client_credentials"},
		"scope":      {scope},
		"resource":   {testutils.SystemResourceIdentifier},
	}
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+path, strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "token at %s: %s", path, raw)
	var token testutils.TokenResponse
	ts.Require().NoError(json.Unmarshal(raw, &token), string(raw))
	ts.Require().Equal(scope, token.Scope, "the token must carry exactly %s", scope)
	return token.AccessToken
}

// listUsers reads every page of the user list with the token and returns, for each user listed, whether it
// is listed as owned.
func (ts *UserOrgScopeTestSuite) listUsers(token string) map[string]bool {
	ts.T().Helper()
	listed := map[string]bool{}
	for offset := 0; ; offset += orgScopePageSize {
		status, raw := ts.do(token, http.MethodGet,
			fmt.Sprintf("/users?limit=%d&offset=%d", orgScopePageSize, offset), nil)
		ts.Require().Equal(http.StatusOK, status, string(raw))
		var page testutils.UserListResponse
		ts.Require().NoError(json.Unmarshal(raw, &page), string(raw))
		for _, user := range page.Users {
			ts.Require().NotNil(user.IsOwnedByOU, "user %s carries no isOwnedByOU", user.ID)
			listed[user.ID] = *user.IsOwnedByOU
		}
		if len(page.Users) < orgScopePageSize || offset+orgScopePageSize >= page.TotalResults {
			return listed
		}
	}
}

// do sends a JSON request with the token and returns the status and body.
func (ts *UserOrgScopeTestSuite) do(token, method, path string, body interface{}) (int, []byte) {
	ts.T().Helper()
	return sendRequest(ts.T(), testutils.GetHTTPClientWithToken(token), method, path, body)
}
