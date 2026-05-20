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

package token

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	tokenExchangeClientID     = "token_exchange_test_client"
	tokenExchangeClientSecret = "token_exchange_test_secret"
	tokenExchangeAppName      = "TokenExchangeTestApp"
	tokenExchangeTestUser     = "te_test_user"
	tokenExchangeTestPassword = "TePassword123!"
	tokenExchangeTestEmail    = "te_test@example.com"
)

type TokenExchangeTestSuite struct {
	suite.Suite
	applicationID    string
	userID           string
	oUID             string
	entityTypeID     string
	resourceServerID string
	client           *http.Client
	assertionToken   string
}

var (
	testUserType = testutils.UserType{
		Name: "token-test-person",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"email": map[string]interface{}{
				"type": "string",
			},
		},
	}
)

func TestTokenExchangeTestSuite(t *testing.T) {
	suite.Run(t, new(TokenExchangeTestSuite))
}

func (ts *TokenExchangeTestSuite) SetupSuite() {
	// Create HTTP client that skips TLS verification
	ts.client = testutils.GetHTTPClient()

	// Create test organization unit for user creation
	ts.oUID = ts.createTestOrganizationUnit()

	// Create user type for person type
	testUserType.OUID = ts.oUID
	schemaID, err := testutils.CreateUserType(testUserType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = schemaID

	// Create test user
	ts.userID = ts.createTestUser()

	// Create OAuth application with token exchange grant type
	ts.applicationID = ts.createTestApplication()

	// Authenticate user to get assertion token for tests
	ts.assertionToken = ts.getUserAssertion()

	// Create resource server for resource parameter tests
	rs := testutils.ResourceServer{
		Name:       "Token Exchange Test RS",
		Handle:     "te-resource-server",
		Identifier: "https://resource.example.com",
		OUID:       ts.oUID,
	}
	rsID, err := testutils.CreateResourceServerWithActions(rs, []testutils.Action{})
	ts.Require().NoError(err, "Failed to create test resource server")
	ts.resourceServerID = rsID
	ts.T().Logf("Created test resource server with ID: %s", rsID)
}

func (ts *TokenExchangeTestSuite) TearDownSuite() {
	// Clean up resource server
	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Logf("Failed to delete resource server during teardown: %v", err)
		}
	}

	// Clean up application
	if ts.applicationID != "" {
		ts.deleteApplication(ts.applicationID)
	}

	// Clean up user
	if ts.userID != "" {
		testutils.DeleteUser(ts.userID)
	}

	// Clean up organization unit
	if ts.oUID != "" {
		ts.deleteOrganizationUnit(ts.oUID)
	}

	// Clean up user type
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}
}

func (ts *TokenExchangeTestSuite) createTestOrganizationUnit() string {
	ouData := map[string]interface{}{
		"handle":      "token-exchange-test-ou",
		"name":        "Token Exchange Test OU",
		"description": "Organization unit for token exchange testing",
		"parent":      nil,
	}

	ouJSON, err := json.Marshal(ouData)
	ts.Require().NoError(err, "Failed to marshal OU data")

	ouReq, err := http.NewRequest("POST", testutils.TestServerURL+"/organization-units", bytes.NewReader(ouJSON))
	ts.Require().NoError(err, "Failed to create OU request")
	ouReq.Header.Set("Content-Type", "application/json")

	ouResp, err := ts.client.Do(ouReq)
	ts.Require().NoError(err, "Failed to send OU request")
	defer ouResp.Body.Close()

	ts.Require().Equal(http.StatusCreated, ouResp.StatusCode, "Failed to create OU")

	var ouRespData map[string]interface{}
	err = json.NewDecoder(ouResp.Body).Decode(&ouRespData)
	ts.Require().NoError(err, "Failed to parse OU response")

	ouID := ouRespData["id"].(string)
	ts.T().Logf("Created test organization unit with ID: %s", ouID)

	return ouID
}

func (ts *TokenExchangeTestSuite) deleteOrganizationUnit(ouID string) {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/organization-units/%s", testutils.TestServerURL, ouID), nil)
	if err != nil {
		ts.T().Errorf("Failed to create delete OU request: %v", err)
		return
	}

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Errorf("Failed to delete OU: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ts.T().Errorf("Failed to delete OU. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
	} else {
		ts.T().Logf("Successfully deleted test organization unit with ID: %s", ouID)
	}
}

func (ts *TokenExchangeTestSuite) createTestUser() string {
	attributes := map[string]interface{}{
		"username": tokenExchangeTestUser,
		"password": tokenExchangeTestPassword,
		"email":    tokenExchangeTestEmail,
	}

	attributesJSON, err := json.Marshal(attributes)
	ts.Require().NoError(err, "Failed to marshal user attributes")

	user := testutils.User{
		Type:       "token-test-person",
		OUID:       ts.oUID,
		Attributes: json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "Failed to create test user")
	ts.T().Logf("Created test user with ID: %s", userID)

	return userID
}

