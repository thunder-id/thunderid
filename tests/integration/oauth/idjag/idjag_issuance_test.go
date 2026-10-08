// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package idjag holds end-to-end integration tests for the ID-JAG issuance
// leg (RFC 8693 token exchange issuing an ID-JAG JWT).
package idjag

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

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	issuanceClientID     = "idjag_issuance_client"
	issuanceClientSecret = "idjag_issuance_secret"
	issuanceAppName      = "IDJAGIssuanceTestApp"
	noConfigClientID     = "idjag_noconfig_client"
	noConfigClientSecret = "idjag_noconfig_secret"
	noConfigAppName      = "IDJAGNoConfigApp"
	publicClientID       = "idjag_public_client"
	publicAppName        = "IDJAGPublicApp"
	issuanceTestUser     = "idjag_issuance_user"
	issuanceTestPassword = "IDJAGTest123!"
	issuanceTestEmail    = "idjag_issuance@example.com"
	issuanceRedirectURI  = "https://localhost:3000"
	allowedAudience      = "https://target-rs.example.com"

	// appErrorInvalidOAuthConfiguration is the APP-1024 error code returned when an OAuth
	// configuration is rejected at application create time (see
	// backend/internal/application/error_constants.go: ErrorInvalidOAuthConfiguration).
	appErrorInvalidOAuthConfiguration = "APP-1024"
)

