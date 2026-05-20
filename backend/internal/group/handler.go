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

package group

import (
	"net/http"
	"net/url"
	"strconv"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const handlerLoggerComponentName = "GroupHandler"

// groupHandler is the handler for group management operations.
type groupHandler struct {
	groupService GroupServiceInterface
}

// newGroupHandler creates a new instance of groupHandler
func newGroupHandler(groupService GroupServiceInterface) *groupHandler {
	return &groupHandler{
		groupService: groupService,
	}
}

// HandleGroupListRequest handles the list groups request.
func (gh *groupHandler) HandleGroupListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		gh.handleError(w, svcErr)
		return
	}

	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	groupListResponse, svcErr := gh.groupService.GetGroupList(ctx, limit, offset, includeDisplay)
	if svcErr != nil {
		gh.handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, groupListResponse)

	logger.Debug("Successfully listed groups with pagination",
		log.Int("limit", limit), log.Int("offset", offset),
		log.Int("totalResults", groupListResponse.TotalResults),
		log.Int("count", groupListResponse.Count))
}

// HandleGroupListByPathRequest handles the list groups by OU path request.
func (gh *groupHandler) HandleGroupListByPathRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	path, pathValidationFailed := extractAndValidatePath(w, r)
	if pathValidationFailed {
		return
	}

	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		gh.handleError(w, svcErr)
		return
	}

	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	groupListResponse, svcErr := gh.groupService.GetGroupsByPath(ctx, path, limit, offset, includeDisplay)
	if svcErr != nil {
		gh.handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, groupListResponse)

	logger.Debug("Successfully listed groups by path", log.String("path", path),
		log.Int("limit", limit), log.Int("offset", offset),
		log.Int("totalResults", groupListResponse.TotalResults),
		log.Int("count", groupListResponse.Count))
}

// HandleGroupPostRequest handles the create group request.
func (gh *groupHandler) HandleGroupPostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	createRequest, err := sysutils.DecodeJSONBody[CreateGroupRequest](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:    ErrorInvalidRequestFormat.Code,
			Message: ErrorInvalidRequestFormat.Error,
			Description: core.I18nMessage{
				Key:          "error.groupservice.create_group_request_parse_failed_description",
				DefaultValue: "Failed to parse request body: " + err.Error()},
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	sanitizedRequest := gh.sanitizeCreateGroupRequest(createRequest)
	createdGroup, svcErr := gh.groupService.CreateGroup(ctx, sanitizedRequest)
	if svcErr != nil {
		gh.handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusCreated, createdGroup)

	logger.Debug("Successfully created group", log.String("group id", createdGroup.ID))
}

// HandleGroupPostByPathRequest handles the create group by OU path request.
func (gh *groupHandler) HandleGroupPostByPathRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	path, pathValidationFailed := extractAndValidatePath(w, r)
	if pathValidationFailed {
		return
	}

	createRequest, err := sysutils.DecodeJSONBody[CreateGroupByPathRequest](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:    ErrorInvalidRequestFormat.Code,
			Message: ErrorInvalidRequestFormat.Error,
			Description: core.I18nMessage{
				Key:          "error.groupservice.create_group_by_path_request_parse_failed_description",
				DefaultValue: "Failed to parse request body: " + err.Error()},
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	group, svcErr := gh.groupService.CreateGroupByPath(ctx, path, *createRequest)
	if svcErr != nil {
		gh.handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusCreated, group)

	logger.Debug("Successfully created group by path", log.String("path", path), log.String("groupName", group.Name))
}

// HandleGroupGetRequest handles the get group by id request.
func (gh *groupHandler) HandleGroupGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	if id == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorMissingGroupID.Code,
			Message:     ErrorMissingGroupID.Error,
			Description: ErrorMissingGroupID.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	group, svcErr := gh.groupService.GetGroup(ctx, id, includeDisplay)
	if svcErr != nil {
		gh.handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, group)

	logger.Debug("Successfully retrieved group", log.String("group id", id))
}

// HandleGroupPutRequest handles the update group request.
func (gh *groupHandler) HandleGroupPutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	if id == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorMissingGroupID.Code,
			Message:     ErrorMissingGroupID.Error,
			Description: ErrorMissingGroupID.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	updateRequest, err := sysutils.DecodeJSONBody[UpdateGroupRequest](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:    ErrorInvalidRequestFormat.Code,
			Message: ErrorInvalidRequestFormat.Error,
			Description: core.I18nMessage{
				Key:          "error.groupservice.update_group_request_parse_failed_description",
				DefaultValue: "Failed to parse request body: " + err.Error(),
			},
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	sanitizedRequest := gh.sanitizeUpdateGroupRequest(updateRequest)
	group, svcErr := gh.groupService.UpdateGroup(ctx, id, sanitizedRequest)
	if svcErr != nil {
		gh.handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, group)

	logger.Debug("Successfully updated group", log.String("group id", id))
}

// HandleGroupDeleteRequest handles the delete group request.
func (gh *groupHandler) HandleGroupDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	if id == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorMissingGroupID.Code,
			Message:     ErrorMissingGroupID.Error,
			Description: ErrorMissingGroupID.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	svcErr := gh.groupService.DeleteGroup(ctx, id)
	if svcErr != nil {
		gh.handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusNoContent, nil)
	logger.Debug("Successfully deleted group", log.String("group id", id))
}

