// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	extIsoConnectionName = "ext-iso-service"
	extIsoRedirectURI    = "https://localhost:3000"
	extIsoClientSecret   = "ext_iso_secret" //nolint:gosec // test credential
	extIsoSSOClientA     = "ext_iso_sso_a_client"
	extIsoSSOClientB     = "ext_iso_sso_b_client"

	extIsoUserEmail  = "ext-iso-user-3f1a@example.com"
	extIsoUserPin    = "4815"
	extIsoSpoofEmail = "ext-iso-spoof-9b2c@example.com"
	extIsoSpoofPin   = "2342"
	extIsoTier       = "gold"
	// extIsoSpoofUserID and extIsoSpoofGroup are identity fields the extension answers with, which must
	// never replace the signed-in user or reach a claim.
	extIsoSpoofUserID = "ext-iso-spoof-user-6d0e"
	extIsoSpoofGroup  = "ext-iso-spoof-group-a4f7"

	pinAction = "action_pin"
)

// extIsoEnrichAnswer is what ext_enrich answers in every flow of the suite: a profile attribute, a
// credential, a user ID, groups and plain data, all of which must stay under extension.ext_enrich.
var extIsoEnrichAnswer = `{"actionStatus":"SUCCESS","email":"` + extIsoSpoofEmail + `","pin":"` + extIsoSpoofPin +
	`","userID":"` + extIsoSpoofUserID + `","groups":["` + extIsoSpoofGroup + `"],"tier":"` + extIsoTier + `"}`

// extIsoSpoofValues are the values of extIsoEnrichAnswer no identity, claim or session may carry.
var extIsoSpoofValues = []string{extIsoSpoofEmail, extIsoSpoofPin, extIsoSpoofUserID, extIsoSpoofGroup}

var (
	extIsoOU = testutils.OrganizationUnit{
		Handle:      "ext-iso-ou",
		Name:        "Extension Isolation Test OU",
		Description: "OU for tests that extension data never becomes identity or session state",
	}

	extIsoUserType = testutils.UserType{
		Handle:      "ext_iso_person",
		DisplayName: "Extension Isolation Test Person",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"pin":      map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string"},
		},
		AllowSelfRegistration: true,
	}

	extIsoUser = testutils.User{
		Type: "ext_iso_person",
		Attributes: json.RawMessage(`{
			"username": "extisouser",
			"password": "ExtIsoPass123!",
			"pin": "` + extIsoUserPin + `",
			"email": "` + extIsoUserEmail + `"
		}`),
	}

	extIsoRegApp = testutils.Application{
		Name:                      "Extension Isolation Registration Application",
		Description:               "Application whose registration flow calls an extension before provisioning",
		IsRegistrationFlowEnabled: true,
		ClientID:                  "ext_iso_reg_client",
		ClientSecret:              extIsoClientSecret,
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"ext_iso_person"},
	}
)

// extIsoSSOFlow is the SSO sign-in both OAuth applications share:
//
//	start -> sso_check
//	      no session -> prompt_credentials -> credentials_auth -> ext_enrich -> session_main
//	      session    -> session_main
//	session_main -> ext_reader -> auth_assert -> end
//
// ext_enrich runs only on a fresh sign-in, before the session is saved. ext_reader runs on both paths
// and names what ext_enrich stored and userID, which must stay the signed-in user's.
func extIsoSSOFlow(connectionID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Extension Isolation SSO Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "ext_iso_sso_flow",
		Nodes: []map[string]interface{}{
			startNode("sso_check"),
			{
				"id":         "sso_check",
				"type":       "TASK_EXECUTION",
				"executor":   map[string]interface{}{"name": "SSOCheckExecutor"},
				"properties": map[string]interface{}{"checkpointRef": "session_main"},
				"onSuccess":  "session_main",
				"onFailure":  "prompt_credentials",
			},
			promptCredentialsNode("credentials_auth"),
			credentialsAuthNode("ext_enrich"),
			extensionNode("ext_enrich", map[string]interface{}{"connectionId": connectionID},
				map[string]string{"onSuccess": "session_main"}),
			{
				"id":        "session_main",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "SessionExecutor"},
				"onSuccess": "ext_reader",
			},
			extensionNode("ext_reader", map[string]interface{}{
				"connectionId": connectionID,
				"runtimeDataKeys": []string{extensionKey("ext_enrich", "email"), extensionKey("ext_enrich", "pin"),
					extensionKey("ext_enrich", "userID"), extensionKey("ext_enrich", "groups"),
					extensionKey("ext_enrich", "tier"), "userID"},
			}, map[string]string{"onSuccess": "auth_assert"}),
			authAssertNode(),
			endNode(),
		},
	}
}

