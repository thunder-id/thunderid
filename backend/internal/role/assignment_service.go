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

package role

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const assignmentLoggerComponentName = "RoleAssignmentService"

// RoleAssignmentServiceInterface defines the interface for role assignment operations.
type RoleAssignmentServiceInterface interface {
	GetRoleAssignments(ctx context.Context, id string, limit, offset int,
		includeDisplay bool) (*AssignmentList, *serviceerror.ServiceError)
	GetRoleAssignmentsByType(ctx context.Context, id string, limit, offset int,
		includeDisplay bool, assigneeType string) (*AssignmentList, *serviceerror.ServiceError)
	AddAssignments(ctx context.Context, id string, assignments []RoleAssignment) *serviceerror.ServiceError
	RemoveAssignments(ctx context.Context, id string, assignments []RoleAssignment) *serviceerror.ServiceError
}

// roleAssignmentService is the default implementation of RoleAssignmentServiceInterface.
type roleAssignmentService struct {
	roleStore         roleStoreInterface
	entityService     entity.EntityServiceInterface
	groupService      group.GroupServiceInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	transactioner     transaction.Transactioner
}

// newRoleAssignmentService creates a new instance of roleAssignmentService.
func newRoleAssignmentService(
	roleStore roleStoreInterface,
	entityService entity.EntityServiceInterface,
	groupService group.GroupServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	transactioner transaction.Transactioner,
) RoleAssignmentServiceInterface {
	return &roleAssignmentService{
		roleStore:         roleStore,
		entityService:     entityService,
		groupService:      groupService,
		entityTypeService: entityTypeService,
		transactioner:     transactioner,
	}
}

// GetRoleAssignments retrieves assignments for a role with pagination.
func (as *roleAssignmentService) GetRoleAssignments(ctx context.Context, id string, limit, offset int,
	includeDisplay bool) (*AssignmentList, *serviceerror.ServiceError) {
	return as.GetRoleAssignmentsByType(ctx, id, limit, offset, includeDisplay, "")
}

