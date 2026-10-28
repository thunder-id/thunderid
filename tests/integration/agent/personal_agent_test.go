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
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	paPassword         = "Personal@Agent123"
	paRedirectURI      = "https://agent.example.com/callback"
	paPortalRedirect   = "http://localhost:3000/portal/callback"
	paRegistrationType = "REGISTRATION"
	paKind             = "personal-assistant"
	paSeededUserType   = "person"
	// paSuiteAgentName is the name the suite's agent is registered with.
	paSuiteAgentName = "pa_suite_assistant"
	// paCodeVerifier is a PKCE verifier for authorization requests that are not redeemed.
	paCodeVerifier = "pa-code-verifier-pa-code-verifier-pa-code-verifier"
	// paSuspensionCutoffLeeway is jwt.leeway as the integration deployment runs it. A suspension's revocation
	// cutoff is its time plus this allowance, so a code issued before then is still refused.
	paSuspensionCutoffLeeway = 30 * time.Second
	// paSignInAction submits the sign-in prompt of the personal agent login flow the agent authorizes through.
	paSignInAction       = "action_001"
	paConsentDecisions   = "consent_decisions"
	paConsentAllowAction = "consent_action_allow"
	paUnauthorizedClient = "unauthorized_client"
	paGrantRefusedCode   = "AGT-1048"
	paUpdateRefusedCode  = "AGT-1045"

	// The agent asks for a permission the owner holds, so the owner has something to consent to.
	paResourceIdentifier = "https://personal-agent-api.example.com"
	paPermission         = "read"
	paScope              = "openid " + paPermission
)

// PersonalAgentTestSuite registers a personal agent through the shipped registration flow and covers
// what it can do afterwards: only its owner signs in to it, consent is always asked, and its tokens carry
// the owner as the subject and the agent as the actor. An administrator can read it but not create or
// update one.
type PersonalAgentTestSuite struct {
	suite.Suite
	resourceServerID string
	roleID           string
	appID            string
	portalAppID      string
	portalClientID   string
	flows            *testutils.PersonalAgentRegistrationFlows
	ownerID          string
	ownerUsername    string
	otherID          string
	otherUsername    string
	agentID          string
	clientID         string
	clientSecret     string
}

func TestPersonalAgentTestSuite(t *testing.T) {
	suite.Run(t, new(PersonalAgentTestSuite))
}

func (ts *PersonalAgentTestSuite) SetupSuite() {
	person := ts.seededPersonType()
	ts.ownerID, ts.ownerUsername = ts.createUser(person, "pa_owner")
	ts.otherID, ts.otherUsername = ts.createUser(person, "pa_other")

	var err error
	ts.resourceServerID, err = testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Personal Agent API",
		Description: "the API the personal agent calls for its owner",
		Identifier:  paResourceIdentifier,
		OUID:        person.OUID,
	}, []testutils.Action{{Name: "Read", Handle: paPermission, Description: "Read access"}})
	ts.Require().NoError(err)
	ts.roleID, err = testutils.CreateRole(testutils.Role{
		Name:        "Personal Agent Owner Role",
		Description: "gives the owner the permission the agent asks for",
		OUID:        person.OUID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.resourceServerID, Permissions: []string{paPermission}},
		},
		Assignments: []testutils.Assignment{{ID: ts.ownerID, Type: "user"}},
	})
	ts.Require().NoError(err)

	ts.flows, err = testutils.CreatePersonalAgentRegistrationFlows("pa-agent-suite")
	ts.Require().NoError(err)
	ts.appID, err = testutils.CreateApplication(testutils.Application{
		Name:                      "pa_agent_suite_app",
		Description:               "registers the personal agent under test",
		OUID:                      person.OUID,
		AuthFlowID:                ts.flows.LoginFlowID,
		RegistrationFlowID:        ts.flows.RegistrationFlowID,
		IsRegistrationFlowEnabled: true,
		ClientID:                  "pa_agent_suite_app_client",
		ClientSecret:              "pa_agent_suite_app_secret",
	})
	ts.Require().NoError(err)

	ts.registerAgent()
}

func (ts *PersonalAgentTestSuite) TearDownSuite() {
	if ts.agentID != "" {
		if err := deleteAgent(ts.agentID); err != nil {
			ts.T().Logf("failed to delete agent %s: %v", ts.agentID, err)
		}
	}
	if ts.portalAppID != "" {
		if err := testutils.DeleteApplication(ts.portalAppID); err != nil {
			ts.T().Logf("failed to delete app %s: %v", ts.portalAppID, err)
		}
	}
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("failed to delete app %s: %v", ts.appID, err)
		}
	}
	if ts.flows != nil {
		if err := ts.flows.Delete(); err != nil {
			ts.T().Logf("failed to delete the registration flows: %v", err)
		}
	}
	if ts.roleID != "" {
		if err := testutils.DeleteRole(ts.roleID); err != nil {
			ts.T().Logf("failed to delete role %s: %v", ts.roleID, err)
		}
	}
	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Logf("failed to delete resource server %s: %v", ts.resourceServerID, err)
		}
	}
	for _, userID := range []string{ts.ownerID, ts.otherID} {
		if userID != "" {
			if err := testutils.DeleteUser(userID); err != nil {
				ts.T().Logf("failed to delete user %s: %v", userID, err)
			}
		}
	}
}

// --- administrator API boundaries ---

// An administrator can register a personal agent for a user through the agent API. The agent is owned by
// that user, carries the personal kind, and gets the same setup as a registered one: PKCE, the personal agent
// login flow when none is given, and consent on every authorization.
func (ts *PersonalAgentTestSuite) TestAdminCreatesAPersonalAgentForAUser() {
	request := ts.personalAgentRequest(ts.otherID, "authorization_code")
	config := request["inboundAuthConfig"].([]map[string]interface{})[0]["config"].(map[string]interface{})
	config["pkceRequired"] = false
	resp, err := doPost(testServerURL+agentBasePath, request)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body := readBody(resp)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, body)
	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(body), &created))
	defer func() { _ = deleteAgent(created.ID) }()

	agent, err := testutils.GetAgent(created.ID)
	ts.Require().NoError(err)
	ts.Equal(paKind, agent.Kind)
	ts.Equal(ts.otherID, agent.Owner)
	ts.Require().Len(agent.InboundAuthConfig, 1)
	ts.True(agent.InboundAuthConfig[0].Config.PKCERequired, "PKCE is required whatever the request said")
	personalLoginFlowID, err := testutils.GetFlowIDByHandle("default-personal-agent-auth-flow", "AUTHENTICATION")
	ts.Require().NoError(err)
	ts.Equal(personalLoginFlowID, agent.AuthFlowID)
	ts.Require().NotNil(agent.LoginConsent)
	ts.Require().NotNil(agent.LoginConsent.AlwaysPrompt)
	ts.True(*agent.LoginConsent.AlwaysPrompt)
}

