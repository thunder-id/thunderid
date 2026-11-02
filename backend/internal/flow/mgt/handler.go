// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"net/http"
	"strconv"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/middleware"
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
	queryParamOUID     = "ouId"
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
		handleError(ctx, w, svcErr)
		return
	}

	flowTypeStr := r.URL.Query().Get(queryParamFlowType)
	flowType := providers.FlowType(flowTypeStr)

	ouID, svcErr := parseOUFilter(r)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	// With an organization unit filter, the list is of the flows it owns and the ones shared with it. Without one, it
	// is of every flow.
	var flowList *FlowListResponse
	if ouID != "" {
		flowList, svcErr = h.service.ListFlowsForOU(ctx, ouID, flowType, limit, offset)
	} else {
		flowList, svcErr = h.service.ListFlows(ctx, limit, offset, flowType)
	}
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	utils.WriteSuccessResponse(ctx, w, http.StatusOK, flowList)
	h.logger.Debug(ctx, "Flows listed successfully", log.Int(logKeyCount, flowList.Count))
}

// createFlow handles POST requests to create a new flow definition.
func (h *flowMgtHandler) createFlow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flowDefRequest, err := utils.DecodeJSONBody[FlowDefinitionRequest](r)
	if err != nil {
		handleInvalidRequestError(ctx, w)
		return
	}

	sanitized := sanitizeFlowDefinitionRequest(flowDefRequest)
	createdFlow, svcErr := h.service.CreateFlow(ctx, sanitized)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	utils.WriteSuccessResponse(ctx, w, http.StatusCreated, createdFlow)
	h.logger.Debug(ctx, "Flow created successfully", log.String(logKeyFlowID, createdFlow.ID))
}

// getFlow handles GET requests to retrieve a flow definition by its ID.
func (h *flowMgtHandler) getFlow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flowID := r.PathValue(pathParamFlowID)
	if flowID == "" {
		handleError(ctx, w, &ErrorMissingFlowID)
		return
	}

	// With an organization unit, the flow is as that unit sees it. Without one, it is the whole definition.
	if ouID := r.URL.Query().Get(queryParamOUID); ouID != "" {
		view, svcErr := h.service.GetFlowForOU(ctx, flowID, ouID)
		if svcErr != nil {
			handleError(ctx, w, svcErr)
			return
		}
		if view.Shared != nil {
			utils.WriteSuccessResponse(ctx, w, http.StatusOK, view.Shared)
		} else {
			utils.WriteSuccessResponse(ctx, w, http.StatusOK, view.Owned)
		}
		h.logger.Debug(ctx, "Flow retrieved successfully", log.String(logKeyFlowID, flowID))
		return
	}

	flow, svcErr := h.service.GetFlow(ctx, flowID)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	utils.WriteSuccessResponse(ctx, w, http.StatusOK, flow)
	h.logger.Debug(ctx, "Flow retrieved successfully", log.String(logKeyFlowID, flowID))
}

// getFlowUsages handles GET requests to retrieve the resources that reference a flow.
func (h *flowMgtHandler) getFlowUsages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flowID := r.PathValue(pathParamFlowID)
	if flowID == "" {
		handleError(ctx, w, &ErrorMissingFlowID)
		return
	}

	if !h.checkFlowAccess(w, r, flowID, false) {
		return
	}

	result, svcErr := h.service.GetFlowUsages(ctx, flowID)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	utils.WriteSuccessResponse(ctx, w, http.StatusOK, result)
	h.logger.Debug(ctx, "Flow usages retrieved successfully", log.String(logKeyFlowID, flowID))
}

// updateFlow handles PUT requests to update an existing flow definition.
func (h *flowMgtHandler) updateFlow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flowID := r.PathValue(pathParamFlowID)
	if flowID == "" {
		handleError(ctx, w, &ErrorMissingFlowID)
		return
	}

	flowDefRequest, err := utils.DecodeJSONBody[FlowDefinitionRequest](r)
	if err != nil {
		handleInvalidRequestError(ctx, w)
		return
	}

	sanitized := sanitizeFlowDefinitionRequest(flowDefRequest)
	updatedFlow, svcErr := h.service.UpdateFlow(ctx, flowID, sanitized)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	utils.WriteSuccessResponse(ctx, w, http.StatusOK, updatedFlow)
	h.logger.Debug(ctx, "Flow updated successfully", log.String(logKeyFlowID, flowID))
}

