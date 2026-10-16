// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

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

// EntityCategoryTestSuite checks how the feature treats agents and applications: an administrative
// hold applies to them, client-secret failures never lock, and unsuspending lands them on ACTIVE.
// Agents live in the OU the shared `default` agent type points at, so the type is not repointed.
type EntityCategoryTestSuite struct {
	suite.Suite
	ouID             string
	agentOUID        string
	entityTypeID     string
	authFlowID       string
	resourceServerID string
	agentIDs         []string
	appIDs           []string
	userIDs          []string
	suffix           string
}

func TestEntityCategoryTestSuite(t *testing.T) {
	suite.Run(t, new(EntityCategoryTestSuite))
}

const categoryResourceIdentifier = "https://governance-category.example.com"

const tokenExchangeGrant = "urn:ietf:params:oauth:grant-type:token-exchange"

var entityCategoryOU = testutils.OrganizationUnit{
	Handle:      "governance_category_ou",
	Name:        "Governance Entity Category Test OU",
	Description: "Organization unit for the agent and application acceptance tests",
}

var entityCategoryUserType = testutils.UserType{
	Handle:      "governance_category_user",
	DisplayName: "governance_category_user",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
	},
}

func (ts *EntityCategoryTestSuite) SetupSuite() {
	applyLockoutPolicy(ts.T())

	ouID, err := testutils.CreateOrganizationUnit(entityCategoryOU)
	ts.Require().NoError(err, "failed to create the test organization unit")
	ts.ouID = ouID

	// Read the shared agent type rather than repointing it; agents must live in the OU of their type.
	agentType, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "failed to read the default agent type")
	ts.agentOUID = agentType.OUID

	entityCategoryUserType.OUID = ts.ouID
	typeID, err := testutils.CreateUserType(entityCategoryUserType)
	ts.Require().NoError(err, "failed to create the test user type")
	ts.entityTypeID = typeID

	authFlowID, err := testutils.CreateFlow(buildCategoryAuthFlow())
	ts.Require().NoError(err, "failed to create the authentication flow")
	ts.authFlowID = authFlowID

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Governance Category Resource Server",
		Description: "Resource server the category acceptance tests bind their tokens to",
		Identifier:  categoryResourceIdentifier,
		OUID:        ts.ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err, "failed to create the resource server")
	ts.resourceServerID = resourceServerID

	ts.suffix = uniqueSuffix()
}

func (ts *EntityCategoryTestSuite) TearDownSuite() {
	for _, agentID := range ts.agentIDs {
		if err := testutils.DeleteAgent(agentID); err != nil {
			ts.T().Logf("teardown: failed to delete agent %s: %v", agentID, err)
		}
	}
	for _, appID := range ts.appIDs {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("teardown: failed to delete application %s: %v", appID, err)
		}
	}
	for _, userID := range ts.userIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("teardown: failed to delete user %s: %v", userID, err)
		}
	}
	if ts.authFlowID != "" {
		if err := testutils.DeleteFlow(ts.authFlowID); err != nil {
			ts.T().Logf("teardown: failed to delete flow %s: %v", ts.authFlowID, err)
		}
	}
	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServerWithChildren(ts.resourceServerID); err != nil {
			ts.T().Logf("teardown: failed to delete resource server: %v", err)
		}
	}
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("teardown: failed to delete user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("teardown: failed to delete organization unit: %v", err)
		}
	}
}

