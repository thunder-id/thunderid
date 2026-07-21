/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

// Package user provides user management functionality.
package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const loggerComponentName = "UserService"

// UserServiceInterface defines the interface for the user service.
type UserServiceInterface interface {
	GetUserList(ctx context.Context, limit, offset int,
		filters map[string]interface{}, includeDisplay bool) (*UserListResponse, *tidcommon.ServiceError)
	GetUsersByPath(ctx context.Context, handlePath string, limit, offset int,
		filters map[string]interface{}, includeDisplay bool) (*UserListResponse, *tidcommon.ServiceError)
	CreateUser(ctx context.Context, user *User) (*User, *tidcommon.ServiceError)
	CreateUserByPath(ctx context.Context, handlePath string,
		request CreateUserByPathRequest) (*User, *tidcommon.ServiceError)
	GetUser(ctx context.Context, userID string, includeDisplay bool) (*User, *tidcommon.ServiceError)
	GetUserGroups(ctx context.Context, userID string,
		limit, offset int) (*UserGroupListResponse, *tidcommon.ServiceError)
	UpdateUser(ctx context.Context, userID string, user *User) (*User, *tidcommon.ServiceError)
	UpdateUserAttributes(ctx context.Context, userID string,
		attributes json.RawMessage) (*User, *tidcommon.ServiceError)
	UpdateUserCredentials(ctx context.Context, userID string,
		credentials json.RawMessage) *tidcommon.ServiceError
	DeleteUser(ctx context.Context, userID string) *tidcommon.ServiceError
	ResolveUserOUHandle(ctx context.Context, user *User) *tidcommon.ServiceError
	SetDependencyRegistry(r resourcedependency.Registry)
	GetUserUsages(ctx context.Context, userID string) (
		*resourcedependency.DependenciesResponse, *tidcommon.ServiceError)
}

// userService is the default implementation of the UserServiceInterface.
type userService struct {
	authzService       sysauthz.SystemAuthorizationServiceInterface
	entityService      entity.EntityServiceInterface
	ouService          oupkg.OrganizationUnitServiceInterface
	entityTypeService  entitytype.EntityTypeServiceInterface
	uuidGenerator      func() (string, error)
	dependencyRegistry resourcedependency.Registry
}

// newUserService creates a new instance of userService with injected dependencies.
func newUserService(
	authzService sysauthz.SystemAuthorizationServiceInterface,
	entityService entity.EntityServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
) UserServiceInterface {
	return &userService{
		authzService:      authzService,
		entityService:     entityService,
		ouService:         ouService,
		entityTypeService: entityTypeService,
		uuidGenerator:     utils.GenerateUUIDv7,
	}
}

// GetUserList retrieves a list of users with pagination and filtering.
func (us *userService) GetUserList(ctx context.Context, limit, offset int,
	filters map[string]interface{}, includeDisplay bool) (*UserListResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	// Resolve the set of organization units the caller is authorized to list users from.
	accessible, svcErr := us.authzService.GetAccessibleResources(
		ctx, security.ActionListUsers, security.ResourceTypeOU)
	if svcErr != nil {
		logger.Error(ctx, "Failed to resolve accessible resources for listing users",
			log.Any("error", svcErr))
		return nil, &tidcommon.InternalServerError
	}

	// Unfiltered path: system-level caller — return all users.
	if accessible.AllAllowed {
		return us.listAllUsers(ctx, limit, offset, filters, includeDisplay, logger)
	}

	// Filtered path: return users belonging to the accessible OUs.
	return us.listUsersByOUIDs(ctx, accessible.IDs, limit, offset, filters, includeDisplay, logger)
}

// listAllUsers retrieves users without OU filtering.
func (us *userService) listAllUsers(
	ctx context.Context, limit, offset int, filters map[string]interface{},
	includeDisplay bool, logger *log.Logger,
) (*UserListResponse, *tidcommon.ServiceError) {
	totalCount, err := us.entityService.GetEntityListCount(ctx, providers.EntityCategoryUser, filters)
	if err != nil {
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to get user list count", err)
	}

	entities, err := us.entityService.GetEntityList(ctx, providers.EntityCategoryUser, limit, offset, filters)
	if err != nil {
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to get user list", err)
	}

	users := entitiesToUsers(entities)
	if includeDisplay {
		us.populateUserDisplayNames(ctx, users, logger)
		us.populateOUHandles(ctx, users, logger)
	}

	return buildUserListResponse(users, totalCount, limit, offset, utils.DisplayQueryParam(includeDisplay)), nil
}

// listUsersByOUIDs retrieves users scoped to the given organization unit IDs.
func (us *userService) listUsersByOUIDs(
	ctx context.Context, ouIDs []string, limit, offset int, filters map[string]interface{},
	includeDisplay bool, logger *log.Logger,
) (*UserListResponse, *tidcommon.ServiceError) {
	displayQuery := utils.DisplayQueryParam(includeDisplay)

	if len(ouIDs) == 0 {
		return buildUserListResponse([]User{}, 0, limit, offset, displayQuery), nil
	}

	totalCount, err := us.entityService.GetEntityListCountByOUIDs(ctx, providers.EntityCategoryUser, ouIDs, filters)
	if err != nil {
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to get user list count", err)
	}

	entities, err := us.entityService.GetEntityListByOUIDs(
		ctx, providers.EntityCategoryUser, ouIDs, limit, offset, filters)
	if err != nil {
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to get user list", err)
	}

	users := entitiesToUsers(entities)
	if includeDisplay {
		us.populateUserDisplayNames(ctx, users, logger)
		us.populateOUHandles(ctx, users, logger)
	}

	return buildUserListResponse(users, totalCount, limit, offset, displayQuery), nil
}

