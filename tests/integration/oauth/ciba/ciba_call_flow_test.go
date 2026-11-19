// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ciba

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// cibaCallClientID/Secret is bound to a CIBA flow that calls an existing authentication flow.
	cibaCallClientID     = "ciba_test_client_call"
	cibaCallClientSecret = "ciba_test_secret_call"

	cibaApproveAction = "action_approve"
	cibaDenyAction    = "action_deny"
)

// setUpCallFlowApp creates a plain sign-in flow, a CIBA flow that calls it, and an application whose
// authentication flow is the sign-in flow and whose CIBA flow is the one that calls it.
func (ts *CIBATestSuite) setUpCallFlowApp() {
	innerFlowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "CIBA Test Called Sign-in Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_ciba_call_inner_test",
		Nodes:    cibaSignInFlowNodes(),
	})
	ts.Require().NoError(err, "Failed to create the sign-in flow the CIBA flow calls")
	ts.callInnerFlowID = innerFlowID

	callFlowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "CIBA Test Call Flow",
		FlowType: "CIBA",
		Handle:   "ciba_flow_call_test",
		Nodes:    cibaCallFlowNodes(ts.senderID, innerFlowID),
	})
	ts.Require().NoError(err, "Failed to create the CIBA flow that calls an authentication flow")
	ts.callFlowID = callFlowID

	ts.callAppID = ts.createCIBAAppWithCIBAFlow("CIBACallTestApp", cibaCallClientID, cibaCallClientSecret,
		[]string{cibaGrantType, "refresh_token"}, innerFlowID, callFlowID)
}

// tearDownCallFlowApp removes what setUpCallFlowApp created.
func (ts *CIBATestSuite) tearDownCallFlowApp() {
	if ts.callAppID != "" {
		_ = testutils.DeleteApplication(ts.callAppID)
	}
	if ts.callFlowID != "" {
		_ = testutils.DeleteFlow(ts.callFlowID)
	}
	if ts.callInnerFlowID != "" {
		_ = testutils.DeleteFlow(ts.callInnerFlowID)
	}
}

// cibaSignInFlowNodes builds an ordinary sign-in flow: credentials, a permission check, and the
// assertion. It has no CIBA knowledge of its own.
func cibaSignInFlowNodes() []map[string]interface{} {
	credentialInputs := []map[string]interface{}{
		{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	return []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
		{
			"id":   "prompt_credentials",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{{
				"inputs": credentialInputs,
				"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
			}},
		},
		{
			"id":        "credentials_auth",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": credentialInputs},
			"onSuccess": "authorization_check",
		},
		{
			"id":        "authorization_check",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthorizationExecutor"},
			"onSuccess": "auth_assert",
		},
		{
			"id":        "auth_assert",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
			"onSuccess": "end",
		},
		{"id": "end", "type": "END"},
	}
}

// cibaCallFlowNodes builds a CIBA flow that identifies the user from the login hint, sends the
// notification, and once the invite is verified asks the user to approve or deny. An approval calls the
// sign-in flow, which issues the assertion, so this flow holds no assertion step of its own. A denial
// shows a message and ends.
func cibaCallFlowNodes(senderID, calledFlowID string) []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "identify_user"},
		{
			"id":         "identify_user",
			"type":       "TASK_EXECUTION",
			"executor":   map[string]interface{}{"name": "IdentifyingExecutor", "mode": "identify"},
			"properties": map[string]interface{}{"loginHintAttribute": "username"},
			"onSuccess":  "generate_invite",
		},
		{
			"id":        "generate_invite",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "InviteExecutor", "mode": "generate"},
			"onSuccess": "send_ciba_notification",
		},
		{
			"id":         "send_ciba_notification",
			"type":       "TASK_EXECUTION",
			"properties": map[string]interface{}{"senderId": senderID, "smsTemplate": "ciba-notification"},
			"executor":   map[string]interface{}{"name": "SMSExecutor"},
			"onSuccess":  "notification_sent",
		},
		{"id": "notification_sent", "type": "PROMPT", "next": "verify_invite"},
		{
			"id":   "verify_invite",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "InviteExecutor",
				"mode": "verify",
				"inputs": []map[string]interface{}{
					{"ref": "input_invite_token", "identifier": "inviteToken", "type": "HIDDEN", "required": true},
				},
			},
			"onSuccess": "approve_request",
		},
		{
			"id":   "approve_request",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{"action": map[string]interface{}{"ref": cibaApproveAction, "nextNode": "call_sign_in"}},
				{"action": map[string]interface{}{"ref": cibaDenyAction, "nextNode": "request_denied"}},
			},
		},
		{"id": "request_denied", "type": "PROMPT", "next": "end"},
		{
			"id":        "call_sign_in",
			"type":      "CALL",
			"flow":      map[string]interface{}{"ref": calledFlowID},
			"onSuccess": "auth_success",
		},
		{"id": "auth_success", "type": "PROMPT", "next": "end"},
		{"id": "end", "type": "END"},
	}
}

