// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	categoryResourceIdentifier = "https://governance-category.example.com"
	categoryRedirectURI        = "https://localhost:3000/callback"
	tokenExchangeGrant         = "urn:ietf:params:oauth:grant-type:token-exchange"
)

// EntityCategoryTestSuite checks how the feature treats agents and applications: an administrative
// hold applies to them, client-secret failures never lock, and unsuspending lands them on ACTIVE.
// Agents live in the OU the shared `default` agent type points at, so the type is not repointed.
type EntityCategoryTestSuite struct {
	suite.Suite
	users      *userFixture
	agentOUID  string
	authFlowID string
}

func TestEntityCategoryTestSuite(t *testing.T) {
	suite.Run(t, new(EntityCategoryTestSuite))
}

func (ts *EntityCategoryTestSuite) SetupSuite() {
	applyLockoutPolicy(ts.T())

	ts.users = newUserFixture(ts.T(), "category", testutils.UserType{Schema: credentialsSchema(nil)})

	// Read the shared agent type rather than repointing it; agents must live in the OU of their type.
	agentType, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "failed to read the default agent type")
	ts.agentOUID = agentType.OUID

	authFlowID, err := testutils.CreateFlow(passwordSignInFlow("Governance Entity Category Test Auth Flow",
		"governance-entity-category-auth", "AUTHENTICATION"))
	ts.Require().NoError(err, "failed to create the authentication flow")
	ts.authFlowID = authFlowID
	cleanupFlow(ts.T(), authFlowID)

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Governance Category Resource Server",
		Description: "Resource server the category acceptance tests bind their tokens to",
		Identifier:  categoryResourceIdentifier,
		OUID:        ts.users.ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err, "failed to create the resource server")
	t := ts.T()
	t.Cleanup(func() {
		if err := testutils.DeleteResourceServerWithChildren(resourceServerID); err != nil {
			t.Logf("teardown: failed to delete resource server: %v", err)
		}
	})
}

// An administrative hold reaches an agent, its read carries the hold's details, the token it held is
// revoked, and the release lands on ACTIVE. Both the status read and the refusal are asserted, since
// either alone is half a hold.
func (ts *EntityCategoryTestSuite) TestAnAdministrativeHoldAppliesToAnAgent() {
	agentID, clientID, clientSecret := ts.newAgent("hold")
	_, introspectorID, introspectorSecret := ts.newApplication("agent_introspector", false)

	held := ts.issuedToken(clientID, clientSecret, "the agent must be able to obtain a token before the hold")
	ts.Require().True(tokenActive(ts.T(), held, introspectorID, introspectorSecret),
		"the agent's token must be active before the hold")

	runAdministrationFlowAs(ts.T(), adminToken(ts.T()), agentSuspendFlowHandle, map[string]string{
		subjectInput:      agentID,
		operatorNoteInput: "Agent key reported leaked",
	})

	status := readEntityStatus(ts.T(), agentCollection, agentID)
	ts.Equal("SUSPENDED", status.Value, "a management read must report the hold on an agent")
	ts.Require().NotNil(status.Details, "an agent read must carry the suspension's details")
	ts.Equal("ADMINISTRATIVE_SUSPENSION", status.Details.Reason)
	ts.Equal([]string{"organization"}, status.Details.AppliedBy, "an operator's suspension is the organization's")
	cutoffBound, err := time.Parse(time.RFC3339Nano, status.Details.Since)
	ts.Require().NoError(err, "the read must carry when the hold was placed")
	ts.Equal("Agent key reported leaked", status.Details.OperatorNote,
		"the operator's note must reach an agent's management read")
	refused := ts.clientCredentials(clientID, clientSecret)
	ts.Equal(http.StatusUnauthorized, refused.StatusCode, "a suspended agent must not obtain tokens: %s", refused.Body)
	ts.Contains(string(refused.Body), "invalid_client", "a suspended agent must be refused as invalid_client")
	ts.False(tokenActive(ts.T(), held, introspectorID, introspectorSecret),
		"a token the agent held before the suspension must be refused")

	runAdministrationFlow(ts.T(), agentUnsuspendFlowHandle, agentID)

	released := readEntityStatus(ts.T(), agentCollection, agentID)
	ts.Equal("ACTIVE", released.Value, "unsuspending an agent must land on ACTIVE, not on a further hold")
	ts.Empty(released.lockedMethods(), "an agent's release must leave no lock")
	ts.False(tokenActive(ts.T(), held, introspectorID, introspectorSecret),
		"the release must not revive the token the suspension revoked")
	waitPastTheCutoff(cutoffBound)
	fresh := ts.issuedToken(clientID, clientSecret,
		"an unsuspended agent must be usable again without a second operator action")
	ts.True(tokenActive(ts.T(), fresh, introspectorID, introspectorSecret),
		"a token issued after the release and the cutoff must be usable")
}

