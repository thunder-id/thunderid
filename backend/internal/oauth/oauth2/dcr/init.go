// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package dcr provides Dynamic Client Registration (DCR) implementation.
package dcr

import (
	"context"
	"fmt"
	"net/http"

	"github.com/thunder-id/thunderid/internal/application"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize initializes the DCR service and registers its routes.
func Initialize(
	mux *http.ServeMux,
	appService application.ApplicationServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
	i18nService i18nmgt.I18nServiceInterface,
	cfg oauthconfig.Config,
) error {
	// Fetch runtime transient transactioner for OAuth services.
	transactioner, err := provider.GetDBProvider().GetRuntimeTransientDBTransactioner()
	if err != nil {
		wrappedErr := fmt.Errorf("failed to get runtime transient DB transactioner for DCR: %w", err)
		// Service initialization runs during application startup, outside any request.
		log.GetLogger().Error(context.Background(),
			"Failed to initialize DCR service", log.Error(wrappedErr))
		return wrappedErr
	}
	dcrService := newDCRService(appService, ouService, i18nService, transactioner, cfg)
	dcrHandler := newDCRHandler(dcrService, cfg)
	registerRoutes(mux, dcrHandler)
	return nil
}

// registerRoutes registers the routes for DCR operations.
func registerRoutes(mux *http.ServeMux, dcrHandler *dcrHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST", "GET", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	noContent := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}

	mux.HandleFunc(middleware.WithCORS("POST /oauth2/dcr/register",
		dcrHandler.HandleDCRRegistration, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /oauth2/dcr/register", noContent, opts))

	// RFC 7592 client configuration endpoint.
	mux.HandleFunc(middleware.WithCORS("GET /oauth2/dcr/register/{client_id}",
		dcrHandler.HandleGetClientConfiguration, opts))
	mux.HandleFunc(middleware.WithCORS("PUT /oauth2/dcr/register/{client_id}",
		dcrHandler.HandleUpdateClientConfiguration, opts))
	mux.HandleFunc(middleware.WithCORS("DELETE /oauth2/dcr/register/{client_id}",
		dcrHandler.HandleDeleteClientConfiguration, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /oauth2/dcr/register/{client_id}", noContent, opts))
}
