// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	registrationFlowType        = "REGISTRATION"
	personalAgentAuthFlowHandle = "default-personal-agent-auth-flow"
	personalKind                = "personal-assistant"
	personalAgentPassword       = "Personal@Agent123"
	personalAgentCallback       = "https://agent.example.com/callback"
	personalAgentJWKSURI        = "https://agent.example.com/jwks"

	// The kind is read as an input; the flow will fix it once flow inputs can carry constant values.
	kindInput = "kind"

	ownerAction                         = "action_owner"
	errCodeCannotRegisterForAnotherUser = "FET-1115"
	errCodeNotAPersonalAgent            = "FET-1091"

	personalAgentPortalClientID = "pa_registration_portal_client"
	personalAgentPortalRedirect = "http://localhost:3000/portal/callback"
)

// PersonalAgentRegistrationFlowTestSuite drives the personal agent branch of a registration flow:
// the user chooses to register a personal agent, signs in, describes the agent, and gets a personal agent
// they own, with the personal client setup and the shipped personal agent login flow.
type PersonalAgentRegistrationFlowTestSuite struct {
	suite.Suite
	// tokenFlowID is a registration flow with no sign-in step, bound to tokenClientAppID, for callers that
	// send a signed-in user's token. tokenClientAppID issues that token, and the flow accepts only a token
	// issued to the application running it, so tokenRegAppID, which runs the same flow, refuses it.
	ouID             string
	flows            *testutils.PersonalAgentRegistrationFlows
	tokenFlowID      string
	tokenRegAppID    string
	tokenClientAppID string
	portalLoginFlow  string
	appID            string
	userID           string
	username         string
	agentIDs         []string
	authFlow         string
	loginFlow        string
}

func TestPersonalAgentRegistrationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(PersonalAgentRegistrationFlowTestSuite))
}

func (ts *PersonalAgentRegistrationFlowTestSuite) SetupSuite() {
	// Applications that bind a registration flow use the matching login flow, so every flow they reach is
	// one they bind.
	flows, err := testutils.CreatePersonalAgentRegistrationFlows("pa-registration-flow-test")
	ts.Require().NoError(err)
	ts.flows = flows
	authFlowID := flows.LoginFlowID
	ts.authFlow = authFlowID

	loginFlowID, err := testutils.GetFlowIDByHandle(personalAgentAuthFlowHandle, "AUTHENTICATION")
	ts.Require().NoError(err)
	ts.loginFlow = loginFlowID

	userTypes, err := testutils.ListUserTypes()
	ts.Require().NoError(err)
	var person *testutils.UserType
	for i := range userTypes {
		if userTypes[i].Handle == seededUserTypeName {
			person = &userTypes[i]
			break
		}
	}
	ts.Require().NotNil(person, "the seeded %q user type is needed", seededUserTypeName)

	ts.ouID = person.OUID
	ts.username = common.GenerateUniqueUsername("pa_owner")
	attributes, err := json.Marshal(map[string]string{
		"username": ts.username, "email": ts.username + "@example.com", "password": personalAgentPassword,
	})
	ts.Require().NoError(err)
	ts.userID, err = testutils.CreateUser(testutils.User{OUID: person.OUID, Type: person.Handle,
		Attributes: attributes})
	ts.Require().NoError(err)

	registrationFlowID := ts.flows.RegistrationFlowID
	ts.appID, err = testutils.CreateApplication(testutils.Application{
		Name:                      "pa_registration_flow_app",
		Description:               "personal agent registration flow test",
		OUID:                      person.OUID,
		AuthFlowID:                authFlowID,
		RegistrationFlowID:        registrationFlowID,
		IsRegistrationFlowEnabled: true,
		ClientID:                  "pa_registration_flow_app_client",
		ClientSecret:              "pa_registration_flow_app_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{seededUserTypeName},
	})
	ts.Require().NoError(err)

	// The token helper submits the default login flow's sign-in action, and the portal binds a login flow
	// with no link to another registration flow, so it can bind the token flow.
	ts.portalLoginFlow, err = testutils.CreateUsernamePasswordLoginFlow("pa-registration-portal-login", "action_001")
	ts.Require().NoError(err)
	ts.tokenFlowID, err = testutils.CreateFlow(testutils.Flow{
		Name:     "Personal Agent Registration Token Flow",
		FlowType: registrationFlowType,
		Handle:   "pa_registration_token_flow",
		Nodes:    tokenRegistrationNodes(),
	})
	ts.Require().NoError(err)
	ts.tokenRegAppID, err = testutils.CreateApplication(testutils.Application{
		Name:                      "pa_registration_token_app",
		Description:               "personal agent registration with a caller token",
		OUID:                      person.OUID,
		AuthFlowID:                authFlowID,
		RegistrationFlowID:        ts.tokenFlowID,
		IsRegistrationFlowEnabled: true,
		ClientID:                  "pa_registration_token_app_client",
		ClientSecret:              "pa_registration_token_app_secret",
	})
	ts.Require().NoError(err)
	ts.tokenClientAppID, err = testutils.CreateApplication(testutils.Application{
		Name:                      "pa_registration_portal",
		Description:               "issues the signed-in user's token",
		OUID:                      person.OUID,
		Type:                      "fullstack",
		AuthFlowID:                ts.portalLoginFlow,
		RegistrationFlowID:        ts.tokenFlowID,
		IsRegistrationFlowEnabled: true,
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                personalAgentPortalClientID,
					"redirectUris":            []string{personalAgentPortalRedirect},
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "none",
					"publicClient":            true,
					"pkceRequired":            true,
				},
			},
		},
	})
	ts.Require().NoError(err)
}

