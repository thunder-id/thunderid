// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
)

// Session modes.
const (
	// SessionModeStateful keeps the tokens in the runtime store behind an opaque cookie.
	SessionModeStateful = "stateful"
	// SessionModeStateless keeps the tokens in encrypted cookies and nothing on the server.
	SessionModeStateless = "stateless"
)

var (
	errNoRefreshToken = errors.New("the session has no refresh token")
	errSessionGone    = errors.New("the session no longer exists")
)

// session is what the BFF holds for a signed-in browser.
type session struct {
	AccessToken  string `json:"at"`
	IDToken      string `json:"it,omitempty"`
	RefreshToken string `json:"rt,omitempty"`
	ExpiresAt    int64  `json:"exp"`
	SessionID    string `json:"sid,omitempty"`

	// key and version identify a stateful session's record, so a refresh can replace it atomically.
	key     string
	version string
}

// renewFunc runs the refresh token grant.
type renewFunc func(ctx context.Context, refreshToken string) (*tokenResponse, error)

// sessionStore keeps sessions, in cookies or on the server.
type sessionStore interface {
	// load returns the session the request carries. A session that cannot be read is cleared and
	// reported as none.
	load(w http.ResponseWriter, r *http.Request) (*session, bool)
	// peek returns the request's session without side effects: it does not count as activity, so it
	// never extends the idle timeout, and never clears a cookie.
	peek(r *http.Request) (*session, bool)
	// carries reports whether the request has a session cookie at all, live or not.
	carries(r *http.Request) bool
	// save starts a session, replacing any the request carried.
	save(w http.ResponseWriter, r *http.Request, sess *session) error
	// refresh renews the session's tokens with renew and stores the result. It fails with
	// errNoRefreshToken when the session's refresh token is not available on this request.
	refresh(w http.ResponseWriter, r *http.Request, sess *session, renew renewFunc) (*session, error)
	// destroy ends the session and clears its cookies.
	destroy(w http.ResponseWriter, r *http.Request)
}

// sessionFromTokens builds a session from a token response. A refresh response may leave out the ID
// token and the refresh token, so those carry over from the previous session, as does its sid unless
// the new ID token names one.
func sessionFromTokens(tokens *tokenResponse, previous *session, now time.Time) *session {
	sess := &session{
		AccessToken:  tokens.AccessToken,
		IDToken:      tokens.IDToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    now.Unix() + tokens.ExpiresIn,
	}
	if previous != nil {
		if sess.IDToken == "" {
			sess.IDToken = previous.IDToken
		}
		if sess.RefreshToken == "" {
			sess.RefreshToken = previous.RefreshToken
		}
		sess.SessionID = previous.SessionID
		sess.key = previous.key
	}
	if claims, err := jwt.DecodeJWTPayload(tokens.IDToken); err == nil {
		if sid := claimString(claims, "sid"); sid != "" {
			sess.SessionID = sid
		}
	}
	return sess
}
