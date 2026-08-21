// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package introspect exercises the RFC 7662 POST /oauth2/introspect endpoint end to end against the
// packaged product: real client authentication, real signed tokens, the real revocation deny list,
// and the real route surface (method handling and CORS preflight). The suite covers the active
// response shape (asserted field by field against the token's own decoded payload), every inactive
// path the RFC requires to be reported as HTTP 200 {"active": false}, the token_type_hint
// pass through, and the client authentication failure matrix produced by the shared clientauth
// middleware.
package introspect

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
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
	introspectEndpoint = testutils.TestServerURL + "/oauth2/introspect"
	revokeEndpoint     = testutils.TestServerURL + "/oauth2/revoke"
	tokenEndpoint      = testutils.TestServerURL + "/oauth2/token"

	// userClient runs the authorization_code login. ObtainAccessTokenWithPassword always sends the
	// client credentials in the token request body, so this client must register client_secret_post.
	userClientID     = "introspect_user_client"
	userClientSecret = "introspect_user_secret"

	// ccClient is a confidential client_secret_basic client used for the client authentication
	// failure matrix and for the route level checks.
	ccClientID     = "introspect_cc_client"
	ccClientSecret = "introspect_cc_secret"

	// shortClient issues client access tokens that live for one second, so the suite can introspect a
	// genuinely product issued expired token.
	shortClientID     = "introspect_short_client"
	shortClientSecret = "introspect_short_secret"
	// shortTokenValiditySeconds is the accessToken.clientConfig.validityPeriod registered for
	// shortClient, in seconds.
	shortTokenValiditySeconds = 1

	// jwtLeewaySeconds mirrors the server's jwt.leeway default (backend/cmd/server/config/default.json).
	// Claim validation treats a token as expired only once now >= exp + leeway, so a test that waits for
	// a real expiry has to clear the leeway window too.
	jwtLeewaySeconds = 30

	redirectURI              = "https://localhost:3000"
	resourceServerIdentifier = "https://introspect.example.com"

	testUsername  = "introspect_user"
	testPassword  = "SecurePass123!"
	testEmail     = "introspect_user@example.com"
	testGivenName = "Intro"
	testFamily    = "Spection"
)

var testUserType = testutils.UserType{
	Name: "introspect-person",
	Schema: map[string]interface{}{
		"username":    map[string]interface{}{"type": "string"},
		"password":    map[string]interface{}{"type": "string", "credential": true},
		"email":       map[string]interface{}{"type": "string"},
		"given_name":  map[string]interface{}{"type": "string"},
		"family_name": map[string]interface{}{"type": "string"},
	},
}

// introspectResult is the parsed outcome of one call to the introspection endpoint.
type introspectResult struct {
	StatusCode int
	Header     http.Header
	Body       map[string]any
	Raw        []byte
}

// errorCode returns the OAuth2 error code carried by an error response, or the empty string.
func (r introspectResult) errorCode() string {
	code, _ := r.Body["error"].(string)
	return code
}

// active returns the value of the RFC 7662 active member.
func (r introspectResult) active() bool {
	value, _ := r.Body["active"].(bool)
	return value
}

// IntrospectionTestSuite covers POST /oauth2/introspect.
type IntrospectionTestSuite struct {
	suite.Suite
	client           *http.Client
	ouID             string
	entityTypeID     string
	userID           string
	flowID           string
	resourceServerID string
	userAppID        string
	ccAppID          string
	shortAppID       string

	// userAccessToken and userRefreshToken come from a single authorization_code login, reused by the
	// tests that only read the token instead of consuming it.
	userAccessToken  string
	userRefreshToken string

	// shortLivedToken is issued during setup so its one second lifetime starts ticking as early as
	// possible; the expiry test only waits out whatever is left of the leeway window.
	shortLivedToken string
}

func TestIntrospectionTestSuite(t *testing.T) {
	suite.Run(t, new(IntrospectionTestSuite))
}

