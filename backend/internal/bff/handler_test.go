// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
)

const (
	testPublicURL = "https://id.example.com"
	// testDevOrigin is a development server origin the client registers a redirect URI on.
	testDevOrigin = "https://localhost:5191"
	testUserAgent = "bff-test"
)

// fakeServer stands in for the server's handler chain: discovery, the token endpoint, and one API.
type fakeServer struct {
	mu            sync.Mutex
	nonce         string
	sid           string
	refreshFails  bool
	noIssuer      bool
	tokenRequests []url.Values
	apiRequests   []*http.Request
	apiBodies     []string
}

func (f *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		md := map[string]string{
			"issuer":                 testPublicURL,
			"authorization_endpoint": testPublicURL + "/oauth2/authorize",
			"token_endpoint":         testPublicURL + "/oauth2/token",
			"end_session_endpoint":   testPublicURL + "/oauth2/logout",
		}
		if f.noIssuer {
			delete(md, "issuer")
		}
		_ = json.NewEncoder(w).Encode(md)
	case "/oauth2/token":
		_ = r.ParseForm()
		f.tokenRequests = append(f.tokenRequests, r.PostForm)
		if r.PostForm.Get("grant_type") == "refresh_token" && f.refreshFails {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		n := len(f.tokenRequests)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access-" + string(rune('0'+n)),
			"expires_in":    3600,
			"refresh_token": "refresh-" + string(rune('0'+n)),
			"id_token": fakeJWT(map[string]any{
				"iss": testPublicURL, "aud": "CONSOLE", "sub": "user-1", "name": "Ada",
				"email": "ada@example.com", "nonce": f.nonce, "sid": f.sid,
			}),
		})
	case "/users":
		body, _ := io.ReadAll(r.Body)
		f.apiRequests = append(f.apiRequests, r)
		f.apiBodies = append(f.apiBodies, string(body))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Internal", "secret")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// HandlerTestSuite runs the endpoint tests against one session mode.
type HandlerTestSuite struct {
	suite.Suite
	mode     string
	server   *fakeServer
	store    *memStore
	verifier *fakeVerifier
	h        *handler
	mux      *http.ServeMux
	now      time.Time
	jar      map[string]*http.Cookie
}

func TestHandlerTestSuite_Stateless(t *testing.T) {
	suite.Run(t, &HandlerTestSuite{mode: SessionModeStateless})
}

func TestHandlerTestSuite_Stateful(t *testing.T) {
	suite.Run(t, &HandlerTestSuite{mode: SessionModeStateful})
}

func (s *HandlerTestSuite) stateless() bool {
	return s.mode == SessionModeStateless
}

func (s *HandlerTestSuite) SetupTest() {
	s.server = &fakeServer{sid: "session-1"}
	s.jar = map[string]*http.Cookie{}
	s.now = time.Unix(1_800_000_000, 0)

	cryptoSvc := cryptomock.NewRuntimeCryptoProviderMock(s.T())
	cryptoSvc.EXPECT().Encrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ *providers.KeyRef, _ string, _ map[string]interface{},
			content []byte) ([]byte, *providers.CryptoDetails, error) {
			return append([]byte("sealed:"), content...), nil, nil
		}).Maybe()
	cryptoSvc.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ *providers.KeyRef, _ string, _ map[string]interface{},
			content []byte) ([]byte, error) {
			plaintext, ok := strings.CutPrefix(string(content), "sealed:")
			if !ok {
				return nil, errors.New("not sealed")
			}
			return []byte(plaintext), nil
		}).Maybe()

	s.store = newMemStore(func() time.Time { return s.now })
	s.verifier = &fakeVerifier{}

	clients := actorprovidermock.NewActorProviderMock(s.T())
	clients.EXPECT().GetOAuthClientByClientID(mock.Anything, "CONSOLE").Return(&providers.OAuthClient{
		RedirectURIs: []string{testPublicURL + "/console", testDevOrigin + "/console"},
	}, nil).Maybe()

	upstream := &syshttp.InProcessTransport{}
	upstream.SetHandler(s.server)
	h, err := newHandler(Config{
		ClientID:             "CONSOLE",
		Scopes:               []string{"openid", "profile"},
		Resource:             testPublicURL + "/mcp",
		PublicURL:            testPublicURL,
		AppPath:              "/console",
		BasePath:             "/console/auth",
		CookieName:           "tid_console",
		SecureCookies:        true,
		BlockedProxyPrefixes: []string{"/console/", "/gate/"},
		SessionMode:          s.mode,
		// Longer than the tests move the clock, so only the access token expires. Idle expiry has
		// its own test.
		IdleTimeout: 4 * 3600,
	}, upstream, cryptoSvc, s.store, newEndedSessions(s.store), clients, s.verifier)
	s.Require().NoError(err)
	s.h = h
	clock := func() time.Time { return s.now }
	s.h.now = clock
	switch sessions := s.h.sessions.(type) {
	case *statelessSessions:
		sessions.now = clock
	case *statefulSessions:
		sessions.now = clock
		sessions.refreshWait = 2 * time.Second
		sessions.refreshPoll = 5 * time.Millisecond
	}
	s.mux = http.NewServeMux()
	registerRoutes(s.mux, s.h)
}