// An administrative hold reaches an agent, its read carries the hold's details, the token it held is
// revoked, and the release lands on ACTIVE. Both the status read and the refusal are asserted, since
// either alone is half a hold. An agent cannot be locked here (its shared type has no credential
// attribute), so an agent lock surviving a suspension is unit-tested only.
func (ts *EntityCategoryTestSuite) TestAnAdministrativeHoldAppliesToAnAgent() {
	agentID, clientID, clientSecret := ts.newAgent("hold")
	_, introspectorID, introspectorSecret := ts.newApplication("agent_introspector", false)

	held := ts.clientCredentialsToken(clientID, clientSecret)
	ts.Require().NotEmpty(held, "the agent must be able to obtain a token before the hold means anything")
	ts.Require().True(ts.introspectsActive(introspectorID, introspectorSecret, held),
		"the agent's token must be active before the hold")

	runAdministrationFlowWithInputs(ts.T(), agentSuspendFlowHandle, map[string]string{
		subjectInput:      agentID,
		operatorNoteInput: "Agent key reported leaked",
	})
	cutoffBound := time.Now()

	status := readEntityStatus(ts.T(), agentCollection, agentID)
	ts.Equal("SUSPENDED", status.Value, "a management read must report the hold on an agent")
	if ts.NotNil(status.Details, "an agent read must carry the suspension's details") {
		ts.Equal("ADMINISTRATIVE_SUSPENSION", status.Details.Reason)
		ts.NotEmpty(status.Details.Since, "the read must carry when the hold was placed")
		ts.Equal("Agent key reported leaked", status.Details.OperatorNote,
			"the operator's note must reach an agent's management read")
	}
	ts.False(ts.clientCredentialsSucceed(clientID, clientSecret),
		"a suspended agent must not obtain tokens")
	ts.False(ts.introspectsActive(introspectorID, introspectorSecret, held),
		"a token the agent held before the suspension must be refused")

	runAdministrationFlow(ts.T(), agentUnsuspendFlowHandle, agentID)

	ts.Equal("ACTIVE", readEntityStatus(ts.T(), agentCollection, agentID).Value,
		"unsuspending an agent must land on ACTIVE, not on a further hold")
	ts.False(ts.introspectsActive(introspectorID, introspectorSecret, held),
		"the release must not revive the token the suspension revoked")
	waitPastTheCutoff(cutoffBound)
	fresh := ts.clientCredentialsToken(clientID, clientSecret)
	ts.NotEmpty(fresh, "an unsuspended agent must be usable again without a second operator action")
	ts.True(ts.introspectsActive(introspectorID, introspectorSecret, fresh),
		"a token issued after the release and the cutoff must be usable")
}

// An agent acting as a client is contained too. A token the agent's client obtained by exchanging
// an application's token names the application in `sub`, so only the agent's client criterion reaches
// it; the application's own token, which the agent did not obtain, stays usable.
func (ts *EntityCategoryTestSuite) TestSuspendingAnAgentRevokesTheTokensItsClientObtained() {
	agentID, agentClientID, agentSecret := ts.newAgent("as_client", tokenExchangeGrant)
	_, appClientID, appSecret := ts.newApplication("exchange_subject", false)
	_, introspectorID, introspectorSecret := ts.newApplication("exchange_introspector", false)

	subjectToken := ts.clientCredentialsToken(appClientID, appSecret)
	ts.Require().NotEmpty(subjectToken, "the application must obtain the token the agent exchanges")
	exchanged := ts.exchangeToken(agentClientID, agentSecret, subjectToken)
	ts.Require().NotEmpty(exchanged, "the agent's client must be able to exchange the token first")
	ts.Require().True(ts.introspectsActive(introspectorID, introspectorSecret, exchanged),
		"the exchanged token must be active before the hold")

	runAdministrationFlow(ts.T(), agentSuspendFlowHandle, agentID)
	defer runAdministrationFlow(ts.T(), agentUnsuspendFlowHandle, agentID)

	ts.False(ts.introspectsActive(introspectorID, introspectorSecret, exchanged),
		"a token the agent's client obtained for another subject must be revoked with the agent")
	ts.True(ts.introspectsActive(introspectorID, introspectorSecret, subjectToken),
		"the application's own token is not the agent's and must survive")
}

// The same, for an application acting on its own behalf.
// An application read exposes only the status value, even when the suspension carried a note.
func (ts *EntityCategoryTestSuite) TestAnAdministrativeHoldAppliesToAnApplication() {
	appID, clientID, clientSecret := ts.newApplication("hold", false)

	ts.Require().True(ts.clientCredentialsSucceed(clientID, clientSecret),
		"the application must be able to obtain a token before the hold means anything")

	runAdministrationFlowWithInputs(ts.T(), applicationSuspendFlowHandle, map[string]string{
		subjectInput:      appID,
		operatorNoteInput: "Client secret found in a public repository",
	})

	status := readEntityStatus(ts.T(), applicationCollection, appID)
	ts.Equal("SUSPENDED", status.Value,
		"a management read must report the hold on an application")
	ts.Nil(status.Details, "an application's status must omit suspension details")
	body := ts.readRaw(applicationCollection, appID)
	ts.NotContains(body, "operatorNote", "an application read must never carry the operator note")
	ts.NotContains(body, "Client secret found", "an application read must never carry the operator note")
	ts.False(ts.clientCredentialsSucceed(clientID, clientSecret),
		"a suspended application must not obtain tokens")

	runAdministrationFlow(ts.T(), applicationUnsuspendFlowHandle, appID)

	ts.Equal("ACTIVE", readEntityStatus(ts.T(), applicationCollection, appID).Value,
		"unsuspending an application must land on ACTIVE")
	ts.True(ts.clientCredentialsSucceed(clientID, clientSecret),
		"an unsuspended application must be usable again")
}

