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

package role

import (
	"net/http"
	"net/url"
	"strconv"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const handlerLoggerComponentName = "RoleHandler"

// roleHandler is the handler for role management operations.
type roleHandler struct {
	roleService       RoleServiceInterface
	assignmentService RoleAssignmentServiceInterface
}

// newRoleHandler creates a new instance of roleHandler
func newRoleHandler(roleService RoleServiceInterface, assignmentService RoleAssignmentServiceInterface) *roleHandler {
	return &roleHandler{
		roleService:       roleService,
		assignmentService: assignmentService,
	}
}

// HandleRoleListRequest handles the list roles request.
func (rh *roleHandler) HandleRoleListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	roleList, svcErr := rh.roleService.GetRoleList(ctx, limit, offset)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	// Convert service response to HTTP response
	roles := make([]RoleSummaryResponse, 0, len(roleList.Roles))
	for _, role := range roleList.Roles {
		roles = append(roles, RoleSummaryResponse(role))
	}

	roleListResponse := &RoleListResponse{
		TotalResults: roleList.TotalResults,
		StartIndex:   roleList.StartIndex,
		Count:        roleList.Count,
		Roles:        roles,
		Links:        roleList.Links,
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, roleListResponse)

	logger.Debug("Successfully listed roles with pagination",
		log.Int("limit", limit), log.Int("offset", offset),
		log.Int("totalResults", roleListResponse.TotalResults),
		log.Int("count", roleListResponse.Count))
}

// HandleRolePostRequest handles the create role request.
func (rh *roleHandler) HandleRolePostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	createRequest, err := sysutils.DecodeJSONBody[CreateRoleRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	sanitizedRequest := rh.sanitizeCreateRoleRequest(createRequest)

	// Convert HTTP request to service request
	serviceRequest := rh.toRoleCreationDetail(sanitizedRequest)

	serviceRole, svcErr := rh.roleService.CreateRole(ctx, serviceRequest)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	// Convert service response to HTTP response
	createdRole := rh.toHTTPCreateRoleResponse(serviceRole)

	sysutils.WriteSuccessResponse(w, http.StatusCreated, createdRole)

	logger.Debug("Successfully created role", log.String("roleId", createdRole.ID))
}

// HandleRoleGetRequest handles the get role by id request.
func (rh *roleHandler) HandleRoleGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	serviceRole, svcErr := rh.roleService.GetRoleWithPermissions(ctx, id)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	// Convert service response to HTTP response
	role := rh.toHTTPRoleResponse(serviceRole)

	sysutils.WriteSuccessResponse(w, http.StatusOK, role)

	logger.Debug("Successfully retrieved role", log.String("role id", id))
}

// HandleRolePutRequest handles the update role request.
func (rh *roleHandler) HandleRolePutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	updateRequest, err := sysutils.DecodeJSONBody[UpdateRoleRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	sanitizedRequest := rh.sanitizeUpdateRoleRequest(updateRequest)

	// Convert HTTP request to service request
	serviceRequest := RoleUpdateDetail(sanitizedRequest)

	serviceRole, svcErr := rh.roleService.UpdateRoleWithPermissions(ctx, id, serviceRequest)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	// Convert service response to HTTP response
	role := rh.toHTTPRoleResponse(serviceRole)

	sysutils.WriteSuccessResponse(w, http.StatusOK, role)

	logger.Debug("Successfully updated role", log.String("role id", id))
}

// HandleRoleDeleteRequest handles the delete role request.
func (rh *roleHandler) HandleRoleDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	svcErr := rh.roleService.DeleteRole(ctx, id)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusNoContent, nil)
	logger.Debug("Successfully deleted role", log.String("role id", id))
}

