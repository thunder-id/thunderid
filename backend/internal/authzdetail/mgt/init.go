// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize initializes the authorization details type management service, routes, and MCP tools.
//
// valueCapturer is optional. Given one, the service hands it every type it creates or changes, so the
// processor credential the type's export refers to is kept where a reference finds it.
func Initialize(
	mux *http.ServeMux, mcpServer *mcp.Server, valueCapturer declarativeresource.ValueCapturer,
) (AuthorizationDetailMgtServiceInterface, declarativeresource.ResourceExporter, error) {
	typeStore, fileStore, dbStore := initializeStore()
	service := newAuthorizationDetailMgtService(typeStore, valueCapturer)

	if fileStore != nil {
		if err := loadDeclarativeResources(fileStore, dbStore); err != nil {
			return nil, nil, err
		}
	}

	registerRoutes(mux, newAuthorizationDetailHandler(service))
	if mcpServer != nil {
		registerMCPTools(mcpServer, service)
	}
	return service, newTypeExporter(service), nil
}

// initializeStore creates the type store for the configured store mode.
func initializeStore() (typeStoreInterface, *typeFileBasedStore, typeStoreInterface) {
	switch getAuthorizationDetailTypeStoreMode() {
	case serverconst.StoreModeComposite:
		fileStore := newTypeFileBasedStore()
		dbStore := newTypeStore()
		return newCompositeTypeStore(fileStore, dbStore), fileStore, dbStore
	case serverconst.StoreModeDeclarative:
		fileStore := newTypeFileBasedStore()
		return fileStore, fileStore, nil
	default:
		return newTypeStore(), nil, nil
	}
}

// registerRoutes registers the authorization details type management routes.
func registerRoutes(mux *http.ServeMux, handler *authorizationDetailHandler) {
	collectionOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /authorization-detail-types",
		handler.HandleTypeListRequest, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("POST /authorization-detail-types",
		handler.HandleTypePostRequest, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /authorization-detail-types",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, collectionOpts))

	itemOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /authorization-detail-types/{id}",
		handler.HandleTypeGetRequest, itemOpts))
	mux.HandleFunc(middleware.WithCORS("PUT /authorization-detail-types/{id}",
		handler.HandleTypePutRequest, itemOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE /authorization-detail-types/{id}",
		handler.HandleTypeDeleteRequest, itemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /authorization-detail-types/{id}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, itemOpts))
}
