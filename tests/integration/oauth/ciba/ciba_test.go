// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ciba

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// cibaGrantType is the OpenID Connect CIBA grant type identifier (providers.GrantTypeCIBA).
	cibaGrantType = "urn:openid:params:grant-type:ciba"
	// cibaBackchannelEndpoint is the backchannel authentication endpoint
	// (oauth2const.OAuth2BackchannelAuthEndpoint).
	cibaBackchannelEndpoint = "/oauth2/bc-authorize"
	// cibaCallbackEndpoint is the shared flow-callback endpoint.
	cibaCallbackEndpoint = "/oauth2/auth/callback"
	// cibaTokenEndpoint is the token endpoint used for polling.
	cibaTokenEndpoint = "/oauth2/token" // #nosec G101
	// cibaPollIntervalSeconds mirrors oauth2const.CIBADefaultIntervalSeconds (the minimum interval
	// between token polls while a request is pending).
	cibaPollIntervalSeconds = 5
	// cibaMaxExpiresInSeconds mirrors oauth2const.CIBAMaxExpiresInSeconds (the server-side cap a
	// client-requested expiry clamps to).
	cibaMaxExpiresInSeconds = 600
	// cibaMaxBindingMessageLength mirrors ciba.cibaMaxBindingMessageLength (the maximum number of
	// characters allowed in a binding_message).
	cibaMaxBindingMessageLength = 256
	// cibaMockNotificationServerPort is the port for this suite's mock notification server. It must
	// not collide with other integration suites (the SMS auth suite uses 8098).
	cibaMockNotificationServerPort = 8099

	cibaClientID     = "ciba_test_client_123"
	cibaClientSecret = "ciba_test_secret_123"
	cibaTestUsername = "ciba_test_user"
	cibaTestPassword = "cibapass123"

	// cibaSecondClientID/Secret is a second CIBA-enabled application sharing the same auth flow,
	// used to prove an auth_req_id is not transferable across clients.
	cibaSecondClientID     = "ciba_test_client_second"
	cibaSecondClientSecret = "ciba_test_secret_second"
	// cibaNoGrantClientID/Secret is an application without the CIBA grant type.
	cibaNoGrantClientID     = "ciba_test_client_no_grant"
	cibaNoGrantClientSecret = "ciba_test_secret_no_grant"
	// cibaIDHintClientID/Secret is bound to a flow whose identify node resolves directly by user ID,
	// so it can be driven purely via id_token_hint.
	cibaIDHintClientID     = "ciba_test_client_idhint"
	cibaIDHintClientSecret = "ciba_test_secret_idhint"

	// cibaResourceServerIdentifier is the resource server bound in resource-indicator tests.
	cibaResourceServerIdentifier = "https://ciba-test-rs.example.com"
	// cibaMismatchResourceIdentifier is a syntactically valid resource never bound to any request,
	// used to prove polling cannot widen or redirect an existing binding.
	cibaMismatchResourceIdentifier = "https://ciba-mismatch-rs.example.com"
)

type CIBATestSuite struct {
	suite.Suite
	ouID             string
	client           *http.Client
	mockServer       *testutils.MockNotificationServer
	senderID         string
	userTypeID       string
	userID           string
	flowID           string
	appID            string
	issuer           string
	resourceServerID string
	roleID           string
	secondAppID      string
	noGrantAppID     string
	idHintFlowID     string
	idHintAppID      string
}

func TestCIBATestSuite(t *testing.T) {
	suite.Run(t, new(CIBATestSuite))
}

func (ts *CIBATestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	issuer, err := ts.fetchIssuer()
	ts.Require().NoError(err, "Failed to fetch OIDC issuer from discovery endpoint")
	ts.issuer = issuer

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "ciba-test-ou",
		Name:        "CIBA Test OU",
		Description: "Organization unit for CIBA integration tests",
		Parent:      nil,
	})
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	// Mock notification server captures the CIBA notification (which carries the invite link with
	// the executionId). It is a plain HTTP server; the sender below is pointed at it via the API.
	ts.mockServer = testutils.NewMockNotificationServer(cibaMockNotificationServerPort)
	ts.Require().NoError(ts.mockServer.Start(), "Failed to start mock notification server")
	time.Sleep(100 * time.Millisecond)

	// A custom notification sender that POSTs rendered messages to the mock server. This is a DB
	// resource created via the API, so no server restart is required.
	senderID, err := testutils.CreateNotificationSender(testutils.NotificationSender{
		Name:        "CIBA Test Sender",
		Description: "Sender for CIBA integration test",
		Provider:    "custom",
		Properties: []testutils.SenderProperty{
			{Name: "url", Value: ts.mockServer.GetSendSMSURL()},
			{Name: "http_method", Value: "POST"},
			{Name: "content_type", Value: "JSON"},
		},
	})
	ts.Require().NoError(err, "Failed to create notification sender")
	ts.senderID = senderID

	// User type + user. mobile_number is the recipient the SMS executor resolves from the
	// identified user; username/password back the credential confirmation step.
	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: "ciba-test-person",
		OUID: ts.ouID,
		Schema: map[string]interface{}{
			"username":      map[string]interface{}{"type": "string"},
			"password":      map[string]interface{}{"type": "string", "credential": true},
			"email":         map[string]interface{}{"type": "string"},
			"mobile_number": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "Failed to create CIBA test user type")
	ts.userTypeID = userTypeID

	userID, err := testutils.CreateUser(testutils.User{
		OUID: ts.ouID,
		Type: "ciba-test-person",
		Attributes: json.RawMessage(`{
			"username": "` + cibaTestUsername + `",
			"password": "` + cibaTestPassword + `",
			"email": "ciba_test_user@example.com",
			"mobile_number": "+1987654321"
		}`),
	})
	ts.Require().NoError(err, "Failed to create CIBA test user")
	ts.userID = userID

	// CIBA authentication flow. bc-authorize runs this server-side with login_hint. The
	// IdentifyingExecutor resolves the user (login_hint -> username), the InviteExecutor mints a
	// link carrying executionId + auth_req_id, the SMSExecutor delivers it via the mock server, and
	// the flow then pauses at the notification-sent prompt. The resumed flow re-enters via the
	// invite-verify node, which skips challenge validation so it can be resumed cold using only the
	// executionId + inviteToken recovered from the notification. An AuthorizationExecutor node
	// evaluates any requested permission scopes against the user's roles before the assertion is
	// minted; when no permission scopes are requested (the common case in this suite) it is a no-op.
	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "CIBA Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_ciba_test",
		Nodes:    cibaAuthFlowNodes("username", senderID, true),
	})
	ts.Require().NoError(err, "Failed to create CIBA auth flow")
	ts.flowID = flowID

	ts.appID = ts.createCIBATestApplication(flowID)

	// Resource server + role used by the resource-indicator tests. The role grants only "read"; the
	// resource server also exposes "write" so a request that asks for both can prove downscoping
	// drops the permission the user was never granted.
	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "CIBA Test Resource Server",
		Description: "Resource server for CIBA integration tests",
		Identifier:  cibaResourceServerIdentifier,
		OUID:        ts.ouID,
	}, []testutils.Action{
		{Name: "Read", Handle: "read", Description: "Read access"},
		{Name: "Write", Handle: "write", Description: "Write access"},
	})
	ts.Require().NoError(err, "Failed to create CIBA test resource server")
	ts.resourceServerID = rsID

	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        "CIBA Test Reader Role",
		Description: "Grants read-only access on the CIBA test resource server",
		OUID:        ts.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"read"}},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.userID, Type: "user"},
		},
	})
	ts.Require().NoError(err, "Failed to create CIBA test role")
	ts.roleID = roleID

	// A second CIBA-enabled application sharing the main flow, used to prove an auth_req_id is not
	// transferable across clients.
	ts.secondAppID = ts.createCIBAApp("CIBASecondTestApp", cibaSecondClientID, cibaSecondClientSecret,
		[]string{cibaGrantType, "refresh_token"}, flowID)

	// An application without the CIBA grant type, used to prove bc-authorize is rejected up front.
	ts.noGrantAppID = ts.createCIBAApp("CIBANoGrantTestApp", cibaNoGrantClientID, cibaNoGrantClientSecret,
		[]string{"client_credentials"}, flowID)

	// A dedicated flow + application for id_token_hint: the identify node resolves the login_hint
	// value directly against the user ID (rather than the username attribute), because the CIBA
	// service resolves id_token_hint down to the token's raw sub claim before initiating the flow.
	idHintFlowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "CIBA IDToken Hint Test Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_ciba_idhint_test",
		Nodes:    cibaAuthFlowNodes("userID", senderID, false),
	})
	ts.Require().NoError(err, "Failed to create CIBA id_token_hint test flow")
	ts.idHintFlowID = idHintFlowID

	ts.idHintAppID = ts.createCIBAApp("CIBAIDHintTestApp", cibaIDHintClientID, cibaIDHintClientSecret,
		[]string{cibaGrantType, "refresh_token"}, idHintFlowID)
}

