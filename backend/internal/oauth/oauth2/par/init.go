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

package par

import (
	"context"
	"net/http"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the PAR handler and registers its routes.
// Returns the PARServiceInterface so the authorization endpoint can resolve request_uri parameters.
func Initialize(
	mux *http.ServeMux,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	jwtService jwt.JWTServiceInterface,
	discoveryService discovery.DiscoveryServiceInterface,
	resourceService providers.ResourceServerProvider,
	dpopVerifier dpop.VerifierInterface,
	cfg oauthconfig.Config,
) PARServiceInterface {
	store := initializePARStore(cfg)
	parSvc := newPARService(store, resourceService, cfg)
	parEndpoint := discoveryService.GetOAuth2AuthorizationServerMetadata(
		context.Background()).PushedAuthorizationRequestEndpoint
	handler := newPARHandler(parSvc, dpopVerifier, parEndpoint)
	registerRoutes(mux, handler, actorProvider, authnProvider, jwtService, discoveryService)
	return parSvc
}

// initializePARStore selects the PAR store implementation based on the configured runtime transient DB type.
func initializePARStore(cfg oauthconfig.Config) parStoreInterface {
	if cfg.RuntimeTransientDBType == provider.DataSourceTypeRedis {
		return newRedisPARRequestStore(provider.GetRedisProvider(), cfg.DeploymentID)
	}
	return newPARRequestStore(cfg.DeploymentID)
}

// registerRoutes registers the PAR endpoint route with client authentication middleware.
func registerRoutes(
	mux *http.ServeMux,
	handler parHandlerInterface,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	jwtService jwt.JWTServiceInterface,
	discoveryService discovery.DiscoveryServiceInterface,
) {
	corsOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	metadata := discoveryService.GetOAuth2AuthorizationServerMetadata(context.Background())
	endpointURL := metadata.PushedAuthorizationRequestEndpoint
	clientAuthMiddleware := clientauth.ClientAuthMiddleware(actorProvider, authnProvider, jwtService, endpointURL)
	wrappedHandler := clientAuthMiddleware(http.HandlerFunc(handler.HandlePARRequest))

	pattern, corsHandler := middleware.WithCORS(
		"POST /oauth2/par",
		wrappedHandler.ServeHTTP,
		corsOpts,
	)

	mux.HandleFunc(pattern, corsHandler)
}
