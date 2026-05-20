/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package services

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/healthcheck/handler"
	"github.com/thunder-id/thunderid/internal/system/healthcheck/service"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// HealthCheckService defines the service for handling readiness and liveness checks.
type HealthCheckService struct {
	healthCheckHandler *handler.HealthCheckHandler
}

// NewHealthCheckService creates a new instance of HealthCheckService.
func NewHealthCheckService(mux *http.ServeMux, svc service.HealthCheckServiceInterface) ServiceInterface {
	instance := &HealthCheckService{
		healthCheckHandler: handler.NewHealthCheckHandler(svc),
	}
	instance.RegisterRoutes(mux)

	return instance
}

// RegisterRoutes registers the routes for the HealthCheckService.
//
//nolint:dupl // Ignoring false positive duplicate code
func (h *HealthCheckService) RegisterRoutes(mux *http.ServeMux) {
	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("OPTIONS /health/liveness",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts1))
	mux.HandleFunc(middleware.WithCORS("GET /health/liveness",
		h.healthCheckHandler.HandleLivenessRequest, opts1))

	mux.HandleFunc(middleware.WithCORS("OPTIONS /health/readiness",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts1))
	mux.HandleFunc(middleware.WithCORS("GET /health/readiness",
		h.healthCheckHandler.HandleReadinessRequest, opts1))
}
