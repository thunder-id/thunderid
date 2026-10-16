// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const sessionCookieName = "__Host-tid_console_session"

func (s *HandlerTestSuite) requireStateful() *statefulSessions {
	if s.stateless() {
		s.T().Skip("stateful sessions only")
	}
	return s.h.sessions.(*statefulSessions)
}

func (s *HandlerTestSuite) sessionKey() string {
	return recordKey(s.jar[sessionCookieName].Value)
}

// endSSOSession records the SSO session as ended, as a back-channel logout does.
func (s *HandlerTestSuite) endSSOSession(sid string) {
	s.Require().NoError(s.store.Put(context.Background(), providers.NamespaceConsoleEnded, sid, []byte("1"), 60))
}

func (s *HandlerTestSuite) signedIn() bool {
	body := decodeBody(s.do(http.MethodGet, "/console/auth/session", nil, true))
	return body["isSignedIn"] == true
}

func (s *HandlerTestSuite) TestStateful_CookieHoldsOnlyAHandle() {
	s.requireStateful()
	s.signIn()

	handle := s.jar[sessionCookieName].Value
	s.Len(handle, 43)
	s.Equal(1, s.store.count(providers.NamespaceConsoleSession))

	raw, err := s.store.Get(context.Background(), providers.NamespaceConsoleSession, recordKey(handle))
	s.Require().NoError(err)
	s.NotContains(string(raw), "access-1")
	s.NotContains(string(raw), "refresh-1")
	s.NotContains(string(raw), handle)
}

func (s *HandlerTestSuite) TestStateful_IdleSessionExpires() {
	sessions := s.requireStateful()
	sessions.idleTimeout = 600
	s.signIn()

	// Used after five minutes, so the idle timeout restarts then.
	s.now = s.now.Add(300 * time.Second)
	s.True(s.signedIn())
	s.Equal(600*time.Second, s.store.ttl(providers.NamespaceConsoleSession, s.sessionKey()))

	s.now = s.now.Add(400 * time.Second)
	s.True(s.signedIn())

	s.now = s.now.Add(601 * time.Second)
	s.False(s.signedIn())
	s.NotContains(s.jar, sessionCookieName)
}

func (s *HandlerTestSuite) TestStateful_ProxyRefreshesExpiredSession() {
	s.requireStateful()
	s.signIn()
	s.now = s.now.Add(2 * time.Hour)

	resp := s.do(http.MethodGet, "/console/auth/proxy/users", nil, true)

	s.Equal(http.StatusCreated, resp.StatusCode)
	s.Require().Len(s.server.tokenRequests, 2)
	s.Equal("refresh-1", s.server.tokenRequests[1].Get("refresh_token"))
	s.Equal("Bearer access-2", s.server.apiRequests[0].Header.Get("Authorization"))
}

func (s *HandlerTestSuite) TestStateful_ConcurrentRefreshWaitsForTheWinner() {
	sessions := s.requireStateful()
	s.signIn()
	s.now = s.now.Add(2 * time.Hour)
	key := s.sessionKey()

	// Another request already holds the refresh lock, and finishes its refresh a moment later.
	locked, err := s.store.PutIfNotExists(context.Background(), providers.NamespaceConsoleRefreshLock, key,
		[]byte("1"), refreshLockTTL)
	s.Require().NoError(err)
	s.Require().True(locked)
	go func() {
		time.Sleep(30 * time.Millisecond)
		raw, _ := s.store.Get(context.Background(), providers.NamespaceConsoleSession, key)
		var record sessionRecord
		_ = json.Unmarshal(raw, &record)
		req, _ := http.NewRequest(http.MethodGet, testPublicURL, nil)
		value, _ := sessions.record(req, &session{
			AccessToken: "access-from-winner", ExpiresAt: s.now.Unix() + 3600, SessionID: "session-1",
		}, "winner")
		_, _ = s.store.CompareFieldAndSwap(context.Background(), providers.NamespaceConsoleSession, key,
			recordVersionField, record.Version, value)
	}()

	resp := s.do(http.MethodGet, "/console/auth/proxy/users", nil, true)

	s.Equal(http.StatusCreated, resp.StatusCode)
	s.Len(s.server.tokenRequests, 1, "the waiting request must not run its own refresh")
	s.Equal("Bearer access-from-winner", s.server.apiRequests[0].Header.Get("Authorization"))
}

func (s *HandlerTestSuite) TestStateful_SignOutDeletesTheRecord() {
	s.requireStateful()
	s.signIn()

	s.do(http.MethodPost, "/console/auth/signout", nil, true)

	s.Equal(0, s.store.count(providers.NamespaceConsoleSession))
}

