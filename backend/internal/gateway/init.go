// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"net/http"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
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
func Initialize(mux *http.ServeMux) (ServiceInterface, error) {
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
	registerRoutes(mux, newHandler(service))

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
