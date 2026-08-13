// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package observability

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
	obsRedirectURI = "https://localhost:3000"

	obsWebClientID          = "obs_principal_web_client"
	obsWebClientSecret      = "obs_principal_web_secret"
	obsM2MClientID          = "obs_principal_m2m_client"
	obsM2MClientSecret      = "obs_principal_m2m_secret"
	obsExchangeClientID     = "obs_principal_exchange_client"
	obsExchangeClientSecret = "obs_principal_exchange_secret"
	obsAgentClientID        = "obs_principal_agent_client"
	obsAgentClientSecret    = "obs_principal_agent_secret"

	obsUserTypeName    = "obs-principal-person"
	obsSubjectUsername = "obs_principal_subject"
	obsSubjectPassword = "ObsPrincipalSubject1!"
	obsActorUsername   = "obs_principal_actor"
	obsActorPassword   = "ObsPrincipalActor1!"

	obsResourceIdentifier = "https://obs-principal.example.com"

	// obsCredentialsNodeID is the flow node that authenticates the subject. Its own completion event
	// is the first one that can name the subject, so it is what the subject assertions target.
	obsCredentialsNodeID = "credentials_auth"

	obsTokenExchangeGrant = "urn:ietf:params:oauth:grant-type:token-exchange"
	obsAccessTokenType    = "urn:ietf:params:oauth:token-type:access_token"
)

var obsUserType = testutils.UserType{
	Name: obsUserTypeName,
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
		"email":    map[string]interface{}{"type": "string"},
	},
}

// obsAuthFlow is a plain username/password authentication flow. The subject is resolved by the
// CredentialsAuthExecutor node, and AuthAssertExecutor mints the assertion the authorization code is
// created from, which is where the subject identity and correlation id cross from flow to token.
var obsAuthFlow = testutils.Flow{
	Name:     "Observability Principal Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_obs_principal_test",
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
					"action": map[string]interface{}{"ref": "action_001", "nextNode": obsCredentialsNodeID},
				},
			},
		},
		{
			"id":   obsCredentialsNodeID,
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
			"id":        "auth_assert",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
			"onSuccess": "end",
		},
		{"id": "end", "type": "END"},
	},
}

// PrincipalEventsTestSuite drives real authentication and token requests against a server with the
// observability file sink enabled, then reads the emitted events back to assert they describe the
// principals involved: the entity that acted (act_type, app_id, client_id), the entity the token was
// issued for (sub, sub_type), whether the two differ (is_delegated, act_sub), and the correlation id
// that stitches a login flow to the tokens issued from it.
type PrincipalEventsTestSuite struct {
	suite.Suite
	client *http.Client
	reader *eventLogReader

	ouID              string
	entityTypeID      string
	agentTypeSnapshot *testutils.AgentTypeSnapshot
	resourceServerID  string
	authFlowID        string
	webAppID          string
	m2mAppID          string
	exchangeAppID     string
	agentID           string
	subjectUserID     string
	actorUserID       string
}

func TestPrincipalEventsTestSuite(t *testing.T) {
	suite.Run(t, new(PrincipalEventsTestSuite))
}