// HandleGroupMembersGetRequest handles the get group members request.
func (gh *groupHandler) HandleGroupMembersGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	if id == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorMissingGroupID.Code,
			Message:     ErrorMissingGroupID.Error,
			Description: ErrorMissingGroupID.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		gh.handleError(w, svcErr)
		return
	}

	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	memberListResponse, svcErr := gh.groupService.GetGroupMembers(ctx, id, limit, offset, includeDisplay)
	if svcErr != nil {
		gh.handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, memberListResponse)

	logger.Debug("Successfully retrieved group members", log.String("group id", id),
		log.Int("limit", limit), log.Int("offset", offset),
		log.Int("totalResults", memberListResponse.TotalResults),
		log.Int("count", memberListResponse.Count))
}

// HandleGroupMembersAddRequest handles the add members to group request.
func (gh *groupHandler) HandleGroupMembersAddRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	if id == "" {
		gh.handleError(w, &ErrorMissingGroupID)
		return
	}

	membersRequest, err := sysutils.DecodeJSONBody[MembersRequest](r)
	if err != nil {
		gh.handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	sanitizedRequest := gh.sanitizeMembersRequest(membersRequest)

	group, svcErr := gh.groupService.AddGroupMembers(ctx, id, sanitizedRequest.Members)
	if svcErr != nil {
		gh.handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, group)
	logger.Debug("Successfully added members to group", log.String("group id", id))
}

// HandleGroupMembersRemoveRequest handles the remove members from group request.
func (gh *groupHandler) HandleGroupMembersRemoveRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	if id == "" {
		gh.handleError(w, &ErrorMissingGroupID)
		return
	}

	membersRequest, err := sysutils.DecodeJSONBody[MembersRequest](r)
	if err != nil {
		gh.handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	sanitizedRequest := gh.sanitizeMembersRequest(membersRequest)

	group, svcErr := gh.groupService.RemoveGroupMembers(ctx, id, sanitizedRequest.Members)
	if svcErr != nil {
		gh.handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, group)
	logger.Debug("Successfully removed members from group", log.String("group id", id))
}

// handleError handles service errors and returns appropriate HTTP responses.
func (gh *groupHandler) handleError(w http.ResponseWriter, svcErr *serviceerror.ServiceError) {
	var statusCode int
	if svcErr.Type == serviceerror.ClientErrorType {
		switch svcErr.Code {
		case ErrorGroupNotFound.Code:
			statusCode = http.StatusNotFound
		case ErrorGroupNameConflict.Code:
			statusCode = http.StatusConflict
		case ErrorInvalidOUID.Code, ErrorCannotDeleteGroup.Code,
			ErrorInvalidRequestFormat.Code, ErrorMissingGroupID.Code,
			ErrorInvalidLimit.Code, ErrorInvalidOffset.Code,
			ErrorEmptyMembers.Code, ErrorInvalidMemberType.Code,
			ErrorInvalidMemberID.Code, ErrorInvalidGroupMemberID.Code:
			statusCode = http.StatusBadRequest
		case serviceerror.ErrorUnauthorized.Code:
			statusCode = http.StatusForbidden
		default:
			statusCode = http.StatusBadRequest
		}
	} else {
		statusCode = http.StatusInternalServerError
	}

	errResp := apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	}
	sysutils.WriteErrorResponse(w, statusCode, errResp)
}

// sanitizeCreateGroupRequest sanitizes the create group request input.
func (gh *groupHandler) sanitizeCreateGroupRequest(request *CreateGroupRequest) CreateGroupRequest {
	sanitized := CreateGroupRequest{
		Name:        sysutils.SanitizeString(request.Name),
		Description: sysutils.SanitizeString(request.Description),
		OUID:        sysutils.SanitizeString(request.OUID),
	}

	if request.Members != nil {
		sanitized.Members = make([]Member, len(request.Members))
		for i, member := range request.Members {
			sanitized.Members[i] = Member{
				ID:   sysutils.SanitizeString(member.ID),
				Type: member.Type,
			}
		}
	}

	return sanitized
}

// sanitizeUpdateGroupRequest sanitizes the update group request input.
func (gh *groupHandler) sanitizeUpdateGroupRequest(request *UpdateGroupRequest) UpdateGroupRequest {
	return UpdateGroupRequest{
		Name:        sysutils.SanitizeString(request.Name),
		Description: sysutils.SanitizeString(request.Description),
		OUID:        sysutils.SanitizeString(request.OUID),
	}
}

// sanitizeMembersRequest sanitizes the members request input.
func (gh *groupHandler) sanitizeMembersRequest(request *MembersRequest) MembersRequest {
	sanitized := MembersRequest{}
	if request.Members != nil {
		sanitized.Members = make([]Member, len(request.Members))
		for i, member := range request.Members {
			sanitized.Members[i] = Member{
				ID:   sysutils.SanitizeString(member.ID),
				Type: member.Type,
			}
		}
	}
	return sanitized
}

// parsePaginationParams parses limit and offset query parameters from the request.
func parsePaginationParams(query url.Values) (int, int, *serviceerror.ServiceError) {
	limit := 0
	offset := 0

	if limitStr := query.Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err != nil {
			return 0, 0, &ErrorInvalidLimit
		} else {
			limit = parsedLimit
		}
	}

	if offsetStr := query.Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err != nil {
			return 0, 0, &ErrorInvalidOffset
		} else {
			offset = parsedOffset
		}
	}

	if limit == 0 {
		limit = serverconst.DefaultPageSize
	}

	return limit, offset, nil
}

// extractAndValidatePath extracts and validates the path parameter from the request.
func extractAndValidatePath(w http.ResponseWriter, r *http.Request) (string, bool) {
	path := r.PathValue("path")
	if path == "" {
		errResp := apierror.ErrorResponse{
			Code:    ErrorInvalidRequestFormat.Code,
			Message: ErrorInvalidRequestFormat.Error,
			Description: core.I18nMessage{
				Key:          "error.groupservice.handle_path_required_description",
				DefaultValue: "Handle path is required",
			},
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return "", true
	}
	return path, false
}