// GetRoleAssignmentsByType retrieves assignments for a role filtered by assignee type with pagination.
func (as *roleAssignmentService) GetRoleAssignmentsByType(ctx context.Context, id string, limit, offset int,
	includeDisplay bool, assigneeType string) (*AssignmentList, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, assignmentLoggerComponentName))

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	if id == "" {
		return nil, &ErrorMissingRoleID
	}

	exists, err := as.roleStore.IsRoleExist(ctx, id)
	if err != nil {
		logger.Error("Failed to check role existence", log.String("id", id), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if !exists {
		logger.Debug("Role not found", log.String("id", id))
		return nil, &ErrorRoleNotFound
	}

	// user/app/agent filters require fetching all entity assignments and post-filtering by category.
	if assigneeType == string(entity.EntityCategoryUser) ||
		assigneeType == string(entity.EntityCategoryApp) ||
		assigneeType == string(entity.EntityCategoryAgent) {
		return as.getAssignmentsByEntityCategory(ctx, id, limit, offset, includeDisplay, assigneeType, logger)
	}

	// For no filter or 'group' filter, use DB-level pagination directly.
	var totalCount int
	var assignments []RoleAssignment
	if assigneeType != "" {
		totalCount, err = as.roleStore.GetRoleAssignmentsCountByType(ctx, id, assigneeType)
	} else {
		totalCount, err = as.roleStore.GetRoleAssignmentsCount(ctx, id)
	}
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ResultLimitExceededInCompositeMode
		}
		logger.Error("Failed to get role assignments count", log.String("id", id), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	if assigneeType != "" {
		assignments, err = as.roleStore.GetRoleAssignmentsByType(ctx, id, limit, offset, assigneeType)
	} else {
		assignments, err = as.roleStore.GetRoleAssignments(ctx, id, limit, offset)
	}
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ResultLimitExceededInCompositeMode
		}
		logger.Error("Failed to get role assignments", log.String("id", id), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	serviceAssignments, svcErr := as.resolveAssignments(ctx, assignments, includeDisplay)
	if svcErr != nil {
		return nil, svcErr
	}

	baseURL := fmt.Sprintf("/roles/%s/assignments", id)
	extraQuery := utils.DisplayQueryParam(includeDisplay)
	if assigneeType != "" {
		extraQuery += "&type=" + assigneeType
	}
	links := utils.BuildPaginationLinks(baseURL, limit, offset, totalCount, extraQuery)

	return &AssignmentList{
		TotalResults: totalCount,
		Assignments:  serviceAssignments,
		StartIndex:   offset + 1,
		Count:        len(serviceAssignments),
		Links:        links,
	}, nil
}

// getAssignmentsByEntityCategory handles ?type=user and ?type=app filter cases.
// Since both are stored as 'entity' internally, it fetches all entity assignments,
// resolves their category, and paginates the filtered results in memory.
func (as *roleAssignmentService) getAssignmentsByEntityCategory(
	ctx context.Context, id string, limit, offset int,
	includeDisplay bool, category string, logger *log.Logger,
) (*AssignmentList, *serviceerror.ServiceError) {
	totalEntityCount, err := as.roleStore.GetRoleAssignmentsCountByType(ctx, id, string(assigneeTypeEntity))
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ResultLimitExceededInCompositeMode
		}
		logger.Error("Failed to get entity assignments count", log.String("id", id), log.Error(err))
		return nil, &ErrorInternalServerError
	}

	var allEntityAssignments []RoleAssignment
	if totalEntityCount > 0 {
		allEntityAssignments, err = as.roleStore.GetRoleAssignmentsByType(
			ctx, id, totalEntityCount, 0, string(assigneeTypeEntity))
		if err != nil {
			if errors.Is(err, errResultLimitExceededInCompositeMode) {
				return nil, &ResultLimitExceededInCompositeMode
			}
			logger.Error("Failed to get entity assignments", log.String("id", id), log.Error(err))
			return nil, &ErrorInternalServerError
		}
	}

	// Batch-resolve entity categories.
	entityCategoryMap := make(map[string]string)
	if len(allEntityAssignments) > 0 {
		entityIDs := make([]string, len(allEntityAssignments))
		for i, a := range allEntityAssignments {
			entityIDs[i] = a.ID
		}
		entities, fetchErr := as.entityService.GetEntitiesByIDs(ctx, entityIDs)
		if fetchErr != nil {
			logger.Error("Failed to batch fetch entities for category filter", log.Error(fetchErr))
			return nil, &ErrorInternalServerError
		}
		for _, e := range entities {
			entityCategoryMap[e.ID] = string(e.Category)
		}
	}

	// Filter to matching category and paginate in memory.
	var filtered []RoleAssignment
	for _, a := range allEntityAssignments {
		if entityCategoryMap[a.ID] == category {
			filtered = append(filtered, a)
		}
	}

	totalCount := len(filtered)
	start := offset
	if start > totalCount {
		start = totalCount
	}
	end := start + limit
	if end > totalCount {
		end = totalCount
	}
	page := filtered[start:end]

	serviceAssignments, svcErr := as.resolveAssignments(ctx, page, includeDisplay)
	if svcErr != nil {
		return nil, svcErr
	}

	baseURL := fmt.Sprintf("/roles/%s/assignments", id)
	extraQuery := utils.DisplayQueryParam(includeDisplay) + "&type=" + category
	links := utils.BuildPaginationLinks(baseURL, limit, offset, totalCount, extraQuery)

	return &AssignmentList{
		TotalResults: totalCount,
		Assignments:  serviceAssignments,
		StartIndex:   offset + 1,
		Count:        len(serviceAssignments),
		Links:        links,
	}, nil
}

// AddAssignments adds assignments to a role.
// Assignments can be added to both mutable (DB-backed) and declarative (file-backed) roles.
func (as *roleAssignmentService) AddAssignments(
	ctx context.Context, id string, assignments []RoleAssignment) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, assignmentLoggerComponentName))
	logger.Debug("Adding assignments to role", log.String("id", id))

	normalized, svcErr := as.prepareAssignments(ctx, id, assignments)
	if svcErr != nil {
		return svcErr
	}

	if err := as.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return as.roleStore.AddAssignments(txCtx, id, normalized)
	}); err != nil {
		logger.Error("Failed to add assignments to role", log.String("id", id), log.Error(err))
		return &serviceerror.InternalServerError
	}

	logger.Debug("Successfully added assignments to role", log.String("id", id))
	return nil
}

// RemoveAssignments removes assignments from a role.
// Assignments can be removed from both mutable (DB-backed) and declarative (file-backed) roles.
func (as *roleAssignmentService) RemoveAssignments(
	ctx context.Context, id string, assignments []RoleAssignment) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, assignmentLoggerComponentName))
	logger.Debug("Removing assignments from role", log.String("id", id))

	normalized, svcErr := as.prepareAssignments(ctx, id, assignments)
	if svcErr != nil {
		return svcErr
	}

	if err := as.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return as.roleStore.RemoveAssignments(txCtx, id, normalized)
	}); err != nil {
		logger.Error("Failed to remove assignments from role", log.String("id", id), log.Error(err))
		return &serviceerror.InternalServerError
	}

	logger.Debug("Successfully removed assignments from role", log.String("id", id))
	return nil
}