// SetupSuite provisions every resource the scenarios need and only then turns the sink on, so the
// log holds the events under test rather than the setup traffic that produced the fixtures.
func (ts *PrincipalEventsTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()
	ts.reader = newEventLogReader()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "obs-principal-ou",
		Name:        "Observability Principal OU",
		Description: "Organization unit for principal-aware observability event tests",
	})
	ts.Require().NoError(err, "Failed to create the test organization unit")
	ts.ouID = ouID

	obsUserType.OUID = ouID
	entityTypeID, err := testutils.CreateUserType(obsUserType)
	ts.Require().NoError(err, "Failed to create the test user type")
	ts.entityTypeID = entityTypeID

	// The `default` agent type is a singleton every suite shares. Snapshot it before pointing it at
	// this suite's OU so teardown can put it back before that OU is deleted.
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "Failed to snapshot the default agent type")
	ts.agentTypeSnapshot = snapshot

	_, err = testutils.CreateAgentType(testutils.UserType{
		Name: "default",
		OUID: ouID,
		Schema: map[string]interface{}{
			"description": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "Failed to point the default agent type at the test OU")

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Observability Principal Resource Server",
		Description: "Resource server for principal-aware observability event tests",
		Identifier:  obsResourceIdentifier,
		OUID:        ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err, "Failed to create the test resource server")
	ts.resourceServerID = resourceServerID

	authFlowID, err := testutils.CreateFlow(obsAuthFlow)
	ts.Require().NoError(err, "Failed to create the test authentication flow")
	ts.authFlowID = authFlowID

	ts.webAppID = ts.createApplication("ObsPrincipalWebApp", map[string]interface{}{
		"clientId":                obsWebClientID,
		"clientSecret":            obsWebClientSecret,
		"redirectUris":            []string{obsRedirectURI},
		"grantTypes":              []string{"authorization_code", "refresh_token"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "client_secret_basic",
	}, true)

	ts.m2mAppID = ts.createApplication("ObsPrincipalM2MApp", map[string]interface{}{
		"clientId":                obsM2MClientID,
		"clientSecret":            obsM2MClientSecret,
		"grantTypes":              []string{"client_credentials"},
		"tokenEndpointAuthMethod": "client_secret_basic",
	}, false)

	ts.exchangeAppID = ts.createApplication("ObsPrincipalExchangeApp", map[string]interface{}{
		"clientId":                obsExchangeClientID,
		"clientSecret":            obsExchangeClientSecret,
		"grantTypes":              []string{obsTokenExchangeGrant},
		"tokenEndpointAuthMethod": "client_secret_basic",
	}, false)

	ts.agentID = ts.createAgent()

	ts.subjectUserID = ts.createUser(obsSubjectUsername, obsSubjectPassword)
	ts.actorUserID = ts.createUser(obsActorUsername, obsActorPassword)

	ts.Require().NoError(testutils.PatchDeploymentConfig(observabilityPatch(true)),
		"Failed to enable observability in the deployment config")
	ts.Require().NoError(testutils.RestartServer(), "Failed to restart the server with observability enabled")
}

// TearDownSuite turns the sink back off before removing the fixtures, so the restore lands while
// every resource the running server still references exists.
func (ts *PrincipalEventsTestSuite) TearDownSuite() {
	if err := testutils.PatchDeploymentConfig(observabilityPatch(false)); err != nil {
		ts.T().Errorf("teardown: failed to disable observability: %v", err)
	} else if err := testutils.RestartServer(); err != nil {
		ts.T().Errorf("teardown: failed to restart the server after disabling observability: %v", err)
	}

	if ts.subjectUserID != "" {
		_ = testutils.DeleteUser(ts.subjectUserID)
	}
	if ts.actorUserID != "" {
		_ = testutils.DeleteUser(ts.actorUserID)
	}
	if ts.agentID != "" {
		_ = testutils.DeleteAgent(ts.agentID)
	}
	for _, appID := range []string{ts.webAppID, ts.m2mAppID, ts.exchangeAppID} {
		if appID != "" {
			_ = testutils.DeleteApplication(appID)
		}
	}
	if ts.authFlowID != "" {
		_ = testutils.DeleteFlow(ts.authFlowID)
	}
	if ts.resourceServerID != "" {
		_ = testutils.DeleteResourceServerWithChildren(ts.resourceServerID)
	}
	// Restore the shared agent type before deleting the OU it points at, or the singleton is left
	// referencing a deleted OU and a later suite's restore fails.
	if ts.agentTypeSnapshot != nil {
		if err := testutils.RestoreAgentType(ts.agentTypeSnapshot); err != nil {
			ts.T().Errorf("teardown: failed to restore the default agent type: %v", err)
		}
	}
	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// TestAuthorizationCode_FlowEventsCarryTheActingApplication verifies that every event of a login
// flow names the application the flow runs for by resource ID, OAuth client_id and principal type,
// and that they all share the execution id as both trace and correlation id.
func (ts *PrincipalEventsTestSuite) TestAuthorizationCode_FlowEventsCarryTheActingApplication() {
	ts.Require().NoError(ts.reader.reset())

	executionID, _ := ts.runLoginFlow(obsWebClientID, obsSubjectUsername, obsSubjectPassword)

	flowCompleted, err := ts.reader.await(ofTypeWith(typeFlowCompleted, keyExecutionID, executionID))
	ts.Require().NoError(err, "No FLOW_COMPLETED event for execution %s", executionID)

	flowStarted, err := ts.reader.await(ofTypeWith(typeFlowStarted, keyExecutionID, executionID))
	ts.Require().NoError(err, "No FLOW_STARTED event for execution %s", executionID)

	for _, evt := range []observabilityEvent{flowStarted, flowCompleted} {
		ts.Equal(executionID, evt.TraceID,
			"%s must be traced on the execution id so the whole flow shares one trace", evt.Type)
		ts.Equal(executionID, evt.str(keyCorrelationID),
			"%s must correlate on the execution id", evt.Type)
		ts.Equal(ts.webAppID, evt.str(keyAppID), "%s must name the application's resource ID", evt.Type)
		ts.Equal(obsWebClientID, evt.str(keyClientID), "%s must name the application's OAuth client_id", evt.Type)
		ts.Equal(principalApplication, evt.str(keyActorType),
			"%s must report an application-driven flow as an application actor", evt.Type)
	}
}

// TestAuthorizationCode_AuthenticatingNodeReportsTheSubject verifies that the node that resolves the
// subject reports it on its own completion event, by opaque resource ID and principal type, rather
// than only from the next node onwards.
func (ts *PrincipalEventsTestSuite) TestAuthorizationCode_AuthenticatingNodeReportsTheSubject() {
	ts.Require().NoError(ts.reader.reset())

	executionID, _ := ts.runLoginFlow(obsWebClientID, obsSubjectUsername, obsSubjectPassword)

	nodeCompleted, err := ts.reader.await(func(evt observabilityEvent) bool {
		return evt.Type == typeNodeExecCompleted &&
			evt.str(keyExecutionID) == executionID &&
			evt.str(keyNodeID) == obsCredentialsNodeID
	})
	ts.Require().NoError(err, "No completion event for node %s of execution %s", obsCredentialsNodeID, executionID)

	ts.Equal(ts.subjectUserID, nodeCompleted.str(keySubject),
		"The authenticating node must report the subject it just resolved, as its resource ID")
	ts.Equal(principalUser, nodeCompleted.str(keySubjectType),
		"A user authenticated by credentials must be reported as a user subject")
	ts.NotEqual(obsSubjectUsername, nodeCompleted.str(keySubject),
		"The subject must be the opaque resource ID, never a directly identifying attribute")
}

// TestAuthorizationCode_TokenEventStitchesToTheLoginFlow verifies that the token issued against a
// login flow's authorization code reports the same correlation id as the flow's own events, and
// names the authenticated user as its subject and the application as its actor.
func (ts *PrincipalEventsTestSuite) TestAuthorizationCode_TokenEventStitchesToTheLoginFlow() {
	ts.Require().NoError(ts.reader.reset())

	executionID, code := ts.runLoginFlow(obsWebClientID, obsSubjectUsername, obsSubjectPassword)

	tokenResult, err := testutils.RequestToken(
		obsWebClientID, obsWebClientSecret, code, obsRedirectURI, "authorization_code")
	ts.Require().NoError(err, "Failed to request a token")
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, string(tokenResult.Body))

	issued, err := ts.reader.await(ofTypeWith(typeTokenIssued, keyCorrelationID, executionID))
	ts.Require().NoError(err, "No TOKEN_ISSUED event correlated to execution %s", executionID)

	ts.Equal("authorization_code", issued.str(keyGrantType))
	ts.Equal(ts.subjectUserID, issued.str(keySubject),
		"The token event must report the authenticated user as its subject")
	ts.Equal(principalUser, issued.str(keySubjectType))
	ts.Equal(ts.webAppID, issued.str(keyAppID))
	ts.Equal(obsWebClientID, issued.str(keyClientID))
	ts.Equal(principalApplication, issued.str(keyActorType))
	ts.Equal(false, issued.Data[keyIsDelegated],
		"A user signing in for themselves is not a delegated grant")
	ts.False(issued.has(keyActorSub), "A non-delegated grant has no acting principal to report")
}

// TestClientCredentials_ApplicationIsBothActorAndSubject verifies that an application's own
// client_credentials token reports that application on both axes: it is the principal acting and the
// principal the token is about.
func (ts *PrincipalEventsTestSuite) TestClientCredentials_ApplicationIsBothActorAndSubject() {
	ts.Require().NoError(ts.reader.reset())

	status, body := ts.requestClientCredentialsToken(obsM2MClientID, obsM2MClientSecret)
	ts.Require().Equal(http.StatusOK, status, "%v", body)

	issued, err := ts.reader.await(ofTypeWith(typeTokenIssued, keyClientID, obsM2MClientID))
	ts.Require().NoError(err, "No TOKEN_ISSUED event for client %s", obsM2MClientID)

	ts.Equal("client_credentials", issued.str(keyGrantType))
	ts.Equal(ts.m2mAppID, issued.str(keyAppID))
	ts.Equal(principalApplication, issued.str(keyActorType))
	ts.Equal(ts.m2mAppID, issued.str(keySubject),
		"A client_credentials token is about the client itself")
	ts.Equal(principalApplication, issued.str(keySubjectType))
	ts.Equal(false, issued.Data[keyIsDelegated])
}

// TestClientCredentials_AgentIsReportedAsAnAgent verifies that an agent's client_credentials token is
// distinguishable from an application's without a registry lookup, on both the actor and the subject
// axis. This is what keeps agent activity separable in the sinks.
func (ts *PrincipalEventsTestSuite) TestClientCredentials_AgentIsReportedAsAnAgent() {
	ts.Require().NoError(ts.reader.reset())

	status, body := ts.requestClientCredentialsToken(obsAgentClientID, obsAgentClientSecret)
	ts.Require().Equal(http.StatusOK, status, "%v", body)

	issued, err := ts.reader.await(ofTypeWith(typeTokenIssued, keyClientID, obsAgentClientID))
	ts.Require().NoError(err, "No TOKEN_ISSUED event for agent client %s", obsAgentClientID)

	ts.Equal(ts.agentID, issued.str(keyAppID))
	ts.Equal(principalAgent, issued.str(keyActorType),
		"An agent authenticating as the client must be reported as an agent actor")
	ts.Equal(ts.agentID, issued.str(keySubject))
	ts.Equal(principalAgent, issued.str(keySubjectType),
		"An agent's own token must be reported as an agent subject, not assumed to be a user")
}

// TestRefreshToken_ReportsTheOriginalSubject verifies that refreshing carries the subject principal
// forward: the refreshed token's event names the same user the login flow authenticated, even though
// the refresh request itself carries no flow and no user interaction.
func (ts *PrincipalEventsTestSuite) TestRefreshToken_ReportsTheOriginalSubject() {
	_, code := ts.runLoginFlow(obsWebClientID, obsSubjectUsername, obsSubjectPassword)

	tokenResult, err := testutils.RequestToken(
		obsWebClientID, obsWebClientSecret, code, obsRedirectURI, "authorization_code")
	ts.Require().NoError(err, "Failed to request a token")
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, string(tokenResult.Body))
	ts.Require().NotEmpty(tokenResult.Token.RefreshToken, "The grant must issue a refresh token")

	// Reset only now, so the awaited event can only be the refresh grant's, never the login's.
	ts.Require().NoError(ts.reader.reset())

	refreshed, err := testutils.RefreshAccessToken(obsWebClientID, obsWebClientSecret, tokenResult.Token.RefreshToken)
	ts.Require().NoError(err, "Failed to refresh the access token")
	ts.Require().NotEmpty(refreshed.AccessToken)

	issued, err := ts.reader.await(ofTypeWith(typeTokenIssued, keyGrantType, "refresh_token"))
	ts.Require().NoError(err, "No TOKEN_ISSUED event for the refresh grant")

	ts.Equal(ts.subjectUserID, issued.str(keySubject),
		"A refreshed token must still report the user the original login authenticated")
	ts.Equal(principalUser, issued.str(keySubjectType))
	ts.Equal(ts.webAppID, issued.str(keyAppID))
	ts.Equal(false, issued.Data[keyIsDelegated])
}