// The personal rules still apply on the agent API: an owner is required, no grant beyond the personal
// ones is accepted, and an unknown kind is refused.
func (ts *PersonalAgentTestSuite) TestAdminCreateKeepsThePersonalRules() {
	for name, tc := range map[string]struct {
		owner  string
		grants []string
		kind   string
		code   string
	}{
		"NoOwner": {grants: []string{"authorization_code"}, code: "AGT-1039"},
		"OwnGrant": {owner: ts.otherID, grants: []string{"authorization_code", "client_credentials"},
			code: paGrantRefusedCode},
		"UnknownKind": {owner: ts.otherID, grants: []string{"authorization_code"}, kind: "personal_assistant",
			code: "AGT-1050"},
	} {
		ts.Run(name, func() {
			request := ts.personalAgentRequest(tc.owner, tc.grants...)
			if tc.kind != "" {
				request["kind"] = tc.kind
			}
			resp, err := doPost(testServerURL+agentBasePath, request)
			ts.Require().NoError(err)
			defer resp.Body.Close()
			body := readBody(resp)
			ts.Equal(http.StatusBadRequest, resp.StatusCode, body)
			ts.Contains(body, tc.code)
		})
	}
}

// An administrator cannot update a personal agent: it belongs to its owner.
func (ts *PersonalAgentTestSuite) TestAdminCannotUpdateAPersonalAgent() {
	resp, err := putAgentRaw(ts.agentID, map[string]interface{}{
		"ouId":        testOUIDForPersonalAgents(ts),
		"type":        "default",
		"description": "changed by an administrator",
		"attributes":  map[string]interface{}{"name": "pa_admin_renamed"},
	})
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body := readBody(resp)
	ts.Equal(http.StatusForbidden, resp.StatusCode, body)
	ts.Contains(body, paUpdateRefusedCode)
}

// An administrator reads the agent with its kind and owner, and the values collected at
// registration.
func (ts *PersonalAgentTestSuite) TestAdminReadsKindOwnerAndDetails() {
	agent, err := testutils.GetAgent(ts.agentID)
	ts.Require().NoError(err)
	ts.Equal(paKind, agent.Kind)
	ts.Equal(ts.ownerID, agent.Owner)
	ts.Equal("Helps with my calendar", agent.Description)
	attributes, ok := agent.Attributes.(map[string]interface{})
	ts.Require().True(ok)
	ts.Equal(paSuiteAgentName, attributes["name"])
}

// An owner lists and reads their personal agents with their own token, reading the same agent object an
// administrator reads. Another user, the agent's own delegated token, a request without a token, and an ordinary
// agent the owner also owns get nothing.
func (ts *PersonalAgentTestSuite) TestOwnerListsAndReadsTheirAgents() {
	ownerToken := ts.userToken(ts.ownerUsername)
	otherToken := ts.userToken(ts.otherUsername)

	status, body := ts.selfServiceGet("/users/me/agents", ownerToken)
	ts.Require().Equal(http.StatusOK, status, body)
	var list struct {
		TotalResults int                      `json:"totalResults"`
		Agents       []map[string]interface{} `json:"agents"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(body), &list))
	ids := make([]interface{}, 0, len(list.Agents))
	for _, agent := range list.Agents {
		ids = append(ids, agent["id"])
		ts.Equal(ts.ownerID, agent["owner"], "each listed agent is the caller's")
		ts.Equal("personal-assistant", agent["kind"])
	}
	ts.Contains(ids, ts.agentID)

	status, body = ts.selfServiceGet("/users/me/agents/"+ts.agentID, ownerToken)
	ts.Require().Equal(http.StatusOK, status, body)
	var agent struct {
		Kind              string `json:"kind"`
		Owner             string `json:"owner"`
		ClientID          string `json:"clientId"`
		InboundAuthConfig []struct {
			Config struct {
				GrantTypes              []string `json:"grantTypes"`
				TokenEndpointAuthMethod string   `json:"tokenEndpointAuthMethod"`
			} `json:"config"`
		} `json:"inboundAuthConfig"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(body), &agent))
	ts.Equal(paKind, agent.Kind)
	ts.Equal(ts.ownerID, agent.Owner)
	ts.Equal(ts.clientID, agent.ClientID)
	ts.Require().Len(agent.InboundAuthConfig, 1)
	ts.Equal([]string{"authorization_code"}, agent.InboundAuthConfig[0].Config.GrantTypes)
	ts.Equal("client_secret_basic", agent.InboundAuthConfig[0].Config.TokenEndpointAuthMethod)
	ts.NotContains(body, "clientSecret", "a read never returns the client secret")

	status, body = ts.selfServiceGet("/users/me/agents/"+ts.agentID, otherToken)
	ts.Equal(http.StatusForbidden, status, body)
	status, body = ts.selfServiceGet("/users/me/agents", otherToken)
	ts.Require().Equal(http.StatusOK, status, body)
	ts.NotContains(body, ts.agentID)

	delegated := ts.obtainToken(ts.ownerUsername).AccessToken
	status, body = ts.selfServiceGet("/users/me/agents", delegated)
	ts.Equal(http.StatusForbidden, status, body)
	ts.Contains(body, "AGT-1053")

	status, _ = ts.selfServiceGet("/users/me/agents", "")
	ts.Equal(http.StatusUnauthorized, status)

	ordinary := ts.personalAgentRequest(ts.ownerID)
	delete(ordinary, "kind")
	ordinary["inboundAuthConfig"] = []map[string]interface{}{{
		"type":   "oauth2",
		"config": map[string]interface{}{"grantTypes": []string{"client_credentials"}},
	}}
	resp, err := doPost(testServerURL+agentBasePath, ordinary)
	ts.Require().NoError(err)
	created := readBody(resp)
	resp.Body.Close()
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, created)
	var ordinaryAgent struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(created), &ordinaryAgent))
	defer func() { _ = deleteAgent(ordinaryAgent.ID) }()
	status, body = ts.selfServiceGet("/users/me/agents/"+ordinaryAgent.ID, ownerToken)
	ts.Equal(http.StatusForbidden, status, "an ordinary agent the owner owns is not self-service: %s", body)
}

// ownedAgentUpdate is the part of an owner's update response the tests read: the same object PUT /agents/{id}
// returns, with the OAuth settings under inboundAuthConfig.
type ownedAgentUpdate struct {
	Description       string `json:"description"`
	Owner             string `json:"owner"`
	InboundAuthConfig []struct {
		Config struct {
			GrantTypes   []string `json:"grantTypes"`
			RedirectURIs []string `json:"redirectUris"`
			ClientSecret string   `json:"clientSecret"`
		} `json:"config"`
	} `json:"inboundAuthConfig"`
}

// ownedAgentBody returns a full owner update of an agent named name.
func ownedAgentBody(name, description, grants, redirectURIs string) string {
	return `{"description":"` + description + `","attributes":{"name":"` + name + `"},` +
		`"grantTypes":` + grants + `,"redirectUris":` + redirectURIs + `}`
}

