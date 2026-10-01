// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package groups

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/group"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize sets up the SCIM Groups module and registers its routes.
func Initialize(mux *http.ServeMux, groupService group.GroupServiceInterface, cfg scimconfig.SCIMConfig) {
	registerRoutes(mux, newSCIMGroupsHandler(newSCIMGroupsService(groupService), cfg))
}

// registerRoutes registers the Groups routes under /scim/v2.
func registerRoutes(mux *http.ServeMux, h *scimGroupsHandler) {
	opts := scim.CRUDCORSOptions
	base := scim.SCIMBasePath + "/Groups"
	route := func(pattern string, fn http.HandlerFunc) {
		mux.HandleFunc(middleware.WithCORS(pattern, fn, opts))
	}

	route("GET "+base, h.HandleGroupsListRequest)
	route("POST "+base, h.HandleGroupsCreateRequest)
	route("GET "+base+"/{id}", h.HandleGroupsGetRequest)
	route("PUT "+base+"/{id}", h.HandleGroupsReplaceRequest)
	route("PATCH "+base+"/{id}", h.HandleGroupsPatchRequest)
	route("DELETE "+base+"/{id}", h.HandleGroupsDeleteRequest)
	route("OPTIONS "+base, scim.HandlePreflightRequest)
	route("OPTIONS "+base+"/{id}", scim.HandlePreflightRequest)
}
