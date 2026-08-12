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
	ccClientAttrsClientID                 = "cc_client_attrs_test_client"
	ccClientAttrsClientSecret             = "cc_client_attrs_test_secret"
	ccClientAttrsResourceServerIdentifier = "https://cc-client-attrs.example.com"
)

// CCClientAttributesTestSuite verifies that client_credentials access tokens surface
// client-scoped claims (ouId/ouName/ouHandle, groups, roles) selected by the app's
// token.accessToken.clientConfig.attributes allow-list.
type CCClientAttributesTestSuite struct {
	suite.Suite
	client           *http.Client
	ouID             string
	resourceServerID string
}

// TestCCClientAttributesTestSuite runs the CCClientAttributesTestSuite.
func TestCCClientAttributesTestSuite(t *testing.T) {
	suite.Run(t, new(CCClientAttributesTestSuite))
}

// SetupSuite creates the shared organization unit and resource server for the suite.
func (s *CCClientAttributesTestSuite) SetupSuite() {
	s.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "cc-client-attrs-ou",
		Name:        "CC Client Attributes OU",
		Description: "Organization unit for CC client-attribute integration tests",
	})
	s.Require().NoError(err)
	s.ouID = ouID

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "CC Client Attributes API",
		Description: "Resource server for CC client-attribute testing",
		Identifier:  ccClientAttrsResourceServerIdentifier,
		OUID:        s.ouID,
	}, []testutils.Action{})
	s.Require().NoError(err)
	s.resourceServerID = rsID
}

// TearDownSuite deletes the shared organization unit and resource server created in SetupSuite.
func (s *CCClientAttributesTestSuite) TearDownSuite() {
	if s.resourceServerID != "" {
		_ = testutils.DeleteResourceServer(s.resourceServerID)
	}
	if s.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(s.ouID)
	}
}

// createOAuthApp creates a client_credentials OAuth application with the given
// clientConfig.attributes allow-list.
func (s *CCClientAttributesTestSuite) createOAuthApp(clientID, clientSecret string, clientAttributes []string) (string, error) {
	app := map[string]interface{}{
		"name":                      "CC Client Attrs Test App " + clientID,
		"description":               "Application for CC client-attribute testing",
		"ouId":                      s.ouID,
		"type":                      "m2m",
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"clientSecret":            clientSecret,
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"token": map[string]interface{}{
						"accessToken": map[string]interface{}{
							"clientConfig": map[string]interface{}{
								"attributes": clientAttributes,
							},
						},
					},
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

// requestToken performs a client_credentials token request for the given client credentials.
func (s *CCClientAttributesTestSuite) requestToken(clientID, clientSecret string) (int, map[string]interface{}) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("resource", ccClientAttrsResourceServerIdentifier)

	req, err := http.NewRequest("POST", testServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	var respBody map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&respBody))
	return resp.StatusCode, respBody
}