// extIsoRegistrationFlow registers a user with a username and password only, with ext_enrich called
// before provisioning. Neither email nor pin is asked for, so provisioning would take them from
// runtime data if the extension's answer were stored under the bare names.
//
//	start -> user_type_resolver -> prompt_credentials -> credentials_auth -> ext_enrich
//	      -> provisioning -> auth_assert -> end
func extIsoRegistrationFlow(connectionID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Extension Isolation Registration Flow",
		FlowType: "REGISTRATION",
		Handle:   "ext_iso_reg_flow",
		Nodes: []map[string]interface{}{
			startNode("user_type_resolver"),
			{
				"id":        "user_type_resolver",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "UserTypeResolver"},
				"onSuccess": "prompt_credentials",
			},
			promptCredentialsNode("credentials_auth"),
			{
				"id":        "credentials_auth",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": credentialInputs},
				"onSuccess": "ext_enrich",
			},
			extensionNode("ext_enrich", map[string]interface{}{"connectionId": connectionID},
				map[string]string{"onSuccess": "provisioning"}),
			{
				"id":        "provisioning",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
				"onSuccess": "auth_assert",
			},
			authAssertNode(),
			endNode(),
		},
	}
}

// extIsoPinFlow signs in with a username and the pin credential, to learn whether a user has a pin.
func extIsoPinFlow() testutils.Flow {
	pinInputs := []map[string]interface{}{
		{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_pin", "identifier": "pin", "type": "PASSWORD_INPUT", "required": true},
	}
	return testutils.Flow{
		Name:     "Extension Isolation PIN Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "ext_iso_pin_flow",
		Nodes: []map[string]interface{}{
			startNode("prompt_pin"),
			{
				"id":   "prompt_pin",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": pinInputs,
						"action": map[string]interface{}{"ref": pinAction, "nextNode": "pin_auth"},
					},
				},
			},
			{
				"id":           "pin_auth",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": pinInputs},
				"onSuccess":    "auth_assert",
				"onIncomplete": "prompt_pin",
			},
			authAssertNode(),
			endNode(),
		},
	}
}

// ExtensionIsolationTestSuite covers that what an extension returns never becomes a claim, a profile
// attribute or a credential (AC6.7), and never travels in the SSO session to another application
// (AC6.6).
type ExtensionIsolationTestSuite struct {
	suite.Suite
	config       *common.TestSuiteConfig
	service      *mockService
	fixtures     *identityFixtures
	connectionID string
	ssoAppIDs    []string
}

func TestExtensionIsolationTestSuite(t *testing.T) {
	suite.Run(t, new(ExtensionIsolationTestSuite))
}

func (ts *ExtensionIsolationTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}
	ts.service = newMockService(seedValues{})
	ts.fixtures = createIdentityFixtures(&ts.Suite, extIsoOU, extIsoUserType, extIsoUser)

	connectionID, err := testutils.CreateHTTPConnection(testutils.HTTPConnection{
		Name: extIsoConnectionName, OUID: ts.fixtures.ouID, URL: ts.service.connectionURL(), TimeoutMS: 5000,
	})
	ts.Require().NoError(err, "Failed to create the HTTP endpoint connection")
	ts.connectionID = connectionID

	ssoFlowID := ts.createSuiteFlow(extIsoSSOFlow(ts.connectionID))
	regFlowID := ts.createSuiteFlow(extIsoRegistrationFlow(ts.connectionID))
	pinFlowID := ts.createSuiteFlow(extIsoPinFlow())

	regApp := extIsoRegApp
	regApp.RegistrationFlowID = regFlowID
	ts.fixtures.createApplication(&ts.Suite, regApp, pinFlowID)

	for _, clientID := range []string{extIsoSSOClientA, extIsoSSOClientB} {
		appID, err := testutils.CreateApplication(testutils.Application{
			Name:             "Extension Isolation SSO Application " + clientID,
			Description:      "Application sharing the extension isolation SSO flow",
			OUID:             ts.fixtures.ouID,
			AuthFlowID:       ssoFlowID,
			AllowedUserTypes: []string{extIsoUserType.Handle},
			InboundAuthConfig: []map[string]interface{}{
				{
					"type": "oauth2",
					"config": map[string]interface{}{
						"clientId":                clientID,
						"clientSecret":            extIsoClientSecret,
						"redirectUris":            []string{extIsoRedirectURI},
						"grantTypes":              []string{"authorization_code"},
						"responseTypes":           []string{"code"},
						"tokenEndpointAuthMethod": "client_secret_basic",
						"scopes":                  []string{"openid"},
						// An authorize request releases only the attributes it asks for; the access
						// token's attributes ask for email, so the issued access token carries it.
						"token": map[string]interface{}{
							"accessToken": map[string]interface{}{
								"userConfig": map[string]interface{}{"attributes": []string{"email"}},
							},
						},
					},
				},
			},
		})
		ts.Require().NoError(err, "Failed to create SSO application %s", clientID)
		ts.ssoAppIDs = append(ts.ssoAppIDs, appID)
	}
}