func (ts *TokenExchangeTestSuite) createTestApplication() string {
	app := map[string]interface{}{
		"name":                      tokenExchangeAppName,
		"description":               "Application for token exchange integration tests",
		"ouId":                      ts.oUID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"token-test-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":     tokenExchangeClientID,
					"clientSecret": tokenExchangeClientSecret,
					"redirectUris": []string{"https://localhost:3000"},
					"grantTypes": []string{
						"urn:ietf:params:oauth:grant-type:token-exchange",
						"authorization_code",
					},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"scopes":                  []string{"openid", "profile", "email", "read", "write"},
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err, "Failed to marshal application data")

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err, "Failed to create request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to create application")
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusCreated, resp.StatusCode, "Failed to create application")

	var respData map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respData)
	ts.Require().NoError(err, "Failed to parse response")

	appID := respData["id"].(string)
	ts.T().Logf("Created test application with ID: %s", appID)
	return appID
}

func (ts *TokenExchangeTestSuite) deleteApplication(appID string) {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, appID), nil)
	if err != nil {
		ts.T().Errorf("Failed to create delete request: %v", err)
		return
	}

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Errorf("Failed to delete application: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ts.T().Errorf("Failed to delete application. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
	} else {
		ts.T().Logf("Successfully deleted test application with ID: %s", appID)
	}
}

func (ts *TokenExchangeTestSuite) getUserAssertion() string {
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": tokenExchangeTestUser,
		},
		"credentials": map[string]interface{}{
			"password": tokenExchangeTestPassword,
		},
	}

	requestJSON, err := json.Marshal(authRequest)
	ts.Require().NoError(err, "Failed to marshal auth request")

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/auth/credentials/authenticate", bytes.NewReader(requestJSON))
	ts.Require().NoError(err, "Failed to create auth request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to authenticate user")
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode, "Authentication failed")

	var authResponse testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	ts.Require().NoError(err, "Failed to parse auth response")
	ts.Require().NotEmpty(authResponse.Assertion, "Assertion token should not be empty")

	return authResponse.Assertion
}

// assertAudienceContains verifies that the JWT audience claim (string or array) contains the expected value.
func (ts *TokenExchangeTestSuite) assertAudienceContains(claims *testutils.JWTClaims, expected string) {
	ts.T().Helper()

	rawAud, ok := claims.Additional["aud"]
	ts.Require().True(ok, "JWT should contain an aud claim")

	switch aud := rawAud.(type) {
	case string:
		ts.Equal(expected, aud, "Audience should match expected value")
	case []interface{}:
		found := false
		for _, v := range aud {
			if s, ok := v.(string); ok && s == expected {
				found = true
				break
			}
		}
		ts.True(found, "Audience array should contain %q, got %v", expected, aud)
	default:
		ts.Failf("unexpected aud type", "expected string or []interface{}, got %T", rawAud)
	}
}

func (ts *TokenExchangeTestSuite) exchangeToken(requestBody string, authHeader string) (*TokenExchangeResponse, int, error) {
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token", strings.NewReader(requestBody))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := ts.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var tokenResp TokenExchangeResponse
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		err = json.Unmarshal(bodyBytes, &tokenResp)
		if err != nil {
			return nil, resp.StatusCode, err
		}
		return &tokenResp, resp.StatusCode, nil
	}

	// Parse error response
	var errorResp map[string]interface{}
	_ = json.Unmarshal(bodyBytes, &errorResp)
	tokenResp.Error = fmt.Sprintf("%v", errorResp["error"])
	if desc, ok := errorResp["error_description"]; ok {
		tokenResp.ErrorDescription = fmt.Sprintf("%v", desc)
	}
	return &tokenResp, resp.StatusCode, nil
}

