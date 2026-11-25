// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package openid4vp exposes the ThunderID-specific OpenID4VP verification session endpoints
// (initiate, status, trust anchors). The protocol-defined wallet endpoints live in
// internal/oauth/oauth2/openid4vp.
package openid4vp

import (
	"net/http"

	vpprotocol "github.com/thunder-id/thunderid/internal/oauth/oauth2/openid4vp"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/vc/truststore"
)

// Initialize registers the OpenID4VP verification session routes on mux.
func Initialize(
	mux *http.ServeMux, verifySvc vpprotocol.OpenID4VPServiceInterface, trustStore truststore.TrustStoreInterface,
) {
	registerRoutes(mux, newOpenID4VPHandler(verifySvc, trustStore))
}

// registerRoutes registers the verification session routes on mux with CORS middleware.
func registerRoutes(mux *http.ServeMux, h *openID4VPHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET "+trustAnchorsPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleTrustAnchors)).ServeHTTP, opts))
	mux.HandleFunc(middleware.WithCORS("POST "+initiatePath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleInitiate)).ServeHTTP, opts))
	mux.HandleFunc(middleware.WithCORS("GET "+statusPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleStatus)).ServeHTTP, opts))

	for _, path := range []string{trustAnchorsPath, initiatePath, statusPath} {
		mux.HandleFunc(middleware.WithCORS("OPTIONS "+path,
			func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, opts))
	}
}
