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

package serverconfig

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize initializes the server config service and registers its routes.
func Initialize(mux *http.ServeMux, cacheManager cache.CacheManagerInterface) ServerConfigService {
	configCache := cache.GetCache[*ServerConfig](cacheManager, serverConfigCacheName)
	store := newCachedBackStore(newServerConfigStore(), configCache)
	service := newServerConfigService(store)

	handler := newServerConfigHandler(service)
	registerRoutes(mux, handler)

	return service
}

// registerRoutes registers the routes for server config operations.
func registerRoutes(mux *http.ServeMux, handler *serverConfigHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /server-config", handler.HandleGetServerConfig, opts))
	mux.HandleFunc(middleware.WithCORS("PUT /server-config", handler.HandleUpdateServerConfig, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /server-config",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))

	// A single config section is exposed as a read-only sub-resource.
	itemOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /server-config/{name}", handler.HandleGetServerConfigByName, itemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /server-config/{name}",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, itemOpts))
}