func (ts *IntrospectionTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "introspect-test-ou",
		Name:        "Introspection Test OU",
		Description: "Organization unit for token introspection integration tests",
		Parent:      nil,
	})
	ts.Require().NoError(err, "failed to create test organization unit")
	ts.ouID = ouID

	testUserType.OUID = ts.ouID
	entityTypeID, err := testutils.CreateUserType(testUserType)
	ts.Require().NoError(err, "failed to create test user type")
	ts.entityTypeID = entityTypeID

	ts.userID = ts.createTestUser()

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Introspection Resource Server",
		Description: "Resource server for token introspection integration tests",
		Identifier:  resourceServerIdentifier,
		OUID:        ts.ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err, "failed to create resource server")
	ts.resourceServerID = resourceServerID

	ts.flowID = ts.createTestAuthenticationFlow()

	ts.userAppID = ts.createApp("IntrospectUserApp", userClientID, userClientSecret, "client_secret_post",
		[]string{"authorization_code", "refresh_token"}, ts.flowID, nil)
	ts.ccAppID = ts.createApp("IntrospectClientCredentialsApp", ccClientID, ccClientSecret, "client_secret_basic",
		[]string{"client_credentials"}, "", nil)
	ts.shortAppID = ts.createApp("IntrospectShortLivedApp", shortClientID, shortClientSecret, "client_secret_basic",
		[]string{"client_credentials"}, "", map[string]interface{}{
			"accessToken": map[string]interface{}{
				"clientConfig": map[string]interface{}{
					"validityPeriod": shortTokenValiditySeconds,
				},
			},
		})

	tokens := ts.loginWithAuthorizationCode()
	ts.userAccessToken = tokens.AccessToken
	ts.userRefreshToken = tokens.RefreshToken

	ts.shortLivedToken = ts.clientCredentialsToken(shortClientID, shortClientSecret)
}

func (ts *IntrospectionTestSuite) TearDownSuite() {
	for _, appID := range []string{ts.userAppID, ts.ccAppID, ts.shortAppID} {
		if appID != "" {
			ts.deleteApp(appID)
		}
	}
	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Logf("Failed to delete resource server: %v", err)
		}
	}
	if ts.flowID != "" {
		if err := testutils.DeleteFlow(ts.flowID); err != nil {
			ts.T().Logf("Failed to delete authentication flow: %v", err)
		}
	}
	if ts.userID != "" {
		if err := testutils.DeleteUser(ts.userID); err != nil {
			ts.T().Logf("Failed to delete test user: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type: %v", err)
		}
	}
}

//
// Fixtures.
//

func (ts *IntrospectionTestSuite) createTestUser() string {
	attributes := map[string]interface{}{
		"username":    testUsername,
		"password":    testPassword,
		"email":       testEmail,
		"given_name":  testGivenName,
		"family_name": testFamily,
	}
	attributesJSON, err := json.Marshal(attributes)
	ts.Require().NoError(err, "failed to marshal user attributes")

	userID, err := testutils.CreateUser(testutils.User{
		Type:       testUserType.Name,
		OUID:       ts.ouID,
		Attributes: json.RawMessage(attributesJSON),
	})
	ts.Require().NoError(err, "failed to create test user")
	return userID
}

func (ts *IntrospectionTestSuite) createTestAuthenticationFlow() string {
	flow := testutils.Flow{
		Name:     "Introspection Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "introspect_auth_flow",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
							{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
					},
				},
			},
			{
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialsAuthExecutor",
					"inputs": []map[string]interface{}{
						{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
				},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}

	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "failed to create test authentication flow")
	return flowID
}

