/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize wires the revocation feature and registers the RFC 7009 revocation endpoint. It returns
// the enforcement service (the read path, injected into the hot paths: refresh grant, token exchange,
// introspection) and the revocation service (the write path, covering single-token revocation via
// RevokeRefreshToken and token-family revocation via RevokeTokenFamily). Consumers depend on the narrow
// RefreshTokenRevokerInterface / CriteriaRevokerInterface subsets of the revocation service.
// tokenFamilyRevocationTTL bounds each token-family deny-list entry; pass the refresh-token lifetime.
func Initialize(
	mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	discoveryService discovery.DiscoveryServiceInterface,
	observabilitySvc providers.ObservabilityProvider,
	tokenFamilyRevocationTTL time.Duration,
	revokeTokenFamilyOnExplicit bool,
) (EnforcementServiceInterface, RevocationServiceInterface) {
	store := newRevocationStore()
	enforcementService := newEnforcementService(observabilitySvc, store)
	criteriaRevoker := newCriteriaRevoker(store, tokenFamilyRevocationTTL)
	revocationService := newRevocationService(jwtService, store, criteriaRevoker,
		revokeTokenFamilyOnExplicit, observabilitySvc)
	revocationHandler := newRevocationHandler(revocationService)
	registerRoutes(mux, revocationHandler, actorProvider, authnProvider, jwtService, discoveryService)
	return enforcementService, revocationService
}

// InitializeCriteriaRevoker builds a standalone criteria revoker for consumers wired at the composition
// root that cannot receive the one from Initialize (which is created inside the OAuth engine after
// those consumers are constructed) — notably the SSO session service, which revokes a session's
// families on sign-out. The criteria revoker is a stateless writer to the criteria deny list, so a
// separate instance shares no mutable state and is safe. tokenFamilyRevocationTTL bounds each entry.
func InitializeCriteriaRevoker(tokenFamilyRevocationTTL time.Duration) CriteriaRevokerInterface {
	return newCriteriaRevoker(newRevocationStore(), tokenFamilyRevocationTTL)
}

// registerRoutes registers the routes for the token revocation endpoint.
func registerRoutes(
	mux *http.ServeMux,
	revocationHandler *revocationHandler,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	jwtService jwt.JWTServiceInterface,
	discoveryService discovery.DiscoveryServiceInterface,
) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST", "OPTIONS"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	issuer := discoveryService.GetOAuth2AuthorizationServerMetadata(context.Background()).Issuer
	clientAuthMiddleware := clientauth.ClientAuthMiddleware(actorProvider, authnProvider, jwtService, issuer)
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