func (ts *PersonalAgentRegistrationFlowTestSuite) TearDownSuite() {
	for _, agentID := range ts.agentIDs {
		if err := testutils.DeleteAgent(agentID); err != nil {
			ts.T().Logf("failed to delete agent %s: %v", agentID, err)
		}
	}
	for _, appID := range []string{ts.appID, ts.tokenRegAppID, ts.tokenClientAppID} {
		if appID == "" {
			continue
		}
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("failed to delete app %s: %v", appID, err)
		}
	}
	if ts.tokenFlowID != "" {
		if err := testutils.DeleteFlow(ts.tokenFlowID); err != nil {
			ts.T().Logf("failed to delete flow %s: %v", ts.tokenFlowID, err)
		}
	}
	if ts.portalLoginFlow != "" {
		if err := testutils.DeleteFlow(ts.portalLoginFlow); err != nil {
			ts.T().Logf("failed to delete flow %s: %v", ts.portalLoginFlow, err)
		}
	}
	if ts.flows != nil {
		if err := ts.flows.Delete(); err != nil {
			ts.T().Logf("failed to delete the registration flows: %v", err)
		}
	}
	if ts.userID != "" {
		if err := testutils.DeleteUser(ts.userID); err != nil {
			ts.T().Logf("failed to delete user %s: %v", ts.userID, err)
		}
	}
}

// Without a key the agent authenticates with a client secret, shown once on the credentials screen.
func (ts *PersonalAgentRegistrationFlowTestSuite) TestRegistersAPersonalAgentWithAClientSecret() {
	step := ts.register(map[string]string{
		agentNameInput: common.GenerateUniqueUsername("pa_secret"), agentRedirectURIsInput: personalAgentCallback,
		// A submitted owner is not the one the agent gets.
		agentOwnerInput: "someone-else",
	})
	ts.NotEmpty(step.Data.AdditionalData[agentClientSecretData])

	agent := ts.agentFrom(step)
	ts.Equal(ts.userID, agent.Owner, "the agent belongs to the user who signed in")
	ts.Equal(personalKind, agent.Kind)
	ts.Equal(ts.loginFlow, agent.AuthFlowID, "the agent signs in through the personal agent login flow")
	oauth := ts.oauthOf(agent)
	ts.Equal([]string{"authorization_code"}, oauth.GrantTypes)
	ts.Equal([]string{personalAgentCallback}, oauth.RedirectURIs)
	ts.True(oauth.PKCERequired)
	ts.Equal("client_secret_basic", oauth.TokenEndpointAuthMethod)
}

// A JWKS URI selects private_key_jwt, so no secret is generated.
func (ts *PersonalAgentRegistrationFlowTestSuite) TestRegistersAPersonalAgentWithAJWKSURI() {
	step := ts.register(map[string]string{
		agentNameInput: common.GenerateUniqueUsername("pa_jwks"), agentRedirectURIsInput: personalAgentCallback,
		"jwksUri": personalAgentJWKSURI,
	})
	ts.Empty(step.Data.AdditionalData[agentClientSecretData])

	agent := ts.agentFrom(step)
	ts.Equal(personalKind, agent.Kind)
	ts.Equal("private_key_jwt", ts.oauthOf(agent).TokenEndpointAuthMethod)
}

