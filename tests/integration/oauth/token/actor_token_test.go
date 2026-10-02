// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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
	actorTokenIssuerClientID     = "actor_token_issuer_client"
	actorTokenIssuerClientSecret = "actor_token_issuer_secret"
	actorTokenExchangeClientID   = "actor_token_exchange_client"
	actorTokenExchangeSecret     = "actor_token_exchange_secret"
	actorTokenRedirectURI        = "https://localhost:3000"
	actorTokenSubjectUsername    = "actor_token_subject_user"
	actorTokenSubjectPassword    = "ActorTokenSubjectPass1!"
	actorTokenActorUsername      = "actor_token_actor_user"
	actorTokenActorPassword      = "ActorTokenActorPass1!"
)

var actorTokenUserType = testutils.UserType{
	Name: "actor-token-person",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
	},
}

var actorTokenAuthFlow = testutils.Flow{
	Name:     "Actor Token Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_actor_token_test",
	Nodes: []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "prompt_credentials",
		},
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

// ActorTokenTestSuite verifies RFC 8693 actor_token handling in the token-exchange grant: the "act"
// claim built for a valid actor_token, the actor_token/actor_token_type pairing requirement, rejection
// of an invalid, expired, or untrusted-issuer actor_token, and nested delegation chains (an actor_token
// that itself already carries an "act" claim from a prior exchange).
type ActorTokenTestSuite struct {
	suite.Suite
	client        *http.Client
	ouID          string
	entityTypeID  string
	authFlowID    string
	issuerAppID   string
	exchangeAppID string
	subjectUserID string
	actorUserID   string
}

// TestActorTokenTestSuite runs the ActorTokenTestSuite.
func TestActorTokenTestSuite(t *testing.T) {
	suite.Run(t, new(ActorTokenTestSuite))
}

// SetupSuite creates the shared organization unit, user type, auth flow, issuer/exchange
// applications, and the two test users (subject and actor) for the suite.
func (ts *ActorTokenTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "actor-token-test-ou",
		Name:        "Actor Token Test OU",
		Description: "Organization unit for actor_token integration tests",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID

	actorTokenUserType.OUID = ouID
	entityTypeID, err := testutils.CreateUserType(actorTokenUserType)
	ts.Require().NoError(err)
	ts.entityTypeID = entityTypeID

	flowID, err := testutils.CreateFlow(actorTokenAuthFlow)
	ts.Require().NoError(err)
	ts.authFlowID = flowID

	ts.issuerAppID = ts.createApplication(map[string]interface{}{
		"clientId":                actorTokenIssuerClientID,
		"clientSecret":            actorTokenIssuerClientSecret,
		"redirectUris":            []string{actorTokenRedirectURI},
		"grantTypes":              []string{"authorization_code"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "client_secret_basic",
	}, true)

	ts.exchangeAppID = ts.createApplication(map[string]interface{}{
		"clientId":                actorTokenExchangeClientID,
		"clientSecret":            actorTokenExchangeSecret,
		"grantTypes":              []string{"urn:ietf:params:oauth:grant-type:token-exchange"},
		"tokenEndpointAuthMethod": "client_secret_basic",
	}, false)

	ts.subjectUserID = ts.createUser(actorTokenSubjectUsername, actorTokenSubjectPassword)
	ts.actorUserID = ts.createUser(actorTokenActorUsername, actorTokenActorPassword)
}

// TearDownSuite deletes the resources created in SetupSuite.
func (ts *ActorTokenTestSuite) TearDownSuite() {
	if ts.subjectUserID != "" {
		_ = testutils.DeleteUser(ts.subjectUserID)
	}
	if ts.actorUserID != "" {
		_ = testutils.DeleteUser(ts.actorUserID)
	}
	if ts.exchangeAppID != "" {
		_ = testutils.DeleteApplication(ts.exchangeAppID)
	}
	if ts.issuerAppID != "" {
		_ = testutils.DeleteApplication(ts.issuerAppID)
	}
	if ts.authFlowID != "" {
		_ = testutils.DeleteFlow(ts.authFlowID)
	}
	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// createApplication creates an application with the given OAuth2 inbound config, optionally
// bound to the shared auth flow.
func (ts *ActorTokenTestSuite) createApplication(oauthConfig map[string]interface{}, withAuthFlow bool) string {
	app := map[string]interface{}{
		"name":                      "ActorTokenTestApp-" + oauthConfig["clientId"].(string),
		"description":               "Application for actor_token integration testing",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"actor-token-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type":   "oauth2",
				"config": oauthConfig,
			},
		},
	}
	if withAuthFlow {
		app["authFlowId"] = ts.authFlowID
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, string(bodyBytes))

	var respData map[string]interface{}
	ts.Require().NoError(json.Unmarshal(bodyBytes, &respData))
	return respData["id"].(string)
}