// startCallFlowRequest sends a backchannel request to the call-flow client and resumes the flow from the
// invite link, returning the request, the execution and the step the link opens, the approve or deny prompt.
func (ts *CIBATestSuite) startCallFlowRequest(bindingMessage string) (authReqID, executionID string,
	prompt *testutils.FlowStep) {
	ts.mockServer.ClearMessages()
	form := url.Values{"login_hint": {cibaTestUsername}, "scope": {"openid"}}
	if bindingMessage != "" {
		form.Set("binding_message", bindingMessage)
	}
	status, bc := ts.cibaBackchannelAuthorizeForm(form, cibaCallClientID, cibaCallClientSecret)
	ts.Require().Equal(http.StatusOK, status, "bc-authorize should succeed")
	ts.Require().NotEmpty(bc.AuthReqID)

	executionID, inviteToken := ts.recoverCIBAInvite(bc.AuthReqID)
	prompt, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{"inviteToken": inviteToken}, "")
	ts.Require().NoError(err, "should resume the flow with the invite token")
	return bc.AuthReqID, executionID, prompt
}

// An approval signs the user in through the called flow, whose own assertion completes the request: this
// flow holds none, so the called flow's assertion carries the request id the callback checks.
func (ts *CIBATestSuite) TestCIBAFlowCallingAuthenticationFlow() {
	const bindingMessage = "Approve the call center login"
	authReqID, executionID, prompt := ts.startCallFlowRequest(bindingMessage)

	ts.Require().Equal("INCOMPLETE", prompt.FlowStatus)
	ts.Require().NotNil(prompt.Data)
	var actions []string
	for _, action := range prompt.Data.Actions {
		actions = append(actions, action.Ref)
	}
	ts.Require().ElementsMatch([]string{cibaApproveAction, cibaDenyAction}, actions,
		"the user is first asked to approve or deny")
	ts.Equal(bindingMessage, prompt.Data.AdditionalData["bindingMessage"],
		"the approve prompt shows the request's binding message")

	signIn, err := testutils.ExecuteAuthenticationFlow(executionID, nil, cibaApproveAction, prompt.ChallengeToken)
	ts.Require().NoError(err, "approving should open the called sign-in flow")
	ts.Require().Equal("INCOMPLETE", signIn.FlowStatus)
	ts.Require().NotNil(signIn.Data)
	ts.Equal(bindingMessage, signIn.Data.AdditionalData["bindingMessage"],
		"the called flow's screens show it too")

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": cibaTestUsername,
		"password": cibaTestPassword,
	}, "action_001", signIn.ChallengeToken)
	ts.Require().NoError(err, "should complete the called sign-in flow")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)
	ts.Require().NotEmpty(flowStep.Assertion, "the called flow should yield the assertion")

	callbackStatus, callbackErr := ts.cibaPostCallback(authReqID, flowStep.Assertion)
	ts.Require().Equal(http.StatusOK, callbackStatus,
		"the callback should accept an assertion bound to the request: %s", callbackErr)

	tokenRes := ts.cibaPollTokenAs(authReqID, "", cibaCallClientID, cibaCallClientSecret)
	if tokenRes.statusCode == http.StatusBadRequest && tokenRes.errorCode == "slow_down" {
		time.Sleep(cibaPollIntervalSeconds * time.Second)
		tokenRes = ts.cibaPollTokenAs(authReqID, "", cibaCallClientID, cibaCallClientSecret)
	}
	ts.Require().Equal(http.StatusOK, tokenRes.statusCode, "the authenticated request should issue tokens")

	claims, err := testutils.DecodeJWT(tokenRes.accessToken)
	ts.Require().NoError(err)
	ts.Require().Equal(ts.userID, claims.Sub, "token subject should be the CIBA user")
}

