// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	// maxLogoutRequestBytes bounds a back-channel logout request body.
	maxLogoutRequestBytes = 16 << 10
	// backchannelLogoutEvent is the event a logout token carries (OIDC Back-Channel Logout 1.0, 2.4).
	backchannelLogoutEvent = "http://schemas.openid.net/event/backchannel-logout"
)

// tokenVerifier checks a JWT's signature against the authorization server's keys, and its issuer,
// audience and lifetime.
type tokenVerifier interface {
	VerifyJWT(ctx context.Context, jwtToken string, expectedAud, expectedIss string) *tidcommon.ServiceError
}

// errInvalidLogoutToken marks a logout token the endpoint refuses. Its message is the error description.
type errInvalidLogoutToken string

func (e errInvalidLogoutToken) Error() string { return string(e) }

// handleBackchannelLogout receives a logout token from the authorization server and ends the SSO
// session it names (OIDC Back-Channel Logout 1.0). The server calls it, not a browser, so it carries
// no CSRF check: the signed token is the authorization.
func (h *handler) handleBackchannelLogout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Cache-Control", "no-store")

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		writeLogoutError(w, "the request must be form encoded")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxLogoutRequestBytes)
	if err := r.ParseForm(); err != nil {
		writeLogoutError(w, "the request body could not be read")
		return
	}
	token := r.PostForm.Get("logout_token")
	if token == "" {
		writeLogoutError(w, "logout_token is required")
		return
	}

	md, err := h.oidc.getMetadata(ctx)
	if err != nil || md.Issuer == "" {
		h.logger.Error(ctx, "Failed to resolve the provider issuer", log.Error(err))
		writeError(w, http.StatusInternalServerError, "server_error")
		return
	}

	claims, err := h.validateLogoutToken(ctx, token, md.Issuer)
	if err != nil {
		var invalid errInvalidLogoutToken
		if errors.As(err, &invalid) {
			h.logger.Debug(ctx, "Refused a logout token", log.String("reason", invalid.Error()))
			writeLogoutError(w, invalid.Error())
			return
		}
		h.logger.Error(ctx, "Failed to check a logout token", log.Error(err))
		writeError(w, http.StatusInternalServerError, "server_error")
		return
	}

	if err := h.ended.end(ctx, claimString(claims, "sid")); err != nil {
		h.logger.Error(ctx, "Failed to end a session", log.Error(err))
		writeError(w, http.StatusInternalServerError, "server_error")
		return
	}
	w.WriteHeader(http.StatusOK)
}

// validateLogoutToken validates a logout token as OIDC Back-Channel Logout 1.0, section 2.6,
// requires, and returns its claims. It also refuses a token it has seen before.
func (h *handler) validateLogoutToken(ctx context.Context, token, issuer string) (map[string]any, error) {
	if strings.Count(token, ".") != 2 {
		return nil, errInvalidLogoutToken("logout_token must be a signed JWT")
	}
	header, claims, err := jwt.DecodeJWT(token)
	if err != nil {
		return nil, errInvalidLogoutToken("logout_token could not be decoded")
	}
	if svcErr := h.verifier.VerifyJWT(ctx, token, h.cfg.ClientID, issuer); svcErr != nil {
		return nil, errInvalidLogoutToken("logout_token failed validation")
	}
	if _, ok := header["typ"]; ok && !strings.EqualFold(claimString(header, "typ"), jwt.TokenTypeLogout) {
		return nil, errInvalidLogoutToken("logout_token has the wrong type")
	}
	if _, ok := claims["iat"].(float64); !ok {
		return nil, errInvalidLogoutToken("logout_token has no iat claim")
	}
	events, _ := claims["events"].(map[string]any)
	if _, ok := events[backchannelLogoutEvent].(map[string]any); !ok {
		return nil, errInvalidLogoutToken("logout_token has no back-channel logout event")
	}
	if _, ok := claims["nonce"]; ok {
		return nil, errInvalidLogoutToken("logout_token must not carry a nonce")
	}
	// Sessions are tracked by SSO session, so a token that names only the subject cannot be matched.
	if claimString(claims, "sid") == "" {
		return nil, errInvalidLogoutToken("logout_token has no sid claim")
	}

	jti := claimString(claims, "jti")
	if jti == "" {
		return nil, errInvalidLogoutToken("logout_token has no jti claim")
	}
	ttl := int64(1)
	if exp, ok := claims["exp"].(float64); ok {
		ttl = max(int64(exp)-h.now().Unix(), 1)
	}
	fresh, err := h.ended.store.PutIfNotExists(ctx, providers.NamespaceConsoleLogoutToken, jti, []byte("1"), ttl)
	if err != nil {
		return nil, err
	}
	if !fresh {
		return nil, errInvalidLogoutToken("logout_token was already used")
	}
	return claims, nil
}

func writeLogoutError(w http.ResponseWriter, description string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{
		"error":             "invalid_request",
		"error_description": description,
	})
}