// createUser creates a test user of the shared actor-token-person type.
func (ts *ActorTokenTestSuite) createUser(username, password string) string {
	attributesJSON, err := json.Marshal(map[string]interface{}{
		"username": username,
		"password": password,
	})
	ts.Require().NoError(err)

	userID, err := testutils.CreateUser(testutils.User{
		OUID:       ts.ouID,
		Type:       "actor-token-person",
		Attributes: json.RawMessage(attributesJSON),
	})
	ts.Require().NoError(err)
	return userID
}

// obtainAccessTokenAs runs the full authorization_code flow against the given client and returns the
// resulting access token for the given user credentials.
func (ts *ActorTokenTestSuite) obtainAccessTokenAs(clientID, clientSecret, username, password string) string {
	resp, err := testutils.InitiateAuthorizationFlow(clientID, actorTokenRedirectURI, "code", "openid", "test-state")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location)

	authID, executionID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err)

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": username,
		"password": password,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)
	ts.Require().NotEmpty(flowStep.Assertion)

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err)

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)

	tokenResult, err := testutils.RequestToken(clientID, clientSecret, code, actorTokenRedirectURI, "authorization_code")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, string(tokenResult.Body))
	ts.Require().NotNil(tokenResult.Token)
	ts.Require().NotEmpty(tokenResult.Token.AccessToken)

	return tokenResult.Token.AccessToken
}

// doExchange submits a token-exchange grant request authenticated as the shared exchange app.
func (ts *ActorTokenTestSuite) doExchange(formData url.Values) (int, map[string]interface{}) {
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token", strings.NewReader(formData.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(actorTokenExchangeClientID, actorTokenExchangeSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var respBody map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().NoError(json.Unmarshal(bodyBytes, &respBody), "body: %s", string(bodyBytes))
	return resp.StatusCode, respBody
}

