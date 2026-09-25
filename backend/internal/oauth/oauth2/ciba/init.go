// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ciba

import (
	"context"
	"net/http"

	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the CIBA backchannel authentication handler, registers its routes,
// and returns the CIBAServiceInterface. The store is created internally and never exposed.
// The returned service is used by both the callback dispatcher and the token grant handler.
func Initialize(
	mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	flowExecService flowexec.FlowExecServiceInterface,
	discoveryService discovery.DiscoveryServiceInterface,
	resourceService providers.ResourceServerProvider,
	runtimeStore providers.RuntimeStoreProvider,
	jtiStore jti.JTIStoreInterface,
	cfg oauthconfig.Config,
) CIBAServiceInterface {
	store := newCIBAStore(runtimeStore)
	cibaSvc := newCIBAService(store, flowExecService, jwtService, actorProvider, resourceService, cfg)
	cibaHandler := newCIBAHandler(cibaSvc)
	registerRoutes(mux, cibaHandler, actorProvider, authnProvider, jwtService, discoveryService,
		jtiStore, clientauth.AssertionValidationConfig(cfg.OAuth.ClientAssertion))
	return cibaSvc
}

// registerRoutes registers the bc-authorize endpoint only. The callback (/oauth2/auth/callback)
// is handled by the shared callback package which dispatches by grant type.
func registerRoutes(
	mux *http.ServeMux,
	cibaHandler CIBAHandlerInterface,
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
	authHandler := clientAuthMiddleware(http.HandlerFunc(cibaHandler.HandleBackchannelAuthRequest))

	authPattern, wrappedAuthHandler := middleware.WithCORS(
		"POST "+constants.OAuth2BackchannelAuthEndpoint,
		authHandler.ServeHTTP,
		corsOpts,
	)
	mux.HandleFunc(authPattern, wrappedAuthHandler)
}
