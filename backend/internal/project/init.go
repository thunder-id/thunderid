// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/ou"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize builds the project service over the database store and registers the project
// management routes and MCP tools. The returned service is also a resource dependency provider, so that a
// project's root organization unit cannot be deleted outside the project API. The returned exporter
// exports projects as declarative resources.
func Initialize(
	mux *http.ServeMux, mcpServer *mcp.Server, ouService ou.OrganizationUnitServiceInterface,
) (ProjectServiceInterface, declarativeresource.ResourceExporter, error) {
	store, transactioner, err := newProjectStore()
	if err != nil {
		return nil, nil, err
	}

	service := newProjectService(store, transactioner, ouService)
	registerRoutes(mux, newProjectHandler(service))
	if mcpServer != nil {
		registerMCPTools(mcpServer, service)
	}
	return service, newProjectExporter(service), nil
}

// registerRoutes registers the project management routes.
func registerRoutes(mux *http.ServeMux, h *projectHandler) {
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

	mux.HandleFunc(middleware.WithCORS("POST "+projectsPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleProjectPostRequest)).ServeHTTP, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("GET "+projectsPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleProjectListRequest)).ServeHTTP, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("GET "+projectsPath+"/{id}",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleProjectGetRequest)).ServeHTTP, resourceOpts))
	mux.HandleFunc(middleware.WithCORS("PUT "+projectsPath+"/{id}",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleProjectPutRequest)).ServeHTTP, resourceOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE "+projectsPath+"/{id}",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleProjectDeleteRequest)).ServeHTTP, resourceOpts))

	mux.HandleFunc(middleware.WithCORS("OPTIONS "+projectsPath,
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+projectsPath+"/{id}",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, resourceOpts))
}
