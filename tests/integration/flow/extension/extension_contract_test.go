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
//	      SUCCESS    (onSuccess)    -> ext_echo -> auth_assert -> end
//	      FAILED     (onFailure)    -> prompt_credentials
//	      INCOMPLETE (onIncomplete) -> prompt_otp -> ext_echo -> auth_assert -> end
//	      ERROR      (onFailure)    -> prompt_credentials
//
// ext_check stores only riskScore and challengeId, and names them and the failure fields in
// runtimeDataKeys, so a retry shows in its request what the previous attempt stored. ext_echo exists
// to read what ext_check stored, and is sent the otp the user gives.
func extContractFlow(connectionID string) testutils.Flow {
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
				"runtimeDataKeys": []string{"riskScore", "challengeId", "failureReason", "failureDescription"},
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
				"connectionId": connectionID,
				"inputKeys":    []string{"otp"},
				"runtimeDataKeys": []string{"riskScore", "challengeId", "failureReason", "failureDescription",
					"actionStatus"},
			}, map[string]string{"onSuccess": "auth_assert"}),
			authAssertNode(),
			endNode(),
		},
	}
}

// ExtensionContractTestSuite covers the fixed response contract (R4), what each answer stores (AC6.3,
// AC6.5) and the failure text the user sees (R7).
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

	flowID, err := testutils.CreateFlow(extContractFlow(ts.connectionID))
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
	return testutils.HTTPConnection{Name: extContractConnectionName, URL: url, TimeoutMS: timeoutMS}
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
	for _, key := range []string{"riskScore", "challengeId", "failureReason", "failureDescription"} {
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
	ts.Equal("3", echoes[0].Body.RuntimeData["riskScore"], "The answer should be stored for the next step")
}

func (ts *ExtensionContractTestSuite) TestFailedRoutesToOnFailureWithDescription() {
	result, raw := ts.check(okAnswer(`{"actionStatus":"FAILED","failureReason":"high_risk",` +
		`"failureDescription":"Sign-in blocked. Contact support."}`))

	ts.assertBackAtCredentialsPrompt(result, raw, errCodeExtensionActionFailed)
	ts.Equal(msgExtensionActionFailed, result.Error.Message.DefaultValue, "The message should be the rejection")
	ts.Equal("Sign-in blocked. Contact support.", result.Error.Description.Params["description"],
		"failureDescription should be passed as params.description")
	ts.Equal("Sign-in blocked. Contact support.", renderedDescription(result.Error),
		"failureDescription should be the description the user sees")
	ts.Equal("high_risk", result.Error.Description.Params["reason"], "failureReason should be passed as params.reason")
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
		ts.Equal("Enter the code we sent you.", renderedDescription(result.Error),
			"failureDescription should be the description the user sees")
		ts.Equal("otp_required", result.Error.Description.Params["reason"],
			"failureReason should be passed as params.reason")
		ts.Empty(ts.service.callsFrom("ext_echo"), "Nothing should run before the otp is given")

		next, nextRaw := submit(&ts.Suite, result, map[string]string{"otp": "123456"}, "action_otp")
		ts.fixtures.assertSignedIn(&ts.Suite, next, nextRaw)
		echoes := ts.service.callsFrom("ext_echo")
		ts.Require().Len(echoes, 1, "The otp prompt should lead to ext_echo")
		ts.Equal(map[string]string{"otp": "123456"}, echoes[0].Body.UserInputs, "ext_echo names otp in inputKeys")
		ts.Equal(map[string]string{
			"challengeId":        "c-1",
			"failureReason":      "otp_required",
			"failureDescription": "Enter the code we sent you.",
		}, echoes[0].Body.RuntimeData, "INCOMPLETE should store the answer's fields and the failure fields")
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
	ts.Equal("at-cap", echoes[0].Body.RuntimeData["riskScore"], "An answer at the cap should be stored")
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

func (ts *ExtensionContractTestSuite) TestReservedKeyInResponseFailsTheStep() {
	for _, key := range []string{"userID", "userId", "mapped_role_ids", "ouId", "groups", "password"} {
		ts.Run(key, func() {
			ts.SetupTest()
			result, raw := ts.check(okAnswer(`{"actionStatus":"SUCCESS","riskScore":"` + extContractLeak +
				`","` + key + `":"` + extContractLeakReason + `"}`))
			ts.assertGenericCallFailure(result, raw)

			retried, _ := ts.retryWithSuccess(result)
			ts.assertNothingStored(retried)
		})
	}
}

func (ts *ExtensionContractTestSuite) TestFailureFieldsClearedAfterRetrySucceeds() {
	result, raw := ts.check(okAnswer(`{"actionStatus":"FAILED","failureReason":"high_risk",` +
		`"failureDescription":"Sign-in blocked."}`))
	ts.assertBackAtCredentialsPrompt(result, raw, errCodeExtensionActionFailed)

	retried, echoed := ts.retryWithSuccess(result)
	// The retried request is built before its answer, so it shows what FAILED stored.
	ts.Equal("high_risk", retried.Body.RuntimeData["failureReason"], "FAILED should store failureReason")
	ts.Equal("Sign-in blocked.", retried.Body.RuntimeData["failureDescription"],
		"FAILED should store failureDescription")
	ts.Empty(echoed.Body.RuntimeData["failureReason"], "SUCCESS should empty an earlier failureReason")
	ts.Empty(echoed.Body.RuntimeData["failureDescription"], "SUCCESS should empty an earlier failureDescription")
}

func (ts *ExtensionContractTestSuite) TestFailureDescriptionIsCleanedAndCut() {
	description := `Blocked\u0007 by    policy. ` + strings.Repeat(`abcdefghi\u001b`, 30)
	result, raw := ts.check(okAnswer(`{"actionStatus":"FAILED","failureDescription":"` + description + `"}`))
	ts.assertBackAtCredentialsPrompt(result, raw, errCodeExtensionActionFailed)

	for label, text := range map[string]string{
		"params.description":   result.Error.Description.Params["description"],
		"rendered description": renderedDescription(result.Error),
	} {
		ts.LessOrEqual(utf8.RuneCountInString(text), 256, "%s should be cut to 256 characters", label)
		ts.False(strings.ContainsFunc(text, unicode.IsControl), "%s should hold no control characters: %q",
			label, text)
		ts.True(strings.HasPrefix(text, "Blocked by policy. abcdefghi"),
			"%s should keep the text with control characters removed and spaces collapsed: %q", label, text)
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
	ts.Equal(descDefaultRejection, renderedDescription(result.Error),
		"FAILED without failureDescription should show the default description")
	ts.NotContains(result.Error.Description.Params, "reason", "No reason param without a failureReason")
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
