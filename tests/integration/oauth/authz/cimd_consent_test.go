// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	cimdConsentClientID            = "https://cimd-consent.example.com/oauth/client-metadata.json"
	cimdConsentClientIDHost        = "cimd-consent.example.com"
	cimdConsentHTTPSRedirectURI    = "https://cimd-consent.example.com/callback"
	cimdConsentLoopbackRedirectURI = "http://127.0.0.1:3118/callback"
	// cimdConsentIDNClientID has a Cyrillic small letter o (U+043E) in its host, so it looks like
	// cimdConsentClientID.
	cimdConsentIDNClientID     = "https://cimd-c\u043ensent.example.com/oauth/client-metadata.json"
	cimdConsentIDNRedirectURI  = "https://cimd-c\u043ensent.example.com/callback"
	cimdConsentIDNPunycodeHost = "xn--cimd-cnsent-wnj.example.com"
	// cimdConsentInvalidIDNClientID mixes Latin and Hebrew in one label, which the IDNA lookup profile
	// rejects, so its host shows as plain punycode.
	cimdConsentInvalidIDNClientID     = "https://ab\u05d0.example.com/oauth/client-metadata.json"
	cimdConsentInvalidIDNPunycodeHost = "xn--ab-wld.example.com"
	// cimdConsentNoASCIIClientID encodes a byte that is not valid UTF-8 in its host, so the host has no
	// ASCII form.
	cimdConsentNoASCIIClientID   = "https://a%FFb.example.com/oauth/client-metadata.json"
	cimdConsentOtherClientID     = "cimd_consent_other_client"
	cimdConsentOtherClientSecret = "cimd_consent_other_secret" //nolint:gosec // test credential
	// cimdConsentOtherRedirectURI is a loopback address, so the hosts are left out for a client that is
	// not registered from a metadata document whatever its redirect URI.
	cimdConsentOtherRedirectURI = "http://localhost:3118/callback"
	cimdConsentPassword         = "CimdConsentPass1!"
	// Each user approves consent only in the test that uses it, so the other tests still reach consent.
	cimdConsentUsername          = "cimd_consent_user"
	cimdConsentCIMDFirstUsername = "cimd_consent_cimd_first_user"
	cimdConsentCIMDJoinUsername  = "cimd_consent_cimd_join_user"

	consentDataClientIDHost        = "clientIdHost"
	consentDataRedirectURIHost     = "redirectUriHost"
	consentDataRedirectURILoopback = "redirectUriLoopback"
)

var cimdConsentUserType = testutils.UserType{
	Handle:      "cimd-consent-person",
	DisplayName: "CIMD Consent Person",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
		"email":    map[string]interface{}{"type": "string"},
	},
}