// do sends a request with the cookies the browser would send for its path, and stores the cookies
// the response sets.
func (s *HandlerTestSuite) do(method, target string, body io.Reader, xhr bool) *http.Response {
	req := httptest.NewRequest(method, testPublicURL+target, body)
	req.Header.Set("User-Agent", testUserAgent)
	if xhr {
		req.Header.Set(headerRequestedWith, "XMLHttpRequest")
	}
	for _, c := range s.jar {
		if strings.HasPrefix(req.URL.Path, c.Path) {
			req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
		}
	}
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	resp := rec.Result()
	for _, c := range resp.Cookies() {
		if c.MaxAge < 0 {
			delete(s.jar, c.Name)
		} else {
			s.jar[c.Name] = c
		}
	}
	return resp
}

func (s *HandlerTestSuite) signIn() {
	resp := s.do(http.MethodGet, "/console/auth/signin?returnTo=/console/users", nil, false)
	s.Require().Equal(http.StatusFound, resp.StatusCode)
	authorize, err := url.Parse(resp.Header.Get("Location"))
	s.Require().NoError(err)
	s.server.nonce = authorize.Query().Get("nonce")

	resp = s.do(http.MethodGet, "/console/auth/callback?code=abc&state="+authorize.Query().Get("state"), nil, false)
	s.Require().Equal(http.StatusFound, resp.StatusCode)
	s.Require().Equal("/console/users", resp.Header.Get("Location"))
}

func decodeBody(resp *http.Response) map[string]any {
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return body
}

func (s *HandlerTestSuite) TestSignIn_RedirectsToAuthorizeWithPKCE() {
	resp := s.do(http.MethodGet, "/console/auth/signin", nil, false)

	s.Equal(http.StatusFound, resp.StatusCode)
	authorize, err := url.Parse(resp.Header.Get("Location"))
	s.Require().NoError(err)
	s.Equal("/oauth2/authorize", authorize.Path)
	q := authorize.Query()
	s.Equal("CONSOLE", q.Get("client_id"))
	s.Equal(testPublicURL+"/console/auth/callback", q.Get("redirect_uri"))
	s.Equal("openid profile", q.Get("scope"))
	s.Equal("S256", q.Get("code_challenge_method"))
	s.Equal(testPublicURL+"/mcp", q.Get("resource"))
	s.NotEmpty(q.Get("state"))
	s.NotEmpty(q.Get("nonce"))

	txn := s.jar["__Secure-tid_console_txn"]
	s.Require().NotNil(txn)
	s.Equal("/console/auth/callback", txn.Path)
	s.True(txn.HttpOnly)
	s.True(txn.Secure)
	s.Equal(http.SameSiteLaxMode, txn.SameSite)
}

