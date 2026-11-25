// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package capability

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
)

// Initialize registers the capabilities route, answered by the server's system authorization service.
func Initialize(mux *http.ServeMux, authzService sysauthz.SystemAuthorizationServiceInterface) {
	registerRoutes(mux, newCapabilityHandler(newCapabilityService(authzService)))
}

// registerRoutes registers the capabilities route and its CORS preflight.
func registerRoutes(mux *http.ServeMux, handler *capabilityHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /capabilities", handler.HandleCapabilitiesGetRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /capabilities",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
