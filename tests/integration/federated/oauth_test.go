// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

/*
Generic OAuth transport failures.

A generic OAuth connection has no ID token: the profile always comes from the userinfo endpoint, so
these cover what happens when the token or userinfo exchange misbehaves. As with the OIDC block, each
identity is given a local user first so a well-formed exchange would authenticate — otherwise every
assertion would hold merely because no user existed.
*/

// oauthUser registers an identity on the OAuth mock and a local user carrying its subject.
func (s *FederatedMappingSuite) oauthUser() string {
	s.T().Helper()
	sub := s.nextSubject()
	email := sub + "@example.com"
	s.mockOAuth.AddUser(&testutils.OAuthUserInfo{Sub: sub, Email: email, Name: "OAuth User"})
	s.createLocalUser(map[string]interface{}{"username": email, "email": email, "sub": sub})
	return sub
}

// authenticateOAuth applies a configuration and authenticates through the OAuth connection.
func (s *FederatedMappingSuite) authenticateOAuth(sub string) int {
	s.T().Helper()
	s.applyConfigTo("oauth", s.oauthIDPID, mapping(fedPersonType.Name, pair("email", "email")))
	status, _, _ := s.authenticateDirectVia(s.oauthIDPID, sub)
	return status
}

// BO16: the token response is well-formed JSON but carries no access token, so there is nothing to
// fetch the profile with.
func (s *FederatedMappingSuite) TestOAuthTokenResponseMissingAccessToken() {
	sub := s.oauthUser()
	defer s.mockOAuth.ClearOverrides()
	s.mockOAuth.SetTokenResponseOverride(func() (int, string) {
		return http.StatusOK, `{"token_type":"Bearer","expires_in":3600}`
	})

	s.authFails(s.authenticateOAuth(sub), "a token response with no access token must not authenticate")
}

// BO17: the token endpoint refuses the exchange.
func (s *FederatedMappingSuite) TestOAuthTokenEndpointError() {
	sub := s.oauthUser()
	defer s.mockOAuth.ClearOverrides()
	s.mockOAuth.SetTokenResponseOverride(func() (int, string) {
		return http.StatusBadRequest, `{"error":"invalid_grant"}`
	})

	s.authFails(s.authenticateOAuth(sub), "a rejected token exchange must not authenticate")
}

// BO18: the token endpoint answers with something that is not JSON.
func (s *FederatedMappingSuite) TestOAuthTokenEndpointMalformedJSON() {
	sub := s.oauthUser()
	defer s.mockOAuth.ClearOverrides()
	s.mockOAuth.SetTokenResponseOverride(func() (int, string) {
		return http.StatusOK, `{"access_token": `
	})

	s.authFails(s.authenticateOAuth(sub), "an unparseable token response must not authenticate")
}

// BO19: the profile cannot be fetched.
func (s *FederatedMappingSuite) TestOAuthUserInfoEndpointError() {
	sub := s.oauthUser()
	defer s.mockOAuth.ClearOverrides()
	s.mockOAuth.SetUserInfoResponseOverride(func() (int, string) {
		return http.StatusUnauthorized, `{"error":"invalid_token"}`
	})

	s.authFails(s.authenticateOAuth(sub),
		"a failed profile fetch must not authenticate: OAuth has no ID token to fall back on")
}

// BO20: the profile response is not JSON.
func (s *FederatedMappingSuite) TestOAuthUserInfoMalformedJSON() {
	sub := s.oauthUser()
	defer s.mockOAuth.ClearOverrides()
	s.mockOAuth.SetUserInfoResponseOverride(func() (int, string) {
		return http.StatusOK, `not-json-at-all`
	})

	s.authFails(s.authenticateOAuth(sub), "an unparseable profile must not authenticate")
}

// BO21: the profile carries no usable subject. Without one there is no identity to resolve.
func (s *FederatedMappingSuite) TestOAuthUserInfoWithoutUsableSub() {
	bodies := map[string]string{
		"missing":    `{"email":"nobody@example.com"}`,
		"empty":      `{"sub":"","email":"nobody@example.com"}`,
		"non_string": `{"sub":42,"email":"nobody@example.com"}`,
	}
	for name, body := range bodies {
		s.Run(name, func() {
			sub := s.oauthUser()
			defer s.mockOAuth.ClearOverrides()
			s.mockOAuth.SetUserInfoResponseOverride(func() (int, string) {
				return http.StatusOK, body
			})

			s.authFails(s.authenticateOAuth(sub), "%s sub must not authenticate", name)
		})
	}
}

// BO22: the provider sends the user back with an error rather than an authorization code, which is what
// a cancelled consent screen looks like.
//
// The direct endpoint understands this: it accepts error and error_description and refuses the exchange.
// The flow executors do not — ProcessAuthFlowResponse reads only code and state, so a cancelled
// authorization arrives as "no code", clears the authenticated user and returns without an error. The
// provider's reason never surfaces. That asymmetry is recorded as G21; this asserts the endpoint that
// does handle it.
func (s *FederatedMappingSuite) TestOAuthProviderErrorCallbackRefused() {
	status, body := s.postJSON(directAuthStart, map[string]interface{}{"idpId": s.oauthIDPID})
	s.Require().Equal(http.StatusOK, status, string(body))

	var start struct {
		SessionToken string `json:"sessionToken"`
	}
	s.Require().NoError(jsonUnmarshal(body, &start))

	status, body = s.postJSON(directAuthFinish, map[string]interface{}{
		"sessionToken":      start.SessionToken,
		"error":             "access_denied",
		"error_description": "User denied access",
	})

	s.Equal(http.StatusBadRequest, status,
		"a provider error callback should be refused as a client error, got %s", string(body))
}

// BO28: scope delimiters. The connection stores scopes as a comma-separated string and the authorize
// request must carry them space-separated, as the specification requires.
//
// All three configurations the plan names are covered, because they exercise different code: a
// multi-scope list is split on commas, a single scope has no delimiter to split on, and an empty value
// must produce no scope parameter rather than an empty one.
func (s *FederatedMappingSuite) TestOAuthScopesOnAuthorizeURL() {
	cases := []struct {
		name     string
		stored   string
		expected []string
	}{
		{"comma_separated", "openid,email,profile", []string{"openid", "email", "profile"}},
		{"single_scope", "openid", []string{"openid"}},
		{"empty", "", nil},
	}

	for _, testCase := range cases {
		s.Run(testCase.name, func() {
			s.setOAuthScopes(testCase.stored)
			defer s.setOAuthScopes("openid,email,profile")

			status, body := s.postJSON(directAuthStart, map[string]interface{}{"idpId": s.oauthIDPID})
			s.Require().Equal(http.StatusOK, status, string(body))

			var start struct {
				RedirectURL string `json:"redirectUrl"`
			}
			s.Require().NoError(jsonUnmarshal(body, &start))

			parsed, err := url.Parse(start.RedirectURL)
			s.Require().NoError(err, "the authorize URL should parse")
			scope := parsed.Query().Get("scope")

			s.NotContains(scope, ",",
				"%s: the stored comma-separated list must not reach the provider verbatim", testCase.name)
			s.Equal(testCase.expected, splitScopeParam(scope),
				"%s: unexpected scope parameter %q", testCase.name, scope)
		})
	}
}

// splitScopeParam returns the scope values, or nil when the parameter is absent or empty.
func splitScopeParam(scope string) []string {
	if strings.TrimSpace(scope) == "" {
		return nil
	}
	return strings.Fields(scope)
}
