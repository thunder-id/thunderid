// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"
)

// cookieNamePrefix prefixes every per-flow SSO cookie name.
const cookieNamePrefix = "tid_sso_"

// cookieName derives the per-flow SSO cookie name from the flow ID. Each flow gets its
// own cookie so sessions from different flows do not clobber each other's handle. The
// flow ID is hashed so the raw ID is not exposed in the cookie name and the name stays
// within the cookie-token character set.
func cookieName(flowID string) string {
	sum := sha256.Sum256([]byte(flowID))
	return cookieNamePrefix + hex.EncodeToString(sum[:])[:16]
}

// InboundHandle carries the request-scoped SSO handles read from a transport. The source decides
// how a handle is keyed and carried; callers only select a handle by flow ID. It is transient: it
// must never be persisted with the flow context.
type InboundHandle interface {
	// HandleFor returns the SSO handle carried for the given flow, or "" when none is present.
	HandleFor(flowID string) string
}

// staticInbound carries a single handle for a single flow.
type staticInbound struct {
	flowID string
	handle string
}

// NewStaticInbound creates an InboundHandle that carries handle for flowID only. It lets a caller
// that already holds the handle supply it without going through a request transport.
func NewStaticInbound(flowID, handle string) InboundHandle {
	return staticInbound{flowID: flowID, handle: handle}
}

// HandleFor returns the handle when flowID matches the carried flow, or "" otherwise.
func (s staticInbound) HandleFor(flowID string) string {
	if flowID != s.flowID {
		return ""
	}
	return s.handle
}

type inboundCtxKey struct{}

// WithInbound stores the inbound SSO transport inputs on the context for the flow service
// to consume once it has resolved the flow ID.
func WithInbound(ctx context.Context, ih InboundHandle) context.Context {
	return context.WithValue(ctx, inboundCtxKey{}, ih)
}

// InboundFrom retrieves the inbound SSO transport inputs from the context. It reports false when
// nothing was attached or a nil InboundHandle was attached.
func InboundFrom(ctx context.Context) (InboundHandle, bool) {
	ih, ok := ctx.Value(inboundCtxKey{}).(InboundHandle)
	return ih, ok
}

// Exchange is one request and its response as the handle transports see them. Each field is one
// place a handle can be carried; a transport uses the fields it needs and ignores the rest. A new
// kind of carrier adds a field here, so existing transports and the HandleTransport methods do not
// change.
type Exchange struct {
	// Request is the inbound HTTP request.
	Request *http.Request
	// Response is the HTTP response being built. It is nil for an endpoint that only reads the handle.
	Response http.ResponseWriter
}

// HandleTransport abstracts how the session handle is read from a request and emitted onto a
// response. The transport owns how a flow's handle is keyed and carried; callers pass only the
// exchange and the flow ID.
type HandleTransport interface {
	// Read extracts the inbound SSO transport inputs from the exchange.
	Read(x *Exchange) InboundHandle
	// Write emits the handle for the given flow onto the exchange, valid for ttl.
	Write(x *Exchange, flowID, handle string, ttl time.Duration)
	// Clear removes the handle for the given flow from the exchange. Seam for logout / session end.
	Clear(x *Exchange, flowID string)
}

// TransportConfig holds the deployment settings the handle transports need.
type TransportConfig struct {
	// SecureCookies marks the SSO cookie Secure; it should be true behind TLS.
	SecureCookies bool
}

// NewHandleTransport creates the HandleTransport every endpoint uses. It is the single place the
// supported transports are assembled.
func NewHandleTransport(cfg TransportConfig) HandleTransport {
	return newCookieTransport(cfg.SecureCookies)
}

// cookieTransport carries the handle as an HTTP cookie.
type cookieTransport struct {
	secure bool
}

// newCookieTransport creates a cookie-backed HandleTransport. secure controls the Secure
// attribute; it should be true behind TLS.
func newCookieTransport(secure bool) HandleTransport {
	return &cookieTransport{secure: secure}
}

// cookieInbound maps every inbound cookie name to its value. The per-flow handle is selected
// from this set by name, because the flow ID is not known when the transport reads the request.
type cookieInbound map[string]string

// HandleFor returns the value of the per-flow SSO cookie, or "" when it is absent.
func (ci cookieInbound) HandleFor(flowID string) string {
	return ci[cookieName(flowID)]
}

// Read collects all inbound cookies from the request.
func (c *cookieTransport) Read(x *Exchange) InboundHandle {
	cookies := make(cookieInbound)
	if x.Request == nil {
		return cookies
	}
	for _, ck := range x.Request.Cookies() {
		cookies[ck.Name] = ck.Value
	}
	return cookies
}

// Write sets the per-flow handle cookie on the response.
func (c *cookieTransport) Write(x *Exchange, flowID, handle string, ttl time.Duration) {
	if x.Response == nil {
		return
	}
	http.SetCookie(x.Response, &http.Cookie{
		Name:     cookieName(flowID),
		Value:    handle,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   c.secure,
		// SameSite=Lax suffices for same-site SSO. Cross-site SSO would require
		// SameSite=None with Secure.
		// TODO(sso): make SameSite configurable for cross-site deployments.
		SameSite: http.SameSiteLaxMode,
	})
}

// Clear expires the per-flow handle cookie on the response.
func (c *cookieTransport) Clear(x *Exchange, flowID string) {
	if x.Response == nil {
		return
	}
	http.SetCookie(x.Response, &http.Cookie{
		Name:     cookieName(flowID),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
	})
}
