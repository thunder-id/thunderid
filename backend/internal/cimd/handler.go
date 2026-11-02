// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cimd

import (
	"context"
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// cimdHandler handles the Client ID Metadata Document requests.
type cimdHandler struct {
	cimdService CIMDServiceInterface
	logger      *log.Logger
}

// newCIMDHandler creates a new instance of cimdHandler.
func newCIMDHandler(cimdService CIMDServiceInterface) *cimdHandler {
	return &cimdHandler{
		cimdService: cimdService,
		logger:      log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CIMDHandler")),
	}
}

// HandlePreviewRequest handles a Client ID Metadata Document preview request.
func (h *cimdHandler) HandlePreviewRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	previewRequest, err := sysutils.DecodeJSONBody[PreviewRequest](r)
	if err != nil {
		h.handleError(ctx, w, r, &ErrorInvalidRequestFormat)
		return
	}

	preview, svcErr := h.cimdService.Preview(ctx, previewRequest.ClientID)
	if svcErr != nil {
		h.handleError(ctx, w, r, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, preview)
}

// handleError writes a service error with the status for its type, and logs a server error.
func (h *cimdHandler) handleError(ctx context.Context, w http.ResponseWriter, r *http.Request,
	svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		statusCode = http.StatusBadRequest
	}

	if statusCode == http.StatusInternalServerError {
		h.logger.Error(ctx, "Internal server error processing Client ID Metadata Document request",
			log.String("method", r.Method),
			log.String("path", r.URL.Path),
			log.String("error_code", svcErr.Code),
			log.String("error", svcErr.Error.DefaultValue),
		)
	}

	sysutils.WriteErrorResponse(ctx, w, statusCode, apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	})
}
