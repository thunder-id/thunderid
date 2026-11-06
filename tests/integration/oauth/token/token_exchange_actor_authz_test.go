// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	actorAuthzIssuerClientID   = "actor_authz_issuer_client"
	actorAuthzIssuerSecret     = "actor_authz_issuer_secret"
	actorAuthzAgentClientID    = "actor_authz_agent_client"
	actorAuthzAgentSecret      = "actor_authz_agent_secret"
	actorAuthzRSIdentifier     = "https://actor-authz.example.com"
	actorAuthzRedirectURI      = "https://localhost:3000"
	actorAuthzUserType         = "actor-authz-person"
	actorAuthzUsername         = "actor_authz_alice"
	actorAuthzPassword         = "ActorAuthzAlicePass1!"
	actorAuthzTokenExchangeGT  = "urn:ietf:params:oauth:grant-type:token-exchange"
	actorAuthzAccessTokenType  = "urn:ietf:params:oauth:token-type:access_token"
	actorAuthzAllScopes        = "read write transfer"
	actorAuthzAgentRoleScopes  = "read write"
	actorAuthzAgentTypeHandle  = "default"
	actorAuthzCustomerRoleName = "Actor Authz Customer"
	actorAuthzReaderRoleName   = "Actor Authz Orders Reader"
)

// actorAuthzFlow signs the user in and resolves the requested permission scopes against the user's roles.
var actorAuthzFlow = testutils.Flow{
	Name:     "Actor Authz Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_actor_authz_test",
	Nodes: []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
		{
			"id":   "prompt_credentials",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{{
				"inputs": []map[string]interface{}{
					{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
				"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
			}},
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
			"onSuccess": "authorization_check",
		},
		{
			"id":        "authorization_check",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthorizationExecutor"},
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

// ActorAuthzTestSuite verifies that a token exchange which names an actor never issues a permission
// the actor itself is not authorized for, even when the subject holds it (RFC 8693 delegation).
type ActorAuthzTestSuite struct {
	suite.Suite
	client       *http.Client
	ouID         string
	userTypeID   string
	flowID       string
	rsID         string
	issuerAppID  string
	userID       string
	agentID      string
	customerRole string
	readerRole   string
	agentSnap    *testutils.AgentTypeSnapshot
}

// TestActorAuthzTestSuite runs the ActorAuthzTestSuite.
func TestActorAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(ActorAuthzTestSuite))
}

// SetupSuite creates a resource server with read, write and transfer scopes, a user holding all three
// through a role, and an agent whose role allows only read and write.
func (ts *ActorAuthzTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "actor-authz-test-ou",
		Name:        "Actor Authz Test OU",
		Description: "Organization unit for token exchange actor authorization tests",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID

	userType := testutils.UserType{
		Handle:      actorAuthzUserType,
		DisplayName: "Actor Authz Person",
		OUID:        ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	}
	ts.userTypeID, err = testutils.CreateUserType(userType)
	ts.Require().NoError(err)

	ts.flowID, err = testutils.CreateFlow(actorAuthzFlow)
	ts.Require().NoError(err)

	ts.rsID, err = testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:       "Actor Authz API",
		Identifier: actorAuthzRSIdentifier,
		OUID:       ouID,
	}, []testutils.Action{
		{Name: "Read", Handle: "read"},
		{Name: "Write", Handle: "write"},
		{Name: "Transfer", Handle: "transfer"},
	})
	ts.Require().NoError(err)

	ts.issuerAppID = ts.createIssuerApp()

	attributes, err := json.Marshal(map[string]interface{}{
		"username": actorAuthzUsername,
		"password": actorAuthzPassword,
	})
	ts.Require().NoError(err)
	ts.userID, err = testutils.CreateUser(testutils.User{
		OUID: ouID, Type: actorAuthzUserType, Attributes: json.RawMessage(attributes),
	})
	ts.Require().NoError(err)

	// The shared default agent type is a singleton: point it at this suite's OU, and restore it on teardown.
	ts.agentSnap, err = testutils.SnapshotAgentType()
	ts.Require().NoError(err)
	_, err = testutils.CreateAgentType(testutils.UserType{
		Handle:      actorAuthzAgentTypeHandle,
		DisplayName: "Default",
		OUID:        ouID,
		Schema: map[string]interface{}{
			"name":        map[string]interface{}{"type": "string", "required": true, "unique": true},
			"description": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err)

	ts.agentID, err = testutils.CreateAgent(testutils.Agent{
		OUID:       ouID,
		Type:       actorAuthzAgentTypeHandle,
		Attributes: map[string]interface{}{"name": "Actor Authz Agent"},
		InboundAuthConfig: []testutils.AgentInboundAuthConfig{{
			Type: "oauth2",
			Config: &testutils.AgentOAuthConfig{
				ClientID:                actorAuthzAgentClientID,
				ClientSecret:            actorAuthzAgentSecret,
				GrantTypes:              []string{"client_credentials", actorAuthzTokenExchangeGT},
				TokenEndpointAuthMethod: "client_secret_basic",
			},
		}},
	})
	ts.Require().NoError(err)

	ts.customerRole, err = testutils.CreateRole(testutils.Role{
		Name: actorAuthzCustomerRoleName,
		OUID: ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.rsID, Permissions: []string{"read", "write", "transfer"}},
		},
		Assignments: []testutils.Assignment{{ID: ts.userID, Type: "user"}},
	})
	ts.Require().NoError(err)

	ts.readerRole, err = testutils.CreateRole(testutils.Role{
		Name: actorAuthzReaderRoleName,
		OUID: ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.rsID, Permissions: []string{"read", "write"}},
		},
		Assignments: []testutils.Assignment{{ID: ts.agentID, Type: "agent"}},
	})
	ts.Require().NoError(err)
}

