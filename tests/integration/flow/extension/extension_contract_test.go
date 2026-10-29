// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	extContractConnectionName = "ext-contract-service"
	extContractTimeoutMS      = 5000

	// extContractLeak is what a misbehaving service says about itself. It must never reach the client
	// or runtime data. extContractLeakReason is the same marker in failureReason's character set.
	extContractLeak       = "ext-contract-leak-4d2b"
	extContractLeakReason = "ext_contract_leak_4d2b"

	// extContractBodyCap is the largest answer body the step reads (64 KiB).
	extContractBodyCap = 65536
)

var (
	extContractOU = testutils.OrganizationUnit{
		Handle:      "ext-contract-ou",
		Name:        "Extension Contract Test OU",
		Description: "OU for extension executor response contract tests",
	}

	extContractUserType = testutils.UserType{
		Handle:      "ext_contract_person",
		DisplayName: "Extension Contract Test Person",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	}

	extContractUser = testutils.User{
		Type: "ext_contract_person",
		Attributes: json.RawMessage(`{
			"username": "extcontractuser",
			"password": "ExtContractPass123!"
		}`),
	}

	extContractApp = testutils.Application{
		Name:                      "Extension Contract Test Application",
		Description:               "Application whose login flow routes on the extension's answer",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "ext_contract_client",
		ClientSecret:              "ext_contract_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"ext_contract_person"},
	}
)

// extContractFlow routes on ext_check's answer:
//
//	start -> prompt_credentials -> credentials_auth -> ext_check
//	      SUCCESS    (onSuccess)    -> ext_echo -> probe_bare -> auth_assert -> end
//	      FAILED     (onFailure)    -> prompt_credentials
//	      INCOMPLETE (onIncomplete) -> prompt_otp -> ext_echo -> probe_bare -> auth_assert -> end
//	      ERROR      (onFailure)    -> prompt_credentials
//
// ext_check stores only riskScore and challengeId, under extension.ext_check., and names those keys
// in runtimeDataKeys, so a retry shows in its request what the previous attempt stored. ext_echo exists
// to read what ext_check stored and is sent the otp the user gives. probe_bare shows whether any field
// of the answer, the contract's included, was stored under its bare name.
func extContractFlow(connectionID, serviceBaseURL string) testutils.Flow {
	riskScore, challengeID := extensionKey("ext_check", "riskScore"), extensionKey("ext_check", "challengeId")
	return testutils.Flow{
		Name:     "Extension Contract Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "ext_contract_flow",
		Nodes: []map[string]interface{}{
			startNode("prompt_credentials"),
			promptCredentialsNode("credentials_auth"),
			credentialsAuthNode("ext_check"),
			extensionNode("ext_check", map[string]interface{}{
				"connectionId":    connectionID,
				"responseKeys":    []string{"riskScore", "challengeId"},
				"runtimeDataKeys": []string{riskScore, challengeID},
			}, map[string]string{
				"onSuccess":    "ext_echo",
				"onFailure":    "prompt_credentials",
				"onIncomplete": "prompt_otp",
			}),
			{
				"id":   "prompt_otp",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_otp", "identifier": "otp", "type": "TEXT_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_otp", "nextNode": "ext_echo"},
					},
				},
			},
			extensionNode("ext_echo", map[string]interface{}{
				"connectionId":    connectionID,
				"inputKeys":       []string{"otp"},
				"runtimeDataKeys": []string{riskScore, challengeID, extensionKey("ext_check", "actionStatus")},
			}, map[string]string{"onSuccess": "probe_bare"}),
			bareProbeNode("probe_bare", "auth_assert", serviceBaseURL, retryBareKeys...),
			authAssertNode(),
			endNode(),
		},
	}
}

// ExtensionContractTestSuite covers the fixed response contract (R4), the attempt limit (AC4.8, AC4.9),
// what each answer stores and replaces (AC6.1, AC6.3, AC6.5, AC6.8) and the failure text the user sees
// (R7, AC7.5).
type ExtensionContractTestSuite struct {
	suite.Suite
	config       *common.TestSuiteConfig
	service      *mockService
	fixtures     *identityFixtures
	connectionID string
}

func TestExtensionContractTestSuite(t *testing.T) {
	suite.Run(t, new(ExtensionContractTestSuite))
}

func (ts *ExtensionContractTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}
	ts.service = newMockService(seedValues{})
	ts.fixtures = createIdentityFixtures(&ts.Suite, extContractOU, extContractUserType, extContractUser)

	connectionID, err := testutils.CreateHTTPConnection(ts.connection(ts.service.connectionURL(),
		extContractTimeoutMS))
	ts.Require().NoError(err, "Failed to create the HTTP endpoint connection")
	ts.connectionID = connectionID

	flowID, err := testutils.CreateFlow(extContractFlow(ts.connectionID, ts.service.server.URL))
	ts.Require().NoError(err, "Failed to create the extension contract flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	ts.fixtures.createApplication(&ts.Suite, extContractApp, flowID)
}

