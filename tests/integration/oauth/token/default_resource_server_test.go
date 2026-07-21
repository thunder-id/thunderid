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
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	defaultRSTestClientID     = "default_rs_test_client"
	defaultRSTestClientSecret = "default_rs_test_secret"
	defaultRSTestIdentifier   = "https://default-rs-token.example.com"
)

type DefaultResourceServerTestSuite struct {
	suite.Suite
	client           *http.Client
	ouID             string
	appID            string
	resourceServerID string
}

func TestDefaultResourceServerTestSuite(t *testing.T) {
	suite.Run(t, new(DefaultResourceServerTestSuite))
}

func (s *DefaultResourceServerTestSuite) SetupSuite() {
	s.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "default-rs-token-ou",
		Name:        "Default Resource Server Token OU",
		Description: "Organization unit for default resource server token tests",
	})
	s.Require().NoError(err)
	s.ouID = ouID

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Default Resource Server Token API",
		Description: "Resource server for default fallback token tests",
		Identifier:  defaultRSTestIdentifier,
		OUID:        s.ouID,
	}, []testutils.Action{})
	s.Require().NoError(err)
	s.resourceServerID = rsID

	s.appID = s.createOAuthApp()
}

func (s *DefaultResourceServerTestSuite) TearDownSuite() {
	_ = testutils.PutDefaultResourceServer("")
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

func (s *DefaultResourceServerTestSuite) createOAuthApp() string {
	app := map[string]interface{}{
		"name":                      "Default Resource Server Token App",
		"description":               "Application for default resource server token tests",
		"ouId":                      s.ouID,
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                defaultRSTestClientID,
					"clientSecret":            defaultRSTestClientSecret,
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}

	payload, err := json.Marshal(app)
	s.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/applications", bytes.NewReader(payload))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	s.Require().Equal(http.StatusCreated, resp.StatusCode, string(body))

	var respBody map[string]interface{}
	s.Require().NoError(json.Unmarshal(body, &respBody))
	return respBody["id"].(string)
}

func (s *DefaultResourceServerTestSuite) requestClientCredentials(scope, resource string) (int, map[string]interface{}) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	if scope != "" {
		form.Set("scope", scope)
	}
	if resource != "" {
		form.Set("resource", resource)
	}

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(defaultRSTestClientID, defaultRSTestClientSecret)

	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	var respBody map[string]interface{}
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&respBody))
	return resp.StatusCode, respBody
}

func (s *DefaultResourceServerTestSuite) TestNoResourceWithPermissionScopeUsesConfiguredDefaultResourceServer() {
	s.Require().NoError(testutils.PutDefaultResourceServer(s.resourceServerID))

	status, body := s.requestClientCredentials("read", "")
	s.Equal(http.StatusOK, status)

	token, ok := body["access_token"].(string)
	s.Require().True(ok, "response should contain an access token")

	claims, err := testutils.DecodeJWT(token)
	s.Require().NoError(err)
	s.Equal(defaultRSTestIdentifier, claims.Aud)
}

func (s *DefaultResourceServerTestSuite) TestNoResourceWithoutDefaultAndNoScopesUsesClientIDAudience() {
	s.Require().NoError(testutils.PutDefaultResourceServer(""))

	status, body := s.requestClientCredentials("", "")
	s.Equal(http.StatusOK, status)

	token, ok := body["access_token"].(string)
	s.Require().True(ok, "response should contain an access token")

	claims, err := testutils.DecodeJWT(token)
	s.Require().NoError(err)
	s.Equal(defaultRSTestClientID, claims.Aud)
	s.NotContains(body, "scope")
}

func (s *DefaultResourceServerTestSuite) TestNoResourceWithoutDefaultAndPermissionScopeRejects() {
	s.Require().NoError(testutils.PutDefaultResourceServer(""))

	status, body := s.requestClientCredentials("read", "")
	s.Equal(http.StatusBadRequest, status)
	s.Equal("invalid_target", body["error"])
}