// The server fetches the JWKS URI when the agent authenticates, so a registration refuses one that is not
// SSRF-safe and creates nothing.
func (ts *PersonalAgentRegistrationFlowTestSuite) TestRefusesAnUnsafeJWKSURI() {
	executionID, challenge := ts.signIn()
	status, step, body := ts.step(executionID, challenge, agentDetailsAction, map[string]string{
		agentNameInput: common.GenerateUniqueUsername("pa_unsafe_jwks"), agentRedirectURIsInput: personalAgentCallback,
		"jwksUri": "https://169.254.169.254/latest/meta-data/",
	})

	ts.Require().Equal(http.StatusOK, status, string(body))
	ts.NotEqual("COMPLETE", step.FlowStatus, string(body))
	ts.Empty(step.Data.AdditionalData[agentIDData], string(body))
	ts.Contains(string(body), "AGT-1026", string(body))
}

// A registration flow registers only personal agents, so without the personal kind nothing is created.
func (ts *PersonalAgentRegistrationFlowTestSuite) TestRefusedWithoutThePersonalKind() {
	step := ts.choosePersonalAgent(ts.appID, "", false)
	status, step, body := ts.step(step.ExecutionID, step.ChallengeToken, testutils.PersonalAgentSignInAction, map[string]string{
		"username": ts.username, "password": personalAgentPassword,
	})

	ts.Require().Equal(http.StatusOK, status, string(body))
	ts.NotEqual("COMPLETE", step.FlowStatus, string(body))
	ts.Empty(step.Data.AdditionalData[agentIDData], string(body))
	ts.Contains(string(body), errCodeNotAPersonalAgent, string(body))
}

// The other choice signs a new user up, as the default registration flow does.
func (ts *PersonalAgentRegistrationFlowTestSuite) TestUserChoiceLeadsToSignUp() {
	status, step, body := ts.step("", "", "", nil)
	ts.Require().Equal(http.StatusOK, status, string(body))
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, string(body))

	status, step, body = ts.step(step.ExecutionID, step.ChallengeToken, testutils.RegisterUserAction, nil)

	ts.Require().Equal(http.StatusOK, status, string(body))
	ts.Equal("INCOMPLETE", step.FlowStatus, string(body))
	ts.Subset(inputIdentifiers(step), []string{"username", "password"}, string(body))
}

// A registration needs somewhere to send the owner back to, so it asks again rather than creating the
// agent.
func (ts *PersonalAgentRegistrationFlowTestSuite) TestAsksForTheRedirectURI() {
	executionID, challenge := ts.signIn()
	status, step, body := ts.step(executionID, challenge, agentDetailsAction, map[string]string{
		agentNameInput: common.GenerateUniqueUsername("pa_no_callback"),
	})

	ts.Require().Equal(http.StatusOK, status, string(body))
	ts.Equal("INCOMPLETE", step.FlowStatus, string(body))
	ts.Contains(inputIdentifiers(step), agentRedirectURIsInput, string(body))
}

// A caller that already has a signed-in user sends that user's token, and a flow with no sign-in step
// registers the agent for the token's subject.
func (ts *PersonalAgentRegistrationFlowTestSuite) TestRegistersForTheTokenSubject() {
	tokenResp, err := testutils.ObtainAccessTokenWithPassword(personalAgentPortalClientID,
		personalAgentPortalRedirect, "openid", ts.username, personalAgentPassword, true)
	ts.Require().NoError(err)
	token := tokenResp.AccessToken

	step := ts.choosePersonalAgent(ts.tokenClientAppID, token, true)
	status, step, body := ts.stepFor(ts.tokenClientAppID, token, step.ExecutionID, step.ChallengeToken,
		agentDetailsAction, map[string]string{
			agentNameInput: common.GenerateUniqueUsername("pa_token"), agentRedirectURIsInput: personalAgentCallback,
		})
	ts.Require().Equal(http.StatusOK, status, string(body))
	ts.Require().Equal("COMPLETE", step.FlowStatus, string(body))
	agentID := step.Data.AdditionalData[agentIDData]
	ts.Require().NotEmpty(agentID, string(body))
	ts.agentIDs = append(ts.agentIDs, agentID)

	agent := ts.agentFrom(step)
	ts.Equal(ts.userID, agent.Owner, "the agent belongs to the token's subject")
	ts.Equal(personalKind, agent.Kind)
}

