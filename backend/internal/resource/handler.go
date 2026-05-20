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

package resource

import (
	"net/http"
	"net/url"
	"strconv"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// resourceHandler handles HTTP requests for resource management.
type resourceHandler struct {
	resourceService ResourceServiceInterface
}

// newResourceHandler creates a new resource handler.
func newResourceHandler(resourceService ResourceServiceInterface) *resourceHandler {
	return &resourceHandler{
		resourceService: resourceService,
	}
}

// Resource Server Handlers

// HandleResourceServerListRequest handles listing resource servers.
func (h *resourceHandler) HandleResourceServerListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	result, svcErr := h.resourceService.GetResourceServerList(ctx, limit, offset)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toResourceServerListResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusOK, response)
}

// HandleResourceServerPostRequest handles creating a resource server.
func (h *resourceHandler) HandleResourceServerPostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, err := sysutils.DecodeJSONBody[CreateResourceServerRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	sanitized := sanitizeCreateResourceServerRequest(req)
	serviceReq := ResourceServer{
		Name:        sanitized.Name,
		Description: sanitized.Description,
		Handle:      sanitized.Handle,
		Identifier:  sanitized.Identifier,
		OUID:        sanitized.OUID,
		Delimiter:   sanitized.Delimiter,
	}

	result, svcErr := h.resourceService.CreateResourceServer(ctx, serviceReq)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toResourceServerResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusCreated, response)
}

// HandleResourceServerGetRequest handles getting a resource server.
func (h *resourceHandler) HandleResourceServerGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	result, svcErr := h.resourceService.GetResourceServer(ctx, id)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toResourceServerResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusOK, response)
}

// HandleResourceServerPutRequest handles updating a resource server.
func (h *resourceHandler) HandleResourceServerPutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	req, err := sysutils.DecodeJSONBody[UpdateResourceServerRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	sanitized := sanitizeUpdateResourceServerRequest(req)
	serviceReq := ResourceServer{
		Name:        sanitized.Name,
		Description: sanitized.Description,
		Handle:      sanitized.Handle,
		Identifier:  sanitized.Identifier,
		OUID:        sanitized.OUID,
	}

	result, svcErr := h.resourceService.UpdateResourceServer(ctx, id, serviceReq)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toResourceServerResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusOK, response)
}

// HandleResourceServerDeleteRequest handles deleting a resource server.
func (h *resourceHandler) HandleResourceServerDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	svcErr := h.resourceService.DeleteResourceServer(ctx, id)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Resource Handlers

// HandleResourceListRequest handles listing resources.
func (h *resourceHandler) HandleResourceListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rsID := r.PathValue("rsId")
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	// Parse parentId parameter (can be empty string for top-level, or UUID for children)
	// If parentId not in query, parentID remains nil (all resources)
	var parentID *string
	if r.URL.Query().Has("parentId") {
		parentParam := r.URL.Query().Get("parentId")
		parentID = &parentParam
	}

	result, svcErr := h.resourceService.GetResourceList(ctx, rsID, parentID, limit, offset)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toResourceListResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusOK, response)
}

// HandleResourcePostRequest handles creating a resource.
func (h *resourceHandler) HandleResourcePostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rsID := r.PathValue("rsId")
	req, err := sysutils.DecodeJSONBody[CreateResourceRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	sanitized := sanitizeCreateResourceRequest(req)
	serviceReq := Resource{
		Name:        sanitized.Name,
		Handle:      sanitized.Handle,
		Description: sanitized.Description,
		Parent:      nil,
	}
	if sanitized.Parent != nil {
		serviceReq.Parent = sanitized.Parent
	}

	result, svcErr := h.resourceService.CreateResource(ctx, rsID, serviceReq)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toResourceResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusCreated, response)
}

// HandleResourceGetRequest handles getting a resource.
func (h *resourceHandler) HandleResourceGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rsID := r.PathValue("rsId")
	id := r.PathValue("id")

	result, svcErr := h.resourceService.GetResource(ctx, rsID, id)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toResourceResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusOK, response)
}

