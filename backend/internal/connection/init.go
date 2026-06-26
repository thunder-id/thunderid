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

package connection

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize wires the connection service over the identity-provider service and registers
// the /connections routes.
func Initialize(mux *http.ServeMux, idpService idp.IDPServiceInterface) {
	svc := newService(idpService)
	h := newHandler(svc)
	registerRoutes(mux, h)
}

func noContent(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// registerRoutes registers the listing route and each IdP-backed vendor's CRUD routes.
func registerRoutes(mux *http.ServeMux, h *handler) {
	collectionOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	itemOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	// Listing.
	listOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /connections", h.handleListConnections, listOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /connections", noContent, listOpts))

	// IdP-backed vendors.
	registerVendorRoutes(mux, h, "/connections/google", providers.IDPTypeGoogle,
		h.handleGoogleCreate, h.handleGoogleGet, h.handleGoogleUpdate, collectionOpts, itemOpts)
	registerVendorRoutes(mux, h, "/connections/github", providers.IDPTypeGitHub,
		h.handleGitHubCreate, h.handleGitHubGet, h.handleGitHubUpdate, collectionOpts, itemOpts)
	registerVendorRoutes(mux, h, "/connections/oidc", providers.IDPTypeOIDC,
		h.handleOIDCCreate, h.handleOIDCGet, h.handleOIDCUpdate, collectionOpts, itemOpts)
}

// registerVendorRoutes registers the collection (list/create) and item (get/update/delete)
// routes for a single vendor, plus their OPTIONS handlers.
func registerVendorRoutes(mux *http.ServeMux, h *handler, base string, idpType providers.IDPType,
	create, get, update http.HandlerFunc, collectionOpts, itemOpts middleware.CORSOptions) {
	mux.HandleFunc(middleware.WithCORS("GET "+base, h.listInstances(idpType), collectionOpts))
	mux.HandleFunc(middleware.WithCORS("POST "+base, create, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base, noContent, collectionOpts))

	mux.HandleFunc(middleware.WithCORS("GET "+base+"/{id}", get, itemOpts))
	mux.HandleFunc(middleware.WithCORS("PUT "+base+"/{id}", update, itemOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE "+base+"/{id}", h.deleteInstance(idpType), itemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+base+"/{id}", noContent, itemOpts))
}
