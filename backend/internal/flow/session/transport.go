/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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

// CookieName derives the per-flow SSO cookie name from the flow ID. Each flow gets its
// own cookie so sessions from different flows do not clobber each other's handle. The
// flow ID is hashed so the raw ID is not exposed in the cookie name and the name stays
// within the cookie-token character set.
func CookieName(flowID string) string {
	sum := sha256.Sum256([]byte(flowID))
	return cookieNamePrefix + hex.EncodeToString(sum[:])[:16]
}

// InboundHandle holds the request-scoped SSO transport inputs read from a transport. It is
// transient: it must never be persisted with the flow context.
type InboundHandle struct {
	// Cookies maps every inbound cookie name to its value. The per-flow handle is selected
	// from this set by name, because the flow ID is not known when the transport reads the request.
	Cookies map[string]string
}

// HandleFor returns the SSO handle carried for the given flow, or "" when none is present.
func (ih InboundHandle) HandleFor(flowID string) string {
	if ih.Cookies == nil {
		return ""
	}
	return ih.Cookies[CookieName(flowID)]
}

type inboundCtxKey struct{}

// WithInbound stores the inbound SSO transport inputs on the context for the flow service
// to consume once it has resolved the flow ID.
func WithInbound(ctx context.Context, ih InboundHandle) context.Context {
	return context.WithValue(ctx, inboundCtxKey{}, ih)
}

// InboundFrom retrieves the inbound SSO transport inputs from the context.
func InboundFrom(ctx context.Context) (InboundHandle, bool) {
	ih, ok := ctx.Value(inboundCtxKey{}).(InboundHandle)
	return ih, ok
}

// HandleTransport abstracts how the session handle is read from a request and emitted onto a
// response. A cookie is one transport; keeping this behind an interface lets a non-cookie
// transport plug in later.
type HandleTransport interface {
	// Read extracts the inbound SSO transport inputs from a request.
	Read(r *http.Request) InboundHandle
	// Write emits the handle to the response under the given (per-flow) cookie name, valid
	// for ttl.
	Write(w http.ResponseWriter, cookieName, handle string, ttl time.Duration)
	// Clear removes the handle from the response. Seam for logout / session end.
	Clear(w http.ResponseWriter, cookieName string)
}

// cookieTransport carries the handle as an HTTP cookie.
type cookieTransport struct {
	secure bool
}

// NewCookieTransport creates a cookie-backed HandleTransport. secure controls the Secure
// attribute; it should be true behind TLS.
func NewCookieTransport(secure bool) HandleTransport {
	return &cookieTransport{secure: secure}
}

// Read collects all inbound cookies from the request.
func (c *cookieTransport) Read(r *http.Request) InboundHandle {
	cookies := make(map[string]string)
	for _, ck := range r.Cookies() {
		cookies[ck.Name] = ck.Value
	}
	return InboundHandle{
		Cookies: cookies,
	}
}

// Write sets the per-flow handle cookie on the response.
func (c *cookieTransport) Write(w http.ResponseWriter, cookieName, handle string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
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
func (c *cookieTransport) Clear(w http.ResponseWriter, cookieName string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   c.secure,
		SameSite: http.SameSiteLaxMode,
	})
}
