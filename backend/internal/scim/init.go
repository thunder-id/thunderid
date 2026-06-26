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

package scim

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/user"
)

// Initialize sets up the SCIM module and registers all /scim/v2 routes.
func Initialize(
	mux *http.ServeMux,
	userService user.UserServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	baseURL string,
	scimCfg config.SCIMConfig,
) {
	svc := newSCIMService(userService, entityTypeService, scimCfg)
	h := newSCIMHandler(svc, baseURL)
	registerSCIMRoutes(mux, h)
}

// registerSCIMRoutes registers all /scim/v2 routes using the same
// middleware.WithCORS pattern as all other ThunderID modules.
func registerSCIMRoutes(mux *http.ServeMux, h *scimHandler) {
	optsGet := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
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

	// Unimplemented endpoints — return 501 per SCIM spec.
	for _, pattern := range []string{
		"GET " + SCIMBasePath + "/Users",
		"POST " + SCIMBasePath + "/Users",
		"PUT " + SCIMBasePath + "/Users",
		"DELETE " + SCIMBasePath + "/Users",
		"GET " + SCIMBasePath + "/Users/{id}",
		"PUT " + SCIMBasePath + "/Users/{id}",
		"DELETE " + SCIMBasePath + "/Users/{id}",
		"GET " + SCIMBasePath + "/Groups",
		"GET " + SCIMBasePath + "/Groups/{id}",
		"POST " + SCIMBasePath + "/Groups",
		"PUT " + SCIMBasePath + "/Groups",
		"DELETE " + SCIMBasePath + "/Groups",
		"POST " + SCIMBasePath + "/Bulk",
		"POST " + SCIMBasePath + "/.search",
	} {
		mux.HandleFunc(pattern, h.HandleUnsupportedRequest)
	}
}
