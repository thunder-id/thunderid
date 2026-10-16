// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	headerRequestedWith = "X-Requested-With"
	transactionMaxAge   = 600
)

// protocolClaims are the ID token claims that describe the token rather than the user. They are left
// out of the user returned by the session endpoint.
var protocolClaims = []string{
	"iss", "aud", "exp", "iat", "nbf", "auth_time", "nonce", "at_hash", "c_hash", "sid", "azp", "jti",
}

// transactionCookie carries a sign-in from /signin to /callback.
type transactionCookie struct {
	State    string `json:"state"`
	Verifier string `json:"verifier"`
	Nonce    string `json:"nonce"`
	ReturnTo string `json:"return_to"`
	// RedirectURI is the redirect URI the authorization request carried, which the code exchange
	// has to repeat.
	RedirectURI string `json:"redirect_uri"`
}

// handler serves the BFF endpoints.
type handler struct {
	cfg      Config
	oidc     *oidcClient
	upstream *http.Client
	sealer   sealer
	sessions sessionStore
	ended    *endedSessions
	verifier tokenVerifier
	clients  providers.ActorProvider
	logger   *log.Logger

	transactionCookie cookieSpec

	now func() time.Time
}

// handleSignIn starts the authorization code flow.
func (h *handler) handleSignIn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	md, err := h.oidc.getMetadata(ctx)
	if err != nil {
		h.logger.Error(ctx, "Failed to resolve the provider metadata", log.Error(err))
		writeError(w, http.StatusBadGateway, "server_error")
		return
	}

	txn := transactionCookie{
		State:       randomToken(),
		Verifier:    randomToken(),
		Nonce:       randomToken(),
		ReturnTo:    h.safeReturnTo(r.URL.Query().Get("returnTo")),
		RedirectURI: h.externalBaseURL(r) + h.cfg.BasePath + "/callback",
	}
	sealed, err := h.sealer.seal(ctx, txn)
	if err != nil {
		h.logger.Error(ctx, "Failed to seal the sign-in transaction", log.Error(err))
		writeError(w, http.StatusInternalServerError, "server_error")
		return
	}
	h.transactionCookie.write(w, r, sealed, transactionMaxAge)

	challenge := sha256.Sum256([]byte(txn.Verifier))
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {h.cfg.ClientID},
		"redirect_uri":          {txn.RedirectURI},
		"scope":                 {strings.Join(h.cfg.Scopes, " ")},
		"state":                 {txn.State},
		"nonce":                 {txn.Nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
	}
	if h.cfg.Resource != "" {
		params.Set("resource", h.cfg.Resource)
	}
	http.Redirect(w, r, appendQuery(md.AuthorizationEndpoint, params), http.StatusFound)
}

// handleCallback finishes the authorization code flow and starts the session.
func (h *handler) handleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()

	var txn transactionCookie
	sealed, ok := h.transactionCookie.read(r)
	h.transactionCookie.clear(w, r)
	if !ok || h.sealer.open(ctx, sealed, &txn) != nil {
		h.redirectWithError(w, r, h.cfg.AppPath, "invalid_request")
		return
	}
	if subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(txn.State)) != 1 {
		h.redirectWithError(w, r, txn.ReturnTo, "invalid_request")
		return
	}
	if errCode := query.Get("error"); errCode != "" {
		h.redirectWithError(w, r, txn.ReturnTo, errCode)
		return
	}

	tokens, err := h.oidc.exchangeCode(ctx, query.Get("code"), txn.Verifier, txn.RedirectURI)
	if err != nil {
		h.logger.Debug(ctx, "Authorization code exchange failed", log.Error(err))
		h.redirectWithError(w, r, txn.ReturnTo, "access_denied")
		return
	}
	claims, err := jwt.DecodeJWTPayload(tokens.IDToken)
	if err != nil || claimString(claims, "nonce") != txn.Nonce {
		h.redirectWithError(w, r, txn.ReturnTo, "invalid_request")
		return
	}

	if err := h.sessions.save(w, r, sessionFromTokens(tokens, nil, h.now())); err != nil {
		h.logger.Error(ctx, "Failed to start the session", log.Error(err))
		h.redirectWithError(w, r, txn.ReturnTo, "server_error")
		return
	}
	http.Redirect(w, r, txn.ReturnTo, http.StatusFound)
}

