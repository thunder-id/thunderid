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

package ciba

import (
	"context"
	"net/http"

	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/oauth/scope"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// NewStore creates a new CIBA authentication request store.
func NewStore() CIBARequestStoreInterface {
	return newCIBARequestStore()
}

// Initialize initializes the CIBA backchannel authentication handler and registers its routes.
func Initialize(
	mux *http.ServeMux,
	store CIBARequestStoreInterface,
	jwtService jwt.JWTServiceInterface,
	inboundClient inboundclient.InboundClientServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	flowExecService flowexec.FlowExecServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	discoveryService discovery.DiscoveryServiceInterface,
	scopeValidator scope.ScopeValidatorInterface,
) CIBAHandlerInterface {
	cibaSvc := newCIBAService(store, flowExecService, entityProvider, jwtService, inboundClient, scopeValidator)
	cibaHandler := newCIBAHandler(cibaSvc)
	registerRoutes(mux, cibaHandler, inboundClient, authnProvider, jwtService, discoveryService)
	return cibaHandler
}

// registerRoutes registers the routes for the CIBA backchannel authentication endpoints.
func registerRoutes(
	mux *http.ServeMux,
	cibaHandler CIBAHandlerInterface,
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

	endpointURL := discoveryService.GetOAuth2AuthorizationServerMetadata(
		context.Background()).BackchannelAuthenticationEndpoint
	clientAuthMiddleware := clientauth.ClientAuthMiddleware(inboundClient, authnProvider, jwtService, endpointURL)
	authHandler := clientAuthMiddleware(http.HandlerFunc(cibaHandler.HandleBackchannelAuthRequest))

	authPattern, wrappedAuthHandler := middleware.WithCORS(
		"POST "+constants.OAuth2BackchannelAuthEndpoint,
		authHandler.ServeHTTP,
		corsOpts,
	)
	mux.HandleFunc(authPattern, wrappedAuthHandler)

	mux.HandleFunc(middleware.WithCORS("POST "+constants.OAuth2BackchannelAuthCallbackEndpoint,
		cibaHandler.HandleBackchannelAuthCallback, corsOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+constants.OAuth2BackchannelAuthCallbackEndpoint,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, corsOpts))
}
