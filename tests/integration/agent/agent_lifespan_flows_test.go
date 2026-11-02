// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	tokenExchangeGrant  = "urn:ietf:params:oauth:grant-type:token-exchange"
	accessTokenType     = "urn:ietf:params:oauth:token-type:access_token"
	jwtTokenType        = "urn:ietf:params:oauth:token-type:jwt"
	lifespanExchangerID = "lifespan_exchanger_client"
)

// lifespanAgent creates an agent from spec and returns its ID and the spec, so a later update can
// resend the same fields with a new validUntil.
func (ts *AgentOAuthFlowsTestSuite) lifespanAgent(spec Agent) (string, Agent) {
	agentID, err := createAgent(spec)
	ts.Require().NoError(err)
	return agentID, spec
}

// shortenLifespan moves the agent's validUntil to a few seconds from now and returns it.
func (ts *AgentOAuthFlowsTestSuite) shortenLifespan(agentID string, spec Agent) time.Time {
	spec.ValidUntil = lifespanEnd(4 * time.Second)
	data, err := json.Marshal(spec)
	ts.Require().NoError(err)
	var body map[string]interface{}
	ts.Require().NoError(json.Unmarshal(data, &body))
	// Resending the secret rotates it, which ends the agent's refresh tokens; leaving it out keeps it.
	for _, entry := range body["inboundAuthConfig"].([]interface{}) {
		delete(entry.(map[string]interface{})["config"].(map[string]interface{}), "clientSecret")
	}
	resp, err := putAgentRaw(agentID, body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))
	_ = resp.Body.Close()
	end, err := time.Parse(time.RFC3339, *spec.ValidUntil)
	ts.Require().NoError(err)
	return end
}

// requireCapped checks that the JWT expires no later than end, and no more than two seconds before
// it, so the agent's lifespan rather than the configured validity set its exp.
func (ts *AgentOAuthFlowsTestSuite) requireCapped(token string, end time.Time, what string) *testutils.JWTClaims {
	claims, err := testutils.DecodeJWT(token)
	ts.Require().NoError(err, what)
	ts.LessOrEqual(int64(claims.Exp), end.Unix(), "%s outlives the agent", what)
	ts.GreaterOrEqual(int64(claims.Exp), end.Unix()-2, "%s is not capped at the agent's validUntil", what)
	return claims
}

// requireRefusedAfter polls the request until it stops succeeding and checks it is refused, no earlier
// than a second before end. In the final second the token builder refuses with invalid_grant; once end
// passes, an expired agent authenticating as the client is refused with invalid_client instead, and the
// poll may land in either phase.
func (ts *AgentOAuthFlowsTestSuite) requireRefusedAfter(end time.Time, request func() (int, string)) {
	status, body := request()
	for status == http.StatusOK && time.Now().Before(end.Add(5*time.Second)) {
		time.Sleep(500 * time.Millisecond)
		status, body = request()
	}
	switch status {
	case http.StatusBadRequest:
		ts.Contains(body, "invalid_grant")
	case http.StatusUnauthorized:
		ts.Contains(body, "invalid_client")
	default:
		ts.Failf("not refused", "status %d: %s", status, body)
	}
	ts.False(time.Now().Before(end.Add(-time.Second)), "refused more than a second before validUntil")
}

func ccAgentSpec(ouID, authFlowID, name, clientID string, validUntil *string) Agent {
	return Agent{
		OUID: ouID, Type: "default", Attributes: agentAttrs(name, nil), AuthFlowID: authFlowID,
		ValidUntil: validUntil,
		InboundAuthConfig: []InboundAuthConfig{{Type: "oauth2", Config: &OAuthAgentConfig{
			ClientID: clientID, ClientSecret: clientID + "_secret",
			GrantTypes: []string{"client_credentials"}, TokenEndpointAuthMethod: "client_secret_basic",
		}}},
	}
}

func (ts *AgentOAuthFlowsTestSuite) clientCredentialsToken(clientID string) (int, string, string) {
	result, err := testutils.RequestTokenWithResource(clientID, clientID+"_secret", "", "", "client_credentials",
		agentOAuthDefaultRSIdentifier)
	ts.Require().NoError(err)
	token := ""
	if result.Token != nil {
		token = result.Token.AccessToken
	}
	return result.StatusCode, string(result.Body), token
}

