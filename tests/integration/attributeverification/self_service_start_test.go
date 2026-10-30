// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The client whose own token names an application rather than a user.
const (
	selfServiceClientID     = "verification_self_service_client"
	selfServiceClientSecret = "Verification-Self-Service-Secret1!"
)

// The client whose own token expires a second after it is issued.
const (
	shortLivedClientID     = "verification_short_lived_client"
	shortLivedClientSecret = "Verification-Short-Lived-Secret1!"
)

// The agent whose own token names an agent rather than a user.
const (
	selfServiceAgentClientID     = "verification_self_service_agent"
	selfServiceAgentClientSecret = "Verification-Self-Service-Agent1!"
)

// The application whose sign-in shows that a handle does not select an authentication flow.
const (
	handleCheckClientID     = "verification_handle_check_client"
	handleCheckClientSecret = "Verification-Handle-Check-Secret1!"
)

// externalIssuerPort is the port of the issuer the server trusts in TestAnExternalIssuersTokenIsRefused.
// It is fixed because the mock issuer fixes its URL, which every token it signs carries, when it is
// built.
const externalIssuerPort = 8105

// SelfServiceStartTestSuite checks what a self-service start must carry before any flow runs: the
// signed-in user's token, issued by this deployment, and a handle naming a self-service flow. It also
// checks the entry points a self-service flow cannot be started from, and that other flow types do not
// select a flow by handle.
type SelfServiceStartTestSuite struct {
	suite.Suite
	fixture        *verificationFixture
	unboundHandle  string
	handleCheckApp string
}

func TestSelfServiceStartTestSuite(t *testing.T) {
	suite.Run(t, new(SelfServiceStartTestSuite))
}

func (ts *SelfServiceStartTestSuite) SetupSuite() {
	t := ts.T()
	ts.fixture = newVerificationFixture(t, "start")

	createApp(t, testutils.Application{
		OUID:        ts.fixture.ouID,
		Name:        "Attribute Verification Self Service Client",
		Description: "Client whose own token starts a self-service flow",
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                selfServiceClientID,
				"clientSecret":            selfServiceClientSecret,
				"redirectUris":            []string{"https://localhost:3000/callback"},
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})

	// Two authentication flows: the application is bound to the one that asks for a password, and the
	// handle a start sends names the one that asks for a nickname.
	boundFlowID := createFlow(t, testutils.Flow{
		Name:     "Attribute Verification Password Sign-in",
		FlowType: "AUTHENTICATION",
		Handle:   "verification-bound-sign-in-" + ts.fixture.suffix,
		Nodes: linearNodes(taskNode("credentials_auth", "CredentialsAuthExecutor", ""),
			taskNode("auth_assert", "AuthAssertExecutor", "")),
	})
	// The nickname is an input no other flow here asks for.
	ts.unboundHandle = "verification-unbound-sign-in-" + ts.fixture.suffix
	createFlow(t, testutils.Flow{
		Name:     "Attribute Verification Nickname Sign-in",
		FlowType: "AUTHENTICATION",
		Handle:   ts.unboundHandle,
		Nodes: linearNodes(promptNode("prompt_nickname", "action_nickname", "nickname"),
			taskNode("auth_assert", "AuthAssertExecutor", "")),
	})
	ts.handleCheckApp = createApp(t, testutils.Application{
		OUID:             ts.fixture.ouID,
		Name:             "Attribute Verification Handle Check App " + ts.fixture.suffix,
		Description:      "Application whose sign-in flow a handle must not replace",
		ClientID:         handleCheckClientID,
		ClientSecret:     handleCheckClientSecret,
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{ts.fixture.userType},
		AuthFlowID:       boundFlowID,
	})
}

func (ts *SelfServiceStartTestSuite) SetupTest() {
	ts.fixture.smtp.ClearEmails()
}

// alteredToken returns the token with the first character of its signature changed, so the server
// cannot verify it. The last character would not do: it carries padding bits a decoder may ignore.
func alteredToken(token string) string {
	at := strings.LastIndex(token, ".") + 1
	replacement := "A"
	if token[at] == 'A' {
		replacement = "B"
	}
	return token[:at] + replacement + token[at+1:]
}