// deleteFlow handles DELETE requests to remove a flow definition by its ID.
func (h *flowMgtHandler) deleteFlow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flowID := r.PathValue(pathParamFlowID)
	if flowID == "" {
		handleError(ctx, w, &ErrorMissingFlowID)
		return
	}

	if !h.checkFlowAccess(w, r, flowID, true) {
		return
	}

	svcErr := h.service.DeleteFlow(ctx, flowID)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	h.logger.Debug(ctx, "Flow deleted successfully", log.String(logKeyFlowID, flowID))
}

// Flow version management HTTP handler methods

// listFlowVersions handles GET requests to list all versions of a specific flow definition.
func (h *flowMgtHandler) listFlowVersions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flowID := r.PathValue(pathParamFlowID)
	if flowID == "" {
		handleError(ctx, w, &ErrorMissingFlowID)
		return
	}

	if !h.checkFlowAccess(w, r, flowID, false) {
		return
	}

	versionList, svcErr := h.service.ListFlowVersions(ctx, flowID)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	utils.WriteSuccessResponse(ctx, w, http.StatusOK, versionList)
	h.logger.Debug(ctx, "Flow versions listed successfully", log.String(logKeyFlowID, flowID))
}

// getFlowVersion handles GET requests to retrieve a specific version of a flow definition.
func (h *flowMgtHandler) getFlowVersion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flowID := r.PathValue(pathParamFlowID)
	versionStr := r.PathValue(pathParamVersion)

	if flowID == "" || versionStr == "" {
		handleError(ctx, w, &ErrorMissingFlowID)
		return
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil || version <= 0 {
		handleError(ctx, w, &ErrorInvalidVersion)
		return
	}

	if !h.checkFlowAccess(w, r, flowID, false) {
		return
	}

	flowVersion, svcErr := h.service.GetFlowVersion(ctx, flowID, version)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	utils.WriteSuccessResponse(ctx, w, http.StatusOK, flowVersion)
	h.logger.Debug(ctx, "Flow version retrieved successfully",
		log.String(logKeyFlowID, flowID), log.Int(logKeyVersion, version))
}

// restoreFlowVersion handles POST requests to restore a specific version of a flow definition.
func (h *flowMgtHandler) restoreFlowVersion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	flowID := r.PathValue(pathParamFlowID)
	if flowID == "" {
		handleError(ctx, w, &ErrorMissingFlowID)
		return
	}

	request, err := utils.DecodeJSONBody[RestoreVersionRequest](r)
	if err != nil {
		handleInvalidRequestError(ctx, w)
		return
	}

	if !h.checkFlowAccess(w, r, flowID, true) {
		return
	}

	flow, svcErr := h.service.RestoreFlowVersion(ctx, flowID, request.Version)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	utils.WriteSuccessResponse(ctx, w, http.StatusOK, flow)
	h.logger.Debug(ctx, "Flow version restored successfully",
		log.String(logKeyFlowID, flowID), log.Int(logKeyVersion, request.Version))
}

// checkFlowAccess checks, when the request names an organization unit, that it may read or with write change the
// definition of the flow. It answers the request itself when it may not, and reports whether to carry on.
func (h *flowMgtHandler) checkFlowAccess(w http.ResponseWriter, r *http.Request, flowID string, write bool) bool {
	ouID := r.URL.Query().Get(queryParamOUID)
	if ouID == "" {
		return true
	}
	if svcErr := h.service.CheckFlowAccess(r.Context(), flowID, ouID, write); svcErr != nil {
		handleError(r.Context(), w, svcErr)
		return false
	}
	return true
}

// parseOUFilter reads the organization unit the request filters on, empty when it names none. The only filter
// accepted is a single ouId eq expression.
func parseOUFilter(r *http.Request) (string, *tidcommon.ServiceError) {
	filters, err := filter.ParseEqualityFilterParam(r.URL.Query())
	if err != nil {
		return "", &ErrorInvalidFilter
	}
	ouID, rest, err := filter.SplitOUIDFilter(filters)
	if err != nil || len(rest) > 0 {
		return "", &ErrorInvalidFilter
	}
	return ouID, nil
}

