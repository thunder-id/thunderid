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

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	paConfigSection    = "personalAgent"
	paPassword         = "Personal@Agent123"
	paRedirectURI      = "https://agent.example.com/callback"
	paMaxTokenValidity = 600
	paRegistrationType = "REGISTRATION"
	paKind             = "personal-assistant"
	paSeededUserType   = "person"
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
// what it can do afterwards: only its owner signs in to it, consent is always asked, its tokens carry
// the owner as the subject and the agent as the actor, are capped in lifetime, and are refused while
// personal agents are disabled. An administrator can read it but not create or update one.
type PersonalAgentTestSuite struct {
	suite.Suite
	originalConfig   json.RawMessage
	resourceServerID string
	roleID           string
	appID            string
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
	original, err := testutils.MergeWritableServerConfig(paConfigSection, map[string]interface{}{
		"enabled": true, "maxAccessTokenValidity": paMaxTokenValidity,
	})
	ts.Require().NoError(err)
	ts.originalConfig = original

	person := ts.seededPersonType()
	ts.ownerID, ts.ownerUsername = ts.createUser(person, "pa_owner")
	ts.otherID, ts.otherUsername = ts.createUser(person, "pa_other")

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
	if ts.originalConfig != nil {
		if err := testutils.PutWritableServerConfig(paConfigSection, ts.originalConfig); err != nil {
			ts.T().Errorf("failed to restore the personalAgent server config: %v", err)
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

// While personal agents are disabled, an administrator cannot register one.
func (ts *PersonalAgentTestSuite) TestAdminCreateRefusedWhileDisabled() {
	ts.setEnabled(false)
	defer ts.setEnabled(true)

	resp, err := doPost(testServerURL+agentBasePath, ts.personalAgentRequest(ts.otherID, "authorization_code"))
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body := readBody(resp)
	ts.Equal(http.StatusBadRequest, resp.StatusCode, body)
	ts.Contains(body, "AGT-1049")
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
	ts.Equal("pa_suite_assistant", attributes["name"])
}

// --- signing the owner in ---

// The owner signs in, is always asked for consent, and the token names the owner as the subject and
// the agent as the actor, within the lifetime cap.
func (ts *PersonalAgentTestSuite) TestOwnerGetsADelegatedToken() {
	token := ts.obtainToken(ts.ownerUsername)

	claims, err := testutils.DecodeJWTPayloadMap(token.AccessToken)
	ts.Require().NoError(err)
	ts.Equal(ts.ownerID, claims["sub"], "the subject is the owner")
	act, ok := claims["act"].(map[string]any)
	ts.Require().True(ok, "the token names the agent as the actor: %v", claims)
	ts.Equal(ts.agentID, act["sub"])
	ts.Contains(strings.Fields(token.Scope), paPermission, "the token holds the approved permission")
	ts.LessOrEqual(token.ExpiresIn, float64(paMaxTokenValidity), "the lifetime is capped")
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

// While personal agents are disabled, the token endpoint refuses the agent, even with a valid code.
func (ts *PersonalAgentTestSuite) TestTokenRefusedWhileDisabled() {
	code, verifier := ts.obtainCode(ts.ownerUsername)
	ts.setEnabled(false)
	defer ts.setEnabled(true)

	status, body := ts.exchangeCode(code, verifier)
	ts.Equal(http.StatusBadRequest, status, body)
	ts.Contains(body, paUnauthorizedClient)
}

// While personal agents are disabled, the authorization is refused before the owner signs in: the agent
// is sent back to its redirect URI with unauthorized_client.
func (ts *PersonalAgentTestSuite) TestAuthorizeRefusedWhileDisabled() {
	ts.setEnabled(false)
	defer ts.setEnabled(true)
	verifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err)

	resp, err := testutils.InitiateAuthorizationFlowWithPKCE(ts.clientID, paRedirectURI, "code", paScope,
		"pa-state", paResourceIdentifier, testutils.GenerateCodeChallenge(verifier), "S256")
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)
	location := resp.Header.Get("Location")
	ts.True(strings.HasPrefix(location, paRedirectURI), "the refusal goes back to the agent: %s", location)
	ts.Contains(location, "error="+paUnauthorizedClient)
	ts.Contains(location, "state=pa-state")
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
		"name": "pa_suite_assistant", "description": "Helps with my calendar", "redirectUris": paRedirectURI,
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

func (ts *PersonalAgentTestSuite) setEnabled(enabled bool) {
	ts.T().Helper()
	_, err := testutils.MergeWritableServerConfig(paConfigSection, map[string]interface{}{"enabled": enabled})
	ts.Require().NoError(err)
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