// buildUserListResponse constructs a paginated UserListResponse.
func buildUserListResponse(users []User, totalCount, limit, offset int, displayQuery string) *UserListResponse {
	return &UserListResponse{
		TotalResults: totalCount,
		StartIndex:   offset + 1,
		Count:        len(users),
		Users:        users,
		Links:        utils.BuildPaginationLinks("/users", limit, offset, totalCount, displayQuery),
	}
}

// GetUsersByPath retrieves a list of users by hierarchical handle path.
func (us *userService) GetUsersByPath(
	ctx context.Context, handlePath string, limit, offset int, filters map[string]interface{},
	includeDisplay bool,
) (*UserListResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Getting users by path", log.String("path", handlePath))

	serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return nil, serviceError
	}

	ou, svcErr := us.ouService.GetOrganizationUnitByPath(ctx, handlePath)
	if svcErr != nil {
		return nil, mapOUServiceError(ctx,
			svcErr,
			logger,
			"resolving organization unit by path",
			map[string]*tidcommon.ServiceError{
				oupkg.ErrorOrganizationUnitNotFound.Code: &ErrorOrganizationUnitNotFound,
				oupkg.ErrorInvalidHandlePath.Code:        &ErrorInvalidHandlePath,
			},
			log.String("path", handlePath),
		)
	}
	oUID := ou.ID

	// Check if caller is authorized to list users in the resolved OU.
	if svcErr := us.checkUserAccess(ctx, security.ActionListUsers, oUID, ""); svcErr != nil {
		return nil, svcErr
	}

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	ouResponse, svcErr := us.ouService.GetOrganizationUnitUsers(ctx, oUID, limit, offset, false)
	if svcErr != nil {
		return nil, mapOUServiceError(ctx,
			svcErr,
			logger,
			"listing organization unit users",
			map[string]*tidcommon.ServiceError{
				oupkg.ErrorOrganizationUnitNotFound.Code: &ErrorOrganizationUnitNotFound,
				oupkg.ErrorInvalidLimit.Code:             &ErrorInvalidLimit,
				oupkg.ErrorInvalidOffset.Code:            &ErrorInvalidOffset,
			},
			log.String("oUID", oUID),
			log.Int("limit", limit),
			log.Int("offset", offset),
		)
	}
	if ouResponse == nil {
		return &UserListResponse{}, nil
	}

	var users []User
	if includeDisplay && len(ouResponse.Users) > 0 {
		// Batch-fetch full user data to resolve display names.
		userIDs := make([]string, len(ouResponse.Users))
		for i, ouUser := range ouResponse.Users {
			userIDs[i] = ouUser.ID
		}
		fetchedEntities, err := us.entityService.GetEntitiesByIDs(ctx, userIDs)
		if err != nil {
			logger.Warn(ctx, "Failed to batch fetch users for display names, skipping display resolution",
				log.Error(err))
			// Fall back to bare IDs without display — partial display is worse than none.
			users = make([]User, len(ouResponse.Users))
			for i, ouUser := range ouResponse.Users {
				users[i] = User{ID: ouUser.ID, OUHandle: ou.Handle}
			}
		} else {
			fetchedUsers := entitiesToUsers(fetchedEntities)
			// Build an ID-keyed map for display resolution, but only expose ID + Display.
			userMap := make(map[string]User, len(fetchedUsers))
			for _, u := range fetchedUsers {
				userMap[u.ID] = u
			}

			// Resolve display attribute paths for the fetched user types.
			userTypes := make([]string, 0, len(fetchedUsers))
			for _, u := range fetchedUsers {
				userTypes = append(userTypes, u.Type)
			}
			displayAttrPaths := ResolveDisplayAttributePaths(ctx, userTypes, us.entityTypeService, logger)

			users = make([]User, len(ouResponse.Users))
			for i, ouUser := range ouResponse.Users {
				if u, ok := userMap[ouUser.ID]; ok {
					users[i] = User{
						ID:       u.ID,
						OUHandle: ou.Handle,
						Display:  utils.ResolveDisplay(u.ID, u.Type, u.Attributes, displayAttrPaths),
					}
				} else {
					users[i] = User{ID: ouUser.ID, OUHandle: ou.Handle}
				}
			}
		}
	} else {
		users = make([]User, len(ouResponse.Users))
		for i, ouUser := range ouResponse.Users {
			users[i] = User{ID: ouUser.ID}
		}
	}

	response := &UserListResponse{
		TotalResults: ouResponse.TotalResults,
		StartIndex:   ouResponse.StartIndex,
		Count:        ouResponse.Count,
		Users:        users,
		Links: buildTreePaginationLinks(
			handlePath, limit, offset, ouResponse.TotalResults, utils.DisplayQueryParam(includeDisplay)),
	}

	return response, nil
}