func (ts *ExtensionContractTestSuite) TearDownSuite() {
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

func (ts *ExtensionContractTestSuite) SetupTest() {
	ts.service.reset()
}

func (ts *ExtensionContractTestSuite) connection(url string, timeoutMS int) testutils.HTTPConnection {
	return testutils.HTTPConnection{
		Name: extContractConnectionName, OUID: ts.fixtures.ouID, URL: url, TimeoutMS: timeoutMS,
	}
}

// useConnection points the suite's connection at url with timeoutMS and restores it when the test
// ends, whatever its outcome. The flow is never edited.
func (ts *ExtensionContractTestSuite) useConnection(url string, timeoutMS int) {
	ts.T().Helper()
	ts.T().Cleanup(func() {
		if err := testutils.UpdateHTTPConnection(ts.connectionID,
			ts.connection(ts.service.connectionURL(), extContractTimeoutMS)); err != nil {
			ts.T().Logf("Failed to restore the HTTP connection: %v", err)
		}
	})
	ts.Require().NoError(testutils.UpdateHTTPConnection(ts.connectionID, ts.connection(url, timeoutMS)),
		"Failed to update the HTTP connection")
}

// check submits the credentials with ext_check set to answer a.
func (ts *ExtensionContractTestSuite) check(a serviceAnswer) (*common.FlowStep, string) {
	ts.T().Helper()
	ts.service.answer("ext_check", a)
	step := ts.fixtures.startLogin(&ts.Suite)
	ts.Require().Empty(ts.service.allCalls(), "Nothing should be called before the credentials are submitted")
	return ts.fixtures.submitCredentials(&ts.Suite, step)
}

// assertBackAtCredentialsPrompt checks that onFailure showed the credentials prompt again with code.
func (ts *ExtensionContractTestSuite) assertBackAtCredentialsPrompt(result *common.FlowStep, raw, code string) {
	ts.T().Helper()
	ts.Require().Equal("INCOMPLETE", result.FlowStatus, "onFailure should show the credentials prompt: %s", raw)
	ts.Empty(result.Assertion, "A failed attempt must not return an assertion")
	ts.True(common.ValidateRequiredInputs(result.Data.Inputs, []string{"username", "password"}),
		"onFailure should lead to the credentials prompt")
	ts.False(common.HasInput(result.Data.Inputs, "otp"), "A failure must not lead to the otp prompt")
	ts.Require().NotNil(result.Error, "The prompt should carry the step's error: %s", raw)
	ts.Equal(code, result.Error.Code, "Unexpected error code on the prompt")
	ts.Empty(ts.service.callsFrom("ext_echo"), "A failed step must not continue to onSuccess")
}

// assertGenericCallFailure checks the client got the generic call failure and nothing from the body.
func (ts *ExtensionContractTestSuite) assertGenericCallFailure(result *common.FlowStep, raw string) {
	ts.T().Helper()
	ts.assertBackAtCredentialsPrompt(result, raw, errCodeExtensionCallFailed)
	ts.Equal(msgExtensionCallFailed, result.Error.Message.DefaultValue, "The client should see the generic message")
	ts.Empty(result.Error.Description.Params, "The generic error should carry no params from the answer")
	ts.Empty(containsAny(raw, extContractLeak, extContractLeakReason),
		"Nothing from the service's answer may reach the client")
}

// retryWithSuccess submits the credentials again on the re-shown prompt with ext_check answering
// SUCCESS, and returns the retried ext_check call and the ext_echo call.
func (ts *ExtensionContractTestSuite) retryWithSuccess(prompt *common.FlowStep) (retried, echoed serviceCall) {
	ts.T().Helper()
	ts.service.answer("ext_check", okAnswer(successBody))
	result, raw := ts.fixtures.submitCredentials(&ts.Suite, prompt)
	ts.fixtures.assertSignedIn(&ts.Suite, result, raw)
	checks := ts.service.callsFrom("ext_check")
	ts.Require().Len(checks, 2, "The retry should call ext_check a second time")
	echoes := ts.service.callsFrom("ext_echo")
	ts.Require().Len(echoes, 1, "SUCCESS on the retry should continue to ext_echo")
	return checks[1], echoes[0]
}

// assertNothingStored checks the retried request carries none of the failed answer's fields.
func (ts *ExtensionContractTestSuite) assertNothingStored(retried serviceCall) {
	ts.T().Helper()
	for _, key := range []string{extensionKey("ext_check", "riskScore"), extensionKey("ext_check", "challengeId")} {
		ts.NotContains(retried.Body.RuntimeData, key, "An ERROR answer must not store %s", key)
	}
	ts.Empty(containsAny(retried.RawBody, extContractLeak, extContractLeakReason),
		"Nothing from an ERROR answer may reach runtime data")
}

// sizedBody is a SUCCESS answer of exactly size bytes.
func sizedBody(size int, riskScore string) string {
	prefix := `{"actionStatus":"SUCCESS","riskScore":"` + riskScore + `","pad":"`
	suffix := `"}`
	return prefix + strings.Repeat("a", size-len(prefix)-len(suffix)) + suffix
}

func (ts *ExtensionContractTestSuite) TestSuccessContinuesOnSuccess() {
	result, raw := ts.check(okAnswer(`{"actionStatus":"SUCCESS","riskScore":"3"}`))
	ts.fixtures.assertSignedIn(&ts.Suite, result, raw)
	ts.Len(ts.service.callsFrom("ext_check"), 1, "ext_check should be called once")
	echoes := ts.service.callsFrom("ext_echo")
	ts.Require().Len(echoes, 1, "SUCCESS should continue to onSuccess")
	ts.Equal("3", echoes[0].Body.RuntimeData[extensionKey("ext_check", "riskScore")],
		"The answer should be stored under the step's namespace for the next step")
	ts.NotContains(echoes[0].Body.RuntimeData, extensionKey("ext_check", "actionStatus"),
		"actionStatus must never be stored")
	assertNoBareKeys(&ts.Suite, ts.service, ts.fixtures.userID, retryBareKeys...)
}

func (ts *ExtensionContractTestSuite) TestFailedRoutesToOnFailureWithDescription() {
	result, raw := ts.check(okAnswer(`{"actionStatus":"FAILED","failureReason":"high_risk",` +
		`"failureDescription":"Sign-in blocked. Contact support."}`))

	ts.assertBackAtCredentialsPrompt(result, raw, errCodeExtensionActionFailed)
	ts.Equal(msgExtensionActionFailed, result.Error.Message.DefaultValue, "The message should be the rejection")
	ts.Equal("Sign-in blocked. Contact support.", result.Error.Description.DefaultValue,
		"failureDescription should be the description the user sees")
	ts.Equal(map[string]string{"reason": "high_risk"}, result.Error.Description.Params,
		"failureReason should be the only param")
}

func (ts *ExtensionContractTestSuite) TestIncompleteRoutesToOnIncompletePrompt() {
	ts.Run("WithDescription", func() {
		ts.SetupTest()
		result, raw := ts.check(okAnswer(`{"actionStatus":"INCOMPLETE","failureReason":"otp_required",` +
			`"failureDescription":"Enter the code we sent you.","challengeId":"c-1"}`))

		ts.Require().Equal("INCOMPLETE", result.FlowStatus, "INCOMPLETE should wait for input: %s", raw)
		ts.True(common.ValidateRequiredInputs(result.Data.Inputs, []string{"otp"}),
			"onIncomplete should show the otp prompt")
		ts.False(common.HasInput(result.Data.Inputs, "password"), "The credentials prompt must not be shown")
		ts.Require().NotNil(result.Error, "The otp prompt should carry the service's message: %s", raw)
		ts.Equal(errCodeExtensionInputRequired, result.Error.Code, "The error should be the input request")
		ts.Equal(msgExtensionInputRequired, result.Error.Message.DefaultValue,
			"The message should be the input request")
		ts.Equal("Enter the code we sent you.", result.Error.Description.DefaultValue,
			"failureDescription should be the description the user sees")
		ts.Equal(map[string]string{"reason": "otp_required"}, result.Error.Description.Params,
			"failureReason should be the only param")
		ts.Empty(ts.service.callsFrom("ext_echo"), "Nothing should run before the otp is given")

		next, nextRaw := submit(&ts.Suite, result, map[string]string{"otp": "123456"}, "action_otp")
		ts.fixtures.assertSignedIn(&ts.Suite, next, nextRaw)
		echoes := ts.service.callsFrom("ext_echo")
		ts.Require().Len(echoes, 1, "The otp prompt should lead to ext_echo")
		ts.Equal(map[string]string{"otp": "123456"}, echoes[0].Body.UserInputs, "ext_echo names otp in inputKeys")
		ts.Equal(map[string]string{extensionKey("ext_check", "challengeId"): "c-1"}, echoes[0].Body.RuntimeData,
			"INCOMPLETE should store the answer's fields under the step's namespace only")
		assertNoBareKeys(&ts.Suite, ts.service, ts.fixtures.userID, retryBareKeys...)
	})

	ts.Run("WithoutDescription", func() {
		ts.SetupTest()
		result, raw := ts.check(okAnswer(`{"actionStatus":"INCOMPLETE"}`))
		ts.Require().Equal("INCOMPLETE", result.FlowStatus, "INCOMPLETE should wait for input: %s", raw)
		ts.True(common.ValidateRequiredInputs(result.Data.Inputs, []string{"otp"}),
			"onIncomplete should show the otp prompt")
		ts.Nil(result.Error, "INCOMPLETE without a description should return no error")
	})
}

func (ts *ExtensionContractTestSuite) TestAnswersOutsideTheContractAreGenericErrors() {
	leakyFields := `"riskScore":"` + extContractLeak + `","challengeId":"` + extContractLeak + `"`
	cases := []struct {
		name   string
		answer serviceAnswer
	}{
		{name: "ActionStatusError", answer: okAnswer(`{"actionStatus":"ERROR","failureReason":"` +
			extContractLeakReason + `","failureDescription":"db at ` + extContractLeak + ` down",` + leakyFields + `}`)},
		{name: "MissingActionStatus", answer: okAnswer(`{` + leakyFields + `}`)},
		{name: "UnknownActionStatus", answer: okAnswer(`{"actionStatus":"MAYBE",` + leakyFields + `}`)},
		{name: "LowercaseActionStatus", answer: okAnswer(`{"actionStatus":"success",` + leakyFields + `}`)},
		{name: "NonStringActionStatus", answer: okAnswer(`{"actionStatus":1,` + leakyFields + `}`)},
		{name: "EmptyBody", answer: okAnswer("")},
		{name: "ArrayBody", answer: okAnswer(`["SUCCESS","` + extContractLeak + `"]`)},
		{name: "StringBody", answer: okAnswer(`"` + extContractLeak + `"`)},
		{name: "NotJSON", answer: okAnswer(`actionStatus=SUCCESS&note=` + extContractLeak)},
		// A non-2xx status is an error even when the body claims SUCCESS.
		{name: "ServerErrorClaimingSuccess", answer: serviceAnswer{status: http.StatusServiceUnavailable,
			body: `{"actionStatus":"SUCCESS",` + leakyFields + `}`}},
		{name: "ClientErrorClaimingFailed", answer: serviceAnswer{status: http.StatusBadRequest,
			body: `{"actionStatus":"FAILED","failureDescription":"` + extContractLeak + `"}`}},
		{name: "Redirect", answer: serviceAnswer{status: http.StatusFound,
			body: `{"actionStatus":"SUCCESS",` + leakyFields + `}`}},
		{name: "BodyOverTheCap", answer: okAnswer(sizedBody(extContractBodyCap+1, extContractLeak))},
	}

	for _, tc := range cases {
		ts.Run(tc.name, func() {
			ts.SetupTest()
			answer := tc.answer
			if answer.status == http.StatusFound {
				answer.location = ts.service.target.URL + "/landing"
			}
			result, raw := ts.check(answer)

			ts.assertGenericCallFailure(result, raw)
			ts.Len(ts.service.callsFrom("ext_check"), 1, "The step should call the service once, with no retry")
			ts.Zero(ts.service.redirectTargetHits(), "A 3xx answer must never be followed")

			retried, _ := ts.retryWithSuccess(result)
			ts.assertNothingStored(retried)
		})
	}
}

func (ts *ExtensionContractTestSuite) TestBodyAtTheCapIsAccepted() {
	result, raw := ts.check(okAnswer(sizedBody(extContractBodyCap, "at-cap")))
	ts.fixtures.assertSignedIn(&ts.Suite, result, raw)
	echoes := ts.service.callsFrom("ext_echo")
	ts.Require().Len(echoes, 1, "An answer at the cap should continue to onSuccess")
	ts.Equal("at-cap", echoes[0].Body.RuntimeData[extensionKey("ext_check", "riskScore")],
		"An answer at the cap should be stored")
}

func (ts *ExtensionContractTestSuite) TestTimeoutIsAGenericError() {
	slow := serviceAnswer{status: http.StatusOK, body: `{"actionStatus":"SUCCESS","riskScore":"` +
		extContractLeak + `"}`, delay: 3 * time.Second}

	// The answer is held past the connection's timeout, which is the boundary under test.
	ts.useConnection(ts.service.connectionURL(), 1000)
	result, raw := ts.check(slow)
	ts.assertGenericCallFailure(result, raw)
	ts.Len(ts.service.callsFrom("ext_check"), 1, "A timeout must not be retried")

	ts.SetupTest()
	ts.useConnection(ts.service.connectionURL(), extContractTimeoutMS)
	result, raw = ts.check(slow)
	ts.fixtures.assertSignedIn(&ts.Suite, result, raw)
}

func (ts *ExtensionContractTestSuite) TestUnreachableServiceIsAGenericError() {
	closed := httptest.NewServer(http.NotFoundHandler())
	unreachableURL := closed.URL + servicePath
	closed.Close()

	ts.useConnection(unreachableURL, extContractTimeoutMS)
	result, raw := ts.check(okAnswer(successBody))
	ts.assertGenericCallFailure(result, raw)
}

func (ts *ExtensionContractTestSuite) TestIdentityNamedFieldOutsideResponseKeysIsDropped() {
	// ext_check stores only riskScore and challengeId, so a field outside them is dropped whatever its
	// name and the answer is used.
	for _, key := range []string{"userID", "userId", "mapped_role_ids", "ouId", "groups", "password"} {
		ts.Run(key, func() {
			ts.SetupTest()
			ts.service.answer("ext_check", okAnswer(`{"actionStatus":"SUCCESS","riskScore":"`+extContractLeak+
				`","`+key+`":"`+extContractLeakReason+`"}`))
			result, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
			ts.fixtures.assertSignedIn(&ts.Suite, result, raw)

			echoes := ts.service.callsFrom("ext_echo")
			ts.Require().Len(echoes, 1, "SUCCESS should continue to ext_echo")
			ts.Equal(extContractLeak, echoes[0].Body.RuntimeData[extensionKey("ext_check", "riskScore")],
				"The stored field should be kept")
			ts.NotContains(echoes[0].RawBody, extContractLeakReason, "The unstored %s must not reach runtime data", key)
			ts.NotContains(raw, extContractLeakReason, "The unstored %s must not reach the client", key)
		})
	}
}

func (ts *ExtensionContractTestSuite) TestFailureFieldsAreNeverStored() {
	result, raw := ts.check(okAnswer(`{"actionStatus":"FAILED","failureReason":"high_risk",` +
		`"failureDescription":"Sign-in blocked.","riskScore":"` + extContractLeak + `"}`))
	ts.assertBackAtCredentialsPrompt(result, raw, errCodeExtensionActionFailed)

	retried, echoed := ts.retryWithSuccess(result)
	// The retried request is built before its answer, so it shows what FAILED stored: nothing.
	ts.Empty(retried.Body.RuntimeData, "FAILED should store nothing")
	ts.Empty(echoed.Body.RuntimeData, "The SUCCESS after it stored no fields either")
	assertNoBareKeys(&ts.Suite, ts.service, ts.fixtures.userID, retryBareKeys...)
}

func (ts *ExtensionContractTestSuite) TestFailureDescriptionIsCleanedAndCut() {
	description := `Blocked\u0007 by    policy. ` + strings.Repeat(`abcdefghi\u001b`, 30)
	result, raw := ts.check(okAnswer(`{"actionStatus":"FAILED","failureDescription":"` + description + `"}`))
	ts.assertBackAtCredentialsPrompt(result, raw, errCodeExtensionActionFailed)

	ts.NotContains(result.Error.Description.Params, "description", "The text should not be passed as a param")
	for label, text := range map[string]string{
		"description":          result.Error.Description.DefaultValue,
		"rendered description": renderedDescription(result.Error),
	} {
		ts.LessOrEqual(utf8.RuneCountInString(text), 256, "%s should be cut to 256 characters", label)
		ts.False(strings.ContainsFunc(text, unicode.IsControl), "%s should hold no control characters: %q",
			label, text)
		ts.True(strings.HasPrefix(text, "Blocked by policy. abcdefghi"),
			"%s should keep the text with control characters removed and spaces collapsed: %q", label, text)
	}
}

func (ts *ExtensionContractTestSuite) TestFailureDescriptionLosesFormatCharacters() {
	// A right-to-left override (U+202E) and its terminator (U+202C) could make the text read
	// differently from what it says; a zero-width space (U+200B) and word joiner (U+2060) hide inside
	// a word. All four are Unicode category Cf and must be removed, not replaced with a space.
	result, raw := ts.check(okAnswer(`{"actionStatus":"FAILED","failureDescription":` +
		`"Sign-in \u202eblocked\u202c by pol\u200bi\u2060cy."}`))
	ts.assertBackAtCredentialsPrompt(result, raw, errCodeExtensionActionFailed)

	ts.NotContains(result.Error.Description.Params, "description", "The text should not be passed as a param")
	for label, text := range map[string]string{
		"description":          result.Error.Description.DefaultValue,
		"rendered description": renderedDescription(result.Error),
	} {
		ts.Equal("Sign-in blocked by policy.", text, "%s should keep the text without format characters", label)
		ts.False(strings.ContainsFunc(text, func(r rune) bool { return unicode.Is(unicode.Cf, r) }),
			"%s should hold no format characters: %q", label, text)
	}
}

func (ts *ExtensionContractTestSuite) TestCredentialFailureIsTheNativeCredentialError() {
	// Control: what a native wrong password shows on the same prompt.
	native, nativeRaw := submit(&ts.Suite, ts.fixtures.startLogin(&ts.Suite),
		map[string]string{"username": ts.fixtures.username, "password": "Wrong" + ts.fixtures.password},
		credentialsAction)
	ts.Require().Equal("INCOMPLETE", native.FlowStatus, "A wrong password should re-show the prompt: %s", nativeRaw)
	ts.Require().NotNil(native.Error, "A wrong password should carry an error: %s", nativeRaw)
	ts.Require().Empty(ts.service.allCalls(), "A wrong password must not reach the extension")
	ts.Equal(common.ErrorResponse{
		Code:        errCodeInvalidCredentials,
		Message:     common.I18nMessage{Key: keyInvalidCredentials, DefaultValue: msgInvalidCredentials},
		Description: common.I18nMessage{Key: keyInvalidCredentialsDesc, DefaultValue: descInvalidCredentials},
	}, *native.Error, "The native wrong-password error")

	errors := map[string]*common.ErrorResponse{}
	for _, status := range []string{"FAILED", "INCOMPLETE"} {
		for _, reason := range []string{"invalid_credentials", "user_not_found"} {
			name := status + "_" + reason
			ts.Run(name, func() {
				ts.SetupTest()
				result, raw := ts.check(okAnswer(`{"actionStatus":"` + status + `","failureReason":"` + reason +
					`","failureDescription":"No account for ` + extContractLeak + `"}`))
				if status == "FAILED" {
					ts.assertBackAtCredentialsPrompt(result, raw, errCodeInvalidCredentials)
					ts.Equal(native.Data.Inputs, result.Data.Inputs, "onFailure should show the native prompt")
				} else {
					ts.Require().Equal("INCOMPLETE", result.FlowStatus, "INCOMPLETE should wait for input: %s", raw)
					ts.True(common.ValidateRequiredInputs(result.Data.Inputs, []string{"otp"}),
						"INCOMPLETE should still follow onIncomplete")
					ts.Require().NotNil(result.Error, "The otp prompt should carry the credential error: %s", raw)
				}
				ts.Equal(native.Error, result.Error, "A credential failure should be the native credential error")
				// The native error's own i18n key contains "invalid_credentials", so only the service's text is
				// searched for; the equality check above proves no reason or description is passed.
				ts.NotContains(raw, extContractLeak, "The service's text must not reach the client")
				errors[name] = result.Error
			})
		}
		ts.Equal(errors[status+"_invalid_credentials"], errors[status+"_user_not_found"],
			"%s: the two credential failures must be indistinguishable", status)
	}
}

func (ts *ExtensionContractTestSuite) TestFailedWithoutDescriptionUsesDefault() {
	result, raw := ts.check(okAnswer(`{"actionStatus":"FAILED"}`))
	ts.assertBackAtCredentialsPrompt(result, raw, errCodeExtensionActionFailed)
	ts.Equal(descDefaultRejection, result.Error.Description.DefaultValue,
		"FAILED without failureDescription should show the default description")
	ts.Empty(result.Error.Description.Params, "No param without a failureReason")
}

func (ts *ExtensionContractTestSuite) TestInvalidFailureReasonIsDropped() {
	cases := []struct {
		name   string
		reason string
		kept   bool
	}{
		{name: "BadCharacters", reason: "bad reason!"},
		{name: "Markup", reason: "<b>x</b>"},
		{name: "SixtyFiveCharacters", reason: strings.Repeat("a", 65)},
		{name: "SixtyFourCharacters", reason: strings.Repeat("a", 63) + ".", kept: true},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			ts.SetupTest()
			result, raw := ts.check(okAnswer(`{"actionStatus":"FAILED","failureReason":"` + tc.reason +
				`","failureDescription":"Sign-in blocked."}`))
			ts.assertBackAtCredentialsPrompt(result, raw, errCodeExtensionActionFailed)
			ts.Equal("Sign-in blocked.", renderedDescription(result.Error), "The description should still be shown")
			if tc.kept {
				ts.Equal(tc.reason, result.Error.Description.Params["reason"], "A valid reason should be passed")
			} else {
				ts.NotContains(result.Error.Description.Params, "reason", "An invalid reason should be dropped")
			}
		})
	}
}

