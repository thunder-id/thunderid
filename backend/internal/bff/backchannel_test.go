// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// fakeVerifier stands in for the server's JWT service. It accepts every token unless err is set.
type fakeVerifier struct {
	err      *tidcommon.ServiceError
	audience string
	issuer   string
}

func (v *fakeVerifier) VerifyJWT(_ context.Context, _ string, expectedAud, expectedIss string) *tidcommon.ServiceError {
	v.audience = expectedAud
	v.issuer = expectedIss
	return v.err
}

// logoutClaims are the claims of a valid logout token for the test session.
func (s *HandlerTestSuite) logoutClaims() map[string]any {
	return map[string]any{
		"iss": testPublicURL, "aud": "CONSOLE", "sub": "user-1", "sid": "session-1", "jti": "jti-1",
		"iat": float64(s.now.Unix()), "exp": float64(s.now.Unix() + 120),
		"events": map[string]any{backchannelLogoutEvent: map[string]any{}},
	}
}

func logoutToken(header, claims map[string]any) string {
	rawHeader, _ := json.Marshal(header)
	rawClaims, _ := json.Marshal(claims)
	return base64.RawURLEncoding.EncodeToString(rawHeader) + "." +
		base64.RawURLEncoding.EncodeToString(rawClaims) + ".sig"
}

func (s *HandlerTestSuite) postLogout(contentType, body string) *http.Response {
	req := httptest.NewRequest(http.MethodPost, testPublicURL+"/console/auth/backchannel-logout",
		strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	return rec.Result()
}

func (s *HandlerTestSuite) sendLogoutToken(token string) *http.Response {
	return s.postLogout("application/x-www-form-urlencoded", url.Values{"logout_token": {token}}.Encode())
}

func (s *HandlerTestSuite) TestBackchannelLogout_EndsTheSession() {
	s.signIn()
	s.Require().True(s.checkSession())

	resp := s.sendLogoutToken(logoutToken(map[string]any{"alg": "RS256", "typ": "logout+jwt"},
		s.logoutClaims()))

	s.Equal(http.StatusOK, resp.StatusCode)
	s.Equal("no-store", resp.Header.Get("Cache-Control"))
	s.Equal("CONSOLE", s.verifier.audience)
	s.Equal(testPublicURL, s.verifier.issuer)
	s.True(s.h.ended.isEnded(context.Background(), "session-1"))
	if !s.stateless() {
		// The tokens leave the store before the browser makes another request.
		s.Equal(0, s.store.count(providers.NamespaceConsoleSession))
		s.Equal(0, s.store.count(providers.NamespaceConsoleSessionByID))
	}
	s.False(s.checkSession())
}

func (s *HandlerTestSuite) TestBackchannelLogout_AcceptsATokenWithoutType() {
	resp := s.sendLogoutToken(logoutToken(map[string]any{"alg": "RS256"}, s.logoutClaims()))

	s.Equal(http.StatusOK, resp.StatusCode)
}

func (s *HandlerTestSuite) TestBackchannelLogout_RefusesInvalidRequests() {
	header := map[string]any{"alg": "RS256", "typ": "logout+jwt"}
	without := func(name string) string {
		claims := s.logoutClaims()
		delete(claims, name)
		return logoutToken(header, claims)
	}
	with := func(name string, value any) string {
		claims := s.logoutClaims()
		claims[name] = value
		return logoutToken(header, claims)
	}
	form := func(token string) string {
		return url.Values{"logout_token": {token}}.Encode()
	}
	formType := "application/x-www-form-urlencoded"

	cases := []struct {
		name        string
		contentType string
		body        string
	}{
		{"JSON body", "application/json", `{"logout_token":"x"}`},
		{"missing token", formType, ""},
		{"oversized body", formType, form(strings.Repeat("a", maxLogoutRequestBytes))},
		{"encrypted token", formType, form("a.b.c.d.e")},
		{"undecodable token", formType, form("a.b.c")},
		{"wrong type", formType, form(logoutToken(map[string]any{"alg": "RS256", "typ": "JWT"},
			s.logoutClaims()))},
		{"no iat", formType, form(without("iat"))},
		{"no events", formType, form(without("events"))},
		{"other event", formType, form(with("events", map[string]any{"urn:other": map[string]any{}}))},
		{"event not an object", formType, form(with("events", map[string]any{backchannelLogoutEvent: "x"}))},
		{"nonce", formType, form(with("nonce", "n"))},
		{"no sid", formType, form(without("sid"))},
		{"no jti", formType, form(without("jti"))},
	}
	for _, tc := range cases {
		resp := s.postLogout(tc.contentType, tc.body)

		s.Equal(http.StatusBadRequest, resp.StatusCode, tc.name)
		s.Equal("invalid_request", decodeBody(resp)["error"], tc.name)
	}
	s.False(s.h.ended.isEnded(context.Background(), "session-1"))
}

func (s *HandlerTestSuite) TestBackchannelLogout_RefusesAFailedSignature() {
	s.signIn()
	s.verifier.err = &tidcommon.ServiceError{Code: "JWT-60001"}

	resp := s.sendLogoutToken(logoutToken(map[string]any{"alg": "RS256"}, s.logoutClaims()))

	s.Equal(http.StatusBadRequest, resp.StatusCode)
	s.True(s.checkSession())
}

func (s *HandlerTestSuite) TestBackchannelLogout_RefusesAReplay() {
	token := logoutToken(map[string]any{"alg": "RS256"}, s.logoutClaims())

	s.Equal(http.StatusOK, s.sendLogoutToken(token).StatusCode)
	resp := s.sendLogoutToken(token)

	s.Equal(http.StatusBadRequest, resp.StatusCode)
	s.Equal("logout_token was already used", decodeBody(resp)["error_description"])
}

func (s *HandlerTestSuite) TestBackchannelLogout_WithoutIssuerFails() {
	s.server.noIssuer = true

	resp := s.sendLogoutToken(logoutToken(map[string]any{"alg": "RS256"}, s.logoutClaims()))

	s.Equal(http.StatusInternalServerError, resp.StatusCode)
}
