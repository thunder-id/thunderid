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

package discovery

import (
	"net/http"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the discovery service and registers its routes
func Initialize(
	mux *http.ServeMux, cryptoProvider providers.RuntimeCryptoProvider, cfg oauthconfig.Config,
) DiscoveryServiceInterface {
	discoveryService := newDiscoveryService(cryptoProvider, cfg)
	discoveryHandler := newDiscoveryHandler(discoveryService)
	registerRoutes(mux, discoveryHandler)
	return discoveryService
}

// registerRoutes registers the routes for discovery endpoints
func registerRoutes(mux *http.ServeMux, handler discoveryHandlerInterface) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /.well-known/oauth-authorization-server",
		handler.HandleOAuth2AuthorizationServerMetadata, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /.well-known/oauth-authorization-server",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))

	mux.HandleFunc(middleware.WithCORS("GET /.well-known/openid-configuration",
		handler.HandleOIDCDiscovery, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /.well-known/openid-configuration",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