// The owner replaces the description, redirect URIs and grant types of their agent. A grant they remove is
// refused from then on, including for a refresh token issued before. A body missing a part, fields outside the
// editable set, grants outside the personal set, another user and a delegated token are all refused. An empty
// grant list removes every grant, so the agent can no longer authorize.
func (ts *PersonalAgentTestSuite) TestOwnerUpdatesTheirAgent() {
	ownerToken := ts.userToken(ts.ownerUsername)
	path := "/users/me/agents/" + ts.agentID
	original := `["` + paRedirectURI + `"]`
	defer func() {
		status, body := ts.selfServiceCall(http.MethodPut, path, ownerToken,
			ownedAgentBody(paSuiteAgentName, "Helps with my calendar", `["authorization_code"]`, original))
		ts.Equal(http.StatusOK, status, "restoring the agent: %s", body)
	}()

	status, body := ts.selfServiceCall(http.MethodPut, path, ownerToken, ownedAgentBody(paSuiteAgentName,
		"Plans my week", `["authorization_code","refresh_token"]`, `["`+paRedirectURI+`","https://agent.example.com/other"]`))
	ts.Require().Equal(http.StatusOK, status, body)
	var updated ownedAgentUpdate
	ts.Require().NoError(json.Unmarshal([]byte(body), &updated))
	ts.Equal("Plans my week", updated.Description)
	ts.Equal(ts.ownerID, updated.Owner)
	ts.Require().Len(updated.InboundAuthConfig, 1)
	ts.Equal([]string{"authorization_code", "refresh_token"}, updated.InboundAuthConfig[0].Config.GrantTypes)
	ts.Len(updated.InboundAuthConfig[0].Config.RedirectURIs, 2)
	ts.Empty(updated.InboundAuthConfig[0].Config.ClientSecret, "the owner's update issues no client secret")

	token := ts.obtainToken(ts.ownerUsername)
	ts.Require().NotEmpty(token.RefreshToken, "with the refresh token grant the agent gets a refresh token")

	status, body = ts.selfServiceCall(http.MethodPut, path, ownerToken,
		ownedAgentBody(paSuiteAgentName, "Plans my week", `["authorization_code"]`, original))
	ts.Require().Equal(http.StatusOK, status, body)
	status, body = ts.postToken(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {token.RefreshToken}})
	ts.NotEqual(http.StatusOK, status, "a removed grant is refused: %s", body)

	for _, refused := range []string{
		`{}`,
		`{"description":"x","attributes":{"name":"pa_suite_assistant"},"grantTypes":["authorization_code"]}`,
		`{"attributes":null,"grantTypes":["authorization_code"],"redirectUris":` + original + `}`,
		`{"authFlowId":"x","attributes":{"name":"pa_suite_assistant"},"grantTypes":["authorization_code"],` +
			`"redirectUris":` + original + `}`,
		ownedAgentBody(paSuiteAgentName, "x", `["client_credentials"]`, original),
		ownedAgentBody(paSuiteAgentName, "x", `["authorization_code"]`, `[]`),
	} {
		status, body = ts.selfServiceCall(http.MethodPut, path, ownerToken, refused)
		ts.Equal(http.StatusBadRequest, status, "%s: %s", refused, body)
	}

	valid := ownedAgentBody(paSuiteAgentName, "x", `["authorization_code"]`, original)
	status, _ = ts.selfServiceCall(http.MethodPut, path, ts.userToken(ts.otherUsername), valid)
	ts.Equal(http.StatusNotFound, status)
	status, _ = ts.selfServiceCall(http.MethodPut, path, ts.obtainToken(ts.ownerUsername).AccessToken, valid)
	ts.Equal(http.StatusForbidden, status)

	status, body = ts.selfServiceCall(http.MethodPut, path, ownerToken,
		ownedAgentBody(paSuiteAgentName, "x", `[]`, original))
	ts.Require().Equal(http.StatusOK, status, body)
	var cleared ownedAgentUpdate
	ts.Require().NoError(json.Unmarshal([]byte(body), &cleared))
	ts.Require().Len(cleared.InboundAuthConfig, 1)
	ts.Empty(cleared.InboundAuthConfig[0].Config.GrantTypes, "an empty list removes every grant")
	verifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err)
	resp, err := testutils.InitiateAuthorizationFlowWithPKCE(ts.clientID, paRedirectURI, "code", paScope,
		"pa-state", paResourceIdentifier, testutils.GenerateCodeChallenge(verifier), "S256")
	ts.Require().NoError(err)
	resp.Body.Close()
	ts.Contains(resp.Header.Get("Location"), paUnauthorizedClient, "without grants the agent cannot authorize")
}

// The owner deletes a personal agent; another user and the agent's own delegated token cannot.
func (ts *PersonalAgentTestSuite) TestOwnerDeletesTheirAgent() {
	request := ts.personalAgentRequest(ts.ownerID, "authorization_code")
	resp, err := doPost(testServerURL+agentBasePath, request)
	ts.Require().NoError(err)
	body := readBody(resp)
	resp.Body.Close()
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, body)
	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(body), &created))
	path := "/users/me/agents/" + created.ID

	status, _ := ts.selfServiceCall(http.MethodDelete, path, ts.userToken(ts.otherUsername), "")
	ts.Equal(http.StatusNotFound, status, "another user cannot delete it")
	status, _ = ts.selfServiceCall(http.MethodDelete, path, ts.obtainToken(ts.ownerUsername).AccessToken, "")
	ts.Equal(http.StatusForbidden, status, "a delegated token cannot delete it")

	ownerToken := ts.userToken(ts.ownerUsername)
	status, body = ts.selfServiceCall(http.MethodDelete, path, ownerToken, "")
	ts.Require().Equal(http.StatusNoContent, status, body)
	status, _ = ts.selfServiceGet(path, ownerToken)
	ts.Equal(http.StatusNotFound, status, "the agent is gone")
}

// --- one owner, several agents ---

// Two agents of one owner stay independent (AC11.4): changing one leaves the other's settings and credentials as
// they were, each agent's tokens name only itself as the actor, and one agent cannot redeem a code issued to the
// other.
func (ts *PersonalAgentTestSuite) TestTwoAgentsOfOneOwnerStayIndependent() {
	ownerToken := ts.userToken(ts.ownerUsername)
	second := ts.createOwnedAgent("authorization_code")
	defer func() { _ = deleteAgent(second.id) }()

	status, before := ts.selfServiceGet("/users/me/agents/"+ts.agentID, ownerToken)
	ts.Require().Equal(http.StatusOK, status, before)
	status, body := ts.selfServiceCall(http.MethodPut, "/users/me/agents/"+second.id, ownerToken,
		ownedAgentBody(second.name, "Books my travel", `["authorization_code","refresh_token"]`,
			`["https://agent.example.com/other","`+paRedirectURI+`"]`))
	ts.Require().Equal(http.StatusOK, status, body)
	status, after := ts.selfServiceGet("/users/me/agents/"+ts.agentID, ownerToken)
	ts.Require().Equal(http.StatusOK, status, after)
	ts.JSONEq(before, after, "the other agent's settings are unchanged")

	firstToken := ts.obtainToken(ts.ownerUsername)
	restore := ts.actAs(second)
	secondToken := ts.obtainToken(ts.ownerUsername)
	restore()
	for agentID, token := range map[string]string{
		ts.agentID: firstToken.AccessToken, second.id: secondToken.AccessToken,
	} {
		claims, err := testutils.DecodeJWTPayloadMap(token)
		ts.Require().NoError(err)
		act, ok := claims["act"].(map[string]any)
		ts.Require().True(ok, "the token names its agent as the actor: %v", claims)
		ts.Equal(agentID, act["sub"])
	}

	code, verifier := ts.obtainCode(ts.ownerUsername)
	restore = ts.actAs(second)
	status, body = ts.exchangeCode(code, verifier)
	restore()
	ts.Equal(http.StatusBadRequest, status, "a code issued to one agent is refused for the other: %s", body)
	status, body = ts.exchangeCode(code, verifier)
	ts.Equal(http.StatusOK, status, "the code still belongs to the agent it was issued to: %s", body)
}