// A start without a usable access token is refused before the handle is resolved: an unknown handle
// gets the same refusal as the shipped one, and nothing is sent.
func (ts *SelfServiceStartTestSuite) TestAStartWithoutAUsableTokenIsRefused() {
	user := ts.fixture.newUser(ts.T(), "no_usable_token")
	tokens := signIn(ts.T(), user)
	ts.Require().NotEmpty(tokens.IDToken, "precondition: the sign-in returns an ID token")

	for name, token := range map[string]string{
		"no token":        "",
		"malformed token": "not-a-token",
		"altered token":   alteredToken(tokens.AccessToken),
		"ID token":        tokens.IDToken,
	} {
		for _, handle := range []string{verificationFlowHandle, "no-such-self-service-flow"} {
			ts.Run(name+" for "+handle, func() {
				result := execute(ts.T(), token,
					selfServiceStart(handle, map[string]string{"attribute": emailAttribute}))

				requireStartRefused(ts.T(), result, http.StatusUnauthorized, errCodeAuthenticationRequired,
					"a start without a signed-in user's access token must be refused")
				ts.fixture.requireNoEmail(ts.T(), "nothing may be sent when the start is refused")
			})
		}
	}
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "nothing may be recorded")
}

// An application's own token passes the start, but preparation refuses it because it names no user,
// and nothing is sent.
func (ts *SelfServiceStartTestSuite) TestAnApplicationTokenIsRefusedAtPreparation() {
	clientToken, err := testutils.RequestClientCredentialsToken(selfServiceClientID, selfServiceClientSecret, "", "")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, clientToken.StatusCode, "precondition: the client gets its own token: %s",
		clientToken.Body)

	result := ts.fixture.startVerification(ts.T(), clientToken.Token.AccessToken,
		map[string]string{"attribute": emailAttribute})

	requireRefused(ts.T(), result, errCodeVerificationRefused, "preparation must refuse a caller that is not a user")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent for a caller that is not a user")
}

// A token that has expired is refused before the handle is resolved. The token is an application's own,
// which the start accepts while it is valid, so its refusal shows the expiry is checked; it is held
// until the server's leeway past its expiry has run out.
func (ts *SelfServiceStartTestSuite) TestAnExpiredTokenIsRefusedBeforeTheHandleIsResolved() {
	createApp(ts.T(), testutils.Application{
		OUID:        ts.fixture.ouID,
		Name:        "Attribute Verification Short-lived Client",
		Description: "Client whose own token expires at once",
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                shortLivedClientID,
				"clientSecret":            shortLivedClientSecret,
				"redirectUris":            []string{"https://localhost:3000/callback"},
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"token": map[string]interface{}{
					"accessToken": map[string]interface{}{
						"clientConfig": map[string]interface{}{"validityPeriod": 1},
					},
				},
			},
		}},
	})
	clientToken, err := testutils.RequestClientCredentialsToken(shortLivedClientID, shortLivedClientSecret, "", "")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, clientToken.StatusCode, "precondition: the client gets its own token: %s",
		clientToken.Body)
	token := clientToken.Token.AccessToken
	const unknownHandle = "no-such-self-service-flow"

	unknownStart := selfServiceStart(unknownHandle, map[string]string{"attribute": emailAttribute})

	requireStartRefused(ts.T(), execute(ts.T(), token, unknownStart), http.StatusBadRequest, errCodeFlowNotFound,
		"precondition: the token passes the start while it is valid")

	// The validity is one second; the wait covers the server's leeway past it.
	ts.Require().Eventually(func() bool {
		result, err := common.ExecuteFlowWithToken(token, unknownStart)
		return err == nil && result.StatusCode == http.StatusUnauthorized
	}, 2*time.Minute, 2*time.Second, "the token must stop being accepted once it expires")

	for _, handle := range []string{unknownHandle, verificationFlowHandle} {
		result := execute(ts.T(), token, selfServiceStart(handle, map[string]string{"attribute": emailAttribute}))
		requireStartRefused(ts.T(), result, http.StatusUnauthorized, errCodeAuthenticationRequired,
			"an expired token must be refused before the handle is resolved")
	}
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent when the start is refused")
}