// useRetryFlow binds the application to a retryExtensionFlow with maxAttempts (nil leaves it unset)
// until the test ends.
func (ts *ExtensionContractTestSuite) useRetryFlow(handle string, maxAttempts interface{}) {
	ts.T().Helper()
	flowID := createTestFlow(&ts.Suite, retryExtensionFlow("Extension Contract Retry Flow "+handle, handle,
		ts.connectionID, ts.service.server.URL, maxAttempts))
	ts.fixtures.bindFlow(&ts.Suite, flowID)
}

// failAtCredentials submits the credentials with ext_check answering FAILED, and checks onFailure
// showed the credentials prompt again.
func (ts *ExtensionContractTestSuite) failAtCredentials(step *common.FlowStep) *common.FlowStep {
	ts.T().Helper()
	ts.service.answer(retryCheckNode, okAnswer(`{"actionStatus":"FAILED","failureDescription":"Wrong code."}`))
	result, raw := ts.fixtures.submitCredentials(&ts.Suite, step)
	ts.Require().Equal("INCOMPLETE", result.FlowStatus, "FAILED should show the credentials prompt again: %s", raw)
	ts.Require().True(common.ValidateRequiredInputs(result.Data.Inputs, []string{"username", "password"}),
		"onFailure should lead to the credentials prompt: %s", raw)
	ts.Require().NotNil(result.Error, "The prompt should carry the rejection: %s", raw)
	ts.Require().Equal(errCodeExtensionActionFailed, result.Error.Code, "The prompt should carry the rejection")
	return result
}