// Acting on one agent leaves the owner's other agents and the owner's own session working (AC11.5): after the
// owner changes and deletes one agent, a refresh token of another agent still works, and so does the owner's
// own token.
func (ts *PersonalAgentTestSuite) TestActingOnOneAgentLeavesTheOthersWorking() {
	ownerToken := ts.userToken(ts.ownerUsername)
	acted := ts.createOwnedAgent("authorization_code", "refresh_token")
	defer func() { _ = deleteAgent(acted.id) }()
	kept := ts.createOwnedAgent("authorization_code", "refresh_token")
	defer func() { _ = deleteAgent(kept.id) }()

	restore := ts.actAs(kept)
	defer restore()
	keptToken := ts.obtainToken(ts.ownerUsername)
	ts.Require().NotEmpty(keptToken.RefreshToken)

	path := "/users/me/agents/" + acted.id
	status, body := ts.selfServiceCall(http.MethodPut, path, ownerToken,
		ownedAgentBody(acted.name, "Changed", `["authorization_code"]`, `["`+paRedirectURI+`"]`))
	ts.Require().Equal(http.StatusOK, status, body)
	status, body = ts.selfServiceCall(http.MethodDelete, path, ownerToken, "")
	ts.Require().Equal(http.StatusNoContent, status, body)

	status, body = ts.postToken(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {keptToken.RefreshToken}})
	ts.Equal(http.StatusOK, status, "the other agent's refresh token still works: %s", body)
	status, body = ts.selfServiceGet("/users/me/agents/"+kept.id, ownerToken)
	ts.Equal(http.StatusOK, status, "the owner's own token still works: %s", body)
}

// --- owner suspension ---

// introspectAsSuiteApp reports whether a token is still active, asked by the suite's own application rather
// than the agent, whose client is refused while it is suspended.
func (ts *PersonalAgentTestSuite) introspectAsSuiteApp(token string) bool {
	ts.T().Helper()
	active, err := testutils.IntrospectToken(token, "pa_agent_suite_app_client", "pa_agent_suite_app_secret")
	ts.Require().NoError(err)
	return active
}

// signInStarts reports whether an authorization request by the agent with clientID starts the owner's sign-in.
// A suspended agent is refused before that.
func (ts *PersonalAgentTestSuite) signInStarts(clientID string) bool {
	ts.T().Helper()
	resp, err := testutils.InitiateAuthorizationFlowWithPKCE(clientID, paRedirectURI, "code", paScope,
		"pa-state", paResourceIdentifier, testutils.GenerateCodeChallenge(paCodeVerifier), "S256")
	ts.Require().NoError(err)
	resp.Body.Close()
	return strings.Contains(resp.Header.Get("Location"), "executionId")
}

// The owner suspends their agent: its tokens fail, it cannot sign the owner in or refresh, and its client is
// refused. Lifting the suspension lets it sign the owner in again with the same credentials, and get tokens
// once the suspension's revocation cutoff (now plus the JWT leeway) has passed. Repeating either changes
// nothing.
func (ts *PersonalAgentTestSuite) TestOwnerSuspendsTheirAgent() {
	ownerToken := ts.userToken(ts.ownerUsername)
	agent := ts.createOwnedAgent("authorization_code", "refresh_token")
	defer func() { _ = deleteAgent(agent.id) }()
	restore := ts.actAs(agent)
	defer restore()
	token := ts.obtainToken(ts.ownerUsername)
	ts.Require().NotEmpty(token.RefreshToken)
	path := "/users/me/agents/" + agent.id

	for range 2 {
		status, body := ts.selfServiceCall(http.MethodPost, path+"/suspend", ownerToken, "")
		ts.Require().Equal(http.StatusNoContent, status, body)
	}

	ts.False(ts.introspectAsSuiteApp(token.AccessToken), "the agent's access token no longer works")
	status, body := ts.postToken(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {token.RefreshToken}})
	ts.NotEqual(http.StatusOK, status, "a suspended agent cannot refresh: %s", body)
	ts.False(ts.signInStarts(agent.clientID), "a suspended agent cannot sign its owner in")

	for range 2 {
		status, body = ts.selfServiceCall(http.MethodPost, path+"/unsuspend", ownerToken, "")
		ts.Require().Equal(http.StatusNoContent, status, body)
	}
	ts.True(ts.signInStarts(agent.clientID), "the agent signs its owner in again")
	time.Sleep(paSuspensionCutoffLeeway + time.Second)
	renewed := ts.obtainToken(ts.ownerUsername)
	ts.True(ts.introspectAsSuiteApp(renewed.AccessToken), "the agent works again with the same credentials")
}

// The owner suspends all their agents at once and is told how many; each is refused. Agents already suspended by
// the owner are not counted again. A separate owner keeps the suite's agent out of it.
func (ts *PersonalAgentTestSuite) TestOwnerSuspendsAllTheirAgents() {
	ownerID, username := ts.createUser(ts.seededPersonType(), "pa_bulk_owner")
	agents := []ownedAgent{ts.createAgentFor(ownerID, "authorization_code"),
		ts.createAgentFor(ownerID, "authorization_code")}
	defer func() {
		for _, agent := range agents {
			_ = deleteAgent(agent.id)
		}
		_ = testutils.DeleteUser(ownerID)
	}()
	ownerToken := ts.userToken(username)

	status, body := ts.selfServiceCall(http.MethodPost, "/users/me/agents/suspend", ownerToken, "")
	ts.Require().Equal(http.StatusOK, status, body)
	ts.JSONEq(`{"suspendedCount":2}`, body)
	for _, agent := range agents {
		ts.False(ts.signInStarts(agent.clientID), "every agent of the owner is suspended")
	}
	ts.True(ts.signInStarts(ts.clientID), "another owner's agent is untouched")

	status, body = ts.selfServiceCall(http.MethodPost, "/users/me/agents/suspend", ownerToken, "")
	ts.Require().Equal(http.StatusOK, status, body)
	ts.JSONEq(`{"suspendedCount":0}`, body, "agents already suspended by the owner are not counted again")
}

