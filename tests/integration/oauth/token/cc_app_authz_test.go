/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package token

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	ccAuthzClientID     = "cc_authz_test_client"
	ccAuthzClientSecret = "cc_authz_test_secret"
)

type CCAppAuthzTestSuite struct {
	suite.Suite
	client           *http.Client
	ouID             string
	appID            string
	resourceServerID string
	roleID           string
	groupID          string
	groupRoleID      string
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
		Identifier:  "cc-authz-api",
		OUID:        s.ouID,
	}, []testutils.Action{
		{Name: "Read", Handle: "read", Description: "Read access"},
		{Name: "Write", Handle: "write", Description: "Write access"},
		{Name: "Delete", Handle: "delete", Description: "Delete access"},
		{Name: "Approve", Handle: "approve", Description: "Approve access"},
	})
	s.Require().NoError(err)
	s.resourceServerID = rsID

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
}

func (s *CCAppAuthzTestSuite) TearDownSuite() {
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
	if s.resourceServerID != "" {
		_ = testutils.DeleteResourceServer(s.resourceServerID)
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
	body := "grant_type=client_credentials"
	if scopes != "" {
		body += "&scope=" + scopes
	}

	req, err := http.NewRequest("POST", testServerURL+"/oauth2/token", strings.NewReader(body))
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

// TestCCWithAuthorizedScopes requests scopes that are assigned to the app via a role.
func (s *CCAppAuthzTestSuite) TestCCWithAuthorizedScopes() {
	status, body := s.requestToken("read write")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")

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

	_, hasScope := body["scope"]
	s.False(hasScope, "Response should not contain scope when no scopes are authorized")
}

// TestCCWithNoScopes requests no scopes — token should be issued without scopes.
func (s *CCAppAuthzTestSuite) TestCCWithNoScopes() {
	status, body := s.requestToken("")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")

	_, hasScope := body["scope"]
	s.False(hasScope, "Response should not contain scope when no scopes are requested")
}

// TestCCWithSingleAuthorizedScope requests a single authorized scope.
func (s *CCAppAuthzTestSuite) TestCCWithSingleAuthorizedScope() {
	status, body := s.requestToken("write")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")

	scopeStr, ok := body["scope"].(string)
	s.Require().True(ok, "Response should contain scope")
	s.Equal("write", scopeStr)
}

// TestCCWithGroupInheritedScope requests a scope assigned via a role bound to a group that contains the app.
func (s *CCAppAuthzTestSuite) TestCCWithGroupInheritedScope() {
	status, body := s.requestToken("approve")
	s.Equal(http.StatusOK, status)
	s.Contains(body, "access_token")

	scopeStr, ok := body["scope"].(string)
	s.Require().True(ok, "Response should contain scope")
	s.Equal("approve", scopeStr)
}