func (ts *ExtensionContractTestSuite) TestAttemptLimitEndsTheFlowWithoutCallingTheService() {
	ts.useRetryFlow("ext_contract_retry_limit_flow", 2)

	// Arrival 1 answers FAILED and arrival 2 answers INCOMPLETE; both count towards the limit.
	prompt := ts.failAtCredentials(ts.fixtures.startLogin(&ts.Suite))
	ts.service.answer(retryCheckNode, okAnswer(`{"actionStatus":"INCOMPLETE","failureDescription":"Enter the code."}`))
	codePrompt, raw := ts.fixtures.submitCredentials(&ts.Suite, prompt)
	ts.Require().Equal("INCOMPLETE", codePrompt.FlowStatus, "INCOMPLETE should show the code prompt: %s", raw)
	ts.Require().True(common.ValidateRequiredInputs(codePrompt.Data.Inputs, []string{codeInput}),
		"onIncomplete should lead to the code prompt: %s", raw)
	ts.Require().Len(ts.service.callsFrom(retryCheckNode), 2, "Both arrivals under the limit should call the service")

	// Arrival 3 is past the limit. The service would now answer SUCCESS, so a call would sign the user in.
	// The refusal is a failure, so it follows onFailure back to the credentials prompt.
	ts.service.answer(retryCheckNode, okAnswer(successBody))
	result, raw := submit(&ts.Suite, codePrompt, map[string]string{codeInput: "123456"}, codeAction)
	assertAttemptsExceededAtCredentials(&ts.Suite, result, raw)
	ts.Len(ts.service.callsFrom(retryCheckNode), 2, "The step must not call the service once its limit is reached")

	// Every later arrival in the execution is refused the same way, still without a call.
	result, raw = ts.fixtures.submitCredentials(&ts.Suite, result)
	assertAttemptsExceededAtCredentials(&ts.Suite, result, raw)
	ts.Len(ts.service.callsFrom(retryCheckNode), 2, "A later arrival must not call the service either")
	ts.Empty(ts.service.callsFrom(retryEchoNode), "A refused step must not continue to onSuccess")

	// The count belongs to one execution, so a new sign-in calls the service again.
	ts.service.reset()
	result, raw = ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
	ts.fixtures.assertSignedIn(&ts.Suite, result, raw)
	ts.Len(ts.service.callsFrom(retryCheckNode), 1, "A new execution should start with no attempts counted")
}