// An agent acting as a client is contained too. A token the agent's client obtained by exchanging
// an application's token names the application in `sub`, so only the agent's client criterion reaches
// it; the application's own token, which the agent did not obtain, stays usable.
func (ts *EntityCategoryTestSuite) TestSuspendingAnAgentRevokesTheTokensItsClientObtained() {
	agentID, agentClientID, agentSecret := ts.newAgent("as_client", tokenExchangeGrant)
	appID, appClientID, appSecret := ts.newApplication("exchange_subject", false)
	_, introspectorID, introspectorSecret := ts.newApplication("exchange_introspector", false)

	subjectToken := ts.issuedToken(appClientID, appSecret, "the application must obtain the token the agent exchanges")
	exchanged := exchangeAccessToken(ts.T(), agentClientID, agentSecret, subjectToken)
	ts.Require().Equal(http.StatusOK, exchanged.StatusCode,
		"the agent's client must be able to exchange the token first: %s", exchanged.Body)
	claims, err := testutils.DecodeJWTPayloadMap(exchanged.Token.AccessToken)
	ts.Require().NoError(err)
	ts.Require().Equal(appID, claims["sub"], "the exchanged token must name the application, not the agent")
	ts.Require().True(tokenActive(ts.T(), exchanged.Token.AccessToken, introspectorID, introspectorSecret),
		"the exchanged token must be active before the hold")

	runAdministrationFlow(ts.T(), agentSuspendFlowHandle, agentID)
	defer runAdministrationFlow(ts.T(), agentUnsuspendFlowHandle, agentID)

	ts.False(tokenActive(ts.T(), exchanged.Token.AccessToken, introspectorID, introspectorSecret),
		"a token the agent's client obtained for another subject must be revoked with the agent")
	ts.True(tokenActive(ts.T(), subjectToken, introspectorID, introspectorSecret),
		"the application's own token is not the agent's and must survive")
}

// An administrative hold reaches an application acting on its own behalf, and its release lands on
// ACTIVE. An application read exposes only the status value, even when the suspension carried a note.
func (ts *EntityCategoryTestSuite) TestAnAdministrativeHoldAppliesToAnApplication() {
	appID, clientID, clientSecret := ts.newApplication("hold", false)
	ts.Require().Equal("ACTIVE", readEntityStatus(ts.T(), applicationCollection, appID).Value,
		"precondition: the application starts active")

	ts.issuedToken(clientID, clientSecret, "the application must be able to obtain a token before the hold")

	runAdministrationFlowAs(ts.T(), adminToken(ts.T()), applicationSuspendFlowHandle, map[string]string{
		subjectInput:      appID,
		operatorNoteInput: "Client secret found in a public repository",
	})

	status := readEntityStatus(ts.T(), applicationCollection, appID)
	ts.Equal("SUSPENDED", status.Value, "a management read must report the hold on an application")
	ts.Nil(status.Details, "an application's status must omit suspension details")
	body := string(getManagement(ts.T(), "/"+applicationCollection+"/"+appID))
	ts.NotContains(body, "operatorNote", "an application read must never carry the operator note")
	ts.NotContains(body, "Client secret found", "an application read must never carry the operator note")
	refused := ts.clientCredentials(clientID, clientSecret)
	ts.Equal(http.StatusUnauthorized, refused.StatusCode,
		"a suspended application must not obtain tokens: %s", refused.Body)
	ts.Contains(string(refused.Body), "invalid_client", "a suspended application must be refused as invalid_client")

	runAdministrationFlow(ts.T(), applicationUnsuspendFlowHandle, appID)

	ts.Equal("ACTIVE", readEntityStatus(ts.T(), applicationCollection, appID).Value,
		"unsuspending an application must land on ACTIVE")
	ts.Equal(http.StatusOK, ts.clientCredentials(clientID, clientSecret).StatusCode,
		"an unsuspended application must be usable again")
}

