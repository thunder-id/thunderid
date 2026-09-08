// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize wires the gateway store, service and routes. The exporter is what an apply sends: the
// configuration this control plane currently holds.
func Initialize(mux *http.ServeMux, exporter export.ExportServiceInterface) (ServiceInterface, error) {
	service := newService(newStore(), exporter, newDataPlaneClient())
	registerRoutes(mux, newHandler(service))

	// A gateway can also be declared in a file rather than registered through the API. The files are
	// read on every start; a declared gateway is matched by name, so re-reading updates it rather
	// than registering it again.
	if err := loadDeclarativeGateways(service); err != nil {
		return service, err
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
		AllowedMethods:   []string{"GET", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /gateways/{id}", h.handleGet, itemOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE /gateways/{id}", h.handleDelete, itemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways/{id}", noContent, itemOpts))

	applyOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /gateways/{id}/apply", h.handleApply, applyOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways/{id}/apply", noContent, applyOpts))
}