// userAssertion signs the suite's user in and returns the assertion, for use as a subject_token.
func (ts *AgentOAuthFlowsTestSuite) userAssertion() string {
	payload, err := json.Marshal(map[string]interface{}{
		"identifiers": map[string]interface{}{"username": oauthFlowsTestUsername},
		"credentials": map[string]interface{}{"password": oauthFlowsTestPassword},
	})
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/auth/credentials/authenticate",
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)
	var auth testutils.AuthenticationResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&auth))
	return auth.Assertion
}

// exchange sends a token exchange request authenticated as clientID.
func exchange(clientID string, form url.Values) (int, string, map[string]interface{}) {
	form.Set("grant_type", tokenExchangeGrant)
	form.Set("resource", agentOAuthDefaultRSIdentifier)
	req, _ := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientID+"_secret")
	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return 0, err.Error(), nil
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]interface{}
	_ = json.Unmarshal(raw, &body)
	return resp.StatusCode, string(raw), body
}

// exchangerAgent creates an unbounded agent that may run token exchange, so an exchange it makes is
// capped only by the other parties.
func (ts *AgentOAuthFlowsTestSuite) exchangerAgent() string {
	agentID, err := createAgent(Agent{
		OUID: ts.ouID, Type: "default", Attributes: agentAttrs("lifespan-exchanger-agent", nil),
		AuthFlowID: ts.authFlowID,
		InboundAuthConfig: []InboundAuthConfig{{Type: "oauth2", Config: &OAuthAgentConfig{
			ClientID: lifespanExchangerID, ClientSecret: lifespanExchangerID + "_secret",
			GrantTypes: []string{tokenExchangeGrant}, TokenEndpointAuthMethod: "client_secret_basic",
		}}},
	})
	ts.Require().NoError(err)
	return agentID
}

// M2M: the agent is both the client and the subject of a client_credentials token.
func (ts *AgentOAuthFlowsTestSuite) TestAgentLifespanFlows_M2MClientCredentials() {
	end := lifespanEnd(2 * time.Minute)
	agentID, spec := ts.lifespanAgent(ccAgentSpec(ts.ouID, ts.authFlowID, "lifespan-m2m-agent", "lifespan_m2m_client",
		end))
	defer func() { _ = deleteAgent(agentID) }()
	endTime, _ := time.Parse(time.RFC3339, *end)

	status, body, token := ts.clientCredentialsToken("lifespan_m2m_client")
	ts.Require().Equal(http.StatusOK, status, body)
	claims := ts.requireCapped(token, endTime, "client_credentials access token")
	ts.Equal(agentID, claims.Sub)

	shortEnd := ts.shortenLifespan(agentID, spec)
	ts.requireRefusedAfter(shortEnd, func() (int, string) {
		status, body, _ := ts.clientCredentialsToken("lifespan_m2m_client")
		return status, body
	})
}

