// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/sharing"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
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
		handleError(ctx, w, svcErr)
		return
	}

	roleList, svcErr := rh.roleService.GetRoleList(ctx, limit, offset)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
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

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, roleListResponse)

	logger.Debug(ctx, "Successfully listed roles with pagination",
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
		var valErr *sysutils.ValidationError
		if errors.As(err, &valErr) {
			sysutils.WriteStructuredErrorResponse(w, http.StatusBadRequest, "Validation Failed", valErr.Errors)
			return
		}
		handleError(ctx, w, &ErrorInvalidRequestFormat)
		return
	}

	sanitizedRequest := rh.sanitizeCreateRoleRequest(createRequest)

	// Convert HTTP request to service request
	serviceRequest := rh.toRoleCreationDetail(sanitizedRequest)

	serviceRole, svcErr := rh.roleService.CreateRole(ctx, serviceRequest)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	// Convert service response to HTTP response
	createdRole := rh.toHTTPCreateRoleResponse(serviceRole)

	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, createdRole)

	logger.Debug(ctx, "Successfully created role", log.String("roleId", createdRole.ID))
}

// HandleRoleGetRequest handles the get role by id request.
func (rh *roleHandler) HandleRoleGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	serviceRole, svcErr := rh.roleService.GetRoleWithPermissions(ctx, id)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	// Convert service response to HTTP response
	role := rh.toHTTPRoleResponse(serviceRole)

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, role)

	logger.Debug(ctx, "Successfully retrieved role", log.String("role id", id))
}

// HandleRolePutRequest handles the update role request.
func (rh *roleHandler) HandleRolePutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	updateRequest, err := sysutils.DecodeJSONBody[UpdateRoleRequest](r)
	if err != nil {
		var valErr *sysutils.ValidationError
		if errors.As(err, &valErr) {
			sysutils.WriteStructuredErrorResponse(w, http.StatusBadRequest, "Validation Failed", valErr.Errors)
			return
		}
		handleError(ctx, w, &ErrorInvalidRequestFormat)
		return
	}

	sanitizedRequest := rh.sanitizeUpdateRoleRequest(updateRequest)

	// Convert HTTP request to service request
	serviceRequest := RoleUpdateDetail(sanitizedRequest)

	serviceRole, svcErr := rh.roleService.UpdateRoleWithPermissions(ctx, id, serviceRequest)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	// Convert service response to HTTP response
	role := rh.toHTTPRoleResponse(serviceRole)

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, role)

	logger.Debug(ctx, "Successfully updated role", log.String("role id", id))
}

// HandleRoleDeleteRequest handles the delete role request.
func (rh *roleHandler) HandleRoleDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	svcErr := rh.roleService.DeleteRole(ctx, id)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
	logger.Debug(ctx, "Successfully deleted role", log.String("role id", id))
}

// HandleRoleAssignmentsGetRequest handles the get role assignments request.
func (rh *roleHandler) HandleRoleAssignmentsGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	// Parse include parameter to check if display names should be included
	includeDisplay := r.URL.Query().Get(sysutils.QueryParamInclude) == sysutils.IncludeValueDisplay

	// Parse optional type parameter to filter assignments by assignee type.
	assigneeType := r.URL.Query().Get("type")
	if assigneeType != "" && assigneeType != string(AssigneeTypeUser) && assigneeType != string(AssigneeTypeGroup) &&
		assigneeType != string(AssigneeTypeApp) && assigneeType != string(AssigneeTypeAgent) {
		handleError(ctx, w, &ErrorInvalidAssigneeType)
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
		handleError(ctx, w, svcErr)
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

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, assignmentListResponse)

	logger.Debug(ctx, "Successfully retrieved role assignments", log.String("role id", id),
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
		handleError(ctx, w, &ErrorInvalidRequestFormat)
		return
	}

	sanitizedRequest := rh.sanitizeAssignmentsRequest(assignmentsRequest)

	// Convert HTTP request to service request
	serviceRequest := rh.toRoleAssignments(sanitizedRequest)

	svcErr := rh.assignmentService.AddAssignments(ctx, id, serviceRequest)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
	logger.Debug(ctx, "Successfully added assignments to role", log.String("role id", id))
}

// HandleRoleRemoveAssignmentsRequest handles the remove assignments from role request.
func (rh *roleHandler) HandleRoleRemoveAssignmentsRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	id := r.PathValue("id")
	assignmentsRequest, err := sysutils.DecodeJSONBody[AssignmentsRequest](r)
	if err != nil {
		handleError(ctx, w, &ErrorInvalidRequestFormat)
		return
	}

	sanitizedRequest := rh.sanitizeAssignmentsRequest(assignmentsRequest)

	// Convert HTTP request to service request
	serviceRequest := rh.toRoleAssignments(sanitizedRequest)

	svcErr := rh.assignmentService.RemoveAssignments(ctx, id, serviceRequest)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusNoContent, nil)
	logger.Debug(ctx, "Successfully removed assignments from role", log.String("role id", id))
}