// SetupTest drops notifications captured by earlier tests, so that each test recovers the executionId
// of its own backchannel request from the shared mock server.
func (ts *CIBATestSuite) SetupTest() {
	if ts.mockServer != nil {
		ts.mockServer.ClearMessages()
	}
}

func (ts *CIBATestSuite) TearDownSuite() {
	if ts.idHintAppID != "" {
		_ = testutils.DeleteApplication(ts.idHintAppID)
	}
	if ts.idHintFlowID != "" {
		_ = testutils.DeleteFlow(ts.idHintFlowID)
	}
	if ts.noGrantAppID != "" {
		_ = testutils.DeleteApplication(ts.noGrantAppID)
	}
	if ts.secondAppID != "" {
		_ = testutils.DeleteApplication(ts.secondAppID)
	}
	if ts.roleID != "" {
		_ = testutils.DeleteRole(ts.roleID)
	}
	if ts.resourceServerID != "" {
		_ = testutils.DeleteResourceServer(ts.resourceServerID)
	}
	if ts.appID != "" {
		_ = testutils.DeleteApplication(ts.appID)
	}
	if ts.flowID != "" {
		_ = testutils.DeleteFlow(ts.flowID)
	}
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}
	if ts.userTypeID != "" {
		_ = testutils.DeleteUserType(ts.userTypeID)
	}
	if ts.senderID != "" {
		_ = testutils.DeleteNotificationSender(ts.senderID)
	}
	if ts.mockServer != nil {
		_ = ts.mockServer.Stop()
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

// TestCIBAGrantFlow exercises the full Client-Initiated Backchannel Authentication (CIBA) grant
// end to end: it initiates a backchannel request, recovers the server-initiated flow's executionId
// from an out-of-band notification, completes the authentication flow, drives the state machine
// (PENDING -> AUTHENTICATED -> CONSUMED) through the callback and token endpoints, and asserts the
// one-time-use enforcement backed by the runtime store's CompareFieldAndSwap primitive.
func (ts *CIBATestSuite) TestCIBAGrantFlow() {
	mockServer := ts.mockServer

	// Step 1: Backchannel authorization request.
	status, bcResp := ts.cibaBackchannelAuthorize(cibaTestUsername, "openid")
	ts.Require().Equal(http.StatusOK, status, "bc-authorize should succeed")
	ts.Require().NotEmpty(bcResp.AuthReqID, "bc-authorize response should carry auth_req_id")
	ts.Require().Equal(int64(cibaPollIntervalSeconds), bcResp.Interval, "interval should be the default")

	// Step 2: While pending, the token endpoint enforces the polling interval — the first poll is
	// authorization_pending, an immediate re-poll is slow_down. Neither consumes the request.
	pending := ts.cibaPollToken(bcResp.AuthReqID, "")
	ts.Require().Equal(http.StatusBadRequest, pending.statusCode)
	ts.Require().Equal("authorization_pending", pending.errorCode)

	slowDown := ts.cibaPollToken(bcResp.AuthReqID, "")
	ts.Require().Equal(http.StatusBadRequest, slowDown.statusCode)
	ts.Require().Equal("slow_down", slowDown.errorCode)

	// Step 3: Recover the executionId, auth_req_id, and inviteToken from the captured notification.
	var executionID, notifiedAuthReqID, inviteToken string
	ts.Require().Eventually(func() bool {
		msg := mockServer.GetLastMessage()
		if msg == nil {
			return false
		}
		executionID = extractCIBALinkParam(msg.Message, "executionId")
		notifiedAuthReqID = extractCIBALinkParam(msg.Message, "auth_req_id")
		inviteToken = extractCIBALinkParam(msg.Message, "inviteToken")
		return executionID != "" && inviteToken != ""
	}, 5*time.Second, 100*time.Millisecond, "Expected CIBA notification carrying the executionId")
	ts.Require().Equal(bcResp.AuthReqID, notifiedAuthReqID,
		"notification auth_req_id should match the bc-authorize response")

	// Step 4: Resume the paused flow cold via the invite-verify node (submitting the inviteToken,
	// which skips challenge validation), then approve by submitting the user's credentials.
	resumeStep, err := testutils.ExecuteAuthenticationFlow(executionID,
		map[string]string{"inviteToken": inviteToken}, "")
	ts.Require().NoError(err, "should resume the flow with the invite token")
	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": cibaTestUsername,
		"password": cibaTestPassword,
	}, "action_001", resumeStep.ChallengeToken)
	ts.Require().NoError(err, "should complete the authentication flow")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)
	ts.Require().NotEmpty(flowStep.Assertion, "flow completion should yield an assertion")

	// Step 5: Post the assertion to the callback to drive MarkAuthenticated (PENDING -> AUTHENTICATED).
	callbackStatus, _ := ts.cibaPostCallback(bcResp.AuthReqID, flowStep.Assertion)
	ts.Require().Equal(http.StatusOK, callbackStatus, "CIBA callback should accept the assertion")

	// Step 6: Poll the token endpoint to drive MarkConsumed and issue tokens. The pending polls
	// recently stamped LastPolledAt; once AUTHENTICATED the handler skips the interval check, but
	// retry once on slow_down for robustness against timing.
	tokenRes := ts.cibaPollToken(bcResp.AuthReqID, "")
	if tokenRes.statusCode == http.StatusBadRequest && tokenRes.errorCode == "slow_down" {
		time.Sleep(cibaPollIntervalSeconds * time.Second)
		tokenRes = ts.cibaPollToken(bcResp.AuthReqID, "")
	}
	ts.Require().Equal(http.StatusOK, tokenRes.statusCode, "AUTHENTICATED request should issue tokens")
	ts.Require().NotEmpty(tokenRes.accessToken, "response should carry an access_token")

	claims, err := testutils.DecodeJWT(tokenRes.accessToken)
	ts.Require().NoError(err, "issued access token should be a decodable JWT")
	ts.Require().Equal(ts.userID, claims.Sub, "token subject should be the CIBA user")

	// Step 7: A second poll is rejected — the request is CONSUMED (one-time use).
	reuse := ts.cibaPollToken(bcResp.AuthReqID, "")
	ts.Require().Equal(http.StatusBadRequest, reuse.statusCode)
	ts.Require().Equal("invalid_grant", reuse.errorCode, "a consumed request must not issue tokens again")
}