// TestTokenExchange_DelegatedGrantReportsActorAndSubject verifies that an exchange presenting an
// actor_token is observable as delegation: the acting principal is reported separately from the
// principal the token is about, and the grant is flagged as delegated.
func (ts *PrincipalEventsTestSuite) TestTokenExchange_DelegatedGrantReportsActorAndSubject() {
	subjectToken := ts.obtainAccessToken(obsSubjectUsername, obsSubjectPassword)
	actorToken := ts.obtainAccessToken(obsActorUsername, obsActorPassword)

	ts.Require().NoError(ts.reader.reset())

	form := url.Values{}
	form.Set("grant_type", obsTokenExchangeGrant)
	form.Set("subject_token", subjectToken)
	form.Set("subject_token_type", obsAccessTokenType)
	form.Set("actor_token", actorToken)
	form.Set("actor_token_type", obsAccessTokenType)

	status, body := ts.postToken(form, obsExchangeClientID, obsExchangeClientSecret)
	ts.Require().Equal(http.StatusOK, status, "%v", body)

	issued, err := ts.reader.await(ofTypeWith(typeTokenIssued, keyClientID, obsExchangeClientID))
	ts.Require().NoError(err, "No TOKEN_ISSUED event for the exchange client")

	ts.Equal(ts.subjectUserID, issued.str(keySubject),
		"The subject must remain the principal the token is about")
	ts.Equal(principalUser, issued.str(keySubjectType))
	ts.Equal(true, issued.Data[keyIsDelegated], "An exchange with an actor_token is a delegated grant")
	ts.Equal(ts.actorUserID, issued.str(keyActorSub),
		"The acting principal must be reported alongside the subject it acts for")
}