// runAdministrationFlow runs a shipped administration flow on subject as the administrator, with an optional
// operator note, and requires it to complete.
func (ts *PersonalAgentTestSuite) runAdministrationFlow(handle, subject, note string) {
	ts.T().Helper()
	flowID, err := testutils.GetFlowIDByHandle(handle, "ADMINISTRATION")
	ts.Require().NoError(err)
	token, err := testutils.GetAccessToken()
	ts.Require().NoError(err)
	inputs := map[string]string{"subject": subject}
	if note != "" {
		inputs["operatorNote"] = note
	}
	status, raw, err := testutils.ExecuteFlowRaw(nil, token, map[string]interface{}{"flowId": flowID, "inputs": inputs})
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var step struct {
		FlowStatus string `json:"flowStatus"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &step))
	ts.Require().Equal("COMPLETE", step.FlowStatus, string(raw))
}

// ownedAgentStatus is the status on the owner's read of an agent.
type ownedAgentStatus struct {
	Value   string `json:"value"`
	Details *struct {
		Reason       string   `json:"reason"`
		Since        string   `json:"since"`
		AppliedBy    []string `json:"appliedBy"`
		OperatorNote string   `json:"operatorNote"`
	} `json:"details"`
}

// ownerReadsStatus returns the status the owner reads for agentID, and the raw read.
func (ts *PersonalAgentTestSuite) ownerReadsStatus(ownerToken, agentID string) (ownedAgentStatus, string) {
	ts.T().Helper()
	status, body := ts.selfServiceGet("/users/me/agents/"+agentID, ownerToken)
	ts.Require().Equal(http.StatusOK, status, body)
	var read struct {
		Status ownedAgentStatus `json:"status"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(body), &read))
	return read.Status, body
}

// The organization suspends an owner's agent through governance's flow: the agent is refused and its tokens fail,
// the owner sees who suspended it, when and the operator's note, and cannot lift it. The owner's own
// suspension is held apart: lifting either leaves the other in force, and the agent is back once both are lifted.
// The owner's own token and their other agents are untouched.
func (ts *PersonalAgentTestSuite) TestOrganizationSuspensionOfAPersonalAgent() {
	ownerToken := ts.userToken(ts.ownerUsername)
	agent := ts.createOwnedAgent("authorization_code")
	defer func() { _ = deleteAgent(agent.id) }()
	restore := ts.actAs(agent)
	token := ts.obtainToken(ts.ownerUsername)
	restore()
	path := "/users/me/agents/" + agent.id

	ts.runAdministrationFlow("default-agent-suspend-flow", agent.id, "Security advisory for this agent")
	ts.False(ts.introspectAsSuiteApp(token.AccessToken), "the suspended agent's tokens fail")
	ts.False(ts.signInStarts(agent.clientID), "the suspended agent cannot sign its owner in")
	ts.True(ts.signInStarts(ts.clientID), "the owner's other agents keep working")
	status, body := ts.selfServiceGet("/users/me/agents", ownerToken)
	ts.Equal(http.StatusOK, status, "the owner's own token keeps working: %s", body)

	read, _ := ts.ownerReadsStatus(ownerToken, agent.id)
	ts.Equal("SUSPENDED", read.Value)
	ts.Require().NotNil(read.Details)
	ts.Equal("ADMINISTRATIVE_SUSPENSION", read.Details.Reason)
	ts.Equal([]string{"organization"}, read.Details.AppliedBy)
	ts.NotEmpty(read.Details.Since)
	ts.Equal("Security advisory for this agent", read.Details.OperatorNote, "the owner reads the operator's note")
	ts.Contains(body, agent.id, "the owner's list carries the agent")
	adminRead, err := testutils.GetAccessStatus("agents", agent.id)
	ts.Require().NoError(err)
	ts.Equal("Security advisory for this agent", adminRead.Details.OperatorNote, "administrators see the note")

	status, body = ts.selfServiceCall(http.MethodPost, path+"/unsuspend", ownerToken, "")
	ts.Require().Equal(http.StatusNoContent, status, body)
	read, _ = ts.ownerReadsStatus(ownerToken, agent.id)
	ts.Equal("SUSPENDED", read.Value, "the owner cannot lift the organization's suspension")

	status, body = ts.selfServiceCall(http.MethodPost, path+"/suspend", ownerToken, "")
	ts.Require().Equal(http.StatusNoContent, status, body)
	read, _ = ts.ownerReadsStatus(ownerToken, agent.id)
	ts.Equal([]string{"organization", "owner"}, read.Details.AppliedBy, "both suspensions are held")

	ts.runAdministrationFlow("default-agent-unsuspend-flow", agent.id, "")
	read, _ = ts.ownerReadsStatus(ownerToken, agent.id)
	ts.Equal("SUSPENDED", read.Value, "the owner's suspension outlasts the organization's")
	ts.Equal("OWNER_SUSPENSION", read.Details.Reason)
	ts.Equal([]string{"owner"}, read.Details.AppliedBy)
	ts.False(ts.signInStarts(agent.clientID), "still suspended by the owner")

	status, body = ts.selfServiceCall(http.MethodPost, path+"/unsuspend", ownerToken, "")
	ts.Require().Equal(http.StatusNoContent, status, body)
	read, _ = ts.ownerReadsStatus(ownerToken, agent.id)
	ts.Equal("ACTIVE", read.Value)
	ts.Nil(read.Details)
	ts.True(ts.signInStarts(agent.clientID), "the agent is back once both suspensions are lifted")
}

// --- owner deletion ---

// ownerWithAgent creates an owner holding the suite's role, gives them a personal agent, and signs them in to it,
// returning the owner, the agent and the agent's token for them.
func (ts *PersonalAgentTestSuite) ownerWithAgent(prefix string) (string, ownedAgent, *testutils.TokenResponse) {
	ts.T().Helper()
	ownerID, username := ts.createUser(ts.seededPersonType(), prefix)
	resp, err := doPost(testServerURL+"/roles/"+ts.roleID+"/assignments/add", map[string]interface{}{
		"assignments": []map[string]string{{"id": ownerID, "type": "user"}},
	})
	ts.Require().NoError(err)
	body := readBody(resp)
	resp.Body.Close()
	ts.Require().Equal(http.StatusNoContent, resp.StatusCode, body)

	agent := ts.createAgentFor(ownerID, "authorization_code")
	restore := ts.actAs(agent)
	defer restore()
	return ownerID, agent, ts.obtainToken(username)
}

