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

package agent

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
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
		writeServiceError(w, svcErr)
		return
	}

	filters, svcErr := parseFilterParams(r.URL.Query())
	if svcErr != nil {
		writeServiceError(w, svcErr)
		return
	}

	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	resp, svcErr := h.service.GetAgentList(ctx, limit, offset, filters, includeDisplay)
	if svcErr != nil {
		writeServiceError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, resp)
	logger.Debug("Agent list returned",
		log.Int("limit", limit), log.Int("offset", offset),
		log.Int("totalResults", resp.TotalResults), log.Int("count", resp.Count))
}

// HandleAgentPostRequest handles POST /agents.
func (h *agentHandler) HandleAgentPostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	req, err := sysutils.DecodeJSONBody[model.CreateAgentRequest](r)
	if err != nil {
		writeServiceError(w, &ErrorInvalidRequestFormat)
		return
	}

	resp, svcErr := h.service.CreateAgent(ctx, req)
	if svcErr != nil {
		writeServiceError(w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(w, http.StatusCreated, resp)
}

// HandleAgentGetRequest handles GET /agents/{id}.
func (h *agentHandler) HandleAgentGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeServiceError(w, &ErrorMissingAgentID)
		return
	}
	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	resp, svcErr := h.service.GetAgent(ctx, id, includeDisplay)
	if svcErr != nil {
		writeServiceError(w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(w, http.StatusOK, resp)
}

// HandleAgentPutRequest handles PUT /agents/{id}.
func (h *agentHandler) HandleAgentPutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeServiceError(w, &ErrorMissingAgentID)
		return
	}

	req, err := sysutils.DecodeJSONBody[model.UpdateAgentRequest](r)
	if err != nil {
		writeServiceError(w, &ErrorInvalidRequestFormat)
		return
	}

	resp, svcErr := h.service.UpdateAgent(ctx, id, req)
	if svcErr != nil {
		writeServiceError(w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(w, http.StatusOK, resp)
}

// HandleAgentDeleteRequest handles DELETE /agents/{id}.
func (h *agentHandler) HandleAgentDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeServiceError(w, &ErrorMissingAgentID)
		return
	}
	if svcErr := h.service.DeleteAgent(ctx, id); svcErr != nil {
		writeServiceError(w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(w, http.StatusNoContent, nil)
}

// HandleAgentGroupsRequest handles GET /agents/{id}/groups.
func (h *agentHandler) HandleAgentGroupsRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeServiceError(w, &ErrorMissingAgentID)
		return
	}
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		writeServiceError(w, svcErr)
		return
	}

	resp, svcErr := h.service.GetAgentGroups(ctx, id, limit, offset)
	if svcErr != nil {
		writeServiceError(w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(w, http.StatusOK, resp)
}

// parsePaginationParams parses limit and offset query parameters.
func parsePaginationParams(query url.Values) (int, int, *serviceerror.ServiceError) {
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

// parseFilterParams parses the filter query parameter using the same simple eq syntax used
// across other resources (attribute eq "value").
func parseFilterParams(query url.Values) (map[string]interface{}, *serviceerror.ServiceError) {
	if !query.Has("filter") {
		return map[string]interface{}{}, nil
	}
	raw := strings.TrimSpace(query.Get("filter"))
	if raw == "" {
		return nil, &ErrorInvalidFilter
	}
	parts := strings.SplitN(raw, " eq ", 2)
	if len(parts) != 2 {
		return nil, &ErrorInvalidFilter
	}
	attr := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])
	if len(val) < 2 || !strings.HasPrefix(val, "\"") || !strings.HasSuffix(val, "\"") {
		return nil, &ErrorInvalidFilter
	}
	val = val[1 : len(val)-1]
	if attr == "" || val == "" {
		return nil, &ErrorInvalidFilter
	}
	return map[string]interface{}{attr: val}, nil
}

// writeServiceError converts a service error into the appropriate HTTP error response.
func writeServiceError(w http.ResponseWriter, svcErr *serviceerror.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == serviceerror.ClientErrorType {
		switch svcErr.Code {
		case ErrorAgentNotFound.Code:
			statusCode = http.StatusNotFound
		case ErrorAgentAlreadyExistsWithName.Code,
			ErrorAttributeConflict.Code,
			ErrorAgentAlreadyExistsWithClientID.Code:
			statusCode = http.StatusConflict
		case ErrorCannotModifyDeclarativeResource.Code:
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
	sysutils.WriteErrorResponse(w, statusCode, errResp)
}
