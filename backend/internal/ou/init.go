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

package ou

import (
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/system/cache"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// Initialize initializes the organization unit service and registers its routes.
// It returns the service, a hierarchy resolver (for injection into the authz service to
// avoid an import cycle), and the declarative resource exporter.
func Initialize(
	mux *http.ServeMux,
	mcpServer *mcp.Server,
	cacheManager cache.CacheManagerInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface,
) (ConfigurableOUService, sysauthz.OUHierarchyResolver, declarativeresource.ResourceExporter, error) {
	ouStore, transactioner, err := initializeStore(cacheManager)
	if err != nil {
		return nil, nil, nil, err
	}

	ouService := newOrganizationUnitService(authzService, ouStore, transactioner)

	ouHandler := newOrganizationUnitHandler(ouService)
	registerRoutes(mux, ouHandler)

	if mcpServer != nil {
		registerMCPTools(mcpServer, ouService)
	}

	// Create the hierarchy resolver backed directly by the store (no authz checks) so
	// the authz service can traverse the OU tree without recursive authorization calls.
	hierarchyResolver := newOUHierarchyAdapter(ouStore)

	// Create and return exporter
	exporter := newOUExporter(ouService)
	return ouService, hierarchyResolver, exporter, nil
}

// Store Selection (based on organization_unit.store configuration):
//
// 1. MUTABLE mode (store: "mutable"):
//   - Uses database store only (organizationUnitStore)
//   - Supports full CRUD operations (Create/Read/Update/Delete)
//   - All OUs are mutable
//   - Export functionality exports DB-backed OUs
//
// 2. IMMUTABLE mode (store: "declarative"):
//   - Uses file-based store only (from YAML resources)
//   - All OUs are immutable (read-only)
//   - No create/update/delete operations allowed
//   - Export functionality not applicable
//
// 3. COMPOSITE mode (store: "composite" - hybrid):
//   - Uses both file-based store (immutable) + database store (mutable)
//   - YAML resources are loaded into file-based store (immutable, read-only)
//   - Database store handles runtime OUs (mutable)
//   - Reads check both stores (merged results)
//   - Writes only go to database store
//   - Declarative OUs cannot be updated or deleted
//   - Export only exports DB-backed OUs (not YAML)
//
// Configuration Fallback:
// - If organization_unit.store is not specified, falls back to global immutable_resources.enabled:
//   - If immutable_resources.enabled = true: behaves as IMMUTABLE mode
//   - If immutable_resources.enabled = false: behaves as MUTABLE mode
func initializeStore(
	cacheManager cache.CacheManagerInterface,
) (organizationUnitStoreInterface, transaction.Transactioner, error) {
	storeMode := getOrganizationUnitStoreMode()

	switch storeMode {
	case serverconst.StoreModeComposite:
		fileStore, _ := newFileBasedStore()
		dbStore, transactioner, err := newOrganizationUnitStore()
		if err != nil {
			return nil, nil, err
		}
		cachedDBStore := wrapWithCache(dbStore, cacheManager)
		ouStore := newCompositeOUStore(fileStore, cachedDBStore)
		if err := loadDeclarativeResources(fileStore, cachedDBStore); err != nil {
			return nil, nil, err
		}
		return ouStore, transactioner, nil

	case serverconst.StoreModeDeclarative:
		fileStore, transactioner := newFileBasedStore()
		if err := loadDeclarativeResources(fileStore, nil); err != nil {
			return nil, nil, err
		}
		return fileStore, transactioner, nil

	default:
		dbStore, transactioner, err := newOrganizationUnitStore()
		if err != nil {
			return nil, nil, err
		}
		return wrapWithCache(dbStore, cacheManager), transactioner, nil
	}
}

// wrapWithCache wraps the given store with a cache-backed store if a cache manager is provided.
func wrapWithCache(
	store organizationUnitStoreInterface, cacheManager cache.CacheManagerInterface,
) organizationUnitStoreInterface {
	if cacheManager == nil {
		return store
	}
	ouByIDCache := cache.GetCache[*providers.OrganizationUnit](cacheManager, "OUByIDCache")
	ouByHandleParentCache := cache.GetCache[*providers.OrganizationUnit](cacheManager, "OUByHandleParentCache")
	return newCacheBackedOUStore(store, ouByIDCache, ouByHandleParentCache)
}

// registerRoutes registers the routes for organization unit management operations.
func registerRoutes(mux *http.ServeMux, ouHandler *organizationUnitHandler) {
	corsOptions1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /organization-units",
		ouHandler.HandleOUPostRequest, corsOptions1))
	mux.HandleFunc(middleware.WithCORS("GET /organization-units",
		ouHandler.HandleOUListRequest, corsOptions1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /organization-units",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, corsOptions1))

	corsOptions2 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /organization-units/",
		func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/organization-units/")
			segments := strings.Split(path, "/")
			r.SetPathValue("id", segments[0])

			if len(segments) == 1 {
				ouHandler.HandleOUGetRequest(w, r)
			} else if len(segments) == 2 {
				switch segments[1] {
				case "ous":
					ouHandler.HandleOUChildrenListRequest(w, r)
				case "users":
					ouHandler.HandleOUUsersListRequest(w, r)
				case "groups":
					ouHandler.HandleOUGroupsListRequest(w, r)
				case "roles":
					ouHandler.HandleOURolesListRequest(w, r)
				default:
					http.NotFound(w, r)
				}
			} else {
				http.NotFound(w, r)
			}
		}, corsOptions2))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /organization-units/",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, corsOptions2))
	mux.HandleFunc(middleware.WithCORS("PUT /organization-units/{id}",
		ouHandler.HandleOUPutRequest, corsOptions2))
	mux.HandleFunc(middleware.WithCORS("DELETE /organization-units/{id}",
		ouHandler.HandleOUDeleteRequest, corsOptions2))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /organization-units/{id}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, corsOptions2))

	mux.HandleFunc(middleware.WithCORS("GET /organization-units/tree/{path...}",
		func(w http.ResponseWriter, r *http.Request) {
			pathValue := r.PathValue("path")
			handlers := map[string]func(http.ResponseWriter, *http.Request){
				"/ous":    ouHandler.HandleOUChildrenListByPathRequest,
				"/users":  ouHandler.HandleOUUsersListByPathRequest,
				"/groups": ouHandler.HandleOUGroupsListByPathRequest,
				"/roles":  ouHandler.HandleOURolesListByPathRequest,
			}

			for suffix, handlerFunc := range handlers {
				if strings.HasSuffix(pathValue, suffix) {
					newPath := strings.TrimSuffix(pathValue, suffix)
					r.SetPathValue("path", newPath)
					handlerFunc(w, r)
					return
				}
			}

			newPath := "/organization-units/tree/" + pathValue
			r.URL.Path = newPath
			ouHandler.HandleOUGetByPathRequest(w, r)
		}, corsOptions2))
	mux.HandleFunc(middleware.WithCORS("PUT /organization-units/tree/{path...}",
		ouHandler.HandleOUPutByPathRequest, corsOptions2))
	mux.HandleFunc(middleware.WithCORS("DELETE /organization-units/tree/{path...}",
		ouHandler.HandleOUDeleteByPathRequest, corsOptions2))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /organization-units/tree/{path...}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, corsOptions2))
}
