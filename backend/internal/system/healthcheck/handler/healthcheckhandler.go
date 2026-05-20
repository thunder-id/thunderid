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

// Package handler provides HTTP handlers for managing health check related API requests.
package handler

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/healthcheck/model"
	"github.com/thunder-id/thunderid/internal/system/healthcheck/service"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// HealthCheckHandler defines the handler for managing health check API requests.
type HealthCheckHandler struct {
	Service service.HealthCheckServiceInterface
}

// NewHealthCheckHandler creates a new instance of HealthCheckHandler with the provided service.
func NewHealthCheckHandler(svc service.HealthCheckServiceInterface) *HealthCheckHandler {
	return &HealthCheckHandler{
		Service: svc,
	}
}

// HandleLivenessRequest handles the health check livenss request.
func (hch *HealthCheckHandler) HandleLivenessRequest(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "HealthCheckHandler"))
	w.WriteHeader(http.StatusOK)
	logger.Debug("Health Check Liveness response sent")
}

// HandleReadinessRequest handles the health check readiness request.
func (hch *HealthCheckHandler) HandleReadinessRequest(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "HealthCheckHandler"))

	serverstatus := hch.Service.CheckReadiness()

	statusCode := http.StatusOK
	if serverstatus.Status != model.StatusUp {
		logger.Error("Readiness check failed", log.String("serverstatus", string(serverstatus.Status)))
		statusCode = http.StatusServiceUnavailable
	} else {
		logger.Debug("Readiness check passed", log.String("serverstatus", string(serverstatus.Status)))
	}

	sysutils.WriteSuccessResponse(w, statusCode, serverstatus)

	logger.Debug("Health Check Readiness response sent")
}
