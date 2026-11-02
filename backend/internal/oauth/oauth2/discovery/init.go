// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package discovery implements the OAuth 2.0 and OpenID Connect discovery endpoints.
package discovery

import (
	"net/http"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the discovery service and registers its routes
func Initialize(
	mux *http.ServeMux, cryptoProvider providers.RuntimeCryptoProvider, jweService jwe.JWEServiceInterface,
	authzDetailTypes providers.AuthorizationDetailTypeProvider, cfg oauthconfig.Config,
) DiscoveryServiceInterface {
	discoveryService := newDiscoveryService(cryptoProvider, jweService, authzDetailTypes, cfg)
	discoveryHandler := newDiscoveryHandler(discoveryService)
	registerRoutes(mux, discoveryHandler)
	if cfg.OAuth.AuthorizationDetails.IsTypeMetadataEnabled() {
		registerAuthorizationDetailsTypesRoutes(mux, discoveryHandler)
	}
	return discoveryService
}

// registerAuthorizationDetailsTypesRoutes registers the type metadata routes; the type value is URL-encoded.
func registerAuthorizationDetailsTypesRoutes(mux *http.ServeMux, handler discoveryHandlerInterface) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "If-None-Match"},
		AllowCredentials: false,
		MaxAge:           600,
	}
	noContent := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}

	mux.HandleFunc(middleware.WithCORS("GET "+constants.OAuth2AuthorizationDetailsTypesEndpoint,
		handler.HandleAuthorizationDetailsTypes, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+constants.OAuth2AuthorizationDetailsTypesEndpoint,
		noContent, opts))
	mux.HandleFunc(middleware.WithCORS("GET "+constants.OAuth2AuthorizationDetailsTypesEndpoint+"/{type}",
		handler.HandleAuthorizationDetailsType, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+constants.OAuth2AuthorizationDetailsTypesEndpoint+"/{type}",
		noContent, opts))
}

// registerRoutes registers the routes for discovery endpoints
func registerRoutes(mux *http.ServeMux, handler discoveryHandlerInterface) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /.well-known/oauth-authorization-server",
		handler.HandleOAuth2AuthorizationServerMetadata, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /.well-known/oauth-authorization-server",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))

	mux.HandleFunc(middleware.WithCORS("GET /.well-known/openid-configuration",
		handler.HandleOIDCDiscovery, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /.well-known/openid-configuration",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