// prepareAssignments validates and normalizes assignments before a mutation.
// Unlike the previous role service implementation, this allows modifying assignments for
// both mutable and declarative (file-backed) roles.
func (as *roleAssignmentService) prepareAssignments(
	ctx context.Context, id string, assignments []RoleAssignment,
) ([]RoleAssignment, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, assignmentLoggerComponentName))

	if id == "" {
		return nil, &ErrorMissingRoleID
	}

	if err := as.validateAssignmentsRequest(assignments); err != nil {
		return nil, err
	}

	exists, err := as.roleStore.IsRoleExist(ctx, id)
	if err != nil {
		logger.Error("Failed to check role existence", log.String("id", id), log.Error(err))
		return nil, &ErrorInternalServerError
	}
	if !exists {
		logger.Debug("Role not found", log.String("id", id))
		return nil, &ErrorRoleNotFound
	}

	if err := as.validateAssignmentIDs(ctx, assignments); err != nil {
		return nil, err
	}

	normalized := normalizeAssignments(assignments)

	return normalized, nil
}

// validateAssignmentsRequest validates the assignments request.
// Accepts public types 'user', 'app', 'agent', 'group'.
func (as *roleAssignmentService) validateAssignmentsRequest(
	assignments []RoleAssignment) *serviceerror.ServiceError {
	if len(assignments) == 0 {
		return &ErrorEmptyAssignments
	}

	for _, assignment := range assignments {
		if !assignment.Type.IsEntityType() && assignment.Type != AssigneeTypeGroup {
			return &ErrorInvalidAssigneeType
		}
		if assignment.ID == "" {
			return &ErrorInvalidRequestFormat
		}
	}

	return nil
}

// validateAssignmentIDs validates assignment IDs before normalization.
func (as *roleAssignmentService) validateAssignmentIDs(
	ctx context.Context, assignments []RoleAssignment) *serviceerror.ServiceError {
	return validateAssignmentIDs(ctx, assignments, as.entityService, as.groupService, assignmentLoggerComponentName)
}

// validateAssignmentIDs validates assignment IDs checking entity/group existence and type matching.
func validateAssignmentIDs(
	ctx context.Context,
	assignments []RoleAssignment,
	entitySvc entity.EntityServiceInterface,
	groupSvc group.GroupServiceInterface,
	loggerComponent string,
) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponent))

	typeByID := make(map[string]AssigneeType)
	var groupIDs []string

	for _, a := range assignments {
		if a.Type.IsEntityType() {
			if existing, ok := typeByID[a.ID]; ok && existing != a.Type {
				return &ErrorInvalidAssignmentID
			}
			typeByID[a.ID] = a.Type
		} else if a.Type == AssigneeTypeGroup {
			groupIDs = append(groupIDs, a.ID)
		}
	}

	groupIDs = utils.UniqueStrings(groupIDs)

	if len(typeByID) > 0 {
		entityIDs := make([]string, 0, len(typeByID))
		for id := range typeByID {
			entityIDs = append(entityIDs, id)
		}

		entities, err := entitySvc.GetEntitiesByIDs(ctx, entityIDs)
		if err != nil {
			logger.Error("Failed to fetch entities for assignment validation", log.Error(err))
			return &ErrorInternalServerError
		}

		if len(entities) != len(entityIDs) {
			return &ErrorInvalidAssignmentID
		}

		for _, e := range entities {
			claimed := typeByID[e.ID]
			actual := AssigneeType(e.Category)
			if claimed != actual {
				logger.Debug("Assignment type mismatch", log.String("id", e.ID),
					log.String("claimed", string(claimed)), log.String("actual", string(actual)))
				return &ErrorInvalidAssignmentID
			}
		}
	}

	if len(groupIDs) > 0 {
		if err := groupSvc.ValidateGroupIDs(ctx, groupIDs); err != nil {
			if err.Code == group.ErrorInvalidGroupMemberID.Code {
				logger.Debug("Invalid group member IDs found")
				return &ErrorInvalidAssignmentID
			}
			logger.Error("Failed to validate group IDs", log.String("error", err.Error.DefaultValue))
			return &serviceerror.InternalServerError
		}
	}

	return nil
}