// parsePaginationParams extracts and validates pagination parameters from the request.
func parsePaginationParams(r *http.Request) (int, int, *tidcommon.ServiceError) {
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
	var interceptors []providers.InterceptorDefinition
	if len(req.Interceptors) > 0 {
		interceptors = make([]providers.InterceptorDefinition, len(req.Interceptors))
		for i, ic := range req.Interceptors {
			interceptors[i] = ic
			interceptors[i].Name = utils.SanitizeString(ic.Name)
		}
	}

	sanitized := &FlowDefinition{
		Handle:       utils.SanitizeString(req.Handle),
		Name:         utils.SanitizeString(req.Name),
		FlowType:     req.FlowType,
		Interceptors: interceptors,
		Nodes:        req.Nodes,
		OUID:         utils.SanitizeString(req.OUID),
	}

	return sanitized
}

// handleInvalidRequestError writes a standardized error response for invalid requests.
func handleInvalidRequestError(ctx context.Context, w http.ResponseWriter) {
	errResp := apierror.ErrorResponse{
		Code:        ErrorInvalidRequestFormat.Code,
		Message:     ErrorInvalidRequestFormat.Error,
		Description: ErrorInvalidRequestFormat.ErrorDescription,
	}
	utils.WriteErrorResponse(ctx, w, http.StatusBadRequest, errResp)
}

// handleError writes an error response based on the provided ServiceError.
func handleError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}

	statusCode := http.StatusBadRequest
	switch svcErr.Code {
	case ErrorFlowNotFound.Code, ErrorVersionNotFound.Code, ErrorSharingPolicyNotFound.Code:
		statusCode = http.StatusNotFound
	case ErrorDuplicateFlowID.Code, ErrorSharingPolicyExists.Code, ErrorSharingPolicyVersionMismatch.Code,
		ErrorFlowConflictsWithSharingPolicy.Code, ErrorFlowConflictsWithSavedValues.Code:
		statusCode = http.StatusConflict
	case ErrorVariableNotEditable.Code, ErrorSharedFlowReadOnly.Code, tidcommon.ErrorUnauthorized.Code:
		statusCode = http.StatusForbidden
	case tidcommon.InternalServerError.Code:
		statusCode = http.StatusInternalServerError
		log.GetLogger().Error(ctx, "Internal server error in flow handler",
			log.String("code", svcErr.Code),
			log.String("error", svcErr.Error.DefaultValue),
			log.String("description", svcErr.ErrorDescription.DefaultValue))
	}

	utils.WriteErrorResponse(ctx, w, statusCode, errResp)
}

// ---------------------------------------------------------------------------
// Sharing, overlay rules and variables
// ---------------------------------------------------------------------------

const pathParamPolicyID = "policyId"

// createSharingPolicy handles POST requests to record how a flow is shared.
func (h *flowMgtHandler) createSharingPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wire, ok := sharing.DecodePolicyBody[sharing.PolicyCreateBody](ctx, w, r, ErrorInvalidRequestFormat)
	if !ok {
		return
	}
	req := wire.ToPolicyRequest()
	// A reach not bounded by where the issuer sits is a deployment decision, so this API leaves it to a resource
	// file or an import.
	if req.IsDeploymentWide() {
		handleError(ctx, w, &ErrorDeploymentWideSharingNotAllowed)
		return
	}

	flowID := r.PathValue(pathParamFlowID)
	policy, svcErr := h.service.CreateSharingPolicy(ctx, flowID, req)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	utils.WriteSuccessResponse(ctx, w, http.StatusCreated, sharing.ToPolicyResponse(policy))
	h.logger.Debug(ctx, "Flow sharing policy created", log.String(logKeyFlowID, flowID))
}

// listSharingPolicies handles GET requests to list the sharing policies of a flow.
func (h *flowMgtHandler) listSharingPolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit, offset, svcErr := parsePaginationParams(r)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	flowID := r.PathValue(pathParamFlowID)
	list, svcErr := h.service.ListSharingPolicies(ctx, flowID, limit, offset)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	utils.WriteSuccessResponse(ctx, w, http.StatusOK,
		sharing.ToPolicyListResponse(list, "/flows/"+flowID+"/sharing-policies", limit, offset))
}

// getSharingPolicy handles GET requests to retrieve one sharing policy of a flow.
func (h *flowMgtHandler) getSharingPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	policy, svcErr := h.service.GetSharingPolicy(ctx, r.PathValue(pathParamFlowID), r.PathValue(pathParamPolicyID))
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	utils.WriteSuccessResponse(ctx, w, http.StatusOK, sharing.ToPolicyResponse(policy))
}

