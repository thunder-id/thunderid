// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package bff

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// externalBaseURL returns the base URL the browser reached the BFF on. When a proxy serves the app
// from another origin, such as a development server, and that origin is trusted, it is that origin.
// Otherwise it is the server's public URL. The redirect URIs the BFF hands out are built on it, so
// the browser comes back to the origin it started on.
func (h *handler) externalBaseURL(r *http.Request) string {
	host, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Host"), ",")
	host = strings.TrimSpace(host)
	if host == "" {
		return h.cfg.PublicURL
	}
	proto, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ",")
	proto = strings.TrimSpace(proto)
	if proto == "" {
		proto = "https"
	}

	if origin := proto + "://" + host; h.isTrustedOrigin(r.Context(), origin) {
		return origin
	}
	return h.cfg.PublicURL
}

// isTrustedOrigin reports whether the app may be served from the origin: the server's own origin, or
// the origin of a redirect URI registered on the app's OAuth client. Trusting nothing the client does
// not already trust keeps a forged forwarding header from steering a redirect anywhere new.
func (h *handler) isTrustedOrigin(ctx context.Context, origin string) bool {
	if origin == "" {
		return false
	}
	if origin == originOf(h.cfg.PublicURL) {
		return true
	}

	client, err := h.clients.GetOAuthClientByClientID(ctx, h.cfg.ClientID)
	if err != nil || client == nil {
		h.logger.Warn(ctx, "Failed to resolve the OAuth client to check an origin",
			log.String("clientId", h.cfg.ClientID))
		return false
	}
	for _, uri := range client.RedirectURIs {
		if originOf(uri) == origin {
			return true
		}
	}
	return false
}

// originOf returns the scheme and host of a URL, or an empty string when it has neither.
func originOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