// CreateUser creates the user.
func (us *userService) CreateUser(ctx context.Context, user *User) (*User, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if user == nil {
		return nil, &ErrorInvalidRequestFormat
	}

	// Check if caller is authorized to create users in the target OU.
	if svcErr := us.checkUserAccess(ctx, security.ActionCreateUser, user.OUID, ""); svcErr != nil {
		return nil, svcErr
	}

	if svcErr := us.validateOrganizationUnitForUserType(ctx, user.Type, user.OUID, logger); svcErr != nil {
		return nil, svcErr
	}

	// Schema validation and uniqueness checks are handled by entity service in CreateEntity.

	var err error
	if user.ID == "" {
		user.ID, err = us.uuidGenerator()
		if err != nil {
			logger.Error(ctx, "Failed to generate UUID", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
	}

	e := userToEntity(user)
	created, err := us.entityService.CreateEntity(ctx, e, nil)
	if err != nil {
		if svcErr := mapEntityError(err); svcErr != nil {
			return nil, svcErr
		}
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to create user", err)
	}

	// Sync cleaned attributes back — entity service removed credential fields from Attributes.
	user.Attributes = created.Attributes

	logger.Debug(ctx, "Successfully created user", log.MaskedString(log.LoggerKeyUserID, user.ID))
	return user, nil
}

// CreateUserByPath creates a new user under the organization unit specified by the handle path.
func (us *userService) CreateUserByPath(
	ctx context.Context, handlePath string, request CreateUserByPathRequest,
) (*User, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Creating user by path",
		log.String("path", handlePath), log.String("type", request.Type))

	serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return nil, serviceError
	}

	ou, svcErr := us.ouService.GetOrganizationUnitByPath(ctx, handlePath)
	if svcErr != nil {
		return nil, mapOUServiceError(ctx,
			svcErr,
			logger,
			"resolving organization unit by path",
			map[string]*tidcommon.ServiceError{
				oupkg.ErrorOrganizationUnitNotFound.Code: &ErrorOrganizationUnitNotFound,
				oupkg.ErrorInvalidHandlePath.Code:        &ErrorInvalidHandlePath,
			},
			log.String("path", handlePath),
		)
	}

	user := &User{
		OUID:       ou.ID,
		Type:       request.Type,
		Attributes: request.Attributes,
	}

	return us.CreateUser(ctx, user)
}

// GetUser retrieves a user by ID.
func (us *userService) GetUser(
	ctx context.Context, userID string, includeDisplay bool,
) (*User, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Retrieving user", log.MaskedString(log.LoggerKeyUserID, userID))

	if userID == "" {
		return nil, &ErrorMissingUserID
	}

	e, err := us.entityService.GetEntity(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			logger.Debug(ctx, "User not found", log.MaskedString(log.LoggerKeyUserID, userID))
			return nil, &ErrorUserNotFound
		}
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to retrieve user", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if e.Category != providers.EntityCategoryUser {
		return nil, &ErrorUserNotFound
	}
	user := entityToUser(e)

	// Check authz using the user's OU ID (fetched from store).
	if svcErr := us.checkUserAccess(ctx, security.ActionReadUser, user.OUID, userID); svcErr != nil {
		return nil, svcErr
	}

	if includeDisplay {
		displayAttrPaths := ResolveDisplayAttributePaths(
			ctx, []string{user.Type}, us.entityTypeService, logger)
		user.Display = utils.ResolveDisplay(
			user.ID, user.Type, user.Attributes, displayAttrPaths)

		handleMap, svcErr := us.ouService.GetOrganizationUnitHandlesByIDs(ctx, []string{user.OUID})
		if svcErr != nil {
			logger.Warn(ctx, "Failed to resolve OU handle for user, skipping",
				log.Any("error", svcErr))
		} else if handle, ok := handleMap[user.OUID]; ok {
			user.OUHandle = handle
		}
	}

	logger.Debug(ctx, "Successfully retrieved user", log.MaskedString(log.LoggerKeyUserID, userID))
	return &user, nil
}