// HandleResourcePutRequest handles updating a resource.
func (h *resourceHandler) HandleResourcePutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rsID := r.PathValue("rsId")
	id := r.PathValue("id")

	req, err := sysutils.DecodeJSONBody[UpdateResourceRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	sanitized := sanitizeUpdateResourceRequest(req)
	serviceReq := Resource{
		Name:        sanitized.Name,
		Description: sanitized.Description,
	}

	result, svcErr := h.resourceService.UpdateResource(ctx, rsID, id, serviceReq)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toResourceResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusOK, response)
}

// HandleResourceDeleteRequest handles deleting a resource.
func (h *resourceHandler) HandleResourceDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rsID := r.PathValue("rsId")
	id := r.PathValue("id")

	svcErr := h.resourceService.DeleteResource(ctx, rsID, id)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Action Handlers (Resource Server Level)

// HandleActionListAtResourceServerRequest handles listing actions at resource server level.
func (h *resourceHandler) HandleActionListAtResourceServerRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rsID := r.PathValue("rsId")
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	result, svcErr := h.resourceService.GetActionList(ctx, rsID, nil, limit, offset)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toActionListResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusOK, response)
}

// HandleActionPostAtResourceServerRequest handles creating an action at resource server level.
func (h *resourceHandler) HandleActionPostAtResourceServerRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rsID := r.PathValue("rsId")
	req, err := sysutils.DecodeJSONBody[CreateActionRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	sanitized := sanitizeCreateActionRequest(req)
	serviceReq := Action{
		Name:        sanitized.Name,
		Handle:      sanitized.Handle,
		Description: sanitized.Description,
	}

	result, svcErr := h.resourceService.CreateAction(ctx, rsID, nil, serviceReq)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toActionResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusCreated, response)
}

// HandleActionGetAtResourceServerRequest handles getting an action at resource server level.
func (h *resourceHandler) HandleActionGetAtResourceServerRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rsID := r.PathValue("rsId")
	id := r.PathValue("id")

	result, svcErr := h.resourceService.GetAction(ctx, rsID, nil, id)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toActionResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusOK, response)
}

// HandleActionPutAtResourceServerRequest handles updating an action at resource server level.
func (h *resourceHandler) HandleActionPutAtResourceServerRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rsID := r.PathValue("rsId")
	id := r.PathValue("id")

	req, err := sysutils.DecodeJSONBody[UpdateActionRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	sanitized := sanitizeUpdateActionRequest(req)
	serviceReq := Action{
		Name:        sanitized.Name,
		Description: sanitized.Description,
	}

	result, svcErr := h.resourceService.UpdateAction(ctx, rsID, nil, id, serviceReq)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toActionResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusOK, response)
}

// HandleActionDeleteAtResourceServerRequest handles deleting an action at resource server level.
func (h *resourceHandler) HandleActionDeleteAtResourceServerRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rsID := r.PathValue("rsId")
	id := r.PathValue("id")

	svcErr := h.resourceService.DeleteAction(ctx, rsID, nil, id)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Action Handlers (Resource Level)

// HandleActionListAtResourceRequest handles listing actions at resource level.
func (h *resourceHandler) HandleActionListAtResourceRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rsID := r.PathValue("rsId")
	resourceID := r.PathValue("resourceId")
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	result, svcErr := h.resourceService.GetActionList(ctx, rsID, &resourceID, limit, offset)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toActionListResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusOK, response)
}

// HandleActionPostAtResourceRequest handles creating an action at resource level.
func (h *resourceHandler) HandleActionPostAtResourceRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rsID := r.PathValue("rsId")
	resourceID := r.PathValue("resourceId")

	req, err := sysutils.DecodeJSONBody[CreateActionRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	sanitized := sanitizeCreateActionRequest(req)
	serviceReq := Action{
		Name:        sanitized.Name,
		Handle:      sanitized.Handle,
		Description: sanitized.Description,
	}

	result, svcErr := h.resourceService.CreateAction(ctx, rsID, &resourceID, serviceReq)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toActionResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusCreated, response)
}

// HandleActionGetAtResourceRequest handles getting an action at resource level.
func (h *resourceHandler) HandleActionGetAtResourceRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rsID := r.PathValue("rsId")
	resourceID := r.PathValue("resourceId")
	id := r.PathValue("id")

	result, svcErr := h.resourceService.GetAction(ctx, rsID, &resourceID, id)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toActionResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusOK, response)
}

