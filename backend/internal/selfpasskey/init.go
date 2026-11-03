// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package selfpasskey

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/authn/passkey"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize registers the routes that let a signed-in user manage their own passkeys.
func Initialize(
	mux *http.ServeMux,
	passkeyService passkey.PasskeyServiceInterface,
	inboundClientService inboundclient.InboundClientServiceInterface,
) {
	handler := newSelfPasskeyHandler(newSelfPasskeyService(passkeyService, inboundClientService))
	registerRoutes(mux, handler)
}

// registerRoutes registers the self-service passkey routes.
func registerRoutes(mux *http.ServeMux, handler *selfPasskeyHandler) {
	optsRegister := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /users/me/passkeys/register/start",
		handler.HandleRegistrationStartRequest, optsRegister))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users/me/passkeys/register/start",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, optsRegister))
	mux.HandleFunc(middleware.WithCORS("POST /users/me/passkeys/register/finish",
		handler.HandleRegistrationFinishRequest, optsRegister))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users/me/passkeys/register/finish",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, optsRegister))

	optsList := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /users/me/passkeys", handler.HandleListRequest, optsList))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users/me/passkeys",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, optsList))

	optsDelete := middleware.CORSOptions{
		AllowedMethods:   []string{"DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("DELETE /users/me/passkeys/{id}", handler.HandleDeleteRequest, optsDelete))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users/me/passkeys/{id}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, optsDelete))
}
