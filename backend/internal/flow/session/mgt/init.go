// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package sessionmgt provides the session management API for listing live SSO sessions.
package sessionmgt

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/application"
	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/user"
)

// Initialize registers the session management routes. The user and application services resolve
// the names shown with each session.
func Initialize(mux *http.ServeMux, svc flowsession.Service, users user.UserServiceInterface,
	apps application.ApplicationServiceInterface) {
	registerRoutes(mux, newSessionMgtHandler(svc, users, apps))
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
	mux.HandleFunc(middleware.WithCORS("GET /users/me/sessions", handler.HandleSelfSessionListRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users/me/sessions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts))
}