// HandleActionPutAtResourceRequest handles updating an action at resource level.
func (h *resourceHandler) HandleActionPutAtResourceRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rsID := r.PathValue("rsId")
	resourceID := r.PathValue("resourceId")
	id := r.PathValue("id")

	req, err := sysutils.DecodeJSONBody[UpdateActionRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	sanitized := sanitizeUpdateActionRequest(req)
	serviceReq := Action{
		Name:        sanitized.Name,
		Description: sanitized.Description,
	}

	result, svcErr := h.resourceService.UpdateAction(ctx, rsID, &resourceID, id, serviceReq)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	response := toActionResponse(result)
	sysutils.WriteSuccessResponse(w, http.StatusOK, response)
}

// HandleActionDeleteAtResourceRequest handles deleting an action at resource level.
func (h *resourceHandler) HandleActionDeleteAtResourceRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rsID := r.PathValue("rsId")
	resourceID := r.PathValue("resourceId")
	id := r.PathValue("id")

	svcErr := h.resourceService.DeleteAction(ctx, rsID, &resourceID, id)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Helper functions

// parsePaginationParams parses 'limit' and 'offset' query parameters.
func parsePaginationParams(query url.Values) (int, int, *serviceerror.ServiceError) {
	limit := serverconst.DefaultPageSize
	offset := 0

	if limitStr := query.Get("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 1 {
			return 0, 0, &ErrorInvalidLimit
		}
		limit = parsedLimit
	}

	if offsetStr := query.Get("offset"); offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err != nil || parsedOffset < 0 {
			return 0, 0, &ErrorInvalidOffset
		}
		offset = parsedOffset
	}

	return limit, offset, nil
}

