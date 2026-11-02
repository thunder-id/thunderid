// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/entitytype"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize sets up the SCIM discovery module and registers its routes.
func Initialize(
	mux *http.ServeMux,
	userTypeService entitytype.EntityTypeServiceInterface,
	cfg scimconfig.SCIMConfig,
	serverStartTime string,
) {
	svc := newSCIMDiscoveryService(userTypeService, cfg, serverStartTime)
	registerRoutes(mux, newSCIMDiscoveryHandler(svc, cfg.PublicURL))
}

// registerRoutes registers the discovery routes under /scim/v2.
func registerRoutes(mux *http.ServeMux, h *scimDiscoveryHandler) {
	opts := scim.ReadOnlyCORSOptions
	routes := []struct {
		path    string
		handler http.HandlerFunc
	}{
		{"/ServiceProviderConfig", h.HandleServiceProviderConfigGetRequest},
		{"/Schemas", h.HandleSchemaListRequest},
		{"/Schemas/{id}", h.HandleSchemaGetRequest},
		{"/ResourceTypes", h.HandleResourceTypeListRequest},
		{"/ResourceTypes/{id}", h.HandleResourceTypeGetRequest},
	}
	for _, route := range routes {
		mux.HandleFunc(middleware.WithCORS("GET "+scim.SCIMBasePath+route.path, route.handler, opts))
		mux.HandleFunc(middleware.WithCORS("OPTIONS "+scim.SCIMBasePath+route.path, scim.HandlePreflightRequest, opts))
	}
}