// Suspending an application stops the users who sign in through it, not only the
// application acting for itself: a new sign-in cannot start, and one already started cannot continue.
// A full threshold of wrong secrets submitted to those continued sign-ins counts nothing against the
// user, whose own account is untouched.
func (ts *EntityCategoryTestSuite) TestSuspendingAnApplicationStopsUsersAuthenticatingThroughIt() {
	appID, clientID, _ := ts.newApplication("user_funnel", true)
	userID, username, password := ts.newUser("through_app")

	ts.Require().True(ts.authorizeReachesTheGate(clientID),
		"a user must be able to start signing in through the application first")
	started := make([]*testutils.FlowStep, 0, lockThreshold)
	for range lockThreshold {
		started = append(started, ts.startSignIn(clientID))
	}

	runAdministrationFlow(ts.T(), applicationSuspendFlowHandle, appID)

	ts.False(ts.authorizeReachesTheGate(clientID),
		"a suspended application must stop users authenticating through it")
	for i, step := range started {
		status, code := ts.continueSignIn(step, username, "DefinitelyNotTheSecret!"+fmt.Sprint(i))
		ts.NotEqual(http.StatusOK, status, "a sign-in started before the hold must not continue")
		ts.Equal("FES-1020", code, "the refusal must be the held application's")
	}

	user := readStatus(ts.T(), userID)
	ts.Equal("ACTIVE", user.Value, "the hold belongs to the application, so nothing may be counted")
	ts.Empty(user.lockedScopes(), "the user must carry no lock")
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the user must still be able to authenticate elsewhere")
}

// Neither category locks automatically under the shipped defaults, however many times it fails:
// a client secret is throttle-only. Failures run well past the user threshold, so a counter that
// started applying here could not hide under it.
func (ts *EntityCategoryTestSuite) TestAgentsAndApplicationsDoNotLockAutomatically() {
	agentID, agentClientID, agentSecret := ts.newAgent("no_lock")
	appID, appClientID, appSecret := ts.newApplication("no_lock", false)

	for range lockThreshold * 2 {
		ts.Require().False(ts.clientCredentialsSucceed(agentClientID, "WrongAgentSecret1!"),
			"the wrong secret must be refused")
		ts.Require().False(ts.clientCredentialsSucceed(appClientID, "WrongAppSecret1!"),
			"the wrong secret must be refused")
	}

	ts.Equal("ACTIVE", readEntityStatus(ts.T(), agentCollection, agentID).Value,
		"an agent must not lock automatically under the shipped defaults")
	ts.Equal("ACTIVE", readEntityStatus(ts.T(), applicationCollection, appID).Value,
		"an application must not lock automatically")

	ts.True(ts.clientCredentialsSucceed(agentClientID, agentSecret),
		"the agent must still be usable with its real secret")
	ts.True(ts.clientCredentialsSucceed(appClientID, appSecret),
		"the application must still be usable with its real secret")
}

// An agent's client_credentials token records lastLoginAt once agent recording is on, so an agent that
// only ever uses that grant is not reported as dormant.
func (ts *EntityCategoryTestSuite) TestAClientCredentialsTokenRecordsAnAgentsLastLogin() {
	agentID, clientID, clientSecret := ts.newAgent("last_login")
	ts.Require().True(ts.clientCredentialsSucceed(clientID, clientSecret), "the agent must obtain a token")
	ts.Empty(readLastLoginAt(ts.T(), agentCollection, agentID),
		"nothing may be recorded while agent recording is off, as shipped")

	policy := accountAccessPatch()["accountAccess"].(map[string]interface{})
	policy["agent"] = map[string]interface{}{"recordLastLogin": true, "lastLoginResolutionSeconds": 0}
	applyPolicy(ts.T(), policy)

	since := time.Now().UTC().Truncate(time.Second)
	ts.Require().True(ts.clientCredentialsSucceed(clientID, clientSecret), "the agent must obtain a token")

	recorded := readLastLoginAt(ts.T(), agentCollection, agentID)
	ts.Require().NotEmpty(recorded, "a client_credentials token must record the agent's lastLoginAt")
	at, err := time.Parse(time.RFC3339, recorded)
	ts.Require().NoError(err, "lastLoginAt must be an RFC 3339 instant, got %q", recorded)
	ts.False(at.Before(since), "lastLoginAt %s is earlier than the token request that set it", recorded)

	ts.False(ts.clientCredentialsSucceed(clientID, "WrongAgentSecret1!"), "the wrong secret must be refused")
	ts.Equal(recorded, readLastLoginAt(ts.T(), agentCollection, agentID),
		"a refused token request must not move lastLoginAt")
}