// updateSharingPolicy handles PUT requests to replace a sharing policy of a flow.
func (h *flowMgtHandler) updateSharingPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wire, ok := sharing.DecodePolicyBody[sharing.PolicyUpdateBody](ctx, w, r, ErrorInvalidRequestFormat)
	if !ok {
		return
	}
	policy, svcErr := h.service.UpdateSharingPolicy(
		ctx, r.PathValue(pathParamFlowID), r.PathValue(pathParamPolicyID), wire.ToPolicyRequest())
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	utils.WriteSuccessResponse(ctx, w, http.StatusOK, sharing.ToPolicyResponse(policy))
}

// deleteSharingPolicy handles DELETE requests to remove a sharing policy of a flow.
func (h *flowMgtHandler) deleteSharingPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if svcErr := h.service.DeleteSharingPolicy(
		ctx, r.PathValue(pathParamFlowID), r.PathValue(pathParamPolicyID)); svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getOverlayRules handles GET requests for what an organization unit may do with the variables of a flow.
func (h *flowMgtHandler) getOverlayRules(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	overlay, svcErr := h.service.GetFlowOverlayRules(
		ctx, r.PathValue(pathParamFlowID), r.URL.Query().Get(queryParamOUID))
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	utils.WriteSuccessResponse(ctx, w, http.StatusOK, sharing.ToResolvedOverlayResponse(overlay))
}

// getVariables handles GET requests for the values an organization unit has set for the variables of a flow.
func (h *flowMgtHandler) getVariables(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp, svcErr := h.service.GetVariables(ctx, r.PathValue(pathParamFlowID), r.URL.Query().Get(queryParamOUID))
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	utils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// setVariables handles PUT requests that set the values an organization unit holds for the variables of a flow.
func (h *flowMgtHandler) setVariables(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, err := utils.DecodeJSONBodyStrict[SetVariablesRequest](r)
	if err != nil || req.Values == nil {
		handleInvalidRequestError(ctx, w)
		return
	}
	resp, svcErr := h.service.SetVariables(ctx, r.PathValue(pathParamFlowID), req.OUID, *req)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	utils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// registerSharingRoutes registers the routes that share a flow and let the organization units it is shared with set
// its variables.
func registerSharingRoutes(mux *http.ServeMux, handler *flowMgtHandler) {
	noContent := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
	options := func(methods ...string) middleware.CORSOptions {
		return middleware.CORSOptions{
			AllowedMethods:   methods,
			AllowedHeaders:   middleware.DefaultAllowedHeaders,
			AllowCredentials: true,
			MaxAge:           600,
		}
	}

	collection := options("GET", "POST")
	mux.HandleFunc(middleware.WithCORS("POST /flows/{flowId}/sharing-policies",
		handler.createSharingPolicy, collection))
	mux.HandleFunc(middleware.WithCORS("GET /flows/{flowId}/sharing-policies",
		handler.listSharingPolicies, collection))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /flows/{flowId}/sharing-policies", noContent, collection))

	item := options("GET", "PUT", "DELETE")
	mux.HandleFunc(middleware.WithCORS("GET /flows/{flowId}/sharing-policies/{policyId}",
		handler.getSharingPolicy, item))
	mux.HandleFunc(middleware.WithCORS("PUT /flows/{flowId}/sharing-policies/{policyId}",
		handler.updateSharingPolicy, item))
	mux.HandleFunc(middleware.WithCORS("DELETE /flows/{flowId}/sharing-policies/{policyId}",
		handler.deleteSharingPolicy, item))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /flows/{flowId}/sharing-policies/{policyId}", noContent, item))

	overlay := options("GET")
	mux.HandleFunc(middleware.WithCORS("GET /flows/{flowId}/overlay-rules", handler.getOverlayRules, overlay))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /flows/{flowId}/overlay-rules", noContent, overlay))

	variables := options("GET", "PUT")
	mux.HandleFunc(middleware.WithCORS("GET /flows/{flowId}/variables", handler.getVariables, variables))
	mux.HandleFunc(middleware.WithCORS("PUT /flows/{flowId}/variables", handler.setVariables, variables))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /flows/{flowId}/variables", noContent, variables))
}
