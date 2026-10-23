// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"net/http"
	"net/netip"

	"github.com/thunder-id/thunderid/internal/system/clientip"
	"github.com/thunder-id/thunderid/internal/system/log"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// ClientIPMiddleware puts the client IP resolved for each request on its context, so the session and
// the access log read one value. It always sets the IP, even when it is just the connecting address,
// so a handler that finds none knows the middleware did not run.
//
// The trusted proxies are parsed once here, and an invalid entry is returned as an error.
func ClientIPMiddleware(proxies []engineconfig.TrustedProxyConfig) (func(http.Handler) http.Handler, error) {
	trusted := make([]clientip.TrustedProxy, 0, len(proxies))
	for _, p := range proxies {
		trusted = append(trusted, clientip.TrustedProxy{CIDR: p.CIDR, Header: p.Header})
	}
	resolver, err := clientip.NewResolver(trusted, logClientIPFallback)
	if err != nil {
		return nil, err
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(clientip.WithIP(r.Context(), resolver.Resolve(r)))
			next.ServeHTTP(w, r)
		})
	}, nil
}

// logClientIPFallback warns that a trusted proxy did not send a valid client IP header, which usually
// means the proxy is not configured to set it or the configured header is the wrong one.
func logClientIPFallback(ctx context.Context, cidr netip.Prefix, header string) {
	log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ClientIPMiddleware")).Warn(ctx,
		"Client IP fell back to the connecting address: trusted proxy did not send a valid header",
		log.String("trustedProxy", cidr.String()), log.String("header", header))
}
