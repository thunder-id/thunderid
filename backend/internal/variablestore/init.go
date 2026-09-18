// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"net/http"

	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize builds the variable store and registers its routes.
//
// The crypto provider is passed in rather than reached for, because it is the one dependency that
// decides whether a secret is safe at rest. Making it an argument means a caller cannot forget it and
// get a store that silently keeps plaintext.
func Initialize(mux *http.ServeMux, crypto kmprovider.ConfigCryptoProvider) ServiceInterface {
	service := newService(newStore(), crypto)
	registerRoutes(mux, newHandler(service))
	return service
}

func registerRoutes(mux *http.ServeMux, h *handler) {
	collectionCORS := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	itemCORS := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET "+pathVariables, h.HandleVariableListRequest, collectionCORS))
	mux.HandleFunc(middleware.WithCORS("POST "+pathVariables, h.HandleVariablePostRequest, collectionCORS))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+pathVariables, noContent, collectionCORS))

	mux.HandleFunc(middleware.WithCORS("GET "+pathVariables+"/{name}", h.HandleVariableGetRequest, itemCORS))
	mux.HandleFunc(middleware.WithCORS("PUT "+pathVariables+"/{name}", h.HandleVariablePutRequest, itemCORS))
	mux.HandleFunc(middleware.WithCORS("DELETE "+pathVariables+"/{name}", h.HandleVariableDeleteRequest, itemCORS))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+pathVariables+"/{name}", noContent, itemCORS))

	mux.HandleFunc(middleware.WithCORS("GET "+pathSecrets, h.HandleSecretListRequest, collectionCORS))
	mux.HandleFunc(middleware.WithCORS("POST "+pathSecrets, h.HandleSecretPostRequest, collectionCORS))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+pathSecrets, noContent, collectionCORS))

	mux.HandleFunc(middleware.WithCORS("GET "+pathSecrets+"/{name}", h.HandleSecretGetRequest, itemCORS))
	mux.HandleFunc(middleware.WithCORS("PUT "+pathSecrets+"/{name}", h.HandleSecretPutRequest, itemCORS))
	mux.HandleFunc(middleware.WithCORS("DELETE "+pathSecrets+"/{name}", h.HandleSecretDeleteRequest, itemCORS))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+pathSecrets+"/{name}", noContent, itemCORS))
}

func noContent(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
