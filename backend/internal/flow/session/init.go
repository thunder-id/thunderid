// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"fmt"
	"net/http"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventlistener"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize builds the SSO session Service and the hook that installs its termination listener, and
// registers the session management routes on mux. Store construction stays inside this package: callers
// receive only the Service and never hold a store. The session timeouts are read from the "session"
// server-config section on every use, so a change applies without a restart. The entity and entity type
// services resolve the display names of the users and agents shown with each session; the application
// service, which is built later, is injected with SetApplicationProvider.
//
// bus is required: session audit events are published on it.
func Initialize(mux *http.ServeMux, dbProvider provider.DBProviderInterface, deploymentID string,
	serverConfig serverconfig.ServerConfigService, cfg flowconfig.Config,
	criteriaRevoker CriteriaRevoker, ouEligibility OUEligibility,
	entities entity.EntityServiceInterface, entityTypes entitytype.EntityTypeServiceInterface,
	bus eventbus.EventBusInterface) (Service, TerminationHook, error) {
	if bus == nil {
		return nil, nil, errNilEventBus
	}

	transactioner, err := dbProvider.GetRuntimePersistentDBTransactioner()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"failed to get runtime persistent DB transactioner for the SSO session service: %w", err)
	}

	store := newStore(dbProvider, deploymentID)
	timeouts := newConfigProvider(serverConfig, newDeploymentLimits(cfg))
	svc := &service{
		store:            store,
		resolver:         newResolver(store, timeouts),
		transactioner:    transactioner,
		criteriaRevoker:  criteriaRevoker,
		ouEligibility:    ouEligibility,
		bus:              bus,
		terminationTopic: eventlistener.NewTopic[TerminatedSession]("session.terminated"),
		timeouts:         timeouts,
		entities:         entities,
		entityTypes:      entityTypes,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SSOSessionService")),
	}
	registerRoutes(mux, newSessionMgtHandler(svc))
	return svc, svc.terminationTopic.Hook(), nil
}

// InitializeConfigHandler returns the "session" server-config handler, for the server-config service to
// be built with. It is apart from Initialize because the handler is registered before that service is
// built, and on a control plane, which runs no sessions and never calls Initialize.
func InitializeConfigHandler(cfg flowconfig.Config) serverconfig.ServerConfigHandlerInterface {
	return newConfigHandler(cfg)
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