// handleError writes an error response based on the provided service error.
func handleError(w http.ResponseWriter, svcErr *serviceerror.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == serviceerror.ClientErrorType {
		switch svcErr.Code {
		case ErrorResourceServerNotFound.Code, ErrorResourceNotFound.Code, ErrorActionNotFound.Code:
			statusCode = http.StatusNotFound
		case ErrorNameConflict.Code, ErrorHandleConflict.Code, ErrorIdentifierConflict.Code:
			statusCode = http.StatusConflict
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

// Sanitization functions

// sanitizeCreateResourceServerRequest sanitizes input for creating a resource server.
func sanitizeCreateResourceServerRequest(req *CreateResourceServerRequest) CreateResourceServerRequest {
	return CreateResourceServerRequest{
		Name:        sysutils.SanitizeString(req.Name),
		Description: sysutils.SanitizeString(req.Description),
		Handle:      sysutils.SanitizeString(req.Handle),
		Identifier:  sysutils.SanitizeString(req.Identifier),
		OUID:        sysutils.SanitizeString(req.OUID),
		Delimiter:   sysutils.SanitizeString(req.Delimiter),
	}
}

// sanitizeUpdateResourceServerRequest sanitizes input for updating a resource server.
func sanitizeUpdateResourceServerRequest(req *UpdateResourceServerRequest) UpdateResourceServerRequest {
	return UpdateResourceServerRequest{
		Name:        sysutils.SanitizeString(req.Name),
		Description: sysutils.SanitizeString(req.Description),
		Handle:      sysutils.SanitizeString(req.Handle),
		Identifier:  sysutils.SanitizeString(req.Identifier),
		OUID:        sysutils.SanitizeString(req.OUID),
	}
}

// sanitizeCreateResourceRequest sanitizes input for creating a resource.
func sanitizeCreateResourceRequest(req *CreateResourceRequest) CreateResourceRequest {
	sanitized := CreateResourceRequest{
		Name:        sysutils.SanitizeString(req.Name),
		Handle:      sysutils.SanitizeString(req.Handle),
		Description: sysutils.SanitizeString(req.Description),
		Parent:      nil,
	}

	if req.Parent != nil {
		sanitizedParent := sysutils.SanitizeString(*req.Parent)
		sanitized.Parent = &sanitizedParent
	}

	return sanitized
}

// sanitizeUpdateResourceRequest sanitizes input for updating a resource.
func sanitizeUpdateResourceRequest(req *UpdateResourceRequest) UpdateResourceRequest {
	return UpdateResourceRequest{
		Name:        sysutils.SanitizeString(req.Name),
		Description: sysutils.SanitizeString(req.Description),
	}
}

// sanitizeCreateActionRequest sanitizes input for creating an action.
func sanitizeCreateActionRequest(req *CreateActionRequest) CreateActionRequest {
	return CreateActionRequest{
		Name:        sysutils.SanitizeString(req.Name),
		Handle:      sysutils.SanitizeString(req.Handle),
		Description: sysutils.SanitizeString(req.Description),
	}
}

// sanitizeUpdateActionRequest sanitizes input for updating an action.
func sanitizeUpdateActionRequest(req *UpdateActionRequest) UpdateActionRequest {
	return UpdateActionRequest{
		Name:        sysutils.SanitizeString(req.Name),
		Description: sysutils.SanitizeString(req.Description),
	}
}

// Response transformation functions

// toResourceServerResponse transforms a ResourceServer to ResourceServerResponse.
func toResourceServerResponse(rs *ResourceServer) *ResourceServerResponse {
	return &ResourceServerResponse{
		ID:          rs.ID,
		Name:        rs.Name,
		Description: rs.Description,
		Handle:      rs.Handle,
		Identifier:  rs.Identifier,
		OUID:        rs.OUID,
		Delimiter:   rs.Delimiter,
		IsReadOnly:  rs.IsReadOnly,
	}
}

// toResourceServerListResponse transforms a ResourceServerList to ResourceServerListResponse.
func toResourceServerListResponse(list *ResourceServerList) *ResourceServerListResponse {
	resourceServers := make([]ResourceServerResponse, len(list.ResourceServers))
	for i, rs := range list.ResourceServers {
		resourceServers[i] = *toResourceServerResponse(&rs)
	}

	links := make([]LinkResponse, len(list.Links))
	for i, link := range list.Links {
		links[i] = LinkResponse(link)
	}

	return &ResourceServerListResponse{
		TotalResults:    list.TotalResults,
		StartIndex:      list.StartIndex,
		Count:           list.Count,
		ResourceServers: resourceServers,
		Links:           links,
	}
}

// toResourceResponse transforms a Resource to ResourceResponse.
func toResourceResponse(res *Resource) *ResourceResponse {
	return &ResourceResponse{
		ID:          res.ID,
		Name:        res.Name,
		Handle:      res.Handle,
		Description: res.Description,
		Parent:      res.Parent,
		Permission:  res.Permission,
	}
}

// toResourceListResponse transforms a ResourceList to ResourceListResponse.
func toResourceListResponse(list *ResourceList) *ResourceListResponse {
	resources := make([]ResourceResponse, len(list.Resources))
	for i, res := range list.Resources {
		resources[i] = *toResourceResponse(&res)
	}

	links := make([]LinkResponse, len(list.Links))
	for i, link := range list.Links {
		links[i] = LinkResponse(link)
	}

	return &ResourceListResponse{
		TotalResults: list.TotalResults,
		StartIndex:   list.StartIndex,
		Count:        list.Count,
		Resources:    resources,
		Links:        links,
	}
}

// toActionResponse transforms an Action to ActionResponse.
func toActionResponse(action *Action) *ActionResponse {
	return &ActionResponse{
		ID:          action.ID,
		Name:        action.Name,
		Handle:      action.Handle,
		Description: action.Description,
		Permission:  action.Permission,
	}
}

// toActionListResponse transforms an ActionList to ActionListResponse.
func toActionListResponse(list *ActionList) *ActionListResponse {
	actions := make([]ActionResponse, len(list.Actions))
	for i, action := range list.Actions {
		actions[i] = *toActionResponse(&action)
	}

	links := make([]LinkResponse, len(list.Links))
	for i, link := range list.Links {
		links[i] = LinkResponse(link)
	}

	return &ActionListResponse{
		TotalResults: list.TotalResults,
		StartIndex:   list.StartIndex,
		Count:        list.Count,
		Actions:      actions,
		Links:        links,
	}
}