// newAgent creates one agent with a client-credentials profile, plus any extra grants, in the OU its
// type belongs to.
func (ts *EntityCategoryTestSuite) newAgent(name string, extraGrants ...string) (
	agentID, clientID, clientSecret string) {
	ts.T().Helper()

	clientID = fmt.Sprintf("governance_agent_%s_%s", name, ts.suffix)
	clientSecret = "Agent-Secret-" + name + "1!"

	agentID, err := testutils.CreateAgent(testutils.Agent{
		OUID:       ts.agentOUID,
		Type:       "default",
		Attributes: map[string]interface{}{"name": fmt.Sprintf("governance-category-%s-%s", name, ts.suffix)},
		AuthFlowID: ts.authFlowID,
		InboundAuthConfig: []testutils.AgentInboundAuthConfig{
			{
				Type: "oauth2",
				Config: &testutils.AgentOAuthConfig{
					ClientID:                clientID,
					ClientSecret:            clientSecret,
					GrantTypes:              append([]string{"client_credentials"}, extraGrants...),
					TokenEndpointAuthMethod: "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err, "failed to create the test agent")
	ts.agentIDs = append(ts.agentIDs, agentID)
	return agentID, clientID, clientSecret
}

// newApplication creates one application. withUserLogin adds the authorization-code grant and the
// user-facing authentication flow, for the case where a user signs in through it.
func (ts *EntityCategoryTestSuite) newApplication(name string, withUserLogin bool) (
	appID, clientID, clientSecret string) {
	ts.T().Helper()

	clientID = fmt.Sprintf("governance_app_%s_%s", name, ts.suffix)
	clientSecret = "App-Secret-" + name + "1!"

	grantTypes := []string{"client_credentials"}
	app := testutils.Application{
		OUID:             ts.ouID,
		Name:             fmt.Sprintf("Governance Category %s %s", name, ts.suffix),
		Description:      "Application the entity category acceptance tests drive",
		AllowedUserTypes: []string{entityCategoryUserType.Handle},
	}
	oauthConfig := map[string]interface{}{
		"clientId":                clientID,
		"clientSecret":            clientSecret,
		"redirectUris":            []string{categoryRedirectURI},
		"grantTypes":              grantTypes,
		"tokenEndpointAuthMethod": "client_secret_basic",
	}
	if withUserLogin {
		grantTypes = append(grantTypes, "authorization_code")
		oauthConfig["grantTypes"] = grantTypes
		// Response types belong to the authorization-code half only: a client-credentials-only
		// client is refused for declaring one.
		oauthConfig["responseTypes"] = []string{"code"}
		app.AuthFlowID = ts.authFlowID
	}
	app.InboundAuthConfig = []map[string]interface{}{
		{"type": "oauth2", "config": oauthConfig},
	}

	appID, err := testutils.CreateApplication(app)
	ts.Require().NoError(err, "failed to create the test application")
	ts.appIDs = append(ts.appIDs, appID)
	return appID, clientID, clientSecret
}

const categoryRedirectURI = "https://localhost:3000/callback"

// newUser creates one account nobody else in this package touches.
func (ts *EntityCategoryTestSuite) newUser(name string) (userID, username, password string) {
	ts.T().Helper()

	username = fmt.Sprintf("category_%s_%s", name, ts.suffix)
	password = "Correct-Horse-" + name + "1!"
	attributes, err := json.Marshal(map[string]interface{}{
		"username": username,
		"password": password,
	})
	ts.Require().NoError(err)

	userID, err = testutils.CreateUser(testutils.User{
		Type:       entityCategoryUserType.Handle,
		OUID:       ts.ouID,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "failed to create the test user")
	ts.userIDs = append(ts.userIDs, userID)
	return userID, username, password
}

// clientCredentialsSucceed reports whether the client can mint a token for itself.
func (ts *EntityCategoryTestSuite) clientCredentialsSucceed(clientID, clientSecret string) bool {
	ts.T().Helper()

	result, err := testutils.RequestTokenWithResource(clientID, clientSecret, "", "",
		"client_credentials", categoryResourceIdentifier)
	ts.Require().NoError(err, "the token request itself must reach the server")
	return result.StatusCode == http.StatusOK && result.Token != nil && result.Token.AccessToken != ""
}

// clientCredentialsToken returns the client's own access token, or nothing if it was refused.
func (ts *EntityCategoryTestSuite) clientCredentialsToken(clientID, clientSecret string) string {
	ts.T().Helper()

	result, err := testutils.RequestTokenWithResource(clientID, clientSecret, "", "",
		"client_credentials", categoryResourceIdentifier)
	ts.Require().NoError(err, "the token request itself must reach the server")
	if result.StatusCode != http.StatusOK || result.Token == nil {
		return ""
	}
	return result.Token.AccessToken
}

// exchangeToken exchanges a locally issued access token as the given client, or returns nothing if
// the exchange was refused.
func (ts *EntityCategoryTestSuite) exchangeToken(clientID, clientSecret, subjectToken string) string {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token", strings.NewReader(url.Values{
		"grant_type":         {tokenExchangeGrant},
		"subject_token":      {subjectToken},
		"subject_token_type": {"urn:ietf:params:oauth:token-type:access_token"},
	}.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var token testutils.TokenResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&token))
	return token.AccessToken
}

// introspectsActive reports whether introspection, as the given client, finds the token active.
func (ts *EntityCategoryTestSuite) introspectsActive(clientID, clientSecret, token string) bool {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/introspect",
		strings.NewReader(url.Values{"token": {token}}.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "introspection itself must answer")
	var result struct {
		Active bool `json:"active"`
	}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&result))
	return result.Active
}

// readRaw returns a management read's body as it came.
func (ts *EntityCategoryTestSuite) readRaw(collection, id string) string {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+"/"+collection+"/"+id, nil)
	ts.Require().NoError(err)
	req.Header.Set("Accept", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))
	return string(body)
}

