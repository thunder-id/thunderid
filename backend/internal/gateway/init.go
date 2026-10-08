// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"net/http"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize wires the gateway store, service and routes.
//
// Which store depends on gateway.store:
//
//  1. mutable: the database alone. Gateways declared in files are not read.
//  2. declarative: files alone. Registration through the API is refused, because there is nowhere
//     to put it that a restart would not discard.
//  3. composite: both. Reads merge the two, registration writes to the database, and a gateway a
//     file declared cannot be changed or removed through the API.
//
// The exporter is what a version is captured through, so a version holds exactly what this plane's
// export writes: references on a control plane, template placeholders with their values elsewhere.
func Initialize(mux *http.ServeMux, exporter export.ExportServiceInterface) (ServiceInterface, error) {
	var (
		gatewayStore storeInterface
		fileStore    *gatewayFileStore
	)

	switch getGatewayStoreMode() {
	case serverconst.StoreModeDeclarative:
		fileStore = newFileStore()
		gatewayStore = fileStore
	case serverconst.StoreModeComposite:
		fileStore = newFileStore()
		gatewayStore = newCompositeStore(fileStore, newStore())
	default:
		gatewayStore = newStore()
	}

	service := newService(gatewayStore)
	versions := newVersionService(gatewayStore, newVersionStore(), exporter, newGatewayClient())
	h := newHandler(service)
	h.afterDelete = versions.Forget
	registerRoutes(mux, h)
	registerVersionRoutes(mux, newVersionHandler(versions))
	registerStoreRoutes(mux, newStoreHandler(newStoreService(gatewayStore, newGatewayClient())))

	// A gateway can also be declared in a file rather than registered through the API. The files are
	// read on every start into the in-memory store, so the file is the whole truth about what it
	// declares and re-reading replaces rather than accumulates.
	if fileStore != nil {
		if err := loadDeclarativeGateways(fileStore); err != nil {
			return service, err
		}
	}
	return service, nil
}

func registerRoutes(mux *http.ServeMux, h *handler) {
	noContent := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}

	collectionOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /gateways", h.handleList, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("POST /gateways", h.handleRegister, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways", noContent, collectionOpts))

	itemOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /gateways/{id}", h.handleGet, itemOpts))
	mux.HandleFunc(middleware.WithCORS("PUT /gateways/{id}", h.handleUpdate, itemOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE /gateways/{id}", h.handleDelete, itemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways/{id}", noContent, itemOpts))
}

func registerVersionRoutes(mux *http.ServeMux, h *versionHandler) {
	noContent := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	readOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	writeOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	collectionOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /configuration-versions", h.handleListVersions, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("POST /configuration-versions", h.handleCapture, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /configuration-versions", noContent, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("GET /configuration-versions/{version}", h.handleGetVersion, readOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /configuration-versions/{version}", noContent, readOpts))

	mux.HandleFunc(middleware.WithCORS("GET /gateways/{id}/applied-version", h.handleGetApplied, readOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways/{id}/applied-version", noContent, readOpts))
	mux.HandleFunc(middleware.WithCORS("GET /gateways/{id}/diff", h.handleDiff, readOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways/{id}/diff", noContent, readOpts))
	mux.HandleFunc(middleware.WithCORS("POST /gateways/{id}/apply", h.handleApply, writeOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways/{id}/apply", noContent, writeOpts))
	mux.HandleFunc(middleware.WithCORS("POST /gateways/{id}/revert", h.handleRevert, writeOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways/{id}/revert", noContent, writeOpts))
}

// registerStoreRoutes serves a gateway's variables and secrets, managed through this plane with the
// gateway's key.
func registerStoreRoutes(mux *http.ServeMux, h *storeHandler) {
	noContent := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	collectionOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	itemOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	routes := []struct {
		collection                     string
		list, create, get, set, remove http.HandlerFunc
	}{
		{collectionVariables, listHandler(h.service.ListVariables), createHandler(h.service.CreateVariable),
			getHandler(h.service.GetVariable), setHandler(h.service.SetVariable),
			deleteHandler(h.service.DeleteVariable)},
		{collectionSecrets, listHandler(h.service.ListSecrets), createHandler(h.service.CreateSecret),
			getHandler(h.service.GetSecret), setHandler(h.service.SetSecret),
			deleteHandler(h.service.DeleteSecret)},
	}
	for _, route := range routes {
		base := "/gateways/{id}/" + route.collection
		mux.HandleFunc(middleware.WithCORS("GET "+base, route.list, collectionOpts))
		mux.HandleFunc(middleware.WithCORS("POST "+base, route.create, collectionOpts))
		mux.HandleFunc(middleware.WithCORS("OPTIONS "+base, noContent, collectionOpts))
		item := base + "/{name}"
		mux.HandleFunc(middleware.WithCORS("GET "+item, route.get, itemOpts))
		mux.HandleFunc(middleware.WithCORS("PUT "+item, route.set, itemOpts))
		mux.HandleFunc(middleware.WithCORS("DELETE "+item, route.remove, itemOpts))
		mux.HandleFunc(middleware.WithCORS("OPTIONS "+item, noContent, itemOpts))
	}
}
