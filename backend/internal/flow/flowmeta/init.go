// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmeta

import (
	"net/http"

	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize creates and configures the flow metadata service components.
func Initialize(
	mux *http.ServeMux,
	actorProvider providers.ActorProvider,
	ouService providers.OrganizationUnitProvider,
	designResolve providers.DesignProvider,
	i18nService providers.I18nProvider,
	serverConfig flowconfig.ServerConfigReader,
) FlowMetaServiceInterface {
	// Create service instance
	flowMetaService := newFlowMetaService(
		actorProvider, ouService, designResolve, i18nService, serverConfig)

	// Create handler and register routes
	handler := newFlowMetaHandler(flowMetaService)
	registerRoutes(mux, handler)

	return flowMetaService
}

func registerRoutes(mux *http.ServeMux, handler *flowMetaHandler) {
	// CORS options for flow metadata endpoint (follows the same security as flow/execute)
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	// Register GET endpoint
	mux.HandleFunc(middleware.WithCORS("GET /flow/meta",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(handler.HandleGetFlowMetadata)).ServeHTTP, opts))

	// Register OPTIONS endpoint for CORS preflight
	mux.HandleFunc(middleware.WithCORS("OPTIONS /flow/meta",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