// An agent's own token passes the start, but preparation refuses it because an agent is not a user,
// even when its type declares an email it holds, and nothing is sent.
func (ts *SelfServiceStartTestSuite) TestAnAgentTokenIsRefusedAtPreparation() {
	t := ts.T()
	// The default agent type is shared with every other suite: it is put back once this test ends,
	// before the organization unit it is moved to is deleted.
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "failed to snapshot the default agent type")
	t.Cleanup(func() {
		if err := testutils.RestoreAgentType(snapshot); err != nil {
			t.Errorf("cleanup: failed to restore the default agent type: %v", err)
		}
	})
	_, err = testutils.CreateAgentType(testutils.UserType{
		DisplayName: "Default",
		OUID:        ts.fixture.ouID,
		Schema: map[string]interface{}{
			"name":  map[string]interface{}{"type": "string", "required": true, "unique": true},
			"email": map[string]interface{}{"type": "email"},
		},
	})
	ts.Require().NoError(err, "failed to give the default agent type an email")
	authFlowID, err := testutils.GetFlowIDByHandle("default-flow", "AUTHENTICATION")
	ts.Require().NoError(err)
	agentEmail := "agent." + ts.fixture.suffix + "@example.com"
	agentID, err := testutils.CreateAgent(testutils.Agent{
		OUID:       ts.fixture.ouID,
		Type:       "default",
		Attributes: map[string]interface{}{"name": "verification-agent-" + ts.fixture.suffix, "email": agentEmail},
		AuthFlowID: authFlowID,
		InboundAuthConfig: []testutils.AgentInboundAuthConfig{{
			Type: "oauth2",
			Config: &testutils.AgentOAuthConfig{
				ClientID:                selfServiceAgentClientID,
				ClientSecret:            selfServiceAgentClientSecret,
				GrantTypes:              []string{"client_credentials"},
				TokenEndpointAuthMethod: "client_secret_basic",
			},
		}},
	})
	ts.Require().NoError(err, "failed to create the agent")
	t.Cleanup(func() {
		if err := testutils.DeleteAgent(agentID); err != nil {
			t.Logf("cleanup: failed to delete the agent: %v", err)
		}
	})
	agentToken, err := testutils.RequestClientCredentialsToken(selfServiceAgentClientID,
		selfServiceAgentClientSecret, "", "")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, agentToken.StatusCode, "precondition: the agent gets its own token: %s",
		agentToken.Body)

	result := ts.fixture.startVerification(ts.T(), agentToken.Token.AccessToken, map[string]string{"attribute": emailAttribute})

	requireRefused(ts.T(), result, errCodeVerificationRefused, "preparation must refuse an agent")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent for an agent")
}

func (ts *SelfServiceStartTestSuite) TestAStartRequiresAHandle() {
	user := ts.fixture.newUser(ts.T(), "missing_handle")
	token := ts.fixture.token(ts.T(), user)
	for name, omit := range map[string]bool{"empty": false, "omitted": true} {
		ts.Run(name, func() {
			request := selfServiceStart("", map[string]string{"attribute": emailAttribute})
			if omit {
				delete(request, "handle")
			}
			result := execute(ts.T(), token, request)
			requireStartRefused(ts.T(), result, http.StatusBadRequest, errCodeFlowHandleRequired,
				"each self-service start must name its flow")
			ts.fixture.requireNoEmail(ts.T(), "a missing handle must not send a challenge")
		})
	}
}

