// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"net/http"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the discovery service and registers its routes
func Initialize(
	mux *http.ServeMux, cryptoProvider providers.RuntimeCryptoProvider, jweService jwe.JWEServiceInterface,
	cfg oauthconfig.Config,
) DiscoveryServiceInterface {
	discoveryService := newDiscoveryService(cryptoProvider, jweService, cfg)
	discoveryHandler := newDiscoveryHandler(discoveryService)
	registerRoutes(mux, discoveryHandler)
	return discoveryService
}

// registerRoutes registers the routes for discovery endpoints
func registerRoutes(mux *http.ServeMux, handler discoveryHandlerInterface) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           600,
	}

	oauth2Metadata := middleware.CorrelationIDMiddleware(
		http.HandlerFunc(handler.HandleOAuth2AuthorizationServerMetadata)).ServeHTTP
	mux.HandleFunc(middleware.WithCORS("GET /.well-known/oauth-authorization-server", oauth2Metadata, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /.well-known/oauth-authorization-server",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))

	oidcDiscovery := middleware.CorrelationIDMiddleware(http.HandlerFunc(handler.HandleOIDCDiscovery)).ServeHTTP
	mux.HandleFunc(middleware.WithCORS("GET /.well-known/openid-configuration", oidcDiscovery, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /.well-known/openid-configuration",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