// HandleRoleAssignmentsGetRequest handles the get role assignments request.
func (rh *roleHandler) HandleRoleAssignmentsGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	// Parse include parameter to check if display names should be included
	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	// Parse optional type parameter to filter assignments by assignee type.
	assigneeType := r.URL.Query().Get("type")
	if assigneeType != "" && assigneeType != string(AssigneeTypeUser) && assigneeType != string(AssigneeTypeGroup) &&
		assigneeType != string(AssigneeTypeApp) && assigneeType != string(AssigneeTypeAgent) {
		handleError(w, &ErrorInvalidAssigneeType)
		return
	}

	var serviceResponse *AssignmentList
	if assigneeType != "" {
		serviceResponse, svcErr = rh.assignmentService.GetRoleAssignmentsByType(
			ctx, id, limit, offset, includeDisplay, assigneeType)
	} else {
		serviceResponse, svcErr = rh.assignmentService.GetRoleAssignments(ctx, id, limit, offset, includeDisplay)
	}
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	// Convert service response to HTTP response
	httpAssignments := make([]AssignmentResponse, len(serviceResponse.Assignments))
	for i, sa := range serviceResponse.Assignments {
		httpAssignments[i] = AssignmentResponse(sa)
	}

	assignmentListResponse := &AssignmentListResponse{
		TotalResults: serviceResponse.TotalResults,
		StartIndex:   serviceResponse.StartIndex,
		Count:        serviceResponse.Count,
		Assignments:  httpAssignments,
		Links:        serviceResponse.Links,
	}

	sysutils.WriteSuccessResponse(w, http.StatusOK, assignmentListResponse)

	logger.Debug("Successfully retrieved role assignments", log.String("role id", id),
		log.Int("limit", limit), log.Int("offset", offset),
		log.Bool("includeDisplay", includeDisplay),
		log.String("assigneeType", assigneeType),
		log.Int("totalResults", assignmentListResponse.TotalResults),
		log.Int("count", assignmentListResponse.Count))
}

// HandleRoleAddAssignmentsRequest handles the add assignments to role request.
func (rh *roleHandler) HandleRoleAddAssignmentsRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	assignmentsRequest, err := sysutils.DecodeJSONBody[AssignmentsRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	sanitizedRequest := rh.sanitizeAssignmentsRequest(assignmentsRequest)

	// Convert HTTP request to service request
	serviceRequest := rh.toRoleAssignments(sanitizedRequest)

	svcErr := rh.assignmentService.AddAssignments(ctx, id, serviceRequest)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusNoContent, nil)
	logger.Debug("Successfully added assignments to role", log.String("role id", id))
}