// handleError handles service errors and returns appropriate HTTP responses.
func handleError(ctx context.Context, w http.ResponseWriter,
	svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		switch svcErr.Code {
		case ErrorRoleNotFound.Code, ErrorSharingPolicyNotFound.Code, ErrorRoleNotSharedToOU.Code:
			statusCode = http.StatusNotFound
		case ErrorRoleNameConflict.Code, ErrorSharingPolicyExists.Code, ErrorSharingPolicyVersionMismatch.Code:
			statusCode = http.StatusConflict
		case ErrorOrganizationUnitNotFound.Code,
			ErrorInvalidRequestFormat.Code, ErrorMissingRoleID.Code,
			ErrorInvalidLimit.Code, ErrorInvalidOffset.Code,
			ErrorEmptyAssignments.Code,
			ErrorInvalidAssignmentID.Code,
			ErrorSharedRoleCannotMove.Code:
			statusCode = http.StatusBadRequest
		case tidcommon.ErrorUnauthorized.Code, sysauthz.ErrorGrantNotPermitted.Code:
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
func parsePaginationParams(query url.Values) (int, int, *tidcommon.ServiceError) {
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
	permissions := role.Permissions
	if permissions == nil {
		permissions = make([]ResourcePermissions, 0)
	}

	return &CreateRoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		OUID:        role.OUID,
		OUHandle:    role.OUHandle,
		Permissions: permissions,
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

// HandleSharingPolicyPostRequest records a sharing policy on a role.
func (rh *roleHandler) HandleSharingPolicyPostRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	wire, ok := sharing.DecodePolicyBody[sharing.PolicyCreateBody](ctx, w, r, ErrorInvalidRequestFormat)
	if !ok {
		return
	}
	req := wire.ToPolicyRequest()
	// A reach not bounded by where the issuer sits is a deployment decision, so this API leaves it
	// to a resource file or an import. An edit may still restate such a section to move its
	// exclusions, and the framework refuses any edit that would introduce one.
	if req.IsDeploymentWide() {
		handleError(ctx, w, &ErrorDeploymentWideSharingNotAllowed)
		return
	}

	policy, svcErr := rh.roleService.CreateSharingPolicy(ctx, r.PathValue("id"), req)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, sharing.ToPolicyResponse(policy))
}

// HandleSharingPolicyListRequest returns one page of a role's sharing policies.
func (rh *roleHandler) HandleSharingPolicyListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit, offset, svcErr := parsePaginationParams(r.URL.Query())
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	roleID := r.PathValue("id")
	list, svcErr := rh.roleService.ListSharingPolicies(ctx, roleID, limit, offset)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, sharing.ToPolicyListResponse(
		list, "/roles/"+roleID+"/sharing-policies", limit, offset))
}

// HandleSharingPolicyGetRequest returns one of a role's sharing policies.
func (rh *roleHandler) HandleSharingPolicyGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	policy, svcErr := rh.roleService.GetSharingPolicy(ctx, r.PathValue("id"), r.PathValue("policyId"))
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, sharing.ToPolicyResponse(policy))
}

// HandleSharingPolicyPutRequest replaces a sharing policy's contents.
func (rh *roleHandler) HandleSharingPolicyPutRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	wire, ok := sharing.DecodePolicyBody[sharing.PolicyUpdateBody](ctx, w, r, ErrorInvalidRequestFormat)
	if !ok {
		return
	}

	policy, svcErr := rh.roleService.UpdateSharingPolicy(
		ctx, r.PathValue("id"), r.PathValue("policyId"), wire.ToPolicyRequest())
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, sharing.ToPolicyResponse(policy))
}

// HandleSharingPolicyDeleteRequest removes a sharing policy.
func (rh *roleHandler) HandleSharingPolicyDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if svcErr := rh.roleService.DeleteSharingPolicy(
		ctx, r.PathValue("id"), r.PathValue("policyId")); svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleOverlayRuleGetRequest answers whether the organization unit named in ouId owns the role or
// is shared it, and which policies reached it.
func (rh *roleHandler) HandleOverlayRuleGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	overlay, svcErr := rh.roleService.ResolveSharingOverlay(ctx, r.PathValue("id"), r.URL.Query().Get("ouId"))
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, sharing.ToResolvedOverlayResponse(overlay))
}
