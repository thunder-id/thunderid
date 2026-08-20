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

package environmentvariable

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize creates the environment variable service and registers its management routes.
func Initialize(mux *http.ServeMux) (EnvironmentVariableServiceInterface, error) {
	store := newEnvironmentVariableStore()
	service := newEnvironmentVariableService(store)
	handler := newEnvironmentVariableHandler(service)
	registerEnvironmentVariableRoutes(mux, handler)
	return service, nil
}

// registerEnvironmentVariableRoutes registers the CRUD routes for the environment variable handler
// under one environment.
//
// A variable belongs to an environment, not to the organization: its value is a property of the
// deployment it is applied to, so the environment is named in the path rather than inferred.
func registerEnvironmentVariableRoutes(mux *http.ServeMux, h *environmentVariableHandler) {
	const basePath = "/environments/{envId}/variables"

	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST "+basePath, h.HandleEnvironmentVariablePostRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("GET "+basePath, h.HandleEnvironmentVariableListRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+basePath,
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts1))

	opts2 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	// The literal "resolve" segment takes precedence over "{id}".
	mux.HandleFunc(middleware.WithCORS("GET "+basePath+"/resolve", h.HandleEnvironmentVariableResolveRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("GET "+basePath+"/{id}", h.HandleEnvironmentVariableGetRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("PUT "+basePath+"/{id}", h.HandleEnvironmentVariablePutRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("DELETE "+basePath+"/{id}", h.HandleEnvironmentVariableDeleteRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+basePath+"/{id}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts2))
}