// A token issued to another application cannot register an agent, or that application could register agents
// for its users without their involvement.
func (ts *PersonalAgentRegistrationFlowTestSuite) TestTokenOfAnotherApplicationIsRefused() {
	tokenResp, err := testutils.ObtainAccessTokenWithPassword(personalAgentPortalClientID,
		personalAgentPortalRedirect, "openid", ts.username, personalAgentPassword, true)
	ts.Require().NoError(err)
	token := tokenResp.AccessToken

	step := ts.choosePersonalAgent(ts.tokenRegAppID, token, true)
	_, step, body := ts.stepFor(ts.tokenRegAppID, token, step.ExecutionID, step.ChallengeToken,
		agentDetailsAction, map[string]string{
			agentNameInput: common.GenerateUniqueUsername("pa_other_app"), agentRedirectURIsInput: personalAgentCallback,
		})
	ts.NotEqual("COMPLETE", step.FlowStatus, string(body))
	ts.Empty(step.Data.AdditionalData[agentIDData], string(body))
}

// Without a sign-in step or a token there is nobody to own the agent, so the registration fails.
func (ts *PersonalAgentRegistrationFlowTestSuite) TestRefusedWithoutAUser() {
	step := ts.choosePersonalAgent(ts.tokenRegAppID, "", true)
	_, step, body := ts.stepFor(ts.tokenRegAppID, "", step.ExecutionID, step.ChallengeToken,
		agentDetailsAction, map[string]string{
			agentNameInput: common.GenerateUniqueUsername("pa_anonymous"), agentRedirectURIsInput: personalAgentCallback,
		})
	ts.NotEqual("COMPLETE", step.FlowStatus, string(body))
	ts.Empty(step.Data.AdditionalData[agentIDData], string(body))
}

// An administrator registers a personal agent for another user: the flow's owner resolver selects the
// user, and the administrator's token may create agents for others, so the agent is that user's.
func (ts *PersonalAgentRegistrationFlowTestSuite) TestAdminRegistersForAnotherUserThroughAnOwnerResolver() {
	appID := ts.ownerResolverApp("pa_registration_owner_resolver_admin")
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err)

	step := ts.selectOwner(appID, adminToken, ts.userID)
	ts.Require().Contains(inputIdentifiers(step), agentNameInput)
	status, step, body := ts.stepFor(appID, adminToken, step.ExecutionID, step.ChallengeToken, agentDetailsAction,
		map[string]string{
			agentNameInput: common.GenerateUniqueUsername("pa_admin_for_user"), agentRedirectURIsInput: personalAgentCallback,
		})
	ts.Require().Equal(http.StatusOK, status, string(body))
	ts.Require().Equal("COMPLETE", step.FlowStatus, string(body))
	agentID := step.Data.AdditionalData[agentIDData]
	ts.Require().NotEmpty(agentID, string(body))
	ts.agentIDs = append(ts.agentIDs, agentID)

	agent := ts.agentFrom(step)
	ts.Equal(ts.userID, agent.Owner, "the agent belongs to the selected user, not the administrator")
	ts.Equal(personalKind, agent.Kind)
}

// A user who may not create agents for others cannot make another user the owner: the registration is
// refused and nothing is created.
func (ts *PersonalAgentRegistrationFlowTestSuite) TestUserCannotRegisterForAnotherUser() {
	appID := ts.ownerResolverApp("pa_registration_owner_resolver_user")
	otherID := ts.createPerson("pa_other_owner")
	tokenResp, err := testutils.ObtainAccessTokenWithPassword(personalAgentPortalClientID,
		personalAgentPortalRedirect, "openid", ts.username, personalAgentPassword, true)
	ts.Require().NoError(err)

	step := ts.selectOwner(appID, tokenResp.AccessToken, otherID)
	_, step, body := ts.stepFor(appID, tokenResp.AccessToken, step.ExecutionID, step.ChallengeToken,
		agentDetailsAction, map[string]string{
			agentNameInput: common.GenerateUniqueUsername("pa_user_for_other"), agentRedirectURIsInput: personalAgentCallback,
		})
	ts.NotEqual("COMPLETE", step.FlowStatus, string(body))
	ts.Empty(step.Data.AdditionalData[agentIDData], string(body))
	ts.Contains(string(body), errCodeCannotRegisterForAnotherUser, string(body))
}

// --- helpers ---