var issuanceAuthFlow = testutils.Flow{
	Name:     "IDJAG Issuance Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_idjag_issuance_test",
	Nodes: []map[string]any{
		{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
		{
			"id": "prompt_credentials", "type": "PROMPT",
			"prompts": []map[string]any{{
				"inputs": []map[string]any{
					{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
				"action": map[string]any{"ref": "action_001", "nextNode": "credentials_auth"},
			}},
		},
		{
			"id": "credentials_auth", "type": "TASK_EXECUTION",
			"executor": map[string]any{
				"name": "CredentialsAuthExecutor",
				"inputs": []map[string]any{
					{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
			},
			"onSuccess": "auth_assert",
		},
		{"id": "auth_assert", "type": "TASK_EXECUTION", "executor": map[string]any{"name": "AuthAssertExecutor"}, "onSuccess": "end"},
		{"id": "end", "type": "END"},
	},
}

// IDJAGIssuanceTestSuite exercises ThunderID's ID-JAG issuance leg: exchanging
// a self-issued ID token for an ID-JAG JWT via the token exchange grant.
type IDJAGIssuanceTestSuite struct {
	suite.Suite
	ouID             string
	userSchemaID     string
	authFlowID       string
	userID           string
	appID            string
	noConfigAppID    string
	resourceServerID string
	client           *http.Client
}

func TestIDJAGIssuanceTestSuite(t *testing.T) {
	suite.Run(t, new(IDJAGIssuanceTestSuite))
}

func (ts *IDJAGIssuanceTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ou := testutils.OrganizationUnit{
		Handle:      "idjag-issuance-test-ou",
		Name:        "IDJAG Issuance Test OU",
		Description: "Organization unit for ID-JAG issuance testing",
	}
	ouID, err := testutils.CreateOrganizationUnit(ou)
	ts.Require().NoError(err, "create test OU")
	ts.ouID = ouID

	userType := testutils.UserType{
		Handle:      "idjag-issuance-person",
		DisplayName: "Idjag Issuance Person",
		OUID:        ts.ouID,
		Schema: map[string]any{
			"username": map[string]any{"type": "string"},
			"password": map[string]any{"type": "string", "credential": true},
			"email":    map[string]any{"type": "string"},
		},
	}
	schemaID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err, "create test user schema")
	ts.userSchemaID = schemaID

	flowID, err := testutils.CreateFlow(issuanceAuthFlow)
	ts.Require().NoError(err, "create auth flow")
	ts.authFlowID = flowID

	user := testutils.User{
		OUID: ts.ouID,
		Type: "idjag-issuance-person",
		Attributes: json.RawMessage(fmt.Sprintf(`{"username":%q,"password":%q,"email":%q}`,
			issuanceTestUser, issuanceTestPassword, issuanceTestEmail)),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "create test user")
	ts.userID = userID

	ts.appID = ts.createApplication(map[string]any{
		"name": issuanceAppName, "description": "ID-JAG issuance test app",
		"ouId": ts.ouID, "type": "fullstack", "authFlowId": ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"idjag-issuance-person"},
		"inboundAuthConfig": []map[string]any{{
			"type": "oauth2",
			"config": map[string]any{
				"clientId": issuanceClientID, "clientSecret": issuanceClientSecret,
				"redirectUris":            []string{issuanceRedirectURI},
				"grantTypes":              []string{"authorization_code", "urn:ietf:params:oauth:grant-type:token-exchange"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"scopes":                  []string{"openid", "profile", "email", "read", "write"},
				"token": map[string]any{
					"idJag": map[string]any{
						"enabled":          true,
						"allowedAudiences": []string{allowedAudience},
					},
				},
			},
		}},
	})

	ts.noConfigAppID = ts.createApplication(map[string]any{
		"name": noConfigAppName, "description": "ID-JAG issuance test app without idJag config",
		"ouId": ts.ouID, "type": "fullstack", "authFlowId": ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"idjag-issuance-person"},
		"inboundAuthConfig": []map[string]any{{
			"type": "oauth2",
			"config": map[string]any{
				"clientId": noConfigClientID, "clientSecret": noConfigClientSecret,
				"redirectUris":            []string{issuanceRedirectURI},
				"grantTypes":              []string{"authorization_code", "urn:ietf:params:oauth:grant-type:token-exchange"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"scopes":                  []string{"openid", "profile", "email", "read", "write"},
			},
		}},
	})

	rs := testutils.ResourceServer{
		Name: "IDJAG Issuance Test RS", Identifier: allowedAudience, OUID: ts.ouID,
	}
	rsID, err := testutils.CreateResourceServerWithActions(rs, []testutils.Action{})
	ts.Require().NoError(err, "create test resource server")
	ts.resourceServerID = rsID
}

func (ts *IDJAGIssuanceTestSuite) TearDownSuite() {
	if ts.resourceServerID != "" {
		_ = testutils.DeleteResourceServer(ts.resourceServerID)
	}
	if ts.appID != "" {
		_ = testutils.DeleteApplication(ts.appID)
	}
	if ts.noConfigAppID != "" {
		_ = testutils.DeleteApplication(ts.noConfigAppID)
	}
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}
	if ts.authFlowID != "" {
		_ = testutils.DeleteFlow(ts.authFlowID)
	}
	if ts.userSchemaID != "" {
		_ = testutils.DeleteUserType(ts.userSchemaID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// createApplication POSTs an application definition and returns its ID.
func (ts *IDJAGIssuanceTestSuite) createApplication(app map[string]any) string {
	ts.T().Helper()
	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equalf(http.StatusCreated, resp.StatusCode, "create app failed: %s", string(body))
	var respData map[string]any
	ts.Require().NoError(json.Unmarshal(body, &respData))
	id, _ := respData["id"].(string)
	ts.Require().NotEmpty(id)
	return id
}

// createApplicationExpectError POSTs an application definition that is expected to be
// rejected, returning the response status code and the decoded error body. If the
// application is unexpectedly created, it is deleted immediately so a regression
// does not leave a stray application behind for later test runs to trip over.
func (ts *IDJAGIssuanceTestSuite) createApplicationExpectError(app map[string]any) (int, testutils.ErrorResponse) {
	ts.T().Helper()
	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	if resp.StatusCode == http.StatusCreated {
		var created map[string]any
		ts.Require().NoError(json.Unmarshal(body, &created))
		if id, ok := created["id"].(string); ok && id != "" {
			_ = testutils.DeleteApplication(id)
		}
		return resp.StatusCode, testutils.ErrorResponse{}
	}
	var errResp testutils.ErrorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	return resp.StatusCode, errResp
}

// obtainSelfIssuedIDToken runs the full authorization code flow for the
// issuance test application and returns the resulting self-issued ID token.
func (ts *IDJAGIssuanceTestSuite) obtainSelfIssuedIDToken() string {
	ts.T().Helper()

	verifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err)
	challenge := testutils.GenerateCodeChallenge(verifier)

	params := url.Values{}
	params.Set("client_id", issuanceClientID)
	params.Set("redirect_uri", issuanceRedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid email")
	params.Set("state", "idjag-issuance-state")
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")

	noRedirect := testutils.GetNoRedirectHTTPClient()
	req, err := http.NewRequest("GET", testutils.TestServerURL+"/oauth2/authorize?"+params.Encode(), nil)
	ts.Require().NoError(err)
	resp, err := noRedirect.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)

	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err)

	initial, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)
	step, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": issuanceTestUser, "password": issuanceTestPassword,
	}, "action_001", initial.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", step.FlowStatus)

	authzResp, err := testutils.CompleteAuthorization(authID, step.Assertion)
	ts.Require().NoError(err)
	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)

	formData := url.Values{}
	formData.Set("grant_type", "authorization_code")
	formData.Set("code", code)
	formData.Set("redirect_uri", issuanceRedirectURI)
	formData.Set("code_verifier", verifier)

	tokenReq, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token",
		strings.NewReader(formData.Encode()))
	ts.Require().NoError(err)
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenReq.Header.Set("Authorization", "Basic "+basicAuth(issuanceClientID, issuanceClientSecret))

	tokenResp, err := testutils.GetRawHTTPClient().Do(tokenReq)
	ts.Require().NoError(err)
	defer tokenResp.Body.Close()
	ts.Require().Equal(http.StatusOK, tokenResp.StatusCode)

	var tokenData map[string]interface{}
	err = json.NewDecoder(tokenResp.Body).Decode(&tokenData)
	ts.Require().NoError(err)

	idToken, ok := tokenData["id_token"].(string)
	ts.Require().True(ok, "Token response should contain id_token")
	ts.Require().NotEmpty(idToken)
	return idToken
}

