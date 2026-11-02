// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// introspectWithHint introspects the given token with an explicit token_type_hint and returns whether
// the token is reported active.
func (ts *TfidTestSuite) introspectWithHint(token, hint string) bool {
	form := url.Values{}
	form.Set("token", token)
	if hint != "" {
		form.Set("token_type_hint", hint)
	}

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/introspect",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err, "Failed to build introspection request")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(tfidTestClientID, tfidTestClientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Introspection request failed")
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "Introspection should return 200")

	var result struct {
		Active bool `json:"active"`
	}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&result), "Failed to parse introspection response")
	return result.Active
}

// revokeWithHint revokes the given token with an explicit token_type_hint and returns the HTTP status.
func (ts *TfidTestSuite) revokeWithHint(token, hint string) int {
	form := url.Values{}
	form.Set("token", token)
	if hint != "" {
		form.Set("token_type_hint", hint)
	}

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/revoke",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err, "Failed to build revocation request")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(tfidTestClientID, tfidTestClientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Revocation request failed")
	defer resp.Body.Close()
	return resp.StatusCode
}

// TestIntrospection_TokenTypeHint_Mismatched_StillCorrect verifies that token_type_hint at
// /oauth2/introspect does not affect the outcome: an access token introspected with a mismatched hint
// (or none at all) still correctly reports active/inactive based on the token itself.
func (ts *TfidTestSuite) TestIntrospection_TokenTypeHint_Mismatched_StillCorrect() {
	tokens := ts.obtainTokens()

	ts.True(ts.introspectWithHint(tokens.AccessToken, ""),
		"Access token should be active with no hint")
	ts.True(ts.introspectWithHint(tokens.AccessToken, "refresh_token"),
		"Access token should still be reported active with a mismatched refresh_token hint")
	ts.True(ts.introspectWithHint(tokens.AccessToken, "access_token"),
		"Access token should be active with the matching hint")

	ts.revokeWithHint(tokens.RefreshToken, "refresh_token")

	ts.False(ts.introspectWithHint(tokens.AccessToken, "access_token"),
		"Access token should be inactive after its login's refresh token is revoked, regardless of hint")
	ts.False(ts.introspectWithHint(tokens.AccessToken, ""),
		"Access token should be inactive after its login's refresh token is revoked, with no hint")
	ts.False(ts.introspectWithHint(tokens.AccessToken, "refresh_token"),
		"Access token should be inactive after its login's refresh token is revoked, even with a mismatched refresh_token hint")
}

// TestRevocation_TokenTypeHint_Mismatched_StillRevokes verifies that token_type_hint at /oauth2/revoke
// does not gate or misdirect revocation: revoking an access token with a mismatched refresh_token hint
// still succeeds and still revokes the presented token.
func (ts *TfidTestSuite) TestRevocation_TokenTypeHint_Mismatched_StillRevokes() {
	tokens := ts.obtainTokens()
	ts.Require().True(ts.introspectWithHint(tokens.AccessToken, ""), "Access token should start active")

	status := ts.revokeWithHint(tokens.AccessToken, "refresh_token")
	ts.Require().Equal(http.StatusOK, status, "Revocation should succeed despite the mismatched hint")

	ts.False(ts.introspectWithHint(tokens.AccessToken, ""),
		"The access token itself should be revoked even though the hint claimed it was a refresh token")
}