// craftJWTWithIssuer builds a JWT-shaped, unsigned token carrying the given sub/iss/exp. The
// signature segment is a dummy value: ValidateSubjectToken rejects an untrusted issuer before it
// ever verifies a signature, so this is sufficient to exercise that error path without access to a
// real signing key.
func craftJWTWithIssuer(sub, iss string, exp time.Time) string {
	header := map[string]interface{}{"alg": "RS256", "typ": "JWT"}
	claims := map[string]interface{}{
		"sub": sub,
		"iss": iss,
		"exp": exp.Unix(),
		"iat": time.Now().Unix(),
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	return headerB64 + "." + claimsB64 + "." + base64.RawURLEncoding.EncodeToString([]byte("dummy-signature"))
}

// exchangeParams builds the base token-exchange form parameters for the given subject token.
func exchangeParams(subjectToken string) url.Values {
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	form.Set("subject_token", subjectToken)
	form.Set("subject_token_type", "urn:ietf:params:oauth:token-type:access_token")
	return form
}

// TestActorToken_ValidActorAndSubject_ActClaimPresent verifies that a valid actor_token accompanying
// a valid subject_token produces an "act" claim naming the actor's subject and issuer.
func (ts *ActorTokenTestSuite) TestActorToken_ValidActorAndSubject_ActClaimPresent() {
	subjectToken := ts.obtainAccessTokenAs(
		actorTokenIssuerClientID, actorTokenIssuerClientSecret, actorTokenSubjectUsername, actorTokenSubjectPassword)
	actorToken := ts.obtainAccessTokenAs(
		actorTokenIssuerClientID, actorTokenIssuerClientSecret, actorTokenActorUsername, actorTokenActorPassword)

	actorClaims, err := testutils.DecodeJWT(actorToken)
	ts.Require().NoError(err)

	form := exchangeParams(subjectToken)
	form.Set("actor_token", actorToken)
	form.Set("actor_token_type", "urn:ietf:params:oauth:token-type:access_token")

	status, body := ts.doExchange(form)
	ts.Require().Equal(http.StatusOK, status, "%v", body)

	token, ok := body["access_token"].(string)
	ts.Require().True(ok, "Response should contain access_token")

	claims, err := testutils.DecodeJWT(token)
	ts.Require().NoError(err)
	ts.Equal(ts.subjectUserID, claims.Sub, "Subject should be the subject_token's own subject")

	act, ok := claims.Additional["act"].(map[string]interface{})
	ts.Require().True(ok, "Issued token should carry an act claim")
	ts.Equal(ts.actorUserID, act["sub"], "act.sub should identify the actor")
	ts.Equal(actorClaims.Iss, act["iss"], "act.iss should identify the actor token's issuer")
	ts.NotContains(act, "act", "act claim should not be nested when the actor_token carries no prior act claim")
}

// TestActorToken_MissingActorTokenType_InvalidRequest verifies that supplying actor_token without
// actor_token_type is rejected as invalid_request.
func (ts *ActorTokenTestSuite) TestActorToken_MissingActorTokenType_InvalidRequest() {
	subjectToken := ts.obtainAccessTokenAs(
		actorTokenIssuerClientID, actorTokenIssuerClientSecret, actorTokenSubjectUsername, actorTokenSubjectPassword)
	actorToken := ts.obtainAccessTokenAs(
		actorTokenIssuerClientID, actorTokenIssuerClientSecret, actorTokenActorUsername, actorTokenActorPassword)

	form := exchangeParams(subjectToken)
	form.Set("actor_token", actorToken)
	// actor_token_type intentionally omitted.

	status, body := ts.doExchange(form)
	ts.Require().Equal(http.StatusBadRequest, status, "%v", body)
	ts.Equal("invalid_request", body["error"])
	ts.Contains(body["error_description"], "actor_token_type is required")
}

// TestActorToken_TypeWithoutActorToken_InvalidRequest verifies that supplying actor_token_type
// without actor_token is rejected as invalid_request.
func (ts *ActorTokenTestSuite) TestActorToken_TypeWithoutActorToken_InvalidRequest() {
	subjectToken := ts.obtainAccessTokenAs(
		actorTokenIssuerClientID, actorTokenIssuerClientSecret, actorTokenSubjectUsername, actorTokenSubjectPassword)

	form := exchangeParams(subjectToken)
	form.Set("actor_token_type", "urn:ietf:params:oauth:token-type:access_token")
	// actor_token intentionally omitted.

	status, body := ts.doExchange(form)
	ts.Require().Equal(http.StatusBadRequest, status, "%v", body)
	ts.Equal("invalid_request", body["error"])
	ts.Contains(body["error_description"], "actor_token_type must not be provided without actor_token")
}

// TestActorToken_Malformed_InvalidRequest verifies that a syntactically invalid actor_token is
// rejected as invalid_request rather than crashing or being silently ignored.
func (ts *ActorTokenTestSuite) TestActorToken_Malformed_InvalidRequest() {
	subjectToken := ts.obtainAccessTokenAs(
		actorTokenIssuerClientID, actorTokenIssuerClientSecret, actorTokenSubjectUsername, actorTokenSubjectPassword)

	form := exchangeParams(subjectToken)
	form.Set("actor_token", "not-a-valid-jwt")
	form.Set("actor_token_type", "urn:ietf:params:oauth:token-type:access_token")

	status, body := ts.doExchange(form)
	ts.Require().Equal(http.StatusBadRequest, status, "%v", body)
	ts.Equal("invalid_request", body["error"])
	ts.Contains(body["error_description"], "Invalid actor_token")
}

// TestActorToken_UntrustedIssuer_InvalidRequest verifies that an actor_token issued by an issuer this
// server does not trust is rejected as invalid_request, attributing the failure to actor_token
// specifically (not subject_token).
func (ts *ActorTokenTestSuite) TestActorToken_UntrustedIssuer_InvalidRequest() {
	subjectToken := ts.obtainAccessTokenAs(
		actorTokenIssuerClientID, actorTokenIssuerClientSecret, actorTokenSubjectUsername, actorTokenSubjectPassword)
	untrustedActorToken := craftJWTWithIssuer(ts.actorUserID, "https://untrusted-issuer.example.com", time.Now().Add(time.Hour))

	form := exchangeParams(subjectToken)
	form.Set("actor_token", untrustedActorToken)
	form.Set("actor_token_type", "urn:ietf:params:oauth:token-type:access_token")

	status, body := ts.doExchange(form)
	ts.Require().Equal(http.StatusBadRequest, status, "%v", body)
	ts.Equal("invalid_request", body["error"])
	ts.Contains(body["error_description"], "actor_token issuer is not registered")
}

// currentJWTLeeway reads the live jwt.leeway value, falling back to the default.json default (30) if
// deployment.yaml does not currently override it.
func currentJWTLeeway() (int64, error) {
	raw, err := testutils.ReadDeploymentConfigKey("jwt")
	if err != nil {
		return 0, err
	}
	if m, ok := raw.(map[string]interface{}); ok {
		if v, ok := m["leeway"].(int); ok {
			return int64(v), nil
		}
	}
	return 30, nil
}

// setJWTLeeway safely overrides jwt.leeway, preserving every other jwt setting: PatchDeploymentConfig
// replaces a top-level key wholesale, so the full existing "jwt" section must be read and only the
// one nested field changed before writing it back.
func setJWTLeeway(seconds int64) error {
	raw, err := testutils.ReadDeploymentConfigKey("jwt")
	if err != nil {
		return err
	}
	jwtSection, _ := raw.(map[string]interface{})
	if jwtSection == nil {
		jwtSection = make(map[string]interface{})
	}
	jwtSection["leeway"] = seconds
	return testutils.PatchDeploymentConfig(map[string]interface{}{"jwt": jwtSection})
}

// TestActorToken_Expired_InvalidRequest verifies that an expired actor_token is rejected as
// invalid_request, distinct from the generic "Invalid actor_token" and untrusted-issuer errors.
//
// The server tolerates jwt.leeway beyond exp before treating a token as expired, so the wait to clear
// expiry must cover validityPeriod + leeway. The default leeway (30s) would make that wait long and
// keep it pinned to whatever the default happens to be, so this test temporarily shrinks leeway via a
// server restart, restoring the original value in a deferred cleanup regardless of outcome.
func (ts *ActorTokenTestSuite) TestActorToken_Expired_InvalidRequest() {
	const shortLeewaySeconds = int64(2)
	originalLeeway, err := currentJWTLeeway()
	ts.Require().NoError(err, "Failed to read current JWT leeway")
	ts.Require().NoError(setJWTLeeway(shortLeewaySeconds), "Failed to shrink JWT leeway")
	// Registered immediately after the config mutation succeeds, before the restart/token calls below
	// that could fail via Require and abort the test: otherwise a failure there would leave the
	// shrunk leeway in place for every test that runs afterward.
	defer func() {
		ts.Assert().NoError(setJWTLeeway(originalLeeway), "Failed to restore JWT leeway")
		ts.Assert().NoError(testutils.RestartServer(), "Failed to restart server after leeway restore")
		ts.Assert().NoError(testutils.ObtainAdminAccessToken(), "Failed to re-obtain admin token after restore")
	}()
	ts.Require().NoError(testutils.RestartServer(), "Failed to restart server with shrunk JWT leeway")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(), "Failed to re-obtain admin token after restart")

	subjectToken := ts.obtainAccessTokenAs(
		actorTokenIssuerClientID, actorTokenIssuerClientSecret, actorTokenSubjectUsername, actorTokenSubjectPassword)

	const shortValidityPeriodSeconds = 2
	shortLivedAppID := ts.createApplication(map[string]interface{}{
		"clientId":                "actor_token_short_lived_client",
		"clientSecret":            "actor_token_short_lived_secret",
		"redirectUris":            []string{actorTokenRedirectURI},
		"grantTypes":              []string{"authorization_code"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "client_secret_basic",
		"token": map[string]interface{}{
			"accessToken": map[string]interface{}{
				"userConfig": map[string]interface{}{
					"validityPeriod": shortValidityPeriodSeconds,
				},
			},
		},
	}, true)
	defer func() { _ = testutils.DeleteApplication(shortLivedAppID) }()

	expiringActorToken := ts.obtainAccessTokenAs(
		"actor_token_short_lived_client", "actor_token_short_lived_secret",
		actorTokenActorUsername, actorTokenActorPassword)

	// Wait must clear validityPeriod + the (now shrunk) leeway, plus a small buffer.
	time.Sleep(time.Duration(shortValidityPeriodSeconds+shortLeewaySeconds+1) * time.Second)

	form := exchangeParams(subjectToken)
	form.Set("actor_token", expiringActorToken)
	form.Set("actor_token_type", "urn:ietf:params:oauth:token-type:access_token")

	status, body := ts.doExchange(form)
	ts.Require().Equal(http.StatusBadRequest, status, "%v", body)
	ts.Equal("invalid_request", body["error"])
	ts.Contains(body["error_description"], "actor_token has expired")
}

// TestActorToken_NestedDelegationChain verifies that exchanging with an actor_token that itself
// already carries an "act" claim (from a prior exchange) produces a nested delegation chain: the new
// token's act claim names the immediate actor and preserves that actor's own act claim underneath it.
func (ts *ActorTokenTestSuite) TestActorToken_NestedDelegationChain() {
	subjectToken := ts.obtainAccessTokenAs(
		actorTokenIssuerClientID, actorTokenIssuerClientSecret, actorTokenSubjectUsername, actorTokenSubjectPassword)
	actorToken := ts.obtainAccessTokenAs(
		actorTokenIssuerClientID, actorTokenIssuerClientSecret, actorTokenActorUsername, actorTokenActorPassword)

	// First exchange: subject acted upon by actor. The resulting token (T1) becomes the actor_token
	// for the second exchange below.
	firstForm := exchangeParams(subjectToken)
	firstForm.Set("actor_token", actorToken)
	firstForm.Set("actor_token_type", "urn:ietf:params:oauth:token-type:access_token")

	status, body := ts.doExchange(firstForm)
	ts.Require().Equal(http.StatusOK, status, "%v", body)
	firstExchangedToken, ok := body["access_token"].(string)
	ts.Require().True(ok)

	firstClaims, err := testutils.DecodeJWT(firstExchangedToken)
	ts.Require().NoError(err)
	firstAct, ok := firstClaims.Additional["act"].(map[string]interface{})
	ts.Require().True(ok, "First exchange should produce an act claim")
	ts.Equal(ts.actorUserID, firstAct["sub"])

	// Second exchange: the same subject, now acted upon by the first exchange's own token, which
	// already carries an act claim of its own.
	secondForm := exchangeParams(subjectToken)
	secondForm.Set("actor_token", firstExchangedToken)
	secondForm.Set("actor_token_type", "urn:ietf:params:oauth:token-type:access_token")

	status, body = ts.doExchange(secondForm)
	ts.Require().Equal(http.StatusOK, status, "%v", body)
	secondExchangedToken, ok := body["access_token"].(string)
	ts.Require().True(ok)

	secondClaims, err := testutils.DecodeJWT(secondExchangedToken)
	ts.Require().NoError(err)

	secondAct, ok := secondClaims.Additional["act"].(map[string]interface{})
	ts.Require().True(ok, "Second exchange should produce an act claim")
	ts.Equal(ts.subjectUserID, secondAct["sub"], "act.sub should identify T1's own subject as the immediate actor")

	nestedAct, ok := secondAct["act"].(map[string]interface{})
	ts.Require().True(ok, "act claim should be nested, preserving T1's own act claim")
	ts.Equal(ts.actorUserID, nestedAct["sub"], "nested act.act.sub should identify the original actor")
}

// TestActorToken_RefreshTokenNotAnActorTokenType verifies that a refresh token is refused as an
// actor_token_type. A refresh token's sub names the OAuth client it was issued to rather than the
// party acting on the subject's behalf, so it cannot identify an actor.
func (ts *ActorTokenTestSuite) TestActorToken_RefreshTokenNotAnActorTokenType() {
	subjectToken := ts.obtainAccessTokenAs(
		actorTokenIssuerClientID, actorTokenIssuerClientSecret, actorTokenSubjectUsername, actorTokenSubjectPassword)
	actorToken := ts.obtainAccessTokenAs(
		actorTokenIssuerClientID, actorTokenIssuerClientSecret, actorTokenActorUsername, actorTokenActorPassword)

	form := exchangeParams(subjectToken)
	form.Set("actor_token", actorToken)
	form.Set("actor_token_type", "urn:ietf:params:oauth:token-type:refresh_token")

	status, body := ts.doExchange(form)
	ts.Require().Equal(http.StatusBadRequest, status, "%v", body)
	ts.Equal("invalid_request", body["error"])
	ts.Contains(body["error_description"], "is not supported as an actor_token_type")
}

// TestActorToken_IDJAGNotAnActorTokenType verifies that an ID-JAG is refused as an actor_token_type.
// It is an authorization grant addressed elsewhere, not a credential identifying an actor.
func (ts *ActorTokenTestSuite) TestActorToken_IDJAGNotAnActorTokenType() {
	subjectToken := ts.obtainAccessTokenAs(
		actorTokenIssuerClientID, actorTokenIssuerClientSecret, actorTokenSubjectUsername, actorTokenSubjectPassword)
	actorToken := ts.obtainAccessTokenAs(
		actorTokenIssuerClientID, actorTokenIssuerClientSecret, actorTokenActorUsername, actorTokenActorPassword)

	form := exchangeParams(subjectToken)
	form.Set("actor_token", actorToken)
	form.Set("actor_token_type", "urn:ietf:params:oauth:token-type:id-jag")

	status, body := ts.doExchange(form)
	ts.Require().Equal(http.StatusBadRequest, status, "%v", body)
	ts.Equal("invalid_request", body["error"])
	ts.Contains(body["error_description"], "is not supported as an actor_token_type")
}
