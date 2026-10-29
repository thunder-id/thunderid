// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	extReqConnectionName = "ext-req-service"
	extReqTenantHeader   = "X-Tenant"
	extReqTenantValue    = "ext-req-tenant"
	extReqClientSecret   = "ext_req_client_secret_9d41"
	extReqUserEmail      = "ext-req-user-attr-5c7e@example.com"
	extReqUpstreamToken  = "ext-req-upstream-token-1b6f"

	// extReqRiskAnswer mixes every JSON type so the stored rendering of each is visible to a later
	// step. score keeps trailing zeros to show numbers are stored as their exact text, and the claims
	// keys are already sorted so the compact JSON is the same however the server re-encodes it.
	extReqRiskAnswer = `{"actionStatus":"SUCCESS","riskTier":"gold","score":12.50,"enabled":true,` +
		`"claims":{"businessId":"b-1","tier":2},"tags":["a","b"],"nothing":null}`
	extReqAllowlistAnswer = `{"actionStatus":"SUCCESS","serviceRef":"r-7","accessToken":"` +
		extReqUpstreamToken + `"}`
)

// extReqSeed is engine state planted in runtime data before the extension steps run. otpSessionToken
// and oauthState are secret engine state; clientId is an engine key that is sent only when named.
var extReqSeed = seedValues{
	OTPSessionToken: "ext-req-seeded-otp-session-3e8a",
	OAuthState:      "ext-req-seeded-oauth-state-77c2",
	ClientID:        "ext-req-seeded-client-id-0f19",
}

var (
	extReqOU = testutils.OrganizationUnit{
		Handle:      "ext-req-ou",
		Name:        "Extension Request Test OU",
		Description: "OU for extension executor request and storage tests",
	}

	extReqUserType = testutils.UserType{
		Handle:      "ext_req_person",
		DisplayName: "Extension Request Test Person",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string"},
		},
	}

	extReqUser = testutils.User{
		Type: "ext_req_person",
		Attributes: json.RawMessage(`{
			"username": "extrequser",
			"password": "ExtReqPass123!",
			"email": "` + extReqUserEmail + `"
		}`),
	}

	extReqApp = testutils.Application{
		Name:                      "Extension Request Test Application",
		Description:               "Application whose login flow calls extensions with every filter setting",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "ext_req_client",
		ClientSecret:              extReqClientSecret,
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"ext_req_person"},
	}
)

