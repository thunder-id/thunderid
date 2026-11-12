// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outype

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

const ouTypesPath = "/organization-unit-types"

// Initialize builds the organization unit type service over the entity type service and registers the
// organization unit type management routes.
func Initialize(
	mux *http.ServeMux, entityTypeService entitytype.EntityTypeServiceInterface,
) OUTypeServiceInterface {
	service := newOUTypeService(entityTypeService)
	registerRoutes(mux, newOUTypeHandler(service))
	return service
}

// registerRoutes registers the organization unit type management routes.
func registerRoutes(mux *http.ServeMux, h *ouTypeHandler) {
	collectionOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	resourceOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("POST "+ouTypesPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleOUTypePostRequest)).ServeHTTP, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("GET "+ouTypesPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleOUTypeListRequest)).ServeHTTP, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("GET "+ouTypesPath+"/{id}",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleOUTypeGetRequest)).ServeHTTP, resourceOpts))
	mux.HandleFunc(middleware.WithCORS("PUT "+ouTypesPath+"/{id}",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleOUTypePutRequest)).ServeHTTP, resourceOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE "+ouTypesPath+"/{id}",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleOUTypeDeleteRequest)).ServeHTTP, resourceOpts))

	mux.HandleFunc(middleware.WithCORS("OPTIONS "+ouTypesPath,
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+ouTypesPath+"/{id}",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, resourceOpts))
}
