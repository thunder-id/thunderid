// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
)

// statelessSessions keeps the session in encrypted cookies. The refresh token has its own cookie,
// scoped to the refresh endpoint, so only a refresh request carries it.
type statelessSessions struct {
	sealer        sealer
	sessionCookie cookieSpec
	refreshCookie cookieSpec
	now           func() time.Time
}

// refreshCookiePayload is the refresh cookie's payload.
type refreshCookiePayload struct {
	RefreshToken string `json:"rt"`
}

func (s *statelessSessions) load(w http.ResponseWriter, r *http.Request) (*session, bool) {
	if !s.carries(r) {
		return nil, false
	}
	sess, ok := s.peek(r)
	if !ok {
		s.destroy(w, r)
		return nil, false
	}
	return sess, true
}

func (s *statelessSessions) carries(r *http.Request) bool {
	_, ok := s.sessionCookie.read(r)
	return ok
}

func (s *statelessSessions) peek(r *http.Request) (*session, bool) {
	ctx := r.Context()
	sealed, ok := s.sessionCookie.read(r)
	if !ok {
		return nil, false
	}
	var sess session
	if err := s.sealer.open(ctx, sealed, &sess); err != nil || sess.AccessToken == "" {
		return nil, false
	}

	var refresh refreshCookiePayload
	if sealed, ok := s.refreshCookie.read(r); ok && s.sealer.open(ctx, sealed, &refresh) == nil {
		sess.RefreshToken = refresh.RefreshToken
	}
	return &sess, true
}

func (s *statelessSessions) save(w http.ResponseWriter, r *http.Request, sess *session) error {
	ctx := r.Context()
	cookieSession := *sess
	cookieSession.RefreshToken = ""
	sealed, err := s.sealer.seal(ctx, cookieSession)
	if err != nil {
		return err
	}
	s.sessionCookie.write(w, r, sealed, 0)

	if sess.RefreshToken == "" {
		s.refreshCookie.clear(w, r)
		return nil
	}
	refresh, err := s.sealer.seal(ctx, refreshCookiePayload{RefreshToken: sess.RefreshToken})
	if err != nil {
		return err
	}
	s.refreshCookie.write(w, r, refresh, s.refreshMaxAge(sess.RefreshToken))
	return nil
}

func (s *statelessSessions) refresh(w http.ResponseWriter, r *http.Request, sess *session,
	renew renewFunc) (*session, error) {
	if sess.RefreshToken == "" {
		return nil, errNoRefreshToken
	}
	tokens, err := renew(r.Context(), sess.RefreshToken)
	if err != nil {
		return nil, err
	}
	renewed := sessionFromTokens(tokens, sess, s.now())
	if err := s.save(w, r, renewed); err != nil {
		return nil, err
	}
	return renewed, nil
}

func (s *statelessSessions) destroy(w http.ResponseWriter, r *http.Request) {
	s.sessionCookie.clear(w, r)
	s.refreshCookie.clear(w, r)
}

// refreshMaxAge keeps the refresh cookie as long as the refresh token is valid. An opaque refresh
// token, or one without an expiry, lives for the browser session.
func (s *statelessSessions) refreshMaxAge(refreshToken string) int {
	claims, err := jwt.DecodeJWTPayload(refreshToken)
	if err != nil {
		return 0
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return 0
	}
	if remaining := int(int64(exp) - s.now().Unix()); remaining > 0 {
		return remaining
	}
	return 0
}
