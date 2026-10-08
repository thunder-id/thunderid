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
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
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
		jtiStore, ouService, cfg.EnableOUQualifiedEndpoints, cfg.OAuth.ClientAssertion,
		cfg.JWT.Leeway)
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
	ouQualifiedEndpointsEnabled bool,
	assertionCfg engineconfig.ClientAssertionConfig,
	jwtLeeway int64,
) {
	corsOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	issuer := discoveryService.GetOAuth2AuthorizationServerMetadata(context.Background()).Issuer
	clientAuthMiddleware := clientauth.ClientAuthMiddleware(actorProvider, authnProvider, jwtService,
		jtiStore, issuer, assertionCfg, jwtLeeway)
	endpoint := http.HandlerFunc(tokenHandler.HandleTokenRequest)
	register := func(route string, handler http.Handler) {
		pattern, wrapped := middleware.WithCORS(route, handler.ServeHTTP, corsOpts)
		mux.HandleFunc(pattern, wrapped)
	}

	register("POST /oauth2/token", clientAuthMiddleware(endpoint))

	// The organization-unit-qualified route is served only where a deployment turned it on.
	if !ouQualifiedEndpointsEnabled {
		return
	}

	// Ordering: authenticate the client first, then resolve the unit, then admit the client to it.
	register("POST /ou/{"+middleware.PathParamOUID+"}/oauth2/token",
		clientAuthMiddleware(
			middleware.AccessingOUMiddleware(ouService, ouAccessRefusal, ouLookupFailure)(
				clientauth.ClientOUAdmissionMiddleware(actorProvider)(endpoint))))
}

// ouAccessRefusal answers an organization unit the request may not act for.
//
// It is deliberately the same answer a client that was never shared receives, down to the text and
// naming no organization unit, so the endpoint cannot be used to discover which ones exist.
var ouAccessRefusal = middleware.AccessingOUResponse{
	Code:        constants.ErrorUnauthorizedClient,
	Description: constants.OUAccessRefusal,
	StatusCode:  http.StatusBadRequest,
}

// ouLookupFailure answers a request whose organization unit could not be resolved because the
// deployment is broken. A server error rather than a refusal: the caller did nothing wrong.
var ouLookupFailure = middleware.AccessingOUResponse{
	Code:        constants.ErrorServerError,
	Description: "Failed to resolve the organization unit",
	StatusCode:  http.StatusInternalServerError,
}
