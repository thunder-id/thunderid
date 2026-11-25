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
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	ccAuthzClientID                 = "cc_authz_test_client"
	ccAuthzClientSecret             = "cc_authz_test_secret"
	ccAuthzResourceServerIdentifier = "https://cc-authz.example.com"
	ccAuthzSecondaryRSIdentifier    = "https://cc-authz-secondary.example.com"
)

type CCAppAuthzTestSuite struct {
	suite.Suite
	client                    *http.Client
	ouID                      string
	appID                     string
	resourceServerID          string
	secondaryResourceServerID string
	roleID                    string
	groupID                   string
	groupRoleID               string
	subtreeRoleID             string
}

func TestCCAppAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(CCAppAuthzTestSuite))
}

func (s *CCAppAuthzTestSuite) SetupSuite() {
	s.client = testutils.GetHTTPClient()

	// Create organization unit
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "cc-authz-test-ou",
		Name:        "CC Authz Test OU",
		Description: "Organization unit for CC app authorization tests",
	})
	s.Require().NoError(err)
	s.ouID = ouID

	// Create resource server with actions
	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "CC Authz API",
		Description: "Resource server for CC authz testing",
		Identifier:  ccAuthzResourceServerIdentifier,
		OUID:        s.ouID,
		// Declared hierarchical so a role granting a resource carries the actions beneath it; the
		// subtree cases below rely on it, and the rest of the suite is unaffected either way.
		HierarchicalPermissions: true,
	}, []testutils.Action{
		{Name: "Read", Handle: "read", Description: "Read access"},
		{Name: "Write", Handle: "write", Description: "Write access"},
		{Name: "Delete", Handle: "delete", Description: "Delete access"},
		{Name: "Approve", Handle: "approve", Description: "Approve access"},
	})
	s.Require().NoError(err)
	s.resourceServerID = rsID

	secondaryRSID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "CC Authz Secondary API",
		Description: "Secondary resource server for CC authz testing",
		Identifier:  ccAuthzSecondaryRSIdentifier,
		OUID:        s.ouID,
	}, []testutils.Action{
		{Name: "Read", Handle: "read", Description: "Read access"},
	})
	s.Require().NoError(err)
	s.secondaryResourceServerID = secondaryRSID

	// Create OAuth application with client_credentials grant
	appID, err := s.createOAuthApp()
	s.Require().NoError(err)
	s.appID = appID

	// Create role with read and write permissions, assigned to the app
	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        "CC Authz Test Role",
		Description: "Role for CC app authz testing",
		OUID:        s.ouID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: s.resourceServerID,
				Permissions:      []string{"read", "write"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: s.appID, Type: "app"},
		},
	})
	s.Require().NoError(err)
	s.roleID = roleID

	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:        "CC Authz App Group",
		Description: "Group for app-based role inheritance",
		OUID:        s.ouID,
		Members: []testutils.Member{
			{Id: s.appID, Type: "app"},
		},
	})
	s.Require().NoError(err)
	s.groupID = groupID

	groupRoleID, err := testutils.CreateRole(testutils.Role{
		Name:        "CC Authz App Group Role",
		Description: "Role assigned to group containing app",
		OUID:        s.ouID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: s.resourceServerID,
				Permissions:      []string{"approve"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: s.groupID, Type: "group"},
		},
	})
	s.Require().NoError(err)
	s.groupRoleID = groupRoleID

	// A resource with an action beneath it, granted by the resource rather than by the action, so the
	// token endpoint has to resolve "bookings:cancel" through the "bookings" the role actually holds.
	bookingsResourceID, err := testutils.CreateResource(s.resourceServerID, "Bookings", "bookings", "")
	s.Require().NoError(err)
	_, err = testutils.CreateResourceAction(s.resourceServerID, bookingsResourceID,
		testutils.Action{Name: "Cancel", Handle: "cancel", Description: "Cancel a booking"})
	s.Require().NoError(err)

	subtreeRoleID, err := testutils.CreateRole(testutils.Role{
		Name:        "CC Authz Bookings Role",
		Description: "Role granting the bookings resource rather than its actions",
		OUID:        s.ouID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: s.resourceServerID,
				Permissions:      []string{"bookings"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: s.appID, Type: "app"},
		},
	})
	s.Require().NoError(err)
	s.subtreeRoleID = subtreeRoleID
}

func (s *CCAppAuthzTestSuite) TearDownSuite() {
	if s.subtreeRoleID != "" {
		_ = testutils.DeleteRole(s.subtreeRoleID)
	}
	if s.groupRoleID != "" {
		_ = testutils.DeleteRole(s.groupRoleID)
	}
	if s.roleID != "" {
		_ = testutils.DeleteRole(s.roleID)
	}
	if s.groupID != "" {
		_ = testutils.DeleteGroup(s.groupID)
	}
	if s.appID != "" {
		_ = testutils.DeleteApplication(s.appID)
	}
	if s.secondaryResourceServerID != "" {
		s.NoError(testutils.DeleteResourceServerWithChildren(s.secondaryResourceServerID))
	}
	if s.resourceServerID != "" {
		s.NoError(testutils.DeleteResourceServerWithChildren(s.resourceServerID))
	}
	if s.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(s.ouID)
	}
}

