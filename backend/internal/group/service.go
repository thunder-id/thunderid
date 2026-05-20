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

// Package group provides group management functionality.
package group

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const loggerComponentName = "GroupMgtService"

// GroupServiceInterface defines the interface for the group service.
type GroupServiceInterface interface {
	GetGroupList(ctx context.Context, limit, offset int,
		includeDisplay bool) (*GroupListResponse, *serviceerror.ServiceError)
	GetGroupsByPath(ctx context.Context, handlePath string, limit, offset int, includeDisplay bool) (
		*GroupListResponse, *serviceerror.ServiceError)
	CreateGroup(ctx context.Context, request CreateGroupRequest) (*Group, *serviceerror.ServiceError)
	CreateGroupByPath(ctx context.Context, handlePath string, request CreateGroupByPathRequest) (
		*Group, *serviceerror.ServiceError)
	GetGroup(ctx context.Context, groupID string, includeDisplay bool) (*Group, *serviceerror.ServiceError)
	UpdateGroup(ctx context.Context, groupID string, request UpdateGroupRequest) (
		*Group, *serviceerror.ServiceError)
	DeleteGroup(ctx context.Context, groupID string) *serviceerror.ServiceError
	GetGroupMembers(ctx context.Context, groupID string, limit, offset int, includeDisplay bool) (
		*MemberListResponse, *serviceerror.ServiceError)
	ValidateGroupIDs(ctx context.Context, groupIDs []string) *serviceerror.ServiceError
	GetGroupsByIDs(ctx context.Context, groupIDs []string) (map[string]*Group, *serviceerror.ServiceError)
	AddGroupMembers(ctx context.Context, groupID string, members []Member) (*Group, *serviceerror.ServiceError)
	RemoveGroupMembers(ctx context.Context, groupID string, members []Member) (*Group, *serviceerror.ServiceError)
}

// groupService is the default implementation of the GroupServiceInterface.
type groupService struct {
	groupStore        groupStoreInterface
	ouService         oupkg.OrganizationUnitServiceInterface
	entityService     entity.EntityServiceInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	transactioner     transaction.Transactioner
	authzService      sysauthz.SystemAuthorizationServiceInterface
}

// newGroupServiceWithStore creates a new instance of GroupService with an externally provided store.
func newGroupServiceWithStore(
	store groupStoreInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	entityService entity.EntityServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface,
	transactioner transaction.Transactioner,
) GroupServiceInterface {
	return &groupService{
		groupStore:        store,
		ouService:         ouService,
		entityService:     entityService,
		entityTypeService: entityTypeService,
		authzService:      authzService,
		transactioner:     transactioner,
	}
}

// GetGroupList retrieves a list of groups. limit should be a positive integer & offset should be non-negative
// integer
func (gs *groupService) GetGroupList(ctx context.Context, limit, offset int, includeDisplay bool) (
	*GroupListResponse, *serviceerror.ServiceError) {
	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	accessibleOUs, svcErr := gs.getAccessibleOUs(ctx, security.ActionListGroups)
	if svcErr != nil {
		return nil, svcErr
	}

	if accessibleOUs.AllAllowed {
		return gs.listAllGroups(ctx, limit, offset, includeDisplay)
	}

	return gs.listGroupsByOUIDs(ctx, accessibleOUs.IDs, limit, offset, includeDisplay)
}