func (s *HandlerTestSuite) TestCallback_StartsSessionWithoutExposingTokens() {
	s.signIn()

	s.Require().Len(s.server.tokenRequests, 1)
	form := s.server.tokenRequests[0]
	s.Equal("authorization_code", form.Get("grant_type"))
	s.Equal("abc", form.Get("code"))
	s.Equal("CONSOLE", form.Get("client_id"))
	s.NotEmpty(form.Get("code_verifier"))

	s.Contains(s.jar, "__Host-tid_console_session")
	if s.stateless() {
		s.Equal("/console/auth/refresh", s.jar["__Secure-tid_console_refresh"].Path)
	} else {
		s.NotContains(s.jar, "__Secure-tid_console_refresh")
		s.NotContains(s.jar["__Host-tid_console_session"].Value, "access-1")
	}
	s.NotContains(s.jar, "__Secure-tid_console_txn")

	resp := s.do(http.MethodGet, "/console/auth/session", nil, true)
	s.Equal(http.StatusOK, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	s.NotContains(string(raw), "access-1")
	s.NotContains(string(raw), "refresh-1")
	var body map[string]any
	s.Require().NoError(json.Unmarshal(raw, &body))
	s.Equal(true, body["isSignedIn"])
	user := body["user"].(map[string]any)
	s.Equal("user-1", user["sub"])
	s.Equal("Ada", user["name"])
	s.NotContains(user, "iss")
	s.NotContains(user, "nonce")
	s.NotContains(user, "sid")
}

func (s *HandlerTestSuite) TestCallback_RejectsStateMismatch() {
	s.do(http.MethodGet, "/console/auth/signin", nil, false)

	resp := s.do(http.MethodGet, "/console/auth/callback?code=abc&state=forged", nil, false)

	s.Equal(http.StatusFound, resp.StatusCode)
	s.Equal("/console?error=invalid_request", resp.Header.Get("Location"))
	s.Empty(s.server.tokenRequests)
	s.NotContains(s.jar, "__Host-tid_console_session")
}

func (s *HandlerTestSuite) TestCallback_RejectsNonceMismatch() {
	resp := s.do(http.MethodGet, "/console/auth/signin", nil, false)
	authorize, _ := url.Parse(resp.Header.Get("Location"))
	s.server.nonce = "other"

	resp = s.do(http.MethodGet, "/console/auth/callback?code=abc&state="+authorize.Query().Get("state"), nil, false)

	s.Equal("/console?error=invalid_request", resp.Header.Get("Location"))
	s.NotContains(s.jar, "__Host-tid_console_session")
}

func (s *HandlerTestSuite) TestCallback_WithoutTransactionCookie() {
	resp := s.do(http.MethodGet, "/console/auth/callback?code=abc&state=x", nil, false)

	s.Equal("/console?error=invalid_request", resp.Header.Get("Location"))
}

func (s *HandlerTestSuite) TestCallback_ForwardsAuthorizationError() {
	resp := s.do(http.MethodGet, "/console/auth/signin", nil, false)
	authorize, _ := url.Parse(resp.Header.Get("Location"))

	resp = s.do(http.MethodGet,
		"/console/auth/callback?error=access_denied&state="+authorize.Query().Get("state"), nil, false)

	s.Equal("/console?error=access_denied", resp.Header.Get("Location"))
	s.Empty(s.server.tokenRequests)
}

func (s *HandlerTestSuite) TestSession_SignedOutWithoutCookie() {
	resp := s.do(http.MethodGet, "/console/auth/session", nil, true)

	s.Equal(http.StatusOK, resp.StatusCode)
	s.Equal(map[string]any{"isSignedIn": false}, decodeBody(resp))
}

func (s *HandlerTestSuite) TestSameOriginChecks() {
	s.signIn()

	resp := s.do(http.MethodGet, "/console/auth/session", nil, false)
	s.Equal(http.StatusForbidden, resp.StatusCode)

	req := httptest.NewRequest(http.MethodGet, testPublicURL+"/console/auth/session", nil)
	req.Header.Set(headerRequestedWith, "XMLHttpRequest")
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	s.Equal(http.StatusForbidden, rec.Code)

	req.Header.Set("Origin", testPublicURL)
	rec = httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	s.Equal(http.StatusOK, rec.Code)
}

func (s *HandlerTestSuite) TestSameOriginChecks_AcceptsRegisteredOrigin() {
	s.signIn()

	for origin, want := range map[string]int{
		testDevOrigin:            http.StatusOK,
		"https://localhost:5192": http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodGet, testPublicURL+"/console/auth/session", nil)
		req.Header.Set(headerRequestedWith, "XMLHttpRequest")
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		s.mux.ServeHTTP(rec, req)
		s.Equal(want, rec.Code, origin)
	}
}

func (s *HandlerTestSuite) TestSignIn_BehindProxyOnRegisteredOrigin() {
	req := httptest.NewRequest(http.MethodGet, testPublicURL+"/console/auth/signin", nil)
	req.Header.Set("X-Forwarded-Host", "localhost:5191")
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	authorize, err := url.Parse(rec.Header().Get("Location"))
	s.Require().NoError(err)
	s.Equal(testDevOrigin+"/console/auth/callback", authorize.Query().Get("redirect_uri"))
}

