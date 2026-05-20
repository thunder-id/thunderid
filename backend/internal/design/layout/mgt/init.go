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

package layoutmgt

import (
	"net/http"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize initializes the layout management service and registers its routes.
func Initialize(mux *http.ServeMux) (LayoutMgtServiceInterface, declarativeresource.ResourceExporter, error) {
	// Step 1: Initialize store based on configuration
	layoutMgtStore, err := initializeStore()
	if err != nil {
		return nil, nil, err
	}

	// Step 2: Create service with store
	layoutMgtService := newLayoutMgtService(layoutMgtStore)
	layoutMgtHandler := newLayoutMgtHandler(layoutMgtService)
	registerRoutes(mux, layoutMgtHandler)

	exporter := newLayoutExporter(layoutMgtService)
	return layoutMgtService, exporter, nil
}

// Store Selection (based on layout.store configuration):
//
// 1. MUTABLE mode (store: "mutable"):
//   - Uses database store only
//   - Supports full CRUD operations (Create/Read/Update/Delete)
//   - All layouts are mutable
//   - Export functionality exports DB-backed layouts
//
// 2. IMMUTABLE mode (store: "declarative"):
//   - Uses file-based store only (from YAML resources)
//   - All layouts are immutable (read-only)
//   - No create/update/delete operations allowed
//   - Export functionality not applicable
//
// 3. COMPOSITE mode (store: "composite" - hybrid):
//   - Uses both file-based store (immutable) + database store (mutable)
//   - YAML resources are loaded into file-based store (immutable, read-only)
//   - Database store handles runtime layouts (mutable)
//   - Reads check both stores (merged results)
//   - Writes only go to database store
//   - Declarative layouts cannot be updated or deleted
//   - Export only exports DB-backed layouts (not YAML)
//
// Configuration Fallback:
// - If layout.store is not specified, falls back to global declarative_resources.enabled:
//   - If declarative_resources.enabled = true: behaves as IMMUTABLE mode
//   - If declarative_resources.enabled = false: behaves as MUTABLE mode
func initializeStore() (layoutMgtStoreInterface, error) {
	var layoutMgtStore layoutMgtStoreInterface

	storeMode := getLayoutStoreMode()

	switch storeMode {
	case serverconst.StoreModeComposite:
		fileStore := newLayoutFileBasedStore()
		dbStore := newLayoutMgtStore()
		layoutMgtStore = newCompositeLayoutStore(fileStore, dbStore)
		if err := loadDeclarativeResources(fileStore, dbStore); err != nil {
			return nil, err
		}

	case serverconst.StoreModeDeclarative:
		fileStore := newLayoutFileBasedStore()
		layoutMgtStore = fileStore
		if err := loadDeclarativeResources(fileStore, nil); err != nil {
			return nil, err
		}

	default:
		layoutMgtStore = newLayoutMgtStore()
	}

	return layoutMgtStore, nil
}

// registerRoutes registers the routes for layout management operations.
func registerRoutes(mux *http.ServeMux, layoutMgtHandler *layoutMgtHandler) {
	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /design/layouts", layoutMgtHandler.HandleLayoutPostRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("GET /design/layouts", layoutMgtHandler.HandleLayoutListRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /design/layouts", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts1))

	opts2 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /design/layouts/{id}", layoutMgtHandler.HandleLayoutGetRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("PUT /design/layouts/{id}", layoutMgtHandler.HandleLayoutPutRequest, opts2))
	mux.HandleFunc(middleware.WithCORS(
		"DELETE /design/layouts/{id}", layoutMgtHandler.HandleLayoutDeleteRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /design/layouts/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts2))
}
