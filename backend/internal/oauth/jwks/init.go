// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package jwks

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the JWKS service and registers its routes.
func Initialize(mux *http.ServeMux, cryptoProvider providers.RuntimeCryptoProvider) JWKSServiceInterface {
	// Initialize the JWKS service
	jwksService := newJWKSService(cryptoProvider)
	jwksHandler := newJWKSHandler(jwksService)
	registerRoutes(mux, jwksHandler)
	return jwksService
}

// registerRoutes registers the routes for the JWKSAPIService.
func registerRoutes(mux *http.ServeMux, jwksHandler *jwksHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /oauth2/jwks",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(jwksHandler.HandleJWKSRequest)).ServeHTTP, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /oauth2/jwks",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
