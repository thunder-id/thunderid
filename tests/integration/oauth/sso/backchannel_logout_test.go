// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	backchannelLogoutEvent = "http://schemas.openid.net/event/backchannel-logout"
	// deliveryTimeout covers the default retry schedule: waits of 2 and 4 seconds plus the attempts.
	deliveryTimeout = 15 * time.Second
)

// logoutReceiver is a relying party's back-channel logout endpoint. It answers with the given
// statuses in turn, repeating the last one, and keeps every request it receives.
type logoutReceiver struct {
	srv      *httptest.Server
	mu       sync.Mutex
	statuses []int
	received []receivedLogout
	arrived  chan struct{}
}

type receivedLogout struct {
	contentType string
	token       string
}

func (ts *SSOLogoutTestSuite) newLogoutReceiver(statuses ...int) *logoutReceiver {
	r := &logoutReceiver{statuses: statuses, arrived: make(chan struct{}, 16)}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = req.ParseForm()
		r.mu.Lock()
		r.received = append(r.received, receivedLogout{
			contentType: req.Header.Get("Content-Type"),
			token:       req.PostForm.Get("logout_token"),
		})
		status := r.statuses[len(r.statuses)-1]
		if n := len(r.received); n <= len(r.statuses) {
			status = r.statuses[n-1]
		}
		r.mu.Unlock()
		if status == http.StatusFound {
			w.Header().Set("Location", "/redirected")
		}
		w.WriteHeader(status)
		r.arrived <- struct{}{}
	}))
	ts.T().Cleanup(r.srv.Close)
	return r
}

// await blocks until n requests arrived and returns them.
func (r *logoutReceiver) await(ts *SSOLogoutTestSuite, n int) []receivedLogout {
	ts.T().Helper()
	deadline := time.After(deliveryTimeout)
	for got := 0; got < n; got++ {
		select {
		case <-r.arrived:
		case <-deadline:
			ts.FailNowf("back-channel logout not delivered", "received %d of %d requests", got, n)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]receivedLogout(nil), r.received...)
}

// quiet asserts that no further request arrives within the given window.
func (r *logoutReceiver) quiet(ts *SSOLogoutTestSuite, window time.Duration) {
	ts.T().Helper()
	select {
	case <-r.arrived:
		ts.Fail("unexpected back-channel logout request")
	case <-time.After(window):
	}
}

// createBackchannelApplication registers a confidential client on the suite's SSO flow with the given
// back-channel logout URI, and deletes it when the test ends.
func (ts *SSOLogoutTestSuite) createBackchannelApplication(name, cID, cSecret, logoutURI string) {
	ts.T().Helper()

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:             name,
		Description:      "Application notified through OIDC Back-Channel Logout",
		OUID:             testOUID,
		Type:             "fullstack",
		AuthFlowID:       ts.authFlowID,
		AllowedUserTypes: []string{testUserType.Name},
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                cID,
					"clientSecret":            cSecret,
					"redirectUris":            []string{redirectURI},
					"backchannelLogoutUri":    logoutURI,
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"scopes":                  []string{"openid"},
				},
			},
		},
	})
	ts.Require().NoError(err, "failed to create back-channel logout application %s", name)
	ts.T().Cleanup(func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete application %s: %v", name, err)
		}
	})
}

// joinSession signs the given client in through the live SSO session and returns its tokens.
func (ts *SSOLogoutTestSuite) joinSession(client *http.Client, cID, cSecret, state string) *testutils.TokenResponse {
	ts.T().Helper()

	authID, executionID := ts.authorizeAsClient(client, cID, "openid", state)
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the authorize for %s should be satisfied by SSO", cID)
	code, err := testutils.ExtractAuthorizationCode(ts.completeAuthorization(client, authID, step.Assertion))
	ts.Require().NoError(err, "failed to extract the authorization code for %s", cID)
	return ts.exchangeCodeAsClient(client, code, cID, cSecret)
}

// signOut runs RP-Initiated Logout for the suite's main client to completion.
func (ts *SSOLogoutTestSuite) signOut(client *http.Client, idToken, state string) {
	ts.T().Helper()

	executionID, logoutID := ts.initiateLogout(client, idToken, postLogoutRedirectURI, state)
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the sign-out flow should complete")
	ts.completeLogout(client, logoutID)
}

// assertLogoutToken checks a delivered logout token against the ID token its relying party holds.
func (ts *SSOLogoutTestSuite) assertLogoutToken(got receivedLogout, cID string, idToken string) {
	ts.T().Helper()

	ts.Equal("application/x-www-form-urlencoded", got.contentType)
	ts.Require().NotEmpty(got.token, "the request should carry a logout_token parameter")

	header, err := testutils.DecodeJWTHeaderMap(got.token)
	ts.Require().NoError(err, "failed to decode the logout token header")
	ts.Equal("logout+jwt", header["typ"])
	ts.NotEmpty(header["kid"])

	claims, err := testutils.DecodeJWTPayloadMap(got.token)
	ts.Require().NoError(err, "failed to decode the logout token")
	idClaims, err := testutils.DecodeJWTPayloadMap(idToken)
	ts.Require().NoError(err, "failed to decode the ID token")

	ts.Equal(idClaims["iss"], claims["iss"])
	ts.Equal(idClaims["sub"], claims["sub"], "sub must match the relying party's ID token")
	ts.Equal(idClaims["sid"], claims["sid"], "sid must name the session the relying party joined")
	ts.Equal(cID, audience(claims["aud"]))
	ts.NotEmpty(claims["jti"])
	ts.NotContains(claims, "nonce", "a logout token must never carry a nonce")

	iat, _ := claims["iat"].(float64)
	exp, _ := claims["exp"].(float64)
	ts.InDelta(120, exp-iat, 1, "the default logout token lifetime is 120 seconds")

	events, ok := claims["events"].(map[string]any)
	ts.Require().True(ok, "the logout token must carry an events claim")
	member, ok := events[backchannelLogoutEvent].(map[string]any)
	ts.Require().True(ok, "events must hold the back-channel logout member")
	ts.Empty(member, "the back-channel logout member is an empty object")
}

