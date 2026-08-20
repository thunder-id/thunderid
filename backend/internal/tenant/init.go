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

package tenant

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/importer"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// DefaultSystemDeploymentID is the reserved deployment id of the platform system tenant, used when
// Config.SystemDeploymentID is empty.
const DefaultSystemDeploymentID = "root"

// Config holds the settings the tenant module needs to provision tenants.
type Config struct {
	// DefaultsDir is the bootstrap bundle directory (e.g. <serverHome>/bootstrap).
	DefaultsDir string
	// PublicURL is the Control Plane's public base URL, used to template the provisioned baseline.
	PublicURL string
	// SystemDeploymentID is the reserved deployment id of the platform system tenant.
	SystemDeploymentID string
}

// Initialize creates the tenant service and registers the /system/tenants routes.
func Initialize(mux *http.ServeMux, importSvc importer.ImportServiceInterface,
	cfg Config) (TenantServiceInterface, error) {
	systemDeploymentID := cfg.SystemDeploymentID
	if systemDeploymentID == "" {
		systemDeploymentID = DefaultSystemDeploymentID
	}
	store := newTenantStore(systemDeploymentID)
	service := newTenantService(store, importSvc, cfg.DefaultsDir, cfg.PublicURL, systemDeploymentID)
	handler := newTenantHandler(service)
	registerTenantRoutes(mux, handler)
	return service, nil
}

// registerTenantRoutes registers the platform tenant-management routes under /system/tenants.
func registerTenantRoutes(mux *http.ServeMux, h *tenantHandler) {
	const basePath = "/system/tenants"

	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST "+basePath, h.HandleTenantPostRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("GET "+basePath, h.HandleTenantListRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("POST "+basePath+"/{id}/environment",
		h.HandleEnvironmentPostRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+basePath,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts1))

	opts2 := middleware.CORSOptions{
		AllowedMethods:   []string{"DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("DELETE "+basePath+"/{id}", h.HandleTenantDeleteRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+basePath+"/{id}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts2))
}