// TestCIBAFlowFailureDeniesRequest verifies that a terminal flow failure is propagated to the polling
// client instead of leaving the request PENDING until it expires. The flow mints a signed error
// assertion, the gate relays it to the callback in the same field a success assertion uses, and the
// request transitions to DENIED so the next token poll returns access_denied.
//
// The trigger is a bogus inviteToken: InviteExecutor's verify mode fails, and verify_invite has no
// onFailure target, so the flow terminates in ERROR.
func (ts *CIBATestSuite) TestCIBAFlowFailureDeniesRequest() {
	// Step 1: Backchannel authorization request.
	status, bcResp := ts.cibaBackchannelAuthorize(cibaTestUsername, "openid")
	ts.Require().Equal(http.StatusOK, status, "bc-authorize should succeed")
	ts.Require().NotEmpty(bcResp.AuthReqID, "bc-authorize response should carry auth_req_id")

	// Step 2: Baseline — before the failure the client is told nothing but "keep waiting".
	pending := ts.cibaPollToken(bcResp.AuthReqID, "")
	ts.Require().Equal(http.StatusBadRequest, pending.statusCode)
	ts.Require().Equal("authorization_pending", pending.errorCode)

	// Step 3: Recover the executionId from the notification captured for this request.
	var executionID string
	ts.Require().Eventually(func() bool {
		msg := ts.mockServer.GetLastMessage()
		if msg == nil {
			return false
		}
		if extractCIBALinkParam(msg.Message, "auth_req_id") != bcResp.AuthReqID {
			return false
		}
		executionID = extractCIBALinkParam(msg.Message, "executionId")
		return executionID != ""
	}, 5*time.Second, 100*time.Millisecond, "Expected CIBA notification carrying the executionId")

	// Step 4: Fail the flow at the invite-verify node.
	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID,
		map[string]string{"inviteToken": "bogus-invite-token"}, "")
	ts.Require().NoError(err, "Flow step should be returned for an invalid invite token")
	ts.Require().Equal("ERROR", flowStep.FlowStatus, "Flow should terminate in ERROR")
	ts.Require().NotEmpty(flowStep.ErrorAssertion,
		"A CIBA-initiated flow failure must mint an error assertion")
	ts.Require().Empty(flowStep.Assertion, "A failed flow must not produce an authentication assertion")

	// Step 5: Relay the error assertion. The callback op itself succeeds; the outcome lives in the
	// request state, which is why this is a 200 and not an error status.
	callbackStatus, _ := ts.cibaPostCallback(bcResp.AuthReqID, flowStep.ErrorAssertion)
	ts.Require().Equal(http.StatusOK, callbackStatus, "CIBA callback should accept the error assertion")

	// Step 6: The polling client now learns the outcome instead of hanging on authorization_pending.
	denied := ts.cibaPollToken(bcResp.AuthReqID, "")
	if denied.statusCode == http.StatusBadRequest && denied.errorCode == "slow_down" {
		time.Sleep(cibaPollIntervalSeconds * time.Second)
		denied = ts.cibaPollToken(bcResp.AuthReqID, "")
	}
	ts.Require().Equal(http.StatusBadRequest, denied.statusCode)
	ts.Require().Equal("access_denied", denied.errorCode,
		"An end-user flow failure must surface as access_denied, not authorization_pending")
	ts.Require().Empty(denied.accessToken, "A denied request must not issue tokens")
}

// TestCIBAExpiredRequestRejectsPolling verifies that a request whose requested_expiry has elapsed is
// rejected by the token endpoint with expired_token (repeatedly, once the request is marked EXPIRED),
// and that resolveExpiresIn only clamps the upper bound: a below-max value like 1 is honored verbatim,
// while an above-max value like 9999 clamps down to the server maximum.
func (ts *CIBATestSuite) TestCIBAExpiredRequestRejectsPolling() {
	form := url.Values{}
	form.Set("login_hint", cibaTestUsername)
	form.Set("scope", "openid")
	form.Set("requested_expiry", "1")
	status, bcResp := ts.cibaBackchannelAuthorizeForm(form, cibaClientID, cibaClientSecret)
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().Equal(int64(1), bcResp.ExpiresIn, "a requested_expiry below the max must be honored")

	time.Sleep(2 * time.Second)

	expired := ts.cibaPollToken(bcResp.AuthReqID, "")
	ts.Require().Equal(http.StatusBadRequest, expired.statusCode)
	ts.Require().Equal("expired_token", expired.errorCode)

	expiredAgain := ts.cibaPollToken(bcResp.AuthReqID, "")
	ts.Require().Equal(http.StatusBadRequest, expiredAgain.statusCode)
	ts.Require().Equal("expired_token", expiredAgain.errorCode,
		"a request already marked EXPIRED must keep returning expired_token")

	clampForm := url.Values{}
	clampForm.Set("login_hint", cibaTestUsername)
	clampForm.Set("scope", "openid")
	clampForm.Set("requested_expiry", "9999")
	clampStatus, clampResp := ts.cibaBackchannelAuthorizeForm(clampForm, cibaClientID, cibaClientSecret)
	ts.Require().Equal(http.StatusOK, clampStatus)
	ts.Require().Equal(int64(cibaMaxExpiresInSeconds), clampResp.ExpiresIn,
		"a requested_expiry above the server maximum must clamp to the maximum")
}

