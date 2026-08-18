// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	logger.Debug(r.Context(), "Health Check Liveness response sent to the caller")
}

// HandleReadinessRequest handles the health check readiness request.
func (hch *HealthCheckHandler) HandleReadinessRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "HealthCheckHandler"))

	serverstatus := hch.Service.CheckReadiness(ctx)

	statusCode := http.StatusOK
	if serverstatus.Status != model.StatusUp {
		logger.Error(ctx, "Readiness check failed",
			log.String("serverstatus", string(serverstatus.Status)))
		statusCode = http.StatusServiceUnavailable
	} else {
		logger.Debug(ctx, "Readiness check passed",
			log.String("serverstatus", string(serverstatus.Status)))
	}

	sysutils.WriteSuccessResponse(ctx, w, statusCode, serverstatus)

	logger.Debug(ctx, "Health Check Readiness response sent")
}
