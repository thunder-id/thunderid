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

// Package mgt provides the session management API for listing live SSO sessions.
package mgt

import (
	"net/http"

	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize registers the read-only session management routes over the given service. names
// resolves the user and application display names embedded in the listing.
func Initialize(mux *http.ServeMux, svc flowsession.ManagementService, names NameResolver) {
	handler := newSessionMgtHandler(svc, names)
	registerRoutes(mux, handler)
}

// registerRoutes registers the routes for session management operations.
func registerRoutes(mux *http.ServeMux, handler *sessionMgtHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /sessions", handler.HandleSessionListRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /sessions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts))
	mux.HandleFunc(middleware.WithCORS("GET /sessions/me", handler.HandleSelfSessionListRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /sessions/me", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts))
}