type TokenExchangeResponse struct {
	AccessToken      string `json:"access_token,omitempty"`
	TokenType        string `json:"token_type,omitempty"`
	ExpiresIn        int64  `json:"expires_in,omitempty"`
	IssuedTokenType  string `json:"issued_token_type,omitempty"`
	Scope            string `json:"scope,omitempty"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// TestTokenExchange_BasicSuccess tests basic successful token exchange
func (ts *TokenExchangeTestSuite) TestTokenExchange_BasicSuccess() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", ts.assertionToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")

	authHeader := "Basic " + basicAuth(tokenExchangeClientID, tokenExchangeClientSecret)

	resp, statusCode, err := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, statusCode)
	ts.NotEmpty(resp.AccessToken, "Access token should be present")
	ts.Equal("Bearer", resp.TokenType, "Token type should be Bearer")
	ts.NotZero(resp.ExpiresIn, "Expires in should be set")
	ts.Equal("urn:ietf:params:oauth:token-type:access_token", resp.IssuedTokenType)

	// Verify the access token is a valid JWT
	claims, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err, "Access token should be a valid JWT")
	ts.Equal(ts.userID, claims.Sub, "Subject should match user ID")
}

// TestTokenExchange_WithAudience tests token exchange with audience parameter
func (ts *TokenExchangeTestSuite) TestTokenExchange_WithAudience() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", ts.assertionToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
	formData.Set("audience", "https://api.example.com")

	authHeader := "Basic " + basicAuth(tokenExchangeClientID, tokenExchangeClientSecret)

	resp, statusCode, err := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, statusCode)
	ts.NotEmpty(resp.AccessToken)

	// Verify audience in JWT contains the requested audience
	claims, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err)
	ts.assertAudienceContains(claims, "https://api.example.com")
}

// TestTokenExchange_WithResource tests token exchange with resource parameter
func (ts *TokenExchangeTestSuite) TestTokenExchange_WithResource() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", ts.assertionToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
	formData.Set("resource", "https://resource.example.com")

	authHeader := "Basic " + basicAuth(tokenExchangeClientID, tokenExchangeClientSecret)

	resp, statusCode, err := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, statusCode)
	ts.NotEmpty(resp.AccessToken)

	// Verify resource is used as audience
	claims, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err)
	ts.assertAudienceContains(claims, "https://resource.example.com")
}

// TestTokenExchange_WithRequestedTokenType tests token exchange with requested_token_type
func (ts *TokenExchangeTestSuite) TestTokenExchange_WithRequestedTokenType() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", ts.assertionToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
	formData.Set("requested_token_type", "urn:ietf:params:oauth:token-type:access_token")

	authHeader := "Basic " + basicAuth(tokenExchangeClientID, tokenExchangeClientSecret)

	resp, statusCode, err := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, statusCode)
	ts.NotEmpty(resp.AccessToken)
	ts.Equal("urn:ietf:params:oauth:token-type:access_token", resp.IssuedTokenType)
}

// TestTokenExchange_MissingSubjectToken tests error when subject_token is missing
func (ts *TokenExchangeTestSuite) TestTokenExchange_MissingSubjectToken() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")

	authHeader := "Basic " + basicAuth(tokenExchangeClientID, tokenExchangeClientSecret)

	resp, statusCode, err := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_request", resp.Error)
	ts.Contains(resp.ErrorDescription, "subject_token")
}

// TestTokenExchange_MissingSubjectTokenType tests error when subject_token_type is missing
func (ts *TokenExchangeTestSuite) TestTokenExchange_MissingSubjectTokenType() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", ts.assertionToken)

	authHeader := "Basic " + basicAuth(tokenExchangeClientID, tokenExchangeClientSecret)

	resp, statusCode, err := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_request", resp.Error)
	ts.Contains(resp.ErrorDescription, "subject_token_type")
}

// TestTokenExchange_InvalidSubjectToken tests error with invalid subject token
func (ts *TokenExchangeTestSuite) TestTokenExchange_InvalidSubjectToken() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", "invalid.jwt.token")
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")

	authHeader := "Basic " + basicAuth(tokenExchangeClientID, tokenExchangeClientSecret)

	resp, statusCode, err := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_request", resp.Error)
	ts.Contains(resp.ErrorDescription, "Invalid subject_token")
}

// TestTokenExchange_UnsupportedSubjectTokenType tests error with unsupported token type
func (ts *TokenExchangeTestSuite) TestTokenExchange_UnsupportedSubjectTokenType() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", ts.assertionToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:saml2")

	authHeader := "Basic " + basicAuth(tokenExchangeClientID, tokenExchangeClientSecret)

	resp, statusCode, err := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_request", resp.Error)
	ts.Contains(resp.ErrorDescription, "Unsupported subject_token_type")
}

// TestTokenExchange_InvalidClientCredentials tests error with invalid client credentials
func (ts *TokenExchangeTestSuite) TestTokenExchange_InvalidClientCredentials() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", ts.assertionToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")

	authHeader := "Basic " + basicAuth("invalid_client", "invalid_secret")

	resp, statusCode, err := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, statusCode)
	ts.Equal("invalid_client", resp.Error)
}

// TestTokenExchange_ApplicationNotRegisteredForGrantType tests error when app doesn't have grant type
func (ts *TokenExchangeTestSuite) TestTokenExchange_ApplicationNotRegisteredForGrantType() {
	// Create an application without token exchange grant type
	app := map[string]interface{}{
		"name":                      tokenExchangeAppName + "_no_te",
		"description":               "Application without token exchange",
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"token-test-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                tokenExchangeClientID + "_no_te",
					"clientSecret":            tokenExchangeClientSecret,
					"redirectUris":            []string{"https://localhost:3000"},
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}

	jsonData, _ := json.Marshal(app)
	req, _ := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		var respData map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&respData)
		appID := respData["id"].(string)

		defer func() {
			req, _ := http.NewRequest("DELETE", testutils.TestServerURL+"/applications/"+appID, nil)
			ts.client.Do(req)
		}()

		// Try token exchange with app that doesn't support it
		formData := url.Values{}
		formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
		formData.Set("subject_token", ts.assertionToken)
		formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")

		authHeader := "Basic " + basicAuth(tokenExchangeClientID+"_no_te", tokenExchangeClientSecret)

		tokenResp, statusCode, err := ts.exchangeToken(formData.Encode(), authHeader)
		ts.Require().NoError(err)
		ts.Equal(http.StatusBadRequest, statusCode)
		ts.Equal("unauthorized_client", tokenResp.Error)
		ts.Contains(tokenResp.ErrorDescription, "not authorized")
	}
}

// TestTokenExchange_PreservesUserAttributes tests that user attributes are preserved in exchanged token
func (ts *TokenExchangeTestSuite) TestTokenExchange_PreservesUserAttributes() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", ts.assertionToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")

	authHeader := "Basic " + basicAuth(tokenExchangeClientID, tokenExchangeClientSecret)

	resp, statusCode, err := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, statusCode)

	// Verify user attributes are preserved
	claims, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err)
	ts.Equal(ts.userID, claims.Sub, "Subject should match user ID")
	// Check that user type and other attributes from assertion are present
	if userType, ok := claims.Additional["userType"].(string); ok {
		ts.NotEmpty(userType, "User type should be preserved")
	}
}

// TestTokenExchange_DefaultIssuedTokenType tests that default issued_token_type is used when not specified
func (ts *TokenExchangeTestSuite) TestTokenExchange_DefaultIssuedTokenType() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", ts.assertionToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
	// Note: requested_token_type is not set

	authHeader := "Basic " + basicAuth(tokenExchangeClientID, tokenExchangeClientSecret)

	resp, statusCode, err := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, statusCode)
	// Should default to access_token type
	ts.Equal("urn:ietf:params:oauth:token-type:access_token", resp.IssuedTokenType)
}

// createTestJWTWithoutIssuer creates a JWT token without an 'iss' claim for testing
func (ts *TokenExchangeTestSuite) createTestJWTWithoutIssuer() string {
	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
	}
	claims := map[string]interface{}{
		"sub": ts.userID,
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"nbf": time.Now().Unix(),
		// Note: intentionally not including "iss" claim
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// Create an unsigned JWT (signature won't be verified at issuer check stage anyway)
	// For testing purposes, we just need valid format
	signature := "dummy-signature"
	return fmt.Sprintf("%s.%s.%s", headerB64, claimsB64, base64.RawURLEncoding.EncodeToString([]byte(signature)))
}

// createTestJWTWithUnsupportedIssuer creates a JWT token with an unsupported issuer for testing
func (ts *TokenExchangeTestSuite) createTestJWTWithUnsupportedIssuer() string {
	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
	}
	claims := map[string]interface{}{
		"sub": ts.userID,
		"iss": "https://external-issuer.com", // Unsupported issuer
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
		"nbf": time.Now().Unix(),
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// Create an unsigned JWT (signature won't be verified at issuer check stage anyway)
	signature := "dummy-signature"
	return fmt.Sprintf("%s.%s.%s", headerB64, claimsB64, base64.RawURLEncoding.EncodeToString([]byte(signature)))
}

// TestTokenExchange_SubjectTokenMissingIssClaim tests error when subject token is missing 'iss' claim
func (ts *TokenExchangeTestSuite) TestTokenExchange_SubjectTokenMissingIssClaim() {
	tokenWithoutIss := ts.createTestJWTWithoutIssuer()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", tokenWithoutIss)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")

	authHeader := "Basic " + basicAuth(tokenExchangeClientID, tokenExchangeClientSecret)

	resp, statusCode, err := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_request", resp.Error)
	ts.Contains(resp.ErrorDescription, "Invalid subject_token")
}

// TestTokenExchange_SubjectTokenUnsupportedIssuer tests error when subject token has unsupported issuer
func (ts *TokenExchangeTestSuite) TestTokenExchange_SubjectTokenUnsupportedIssuer() {
	tokenWithUnsupportedIssuer := ts.createTestJWTWithUnsupportedIssuer()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", tokenWithUnsupportedIssuer)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")

	authHeader := "Basic " + basicAuth(tokenExchangeClientID, tokenExchangeClientSecret)

	resp, statusCode, err := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_request", resp.Error)
	ts.Contains(resp.ErrorDescription, "Invalid subject_token")
}