// resolveAssignments resolves the public types and optionally display names for role assignments.
func (as *roleAssignmentService) resolveAssignments(
	ctx context.Context,
	assignments []RoleAssignment,
	includeDisplay bool,
) ([]RoleAssignmentWithDisplay, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, assignmentLoggerComponentName))

	var entityIDs, groupIDs []string
	for _, a := range assignments {
		switch a.Type {
		case assigneeTypeEntity:
			entityIDs = append(entityIDs, a.ID)
		case AssigneeTypeGroup:
			groupIDs = append(groupIDs, a.ID)
		}
	}

	// Always batch-fetch entities to resolve their category (user vs app) for the API response type.
	var entityMap map[string]*entity.Entity
	if len(entityIDs) > 0 {
		entities, err := as.entityService.GetEntitiesByIDs(ctx, entityIDs)
		if err != nil {
			logger.Error("Failed to batch fetch entities for assignments", log.Error(err))
			return nil, &ErrorInternalServerError
		}
		entityMap = make(map[string]*entity.Entity, len(entities))
		for i := range entities {
			entityMap[entities[i].ID] = &entities[i]
		}
	}

	var groupsMap map[string]*group.Group
	if includeDisplay && len(groupIDs) > 0 {
		var svcErr *serviceerror.ServiceError
		groupsMap, svcErr = as.groupService.GetGroupsByIDs(ctx, groupIDs)
		if svcErr != nil {
			logger.Warn("Failed to batch fetch groups for display names", log.Any("error", svcErr))
		}
	}

	// Resolve display attribute paths for user-category entities.
	var displayAttrPaths map[string]string
	if includeDisplay && entityMap != nil {
		var userTypes []string
		for _, e := range entityMap {
			if e.Category == entity.EntityCategoryUser {
				userTypes = append(userTypes, e.Type)
			}
		}
		displayAttrPaths = resolveDisplayAttributePaths(ctx, userTypes, as.entityTypeService, logger)
	}

	// Build the result slice, skipping orphaned entity assignments.
	result := make([]RoleAssignmentWithDisplay, 0, len(assignments))
	for _, a := range assignments {
		ra := RoleAssignmentWithDisplay{ID: a.ID}
		switch a.Type {
		case assigneeTypeEntity:
			e, ok := entityMap[a.ID]
			if !ok {
				logger.Warn("Skipping orphaned entity assignment", log.String("id", a.ID))
				continue
			}
			ra.Type = AssigneeType(e.Category)
			if includeDisplay {
				if e.Category == entity.EntityCategoryUser {
					ra.Display = utils.ResolveDisplay(e.ID, e.Type, e.Attributes, displayAttrPaths)
				} else {
					ra.Display = resolveAppDisplay(*e)
				}
			}
		case AssigneeTypeGroup:
			ra.Type = AssigneeTypeGroup
			if includeDisplay {
				if groupsMap != nil {
					if g, ok := groupsMap[a.ID]; ok {
						ra.Display = g.Name
					} else {
						ra.Display = a.ID
					}
				} else {
					ra.Display = a.ID
				}
			}
		default:
			ra.Type = a.Type
			ra.Display = a.ID
		}
		result = append(result, ra)
	}
	return result, nil
}

// resolveAppDisplay extracts a display name for an app entity from its system attributes.
func resolveAppDisplay(e entity.Entity) string {
	if len(e.SystemAttributes) > 0 {
		var sysAttrs map[string]interface{}
		if err := json.Unmarshal(e.SystemAttributes, &sysAttrs); err == nil {
			if name, ok := sysAttrs["name"].(string); ok && name != "" {
				return name
			}
		}
	}
	return e.ID
}

// resolveDisplayAttributePaths collects unique user types and resolves their display
// attribute paths from the entity type service.
func resolveDisplayAttributePaths(
	ctx context.Context, userTypes []string, schemaService entitytype.EntityTypeServiceInterface,
	logger *log.Logger,
) map[string]string {
	if schemaService == nil || len(userTypes) == 0 {
		return nil
	}

	uniqueTypes := utils.UniqueNonEmptyStrings(userTypes)
	if len(uniqueTypes) == 0 {
		return nil
	}

	displayPaths, svcErr := schemaService.GetDisplayAttributesByNames(ctx, entitytype.TypeCategoryUser, uniqueTypes)
	if svcErr != nil {
		if logger != nil {
			logger.Warn("Failed to resolve display attribute paths, skipping display resolution",
				log.Any("error", svcErr))
		}
		return nil
	}

	return displayPaths
}

// normalizeAssignments converts public 'user'/'app'/'agent' types to the internal 'entity' type.
func normalizeAssignments(assignments []RoleAssignment) []RoleAssignment {
	normalized := make([]RoleAssignment, len(assignments))
	for i, a := range assignments {
		t := a.Type
		if t.IsEntityType() {
			t = assigneeTypeEntity
		}
		normalized[i] = RoleAssignment{ID: a.ID, Type: t}
	}
	return normalized
}
