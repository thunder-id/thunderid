// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"context"
	"net/http"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/granthandlers"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/oauth/scope"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the token handler and registers its routes.
func Initialize(
	mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	grantHandlerProvider granthandlers.GrantHandlerProviderInterface,
	scopeValidator scope.ScopeValidatorInterface,
	observabilitySvc providers.ObservabilityProvider,
	discoveryService discovery.DiscoveryServiceInterface,
	dpopVerifier dpop.VerifierInterface,
	jtiStore jti.JTIStoreInterface,
	ouService providers.OrganizationUnitProvider,
	cfg oauthconfig.Config,
) TokenHandlerInterface {
	tokenEndpoint := discoveryService.GetOAuth2AuthorizationServerMetadata(context.Background()).TokenEndpoint
	dpopRequired := cfg.OAuth.DPoP.Required
	tokenSvc := newTokenService(grantHandlerProvider, scopeValidator, observabilitySvc,
		dpopVerifier, tokenEndpoint, dpopRequired)
	tokenHandler := newTokenHandler(tokenSvc, observabilitySvc)
	registerRoutes(mux, tokenHandler, actorProvider, authnProvider, jwtService, discoveryService,
		jtiStore, ouService, cfg.JWT.Leeway)
	return tokenHandler
}

// registerRoutes registers the routes for the TokenService.
func registerRoutes(
	mux *http.ServeMux,
	tokenHandler TokenHandlerInterface,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	jwtService jwt.JWTServiceInterface,
	discoveryService discovery.DiscoveryServiceInterface,
	jtiStore jti.JTIStoreInterface,
	ouService providers.OrganizationUnitProvider,
	leeway int64,
) {
	corsOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	issuer := discoveryService.GetOAuth2AuthorizationServerMetadata(context.Background()).Issuer
	clientAuthMiddleware := clientauth.ClientAuthMiddleware(actorProvider, authnProvider, jwtService,
		jtiStore, issuer, leeway)
	// The accessing-organization-unit middleware wraps client authentication rather than the other
	// way round: resolving the client is itself scoped by that organization unit, so it has to be
	// settled first.
	handler := accessingOUMiddleware(ouService)(
		clientAuthMiddleware(http.HandlerFunc(tokenHandler.HandleTokenRequest)))

	for _, route := range []string{
		"POST /oauth2/token",
		"POST /ou/{" + constants.PathParamOUID + "}/oauth2/token",
	} {
		pattern, wrappedHandler := middleware.WithCORS(route, handler.ServeHTTP, corsOpts)
		mux.HandleFunc(pattern, wrappedHandler)
	}
}
