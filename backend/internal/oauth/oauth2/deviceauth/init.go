// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package deviceauth

import (
	"context"
	"net/http"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the Device Authorization Grant handler and registers its routes.
func Initialize(
	mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	discoveryService discovery.DiscoveryServiceInterface,
	resourceService providers.ResourceServerProvider,
	runtimeStore providers.RuntimeStoreProvider,
	jtiStore jti.JTIStoreInterface,
	governance providers.IdentityGovernanceProvider,
	cfg oauthconfig.Config,
) DeviceAuthServiceInterface {
	deviceAuthStore := newDeviceAuthStore(runtimeStore, cfg.DeviceAuthUserCodeIndexKey)
	deviceAuthService := newDeviceAuthService(deviceAuthStore, resourceService, jwtService, cfg, actorProvider)
	deviceAuthHandler := newHandler(deviceAuthService)
	registerRoutes(mux, deviceAuthHandler, actorProvider, authnProvider, jwtService, discoveryService,
		jtiStore, governance, cfg.OAuth.ClientAssertion, cfg.JWT.Leeway)
	return deviceAuthService
}

// registerRoutes registers the device authorization endpoint.
func registerRoutes(
	mux *http.ServeMux,
	deviceAuthHandler DeviceAuthHandlerInterface,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	jwtService jwt.JWTServiceInterface,
	discoveryService discovery.DiscoveryServiceInterface,
	jtiStore jti.JTIStoreInterface,
	governance providers.IdentityGovernanceProvider,
	assertionCfg engineconfig.ClientAssertionConfig,
	leeway int64,
) {
	corsOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	issuer := discoveryService.GetOAuth2AuthorizationServerMetadata(context.Background()).Issuer
	clientAuthMiddleware := clientauth.ClientAuthMiddleware(actorProvider, authnProvider, governance, jwtService,
		jtiStore, issuer, assertionCfg, leeway)
	authHandler := clientAuthMiddleware(http.HandlerFunc(deviceAuthHandler.HandleDeviceAuthorizationRequest))

	authPattern, wrappedAuthHandler := middleware.WithCORS(
		"POST "+constants.OAuth2DeviceAuthorizationEndpoint,
		authHandler.ServeHTTP,
		corsOpts,
	)
	mux.HandleFunc(authPattern, wrappedAuthHandler)
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+constants.OAuth2DeviceAuthorizationEndpoint,
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) },
		corsOpts,
	))
}