// TearDownSuite deletes the resources created in SetupSuite.
func (ts *ActorAuthzTestSuite) TearDownSuite() {
	if ts.readerRole != "" {
		_ = testutils.DeleteRole(ts.readerRole)
	}
	if ts.customerRole != "" {
		_ = testutils.DeleteRole(ts.customerRole)
	}
	if ts.agentID != "" {
		_ = testutils.DeleteAgent(ts.agentID)
	}
	if ts.agentSnap != nil {
		_ = testutils.RestoreAgentType(ts.agentSnap)
	}
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}
	if ts.issuerAppID != "" {
		_ = testutils.DeleteApplication(ts.issuerAppID)
	}
	if ts.rsID != "" {
		_ = testutils.DeleteResourceServer(ts.rsID)
	}
	if ts.flowID != "" {
		_ = testutils.DeleteFlow(ts.flowID)
	}
	if ts.userTypeID != "" {
		_ = testutils.DeleteUserType(ts.userTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// createIssuerApp creates the browser-style application the user signs in to.
func (ts *ActorAuthzTestSuite) createIssuerApp() string {
	app := map[string]interface{}{
		"name":                      "Actor Authz Issuer App",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"isRegistrationFlowEnabled": false,
		"authFlowId":                ts.flowID,
		"allowedUserTypes":          []string{actorAuthzUserType},
		"inboundAuthConfig": []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                actorAuthzIssuerClientID,
				"clientSecret":            actorAuthzIssuerSecret,
				"redirectUris":            []string{actorAuthzRedirectURI},
				"grantTypes":              []string{"authorization_code"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	}
	data, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(data))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, string(body))

	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &created))
	return created["id"].(string)
}

// userAccessToken signs the user in and returns an access token bound to the resource server.
func (ts *ActorAuthzTestSuite) userAccessToken() string {
	resp, err := testutils.InitiateAuthorizationFlowWithResource(actorAuthzIssuerClientID, actorAuthzRedirectURI,
		"code", actorAuthzAllScopes, "state", actorAuthzRSIdentifier)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)

	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err)

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)
	step, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": actorAuthzUsername, "password": actorAuthzPassword,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", step.FlowStatus)

	authzResp, err := testutils.CompleteAuthorization(authID, step.Assertion)
	ts.Require().NoError(err)
	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)

	result, err := testutils.RequestTokenWithResource(actorAuthzIssuerClientID, actorAuthzIssuerSecret, code,
		actorAuthzRedirectURI, "authorization_code", actorAuthzRSIdentifier)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, result.StatusCode, string(result.Body))
	return result.Token.AccessToken
}