// TestCIBAUnknownLoginHintReturnsUnknownUserID verifies that a login_hint matching no user maps to
// unknown_user_id (CIBA Core 1.0 ​§7.3), not server_error. mapFlowErrorToCIBAError switches on the
// literal flow-engine failure string "User not found"; the unit tests mock the flow entirely, so this
// coupling between the flow engine's wording and the CIBA error mapping is otherwise unverified.
func (ts *CIBATestSuite) TestCIBAUnknownLoginHintReturnsUnknownUserID() {
	status, bcResp := ts.cibaBackchannelAuthorize("no_such_ciba_user_xyz", "openid")
	ts.Require().Equal(http.StatusBadRequest, status)
	ts.Require().Equal("unknown_user_id", bcResp.ErrorCode,
		"an unresolvable login_hint must map to unknown_user_id, not server_error")
}

// TestCIBAIDTokenHintResolvesUser mints a real ID token via one full CIBA cycle, then uses it as
// id_token_hint (with no login_hint) on a second, independent request against an application whose
// flow resolves the identify step directly by user ID. It verifies the same user is resolved end to
// end, and that a tampered id_token_hint signature is rejected with invalid_request.
func (ts *CIBATestSuite) TestCIBAIDTokenHintResolvesUser() {
	// Mint a real ID token via a normal login_hint-based CIBA cycle.
	mintStatus, mintResp := ts.cibaBackchannelAuthorize(cibaTestUsername, "openid")
	ts.Require().Equal(http.StatusOK, mintStatus)
	ts.completeCIBAFlow(mintResp.AuthReqID)

	mintedTokens := ts.cibaPollToken(mintResp.AuthReqID, "")
	if mintedTokens.statusCode == http.StatusBadRequest && mintedTokens.errorCode == "slow_down" {
		time.Sleep(cibaPollIntervalSeconds * time.Second)
		mintedTokens = ts.cibaPollToken(mintResp.AuthReqID, "")
	}
	ts.Require().Equal(http.StatusOK, mintedTokens.statusCode,
		"should mint a real ID token to reuse as id_token_hint")
	ts.Require().NotEmpty(mintedTokens.idToken)

	// Use the minted ID token as id_token_hint, with no login_hint, on the id_token_hint test app.
	hintForm := url.Values{}
	hintForm.Set("id_token_hint", mintedTokens.idToken)
	hintForm.Set("scope", "openid")
	hintStatus, hintResp := ts.cibaBackchannelAuthorizeForm(hintForm, cibaIDHintClientID, cibaIDHintClientSecret)
	ts.Require().Equal(http.StatusOK, hintStatus, "id_token_hint alone should resolve the same user")
	ts.Require().NotEmpty(hintResp.AuthReqID)

	ts.completeCIBAFlow(hintResp.AuthReqID)

	finalTokens := ts.cibaPollTokenAs(hintResp.AuthReqID, "", cibaIDHintClientID, cibaIDHintClientSecret)
	if finalTokens.statusCode == http.StatusBadRequest && finalTokens.errorCode == "slow_down" {
		time.Sleep(cibaPollIntervalSeconds * time.Second)
		finalTokens = ts.cibaPollTokenAs(hintResp.AuthReqID, "", cibaIDHintClientID, cibaIDHintClientSecret)
	}
	ts.Require().Equal(http.StatusOK, finalTokens.statusCode)
	ts.Require().NotEmpty(finalTokens.accessToken)

	claims, err := testutils.DecodeJWT(finalTokens.accessToken)
	ts.Require().NoError(err)
	ts.Require().Equal(ts.userID, claims.Sub, "id_token_hint must resolve to the same user as login_hint")

	// A tampered id_token_hint signature must be rejected before any flow is initiated.
	tamperedForm := url.Values{}
	tamperedForm.Set("id_token_hint", tamperJWTSignature(mintedTokens.idToken))
	tamperedForm.Set("scope", "openid")
	tamperedStatus, tamperedResp := ts.cibaBackchannelAuthorizeForm(
		tamperedForm, cibaIDHintClientID, cibaIDHintClientSecret)
	ts.Require().Equal(http.StatusBadRequest, tamperedStatus)
	ts.Require().Equal("invalid_request", tamperedResp.ErrorCode,
		"a tampered id_token_hint signature must be rejected")
}

// TestCIBAResourceBoundTokenAudienceAndScopes verifies that a resource-bound CIBA request issues an
// access token whose audience is the resource server identifier, and whose scopes are downscoped to
// the permissions the user actually holds: "write" is requested but never granted, so it must not
// appear in the issued token even though it is a valid action on the resource server.
func (ts *CIBATestSuite) TestCIBAResourceBoundTokenAudienceAndScopes() {
	form := url.Values{}
	form.Set("login_hint", cibaTestUsername)
	form.Set("scope", "openid read write")
	form.Set("resource", cibaResourceServerIdentifier)
	status, bcResp := ts.cibaBackchannelAuthorizeForm(form, cibaClientID, cibaClientSecret)
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().NotEmpty(bcResp.AuthReqID)

	ts.completeCIBAFlow(bcResp.AuthReqID)

	tokenRes := ts.cibaPollToken(bcResp.AuthReqID, cibaResourceServerIdentifier)
	if tokenRes.statusCode == http.StatusBadRequest && tokenRes.errorCode == "slow_down" {
		time.Sleep(cibaPollIntervalSeconds * time.Second)
		tokenRes = ts.cibaPollToken(bcResp.AuthReqID, cibaResourceServerIdentifier)
	}
	ts.Require().Equal(http.StatusOK, tokenRes.statusCode, "resource-bound request should issue tokens")
	ts.Require().NotEmpty(tokenRes.accessToken)

	claims, err := testutils.DecodeJWT(tokenRes.accessToken)
	ts.Require().NoError(err)
	ts.Require().Equal(cibaResourceServerIdentifier, claims.Aud,
		"the access token audience must be the bound resource server")

	scopes := strings.Fields(tokenRes.scope)
	ts.Require().Contains(scopes, "read", "the granted permission must be present")
	ts.Require().NotContains(scopes, "write",
		"a requested but never-granted permission must be dropped by downscoping")
}