// OBO through authorization_code: a user signs in to the agent, which is the client. The access,
// refresh and ID tokens are capped at the agent's validUntil, as is a refreshed token, and refresh
// is refused once the agent's lifespan ends.
func (ts *AgentOAuthFlowsTestSuite) TestAgentLifespanFlows_OBOAuthorizationCode() {
	end := lifespanEnd(2 * time.Minute)
	clientID := "lifespan_obo_code_client"
	agentID, spec := ts.lifespanAgent(Agent{
		OUID: ts.ouID, Type: "default", Attributes: agentAttrs("lifespan-obo-code-agent", nil),
		AuthFlowID: ts.authFlowID, AllowedUserTypes: []string{"agent-oauth-flow-person"}, ValidUntil: end,
		InboundAuthConfig: []InboundAuthConfig{{Type: "oauth2", Config: &OAuthAgentConfig{
			ClientID: clientID, ClientSecret: clientID + "_secret", RedirectURIs: []string{oauthFlowsRedirectURI},
			GrantTypes: []string{"authorization_code", "refresh_token"}, ResponseTypes: []string{"code"},
			TokenEndpointAuthMethod: "client_secret_post",
		}}},
	})
	defer func() { _ = deleteAgent(agentID) }()
	endTime, _ := time.Parse(time.RFC3339, *end)

	token, err := obtainAgentAccessTokenWithResource(clientID, oauthFlowsRedirectURI, "openid",
		oauthFlowsTestUsername, oauthFlowsTestPassword, false, clientID+"_secret", agentOAuthDefaultRSIdentifier)
	ts.Require().NoError(err)
	claims := ts.requireCapped(token.AccessToken, endTime, "authorization_code access token")
	ts.Equal(ts.userID, claims.Sub, "the user is the subject")
	act, _ := claims.Additional["act"].(map[string]interface{})
	ts.Equal(agentID, act["sub"], "the agent acts for the user")
	ts.Require().NotEmpty(token.IDToken)
	ts.requireCapped(token.IDToken, endTime, "ID token")
	ts.Require().NotEmpty(token.RefreshToken)
	ts.requireCapped(token.RefreshToken, endTime, "refresh token")

	refreshed, err := testutils.RefreshAccessTokenWithClientCredentialsInBody(clientID, clientID+"_secret",
		token.RefreshToken)
	ts.Require().NoError(err)
	ts.requireCapped(refreshed.AccessToken, endTime, "refreshed access token")

	shortEnd := ts.shortenLifespan(agentID, spec)
	refreshToken := refreshed.RefreshToken
	if refreshToken == "" {
		refreshToken = token.RefreshToken
	}
	ts.requireRefusedAfter(shortEnd, func() (int, string) {
		form := url.Values{
			"grant_type": {"refresh_token"}, "refresh_token": {refreshToken},
			"client_id": {clientID}, "client_secret": {clientID + "_secret"},
		}
		body, status, err := agentTokenRequest(form.Encode(), clientID, clientID+"_secret", true)
		ts.Require().NoError(err)
		if next, ok := body["refresh_token"].(string); ok && next != "" {
			refreshToken = next
		}
		raw, _ := json.Marshal(body)
		return status, string(raw)
	})
}

// OBO through token exchange: the agent is the client exchanging the user's assertion.
func (ts *AgentOAuthFlowsTestSuite) TestAgentLifespanFlows_OBOTokenExchangeAsClient() {
	end := lifespanEnd(2 * time.Minute)
	clientID := "lifespan_obo_te_client"
	agentID, spec := ts.lifespanAgent(Agent{
		OUID: ts.ouID, Type: "default", Attributes: agentAttrs("lifespan-obo-te-agent", nil),
		AuthFlowID: ts.authFlowID, ValidUntil: end,
		InboundAuthConfig: []InboundAuthConfig{{Type: "oauth2", Config: &OAuthAgentConfig{
			ClientID: clientID, ClientSecret: clientID + "_secret",
			GrantTypes: []string{tokenExchangeGrant}, TokenEndpointAuthMethod: "client_secret_basic",
		}}},
	})
	defer func() { _ = deleteAgent(agentID) }()
	endTime, _ := time.Parse(time.RFC3339, *end)

	request := func() (int, string, map[string]interface{}) {
		return exchange(clientID, url.Values{
			"subject_token": {ts.userAssertion()}, "subject_token_type": {jwtTokenType},
		})
	}
	status, raw, body := request()
	ts.Require().Equal(http.StatusOK, status, raw)
	claims := ts.requireCapped(body["access_token"].(string), endTime, "exchanged access token")
	ts.Equal(ts.userID, claims.Sub, "the user is the subject")

	shortEnd := ts.shortenLifespan(agentID, spec)
	ts.requireRefusedAfter(shortEnd, func() (int, string) {
		status, raw, _ := request()
		return status, raw
	})
}