func (s *CCAppAuthzTestSuite) createOAuthApp() (string, error) {
	app := map[string]interface{}{
		"name":                      "CC Authz Test App",
		"description":               "Application for CC authorization testing",
		"ouId":                      s.ouID,
		"type":                      "m2m",
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                ccAuthzClientID,
					"clientSecret":            ccAuthzClientSecret,
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", testServerURL+"/applications", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to create app: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &respData); err != nil {
		return "", err
	}

	return respData["id"].(string), nil
}

func (s *CCAppAuthzTestSuite) requestToken(scopes string) (int, map[string]interface{}) {
	return s.requestTokenWithResource(scopes, ccAuthzResourceServerIdentifier)
}

func (s *CCAppAuthzTestSuite) requestTokenWithResource(scopes string, resource string) (int, map[string]interface{}) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	if scopes != "" {
		form.Set("scope", scopes)
	}
	if resource != "" {
		form.Set("resource", resource)
	}

	req, err := http.NewRequest("POST", testServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(ccAuthzClientID, ccAuthzClientSecret)

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	var respBody map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&respBody))

	return resp.StatusCode, respBody
}

func (s *CCAppAuthzTestSuite) assertAccessTokenAudience(body map[string]interface{}, expected string) {
	token, ok := body["access_token"].(string)
	s.Require().True(ok, "Response should contain access_token")

	claims, err := testutils.DecodeJWT(token)
	s.Require().NoError(err)
	s.Equal(expected, claims.Aud)
}

// TestCCWithAuthorizedScopes requests scopes that are assigned to the app via a role.
func (s *CCAppAuthzTestSuite) TestCCWithAuthorizedScopes() {
	status, body := s.requestToken("read write")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")
	s.assertAccessTokenAudience(body, ccAuthzResourceServerIdentifier)

	scopeStr, ok := body["scope"].(string)
	s.Require().True(ok, "Response should contain scope")
	scopes := strings.Fields(scopeStr)
	s.ElementsMatch([]string{"read", "write"}, scopes)
}

// TestCCWithPartialAuthorization requests both authorized and unauthorized scopes.
func (s *CCAppAuthzTestSuite) TestCCWithPartialAuthorization() {
	status, body := s.requestToken("read delete")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")
	s.assertAccessTokenAudience(body, ccAuthzResourceServerIdentifier)

	scopeStr, ok := body["scope"].(string)
	s.Require().True(ok, "Response should contain scope")
	scopes := strings.Fields(scopeStr)
	s.ElementsMatch([]string{"read"}, scopes)
}

// TestCCWithUnauthorizedScopes requests only scopes the app is not authorized for.
func (s *CCAppAuthzTestSuite) TestCCWithUnauthorizedScopes() {
	status, body := s.requestToken("delete")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")
	s.assertAccessTokenAudience(body, ccAuthzResourceServerIdentifier)

	_, hasScope := body["scope"]
	s.False(hasScope, "Response should not contain scope when no scopes are authorized")
}

// TestCCWithNoScopes requests no scopes — token should be issued without scopes.
func (s *CCAppAuthzTestSuite) TestCCWithNoScopes() {
	status, body := s.requestToken("")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")
	s.assertAccessTokenAudience(body, ccAuthzResourceServerIdentifier)

	_, hasScope := body["scope"]
	s.False(hasScope, "Response should not contain scope when no scopes are requested")
}

// TestCCWithSingleAuthorizedScope requests a single authorized scope.
func (s *CCAppAuthzTestSuite) TestCCWithSingleAuthorizedScope() {
	status, body := s.requestToken("write")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")
	s.assertAccessTokenAudience(body, ccAuthzResourceServerIdentifier)

	scopeStr, ok := body["scope"].(string)
	s.Require().True(ok, "Response should contain scope")
	s.Equal("write", scopeStr)
}

// TestCCWithGroupInheritedScope requests a scope assigned via a role bound to a group that contains the app.
func (s *CCAppAuthzTestSuite) TestCCWithGroupInheritedScope() {
	status, body := s.requestToken("approve")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")
	s.assertAccessTokenAudience(body, ccAuthzResourceServerIdentifier)

	scopeStr, ok := body["scope"].(string)
	s.Require().True(ok, "Response should contain scope")
	s.Equal("approve", scopeStr)
}

func (s *CCAppAuthzTestSuite) TestCCWithExplicitResourceBindsAudience() {
	status, body := s.requestTokenWithResource("read write", ccAuthzResourceServerIdentifier)
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")
	s.assertAccessTokenAudience(body, ccAuthzResourceServerIdentifier)

	scopeStr, ok := body["scope"].(string)
	s.Require().True(ok, "Response should contain scope")
	s.ElementsMatch([]string{"read", "write"}, strings.Fields(scopeStr))
}