// cimdConsentAuthFlow signs the user in, establishing an SSO session, then checks authorization and
// prompts for consent when it is missing. Both applications use it, so one can join the other's session.
var cimdConsentAuthFlow = testutils.Flow{
	Name:     "CIMD Consent Test Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_cimd_consent_test",
	Nodes: []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "sso_check"},
		{"id": "sso_check", "type": "TASK_EXECUTION",
			"executor":   map[string]interface{}{"name": "SSOCheckExecutor"},
			"properties": map[string]interface{}{"checkpointRef": "session_main"},
			"onSuccess":  "session_main", "onFailure": "prompt_credentials"},
		{"id": "prompt_credentials", "type": "PROMPT", "prompts": []map[string]interface{}{
			{
				"inputs": []map[string]interface{}{
					{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
				"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
			},
		}},
		{"id": "credentials_auth", "type": "TASK_EXECUTION", "executor": map[string]interface{}{
			"name": "CredentialsAuthExecutor",
			"inputs": []map[string]interface{}{
				{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
				{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
			},
		}, "onSuccess": "session_main", "onIncomplete": "prompt_credentials"},
		{"id": "session_main", "type": "TASK_EXECUTION",
			"executor": map[string]interface{}{"name": "SessionExecutor"}, "onSuccess": "authorization_check"},
		{"id": "authorization_check", "type": "TASK_EXECUTION",
			"executor": map[string]interface{}{"name": "AuthorizationExecutor"}, "onSuccess": "consent_check"},
		{"id": "consent_check", "type": "TASK_EXECUTION", "executor": map[string]interface{}{
			"name": "ConsentExecutor",
			"inputs": []map[string]interface{}{
				{"ref": "input_f3px", "identifier": "consent_decisions", "type": "CONSENT_INPUT", "required": true},
			},
		}, "onSuccess": "auth_assert", "onIncomplete": "prompt_consent"},
		{"id": "prompt_consent", "type": "PROMPT", "prompts": []map[string]interface{}{
			{
				"inputs": []map[string]interface{}{
					{"ref": "input_0ho7", "identifier": "consent_decisions", "type": "CONSENT_INPUT", "required": true},
				},
				"action": map[string]interface{}{"ref": "consent_action_allow", "nextNode": "consent_check"},
			},
		}},
		{"id": "auth_assert", "type": "TASK_EXECUTION",
			"executor": map[string]interface{}{"name": "AuthAssertExecutor"}, "onSuccess": "end"},
		{"id": "end", "type": "END"},
	},
}

// CIMDConsentTestSuite verifies that the consent prompt for a client registered from a Client ID
// Metadata Document carries its client ID host and its redirect URI host, in ASCII form, and whether
// that host is a loopback host, through both the authorization endpoint and pushed authorization
// requests, and that other clients' prompts carry none of them.
type CIMDConsentTestSuite struct {
	suite.Suite
	ouID            string
	entityTypeID    string
	authFlowID      string
	cimdAppID       string
	idnAppID        string
	invalidIDNAppID string
	noASCIIAppID    string
	otherAppID      string
	userIDs         []string
}

func TestCIMDConsentTestSuite(t *testing.T) {
	suite.Run(t, new(CIMDConsentTestSuite))
}

func (ts *CIMDConsentTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "cimd-consent-test-ou",
		Name:        "CIMD Consent Test OU",
		Description: "Organization unit for the CIMD consent integration tests",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID

	userType := cimdConsentUserType
	userType.OUID = ouID
	entityTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err)
	ts.entityTypeID = entityTypeID

	flowID, err := testutils.CreateFlow(cimdConsentAuthFlow)
	ts.Require().NoError(err)
	ts.authFlowID = flowID

	ts.cimdAppID = ts.createApplication(cimdConsentApplication("CIMDConsentTestApp",
		cimdConsentClientID, cimdConsentHTTPSRedirectURI, cimdConsentLoopbackRedirectURI))
	ts.idnAppID = ts.createApplication(cimdConsentApplication("CIMDConsentIDNTestApp",
		cimdConsentIDNClientID, cimdConsentIDNRedirectURI))
	ts.invalidIDNAppID = ts.createApplication(cimdConsentApplication("CIMDConsentInvalidIDNTestApp",
		cimdConsentInvalidIDNClientID, cimdConsentLoopbackRedirectURI))
	ts.noASCIIAppID = ts.createApplication(cimdConsentApplication("CIMDConsentNoASCIITestApp",
		cimdConsentNoASCIIClientID, cimdConsentLoopbackRedirectURI))
	ts.otherAppID = ts.createApplication(map[string]interface{}{
		"name": "CIMDConsentOtherTestApp",
		"type": "fullstack",
		"inboundAuthConfig": []map[string]interface{}{
			{"type": "oauth2", "config": map[string]interface{}{
				"clientId":                cimdConsentOtherClientID,
				"clientSecret":            cimdConsentOtherClientSecret,
				"redirectUris":            []string{cimdConsentOtherRedirectURI},
				"grantTypes":              []string{"authorization_code"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_post",
			}},
		},
	})

	for _, username := range []string{cimdConsentUsername, cimdConsentCIMDFirstUsername, cimdConsentCIMDJoinUsername} {
		attributes, err := json.Marshal(map[string]interface{}{
			"username": username,
			"password": cimdConsentPassword,
			"email":    username + "@example.com",
		})
		ts.Require().NoError(err)
		userID, err := testutils.CreateUser(testutils.User{
			OUID:       ouID,
			Type:       cimdConsentUserType.Handle,
			Attributes: json.RawMessage(attributes),
		})
		ts.Require().NoError(err)
		ts.userIDs = append(ts.userIDs, userID)
	}
}

func (ts *CIMDConsentTestSuite) TearDownSuite() {
	for _, userID := range ts.userIDs {
		_ = testutils.DeleteUser(userID)
	}
	for _, appID := range []string{ts.cimdAppID, ts.idnAppID, ts.invalidIDNAppID, ts.noASCIIAppID,
		ts.otherAppID} {
		if appID != "" {
			_ = testutils.DeleteApplication(appID)
		}
	}
	if ts.authFlowID != "" {
		_ = testutils.DeleteFlow(ts.authFlowID)
	}
	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

func (ts *CIMDConsentTestSuite) TestAuthorize_CIMDClientHTTPSRedirectShowsHosts() {
	step := ts.signInToConsent(ts.newSessionClient(), ts.authorize(cimdConsentClientID, cimdConsentHTTPSRedirectURI),
		cimdConsentUsername)

	ts.assertHosts(step, cimdConsentClientIDHost, cimdConsentClientIDHost, "false")
}

func (ts *CIMDConsentTestSuite) TestAuthorize_CIMDClientLoopbackRedirectShowsDeviceFlag() {
	step := ts.signInToConsent(ts.newSessionClient(),
		ts.authorize(cimdConsentClientID, cimdConsentLoopbackRedirectURI), cimdConsentUsername)

	ts.assertHosts(step, cimdConsentClientIDHost, "127.0.0.1", "true")
}

func (ts *CIMDConsentTestSuite) TestAuthorize_OtherClientHasNoHosts() {
	step := ts.signInToConsent(ts.newSessionClient(),
		ts.authorize(cimdConsentOtherClientID, cimdConsentOtherRedirectURI), cimdConsentUsername)

	ts.assertNoHosts(step)
}

func (ts *CIMDConsentTestSuite) TestPAR_CIMDClientHTTPSRedirectShowsHosts() {
	step := ts.signInToConsent(ts.newSessionClient(),
		ts.pushAuthorization(cimdConsentClientID, "", cimdConsentHTTPSRedirectURI), cimdConsentUsername)

	ts.assertHosts(step, cimdConsentClientIDHost, cimdConsentClientIDHost, "false")
}

func (ts *CIMDConsentTestSuite) TestPAR_CIMDClientLoopbackRedirectShowsDeviceFlag() {
	step := ts.signInToConsent(ts.newSessionClient(),
		ts.pushAuthorization(cimdConsentClientID, "", cimdConsentLoopbackRedirectURI), cimdConsentUsername)

	ts.assertHosts(step, cimdConsentClientIDHost, "127.0.0.1", "true")
}

func (ts *CIMDConsentTestSuite) TestPAR_OtherClientHasNoHosts() {
	step := ts.signInToConsent(ts.newSessionClient(), ts.pushAuthorization(
		cimdConsentOtherClientID, cimdConsentOtherClientSecret, cimdConsentOtherRedirectURI), cimdConsentUsername)

	ts.assertNoHosts(step)
}

func (ts *CIMDConsentTestSuite) TestAuthorize_CIMDClientInternationalizedHostShowsPunycode() {
	step := ts.signInToConsent(ts.newSessionClient(),
		ts.authorize(cimdConsentIDNClientID, cimdConsentIDNRedirectURI), cimdConsentUsername)

	ts.assertHosts(step, cimdConsentIDNPunycodeHost, cimdConsentIDNPunycodeHost, "false")
}

func (ts *CIMDConsentTestSuite) TestPAR_CIMDClientInternationalizedHostShowsPunycode() {
	step := ts.signInToConsent(ts.newSessionClient(),
		ts.pushAuthorization(cimdConsentIDNClientID, "", cimdConsentIDNRedirectURI), cimdConsentUsername)

	ts.assertHosts(step, cimdConsentIDNPunycodeHost, cimdConsentIDNPunycodeHost, "false")
}

func (ts *CIMDConsentTestSuite) TestAuthorize_CIMDClientInvalidIDNHostShowsPunycode() {
	step := ts.signInToConsent(ts.newSessionClient(),
		ts.authorize(cimdConsentInvalidIDNClientID, cimdConsentLoopbackRedirectURI), cimdConsentUsername)

	ts.assertHosts(step, cimdConsentInvalidIDNPunycodeHost, "127.0.0.1", "true")
}

func (ts *CIMDConsentTestSuite) TestPAR_CIMDClientInvalidIDNHostShowsPunycode() {
	step := ts.signInToConsent(ts.newSessionClient(),
		ts.pushAuthorization(cimdConsentInvalidIDNClientID, "", cimdConsentLoopbackRedirectURI), cimdConsentUsername)

	ts.assertHosts(step, cimdConsentInvalidIDNPunycodeHost, "127.0.0.1", "true")
}

func (ts *CIMDConsentTestSuite) TestAuthorize_CIMDClientHostWithoutASCIIFormHasNoHosts() {
	step := ts.signInToConsent(ts.newSessionClient(),
		ts.authorize(cimdConsentNoASCIIClientID, cimdConsentLoopbackRedirectURI), cimdConsentUsername)

	ts.assertNoHosts(step)
}

func (ts *CIMDConsentTestSuite) TestPAR_CIMDClientHostWithoutASCIIFormHasNoHosts() {
	step := ts.signInToConsent(ts.newSessionClient(),
		ts.pushAuthorization(cimdConsentNoASCIIClientID, "", cimdConsentLoopbackRedirectURI), cimdConsentUsername)

	ts.assertNoHosts(step)
}

// TestSSO_OtherClientJoiningDoesNotShowCIMDHosts signs in through the CIMD client, then has the other
// client join that SSO session: its consent prompt must not carry the hosts of the client that
// established the session.
func (ts *CIMDConsentTestSuite) TestSSO_OtherClientJoiningDoesNotShowCIMDHosts() {
	client := ts.newSessionClient()
	authID, step := ts.signInToConsentWithAuthID(client,
		ts.authorize(cimdConsentClientID, cimdConsentLoopbackRedirectURI), cimdConsentCIMDFirstUsername)
	ts.assertHosts(step, cimdConsentClientIDHost, "127.0.0.1", "true")
	completed := ts.approveAllConsent(client, step)
	ts.Require().Equal("COMPLETE", completed.FlowStatus)
	_, err := testutils.CompleteAuthorizationWithClient(client, authID, completed.Assertion)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "the first sign-in should establish an SSO session")

	_, executionID := ts.startAuthorization(client, ts.authorize(cimdConsentOtherClientID, cimdConsentOtherRedirectURI))
	joined := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})

	ts.Require().NotEqual("COMPLETE", joined.FlowStatus, "the joining client has no consent yet")
	ts.Require().Contains(joined.Data.AdditionalData, "consentPrompt", "the SSO join should reach consent")
	ts.assertNoHosts(joined)
}

