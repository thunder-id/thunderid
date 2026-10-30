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
	sharedrevocation "github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/eventlistener"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// EventHook registers listeners that are told of every revocation once it is recorded. Initialize
// hands it to the composition root, which registers the revocation push dispatcher on it.
type EventHook = eventlistener.Hook[sharedrevocation.Event]

// Initialize constructs the shared revocation read and write services from the OAuth config, and the
// hook for listeners of recorded revocations.
func Initialize(
	cfg oauthconfig.Config,
	jwtService jwt.JWTServiceInterface,
	observabilitySvc providers.ObservabilityProvider,
) (EnforcementServiceInterface, RevocationServiceInterface, EventHook, error) {
	revocationCfg := cfg.OAuth.Revocation
	maxEntriesPerChange := revocationCfg.Criteria.MaxEntriesPerChange
	if maxEntriesPerChange < 0 {
		return nil, nil, nil, fmt.Errorf(
			"oauth.revocation.criteria.max_entries_per_change must not be negative, got %d", maxEntriesPerChange)
	}
	tokenFamilyRevocationTTL := time.Duration(cfg.OAuth.RefreshToken.ValidityPeriod) * time.Second
	validationLeeway := time.Duration(cfg.JWT.Leeway) * time.Second
	store := newRevocationStore()
	service := buildRevocationService(
		jwtService, store, tokenFamilyRevocationTTL, validationLeeway,
		revocationCfg.TokenFamily.OnExplicitRevokeEnabled(), observabilitySvc, maxEntriesPerChange)
	return newEnforcementService(observabilitySvc, store), service, service.events.Hook(), nil
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
