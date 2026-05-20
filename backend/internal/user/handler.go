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

package user

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const handlerLoggerComponentName = "UserHandler"

// userHandler is the handler for user management operations.
type userHandler struct {
	userService UserServiceInterface
}

// newUserHandler creates a new instance of userHandler with dependency injection.
func newUserHandler(userService UserServiceInterface) *userHandler {
	return &userHandler{
		userService: userService,
	}
}

// HandleUserListRequest handles the user list request.
func (uh *userHandler) HandleUserListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	if limit == 0 {
		limit = serverconst.DefaultPageSize
	}

	filters, svcErr := parseFilterParams(r.URL.Query())
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	// Parse include parameter to check if display names should be included.
	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	// Get the user list using the user service.
	userListResponse, svcErr := uh.userService.GetUserList(ctx, limit, offset, filters, includeDisplay)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, userListResponse)

	logger.Debug("Successfully listed users with pagination",
		log.Int("limit", limit), log.Int("offset", offset),
		log.Int("totalResults", userListResponse.TotalResults),
		log.Int("count", userListResponse.Count),
		log.MaskedMap("filters", filters))
}

// HandleUserPostRequest handles the user request.
func (uh *userHandler) HandleUserPostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	createRequest, err := sysutils.DecodeJSONBody[User](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidRequestFormat.Code,
			Message:     ErrorInvalidRequestFormat.Error,
			Description: ErrorInvalidRequestFormat.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	// Create the user using the user service.
	createdUser, svcErr := uh.userService.CreateUser(ctx, createRequest)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusCreated, createdUser)

	// Log the user creation response.
	logger.Debug("User POST response sent", log.MaskedString(log.LoggerKeyUserID, createdUser.ID))
}

// HandleUserGetRequest handles the user request.
func (uh *userHandler) HandleUserGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	if id == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorMissingUserID.Code,
			Message:     ErrorMissingUserID.Error,
			Description: ErrorMissingUserID.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	// Parse include parameter to check if display name should be included.
	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	// Get the user using the user service.
	user, svcErr := uh.userService.GetUser(ctx, id, includeDisplay)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, user)

	// Log the user response.
	logger.Debug("User GET response sent", log.MaskedString(log.LoggerKeyUserID, id))
}

// HandleUserGroupsGetRequest handles the get user groups request.
func (ah *userHandler) HandleUserGroupsGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	id := r.PathValue("id")
	if id == "" {
		handleError(w, &ErrorMissingUserID)
		return
	}

	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	if limit == 0 {
		limit = serverconst.DefaultPageSize
	}

	groupListResponse, svcErr := ah.userService.GetUserGroups(ctx, id, limit, offset)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, groupListResponse)

	logger.Debug("Successfully retrieved user groups", log.MaskedString(log.LoggerKeyUserID, id),
		log.Int("limit", limit), log.Int("offset", offset),
		log.Int("totalResults", groupListResponse.TotalResults),
		log.Int("count", groupListResponse.Count))
}

// HandleUserPutRequest handles the user request.
func (uh *userHandler) HandleUserPutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := strings.TrimPrefix(r.URL.Path, "/users/")
	if id == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorMissingUserID.Code,
			Message:     ErrorMissingUserID.Error,
			Description: ErrorMissingUserID.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	updateRequest, err := sysutils.DecodeJSONBody[User](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:        ErrorInvalidRequestFormat.Code,
			Message:     ErrorInvalidRequestFormat.Error,
			Description: ErrorInvalidRequestFormat.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}
	updateRequest.ID = id

	// Update the user using the user service.
	user, svcErr := uh.userService.UpdateUser(ctx, id, updateRequest)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, user)

	// Log the user response.
	logger.Debug("User PUT response sent", log.MaskedString(log.LoggerKeyUserID, id))
}

// HandleUserDeleteRequest handles the delete user request.
func (uh *userHandler) HandleUserDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := strings.TrimPrefix(r.URL.Path, "/users/")
	if id == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorMissingUserID.Code,
			Message:     ErrorMissingUserID.Error,
			Description: ErrorMissingUserID.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	// Delete the user using the user service.
	svcErr := uh.userService.DeleteUser(ctx, id)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusNoContent, nil)

	// Log the user response.
	logger.Debug("User DELETE response sent", log.MaskedString(log.LoggerKeyUserID, id))
}