func audience(aud any) string {
	switch v := aud.(type) {
	case string:
		return v
	case []any:
		if len(v) == 1 {
			s, _ := v[0].(string)
			return s
		}
	}
	return ""
}

// Signing out of one application notifies every other participant that registered an endpoint, and a
// participant whose endpoint is down does not hold back the others.
func (ts *SSOLogoutTestSuite) TestBackchannelLogout_SignOutNotifiesOtherParticipants() {
	username := "sso_bcl_signout_user"
	ts.createUser(username)

	healthy := ts.newLogoutReceiver(http.StatusOK)
	down := httptest.NewServer(http.NotFoundHandler())
	downURI := down.URL + "/bcl"
	down.Close()

	ts.createBackchannelApplication("SSOBackchannelHealthyApp", "sso_bcl_healthy_client", "sso_bcl_healthy_secret",
		healthy.srv.URL+"/bcl")
	ts.createBackchannelApplication("SSOBackchannelDownApp", "sso_bcl_down_client", "sso_bcl_down_secret", downURI)

	client := ts.newSessionClient()
	main := ts.loginTokens(client, username, "bcl_signout_1")
	partner := ts.joinSession(client, "sso_bcl_healthy_client", "sso_bcl_healthy_secret", "bcl_signout_2")
	ts.joinSession(client, "sso_bcl_down_client", "sso_bcl_down_secret", "bcl_signout_3")

	ts.signOut(client, main.IDToken, "bcl_signout_4")

	got := healthy.await(ts, 1)
	ts.assertLogoutToken(got[0], "sso_bcl_healthy_client", partner.IDToken)
	healthy.quiet(ts, time.Second)
}

// Deleting a user ends their sessions through administrative subject revocation, and that notifies
// the participants just as a sign-out does.
func (ts *SSOLogoutTestSuite) TestBackchannelLogout_UserDeletionNotifiesParticipants() {
	flowID := ts.deletionFlowID()
	username := "sso_bcl_deletion_user"
	userID := ts.createUser(username)

	receiver := ts.newLogoutReceiver(http.StatusOK)
	ts.createBackchannelApplication("SSOBackchannelDeletionApp", "sso_bcl_deletion_client",
		"sso_bcl_deletion_secret", receiver.srv.URL+"/bcl")

	client := ts.newSessionClient()
	ts.loginTokens(client, username, "bcl_deletion_1")
	partner := ts.joinSession(client, "sso_bcl_deletion_client", "sso_bcl_deletion_secret", "bcl_deletion_2")

	ts.deleteUserThroughFlow(flowID, userID)

	got := receiver.await(ts, 1)
	ts.assertLogoutToken(got[0], "sso_bcl_deletion_client", partner.IDToken)
}

// A transient failure is retried with a freshly built token, and a redirect is never followed.
func (ts *SSOLogoutTestSuite) TestBackchannelLogout_RetriesTransientFailureAndRefusesRedirect() {
	username := "sso_bcl_retry_user"
	ts.createUser(username)

	flaky := ts.newLogoutReceiver(http.StatusServiceUnavailable, http.StatusOK)
	redirecting := ts.newLogoutReceiver(http.StatusFound)
	ts.createBackchannelApplication("SSOBackchannelFlakyApp", "sso_bcl_flaky_client", "sso_bcl_flaky_secret",
		flaky.srv.URL+"/bcl")
	ts.createBackchannelApplication("SSOBackchannelRedirectApp", "sso_bcl_redirect_client",
		"sso_bcl_redirect_secret", redirecting.srv.URL+"/bcl")

	client := ts.newSessionClient()
	main := ts.loginTokens(client, username, "bcl_retry_1")
	partner := ts.joinSession(client, "sso_bcl_flaky_client", "sso_bcl_flaky_secret", "bcl_retry_2")
	ts.joinSession(client, "sso_bcl_redirect_client", "sso_bcl_redirect_secret", "bcl_retry_3")

	ts.signOut(client, main.IDToken, "bcl_retry_4")

	got := flaky.await(ts, 2)
	ts.assertLogoutToken(got[1], "sso_bcl_flaky_client", partner.IDToken)
	first, err := testutils.DecodeJWTPayloadMap(got[0].token)
	ts.Require().NoError(err)
	second, err := testutils.DecodeJWTPayloadMap(got[1].token)
	ts.Require().NoError(err)
	ts.NotEqual(first["jti"], second["jti"], "each attempt carries a freshly built token")

	// The 302 is answered once and its Location is never requested.
	redirecting.await(ts, 1)
	redirecting.quiet(ts, time.Second)
}
