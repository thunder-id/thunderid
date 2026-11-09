// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

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
	appAuthzRSIdentifier = "https://authz-test.example.com/application"
	appAuthzOUHandle     = "authz-app-ou"
	appAuthzTypeName     = "authz-app-type"
	appAuthzMgrUsername  = "authz-app-manager"
	appAuthzMgrPassword  = "AppManager@123"
)

// ApplicationAuthzTestSuite verifies an application manager holding org cannot take control of an
// application that holds a permission the manager lacks.
//
// Fixture topology, all within one OU:
//
//	app manager       — holds org, which the application API requires on the data plane
//	privileged app    — assigned a role conferring system, which org does not cover
type ApplicationAuthzTestSuite struct {
	suite.Suite

	signIn testutils.OUSignInApps

	ouID            string
	typeID          string
	managerID       string
	scopedRSID      string
	managerRoleID   string
	privilegedRole  string
	privilegedAppID string

	managerClient *http.Client
}

func TestApplicationAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationAuthzTestSuite))
}

func (ts *ApplicationAuthzTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: appAuthzOUHandle,
		Name:   "Application Authz Test OU",
	})
	ts.Require().NoError(err, "create OU")
	ts.ouID = ouID

	typeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      appAuthzTypeName,
		DisplayName: "Authz App Type",
		OUID:        ts.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "create user type")
	ts.typeID = typeID

	managerID, err := testutils.CreateUser(testutils.User{
		Type: appAuthzTypeName,
		OUID: ts.ouID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q}`, appAuthzMgrUsername, appAuthzMgrPassword)),
	})
	ts.Require().NoError(err, "create app manager")
	ts.managerID = managerID

	rsID, err := testutils.CreateOrgScopedResourceServer(
		ts.ouID, "Authz Test RS (application)", appAuthzRSIdentifier)
	ts.Require().NoError(err, "create scoped resource server")
	ts.scopedRSID = rsID

	managerRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "authz-app-manager-role",
		OUID: ts.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"org"}},
		},
		Assignments: []testutils.Assignment{{ID: ts.managerID, Type: "user"}},
	})
	ts.Require().NoError(err, "create app manager role")
	ts.managerRoleID = managerRoleID

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:     "authz-app-privileged",
		OUID:     ts.ouID,
		ClientID: "authz_app_privileged_client",
	})
	ts.Require().NoError(err, "create privileged application")
	ts.privilegedAppID = appID

	privilegedRole, err := testutils.CreateRole(testutils.Role{
		Name: "authz-app-privileged-role",
		OUID: ts.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"system"}},
		},
		Assignments: []testutils.Assignment{{ID: ts.privilegedAppID, Type: "app"}},
	})
	ts.Require().NoError(err, "create privileged role")
	ts.privilegedRole = privilegedRole

	token, err := ts.signIn.ObtainAccessToken(ts.ouID,
		"org", appAuthzMgrUsername, appAuthzMgrPassword, appAuthzRSIdentifier)
	ts.Require().NoError(err, "obtain app manager token")
	ts.managerClient = testutils.GetHTTPClientWithToken(token.AccessToken)
}

func (ts *ApplicationAuthzTestSuite) TearDownSuite() {
	if err := ts.signIn.DeleteAll(); err != nil {
		ts.T().Logf("teardown: delete sign-in applications: %v", err)
	}
	for _, id := range []string{ts.privilegedRole, ts.managerRoleID} {
		if id != "" {
			if err := testutils.DeleteRole(id); err != nil {
				ts.T().Logf("teardown: delete role %s: %v", id, err)
			}
		}
	}
	if ts.privilegedAppID != "" {
		if err := testutils.DeleteApplication(ts.privilegedAppID); err != nil {
			ts.T().Logf("teardown: delete application: %v", err)
		}
	}
	if ts.scopedRSID != "" {
		if err := testutils.DeleteResourceServerWithChildren(ts.scopedRSID); err != nil {
			ts.T().Errorf("teardown: delete scoped resource server: %v", err)
		}
	}
	if ts.managerID != "" {
		if err := testutils.DeleteUser(ts.managerID); err != nil {
			ts.T().Logf("teardown: delete user: %v", err)
		}
	}
	if ts.typeID != "" {
		if err := testutils.DeleteUserType(ts.typeID); err != nil {
			ts.T().Logf("teardown: delete user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("teardown: delete OU: %v", err)
		}
	}
}

// requireGrantRefused sends the request as the app manager and asserts the grant guard refused it.
func (ts *ApplicationAuthzTestSuite) requireGrantRefused(method string, body []byte) {
	ts.T().Helper()

	req, err := http.NewRequest(method, testutils.TestServerURL+"/applications/"+ts.privilegedAppID,
		bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.managerClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusForbidden, resp.StatusCode, "error body: %s", respBody)

	var errResp testutils.ErrorResponse
	ts.Require().NoError(json.Unmarshal(respBody, &errResp), "error body: %s", respBody)
	ts.Equal("SAZ-4030", errResp.Code)
}

func (ts *ApplicationAuthzTestSuite) TestUpdatePrivilegedApplicationRefused() {
	body, err := json.Marshal(map[string]interface{}{
		"name": "authz-app-privileged",
		"type": "fullstack",
		"ouId": ts.ouID,
	})
	ts.Require().NoError(err)
	ts.requireGrantRefused(http.MethodPut, body)
}

func (ts *ApplicationAuthzTestSuite) TestDeletePrivilegedApplicationRefused() {
	ts.requireGrantRefused(http.MethodDelete, nil)

	resp, err := testutils.GetHTTPClient().Get(testutils.TestServerURL + "/applications/" + ts.privilegedAppID)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Equal(http.StatusOK, resp.StatusCode, "the privileged application must survive the refused delete")
}