// HandleUserListByPathRequest handles the list users by OU path request.
func (uh *userHandler) HandleUserListByPathRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	path, pathValidationFailed := extractAndValidatePath(w, r)
	if pathValidationFailed {
		return
	}

	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	if limit == 0 {
		limit = serverconst.DefaultPageSize
	}

	filters, svcErr := parseFilterParams(r.URL.Query())
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	// Parse include parameter to check if display names should be included.
	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	userListResponse, svcErr := uh.userService.GetUsersByPath(ctx, path, limit, offset, filters, includeDisplay)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, userListResponse)

	logger.Debug("Successfully listed users by path", log.String("path", path),
		log.Int("limit", limit), log.Int("offset", offset),
		log.Int("totalResults", userListResponse.TotalResults),
		log.Int("count", userListResponse.Count),
		log.MaskedMap("filters", filters))
}

// HandleUserPostByPathRequest handles the create user by OU path request.
func (uh *userHandler) HandleUserPostByPathRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	path, pathValidationFailed := extractAndValidatePath(w, r)
	if pathValidationFailed {
		return
	}

	createRequest, err := sysutils.DecodeJSONBody[CreateUserByPathRequest](r)
	if err != nil {
		errResp := apierror.ErrorResponse{
			Code:    ErrorInvalidRequestFormat.Code,
			Message: ErrorInvalidRequestFormat.Error,
			Description: core.I18nMessage{
				Key:          "error.userservice.invalid_request_format_description",
				DefaultValue: "Failed to parse request body: " + err.Error(),
			},
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return
	}

	user, svcErr := uh.userService.CreateUserByPath(ctx, path, *createRequest)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusCreated, user)

	logger.Debug("Successfully created user by path", log.String("path", path), log.String("userType", user.Type))
}

// HandleSelfUserGetRequest handles the self user retrieval.
func (uh *userHandler) HandleSelfUserGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	userID := security.GetSubject(ctx)
	if strings.TrimSpace(userID) == "" {
		handleError(w, &ErrorAuthenticationFailed)
		return
	}

	// Parse include parameter to check if display name should be included.
	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	user, svcErr := uh.userService.GetUser(ctx, userID, includeDisplay)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, user)

	logger.Debug("Self user GET response sent", log.MaskedString(log.LoggerKeyUserID, userID))
}

// HandleSelfUserPutRequest handles the self user update.
func (uh *userHandler) HandleSelfUserPutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	userID := security.GetSubject(ctx)
	if strings.TrimSpace(userID) == "" {
		handleError(w, &ErrorAuthenticationFailed)
		return
	}

	updateRequest, err := sysutils.DecodeJSONBody[UpdateSelfUserRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	if updateRequest == nil || len(updateRequest.Attributes) == 0 {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	updatedUser, svcErr := uh.userService.UpdateUserAttributes(ctx, userID, updateRequest.Attributes)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, updatedUser)

	logger.Debug("Self user PUT response sent", log.MaskedString(log.LoggerKeyUserID, userID))
}

// HandleSelfUserCredentialUpdateRequest handles the credential update for the authenticated user.
func (uh *userHandler) HandleSelfUserCredentialUpdateRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	userID := security.GetSubject(ctx)
	if strings.TrimSpace(userID) == "" {
		handleError(w, &ErrorAuthenticationFailed)
		return
	}

	updateRequest, err := sysutils.DecodeJSONBody[UpdateSelfUserRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	if updateRequest == nil || len(updateRequest.Attributes) == 0 || string(updateRequest.Attributes) == "{}" {
		handleError(w, &ErrorMissingCredentials)
		return
	}

	if svcErr := uh.userService.UpdateUserCredentials(ctx, userID, updateRequest.Attributes); svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusNoContent, nil)
	logger.Debug("Self user credential update response sent", log.MaskedString(log.LoggerKeyUserID, userID))
}