// OBO with the agent as the actor: another client exchanges the user's assertion with the agent's
// own token as actor_token, so the agent appears only in act and still caps the token.
func (ts *AgentOAuthFlowsTestSuite) TestAgentLifespanFlows_OBOAgentAsActor() {
	end := lifespanEnd(2 * time.Minute)
	agentID, spec := ts.lifespanAgent(ccAgentSpec(ts.ouID, ts.authFlowID, "lifespan-actor-agent",
		"lifespan_actor_client", end))
	defer func() { _ = deleteAgent(agentID) }()
	exchangerID := ts.exchangerAgent()
	defer func() { _ = deleteAgent(exchangerID) }()
	endTime, _ := time.Parse(time.RFC3339, *end)

	status, raw, actorToken := ts.clientCredentialsToken("lifespan_actor_client")
	ts.Require().Equal(http.StatusOK, status, raw)

	request := func() (int, string, map[string]interface{}) {
		return exchange(lifespanExchangerID, url.Values{
			"subject_token": {ts.userAssertion()}, "subject_token_type": {jwtTokenType},
			"actor_token": {actorToken}, "actor_token_type": {accessTokenType},
		})
	}
	status, raw, body := request()
	ts.Require().Equal(http.StatusOK, status, raw)
	claims := ts.requireCapped(body["access_token"].(string), endTime, "delegated access token")
	ts.Equal(ts.userID, claims.Sub, "the user is the subject")
	act, ok := claims.Additional["act"].(map[string]interface{})
	ts.Require().True(ok, "the delegated token carries act")
	ts.Equal(agentID, act["sub"], "the agent is the actor")

	shortEnd := ts.shortenLifespan(agentID, spec)
	ts.requireRefusedAfter(shortEnd, func() (int, string) {
		status, raw, _ := request()
		return status, raw
	})
}

// The agent obtains a token as the subject: another client exchanges the agent's own access token,
// so the new token's sub is the agent.
func (ts *AgentOAuthFlowsTestSuite) TestAgentLifespanFlows_AgentAsSubjectOfExchange() {
	end := lifespanEnd(2 * time.Minute)
	agentID, spec := ts.lifespanAgent(ccAgentSpec(ts.ouID, ts.authFlowID, "lifespan-subject-agent",
		"lifespan_subject_client", end))
	defer func() { _ = deleteAgent(agentID) }()
	exchangerID := ts.exchangerAgent()
	defer func() { _ = deleteAgent(exchangerID) }()
	endTime, _ := time.Parse(time.RFC3339, *end)

	status, raw, subjectToken := ts.clientCredentialsToken("lifespan_subject_client")
	ts.Require().Equal(http.StatusOK, status, raw)

	request := func() (int, string, map[string]interface{}) {
		return exchange(lifespanExchangerID, url.Values{
			"subject_token": {subjectToken}, "subject_token_type": {accessTokenType},
		})
	}
	status, raw, body := request()
	ts.Require().Equal(http.StatusOK, status, raw)
	claims := ts.requireCapped(body["access_token"].(string), endTime, "exchanged agent token")
	ts.Equal(agentID, claims.Sub, "the agent is the subject")

	shortEnd := ts.shortenLifespan(agentID, spec)
	ts.requireRefusedAfter(shortEnd, func() (int, string) {
		status, raw, _ := request()
		return status, raw
	})
}

// A subject token issued while the agent had no end outlives the end set later. Exchanging it once
// that end has passed is refused by the token builder's issuance check, not by the subject token's own
// expiry, so a token minted before a reduction cannot be exchanged for a fresh one.
func (ts *AgentOAuthFlowsTestSuite) TestAgentLifespanFlows_ExpiredSubjectRefusedAtExchange() {
	agentID, spec := ts.lifespanAgent(ccAgentSpec(ts.ouID, ts.authFlowID, "lifespan-late-subject-agent",
		"lifespan_late_subject_client", nil))
	defer func() { _ = deleteAgent(agentID) }()
	exchangerID := ts.exchangerAgent()
	defer func() { _ = deleteAgent(exchangerID) }()

	status, raw, subjectToken := ts.clientCredentialsToken("lifespan_late_subject_client")
	ts.Require().Equal(http.StatusOK, status, raw)
	subjectClaims, err := testutils.DecodeJWT(subjectToken)
	ts.Require().NoError(err)

	end := ts.shortenLifespan(agentID, spec)
	ts.Require().Greater(int64(subjectClaims.Exp), end.Unix()+60,
		"the subject token must outlive the new end, or its own expiry would refuse the exchange")
	time.Sleep(time.Until(end) + time.Second)

	status, raw, body := exchange(lifespanExchangerID, url.Values{
		"subject_token": {subjectToken}, "subject_token_type": {accessTokenType},
	})
	ts.Equal(http.StatusBadRequest, status, raw)
	ts.Equal("invalid_grant", body["error"], raw)
	ts.Equal("The authorization grant can no longer be used to issue tokens", body["error_description"], raw)
}
