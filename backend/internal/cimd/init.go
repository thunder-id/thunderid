// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cimd

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/middleware"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// Initialize initializes the Client ID Metadata Document service and registers its routes.
func Initialize(mux *http.ServeMux, cfg engineconfig.CIMDConfig) CIMDServiceInterface {
	cimdService := newCIMDService(cfg)
	registerRoutes(mux, newCIMDHandler(cimdService))
	return cimdService
}

// registerRoutes registers the routes for Client ID Metadata Document operations.
func registerRoutes(mux *http.ServeMux, cimdHandler *cimdHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /cimd/preview", cimdHandler.HandlePreviewRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /cimd/preview",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