func (s *HandlerTestSuite) TestSignIn_IgnoresUnregisteredForwardedHost() {
	req := httptest.NewRequest(http.MethodGet, testPublicURL+"/console/auth/signin", nil)
	req.Header.Set("X-Forwarded-Host", "evil.example.com")
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	authorize, err := url.Parse(rec.Header().Get("Location"))
	s.Require().NoError(err)
	s.Equal(testPublicURL+"/console/auth/callback", authorize.Query().Get("redirect_uri"))
}

func (s *HandlerTestSuite) TestCallback_ReusesTheAuthorizationRedirectURI() {
	req := httptest.NewRequest(http.MethodGet, testPublicURL+"/console/auth/signin", nil)
	req.Header.Set("X-Forwarded-Host", "localhost:5191")
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		s.jar[c.Name] = c
	}
	authorize, _ := url.Parse(rec.Header().Get("Location"))
	s.server.nonce = authorize.Query().Get("nonce")

	resp := s.do(http.MethodGet, "/console/auth/callback?code=abc&state="+authorize.Query().Get("state"), nil, false)

	s.Equal(http.StatusFound, resp.StatusCode)
	s.Require().Len(s.server.tokenRequests, 1)
	s.Equal(testDevOrigin+"/console/auth/callback", s.server.tokenRequests[0].Get("redirect_uri"))
}

func (s *HandlerTestSuite) TestSignOut_BehindProxyReturnsToRegisteredOrigin() {
	s.signIn()

	req := httptest.NewRequest(http.MethodPost, testPublicURL+"/console/auth/signout", nil)
	req.Header.Set(headerRequestedWith, "XMLHttpRequest")
	req.Header.Set("X-Forwarded-Host", "localhost:5191")
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)

	redirect, err := url.Parse(decodeBody(rec.Result())["redirectUrl"].(string))
	s.Require().NoError(err)
	s.Equal(testDevOrigin+"/console", redirect.Query().Get("post_logout_redirect_uri"))
}

func (s *HandlerTestSuite) TestRefresh_RenewsExpiredSession() {
	s.signIn()
	s.now = s.now.Add(2 * time.Hour)

	if s.stateless() {
		// Only the refresh endpoint receives a stateless session's refresh token.
		resp := s.do(http.MethodGet, "/console/auth/session", nil, true)
		s.Equal(http.StatusUnauthorized, resp.StatusCode)
		s.Equal("session_expired", decodeBody(resp)["error"])
	}

	resp := s.do(http.MethodPost, "/console/auth/refresh", nil, true)
	s.Equal(http.StatusNoContent, resp.StatusCode)
	s.Require().Len(s.server.tokenRequests, 2)
	s.Equal("refresh_token", s.server.tokenRequests[1].Get("grant_type"))
	s.Equal("refresh-1", s.server.tokenRequests[1].Get("refresh_token"))

	resp = s.do(http.MethodGet, "/console/auth/session", nil, true)
	s.Equal(http.StatusOK, resp.StatusCode)
}

func (s *HandlerTestSuite) TestRefresh_FailureSignsOut() {
	s.signIn()
	s.server.refreshFails = true

	resp := s.do(http.MethodPost, "/console/auth/refresh", nil, true)

	s.Equal(http.StatusUnauthorized, resp.StatusCode)
	s.NotContains(s.jar, "__Host-tid_console_session")
	s.NotContains(s.jar, "__Secure-tid_console_refresh")
}

func (s *HandlerTestSuite) TestEndedSession_SignsOut() {
	s.signIn()
	s.endSSOSession("session-1")

	resp := s.do(http.MethodGet, "/console/auth/session", nil, true)

	s.Equal(map[string]any{"isSignedIn": false}, decodeBody(resp))
	s.NotContains(s.jar, "__Host-tid_console_session")
}

func (s *HandlerTestSuite) TestSignOut_ReturnsEndSessionURL() {
	s.signIn()

	resp := s.do(http.MethodPost, "/console/auth/signout", nil, true)

	s.Equal(http.StatusOK, resp.StatusCode)
	redirect, err := url.Parse(decodeBody(resp)["redirectUrl"].(string))
	s.Require().NoError(err)
	s.Equal("/oauth2/logout", redirect.Path)
	s.Equal("CONSOLE", redirect.Query().Get("client_id"))
	s.Equal(testPublicURL+"/console", redirect.Query().Get("post_logout_redirect_uri"))
	s.NotEmpty(redirect.Query().Get("id_token_hint"))
	s.NotContains(s.jar, "__Host-tid_console_session")
	s.NotContains(s.jar, "__Secure-tid_console_refresh")
}

