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

package role

import (
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/group"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// Initialize initializes the role service and registers its routes.
func Initialize(
	mux *http.ServeMux,
	entityService entity.EntityServiceInterface,
	groupService group.GroupServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	resourceService resourcepkg.ResourceServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
) (RoleServiceInterface, RoleAssignmentServiceInterface, declarativeresource.ResourceExporter, error) {
	// Step 1: Initialize store and transactioner based on store mode
	roleStore, transactioner, err := initializeStore()
	if err != nil {
		return nil, nil, nil, err
	}

	// Step 2: Create service with store
	roleService := newRoleService(
		roleStore, entityService, groupService, ouService, resourceService,
		transactioner,
	)
	assignmentService := newRoleAssignmentService(
		roleStore, entityService, groupService, entityTypeService, transactioner,
	)
	roleHandler := newRoleHandler(roleService, assignmentService)
	registerRoutes(mux, roleHandler)
	exporter := newRoleExporter(roleService, assignmentService)
	return roleService, assignmentService, exporter, nil
}

// Store Selection (based on role.store configuration):
//
// 1. MUTABLE mode (store: "mutable"):
//   - Uses database store only
//   - Supports full CRUD operations (Create/Read/Update/Delete)
//   - All roles are mutable
//
// 2. IMMUTABLE mode (store: "declarative"):
//   - Uses file-based store only (from YAML resources)
//   - All roles are immutable (read-only)
//   - No create/update/delete operations allowed
//
// 3. COMPOSITE mode (store: "composite" - hybrid):
//   - Uses both file-based store (immutable) + database store (mutable)
//   - YAML resources are loaded into file-based store (immutable, read-only)
//   - Database store handles runtime roles (mutable)
//   - Reads check both stores (merged results)
//   - Writes only go to database store
//   - Declarative roles cannot be updated or deleted
//
// Configuration Fallback:
// - If role.store is not specified, falls back to global declarative_resources.enabled:
//   - If declarative_resources.enabled = true: behaves as IMMUTABLE mode
//   - If declarative_resources.enabled = false: behaves as MUTABLE mode
func initializeStore() (roleStoreInterface, transaction.Transactioner, error) {
	storeMode := getRoleStoreMode()

	switch storeMode {
	case serverconst.StoreModeComposite:
		fileStoreInterface, _ := newFileBasedStore()
		fileStore := fileStoreInterface.(*fileBasedStore)
		dbStore, transactioner, err := newRoleStore()
		if err != nil {
			return nil, nil, err
		}
		roleStore := newCompositeRoleStore(fileStoreInterface, dbStore)
		if err := loadDeclarativeResources(fileStore, dbStore); err != nil {
			return nil, nil, err
		}
		return roleStore, transactioner, nil

	case serverconst.StoreModeDeclarative:
		fileStoreInterface, transactioner := newFileBasedStore()
		fileStore := fileStoreInterface.(*fileBasedStore)
		if err := loadDeclarativeResources(fileStore, nil); err != nil {
			return nil, nil, err
		}
		return fileStoreInterface, transactioner, nil

	default:
		return newRoleStore()
	}
}

// registerRoutes registers the routes for role management operations.
func registerRoutes(mux *http.ServeMux, roleHandler *roleHandler) {
	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /roles", roleHandler.HandleRolePostRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("GET /roles", roleHandler.HandleRoleListRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /roles", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts1))

	opts2 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	// Special handling for /roles/{id} and /roles/{id}/assignments
	mux.HandleFunc(middleware.WithCORS("GET /roles/",
		func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/roles/")
			segments := strings.Split(path, "/")
			r.SetPathValue("id", segments[0])

			if len(segments) == 1 {
				roleHandler.HandleRoleGetRequest(w, r)
			} else if len(segments) == 2 && segments[1] == "assignments" {
				roleHandler.HandleRoleAssignmentsGetRequest(w, r)
			} else {
				http.NotFound(w, r)
			}
		}, opts2))
	mux.HandleFunc(middleware.WithCORS("PUT /roles/{id}", roleHandler.HandleRolePutRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("DELETE /roles/{id}", roleHandler.HandleRoleDeleteRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /roles/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts2))
	opts4 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("OPTIONS /roles/{id}/assignments", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts4))

	opts3 := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /roles/{id}/assignments/add",
		roleHandler.HandleRoleAddAssignmentsRequest, opts3))
	mux.HandleFunc(middleware.WithCORS("POST /roles/{id}/assignments/remove",
		roleHandler.HandleRoleRemoveAssignmentsRequest, opts3))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /roles/{id}/assignments/add",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts3))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /roles/{id}/assignments/remove",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts3))
}