// Denying shows the denial message and ends the flow without calling the sign-in flow, so no assertion is
// issued. The flow completes without relaying anything to the request, which therefore stays pending.
func (ts *CIBATestSuite) TestCIBADenyEndsTheFlowWithoutSigningIn() {
	authReqID, executionID, prompt := ts.startCallFlowRequest("")

	denied, err := testutils.ExecuteAuthenticationFlow(executionID, nil, cibaDenyAction, prompt.ChallengeToken)
	ts.Require().NoError(err, "denying should end the flow")
	ts.Equal("COMPLETE", denied.FlowStatus)
	ts.Empty(denied.Assertion, "a denied request must not produce an assertion")
	ts.Empty(denied.ErrorAssertion)

	pending := ts.cibaPollTokenAs(authReqID, "", cibaCallClientID, cibaCallClientSecret)
	ts.Equal(http.StatusBadRequest, pending.statusCode)
	ts.Equal("authorization_pending", pending.errorCode, "the request waits until it expires")
}

// Credentials of a different user must not complete a request made for another user, whether the
// credential steps sit in the CIBA flow itself or in a flow it calls, and the user it names can still approve.
func (ts *CIBATestSuite) TestCIBAApprovalByAnotherUserIsRefused() {
	otherUsername, otherPassword := "ciba_other_user", "cibaother123"
	otherID, err := testutils.CreateUser(testutils.User{
		OUID: ts.ouID,
		Type: "ciba-test-person",
		Attributes: json.RawMessage(`{
			"username": "` + otherUsername + `",
			"password": "` + otherPassword + `",
			"email": "ciba_other_user@example.com",
			"mobile_number": "+1987654322"
		}`),
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteUser(otherID) }()

	for name, tc := range map[string]struct {
		clientID, clientSecret string
		approveFirst           bool
	}{
		"flow with its own credential steps":  {cibaClientID, cibaClientSecret, false},
		"flow calling an authentication flow": {cibaCallClientID, cibaCallClientSecret, true},
	} {
		ts.Run(name, func() {
			ts.mockServer.ClearMessages()
			form := url.Values{"login_hint": {cibaTestUsername}, "scope": {"openid"}}
			status, bcResp := ts.cibaBackchannelAuthorizeForm(form, tc.clientID, tc.clientSecret)
			ts.Require().Equal(http.StatusOK, status)

			executionID, inviteToken := ts.recoverCIBAInvite(bcResp.AuthReqID)
			step, err := testutils.ExecuteAuthenticationFlow(executionID,
				map[string]string{"inviteToken": inviteToken}, "")
			ts.Require().NoError(err)
			if tc.approveFirst {
				step, err = testutils.ExecuteAuthenticationFlow(executionID, nil, cibaApproveAction, step.ChallengeToken)
				ts.Require().NoError(err)
			}

			flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
				"username": otherUsername,
				"password": otherPassword,
			}, "action_001", step.ChallengeToken)
			ts.Require().NoError(err)
			ts.Require().NotEqual("COMPLETE", flowStep.FlowStatus,
				"another user's credentials must not complete the request")
			ts.Require().Empty(flowStep.Assertion)

			// The refusal asks again instead of ending the request, so the user it names can still approve.
			ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
			approved, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
				"username": cibaTestUsername,
				"password": cibaTestPassword,
			}, "action_001", flowStep.ChallengeToken)
			ts.Require().NoError(err)
			ts.Require().Equal("COMPLETE", approved.FlowStatus)
			ts.Require().NotEmpty(approved.Assertion)
		})
	}
}

// The application keeps the CIBA flow it was given.
func (ts *CIBATestSuite) TestApplicationReturnsItsCIBAFlow() {
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+"/applications/"+ts.callAppID, nil)
	ts.Require().NoError(err)
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var app map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&app))
	ts.Equal(ts.callFlowID, app["cibaFlowId"])
	ts.Equal(ts.callInnerFlowID, app["authFlowId"])
}

// postApplication creates an application with the given CIBA flow and returns the status and response.
func (ts *CIBATestSuite) postApplication(name, clientID, authFlowID, cibaFlowID string) (int, string) {
	app := map[string]interface{}{
		"name":                      name,
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"authFlowId":                authFlowID,
		"cibaFlowId":                cibaFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"ciba-test-person"},
		"inboundAuthConfig": []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                clientID,
				"clientSecret":            "ciba_rejected_secret",
				"redirectUris":            []string{"https://localhost:3000"},
				"grantTypes":              []string{cibaGrantType},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	}
	payload, err := json.Marshal(app)
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/applications", bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, string(body)
}

