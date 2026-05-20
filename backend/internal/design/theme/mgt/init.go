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

package thememgt

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize initializes the theme management service and registers its routes.
func Initialize(
	mux *http.ServeMux,
	mcpServer *mcp.Server,
) (ThemeMgtServiceInterface, declarativeresource.ResourceExporter, error) {
	// Step 1: Initialize store based on configuration
	themeMgtStore, err := initializeStore()
	if err != nil {
		return nil, nil, err
	}

	// Step 2: Create service with store
	themeMgtService := newThemeMgtService(themeMgtStore)
	themeMgtHandler := newThemeMgtHandler(themeMgtService)
	registerRoutes(mux, themeMgtHandler)

	if mcpServer != nil {
		registerMCPTools(mcpServer, themeMgtService)
	}

	exporter := newThemeExporter(themeMgtService)
	return themeMgtService, exporter, nil
}

// Store Selection (based on theme.store configuration):
//
// 1. MUTABLE mode (store: "mutable"):
//   - Uses database store only
//   - Supports full CRUD operations (Create/Read/Update/Delete)
//   - All themes are mutable
//   - Export functionality exports DB-backed themes
//
// 2. IMMUTABLE mode (store: "declarative"):
//   - Uses file-based store only (from YAML resources)
//   - All themes are immutable (read-only)
//   - No create/update/delete operations allowed
//   - Export functionality not applicable
//
// 3. COMPOSITE mode (store: "composite" - hybrid):
//   - Uses both file-based store (immutable) + database store (mutable)
//   - YAML resources are loaded into file-based store (immutable, read-only)
//   - Database store handles runtime themes (mutable)
//   - Reads check both stores (merged results)
//   - Writes only go to database store
//   - Declarative themes cannot be updated or deleted
//   - Export only exports DB-backed themes (not YAML)
//
// Configuration Fallback:
// - If theme.store is not specified, falls back to global declarative_resources.enabled:
//   - If declarative_resources.enabled = true: behaves as IMMUTABLE mode
//   - If declarative_resources.enabled = false: behaves as MUTABLE mode
func initializeStore() (themeMgtStoreInterface, error) {
	var themeMgtStore themeMgtStoreInterface

	storeMode := getThemeStoreMode()

	switch storeMode {
	case serverconst.StoreModeComposite:
		fileStore := newThemeFileBasedStore()
		dbStore := newThemeMgtStore()
		themeMgtStore = newCompositeThemeStore(fileStore, dbStore)
		if err := loadDeclarativeResources(fileStore, dbStore); err != nil {
			return nil, err
		}

	case serverconst.StoreModeDeclarative:
		fileStore := newThemeFileBasedStore()
		themeMgtStore = fileStore
		if err := loadDeclarativeResources(fileStore, nil); err != nil {
			return nil, err
		}

	default:
		themeMgtStore = newThemeMgtStore()
	}

	return themeMgtStore, nil
}

// registerRoutes registers the routes for theme management operations.
func registerRoutes(mux *http.ServeMux, themeMgtHandler *themeMgtHandler) {
	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /design/themes", themeMgtHandler.HandleThemePostRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("GET /design/themes", themeMgtHandler.HandleThemeListRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /design/themes", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts1))

	opts2 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /design/themes/{id}", themeMgtHandler.HandleThemeGetRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("PUT /design/themes/{id}", themeMgtHandler.HandleThemePutRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("DELETE /design/themes/{id}", themeMgtHandler.HandleThemeDeleteRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /design/themes/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts2))
}
