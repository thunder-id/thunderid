// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"fmt"
	"net/http"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the resource service and registers its routes.
// Returns the service interface and resource server exporter for declarative resource export functionality.
func Initialize(
	mux *http.ServeMux,
	ouService oupkg.OrganizationUnitServiceInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface,
	sharingService sharing.ServiceInterface,
) (ResourceServiceInterface, declarativeresource.ResourceExporter, error) {
	// Initialize store and transactioner based on store mode
	resourceStore, transactioner, err := initializeStore()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize resource store: %w", err)
	}

	resourceService, err := newResourceService(
		ouService, authzService, sharingService, resourceStore, transactioner)
	if err != nil {
		return nil, nil, err
	}

	// The declaration is registered before declarative resources load, because loading seeds the
	// sharing policies those files declare and the framework refuses a type it does not know.
	if sharingService != nil {
		sharingService.RegisterResourceType(newResourceServerSharing(resourceService))
	}

	// Load declarative resources if applicable (declarative or composite mode)
	storeMode := getResourceStoreMode()
	if storeMode == serverconst.StoreModeDeclarative || storeMode == serverconst.StoreModeComposite {
		if err := loadDeclarativeResources(resourceStore, resourceService, sharingService); err != nil {
			return nil, nil, fmt.Errorf("failed to load declarative resources: %w", err)
		}
	}

	// Create exporter for declarative resource export functionality
	exporter := newResourceServerExporter(resourceService)

	resourceHandler := newResourceHandler(resourceService, sharingService)
	registerRoutes(mux, resourceHandler)

	// Sharing is optional so the resource service still starts when the framework is not wired,
	// which keeps the two initialization orders independent.
	if sharingService != nil {
		registerSharingRoutes(mux, newSharingHandler(resourceService, sharingService))
	}

	return resourceService, exporter, nil
}

// initializeStore creates and initializes the appropriate store based on configuration.
func initializeStore() (resourceStoreInterface, providers.Transactioner, error) {
	storeMode := getResourceStoreMode()

	switch storeMode {
	case serverconst.StoreModeMutable:
		return newResourceStore()
	case serverconst.StoreModeDeclarative:
		return newFileBasedResourceStore()
	case serverconst.StoreModeComposite:
		fileStore, _, err := newFileBasedResourceStore()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create file-based store: %w", err)
		}
		dbStore, transactioner, err := newResourceStore()
		if err != nil {
			return nil, nil, err
		}
		return newCompositeResourceStore(fileStore, dbStore), transactioner, nil
	default:
		return nil, nil, fmt.Errorf("unsupported store mode: %s", storeMode)
	}
}