// TestSSO_CIMDClientJoiningShowsItsHosts has the CIMD client join a session the other client
// established: its consent prompt carries its own hosts.
func (ts *CIMDConsentTestSuite) TestSSO_CIMDClientJoiningShowsItsHosts() {
	client := ts.newSessionClient()
	authID, step := ts.signInToConsentWithAuthID(client,
		ts.authorize(cimdConsentOtherClientID, cimdConsentOtherRedirectURI), cimdConsentCIMDJoinUsername)
	completed := ts.approveAllConsent(client, step)
	ts.Require().Equal("COMPLETE", completed.FlowStatus)
	_, err := testutils.CompleteAuthorizationWithClient(client, authID, completed.Assertion)
	ts.Require().NoError(err)
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "the first sign-in should establish an SSO session")

	_, executionID := ts.startAuthorization(client, ts.authorize(cimdConsentClientID, cimdConsentHTTPSRedirectURI))
	joined := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})

	ts.Require().NotEqual("COMPLETE", joined.FlowStatus, "the joining client has no consent yet")
	ts.assertHosts(joined, cimdConsentClientIDHost, cimdConsentClientIDHost, "false")
}

// cimdConsentApplication returns a public client registered from a Client ID Metadata Document.
func cimdConsentApplication(name, clientID string, redirectURIs ...string) map[string]interface{} {
	return map[string]interface{}{
		"name": name,
		"type": "mcp",
		"inboundAuthConfig": []map[string]interface{}{
			{"type": "oauth2", "config": map[string]interface{}{
				"clientId":                 clientID,
				"clientIdMetadataDocument": true,
				"redirectUris":             redirectURIs,
				"grantTypes":               []string{"authorization_code"},
				"responseTypes":            []string{"code"},
				"tokenEndpointAuthMethod":  "none",
				"pkceRequired":             true,
				"publicClient":             true,
			}},
		},
	}
}

