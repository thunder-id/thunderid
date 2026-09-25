// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package introspect

import (
	"context"
	"net/http"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the token introspection handler and registers its routes.
func Initialize(
	mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	discoveryService discovery.DiscoveryServiceInterface,
	tokenValidator tokenservice.TokenValidatorInterface,
	jtiStore jti.JTIStoreInterface,
	assertionCfg clientauth.AssertionValidationConfig,
) TokenIntrospectionServiceInterface {
	introspectionService := newTokenIntrospectionService(tokenValidator)
	introspectHandler := newTokenIntrospectionHandler(introspectionService)
	registerRoutes(mux, introspectHandler, actorProvider, authnProvider, jwtService, discoveryService,
		jtiStore, assertionCfg)
	return introspectionService
}

// registerRoutes registers the routes for the IntrospectionAPIService.
func registerRoutes(
	mux *http.ServeMux,
	introspectHandler *tokenIntrospectionHandler,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	jwtService jwt.JWTServiceInterface,
	discoveryService discovery.DiscoveryServiceInterface,
	jtiStore jti.JTIStoreInterface,
	assertionCfg clientauth.AssertionValidationConfig,
) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST", "OPTIONS"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	issuer := discoveryService.GetOAuth2AuthorizationServerMetadata(context.Background()).Issuer
	clientAuthMiddleware := clientauth.ClientAuthMiddleware(actorProvider, authnProvider, jwtService,
		jtiStore, issuer, assertionCfg)
	handler := clientAuthMiddleware(http.HandlerFunc(introspectHandler.HandleIntrospect))

	pattern, wrappedHandler := middleware.WithCORS(
		"POST /oauth2/introspect",
		handler.ServeHTTP,
		opts,
	)
	mux.HandleFunc(pattern, wrappedHandler)
	mux.HandleFunc(middleware.WithCORS("OPTIONS /oauth2/introspect",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
