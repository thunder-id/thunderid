// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"fmt"
	"net/http"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/eventlistener"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize builds the SSO session Service and the hook that installs its termination listener, and
// registers the session management routes on mux. Store construction stays inside this package: callers
// receive only the Service and never hold a store. Timeouts fall back per field to the built-in
// defaults so an unset (zero) value never makes sessions expire immediately. The entity and entity type
// services resolve the display names of the users and agents shown with each session; the application
// service, which is built later, is injected with SetApplicationProvider.
func Initialize(mux *http.ServeMux, dbProvider provider.DBProviderInterface, deploymentID string,
	timeouts Timeouts, criteriaRevoker CriteriaRevoker, ouEligibility OUEligibility,
	entities entity.EntityServiceInterface, entityTypes entitytype.EntityTypeServiceInterface) (
	Service, TerminationHook, error) {
	transactioner, err := dbProvider.GetRuntimePersistentDBTransactioner()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"failed to get runtime persistent DB transactioner for the SSO session service: %w", err)
	}

	def := DefaultTimeouts()
	if timeouts.Idle <= 0 {
		timeouts.Idle = def.Idle
	}
	if timeouts.Absolute <= 0 {
		timeouts.Absolute = def.Absolute
	}
	if timeouts.ActivityRefresh <= 0 {
		timeouts.ActivityRefresh = def.ActivityRefresh
	}

	store := newStore(dbProvider, deploymentID)
	svc := &service{
		store:            store,
		resolver:         newResolver(store),
		transactioner:    transactioner,
		criteriaRevoker:  criteriaRevoker,
		ouEligibility:    ouEligibility,
		terminationTopic: eventlistener.NewTopic[TerminatedSession]("session.terminated"),
		timeouts:         timeouts,
		entities:         entities,
		entityTypes:      entityTypes,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SSOSessionService")),
	}
	registerRoutes(mux, newSessionMgtHandler(svc))
	return svc, svc.terminationTopic.Hook(), nil
}

// SetApplicationProvider gives the session service the application service, through which the session
// management API resolves the applications shown with each session. It is bound after the service, since the
// application package depends on this one. A service that cannot take it is reported rather than left without
// one.
func SetApplicationProvider(sessions Service, provider applicationProvider) error {
	svc, ok := sessions.(*service)
	if !ok {
		return fmt.Errorf("session service does not support application provider injection")
	}
	svc.apps = provider
	return nil
}

// registerRoutes registers the routes for session management operations.
func registerRoutes(mux *http.ServeMux, handler *sessionMgtHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	idOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	preflight := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	mux.HandleFunc(middleware.WithCORS("GET /sessions", handler.HandleSessionListRequest, opts))
	mux.HandleFunc(middleware.WithCORS("DELETE /sessions", handler.HandleUserSessionsRevokeRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /sessions", preflight, opts))
	mux.HandleFunc(middleware.WithCORS("DELETE /sessions/{id}", handler.HandleSessionRevokeRequest, idOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /sessions/{id}", preflight, idOpts))
	mux.HandleFunc(middleware.WithCORS("GET /users/me/sessions", handler.HandleSelfSessionListRequest, opts))
	mux.HandleFunc(middleware.WithCORS("DELETE /users/me/sessions", handler.HandleSelfSessionsRevokeRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users/me/sessions", preflight, opts))
	mux.HandleFunc(middleware.WithCORS("DELETE /users/me/sessions/{id}",
		handler.HandleSelfSessionRevokeRequest, idOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users/me/sessions/{id}", preflight, idOpts))
}
