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

package entitytype

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/cache"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// Initialize initializes the entity type service and registers its routes.
func Initialize(
	mux *http.ServeMux,
	mcpServer *mcp.Server,
	cacheManager cache.CacheManagerInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface,
) (EntityTypeServiceInterface, declarativeresource.ResourceExporter, error) {
	// Step 1: Determine store mode and initialize store and transactioner
	storeMode := getEntityTypeStoreMode()
	entityTypeStore, transactioner, err := initializeStore(storeMode, cacheManager)
	if err != nil {
		return nil, nil, err
	}

	// Step 2: Create service with store
	entityTypeService := newEntityTypeService(ouService, entityTypeStore, transactioner,
		authzService)

	// Step 3: Load declarative resources into store (if applicable)
	if storeMode == serverconst.StoreModeComposite || storeMode == serverconst.StoreModeDeclarative {
		if err := loadDeclarativeResources(entityTypeStore, entityTypeService); err != nil {
			return nil, nil, err
		}
	}

	userTypeHandler := newEntityTypeHandler(entityTypeService, TypeCategoryUser)
	agentTypeHandler := newEntityTypeHandler(entityTypeService, TypeCategoryAgent)
	registerUserTypeRoutes(mux, userTypeHandler)
	registerAgentTypeRoutes(mux, agentTypeHandler)

	if mcpServer != nil {
		registerMCPTools(mcpServer, entityTypeService)
	}

	exporter := newEntityTypeExporter(entityTypeService)
	return entityTypeService, exporter, nil
}

// Store Selection (based on entity_type.store configuration):
//
// 1. MUTABLE mode (store: "mutable"):
//   - Uses database store only
//   - Supports full CRUD operations (Create/Read/Update/Delete)
//   - All entity types are mutable
//   - Export functionality exports DB-backed entity types
//
// 2. IMMUTABLE mode (store: "declarative"):
//   - Uses file-based store only (from YAML resources)
//   - All entity types are immutable (read-only)
//   - No create/update/delete operations allowed
//   - Export functionality not applicable
//
// 3. COMPOSITE mode (store: "composite" - hybrid):
//   - Uses both file-based store (immutable) + database store (mutable)
//   - YAML resources are loaded into file-based store (immutable, read-only)
//   - Database store handles runtime entity types (mutable)
//   - Reads check both stores (merged results)
//   - Writes only go to database store
//   - Declarative entity types cannot be updated or deleted
//   - Export only exports DB-backed entity types (not YAML)
//
// Configuration Fallback:
// - If entity_type.store is not specified, falls back to global declarative_resources.enabled:
//   - If declarative_resources.enabled = true: behaves as IMMUTABLE mode
//   - If declarative_resources.enabled = false: behaves as MUTABLE mode
func initializeStore(storeMode serverconst.StoreMode, cacheManager cache.CacheManagerInterface) (
	entityTypeStoreInterface, transaction.Transactioner, error) {
	entityTypeByIDCache := cache.GetCache[*EntityType](cacheManager, "EntityTypeByIDCache")
	entityTypeByNameCache := cache.GetCache[*EntityType](cacheManager, "EntityTypeByNameCache")

	switch storeMode {
	case serverconst.StoreModeComposite:
		fileStore, _ := newEntityTypeFileBasedStore()
		dbStore, transactioner, err := newEntityTypeStore()
		if err != nil {
			return nil, nil, err
		}
		cachedDBStore := newCachedBackedEntityTypeStore(dbStore, entityTypeByIDCache, entityTypeByNameCache)
		return newCompositeEntityTypeStore(fileStore, cachedDBStore), transactioner, nil

	case serverconst.StoreModeDeclarative:
		fileStore, transactioner := newEntityTypeFileBasedStore()
		return fileStore, transactioner, nil

	default:
		dbStore, transactioner, err := newEntityTypeStore()
		if err != nil {
			return nil, nil, err
		}
		return newCachedBackedEntityTypeStore(dbStore, entityTypeByIDCache, entityTypeByNameCache), transactioner, nil
	}
}

// registerSchemaRoutes registers the CRUD routes for an entity type handler under the given URL
// base path. Used to wire both /user-types and /agent-types to category-bound handler instances
// that share the same code path.
func registerSchemaRoutes(mux *http.ServeMux, basePath string, h *entityTypeHandler) {
	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST "+basePath, h.HandleEntityTypePostRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("GET "+basePath, h.HandleEntityTypeListRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+basePath,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts1))

	opts2 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET "+basePath+"/{id}", h.HandleEntityTypeGetRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("PUT "+basePath+"/{id}", h.HandleEntityTypePutRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("DELETE "+basePath+"/{id}", h.HandleEntityTypeDeleteRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+basePath+"/{id}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts2))
}

func registerUserTypeRoutes(mux *http.ServeMux, h *entityTypeHandler) {
	registerSchemaRoutes(mux, "/user-types", h)
}

func registerAgentTypeRoutes(mux *http.ServeMux, h *entityTypeHandler) {
	registerSchemaRoutes(mux, "/agent-types", h)
}