// TestCIBAPollingResourceMismatchRejected verifies enforceCIBAPollingResource: polling with a resource
// different from the one bound at bc-authorize time is rejected, and so is polling an OIDC-only
// (unbound) request with any resource — neither can widen or redirect the original binding.
func (ts *CIBATestSuite) TestCIBAPollingResourceMismatchRejected() {
	boundForm := url.Values{}
	boundForm.Set("login_hint", cibaTestUsername)
	boundForm.Set("scope", "openid read")
	boundForm.Set("resource", cibaResourceServerIdentifier)
	boundStatus, boundResp := ts.cibaBackchannelAuthorizeForm(boundForm, cibaClientID, cibaClientSecret)
	ts.Require().Equal(http.StatusOK, boundStatus)

	mismatch := ts.cibaPollToken(boundResp.AuthReqID, cibaMismatchResourceIdentifier)
	ts.Require().Equal(http.StatusBadRequest, mismatch.statusCode)
	ts.Require().Equal("invalid_target", mismatch.errorCode,
		"polling with a resource different from the bound one must be rejected")

	unboundStatus, unboundResp := ts.cibaBackchannelAuthorize(cibaTestUsername, "openid")
	ts.Require().Equal(http.StatusOK, unboundStatus)

	unboundMismatch := ts.cibaPollToken(unboundResp.AuthReqID, cibaResourceServerIdentifier)
	ts.Require().Equal(http.StatusBadRequest, unboundMismatch.statusCode)
	ts.Require().Equal("invalid_target", unboundMismatch.errorCode,
		"polling an OIDC-only unbound request with a resource must be rejected")
}

// TestCIBAAuthReqIDNotTransferableAcrossClients verifies that an auth_req_id is scoped to the client
// that created it: a second CIBA-enabled application cannot poll it (invalid_grant, even after the
// request is fully AUTHENTICATED), and an application without the CIBA grant type is rejected before
// any flow is initiated (unauthorized_client).
func (ts *CIBATestSuite) TestCIBAAuthReqIDNotTransferableAcrossClients() {
	status, bcResp := ts.cibaBackchannelAuthorize(cibaTestUsername, "openid")
	ts.Require().Equal(http.StatusOK, status)

	ts.completeCIBAFlow(bcResp.AuthReqID)

	crossPoll := ts.cibaPollTokenAs(bcResp.AuthReqID, "", cibaSecondClientID, cibaSecondClientSecret)
	ts.Require().Equal(http.StatusBadRequest, crossPoll.statusCode)
	ts.Require().Equal("invalid_grant", crossPoll.errorCode,
		"a second CIBA-enabled client must not be able to poll another client's auth_req_id")

	noGrantForm := url.Values{}
	noGrantForm.Set("login_hint", cibaTestUsername)
	noGrantForm.Set("scope", "openid")
	noGrantStatus, noGrantResp := ts.cibaBackchannelAuthorizeForm(
		noGrantForm, cibaNoGrantClientID, cibaNoGrantClientSecret)
	ts.Require().Equal(http.StatusBadRequest, noGrantStatus)
	ts.Require().Equal("unauthorized_client", noGrantResp.ErrorCode,
		"a client without the CIBA grant type must be rejected before any flow is initiated")
}

// TestCIBABindingMessageDeliveredToUser verifies that a client-supplied binding_message reaches the
// user's notification verbatim, and that an omitted one falls back to defaultBindingMessage's
// generated, request-specific "Code: XXXX-XXXX" text.
func (ts *CIBATestSuite) TestCIBABindingMessageDeliveredToUser() {
	customMessage := "Approve sign-in to Acme Console"
	form := url.Values{}
	form.Set("login_hint", cibaTestUsername)
	form.Set("scope", "openid")
	form.Set("binding_message", customMessage)
	status, bcResp := ts.cibaBackchannelAuthorizeForm(form, cibaClientID, cibaClientSecret)
	ts.Require().Equal(http.StatusOK, status)

	var messageA string
	ts.Require().Eventually(func() bool {
		msg := ts.mockServer.GetLastMessage()
		if msg == nil || extractCIBALinkParam(msg.Message, "auth_req_id") != bcResp.AuthReqID {
			return false
		}
		messageA = msg.Message
		return true
	}, 5*time.Second, 100*time.Millisecond, "Expected CIBA notification for the custom binding_message request")
	ts.Require().Contains(messageA, customMessage,
		"a custom binding_message must be delivered verbatim in the notification")

	defaultStatus, defaultResp := ts.cibaBackchannelAuthorize(cibaTestUsername, "openid")
	ts.Require().Equal(http.StatusOK, defaultStatus)

	var messageB string
	ts.Require().Eventually(func() bool {
		msg := ts.mockServer.GetLastMessage()
		if msg == nil || extractCIBALinkParam(msg.Message, "auth_req_id") != defaultResp.AuthReqID {
			return false
		}
		messageB = msg.Message
		return true
	}, 5*time.Second, 100*time.Millisecond, "Expected CIBA notification for the default binding_message request")
	ts.Require().Regexp(regexp.MustCompile(`Code: [0-9A-F]{4}-[0-9A-F]{4}`), messageB,
		"an omitted binding_message must fall back to the generated default with a request-specific code")
}

// TestCIBACallbackRejectsCrossBoundAssertion is a security regression test for the attack described at
// service.go's loadPendingRequestForCallback / handleSuccessCallback: a valid, correctly signed
// assertion completed for one CIBA request must not authorize a different, concurrently pending
// request for the same user. It also confirms the targeted request is left untouched by the rejected
// attempt: still pending, and still completable normally afterwards.
func (ts *CIBATestSuite) TestCIBACallbackRejectsCrossBoundAssertion() {
	statusA, bcRespA := ts.cibaBackchannelAuthorize(cibaTestUsername, "openid")
	ts.Require().Equal(http.StatusOK, statusA)
	// Drain A's notification immediately: GetLastMessage destructively pops, so this must happen
	// before B's bc-authorize call adds a second notification to the queue.
	executionIDA, inviteTokenA := ts.recoverCIBAInvite(bcRespA.AuthReqID)

	statusB, bcRespB := ts.cibaBackchannelAuthorize(cibaTestUsername, "openid")
	ts.Require().Equal(http.StatusOK, statusB)
	ts.Require().NotEqual(bcRespA.AuthReqID, bcRespB.AuthReqID)
	executionIDB, inviteTokenB := ts.recoverCIBAInvite(bcRespB.AuthReqID)

	assertionA := ts.driveCIBAFlow(executionIDA, inviteTokenA, bcRespA.AuthReqID)

	crossStatus, crossErr := ts.cibaPostCallback(bcRespB.AuthReqID, assertionA)
	ts.Require().Equal(http.StatusBadRequest, crossStatus)
	ts.Require().Equal("access_denied", crossErr,
		"an assertion minted for a different CIBA request must be rejected")

	// B must remain untouched by the rejected cross-bound callback.
	pendingB := ts.cibaPollToken(bcRespB.AuthReqID, "")
	ts.Require().Equal(http.StatusBadRequest, pendingB.statusCode)
	ts.Require().Equal("authorization_pending", pendingB.errorCode,
		"request B must still be pending after the cross-bound assertion was rejected")

	// B is still completable normally afterwards.
	ts.driveCIBAFlow(executionIDB, inviteTokenB, bcRespB.AuthReqID)
	tokenB := ts.cibaPollToken(bcRespB.AuthReqID, "")
	if tokenB.statusCode == http.StatusBadRequest && tokenB.errorCode == "slow_down" {
		time.Sleep(cibaPollIntervalSeconds * time.Second)
		tokenB = ts.cibaPollToken(bcRespB.AuthReqID, "")
	}
	ts.Require().Equal(http.StatusOK, tokenB.statusCode, "request B should still be completable normally")
	ts.Require().NotEmpty(tokenB.accessToken)
}