// ownerResolverApp creates a registration flow whose owner resolver lets the caller pick the owner, and an
// application bound to it. Both are removed after the test.
func (ts *PersonalAgentRegistrationFlowTestSuite) ownerResolverApp(handle string) string {
	ts.T().Helper()
	nodes := tokenRegistrationNodes()
	nodes[1]["prompts"].([]map[string]interface{})[1]["action"].(map[string]interface{})["nextNode"] =
		"owner_resolution"
	nodes = append(nodes,
		map[string]interface{}{
			"id":           "owner_resolution",
			"type":         "TASK_EXECUTION",
			"executor":     map[string]interface{}{"name": "OwnerResolver"},
			"onSuccess":    "agent_type_resolution",
			"onIncomplete": "prompt_owner",
		},
		map[string]interface{}{
			"id":   "prompt_owner",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{{
				"inputs": []map[string]interface{}{
					{"ref": "owner_input", "identifier": agentOwnerInput, "type": "USER_SELECT", "required": false},
				},
				"action": map[string]interface{}{"ref": ownerAction, "nextNode": "owner_resolution"},
			}},
		})
	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name: "Personal Agent Registration " + handle, FlowType: registrationFlowType, Handle: handle, Nodes: nodes,
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:                      handle + "_app",
		Description:               "personal agent registration with an owner resolver",
		OUID:                      ts.ouID,
		AuthFlowID:                ts.authFlow,
		RegistrationFlowID:        flowID,
		IsRegistrationFlowEnabled: true,
		ClientID:                  handle + "_client",
		ClientSecret:              handle + "_secret",
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })
	return appID
}

// selectOwner starts the owner resolver flow with token and selects owner, returning the details step.
func (ts *PersonalAgentRegistrationFlowTestSuite) selectOwner(appID, token, owner string) common.FlowStep {
	ts.T().Helper()
	step := ts.choosePersonalAgent(appID, token, true)
	ts.Require().Contains(inputIdentifiers(step), agentOwnerInput, "the owner is asked for")
	status, step, body := ts.stepFor(appID, token, step.ExecutionID, step.ChallengeToken, ownerAction,
		map[string]string{agentOwnerInput: owner})
	ts.Require().Equal(http.StatusOK, status, string(body))
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, string(body))
	return step
}

// createPerson creates a user of the seeded person type, removed after the test, and returns its ID.
func (ts *PersonalAgentRegistrationFlowTestSuite) createPerson(prefix string) string {
	ts.T().Helper()
	username := common.GenerateUniqueUsername(prefix)
	attributes, err := json.Marshal(map[string]string{"username": username, "email": username + "@example.com"})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{OUID: ts.ouID, Type: seededUserTypeName, Attributes: attributes})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteUser(userID) })
	return userID
}

// choosePersonalAgent starts the registration flow of appID, with token as the bearer token when given, and
// chooses to register a personal agent, submitting the personal kind when withKind is set. It returns the
// step that follows the choice.
func (ts *PersonalAgentRegistrationFlowTestSuite) choosePersonalAgent(appID, token string,
	withKind bool) common.FlowStep {
	ts.T().Helper()
	status, step, body := ts.stepFor(appID, token, "", "", "", nil)
	ts.Require().Equal(http.StatusOK, status, string(body))
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, string(body))
	var inputs map[string]string
	if withKind {
		inputs = map[string]string{kindInput: personalKind}
	}
	status, step, body = ts.stepFor(appID, token, step.ExecutionID, step.ChallengeToken,
		testutils.RegisterPersonalAgentAction, inputs)
	ts.Require().Equal(http.StatusOK, status, string(body))
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, string(body))
	return step
}

// signIn starts the registration, chooses a personal agent and signs the user in through the
// called authentication flow. It returns the execution and the challenge token of the details prompt that
// follows.
func (ts *PersonalAgentRegistrationFlowTestSuite) signIn() (string, string) {
	ts.T().Helper()
	step := ts.choosePersonalAgent(ts.appID, "", true)

	executionID := step.ExecutionID
	status, step, body := ts.step(executionID, step.ChallengeToken, testutils.PersonalAgentSignInAction, map[string]string{
		"username": ts.username, "password": personalAgentPassword,
	})
	ts.Require().Equal(http.StatusOK, status, string(body))
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, string(body))
	// The prompt is driven by the agent type's schema, plus the redirect URI and the key inputs.
	ts.Require().Subset(inputIdentifiers(step), []string{agentNameInput, agentModelInput, agentRedirectURIsInput},
		"signing in leads to the details prompt: %s", string(body))
	return executionID, step.ChallengeToken
}

