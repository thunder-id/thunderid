// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocationcache

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/revocation"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// EventsPath is the route revocation events are pushed to. The security layer lists it as public:
// the event token is the credential.
const EventsPath = "/revocation/events"

// maxEventBodyBytes bounds a pushed event token, which carries one event.
const maxEventBodyBytes = 16 << 10

// Error codes of a rejected push (RFC 8935 §2.4).
const (
	pushErrInvalidRequest       = "invalid_request"
	pushErrAuthenticationFailed = "authentication_failed"
)

// eventReceiver accepts revocation events pushed by the trusted issuer and applies them to the cache.
// An event token is accepted only when its signature verifies against the issuer's JWKS and its iss and
// aud match the trusted issuer configuration. Events only ever add to the deny list, so a replayed
// token changes nothing.
type eventReceiver struct {
	issuer     TrustedIssuer
	jwtService jwt.JWTServiceInterface
	cache      *revokedCache
	logger     *log.Logger
}

// newEventReceiver creates a receiver that applies verified events to cache.
func newEventReceiver(issuer TrustedIssuer, jwtService jwt.JWTServiceInterface, cache *revokedCache) *eventReceiver {
	return &eventReceiver{
		issuer:     issuer,
		jwtService: jwtService,
		cache:      cache,
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RevocationEventReceiver")),
	}
}

// pushErrorResponse is the body of a rejected push (RFC 8935 §2.4).
type pushErrorResponse struct {
	Err         string `json:"err"`
	Description string `json:"description"`
}

// handlePush verifies one pushed event token and applies its event. It answers 202 when the event was
// accepted, including one this server has no use for, and 400 when the token is not acceptable.
func (r *eventReceiver) handlePush(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if mediaType, _, err := mime.ParseMediaType(req.Header.Get(serverconst.ContentTypeHeaderName)); err != nil ||
		mediaType != serverconst.ContentTypeSecurityEventToken {
		writePushError(w, pushErrInvalidRequest, "Content-Type must be "+serverconst.ContentTypeSecurityEventToken)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, req.Body, maxEventBodyBytes))
	if err != nil {
		writePushError(w, pushErrInvalidRequest, "The request body could not be read")
		return
	}
	token := strings.TrimSpace(string(body))

	header, err := jwt.DecodeJWTHeader(token)
	if err != nil {
		writePushError(w, pushErrInvalidRequest, "The body is not a JWT")
		return
	}
	if typ, _ := header["typ"].(string); !strings.EqualFold(typ, revocation.SecurityEventTokenType) {
		writePushError(w, pushErrInvalidRequest, "The token is not a security event token")
		return
	}
	if svcErr := r.jwtService.VerifyJWTWithJWKS(ctx, token, r.issuer.JWKSURL, r.issuer.Audience,
		r.issuer.Issuer); svcErr != nil {
		writePushError(w, pushErrAuthenticationFailed, "The token could not be verified")
		return
	}

	claims, err := jwt.DecodeJWTPayload(token)
	if err != nil {
		writePushError(w, pushErrInvalidRequest, "The token payload could not be read")
		return
	}
	event, err := revocation.ParseEvent(claims)
	switch {
	case errors.Is(err, revocation.ErrUnsupportedEventType):
		r.logger.Debug(ctx, "Ignored a revocation push carrying no supported event type")
	case err != nil:
		writePushError(w, pushErrInvalidRequest, "The event is malformed")
		return
	default:
		r.cache.apply(event)
	}
	w.WriteHeader(http.StatusAccepted)
}

// writePushError writes the 400 response of a rejected push.
func writePushError(w http.ResponseWriter, code, description string) {
	w.Header().Set(serverconst.ContentTypeHeaderName, serverconst.ContentTypeJSON)
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(pushErrorResponse{Err: code, Description: description})
}