// TestCIBACallbackNegatives table-drives the guards in loadPendingRequestForCallback (and the callback
// dispatcher's own authId/assertion presence checks ahead of it).
func (ts *CIBATestSuite) TestCIBACallbackNegatives() {
	freshAuthReqID := func() string {
		status, bcResp := ts.cibaBackchannelAuthorize(cibaTestUsername, "openid")
		ts.Require().Equal(http.StatusOK, status)
		return bcResp.AuthReqID
	}

	tests := []struct {
		name          string
		setup         func() (authReqID, assertion string)
		wantErrorCode string
	}{
		{
			name: "tampered assertion",
			setup: func() (string, string) {
				return freshAuthReqID(), "this-is-not-a-valid-jwt-assertion"
			},
			wantErrorCode: "invalid_request",
		},
		{
			name: "empty assertion",
			setup: func() (string, string) {
				return freshAuthReqID(), ""
			},
			wantErrorCode: "invalid_request",
		},
		{
			name: "empty auth_req_id",
			setup: func() (string, string) {
				return "", "some-assertion"
			},
			wantErrorCode: "invalid_request",
		},
		{
			name: "unknown auth_req_id",
			setup: func() (string, string) {
				return "00000000-0000-0000-0000-000000000000", "some-assertion"
			},
			wantErrorCode: "invalid_request",
		},
		{
			name: "already authenticated",
			setup: func() (string, string) {
				authReqID := freshAuthReqID()
				assertion := ts.completeCIBAFlow(authReqID)
				return authReqID, assertion
			},
			wantErrorCode: "invalid_request",
		},
		{
			name: "expired request",
			setup: func() (string, string) {
				form := url.Values{}
				form.Set("login_hint", cibaTestUsername)
				form.Set("scope", "openid")
				form.Set("requested_expiry", "1")
				status, bcResp := ts.cibaBackchannelAuthorizeForm(form, cibaClientID, cibaClientSecret)
				ts.Require().Equal(http.StatusOK, status)
				time.Sleep(2 * time.Second)
				return bcResp.AuthReqID, "some-assertion"
			},
			wantErrorCode: "expired_token",
		},
	}

	for _, tc := range tests {
		ts.Run(tc.name, func() {
			authReqID, assertion := tc.setup()
			status, errorCode := ts.cibaPostCallback(authReqID, assertion)
			ts.Require().Equal(http.StatusBadRequest, status)
			ts.Require().Equal(tc.wantErrorCode, errorCode)
		})
	}
}

// TestCIBABackchannelValidationNegatives table-drives the bc-authorize request-validation guards that
// reject before any flow is initiated, so none of these rows produce a notification.
func (ts *CIBATestSuite) TestCIBABackchannelValidationNegatives() {
	tests := []struct {
		name          string
		form          url.Values
		wantErrorCode string
	}{
		{
			name:          "no hint",
			form:          url.Values{"scope": {"openid"}},
			wantErrorCode: "invalid_request",
		},
		{
			name: "login_hint and id_token_hint both provided",
			form: url.Values{
				"login_hint":    {cibaTestUsername},
				"id_token_hint": {"a.b.c"},
				"scope":         {"openid"},
			},
			wantErrorCode: "invalid_request",
		},
		{
			name: "login_hint_token unsupported",
			form: url.Values{
				"login_hint_token": {"some-token"},
				"scope":            {"openid"},
			},
			wantErrorCode: "invalid_request",
		},
		{
			name:          "no scope",
			form:          url.Values{"login_hint": {cibaTestUsername}},
			wantErrorCode: "invalid_request",
		},
		{
			name: "scope without openid",
			form: url.Values{
				"login_hint": {cibaTestUsername},
				"scope":      {"profile"},
			},
			wantErrorCode: "invalid_scope",
		},
		{
			name: "binding_message over the length cap",
			form: url.Values{
				"login_hint":      {cibaTestUsername},
				"scope":           {"openid"},
				"binding_message": {strings.Repeat("a", cibaMaxBindingMessageLength+1)},
			},
			wantErrorCode: "invalid_binding_message",
		},
		{
			name: "binding_message with a non-printable rune",
			form: url.Values{
				"login_hint":      {cibaTestUsername},
				"scope":           {"openid"},
				"binding_message": {"helloworld"},
			},
			wantErrorCode: "invalid_binding_message",
		},
		{
			name: "id_token_hint not a JWT",
			form: url.Values{
				"id_token_hint": {"not-a-jwt-at-all"},
				"scope":         {"openid"},
			},
			wantErrorCode: "invalid_request",
		},
		{
			name: "id_token_hint foreign issuer",
			form: url.Values{
				"id_token_hint": {buildFakeIDTokenHint(map[string]interface{}{
					"iss": "https://foreign-issuer.example.com",
					"sub": "some-subject",
					"exp": time.Now().Add(time.Hour).Unix(),
				})},
				"scope": {"openid"},
			},
			wantErrorCode: "invalid_request",
		},
		{
			name: "id_token_hint missing sub",
			form: url.Values{
				"id_token_hint": {buildFakeIDTokenHint(map[string]interface{}{
					"iss": ts.issuer,
					"exp": time.Now().Add(time.Hour).Unix(),
				})},
				"scope": {"openid"},
			},
			wantErrorCode: "invalid_request",
		},
		{
			name: "id_token_hint missing exp",
			form: url.Values{
				"id_token_hint": {buildFakeIDTokenHint(map[string]interface{}{
					"iss": ts.issuer,
					"sub": "some-subject",
				})},
				"scope": {"openid"},
			},
			wantErrorCode: "invalid_request",
		},
		{
			name: "two resource parameters",
			form: url.Values{
				"login_hint": {cibaTestUsername},
				"scope":      {"openid"},
				"resource":   {cibaResourceServerIdentifier, cibaMismatchResourceIdentifier},
			},
			wantErrorCode: "invalid_target",
		},
		{
			name: "unknown resource",
			form: url.Values{
				"login_hint": {cibaTestUsername},
				"scope":      {"openid"},
				"resource":   {"https://unregistered-rs.example.com"},
			},
			wantErrorCode: "invalid_target",
		},
	}

	for _, tc := range tests {
		ts.Run(tc.name, func() {
			status, resp := ts.cibaBackchannelAuthorizeForm(tc.form, cibaClientID, cibaClientSecret)
			ts.Require().Equal(http.StatusBadRequest, status)
			ts.Require().Equal(tc.wantErrorCode, resp.ErrorCode)
		})
	}
}

