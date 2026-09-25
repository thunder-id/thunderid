// Copyright 2026 The ThunderID Authors
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
	"github.com/thunder-id/thunderid/internal/system/log"
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
	discovery.Initialize(mux, userTypeService, cfg, scimServerStartTime)
	users.Initialize(mux, userService, userTypeService, cfg)
	groups.Initialize(mux, groupService, cfg)
	registerUnsupportedRoutes(mux)
}

// registerUnsupportedRoutes registers the endpoints that are not implemented and answer 501.
func registerUnsupportedRoutes(mux *http.ServeMux) {
	opts := scim.CRUDCORSOptions
	// Unimplemented endpoints
	for _, pattern := range []string{
		"POST " + scim.SCIMBasePath + "/Bulk",
		"POST " + scim.SCIMBasePath + "/.search",
		"PATCH " + scim.SCIMBasePath + "/Users/{id}",
	} {
		mux.HandleFunc(middleware.WithCORS(pattern, func(w http.ResponseWriter, r *http.Request) {
			scim.HandleUnsupportedRequest(w, r,
				*log.GetLogger().With(log.String(log.LoggerKeyComponentName, unsupportedRouteComponentName)))
		}, opts))
	}
}