// TestCCClientAttrs_OUAndGroupsAndRoles verifies that ouId/ouName/ouHandle, groups, and
// roles (both directly assigned and group-inherited) are all present in the access token
// when allow-listed.
func (s *CCClientAttributesTestSuite) TestCCClientAttrs_OUAndGroupsAndRoles() {
	clientID := ccClientAttrsClientID + "_full"
	appID, err := s.createOAuthApp(clientID, ccClientAttrsClientSecret,
		[]string{"ouId", "ouName", "ouHandle", "groups", "roles"})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	directRoleID, err := testutils.CreateRole(testutils.Role{
		Name:        "CC Client Attrs Direct Role",
		Description: "Role assigned directly to the app",
		OUID:        s.ouID,
		Assignments: []testutils.Assignment{{ID: appID, Type: "app"}},
	})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteRole(directRoleID) }()

	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:    "CC Client Attrs Group",
		OUID:    s.ouID,
		Members: []testutils.Member{{Id: appID, Type: "app"}},
	})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteGroup(groupID) }()

	groupRoleID, err := testutils.CreateRole(testutils.Role{
		Name:        "CC Client Attrs Group Role",
		Description: "Role assigned to the group containing the app",
		OUID:        s.ouID,
		Assignments: []testutils.Assignment{{ID: groupID, Type: "group"}},
	})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteRole(groupRoleID) }()

	status, body := s.requestToken(clientID, ccClientAttrsClientSecret)
	s.Require().Equal(http.StatusOK, status)
	token, ok := body["access_token"].(string)
	s.Require().True(ok, "Response should contain access_token")

	claims, err := testutils.DecodeJWT(token)
	s.Require().NoError(err)

	s.Assert().Equal(s.ouID, claims.Additional["ouId"], "ouId claim should match the app's OU")
	s.Assert().Equal("CC Client Attributes OU", claims.Additional["ouName"])
	s.Assert().Equal("cc-client-attrs-ou", claims.Additional["ouHandle"])

	groups, ok := claims.Additional["groups"].([]interface{})
	s.Require().True(ok, "groups claim should be present as an array")
	s.Assert().Contains(groups, "CC Client Attrs Group")

	roles, ok := claims.Additional["roles"].([]interface{})
	s.Require().True(ok, "roles claim should be present as an array")
	s.Assert().ElementsMatch([]interface{}{"CC Client Attrs Direct Role", "CC Client Attrs Group Role"}, roles)
}

// TestCCClientAttrs_PartialAllowList verifies that only the allow-listed attributes are
// included, even when the app is eligible for others (OU, groups, roles all resolvable).
func (s *CCClientAttributesTestSuite) TestCCClientAttrs_PartialAllowList() {
	clientID := ccClientAttrsClientID + "_partial"
	appID, err := s.createOAuthApp(clientID, ccClientAttrsClientSecret, []string{"ouId"})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:    "CC Client Attrs Partial Group",
		OUID:    s.ouID,
		Members: []testutils.Member{{Id: appID, Type: "app"}},
	})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteGroup(groupID) }()

	status, body := s.requestToken(clientID, ccClientAttrsClientSecret)
	s.Require().Equal(http.StatusOK, status)
	token, ok := body["access_token"].(string)
	s.Require().True(ok)

	claims, err := testutils.DecodeJWT(token)
	s.Require().NoError(err)

	s.Assert().Equal(s.ouID, claims.Additional["ouId"])
	s.Assert().NotContains(claims.Additional, "ouName", "ouName must be excluded when not allow-listed")
	s.Assert().NotContains(claims.Additional, "ouHandle", "ouHandle must be excluded when not allow-listed")
	s.Assert().NotContains(claims.Additional, "groups", "groups must be excluded when not allow-listed")
	s.Assert().NotContains(claims.Additional, "roles", "roles must be excluded when not allow-listed")
}

// TestCCClientAttrs_NoAllowListConfigured verifies that no client-scoped claims are added
// when the app has no clientConfig.attributes configured at all.
func (s *CCClientAttributesTestSuite) TestCCClientAttrs_NoAllowListConfigured() {
	clientID := ccClientAttrsClientID + "_none"
	appID, err := s.createOAuthApp(clientID, ccClientAttrsClientSecret, nil)
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteApplication(appID) }()

	status, body := s.requestToken(clientID, ccClientAttrsClientSecret)
	s.Require().Equal(http.StatusOK, status)
	token, ok := body["access_token"].(string)
	s.Require().True(ok)

	claims, err := testutils.DecodeJWT(token)
	s.Require().NoError(err)

	s.Assert().NotContains(claims.Additional, "ouId")
	s.Assert().NotContains(claims.Additional, "ouName")
	s.Assert().NotContains(claims.Additional, "ouHandle")
	s.Assert().NotContains(claims.Additional, "groups")
	s.Assert().NotContains(claims.Additional, "roles")
}