// createApplication creates an application on the consent flow and returns its ID.
func (ts *CIMDConsentTestSuite) createApplication(app map[string]interface{}) string {
	app["ouId"] = ts.ouID
	app["authFlowId"] = ts.authFlowID
	app["isRegistrationFlowEnabled"] = false
	app["allowedUserTypes"] = []string{cimdConsentUserType.Handle}
	app["assertion"] = map[string]interface{}{"userAttributes": []string{"email"}}
	app["loginConsent"] = map[string]interface{}{"validityPeriod": 3600}
	body, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/applications", bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, string(respBody))
	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &created))
	return created["id"].(string)
}

// authorize returns the authorization request parameters for a client and redirect URI, with PKCE.
func (ts *CIMDConsentTestSuite) authorize(clientID, redirectURI string) url.Values {
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid")
	params.Set("state", "cimd-consent-state")
	params.Set("code_challenge", ts.codeChallenge())
	params.Set("code_challenge_method", "S256")
	return params
}

// pushAuthorization pushes an authorization request and returns the parameters that redeem it at the
// authorization endpoint.
func (ts *CIMDConsentTestSuite) pushAuthorization(clientID, clientSecret, redirectURI string) url.Values {
	pushed := map[string]string{}
	for key, values := range ts.authorize(clientID, redirectURI) {
		if key != "client_id" {
			pushed[key] = values[0]
		}
	}
	result, err := testutils.SubmitPARRequest(clientID, clientSecret, pushed)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, result.StatusCode, string(result.Body))

	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("request_uri", result.PAR.RequestURI)
	return params
}