// registerRoutes registers all routes for the resource management API.
func registerRoutes(mux *http.ServeMux, handler *resourceHandler) {
	// Resource Server routes
	resourceServerOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /resource-servers",
		handler.HandleResourceServerListRequest, resourceServerOpts))
	mux.HandleFunc(middleware.WithCORS("POST /resource-servers",
		handler.HandleResourceServerPostRequest, resourceServerOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /resource-servers",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, resourceServerOpts))

	resourceServerDetailOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /resource-servers/{id}",
		handler.HandleResourceServerGetRequest, resourceServerDetailOpts))
	mux.HandleFunc(middleware.WithCORS("PUT /resource-servers/{id}",
		handler.HandleResourceServerPutRequest, resourceServerDetailOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE /resource-servers/{id}",
		handler.HandleResourceServerDeleteRequest, resourceServerDetailOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /resource-servers/{id}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, resourceServerDetailOpts))

	// Resource routes
	resourceOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /resource-servers/{rsId}/resources",
		handler.HandleResourceListRequest, resourceOpts))
	mux.HandleFunc(middleware.WithCORS("POST /resource-servers/{rsId}/resources",
		handler.HandleResourcePostRequest, resourceOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /resource-servers/{rsId}/resources",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, resourceOpts))

	resourceDetailOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /resource-servers/{rsId}/resources/{id}",
		handler.HandleResourceGetRequest, resourceDetailOpts))
	mux.HandleFunc(middleware.WithCORS("PUT /resource-servers/{rsId}/resources/{id}",
		handler.HandleResourcePutRequest, resourceDetailOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE /resource-servers/{rsId}/resources/{id}",
		handler.HandleResourceDeleteRequest, resourceDetailOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /resource-servers/{rsId}/resources/{id}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, resourceDetailOpts))

	// Action routes (Resource Server level)
	actionRSOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /resource-servers/{rsId}/actions",
		handler.HandleActionListAtResourceServerRequest, actionRSOpts))
	mux.HandleFunc(middleware.WithCORS("POST /resource-servers/{rsId}/actions",
		handler.HandleActionPostAtResourceServerRequest, actionRSOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /resource-servers/{rsId}/actions",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, actionRSOpts))

	actionRSDetailOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /resource-servers/{rsId}/actions/{id}",
		handler.HandleActionGetAtResourceServerRequest, actionRSDetailOpts))
	mux.HandleFunc(middleware.WithCORS("PUT /resource-servers/{rsId}/actions/{id}",
		handler.HandleActionPutAtResourceServerRequest, actionRSDetailOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE /resource-servers/{rsId}/actions/{id}",
		handler.HandleActionDeleteAtResourceServerRequest, actionRSDetailOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /resource-servers/{rsId}/actions/{id}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, actionRSDetailOpts))

	// Action routes (Resource level)
	actionResourceOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /resource-servers/{rsId}/resources/{resourceId}/actions",
		handler.HandleActionListAtResourceRequest, actionResourceOpts))
	mux.HandleFunc(middleware.WithCORS("POST /resource-servers/{rsId}/resources/{resourceId}/actions",
		handler.HandleActionPostAtResourceRequest, actionResourceOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /resource-servers/{rsId}/resources/{resourceId}/actions",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, actionResourceOpts))

	actionResourceDetailOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /resource-servers/{rsId}/resources/{resourceId}/actions/{id}",
		handler.HandleActionGetAtResourceRequest, actionResourceDetailOpts))
	mux.HandleFunc(middleware.WithCORS("PUT /resource-servers/{rsId}/resources/{resourceId}/actions/{id}",
		handler.HandleActionPutAtResourceRequest, actionResourceDetailOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE /resource-servers/{rsId}/resources/{resourceId}/actions/{id}",
		handler.HandleActionDeleteAtResourceRequest, actionResourceDetailOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /resource-servers/{rsId}/resources/{resourceId}/actions/{id}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, actionResourceDetailOpts))
}

// registerSharingRoutes registers the sharing-policy endpoints of the resource-server API.
//
// The paths sit under the resource server they govern, so they inherit the same permission entries
// as every other write to it rather than needing a tier of their own.
func registerSharingRoutes(mux *http.ServeMux, handler *sharingHandler) {
	policiesOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	policyOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	overlayOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("POST /resource-servers/{id}/sharing-policies",
		handler.HandleSharingPolicyPostRequest, policiesOpts))
	mux.HandleFunc(middleware.WithCORS("GET /resource-servers/{id}/sharing-policies",
		handler.HandleSharingPolicyListRequest, policiesOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /resource-servers/{id}/sharing-policies",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, policiesOpts))

	mux.HandleFunc(middleware.WithCORS("GET /resource-servers/{id}/sharing-policies/{policyId}",
		handler.HandleSharingPolicyGetRequest, policyOpts))
	mux.HandleFunc(middleware.WithCORS("PUT /resource-servers/{id}/sharing-policies/{policyId}",
		handler.HandleSharingPolicyPutRequest, policyOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE /resource-servers/{id}/sharing-policies/{policyId}",
		handler.HandleSharingPolicyDeleteRequest, policyOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /resource-servers/{id}/sharing-policies/{policyId}",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, policyOpts))

	mux.HandleFunc(middleware.WithCORS("GET /resource-servers/{id}/overlay-rules",
		handler.HandleSharingOverlayGetRequest, overlayOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /resource-servers/{id}/overlay-rules",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, overlayOpts))
}
