// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// agentHandler handles HTTP requests for agent operations.
type agentHandler struct {
	service AgentServiceInterface
}

// newAgentHandler constructs an agentHandler bound to the given service.
func newAgentHandler(service AgentServiceInterface) *agentHandler {
	return &agentHandler{service: service}
}

// HandleAgentListRequest handles GET /agents.
func (h *agentHandler) HandleAgentListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AgentHandler"))

	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}

	filters, svcErr := parseFilterParams(r.URL.Query())
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}

	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	var resp *model.AgentListResponse
	if query := r.URL.Query(); query.Has(projectIDQueryParam) {
		resp, svcErr = h.service.GetAgentListByProject(ctx, query.Get(projectIDQueryParam), limit, offset,
			filters, includeDisplay)
	} else {
		resp, svcErr = h.service.GetAgentList(ctx, limit, offset, filters, includeDisplay)
	}
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
	logger.Debug(ctx, "Agent list returned",
		log.Int("limit", limit), log.Int("offset", offset),
		log.Int("totalResults", resp.TotalResults), log.Int("count", resp.Count))
}

// HandleAgentPostRequest handles POST /agents.
func (h *agentHandler) HandleAgentPostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	req, err := sysutils.DecodeJSONBody[model.CreateAgentRequest](r)
	if err != nil {
		var valErr *sysutils.ValidationError
		if errors.As(err, &valErr) {
			sysutils.WriteStructuredErrorResponse(w, http.StatusBadRequest, "Validation Failed", valErr.Errors)
			return
		}
		writeServiceError(ctx, w, &ErrorInvalidRequestFormat)
		return
	}

	agent := &providers.Agent{
		OUID:        req.OUID,
		OUHandle:    req.OUHandle,
		Type:        req.Type,
		Description: req.Description,
		LogoURL:     req.LogoURL,
		Owner:       req.Owner,
		Delegated:   req.Delegated,
		Attributes:  req.Attributes,
		ValidUntil:  req.ValidUntil,
		ProjectID:   req.ProjectID,
		Kind:        req.Kind,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:                req.AuthFlowID,
			DeviceGrantFlowID:         req.DeviceGrantFlowID,
			DeviceGrantFlowHandle:     req.DeviceGrantFlowHandle,
			RegistrationFlowID:        req.RegistrationFlowID,
			IsRegistrationFlowEnabled: req.IsRegistrationFlowEnabled,
			RecoveryFlowID:            req.RecoveryFlowID,
			IsRecoveryFlowEnabled:     req.IsRecoveryFlowEnabled,
			SignOutFlowID:             req.SignOutFlowID,
			ThemeID:                   req.ThemeID,
			LayoutID:                  req.LayoutID,
			Assertion:                 req.Assertion,
			LoginConsent:              req.LoginConsent,
			AllowedUserTypes:          req.AllowedUserTypes,
			AllowedAgentTypes:         req.AllowedAgentTypes,
			PasskeyAllowedOrigins:     req.PasskeyAllowedOrigins,
			Attestation:               req.Attestation,
		},
		InboundAuthConfig: req.InboundAuthConfig,
	}

	resp, svcErr := h.service.CreateAgent(ctx, agent)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, resp)
}

// HandleAgentGetRequest handles GET /agents/{id}.
func (h *agentHandler) HandleAgentGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeServiceError(ctx, w, &ErrorMissingAgentID)
		return
	}
	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	resp, svcErr := h.service.GetAgent(ctx, id, includeDisplay)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// HandleAgentPutRequest handles PUT /agents/{id}.
func (h *agentHandler) HandleAgentPutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeServiceError(ctx, w, &ErrorMissingAgentID)
		return
	}

	req, err := sysutils.DecodeJSONBody[model.UpdateAgentRequest](r)
	if err != nil {
		var valErr *sysutils.ValidationError
		if errors.As(err, &valErr) {
			sysutils.WriteStructuredErrorResponse(w, http.StatusBadRequest, "Validation Failed", valErr.Errors)
			return
		}
		writeServiceError(ctx, w, &ErrorInvalidRequestFormat)
		return
	}

	resp, svcErr := h.service.UpdateAgent(ctx, id, req)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// HandleAgentDeleteRequest handles DELETE /agents/{id}.
