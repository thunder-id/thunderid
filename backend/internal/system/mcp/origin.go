// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/cors"
)

// requireAllowedOrigin refuses a request whose Origin header names an untrusted origin with 403
// Forbidden, as MCP Streamable HTTP requires to prevent DNS rebinding. The trusted origins are
// ownOrigin, the server's public origin, and the CORS allowed origins. A request without an Origin
// header, as non-browser MCP clients send, passes through.
func requireAllowedOrigin(ownOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origins := r.Header.Values(constants.OriginHeaderName)
		if len(origins) == 0 || (len(origins) == 1 && isAllowedOrigin(r.Context(), origins[0], ownOrigin)) {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "Forbidden: origin not allowed", http.StatusForbidden)
	})
}

// isAllowedOrigin reports whether origin is ownOrigin or a CORS allowed origin.
func isAllowedOrigin(ctx context.Context, origin, ownOrigin string) bool {
	parsed, err := cors.ParseOrigin(origin)
	if err != nil {
		return false
	}
	if parsed.Canonical != "" && parsed.Canonical == ownOrigin {
		return true
	}
	allowed, _ := cors.GetDynamicMatcher(ctx).Match(parsed)
	return allowed
}

// serverOrigin returns the canonical origin of the server's public URL, or "" if it cannot be parsed.
func serverOrigin() string {
	serverURL, err := url.Parse(config.GetServerURL(&config.GetServerRuntime().Config.Server))
	if err != nil {
		return ""
	}
	parsed, err := cors.ParseOrigin(serverURL.Scheme + "://" + serverURL.Host)
	if err != nil {
		return ""
	}
	return parsed.Canonical
}
