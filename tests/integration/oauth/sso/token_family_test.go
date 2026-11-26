// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	rejoinUsername        = "sso_rejoin_user"
	rejoinRevokedUsername = "sso_rejoin_revoked_user"
	revokeSharedUsername  = "sso_revoke_shared_user"
	reauthFamilyUsername  = "sso_reauth_family_user"
)

// TestSignOut_RevokesEveryGrantOfARejoinedApplication covers an application that signs in again through SSO:
// signing out revokes both grants, not only the latest one.
func (ts *SSOLogoutTestSuite) TestSignOut_RevokesEveryGrantOfARejoinedApplication() {
	ts.createUser(rejoinUsername)
	client := ts.newSessionClient()

	first := ts.loginTokens(client, rejoinUsername, "rejoin_state_1")
	second := ts.joinSession(client, clientID, clientSecret, "rejoin_state_2")
	ts.Equal(ts.idTokenClaim(first.IDToken, "sid"), ts.idTokenClaim(second.IDToken, "sid"),
		"both grants belong to one session")
	ts.Equal(ts.tokenFamily(first.AccessToken), ts.tokenFamily(second.AccessToken),
		"the second grant joins the participant's token family")
	grants := []*testutils.TokenResponse{first, second}
	for i, grant := range grants {
		ts.Require().True(ts.introspectActive(client, grant.AccessToken), "grant %d is active before sign-out", i+1)
	}

	ts.signOut(client, second.IDToken, "rejoin_state_3")

	for i, grant := range grants {
		ts.False(ts.introspectActive(client, grant.AccessToken), "sign-out revokes grant %d's access token", i+1)
		ts.assertRefreshRevoked(grant.RefreshToken, "sign-out revokes grant %d's refresh token", i+1)
	}
}

// TestReauthentication_KeepsTheApplicationsTokenFamily covers a second sign-in with prompt=login: signing out
// still revokes both grants.
func (ts *SSOLogoutTestSuite) TestReauthentication_KeepsTheApplicationsTokenFamily() {
	ts.createUser(reauthFamilyUsername)
	client := ts.newSessionClient()

	first := ts.loginTokens(client, reauthFamilyUsername, "reauth_family_state_1")
	resp := ts.authorizeRaw(client, "openid", "reauth_family_state_2", map[string]string{"prompt": "login"})
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "authorize should redirect to the gate")
	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "failed to extract auth data from authorize redirect")
	step, _ := ts.runCredentialLogin(client, executionID, reauthFamilyUsername)
	code, err := testutils.ExtractAuthorizationCode(ts.completeAuthorization(client, authID, step.Assertion))
	ts.Require().NoError(err, "failed to extract authorization code")
	second := ts.exchangeCode(client, code)

	ts.Equal(ts.idTokenClaim(first.IDToken, "sid"), ts.idTokenClaim(second.IDToken, "sid"),
		"the re-authentication stays in the same session")
	ts.Equal(ts.tokenFamily(first.AccessToken), ts.tokenFamily(second.AccessToken),
		"the re-authenticated grant joins the participant's token family")

	ts.signOut(client, second.IDToken, "reauth_family_state_3")

	for i, grant := range []*testutils.TokenResponse{first, second} {
		ts.False(ts.introspectActive(client, grant.AccessToken), "sign-out revokes grant %d's access token", i+1)
		ts.assertRefreshRevoked(grant.RefreshToken, "sign-out revokes grant %d's refresh token", i+1)
	}
}

// TestRejoinAfterRevocation_GetsANewTokenFamily covers a sign-in after the application's family was revoked: it
// gets a new family, which signing out revokes.
func (ts *SSOLogoutTestSuite) TestRejoinAfterRevocation_GetsANewTokenFamily() {
	ts.createUser(rejoinRevokedUsername)
	client := ts.newSessionClient()

	first := ts.loginTokens(client, rejoinRevokedUsername, "rejoin_revoked_state_1")
	ts.revokeRefreshToken(client, first.RefreshToken)
	ts.Require().False(ts.introspectActive(client, first.AccessToken),
		"revoking the refresh token revokes its token family")

	second := ts.joinSession(client, clientID, clientSecret, "rejoin_revoked_state_2")
	ts.NotEqual(ts.tokenFamily(first.AccessToken), ts.tokenFamily(second.AccessToken),
		"the rejoin must not reuse the revoked token family")
	ts.Require().True(ts.introspectActive(client, second.AccessToken), "the rejoin's grant is active")

	ts.signOut(client, second.IDToken, "rejoin_revoked_state_3")

	ts.False(ts.introspectActive(client, second.AccessToken), "sign-out revokes the new token family")
	ts.assertRefreshRevoked(second.RefreshToken, "sign-out revokes the new token family's refresh token")
}

// TestRevokingOneGrant_RevokesTheApplicationsOtherGrants pins the trade-off of a shared family: revoking one
// grant also revokes the application's other grant in the session.
func (ts *SSOLogoutTestSuite) TestRevokingOneGrant_RevokesTheApplicationsOtherGrants() {
	ts.createUser(revokeSharedUsername)
	client := ts.newSessionClient()

	first := ts.loginTokens(client, revokeSharedUsername, "revoke_shared_state_1")
	second := ts.joinSession(client, clientID, clientSecret, "revoke_shared_state_2")
	ts.Require().True(ts.introspectActive(client, second.AccessToken), "the second grant starts active")

	ts.revokeRefreshToken(client, first.RefreshToken)

	ts.False(ts.introspectActive(client, second.AccessToken),
		"revoking the first grant also revokes the second grant's access token")
	ts.assertRefreshRevoked(second.RefreshToken, "revoking the first grant also revokes the second grant's refresh token")
}

// revokeRefreshToken revokes a refresh token of the suite's main client at the RFC 7009 endpoint.
func (ts *SSOLogoutTestSuite) revokeRefreshToken(client *http.Client, refreshToken string) {
	form := url.Values{}
	form.Set("token", refreshToken)
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/oauth2/revoke",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := client.Do(req)
	ts.Require().NoError(err, "revocation request failed")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "revocation should succeed: %s", string(body))
}

// assertRefreshRevoked asserts the token endpoint rejects a refresh token of the suite's main client.
func (ts *SSOLogoutTestSuite) assertRefreshRevoked(refreshToken string, msgAndArgs ...interface{}) {
	ts.Require().NotEmpty(refreshToken, "a refresh token should be issued")
	result, err := testutils.RefreshAccessTokenRaw(clientID, clientSecret, refreshToken, "")
	ts.Require().NoError(err, "refresh request failed")
	ts.Equal(http.StatusBadRequest, result.StatusCode, msgAndArgs...)

	var body struct {
		Error string `json:"error"`
	}
	ts.Require().NoError(json.Unmarshal(result.Body, &body), "failed to decode the refresh error: %s", result.Body)
	ts.Equal("invalid_grant", body.Error, msgAndArgs...)
}
