// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package layoutmgt

import (
	"net/http"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize initializes the layout management service and registers its routes. bus is required: the
// service publishes the audit event of every layout it creates, updates or deletes on it.
func Initialize(mux *http.ServeMux, bus eventbus.EventBusInterface) (LayoutMgtServiceInterface,
	declarativeresource.ResourceExporter, error) {
	if bus == nil {
		return nil, nil, errNilEventBus
	}

	// Step 1: Initialize store based on configuration
	layoutMgtStore, err := initializeStore()
	if err != nil {
		return nil, nil, err
	}

	// Step 2: Create service with store
	layoutMgtService := newLayoutMgtService(layoutMgtStore, bus)
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

	opts3 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /design/layouts/{id}/usages",
		layoutMgtHandler.HandleLayoutUsagesGetRequest, opts3))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /design/layouts/{id}/usages",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }, opts3))
}