func (ts *ExtensionContractTestSuite) TestAttemptLimitAllowsTheLastAttempt() {
	ts.useRetryFlow("ext_contract_retry_last_flow", 2)

	// The control for the limit test: the second arrival is still under a limit of 2 and is called.
	prompt := ts.failAtCredentials(ts.fixtures.startLogin(&ts.Suite))
	ts.service.answer(retryCheckNode, okAnswer(successBody))
	result, raw := ts.fixtures.submitCredentials(&ts.Suite, prompt)
	ts.fixtures.assertSignedIn(&ts.Suite, result, raw)
	ts.Len(ts.service.callsFrom(retryCheckNode), 2, "The last attempt under the limit should call the service")
}

func (ts *ExtensionContractTestSuite) TestErrorAnswersDoNotCountTowardsTheLimit() {
	ts.useRetryFlow("ext_contract_retry_error_flow", 1)

	// With a limit of 1, a counted ERROR would stop the second arrival.
	prompt := ts.fixtures.startLogin(&ts.Suite)
	for attempt := 1; attempt <= 2; attempt++ {
		ts.service.answer(retryCheckNode, okAnswer(`{"actionStatus":"ERROR"}`))
		result, raw := ts.fixtures.submitCredentials(&ts.Suite, prompt)
		ts.Require().Equal("INCOMPLETE", result.FlowStatus, "ERROR %d should follow onFailure: %s", attempt, raw)
		ts.Require().NotNil(result.Error, "ERROR %d should carry the generic error: %s", attempt, raw)
		ts.Require().Equal(errCodeExtensionCallFailed, result.Error.Code, "ERROR %d should not be the limit", attempt)
		prompt = result
	}
	ts.service.answer(retryCheckNode, okAnswer(successBody))
	result, raw := ts.fixtures.submitCredentials(&ts.Suite, prompt)
	ts.fixtures.assertSignedIn(&ts.Suite, result, raw)
	ts.Len(ts.service.callsFrom(retryCheckNode), 3, "Every arrival after an ERROR should call the service")
}

