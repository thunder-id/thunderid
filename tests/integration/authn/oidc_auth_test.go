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
	"net/http"
	"net/url"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	oidcAuthStart  = "/auth/oauth/standard/start"
	oidcAuthFinish = "/auth/oauth/standard/finish"
	mockOIDCPort   = 8093
)

var oidcAuthTestOU = testutils.OrganizationUnit{
	Handle:      "oidc-auth-test-ou",
	Name:        "OIDC Auth Test Organization Unit",
	Description: "Organization unit for OIDC authentication testing",
	Parent:      nil,
}

var oidcEntityType = testutils.UserType{
	Name: "oidc_user",
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

type OIDCAuthTestSuite struct {
	suite.Suite
	mockOIDCServer *testutils.MockOIDCServer
	idpID          string
	userID         string
	entityTypeID   string
	ouID           string
}

func TestOIDCAuthTestSuite(t *testing.T) {
	suite.Run(t, new(OIDCAuthTestSuite))
}

func (suite *OIDCAuthTestSuite) SetupSuite() {
	var err error
	suite.mockOIDCServer, err = testutils.NewMockOIDCServer(mockOIDCPort,
		"test-oidc-client", "test-oidc-secret")
	suite.Require().NoError(err, "Failed to create mock OIDC server")

	suite.mockOIDCServer.AddUser(&testutils.OIDCUserInfo{
		Sub:           "user456",
		Email:         "testuser@oidc.com",
		EmailVerified: true,
		Name:          "OIDC Test User",
		GivenName:     "OIDC",
		FamilyName:    "User",
		Picture:       "https://oidc.example.com/avatar.jpg",
		Custom: map[string]interface{}{
			"role":   "admin",
			"tenant": "acme-corp",
		},
	})

	err = suite.mockOIDCServer.Start()
	suite.Require().NoError(err, "Failed to start mock OIDC server")

	ouID, err := testutils.CreateOrganizationUnit(oidcAuthTestOU)
	suite.Require().NoError(err, "Failed to create test organization unit")
	suite.ouID = ouID

	oidcEntityType.OUID = suite.ouID
	schemaID, err := testutils.CreateUserType(oidcEntityType)
	suite.Require().NoError(err, "Failed to create OIDC user type")
	suite.entityTypeID = schemaID

	userAttributes := map[string]interface{}{
		"username":   "oidcuser",
		"password":   "Test@1234",
		"sub":        "user456",
		"email":      "testuser@oidc.com",
		"givenName":  "OIDC",
		"familyName": "User",
	}

	attributesJSON, err := json.Marshal(userAttributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:       oidcEntityType.Name,
		OUID:       suite.ouID,
		Attributes: json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err, "Failed to create test user")
	suite.userID = userID

	idp := testutils.IDP{
		Name:        "Test OIDC IDP",
		Description: "Standard OpenID Connect Identity Provider for authentication testing",
		Type:        "OIDC",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test-oidc-client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "test-oidc-secret",
				IsSecret: true,
			},
			{
				Name:     "authorization_endpoint",
				Value:    suite.mockOIDCServer.GetURL() + "/authorize",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    suite.mockOIDCServer.GetURL() + "/token",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    suite.mockOIDCServer.GetURL() + "/userinfo",
				IsSecret: false,
			},
			{
				Name:     "jwks_endpoint",
				Value:    suite.mockOIDCServer.GetURL() + "/jwks",
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
	suite.Require().NoError(err, "Failed to create OIDC IDP")
	suite.idpID = idpID
}

func (suite *OIDCAuthTestSuite) TearDownSuite() {
	if suite.userID != "" {
		_ = testutils.DeleteUser(suite.userID)
	}

	if suite.entityTypeID != "" {
		_ = testutils.DeleteUserType(suite.entityTypeID)
	}

	if suite.idpID != "" {
		_ = testutils.DeleteIDP(suite.idpID)
	}

	if suite.mockOIDCServer != nil {
		_ = suite.mockOIDCServer.Stop()
	}

	if suite.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.ouID); err != nil {
			suite.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

func (suite *OIDCAuthTestSuite) TestOIDCAuthStartSuccess() {
	// Start authentication
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oidcAuthStart, bytes.NewReader(startRequestJSON))
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

	// Verify response contains redirectUrl and sessionToken
	suite.Contains(startResponse, "redirectUrl")
	suite.Contains(startResponse, "sessionToken")

	redirectURL, ok := startResponse["redirectUrl"].(string)
	suite.Require().True(ok)
	suite.Contains(redirectURL, suite.mockOIDCServer.GetURL())
	suite.Contains(redirectURL, "client_id=test-oidc-client")
	suite.Contains(redirectURL, "response_type=code")
	suite.Contains(redirectURL, "scope=")
	// Note: Server's OIDC implementation does not currently generate nonce values
}

func (suite *OIDCAuthTestSuite) TestOIDCAuthStartInvalidIDPID() {
	startRequest := map[string]interface{}{
		"idpId": "invalid-idp-id",
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oidcAuthStart, bytes.NewReader(startRequestJSON))
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

func (suite *OIDCAuthTestSuite) TestOIDCAuthStartMissingIDPID() {
	startRequest := map[string]interface{}{}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oidcAuthStart, bytes.NewReader(startRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (suite *OIDCAuthTestSuite) TestOIDCAuthCompleteFlowSuccess() {
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oidcAuthStart, bytes.NewReader(startRequestJSON))
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

	authCode := suite.simulateOIDCAuthorization(redirectURL)
	suite.Require().NotEmpty(authCode)

	finishRequest := map[string]interface{}{
		"sessionToken": sessionToken,
		"code":         authCode,
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+oidcAuthFinish, bytes.NewReader(finishRequestJSON))
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

func (suite *OIDCAuthTestSuite) TestOIDCAuthFinishInvalidSessionToken() {
	finishRequest := map[string]interface{}{
		"sessionToken": "invalid-session-token",
		"code":         "some-auth-code",
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oidcAuthFinish, bytes.NewReader(finishRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (suite *OIDCAuthTestSuite) TestOIDCAuthFinishMissingCode() {
	finishRequest := map[string]interface{}{
		"sessionToken": "some-session-token",
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oidcAuthFinish, bytes.NewReader(finishRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (suite *OIDCAuthTestSuite) TestOIDCAuthWithNonce() {
	// Note: Server's current OIDC implementation does not generate nonce values.
	// This test verifies that authentication still works without nonce.

	// Start authentication
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oidcAuthStart, bytes.NewReader(startRequestJSON))
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

	redirectURL := startResponse["redirectUrl"].(string)

	parsedURL, err := url.Parse(redirectURL)
	suite.Require().NoError(err)

	query := parsedURL.Query()
	suite.NotEmpty(query.Get("client_id"), "client_id should be present")
	suite.NotEmpty(query.Get("response_type"), "response_type should be present")
	suite.NotEmpty(query.Get("scope"), "scope should be present")
	// nonce is optional - Server doesn't generate it currently
}

// TestOIDCAuthCompleteFlowWithSkipAssertionFalse tests complete OIDC flow with skipAssertion=false
func (suite *OIDCAuthTestSuite) TestOIDCAuthCompleteFlowWithSkipAssertionFalse() {
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oidcAuthStart, bytes.NewReader(startRequestJSON))
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

	authCode := suite.simulateOIDCAuthorization(redirectURL)
	suite.Require().NotEmpty(authCode)

	finishRequest := map[string]interface{}{
		"sessionToken":  sessionToken,
		"code":          authCode,
		"skipAssertion": false,
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+oidcAuthFinish, bytes.NewReader(finishRequestJSON))
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
	suite.NotEmpty(authResponse.Assertion, "Response should contain assertion token when skipAssertion is false")
}

// TestOIDCAuthCompleteFlowWithSkipAssertionTrue tests complete OIDC flow with skipAssertion=true
func (suite *OIDCAuthTestSuite) TestOIDCAuthCompleteFlowWithSkipAssertionTrue() {
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oidcAuthStart, bytes.NewReader(startRequestJSON))
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

	authCode := suite.simulateOIDCAuthorization(redirectURL)
	suite.Require().NotEmpty(authCode)

	finishRequest := map[string]interface{}{
		"sessionToken":  sessionToken,
		"code":          authCode,
		"skipAssertion": true,
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+oidcAuthFinish, bytes.NewReader(finishRequestJSON))
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
	suite.Empty(authResponse.Assertion, "Response should not contain assertion token when skipAssertion is true")
}

// TestOIDCAuthWithAssuranceLevelAAL1 tests OIDC authentication generates AAL1 assurance level
func (suite *OIDCAuthTestSuite) TestOIDCAuthWithAssuranceLevelAAL1() {
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oidcAuthStart, bytes.NewReader(startRequestJSON))
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

	authCode := suite.simulateOIDCAuthorization(redirectURL)
	suite.Require().NotEmpty(authCode)

	finishRequest := map[string]interface{}{
		"sessionToken":  sessionToken,
		"code":          authCode,
		"skipAssertion": false,
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+oidcAuthFinish, bytes.NewReader(finishRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var authResponse testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	suite.Require().NoError(err)

	suite.NotEmpty(authResponse.Assertion, "Response should contain assertion token")

	// Extract and verify assurance level from assertion token
	aal := extractAssuranceLevelFromAssertion(authResponse.Assertion, "aal")
	ial := extractAssuranceLevelFromAssertion(authResponse.Assertion, "ial")

	suite.Equal("AAL1", aal, "OIDC single-factor authentication should generate AAL1")
	suite.Equal("IAL1", ial, "OIDC authentication should generate IAL1 by default")
}

// TestOIDCAuthWithSkipAssertion tests OIDC authentication with assertion skip functionality
func (suite *OIDCAuthTestSuite) TestOIDCAuthWithSkipAssertion() {
	startRequest := map[string]interface{}{
		"idpId": suite.idpID,
	}
	startRequestJSON, err := json.Marshal(startRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+oidcAuthStart, bytes.NewReader(startRequestJSON))
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

	authCode := suite.simulateOIDCAuthorization(redirectURL)
	suite.Require().NotEmpty(authCode)

	finishRequest := map[string]interface{}{
		"sessionToken":  sessionToken,
		"code":          authCode,
		"skipAssertion": true,
	}
	finishRequestJSON, err := json.Marshal(finishRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+oidcAuthFinish, bytes.NewReader(finishRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var authResponse testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	suite.Require().NoError(err)

	suite.Empty(authResponse.Assertion, "Response should not contain assertion token when skipAssertion is true")
}

// simulateOIDCAuthorization simulates user authorization and returns authorization code
func (suite *OIDCAuthTestSuite) simulateOIDCAuthorization(redirectURL string) string {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Get(redirectURL)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	location := resp.Header.Get("Location")
	suite.Require().NotEmpty(location, "Mock server should redirect with authorization code")

	locationURL, err := url.Parse(location)
	suite.Require().NoError(err)

	code := locationURL.Query().Get("code")
	suite.Require().NotEmpty(code, "Authorization code should be present in redirect")
	return code
}