// signInToConsent sends the authorization request and signs the user in, returning the consent step.
func (ts *CIMDConsentTestSuite) signInToConsent(
	client *http.Client, params url.Values, username string) *testutils.FlowStep {
	_, step := ts.signInToConsentWithAuthID(client, params, username)
	return step
}

// signInToConsentWithAuthID is signInToConsent that also returns the authorization request's ID.
func (ts *CIMDConsentTestSuite) signInToConsentWithAuthID(
	client *http.Client, params url.Values, username string) (string, *testutils.FlowStep) {
	authID, executionID := ts.startAuthorization(client, params)
	initial := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().NotEqual("COMPLETE", initial.FlowStatus, "the user should be asked to sign in")

	step := ts.flowExecute(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": username, "password": cimdConsentPassword},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})
	ts.Require().NotEqual("COMPLETE", step.FlowStatus, "the user should be asked to consent")
	ts.Require().NotNil(step.Data)
	ts.Require().Contains(step.Data.AdditionalData, "consentPrompt")
	return authID, step
}

// startAuthorization sends the authorization request and returns the authorization request ID and the
// flow execution ID from the redirect to the sign-in page.
func (ts *CIMDConsentTestSuite) startAuthorization(client *http.Client, params url.Values) (string, string) {
	authID, executionID, err := testutils.StartAuthorizationWithSession(client, params)
	ts.Require().NoError(err)
	return authID, executionID
}

// approveAllConsent approves every purpose the consent step prompts for and returns the next step.
func (ts *CIMDConsentTestSuite) approveAllConsent(
	client *http.Client, step *testutils.FlowStep) *testutils.FlowStep {
	var purposes []consentPurposePrompt
	ts.Require().NoError(json.Unmarshal([]byte(step.Data.AdditionalData["consentPrompt"].(string)), &purposes))

	type elementDecision struct {
		Name     string `json:"name"`
		Approved bool   `json:"approved"`
	}
	type purposeDecision struct {
		PurposeName string            `json:"purposeName"`
		Approved    bool              `json:"approved"`
		Elements    []elementDecision `json:"elements"`
	}
	decisions := struct {
		Approved bool              `json:"approved"`
		Purposes []purposeDecision `json:"purposes"`
	}{Approved: true}
	for _, purpose := range purposes {
		decision := purposeDecision{PurposeName: purpose.PurposeName, Approved: true}
		for _, element := range append(append([]consentElementRef{}, purpose.Essential...), purpose.Optional...) {
			decision.Elements = append(decision.Elements, elementDecision{Name: element.Name, Approved: true})
		}
		decisions.Purposes = append(decisions.Purposes, decision)
	}
	decisionsJSON, err := json.Marshal(decisions)
	ts.Require().NoError(err)

	return ts.flowExecute(client, map[string]interface{}{
		"executionId":    step.ExecutionID,
		"inputs":         map[string]string{"consent_decisions": string(decisionsJSON)},
		"action":         "consent_action_allow",
		"challengeToken": step.ChallengeToken,
	})
}

// flowExecute posts a step to the flow execution API through the session client.
func (ts *CIMDConsentTestSuite) flowExecute(client *http.Client, body map[string]interface{}) *testutils.FlowStep {
	status, respBody, err := testutils.ExecuteFlowRaw(client, "", body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, string(respBody))

	var step testutils.FlowStep
	ts.Require().NoError(json.Unmarshal(respBody, &step))
	return &step
}

// newSessionClient returns a client that carries the SSO cookie across requests.
func (ts *CIMDConsentTestSuite) newSessionClient() *http.Client {
	client, err := testutils.NewSessionClient()
	ts.Require().NoError(err)
	return client
}

// ssoCookieNames returns the SSO cookies the session client holds.
func (ts *CIMDConsentTestSuite) ssoCookieNames(client *http.Client) []string {
	names, err := testutils.SSOCookieNames(client)
	ts.Require().NoError(err)
	return names
}

// codeChallenge returns a fresh S256 PKCE code challenge.
func (ts *CIMDConsentTestSuite) codeChallenge() string {
	verifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err)
	return testutils.GenerateCodeChallenge(verifier)
}

// assertHosts checks the consent step carries the CIMD client's hosts and the loopback flag.
func (ts *CIMDConsentTestSuite) assertHosts(
	step *testutils.FlowStep, clientIDHost, redirectURIHost, loopback string) {
	ts.Require().NotNil(step.Data)
	data := step.Data.AdditionalData
	ts.Equal(clientIDHost, data[consentDataClientIDHost])
	ts.Equal(redirectURIHost, data[consentDataRedirectURIHost])
	ts.Equal(loopback, data[consentDataRedirectURILoopback])
}

// assertNoHosts checks the consent step carries none of the client hosts.
func (ts *CIMDConsentTestSuite) assertNoHosts(step *testutils.FlowStep) {
	ts.Require().NotNil(step.Data)
	data := step.Data.AdditionalData
	ts.NotContains(data, consentDataClientIDHost)
	ts.NotContains(data, consentDataRedirectURIHost)
	ts.NotContains(data, consentDataRedirectURILoopback)
}