// fetchIssuer reads the configured OIDC issuer from the discovery endpoint.
func (ts *CIBATestSuite) fetchIssuer() (string, error) {
	resp, err := ts.client.Get(testutils.TestServerURL + "/.well-known/openid-configuration")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var meta struct {
		Issuer string `json:"issuer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return "", err
	}
	return meta.Issuer, nil
}

// createCIBATestApplication creates the primary OAuth application that allows the CIBA grant and is
// bound to the given authentication flow, returning its application ID.
func (ts *CIBATestSuite) createCIBATestApplication(authFlowID string) string {
	return ts.createCIBAApp("CIBATestApp", cibaClientID, cibaClientSecret,
		[]string{cibaGrantType, "refresh_token"}, authFlowID)
}

// createCIBAApp creates an OAuth application with the given client credentials, grant types, and
// bound authentication flow, returning its application ID.
func (ts *CIBATestSuite) createCIBAApp(
	name, clientID, clientSecret string, grantTypes []string, authFlowID string,
) string {
	app := map[string]interface{}{
		"name":                      name,
		"description":               "Application for CIBA integration test",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"authFlowId":                authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"ciba-test-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"clientSecret":            clientSecret,
					"redirectUris":            []string{"https://localhost:3000"},
					"grantTypes":              grantTypes,
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Failed to create CIBA application %q. Status: %d, Response: %s",
			name, resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&respData))
	return respData["id"].(string)
}

// cibaAuthFlowNodes builds the CIBA authentication flow used by this suite. loginHintAttribute
// controls how the identify node resolves the login_hint value: "username" for ordinary login_hint
// requests, or "userID" so an id_token_hint-derived subject (the raw user ID) resolves directly via
// entity lookup. When withAuthzCheck is true, an AuthorizationExecutor node runs before the assertion
// is minted, evaluating any requested permission scopes against the user's roles; it is a no-op when
// no permission scopes are requested.
func cibaAuthFlowNodes(loginHintAttribute, senderID string, withAuthzCheck bool) []map[string]interface{} {
	credentialsAuthSuccess := "auth_assert"
	if withAuthzCheck {
		credentialsAuthSuccess = "authorization_check"
	}

	nodes := []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "identify_user",
		},
		{
			"id":   "identify_user",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "IdentifyingExecutor",
				"mode": "identify",
			},
			"properties": map[string]interface{}{
				"loginHintAttribute": loginHintAttribute,
			},
			"onSuccess": "generate_invite",
		},
		{
			"id":   "generate_invite",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "InviteExecutor",
				"mode": "generate",
			},
			"onSuccess": "send_ciba_notification",
		},
		{
			"id":   "send_ciba_notification",
			"type": "TASK_EXECUTION",
			"properties": map[string]interface{}{
				"senderId":    senderID,
				"smsTemplate": "CIBA_NOTIFICATION",
			},
			"executor": map[string]interface{}{
				"name": "SMSExecutor",
			},
			"onSuccess": "notification_sent",
		},
		{
			// The server-initiated segment pauses here after the notification is sent; the
			// resumed flow re-enters via the invite-verify node below, which skips challenge
			// validation so it can be resumed cold using only the executionId + inviteToken.
			"id":   "notification_sent",
			"type": "PROMPT",
			"next": "verify_invite",
		},
		{
			"id":   "verify_invite",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "InviteExecutor",
				"mode": "verify",
				"inputs": []map[string]interface{}{
					{
						"ref":        "input_invite_token",
						"identifier": "inviteToken",
						"type":       "HIDDEN",
						"required":   true,
					},
				},
			},
			"onSuccess": "prompt_credentials",
		},
		{
			"id":   "prompt_credentials",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_001",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_002",
							"identifier": "password",
							"type":       "PASSWORD_INPUT",
							"required":   true,
						},
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
					{
						"ref":        "input_001",
						"identifier": "username",
						"type":       "TEXT_INPUT",
						"required":   true,
					},
					{
						"ref":        "input_002",
						"identifier": "password",
						"type":       "PASSWORD_INPUT",
						"required":   true,
					},
				},
			},
			"onSuccess": credentialsAuthSuccess,
		},
	}

	if withAuthzCheck {
		nodes = append(nodes, map[string]interface{}{
			"id":   "authorization_check",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "AuthorizationExecutor",
			},
			"onSuccess": "auth_assert",
		})
	}

	nodes = append(nodes,
		map[string]interface{}{
			"id":   "auth_assert",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "AuthAssertExecutor",
			},
			"onSuccess": "end",
		},
		map[string]interface{}{
			"id":   "end",
			"type": "END",
		},
	)

	return nodes
}

// cibaBackchannelResponse is the JSON body of a bc-authorize response, success or error.
type cibaBackchannelResponse struct {
	AuthReqID string `json:"auth_req_id"`
	ExpiresIn int64  `json:"expires_in"`
	Interval  int64  `json:"interval"`
	ErrorCode string `json:"error"`
}

// cibaBackchannelAuthorize submits a POST /oauth2/bc-authorize request with client_secret_basic
// authentication for the primary CIBA test client and returns the HTTP status and parsed response.
func (ts *CIBATestSuite) cibaBackchannelAuthorize(loginHint, scope string) (int, cibaBackchannelResponse) {
	form := url.Values{}
	form.Set("login_hint", loginHint)
	form.Set("scope", scope)
	return ts.cibaBackchannelAuthorizeForm(form, cibaClientID, cibaClientSecret)
}

// cibaBackchannelAuthorizeForm submits a POST /oauth2/bc-authorize request with the given form body
// and client_secret_basic authentication, returning the HTTP status and parsed response.
func (ts *CIBATestSuite) cibaBackchannelAuthorizeForm(
	form url.Values, clientID, clientSecret string,
) (int, cibaBackchannelResponse) {
	req, err := http.NewRequest("POST", testutils.TestServerURL+cibaBackchannelEndpoint,
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var body cibaBackchannelResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

// cibaPostCallback posts the completed flow assertion to the CIBA callback and returns the HTTP
// status and, on an error response, the error code.
func (ts *CIBATestSuite) cibaPostCallback(authID, assertion string) (int, string) {
	payload := map[string]string{
		"authId":    authID,
		"assertion": assertion,
		"type":      cibaGrantType,
	}
	jsonData, err := json.Marshal(payload)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+cibaCallbackEndpoint, bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var raw map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&raw)
	errorCode, _ := raw["error"].(string)
	return resp.StatusCode, errorCode
}

// cibaTokenResult captures the outcome of a CIBA token poll.
type cibaTokenResult struct {
	statusCode  int
	accessToken string
	idToken     string
	scope       string
	errorCode   string
}

// cibaPollToken polls POST /oauth2/token with the CIBA grant, using the primary CIBA test client and
// optionally scoping the poll to a resource parameter, and returns the parsed outcome.
func (ts *CIBATestSuite) cibaPollToken(authReqID, resource string) cibaTokenResult {
	return ts.cibaPollTokenAs(authReqID, resource, cibaClientID, cibaClientSecret)
}

// cibaPollTokenAs polls POST /oauth2/token with the CIBA grant as an arbitrary client, optionally
// scoping the poll to a resource parameter, and returns the parsed outcome.
func (ts *CIBATestSuite) cibaPollTokenAs(authReqID, resource, clientID, clientSecret string) cibaTokenResult {
	form := url.Values{}
	form.Set("grant_type", cibaGrantType)
	form.Set("auth_req_id", authReqID)
	if resource != "" {
		form.Set("resource", resource)
	}

	req, err := http.NewRequest("POST", testutils.TestServerURL+cibaTokenEndpoint,
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	var raw map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&raw)

	res := cibaTokenResult{statusCode: resp.StatusCode}
	if v, ok := raw["access_token"].(string); ok {
		res.accessToken = v
	}
	if v, ok := raw["id_token"].(string); ok {
		res.idToken = v
	}
	if v, ok := raw["scope"].(string); ok {
		res.scope = v
	}
	if v, ok := raw["error"].(string); ok {
		res.errorCode = v
	}
	return res
}

// recoverCIBAInvite recovers the executionId and inviteToken of the server-initiated flow for
// authReqID from the captured notification. MockNotificationServer.GetLastMessage destructively
// pops, so when more than one CIBA request is in flight this must be called immediately after the
// corresponding bc-authorize call, before a concurrent request's notification arrives and gets
// popped (and discarded) by a mismatched poll here.
func (ts *CIBATestSuite) recoverCIBAInvite(authReqID string) (executionID, inviteToken string) {
	ts.Require().Eventually(func() bool {
		msg := ts.mockServer.GetLastMessage()
		if msg == nil || extractCIBALinkParam(msg.Message, "auth_req_id") != authReqID {
			return false
		}
		executionID = extractCIBALinkParam(msg.Message, "executionId")
		inviteToken = extractCIBALinkParam(msg.Message, "inviteToken")
		return executionID != "" && inviteToken != ""
	}, 5*time.Second, 100*time.Millisecond,
		"Expected CIBA notification carrying the executionId for auth_req_id "+authReqID)
	return executionID, inviteToken
}

// driveCIBAFlow resumes the paused flow identified by executionID via the invite token, submits the
// test user's credentials, posts the resulting assertion to the CIBA callback for authReqID
// (asserting it is accepted), and returns the assertion so the caller can reuse it (for example
// against a different auth_req_id, to test the cross-bound-assertion rejection).
func (ts *CIBATestSuite) driveCIBAFlow(executionID, inviteToken, authReqID string) string {
	resumeStep, err := testutils.ExecuteAuthenticationFlow(executionID,
		map[string]string{"inviteToken": inviteToken}, "")
	ts.Require().NoError(err, "should resume the flow with the invite token")

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": cibaTestUsername,
		"password": cibaTestPassword,
	}, "action_001", resumeStep.ChallengeToken)
	ts.Require().NoError(err, "should complete the authentication flow")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)
	ts.Require().NotEmpty(flowStep.Assertion, "flow completion should yield an assertion")

	status, errorCode := ts.cibaPostCallback(authReqID, flowStep.Assertion)
	ts.Require().Equal(http.StatusOK, status, "CIBA callback should accept the assertion: %s", errorCode)

	return flowStep.Assertion
}

// completeCIBAFlow recovers the executionId/inviteToken of the server-initiated flow for authReqID
// from the captured notification and drives it to completion. Only safe when at most one CIBA
// request is in flight; see recoverCIBAInvite for the concurrent case.
func (ts *CIBATestSuite) completeCIBAFlow(authReqID string) string {
	executionID, inviteToken := ts.recoverCIBAInvite(authReqID)
	return ts.driveCIBAFlow(executionID, inviteToken, authReqID)
}

// extractCIBALinkParam pulls a query parameter value out of the invite link embedded in a captured
// notification body. The body is free text, so the parameter is matched directly.
func extractCIBALinkParam(text, param string) string {
	re := regexp.MustCompile(regexp.QuoteMeta(param) + `=([^&"\s\\]+)`)
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// buildFakeIDTokenHint builds a well-formed but unsigned JWT carrying the given claims. It is used to
// exercise the id_token_hint validation guards that run before signature verification (issuer, sub
// presence); guards that only run after signature verification (staleness) are unreachable this way
// and are covered by an equivalent invalid_request outcome from the failed signature check instead.
func buildFakeIDTokenHint(claims map[string]interface{}) string {
	header := map[string]interface{}{"alg": "RS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(claims)
	return base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON) + ".fakesignature"
}

// tamperJWTSignature flips a bit in the first byte of a real JWT's decoded signature, invalidating
// it while keeping the token well-formed (three base64url segments). The mutation must operate on
// the decoded bytes rather than the base64url characters: a character-level edit can land on the
// trailing padding bits of the encoding (e.g. the last character of a 64- or 256-byte signature
// carries only 2 significant bits, with the rest unused padding), in which case the "tampered"
// token decodes to byte-identical signature bytes, still verifies successfully, and the test
// flakes. Flipping a bit in the first byte guarantees the decoded signature actually changes.
func tamperJWTSignature(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(parts[2]) == 0 {
		return token
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(raw) == 0 {
		return token
	}
	raw[0] ^= 0x01
	parts[2] = base64.RawURLEncoding.EncodeToString(raw)
	return strings.Join(parts, ".")
}