func (ts *ExtensionContractTestSuite) TestDefaultAttemptLimitIsFive() {
	ts.useRetryFlow("ext_contract_retry_default_flow", nil)

	prompt := ts.fixtures.startLogin(&ts.Suite)
	for attempt := 1; attempt <= 5; attempt++ {
		prompt = ts.failAtCredentials(prompt)
	}
	ts.Require().Len(ts.service.callsFrom(retryCheckNode), 5, "Five arrivals should be under the default limit")

	ts.service.answer(retryCheckNode, okAnswer(successBody))
	result, raw := ts.fixtures.submitCredentials(&ts.Suite, prompt)
	assertAttemptsExceededAtCredentials(&ts.Suite, result, raw)
	ts.Len(ts.service.callsFrom(retryCheckNode), 5, "The sixth arrival must not call the service")
}

func (ts *ExtensionContractTestSuite) TestAttemptLimitWithoutOnFailureEndsTheFlow() {
	// INCOMPLETE leads to the code prompt and back, and there is no onFailure for the refusal to follow.
	flowID := createTestFlow(&ts.Suite, testutils.Flow{
		Name:     "Extension Contract Limit Without OnFailure Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "ext_contract_limit_no_failure_flow",
		Nodes: []map[string]interface{}{
			startNode("prompt_credentials"),
			promptCredentialsNode("credentials_auth"),
			credentialsAuthNode(retryCheckNode),
			extensionNode(retryCheckNode, map[string]interface{}{"connectionId": ts.connectionID, "maxAttempts": 1},
				map[string]string{"onSuccess": "auth_assert", "onIncomplete": "prompt_code"}),
			codePromptNode("prompt_code", retryCheckNode),
			authAssertNode(),
			endNode(),
		},
	})
	ts.fixtures.bindFlow(&ts.Suite, flowID)

	ts.service.answer(retryCheckNode, okAnswer(`{"actionStatus":"INCOMPLETE"}`))
	codePrompt, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
	ts.Require().Equal("INCOMPLETE", codePrompt.FlowStatus, "INCOMPLETE should show the code prompt: %s", raw)
	ts.Require().True(common.ValidateRequiredInputs(codePrompt.Data.Inputs, []string{codeInput}),
		"onIncomplete should lead to the code prompt: %s", raw)

	ts.service.answer(retryCheckNode, okAnswer(successBody))
	result, raw := submit(&ts.Suite, codePrompt, map[string]string{codeInput: "123456"}, codeAction)
	ts.Equal("ERROR", result.FlowStatus, "Without onFailure the refusal should end the flow: %s", raw)
	assertAttemptsExceeded(&ts.Suite, result, raw)
	ts.Len(ts.service.callsFrom(retryCheckNode), 1, "The refused arrival must not call the service")
}

