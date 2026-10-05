// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package logout

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize wires the RP-initiated logout feature and registers the end_session_endpoint.
func Initialize(
	mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface,
	actorProvider providers.ActorProvider,
	flowExecService flowexec.FlowExecServiceInterface,
	runtimeStore providers.RuntimeStoreProvider,
	cfg oauthconfig.Config,
) {
	store := newLogoutRequestStore(runtimeStore)
	service := newLogoutService(jwtService, actorProvider, flowExecService, store, cfg.JWT.Issuer)
	handler := newLogoutHandler(service, cfg)
	registerRoutes(mux, handler)
}

// registerRoutes registers the GET/POST/OPTIONS routes for the logout endpoint and its completion
// callback (POST /oauth2/logout/callback), which the gate calls once the sign-out flow finishes.
func registerRoutes(mux *http.ServeMux, handler *logoutHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	callbackEndpoint := constants.OAuth2LogoutEndpoint + "/callback"

	logout := middleware.CorrelationIDMiddleware(http.HandlerFunc(handler.HandleLogout)).ServeHTTP
	logoutCallback := middleware.CorrelationIDMiddleware(http.HandlerFunc(handler.HandleLogoutCallback)).ServeHTTP

	mux.HandleFunc(middleware.WithCORS("GET "+constants.OAuth2LogoutEndpoint, logout, opts))
	mux.HandleFunc(middleware.WithCORS("POST "+constants.OAuth2LogoutEndpoint, logout, opts))
	mux.HandleFunc(middleware.WithCORS("POST "+callbackEndpoint, logoutCallback, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+constants.OAuth2LogoutEndpoint,
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+callbackEndpoint,
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
