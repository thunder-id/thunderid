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

package credential

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

// Initialize builds the credential-configuration store and service, registers the
// management API routes, and returns the service for the issuer engine to read
// configurations from along with the exporter for declarative-resource export.
//
// Store selection (openid4vci.store): "mutable" (database, full CRUD),
// "declarative" (file-based, read-only), or "composite" (file-based immutable +
// database mutable). When unset it follows the global declarative_resources setting.
func Initialize(
	mux *http.ServeMux, ouService ou.OrganizationUnitServiceInterface,
) (CredentialConfigurationServiceInterface, declarativeresource.ResourceExporter, error) {
	store, err := initializeStore()
	if err != nil {
		return nil, nil, err
	}
	svc := newCredentialConfigurationService(store, ouService)
	registerRoutes(mux, newConfigurationHandler(svc))
	return svc, newConfigurationExporter(svc), nil
}

// registerRoutes registers the admin-facing management endpoints. They are
// intentionally NOT in the public-paths allowlist, so the platform auth
// middleware protects them.
func registerRoutes(mux *http.ServeMux, h *configurationHandler) {
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

	mux.HandleFunc(middleware.WithCORS("POST "+configurationsPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleCreate)).ServeHTTP, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("GET "+configurationsPath,
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleList)).ServeHTTP, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("GET "+configurationsPath+"/{id}",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleGet)).ServeHTTP, resourceOpts))
	mux.HandleFunc(middleware.WithCORS("PUT "+configurationsPath+"/{id}",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleUpdate)).ServeHTTP, resourceOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE "+configurationsPath+"/{id}",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(h.HandleDelete)).ServeHTTP, resourceOpts))

	mux.HandleFunc(middleware.WithCORS("OPTIONS "+configurationsPath,
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+configurationsPath+"/{id}",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, resourceOpts))
}

// initializeStore creates the credential store for the configured store mode, loading declarative resources as needed.
func initializeStore() (credentialStoreInterface, error) {
	mode, err := getCredentialStoreMode()
	if err != nil {
		return nil, err
	}
	switch mode {
	case serverconst.StoreModeComposite:
		fileStore := newCredentialFileBasedStore()
		if err := loadDeclarativeResources(&credentialStorer{store: fileStore}); err != nil {
			return nil, err
		}
		return newCompositeCredentialStore(fileStore, newCredentialStore()), nil
	case serverconst.StoreModeDeclarative:
		fileStore := newCredentialFileBasedStore()
		if err := loadDeclarativeResources(&credentialStorer{store: fileStore}); err != nil {
			return nil, err
		}
		return fileStore, nil
	default:
		return newCredentialStore(), nil
	}
}

// getCredentialStoreMode determines the credential store mode from configuration, defaulting based on declarative mode.
func getCredentialStoreMode() (serverconst.StoreMode, error) {
	cfg := config.GetServerRuntime().Config
	if cfg.OpenID4VCI.Store != "" {
		mode := serverconst.StoreMode(strings.ToLower(strings.TrimSpace(cfg.OpenID4VCI.Store)))
		switch mode {
		case serverconst.StoreModeMutable, serverconst.StoreModeDeclarative, serverconst.StoreModeComposite:
			return mode, nil
		default:
			return "", fmt.Errorf("invalid openid4vci store mode %q: must be one of %q, %q, or %q",
				cfg.OpenID4VCI.Store, serverconst.StoreModeMutable,
				serverconst.StoreModeDeclarative, serverconst.StoreModeComposite)
		}
	}
	if declarativeresource.IsDeclarativeModeEnabled() {
		return serverconst.StoreModeDeclarative, nil
	}
	return serverconst.StoreModeMutable, nil
}
