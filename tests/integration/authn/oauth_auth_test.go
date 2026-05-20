/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package authn

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	oauthAuthStart  = "/auth/oauth/standard/start"
	oauthAuthFinish = "/auth/oauth/standard/finish"
	mockOAuthPort   = 8092
)

var oauthAuthTestOU = testutils.OrganizationUnit{
	Handle:      "oauth-auth-test-ou",
	Name:        "OAuth Auth Test Organization Unit",
	Description: "Organization unit for OAuth authentication testing",
	Parent:      nil,
}

var oauthEntityType = testutils.UserType{
	Name: "oauth_user",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{
			"type": "string",
		},
		"password": map[string]interface{}{
			"type":       "string",
			"credential": true,
		},
		"sub": map[string]interface{}{
			"type": "string",
		},
		"email": map[string]interface{}{
			"type": "string",
		},
		"givenName": map[string]interface{}{
			"type": "string",
		},
		"familyName": map[string]interface{}{
			"type": "string",
		},
	},
}

type OAuthAuthTestSuite struct {
	suite.Suite
	mockOAuthServer *testutils.MockOAuthServer
	idpID           string
	userID          string
	entityTypeID    string
	ouID            string
}

func TestOAuthAuthTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthAuthTestSuite))
}

func (suite *OAuthAuthTestSuite) SetupSuite() {
	suite.mockOAuthServer = testutils.NewMockOAuthServer(mockOAuthPort,
		"test-oauth-client", "test-oauth-secret")

	suite.mockOAuthServer.AddUser(&testutils.OAuthUserInfo{
		Sub:     "user123",
		Email:   "testuser@example.com",
		Name:    "Test User",
		Picture: "https://example.com/avatar.jpg",
		Custom: map[string]interface{}{
			"organization": "Test Org",
			"department":   "Engineering",
		},
	})

	err := suite.mockOAuthServer.Start()
	suite.Require().NoError(err, "Failed to start mock OAuth server")

	ouID, err := testutils.CreateOrganizationUnit(oauthAuthTestOU)
	suite.Require().NoError(err, "Failed to create test organization unit")
	suite.ouID = ouID

	oauthEntityType.OUID = suite.ouID
	schemaID, err := testutils.CreateUserType(oauthEntityType)
	suite.Require().NoError(err, "Failed to create OAuth user type")
	suite.entityTypeID = schemaID

	userAttributes := map[string]interface{}{
		"username":   "oauthuser",
		"password":   "Test@1234",
		"sub":        "user123", // Must match OAuth user sub
		"email":      "testuser@example.com",
		"givenName":  "Test",
		"familyName": "User",
	}

	attributesJSON, err := json.Marshal(userAttributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:       oauthEntityType.Name,
		OUID:       suite.ouID,
		Attributes: json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err, "Failed to create test user")
	suite.userID = userID

	idp := testutils.IDP{
		Name:        "Test OAuth IDP",
		Description: "Standard OAuth 2.0 Identity Provider for authentication testing",
		Type:        "OAUTH",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test-oauth-client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "test-oauth-secret",
				IsSecret: true,
			},
			{
				Name:     "authorization_endpoint",
				Value:    suite.mockOAuthServer.GetURL() + "/oauth/authorize",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    suite.mockOAuthServer.GetURL() + "/oauth/token",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    suite.mockOAuthServer.GetURL() + "/oauth/userinfo",
				IsSecret: false,
			},
			{
				Name:     "scopes",
				Value:    "openid profile email",
				IsSecret: false,
			},
			{
				Name:     "redirect_uri",
				Value:    "https://localhost:8095/callback",
				IsSecret: false,
			},
		},
	}

	idpID, err := testutils.CreateIDP(idp)
	suite.Require().NoError(err, "Failed to create OAuth IDP")
	suite.idpID = idpID
}

