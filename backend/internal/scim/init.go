// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package scim implements the SCIM v2.0 API endpoints for ThunderID,
// following RFC 7643 and RFC 7644. It is the composition root that wires the
// discovery, users, and groups packages together and registers
// all /scim/v2 routes.
package scim

import (
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/group"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/scim/discovery"
	"github.com/thunder-id/thunderid/internal/scim/groups"
	"github.com/thunder-id/thunderid/internal/scim/users"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/user"
)

var scimServerStartTime = time.Now().UTC().Format(time.RFC3339)

// unsupportedRouteComponentName identifies the unimplemented-endpoint routes for log attribution.
const unsupportedRouteComponentName = "SCIMUnsupported"

// Initialize sets up the SCIM module and registers all /scim/v2 routes.
func Initialize(
	mux *http.ServeMux,
	userService user.UserServiceInterface,
	userTypeService entitytype.EntityTypeServiceInterface,
	groupService group.GroupServiceInterface,
	cfg scimconfig.SCIMConfig,
) {
	dh := discovery.NewHandler(userTypeService, cfg, scimServerStartTime, cfg.PublicURL)
	uh := users.NewHandler(userService, userTypeService, cfg)
	gh := groups.NewHandler(groupService, cfg)
	registerRoutes(mux, dh, uh, gh)
}

// registerRoutes registers all /scim/v2 routes using the same
// middleware.WithCORS pattern as all other ThunderID modules.
func registerRoutes(
	mux *http.ServeMux, h *discovery.Handler, uh *users.Handler, gh *groups.Handler,
) {
	optsGet := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	optsCRUD := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		AllowedHeaders:   append(append([]string{}, middleware.DefaultAllowedHeaders...), "If-Match"),
		AllowCredentials: true,
		MaxAge:           600,
	}

	// ServiceProviderConfig — Phase 1 implemented endpoint.
	mux.HandleFunc(middleware.WithCORS(
		"GET "+scim.SCIMBasePath+"/ServiceProviderConfig",
		h.HandleServiceProviderConfigGetRequest,
		optsGet,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+scim.SCIMBasePath+"/ServiceProviderConfig",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsGet,
	))

	// Schemas — list all and get single by URN.
	mux.HandleFunc(middleware.WithCORS(
		"GET "+scim.SCIMBasePath+"/Schemas",
		h.HandleSchemaListRequest,
		optsGet,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+scim.SCIMBasePath+"/Schemas",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsGet,
	))
	mux.HandleFunc(middleware.WithCORS(
		"GET "+scim.SCIMBasePath+"/Schemas/{id}",
		h.HandleSchemaGetRequest,
		optsGet,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+scim.SCIMBasePath+"/Schemas/{id}",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsGet,
	))

	// ResourceTypes — list all and get single by ID.
	mux.HandleFunc(middleware.WithCORS(
		"GET "+scim.SCIMBasePath+"/ResourceTypes",
		h.HandleResourceTypeListRequest,
		optsGet,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+scim.SCIMBasePath+"/ResourceTypes",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsGet,
	))
	mux.HandleFunc(middleware.WithCORS(
		"GET "+scim.SCIMBasePath+"/ResourceTypes/{id}",
		h.HandleResourceTypeGetRequest,
		optsGet,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+scim.SCIMBasePath+"/ResourceTypes/{id}",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsGet,
	))

	// Users - CRUD endpoints
	mux.HandleFunc(middleware.WithCORS(
		"GET "+scim.SCIMBasePath+"/Users",
		uh.HandleUsersListRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"POST "+scim.SCIMBasePath+"/Users",
		uh.HandleUsersCreateRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"GET "+scim.SCIMBasePath+"/Users/{id}",
		uh.HandleUsersGetRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"PUT "+scim.SCIMBasePath+"/Users/{id}",
		uh.HandleUsersReplaceRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"DELETE "+scim.SCIMBasePath+"/Users/{id}",
		uh.HandleUsersDeleteRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+scim.SCIMBasePath+"/Users",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+scim.SCIMBasePath+"/Users/{id}",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsCRUD,
	))
	// Me — RFC 7644 §3.11 authenticated-subject alias, processed directly.
	mux.HandleFunc(middleware.WithCORS(
		"GET "+scim.SCIMBasePath+"/Me",
		uh.HandleMeGetRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"PUT "+scim.SCIMBasePath+"/Me",
		uh.HandleMeReplaceRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+scim.SCIMBasePath+"/Me",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsCRUD,
	))
	// users/.search endpoint
	mux.HandleFunc(middleware.WithCORS(
		"POST "+scim.SCIMBasePath+"/Users/.search",
		uh.HandleUsersSearchRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+scim.SCIMBasePath+"/Users/.search",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsCRUD,
	))

	// groups CRUD operations
	mux.HandleFunc(middleware.WithCORS(
		"GET "+scim.SCIMBasePath+"/Groups",
		gh.HandleGroupsListRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"POST "+scim.SCIMBasePath+"/Groups",
		gh.HandleGroupsCreateRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"GET "+scim.SCIMBasePath+"/Groups/{id}",
		gh.HandleGroupsGetRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"PUT "+scim.SCIMBasePath+"/Groups/{id}",
		gh.HandleGroupsReplaceRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"PATCH "+scim.SCIMBasePath+"/Groups/{id}",
		gh.HandleGroupsPatchRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"DELETE "+scim.SCIMBasePath+"/Groups/{id}",
		gh.HandleGroupsDeleteRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+scim.SCIMBasePath+"/Groups",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+scim.SCIMBasePath+"/Groups/{id}",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsCRUD,
	))
	// Unimplemented endpoints
	for _, pattern := range []string{
		"POST " + scim.SCIMBasePath + "/Bulk",
		"POST " + scim.SCIMBasePath + "/.search",
		"PATCH " + scim.SCIMBasePath + "/Users/{id}",
	} {
		mux.HandleFunc(middleware.WithCORS(pattern, func(w http.ResponseWriter, r *http.Request) {
			scim.HandleUnsupportedRequest(w, r, unsupportedRouteComponentName)
		}, optsCRUD))
	}
}