func (s *HandlerTestSuite) TestProxy_ForwardsWithAccessToken() {
	s.signIn()

	resp := s.do(http.MethodPost, "/console/auth/proxy/users?limit=5", strings.NewReader(`{"a":1}`), true)

	s.Equal(http.StatusCreated, resp.StatusCode)
	s.Equal("application/json", resp.Header.Get("Content-Type"))
	s.Empty(resp.Header.Get("X-Internal"))
	body, _ := io.ReadAll(resp.Body)
	s.JSONEq(`{"ok":true}`, string(body))

	s.Require().Len(s.server.apiRequests, 1)
	upstream := s.server.apiRequests[0]
	s.Equal(http.MethodPost, upstream.Method)
	s.Equal("limit=5", upstream.URL.RawQuery)
	s.Equal("Bearer access-1", upstream.Header.Get("Authorization"))
	s.Empty(upstream.Header.Get("Cookie"))
	s.Empty(upstream.Header.Get("User-Agent"))
	s.Equal(`{"a":1}`, s.server.apiBodies[0])
}

func (s *HandlerTestSuite) TestProxy_RefusesBlockedPaths() {
	s.signIn()

	for _, target := range []string{
		"/console/auth/proxy/console/index.html",
		"/console/auth/proxy/gate/",
		"/console/auth/proxy/console/auth/session",
	} {
		resp := s.do(http.MethodGet, target, nil, true)
		s.Equal(http.StatusNotFound, resp.StatusCode, target)
	}
	s.Empty(s.server.apiRequests)
}

func (s *HandlerTestSuite) TestProxyAllowed() {
	s.True(s.h.proxyAllowed("/users"))
	s.True(s.h.proxyAllowed("/consoles"))
	s.False(s.h.proxyAllowed("/"))
	s.False(s.h.proxyAllowed("/console"))
	s.False(s.h.proxyAllowed("/console/auth/session"))
	s.False(s.h.proxyAllowed("/gate"))
}

func (s *HandlerTestSuite) TestProxy_ForwardsWithoutTokenWithoutSession() {
	resp := s.do(http.MethodGet, "/console/auth/proxy/users", nil, true)

	s.Equal(http.StatusCreated, resp.StatusCode)
	s.Require().Len(s.server.apiRequests, 1)
	s.Empty(s.server.apiRequests[0].Header.Get("Authorization"))
}

func (s *HandlerTestSuite) TestProxy_ExpiredSessionAsksForRefresh() {
	if !s.stateless() {
		s.T().Skip("a stateful session refreshes on the server; see TestStateful_ProxyRefreshesExpiredSession")
	}
	s.signIn()
	s.now = s.now.Add(2 * time.Hour)

	resp := s.do(http.MethodGet, "/console/auth/proxy/users", nil, true)

	s.Equal(http.StatusUnauthorized, resp.StatusCode)
	s.Equal("session_expired", decodeBody(resp)["error"])
	s.Empty(s.server.apiRequests)
}

func (s *HandlerTestSuite) TestSafeReturnTo() {
	cases := map[string]string{
		"":                           "/console",
		"/console":                   "/console",
		"/console/users?tab=1":       "/console/users?tab=1",
		"/console/../admin":          "/console",
		"/consolex":                  "/console",
		"https://evil.example.com/x": "/console",
		"//evil.example.com/console": "/console",
		"/console\\..\\x":            "/console",
		"/console/auth/signout":      "/console",
	}
	for input, want := range cases {
		s.Equal(want, s.h.safeReturnTo(input), input)
	}
}

func (s *HandlerTestSuite) TestEndedSessions_RecordsTheEndedSession() {
	s.Require().NoError(s.h.ended.end(context.Background(), "session-9"))

	s.True(s.h.ended.isEnded(context.Background(), "session-9"))
	s.Equal(1, s.store.count(providers.NamespaceConsoleEnded))
	s.Equal(time.Duration(endedSessionTTL)*time.Second, s.store.ttl(providers.NamespaceConsoleEnded, "session-9"))
}

func fakeJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, _ := json.Marshal(claims)
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}