// startSignIn starts a sign-in through the application and runs it to its credential prompt.
func (ts *EntityCategoryTestSuite) startSignIn(clientID string) *testutils.FlowStep {
	ts.T().Helper()

	resp, err := testutils.InitiateAuthorizationFlow(clientID, categoryRedirectURI, "code",
		"openid", "category-started-"+ts.suffix)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "authorize must redirect to the gate")
	_, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err)

	status, raw := ts.postFlow(map[string]interface{}{"executionId": executionID})
	ts.Require().Equal(http.StatusOK, status, raw)
	var step testutils.FlowStep
	ts.Require().NoError(json.Unmarshal([]byte(raw), &step))
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the sign-in must stop at the credential prompt")
	return &step
}

// continueSignIn submits credentials to a started sign-in and returns the status and error code.
func (ts *EntityCategoryTestSuite) continueSignIn(step *testutils.FlowStep, username,
	password string) (int, string) {
	ts.T().Helper()

	status, raw := ts.postFlow(map[string]interface{}{
		"executionId":    step.ExecutionID,
		"inputs":         map[string]string{"username": username, "password": password},
		"action":         "action_001",
		"challengeToken": step.ChallengeToken,
	})
	var read struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal([]byte(raw), &read)
	return status, read.Code
}

// postFlow posts one unauthenticated flow request.
func (ts *EntityCategoryTestSuite) postFlow(body map[string]interface{}) (int, string) {
	ts.T().Helper()

	payload, err := json.Marshal(body)
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, string(raw)
}

// authorizeReachesTheGate reports whether a user could begin signing in through this application.
// It does not assert which error a hold produces; that belongs to the enumeration review.
func (ts *EntityCategoryTestSuite) authorizeReachesTheGate(clientID string) bool {
	ts.T().Helper()

	resp, err := testutils.InitiateAuthorizationFlow(clientID, categoryRedirectURI, "code",
		"openid", "category-"+ts.suffix)
	ts.Require().NoError(err, "the authorize request itself must reach the server")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		return false
	}
	_, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	return err == nil && executionID != ""
}

// buildCategoryAuthFlow is a plain credential login: enough for a user to sign in through an
// application, with no session or SSO machinery this suite would then have to reason about.
func buildCategoryAuthFlow() testutils.Flow {
	return testutils.Flow{
		Name:     "Governance Entity Category Test Auth Flow",
		Handle:   "governance-entity-category-auth",
		FlowType: "AUTHENTICATION",
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
						"action": map[string]interface{}{
							"ref":      "action_001",
							"nextNode": "credentials_auth",
						},
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
				"onSuccess":    "auth_assert",
				"onIncomplete": "prompt_credentials",
			},
			{
				"id":   "auth_assert",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthAssertExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}
}