// A handle no self-service flow has is refused before any flow runs, including the handle of a flow of
// another type.
func (ts *SelfServiceStartTestSuite) TestAHandleWithNoSelfServiceFlowIsRefused() {
	user := ts.fixture.newUser(ts.T(), "unknown_handle")
	token := ts.fixture.token(ts.T(), user)

	for name, handle := range map[string]string{
		"unknown handle":               "no-such-self-service-flow",
		"authentication flow's handle": ts.unboundHandle,
	} {
		ts.Run(name, func() {
			result := execute(ts.T(), token, selfServiceStart(handle, map[string]string{"attribute": emailAttribute}))

			requireStartRefused(ts.T(), result, http.StatusBadRequest, errCodeFlowNotFound,
				"a handle no self-service flow has must be refused")
			ts.fixture.requireNoEmail(ts.T(), "nothing may be sent when the start is refused")
		})
	}
}

// An application ID on a self-service start is ignored, whether it names an application or not: the
// flow the handle names runs for the token's user.
func (ts *SelfServiceStartTestSuite) TestAnApplicationIDIsIgnored() {
	for name, appID := range map[string]string{
		"an application": ts.handleCheckApp,
		"no application": "019a0000-0000-7000-8000-000000000000",
	} {
		ts.Run(name, func() {
			ts.fixture.smtp.ClearEmails()
			user := ts.fixture.newUser(ts.T(), "app_id_"+strings.ReplaceAll(name, " ", "_"))
			request := selfServiceStart(ts.fixture.flowHandle, map[string]string{"attribute": emailAttribute})
			request["applicationId"] = appID

			step := requireCodePrompt(ts.T(), execute(ts.T(), ts.fixture.token(ts.T(), user), request),
				actionEmailVerify)
			done := submitCode(ts.T(), step, actionEmailVerify, ts.fixture.awaitCode(ts.T(), user.email))

			ts.Equal(flowStatusComplete, done.FlowStatus, "the flow must run as without the application ID")
			ts.NotEmpty(verifiedAt(ts.T(), user.id, emailAttribute), "the token's user must be verified")
		})
	}
}

// A self-service flow cannot be started by its flow ID: an administrator is told the flow is not
// permitted there, and any other caller that they are not authenticated for it.
func (ts *SelfServiceStartTestSuite) TestAStartByFlowIDIsRefused() {
	user := ts.fixture.newUser(ts.T(), "by_id")
	flowID, err := testutils.GetFlowIDByHandle(verificationFlowHandle, selfServiceFlowType)
	ts.Require().NoError(err, "the self attribute verification flow must ship")

	for name, tc := range map[string]struct {
		token  string
		status int
		code   string
	}{
		"administrator": {adminToken(ts.T()), http.StatusForbidden, errCodeFlowIDNotPermitted},
		"user":          {ts.fixture.token(ts.T(), user), http.StatusUnauthorized, errCodeAuthenticationRequired},
		"no token":      {"", http.StatusUnauthorized, errCodeAuthenticationRequired},
	} {
		ts.Run(name, func() {
			result := execute(ts.T(), tc.token, map[string]interface{}{
				"flowId": flowID,
				"inputs": map[string]string{"attribute": emailAttribute, "subject": user.id},
			})

			requireStartRefused(ts.T(), result, tc.status, tc.code, "a self-service flow ID must be refused")
			ts.fixture.requireNoEmail(ts.T(), "nothing may be sent when the start is refused")
		})
	}
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "nothing may be recorded")
}

// An authentication start that names another flow by handle runs the flow its application is bound
// to, as a start without the handle does.
func (ts *SelfServiceStartTestSuite) TestAHandleDoesNotSelectAnAuthenticationFlow() {
	without, err := common.InitiateFlowWithHandle(ts.handleCheckApp, "AUTHENTICATION", "", nil)
	ts.Require().NoError(err)
	ts.Require().True(common.HasInput(without.Data.Inputs, "password"),
		"precondition: the application's flow asks for a password: %+v", without.Data)

	for name, handle := range map[string]string{
		"another authentication flow": ts.unboundHandle,
		"a self-service flow":         verificationFlowHandle,
	} {
		ts.Run(name, func() {
			step, err := common.InitiateFlowWithHandle(ts.handleCheckApp, "AUTHENTICATION", handle, nil)
			ts.Require().NoError(err, "the start must run the application's flow")

			ts.Equal(flowStatusIncomplete, step.FlowStatus)
			ts.True(common.HasInput(step.Data.Inputs, "password"),
				"the start must ask for what the application's flow asks for: %+v", step.Data)
			ts.False(common.HasInput(step.Data.Inputs, "nickname"),
				"the flow the handle names must not run: %+v", step.Data)
		})
	}
}

