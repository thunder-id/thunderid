// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package capability

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// capabilityHandler serves the caller's capabilities.
type capabilityHandler struct {
	service *capabilityService
	logger  *log.Logger
}

// newCapabilityHandler returns a capabilityHandler.
func newCapabilityHandler(service *capabilityService) *capabilityHandler {
	return &capabilityHandler{
		service: service,
		logger:  log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CapabilityHandler")),
	}
}

// HandleCapabilitiesGetRequest returns the management actions the caller may perform.
func (h *capabilityHandler) HandleCapabilitiesGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	capabilities, svcErr := h.service.GetCapabilities(ctx)
	if svcErr != nil {
		h.logger.Error(ctx, "Error evaluating capabilities", log.Any("serviceError", svcErr))
		sysutils.WriteErrorResponse(ctx, w, http.StatusInternalServerError, apierror.ErrorResponse{
			Code:        svcErr.Code,
			Message:     svcErr.Error,
			Description: svcErr.ErrorDescription,
		})
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, capabilities)
}
