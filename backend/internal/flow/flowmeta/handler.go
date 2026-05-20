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

package flowmeta

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// flowMetaHandler handles flow metadata HTTP requests.
type flowMetaHandler struct {
	flowMetaService FlowMetaServiceInterface
	logger          *log.Logger
}

// newFlowMetaHandler creates a new instance of flowMetaHandler.
func newFlowMetaHandler(flowMetaService FlowMetaServiceInterface) *flowMetaHandler {
	return &flowMetaHandler{
		flowMetaService: flowMetaService,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowMetaHandler")),
	}
}

// HandleGetFlowMetadata handles the GET /flow/meta endpoint.
func (h *flowMetaHandler) HandleGetFlowMetadata(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	metaType := sysutils.SanitizeString(r.URL.Query().Get("type"))
	id := sysutils.SanitizeString(r.URL.Query().Get("id"))

	var language *string
	var namespace *string

	if lang := r.URL.Query().Get("language"); lang != "" {
		language = &lang
	}

	if ns := r.URL.Query().Get("namespace"); ns != "" {
		namespace = &ns
	}

	// Validate parameter combinations: id requires type, and type requires id
	if id != "" && metaType == "" {
		handleServiceError(w, &ErrorMissingType)
		return
	}

	if metaType != "" && id == "" {
		handleServiceError(w, &ErrorMissingID)
		return
	}
	if language != nil {
		lang := sysutils.SanitizeString(*language)
		language = &lang
	}
	if namespace != nil {
		ns := sysutils.SanitizeString(*namespace)
		namespace = &ns
	}

	// Call service
	metadata, svcErr := h.flowMetaService.GetFlowMetadata(r.Context(), MetaType(metaType), id, language, namespace)
	if svcErr != nil {
		handleServiceError(w, svcErr)
		return
	}

	// Return success response
	sysutils.WriteSuccessResponse(w, http.StatusOK, metadata)
	h.logger.Debug("Flow metadata retrieved successfully",
		log.String("type", metaType),
		log.String("id", id))
}

// handleServiceError converts service errors to appropriate HTTP responses.
func handleServiceError(w http.ResponseWriter, svcErr *serviceerror.ServiceError) {
	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	statusCode := http.StatusInternalServerError
	if svcErr.Type == serviceerror.ClientErrorType {
		// Determine specific client error status code
		if svcErr.Code == ErrorApplicationNotFound.Code || svcErr.Code == ErrorOUNotFound.Code {
			statusCode = http.StatusNotFound
		} else {
			statusCode = http.StatusBadRequest
		}
	}

	sysutils.WriteErrorResponse(w, statusCode, errResp)
}