// postAgentToken posts a token request authenticated as the agent and returns its status and body.
func (ts *ActorAuthzTestSuite) postAgentToken(form url.Values) (int, map[string]interface{}) {
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(actorAuthzAgentClientID, actorAuthzAgentSecret)
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var parsed map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &parsed), "body: %s", string(body))
	return resp.StatusCode, parsed
}

// agentClientCredentialsToken returns the agent's own token, which doubles as the actor token.
func (ts *ActorAuthzTestSuite) agentClientCredentialsToken() (string, string) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", actorAuthzAllScopes)
	form.Set("resource", actorAuthzRSIdentifier)
	status, body := ts.postAgentToken(form)
	ts.Require().Equal(http.StatusOK, status, "%v", body)
	token, _ := body["access_token"].(string)
	scope, _ := body["scope"].(string)
	return token, scope
}

// TestExchangeWithActorToken_DropsScopesTheAgentLacks follows the reported scenario: the user holds
// read, write and transfer, the agent only read and write, so the exchanged token must not carry transfer.
func (ts *ActorAuthzTestSuite) TestExchangeWithActorToken_DropsScopesTheAgentLacks() {
	userToken := ts.userAccessToken()
	userClaims, err := testutils.DecodeJWT(userToken)
	ts.Require().NoError(err)
	ts.ElementsMatch(strings.Fields(actorAuthzAllScopes), strings.Fields(scopeClaim(userClaims)),
		"the user's own token should carry all three scopes")

	agentToken, agentScope := ts.agentClientCredentialsToken()
	ts.ElementsMatch(strings.Fields(actorAuthzAgentRoleScopes), strings.Fields(agentScope),
		"the agent's own token should be refused transfer")

	form := url.Values{}
	form.Set("grant_type", actorAuthzTokenExchangeGT)
	form.Set("subject_token", userToken)
	form.Set("subject_token_type", actorAuthzAccessTokenType)
	form.Set("actor_token", agentToken)
	form.Set("actor_token_type", actorAuthzAccessTokenType)
	form.Set("scope", actorAuthzAllScopes)
	form.Set("resource", actorAuthzRSIdentifier)
	status, body := ts.postAgentToken(form)
	ts.Require().Equal(http.StatusOK, status, "%v", body)

	exchanged, _ := body["access_token"].(string)
	claims, err := testutils.DecodeJWT(exchanged)
	ts.Require().NoError(err)
	ts.ElementsMatch(strings.Fields(actorAuthzAgentRoleScopes), strings.Fields(scopeClaim(claims)),
		"the exchanged token must carry only the scopes both the user and the agent hold")
	ts.NotContains(strings.Fields(scopeClaim(claims)), "transfer")
}

// TestExchangeWithoutActorToken_DropsScopesTheAgentLacks checks the same when the authenticated agent
// is recorded as the actor implicitly.
func (ts *ActorAuthzTestSuite) TestExchangeWithoutActorToken_DropsScopesTheAgentLacks() {
	userToken := ts.userAccessToken()

	form := url.Values{}
	form.Set("grant_type", actorAuthzTokenExchangeGT)
	form.Set("subject_token", userToken)
	form.Set("subject_token_type", actorAuthzAccessTokenType)
	form.Set("scope", actorAuthzAllScopes)
	form.Set("resource", actorAuthzRSIdentifier)
	status, body := ts.postAgentToken(form)
	ts.Require().Equal(http.StatusOK, status, "%v", body)

	exchanged, _ := body["access_token"].(string)
	claims, err := testutils.DecodeJWT(exchanged)
	ts.Require().NoError(err)
	ts.ElementsMatch(strings.Fields(actorAuthzAgentRoleScopes), strings.Fields(scopeClaim(claims)))
}

// scopeClaim returns the space-separated scope claim of a decoded access token.
func scopeClaim(claims *testutils.JWTClaims) string {
	scope, _ := claims.Additional["scope"].(string)
	return scope
}
