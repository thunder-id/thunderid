// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

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

const (
	orgBoundaryPassword   = "OrgBoundary@123"
	orgBoundaryIdentifier = "https://org-boundary-test.example.com/system"
)

// OrgPermissionBoundaryTestSuite verifies the reach of the Data Plane's two permission roots. The
// system root reaches every organization unit. The org root, and every permission under it, stays
// within the organization unit the token names.
//
//	OU A (handle: org-boundary-a)  ← every administrator belongs here
//	OU B (handle: org-boundary-b)  ← a sibling tree
type OrgPermissionBoundaryTestSuite struct {
	suite.Suite

	signIn testutils.OUSignInApps

	ouAID     string
	ouBID     string
	typeAID   string
	typeBID   string
	userAID   string
	userBID   string
	rsID      string
	userIDs   []string
	roleIDs   []string
	systemCl  *http.Client
	orgCl     *http.Client
	userViewC *http.Client
}

func TestOrgPermissionBoundaryTestSuite(t *testing.T) {
	suite.Run(t, new(OrgPermissionBoundaryTestSuite))
}

func (ts *OrgPermissionBoundaryTestSuite) SetupSuite() {
	var err error
	ts.ouAID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "org-boundary-a", Name: "Org Boundary A", Description: "The administrators' unit",
	})
	ts.Require().NoError(err, "create OU A")
	ts.ouBID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "org-boundary-b", Name: "Org Boundary B", Description: "A sibling unit",
	})
	ts.Require().NoError(err, "create OU B")

	ts.typeAID = ts.createUserType("org-boundary-type-a", ts.ouAID)
	ts.typeBID = ts.createUserType("org-boundary-type-b", ts.ouBID)
	ts.userAID = ts.createUser("org-boundary-type-a", ts.ouAID, "org-boundary-target-a")
	ts.userBID = ts.createUser("org-boundary-type-b", ts.ouBID, "org-boundary-target-b")

	ts.rsID, err = testutils.CreateOrgScopedResourceServer(ts.ouAID, "Org Boundary RS", orgBoundaryIdentifier,
		"user")
	ts.Require().NoError(err, "create resource server")

	ts.systemCl = ts.adminClient("org-boundary-system-admin", "system")
	ts.orgCl = ts.adminClient("org-boundary-org-admin", "org")
	ts.userViewC = ts.adminClient("org-boundary-user-viewer", "org:user:view")
}

func (ts *OrgPermissionBoundaryTestSuite) TearDownSuite() {
	if err := ts.signIn.DeleteAll(); err != nil {
		ts.T().Logf("teardown: delete sign-in applications: %v", err)
	}
	for _, id := range ts.roleIDs {
		if err := testutils.DeleteRole(id); err != nil {
			ts.T().Logf("teardown: delete role %s: %v", id, err)
		}
	}
	if ts.rsID != "" {
		if err := testutils.DeleteResourceServerWithChildren(ts.rsID); err != nil {
			ts.T().Logf("teardown: delete resource server: %v", err)
		}
	}
	for _, id := range append(ts.userIDs, ts.userAID, ts.userBID) {
		if id != "" {
			if err := testutils.DeleteUser(id); err != nil {
				ts.T().Logf("teardown: delete user %s: %v", id, err)
			}
		}
	}
	for _, id := range []string{ts.typeAID, ts.typeBID} {
		if id != "" {
			if err := testutils.DeleteUserType(id); err != nil {
				ts.T().Logf("teardown: delete user type %s: %v", id, err)
			}
		}
	}
	for _, id := range []string{ts.ouAID, ts.ouBID} {
		if id != "" {
			if err := testutils.DeleteOrganizationUnit(id); err != nil {
				ts.T().Logf("teardown: delete OU %s: %v", id, err)
			}
		}
	}
}

func (ts *OrgPermissionBoundaryTestSuite) TestSystemReadsUserInOtherOU() {
	status, raw := ts.do(ts.systemCl, http.MethodGet, "/users/"+ts.userBID, nil)
	ts.Equal(http.StatusOK, status, string(raw))
}

