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

package export

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"strings"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
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
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	// Export resources using the export service
	exportResponse, svcErr := eh.service.ExportResources(r.Context(), exportRequest)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ServerErrorType {
			logger.Error("Error exporting resources", log.Any("serviceError", svcErr))
		}
		eh.handleError(w, svcErr)
		return
	}

	jsonResponse := JSONExportResponse{
		Resources:            buildCombinedResources(exportResponse.Files),
		EnvironmentVariables: "",
	}
	if exportResponse.EnvFile != nil {
		jsonResponse.EnvironmentVariables = exportResponse.EnvFile.Content
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, jsonResponse)
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

// HandleExportZipRequest handles the export request and returns a ZIP file containing all resources.
func (eh *exportHandler) HandleExportZipRequest(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ExportHandler"))

	exportRequest, err := sysutils.DecodeJSONBody[ExportRequest](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidRequest.Code,
			Message:     ErrorInvalidRequest.Error,
			Description: ErrorInvalidRequest.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	// Export resources using the export service
	exportResponse, svcErr := eh.service.ExportResources(r.Context(), exportRequest)
	if svcErr != nil {
		if svcErr.Type == serviceerror.ServerErrorType {
			logger.Error("Error exporting resources", log.Any("serviceError", svcErr))
		}
		eh.handleError(w, svcErr)
		return
	}

	// Generate ZIP file and send response
	if err := eh.generateAndSendZipResponse(w, logger, exportResponse); err != nil {
		logger.Error("Error generating ZIP response", log.Error(err))
		errResp := apierror.ErrorResponse{
			Code:        serviceerror.InternalServerError.Code,
			Message:     serviceerror.InternalServerError.Error,
			Description: serviceerror.InternalServerError.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusInternalServerError, errResp)
		return
	}
}

// generateAndSendZipResponse creates a ZIP file from export files and sends it as HTTP response.
func (eh *exportHandler) generateAndSendZipResponse(
	w http.ResponseWriter, logger *log.Logger, exportResponse *ExportResponse) error {
	// Create ZIP file in memory
	var zipBuffer bytes.Buffer
	zipWriter := zip.NewWriter(&zipBuffer)

	// Add each file to the ZIP
	for _, file := range exportResponse.Files {
		// Create the full path within the ZIP
		zipPath := file.FileName
		if file.FolderPath != "" {
			zipPath = file.FolderPath + "/" + file.FileName
		}

		fileWriter, err := zipWriter.Create(zipPath)
		if err != nil {
			logger.Error("Error creating file in ZIP", log.String("zipPath", zipPath), log.Error(err))
			return fmt.Errorf("failed to create file in ZIP: %w", err)
		}

		if _, err := fileWriter.Write([]byte(file.Content)); err != nil {
			logger.Error("Error writing file content to ZIP", log.String("zipPath", zipPath), log.Error(err))
			return fmt.Errorf("failed to write content to ZIP: %w", err)
		}
	}

	if exportResponse.EnvFile != nil {
		envWriter, err := zipWriter.Create(exportResponse.EnvFile.FileName)
		if err != nil {
			logger.Error("Error creating env file in ZIP", log.String("fileName", exportResponse.EnvFile.FileName),
				log.Error(err))
			return fmt.Errorf("failed to create env file in ZIP: %w", err)
		}

		if _, err = envWriter.Write([]byte(exportResponse.EnvFile.Content)); err != nil {
			logger.Error("Error writing env file content to ZIP",
				log.String("fileName", exportResponse.EnvFile.FileName),
				log.Error(err))
			return fmt.Errorf("failed to write env file content to ZIP: %w", err)
		}
	}

	// Close the ZIP writer
	if err := zipWriter.Close(); err != nil {
		logger.Error("Error closing ZIP writer", log.Error(err))
		return fmt.Errorf("failed to close ZIP writer: %w", err)
	}

	// Set headers for ZIP file download
	w.Header().Set(serverconst.ContentTypeHeaderName, "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=exported_resources.zip")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", zipBuffer.Len()))
	w.WriteHeader(http.StatusOK)

	// Write the ZIP content
	if _, err := w.Write(zipBuffer.Bytes()); err != nil {
		logger.Error("Error writing ZIP response", log.Error(err))
		return fmt.Errorf("failed to write ZIP response: %w", err)
	}

	return nil
}

// handleError handles service errors and sends appropriate HTTP responses.
func (eh *exportHandler) handleError(w http.ResponseWriter, svcErr *serviceerror.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == serviceerror.ClientErrorType {
		statusCode = http.StatusBadRequest
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	sysutils.WriteErrorResponse(w, statusCode, errResp)
}