func (s *HandlerTestSuite) TestStateful_DeletedRecordSignsOut() {
	s.requireStateful()
	s.signIn()
	s.Require().NoError(s.store.Delete(context.Background(), providers.NamespaceConsoleSession, s.sessionKey()))

	s.False(s.signedIn())
	s.NotContains(s.jar, sessionCookieName)
}

func (s *HandlerTestSuite) TestStateful_SignInAgainReplacesTheRecord() {
	s.requireStateful()
	s.signIn()
	s.signIn()

	s.Equal(1, s.store.count(providers.NamespaceConsoleSession))
	resp := s.do(http.MethodGet, "/console/auth/session", nil, true)
	body, _ := io.ReadAll(resp.Body)
	s.Contains(string(body), `"isSignedIn":true`)
}

func (s *HandlerTestSuite) checkSession() bool {
	resp := s.do(http.MethodGet, "/console/auth/checksession", nil, true)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	return decodeBody(resp)["isSignedIn"] == true
}

func (s *HandlerTestSuite) TestCheckSession_ReportsTheSession() {
	s.False(s.checkSession())

	s.signIn()
	s.True(s.checkSession())

	s.endSSOSession("session-1")
	s.False(s.checkSession())
}

func (s *HandlerTestSuite) TestCheckSession_NeitherRefreshesNorKeepsTheSessionAlive() {
	s.signIn()
	s.now = s.now.Add(2 * time.Hour)

	s.True(s.checkSession(), "an expired access token is refreshed by the next real request")
	s.Len(s.server.tokenRequests, 1)

	if !s.stateless() {
		before := s.store.ttl(providers.NamespaceConsoleSession, s.sessionKey())
		s.now = s.now.Add(5 * time.Minute)
		s.True(s.checkSession())
		s.Equal(before-5*time.Minute, s.store.ttl(providers.NamespaceConsoleSession, s.sessionKey()))
	}
}

func (s *HandlerTestSuite) TestProxy_EndedSessionAsksToSignInAgain() {
	s.signIn()
	s.endSSOSession("session-1")

	resp := s.do(http.MethodGet, "/console/auth/proxy/users", nil, true)

	s.Equal(http.StatusUnauthorized, resp.StatusCode)
	s.Equal("login_required", decodeBody(resp)["error"])
	s.Empty(s.server.apiRequests)
	s.NotContains(s.jar, sessionCookieName)
}

func (s *HandlerTestSuite) TestProxy_FailedRefreshAsksToSignInAgain() {
	s.requireStateful()
	s.signIn()
	s.server.refreshFails = true
	s.now = s.now.Add(2 * time.Hour)

	resp := s.do(http.MethodGet, "/console/auth/proxy/users", nil, true)

	s.Equal(http.StatusUnauthorized, resp.StatusCode)
	s.Equal("login_required", decodeBody(resp)["error"])
	s.Empty(s.server.apiRequests)
}

// indexEntry returns the record key the test session's index entry points at.
func (s *HandlerTestSuite) indexEntry() string {
	value, err := s.store.Get(context.Background(), providers.NamespaceConsoleSessionByID, "session-1")
	if err != nil {
		return ""
	}
	return string(value)
}

func (s *HandlerTestSuite) TestStateful_IndexFollowsTheSession() {
	sessions := s.requireStateful()
	sessions.idleTimeout = 600
	s.signIn()

	s.Equal(s.sessionKey(), s.indexEntry())
	s.Equal(600*time.Second, s.store.ttl(providers.NamespaceConsoleSessionByID, "session-1"))

	// Used after five minutes, so the index entry's timeout restarts with the record's.
	s.now = s.now.Add(300 * time.Second)
	s.True(s.signedIn())
	s.Equal(600*time.Second, s.store.ttl(providers.NamespaceConsoleSessionByID, "session-1"))

	s.do(http.MethodPost, "/console/auth/signout", nil, true)
	s.Empty(s.indexEntry())
}

func (s *HandlerTestSuite) TestStateful_SignOutKeepsANewerIndexEntry() {
	s.requireStateful()
	s.signIn()
	s.Require().NoError(s.store.Put(context.Background(), providers.NamespaceConsoleSessionByID, "session-1",
		[]byte("newer"), 60))

	s.do(http.MethodPost, "/console/auth/signout", nil, true)

	s.Equal("newer", s.indexEntry())
}

func (s *HandlerTestSuite) TestStateful_ForgetWithoutIndexEntry() {
	sessions := s.requireStateful()

	s.NoError(sessions.forget(context.Background(), "unknown"))
}