// setClientAttributes replaces an application's client-token attribute selection.
func (ts *IntrospectionTestSuite) setClientAttributes(appID, name, clientID, clientSecret string,
	attributes []string) {
	app := map[string]interface{}{
		"name":        name,
		"description": "Application for token introspection integration tests",
		"ouId":        ts.ouID,
		// Matches the type createApp used: an application's type is immutable after creation.
		"type":                      "fullstack",
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]interface{}{
			{"type": "oauth2", "config": map[string]interface{}{
				"clientId":                clientID,
				"clientSecret":            clientSecret,
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"token": map[string]interface{}{
					"accessToken": map[string]interface{}{
						"clientConfig": map[string]interface{}{"attributes": attributes},
					},
				},
			}},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPut, testutils.TestServerURL+"/applications/"+appID,
		bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equalf(http.StatusOK, resp.StatusCode, "failed to update application: %s", string(body))
}

// createApp registers an OAuth2 application. authFlowID and tokenConfig are optional.
func (ts *IntrospectionTestSuite) createApp(name, clientID, clientSecret, authMethod string,
	grantTypes []string, authFlowID string, tokenConfig map[string]interface{}) string {
	oauthConfig := map[string]interface{}{
		"clientId":                clientID,
		"clientSecret":            clientSecret,
		"redirectUris":            []string{redirectURI},
		"grantTypes":              grantTypes,
		"tokenEndpointAuthMethod": authMethod,
		"scopes":                  []string{"openid", "profile", "email"},
	}
	// The server rejects response types on a client_credentials only client, so declare them only for
	// the client that actually runs the authorization_code flow.
	for _, grantType := range grantTypes {
		if grantType == "authorization_code" {
			oauthConfig["responseTypes"] = []string{"code"}
			break
		}
	}
	if tokenConfig != nil {
		oauthConfig["token"] = tokenConfig
	}

	app := map[string]interface{}{
		"name":                      name,
		"description":               "Application for token introspection integration tests",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"isRegistrationFlowEnabled": false,
		"inboundAuthConfig": []map[string]interface{}{
			{"type": "oauth2", "config": oauthConfig},
		},
	}
	if authFlowID != "" {
		app["authFlowId"] = authFlowID
		app["allowedUserTypes"] = []string{testUserType.Name}
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err, "failed to marshal application payload")

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equalf(http.StatusCreated, resp.StatusCode, "create application failed: %s", string(body))

	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &created))
	id, ok := created["id"].(string)
	ts.Require().True(ok, "application id missing from create response")
	return id
}

func (ts *IntrospectionTestSuite) deleteApp(appID string) {
	deleteURL := fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, appID)
	req, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
	if err != nil {
		ts.T().Logf("Failed to build delete request: %v", err)
		return
	}
	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Logf("Failed to delete application: %v", err)
		return
	}
	resp.Body.Close()
}

//
// Token helpers.
//

// loginWithAuthorizationCode drives a full authorization_code login for the user client and returns
// the issued tokens.
func (ts *IntrospectionTestSuite) loginWithAuthorizationCode() *testutils.TokenResponse {
	tokens, err := testutils.ObtainAccessTokenWithPassword(userClientID, redirectURI, "openid profile email",
		testUsername, testPassword, true, userClientSecret, resourceServerIdentifier)
	ts.Require().NoError(err, "failed to obtain tokens via the authorization_code flow")
	ts.Require().NotEmpty(tokens.AccessToken, "authorization_code flow returned no access token")
	ts.Require().NotEmpty(tokens.RefreshToken, "authorization_code flow returned no refresh token")
	return tokens
}

// clientCredentialsToken obtains a client_credentials access token bound to the suite resource server.
func (ts *IntrospectionTestSuite) clientCredentialsToken(clientID, clientSecret string) string {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("resource", resourceServerIdentifier)

	req, err := http.NewRequest(http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equalf(http.StatusOK, resp.StatusCode, "client_credentials request failed: %s", string(body))

	var parsed map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &parsed))
	token, ok := parsed["access_token"].(string)
	ts.Require().True(ok, "access_token missing from token response")
	return token
}