func (s *CCAppAuthzTestSuite) TestCCWithExplicitResourceFiltersPermissionsByTargetRS() {
	status, body := s.requestTokenWithResource("read", ccAuthzSecondaryRSIdentifier)
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")
	s.assertAccessTokenAudience(body, ccAuthzSecondaryRSIdentifier)

	_, hasScope := body["scope"]
	s.False(hasScope, "Response should not contain scopes authorized only on another resource server")
}

// A role granting a resource authorizes a scope naming an action beneath it, without that action
// being granted individually.
func (s *CCAppAuthzTestSuite) TestCCScopeBeneathAGrantedResource() {
	status, body := s.requestToken("bookings:cancel")
	s.Equal(http.StatusOK, status)
	s.assertAccessTokenAudience(body, ccAuthzResourceServerIdentifier)

	scopeStr, ok := body["scope"].(string)
	s.Require().True(ok, "Response should contain scope")
	s.ElementsMatch([]string{"bookings:cancel"}, strings.Fields(scopeStr))
}

// The granted resource is also a permission in its own right, so requesting it directly works too.
func (s *CCAppAuthzTestSuite) TestCCScopeForTheGrantedResourceItself() {
	status, body := s.requestToken("bookings")
	s.Equal(http.StatusOK, status)

	scopeStr, ok := body["scope"].(string)
	s.Require().True(ok, "Response should contain scope")
	s.ElementsMatch([]string{"bookings"}, strings.Fields(scopeStr))
}

// Holding an action never confers the resource it sits under. "read" is a server-level action, so
// nothing the app holds covers a request for a resource it was not granted.
func (s *CCAppAuthzTestSuite) TestCCScopeIsNotWidenedUpwards() {
	status, body := s.requestToken("read bookings:cancel")
	s.Equal(http.StatusOK, status)

	scopeStr, _ := body["scope"].(string)
	scopes := strings.Fields(scopeStr)
	s.ElementsMatch([]string{"read", "bookings:cancel"}, scopes)
	s.NotContains(scopes, "write", "nothing beyond what was requested is issued")
}

func (s *CCAppAuthzTestSuite) TestCCRootAndChildScopesAreCompacted() {
	status, body := s.requestToken("bookings bookings:cancel read")
	s.Require().Equal(http.StatusOK, status)
	s.ElementsMatch([]string{"bookings", "read"}, strings.Fields(body["scope"].(string)))
	claims, err := testutils.DecodeJWT(body["access_token"].(string))
	s.Require().NoError(err)
	s.Equal(body["scope"], claims.Additional["scope"])
}

func (s *CCAppAuthzTestSuite) TestCCFileRoleFallbackExcludesUndeclaredPermissions() {
	const roleID = "decl-hierarchical-fallback-role"
	const clientID = "cc_catalogue_fallback_client"
	const clientSecret = "cc_catalogue_fallback_secret"
	appID, err := testutils.CreateApplication(testutils.Application{
		Name: "CC Catalogue Fallback App", OUID: "01900000-0000-7000-8000-000000000001", Type: "m2m",
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId": clientID, "clientSecret": clientSecret,
				"grantTypes": []string{"client_credentials"}, "tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	s.Require().NoError(err)
	s.T().Cleanup(func() { s.NoError(testutils.DeleteApplication(appID)) })

	body, err := json.Marshal(map[string]interface{}{
		"assignments": []testutils.Assignment{{ID: appID, Type: "app"}},
	})
	s.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost,
		testServerURL+"/roles/"+roleID+"/assignments/add", bytes.NewReader(body))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(req)
	s.Require().NoError(err)
	defer response.Body.Close()
	s.Require().Equal(http.StatusNoContent, response.StatusCode)
	s.T().Cleanup(func() {
		s.NoError(testutils.RemoveRoleAssignments(roleID, []testutils.Assignment{{ID: appID, Type: "app"}}))
	})

	for _, requested := range []string{"org:retired", "org"} {
		s.Run(requested, func() {
			result, tokenErr := testutils.RequestClientCredentialsToken(
				clientID, clientSecret, requested, testutils.SystemResourceIdentifier)
			s.Require().NoError(tokenErr)
			s.Require().Equal(http.StatusOK, result.StatusCode)
			s.Require().NotNil(result.Token)
			if requested == "org" {
				s.Equal("org:user:view", result.Token.Scope)
			} else {
				s.Empty(result.Token.Scope)
			}
			claims, decodeErr := testutils.DecodeJWT(result.Token.AccessToken)
			s.Require().NoError(decodeErr)
			if requested == "org" {
				s.Equal(result.Token.Scope, claims.Additional["scope"])
			} else {
				s.Empty(claims.Additional["scope"])
			}
		})
	}
}