func (ts *ExtensionIsolationTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to delete registered users during teardown: %v", err)
	}
	for i := len(ts.ssoAppIDs) - 1; i >= 0; i-- {
		if err := testutils.DeleteApplication(ts.ssoAppIDs[i]); err != nil {
			ts.T().Logf("Failed to delete SSO application during teardown: %v", err)
		}
	}
	ts.fixtures.deleteApplication(&ts.Suite)
	for i := len(ts.config.CreatedFlowIDs) - 1; i >= 0; i-- {
		if err := testutils.DeleteFlow(ts.config.CreatedFlowIDs[i]); err != nil {
			ts.T().Logf("Failed to delete test flow %s during teardown: %v", ts.config.CreatedFlowIDs[i], err)
		}
	}
	if ts.connectionID != "" {
		if err := testutils.DeleteHTTPConnection(ts.connectionID); err != nil {
			ts.T().Logf("Failed to delete HTTP connection during teardown: %v", err)
		}
	}
	ts.fixtures.deleteIdentities(&ts.Suite)
	if ts.service != nil {
		ts.service.close()
	}
}

func (ts *ExtensionIsolationTestSuite) SetupTest() {
	ts.service.reset()
	ts.service.answer("ext_enrich", okAnswer(extIsoEnrichAnswer))
}

func (ts *ExtensionIsolationTestSuite) createSuiteFlow(flow testutils.Flow) string {
	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "Failed to create flow %s", flow.Handle)
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	return flowID
}

func (ts *ExtensionIsolationTestSuite) newSessionClient() *http.Client {
	client, err := testutils.NewSessionClient()
	ts.Require().NoError(err, "Failed to create the session client")
	return client
}

// authorize starts an authorization code request for clientID and returns the auth and execution IDs
// the gate redirect carries.
func (ts *ExtensionIsolationTestSuite) authorize(client *http.Client, clientID string) (string, string) {
	ts.T().Helper()
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", extIsoRedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid")
	params.Set("state", "ext_iso_state")
	authID, executionID, err := testutils.StartAuthorizationWithSession(client, params)
	ts.Require().NoError(err, "The authorize request should redirect to the gate")
	return authID, executionID
}

func (ts *ExtensionIsolationTestSuite) flowExecute(client *http.Client, body map[string]interface{}) (
	*common.FlowStep, string) {
	ts.T().Helper()
	status, raw, err := testutils.ExecuteFlowRaw(client, "", body)
	ts.Require().NoError(err, "The flow step request failed")
	ts.Require().Equal(http.StatusOK, status, "The flow step should succeed: %s", string(raw))
	var step common.FlowStep
	ts.Require().NoError(json.Unmarshal(raw, &step), "The flow step should decode: %s", string(raw))
	return &step, string(raw)
}

// accessTokenFor completes the authorization at clientID and returns the access token's claims. In an
// OAuth flow the user's attributes travel in the issued tokens, not in the flow assertion.
func (ts *ExtensionIsolationTestSuite) accessTokenFor(client *http.Client, clientID, authID,
	assertion string) map[string]interface{} {
	ts.T().Helper()
	response, err := testutils.CompleteAuthorizationWithClient(client, authID, assertion)
	ts.Require().NoError(err, "The authorization callback should succeed")
	code, err := testutils.ExtractAuthorizationCode(response.RedirectURI)
	ts.Require().NoError(err, "The callback should redirect with a code")
	ts.Require().NotEmpty(code, "The callback should issue an authorization code")
	result, err := testutils.RequestToken(clientID, extIsoClientSecret, code, extIsoRedirectURI, "authorization_code")
	ts.Require().NoError(err, "The token request failed")
	ts.Require().Equal(http.StatusOK, result.StatusCode, "The code should be exchanged: %s", string(result.Body))
	ts.Require().NotNil(result.Token, "The token response should decode: %s", string(result.Body))
	claims, err := testutils.DecodeJWTPayloadMap(result.Token.AccessToken)
	ts.Require().NoError(err, "The access token should be a decodable JWT")
	return claims
}