// register signs in and submits the details, and returns the completed step.
func (ts *PersonalAgentRegistrationFlowTestSuite) register(details map[string]string) common.FlowStep {
	ts.T().Helper()
	executionID, challenge := ts.signIn()
	status, step, body := ts.step(executionID, challenge, agentDetailsAction, details)
	ts.Require().Equal(http.StatusOK, status, string(body))
	ts.Require().Equal("COMPLETE", step.FlowStatus, string(body))
	ts.Require().NotEmpty(step.Data.AdditionalData[agentIDData], string(body))
	ts.agentIDs = append(ts.agentIDs, step.Data.AdditionalData[agentIDData])
	ts.NotEmpty(step.Data.AdditionalData[agentClientIDData])
	return step
}

func (ts *PersonalAgentRegistrationFlowTestSuite) agentFrom(step common.FlowStep) *testutils.Agent {
	ts.T().Helper()
	agent, err := testutils.GetAgent(step.Data.AdditionalData[agentIDData])
	ts.Require().NoError(err)
	return agent
}

func (ts *PersonalAgentRegistrationFlowTestSuite) oauthOf(agent *testutils.Agent) *testutils.AgentOAuthConfig {
	ts.T().Helper()
	ts.Require().Len(agent.InboundAuthConfig, 1)
	ts.Require().NotNil(agent.InboundAuthConfig[0].Config)
	return agent.InboundAuthConfig[0].Config
}

// step posts one interaction to the registration flow, anonymously: the registration signs the user in itself.
func (ts *PersonalAgentRegistrationFlowTestSuite) step(executionID, challengeToken, action string,
	inputs map[string]string) (int, common.FlowStep, []byte) {
	ts.T().Helper()
	return ts.stepFor(ts.appID, "", executionID, challengeToken, action, inputs)
}

// stepFor posts one interaction to the registration flow of appID, with token as the bearer token when
// given. The first call starts the flow for the application.
func (ts *PersonalAgentRegistrationFlowTestSuite) stepFor(appID, token, executionID, challengeToken,
	action string, inputs map[string]string) (int, common.FlowStep, []byte) {
	ts.T().Helper()
	payload := map[string]interface{}{"applicationId": appID, "flowType": registrationFlowType}
	if executionID != "" {
		payload = map[string]interface{}{"executionId": executionID, "challengeToken": challengeToken}
	}
	if len(inputs) > 0 {
		payload["inputs"] = inputs
	}
	if action != "" {
		payload["action"] = action
	}
	reqBody, err := json.Marshal(payload)
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(reqBody))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	var step common.FlowStep
	_ = json.Unmarshal(body, &step)
	return resp.StatusCode, step, body
}

// tokenRegistrationNodes is a registration flow whose personal agent branch has no sign-in step: the owner
// comes from the caller's token. The branch resolves the bundled agent type, collects the details and
// provisions. A registration flow also needs a user branch, which these tests do not take.
func tokenRegistrationNodes() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "prompt_registration_choice"},
		{
			"id":   "prompt_registration_choice",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{"action": map[string]interface{}{"ref": testutils.RegisterUserAction, "nextNode": "user_type_resolver"}},
				{"action": map[string]interface{}{"ref": testutils.RegisterPersonalAgentAction,
					"nextNode": "agent_type_resolution"}},
			},
		},
		{
			"id":        "user_type_resolver",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "UserTypeResolver"},
			"onSuccess": "end",
		},
		{
			"id":         "agent_type_resolution",
			"type":       "TASK_EXECUTION",
			"properties": map[string]interface{}{"allowedAgentTypes": []string{"default"}},
			"executor":   map[string]interface{}{"name": "AgentTypeResolver"},
			"onSuccess":  "prompt_agent_details",
		},
		{
			"id":   "prompt_agent_details",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{"ref": "input_name", "identifier": agentNameInput, "type": "TEXT_INPUT", "required": true},
						{"ref": "input_redirect_uris", "identifier": agentRedirectURIsInput, "type": "TEXT_INPUT",
							"required": true},
					},
					"action": map[string]interface{}{"ref": agentDetailsAction, "nextNode": "provisioning"},
				},
			},
		},
		{
			"id":           "provisioning",
			"type":         "TASK_EXECUTION",
			"properties":   map[string]interface{}{"mode": "agent"},
			"executor":     map[string]interface{}{"name": "ProvisioningExecutor"},
			"onSuccess":    "end",
			"onIncomplete": "prompt_agent_details",
			"onFailure":    "prompt_agent_details",
		},
		{"id": "end", "type": "END"},
	}
}
