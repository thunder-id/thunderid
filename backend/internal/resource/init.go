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

package resource

import (
	"fmt"
	"net/http"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// Initialize initializes the resource service and registers its routes.
// Returns the service interface and resource server exporter for declarative resource export functionality.
func Initialize(
	mux *http.ServeMux,
	ouService oupkg.OrganizationUnitServiceInterface,
) (ResourceServiceInterface, declarativeresource.ResourceExporter, error) {
	// Initialize store and transactioner based on store mode
	resourceStore, transactioner, err := initializeStore()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize resource store: %w", err)
	}

	resourceService, err := newResourceService(ouService, resourceStore, transactioner)
	if err != nil {
		return nil, nil, err
	}

	// Load declarative resources if applicable (declarative or composite mode)
	storeMode := getResourceStoreMode()
	if storeMode == serverconst.StoreModeDeclarative || storeMode == serverconst.StoreModeComposite {
		if err := loadDeclarativeResources(resourceStore, resourceService); err != nil {
			return nil, nil, fmt.Errorf("failed to load declarative resources: %w", err)
		}
	}

	// Create exporter for declarative resource export functionality
	exporter := newResourceServerExporter(resourceService)

	resourceHandler := newResourceHandler(resourceService)
	registerRoutes(mux, resourceHandler)

	return resourceService, exporter, nil
}

// initializeStore creates and initializes the appropriate store based on configuration.
func initializeStore() (resourceStoreInterface, transaction.Transactioner, error) {
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
