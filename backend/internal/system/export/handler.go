// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package export

import (
	"context"
	"net/http"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// exportHandler defines the handler for managing export API requests.
type exportHandler struct {
	service ExportServiceInterface
}

func newExportHandler(service ExportServiceInterface) *exportHandler {
	return &exportHandler{
		service: service,
	}
}

// HandleExportJSONRequest handles the export request and returns JSON with files.
func (eh *exportHandler) HandleExportRequest(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ExportHandler"))

	exportRequest, err := sysutils.DecodeJSONBody[ExportRequest](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidRequest.Code,
			Message:     ErrorInvalidRequest.Error,
			Description: ErrorInvalidRequest.ErrorDescription,
		}
		sysutils.WriteErrorResponse(r.Context(), w, http.StatusBadRequest, errResp)
		return
	}

	// Export resources using the export service
	exportResponse, svcErr := eh.service.ExportResources(r.Context(), exportRequest)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			logger.Error(r.Context(), "Error exporting resources", log.Any("serviceError", svcErr))
		}
		eh.handleError(r.Context(), w, svcErr)
		return
	}

	jsonResponse := JSONExportResponse{
		Resources:            buildCombinedResources(exportResponse.Files),
		EnvironmentVariables: "",
	}
	if exportResponse.EnvFile != nil {
		jsonResponse.EnvironmentVariables = exportResponse.EnvFile.Content
	}

	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, jsonResponse)
}

// CombineResources joins exported files into the single document the import API takes, so a caller
// that applies an export without going through HTTP builds the same payload the endpoint returns.
func CombineResources(files []ExportFile) string {
	return buildCombinedResources(files)
}

func buildCombinedResources(files []ExportFile) string {
	var builder strings.Builder

	for i, file := range files {
		if i > 0 {
			builder.WriteString("\n---\n")
		}
		builder.WriteString("# File: ")
		builder.WriteString(file.FileName)
		builder.WriteString("\n")
		builder.WriteString(file.Content)
	}

	return builder.String()
}

// handleError handles service errors and sends appropriate HTTP responses.
func (eh *exportHandler) handleError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		statusCode = http.StatusBadRequest
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	sysutils.WriteErrorResponse(ctx, w, statusCode, errResp)
}