// agentExists reports whether an administrator can still read the agent.
func (ts *PersonalAgentTestSuite) agentExists(agentID string) bool {
	ts.T().Helper()
	resp, err := doGet(testServerURL + agentBasePath + "/" + agentID)
	ts.Require().NoError(err)
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Deleting an owner deletes their personal agents, on the REST path and through the deletion flow, after
// revoking the agents' tokens.
func (ts *PersonalAgentTestSuite) TestDeletingAnOwnerDeletesTheirPersonalAgents() {
	for name, deleteOwner := range map[string]func(ownerID string){
		"through the API": func(ownerID string) { ts.Require().NoError(testutils.DeleteUser(ownerID)) },
		"through the deletion flow": func(ownerID string) {
			ts.runAdministrationFlow("default-user-deletion-flow", ownerID, "")
		},
	} {
		ts.Run(name, func() {
			ownerID, agent, token := ts.ownerWithAgent("pa_deleted_owner")
			defer func() { _ = deleteAgent(agent.id) }()
			ts.Require().True(ts.introspectAsSuiteApp(token.AccessToken), "the token works before the deletion")

			deleteOwner(ownerID)

			ts.False(ts.agentExists(agent.id), "the owner's personal agent is deleted with them")
			ts.False(ts.introspectAsSuiteApp(token.AccessToken), "the deleted agent's token no longer works")
		})
	}
}

// createOrdinaryAgentFor creates an agent owned by ownerID that is not a personal agent, and returns its ID.
func (ts *PersonalAgentTestSuite) createOrdinaryAgentFor(ownerID string) string {
	ts.T().Helper()
	ordinary := ts.personalAgentRequest(ownerID)
	delete(ordinary, "kind")
	ordinary["inboundAuthConfig"] = []map[string]interface{}{{
		"type":   "oauth2",
		"config": map[string]interface{}{"grantTypes": []string{"client_credentials"}},
	}}
	resp, err := doPost(testServerURL+agentBasePath, ordinary)
	ts.Require().NoError(err)
	body := readBody(resp)
	resp.Body.Close()
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, body)
	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(body), &created))
	return created.ID
}

// An agent that is not a personal agent outlives its owner, so it still blocks the owner's deletion.
func (ts *PersonalAgentTestSuite) TestAnOrdinaryAgentStillBlocksItsOwnersDeletion() {
	ownerID, _ := ts.createUser(ts.seededPersonType(), "pa_ordinary_owner")
	ordinaryID := ts.createOrdinaryAgentFor(ownerID)
	defer func() {
		_ = deleteAgent(ordinaryID)
		_ = testutils.DeleteUser(ownerID)
	}()

	err := testutils.DeleteUser(ownerID)
	ts.Require().Error(err, "the owner of an ordinary agent cannot be deleted")
	ts.Contains(err.Error(), "USR-1027")
	ts.True(ts.agentExists(ordinaryID), "the ordinary agent is kept")
}

// Another user cannot suspend the owner's agent, and neither can a token the agent holds for its owner.
func (ts *PersonalAgentTestSuite) TestOwnerSuspensionGuards() {
	path := "/users/me/agents/" + ts.agentID
	for _, action := range []string{"/suspend", "/unsuspend"} {
		status, body := ts.selfServiceCall(http.MethodPost, path+action, ts.userToken(ts.otherUsername), "")
		ts.Equal(http.StatusNotFound, status, "%s by another user: %s", action, body)
		status, body = ts.selfServiceCall(http.MethodPost, path+action, ts.obtainToken(ts.ownerUsername).AccessToken,
			"")
		ts.Equal(http.StatusForbidden, status, "%s with a delegated token: %s", action, body)
	}
	status, body := ts.selfServiceCall(http.MethodPost, "/users/me/agents/suspend", "", "")
	ts.Equal(http.StatusUnauthorized, status, body)
}

// --- signing the owner in ---

// The owner signs in, is always asked for consent, and the token names the owner as the subject and
// the agent as the actor.
func (ts *PersonalAgentTestSuite) TestOwnerGetsADelegatedToken() {
	token := ts.obtainToken(ts.ownerUsername)

	claims, err := testutils.DecodeJWTPayloadMap(token.AccessToken)
	ts.Require().NoError(err)
	ts.Equal(ts.ownerID, claims["sub"], "the subject is the owner")
	act, ok := claims["act"].(map[string]any)
	ts.Require().True(ok, "the token names the agent as the actor: %v", claims)
	ts.Equal(ts.agentID, act["sub"])
	ts.Contains(strings.Fields(token.Scope), paPermission, "the token holds the approved permission")
}

// Consent is asked on every sign-in, even after the owner approved it before.
func (ts *PersonalAgentTestSuite) TestConsentIsAskedEveryTime() {
	ts.obtainToken(ts.ownerUsername)

	verifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err)
	_, step := ts.signIn(ts.ownerUsername, testutils.GenerateCodeChallenge(verifier))
	ts.NotEqual("COMPLETE", step.FlowStatus, "consent is asked again")
	ts.Require().NotNil(step.Data)
	ts.Contains(step.Data.AdditionalData, "consentPrompt")
}

// A user other than the owner cannot sign in to the agent: even with correct credentials they are asked to
// sign in again, and no assertion is issued.
func (ts *PersonalAgentTestSuite) TestAnotherUserIsRefused() {
	verifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err)
	_, executionID := ts.authorize(testutils.GenerateCodeChallenge(verifier))
	initial, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)

	step, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": ts.otherUsername, "password": paPassword,
	}, paSignInAction, initial.ChallengeToken)
	ts.Require().NoError(err)
	ts.Equal("INCOMPLETE", step.FlowStatus, "the sign-in is asked for again")
	ts.Empty(step.Assertion, "no assertion is issued for another user")
}

// A personal agent has no grants of its own (client credentials, token exchange): it only acts for its owner.
func (ts *PersonalAgentTestSuite) TestNoGrantsOfItsOwn() {
	for _, form := range []url.Values{
		{"grant_type": {"client_credentials"}},
		{
			"grant_type":         {"urn:ietf:params:oauth:grant-type:token-exchange"},
			"subject_token":      {"token"},
			"subject_token_type": {"urn:ietf:params:oauth:token-type:access_token"},
		},
	} {
		status, body := ts.postToken(form)
		ts.Equal(http.StatusBadRequest, status, body)
		ts.Contains(body, paUnauthorizedClient, "grant %s", form.Get("grant_type"))
	}
}

// --- helpers ---

// personalAgentRequest is an agent API create request for a personal agent owned by owner.
func (ts *PersonalAgentTestSuite) personalAgentRequest(owner string, grants ...string) map[string]interface{} {
	request := map[string]interface{}{
		"ouId":       testOUIDForPersonalAgents(ts),
		"type":       "default",
		"kind":       paKind,
		"attributes": map[string]interface{}{"name": common.GenerateUniqueUsername("pa_admin_created")},
		"inboundAuthConfig": []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"grantTypes":    grants,
				"responseTypes": []string{"code"},
				"redirectUris":  []string{paRedirectURI},
				"pkceRequired":  true,
			},
		}},
	}
	if owner != "" {
		request["owner"] = owner
	}
	return request
}

// ownedAgent is a personal agent of the suite's owner created through the agent API, with its client credentials.
type ownedAgent struct {
	id           string
	name         string
	clientID     string
	clientSecret string
}

// createOwnedAgent creates another personal agent for the suite's owner with the given grants.
func (ts *PersonalAgentTestSuite) createOwnedAgent(grants ...string) ownedAgent {
	ts.T().Helper()
	return ts.createAgentFor(ts.ownerID, grants...)
}

