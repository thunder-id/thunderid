// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outype

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

const (
	ouTypeAuthzOUHandle       = "ou-type-authz-ou"
	ouTypeAuthzUserTypeHandle = "ou-type-authz-person"
	ouTypeAuthzUsername       = "ou-type-authz-user"
	ouTypeAuthzPassword       = "OUTypeAuthz@123"
	// The user signs in through an application in its own organization unit, since a sign-in finds only
	// the users of the application's organization unit.
	ouTypeAuthzClientID   = "ou_type_authz_sign_in"
	ouTypeAuthzRSIdentity = "https://authz-test.example.com/outype"
	ouTypeAuthzTypeHandle = "ou-type-authz-partner"

	middlewareForbiddenCode = "AUTH-4030"
)

// OUTypeAuthzTestSuite covers a caller whose only organization unit type permission is org:outype:view.
// It lists and reads the organization unit types of its own organization unit, and the security
// middleware refuses it every write.
type OUTypeAuthzTestSuite struct {
	suite.Suite

	ouID       string
	ouTypeID   string
	userTypeID string
	userID     string
	roleID     string
	scopedRSID string
	signInApp  string

	viewerClient *http.Client
}

func TestOUTypeAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(OUTypeAuthzTestSuite))
}

func (s *OUTypeAuthzTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      ouTypeAuthzOUHandle,
		Name:        "OU Type Authz OU",
		Description: "Organization unit of the view-only caller of the organization unit type authz test",
	})
	s.Require().NoError(err)
	s.ouID = ouID

	status, body := s.send(testutils.GetHTTPClient(), http.MethodPost, ouTypeBasePath, OUTypeRequest{
		Handle: ouTypeAuthzTypeHandle, DisplayName: "Partner", OUID: s.ouID, Schema: partnerSchema,
	})
	s.Require().Equal(http.StatusCreated, status, "body: %s", string(body))
	var created OUType
	s.Require().NoError(json.Unmarshal(body, &created))
	s.ouTypeID = created.ID

	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      ouTypeAuthzUserTypeHandle,
		DisplayName: "OU Type Authz Person",
		OUID:        s.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	s.Require().NoError(err)
	s.userTypeID = userTypeID

	userID, err := testutils.CreateUser(testutils.User{
		Type: ouTypeAuthzUserTypeHandle,
		OUID: s.ouID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q}`, ouTypeAuthzUsername, ouTypeAuthzPassword)),
	})
	s.Require().NoError(err)
	s.userID = userID

	rsID, err := testutils.CreateOrgScopedResourceServer(
		s.ouID, "Authz Test RS (outype)", ouTypeAuthzRSIdentity, "outype")
	s.Require().NoError(err)
	s.scopedRSID = rsID

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "OU Type Viewer Role",
		OUID: s.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"org:outype:view"}},
		},
		Assignments: []testutils.Assignment{{ID: s.userID, Type: "user"}},
	})
	s.Require().NoError(err)
	s.roleID = roleID

	s.signInApp, err = testutils.CreateOUSignInApplication(s.ouID, ouTypeAuthzClientID)
	s.Require().NoError(err)

	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		ouTypeAuthzClientID, testutils.OUSignInRedirectURI, "org:outype:view",
		ouTypeAuthzUsername, ouTypeAuthzPassword, true, "", ouTypeAuthzRSIdentity)
	s.Require().NoError(err)
	granted := strings.Fields(tokenResp.Scope)
	s.Require().Contains(granted, "org:outype:view", "the token must carry the view scope")
	for _, scope := range granted {
		s.Require().NotContains([]string{"org:outype", "org:outype:create", "org:outype:update",
			"org:outype:delete"}, scope, "the token must carry no write scope, or the suite proves nothing")
	}
	s.viewerClient = testutils.GetHTTPClientWithToken(tokenResp.AccessToken)
}

func (s *OUTypeAuthzTestSuite) TearDownSuite() {
	if s.ouTypeID != "" {
		status, body := s.send(testutils.GetHTTPClient(), http.MethodDelete, ouTypeBasePath+"/"+s.ouTypeID, nil)
		s.Equal(http.StatusNoContent, status, "teardown: delete organization unit type: %s", string(body))
	}
	if s.signInApp != "" {
		s.NoError(testutils.DeleteApplication(s.signInApp), "teardown: delete sign-in application")
	}
	if s.roleID != "" {
		s.NoError(testutils.DeleteRole(s.roleID), "teardown: delete role")
	}
	if s.scopedRSID != "" {
		s.NoError(testutils.DeleteResourceServerWithChildren(s.scopedRSID),
			"teardown: delete scoped resource server and its resource tree")
	}
	if s.userID != "" {
		s.NoError(testutils.DeleteUser(s.userID), "teardown: delete user")
	}
	if s.userTypeID != "" {
		s.NoError(testutils.DeleteUserType(s.userTypeID), "teardown: delete user type")
	}
	if s.ouID != "" {
		s.NoError(testutils.DeleteOrganizationUnit(s.ouID), "teardown: delete OU")
	}
}

// send issues a request with the given client and returns the status and body.
func (s *OUTypeAuthzTestSuite) send(client *http.Client, method, path string, body any) (int, []byte) {
	s.T().Helper()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		s.Require().NoError(err)
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	s.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	return resp.StatusCode, respBody
}

// requireForbidden asserts the security middleware's permission refusal.
func (s *OUTypeAuthzTestSuite) requireForbidden(status int, body []byte) {
	s.T().Helper()

	s.Require().Equal(http.StatusForbidden, status, "body: %s", string(body))
	var errResp ErrorResponse
	s.Require().NoError(json.Unmarshal(body, &errResp), "body: %s", string(body))
	s.Equal(middlewareForbiddenCode, errResp.Code)
}

// requireUnchanged asserts the organization unit type is still stored as created.
func (s *OUTypeAuthzTestSuite) requireUnchanged() {
	s.T().Helper()

	status, body := s.send(testutils.GetHTTPClient(), http.MethodGet, ouTypeBasePath+"/"+s.ouTypeID, nil)
	s.Require().Equal(http.StatusOK, status, "body: %s", string(body))
	var stored OUType
	s.Require().NoError(json.Unmarshal(body, &stored))
	s.Equal("Partner", stored.DisplayName)
}

func (s *OUTypeAuthzTestSuite) TestViewerListsOUTypes() {
	status, body := s.send(s.viewerClient, http.MethodGet, ouTypeBasePath+"?limit=100", nil)
	s.Require().Equal(http.StatusOK, status, "body: %s", string(body))
	var list OUTypeListResponse
	s.Require().NoError(json.Unmarshal(body, &list))
	s.Contains(listIDs(list), s.ouTypeID)
}

func (s *OUTypeAuthzTestSuite) TestViewerReadsOUType() {
	status, body := s.send(s.viewerClient, http.MethodGet, ouTypeBasePath+"/"+s.ouTypeID, nil)
	s.Require().Equal(http.StatusOK, status, "body: %s", string(body))
}

func (s *OUTypeAuthzTestSuite) TestViewerCannotCreateOUType() {
	s.requireForbidden(s.send(s.viewerClient, http.MethodPost, ouTypeBasePath, OUTypeRequest{
		Handle: "ou-type-authz-refused", DisplayName: "Refused", OUID: s.ouID, Schema: partnerSchema,
	}))
}

func (s *OUTypeAuthzTestSuite) TestViewerCannotUpdateOUType() {
	s.requireForbidden(s.send(s.viewerClient, http.MethodPut, ouTypeBasePath+"/"+s.ouTypeID, OUTypeRequest{
		DisplayName: "Changed", OUID: s.ouID, Schema: partnerSchema,
	}))
	s.requireUnchanged()
}

func (s *OUTypeAuthzTestSuite) TestViewerCannotDeleteOUType() {
	s.requireForbidden(s.send(s.viewerClient, http.MethodDelete, ouTypeBasePath+"/"+s.ouTypeID, nil))
	s.requireUnchanged()
}