func (gs *groupService) listAllGroups(ctx context.Context, limit, offset int, includeDisplay bool) (
	*GroupListResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	totalCount, err := gs.groupStore.GetGroupListCount(ctx)
	if err != nil {
		logger.Error("Failed to get group count", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	groups, err := gs.groupStore.GetGroupList(ctx, limit, offset)
	if err != nil {
		logger.Error("Failed to list groups", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	groupBasics := make([]GroupBasic, 0, len(groups))
	for _, groupDAO := range groups {
		groupBasics = append(groupBasics, buildGroupBasic(groupDAO))
	}

	if includeDisplay {
		gs.populateGroupOUHandles(ctx, groupBasics, logger)
	}

	displayQuery := utils.DisplayQueryParam(includeDisplay)
	response := &GroupListResponse{
		TotalResults: totalCount,
		Groups:       groupBasics,
		StartIndex:   offset + 1,
		Count:        len(groupBasics),
		Links:        utils.BuildPaginationLinks("/groups", limit, offset, totalCount, displayQuery),
	}

	return response, nil
}

func (gs *groupService) listGroupsByOUIDs(ctx context.Context, ouIDs []string, limit, offset int,
	includeDisplay bool) (*GroupListResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	displayQuery := utils.DisplayQueryParam(includeDisplay)

	if len(ouIDs) == 0 {
		return &GroupListResponse{
			TotalResults: 0,
			Groups:       []GroupBasic{},
			StartIndex:   offset + 1,
			Count:        0,
			Links:        []utils.Link{},
		}, nil
	}

	totalCount, err := gs.groupStore.GetGroupListCountByOUIDs(ctx, ouIDs)
	if err != nil {
		logger.Error("Failed to get group count by OU IDs", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	if totalCount == 0 {
		return &GroupListResponse{
			TotalResults: 0,
			Groups:       []GroupBasic{},
			StartIndex:   offset + 1,
			Count:        0,
			Links:        []utils.Link{},
		}, nil
	}

	groups, err := gs.groupStore.GetGroupListByOUIDs(ctx, ouIDs, limit, offset)
	if err != nil {
		logger.Error("Failed to list groups by OU IDs", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	groupBasics := make([]GroupBasic, 0, len(groups))
	for _, groupDAO := range groups {
		groupBasics = append(groupBasics, buildGroupBasic(groupDAO))
	}

	if includeDisplay {
		gs.populateGroupOUHandles(ctx, groupBasics, logger)
	}

	response := &GroupListResponse{
		TotalResults: totalCount,
		Groups:       groupBasics,
		StartIndex:   offset + 1,
		Count:        len(groupBasics),
		Links:        utils.BuildPaginationLinks("/groups", limit, offset, totalCount, displayQuery),
	}

	return response, nil
}

// GetGroupsByPath retrieves a list of groups by hierarchical handle path.
func (gs *groupService) GetGroupsByPath(
	ctx context.Context, handlePath string, limit, offset int, includeDisplay bool,
) (*GroupListResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Getting groups by path", log.String("path", handlePath))

	serviceError := gs.validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return nil, serviceError
	}

	ou, svcErr := gs.ouService.GetOrganizationUnitByPath(ctx, handlePath)
	if svcErr != nil {
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			return nil, &ErrorGroupNotFound
		}
		return nil, svcErr
	}
	oUID := ou.ID

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	if err := gs.checkGroupAccess(ctx, security.ActionListGroups, oUID, ""); err != nil {
		return nil, err
	}

	totalCount, err := gs.groupStore.GetGroupsByOrganizationUnitCount(ctx, oUID)
	if err != nil {
		logger.Error("Failed to get group count by organization unit", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	groups, err := gs.groupStore.GetGroupsByOrganizationUnit(ctx, oUID, limit, offset)
	if err != nil {
		logger.Error("Failed to list groups by organization unit", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	groupBasics := make([]GroupBasic, 0, len(groups))
	for _, groupDAO := range groups {
		g := buildGroupBasic(groupDAO)
		if includeDisplay {
			g.OUHandle = ou.Handle
		}
		groupBasics = append(groupBasics, g)
	}

	displayQuery := utils.DisplayQueryParam(includeDisplay)
	response := &GroupListResponse{
		TotalResults: totalCount,
		Groups:       groupBasics,
		StartIndex:   offset + 1,
		Count:        len(groupBasics),
		Links:        utils.BuildPaginationLinks("/groups/tree/"+handlePath, limit, offset, totalCount, displayQuery),
	}

	return response, nil
}

// CreateGroup creates a new group.
func (gs *groupService) CreateGroup(ctx context.Context, request CreateGroupRequest) (
	*Group, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Creating group", log.String("name", request.Name))

	if err := gs.validateCreateGroupRequest(request); err != nil {
		return nil, err
	}

	if err := gs.validateOU(ctx, request.OUID); err != nil {
		return nil, err
	}

	if err := gs.checkGroupAccess(ctx, security.ActionCreateGroup, request.OUID, ""); err != nil {
		return nil, err
	}

	if err := gs.validateEntityMembers(ctx, request.Members, security.ActionCreateGroup); err != nil {
		return nil, err
	}

	var groupIDs []string
	for _, m := range request.Members {
		if m.Type == MemberTypeGroup {
			groupIDs = append(groupIDs, m.ID)
		}
	}

	if len(groupIDs) > 0 {
		if err := gs.ValidateGroupIDs(ctx, groupIDs); err != nil {
			return nil, err
		}
	}

	request.Members = normalizeMembers(request.Members)

	var createdGroup *Group
	var capturedSvcErr *serviceerror.ServiceError

	err := gs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := gs.groupStore.CheckGroupNameConflictForCreate(
			txCtx, request.Name, request.OUID); err != nil {
			if errors.Is(err, ErrGroupNameConflict) {
				logger.Debug("Group name conflict detected", log.String("name", request.Name))
				capturedSvcErr = &ErrorGroupNameConflict
				return errors.New("rollback for group name conflict")
			}
			return err
		}

		groupDaoID := request.ID
		if groupDaoID == "" {
			var genErr error
			groupDaoID, genErr = utils.GenerateUUIDv7()
			if genErr != nil {
				return genErr
			}
		}

		groupDAO := GroupDAO{
			ID:          groupDaoID,
			Name:        request.Name,
			Description: request.Description,
			OUID:        request.OUID,
			Members:     request.Members,
		}

		if err := gs.groupStore.CreateGroup(txCtx, groupDAO); err != nil {
			return err
		}

		group := convertGroupDAOToGroup(groupDAO)
		createdGroup = &group
		return nil
	})

	if capturedSvcErr != nil {
		return nil, capturedSvcErr
	}

	if err != nil {
		logger.Error("Failed to create group", log.Error(err), log.String("name", request.Name))
		return nil, &serviceerror.InternalServerError
	}

	// Resolve member types (entity → user/app) for the API response.
	resolvedMembers, svcErr := gs.resolveMembers(ctx, createdGroup.Members, false, logger)
	if svcErr != nil {
		return nil, svcErr
	}
	createdGroup.Members = resolvedMembers

	logger.Debug("Successfully created group", log.String("id", createdGroup.ID), log.String("name", createdGroup.Name))
	return createdGroup, nil
}

// CreateGroupByPath creates a new group under the organization unit specified by the handle path.
func (gs *groupService) CreateGroupByPath(
	ctx context.Context, handlePath string, request CreateGroupByPathRequest,
) (*Group, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Creating group by path", log.String("path", handlePath), log.String("name", request.Name))

	serviceError := gs.validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return nil, serviceError
	}

	ou, svcErr := gs.ouService.GetOrganizationUnitByPath(ctx, handlePath)
	if svcErr != nil {
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			return nil, &ErrorGroupNotFound
		}
		return nil, svcErr
	}

	// Convert CreateGroupByPathRequest to CreateGroupRequest
	createRequest := CreateGroupRequest{
		Name:        request.Name,
		Description: request.Description,
		OUID:        ou.ID,
		Members:     request.Members,
	}

	return gs.CreateGroup(ctx, createRequest)
}

// GetGroup retrieves a specific group by its id.
func (gs *groupService) GetGroup(
	ctx context.Context, groupID string, includeDisplay bool,
) (*Group, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Retrieving group", log.String("id", groupID))

	if groupID == "" {
		return nil, &ErrorMissingGroupID
	}

	groupDAO, err := gs.groupStore.GetGroup(ctx, groupID)
	if err != nil {
		if errors.Is(err, ErrGroupNotFound) {
			logger.Debug("Group not found", log.String("id", groupID))
			return nil, &ErrorGroupNotFound
		}
		logger.Error("Failed to retrieve group", log.String("id", groupID), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	if err := gs.checkGroupAccess(ctx, security.ActionReadGroup, groupDAO.OUID, groupID); err != nil {
		return nil, err
	}

	group := convertGroupDAOToGroup(groupDAO)

	resolvedMembers, svcErr := gs.resolveMembers(ctx, group.Members, includeDisplay, logger)
	if svcErr != nil {
		return nil, svcErr
	}
	group.Members = resolvedMembers

	if includeDisplay {
		handleMap, svcErr := gs.ouService.GetOrganizationUnitHandlesByIDs(
			ctx, []string{group.OUID})
		if svcErr != nil {
			logger.Warn("Failed to resolve OU handle for group, skipping",
				log.String("id", groupID), log.Any("error", svcErr))
		} else if handle, ok := handleMap[group.OUID]; ok {
			group.OUHandle = handle
		}
	}

	logger.Debug("Successfully retrieved group", log.String("id", group.ID), log.String("name", group.Name))
	return &group, nil
}

// UpdateGroup updates an existing group.
func (gs *groupService) UpdateGroup(
	ctx context.Context, groupID string, request UpdateGroupRequest) (*Group, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Updating group", log.String("id", groupID), log.String("name", request.Name))

	if groupID == "" {
		return nil, &ErrorMissingGroupID
	}

	if err := gs.validateUpdateGroupRequest(request); err != nil {
		return nil, err
	}

	var updatedGroup *Group
	var capturedSvcErr *serviceerror.ServiceError

	err := gs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		existingGroupDAO, err := gs.groupStore.GetGroup(txCtx, groupID)
		if err != nil {
			if errors.Is(err, ErrGroupNotFound) {
				logger.Debug("Group not found", log.String("id", groupID))
				capturedSvcErr = &ErrorGroupNotFound
				return errors.New("rollback for group not found")
			}
			return err
		}

		existingGroup := convertGroupDAOToGroup(existingGroupDAO)
		updateOUID := existingGroupDAO.OUID

		if gs.isOrganizationUnitChanged(existingGroup, request) {
			if err := gs.validateOU(txCtx, request.OUID); err != nil {
				capturedSvcErr = err
				return errors.New("rollback for invalid OU")
			}
			updateOUID = request.OUID
		}

		if err := gs.checkGroupAccess(
			txCtx,
			security.ActionUpdateGroup,
			existingGroupDAO.OUID,
			groupID,
		); err != nil {
			capturedSvcErr = err
			return errors.New("rollback for unauthorized access")
		}

		if updateOUID != existingGroupDAO.OUID {
			if err := gs.checkGroupAccess(
				txCtx,
				security.ActionUpdateGroup,
				updateOUID,
				groupID,
			); err != nil {
				capturedSvcErr = err
				return errors.New("rollback for unauthorized access to target OU")
			}
		}

		if existingGroup.Name != request.Name || existingGroup.OUID != request.OUID {
			err := gs.groupStore.CheckGroupNameConflictForUpdate(
				txCtx, request.Name, request.OUID, groupID)
			if err != nil {
				if errors.Is(err, ErrGroupNameConflict) {
					logger.Debug("Group name conflict detected during update", log.String("name", request.Name))
					capturedSvcErr = &ErrorGroupNameConflict
					return errors.New("rollback for group name conflict")
				}
				return err
			}
		}

		updatedGroupDAO := GroupDAO{
			ID:          existingGroup.ID,
			Name:        request.Name,
			Description: request.Description,
			OUID:        updateOUID,
		}

		if err := gs.groupStore.UpdateGroup(txCtx, updatedGroupDAO); err != nil {
			return err
		}

		group := convertGroupDAOToGroup(updatedGroupDAO)
		updatedGroup = &group
		return nil
	})

	if capturedSvcErr != nil {
		return nil, capturedSvcErr
	}

	if err != nil {
		logger.Error("Failed to update group", log.Error(err), log.String("groupID", groupID))
		return nil, &serviceerror.InternalServerError
	}

	logger.Debug("Successfully updated group", log.String("id", groupID), log.String("name", request.Name))
	return updatedGroup, nil
}

// DeleteGroup delete the specified group by its id.
func (gs *groupService) DeleteGroup(ctx context.Context, groupID string) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Deleting group", log.String("id", groupID))

	if groupID == "" {
		return &ErrorMissingGroupID
	}

	var capturedSvcErr *serviceerror.ServiceError

	err := gs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		existingGroupDAO, err := gs.groupStore.GetGroup(txCtx, groupID)
		if err != nil {
			if errors.Is(err, ErrGroupNotFound) {
				logger.Debug("Group not found", log.String("id", groupID))
				capturedSvcErr = &ErrorGroupNotFound
				return errors.New("rollback for group not found")
			}
			return err
		}

		if err := gs.checkGroupAccess(
			txCtx,
			security.ActionDeleteGroup,
			existingGroupDAO.OUID,
			groupID,
		); err != nil {
			capturedSvcErr = err
			return errors.New("rollback for unauthorized access")
		}

		if err := gs.groupStore.DeleteGroup(txCtx, groupID); err != nil {
			return err
		}
		return nil
	})

	if capturedSvcErr != nil {
		return capturedSvcErr
	}

	if err != nil {
		logger.Error("Failed to delete group", log.Error(err), log.String("groupID", groupID))
		return &serviceerror.InternalServerError
	}

	logger.Debug("Successfully deleted group", log.String("id", groupID))
	return nil
}

// GetGroupMembers retrieves members of a group with pagination.
func (gs *groupService) GetGroupMembers(ctx context.Context, groupID string, limit, offset int,
	includeDisplay bool) (*MemberListResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	if groupID == "" {
		return nil, &ErrorMissingGroupID
	}

	existingGroupDAO, err := gs.groupStore.GetGroup(ctx, groupID)
	if err != nil {
		if errors.Is(err, ErrGroupNotFound) {
			logger.Debug("Group not found", log.String("id", groupID))
			return nil, &ErrorGroupNotFound
		}
		logger.Error("Failed to retrieve group", log.String("id", groupID), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	if err := gs.checkGroupAccess(
		ctx,
		security.ActionReadGroup,
		existingGroupDAO.OUID,
		groupID,
	); err != nil {
		return nil, err
	}

	totalCount, err := gs.groupStore.GetGroupMemberCount(ctx, groupID)
	if err != nil {
		logger.Error("Failed to get group member count", log.String("groupID", groupID), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	members, err := gs.groupStore.GetGroupMembers(ctx, groupID, limit, offset)
	if err != nil {
		logger.Error("Failed to get group members", log.String("groupID", groupID), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	// Always resolve member types (entity → user/app) and optionally resolve display names.
	members, svcErr := gs.resolveMembers(ctx, members, includeDisplay, logger)
	if svcErr != nil {
		return nil, svcErr
	}

	baseURL := fmt.Sprintf("/groups/%s/members", groupID)
	links := utils.BuildPaginationLinks(baseURL, limit, offset, totalCount, utils.DisplayQueryParam(includeDisplay))

	response := &MemberListResponse{
		TotalResults: totalCount,
		Members:      members,
		StartIndex:   offset + 1,
		Count:        len(members),
		Links:        links,
	}

	return response, nil
}

// resolveMembers resolves the public member type (user/app) from the internal 'entity' type
// and optionally populates display names.
func (gs *groupService) resolveMembers(
	ctx context.Context, members []Member, includeDisplay bool, logger *log.Logger,
) ([]Member, *serviceerror.ServiceError) {
	if len(members) == 0 {
		return members, nil
	}

	// Separate entity and group member IDs.
	var entityIDs, groupIDs []string
	for _, m := range members {
		switch m.Type {
		case memberTypeEntity:
			entityIDs = append(entityIDs, m.ID)
		case MemberTypeGroup:
			groupIDs = append(groupIDs, m.ID)
		}
	}

	// Batch-fetch entities to resolve category and optionally display names.
	var entityMap map[string]*entity.Entity
	var displayAttrPaths map[string]string
	if len(entityIDs) > 0 {
		entities, err := gs.entityService.GetEntitiesByIDs(ctx, entityIDs)
		if err != nil {
			logger.Error("Failed to batch-fetch entities for member resolution", log.Error(err))
			return nil, &ErrorInternalServerError
		}
		entityMap = make(map[string]*entity.Entity, len(entities))
		for i := range entities {
			entityMap[entities[i].ID] = &entities[i]
		}
		if includeDisplay {
			var userTypes []string
			for _, e := range entities {
				if e.Category == entity.EntityCategoryUser {
					userTypes = append(userTypes, e.Type)
				}
			}
			displayAttrPaths = resolveDisplayAttributePaths(ctx, userTypes, gs.entityTypeService, logger)
		}
	}

	// Batch-fetch groups for group member display names.
	var groupsMap map[string]*Group
	if includeDisplay && len(groupIDs) > 0 {
		var svcErr *serviceerror.ServiceError
		groupsMap, svcErr = gs.GetGroupsByIDs(ctx, groupIDs)
		if svcErr != nil {
			logger.Warn("Failed to batch-fetch groups for display resolution", log.Any("error", svcErr))
		}
	}

	// Set public type and optionally display on each member.
	// Orphaned entity members (deleted entity with stale assignment row) are dropped.
	resolved := make([]Member, 0, len(members))
	for i := range members {
		switch members[i].Type {
		case memberTypeEntity:
			e, ok := entityMap[members[i].ID]
			if !ok {
				logger.Warn("Skipping orphaned entity member", log.String("id", members[i].ID))
				continue
			}
			// Set the public type from the entity category ("user", "app", or "agent").
			members[i].Type = MemberType(e.Category)
			if includeDisplay {
				switch e.Category {
				case entity.EntityCategoryUser:
					members[i].Display = utils.ResolveDisplay(e.ID, e.Type, e.Attributes, displayAttrPaths)
				case entity.EntityCategoryApp, entity.EntityCategoryAgent:
					members[i].Display = resolveAppDisplay(*e)
				}
			}
		case MemberTypeGroup:
			if includeDisplay {
				if groupsMap != nil {
					if g, ok := groupsMap[members[i].ID]; ok && g.Name != "" {
						members[i].Display = g.Name
					} else {
						members[i].Display = members[i].ID
					}
				} else {
					members[i].Display = members[i].ID
				}
			}
		}
		resolved = append(resolved, members[i])
	}
	return resolved, nil
}

// AddGroupMembers adds members to a group.
func (gs *groupService) AddGroupMembers(
	ctx context.Context, groupID string, members []Member) (*Group, *serviceerror.ServiceError) {
	log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)).
		Debug("Adding members to group", log.String("id", groupID))
	return gs.modifyGroupMembers(ctx, groupID, members,
		gs.groupStore.AddGroupMembers,
		"Failed to add members to group",
		"Successfully added members to group",
	)
}

// RemoveGroupMembers removes members from a group.
func (gs *groupService) RemoveGroupMembers(
	ctx context.Context, groupID string, members []Member) (*Group, *serviceerror.ServiceError) {
	log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)).
		Debug("Removing members from group", log.String("id", groupID))
	return gs.modifyGroupMembers(ctx, groupID, members,
		gs.groupStore.RemoveGroupMembers,
		"Failed to remove members from group",
		"Successfully removed members from group",
	)
}

// modifyGroupMembers is the shared implementation for AddGroupMembers and RemoveGroupMembers.
// It validates, normalizes, and applies storeOp inside a transaction, then resolves member types.
func (gs *groupService) modifyGroupMembers(
	ctx context.Context,
	groupID string,
	members []Member,
	storeOp func(context.Context, string, []Member) error,
	errMsg, successMsg string,
) (*Group, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if groupID == "" {
		return nil, &ErrorMissingGroupID
	}

	if len(members) == 0 {
		return nil, &ErrorEmptyMembers
	}

	if svcErr := validateMemberTypes(members); svcErr != nil {
		return nil, svcErr
	}

	existingGroup, err := gs.groupStore.GetGroup(ctx, groupID)
	if err != nil {
		if errors.Is(err, ErrGroupNotFound) {
			logger.Debug("Group not found", log.String("id", groupID))
			return nil, &ErrorGroupNotFound
		}
		logger.Error("Failed to fetch group", log.String("id", groupID), log.Error(err))
		return nil, &ErrorInternalServerError
	}

	if svcErr := gs.checkGroupAccess(ctx, security.ActionUpdateGroup, existingGroup.OUID, groupID); svcErr != nil {
		return nil, svcErr
	}

	if svcErr := gs.validateEntityMembers(ctx, members, security.ActionUpdateGroup); svcErr != nil {
		return nil, svcErr
	}

	var groupIDs []string
	for _, m := range members {
		if m.Type == MemberTypeGroup {
			groupIDs = append(groupIDs, m.ID)
		}
	}
	if len(groupIDs) > 0 {
		if svcErr := gs.ValidateGroupIDs(ctx, groupIDs); svcErr != nil {
			return nil, svcErr
		}
	}

	members = normalizeMembers(members)

	var capturedSvcErr *serviceerror.ServiceError
	var updatedGroupDAO GroupDAO

	err = gs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		existingGroupDAO, err := gs.groupStore.GetGroup(txCtx, groupID)
		if err != nil {
			if errors.Is(err, ErrGroupNotFound) {
				logger.Debug("Group not found", log.String("id", groupID))
				capturedSvcErr = &ErrorGroupNotFound
				return errors.New("rollback for group not found")
			}
			return err
		}

		if err := gs.checkGroupAccess(
			txCtx,
			security.ActionUpdateGroup,
			existingGroupDAO.OUID,
			groupID,
		); err != nil {
			capturedSvcErr = err
			return errors.New("rollback for unauthorized access")
		}

		if err := storeOp(txCtx, groupID, members); err != nil {
			return err
		}

		groupDAO, err := gs.groupStore.GetGroup(txCtx, groupID)
		if err != nil {
			return err
		}
		updatedGroupDAO = groupDAO

		return nil
	})

	if capturedSvcErr != nil {
		return nil, capturedSvcErr
	}

	if err != nil {
		logger.Error(errMsg, log.String("id", groupID), log.Error(err))
		return nil, &ErrorInternalServerError
	}

	updatedGroup := convertGroupDAOToGroup(updatedGroupDAO)
	resolvedMembers, svcErr := gs.resolveMembers(ctx, updatedGroup.Members, false, logger)
	if svcErr != nil {
		return nil, svcErr
	}
	updatedGroup.Members = resolvedMembers
	logger.Debug(successMsg, log.String("id", groupID))
	return &updatedGroup, nil
}

// validateCreateGroupRequest validates the create group request.
func (gs *groupService) validateCreateGroupRequest(request CreateGroupRequest) *serviceerror.ServiceError {
	if request.Name == "" {
		return &ErrorInvalidRequestFormat
	}

	if request.OUID == "" {
		return &ErrorInvalidRequestFormat
	}

	return validateMemberTypes(request.Members)
}

// validateUpdateGroupRequest validates the update group request.
func (gs *groupService) validateUpdateGroupRequest(request UpdateGroupRequest) *serviceerror.ServiceError {
	if request.Name == "" {
		return &ErrorInvalidRequestFormat
	}

	if request.OUID == "" {
		return &ErrorInvalidRequestFormat
	}

	return nil
}

// validateMemberTypes validates that all members have a valid public type ('user', 'app', 'group')
// and a non-empty ID.
func validateMemberTypes(members []Member) *serviceerror.ServiceError {
	for _, member := range members {
		if !member.Type.IsEntityType() && member.Type != MemberTypeGroup {
			return &ErrorInvalidMemberType
		}
		if member.ID == "" {
			return &ErrorInvalidRequestFormat
		}
	}
	return nil
}

// validateEntityMembers validates user/app members before normalization.
// It checks existence, verifies the claimed type matches the actual entity category,
// and applies OU-scope access checking for user-category entities using the given action.
func (gs *groupService) validateEntityMembers(
	ctx context.Context, members []Member, action security.Action,
) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	typeByID := make(map[string]MemberType)
	for _, m := range members {
		if m.Type.IsEntityType() {
			if existing, ok := typeByID[m.ID]; ok && existing != m.Type {
				return &ErrorInvalidMemberID
			}
			typeByID[m.ID] = m.Type
		}
	}
	if len(typeByID) == 0 {
		return nil
	}

	entityIDs := make([]string, 0, len(typeByID))
	for id := range typeByID {
		entityIDs = append(entityIDs, id)
	}

	entities, err := gs.entityService.GetEntitiesByIDs(ctx, entityIDs)
	if err != nil {
		logger.Error("Failed to fetch entities for member validation", log.Error(err))
		return &ErrorInternalServerError
	}

	if len(entities) != len(entityIDs) {
		return &ErrorInvalidMemberID
	}

	var userIDs []string
	for _, e := range entities {
		claimed := typeByID[e.ID]
		actual := MemberType(e.Category)
		if claimed != actual {
			logger.Debug("Member type mismatch", log.String("id", e.ID),
				log.String("claimed", string(claimed)), log.String("actual", string(actual)))
			return &ErrorInvalidMemberID
		}
		if e.Category == entity.EntityCategoryUser {
			userIDs = append(userIDs, e.ID)
		}
	}

	if len(userIDs) == 0 {
		return nil
	}

	accessibleOUs, svcErr := gs.getAccessibleOUs(ctx, action)
	if svcErr != nil {
		return svcErr
	}

	if accessibleOUs.AllAllowed {
		return nil
	}

	outOfScopeIDs, err := gs.entityService.ValidateEntityIDsInOUs(ctx, userIDs, accessibleOUs.IDs)
	if err != nil {
		logger.Error("Failed to validate user IDs in OUs", log.Error(err))
		return &ErrorInternalServerError
	}

	if len(outOfScopeIDs) > 0 {
		logger.Debug("User IDs outside accessible OUs", log.MaskedStrings("outOfScopeIDs", outOfScopeIDs))
		return &serviceerror.ErrorUnauthorized
	}

	return nil
}

// normalizeMembers converts public 'user'/'app' member types to the internal 'entity' type.
func normalizeMembers(members []Member) []Member {
	normalized := make([]Member, len(members))
	for i, m := range members {
		t := m.Type
		if t.IsEntityType() {
			t = memberTypeEntity
		}
		normalized[i] = Member{ID: m.ID, Type: t}
	}
	return normalized
}

// isOrganizationUnitChanged checks if the organization unit of the group has changed during an update.
func (gs *groupService) isOrganizationUnitChanged(existingGroup Group, request UpdateGroupRequest) bool {
	return existingGroup.OUID != request.OUID
}

// validateOU validates that provided organization unit ID exist.
func (gs *groupService) validateOU(ctx context.Context, ouID string) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	isExists, err := gs.ouService.IsOrganizationUnitExists(ctx, ouID)
	if err != nil {
		logger.Error("Failed to check organization unit existence", log.Any("error: ", err))
		return &serviceerror.InternalServerError
	}

	if !isExists {
		return &ErrorInvalidOUID
	}

	return nil
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

// resolveAppDisplay extracts a display name for an app entity from its system attributes.
// Falls back to the entity ID if no name is found.
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

// ValidateGroupIDs validates that all provided group IDs exist.
func (gs *groupService) ValidateGroupIDs(ctx context.Context, groupIDs []string) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	invalidGroupIDs, err := gs.groupStore.ValidateGroupIDs(ctx, groupIDs)
	if err != nil {
		logger.Error("Failed to validate group IDs", log.Error(err))
		return &serviceerror.InternalServerError
	}

	if len(invalidGroupIDs) > 0 {
		logger.Debug("Invalid group IDs found", log.Any("invalidGroupIDs", invalidGroupIDs))
		return &ErrorInvalidGroupMemberID
	}

	return nil
}

// GetGroupsByIDs retrieves groups by a list of IDs.
func (gs *groupService) GetGroupsByIDs(
	ctx context.Context, groupIDs []string,
) (map[string]*Group, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if len(groupIDs) == 0 {
		return map[string]*Group{}, nil
	}

	// Deduplicate IDs before passing to store.
	seen := make(map[string]struct{}, len(groupIDs))
	uniqueIDs := make([]string, 0, len(groupIDs))
	for _, id := range groupIDs {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			uniqueIDs = append(uniqueIDs, id)
		}
	}

	groupDAOs, err := gs.groupStore.GetGroupsByIDs(ctx, uniqueIDs)
	if err != nil {
		logger.Error("Failed to get groups by IDs", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	result := make(map[string]*Group, len(groupDAOs))
	for _, dao := range groupDAOs {
		group := convertGroupDAOToGroup(GroupDAO{
			ID:          dao.ID,
			Name:        dao.Name,
			Description: dao.Description,
			OUID:        dao.OUID,
		})
		result[dao.ID] = &group
	}

	return result, nil
}

// convertGroupDAOToGroup constructs a Group from a GroupDAO.
func convertGroupDAOToGroup(groupDAO GroupDAO) Group {
	return Group{
		ID:          groupDAO.ID,
		Name:        groupDAO.Name,
		Description: groupDAO.Description,
		OUID:        groupDAO.OUID,
		Members:     groupDAO.Members,
	}
}

// buildGroupBasic constructs a GroupBasic from a GroupBasicDAO.
func buildGroupBasic(groupDAO GroupBasicDAO) GroupBasic {
	return GroupBasic{
		ID:          groupDAO.ID,
		Name:        groupDAO.Name,
		Description: groupDAO.Description,
		OUID:        groupDAO.OUID,
	}
}

// populateGroupOUHandles resolves OU handles for a slice of groups in-place.
func (gs *groupService) populateGroupOUHandles(ctx context.Context, groups []GroupBasic, logger *log.Logger) {
	ouIDs := make([]string, 0, len(groups))
	seen := make(map[string]bool, len(groups))
	for _, g := range groups {
		if g.OUID != "" && !seen[g.OUID] {
			ouIDs = append(ouIDs, g.OUID)
			seen[g.OUID] = true
		}
	}

	handleMap, svcErr := gs.ouService.GetOrganizationUnitHandlesByIDs(ctx, ouIDs)
	if svcErr != nil {
		logger.Warn("Failed to resolve OU handles for groups, skipping", log.Any("error", svcErr))
		return
	}

	for i := range groups {
		if handle, ok := handleMap[groups[i].OUID]; ok {
			groups[i].OUHandle = handle
		}
	}
}

// validatePaginationParams validates pagination parameters.
func validatePaginationParams(limit, offset int) *serviceerror.ServiceError {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return &ErrorInvalidLimit
	}
	if offset < 0 {
		return &ErrorInvalidOffset
	}
	return nil
}

// checkGroupAccess performs an authorization check on the group resource against the current caller.
func (gs *groupService) checkGroupAccess(
	ctx context.Context, action security.Action, ouID string, groupID string) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	actionCtx := sysauthz.ActionContext{
		ResourceType: security.ResourceTypeGroup,
		OUID:         ouID,
		ResourceID:   groupID,
	}

	hasAccess, err := gs.authzService.IsActionAllowed(ctx, action, &actionCtx)
	if err != nil {
		logger.Error("Failed to check authorization", log.String("err", err.Error.DefaultValue))
		return &serviceerror.InternalServerError
	}
	if !hasAccess {
		return &serviceerror.ErrorUnauthorized
	}
	return nil
}

// getAccessibleOUs retrieves the accessible resources for the group.
func (gs *groupService) getAccessibleOUs(
	ctx context.Context, action security.Action) (*sysauthz.AccessibleResources, *serviceerror.ServiceError) {
	accessibleResources, err := gs.authzService.GetAccessibleResources(ctx, action, security.ResourceTypeOU)
	if err != nil {
		return nil, err
	}
	return accessibleResources, nil
}

// validateAndProcessHandlePath validates and processes the handle path.
func (gs *groupService) validateAndProcessHandlePath(handlePath string) *serviceerror.ServiceError {
	trimmedPath := strings.TrimSpace(handlePath)
	if trimmedPath == "" {
		return &ErrorInvalidRequestFormat
	}

	trimmedPath = strings.Trim(trimmedPath, "/")
	if trimmedPath == "" {
		return &ErrorInvalidRequestFormat
	}

	handles := strings.Split(trimmedPath, "/")
	for _, handle := range handles {
		if strings.TrimSpace(handle) == "" {
			return &ErrorInvalidRequestFormat
		}
	}
	return nil
}
