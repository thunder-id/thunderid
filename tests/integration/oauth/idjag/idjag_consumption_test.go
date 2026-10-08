// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// ID-JAG consumer leg integration tests: ThunderID accepting external ID-JAG
// assertions via the JWT bearer grant, plus cross-cutting round-trip tests
// covering issuance immediately followed by consumption.

package idjag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	consumptionClientID     = "idjag_consumption_client"
	consumptionClientSecret = "idjag_consumption_secret"
	consumptionAppName      = "IDJAGConsumptionTestApp"
	publicConsClientID      = "idjag_public_cons_client"
	publicConsAppName       = "IDJAGPublicConsApp"
	roundtripClientID       = "idjag_roundtrip_client"
	roundtripClientSecret   = "idjag_roundtrip_secret"
	roundtripAppName        = "IDJAGRoundtripApp"
	consumptionTestUser     = "idjag_consumption_user"
	consumptionTestPassword = "IDJAGConsTest123!"
	consumptionTestEmail    = "idjag_cons@example.com"
	consumptionRedirectURI  = "https://localhost:3001"
	idJagMockOIDCPort       = 8096
	mockExternalSubject     = "external-idjag-subject"
	consumptionRSIdentifier = "https://idjag-consumption-rs.example.com"
)

var consumptionAuthFlow = testutils.Flow{
	Name:     "IDJAG Consumption Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_idjag_consumption_test",
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

// IDJAGConsumptionTestSuite exercises ThunderID's ID-JAG consumer leg: accepting
// an external ID-JAG assertion via the JWT bearer grant, plus round-trip tests
// where ThunderID both issues and consumes the ID-JAG.
type IDJAGConsumptionTestSuite struct {
	suite.Suite
	ouID             string
	userSchemaID     string
	authFlowID       string
	userID           string
	consumptionAppID string
	roundtripAppID   string
	resourceServerID string
	idpID            string
	selfRefIDPID     string
	mockOIDCServer   *testutils.MockOIDCServer
	jwksProxy        *jwksHTTPProxy
	client           *http.Client
}

func TestIDJAGConsumptionTestSuite(t *testing.T) {
	suite.Run(t, new(IDJAGConsumptionTestSuite))
}

func (ts *IDJAGConsumptionTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ou := testutils.OrganizationUnit{
		Handle:      "idjag-consumption-test-ou",
		Name:        "IDJAG Consumption Test OU",
		Description: "OU for ID-JAG consumption testing",
	}
	ouID, err := testutils.CreateOrganizationUnit(ou)
	ts.Require().NoError(err, "create test OU")
	ts.ouID = ouID

	userType := testutils.UserType{
		Handle:      "idjag-consumption-person",
		DisplayName: "Idjag Consumption Person",
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

	flowID, err := testutils.CreateFlow(consumptionAuthFlow)
	ts.Require().NoError(err, "create auth flow")
	ts.authFlowID = flowID

	user := testutils.User{
		OUID: ts.ouID,
		Type: "idjag-consumption-person",
		Attributes: json.RawMessage(fmt.Sprintf(`{"username":%q,"password":%q,"email":%q}`,
			consumptionTestUser, consumptionTestPassword, consumptionTestEmail)),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "create test user")
	ts.userID = userID

	mockOIDCServer, err := testutils.NewMockOIDCServer(idJagMockOIDCPort, "mock-idjag-client", "mock-idjag-secret")
	ts.Require().NoError(err, "create mock OIDC server")
	err = mockOIDCServer.Start()
	ts.Require().NoError(err, "start mock OIDC server")
	ts.mockOIDCServer = mockOIDCServer

	idpID, err := testutils.CreateIDP(testutils.IDP{
		Name:        fmt.Sprintf("IDJAG Test IdP %d", time.Now().UnixNano()),
		Description: "Mock OIDC IdP for ID-JAG consumption testing",
		Type:        "OIDC",
		Properties: []testutils.IDPProperty{
			{Name: "issuer", Value: mockOIDCServer.GetURL()},
			{Name: "jwks_endpoint", Value: mockOIDCServer.GetJWKSURL()},
			{Name: "id_jag_enabled", Value: "true"},
		},
	})
	ts.Require().NoError(err, "create test IDP")
	ts.idpID = idpID

	jwksProxy, err := newJWKSHTTPProxy()
	ts.Require().NoError(err, "start JWKS HTTP proxy")
	ts.jwksProxy = jwksProxy

	selfRefIDPID, err := testutils.CreateIDP(testutils.IDP{
		Name:        fmt.Sprintf("IDJAG Self-Ref IdP %d", time.Now().UnixNano()),
		Description: "Self-referencing IdP for ID-JAG round-trip test",
		Type:        "OIDC",
		Properties: []testutils.IDPProperty{
			{Name: "issuer", Value: "https://localhost:8095"},
			{Name: "jwks_endpoint", Value: jwksProxy.URL()},
			{Name: "id_jag_enabled", Value: "true"},
		},
	})
	ts.Require().NoError(err, "create self-referencing IDP")
	ts.selfRefIDPID = selfRefIDPID

	ts.consumptionAppID = ts.createApp(map[string]any{
		"name": consumptionAppName, "description": "ID-JAG consumption test app",
		"ouId": ts.ouID, "type": "fullstack",
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"idjag-consumption-person"},
		"inboundAuthConfig": []map[string]any{{
			"type": "oauth2",
			"config": map[string]any{
				"clientId": consumptionClientID, "clientSecret": consumptionClientSecret,
				"redirectUris":            []string{consumptionRedirectURI},
				"grantTypes":              []string{"urn:ietf:params:oauth:grant-type:jwt-bearer"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"scopes":                  []string{"openid", "read", "write", "admin"},
			},
		}},
	})

	ts.roundtripAppID = ts.createApp(map[string]any{
		"name": roundtripAppName, "description": "ID-JAG roundtrip issuer app",
		"ouId": ts.ouID, "type": "fullstack", "authFlowId": ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"idjag-consumption-person"},
		"inboundAuthConfig": []map[string]any{{
			"type": "oauth2",
			"config": map[string]any{
				"clientId": roundtripClientID, "clientSecret": roundtripClientSecret,
				"redirectUris": []string{consumptionRedirectURI},
				"grantTypes": []string{
					"authorization_code",
					"urn:ietf:params:oauth:grant-type:token-exchange",
					"urn:ietf:params:oauth:grant-type:jwt-bearer",
				},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"scopes":                  []string{"openid", "profile", "email"},
				"token": map[string]any{
					"idJag": map[string]any{
						"enabled":          true,
						"allowedAudiences": []string{"https://localhost:8095"},
					},
				},
			},
		}},
	})

	rs := testutils.ResourceServer{
		Name: "IDJAG Consumption Test RS", Identifier: consumptionRSIdentifier, OUID: ts.ouID,
	}
	rsActions := []testutils.Action{
		{Name: "Read", Handle: "read", Description: "Read permission"},
		{Name: "Write", Handle: "write", Description: "Write permission"},
		{Name: "Admin", Handle: "admin", Description: "Admin permission"},
	}
	rsID, err := testutils.CreateResourceServerWithActions(rs, rsActions)
	ts.Require().NoError(err, "create test resource server")
	ts.resourceServerID = rsID
}

func (ts *IDJAGConsumptionTestSuite) TearDownSuite() {
	if ts.resourceServerID != "" {
		_ = testutils.DeleteResourceServer(ts.resourceServerID)
	}
	if ts.selfRefIDPID != "" {
		_ = testutils.DeleteIDP(ts.selfRefIDPID)
	}
	if ts.idpID != "" {
		_ = testutils.DeleteIDP(ts.idpID)
	}
	if ts.consumptionAppID != "" {
		_ = testutils.DeleteApplication(ts.consumptionAppID)
	}
	if ts.roundtripAppID != "" {
		_ = testutils.DeleteApplication(ts.roundtripAppID)
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
	if ts.mockOIDCServer != nil {
		_ = ts.mockOIDCServer.Stop()
	}
	if ts.jwksProxy != nil {
		_ = ts.jwksProxy.Stop()
	}
}

// createApp POSTs an application definition and returns its ID.
func (ts *IDJAGConsumptionTestSuite) createApp(app map[string]any) string {
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

// createAppExpectError POSTs an application definition that is expected to be
// rejected, returning the response status code and the decoded error body. If the
// application is unexpectedly created, it is deleted immediately so a regression
// does not leave a stray application behind for later test runs to trip over.
func (ts *IDJAGConsumptionTestSuite) createAppExpectError(app map[string]any) (int, testutils.ErrorResponse) {
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

// buildIDJAGAssertion constructs a valid ID-JAG JWT signed by the mock OIDC
// server, optionally mutated via the supplied overrides.
func (ts *IDJAGConsumptionTestSuite) buildIDJAGAssertion(
	overrides ...func(header, claims map[string]interface{}),
) string {
	ts.T().Helper()

	header := map[string]interface{}{
		"typ": "oauth-id-jag+jwt",
		"alg": "RS256",
		"kid": "oidc-key-1",
	}

	now := time.Now()
	claims := map[string]interface{}{
		"iss":       ts.mockOIDCServer.GetURL(),
		"sub":       mockExternalSubject,
		"aud":       "https://localhost:8095",
		"client_id": consumptionClientID,
		"exp":       now.Add(5 * time.Minute).Unix(),
		"iat":       now.Unix(),
		"nbf":       now.Unix(),
		"jti":       fmt.Sprintf("jti-%d", now.UnixNano()),
		"scope":     "read write",
		"resource":  consumptionRSIdentifier,
	}

	for _, override := range overrides {
		override(header, claims)
	}

	token, err := ts.mockOIDCServer.SignJWT(header, claims)
	ts.Require().NoError(err)
	return token
}

// consumeIDJAG sends a JWT bearer grant request to /oauth2/token with the
// given assertion, authenticating with clientID/clientSecret (or as a public
// client when clientSecret is empty).
func (ts *IDJAGConsumptionTestSuite) consumeIDJAG(
	assertion string, extraParams map[string]string, clientID, clientSecret string,
) (IDJAGTokenResponse, int, error) {
	ts.T().Helper()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	formData.Set("assertion", assertion)
	for k, v := range extraParams {
		formData.Set(k, v)
	}

	var req *http.Request
	var err error
	if clientSecret != "" {
		req, err = http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token",
			strings.NewReader(formData.Encode()))
		if err != nil {
			return IDJAGTokenResponse{}, 0, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Basic "+basicAuth(clientID, clientSecret))
	} else {
		formData.Set("client_id", clientID)
		req, err = http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token",
			strings.NewReader(formData.Encode()))
		if err != nil {
			return IDJAGTokenResponse{}, 0, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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

// obtainIDTokenForRoundtrip runs the full authorization code flow for the
// roundtrip issuance application and returns the resulting self-issued ID
// token.
func (ts *IDJAGConsumptionTestSuite) obtainIDTokenForRoundtrip() string {
	ts.T().Helper()

	verifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err)
	challenge := testutils.GenerateCodeChallenge(verifier)

	params := url.Values{}
	params.Set("client_id", roundtripClientID)
	params.Set("redirect_uri", consumptionRedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid email")
	params.Set("state", "idjag-roundtrip-state")
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
		"username": consumptionTestUser, "password": consumptionTestPassword,
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
	formData.Set("redirect_uri", consumptionRedirectURI)
	formData.Set("code_verifier", verifier)

	tokenReq, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token",
		strings.NewReader(formData.Encode()))
	ts.Require().NoError(err)
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenReq.Header.Set("Authorization", "Basic "+basicAuth(roundtripClientID, roundtripClientSecret))

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

func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_Success() {
	assertion := ts.buildIDJAGAssertion()

	resp, statusCode, err := ts.consumeIDJAG(assertion, nil, consumptionClientID, consumptionClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, statusCode)
	ts.NotEmpty(resp.AccessToken)
	ts.Equal("Bearer", resp.TokenType)
	ts.NotZero(resp.ExpiresIn)

	claims, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err)
	ts.Equal(mockExternalSubject, claims.Sub)
	ts.Equal(ts.mockOIDCServer.GetURL(), claims.Additional["idp"])
	ts.Equal("urn:ietf:params:oauth:grant-type:jwt-bearer", claims.Additional["grant_type"])
}

func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_ScopeIntersection() {
	assertion := ts.buildIDJAGAssertion(func(h, c map[string]interface{}) {
		c["scope"] = "read write admin"
	})

	resp, statusCode, err := ts.consumeIDJAG(assertion,
		map[string]string{"scope": "read write"},
		consumptionClientID, consumptionClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, statusCode)

	claims, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err)
	scope, ok := claims.Additional["scope"].(string)
	ts.Require().True(ok, "access token should contain scope claim")
	ts.Contains(scope, "read")
	ts.Contains(scope, "write")
	ts.NotContains(scope, "admin")
}

func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_ResourceBinding() {
	assertion := ts.buildIDJAGAssertion()

	resp, statusCode, err := ts.consumeIDJAG(assertion, map[string]string{"resource": consumptionRSIdentifier},
		consumptionClientID, consumptionClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, statusCode)

	claims, err := testutils.DecodeJWT(resp.AccessToken)
	ts.Require().NoError(err)
	ts.Equal(consumptionRSIdentifier, claims.Aud)
}

func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_ResourceNotInAssertion() {
	assertion := ts.buildIDJAGAssertion()

	resp, statusCode, err := ts.consumeIDJAG(assertion,
		map[string]string{"resource": "https://unauthorized-rs.example.com"},
		consumptionClientID, consumptionClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_target", resp.Error)
}

// TestIDJAGConsumption_NoResourceNoDefault verifies that when the assertion carries
// permission scopes but no resource claim, the request supplies no resource parameter
// either, and no default resource server is configured, the token endpoint rejects the
// request with invalid_target rather than binding the access token to an unresolved
// audience.
func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_NoResourceNoDefault() {
	assertion := ts.buildIDJAGAssertion(func(h, c map[string]interface{}) {
		delete(c, "resource")
	})

	resp, statusCode, err := ts.consumeIDJAG(assertion, nil, consumptionClientID, consumptionClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_target", resp.Error)
}

func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_ExpiredAssertion() {
	assertion := ts.buildIDJAGAssertion(func(h, c map[string]interface{}) {
		c["exp"] = time.Now().Add(-5 * time.Minute).Unix()
	})

	resp, statusCode, err := ts.consumeIDJAG(assertion, nil, consumptionClientID, consumptionClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_grant", resp.Error)
	ts.Contains(resp.ErrorDescription, "expired")
}

func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_UntrustedIssuer() {
	assertion := ts.buildIDJAGAssertion(func(h, c map[string]interface{}) {
		c["iss"] = "https://unknown-issuer.example.com"
	})

	resp, statusCode, err := ts.consumeIDJAG(assertion, nil, consumptionClientID, consumptionClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_grant", resp.Error)
}

// TestIDJAGConsumption_IDJAGNotEnabled uses an issuer with no registered IDP
// at all, which exercises the same resolveIDJAGIssuer failure path as an
// IDP that exists but has ID-JAG disabled.
func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_IDJAGNotEnabled() {
	assertion := ts.buildIDJAGAssertion(func(h, c map[string]interface{}) {
		c["iss"] = "http://localhost:9999"
	})

	resp, statusCode, err := ts.consumeIDJAG(assertion, nil, consumptionClientID, consumptionClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_grant", resp.Error)
}

func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_AudienceMismatch() {
	assertion := ts.buildIDJAGAssertion(func(h, c map[string]interface{}) {
		c["aud"] = "https://wrong-server.example.com"
	})

	resp, statusCode, err := ts.consumeIDJAG(assertion, nil, consumptionClientID, consumptionClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_grant", resp.Error)
}

func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_CrossClientExchange() {
	assertion := ts.buildIDJAGAssertion(func(h, c map[string]interface{}) {
		c["client_id"] = "external-source-client"
	})

	resp, statusCode, err := ts.consumeIDJAG(assertion, nil, consumptionClientID, consumptionClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, statusCode)
	ts.NotEmpty(resp.AccessToken)
	ts.Equal("Bearer", resp.TokenType)
}

func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_WrongTypHeader() {
	assertion := ts.buildIDJAGAssertion(func(h, c map[string]interface{}) {
		h["typ"] = "JWT"
	})

	resp, statusCode, err := ts.consumeIDJAG(assertion, nil, consumptionClientID, consumptionClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_grant", resp.Error)
}

func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_MissingAssertion() {
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token",
		strings.NewReader(formData.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+basicAuth(consumptionClientID, consumptionClientSecret))

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusBadRequest, resp.StatusCode)
	var result IDJAGTokenResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&result))
	ts.Equal("invalid_request", result.Error)
}

func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_JTIReplay() {
	fixedJTI := "replay-test-jti-fixed"
	assertion := ts.buildIDJAGAssertion(func(h, c map[string]interface{}) {
		c["jti"] = fixedJTI
	})

	resp1, statusCode1, err := ts.consumeIDJAG(assertion, nil, consumptionClientID, consumptionClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, statusCode1)
	ts.NotEmpty(resp1.AccessToken)

	resp2, statusCode2, err := ts.consumeIDJAG(assertion, nil, consumptionClientID, consumptionClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode2)
	ts.Equal("invalid_grant", resp2.Error)
}

func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_MissingJTI() {
	assertion := ts.buildIDJAGAssertion(func(h, c map[string]interface{}) {
		delete(c, "jti")
	})
	resp, statusCode, err := ts.consumeIDJAG(assertion, nil, consumptionClientID, consumptionClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_grant", resp.Error)
}

// TestIDJAGConsumption_PublicClientConfigRejectedAtCreate verifies that an application
// combining a public client (tokenEndpointAuthMethod: "none") with the jwt-bearer grant
// type is rejected at creation time, since jwt-bearer requires a confidential client.
func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_PublicClientConfigRejectedAtCreate() {
	statusCode, errResp := ts.createAppExpectError(map[string]any{
		"name": publicConsAppName, "description": "ID-JAG consumption test public app",
		"ouId": ts.ouID, "type": "fullstack",
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"idjag-consumption-person"},
		"inboundAuthConfig": []map[string]any{{
			"type": "oauth2",
			"config": map[string]any{
				"clientId":                publicConsClientID,
				"redirectUris":            []string{consumptionRedirectURI},
				"grantTypes":              []string{"authorization_code", "urn:ietf:params:oauth:grant-type:jwt-bearer"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "none",
				"publicClient":            true,
				"pkceRequired":            true,
				"scopes":                  []string{"openid", "read", "write"},
			},
		}},
	})

	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal(appErrorInvalidOAuthConfiguration, errResp.Code)
	ts.Contains(errResp.Description.DefaultValue, "jwt-bearer")
}

// TestIDJAGConsumption_UnauthenticatedClientRejected verifies that the jwt-bearer grant
// rejects a request at the token endpoint that omits client authentication entirely for a
// client registered as confidential (client_secret_basic): the client authentication layer
// rejects it as invalid_client before the grant handler is ever reached.
func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_UnauthenticatedClientRejected() {
	resp, statusCode, err := ts.consumeIDJAG(ts.buildIDJAGAssertion(), nil, consumptionClientID, "")
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, statusCode)
	ts.Equal("invalid_client", resp.Error)
}

func (ts *IDJAGConsumptionTestSuite) TestIDJAGConsumption_MissingSubject() {
	assertion := ts.buildIDJAGAssertion(func(h, c map[string]interface{}) {
		delete(c, "sub")
	})
	resp, statusCode, err := ts.consumeIDJAG(assertion, nil, consumptionClientID, consumptionClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_grant", resp.Error)
}

// TestIDJAG_FullRoundtrip issues an ID-JAG from the roundtrip app via token
// exchange, then consumes that same ID-JAG through the JWT bearer grant on
// the same app, verifying issuance and consumption chain correctly.
func (ts *IDJAGConsumptionTestSuite) TestIDJAG_FullRoundtrip() {
	idToken := ts.obtainIDTokenForRoundtrip()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", idToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:id_token")
	formData.Set("requested_token_type", "urn:ietf:params:oauth:token-type:id-jag")
	formData.Set("audience", "https://localhost:8095")

	exchangeReq, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token",
		strings.NewReader(formData.Encode()))
	ts.Require().NoError(err)
	exchangeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	exchangeReq.Header.Set("Authorization", "Basic "+basicAuth(roundtripClientID, roundtripClientSecret))

	exchangeResp, err := testutils.GetRawHTTPClient().Do(exchangeReq)
	ts.Require().NoError(err)
	defer exchangeResp.Body.Close()
	ts.Require().Equal(http.StatusOK, exchangeResp.StatusCode)

	var exchangeResult IDJAGTokenResponse
	ts.Require().NoError(json.NewDecoder(exchangeResp.Body).Decode(&exchangeResult))
	ts.NotEmpty(exchangeResult.AccessToken)
	ts.Equal("urn:ietf:params:oauth:token-type:id-jag", exchangeResult.IssuedTokenType)

	idJagToken := exchangeResult.AccessToken

	header, err := testutils.DecodeJWTHeaderMap(idJagToken)
	ts.Require().NoError(err)
	ts.Equal("oauth-id-jag+jwt", header["typ"])

	consResp, consStatus, err := ts.consumeIDJAG(idJagToken, nil, roundtripClientID, roundtripClientSecret)
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, consStatus)
	ts.NotEmpty(consResp.AccessToken)
	ts.Equal("Bearer", consResp.TokenType)

	consClaims, err := testutils.DecodeJWT(consResp.AccessToken)
	ts.Require().NoError(err)
	ts.NotEmpty(consClaims.Sub)
	ts.Equal("https://localhost:8095", consClaims.Additional["idp"])
	ts.Equal("urn:ietf:params:oauth:grant-type:jwt-bearer", consClaims.Additional["grant_type"])
}

func (ts *IDJAGConsumptionTestSuite) TestIDJAG_DPoPBoundAccessToken() {
	assertion := ts.buildIDJAGAssertion()

	dpopKey, err := testutils.GenerateDPoPKey("ES256")
	ts.Require().NoError(err)

	proof, err := dpopKey.CreateProof("POST", testutils.TestServerURL+"/oauth2/token", testutils.DPoPProofOptions{})
	ts.Require().NoError(err)

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	formData.Set("assertion", assertion)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token",
		strings.NewReader(formData.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+basicAuth(consumptionClientID, consumptionClientSecret))
	req.Header.Set("DPoP", proof)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var result IDJAGTokenResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&result))
	ts.Equal("DPoP", result.TokenType)

	claims, err := testutils.DecodeJWT(result.AccessToken)
	ts.Require().NoError(err)
	cnf, ok := claims.Additional["cnf"].(map[string]interface{})
	ts.Require().True(ok, "access token should contain cnf claim")
	ts.NotEmpty(cnf["jkt"], "cnf.jkt should be present for DPoP-bound token")
}
