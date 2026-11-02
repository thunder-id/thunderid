// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package revocation implements single-token revocation over the database.runtime_persistent deny list (the
// JTI deny list): the RFC 7009 POST /oauth2/revoke write path (RevocationService) and the read/
// enforcement path (the enforcement service) that rejects revoked tokens on the AS hot path — introspection, the
// refresh grant, and token exchange — under a fail-closed policy.
package revocation

import (
	"context"
	"fmt"
	"net/http"
	"time"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize constructs the shared revocation read and write services from the OAuth config.
func Initialize(
	cfg oauthconfig.Config,
	jwtService jwt.JWTServiceInterface,
	observabilitySvc providers.ObservabilityProvider,
) (EnforcementServiceInterface, RevocationServiceInterface, error) {
	revocationCfg := cfg.OAuth.Revocation
	maxEntriesPerChange := revocationCfg.Criteria.MaxEntriesPerChange
	if maxEntriesPerChange < 0 {
		return nil, nil, fmt.Errorf(
			"oauth.revocation.criteria.max_entries_per_change must not be negative, got %d", maxEntriesPerChange)
	}
	tokenFamilyRevocationTTL := time.Duration(cfg.OAuth.RefreshToken.ValidityPeriod) * time.Second
	validationLeeway := time.Duration(cfg.JWT.Leeway) * time.Second
	store := newRevocationStore()
	return newEnforcementService(observabilitySvc, store), newRevocationService(
		jwtService, store, tokenFamilyRevocationTTL, validationLeeway,
		revocationCfg.TokenFamily.OnExplicitRevokeEnabled(), observabilitySvc, maxEntriesPerChange), nil
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
	governance providers.IdentityGovernanceProvider,
	assertionCfg engineconfig.ClientAssertionConfig,
	jwtLeeway int64,
) {
	revocationHandler := newRevocationHandler(revocationService)
	registerRoutes(mux, revocationHandler, actorProvider, authnProvider, jwtService, discoveryService,
		jtiStore, governance, assertionCfg, jwtLeeway)
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
	governance providers.IdentityGovernanceProvider,
	assertionCfg engineconfig.ClientAssertionConfig,
	jwtLeeway int64,
) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST", "OPTIONS"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	issuer := discoveryService.GetOAuth2AuthorizationServerMetadata(context.Background()).Issuer
	clientAuthMiddleware := clientauth.ClientAuthMiddleware(actorProvider, authnProvider, governance, jwtService,
		jtiStore, issuer, assertionCfg, jwtLeeway)
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