// handleSession reports whether the browser is signed in, and who as.
func (h *handler) handleSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.loadSession(w, r)
	if ok {
		sess, ok = h.freshSession(w, r, sess)
	}
	if !ok {
		if sess != nil {
			writeError(w, http.StatusUnauthorized, "session_expired")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"isSignedIn": false})
		return
	}

	user := map[string]any{}
	if claims, err := jwt.DecodeJWTPayload(sess.IDToken); err == nil {
		user = claims
		for _, claim := range protocolClaims {
			delete(user, claim)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"isSignedIn": true,
		"user":       user,
		"expiresAt":  sess.ExpiresAt,
	})
}

// handleCheckSession reports whether the browser still has a live session, for clients that poll.
// It is read-only: it neither refreshes the tokens nor counts as activity, so polling does not keep an
// idle session alive. A session whose access token has expired still counts as live, because the next
// real request refreshes it.
func (h *handler) handleCheckSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := h.sessions.peek(r)
	signedIn := ok && !h.ended.isEnded(r.Context(), sess.SessionID)
	writeJSON(w, http.StatusOK, map[string]bool{"isSignedIn": signedIn})
}

// handleRefresh renews the session's tokens.
func (h *handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess, ok := h.loadSession(w, r)
	if !ok {
		h.sessions.destroy(w, r)
		writeError(w, http.StatusUnauthorized, "login_required")
		return
	}
	if _, err := h.sessions.refresh(w, r, sess, h.oidc.refresh); err != nil {
		h.logger.Debug(ctx, "Session refresh failed", log.Error(err))
		h.sessions.destroy(w, r)
		writeError(w, http.StatusUnauthorized, "login_required")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSignOut ends the BFF session and returns the end-session URL for the browser to visit.
func (h *handler) handleSignOut(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idTokenHint := ""
	if sess, ok := h.sessions.load(w, r); ok {
		idTokenHint = sess.IDToken
	}
	h.sessions.destroy(w, r)

	redirectURL := h.cfg.AppPath
	md, err := h.oidc.getMetadata(ctx)
	if err == nil && md.EndSessionEndpoint != "" {
		params := url.Values{
			"client_id":                {h.cfg.ClientID},
			"post_logout_redirect_uri": {h.externalBaseURL(r) + h.cfg.AppPath},
		}
		if idTokenHint != "" {
			params.Set("id_token_hint", idTokenHint)
		}
		redirectURL = appendQuery(md.EndSessionEndpoint, params)
	} else if err != nil {
		h.logger.Error(ctx, "Failed to resolve the provider metadata", log.Error(err))
	}
	writeJSON(w, http.StatusOK, map[string]string{"redirectUrl": redirectURL})
}

// handleProxy forwards an API call upstream with the session's access token. Without a session it
// forwards the call without a token, so public APIs work before sign-in and protected ones answer
// with their own 401. A request whose session cookie names a session that has ended, or that could
// not be refreshed, gets 401 login_required instead, so the client learns it was signed out.
func (h *handler) handleProxy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	target := path.Clean("/" + r.PathValue("path"))
	if !h.proxyAllowed(target) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}

	carried := h.sessions.carries(r)
	sess, hasSession := h.loadSession(w, r)
	if hasSession {
		var fresh bool
		if sess, fresh = h.freshSession(w, r, sess); !fresh && sess != nil {
			writeError(w, http.StatusUnauthorized, "session_expired")
			return
		}
		hasSession = fresh
	}
	if carried && !hasSession {
		writeError(w, http.StatusUnauthorized, "login_required")
		return
	}

	upstreamURL := h.cfg.PublicURL + target
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, upstreamURL, r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	req.ContentLength = r.ContentLength
	for _, name := range []string{"Content-Type", "Accept", "Accept-Language"} {
		if value := r.Header.Get(name); value != "" {
			req.Header.Set(name, value)
		}
	}
	if hasSession {
		req.Header.Set("Authorization", "Bearer "+sess.AccessToken)
	}

	resp, err := h.upstream.Do(req)
	if err != nil {
		h.logger.Error(ctx, "Upstream request failed", log.Error(err))
		writeError(w, http.StatusBadGateway, "server_error")
		return
	}
	defer func() { _ = resp.Body.Close() }()

	for _, name := range []string{"Content-Type", "Content-Disposition", "Location"} {
		if value := resp.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		h.logger.Debug(ctx, "Failed to copy the upstream response", log.Error(err))
	}
}