// The hold refuses client authentication itself, not only a secret check: a public client presents no
// secret, so before the hold an unknown code gets past client authentication to the grant, and after
// it the request stops there.
func (ts *EntityCategoryTestSuite) TestAHeldPublicClientIsRefusedAtClientAuthentication() {
	appID, clientID := newPublicClientApp(ts.T(), ts.users, "category_hold")

	admitted := redeemUnknownCode(ts.T(), clientID)
	ts.Require().Equal(http.StatusBadRequest, admitted.StatusCode,
		"precondition: the public client must authenticate and reach the grant: %s", admitted.Body)
	ts.Require().Contains(string(admitted.Body), "invalid_grant",
		"precondition: the unknown code must be the grant's refusal")

	runAdministrationFlow(ts.T(), applicationSuspendFlowHandle, appID)
	defer runAdministrationFlow(ts.T(), applicationUnsuspendFlowHandle, appID)

	refused := redeemUnknownCode(ts.T(), clientID)
	ts.Equal(http.StatusUnauthorized, refused.StatusCode,
		"a suspended public client must be refused at client authentication: %s", refused.Body)
	ts.Contains(string(refused.Body), "invalid_client", "a suspended public client must be refused as invalid_client")
}

// A wrong client secret on a suspended application or agent gets exactly the answer a wrong secret on
// an active one gets: a client secret is not something a lock limits guessing, so its failure states no
// hold, and the suspension is not disclosed to a caller who does not hold the secret.
func (ts *EntityCategoryTestSuite) TestAWrongClientSecretOnASuspendedClientIsAPlainFailure() {
	suspendedAppID, suspendedAppClientID, _ := ts.newApplication("wrong_secret_held", false)
	_, activeAppClientID, _ := ts.newApplication("wrong_secret_live", false)
	runAdministrationFlow(ts.T(), applicationSuspendFlowHandle, suspendedAppID)
	suspendedAgentID, suspendedAgentClientID, _ := ts.newAgent("wrong_secret_held")
	_, activeAgentClientID, _ := ts.newAgent("wrong_secret_live")
	runAdministrationFlow(ts.T(), agentSuspendFlowHandle, suspendedAgentID)

	for _, client := range []struct{ name, activeClientID, suspendedClientID string }{
		{"application", activeAppClientID, suspendedAppClientID},
		{"agent", activeAgentClientID, suspendedAgentClientID},
	} {
		active := clientCredentials(ts.T(), client.activeClientID, "WrongClientSecret1!", "")
		held := clientCredentials(ts.T(), client.suspendedClientID, "WrongClientSecret1!", "")

		ts.Require().NotEqual(http.StatusOK, active.StatusCode, "%s: the wrong secret must be refused", client.name)
		ts.Equal(active.StatusCode, held.StatusCode,
			"%s: a wrong secret on a suspended client must keep a wrong secret's status", client.name)
		ts.JSONEq(string(active.Body), string(held.Body),
			"%s: a wrong secret on a suspended client must answer as any wrong secret", client.name)
		ts.NotContains(string(held.Body), statedSuspensionCode,
			"%s: a wrong client secret must not state the suspension", client.name)
	}
}

// Suspending an application stops the users who sign in through it, not only the
// application acting for itself: a new sign-in cannot start, and one already started cannot continue.
// A full threshold of wrong secrets submitted to those continued sign-ins counts nothing against the
// user, whose own account is untouched.
func (ts *EntityCategoryTestSuite) TestSuspendingAnApplicationStopsUsersAuthenticatingThroughIt() {
	appID, clientID, _ := ts.newApplication("user_funnel", true)
	userID, username, password := ts.users.newUser(ts.T(), "through_app", nil)

	started := make([]*testutils.FlowStep, 0, lockThreshold)
	for range lockThreshold {
		started = append(started, ts.startSignIn(clientID))
	}

	runAdministrationFlow(ts.T(), applicationSuspendFlowHandle, appID)

	// A held application is shown as unavailable on the error page, where a person reads it, and its
	// redirect URI is never followed.
	status, location := ts.authorize(clientID, "category-held-"+ts.users.suffix)
	ts.Equal(http.StatusFound, status, "authorize must send the browser to the error page")
	refused, err := url.Parse(location)
	ts.Require().NoError(err, "authorize must answer with a readable Location")
	ts.NotEqual(categoryRedirectURI, refused.Scheme+"://"+refused.Host+refused.Path,
		"a suspended application's redirect URI must not be followed")
	ts.Empty(refused.Query().Get("executionId"), "a suspended application must not start a sign-in")
	ts.Equal("unauthorized_client", refused.Query().Get("errorCode"),
		"a suspended application must stop users authenticating through it")
	ts.Equal("This application is currently unavailable", refused.Query().Get("errorMessage"),
		"the error page must say the application is unavailable")

	for i, step := range started {
		answer := answerOf(postFlow(ts.T(), nil, "", map[string]interface{}{
			"executionId":    step.ExecutionID,
			"inputs":         map[string]string{"username": username, "password": fmt.Sprintf("NotTheSecret%d!", i)},
			"action":         signInAction,
			"challengeToken": step.ChallengeToken,
		}))
		ts.NotEqual(http.StatusOK, answer.status, "a sign-in started before the hold must not continue")
		ts.Equal("FES-1020", answer.code, "the refusal must be the held application's")
	}

	user := readStatus(ts.T(), userID)
	ts.Equal("ACTIVE", user.Value, "the hold belongs to the application, so nothing may be counted")
	ts.Empty(user.lockedMethods(), "the user must carry no lock")
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the user must still be able to authenticate elsewhere")
}

