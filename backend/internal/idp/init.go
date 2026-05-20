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

// Package idp handles the identity provider management operations.
package idp

import (
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// Initialize initializes the IDP service and registers its routes.
func Initialize(
	cacheManager cache.CacheManagerInterface, mux *http.ServeMux,
) (IDPServiceInterface, declarativeresource.ResourceExporter, error) {
	// Create store and transactioner based on store mode
	idpStore, transactioner, err := initializeStore(cacheManager)
	if err != nil {
		return nil, nil, err
	}

	idpService := newIDPService(idpStore, transactioner)

	idpHandler := newIDPHandler(idpService)
	registerRoutes(mux, idpHandler)

	// Create and return exporter
	exporter := newIDPExporter(idpService)
	return idpService, exporter, nil
}

// Store Selection (based on identity_provider.store configuration):
//
// 1. MUTABLE mode (store: "mutable"):
//   - Uses database store only (idpStore)
//   - Supports full CRUD operations (Create/Read/Update/Delete)
//   - All IDPs are mutable
//   - Export functionality exports DB-backed IDPs
//
// 2. IMMUTABLE mode (store: "declarative"):
//   - Uses file-based store only (from YAML resources)
//   - All IDPs are immutable (read-only)
//   - No create/update/delete operations allowed
//   - Export functionality not applicable
//
// 3. COMPOSITE mode (store: "composite" - hybrid):
//   - Uses both file-based store (immutable) + database store (mutable)
//   - YAML resources are loaded into file-based store (immutable, read-only)
//   - Database store handles runtime IDPs (mutable)
//   - Reads check both stores (merged results)
//   - Writes only go to database store
//   - Declarative IDPs cannot be updated or deleted
//   - Export only exports DB-backed IDPs (not YAML)
//
// Configuration Fallback:
// - If identity_provider.store is not specified, falls back to global declarative_resources.enabled:
//   - If declarative_resources.enabled = true: behaves as IMMUTABLE mode
//   - If declarative_resources.enabled = false: behaves as MUTABLE mode
func initializeStore(cacheManager cache.CacheManagerInterface) (idpStoreInterface, transaction.Transactioner, error) {
	storeMode := getIdentityProviderStoreMode()

	idpByIDCache := cache.GetCache[*IDPDTO](cacheManager, "IDPByIDCache")
	idpByIssuerCache := cache.GetCache[*IDPDTO](cacheManager, "IDPByIssuerCache")

	switch storeMode {
	case serverconst.StoreModeComposite:
		fileStore, _ := newIDPFileBasedStore()
		dbStore, transactioner, err := newIDPStore()
		if err != nil {
			return nil, nil, err
		}
		idpStore := newCompositeIDPStore(fileStore, dbStore)
		if err := loadDeclarativeResources(fileStore); err != nil {
			return nil, nil, err
		}
		return newCacheBackedIDPStore(idpByIDCache, idpByIssuerCache, idpStore), transactioner, nil

	case serverconst.StoreModeDeclarative:
		fileStore, transactioner := newIDPFileBasedStore()
		if err := loadDeclarativeResources(fileStore); err != nil {
			return nil, nil, err
		}
		return newCacheBackedIDPStore(idpByIDCache, idpByIssuerCache, fileStore), transactioner, nil

	default:
		store, transactioner, err := newIDPStore()
		if err != nil {
			return nil, nil, err
		}
		return newCacheBackedIDPStore(idpByIDCache, idpByIssuerCache, store), transactioner, nil
	}
}

// getIdentityProviderStoreMode determines the store mode for identity providers.
//
// Resolution order:
//  1. If IdentityProvider.Store is explicitly configured, use it
//  2. Otherwise, fall back to global DeclarativeResources.Enabled:
//     - If enabled: return "declarative"
//     - If disabled: return "mutable"
//
// Returns normalized store mode: "mutable", "declarative", or "composite"
func getIdentityProviderStoreMode() serverconst.StoreMode {
	cfg := config.GetServerRuntime().Config
	// Check if service-level configuration is explicitly set
	if cfg.IdentityProvider.Store != "" {
		mode := serverconst.StoreMode(strings.ToLower(strings.TrimSpace(cfg.IdentityProvider.Store)))
		// Validate and normalize
		switch mode {
		case serverconst.StoreModeMutable, serverconst.StoreModeDeclarative, serverconst.StoreModeComposite:
			return mode
		}
	}

	// Fall back to global declarative resources setting
	if declarativeresource.IsDeclarativeModeEnabled() {
		return serverconst.StoreModeDeclarative
	}

	return serverconst.StoreModeMutable
}

// isCompositeModeEnabled checks if composite store mode is enabled for identity providers.
func isCompositeModeEnabled() bool {
	return getIdentityProviderStoreMode() == serverconst.StoreModeComposite
}

// isMutableModeEnabled checks if mutable-only store mode is enabled for identity providers.
func isMutableModeEnabled() bool {
	return getIdentityProviderStoreMode() == serverconst.StoreModeMutable
}

// isDeclarativeModeEnabled checks if immutable-only store mode is enabled for identity providers.
func isDeclarativeModeEnabled() bool {
	return getIdentityProviderStoreMode() == serverconst.StoreModeDeclarative
}

// RegisterRoutes registers the routes for identity provider operations.
func registerRoutes(mux *http.ServeMux, idpHandler *idpHandler) {
	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /identity-providers", idpHandler.HandleIDPPostRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("GET /identity-providers", idpHandler.HandleIDPListRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /identity-providers",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts1))

	opts2 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /identity-providers/{id}",
		idpHandler.HandleIDPGetRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("PUT /identity-providers/{id}",
		idpHandler.HandleIDPPutRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("DELETE /identity-providers/{id}",
		idpHandler.HandleIDPDeleteRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /identity-providers/{id}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts2))
}
