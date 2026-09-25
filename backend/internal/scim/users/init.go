// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/entitytype"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/user"
)

// Initialize sets up the SCIM Users module and registers its routes.
func Initialize(
	mux *http.ServeMux,
	userService user.UserServiceInterface,
	userTypeService entitytype.EntityTypeServiceInterface,
	cfg scimconfig.SCIMConfig,
) {
	svc := newSCIMUsersService(userService, userTypeService, cfg)
	registerRoutes(mux, newSCIMUsersHandler(svc, cfg))
}

// registerRoutes registers the Users and Me routes under /scim/v2.
func registerRoutes(mux *http.ServeMux, h *scimUsersHandler) {
	opts := scim.CRUDCORSOptions
	users := scim.SCIMBasePath + "/Users"
	me := scim.SCIMBasePath + "/Me"
	route := func(pattern string, fn http.HandlerFunc) {
		mux.HandleFunc(middleware.WithCORS(pattern, fn, opts))
	}

	route("GET "+users, h.HandleUsersListRequest)
	route("POST "+users, h.HandleUsersCreateRequest)
	route("GET "+users+"/{id}", h.HandleUsersGetRequest)
	route("PUT "+users+"/{id}", h.HandleUsersReplaceRequest)
	route("DELETE "+users+"/{id}", h.HandleUsersDeleteRequest)
	route("OPTIONS "+users, scim.HandlePreflightRequest)
	route("OPTIONS "+users+"/{id}", scim.HandlePreflightRequest)

	// Me is the RFC 7644 §3.11 authenticated-subject alias, processed directly.
	route("GET "+me, h.HandleMeGetRequest)
	route("PUT "+me, h.HandleMeReplaceRequest)
	route("OPTIONS "+me, scim.HandlePreflightRequest)

	route("POST "+users+"/.search", h.HandleUsersSearchRequest)
	route("OPTIONS "+users+"/.search", scim.HandlePreflightRequest)
}
