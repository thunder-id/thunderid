// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package application provides functionality for managing applications.
package application

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/sharing"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the application service and registers its routes.
func Initialize(
	mux *http.ServeMux,
	mcpServer *mcp.Server,
	entityService entity.EntityServiceInterface,
	inboundClient inboundclient.InboundClientServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	i18nService i18nmgt.I18nServiceInterface,
	cryptoSvc providers.RuntimeCryptoProvider,
	serverConfigSvc serverconfig.ServerConfigService,
	artifactLifetime artifactLifetimeResolver,
	sharingService sharing.ServiceInterface,
) (ApplicationServiceInterface, declarativeresource.ResourceExporter, error) {
	appService := newApplicationService(
		inboundClient, entityService, ouService, i18nService, cryptoSvc, serverConfigSvc, artifactLifetime,
		sharingService,
	)

	// Registered before declarative resources load, because loading seeds the sharing policies those
	// files declare and the framework refuses a type it does not know.
	if sharingService != nil {
		sharingService.RegisterResourceType(newApplicationSharing(appService))
	}

	if err := entityService.LoadIndexedAttributes(getAppIndexedAttributes()); err != nil {
		return nil, nil, err
	}

	storeMode := getApplicationStoreMode()
	// TODO: Revisit once the declarative resource loading pattern is finalized.
	if storeMode == serverconst.StoreModeComposite || storeMode == serverconst.StoreModeDeclarative {
		// An application's parsed form is not retained anywhere afterwards: the entity store keeps
		// the identity row and the inbound-client store keeps the OAuth config, and neither holds the
		// declared policies. They are captured during parsing instead, and replayed once the whole
		// batch has loaded, because a policy may name an organization unit whose own document is
		// parsed later than the application's.
		var declared []declaredAppPolicies
		collect := func(d declaredAppPolicies) { declared = append(declared, d) }

		if err := entityService.LoadDeclarativeResources(
			makeAppDeclarativeConfig(appService, collect)); err != nil {
			return nil, nil, err
		}
		if err := inboundClient.LoadDeclarativeResources(
			context.Background(), makeAppInboundConfig(appService)); err != nil {
			return nil, nil, err
		}
		if err := seedDeclaredApplicationPolicies(declared, sharingService); err != nil {
			return nil, nil, err
		}
	}

	appHandler := newApplicationHandler(appService)
	registerRoutes(mux, appHandler)

	if mcpServer != nil {
		registerMCPTools(mcpServer, appService)
	}

	exporter := newApplicationExporter(appService)
	return appService, exporter, nil
}

func registerRoutes(mux *http.ServeMux, appHandler *applicationHandler) {
	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /applications",
		appHandler.HandleApplicationPostRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("GET /applications",
		appHandler.HandleApplicationListRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /applications",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts1))

	opts2 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /applications/{id}",
		appHandler.HandleApplicationGetRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("PUT /applications/{id}",
		appHandler.HandleApplicationPutRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("DELETE /applications/{id}",
		appHandler.HandleApplicationDeleteRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /applications/",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts2))
}
