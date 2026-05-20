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

package flowmgt

import (
	"net/http"
	"strconv"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	handlerLoggerComponentName = "FlowMgtHandler"
	logKeyFlowID               = "flowID"
	logKeyVersion              = "version"
	logKeyCount                = "count"
)

// Path and query parameter keys
const (
	pathParamFlowID    = "flowId"
	pathParamVersion   = "version"
	queryParamFlowType = "flowType"
	queryParamLimit    = "limit"
	queryParamOffset   = "offset"
)

// flowMgtHandler handles HTTP requests for flow management
type flowMgtHandler struct {
	service FlowMgtServiceInterface
	logger  *log.Logger
}

// newFlowMgtHandler creates a new instance of flowMgtHandler.
func newFlowMgtHandler(
	service FlowMgtServiceInterface,
) *flowMgtHandler {
	return &flowMgtHandler{
		service: service,
		logger:  log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName)),
	}
}

// Flow management HTTP handler methods

// listFlows handles GET requests to list flow definitions with pagination and optional filtering.
func (h *flowMgtHandler) listFlows(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit, offset, svcErr := parsePaginationParams(r)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	flowTypeStr := r.URL.Query().Get(queryParamFlowType)
	flowType := common.FlowType(flowTypeStr)

	flowList, svcErr := h.service.ListFlows(ctx, limit, offset, flowType)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	utils.WriteSuccessResponse(w, http.StatusOK, flowList)
	h.logger.Debug("Flows listed successfully", log.Int(logKeyCount, flowList.Count))
}

// createFlow handles POST requests to create a new flow definition.
func (h *flowMgtHandler) createFlow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flowDefRequest, err := utils.DecodeJSONBody[FlowDefinitionRequest](r)
	if err != nil {
		handleInvalidRequestError(w)
		return
	}

	sanitized := sanitizeFlowDefinitionRequest(flowDefRequest)
	createdFlow, svcErr := h.service.CreateFlow(ctx, sanitized)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	utils.WriteSuccessResponse(w, http.StatusCreated, createdFlow)
	h.logger.Debug("Flow created successfully", log.String(logKeyFlowID, createdFlow.ID))
}

// getFlow handles GET requests to retrieve a flow definition by its ID.
func (h *flowMgtHandler) getFlow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flowID := r.PathValue(pathParamFlowID)
	if flowID == "" {
		handleError(w, &ErrorMissingFlowID)
		return
	}

	flow, svcErr := h.service.GetFlow(ctx, flowID)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	utils.WriteSuccessResponse(w, http.StatusOK, flow)
	h.logger.Debug("Flow retrieved successfully", log.String(logKeyFlowID, flowID))
}

// updateFlow handles PUT requests to update an existing flow definition.
func (h *flowMgtHandler) updateFlow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flowID := r.PathValue(pathParamFlowID)
	if flowID == "" {
		handleError(w, &ErrorMissingFlowID)
		return
	}

	flowDefRequest, err := utils.DecodeJSONBody[FlowDefinitionRequest](r)
	if err != nil {
		handleInvalidRequestError(w)
		return
	}

	sanitized := sanitizeFlowDefinitionRequest(flowDefRequest)
	updatedFlow, svcErr := h.service.UpdateFlow(ctx, flowID, sanitized)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	utils.WriteSuccessResponse(w, http.StatusOK, updatedFlow)
	h.logger.Debug("Flow updated successfully", log.String(logKeyFlowID, flowID))
}

// deleteFlow handles DELETE requests to remove a flow definition by its ID.
func (h *flowMgtHandler) deleteFlow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flowID := r.PathValue(pathParamFlowID)
	if flowID == "" {
		handleError(w, &ErrorMissingFlowID)
		return
	}

	svcErr := h.service.DeleteFlow(ctx, flowID)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	h.logger.Debug("Flow deleted successfully", log.String(logKeyFlowID, flowID))
}

// Flow version management HTTP handler methods

