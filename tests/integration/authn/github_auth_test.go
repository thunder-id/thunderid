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
	githubAuthStart  = "/auth/oauth/github/start"
	githubAuthFinish = "/auth/oauth/github/finish"
	mockGithubPort   = 8091
)

var githubAuthTestOU = testutils.OrganizationUnit{
	Handle:      "github-auth-test-ou",
	Name:        "GitHub Auth Test Organization Unit",
	Description: "Organization unit for GitHub authentication testing",
	Parent:      nil,
}

var githubEntityType = testutils.UserType{
	Name: "github_user",
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

type GithubAuthTestSuite struct {
	suite.Suite
	mockGithubServer *testutils.MockGithubOAuthServer
	idpID            string
	userID           string
	entityTypeID     string
	ouID             string
}

func TestGithubAuthTestSuite(t *testing.T) {
	suite.Run(t, new(GithubAuthTestSuite))
}

func (suite *GithubAuthTestSuite) SetupSuite() {
	suite.mockGithubServer = testutils.NewMockGithubOAuthServer(mockGithubPort,
		"test-github-client", "test-github-secret")

	email := "testuser@github.com"
	suite.mockGithubServer.AddUser(&testutils.GithubUserInfo{
		Login:     "testuser",
		ID:        12345,
		NodeID:    "MDQ6VXNlcjEyMzQ1",
		Email:     &email,
		Name:      "Test User",
		AvatarURL: "https://avatars.githubusercontent.com/u/12345",
		Type:      "User",
		CreatedAt: "2020-01-01T00:00:00Z",
		UpdatedAt: "2024-01-01T00:00:00Z",
	}, []*testutils.GithubEmail{
		{
			Email:    email,
			Primary:  true,
			Verified: true,
		},
	})

	err := suite.mockGithubServer.Start()
	suite.Require().NoError(err, "Failed to start mock GitHub server")

	ouID, err := testutils.CreateOrganizationUnit(githubAuthTestOU)
	suite.Require().NoError(err, "Failed to create test organization unit")
	suite.ouID = ouID

	githubEntityType.OUID = suite.ouID
	schemaID, err := testutils.CreateUserType(githubEntityType)
	suite.Require().NoError(err, "Failed to create GitHub user type")
	suite.entityTypeID = schemaID

	userAttributes := map[string]interface{}{
		"username":   "githubuser",
		"password":   "Test@1234",
		"sub":        "12345",
		"email":      "testuser@github.com",
		"givenName":  "Test",
		"familyName": "User",
	}

	attributesJSON, err := json.Marshal(userAttributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:       githubEntityType.Name,
		OUID:       suite.ouID,
		Attributes: json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err, "Failed to create test user")
	suite.userID = userID

	idp := testutils.IDP{
		Name:        "Test GitHub IDP",
		Description: "GitHub Identity Provider for authentication testing",
		Type:        "GITHUB",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test-github-client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "test-github-secret",
				IsSecret: true,
			},
			{
				Name:     "authorization_endpoint",
				Value:    suite.mockGithubServer.GetURL() + "/login/oauth/authorize",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    suite.mockGithubServer.GetURL() + "/login/oauth/access_token",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    suite.mockGithubServer.GetURL() + "/user",
				IsSecret: false,
			},
			{
				Name:     "scopes",
				Value:    "user:email,read:user",
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
	suite.Require().NoError(err, "Failed to create GitHub IDP")
	suite.idpID = idpID
}

func (suite *GithubAuthTestSuite) TearDownSuite() {
	if suite.userID != "" {
		_ = testutils.DeleteUser(suite.userID)
	}

	if suite.entityTypeID != "" {
		_ = testutils.DeleteUserType(suite.entityTypeID)
	}

	if suite.idpID != "" {
		_ = testutils.DeleteIDP(suite.idpID)
	}

	if suite.mockGithubServer != nil {
		_ = suite.mockGithubServer.Stop()
	}

	if suite.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.ouID); err != nil {
			suite.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

func (suite *GithubAuthTestSuite) TestGithubAuthStartSuccess() {
	// Start authentication
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+githubAuthStart, bytes.NewReader(startRequestJSON))
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
	suite.Contains(redirectURL, suite.mockGithubServer.GetURL())
	suite.Contains(redirectURL, "client_id=test-github-client")
}

func (suite *GithubAuthTestSuite) TestGithubAuthStartInvalidIDPID() {
	startRequest := map[string]interface{}{
		"idpId": "invalid-idp-id",
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+githubAuthStart, bytes.NewReader(startRequestJSON))
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

func (suite *GithubAuthTestSuite) TestGithubAuthStartMissingIDPID() {
	startRequest := map[string]interface{}{}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+githubAuthStart, bytes.NewReader(startRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (suite *GithubAuthTestSuite) TestGithubAuthCompleteFlowSuccess() {
	// Step 1: Start authentication
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+githubAuthStart, bytes.NewReader(startRequestJSON))
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

	// Step 2: Simulate user authorization at GitHub (get authorization code)
	authCode := suite.simulateGithubAuthorization(redirectURL)
	suite.Require().NotEmpty(authCode)

	// Step 3: Finish authentication
	finishRequest := map[string]interface{}{
		"sessionToken": sessionToken,
		"code":         authCode,
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+githubAuthFinish, bytes.NewReader(finishRequestJSON))
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

func (suite *GithubAuthTestSuite) TestGithubAuthFinishInvalidSessionToken() {
	finishRequest := map[string]interface{}{
		"sessionToken": "invalid-session-token",
		"code":         "some-auth-code",
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+githubAuthFinish, bytes.NewReader(finishRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (suite *GithubAuthTestSuite) TestGithubAuthFinishMissingCode() {
	finishRequest := map[string]interface{}{
		"sessionToken": "some-session-token",
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+githubAuthFinish, bytes.NewReader(finishRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

// TestGithubAuthCompleteFlowWithSkipAssertionFalse tests complete GitHub auth flow with skip_assertion=false
func (suite *GithubAuthTestSuite) TestGithubAuthCompleteFlowWithSkipAssertionFalse() {
	// Step 1: Start authentication
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+githubAuthStart, bytes.NewReader(startRequestJSON))
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

	// Step 2: Simulate user authorization at GitHub
	authCode := suite.simulateGithubAuthorization(redirectURL)
	suite.Require().NotEmpty(authCode)

	// Step 3: Finish authentication with skip_assertion=false
	finishRequest := map[string]interface{}{
		"sessionToken":  sessionToken,
		"code":          authCode,
		"skipAssertion": false,
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+githubAuthFinish, bytes.NewReader(finishRequestJSON))
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

// TestGithubAuthCompleteFlowWithSkipAssertionTrue tests complete GitHub auth flow with skip_assertion=true
func (suite *GithubAuthTestSuite) TestGithubAuthCompleteFlowWithSkipAssertionTrue() {
	// Step 1: Start authentication
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+githubAuthStart, bytes.NewReader(startRequestJSON))
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

	// Step 2: Simulate user authorization at GitHub
	authCode := suite.simulateGithubAuthorization(redirectURL)
	suite.Require().NotEmpty(authCode)

	// Step 3: Finish authentication with skip_assertion=true
	finishRequest := map[string]interface{}{
		"sessionToken":  sessionToken,
		"code":          authCode,
		"skipAssertion": true,
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+githubAuthFinish, bytes.NewReader(finishRequestJSON))
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

// TestGithubAuthWithAssuranceLevelAAL1 tests that GitHub authentication generates AAL1 assurance level
func (suite *GithubAuthTestSuite) TestGithubAuthWithAssuranceLevelAAL1() {
	// Step 1: Start authentication
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+githubAuthStart, bytes.NewReader(startRequestJSON))
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
	authCode := suite.simulateGithubAuthorization(startResponse["redirectUrl"].(string))

	// Step 2: Finish authentication
	finishRequest := map[string]interface{}{
		"sessionToken": sessionToken,
		"code":         authCode,
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+githubAuthFinish, bytes.NewReader(finishRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	var authResponse testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	suite.Require().NoError(err)

	suite.NotEmpty(authResponse.Assertion, "Response should contain assertion token by default")

	// Verify assertion contains AAL1 for single-factor GitHub authentication
	aal := extractAssuranceLevelFromAssertion(authResponse.Assertion, "aal")
	suite.NotEmpty(aal, "Assertion should contain AAL information")
	suite.Equal("AAL1", aal, "Single-factor GitHub authentication should result in AAL1")

	// Verify IAL is present
	ial := extractAssuranceLevelFromAssertion(authResponse.Assertion, "ial")
	suite.NotEmpty(ial, "Assertion should contain IAL information")
	suite.Equal("IAL1", ial, "Self-asserted identity should result in IAL1")
}

// simulateGithubAuthorization simulates user authorization and returns authorization code
func (suite *GithubAuthTestSuite) simulateGithubAuthorization(redirectURL string) string {
	// Parse redirect URL to extract parameters
	parsedURL, err := url.Parse(redirectURL)
	suite.Require().NoError(err)

	query := parsedURL.Query()
	clientID := query.Get("client_id")
	state := query.Get("state")
	redirectURI := query.Get("redirect_uri")
	scope := query.Get("scope")

	// Build authorization request to mock server
	authURL := fmt.Sprintf("%s/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=%s&state=%s",
		suite.mockGithubServer.GetURL(),
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
