// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/group"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/sharing"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the role service and registers its routes.
func Initialize(
	mux *http.ServeMux,
	entityService entity.EntityServiceInterface,
	groupService group.GroupServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	resourceService resourcepkg.ResourceServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface,
	sharingService sharing.SharingServiceInterface,
) (
	RoleServiceInterface, RoleAssignmentServiceInterface, oupkg.OURoleResolver,
	declarativeresource.ResourceExporter, error,
) {
	// Step 1: Initialize store and transactioner based on store mode (no declarative loading yet)
	roleStore, transactioner, fileStore, dbStore, err := initializeStore()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Step 2: Create service with store
	roleService := newRoleService(
		roleStore, entityService, groupService, ouService, resourceService, transactioner, authzService,
		sharingService)

	// Registered before declarative resources load, because loading seeds the sharing policies those
	// files declare and the framework refuses a type it does not know.
	sharingService.RegisterResourceType(newRoleSharingDeclaration(roleService))

	// Step 3: Load declarative resources into store (if applicable)
	if fileStore != nil {
		// One document, read once per thing it carries: the role, then its sharing policies.
		if err := loadDeclarativeResources(fileStore, dbStore, roleService); err != nil {
			return nil, nil, nil, nil, err
		}
		if err := sharingService.LoadDeclarativeResources(
			context.Background(), makeRoleSharingConfig(roleService)); err != nil {
			return nil, nil, nil, nil, err
		}
	}

	assignmentService := newRoleAssignmentService(
		roleStore, entityService, groupService, entityTypeService, resourceService, transactioner, authzService,
	)
	roleHandler := newRoleHandler(roleService, assignmentService)
	registerRoutes(mux, roleHandler)
	exporter := newRoleExporter(roleService, assignmentService, sharingService)
	ouRoleResolver := newOURoleResolver(roleStore)
	return roleService, assignmentService, ouRoleResolver, exporter, nil
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
//
// Returns the active role store, transactioner, and the file/db stores used for declarative
// resource loading. fileStore is non-nil only in declarative or composite modes; dbStore is non-nil
// only in composite mode. Callers in those modes invoke loadDeclarativeResources after the
// role service has been constructed so it can resolve ou_handle.
func initializeStore() (
	roleStoreInterface, providers.Transactioner, *fileBasedStore, roleStoreInterface, error,
) {
	storeMode := getRoleStoreMode()

	switch storeMode {
	case serverconst.StoreModeComposite:
		fileStoreInterface, _ := newFileBasedStore()
		fileStore := fileStoreInterface.(*fileBasedStore)
		dbStore, transactioner, err := newRoleStore()
		if err != nil {
			return nil, nil, nil, nil, err
		}
		roleStore := newCompositeRoleStore(fileStoreInterface, dbStore)
		return roleStore, transactioner, fileStore, dbStore, nil

	case serverconst.StoreModeDeclarative:
		fileStoreInterface, transactioner := newFileBasedStore()
		fileStore := fileStoreInterface.(*fileBasedStore)
		return fileStoreInterface, transactioner, fileStore, nil, nil

	default:
		store, transactioner, err := newRoleStore()
		if err != nil {
			return nil, nil, nil, nil, err
		}
		return store, transactioner, nil, nil, nil
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

	// Sharing policies live under the role they govern: a policy has no life of its own.
	opts5 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /roles/{id}/sharing-policies",
		roleHandler.HandleSharingPolicyPostRequest, opts5))
	mux.HandleFunc(middleware.WithCORS("GET /roles/{id}/sharing-policies",
		roleHandler.HandleSharingPolicyListRequest, opts5))
	// Without its own preflight route this collection falls to OPTIONS /roles/{id}, which does not
	// allow POST, and the browser blocks the create.
	mux.HandleFunc(middleware.WithCORS("OPTIONS /roles/{id}/sharing-policies",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts5))
	mux.HandleFunc(middleware.WithCORS("GET /roles/{id}/overlay-rules",
		roleHandler.HandleOverlayRuleGetRequest, opts5))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /roles/{id}/overlay-rules",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts5))

	opts6 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /roles/{id}/sharing-policies/{policyId}",
		roleHandler.HandleSharingPolicyGetRequest, opts6))
	mux.HandleFunc(middleware.WithCORS("PUT /roles/{id}/sharing-policies/{policyId}",
		roleHandler.HandleSharingPolicyPutRequest, opts6))
	mux.HandleFunc(middleware.WithCORS("DELETE /roles/{id}/sharing-policies/{policyId}",
		roleHandler.HandleSharingPolicyDeleteRequest, opts6))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /roles/{id}/sharing-policies/{policyId}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts6))
}