// A flow started directly, without authorize, is refused for a suspended application as one already
// started is. Registration is used because an application's sign-in cannot be started directly: a
// redirect client's is refused as not permitted, and an embedded one's flow secret is itself refused.
func (ts *EntityCategoryTestSuite) TestASuspendedApplicationCannotStartAFlowDirectly() {
	appID := newShippedFlowApp(ts.T(), ts.users, "category_direct")
	start := map[string]interface{}{"applicationId": appID, "flowType": "REGISTRATION"}
	ts.Require().Equal("INCOMPLETE", answerOf(postFlow(ts.T(), nil, "", start)).flowStatus,
		"precondition: the application must be able to start a registration directly")

	runAdministrationFlow(ts.T(), applicationSuspendFlowHandle, appID)
	defer runAdministrationFlow(ts.T(), applicationUnsuspendFlowHandle, appID)

	ts.Equal("FES-1020", answerOf(postFlow(ts.T(), nil, "", start)).code,
		"a flow started directly must be refused as the held application's")
}

// Neither category locks automatically under the shipped defaults, however many times it fails:
// a client secret is throttle-only. Failures run well past the user threshold, so a counter that
// started applying here could not hide under it.
func (ts *EntityCategoryTestSuite) TestAgentsAndApplicationsDoNotLockAutomatically() {
	agentID, agentClientID, agentSecret := ts.newAgent("no_lock")
	appID, appClientID, appSecret := ts.newApplication("no_lock", false)

	for range lockThreshold * 2 {
		ts.Require().NotEqual(http.StatusOK, ts.clientCredentials(agentClientID, "WrongAgentSecret1!").StatusCode,
			"the wrong secret must be refused")
		ts.Require().NotEqual(http.StatusOK, ts.clientCredentials(appClientID, "WrongAppSecret1!").StatusCode,
			"the wrong secret must be refused")
	}

	ts.Equal("ACTIVE", readEntityStatus(ts.T(), agentCollection, agentID).Value,
		"an agent must not lock automatically under the shipped defaults")
	ts.Equal("ACTIVE", readEntityStatus(ts.T(), applicationCollection, appID).Value,
		"an application must not lock automatically")

	ts.Equal(http.StatusOK, ts.clientCredentials(agentClientID, agentSecret).StatusCode,
		"the agent must still be usable with its real secret")
	ts.Equal(http.StatusOK, ts.clientCredentials(appClientID, appSecret).StatusCode,
		"the application must still be usable with its real secret")
}

// An agent's client_credentials token records lastLoginAt once agent recording is on, so an agent that
// only ever uses that grant is not reported as dormant.
func (ts *EntityCategoryTestSuite) TestAClientCredentialsTokenRecordsAnAgentsLastLogin() {
	agentID, clientID, clientSecret := ts.newAgent("last_login")
	ts.issuedToken(clientID, clientSecret, "the agent must obtain a token")
	ts.Empty(readLastLoginAt(ts.T(), agentCollection, agentID),
		"nothing may be recorded while agent recording is off, as shipped")

	applyPolicy(ts.T(), lockoutPolicyWith(map[string]interface{}{
		"agent": map[string]interface{}{"recordLastLogin": true, "lastLoginResolutionSeconds": 0},
	}))

	since := time.Now().UTC().Truncate(time.Second)
	ts.issuedToken(clientID, clientSecret, "the agent must obtain a token")

	recorded := requireRecordedSince(ts.T(), agentCollection, agentID, since, "a client_credentials token")
	awaitTheSecondAfter(ts.T(), recorded)

	ts.NotEqual(http.StatusOK, ts.clientCredentials(clientID, "WrongAgentSecret1!").StatusCode,
		"the wrong secret must be refused")
	ts.Equal(recorded, readLastLoginAt(ts.T(), agentCollection, agentID),
		"a refused token request must not move lastLoginAt")
}