// parsePaginationParams parses limit and offset query parameters from the request.
func parsePaginationParams(query url.Values) (int, int, *serviceerror.ServiceError) {
	limit := 0
	offset := 0

	if limitStr := query.Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err != nil {
			return 0, 0, &ErrorInvalidLimit
		} else if parsedLimit <= 0 {
			return 0, 0, &ErrorInvalidLimit
		} else {
			limit = parsedLimit
		}
	}

	if offsetStr := query.Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err != nil {
			return 0, 0, &ErrorInvalidOffset
		} else if parsedOffset < 0 {
			return 0, 0, &ErrorInvalidOffset
		} else {
			offset = parsedOffset
		}
	}

	return limit, offset, nil
}

// handleError handles service errors and writes appropriate HTTP responses.
func handleError(w http.ResponseWriter, svcErr *serviceerror.ServiceError) {
	var statusCode int
	if svcErr.Type == serviceerror.ClientErrorType {
		switch svcErr.Code {
		case ErrorMissingUserID.Code,
			ErrorUserNotFound.Code,
			ErrorOrganizationUnitNotFound.Code:
			statusCode = http.StatusNotFound
		case ErrorAttributeConflict.Code:
			statusCode = http.StatusConflict
		case ErrorHandlePathRequired.Code,
			ErrorInvalidHandlePath.Code,
			ErrorMissingRequiredFields.Code,
			ErrorMissingCredentials.Code,
			ErrorEntityTypeNotFound.Code:
			statusCode = http.StatusBadRequest
		case ErrorAuthenticationFailed.Code:
			statusCode = http.StatusUnauthorized
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

// extractAndValidatePath extracts and validates the path parameter from the request.
func extractAndValidatePath(w http.ResponseWriter, r *http.Request) (string, bool) {
	path := r.PathValue("path")
	if path == "" {
		errResp := apierror.ErrorResponse{
			Code:        ErrorHandlePathRequired.Code,
			Message:     ErrorHandlePathRequired.Error,
			Description: ErrorHandlePathRequired.ErrorDescription,
		}
		sysutils.WriteErrorResponse(w, http.StatusBadRequest, errResp)
		return "", true
	}
	return path, false
}

// parseFilterParams parses and sanitizes filter query parameters from the request.
func parseFilterParams(query url.Values) (map[string]interface{}, *serviceerror.ServiceError) {
	if !query.Has("filter") {
		return make(map[string]interface{}), nil
	}

	filterStr := query.Get("filter")
	filterStr = strings.TrimSpace(filterStr)
	if filterStr == "" {
		return nil, &ErrorInvalidFilter
	}

	parsedFilter, err := parseFilterExpression(filterStr)
	if err != nil {
		return nil, &ErrorInvalidFilter
	}

	sanitized := sanitizeFilter(parsedFilter)

	return sanitized, nil
}

// parseFilterExpression parses filter expressions in the format: attribute eq "value"
func parseFilterExpression(filterStr string) (map[string]interface{}, error) {
	// Regex to match: attribute_name eq "value" or attribute_name eq value
	pattern := `^(\w+(?:\.\w+)*)\s+(eq)\s+(?:"([^"]*)"|(\w+|\d+))$`
	regex := regexp.MustCompile(pattern)

	matches := regex.FindStringSubmatch(filterStr)
	if len(matches) == 0 {
		return nil, fmt.Errorf("invalid filter format")
	}

	attribute := matches[1]
	operator := matches[2]

	if operator != "eq" {
		return nil, fmt.Errorf("unsupported operator: %s", operator)
	}

	// Get the value (either quoted string or unquoted value)
	if matches[3] != "" {
		return map[string]interface{}{attribute: matches[3]}, nil
	} else {
		value := matches[4] // Unquoted value
		// Try to convert numeric values
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return map[string]interface{}{attribute: intVal}, nil
		}
		// If not an integer, try to parse as float
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return map[string]interface{}{attribute: floatVal}, nil
		}

		// Check for boolean values
		if bool, err := strconv.ParseBool(value); err == nil {
			return map[string]interface{}{attribute: bool}, nil
		}

		return nil, fmt.Errorf("invalid filter value")
	}
}

// sanitizeFilter performs additional sanitization on parsed filters
func sanitizeFilter(filters map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})

	for key, value := range filters {
		sanitizedKey := sysutils.SanitizeString(key)

		if strValue, ok := value.(string); ok {
			sanitized[sanitizedKey] = sysutils.SanitizeString(strValue)
		} else {
			sanitized[sanitizedKey] = value
		}
	}

	return sanitized
}