// TestTokenExchange_UndelegatedGrantIsNotFlaggedAsDelegated verifies that the delegation flag tracks
// the actor_token rather than the grant type: the same exchange without one is not delegation.
func (ts *PrincipalEventsTestSuite) TestTokenExchange_UndelegatedGrantIsNotFlaggedAsDelegated() {
	subjectToken := ts.obtainAccessToken(obsSubjectUsername, obsSubjectPassword)

	ts.Require().NoError(ts.reader.reset())

	form := url.Values{}
	form.Set("grant_type", obsTokenExchangeGrant)
	form.Set("subject_token", subjectToken)
	form.Set("subject_token_type", obsAccessTokenType)

	status, body := ts.postToken(form, obsExchangeClientID, obsExchangeClientSecret)
	ts.Require().Equal(http.StatusOK, status, "%v", body)

	issued, err := ts.reader.await(ofTypeWith(typeTokenIssued, keyClientID, obsExchangeClientID))
	ts.Require().NoError(err, "No TOKEN_ISSUED event for the exchange client")

	ts.Equal(ts.subjectUserID, issued.str(keySubject))
	ts.Equal(false, issued.Data[keyIsDelegated])
	ts.False(issued.has(keyActorSub), "An exchange without an actor_token has no acting principal")
}

// TestTokenIssuanceFailed_ReportsTheActingClient verifies that a rejected token request still names
// the principal that made it, so failures are attributable to an actor rather than only to a
// client_id string.
func (ts *PrincipalEventsTestSuite) TestTokenIssuanceFailed_ReportsTheActingClient() {
	ts.Require().NoError(ts.reader.reset())

	// The M2M client is registered for client_credentials only, so an authorization_code request is
	// rejected after the client is resolved but before any grant handling.
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", "not-a-real-code")
	form.Set("redirect_uri", obsRedirectURI)

	status, body := ts.postToken(form, obsM2MClientID, obsM2MClientSecret)
	ts.Require().Equal(http.StatusBadRequest, status, "%v", body)
	ts.Require().Equal("unauthorized_client", body["error"])

	failed, err := ts.reader.await(ofTypeWith(typeTokenIssuanceFailed, keyClientID, obsM2MClientID))
	ts.Require().NoError(err, "No TOKEN_ISSUANCE_FAILED event for client %s", obsM2MClientID)

	ts.Equal(ts.m2mAppID, failed.str(keyAppID), "A failure must name the acting application's resource ID")
	ts.Equal(principalApplication, failed.str(keyActorType))
	ts.NotEmpty(failed.str(keyCorrelationID), "A failure must carry a correlation id of its own")
	ts.False(failed.has(keySubject), "A grant that never issued a token has no subject to report")
}