// ssoSignIn signs the fixture user in at clientID with the browser client and returns the assertion
// and the issued access token's claims.
func (ts *ExtensionIsolationTestSuite) ssoSignIn(client *http.Client, clientID string) (
	string, map[string]interface{}) {
	ts.T().Helper()
	authID, executionID := ts.authorize(client, clientID)
	prompt, raw := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("INCOMPLETE", prompt.FlowStatus, "A fresh sign-in should prompt for credentials: %s", raw)
	result, raw := ts.flowExecute(client, map[string]interface{}{
		"executionId":    executionID,
		"challengeToken": prompt.ChallengeToken,
		"action":         credentialsAction,
		"inputs":         map[string]string{"username": ts.fixtures.username, "password": ts.fixtures.password},
	})
	ts.Require().Equal("COMPLETE", result.FlowStatus, "The sign-in should complete: %s", raw)
	ts.Require().NotEmpty(result.Assertion, "The sign-in should return an assertion")
	return result.Assertion, ts.accessTokenFor(client, clientID, authID, result.Assertion)
}

// assertClaimsFromTheUserStore checks the access token names the fixture user with their stored
// email, and that neither it nor the assertion carries anything the extension returned.
func (ts *ExtensionIsolationTestSuite) assertClaimsFromTheUserStore(assertion string,
	tokenClaims map[string]interface{}, context string) {
	ts.T().Helper()
	ts.Equal(ts.fixtures.userID, tokenClaims["sub"], "%s: the access token should name the signed-in user", context)
	ts.Equal(extIsoUserEmail, tokenClaims["email"], "%s: the email claim should be the user's stored email",
		context)
	ts.assertNothingFromTheExtension(tokenClaims, context+" access token")

	assertionClaims, err := testutils.DecodeJWTPayloadMap(assertion)
	ts.Require().NoError(err, "%s: the assertion should be a decodable JWT", context)
	ts.Equal(ts.fixtures.userID, assertionClaims["sub"], "%s: the assertion should name the signed-in user", context)
	ts.assertNothingFromTheExtension(assertionClaims, context+" assertion")
}

// assertNothingFromTheExtension checks a token carries no claim named or valued from the extension.
func (ts *ExtensionIsolationTestSuite) assertNothingFromTheExtension(claims map[string]interface{}, context string) {
	ts.T().Helper()
	ts.NotContains(claims, "pin", "%s: no credential may become a claim", context)
	for claim, value := range claims {
		ts.False(strings.HasPrefix(claim, extensionNamespace), "%s: claim %q must not come from extension data",
			context, claim)
		encoded, err := json.Marshal(value)
		ts.Require().NoError(err, "%s: claim %q should encode", context, claim)
		ts.Empty(containsAny(string(encoded), extIsoSpoofValues...),
			"%s: claim %q must not carry the extension's answer", context, claim)
	}
}

func (ts *ExtensionIsolationTestSuite) TestExtensionFieldsNeverBecomeClaims() {
	assertion, tokenClaims := ts.ssoSignIn(ts.newSessionClient(), extIsoSSOClientA)
	ts.assertClaimsFromTheUserStore(assertion, tokenClaims, "Fresh sign-in")

	// Control: the answer was accepted and stored under the step's namespace, while userID stayed the
	// signed-in user's.
	readers := ts.service.callsFrom("ext_reader")
	ts.Require().Len(readers, 1, "ext_reader should run once")
	ts.Equal(map[string]string{
		extensionKey("ext_enrich", "email"):  extIsoSpoofEmail,
		extensionKey("ext_enrich", "pin"):    extIsoSpoofPin,
		extensionKey("ext_enrich", "userID"): extIsoSpoofUserID,
		extensionKey("ext_enrich", "groups"): `["` + extIsoSpoofGroup + `"]`,
		extensionKey("ext_enrich", "tier"):   extIsoTier,
		"userID":                             ts.fixtures.userID,
	}, readers[0].Body.RuntimeData, "ext_enrich's answer should be stored under extension.ext_enrich. only")
}

