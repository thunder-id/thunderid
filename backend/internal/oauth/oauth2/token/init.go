/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package token

import (
	"context"
	"net/http"

	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/granthandlers"
	"github.com/thunder-id/thunderid/internal/oauth/scope"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// Initialize initializes the token handler and registers its routes.
func Initialize(
	mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface,
	inboundClient inboundclient.InboundClientServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	grantHandlerProvider granthandlers.GrantHandlerProviderInterface,
	scopeValidator scope.ScopeValidatorInterface,
	observabilitySvc observability.ObservabilityServiceInterface,
	discoveryService discovery.DiscoveryServiceInterface,
	transactioner transaction.Transactioner,
) TokenHandlerInterface {
	tokenSvc := newTokenService(grantHandlerProvider, scopeValidator, observabilitySvc, transactioner)
	tokenHandler := newTokenHandler(tokenSvc, observabilitySvc)
	registerRoutes(mux, tokenHandler, inboundClient, authnProvider, jwtService, discoveryService)
	return tokenHandler
}

// registerRoutes registers the routes for the TokenService.
func registerRoutes(
	mux *http.ServeMux,
	tokenHandler TokenHandlerInterface,
	inboundClient inboundclient.InboundClientServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	jwtService jwt.JWTServiceInterface,
	discoveryService discovery.DiscoveryServiceInterface,
) {
	corsOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	endpointURL := discoveryService.GetOAuth2AuthorizationServerMetadata(context.Background()).TokenEndpoint
	clientAuthMiddleware := clientauth.ClientAuthMiddleware(inboundClient, authnProvider, jwtService, endpointURL)
	handler := clientAuthMiddleware(http.HandlerFunc(tokenHandler.HandleTokenRequest))

	pattern, wrappedHandler := middleware.WithCORS(
		"POST /oauth2/token",
		handler.ServeHTTP,
		corsOpts,
	)

	mux.HandleFunc(pattern, wrappedHandler)
}