// newAgent creates one agent with a client-credentials profile, plus any extra grants, in the OU its
// type belongs to.
func (ts *EntityCategoryTestSuite) newAgent(name string, extraGrants ...string) (
	agentID, clientID, clientSecret string) {
	ts.T().Helper()

	clientID = fmt.Sprintf("governance_agent_%s_%s", name, ts.users.suffix)
	clientSecret = "Agent-Secret-" + name + "1!"

	agentID, err := testutils.CreateAgent(testutils.Agent{
		OUID:       ts.agentOUID,
		Type:       "default",
		Attributes: map[string]interface{}{"name": fmt.Sprintf("governance-category-%s-%s", name, ts.users.suffix)},
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
	t := ts.T()
	t.Cleanup(func() {
		if err := testutils.DeleteAgent(agentID); err != nil {
			t.Logf("teardown: failed to delete agent %s: %v", agentID, err)
		}
	})
	return agentID, clientID, clientSecret
}

// newApplication creates one application. withUserLogin adds the authorization-code grant and the
// user-facing authentication flow, for the case where a user signs in through it.
func (ts *EntityCategoryTestSuite) newApplication(name string, withUserLogin bool) (
	appID, clientID, clientSecret string) {
	ts.T().Helper()

	clientID = fmt.Sprintf("governance_app_%s_%s", name, ts.users.suffix)
	clientSecret = "App-Secret-" + name + "1!"

	grantTypes := []string{"client_credentials"}
	app := testutils.Application{
		OUID:             ts.users.ouID,
		Name:             fmt.Sprintf("Governance Category %s %s", name, ts.users.suffix),
		Description:      "Application the entity category acceptance tests drive",
		AllowedUserTypes: []string{ts.users.userType},
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
	cleanupApplication(ts.T(), appID)
	return appID, clientID, clientSecret
}

// clientCredentials requests the client's own token bound to the suite's resource server.
func (ts *EntityCategoryTestSuite) clientCredentials(clientID, clientSecret string) *testutils.TokenHTTPResult {
	ts.T().Helper()
	return clientCredentials(ts.T(), clientID, clientSecret, categoryResourceIdentifier)
}

// issuedToken requires the client to obtain its own token and returns it.
func (ts *EntityCategoryTestSuite) issuedToken(clientID, clientSecret, msg string) string {
	ts.T().Helper()
	result := ts.clientCredentials(clientID, clientSecret)
	ts.Require().Equal(http.StatusOK, result.StatusCode, "%s: %s", msg, result.Body)
	ts.Require().NotEmpty(result.Token.AccessToken, msg)
	return result.Token.AccessToken
}

// startSignIn starts a sign-in through the application and runs it to its credential prompt.
func (ts *EntityCategoryTestSuite) startSignIn(clientID string) *testutils.FlowStep {
	ts.T().Helper()

	resp, err := testutils.InitiateAuthorizationFlow(clientID, categoryRedirectURI, "code",
		"openid", "category-started-"+ts.users.suffix)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "authorize must redirect to the gate")
	_, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err)

	status, raw := postFlow(ts.T(), nil, "", map[string]interface{}{"executionId": executionID})
	ts.Require().Equal(http.StatusOK, status, raw)
	var step testutils.FlowStep
	ts.Require().NoError(json.Unmarshal([]byte(raw), &step))
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the sign-in must stop at the credential prompt")
	return &step
}

// authorize sends the authorization request a user's browser would and returns the status and the
// Location the server answered with.
func (ts *EntityCategoryTestSuite) authorize(clientID, state string) (int, string) {
	ts.T().Helper()

	resp, err := testutils.InitiateAuthorizationFlow(clientID, categoryRedirectURI, "code", "openid", state)
	ts.Require().NoError(err, "the authorize request itself must reach the server")
	defer resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Location")
}
