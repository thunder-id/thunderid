// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/clientip"
	sysContext "github.com/thunder-id/thunderid/internal/system/context"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// RequestMetadataMiddleware puts the client metadata of each request on its context: the client IP
// resolved from the trusted proxies, and the User-Agent. The session and the access log read the same
// value on every path. The client IP is set even when it is just the connecting address, so no reader
// reads RemoteAddr itself.
//
// The trusted proxies are parsed once here, and an invalid entry is returned as an error.
func RequestMetadataMiddleware(proxies []engineconfig.TrustedProxyConfig) (func(http.Handler) http.Handler, error) {
	resolver, err := clientip.Initialize(proxies)
	if err != nil {
		return nil, err
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(sysContext.WithRequestMetadata(r.Context(), sysContext.RequestMetadata{
				ClientIP:  resolver.Resolve(r),
				UserAgent: r.UserAgent(),
			}))
			next.ServeHTTP(w, r)
		})
	}, nil
}
