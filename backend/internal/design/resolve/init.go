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

package resolve

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/application"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize initializes the design resolve service and registers its routes.
func Initialize(
	mux *http.ServeMux,
	themeMgtService thememgt.ThemeMgtServiceInterface,
	layoutMgtService layoutmgt.LayoutMgtServiceInterface,
	applicationService application.ApplicationServiceInterface,
) DesignResolveServiceInterface {
	designResolveService := newDesignResolveService(themeMgtService, layoutMgtService, applicationService)
	designResolveHandler := newDesignResolveHandler(designResolveService)
	registerRoutes(mux, designResolveHandler)
	return designResolveService
}

// registerRoutes registers the routes for design resolve operations.
func registerRoutes(mux *http.ServeMux, resolveHandler *designResolveHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /design/resolve", resolveHandler.HandleResolveRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /design/resolve", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts))
}