// Only a CIBA flow fits the CIBA slot, and it has to call the application's own authentication flow.
func (ts *CIBATestSuite) TestApplicationRefusesAnInvalidCIBAFlow() {
	status, body := ts.postApplication("CIBAWrongTypeApp", "ciba_wrong_type_client", ts.callInnerFlowID,
		ts.callInnerFlowID)
	ts.Equal(http.StatusBadRequest, status, "an authentication flow is not a CIBA flow: %s", body)
	ts.Contains(body, "APP-1071")

	otherAuthFlowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "CIBA Test Other Sign-in Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_ciba_call_other_test",
		Nodes:    cibaSignInFlowNodes(),
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteFlow(otherAuthFlowID) }()
	callsOtherFlowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "CIBA Test Flow Calling Another Sign-in Flow",
		FlowType: "CIBA",
		Handle:   "ciba_flow_call_other_test",
		Nodes:    cibaCallFlowNodes(ts.senderID, otherAuthFlowID),
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteFlow(callsOtherFlowID) }()

	status, body = ts.postApplication("CIBAOtherAuthFlowApp", "ciba_other_auth_client", ts.callInnerFlowID,
		callsOtherFlowID)
	ts.Equal(http.StatusBadRequest, status,
		"a CIBA flow that calls another authentication flow than the application's is refused: %s", body)
}

// A CIBA flow has to call an authentication flow, and only after the approval link is verified.
func (ts *CIBATestSuite) TestCIBAFlowIsValidated() {
	noCall := []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "verify_invite"},
		{
			"id":        "verify_invite",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "InviteExecutor", "mode": "verify"},
			"onSuccess": "end",
		},
		{"id": "end", "type": "END"},
	}
	_, err := testutils.CreateFlow(testutils.Flow{
		Name: "CIBA Test Flow Without A Call", FlowType: "CIBA", Handle: "ciba_flow_no_call_test", Nodes: noCall,
	})
	ts.Error(err, "a CIBA flow without a Call node is refused")

	callFirst := []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "call_sign_in"},
		{
			"id": "call_sign_in", "type": "CALL", "onSuccess": "verify_invite",
			"flow": map[string]interface{}{"ref": ts.callInnerFlowID},
		},
		{
			"id":        "verify_invite",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "InviteExecutor", "mode": "verify"},
			"onSuccess": "end",
		},
		{"id": "end", "type": "END"},
	}
	_, err = testutils.CreateFlow(testutils.Flow{
		Name: "CIBA Test Flow Calling Early", FlowType: "CIBA", Handle: "ciba_flow_call_early_test", Nodes: callFirst,
	})
	ts.Error(err, "a CIBA flow that calls before the approval link is verified is refused")

	registration := []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "verify_invite"},
		{
			"id":        "verify_invite",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "InviteExecutor", "mode": "verify"},
			"onSuccess": "call_registration",
		},
		{
			"id": "call_registration", "type": "CALL", "onSuccess": "end",
			"flow": map[string]interface{}{"ref": ts.callFlowID},
		},
		{"id": "end", "type": "END"},
	}
	_, err = testutils.CreateFlow(testutils.Flow{
		Name: "CIBA Test Flow Calling A CIBA Flow", FlowType: "CIBA", Handle: "ciba_flow_call_ciba_test",
		Nodes: registration,
	})
	ts.Error(err, "a CIBA flow can only call an authentication flow")
}

// Only a backchannel request starts a CIBA flow, so no other flow can call one.
func (ts *CIBATestSuite) TestAFlowCannotCallACIBAFlow() {
	_, err := testutils.CreateFlow(testutils.Flow{
		Name:     "CIBA Test Authentication Flow Calling A CIBA Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_calls_ciba_flow_test",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "call_ciba"},
			{
				"id": "call_ciba", "type": "CALL", "onSuccess": "auth_assert",
				"flow": map[string]interface{}{"ref": ts.callFlowID},
			},
			{
				"id": "auth_assert", "type": "TASK_EXECUTION", "onSuccess": "end",
				"executor": map[string]interface{}{"name": "AuthAssertExecutor"},
			},
			{"id": "end", "type": "END"},
		},
	})

	ts.Require().Error(err, "a flow that calls a CIBA flow is refused")
	ts.Contains(err.Error(), "only a request starts")
}

// Only a backchannel request starts a CIBA flow. The execute path refuses its type, so a client cannot
// start one by naming it.
func (ts *CIBATestSuite) TestCIBAFlowCannotBeStartedThroughTheExecutePath() {
	status, body, err := testutils.ExecuteFlowRaw(nil, "", map[string]interface{}{
		"applicationId": ts.callAppID,
		"flowType":      "CIBA",
	})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, string(body))
}