func (suite *OAuthAuthTestSuite) TearDownSuite() {
	if suite.userID != "" {
		_ = testutils.DeleteUser(suite.userID)
	}

	if suite.entityTypeID != "" {
		_ = testutils.DeleteUserType(suite.entityTypeID)
	}

	if suite.idpID != "" {
		_ = testutils.DeleteIDP(suite.idpID)
	}

	if suite.mockOAuthServer != nil {
		_ = suite.mockOAuthServer.Stop()
	}

	if suite.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.ouID); err != nil {
			suite.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

func (suite *OAuthAuthTestSuite) TestOAuthAuthStartSuccess() {
	// Start authentication
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oauthAuthStart, bytes.NewReader(startRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var startResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&startResponse)
	suite.Require().NoError(err)

	// Verify response contains redirect_url and session_token
	suite.Contains(startResponse, "redirectUrl")
	suite.Contains(startResponse, "sessionToken")

	redirectURL, ok := startResponse["redirectUrl"].(string)
	suite.Require().True(ok)
	suite.Contains(redirectURL, suite.mockOAuthServer.GetURL())
	suite.Contains(redirectURL, "client_id=test-oauth-client")
}

func (suite *OAuthAuthTestSuite) TestOAuthAuthStartInvalidIDPID() {
	startRequest := map[string]interface{}{
		"idpId": "invalid-idp-id",
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oauthAuthStart, bytes.NewReader(startRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)

	var errorResponse testutils.ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errorResponse)
	suite.Require().NoError(err)
	suite.NotEmpty(errorResponse.Code)
	suite.NotEmpty(errorResponse.Message.DefaultValue)
}

func (suite *OAuthAuthTestSuite) TestOAuthAuthStartMissingIDPID() {
	startRequest := map[string]interface{}{}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oauthAuthStart, bytes.NewReader(startRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (suite *OAuthAuthTestSuite) TestOAuthAuthCompleteFlowSuccess() {
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oauthAuthStart, bytes.NewReader(startRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var startResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&startResponse)
	suite.Require().NoError(err)

	sessionToken := startResponse["sessionToken"].(string)
	redirectURL := startResponse["redirectUrl"].(string)

	authCode := suite.simulateOAuthAuthorization(redirectURL)
	suite.Require().NotEmpty(authCode)

	finishRequest := map[string]interface{}{
		"sessionToken": sessionToken,
		"code":         authCode,
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+oauthAuthFinish, bytes.NewReader(finishRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var authResponse testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	suite.Require().NoError(err)

	suite.NotEmpty(authResponse.ID, "Response should contain user ID")
	suite.NotEmpty(authResponse.Type, "Response should contain user type")
	suite.NotEmpty(authResponse.OUID, "Response should contain organization unit")
	suite.Equal(suite.userID, authResponse.ID, "Response should contain the correct user ID")
	suite.NotEmpty(authResponse.Assertion, "Response should contain assertion token by default")
}

func (suite *OAuthAuthTestSuite) TestOAuthAuthFinishInvalidSessionToken() {
	finishRequest := map[string]interface{}{
		"sessionToken": "invalid-session-token",
		"code":         "some-auth-code",
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oauthAuthFinish, bytes.NewReader(finishRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (suite *OAuthAuthTestSuite) TestOAuthAuthFinishMissingCode() {
	finishRequest := map[string]interface{}{
		"sessionToken": "some-session-token",
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oauthAuthFinish, bytes.NewReader(finishRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (suite *OAuthAuthTestSuite) TestOAuthAuthFinishWithError() {
	// Start authentication to get a session token
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oauthAuthStart, bytes.NewReader(startRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	var startResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&startResponse)
	suite.Require().NoError(err)

	sessionToken := startResponse["sessionToken"].(string)

	// Try to finish with error parameter instead of code
	finishRequest := map[string]interface{}{
		"sessionToken":      sessionToken,
		"error":             "access_denied",
		"error_description": "User denied access",
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+oauthAuthFinish, bytes.NewReader(finishRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestOAuthAuthCompleteFlowWithSkipAssertionFalse tests complete OAuth flow with skip_assertion=false
func (suite *OAuthAuthTestSuite) TestOAuthAuthCompleteFlowWithSkipAssertionFalse() {
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oauthAuthStart, bytes.NewReader(startRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var startResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&startResponse)
	suite.Require().NoError(err)

	sessionToken := startResponse["sessionToken"].(string)
	redirectURL := startResponse["redirectUrl"].(string)

	authCode := suite.simulateOAuthAuthorization(redirectURL)
	suite.Require().NotEmpty(authCode)

	finishRequest := map[string]interface{}{
		"sessionToken":  sessionToken,
		"code":          authCode,
		"skipAssertion": false,
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+oauthAuthFinish, bytes.NewReader(finishRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var authResponse testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	suite.Require().NoError(err)

	suite.NotEmpty(authResponse.ID, "Response should contain user ID")
	suite.NotEmpty(authResponse.Type, "Response should contain user type")
	suite.NotEmpty(authResponse.OUID, "Response should contain organization unit")
	suite.Equal(suite.userID, authResponse.ID, "Response should contain the correct user ID")
	suite.NotEmpty(authResponse.Assertion, "Response should contain assertion token when skip_assertion is false")
}

// TestOAuthAuthCompleteFlowWithSkipAssertionTrue tests complete OAuth flow with skip_assertion=true
func (suite *OAuthAuthTestSuite) TestOAuthAuthCompleteFlowWithSkipAssertionTrue() {
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oauthAuthStart, bytes.NewReader(startRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var startResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&startResponse)
	suite.Require().NoError(err)

	sessionToken := startResponse["sessionToken"].(string)
	redirectURL := startResponse["redirectUrl"].(string)

	authCode := suite.simulateOAuthAuthorization(redirectURL)
	suite.Require().NotEmpty(authCode)

	finishRequest := map[string]interface{}{
		"sessionToken":  sessionToken,
		"code":          authCode,
		"skipAssertion": true,
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+oauthAuthFinish, bytes.NewReader(finishRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var authResponse testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	suite.Require().NoError(err)

	suite.NotEmpty(authResponse.ID, "Response should contain user ID")
	suite.NotEmpty(authResponse.Type, "Response should contain user type")
	suite.NotEmpty(authResponse.OUID, "Response should contain organization unit")
	suite.Equal(suite.userID, authResponse.ID, "Response should contain the correct user ID")
	suite.Empty(authResponse.Assertion, "Response should not contain assertion token when skip_assertion is true")
}

// TestOAuthAuthWithAssuranceLevelAAL1 tests that OAuth authentication generates AAL1 assurance level
func (suite *OAuthAuthTestSuite) TestOAuthAuthWithAssuranceLevelAAL1() {
	// Step 1: Start authentication
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oauthAuthStart, bytes.NewReader(startRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	var startResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&startResponse)
	suite.Require().NoError(err)

	sessionToken := startResponse["sessionToken"].(string)
	authCode := suite.simulateOAuthAuthorization(startResponse["redirectUrl"].(string))

	// Step 2: Finish authentication
	finishRequest := map[string]interface{}{
		"sessionToken": sessionToken,
		"code":         authCode,
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+oauthAuthFinish, bytes.NewReader(finishRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	var authResponse testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	suite.Require().NoError(err)

	suite.NotEmpty(authResponse.Assertion, "Response should contain assertion token by default")

	// Verify assertion contains AAL1 for single-factor OAuth authentication
	aal := extractAssuranceLevelFromAssertion(authResponse.Assertion, "aal")
	suite.NotEmpty(aal, "Assertion should contain AAL information")
	suite.Equal("AAL1", aal, "Single-factor OAuth authentication should result in AAL1")

	// Verify IAL is present
	ial := extractAssuranceLevelFromAssertion(authResponse.Assertion, "ial")
	suite.NotEmpty(ial, "Assertion should contain IAL information")
	suite.Equal("IAL1", ial, "Self-asserted identity should result in IAL1")
}

// simulateOAuthAuthorization simulates user authorization and returns authorization code
func (suite *OAuthAuthTestSuite) simulateOAuthAuthorization(redirectURL string) string {
	parsedURL, err := url.Parse(redirectURL)
	suite.Require().NoError(err)

	query := parsedURL.Query()
	clientID := query.Get("client_id")
	state := query.Get("state")
	redirectURI := query.Get("redirect_uri")
	scope := query.Get("scope")

	// Build authorization request to mock server
	authURL := fmt.Sprintf("%s/oauth/authorize?client_id=%s&redirect_uri=%s&scope=%s&state=%s&response_type=code",
		suite.mockOAuthServer.GetURL(),
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(scope),
		url.QueryEscape(state))

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Get(authURL)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	location := resp.Header.Get("Location")
	suite.Require().NotEmpty(location)

	locationURL, err := url.Parse(location)
	suite.Require().NoError(err)

	code := locationURL.Query().Get("code")
	return code
}