// introspect posts form to the introspection endpoint. A non empty basicID attaches HTTP Basic
// client credentials; otherwise the form alone has to carry whatever client authentication is used.
func (ts *IntrospectionTestSuite) introspect(form url.Values, basicID, basicSecret string) introspectResult {
	req, err := http.NewRequest(http.MethodPost, introspectEndpoint, strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basicID != "" {
		req.SetBasicAuth(basicID, basicSecret)
	}

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	result := introspectResult{StatusCode: resp.StatusCode, Header: resp.Header, Raw: body}
	if len(body) > 0 {
		ts.Require().NoErrorf(json.Unmarshal(body, &result.Body), "introspection body is not JSON: %s", string(body))
	}
	return result
}

// introspectBasic introspects a token as the given client_secret_basic client.
func (ts *IntrospectionTestSuite) introspectBasic(token, clientID, clientSecret string) introspectResult {
	form := url.Values{}
	form.Set("token", token)
	return ts.introspect(form, clientID, clientSecret)
}

// introspectPost introspects a token as the given client_secret_post client.
func (ts *IntrospectionTestSuite) introspectPost(token, clientID, clientSecret string) introspectResult {
	form := url.Values{}
	form.Set("token", token)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	return ts.introspect(form, "", "")
}

// revokePost revokes a token as the given client_secret_post client.
func (ts *IntrospectionTestSuite) revokePost(token, clientID, clientSecret string) {
	form := url.Values{}
	form.Set("token", token)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req, err := http.NewRequest(http.MethodPost, revokeEndpoint, strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equalf(http.StatusOK, resp.StatusCode, "revocation failed: %s", string(body))
}

// signWithForeignKey rebuilds the given JWS with a signature produced by a freshly generated RSA key
// that the server has never seen. Header and payload are preserved verbatim, so the only reason the
// server can reject the result is the signature itself.
func (ts *IntrospectionTestSuite) signWithForeignKey(token string) string {
	parts := strings.Split(token, ".")
	ts.Require().Len(parts, 3, "expected a three part JWS")
	signingInput := parts[0] + "." + parts[1]

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	ts.Require().NoError(err, "failed to generate the foreign signing key")

	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	ts.Require().NoError(err, "failed to sign with the foreign key")

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

//
// Case 1: the active response for an authorization_code access token.
//

// TestIntrospect_ActiveAccessToken_MatchesTokenClaims asserts that introspecting an access token from
// an authorization_code login reports active with token_type Bearer, and that every member the
// response carries (scope, client_id, sub, aud, iss, exp, iat, nbf, jti) is present and equal to the
// corresponding claim in the token's own decoded payload.
func (ts *IntrospectionTestSuite) TestIntrospect_ActiveAccessToken_MatchesTokenClaims() {
	claims, err := testutils.DecodeJWTPayloadMap(ts.userAccessToken)
	ts.Require().NoError(err, "failed to decode the access token payload")

	res := ts.introspectPost(ts.userAccessToken, userClientID, userClientSecret)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.Require().True(res.active(), "an unexpired, unrevoked access token must be active")
	ts.Equal("Bearer", res.Body["token_type"], "a token without cnf must report token_type Bearer")

	for _, claim := range []string{"scope", "client_id", "sub", "aud", "iss", "jti"} {
		ts.Require().Containsf(res.Body, claim, "introspection response is missing %q", claim)
		ts.Require().Containsf(claims, claim, "the access token itself is missing the %q claim", claim)
		ts.Equalf(claims[claim], res.Body[claim], "introspection %q must match the token claim", claim)
	}
	for _, claim := range []string{"exp", "iat", "nbf"} {
		ts.Require().Containsf(res.Body, claim, "introspection response is missing %q", claim)
		ts.Require().Containsf(claims, claim, "the access token itself is missing the %q claim", claim)
		responseValue, ok := res.Body[claim].(float64)
		ts.Require().Truef(ok, "introspection %q must be a number", claim)
		tokenValue, ok := claims[claim].(float64)
		ts.Require().Truef(ok, "token claim %q must be a number", claim)
		ts.Equalf(int64(tokenValue), int64(responseValue), "introspection %q must match the token claim", claim)
	}

	ts.Equal(userClientID, res.Body["client_id"], "client_id must identify the client the token was issued to")
	ts.NotContains(res.Body, "cnf", "a bearer token must not carry a cnf member")
	ts.NotContains(res.Body, "username",
		"no self issued token carries a username claim, so the member must stay omitted")
}

// TestIntrospect_ClientToken_CarriesSubType asserts that introspection reports the subject's identity
// class, matching the token's own claim, so an introspecting resource server sees the same signal.
func (ts *IntrospectionTestSuite) TestIntrospect_ClientToken_CarriesSubType() {
	token := ts.clientCredentialsToken(ccClientID, ccClientSecret)

	claims, err := testutils.DecodeJWTPayloadMap(token)
	ts.Require().NoError(err, "failed to decode the client_credentials token payload")
	ts.Require().Equal("application", claims["sub_type"], "an application's client token must carry sub_type")

	res := ts.introspectBasic(token, ccClientID, ccClientSecret)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.Require().True(res.active())
	ts.Equal(claims["sub_type"], res.Body["sub_type"], "introspection sub_type must match the token claim")
}

// TestIntrospect_ClientToken_OmitsUnselectedSubType asserts that a client whose selection omits
// sub_type is reported without it by introspection too, so no class is recoverable that the token does
// not assert. Removal is an update, since creation selects the claim.
func (ts *IntrospectionTestSuite) TestIntrospect_ClientToken_OmitsUnselectedSubType() {
	const clientID = "introspect_cc_no_subtype_client"
	const clientSecret = "introspect_cc_no_subtype_secret"

	appID := ts.createApp("IntrospectNoSubTypeApp", clientID, clientSecret, "client_secret_basic",
		[]string{"client_credentials"}, "", nil)
	defer ts.deleteApp(appID)
	ts.setClientAttributes(appID, "IntrospectNoSubTypeApp", clientID, clientSecret, []string{})

	token := ts.clientCredentialsToken(clientID, clientSecret)

	claims, err := testutils.DecodeJWTPayloadMap(token)
	ts.Require().NoError(err, "failed to decode the client_credentials token payload")
	ts.Require().NotContains(claims, "sub_type",
		"the token must not carry sub_type when the client's selection omits it")

	res := ts.introspectBasic(token, clientID, clientSecret)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.Require().True(res.active())
	ts.NotContains(res.Body, "sub_type", "introspection must not report a class the token does not assert")
}

// TestIntrospect_UserToken_OmitsSubType asserts that a user-subject token reports no identity class:
// its subject is a user, so the member must not report the client's class.
func (ts *IntrospectionTestSuite) TestIntrospect_UserToken_OmitsSubType() {
	res := ts.introspectPost(ts.userAccessToken, userClientID, userClientSecret)

	ts.Require().Equalf(http.StatusOK, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.Require().True(res.active())
	ts.NotContains(res.Body, "sub_type", "a user-subject token must not report an identity class")
}

//
// Cases 2 to 5: everything that is inactive is still HTTP 200 {"active": false}.
//

// TestIntrospect_ExpiredToken_IsInactive introspects a token issued by a client registered with a one
// second access token validity, after waiting out both the validity and the server's claim
// validation leeway. The token is asserted to be genuinely past its own exp before the call, and the
// endpoint must answer 200 {"active": false} rather than any 4xx.
func (ts *IntrospectionTestSuite) TestIntrospect_ExpiredToken_IsInactive() {
	claims, err := testutils.DecodeJWTPayloadMap(ts.shortLivedToken)
	ts.Require().NoError(err, "failed to decode the short lived token payload")
	exp, ok := claims["exp"].(float64)
	ts.Require().True(ok, "the short lived token must carry a numeric exp claim")
	iat, ok := claims["iat"].(float64)
	ts.Require().True(ok, "the short lived token must carry a numeric iat claim")
	ts.Require().EqualValues(shortTokenValiditySeconds, int64(exp)-int64(iat),
		"the registered accessToken.clientConfig.validityPeriod was not applied to the issued token")

	// Claim validation only rejects once now >= exp + leeway, so wait past the leeway window too.
	if remaining := time.Until(time.Unix(int64(exp)+jwtLeewaySeconds+1, 0)); remaining > 0 {
		time.Sleep(remaining)
	}
	ts.Require().Greater(time.Now().Unix(), int64(exp)+jwtLeewaySeconds,
		"the token must actually be expired before it is introspected")

	res := ts.introspectBasic(ts.shortLivedToken, shortClientID, shortClientSecret)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.False(res.active(), "an expired token must be reported inactive")
	ts.Len(res.Body, 1, "an inactive response must be exactly {\"active\": false}")
}

// TestIntrospect_MalformedToken_IsInactive asserts a token value that is not even a JWS is reported
// inactive with HTTP 200 instead of being treated as a protocol error.
func (ts *IntrospectionTestSuite) TestIntrospect_MalformedToken_IsInactive() {
	res := ts.introspectBasic("this-is-not-a-json-web-token", ccClientID, ccClientSecret)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.False(res.active(), "a malformed token must be reported inactive")
	ts.Len(res.Body, 1, "an inactive response must be exactly {\"active\": false}")
}

// TestIntrospect_ForeignlySignedToken_IsInactive takes a live access token, keeps its header and
// payload, and re-signs it with a key the server has never seen. Only the signature differs, so a
// response of active would mean the signature was never verified.
func (ts *IntrospectionTestSuite) TestIntrospect_ForeignlySignedToken_IsInactive() {
	forged := ts.signWithForeignKey(ts.userAccessToken)
	ts.Require().NotEqual(ts.userAccessToken, forged, "the forged token must differ from the original")

	res := ts.introspectPost(forged, userClientID, userClientSecret)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.False(res.active(), "a token signed by a foreign key must be reported inactive")
	ts.Len(res.Body, 1, "an inactive response must be exactly {\"active\": false}")
}

// TestIntrospect_RevokedToken_IsInactive drives a dedicated login, revokes the resulting access token
// through /oauth2/revoke, and asserts the deny list is consulted on the introspection hot path.
func (ts *IntrospectionTestSuite) TestIntrospect_RevokedToken_IsInactive() {
	tokens := ts.loginWithAuthorizationCode()

	before := ts.introspectPost(tokens.AccessToken, userClientID, userClientSecret)
	ts.Require().Equal(http.StatusOK, before.StatusCode)
	ts.Require().True(before.active(), "the token must be active before it is revoked")

	ts.revokePost(tokens.AccessToken, userClientID, userClientSecret)

	res := ts.introspectPost(tokens.AccessToken, userClientID, userClientSecret)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.False(res.active(), "a revoked token must be reported inactive")
	ts.Len(res.Body, 1, "an inactive response must be exactly {\"active\": false}")
}

//
// Case 6: introspection is deliberately token type agnostic.
//

// TestIntrospect_RefreshToken_IsActive asserts a refresh token introspects as active, and records the
// sub it reports. Refresh tokens are subject to the client, so sub is the client id and the end user
// stays in the separate access_token_sub claim.
func (ts *IntrospectionTestSuite) TestIntrospect_RefreshToken_IsActive() {
	claims, err := testutils.DecodeJWTPayloadMap(ts.userRefreshToken)
	ts.Require().NoError(err, "failed to decode the refresh token payload")

	res := ts.introspectPost(ts.userRefreshToken, userClientID, userClientSecret)
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.Require().True(res.active(), "a refresh token must introspect as active")

	ts.T().Logf("refresh token introspection reported sub=%q (access_token_sub=%v)",
		res.Body["sub"], claims["access_token_sub"])
	ts.Equal(claims["sub"], res.Body["sub"], "introspection sub must match the refresh token sub claim")
	ts.Equal(userClientID, res.Body["sub"], "a refresh token is subject to the client, so sub is the client id")
	ts.Equal(ts.userID, claims["access_token_sub"],
		"the end user stays in access_token_sub, which introspection does not surface")
}

//
// Cases 7 and 8: token_type_hint is accepted and ignored.
//

// TestIntrospect_TokenTypeHintAccessToken_IsAccepted asserts the documented hint value changes
// nothing about the outcome.
func (ts *IntrospectionTestSuite) TestIntrospect_TokenTypeHintAccessToken_IsAccepted() {
	form := url.Values{}
	form.Set("token", ts.userAccessToken)
	form.Set("token_type_hint", "access_token")
	form.Set("client_id", userClientID)
	form.Set("client_secret", userClientSecret)

	res := ts.introspect(form, "", "")
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.True(res.active(), "a valid access token stays active when token_type_hint is supplied")
	ts.Equal("Bearer", res.Body["token_type"])
}

// TestIntrospect_UnknownTokenTypeHint_IsIgnored asserts an unregistered hint value is not validated:
// the server reads token_type_hint and then ignores it, so the outcome is unchanged.
func (ts *IntrospectionTestSuite) TestIntrospect_UnknownTokenTypeHint_IsIgnored() {
	form := url.Values{}
	form.Set("token", ts.userAccessToken)
	form.Set("token_type_hint", "bogus_value")
	form.Set("client_id", userClientID)
	form.Set("client_secret", userClientSecret)

	res := ts.introspect(form, "", "")
	ts.Require().Equalf(http.StatusOK, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.True(res.active(), "an unknown token_type_hint must not change the outcome")
	ts.Equal("Bearer", res.Body["token_type"])
}

//
// Cases 9 and 10: the token parameter is required.
//

// TestIntrospect_MissingTokenParameter_IsInvalidRequest asserts a request that authenticates
// correctly but omits token is a protocol error, not an inactive token.
func (ts *IntrospectionTestSuite) TestIntrospect_MissingTokenParameter_IsInvalidRequest() {
	form := url.Values{}
	form.Set("client_id", ccClientID)

	res := ts.introspect(form, ccClientID, ccClientSecret)
	ts.Require().Equalf(http.StatusBadRequest, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.Equal("invalid_request", res.errorCode())
	ts.NotContains(res.Body, "active", "a protocol error must not carry an active member")
}

// TestIntrospect_EmptyTokenParameter_IsInvalidRequest asserts an empty token value is treated the
// same as an absent one.
func (ts *IntrospectionTestSuite) TestIntrospect_EmptyTokenParameter_IsInvalidRequest() {
	form := url.Values{}
	form.Set("token", "")

	res := ts.introspect(form, ccClientID, ccClientSecret)
	ts.Require().Equalf(http.StatusBadRequest, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.Equal("invalid_request", res.errorCode())
	ts.NotContains(res.Body, "active", "a protocol error must not carry an active member")
}

//
// Cases 11 to 15: the client authentication failure matrix.
//

// TestIntrospect_NoClientAuthentication_IsInvalidRequest asserts a request that presents no
// credentials and no client_id cannot identify a client at all, so the shared middleware rejects it
// with 400 invalid_request before any client lookup happens.
func (ts *IntrospectionTestSuite) TestIntrospect_NoClientAuthentication_IsInvalidRequest() {
	form := url.Values{}
	form.Set("token", ts.userAccessToken)

	res := ts.introspect(form, "", "")
	ts.Require().Equalf(http.StatusBadRequest, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.Equal("invalid_request", res.errorCode())
	ts.Equal("Missing client_id parameter", res.Body["error_description"])
}

// TestIntrospect_ClientIDOnlyForConfidentialClient_IsInvalidClient asserts a bare client_id, which
// only identifies a public client, is rejected with 401 invalid_client for a client registered with
// client_secret_basic. No Authorization header was presented, so the middleware also does not add a
// WWW-Authenticate challenge; the challenge case is covered by the wrong secret test below.
func (ts *IntrospectionTestSuite) TestIntrospect_ClientIDOnlyForConfidentialClient_IsInvalidClient() {
	form := url.Values{}
	form.Set("token", ts.userAccessToken)
	form.Set("client_id", ccClientID)

	res := ts.introspect(form, "", "")
	ts.Require().Equalf(http.StatusUnauthorized, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.Equal("invalid_client", res.errorCode())
	ts.Equal("Client authentication is required", res.Body["error_description"])
	ts.Empty(res.Header.Get("WWW-Authenticate"),
		"the challenge is only added when the request carried an Authorization header")
}

// TestIntrospect_WrongClientSecret_IsInvalidClient asserts a Basic credential with the wrong secret
// is rejected with 401 invalid_client and challenged with WWW-Authenticate: Basic.
func (ts *IntrospectionTestSuite) TestIntrospect_WrongClientSecret_IsInvalidClient() {
	res := ts.introspectBasic(ts.userAccessToken, ccClientID, "definitely_not_the_secret")
	ts.Require().Equalf(http.StatusUnauthorized, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.Equal("invalid_client", res.errorCode())
	ts.Equal("Basic", res.Header.Get("WWW-Authenticate"),
		"a 401 on a request that carried an Authorization header must challenge with Basic")
}

// TestIntrospect_UnknownClientID_IsInvalidClient asserts an unregistered client id is rejected the
// same way as a wrong secret, so introspection does not leak which client ids exist.
func (ts *IntrospectionTestSuite) TestIntrospect_UnknownClientID_IsInvalidClient() {
	res := ts.introspectBasic(ts.userAccessToken, "no_such_client_at_all", "any_secret")
	ts.Require().Equalf(http.StatusUnauthorized, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.Equal("invalid_client", res.errorCode())
	ts.Equal("Invalid client credentials", res.Body["error_description"],
		"an unknown client must be indistinguishable from a wrong secret")
	ts.Equal("Basic", res.Header.Get("WWW-Authenticate"))
}

// TestIntrospect_MismatchedAuthMethod_IsUnauthorizedClient presents correct credentials for the
// client_secret_basic client, but sends them the client_secret_post way. The presented method must
// match the registered one, so the request is rejected with 400 unauthorized_client.
func (ts *IntrospectionTestSuite) TestIntrospect_MismatchedAuthMethod_IsUnauthorizedClient() {
	res := ts.introspectPost(ts.userAccessToken, ccClientID, ccClientSecret)
	ts.Require().Equalf(http.StatusBadRequest, res.StatusCode, "introspection body: %s", string(res.Raw))
	ts.Equal("unauthorized_client", res.errorCode())
	ts.Equal("Client is not allowed to use the specified authentication method", res.Body["error_description"])
}

//
// Cases 16 and 17: the route surface.
//

// TestIntrospect_GetIsNotAllowed asserts only POST is routed, so a GET is rejected by the mux with
// 405 rather than reaching the handler.
func (ts *IntrospectionTestSuite) TestIntrospect_GetIsNotAllowed() {
	req, err := http.NewRequest(http.MethodGet, introspectEndpoint, nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusMethodNotAllowed, resp.StatusCode, "GET /oauth2/introspect must not be routed")
}

// TestIntrospect_OptionsReturnsNoContent asserts the CORS preflight route answers 204 without any
// client authentication, since the OPTIONS handler is registered outside the auth middleware.
func (ts *IntrospectionTestSuite) TestIntrospect_OptionsReturnsNoContent() {
	req, err := http.NewRequest(http.MethodOptions, introspectEndpoint, nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusNoContent, resp.StatusCode, "OPTIONS /oauth2/introspect must return 204")
}