// listFlowVersions handles GET requests to list all versions of a specific flow definition.
func (h *flowMgtHandler) listFlowVersions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flowID := r.PathValue(pathParamFlowID)
	if flowID == "" {
		handleError(w, &ErrorMissingFlowID)
		return
	}

	versionList, svcErr := h.service.ListFlowVersions(ctx, flowID)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	utils.WriteSuccessResponse(w, http.StatusOK, versionList)
	h.logger.Debug("Flow versions listed successfully", log.String(logKeyFlowID, flowID))
}

// getFlowVersion handles GET requests to retrieve a specific version of a flow definition.
func (h *flowMgtHandler) getFlowVersion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flowID := r.PathValue(pathParamFlowID)
	versionStr := r.PathValue(pathParamVersion)

	if flowID == "" || versionStr == "" {
		handleError(w, &ErrorMissingFlowID)
		return
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil || version <= 0 {
		handleError(w, &ErrorInvalidVersion)
		return
	}

	flowVersion, svcErr := h.service.GetFlowVersion(ctx, flowID, version)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	utils.WriteSuccessResponse(w, http.StatusOK, flowVersion)
	h.logger.Debug("Flow version retrieved successfully",
		log.String(logKeyFlowID, flowID), log.Int(logKeyVersion, version))
}

// restoreFlowVersion handles POST requests to restore a specific version of a flow definition.
func (h *flowMgtHandler) restoreFlowVersion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flowID := r.PathValue(pathParamFlowID)
	if flowID == "" {
		handleError(w, &ErrorMissingFlowID)
		return
	}

	request, err := utils.DecodeJSONBody[RestoreVersionRequest](r)
	if err != nil {
		handleInvalidRequestError(w)
		return
	}

	flow, svcErr := h.service.RestoreFlowVersion(ctx, flowID, request.Version)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	utils.WriteSuccessResponse(w, http.StatusOK, flow)
	h.logger.Debug("Flow version restored successfully",
		log.String(logKeyFlowID, flowID), log.Int(logKeyVersion, request.Version))
}

// parsePaginationParams extracts and validates pagination parameters from the request.
func parsePaginationParams(r *http.Request) (int, int, *serviceerror.ServiceError) {
	limitStr := r.URL.Query().Get(queryParamLimit)
	offsetStr := r.URL.Query().Get(queryParamOffset)

	limit := defaultPageSize
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 0 {
			return 0, 0, &ErrorInvalidLimit
		}
		limit = parsedLimit
	}

	offset := 0
	if offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err != nil || parsedOffset < 0 {
			return 0, 0, &ErrorInvalidOffset
		}
		offset = parsedOffset
	}

	return limit, offset, nil
}

// sanitizeFlowDefinitionRequest sanitizes input for creating or updating a flow definition.
// TODO: Currently we're storing node representation as it is. In the future, we should sanitize and
// validate it properly.
func sanitizeFlowDefinitionRequest(req *FlowDefinitionRequest) *FlowDefinition {
	sanitized := &FlowDefinition{
		Handle:   utils.SanitizeString(req.Handle),
		Name:     utils.SanitizeString(req.Name),
		FlowType: req.FlowType,
		Nodes:    req.Nodes,
	}

	return sanitized
}

// handleInvalidRequestError writes a standardized error response for invalid requests.
func handleInvalidRequestError(w http.ResponseWriter) {
	errResp := apierror.ErrorResponse{
		Code:        ErrorInvalidRequestFormat.Code,
		Message:     ErrorInvalidRequestFormat.Error,
		Description: ErrorInvalidRequestFormat.ErrorDescription,
	}
	utils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
}

// handleError writes an error response based on the provided ServiceError.
func handleError(w http.ResponseWriter, svcErr *serviceerror.ServiceError) {
	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	statusCode := http.StatusBadRequest
	switch svcErr.Code {
	case ErrorFlowNotFound.Code, ErrorVersionNotFound.Code:
		statusCode = http.StatusNotFound
	case ErrorDuplicateFlowID.Code:
		statusCode = http.StatusConflict
	case serviceerror.InternalServerError.Code:
		statusCode = http.StatusInternalServerError
		log.GetLogger().Error("Internal server error in flow handler",
			log.String("code", svcErr.Code),
			log.String("error", svcErr.Error.DefaultValue),
			log.String("description", svcErr.ErrorDescription.DefaultValue))
	}

	utils.WriteErrorResponse(w, statusCode, errResp)
}
