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
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize initializes the CIBA backchannel authentication handler, registers its routes,
// and returns the CIBAServiceInterface. The store is created internally and never exposed.
// The returned service is used by both the callback dispatcher and the token grant handler.
func Initialize(
	mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface,
	inboundClient inboundclient.InboundClientServiceInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	flowExecService flowexec.FlowExecServiceInterface,
	discoveryService discovery.DiscoveryServiceInterface,
	resourceService resource.ResourceServiceInterface,
) CIBAServiceInterface {
	store := newCIBAStore()
	cibaSvc := newCIBAService(store, flowExecService, jwtService, inboundClient, resourceService)
	cibaHandler := newCIBAHandler(cibaSvc)
	registerRoutes(mux, cibaHandler, inboundClient, authnProvider, jwtService, discoveryService)
	return cibaSvc
}

// registerRoutes registers the bc-authorize endpoint only. The callback (/oauth2/auth/callback)
// is handled by the shared callback package which dispatches by grant type.
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
}