// runLoginFlow drives the authorization-code login flow to completion and returns the flow's
// execution id together with the authorization code it produced.
func (ts *PrincipalEventsTestSuite) runLoginFlow(clientID, username, password string) (string, string) {
	resp, err := testutils.InitiateAuthorizationFlow(clientID, obsRedirectURI, "code", "openid", "obs-state")
	ts.Require().NoError(err, "Failed to initiate the authorization flow")
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "Expected a redirect from the authorization endpoint")

	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Expected a Location header")

	authID, executionID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err, "Failed to extract the auth data")

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err, "Failed to start the authentication flow")

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": username,
		"password": password,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to execute the authentication flow")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "The authentication flow should complete")

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err, "Failed to complete the authorization")

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err, "Failed to extract the authorization code")

	return executionID, code
}

// obtainAccessToken runs the login flow for the given credentials and returns the access token.
func (ts *PrincipalEventsTestSuite) obtainAccessToken(username, password string) string {
	_, code := ts.runLoginFlow(obsWebClientID, username, password)

	tokenResult, err := testutils.RequestToken(
		obsWebClientID, obsWebClientSecret, code, obsRedirectURI, "authorization_code")
	ts.Require().NoError(err, "Failed to request a token")
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, string(tokenResult.Body))
	ts.Require().NotEmpty(tokenResult.Token.AccessToken)

	return tokenResult.Token.AccessToken
}