func (h *agentHandler) HandleAgentDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeServiceError(ctx, w, &ErrorMissingAgentID)
		return
	}
	if svcErr := h.service.DeleteAgent(ctx, id); svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
}

// HandleClientSecretRegenerateRequest handles POST /agents/{id}/client-secret/regenerate. The new secret is
// in the response, the only time it can be read.
func (h *agentHandler) HandleClientSecretRegenerateRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeServiceError(ctx, w, &ErrorMissingAgentID)
		return
	}
	regenerated, svcErr := h.service.RegenerateAgentSecret(ctx, id)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, regenerated)
}

// HandleAgentGroupsRequest handles GET /agents/{id}/groups.
func (h *agentHandler) HandleAgentGroupsRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeServiceError(ctx, w, &ErrorMissingAgentID)
		return
	}
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}

	resp, svcErr := h.service.GetAgentGroups(ctx, id, limit, offset)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// HandleAgentRolesRequest handles GET /agents/{id}/roles.
func (h *agentHandler) HandleAgentRolesRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeServiceError(ctx, w, &ErrorMissingAgentID)
		return
	}
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}

	resp, svcErr := h.service.GetAgentRoles(ctx, id, limit, offset)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// parsePaginationParams parses limit and offset query parameters.
func parsePaginationParams(query url.Values) (int, int, *tidcommon.ServiceError) {
	limit := 0
	offset := 0
	if v := query.Get("limit"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed <= 0 || parsed > 100 {
			return 0, 0, &ErrorInvalidLimit
		}
		limit = parsed
	}
	if v := query.Get("offset"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 0 {
			return 0, 0, &ErrorInvalidOffset
		}
		offset = parsed
	}
	return limit, offset, nil
}

// parseFilterParams parses the filter query parameter: one `attribute eq "value"` term, or terms on the kind,
// the owner and the name joined with "and", each at most once.
func parseFilterParams(query url.Values) (map[string]interface{}, *tidcommon.ServiceError) {
	group, err := filter.ParseFilterParam(query)
	if err != nil {
		return nil, &ErrorInvalidFilter
	}
	filters := map[string]interface{}{}
	if group == nil {
		return filters, nil
	}
	for _, clause := range group.Clauses {
		value, ok := clause.Expr.Value.(string)
		if !ok || value == "" || clause.Expr.Operator != tidcommon.OperatorEq ||
			(clause.Connector != "" && clause.Connector != tidcommon.LogicalAnd) {
			return nil, &ErrorInvalidFilter
		}
		attribute := clause.Expr.Attribute
		if _, duplicate := filters[attribute]; duplicate {
			return nil, &ErrorInvalidFilter
		}
		if len(group.Clauses) > 1 && !slices.Contains(combinableFilterAttributes, attribute) {
			return nil, &ErrorInvalidFilter
		}
		filters[attribute] = value
	}
	return filters, nil
}

// HandleOwnedAgentListRequest handles GET /users/me/agents: the agents the caller owns, of the kind the kind query
// parameter names, or of every kind without one.
func (h *agentHandler) HandleOwnedAgentListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	callerID, svcErr := selfServiceCaller(ctx)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}

	kind := providers.AgentKind(r.URL.Query().Get(kindQueryParam))
	if !kind.IsValid() {
		writeServiceError(ctx, w, &ErrorInvalidKind)
		return
	}

	resp, svcErr := h.service.GetOwnedAgents(ctx, callerID, kind, limit, offset)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// HandleOwnedAgentGetRequest handles GET /users/me/agents/{id}: one of the caller's agents, of any kind.