// createAgentFor creates a personal agent for owner with the given grants.
func (ts *PersonalAgentTestSuite) createAgentFor(owner string, grants ...string) ownedAgent {
	ts.T().Helper()
	request := ts.personalAgentRequest(owner, grants...)
	resp, err := doPost(testServerURL+agentBasePath, request)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body := readBody(resp)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, body)
	var created struct {
		ID                string `json:"id"`
		InboundAuthConfig []struct {
			Config struct {
				ClientID     string `json:"clientId"`
				ClientSecret string `json:"clientSecret"`
			} `json:"config"`
		} `json:"inboundAuthConfig"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(body), &created))
	ts.Require().Len(created.InboundAuthConfig, 1)
	name, _ := request["attributes"].(map[string]interface{})["name"].(string)
	agent := ownedAgent{id: created.ID, name: name, clientID: created.InboundAuthConfig[0].Config.ClientID,
		clientSecret: created.InboundAuthConfig[0].Config.ClientSecret}
	ts.Require().NotEmpty(agent.clientID)
	ts.Require().NotEmpty(agent.clientSecret)
	return agent
}

// actAs makes the suite's OAuth helpers act as agent until the returned function restores the suite's agent.
func (ts *PersonalAgentTestSuite) actAs(agent ownedAgent) func() {
	clientID, clientSecret := ts.clientID, ts.clientSecret
	ts.clientID, ts.clientSecret = agent.clientID, agent.clientSecret
	return func() { ts.clientID, ts.clientSecret = clientID, clientSecret }
}

func testOUIDForPersonalAgents(ts *PersonalAgentTestSuite) string {
	return ts.seededPersonType().OUID
}

func (ts *PersonalAgentTestSuite) seededPersonType() testutils.UserType {
	ts.T().Helper()
	userTypes, err := testutils.ListUserTypes()
	ts.Require().NoError(err)
	for _, userType := range userTypes {
		if userType.Handle == paSeededUserType {
			return userType
		}
	}
	ts.FailNow("the seeded person user type is missing")
	return testutils.UserType{}
}

func (ts *PersonalAgentTestSuite) createUser(person testutils.UserType, prefix string) (string, string) {
	ts.T().Helper()
	username := common.GenerateUniqueUsername(prefix)
	attributes, err := json.Marshal(map[string]string{
		"username": username, "email": username + "@example.com", "password": paPassword,
	})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{OUID: person.OUID, Type: person.Handle,
		Attributes: attributes})
	ts.Require().NoError(err)
	return userID, username
}

// registerAgent registers the agent under test as the owner, through the registration flow.
func (ts *PersonalAgentTestSuite) registerAgent() {
	ts.T().Helper()
	step := ts.flowStep(nil, "", map[string]interface{}{"applicationId": ts.appID, "flowType": paRegistrationType})
	// The kind is read as an input until the flow can fix it as a constant.
	step = ts.flowStep(step, testutils.RegisterPersonalAgentAction, map[string]interface{}{"inputs": map[string]string{
		"kind": paKind,
	}})
	step = ts.flowStep(step, testutils.PersonalAgentSignInAction, map[string]interface{}{"inputs": map[string]string{
		"username": ts.ownerUsername, "password": paPassword,
	}})
	step = ts.flowStep(step, testutils.PersonalAgentDetailsAction, map[string]interface{}{"inputs": map[string]string{
		"name": paSuiteAgentName, "description": "Helps with my calendar", "redirectUris": paRedirectURI,
	}})
	ts.Require().Equal("COMPLETE", step["flowStatus"], "%v", step)
	data, _ := step["data"].(map[string]interface{})
	additional, _ := data["additionalData"].(map[string]interface{})
	ts.agentID, _ = additional["agentId"].(string)
	ts.clientID, _ = additional["clientId"].(string)
	ts.clientSecret, _ = additional["clientSecret"].(string)
	ts.Require().NotEmpty(ts.agentID)
	ts.Require().NotEmpty(ts.clientID)
	ts.Require().NotEmpty(ts.clientSecret)
}

// userToken signs username in through a public portal application with the default login flow and returns the
// user's own access token, which carries no actor.
func (ts *PersonalAgentTestSuite) userToken(username string) string {
	ts.T().Helper()
	if ts.portalClientID == "" {
		authFlowID, err := testutils.GetFlowIDByHandle("default-flow", "AUTHENTICATION")
		ts.Require().NoError(err)
		ts.portalClientID = "pa_suite_portal"
		portalID, err := testutils.CreateApplication(testutils.Application{
			Name:        "pa_suite_portal",
			Description: "issues the owner's own token",
			OUID:        testOUIDForPersonalAgents(ts),
			Type:        "fullstack",
			AuthFlowID:  authFlowID,
			InboundAuthConfig: []map[string]interface{}{{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                ts.portalClientID,
					"redirectUris":            []string{paPortalRedirect},
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "none",
					"publicClient":            true,
					"pkceRequired":            true,
				},
			}},
		})
		ts.Require().NoError(err)
		ts.portalAppID = portalID
	}
	token, err := testutils.ObtainAccessTokenWithPassword(ts.portalClientID, paPortalRedirect, "openid",
		username, paPassword, true)
	ts.Require().NoError(err)
	return token.AccessToken
}

// selfServiceGet calls a self-service route with token as the bearer token, or none when it is empty.
func (ts *PersonalAgentTestSuite) selfServiceGet(path, token string) (int, string) {
	ts.T().Helper()
	return ts.selfServiceCall(http.MethodGet, path, token, "")
}

// selfServiceCall calls a self-service route with the given method and JSON body, and token as the bearer
// token, or none when it is empty.
func (ts *PersonalAgentTestSuite) selfServiceCall(method, path, token, body string) (int, string) {
	ts.T().Helper()
	req, err := http.NewRequest(method, testServerURL+path, strings.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	return resp.StatusCode, readBody(resp)
}

// flowStep posts one anonymous registration step. previous carries the execution forward; the first
// call passes nil and the start payload.
func (ts *PersonalAgentTestSuite) flowStep(previous map[string]interface{}, action string,
	payload map[string]interface{}) map[string]interface{} {
	ts.T().Helper()
	if previous != nil {
		payload["executionId"] = previous["executionId"]
		payload["challengeToken"] = previous["challengeToken"]
	}
	if action != "" {
		payload["action"] = action
	}
	reqBody, err := json.Marshal(payload)
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(reqBody))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))
	var step map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &step))
	return step
}

// authorize starts an authorization request for the agent and returns the auth and execution IDs.
func (ts *PersonalAgentTestSuite) authorize(codeChallenge string) (string, string) {
	ts.T().Helper()
	resp, err := testutils.InitiateAuthorizationFlowWithPKCE(ts.clientID, paRedirectURI, "code", paScope,
		"pa-state", paResourceIdentifier, codeChallenge, "S256")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, readBody(resp))
	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err)
	return authID, executionID
}

// signIn authorizes the agent and signs username in, returning the auth ID and the step after the
// credentials, which is the consent prompt for the owner.
func (ts *PersonalAgentTestSuite) signIn(username, codeChallenge string) (string, *testutils.FlowStep) {
	ts.T().Helper()
	authID, executionID := ts.authorize(codeChallenge)
	initial, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)
	step, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": username, "password": paPassword,
	}, paSignInAction, initial.ChallengeToken)
	ts.Require().NoError(err)
	return authID, step
}

// approveConsent approves every purpose the consent prompt offers.
func (ts *PersonalAgentTestSuite) approveConsent(step *testutils.FlowStep) *testutils.FlowStep {
	ts.T().Helper()
	ts.Require().NotNil(step.Data)
	raw, _ := step.Data.AdditionalData["consentPrompt"].(string)
	var purposes []struct {
		PurposeName string `json:"purposeName"`
		Essential   []struct {
			Name string `json:"name"`
		} `json:"essential"`
		Optional []struct {
			Name string `json:"name"`
		} `json:"optional"`
	}
	if raw != "" {
		ts.Require().NoError(json.Unmarshal([]byte(raw), &purposes))
	}

	type element struct {
		Name     string `json:"name"`
		Approved bool   `json:"approved"`
	}
	type purpose struct {
		PurposeName string    `json:"purposeName"`
		Approved    bool      `json:"approved"`
		Elements    []element `json:"elements"`
	}
	decisions := struct {
		Approved bool      `json:"approved"`
		Purposes []purpose `json:"purposes"`
	}{Approved: true}
	for _, p := range purposes {
		decision := purpose{PurposeName: p.PurposeName, Approved: true}
		for _, e := range p.Essential {
			decision.Elements = append(decision.Elements, element{Name: e.Name, Approved: true})
		}
		for _, e := range p.Optional {
			decision.Elements = append(decision.Elements, element{Name: e.Name, Approved: true})
		}
		decisions.Purposes = append(decisions.Purposes, decision)
	}
	decisionsJSON, err := json.Marshal(decisions)
	ts.Require().NoError(err)

	next, err := testutils.ExecuteAuthenticationFlow(step.ExecutionID, map[string]string{
		paConsentDecisions: string(decisionsJSON),
	}, paConsentAllowAction, step.ChallengeToken)
	ts.Require().NoError(err)
	return next
}

// obtainCode signs username in, approves consent, and returns the authorization code and its PKCE
// verifier.
func (ts *PersonalAgentTestSuite) obtainCode(username string) (string, string) {
	ts.T().Helper()
	verifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err)
	authID, step := ts.signIn(username, testutils.GenerateCodeChallenge(verifier))
	ts.Require().NotEqual("COMPLETE", step.FlowStatus, "the owner is asked for consent")
	step = ts.approveConsent(step)
	ts.Require().Equal("COMPLETE", step.FlowStatus)
	ts.Require().NotEmpty(step.Assertion)

	authzResp, err := testutils.CompleteAuthorization(authID, step.Assertion)
	ts.Require().NoError(err)
	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)
	return code, verifier
}

// obtainToken runs the whole authorization code exchange for username.
func (ts *PersonalAgentTestSuite) obtainToken(username string) *testutils.TokenResponse {
	ts.T().Helper()
	code, verifier := ts.obtainCode(username)
	status, body := ts.exchangeCode(code, verifier)
	ts.Require().Equal(http.StatusOK, status, body)
	var token testutils.TokenResponse
	ts.Require().NoError(json.Unmarshal([]byte(body), &token))
	ts.Require().NotEmpty(token.AccessToken)
	return &token
}

func (ts *PersonalAgentTestSuite) exchangeCode(code, verifier string) (int, string) {
	ts.T().Helper()
	return ts.postToken(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {paRedirectURI},
		"code_verifier": {verifier},
		"resource":      {paResourceIdentifier},
	})
}

// postToken calls the token endpoint as the agent, with client_secret_basic.
func (ts *PersonalAgentTestSuite) postToken(form url.Values) (int, string) {
	ts.T().Helper()
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(ts.clientID, ts.clientSecret)
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, string(body)
}

// --- owner suspension ---

// Suspending an owner suspends their personal agents alongside the organization's or the owner's own holds, and
// unsuspending the owner lifts only that cascaded hold. The owner's other agents are not suspended.
func (ts *PersonalAgentTestSuite) TestSuspendingAnOwnerSuspendsTheirPersonalAgents() {
	ownerID, agent, token := ts.ownerWithAgent("pa_suspended_owner")
	defer func() { _ = deleteAgent(agent.id) }()
	held := ts.createAgentFor(ownerID, "authorization_code")
	defer func() { _ = deleteAgent(held.id) }()
	ordinaryID := ts.createOrdinaryAgentFor(ownerID)
	defer func() { _ = deleteAgent(ordinaryID) }()
	ts.runAdministrationFlow("default-agent-suspend-flow", held.id, "")

	ts.runAdministrationFlow("default-user-suspend-flow", ownerID, "")
	read, err := testutils.GetAccessStatus("agents", agent.id)
	ts.Require().NoError(err)
	ts.Equal("SUSPENDED", read.Value)
	ts.Require().NotNil(read.Details)
	ts.Equal("CASCADED_SUSPENSION", read.Details.Reason)
	ts.Equal([]string{"cascade"}, read.Details.AppliedBy)
	ts.False(ts.signInStarts(agent.clientID), "the suspended owner's agent is refused")
	ts.False(ts.introspectAsSuiteApp(token.AccessToken), "the agent's token for the owner fails")
	read, err = testutils.GetAccessStatus("agents", held.id)
	ts.Require().NoError(err)
	ts.Equal([]string{"organization", "cascade"}, read.Details.AppliedBy, "both suspensions are held")
	read, err = testutils.GetAccessStatus("agents", ordinaryID)
	ts.Require().NoError(err)
	ts.Equal("ACTIVE", read.Value, "an agent that is not a personal agent is not suspended with its owner")

	ts.runAdministrationFlow("default-user-unsuspend-flow", ownerID, "")
	read, err = testutils.GetAccessStatus("agents", agent.id)
	ts.Require().NoError(err)
	ts.Equal("ACTIVE", read.Value)
	ts.True(ts.signInStarts(agent.clientID), "the agent is back with its owner")
	read, err = testutils.GetAccessStatus("agents", held.id)
	ts.Require().NoError(err)
	ts.Equal("SUSPENDED", read.Value, "the organization's suspension outlasts the owner's")
	ts.Equal([]string{"organization"}, read.Details.AppliedBy)
}

// A personal agent cannot be created for a user whose account is suspended, since it would start active while its
// owner's other personal agents are suspended with them. Once the owner is unsuspended, it can.
func (ts *PersonalAgentTestSuite) TestNoPersonalAgentForASuspendedOwner() {
	ownerID, _ := ts.createUser(ts.seededPersonType(), "pa_suspended_new_owner")
	defer func() { _ = testutils.DeleteUser(ownerID) }()

	ts.runAdministrationFlow("default-user-suspend-flow", ownerID, "")
	resp, err := doPost(testServerURL+agentBasePath, ts.personalAgentRequest(ownerID))
	ts.Require().NoError(err)
	body := readBody(resp)
	resp.Body.Close()
	ts.Equal(http.StatusBadRequest, resp.StatusCode, body)
	ts.Contains(body, "AGT-1054")

	ts.runAdministrationFlow("default-user-unsuspend-flow", ownerID, "")
	resp, err = doPost(testServerURL+agentBasePath, ts.personalAgentRequest(ownerID))
	ts.Require().NoError(err)
	body = readBody(resp)
	resp.Body.Close()
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, body)
	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(body), &created))
	_ = deleteAgent(created.ID)
}
