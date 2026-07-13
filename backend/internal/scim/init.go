/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package scim implements the SCIM v2.0 API endpoints for ThunderID,
// following RFC 7643 and RFC 7644.
package scim

import (
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/entitytype"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/user"
)

var scimServerStartTime = time.Now().UTC().Format(time.RFC3339)

// Initialize sets up the SCIM module and registers all /scim/v2 routes.
func Initialize(
	mux *http.ServeMux,
	userService user.UserServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	cfg scimconfig.SCIMConfig,
) {
	svc := newSCIMService(userService, entityTypeService, cfg)
	h := newSCIMHandler(svc, cfg.PublicURL)

	uSvc := newSCIMUsersService(userService, entityTypeService)
	uh := newSCIMUsersHandler(uSvc, cfg.PublicURL)

	registerRoutes(mux, h, uh)
}

// registerRoutes registers all /scim/v2 routes using the same
// middleware.WithCORS pattern as all other ThunderID modules.
func registerRoutes(mux *http.ServeMux, h *scimHandler, uh *scimUsersHandler) {
	optsGet := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	optsCRUD := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	// ServiceProviderConfig — Phase 1 implemented endpoint.
	mux.HandleFunc(middleware.WithCORS(
		"GET "+SCIMBasePath+"/ServiceProviderConfig",
		h.HandleServiceProviderConfigGetRequest,
		optsGet,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+SCIMBasePath+"/ServiceProviderConfig",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsGet,
	))

	// Schemas — list all and get single by URN.
	mux.HandleFunc(middleware.WithCORS(
		"GET "+SCIMBasePath+"/Schemas",
		h.HandleSchemaListRequest,
		optsGet,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+SCIMBasePath+"/Schemas",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsGet,
	))
	mux.HandleFunc(middleware.WithCORS(
		"GET "+SCIMBasePath+"/Schemas/{id}",
		h.HandleSchemaGetRequest,
		optsGet,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+SCIMBasePath+"/Schemas/{id}",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsGet,
	))

	// ResourceTypes — list all and get single by ID.
	mux.HandleFunc(middleware.WithCORS(
		"GET "+SCIMBasePath+"/ResourceTypes",
		h.HandleResourceTypeListRequest,
		optsGet,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+SCIMBasePath+"/ResourceTypes",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsGet,
	))
	mux.HandleFunc(middleware.WithCORS(
		"GET "+SCIMBasePath+"/ResourceTypes/{id}",
		h.HandleResourceTypeGetRequest,
		optsGet,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+SCIMBasePath+"/ResourceTypes/{id}",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsGet,
	))

	// Users - CRUD endpoints
	mux.HandleFunc(middleware.WithCORS(
		"GET "+SCIMBasePath+"/Users",
		uh.HandleUsersListRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"POST "+SCIMBasePath+"/Users",
		uh.HandleUsersCreateRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"GET "+SCIMBasePath+"/Users/{id}",
		uh.HandleUsersGetRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"PUT "+SCIMBasePath+"/Users/{id}",
		uh.HandleUsersReplaceRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"DELETE "+SCIMBasePath+"/Users/{id}",
		uh.HandleUsersDeleteRequest,
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+SCIMBasePath+"/Users",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsCRUD,
	))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS "+SCIMBasePath+"/Users/{id}",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) },
		optsCRUD,
	))

	// Unimplemented endpoints
	for _, pattern := range []string{
		"GET " + SCIMBasePath + "/Groups",
		"GET " + SCIMBasePath + "/Groups/{id}",
		"POST " + SCIMBasePath + "/Groups",
		"PUT " + SCIMBasePath + "/Groups",
		"DELETE " + SCIMBasePath + "/Groups",
		"POST " + SCIMBasePath + "/Bulk",
		"POST " + SCIMBasePath + "/.search",
		"PATCH " + SCIMBasePath + "/Users/{id}",
	} {
		mux.HandleFunc(pattern, h.handleUnsupportedRequest)
	}
}
