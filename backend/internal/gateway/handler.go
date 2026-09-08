// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// handler serves the gateway registry.
type handler struct {
	service ServiceInterface
}

func newHandler(service ServiceInterface) *handler {
	return &handler{service: service}
}

// handleList returns the registered gateways. Credentials are never included.
func (h *handler) handleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	gateways, svcErr := h.service.List(ctx)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, gateways)
}

// handleGet returns one gateway.
func (h *handler) handleGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	gw, svcErr := h.service.Get(ctx, r.PathValue("id"))
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, gw)
}

// handleRegister records a data plane this control plane administers.
func (h *handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handleError(ctx, w, &ErrorInvalidRegistrationBody)
		return
	}
	gw, svcErr := h.service.Register(ctx, req)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, gw)
}

// handleDelete removes a gateway from the registry. The data plane itself is untouched.
func (h *handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if svcErr := h.service.Delete(ctx, r.PathValue("id")); svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleApply sends the configuration this control plane holds to the gateway.
func (h *handler) handleApply(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result, svcErr := h.service.Apply(ctx, r.PathValue("id"))
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, result)
}

// handleError maps a service error to an HTTP error response.
func handleError(ctx context.Context, w http.ResponseWriter, svcErr *common.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == common.ClientErrorType {
		statusCode = http.StatusBadRequest
	}
	if svcErr.Code == ErrorGatewayNotFound.Code {
		statusCode = http.StatusNotFound
	}
	if svcErr.Code == ErrorGatewayLimitReached.Code || svcErr.Code == ErrorGatewayNameTaken.Code {
		statusCode = http.StatusConflict
	}

	sysutils.WriteErrorResponse(ctx, w, statusCode, apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	})
}
