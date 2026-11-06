// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net"
	"net/http"

	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// maxUserAgentLength is the most characters of a user agent the request context carries.
const maxUserAgentLength = 256

// ClientMiddleware records the client of the request, its TCP peer address and its user agent cut to
// 256 characters, on the request context. Forwarded headers are caller controlled and are never read.
func ClientMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		address, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			address = r.RemoteAddr
		}
		userAgent := utils.TruncateRunes(r.UserAgent(), maxUserAgentLength)
		next.ServeHTTP(w, r.WithContext(syscontext.WithClient(r.Context(), address, userAgent)))
	})
}
