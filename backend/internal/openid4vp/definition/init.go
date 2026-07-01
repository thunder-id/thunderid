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

package definition

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize builds the presentation-definition store and service, registers the
// management API routes, and returns the service for the verifier engine to read
// definitions from along with the exporter used for declarative-resource export.
// The store is private to this package.
//
// Store Selection (based on openid4vp.store configuration):
//
//  1. MUTABLE mode (store: "mutable"): database store only, full CRUD.
//  2. DECLARATIVE mode (store: "declarative"): file-based store only, read-only;
//     YAML resources are loaded into the file store.
//  3. COMPOSITE mode (store: "composite"): file-based (immutable) + database (mutable),
//     reads merged, writes routed to the database, declarative definitions immutable.
//
// When openid4vp.store is unset, it falls back to global declarative_resources.enabled
// (enabled => declarative, disabled => mutable).
//
// The single service built here is shared by both the management API and the
// verifier engine, which resolves definitions through it on demand.
func Initialize(
	mux *http.ServeMux, ouService ou.OrganizationUnitServiceInterface,
) (PresentationDefinitionServiceInterface, declarativeresource.ResourceExporter, error) {
	store, err := initializeStore()
	if err != nil {
		return nil, nil, err
	}
	svc := newPresentationDefinitionService(store, ouService)
	registerRoutes(mux, newDefinitionHandler(svc))
	return svc, newDefinitionExporter(svc), nil
}

// registerRoutes registers the management endpoints. These are
// admin-facing and intentionally NOT in the public-paths allowlist, so the
// platform auth middleware protects them.
func registerRoutes(mux *http.ServeMux, h *definitionHandler) {
	collectionOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	resourceOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("POST "+definitionsPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleCreate)).ServeHTTP, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("GET "+definitionsPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleList)).ServeHTTP, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("GET "+definitionsPath+"/{id}",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleGet)).ServeHTTP, resourceOpts))
	mux.HandleFunc(middleware.WithCORS("PUT "+definitionsPath+"/{id}",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleUpdate)).ServeHTTP, resourceOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE "+definitionsPath+"/{id}",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleDelete)).ServeHTTP, resourceOpts))

	mux.HandleFunc(middleware.WithCORS("OPTIONS "+definitionsPath,
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+definitionsPath+"/{id}",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, resourceOpts))
}

// initializeStore builds the presentation-definition store based on the configured store mode.
func initializeStore() (definitionStoreInterface, error) {
	storeMode, err := getDefinitionStoreMode()
	if err != nil {
		return nil, err
	}

	switch storeMode {
	case serverconst.StoreModeComposite:
		fileStore := newDefinitionFileBasedStore()
		dbStore := newDefinitionStore()
		if err := loadDeclarativeResources(&definitionStorer{store: fileStore}); err != nil {
			return nil, err
		}
		return newCompositeDefinitionStore(fileStore, dbStore), nil

	case serverconst.StoreModeDeclarative:
		fileStore := newDefinitionFileBasedStore()
		if err := loadDeclarativeResources(&definitionStorer{store: fileStore}); err != nil {
			return nil, err
		}
		return fileStore, nil

	default:
		return newDefinitionStore(), nil
	}
}

// getDefinitionStoreMode determines the store mode for presentation definitions.
//
// Resolution order:
//  1. If OpenID4VP.Store is explicitly configured, validate and use it — an
//     unrecognized value is a hard error so the server cannot boot silently with a
//     mistyped mode.
//  2. Otherwise, fall back to global DeclarativeResources.Enabled:
//     - If enabled: return "declarative"
//     - If disabled: return "mutable"
//
// Returns normalized store mode: "mutable", "declarative", or "composite".
func getDefinitionStoreMode() (serverconst.StoreMode, error) {
	cfg := config.GetServerRuntime().Config
	if cfg.OpenID4VP.Store != "" {
		mode := serverconst.StoreMode(strings.ToLower(strings.TrimSpace(cfg.OpenID4VP.Store)))
		switch mode {
		case serverconst.StoreModeMutable, serverconst.StoreModeDeclarative, serverconst.StoreModeComposite:
			return mode, nil
		default:
			return "", fmt.Errorf("invalid openid4vp store mode %q: must be one of %q, %q, or %q",
				cfg.OpenID4VP.Store,
				serverconst.StoreModeMutable,
				serverconst.StoreModeDeclarative,
				serverconst.StoreModeComposite,
			)
		}
	}

	if declarativeresource.IsDeclarativeModeEnabled() {
		return serverconst.StoreModeDeclarative, nil
	}

	return serverconst.StoreModeMutable, nil
}