// requestClientCredentialsToken requests a client_credentials token for the suite's resource server.
func (ts *PrincipalEventsTestSuite) requestClientCredentialsToken(
	clientID, clientSecret string,
) (int, map[string]interface{}) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("resource", obsResourceIdentifier)

	return ts.postToken(form, clientID, clientSecret)
}

// postToken submits a token request authenticated with client_secret_basic.
func (ts *PrincipalEventsTestSuite) postToken(
	form url.Values, clientID, clientSecret string,
) (int, map[string]interface{}) {
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
	ts.Require().NoError(err, "Failed to build the token request")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "The token request failed")
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read the token response")

	var body map[string]interface{}
	ts.Require().NoError(json.Unmarshal(bodyBytes, &body), "body: %s", string(bodyBytes))

	return resp.StatusCode, body
}

// createApplication creates an application with the given OAuth inbound config, optionally bound to
// the suite's authentication flow, and returns its resource ID.
func (ts *PrincipalEventsTestSuite) createApplication(
	name string, oauthConfig map[string]interface{}, withAuthFlow bool,
) string {
	app := map[string]interface{}{
		"name":                      name,
		"description":               "Application for principal-aware observability event tests",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{obsUserTypeName},
		"inboundAuthConfig": []map[string]interface{}{
			{"type": "oauth2", "config": oauthConfig},
		},
	}
	if withAuthFlow {
		app["authFlowId"] = ts.authFlowID
	}

	return ts.createEntity("/applications", app)
}

// createAgent creates an agent with a client_credentials OAuth profile and returns its resource ID.
func (ts *PrincipalEventsTestSuite) createAgent() string {
	agent := map[string]interface{}{
		"name": "Observability Principal Agent",
		"type": "default",
		"ouId": ts.ouID,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                obsAgentClientID,
					"clientSecret":            obsAgentClientSecret,
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}

	return ts.createEntity("/agents", agent)
}

// createEntity POSTs the payload to the given management path and returns the created resource's ID.
func (ts *PrincipalEventsTestSuite) createEntity(path string, payload map[string]interface{}) string {
	jsonData, err := json.Marshal(payload)
	ts.Require().NoError(err, "Failed to marshal the %s payload", path)

	req, err := http.NewRequest("POST", testutils.TestServerURL+path, bytes.NewBuffer(jsonData))
	ts.Require().NoError(err, "Failed to build the %s request", path)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "The %s request failed", path)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, string(bodyBytes))

	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(bodyBytes, &created), "body: %s", string(bodyBytes))

	id, ok := created["id"].(string)
	ts.Require().True(ok, "The %s response carries no id: %s", path, string(bodyBytes))

	return id
}

// createUser creates a user of the suite's user type and returns its resource ID.
func (ts *PrincipalEventsTestSuite) createUser(username, password string) string {
	attributes, err := json.Marshal(map[string]interface{}{
		"username": username,
		"password": password,
		"email":    username + "@obs-principal.example.com",
	})
	ts.Require().NoError(err, "Failed to marshal the user attributes")

	userID, err := testutils.CreateUser(testutils.User{
		OUID:       ts.ouID,
		Type:       obsUserTypeName,
		Attributes: json.RawMessage(attributes),
	})
	ts.Require().NoError(err, "Failed to create the test user")

	return userID
}