// HandleRoleRemoveAssignmentsRequest handles the remove assignments from role request.
func (rh *roleHandler) HandleRoleRemoveAssignmentsRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	assignmentsRequest, err := sysutils.DecodeJSONBody[AssignmentsRequest](r)
	if err != nil {
		handleError(w, &ErrorInvalidRequestFormat)
		return
	}

	sanitizedRequest := rh.sanitizeAssignmentsRequest(assignmentsRequest)

	// Convert HTTP request to service request
	serviceRequest := rh.toRoleAssignments(sanitizedRequest)

	svcErr := rh.assignmentService.RemoveAssignments(ctx, id, serviceRequest)
	if svcErr != nil {
		handleError(w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(w, http.StatusNoContent, nil)
	logger.Debug("Successfully removed assignments from role", log.String("role id", id))
}

// handleError handles service errors and returns appropriate HTTP responses.
func handleError(w http.ResponseWriter,
	svcErr *serviceerror.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == serviceerror.ClientErrorType {
		switch svcErr.Code {
		case ErrorRoleNotFound.Code:
			statusCode = http.StatusNotFound
		case ErrorRoleNameConflict.Code:
			statusCode = http.StatusConflict
		case ErrorOrganizationUnitNotFound.Code,
			ErrorInvalidRequestFormat.Code, ErrorMissingRoleID.Code,
			ErrorInvalidLimit.Code, ErrorInvalidOffset.Code,
			ErrorEmptyAssignments.Code,
			ErrorInvalidAssignmentID.Code:
			statusCode = http.StatusBadRequest
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

// sanitizeCreateRoleRequest sanitizes the create role request input.
func (rh *roleHandler) sanitizeCreateRoleRequest(request *CreateRoleRequest) CreateRoleRequest {
	sanitized := CreateRoleRequest{
		Name:        sysutils.SanitizeString(request.Name),
		Description: sysutils.SanitizeString(request.Description),
		OUID:        sysutils.SanitizeString(request.OUID),
	}

	if request.Permissions != nil {
		sanitized.Permissions = make([]ResourcePermissions, len(request.Permissions))
		for i, resPerm := range request.Permissions {
			sanitizedPerms := make([]string, len(resPerm.Permissions))
			for j, perm := range resPerm.Permissions {
				sanitizedPerms[j] = sysutils.SanitizeString(perm)
			}
			sanitized.Permissions[i] = ResourcePermissions{
				ResourceServerID: sysutils.SanitizeString(resPerm.ResourceServerID),
				Permissions:      sanitizedPerms,
			}
		}
	}

	if request.Assignments != nil {
		sanitized.Assignments = make([]AssignmentRequest, len(request.Assignments))
		for i, assignment := range request.Assignments {
			sanitized.Assignments[i] = AssignmentRequest{
				ID:   sysutils.SanitizeString(assignment.ID),
				Type: assignment.Type,
			}
		}
	}

	return sanitized
}

// sanitizeUpdateRoleRequest sanitizes the update role request input.
func (rh *roleHandler) sanitizeUpdateRoleRequest(request *UpdateRoleRequest) UpdateRoleRequest {
	sanitized := UpdateRoleRequest{
		Name:        sysutils.SanitizeString(request.Name),
		Description: sysutils.SanitizeString(request.Description),
		OUID:        sysutils.SanitizeString(request.OUID),
	}

	if request.Permissions != nil {
		sanitized.Permissions = make([]ResourcePermissions, len(request.Permissions))
		for i, resPerm := range request.Permissions {
			sanitizedPerms := make([]string, len(resPerm.Permissions))
			for j, perm := range resPerm.Permissions {
				sanitizedPerms[j] = sysutils.SanitizeString(perm)
			}
			sanitized.Permissions[i] = ResourcePermissions{
				ResourceServerID: sysutils.SanitizeString(resPerm.ResourceServerID),
				Permissions:      sanitizedPerms,
			}
		}
	}

	return sanitized
}

// sanitizeAssignmentsRequest sanitizes the assignments request input.
func (rh *roleHandler) sanitizeAssignmentsRequest(request *AssignmentsRequest) AssignmentsRequest {
	sanitized := AssignmentsRequest{}

	if request.Assignments != nil {
		sanitized.Assignments = make([]AssignmentRequest, len(request.Assignments))
		for i, assignment := range request.Assignments {
			sanitized.Assignments[i] = AssignmentRequest{
				ID:   sysutils.SanitizeString(assignment.ID),
				Type: assignment.Type,
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

// toRoleCreationDetail converts HTTP CreateRoleRequest to service layer RoleCreationDetail.
func (rh *roleHandler) toRoleCreationDetail(req CreateRoleRequest) RoleCreationDetail {
	serviceAssignments := make([]RoleAssignment, len(req.Assignments))
	for i, a := range req.Assignments {
		serviceAssignments[i] = RoleAssignment(a)
	}

	return RoleCreationDetail{
		Name:        req.Name,
		Description: req.Description,
		OUID:        req.OUID,
		Permissions: req.Permissions,
		Assignments: serviceAssignments,
	}
}

// toHTTPRole converts service layer RoleWithPermissions to HTTP Role.
func (rh *roleHandler) toHTTPRoleResponse(role *RoleWithPermissions) *RoleResponse {
	r := RoleResponse(*role)
	return &r
}

// toHTTPCreateRoleResponse converts service layer RoleDetails to HTTP CreateRoleResponse.
func (rh *roleHandler) toHTTPCreateRoleResponse(role *RoleWithPermissionsAndAssignments) *CreateRoleResponse {
	httpAssignments := make([]AssignmentResponse, len(role.Assignments))
	for i, sa := range role.Assignments {
		httpAssignments[i] = AssignmentResponse{
			ID:   sa.ID,
			Type: sa.Type,
		}
	}

	return &CreateRoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		OUID:        role.OUID,
		OUHandle:    role.OUHandle,
		Permissions: role.Permissions,
		Assignments: httpAssignments,
	}
}

// toRoleAssignments converts HTTP AssignmentsRequest to service layer RoleAssignments.
func (rh *roleHandler) toRoleAssignments(req AssignmentsRequest) []RoleAssignment {
	serviceAssignments := make([]RoleAssignment, len(req.Assignments))
	for i, a := range req.Assignments {
		serviceAssignments[i] = RoleAssignment(a)
	}
	return serviceAssignments
}
