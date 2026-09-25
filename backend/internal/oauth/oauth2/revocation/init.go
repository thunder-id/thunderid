// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package revocation implements single-token revocation over the database.runtime_persistent deny list (the
// JTI deny list): the RFC 7009 POST /oauth2/revoke write path (RevocationService) and the read/
// enforcement path (the enforcement service) that rejects revoked tokens on the AS hot path — introspection, the
// refresh grant, and token exchange — under a fail-closed policy.
package revocation

import (
	"context"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize constructs the shared revocation read and write services.
func Initialize(
	jwtService jwt.JWTServiceInterface,
	observabilitySvc providers.ObservabilityProvider,
	tokenFamilyRevocationTTL time.Duration,
	revokeTokenFamilyOnExplicit bool,
) (EnforcementServiceInterface, RevocationServiceInterface) {
	store := newRevocationStore()
	return newEnforcementService(observabilitySvc, store), newRevocationService(
		jwtService, store, tokenFamilyRevocationTTL, revokeTokenFamilyOnExplicit, observabilitySvc)
}

// RegisterRoutes registers the RFC 7009 revocation endpoint using the shared revocation service.
func RegisterRoutes(
	mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	discoveryService discovery.DiscoveryServiceInterface,
	revocationService RevocationServiceInterface,
	jtiStore jti.JTIStoreInterface,
	assertionCfg clientauth.AssertionValidationConfig,
) {
	revocationHandler := newRevocationHandler(revocationService)
	registerRoutes(mux, revocationHandler, actorProvider, authnProvider, jwtService, discoveryService,
		jtiStore, assertionCfg)
}

// registerRoutes registers the routes for the token revocation endpoint.
func registerRoutes(
	mux *http.ServeMux,
	revocationHandler *revocationHandler,
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
	handler := clientAuthMiddleware(http.HandlerFunc(revocationHandler.HandleRevoke))

	pattern, wrappedHandler := middleware.WithCORS(
		"POST /oauth2/revoke",
		handler.ServeHTTP,
		opts,
	)
	mux.HandleFunc(pattern, wrappedHandler)
	mux.HandleFunc(middleware.WithCORS("OPTIONS /oauth2/revoke",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
