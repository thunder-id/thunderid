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
//
// valueCapturer is optional. Given one, the service hands it every application it creates or changes,
// so the values the application's export refers to are kept where a reference finds them.
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
	sharingService sharing.SharingServiceInterface,
	valueCapturer declarativeresource.ValueCapturer,
) (ApplicationServiceInterface, declarativeresource.ResourceExporter, error) {
	appService := newApplicationService(
		inboundClient, entityService, ouService, i18nService, cryptoSvc, serverConfigSvc, artifactLifetime,
		valueCapturer, sharingService,
	)

	// Registered before declarative resources load, because loading seeds the sharing policies those
	// files declare and the framework refuses a type it does not know.
	sharingService.RegisterResourceType(newApplicationSharingDeclaration(appService))

	if err := entityService.LoadIndexedAttributes(getAppIndexedAttributes()); err != nil {
		return nil, nil, err
	}

	storeMode := getApplicationStoreMode()
	// TODO: Revisit once the declarative resource loading pattern is finalized.
	if storeMode == serverconst.StoreModeComposite || storeMode == serverconst.StoreModeDeclarative {
		// One document, read once per thing it carries: its identity, its inbound client profile,
		// and its sharing policies. Each loader takes the applications directory and the parser for
		// its own half, which is the arrangement inbound client profiles already use.
		if err := entityService.LoadDeclarativeResources(
			makeAppDeclarativeConfig(appService)); err != nil {
			return nil, nil, err
		}
		if err := inboundClient.LoadDeclarativeResources(
			context.Background(), makeAppInboundConfig(appService)); err != nil {
			return nil, nil, err
		}
		if err := sharingService.LoadDeclarativeResources(
			context.Background(), makeAppSharingConfig(appService)); err != nil {
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

	// Sharing policies live under the application they govern: a policy has no life of its own.
	opts3 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /applications/{id}/sharing-policies",
		appHandler.HandleSharingPolicyPostRequest, opts3))
	mux.HandleFunc(middleware.WithCORS("GET /applications/{id}/sharing-policies",
		appHandler.HandleSharingPolicyListRequest, opts3))
	// Without its own preflight route this collection falls to OPTIONS /applications/, which does
	// not allow POST, and the browser blocks the create.
	mux.HandleFunc(middleware.WithCORS("OPTIONS /applications/{id}/sharing-policies",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts3))
	mux.HandleFunc(middleware.WithCORS("GET /applications/{id}/overlay-rules",
		appHandler.HandleOverlayRuleGetRequest, opts3))

	opts4 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /applications/{id}/sharing-policies/{policyId}",
		appHandler.HandleSharingPolicyGetRequest, opts4))
	mux.HandleFunc(middleware.WithCORS("PUT /applications/{id}/sharing-policies/{policyId}",
		appHandler.HandleSharingPolicyPutRequest, opts4))
	mux.HandleFunc(middleware.WithCORS("DELETE /applications/{id}/sharing-policies/{policyId}",
		appHandler.HandleSharingPolicyDeleteRequest, opts4))
}
