// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/cache"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize builds the variable store and registers its routes.
//
// The crypto provider is passed in rather than reached for, because it is the one dependency that
// decides whether a secret is safe at rest. Making it an argument means a caller cannot forget it and
// get a store that silently keeps plaintext.
//
// Reads by name go through a cache. A reference to a value here is resolved wherever it is used, and
// a credential held as a reference is resolved on every authentication against it, so an uncached
// store puts a database read on that path. The cache is invalidated by every write rather than
// expiring, so replacing a value takes effect on the next read.
//
// managementAPIKeyOnly makes every variable and secret API answer only the management API key. A
// server is given that key so that what deploys to it (a control plane) can push the values its
// configuration refers to, and those values are then that deployer's to keep: an administrator's token,
// however privileged, must not be able to change or remove them under it. A server with no key has
// nothing deploying to it, so its own administrators keep managing the store with their tokens.
//
// It also returns the store as a reference resolver, which is how a reference a resource carries is
// turned into the value held here. That half is not served over HTTP, so it is not restricted.
func Initialize(mux *http.ServeMux, crypto kmprovider.ConfigCryptoProvider,
	cacheManager cache.CacheManagerInterface, managementAPIKeyOnly bool,
) (ServiceInterface, ReferenceResolverInterface) {
	variables := cache.GetCache[*Variable](cacheManager, "VariableByNameCache")
	secrets := cache.GetCache[*Secret](cacheManager, "SecretByNameCache")

	service := newService(newCacheBackedStore(newStore(), variables, secrets), crypto)
	registerRoutes(mux, newHandler(service, managementAPIKeyOnly))
	// The same service resolves references; ServiceInterface leaves that out, so nothing served over
	// HTTP can reach the one method that returns a secret's value.
	return service, service.(ReferenceResolverInterface)
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
	// A preflight carries no credentials, so the OPTIONS routes are left open.
	keyOnly := h.requireManagementAPIKey

	mux.HandleFunc(middleware.WithCORS("GET "+pathVariables, keyOnly(h.HandleVariableListRequest), collectionCORS))
	mux.HandleFunc(middleware.WithCORS("POST "+pathVariables, keyOnly(h.HandleVariablePostRequest), collectionCORS))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+pathVariables, noContent, collectionCORS))

	mux.HandleFunc(middleware.WithCORS("GET "+pathVariables+"/{name}", keyOnly(h.HandleVariableGetRequest), itemCORS))
	mux.HandleFunc(middleware.WithCORS("PUT "+pathVariables+"/{name}", keyOnly(h.HandleVariablePutRequest), itemCORS))
	mux.HandleFunc(middleware.WithCORS("DELETE "+pathVariables+"/{name}",
		keyOnly(h.HandleVariableDeleteRequest), itemCORS))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+pathVariables+"/{name}", noContent, itemCORS))

	mux.HandleFunc(middleware.WithCORS("GET "+pathSecrets, keyOnly(h.HandleSecretListRequest), collectionCORS))
	mux.HandleFunc(middleware.WithCORS("POST "+pathSecrets, keyOnly(h.HandleSecretPostRequest), collectionCORS))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+pathSecrets, noContent, collectionCORS))

	mux.HandleFunc(middleware.WithCORS("GET "+pathSecrets+"/{name}", keyOnly(h.HandleSecretGetRequest), itemCORS))
	mux.HandleFunc(middleware.WithCORS("PUT "+pathSecrets+"/{name}", keyOnly(h.HandleSecretPutRequest), itemCORS))
	mux.HandleFunc(middleware.WithCORS("DELETE "+pathSecrets+"/{name}", keyOnly(h.HandleSecretDeleteRequest), itemCORS))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+pathSecrets+"/{name}", noContent, itemCORS))
}

func noContent(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