// GetUserGroups retrieves groups of a user with pagination.
func (as *userService) GetUserGroups(ctx context.Context, userID string, limit, offset int) (
	*UserGroupListResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if userID == "" {
		return nil, &ErrorMissingUserID
	}

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	// Fetch user to resolve the OU ID for the authorization check.
	userEntity, err := as.entityService.GetEntity(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			logger.Debug(ctx, "User not found", log.MaskedString(log.LoggerKeyUserID, userID))
			return nil, &ErrorUserNotFound
		}
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to retrieve user", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if userEntity.Category != providers.EntityCategoryUser {
		return nil, &ErrorUserNotFound
	}

	// Check authz using the user's OU ID.
	if svcErr := as.checkUserAccess(
		ctx, security.ActionReadUser, userEntity.OUID, userID); svcErr != nil {
		return nil, svcErr
	}

	totalCount, err := as.entityService.GetGroupCountForEntity(ctx, userID)
	if err != nil {
		logger.Error(ctx, "Failed to get group count for user",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	entityGroups, err := as.entityService.GetEntityGroups(ctx, userID, limit, offset)
	if err != nil {
		logger.Error(ctx, "Failed to get user groups",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	path := fmt.Sprintf("/users/%s/groups", userID)
	links := utils.BuildPaginationLinks(path, limit, offset, totalCount, "")

	response := &UserGroupListResponse{
		TotalResults: totalCount,
		Groups:       entityGroups,
		StartIndex:   offset + 1,
		Count:        len(entityGroups),
		Links:        links,
	}

	return response, nil
}

// UpdateUser update the user for given user id.
func (us *userService) UpdateUser(
	ctx context.Context, userID string, user *User) (*User, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Updating user", log.MaskedString(log.LoggerKeyUserID, userID))

	if userID == "" {
		return nil, &ErrorMissingUserID
	}

	if user == nil {
		return nil, &ErrorInvalidRequestFormat
	}

	// Fetch the existing user to obtain its OU ID for the authorization check.
	existingEntity, err := us.entityService.GetEntity(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			logger.Debug(ctx, "User not found", log.MaskedString(log.LoggerKeyUserID, userID))
			return nil, &ErrorUserNotFound
		}
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to retrieve user", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if existingEntity.Category != providers.EntityCategoryUser {
		return nil, &ErrorUserNotFound
	}
	existingUser := entityToUser(existingEntity)

	// Check authz using the existing user's OU ID.
	if svcErr := us.checkUserAccess(
		ctx, security.ActionUpdateUser, existingUser.OUID, userID); svcErr != nil {
		return nil, svcErr
	}

	// If the user is moving to a different OU, require authorization for the destination OU as well.
	if user.OUID != existingUser.OUID {
		if svcErr := us.checkUserAccess(
			ctx, security.ActionUpdateUser, user.OUID, userID); svcErr != nil {
			return nil, svcErr
		}
	}

	// Check if user is declarative (immutable)
	if svcErr := us.checkUserDeclarative(ctx, userID, logger); svcErr != nil {
		return nil, svcErr
	}

	// Ensure the user object has the correct ID
	user.ID = userID

	if svcErr := us.validateOrganizationUnitForUserType(
		ctx, user.Type, user.OUID, logger,
	); svcErr != nil {
		return nil, svcErr
	}

	// Reject credential fields: this endpoint is for attribute and user metadata updates only.
	// Credentials must go through the dedicated update-credentials endpoint.
	if len(user.Attributes) > 0 {
		schemaCredentialInfos, svcErr := us.entityTypeService.GetAttributes(ctx,
			entitytype.TypeCategoryUser, user.Type, true, false, false)
		if svcErr != nil {
			if svcErr.Code == entitytype.ErrorEntityTypeNotFound.Code {
				return nil, &ErrorEntityTypeNotFound
			}
			return nil, logErrorAndReturnServerError(ctx, logger, "Failed to get credential attributes from schema",
				fmt.Errorf("schema service error: %s", svcErr.ErrorDescription.DefaultValue),
				log.MaskedString(log.LoggerKeyUserID, userID))
		}
		if len(schemaCredentialInfos) > 0 {
			var attrs map[string]any
			if err := json.Unmarshal(user.Attributes, &attrs); err != nil {
				return nil, &ErrorInvalidRequestFormat
			}
			for _, credInfo := range schemaCredentialInfos {
				if _, ok := attrs[credInfo.Attribute]; ok {
					return nil, &ErrorCredentialUpdateNotAllowed
				}
			}
		}
	}

	e := userToEntity(user)
	e.SystemAttributes = existingEntity.SystemAttributes
	updated, err := us.entityService.UpdateEntity(ctx, userID, e)
	if err != nil {
		if svcErr := mapEntityError(err); svcErr != nil {
			return nil, svcErr
		}
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to update user", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}

	// Sync cleaned attributes back — entity service removed credential fields from Attributes.
	user.Attributes = updated.Attributes
	logger.Debug(ctx, "Successfully updated user", log.MaskedString(log.LoggerKeyUserID, userID))
	return user, nil
}

// UpdateUserAttributes updates only the attributes of a user while preserving immutable fields.
func (us *userService) UpdateUserAttributes(
	ctx context.Context, userID string, attributes json.RawMessage,
) (*User, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Updating user attributes", log.MaskedString(log.LoggerKeyUserID, userID))

	if strings.TrimSpace(userID) == "" {
		return nil, &ErrorMissingUserID
	}

	if len(attributes) == 0 {
		return nil, &ErrorInvalidRequestFormat
	}

	// Pre-fetch user to get the type for credential field lookup (outside transaction).
	existingEntity, getErr := us.entityService.GetEntity(ctx, userID)
	if getErr != nil {
		if errors.Is(getErr, entity.ErrEntityNotFound) {
			logger.Debug(ctx, "User not found", log.MaskedString(log.LoggerKeyUserID, userID))
			return nil, &ErrorUserNotFound
		}
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to get user", getErr,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if existingEntity.Category != providers.EntityCategoryUser {
		return nil, &ErrorUserNotFound
	}
	existingUser := entityToUser(existingEntity)

	// Reject credential fields here: this endpoint is for attribute updates only.
	// Credentials must go through UpdateUserCredentials, which enforces its own authz and validation.
	if us.entityTypeService == nil {
		logger.Error(ctx, "Entity type service is not configured for user operations")
		return nil, &tidcommon.InternalServerError
	}
	schemaCredentialInfos, svcErr := us.entityTypeService.GetAttributes(ctx,
		entitytype.TypeCategoryUser, existingUser.Type, true, false, false)
	if svcErr != nil {
		if svcErr.Code == entitytype.ErrorEntityTypeNotFound.Code {
			return nil, &ErrorEntityTypeNotFound
		}
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to get credential attributes from schema",
			fmt.Errorf("schema service error: %s", svcErr.ErrorDescription.DefaultValue),
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if len(schemaCredentialInfos) > 0 {
		var attrs map[string]any
		if err := json.Unmarshal(attributes, &attrs); err != nil {
			return nil, &ErrorInvalidRequestFormat
		}
		for _, credInfo := range schemaCredentialInfos {
			if _, ok := attrs[credInfo.Attribute]; ok {
				return nil, &ErrorInvalidRequestFormat
			}
		}
	}

	// Check authz outside the transaction so a denial is returned directly without a rollback.
	if svcErr := us.checkUserAccess(
		ctx, security.ActionUpdateUser, existingUser.OUID, userID); svcErr != nil {
		return nil, svcErr
	}

	// Check if user is declarative (immutable)
	if svcErr := us.checkUserDeclarative(ctx, userID, logger); svcErr != nil {
		return nil, svcErr
	}

	existingUser.Attributes = attributes

	if err := us.entityService.UpdateAttributes(ctx, userID, attributes); err != nil {
		if svcErr := mapEntityError(err); svcErr != nil {
			return nil, svcErr
		}
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to update user attributes", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}

	logger.Debug(ctx, "Successfully updated user attributes", log.MaskedString(log.LoggerKeyUserID, userID))
	return &existingUser, nil
}

// UpdateUserCredentials updates schema-defined credentials for a user.
func (us *userService) UpdateUserCredentials(
	ctx context.Context,
	userID string,
	credentials json.RawMessage,
) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Updating user credentials", log.MaskedString(log.LoggerKeyUserID, userID))

	if strings.TrimSpace(userID) == "" {
		return &ErrorAuthenticationFailed
	}

	if len(credentials) == 0 {
		return &ErrorMissingCredentials
	}

	// Parse credentials to extract credential types
	var credentialsMap map[string]json.RawMessage
	if err := json.Unmarshal(credentials, &credentialsMap); err != nil {
		logger.Debug(ctx, "Failed to parse credentials", log.Error(err))
		return &ErrorInvalidRequestFormat
	}

	if len(credentialsMap) == 0 {
		return &ErrorMissingCredentials
	}

	// Reject system-managed credential types (e.g., passkey). Only schema-defined
	// credentials may be updated through this endpoint.
	for credTypeStr := range credentialsMap {
		if CredentialType(credTypeStr).IsSystemManaged() {
			return &ErrorInvalidCredential
		}
	}

	// Fetch user outside the transaction to resolve the OU ID for the authorization check.
	existingEntity, err := us.entityService.GetEntity(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			logger.Debug(ctx, "User not found", log.MaskedString(log.LoggerKeyUserID, userID))
			return &ErrorUserNotFound
		}
		return logErrorAndReturnServerError(ctx, logger, "Failed to retrieve user", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if existingEntity.Category != providers.EntityCategoryUser {
		return &ErrorUserNotFound
	}
	existingUser := entityToUser(existingEntity)

	// Check authz outside the transaction so a denial is returned directly without a rollback.
	if svcErr := us.checkUserAccess(
		ctx, security.ActionUpdateUser, existingUser.OUID, userID); svcErr != nil {
		return svcErr
	}

	// Check if user is declarative (immutable)
	if svcErr := us.checkUserDeclarative(ctx, userID, logger); svcErr != nil {
		return svcErr
	}

	// Normalize credential values to plaintext strings. Entity service enforces the
	// schema-credential allowlist and non-empty checks.
	plaintextCreds := make(map[string]string, len(credentialsMap))
	for credTypeStr, credValue := range credentialsMap {
		if len(credValue) == 0 {
			return &ErrorMissingCredentials
		}
		var stringValue string
		if err := json.Unmarshal(credValue, &stringValue); err != nil {
			return &ErrorInvalidRequestFormat
		}
		plaintextCreds[credTypeStr] = stringValue
	}

	plaintextJSON, err := json.Marshal(plaintextCreds)
	if err != nil {
		return logErrorAndReturnServerError(ctx, logger, "Failed to marshal credentials", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if err = us.entityService.UpdateCredentials(ctx, userID, plaintextJSON); err != nil {
		if svcErr := mapEntityError(err); svcErr != nil {
			return svcErr
		}
		return logErrorAndReturnServerError(ctx, logger, "Failed to update user credentials", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}

	logger.Debug(ctx, "Successfully updated user credentials",
		log.MaskedString(log.LoggerKeyUserID, userID),
		log.Int("credentialTypesCount", len(credentialsMap)))
	return nil
}

// DeleteUser delete the user for given user id.
func (us *userService) DeleteUser(ctx context.Context, userID string) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Deleting user", log.MaskedString(log.LoggerKeyUserID, userID))

	if userID == "" {
		return &ErrorMissingUserID
	}

	// Fetch the user to resolve the OU ID for the authorization check.
	existingEntity, err := us.entityService.GetEntity(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			logger.Debug(ctx, "User not found", log.MaskedString(log.LoggerKeyUserID, userID))
			return &ErrorUserNotFound
		}
		return logErrorAndReturnServerError(ctx, logger, "Failed to retrieve user", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if existingEntity.Category != providers.EntityCategoryUser {
		return &ErrorUserNotFound
	}
	existingUser := entityToUser(existingEntity)

	// Check authz using the user's OU ID.
	if svcErr := us.checkUserAccess(
		ctx, security.ActionDeleteUser, existingUser.OUID, userID); svcErr != nil {
		return svcErr
	}

	// Check if user is declarative (immutable)
	if svcErr := us.checkUserDeclarative(ctx, userID, logger); svcErr != nil {
		return svcErr
	}

	// Refuse deletion while other resources block it (e.g. agents owned by the user).
	if svcErr := us.ensureNoBlockingDependencies(ctx, userID, logger); svcErr != nil {
		return svcErr
	}

	// Remove dependents that must be deleted with the user (e.g. role assignments). Run before the
	// entity delete so a cleanup failure aborts the operation and leaves the user retriable. The
	// registry is guaranteed non-nil here because ensureNoBlockingDependencies fails closed otherwise.
	if _, err = us.dependencyRegistry.CascadeDelete(ctx, resourcedependency.ResourceTypeUser, userID); err != nil {
		return logErrorAndReturnServerError(ctx, logger, "Failed to cascade-delete user dependencies", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}

	err = us.entityService.DeleteEntity(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			logger.Debug(ctx, "User not found", log.MaskedString(log.LoggerKeyUserID, userID))
			return &ErrorUserNotFound
		}
		return logErrorAndReturnServerError(ctx, logger, "Failed to delete user", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}

	logger.Debug(ctx, "Successfully deleted user", log.MaskedString(log.LoggerKeyUserID, userID))
	return nil
}

// ensureNoBlockingDependencies refuses deletion when other resources depend on the user in a way
// that forbids it (behaviorOnDelete == restrict), such as agents that list the user as their owner.
// Because deletion is destructive, it fails closed: if dependency data cannot be determined, the
// deletion is refused rather than allowed.
func (us *userService) ensureNoBlockingDependencies(
	ctx context.Context, userID string, logger *log.Logger) *tidcommon.ServiceError {
	if us.dependencyRegistry == nil {
		logger.Error(ctx, "Dependency registry not set; refusing to delete user",
			log.MaskedString(log.LoggerKeyUserID, userID))
		return &tidcommon.InternalServerError
	}

	deps, err := us.dependencyRegistry.GetDependencies(ctx, resourcedependency.ResourceTypeUser, userID)
	if err != nil {
		return logErrorAndReturnServerError(ctx, logger, "Failed to evaluate user dependencies", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	// Fail closed: nil TotalResults means a provider failed to report, so usage is unknown.
	if deps == nil || deps.TotalResults == nil {
		logger.Error(ctx, "User dependency data unavailable; refusing to delete user",
			log.MaskedString(log.LoggerKeyUserID, userID))
		return &tidcommon.InternalServerError
	}

	blocking := resourcedependency.BlockingUsages(deps)
	if len(blocking) == 0 {
		return nil
	}

	logger.Debug(ctx, "User has blocking dependencies; deletion refused",
		log.MaskedString(log.LoggerKeyUserID, userID), log.Int("blockingCount", len(blocking)))
	return tidcommon.CustomServiceError(ErrorUserHasBlockingDependencies, tidcommon.I18nMessage{
		Key: "error.userservice.user_has_blocking_dependencies_description",
		DefaultValue: fmt.Sprintf(
			"The user cannot be deleted because %s depend on it. Remove or reassign them first.",
			summarizeBlockingUsages(blocking)),
	})
}

// summarizeBlockingUsages renders a deterministic, human-readable summary of blocking dependencies
// grouped by resource type, e.g. "2 agent(s)".
func summarizeBlockingUsages(usages []resourcedependency.ResourceDependency) string {
	counts := make(map[string]int)
	for _, u := range usages {
		counts[u.ResourceType]++
	}
	types := make([]string, 0, len(counts))
	for rt := range counts {
		types = append(types, rt)
	}
	sort.Strings(types)
	parts := make([]string, 0, len(types))
	for _, rt := range types {
		parts = append(parts, fmt.Sprintf("%d %s(s)", counts[rt], rt))
	}
	return strings.Join(parts, ", ")
}

// SetDependencyRegistry injects the dependency registry. Called by servicemanager after the
// provider services are initialized to avoid a cyclic import.
func (us *userService) SetDependencyRegistry(r resourcedependency.Registry) {
	us.dependencyRegistry = r
}

// GetUserUsages returns the resources that reference this user, such as agents that list the user
// as their owner. It is informational — it drives the pre-delete confirmation dialog and does not
// gate deletion on the server.
func (us *userService) GetUserUsages(
	ctx context.Context, userID string,
) (*resourcedependency.DependenciesResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if userID == "" {
		return nil, &ErrorMissingUserID
	}

	existingEntity, err := us.entityService.GetEntity(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil, &ErrorUserNotFound
		}
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to retrieve user", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if existingEntity.Category != providers.EntityCategoryUser {
		return nil, &ErrorUserNotFound
	}

	if us.dependencyRegistry == nil {
		logger.Warn(ctx, "Dependency registry not set; returning unknown dependencies",
			log.MaskedString(log.LoggerKeyUserID, userID))
		return &resourcedependency.DependenciesResponse{
			TotalResults: nil,
			Count:        0,
			Summary:      nil,
			Usages:       []resourcedependency.ResourceDependency{},
		}, nil
	}

	result, err := us.dependencyRegistry.GetDependencies(ctx, resourcedependency.ResourceTypeUser, userID)
	if err != nil {
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to get user usages", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}

	return result, nil
}

// populateUserDisplayNames resolves display names for a slice of users in-place.
// It batch-fetches display attribute paths from the entity type service and extracts the
// display value from each user's attributes. Falls back to user ID if extraction fails.
func (us *userService) populateUserDisplayNames(ctx context.Context, users []User, logger *log.Logger) {
	// Collect user types for display attribute resolution.
	userTypes := make([]string, 0, len(users))
	for _, u := range users {
		userTypes = append(userTypes, u.Type)
	}

	displayAttrPaths := ResolveDisplayAttributePaths(
		ctx, userTypes, us.entityTypeService, logger)

	// Resolve display for each user.
	for i := range users {
		users[i].Display = utils.ResolveDisplay(
			users[i].ID, users[i].Type, users[i].Attributes, displayAttrPaths)
	}
}

// populateOUHandles resolves OU handles for a slice of users in-place.
func (us *userService) populateOUHandles(ctx context.Context, users []User, logger *log.Logger) {
	ouIDs := make([]string, 0, len(users))
	seen := make(map[string]bool, len(users))
	for _, u := range users {
		if u.OUID != "" && !seen[u.OUID] {
			ouIDs = append(ouIDs, u.OUID)
			seen[u.OUID] = true
		}
	}

	handleMap, svcErr := us.ouService.GetOrganizationUnitHandlesByIDs(ctx, ouIDs)
	if svcErr != nil {
		logger.Warn(ctx, "Failed to resolve OU handles, skipping", log.Any("error", svcErr))
		return
	}

	for i := range users {
		if handle, ok := handleMap[users[i].OUID]; ok {
			users[i].OUHandle = handle
		}
	}
}

// validateOrganizationUnitForUserType ensures that the organization unit ID is valid and belongs to the user type.
func (us *userService) validateOrganizationUnitForUserType(
	ctx context.Context, userType, oUID string, logger *log.Logger,
) *tidcommon.ServiceError {
	if strings.TrimSpace(userType) == "" {
		return &ErrorEntityTypeNotFound
	}

	if strings.TrimSpace(oUID) == "" {
		return &ErrorInvalidOUID
	}

	if us.ouService == nil {
		logger.Error(ctx, "Organization unit service is not configured for user operations")
		return &tidcommon.InternalServerError
	}

	exists, svcErr := us.ouService.IsOrganizationUnitExists(ctx, oUID)
	if svcErr != nil {
		return mapOUServiceError(ctx,
			svcErr,
			logger,
			"verifying organization unit existence",
			map[string]*tidcommon.ServiceError{
				oupkg.ErrorOrganizationUnitNotFound.Code: &ErrorOrganizationUnitNotFound,
				oupkg.ErrorInvalidRequestFormat.Code:     &ErrorInvalidOUID,
				oupkg.ErrorMissingOUID.Code:              &ErrorInvalidOUID,
			},
			log.String("oUID", oUID),
		)
	}
	if !exists {
		return &ErrorOrganizationUnitNotFound
	}

	if us.entityTypeService == nil {
		logger.Error(ctx, "Entity type service is not configured for user operations")
		return &tidcommon.InternalServerError
	}

	entityType, svcErr := us.entityTypeService.GetEntityTypeByName(ctx,
		entitytype.TypeCategoryUser, userType)
	if svcErr != nil {
		if svcErr.Code == entitytype.ErrorEntityTypeNotFound.Code {
			return &ErrorEntityTypeNotFound
		}
		logger.Error(ctx, "Failed to retrieve user type",
			log.String("userType", userType), log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}

	if entityType == nil {
		logger.Error(ctx, "Entity type service returned nil response", log.String("userType", userType))
		return &tidcommon.InternalServerError
	}

	if entityType.OUID == oUID {
		return nil
	}

	isParent, svcErr := us.ouService.IsParent(ctx, entityType.OUID, oUID)
	if svcErr != nil {
		return mapOUServiceError(ctx,
			svcErr,
			logger,
			"validating organization unit hierarchy",
			map[string]*tidcommon.ServiceError{
				oupkg.ErrorOrganizationUnitNotFound.Code: &ErrorOrganizationUnitNotFound,
			},
			log.String("userType", userType),
			log.String("oUID", oUID),
			log.String("schemaOUID", entityType.OUID),
		)
	}

	if !isParent {
		logger.Debug(ctx, "Organization unit mismatch for user type",
			log.String("userType", userType),
			log.String("oUID", oUID),
			log.String("schemaOUID", entityType.OUID))
		return &ErrorOrganizationUnitMismatch
	}

	return nil
}

// validateAndProcessHandlePath validates and processes the handle path.
func validateAndProcessHandlePath(handlePath string) *tidcommon.ServiceError {
	if strings.TrimSpace(handlePath) == "" {
		return &ErrorInvalidHandlePath
	}

	handles := strings.Split(strings.Trim(handlePath, "/"), "/")
	if len(handles) == 0 {
		return &ErrorInvalidHandlePath
	}

	for _, handle := range handles {
		if strings.TrimSpace(handle) == "" {
			return &ErrorInvalidHandlePath
		}
	}
	return nil
}

// validatePaginationParams validates pagination parameters.
func validatePaginationParams(limit, offset int) *tidcommon.ServiceError {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return &ErrorInvalidLimit
	}
	if offset < 0 {
		return &ErrorInvalidOffset
	}
	return nil
}

// logErrorAndReturnServerError logs the error and returns a server error.
func logErrorAndReturnServerError(ctx context.Context,
	logger *log.Logger,
	message string,
	err error,
	additionalFields ...log.Field,
) *tidcommon.ServiceError {
	fields := additionalFields
	if err != nil {
		fields = append(fields, log.Error(err))
	}
	logger.Error(ctx, message, fields...)
	return &tidcommon.InternalServerError
}

// mapEntityError maps entity service errors to user service errors.
// Returns nil if the error is not a recognized entity error.
func mapEntityError(err error) *tidcommon.ServiceError {
	switch {
	case errors.Is(err, entity.ErrEntityNotFound):
		return &ErrorUserNotFound
	case errors.Is(err, entity.ErrAuthenticationFailed):
		return &ErrorAuthenticationFailed
	case errors.Is(err, entity.ErrSchemaValidationFailed):
		return &ErrorSchemaValidationFailed
	case errors.Is(err, entity.ErrAttributeConflict):
		return &ErrorAttributeConflict
	case errors.Is(err, entity.ErrInvalidCredential):
		return &ErrorInvalidCredential
	default:
		return nil
	}
}

// mapOUServiceError converts organization unit service errors to user service errors.
func mapOUServiceError(ctx context.Context,
	svcErr *tidcommon.ServiceError,
	logger *log.Logger,
	context string,
	mappings map[string]*tidcommon.ServiceError,
	fields ...log.Field,
) *tidcommon.ServiceError {
	if svcErr == nil {
		return nil
	}

	if mappedErr, ok := mappings[svcErr.Code]; ok {
		return mappedErr
	}

	if svcErr.Type == tidcommon.ClientErrorType {
		logFields := append([]log.Field{}, fields...)
		logFields = append(logFields, log.Any("error", svcErr))
		logger.Error(ctx, fmt.Sprintf("Unexpected organization unit client error while %s", context),
			logFields...)
		return &tidcommon.InternalServerError
	}

	logFields := append([]log.Field{}, fields...)
	logFields = append(logFields, log.Any("error", svcErr))
	logger.Error(ctx, fmt.Sprintf("Organization unit service error while %s", context), logFields...)
	return &tidcommon.InternalServerError
}

// checkUserDeclarative checks if a user is declarative and returns an error if it is.
func (us *userService) checkUserDeclarative(
	ctx context.Context, userID string, logger *log.Logger,
) *tidcommon.ServiceError {
	isDeclarative, err := us.entityService.IsEntityDeclarative(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return &ErrorUserNotFound
		}
		logger.Error(ctx, "Failed to check if user is declarative",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if isDeclarative {
		return &ErrorCannotModifyDeclarativeResource
	}
	return nil
}

// checkUserAccess validates that the caller is authorized to perform the given action on a user.
func (us *userService) checkUserAccess(
	ctx context.Context, action security.Action, ouID string, resourceID string,
) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	allowed, svcErr := us.authzService.IsActionAllowed(ctx, action,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeUser, OUID: ouID, ResourceID: resourceID})
	if svcErr != nil {
		logger.Error(ctx, "Failed to check authorization for action",
			log.String("action", string(action)), log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}
	if !allowed {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
}

// buildTreePaginationLinks builds pagination links for user responses.
func buildTreePaginationLinks(handlePath string, limit, offset, totalResults int, displayQuery string) []utils.Link {
	treePath := fmt.Sprintf("/users/tree/%s", path.Clean(handlePath))
	return utils.BuildPaginationLinks(treePath, limit, offset, totalResults, displayQuery)
}

// ResolveUserOUHandle resolves ou_handle to an OU ID on the given user in-place.
// Called by the declarative loader parser so that file-based users support ou_handle.
// If both ou_id and ou_handle are provided, ou_id wins and a warning is logged.
func (us *userService) ResolveUserOUHandle(
	ctx context.Context, user *User,
) *tidcommon.ServiceError {
	if user.OUID != "" && user.OUHandle != "" {
		logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
		logger.Warn(ctx, "Both ouId and ouHandle provided for user; ouHandle ignored",
			log.MaskedString(log.LoggerKeyUserID, user.ID))
		return nil
	}
	if user.OUID == "" && user.OUHandle != "" {
		if us.ouService == nil {
			return &tidcommon.InternalServerError
		}
		ou, svcErr := us.ouService.GetOrganizationUnitByPath(
			security.WithRuntimeContext(ctx), user.OUHandle)
		if svcErr != nil {
			return &ErrorInvalidRequestFormat
		}
		user.OUID = ou.ID
	}
	return nil
}