func (ts *ExtensionContractTestSuite) TestEachCallReplacesTheStepsNamespace() {
	ts.useRetryFlow("ext_contract_retry_replace_flow", nil)
	riskScore, challengeID := extensionKey(retryCheckNode, "riskScore"), extensionKey(retryCheckNode, "challengeId")

	ts.service.answer(retryCheckNode, okAnswer(`{"actionStatus":"INCOMPLETE","challengeId":"c-1","riskScore":"40"}`))
	codePrompt, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
	ts.Require().Equal("INCOMPLETE", codePrompt.FlowStatus, "INCOMPLETE should show the code prompt: %s", raw)
	ts.Require().True(common.ValidateRequiredInputs(codePrompt.Data.Inputs, []string{codeInput}),
		"onIncomplete should lead to the code prompt: %s", raw)

	ts.service.answer(retryCheckNode, okAnswer(`{"actionStatus":"SUCCESS","riskScore":"5"}`))
	result, raw := submit(&ts.Suite, codePrompt, map[string]string{codeInput: "123456"}, codeAction)
	ts.fixtures.assertSignedIn(&ts.Suite, result, raw)

	checks := ts.service.callsFrom(retryCheckNode)
	ts.Require().Len(checks, 2, "The code prompt should lead back to ext_check")
	// The request is built before the namespace is replaced, so the step can send its own last answer.
	ts.Equal(map[string]string{riskScore: "40", challengeID: "c-1"}, checks[1].Body.RuntimeData,
		"The second call should be sent what the first answer stored")

	echoes := ts.service.callsFrom(retryEchoNode)
	ts.Require().Len(echoes, 1, "SUCCESS should continue to ext_echo")
	ts.Equal("5", echoes[0].Body.RuntimeData[riskScore], "The namespace should hold the latest answer")
	ts.NotContains(echoes[0].Body.RuntimeData, challengeID,
		"A field the latest answer did not return must not survive from an earlier answer")
	assertNoBareKeys(&ts.Suite, ts.service, ts.fixtures.userID, retryBareKeys...)
}
