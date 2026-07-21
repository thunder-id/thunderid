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

package group

import (
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// Initialize initializes the group service and registers its routes.
func Initialize(
	mux *http.ServeMux,
	dbProvider provider.DBProviderInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	entityService entity.EntityServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface,
) (GroupServiceInterface, oupkg.OUGroupResolver, declarativeresource.ResourceExporter, error) {
	// Step 1: Initialize store and transactioner based on store mode (no declarative loading yet).
	store, transactioner, fileStore, dbStore, err := initializeGroupStore(dbProvider)
	if err != nil {
		return nil, nil, nil, err
	}

	// Step 2: Create service with store.
	groupService := newGroupServiceWithStore(
		store, ouService, entityService, entityTypeService, authzService, transactioner,
	)

	// Step 3: Load declarative resources into file store (if applicable).
	if fileStore != nil {
		if err := loadDeclarativeResources(fileStore, dbStore, ouService); err != nil {
			return nil, nil, nil, err
		}
	}

	// Register the group store as the membership provider on the entity service so that
	// GetTransitiveEntityGroups is owned entirely by the group layer (DB + declarative).
	entityService.SetGroupMembershipProvider(store)

	// Create resolver for OU package to query group data without cross-DB access.
	ouGroupResolver := newOUGroupResolver(store)

	exporter := newGroupExporter(groupService)
	groupHandler := newGroupHandler(groupService)
	registerRoutes(mux, groupHandler)
	return groupService, ouGroupResolver, exporter, nil
}

// Store selection (based on group.store configuration):
//
// 1. MUTABLE mode (store: "mutable"):
//   - Uses database store only
//   - Supports full CRUD operations
//
// 2. DECLARATIVE mode (store: "declarative"):
//   - Uses file-based store only (from YAML resources)
//   - All groups are read-only
//
// 3. COMPOSITE mode (store: "composite" – hybrid):
//   - Uses both file-based store (immutable) + database store (mutable)
//   - YAML resources are loaded into file-based store (read-only)
//   - Database store handles runtime groups (mutable)
//   - Reads check both stores (merged results)
//   - Writes only go to database store
//
// Configuration fallback:
//   - If group.store is not set, falls back to global declarative_resources.enabled:
//   - true  -> DECLARATIVE
//   - false -> MUTABLE
//
// Returns the active group store, transactioner, and the file/db stores used for declarative
// resource loading. fileStore is non-nil only in declarative or composite modes; dbStore is
// non-nil only in composite mode.
func initializeGroupStore(
	dbProvider provider.DBProviderInterface,
) (groupStoreInterface, transaction.Transactioner, *fileBasedGroupStore, groupStoreInterface, error) {
	storeMode, err := getGroupStoreMode()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	switch storeMode {
	case serverconst.StoreModeComposite:
		fileStoreInterface, _ := newFileBasedGroupStore()
		fileStore := fileStoreInterface.(*fileBasedGroupStore)
		transactioner, err := dbProvider.GetEntityDBTransactioner()
		if err != nil {
			return nil, nil, nil, nil, err
		}
		dbStore := newGroupStore()
		compositeStore := newCompositeGroupStore(fileStoreInterface, dbStore)
		return compositeStore, transactioner, fileStore, dbStore, nil

	case serverconst.StoreModeDeclarative:
		fileStoreInterface, transactioner := newFileBasedGroupStore()
		fileStore := fileStoreInterface.(*fileBasedGroupStore)
		return fileStoreInterface, transactioner, fileStore, nil, nil

	default:
		transactioner, err := dbProvider.GetEntityDBTransactioner()
		if err != nil {
			return nil, nil, nil, nil, err
		}
		return newGroupStore(), transactioner, nil, nil, nil
	}
}

// registerRoutes registers the routes for group management operations.
func registerRoutes(mux *http.ServeMux, groupHandler *groupHandler) {
	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /groups", groupHandler.HandleGroupPostRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("GET /groups", groupHandler.HandleGroupListRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /groups", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts1))

	opts2 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	// Special handling for /groups/{id} and /groups/{id}/members
	mux.HandleFunc(middleware.WithCORS("GET /groups/",
		func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/groups/")
			segments := strings.Split(path, "/")
			r.SetPathValue("id", segments[0])

			if len(segments) == 1 {
				groupHandler.HandleGroupGetRequest(w, r)
			} else if len(segments) == 2 && segments[1] == "members" {
				groupHandler.HandleGroupMembersGetRequest(w, r)
			} else {
				http.NotFound(w, r)
			}
		}, opts2))
	mux.HandleFunc(middleware.WithCORS("PUT /groups/{id}", groupHandler.HandleGroupPutRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("DELETE /groups/{id}", groupHandler.HandleGroupDeleteRequest, opts2))
	// Handle OPTIONS preflight for /groups/{id} and /groups/{id}/members using the same
	// catch-all pattern as the GET handler above, to avoid conflicts with /groups/tree/{path...}.
	mux.HandleFunc(middleware.WithCORS("OPTIONS /groups/",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts2))

	opts3 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /groups/tree/{path...}",
		groupHandler.HandleGroupListByPathRequest, opts3))
	mux.HandleFunc(middleware.WithCORS("POST /groups/tree/{path...}",
		groupHandler.HandleGroupPostByPathRequest, opts3))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /groups/tree/{path...}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts3))

	// POST routes for /groups/{id}/members/add and /groups/{id}/members/remove.
	// These use a catch-all pattern to avoid route conflicts with /groups/tree/{path...}.
	opts4 := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /groups/",
		func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/groups/")
			segments := strings.Split(path, "/")

			// Match /groups/{id}/members/add and /groups/{id}/members/remove
			if len(segments) == 3 && segments[0] != "" && segments[1] == "members" {
				r.SetPathValue("id", segments[0])
				switch segments[2] {
				case "add":
					groupHandler.HandleGroupMembersAddRequest(w, r)
				case "remove":
					groupHandler.HandleGroupMembersRemoveRequest(w, r)
				default:
					http.NotFound(w, r)
				}
			} else {
				http.NotFound(w, r)
			}
		}, opts4))
}
