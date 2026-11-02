// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzen

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// directAuthGuardProvider gates the Direct API access handlers with the Direct Auth Secret. It is
// satisfied by the guard owned by the authn service.
type directAuthGuardProvider interface {
	Wrap(next http.HandlerFunc) http.HandlerFunc
}

// Initialize initializes the AuthZEN API adapter and registers its routes.
func Initialize(
	mux *http.ServeMux,
	authzService providers.AuthorizationProvider,
	entityProvider entityprovider.EntityProviderInterface,
	governance identitygovernance.IdentityGovernanceServiceInterface,
	resourceService resource.ResourceServiceInterface,
	directAuthGuard directAuthGuardProvider,
) AuthZENServiceInterface {
	service := newService(authzService, entityProvider, governance, resourceService)
	handler := newHandler(service)
	registerRoutes(mux, handler, directAuthGuard)
	return service
}

// registerRoutes registers AuthZEN discovery and access API routes. The access handlers are gated by
// the Direct Auth Secret; the discovery and CORS preflight (OPTIONS) handlers are not.
func registerRoutes(mux *http.ServeMux, h *handler, guard directAuthGuardProvider) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	discoveryOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: false,
		MaxAge:           600,
	}

	// directRoute gates a handler with the guard, applied outside CORS so a rejected request gets a
	// bare 401 with no CORS headers.
	directRoute := func(pattern string, handler http.HandlerFunc) {
		p, corsHandler := middleware.WithCORS(pattern, handler, opts)
		mux.HandleFunc(p, guard.Wrap(corsHandler))
	}

	mux.HandleFunc(middleware.WithCORS("GET /.well-known/authzen-configuration",
		h.HandleMetadataRequest, discoveryOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /.well-known/authzen-configuration",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, discoveryOpts))
	directRoute("POST /access/v1/evaluation", h.HandleAccessEvaluationRequest)
	mux.HandleFunc(middleware.WithCORS("OPTIONS /access/v1/evaluation",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
	directRoute("POST /access/v1/evaluations", h.HandleAccessEvaluationsRequest)
	mux.HandleFunc(middleware.WithCORS("OPTIONS /access/v1/evaluations",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
	directRoute("POST /access/v1/search/action", h.HandleActionSearchRequest)
	mux.HandleFunc(middleware.WithCORS("OPTIONS /access/v1/search/action",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