func (ts *ExtensionIsolationTestSuite) TestExtensionDataStaysOutOfTheSSOSession() {
	client := ts.newSessionClient()
	ts.ssoSignIn(client, extIsoSSOClientA)
	cookies, err := testutils.SSOCookieNames(client)
	ts.Require().NoError(err, "The session client's cookies should be readable")
	ts.Require().NotEmpty(cookies, "The first sign-in should establish an SSO session")
	ts.Require().Len(ts.service.callsFrom("ext_reader"), 1, "ext_reader should run on the first sign-in")

	// The second application shares the flow and so joins the session without a prompt.
	authID, executionID := ts.authorize(client, extIsoSSOClientB)
	result, raw := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", result.FlowStatus, "The second application should join the SSO session: %s", raw)
	ts.Require().NotEmpty(result.Assertion, "The SSO sign-in should return an assertion")
	tokenClaims := ts.accessTokenFor(client, extIsoSSOClientB, authID, result.Assertion)

	ts.Len(ts.service.callsFrom("ext_enrich"), 1, "The SSO sign-in must not call ext_enrich again")
	readers := ts.service.callsFrom("ext_reader")
	ts.Require().Len(readers, 2, "ext_reader should run on the SSO sign-in too")
	for key := range readers[1].Body.RuntimeData {
		ts.False(strings.HasPrefix(key, extensionNamespace),
			"The SSO sign-in must not receive %q from the earlier sign-in's extension", key)
	}
	ts.Empty(containsAny(readers[1].RawBody, append(extIsoSpoofValues, extIsoTier)...),
		"No value the extension returned may reach the second application's flow")
	ts.Equal(ts.fixtures.userID, readers[1].Body.RuntimeData["userID"],
		"The SSO sign-in should carry the session's user")
	ts.assertClaimsFromTheUserStore(result.Assertion, tokenClaims, "SSO sign-in")
}

// pinSignIn signs in through the PIN flow and returns the final step.
func (ts *ExtensionIsolationTestSuite) pinSignIn(username, pin string) (*common.FlowStep, string) {
	ts.T().Helper()
	step, err := common.InitiateAuthenticationFlow(ts.fixtures.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate the PIN sign-in")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The PIN sign-in should prompt for a username and PIN")
	return submit(&ts.Suite, step, map[string]string{"username": username, "pin": pin}, pinAction)
}

func (ts *ExtensionIsolationTestSuite) TestProvisioningTakesNothingFromExtensionData() {
	username := common.GenerateUniqueUsername("extisoreg")
	step, err := common.InitiateRegistrationFlow(ts.fixtures.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate the registration flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Registration should prompt for credentials")
	ts.Require().True(common.ValidateRequiredInputs(step.Data.Inputs, []string{"username", "password"}),
		"Registration should ask for a username and password")

	result, raw := submit(&ts.Suite, step, map[string]string{"username": username, "password": "ExtIsoReg123!"},
		credentialsAction)
	ts.Require().Equal("COMPLETE", result.FlowStatus, "Registration should complete: %s", raw)
	ts.Require().NotEmpty(result.Assertion, "Registration should return an assertion")
	ts.Require().Len(ts.service.callsFrom("ext_enrich"), 1, "ext_enrich should run before provisioning")

	user, err := testutils.GetUserFromAssertion(result.Assertion)
	ts.Require().NoError(err, "The registered user should be readable")
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
	attributes, err := testutils.GetUserAttributes(*user)
	ts.Require().NoError(err, "The registered user's attributes should decode")
	ts.Equal(username, attributes["username"], "The registered user should keep the username given")
	ts.NotContains(attributes, "email", "Provisioning must not take an attribute from the extension's answer")
	for name := range attributes {
		ts.False(strings.HasPrefix(name, extensionNamespace), "Attribute %q must not come from extension data", name)
	}

	// Control: a user who has a PIN can sign in with it, so the refusal below is about the credential.
	control, controlRaw := ts.pinSignIn(ts.fixtures.username, extIsoUserPin)
	ts.Require().Equal("COMPLETE", control.FlowStatus, "The fixture user should sign in with their PIN: %s",
		controlRaw)

	refused, refusedRaw := ts.pinSignIn(username, extIsoSpoofPin)
	ts.NotEqual("COMPLETE", refused.FlowStatus, "The extension's pin must not become the user's credential: %s",
		refusedRaw)
	ts.Empty(refused.Assertion, "A refused PIN sign-in must not return an assertion")
	ts.Require().NotNil(refused.Error, "The refused PIN sign-in should carry an error: %s", refusedRaw)
	ts.Equal(errCodeInvalidCredentials, refused.Error.Code, "The refusal should be the invalid-credentials error")
}