// extReqFlow is the login flow under test. Every extension step calls the same connection; the
// mock service tells them apart by currentNodeId.
//
//	start -> prompt_credentials -> seed_state -> ext_bare -> ext_named -> probe_state
//	      -> credentials_auth -> ext_risk -> ext_default -> ext_allowlist -> ext_reader
//	      -> probe_bare -> auth_assert -> end
//
// seed_state (HTTP request executor) maps the seed endpoint's answer into otpSessionToken,
// oauthState and clientId. ext_bare names nothing and runs while the flow still holds the submitted
// password. ext_named names username and password, and names userID and a key ext_risk has not stored
// yet, before the user is known. probe_state (HTTP request executor) echoes the two secret keys,
// proving they were in runtime data when ext_named ran. ext_risk returns values of every JSON type,
// stored with no allowlist under extension.ext_risk. ext_default names nothing, after sign-in and once
// flow data exists. ext_allowlist stores only serviceRef from an answer that also carries an access
// token, and names userID and one stored and one absent ext_risk key. ext_reader names every stored and
// dropped key, and probe_bare shows whether any field was stored under its bare name.
func extReqFlow(connectionID, serviceBaseURL string) testutils.Flow {
	next := func(target string) map[string]string { return map[string]string{"onSuccess": target} }
	risk := func(field string) string { return extensionKey("ext_risk", field) }
	return testutils.Flow{
		Name:     "Extension Request Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "ext_req_flow",
		Nodes: []map[string]interface{}{
			startNode("prompt_credentials"),
			promptCredentialsNode("seed_state"),
			{
				"id":   "seed_state",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"url":     serviceBaseURL + seedPath,
					"method":  "POST",
					"headers": `{"Content-Type": "application/json"}`,
					"body":    `{}`,
					"responseMapping": map[string]interface{}{
						"otpSessionToken": "response.data.otp",
						"oauthState":      "response.data.state",
						"clientId":        "response.data.client",
					},
					"timeout": "5",
				},
				"executor":  map[string]interface{}{"name": "HTTPRequestExecutor"},
				"onSuccess": "ext_bare",
			},
			extensionNode("ext_bare", map[string]interface{}{"connectionId": connectionID}, next("ext_named")),
			extensionNode("ext_named", map[string]interface{}{
				"connectionId":    connectionID,
				"inputKeys":       []string{"username", "password"},
				"runtimeDataKeys": []string{"userID", risk("riskTier")},
			}, next("probe_state")),
			{
				"id":   "probe_state",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"url":     serviceBaseURL + probePath,
					"method":  "POST",
					"headers": `{"Content-Type": "application/json"}`,
					"body":    `{"otp": "{{ctx(otpSessionToken)}}", "state": "{{ctx(oauthState)}}"}`,
					"timeout": "5",
				},
				"executor":  map[string]interface{}{"name": "HTTPRequestExecutor"},
				"onSuccess": "credentials_auth",
			},
			credentialsAuthNode("ext_risk"),
			extensionNode("ext_risk", map[string]interface{}{"connectionId": connectionID}, next("ext_default")),
			extensionNode("ext_default", map[string]interface{}{"connectionId": connectionID},
				next("ext_allowlist")),
			extensionNode("ext_allowlist", map[string]interface{}{
				"connectionId":    connectionID,
				"responseKeys":    []string{"serviceRef"},
				"runtimeDataKeys": []string{"userID", risk("score"), risk("riskScore")},
			}, next("ext_reader")),
			extensionNode("ext_reader", map[string]interface{}{
				"connectionId": connectionID,
				"runtimeDataKeys": []string{risk("riskTier"), risk("score"), risk("enabled"), risk("claims"),
					risk("tags"), risk("nothing"), risk("actionStatus"), extensionKey("ext_allowlist", "serviceRef"),
					extensionKey("ext_allowlist", "accessToken")},
			}, next("probe_bare")),
			bareProbeNode("probe_bare", "auth_assert", serviceBaseURL, extReqBareKeys...),
			authAssertNode(),
			endNode(),
		},
	}
}

// extReqBareKeys are the answer fields probe_bare looks for under their bare names.
var extReqBareKeys = []string{"riskTier", "score", "claims", "serviceRef", "accessToken", "actionStatus"}

// extReqExtensionSteps are the extension steps of extReqFlow in the order they run.
var extReqExtensionSteps = []string{"ext_bare", "ext_named", "ext_risk", "ext_default", "ext_allowlist",
	"ext_reader"}

// ExtensionRequestTestSuite covers what an extension step sends (R5, AC5.4, AC5.5), where it sends it
// (AC1.1), and what it stores for later steps under its namespace (AC6.1, AC6.2).
type ExtensionRequestTestSuite struct {
	suite.Suite
	config       *common.TestSuiteConfig
	service      *mockService
	fixtures     *identityFixtures
	connectionID string
}

func TestExtensionRequestTestSuite(t *testing.T) {
	suite.Run(t, new(ExtensionRequestTestSuite))
}

func (ts *ExtensionRequestTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}
	ts.service = newMockService(extReqSeed)
	ts.fixtures = createIdentityFixtures(&ts.Suite, extReqOU, extReqUserType, extReqUser)

	connectionID, err := testutils.CreateHTTPConnection(testutils.HTTPConnection{
		Name:      extReqConnectionName,
		OUID:      ts.fixtures.ouID,
		URL:       ts.service.connectionURL(),
		TimeoutMS: 5000,
		Headers:   []testutils.HTTPConnectionHeader{{Name: extReqTenantHeader, Value: extReqTenantValue}},
	})
	ts.Require().NoError(err, "Failed to create the HTTP endpoint connection")
	ts.connectionID = connectionID

	flowID, err := testutils.CreateFlow(extReqFlow(ts.connectionID, ts.service.server.URL))
	ts.Require().NoError(err, "Failed to create the extension request flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	ts.fixtures.createApplication(&ts.Suite, extReqApp, flowID)
}