// requireSameOrigin refuses a request a cross-site page could have sent. A custom header cannot be
// set cross-origin without a CORS preflight, which the BFF never answers.
func (h *handler) requireSameOrigin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(headerRequestedWith) == "" {
			writeError(w, http.StatusForbidden, "csrf_check_failed")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && !h.isTrustedOrigin(r.Context(), origin) {
			writeError(w, http.StatusForbidden, "csrf_check_failed")
			return
		}
		next(w, r)
	}
}

// loadSession returns the request's session. A session whose SSO session has ended is destroyed and
// reported as none.
func (h *handler) loadSession(w http.ResponseWriter, r *http.Request) (*session, bool) {
	sess, ok := h.sessions.load(w, r)
	if !ok {
		return nil, false
	}
	if h.ended.isEnded(r.Context(), sess.SessionID) {
		h.sessions.destroy(w, r)
		return nil, false
	}
	return sess, true
}

// freshSession returns the session with a live access token, refreshing it on the server when the
// refresh token is available here. It returns:
//   - the session and true when the access token is live;
//   - the session and false when it has expired and the client has to call the refresh endpoint,
//     which is how a stateless session refreshes, because only that endpoint receives its refresh token;
//   - nil and false when the refresh failed and the session is gone.
func (h *handler) freshSession(w http.ResponseWriter, r *http.Request, sess *session) (*session, bool) {
	if !h.expired(sess) {
		return sess, true
	}
	renewed, err := h.sessions.refresh(w, r, sess, h.oidc.refresh)
	if errors.Is(err, errNoRefreshToken) {
		return sess, false
	}
	if err != nil {
		h.logger.Debug(r.Context(), "Session refresh failed", log.Error(err))
		h.sessions.destroy(w, r)
		return nil, false
	}
	return renewed, true
}

func (h *handler) expired(sess *session) bool {
	return h.now().Unix() >= sess.ExpiresAt
}

func (h *handler) proxyAllowed(target string) bool {
	if target == "/" || strings.HasPrefix(target+"/", h.cfg.BasePath+"/") {
		return false
	}
	for _, prefix := range h.cfg.BlockedProxyPrefixes {
		if strings.HasPrefix(target+"/", prefix) {
			return false
		}
	}
	return true
}

// safeReturnTo accepts a path on the app's own base path and falls back to the app root otherwise,
// so the BFF cannot be used as an open redirect.
func (h *handler) safeReturnTo(returnTo string) string {
	if returnTo == "" || strings.ContainsAny(returnTo, "\\\r\n") {
		return h.cfg.AppPath
	}
	u, err := url.Parse(returnTo)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil {
		return h.cfg.AppPath
	}
	cleaned := path.Clean(u.Path)
	if cleaned != h.cfg.AppPath && !strings.HasPrefix(cleaned, h.cfg.AppPath+"/") {
		return h.cfg.AppPath
	}
	if strings.HasPrefix(cleaned+"/", h.cfg.BasePath+"/") {
		return h.cfg.AppPath
	}
	u.Path = cleaned
	return u.String()
}

func (h *handler) redirectWithError(w http.ResponseWriter, r *http.Request, target, code string) {
	http.Redirect(w, r, appendQuery(target, url.Values{"error": {code}}), http.StatusFound)
}

func appendQuery(rawURL string, params url.Values) string {
	separator := "?"
	if strings.Contains(rawURL, "?") {
		separator = "&"
	}
	return rawURL + separator + params.Encode()
}

func claimString(claims map[string]any, name string) string {
	value, _ := claims[name].(string)
	return value
}

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
