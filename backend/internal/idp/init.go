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
	"strings"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the IDP service. Declarative resource loading and HTTP routing for
// identity providers now happen in the connection package (/connections/{vendor}), which is the
// sole owner of the "connection" declarative resource type.
func Initialize(
	cacheManager cache.CacheManagerInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
) (IDPServiceInterface, error) {
	// Create store and transactioner based on store mode
	idpStore, transactioner, err := initializeStore(cacheManager)
	if err != nil {
		return nil, err
	}

	idpService := newIDPService(idpStore, entityTypeService, transactioner)
	return idpService, nil
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
//
// Note: the file-based store here is populated by the connection package's declarative loader
// (backend/internal/connection/declarative_resource.go), not by this package — see
// ShouldLoadDeclarativeIDPResources.
func initializeStore(cacheManager cache.CacheManagerInterface) (idpStoreInterface, transaction.Transactioner, error) {
	storeMode := getIdentityProviderStoreMode()

	idpByIDCache := cache.GetCache[*providers.IDPDTO](cacheManager, "IDPByIDCache")
	idpByPropertyCache := cache.GetCache[[]providers.IDPDTO](cacheManager, "IDPByPropertyCache")

	switch storeMode {
	case serverconst.StoreModeComposite:
		fileStore, _ := newIDPFileBasedStore()
		dbStore, transactioner, err := newIDPStore()
		if err != nil {
			return nil, nil, err
		}
		idpStore := newCompositeIDPStore(fileStore, dbStore)
		return newCacheBackedIDPStore(idpByIDCache, idpByPropertyCache, idpStore), transactioner, nil

	case serverconst.StoreModeDeclarative:
		fileStore, transactioner := newIDPFileBasedStore()
		return newCacheBackedIDPStore(idpByIDCache, idpByPropertyCache, fileStore), transactioner, nil

	default:
		store, transactioner, err := newIDPStore()
		if err != nil {
			return nil, nil, err
		}
		return newCacheBackedIDPStore(idpByIDCache, idpByPropertyCache, store), transactioner, nil
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

// ShouldLoadDeclarativeIDPResources reports whether the identity-provider file-based store
// should be populated from declarative files — true when the resolved store mode (per
// getIdentityProviderStoreMode, honoring the identity_provider.store override) is composite or
// declarative. Called by the connection package's declarative loader, which is the sole owner of
// reading connection declarative files but must still respect this package's per-service store
// mode configuration.
func ShouldLoadDeclarativeIDPResources() bool {
	mode := getIdentityProviderStoreMode()
	return mode == serverconst.StoreModeComposite || mode == serverconst.StoreModeDeclarative
}