func (ts *ExtensionRequestTestSuite) TearDownSuite() {
	ts.fixtures.deleteApplication(&ts.Suite)
	// Flows go before the connection: a flow that uses a connection blocks its deletion.
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

func (ts *ExtensionRequestTestSuite) SetupTest() {
	ts.service.reset()
	ts.service.answer("ext_risk", okAnswer(extReqRiskAnswer))
	ts.service.answer("ext_allowlist", okAnswer(extReqAllowlistAnswer))
}

// signIn runs the whole flow and returns the execution ID. Every extension step has been called once
// when it returns.
func (ts *ExtensionRequestTestSuite) signIn() string {
	ts.T().Helper()
	step := ts.fixtures.startLogin(&ts.Suite)
	ts.Require().Empty(ts.service.allCalls(), "Nothing should be called before the credentials are submitted")
	result, raw := ts.fixtures.submitCredentials(&ts.Suite, step)
	ts.fixtures.assertSignedIn(&ts.Suite, result, raw)
	for _, nodeID := range extReqExtensionSteps {
		ts.Require().Len(ts.service.callsFrom(nodeID), 1, "Extension step %s should call the service once", nodeID)
	}
	return step.ExecutionID
}

func (ts *ExtensionRequestTestSuite) call(nodeID string) serviceCall {
	ts.T().Helper()
	calls := ts.service.callsFrom(nodeID)
	ts.Require().Len(calls, 1, "Extension step %s should have called the service once", nodeID)
	return calls[0]
}

func (ts *ExtensionRequestTestSuite) TestCallGoesToTheConnectionWithItsHeaders() {
	executionID := ts.signIn()

	ts.Len(ts.service.callsAt(servicePath), len(extReqExtensionSteps),
		"Only the extension steps should call the connection's URL")
	for _, nodeID := range extReqExtensionSteps {
		c := ts.call(nodeID)
		ts.Equal(http.MethodPost, c.Method, "%s: the call should be a POST", nodeID)
		ts.Equal(serviceQuery, c.Query, "%s: the call should keep the connection URL's query", nodeID)
		ts.Equal("application/json", c.Headers.Get("Content-Type"), "%s: the call should send JSON", nodeID)
		ts.Equal(extReqTenantValue, c.Headers.Get(extReqTenantHeader),
			"%s: the connection's header should be sent", nodeID)
		ts.NotEmpty(c.Headers.Get("X-Correlation-ID"), "%s: the call should carry a correlation ID", nodeID)
		ts.Empty(c.Headers.Get("Authorization"), "%s: authentication none sends no Authorization", nodeID)

		ts.Equal("1", c.Body.Version, "%s: the body should carry contract version 1", nodeID)
		ts.Equal(executionID, c.Body.ExecutionID, "%s: the body should carry the flow's execution ID", nodeID)
		ts.Equal("AUTHENTICATION", c.Body.FlowType, "%s: the body should carry the flow type", nodeID)
		ts.Equal(nodeID, c.Body.CurrentNodeID, "%s: the body should name the calling step", nodeID)
	}
}

func (ts *ExtensionRequestTestSuite) TestApplicationAndAuthUserCarryNoSecrets() {
	ts.signIn()

	allowedTop := map[string]bool{"version": true, "executionId": true, "flowType": true, "currentAction": true,
		"currentNodeId": true, "userInputs": true, "runtimeData": true, "application": true, "authUser": true}
	required := []string{"version", "executionId", "flowType", "currentNodeId", "userInputs", "runtimeData",
		"application", "authUser"}
	allowedApp := map[string]bool{"id": true, "ouId": true, "name": true, "type": true}

	for _, nodeID := range extReqExtensionSteps {
		c := ts.call(nodeID)
		for field := range c.Fields {
			ts.True(allowedTop[field], "%s: the body must not carry the field %q", nodeID, field)
		}
		for _, field := range required {
			ts.Contains(c.Fields, field, "%s: the body should carry %q", nodeID, field)
		}
		for field := range c.Body.Application {
			ts.True(allowedApp[field], "%s: application must not carry %q", nodeID, field)
		}
		var appID, appOU, appName string
		_ = json.Unmarshal(c.Body.Application["id"], &appID)
		_ = json.Unmarshal(c.Body.Application["ouId"], &appOU)
		_ = json.Unmarshal(c.Body.Application["name"], &appName)
		ts.Equal(ts.fixtures.appID, appID, "%s: application.id should be the application", nodeID)
		ts.Equal(ts.fixtures.ouID, appOU, "%s: application.ouId should be the application's OU", nodeID)
		ts.Equal(extReqApp.Name, appName, "%s: application.name should be the application's name", nodeID)

		ts.NotContains(c.RawBody, extReqClientSecret, "%s: the application's client secret must never be sent",
			nodeID)
		ts.Empty(containsAny(c.RawBody, extReqUserEmail), "%s: user attributes must never be sent", nodeID)
	}

	// Before anyone signs in, authUser is an empty object.
	ts.JSONEq(`{}`, string(ts.call("ext_bare").Fields["authUser"]),
		"authUser should be {} before the user is authenticated")

	// After sign-in, each provider's state is reduced to its entity reference.
	after := ts.call("ext_default")
	ts.Require().NotEmpty(after.Body.AuthUser, "authUser should name the provider that authenticated the user")
	for provider, state := range after.Body.AuthUser {
		ts.Len(state, 1, "authUser.%s should carry only entityReference, got %v", provider,
			slices.Sorted(maps.Keys(state)))
		ts.Contains(state, "entityReference", "authUser.%s should carry entityReference", provider)
		var ref struct {
			EntityID       string `json:"entityId"`
			EntityCategory string `json:"entityCategory"`
			OUID           string `json:"ouId"`
		}
		ts.Require().NoError(json.Unmarshal(state["entityReference"], &ref),
			"authUser.%s.entityReference should decode", provider)
		ts.Equal(ts.fixtures.userID, ref.EntityID, "authUser.%s should reference the signed-in user", provider)
		ts.Equal("user", ref.EntityCategory, "authUser.%s should reference a user", provider)
		ts.Equal(ts.fixtures.ouID, ref.OUID, "authUser.%s should carry the user's OU", provider)
	}
}

func (ts *ExtensionRequestTestSuite) TestUserInputsAreSentOnlyWhenNamed() {
	ts.signIn()

	bare := ts.call("ext_bare")
	ts.Require().NotNil(bare.Body.UserInputs, "userInputs should be sent as an object")
	ts.Empty(bare.Body.UserInputs, "A step without inputKeys should be sent no user inputs")
	ts.NotContains(bare.RawBody, ts.fixtures.password, "The password must not be sent without inputKeys")

	named := ts.call("ext_named")
	ts.Equal(map[string]string{"username": ts.fixtures.username, "password": ts.fixtures.password},
		named.Body.UserInputs, "A step naming username and password should be sent exactly those inputs")

	for _, nodeID := range []string{"ext_risk", "ext_default", "ext_allowlist", "ext_reader"} {
		c := ts.call(nodeID)
		ts.Empty(c.Body.UserInputs, "%s names no inputs and should be sent none", nodeID)
		ts.NotContains(c.RawBody, ts.fixtures.password, "%s must not be sent the password", nodeID)
	}
}

func (ts *ExtensionRequestTestSuite) TestSecretEngineStateIsNeverSent() {
	ts.signIn()

	// Control: the probe echoes both secret keys, so they were in runtime data when ext_named ran.
	probes := ts.service.callsAt(probePath)
	ts.Require().Len(probes, 1, "The probe step should run once")
	var probed map[string]string
	ts.Require().NoError(json.Unmarshal([]byte(probes[0].RawBody), &probed), "The probe body should be JSON")
	ts.Equal(extReqSeed.OTPSessionToken, probed["otp"], "otpSessionToken should be in runtime data")
	ts.Equal(extReqSeed.OAuthState, probed["state"], "oauthState should be in runtime data")

	ts.Empty(ts.call("ext_named").Body.RuntimeData,
		"Before sign-in neither key ext_named may name is present, so none of the engine state is sent")

	for _, nodeID := range extReqExtensionSteps {
		ts.Empty(containsAny(ts.call(nodeID).RawBody, extReqSeed.OTPSessionToken, extReqSeed.OAuthState),
			"%s: secret engine state must never be sent", nodeID)
	}
}

func (ts *ExtensionRequestTestSuite) TestDefaultRuntimeDataIsUserIDOnly() {
	ts.signIn()

	bare := ts.call("ext_bare")
	ts.Require().NotNil(bare.Body.RuntimeData, "runtimeData should be sent as an object")
	ts.Empty(bare.Body.RuntimeData, "Before sign-in, a step without runtimeDataKeys should receive nothing")

	// ext_risk has stored extension.ext_risk.riskTier by now, and seed_state stored clientId.
	ts.Equal(map[string]string{"userID": ts.fixtures.userID}, ts.call("ext_default").Body.RuntimeData,
		"After sign-in, a step without runtimeDataKeys should receive userID only")
}

func (ts *ExtensionRequestTestSuite) TestStoredResponseValuesReachALaterStep() {
	ts.signIn()

	ts.NotContains(ts.call("ext_risk").Body.RuntimeData, extensionKey("ext_risk", "riskTier"),
		"A step must not see its own answer")
	ts.Equal(map[string]string{
		extensionKey("ext_risk", "riskTier"):        "gold",
		extensionKey("ext_risk", "score"):           "12.50",
		extensionKey("ext_risk", "enabled"):         "true",
		extensionKey("ext_risk", "claims"):          `{"businessId":"b-1","tier":2}`,
		extensionKey("ext_risk", "tags"):            `["a","b"]`,
		extensionKey("ext_allowlist", "serviceRef"): "r-7",
	}, ts.call("ext_reader").Body.RuntimeData,
		"A later step naming the namespaced keys should read the stored values, without null, actionStatus "+
			"or fields outside responseKeys")
	ts.NotContains(ts.call("ext_reader").RawBody, extReqUpstreamToken,
		"A field outside responseKeys must never be stored")
	assertNoBareKeys(&ts.Suite, ts.service, ts.fixtures.userID, extReqBareKeys...)
}

func (ts *ExtensionRequestTestSuite) TestRuntimeDataKeysSendOnlyTheNamedKeysPresent() {
	ts.signIn()

	// ext_named runs before ext_risk, so the ext_risk key it names is not there yet.
	ts.NotContains(ts.call("ext_named").Body.RuntimeData, extensionKey("ext_risk", "riskTier"),
		"A named key that is not present should not be sent")

	ts.Equal(map[string]string{
		"userID":                          ts.fixtures.userID,
		extensionKey("ext_risk", "score"): "12.50",
	}, ts.call("ext_allowlist").Body.RuntimeData,
		"runtimeDataKeys should send userID and the stored extension key it names, and nothing else")
}

func (ts *ExtensionRequestTestSuite) TestFailureWithoutOnFailureEndsTheFlow() {
	const leak = "ext-req-leak-81c3"
	cases := []struct {
		name    string
		answer  string
		code    string
		message string
	}{
		{name: "FailedWithoutOnFailure", code: errCodeExtensionActionFailed, message: msgExtensionActionFailed,
			answer: `{"actionStatus":"FAILED","failureReason":"high_risk","failureDescription":"Blocked."}`},
		{name: "ErrorWithoutOnFailure", code: errCodeExtensionCallFailed, message: msgExtensionCallFailed,
			answer: `{"actionStatus":"ERROR"}`},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			ts.SetupTest()
			ts.service.answer("ext_risk", okAnswer(tc.answer))

			result, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
			ts.Equal("ERROR", result.FlowStatus, "The flow should end in error: %s", raw)
			ts.Empty(result.Assertion, "A failed flow must not return an assertion")
			ts.Require().NotNil(result.Error, "The flow should report the step's error: %s", raw)
			ts.Equal(tc.code, result.Error.Code, "The flow should end with the step's error code")
			ts.Equal(tc.message, result.Error.Message.DefaultValue, "The flow should end with the step's error message")
			ts.NotContains(raw, leak, "Nothing from an answer that ends in the generic error may reach the client")
			ts.Len(ts.service.callsFrom("ext_risk"), 1, "The failing step should be called once")
			ts.Empty(ts.service.callsFrom("ext_default"), "No step after the failing one should run")
		})
	}
}

func (ts *ExtensionRequestTestSuite) TestIncompleteWithoutOnIncompleteFailsClosed() {
	const leak = "ext-req-leak-5a90"
	ts.service.answer("ext_risk", okAnswer(`{"actionStatus":"INCOMPLETE","failureReason":"otp_required",`+
		`"failureDescription":"`+leak+`"}`))
	step := ts.fixtures.startLogin(&ts.Suite)

	// ext_risk has no onIncomplete, so a request for input has nowhere to go. The engine refuses the
	// whole request with its internal error, without following onFailure.
	body := submitExpectingInternalError(&ts.Suite, step,
		map[string]string{"username": ts.fixtures.username, "password": ts.fixtures.password}, credentialsAction)
	ts.NotContains(body, leak, "The service's text must not reach the client")
	ts.Len(ts.service.callsFrom("ext_risk"), 1, "The step should be called once")
	ts.Empty(ts.service.callsFrom("ext_default"), "No later step may run")
}