func (ts *OrgPermissionBoundaryTestSuite) TestOrgUserConfinedToTokenOU() {
	status, raw := ts.do(ts.userViewC, http.MethodGet, "/users/"+ts.userAID, nil)
	ts.Equal(http.StatusOK, status, string(raw))

	status, raw = ts.do(ts.userViewC, http.MethodGet, "/users/"+ts.userBID, nil)
	ts.Equal(http.StatusForbidden, status, string(raw))
}

func (ts *OrgPermissionBoundaryTestSuite) TestOrgReadsButCannotWriteThemes() {
	status, raw := ts.do(ts.orgCl, http.MethodGet, "/design/themes", nil)
	ts.Equal(http.StatusOK, status, string(raw))

	status, raw = ts.do(ts.orgCl, http.MethodPost, "/design/themes", map[string]interface{}{"handle": "denied"})
	ts.Equal(http.StatusForbidden, status, string(raw))
}

func (ts *OrgPermissionBoundaryTestSuite) TestOrgCannotReadServerConfig() {
	status, raw := ts.do(ts.orgCl, http.MethodGet, "/server-config/cors", nil)
	ts.Equal(http.StatusForbidden, status, string(raw))
}

// The admin console shows a page when the capabilities endpoint reports the page's view action. An org
// admin sees the organization's pages but not the deployment-wide server configuration, and a system
// admin sees everything.
func (ts *OrgPermissionBoundaryTestSuite) TestCapabilitiesReportEachRootsReach() {
	cases := []struct {
		name         string
		client       *http.Client
		serverConfig bool
	}{
		{name: "org", client: ts.orgCl},
		{name: "system", client: ts.systemCl, serverConfig: true},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			status, raw := ts.do(tc.client, http.MethodGet, "/capabilities", nil)
			ts.Require().Equal(http.StatusOK, status, raw)
			var resp struct {
				Resources map[string]map[string]bool `json:"resources"`
			}
			ts.Require().NoError(json.Unmarshal([]byte(raw), &resp))

			for _, page := range []string{"user", "group", "role", "agent"} {
				ts.True(resp.Resources[page]["view"], "%s view", page)
			}
			ts.Equal(tc.serverConfig, resp.Resources["serverconfig"]["view"], "serverconfig view")
			ts.Equal(tc.serverConfig, resp.Resources["export"]["create"], "export create")
		})
	}
}

func (ts *OrgPermissionBoundaryTestSuite) createUserType(handle, ouID string) string {
	id, err := testutils.CreateUserType(testutils.UserType{
		Handle:      handle,
		DisplayName: handle,
		OUID:        ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "create user type %s", handle)
	return id
}

func (ts *OrgPermissionBoundaryTestSuite) createUser(userType, ouID, username string) string {
	id, err := testutils.CreateUser(testutils.User{
		Type: userType,
		OUID: ouID,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q, "password": %q}`,
			username, orgBoundaryPassword)),
	})
	ts.Require().NoError(err, "create user %s", username)
	return id
}

// adminClient makes a user of OU A hold permission there through a role on the suite's resource server,
// and returns a client carrying that user's token.
func (ts *OrgPermissionBoundaryTestSuite) adminClient(username, permission string) *http.Client {
	userID := ts.createUser("org-boundary-type-a", ts.ouAID, username)
	ts.userIDs = append(ts.userIDs, userID)

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: username + "-role",
		OUID: ts.ouAID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.rsID, Permissions: []string{permission}},
		},
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	})
	ts.Require().NoError(err, "create role for %s", username)
	ts.roleIDs = append(ts.roleIDs, roleID)

	token, err := ts.signIn.ObtainAccessToken(ts.ouAID, permission, username, orgBoundaryPassword,
		orgBoundaryIdentifier)
	ts.Require().NoError(err, "obtain token for %s", username)
	ts.Require().Equal(permission, token.Scope, "the token must carry exactly %s", permission)
	return testutils.GetHTTPClientWithToken(token.AccessToken)
}

func (ts *OrgPermissionBoundaryTestSuite) do(client *http.Client, method, path string, body interface{}) (
	int, string) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		ts.Require().NoError(err)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, string(raw)
}