func basicAuth(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

// IDJAGTokenResponse models the /oauth2/token response for an ID-JAG
// token exchange request, including both success and error fields.
type IDJAGTokenResponse struct {
	AccessToken      string `json:"access_token,omitempty"`
	TokenType        string `json:"token_type,omitempty"`
	ExpiresIn        int64  `json:"expires_in,omitempty"`
	IssuedTokenType  string `json:"issued_token_type,omitempty"`
	Scope            string `json:"scope,omitempty"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

func (ts *IDJAGIssuanceTestSuite) exchangeForIDJAG(formBody, authHeader string) (IDJAGTokenResponse, int, error) {
	ts.T().Helper()
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token",
		strings.NewReader(formBody))
	if err != nil {
		return IDJAGTokenResponse{}, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := testutils.GetRawHTTPClient().Do(req)
	if err != nil {
		return IDJAGTokenResponse{}, 0, err
	}
	defer resp.Body.Close()

	var result IDJAGTokenResponse
	if decErr := json.NewDecoder(resp.Body).Decode(&result); decErr != nil {
		return IDJAGTokenResponse{}, resp.StatusCode, decErr
	}
	return result, resp.StatusCode, nil
}

func (ts *IDJAGIssuanceTestSuite) TestIDJAGIssuance_Success() {
	idToken := ts.obtainSelfIssuedIDToken()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", idToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:id_token")
	formData.Set("requested_token_type", "urn:ietf:params:oauth:token-type:id-jag")
	formData.Set("audience", allowedAudience)

	resp, statusCode, err := ts.exchangeForIDJAG(formData.Encode(),
		"Basic "+basicAuth(issuanceClientID, issuanceClientSecret))
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, statusCode)
	ts.NotEmpty(resp.AccessToken)
	ts.Equal("N_A", resp.TokenType)
	ts.Equal("urn:ietf:params:oauth:token-type:id-jag", resp.IssuedTokenType)

	header, err := testutils.DecodeJWTHeaderMap(resp.AccessToken)
	ts.Require().NoError(err)
	ts.Equal("oauth-id-jag+jwt", header["typ"])

	claims, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err)
	ts.NotEmpty(claims.Sub)
	ts.Equal(allowedAudience, claims.Aud)
	ts.Equal("https://localhost:8095", claims.Iss)
	ts.Equal(issuanceClientID, claims.Additional["client_id"])
}

func (ts *IDJAGIssuanceTestSuite) TestIDJAGIssuance_WithScope() {
	idToken := ts.obtainSelfIssuedIDToken()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", idToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:id_token")
	formData.Set("requested_token_type", "urn:ietf:params:oauth:token-type:id-jag")
	formData.Set("audience", allowedAudience)
	formData.Set("scope", "read write")

	resp, statusCode, err := ts.exchangeForIDJAG(formData.Encode(),
		"Basic "+basicAuth(issuanceClientID, issuanceClientSecret))
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, statusCode)
	ts.NotEmpty(resp.AccessToken)

	claims, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err)
	scope, ok := claims.Additional["scope"].(string)
	ts.True(ok, "ID-JAG JWT should contain scope claim")
	ts.Contains(scope, "read")
	ts.Contains(scope, "write")
}

func (ts *IDJAGIssuanceTestSuite) TestIDJAGIssuance_WithResource() {
	idToken := ts.obtainSelfIssuedIDToken()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", idToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:id_token")
	formData.Set("requested_token_type", "urn:ietf:params:oauth:token-type:id-jag")
	formData.Set("audience", allowedAudience)
	formData.Set("resource", allowedAudience)

	resp, statusCode, err := ts.exchangeForIDJAG(formData.Encode(),
		"Basic "+basicAuth(issuanceClientID, issuanceClientSecret))
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, statusCode)

	claims, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err)
	resource, ok := claims.Additional["resource"]
	ts.True(ok, "ID-JAG JWT should contain resource claim")
	ts.Equal(allowedAudience, resource)
}

func (ts *IDJAGIssuanceTestSuite) TestIDJAGIssuance_AudienceNotAllowed() {
	idToken := ts.obtainSelfIssuedIDToken()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", idToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:id_token")
	formData.Set("requested_token_type", "urn:ietf:params:oauth:token-type:id-jag")
	formData.Set("audience", "https://unauthorized.example.com")

	resp, statusCode, err := ts.exchangeForIDJAG(formData.Encode(),
		"Basic "+basicAuth(issuanceClientID, issuanceClientSecret))
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_target", resp.Error)
}

func (ts *IDJAGIssuanceTestSuite) TestIDJAGIssuance_MissingAudience() {
	idToken := ts.obtainSelfIssuedIDToken()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", idToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:id_token")
	formData.Set("requested_token_type", "urn:ietf:params:oauth:token-type:id-jag")

	resp, statusCode, err := ts.exchangeForIDJAG(formData.Encode(),
		"Basic "+basicAuth(issuanceClientID, issuanceClientSecret))
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_target", resp.Error)
}

func (ts *IDJAGIssuanceTestSuite) TestIDJAGIssuance_MultipleAudiences() {
	idToken := ts.obtainSelfIssuedIDToken()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", idToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:id_token")
	formData.Set("requested_token_type", "urn:ietf:params:oauth:token-type:id-jag")
	formData.Add("audience", allowedAudience)
	formData.Add("audience", "https://other-target.example.com")

	resp, statusCode, err := ts.exchangeForIDJAG(formData.Encode(),
		"Basic "+basicAuth(issuanceClientID, issuanceClientSecret))
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_target", resp.Error)
}

func (ts *IDJAGIssuanceTestSuite) TestIDJAGIssuance_NoIDJAGConfig() {
	idToken := ts.obtainSelfIssuedIDToken()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", idToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:id_token")
	formData.Set("requested_token_type", "urn:ietf:params:oauth:token-type:id-jag")
	formData.Set("audience", allowedAudience)

	resp, statusCode, err := ts.exchangeForIDJAG(formData.Encode(),
		"Basic "+basicAuth(noConfigClientID, noConfigClientSecret))
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.NotEmpty(resp.Error)
}

// TestIDJAGIssuance_PublicClientConfigRejectedAtCreate verifies that an application
// combining a public client (tokenEndpointAuthMethod: "none") with token.idJag.enabled
// is rejected at creation time, since ID-JAG issuance requires a confidential client.
func (ts *IDJAGIssuanceTestSuite) TestIDJAGIssuance_PublicClientConfigRejectedAtCreate() {
	statusCode, errResp := ts.createApplicationExpectError(map[string]any{
		"name": publicAppName, "description": "ID-JAG issuance test public app",
		"ouId": ts.ouID, "type": "fullstack", "authFlowId": ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"idjag-issuance-person"},
		"inboundAuthConfig": []map[string]any{{
			"type": "oauth2",
			"config": map[string]any{
				"clientId":                publicClientID,
				"redirectUris":            []string{issuanceRedirectURI},
				"grantTypes":              []string{"authorization_code", "urn:ietf:params:oauth:grant-type:token-exchange"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "none",
				"publicClient":            true,
				"pkceRequired":            true,
				"scopes":                  []string{"openid", "profile", "email", "read", "write"},
				"token": map[string]any{
					"idJag": map[string]any{
						"enabled":          true,
						"allowedAudiences": []string{allowedAudience},
					},
				},
			},
		}},
	})

	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal(appErrorInvalidOAuthConfiguration, errResp.Code)
	ts.Contains(errResp.Description.DefaultValue, "ID-JAG")
}

func (ts *IDJAGIssuanceTestSuite) TestIDJAGIssuance_WrongSubjectTokenType() {
	idToken := ts.obtainSelfIssuedIDToken()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", idToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
	formData.Set("requested_token_type", "urn:ietf:params:oauth:token-type:id-jag")
	formData.Set("audience", allowedAudience)

	resp, statusCode, err := ts.exchangeForIDJAG(formData.Encode(),
		"Basic "+basicAuth(issuanceClientID, issuanceClientSecret))
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_request", resp.Error)
}

func (ts *IDJAGIssuanceTestSuite) TestIDJAGIssuance_InvalidSubjectToken() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", "invalid.jwt.token")
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:id_token")
	formData.Set("requested_token_type", "urn:ietf:params:oauth:token-type:id-jag")
	formData.Set("audience", allowedAudience)

	resp, statusCode, err := ts.exchangeForIDJAG(formData.Encode(),
		"Basic "+basicAuth(issuanceClientID, issuanceClientSecret))
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.NotEmpty(resp.Error)
}
