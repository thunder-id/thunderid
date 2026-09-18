// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	req, ok := decodeRegistration(w, r)
	if !ok {
		return
	}
	gw, svcErr := h.service.Register(ctx, *req)
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

// decodeRegistration reads the request body as a registration, writing the response itself when it
// cannot. It reports whether the caller should carry on.
//
// A body has to be one JSON object and nothing else. Decode stops at the end of the first value, so
// without the second read a request carrying a valid registration followed by more JSON would be
// accepted, and the caller would be told that whatever it sent afterwards had been understood.
func decodeRegistration(w http.ResponseWriter, r *http.Request) (*RegisterRequest, bool) {
	var req RegisterRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		handleError(r.Context(), w, &ErrorInvalidRegistrationBody)
		return nil, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		handleError(r.Context(), w, &ErrorInvalidRegistrationBody)
		return nil, false
	}
	return &req, true
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
	if svcErr.Code == ErrorGatewayLimitReached.Code || svcErr.Code == ErrorGatewayNameTaken.Code ||
		svcErr.Code == ErrorDataPlaneAlreadyRegistered.Code {
		statusCode = http.StatusConflict
	}

	sysutils.WriteErrorResponse(ctx, w, statusCode, apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	})
}