// An access token a trusted external issuer signed for a local user's ID is refused at the start, as a
// start without a token is, and nothing is sent. The issuer's token for the administrator is accepted
// on the management API, so the refusal is not because the server cannot verify the issuer's tokens.
func (ts *SelfServiceStartTestSuite) TestAnExternalIssuersTokenIsRefused() {
	issuer := ts.trustExternalIssuer()
	managementToken, err := testutils.IssueManagementTokenFor(issuer, testutils.TrustedIssuerAdminSubject,
		"system", time.Hour)
	ts.Require().NoError(err)
	status, body := sendJSON(ts.T(), testutils.GetHTTPClientWithToken(managementToken), http.MethodGet,
		"/users?limit=1", nil)
	ts.Require().Equal(http.StatusOK, status, "precondition: the server accepts the issuer's tokens: %s", body)

	user := ts.fixture.newUser(ts.T(), "external_token")
	userToken, err := testutils.IssueManagementTokenFor(issuer, user.id, "", time.Hour)
	ts.Require().NoError(err)

	result := ts.fixture.startVerification(ts.T(), userToken, map[string]string{"attribute": emailAttribute})

	requireStartRefused(ts.T(), result, http.StatusUnauthorized, errCodeAuthenticationRequired,
		"a token another issuer signed must not start a self-service flow")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent when the start is refused")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "nothing may be recorded")
}

// trustExternalIssuer starts an issuer, restarts the server trusting it, and returns it. Cleanups run
// newest first, so the deployment is restored and the server restarted before the issuer stops, and the
// server never runs trusting an issuer that is gone.
func (ts *SelfServiceStartTestSuite) trustExternalIssuer() *testutils.MockOIDCServer {
	t := ts.T()
	t.Helper()

	issuer, err := testutils.NewMockOIDCServer(externalIssuerPort, "", "")
	ts.Require().NoError(err)
	ts.Require().NoError(issuer.Start(), "failed to start the external issuer")
	t.Cleanup(func() { _ = issuer.Stop() })

	original, err := testutils.ReadDeploymentConfigKey("server")
	ts.Require().NoError(err)
	// A second read is the copy the patch changes, so the original stays as read.
	patched, err := testutils.ReadDeploymentConfigKey("server")
	ts.Require().NoError(err)
	server, ok := patched.(map[string]interface{})
	ts.Require().True(ok, "the deployment must have a server section")
	security, _ := server["security"].(map[string]interface{})
	if security == nil {
		security = map[string]interface{}{}
	}
	security["trusted_issuer"] = map[string]interface{}{
		"issuer":   issuer.GetURL(),
		"jwks_url": issuer.GetJWKSURL(),
		"audience": testutils.TrustedIssuerAudience,
	}
	server["security"] = security
	t.Cleanup(func() {
		if err := testutils.PatchDeploymentConfig(map[string]interface{}{"server": original}); err != nil {
			t.Errorf("cleanup: failed to restore the server section: %v", err)
		} else if err := testutils.RestartServer(); err != nil {
			t.Errorf("cleanup: failed to restart with the original deployment: %v", err)
		} else if err := testutils.ObtainAdminAccessToken(); err != nil {
			t.Errorf("cleanup: failed to re-obtain the admin token: %v", err)
		}
	})
	ts.Require().NoError(testutils.PatchDeploymentConfig(map[string]interface{}{"server": server}),
		"failed to trust the external issuer")
	ts.Require().NoError(testutils.RestartServer(), "failed to restart trusting the external issuer")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(), "failed to re-obtain the admin token")
	return issuer
}
