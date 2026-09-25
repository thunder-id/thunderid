// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package par

import (
	"context"
	"net/http"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the PAR handler and registers its routes.
// Returns the PARServiceInterface so the authorization endpoint can resolve request_uri parameters.
func Initialize(
	mux *http.ServeMux,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	jwtService jwt.JWTServiceInterface,
	discoveryService discovery.DiscoveryServiceInterface,
	resourceService providers.ResourceServerProvider,
	dpopVerifier dpop.VerifierInterface,
	cfg oauthconfig.Config,
	storeProvider providers.RuntimeStoreProvider,
	jtiStore jti.JTIStoreInterface,
) PARServiceInterface {
	store := newPARRequestStore(storeProvider)
	parSvc := newPARService(store, resourceService, cfg)
	parEndpoint := discoveryService.GetOAuth2AuthorizationServerMetadata(
		context.Background()).PushedAuthorizationRequestEndpoint
	handler := newPARHandler(parSvc, dpopVerifier, parEndpoint)
	registerRoutes(mux, handler, actorProvider, authnProvider, jwtService, discoveryService,
		jtiStore, clientauth.AssertionValidationConfig(cfg.OAuth.ClientAssertion))
	return parSvc
}

// registerRoutes registers the PAR endpoint route with client authentication middleware.
func registerRoutes(
	mux *http.ServeMux,
	handler parHandlerInterface,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	jwtService jwt.JWTServiceInterface,
	discoveryService discovery.DiscoveryServiceInterface,
	jtiStore jti.JTIStoreInterface,
	assertionCfg clientauth.AssertionValidationConfig,
) {
	corsOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	issuer := discoveryService.GetOAuth2AuthorizationServerMetadata(context.Background()).Issuer
	clientAuthMiddleware := clientauth.ClientAuthMiddleware(actorProvider, authnProvider, jwtService,
		jtiStore, issuer, assertionCfg)
	wrappedHandler := clientAuthMiddleware(http.HandlerFunc(handler.HandlePARRequest))

	pattern, corsHandler := middleware.WithCORS(
		"POST /oauth2/par",
		wrappedHandler.ServeHTTP,
		corsOpts,
	)

	mux.HandleFunc(pattern, corsHandler)
}