func (h *agentHandler) HandleOwnedAgentGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	callerID, svcErr := selfServiceCaller(ctx)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	resp, svcErr := h.service.GetOwnedAgent(ctx, callerID, r.PathValue("id"), includeDisplay)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// HandleOwnedAgentPutRequest handles PUT /users/me/agents/{id}: the owner changes what they may change on one of
// their personal agents. Fields the body leaves out keep their stored values; any other field is refused.
func (h *agentHandler) HandleOwnedAgentPutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	callerID, svcErr := selfServiceCaller(ctx)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	req, err := sysutils.DecodeJSONBodyStrict[model.UpdateOwnedAgentRequest](r)
	if err != nil {
		writeServiceError(ctx, w, &ErrorInvalidRequestFormat)
		return
	}

	resp, svcErr := h.service.UpdateOwnedAgent(ctx, callerID, r.PathValue("id"),
		providers.AgentKindPersonalAssistant, req)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// HandleOwnedAgentDeleteRequest handles DELETE /users/me/agents/{id}: the owner deletes one of their personal
// agents.
func (h *agentHandler) HandleOwnedAgentDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	callerID, svcErr := selfServiceCaller(ctx)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}

	if svcErr := h.service.DeleteOwnedAgent(ctx, callerID, r.PathValue("id"),
		providers.AgentKindPersonalAssistant); svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
}

// HandleOwnedAgentSuspendRequest handles POST /users/me/agents/{id}/suspend: the owner suspends one of their
// personal agents.
func (h *agentHandler) HandleOwnedAgentSuspendRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	callerID, svcErr := selfServiceCaller(ctx)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}

	if svcErr := h.service.SuspendOwnedAgent(ctx, callerID, r.PathValue("id"),
		providers.AgentKindPersonalAssistant); svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
}

// HandleOwnedAgentUnsuspendRequest handles POST /users/me/agents/{id}/unsuspend: the owner lifts their own
// suspension of one of their personal agents.
func (h *agentHandler) HandleOwnedAgentUnsuspendRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	callerID, svcErr := selfServiceCaller(ctx)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}

	if svcErr := h.service.UnsuspendOwnedAgent(ctx, callerID, r.PathValue("id"),
		providers.AgentKindPersonalAssistant); svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
}

// HandleOwnedAgentsSuspendRequest handles POST /users/me/agents/suspend: the owner suspends all their personal
// agents.
func (h *agentHandler) HandleOwnedAgentsSuspendRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	callerID, svcErr := selfServiceCaller(ctx)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	suspended, svcErr := h.service.SuspendOwnedAgents(ctx, callerID, providers.AgentKindPersonalAssistant)
	if svcErr != nil {
		writeServiceError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, model.SuspendOwnedAgentsResponse{SuspendedCount: suspended})
}

// selfServiceCaller returns the user an owner's request acts for: the subject of its token. A request without
// one is refused, and so is a delegated token, so an agent cannot manage its owner's agents with the token it
// holds for the owner.
func selfServiceCaller(ctx context.Context) (string, *tidcommon.ServiceError) {
	subject := security.GetSubject(ctx)
	if subject == "" {
		return "", &ErrorAuthenticationRequired
	}
	if security.IsDelegated(ctx) {
		return "", &ErrorDelegatedTokenNotAllowed
	}
	return subject, nil
}

// writeServiceError converts a service error into the appropriate HTTP error response.
func writeServiceError(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		switch svcErr.Code {
		case ErrorAgentNotFound.Code:
			statusCode = http.StatusNotFound
		case ErrorAuthenticationRequired.Code:
			statusCode = http.StatusUnauthorized
		case ErrorAttributeConflict.Code,
			ErrorAgentAlreadyExistsWithClientID.Code:
			statusCode = http.StatusConflict
		case ErrorCannotModifyDeclarativeResource.Code, ErrorManagedAgent.Code,
			ErrorDelegatedTokenNotAllowed.Code,
			tidcommon.ErrorUnauthorized.Code,
			sysauthz.ErrorGrantNotPermitted.Code:
			statusCode = http.StatusForbidden
		default:
			statusCode = http.StatusBadRequest
		}
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}
	sysutils.WriteErrorResponse(ctx, w, statusCode, errResp)
}
