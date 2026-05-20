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

// Package user provides user management functionality.
package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const loggerComponentName = "UserService"

// UserServiceInterface defines the interface for the user service.
type UserServiceInterface interface {
	GetUserList(ctx context.Context, limit, offset int,
		filters map[string]interface{}, includeDisplay bool) (*UserListResponse, *serviceerror.ServiceError)
	GetUsersByPath(ctx context.Context, handlePath string, limit, offset int,
		filters map[string]interface{}, includeDisplay bool) (*UserListResponse, *serviceerror.ServiceError)
	CreateUser(ctx context.Context, user *User) (*User, *serviceerror.ServiceError)
	CreateUserByPath(ctx context.Context, handlePath string,
		request CreateUserByPathRequest) (*User, *serviceerror.ServiceError)
	GetUser(ctx context.Context, userID string, includeDisplay bool) (*User, *serviceerror.ServiceError)
	GetUserGroups(ctx context.Context, userID string,
		limit, offset int) (*UserGroupListResponse, *serviceerror.ServiceError)
	UpdateUser(ctx context.Context, userID string, user *User) (*User, *serviceerror.ServiceError)
	UpdateUserAttributes(ctx context.Context, userID string,
		attributes json.RawMessage) (*User, *serviceerror.ServiceError)
	UpdateUserCredentials(ctx context.Context, userID string,
		credentials json.RawMessage) *serviceerror.ServiceError
	DeleteUser(ctx context.Context, userID string) *serviceerror.ServiceError
}

// userService is the default implementation of the UserServiceInterface.
type userService struct {
	authzService      sysauthz.SystemAuthorizationServiceInterface
	entityService     entity.EntityServiceInterface
	ouService         oupkg.OrganizationUnitServiceInterface
	entityTypeService entitytype.EntityTypeServiceInterface
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
	}
}

// GetUserList retrieves a list of users with pagination and filtering.
func (us *userService) GetUserList(ctx context.Context, limit, offset int,
	filters map[string]interface{}, includeDisplay bool) (*UserListResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	// Resolve the set of organization units the caller is authorized to list users from.
	accessible, svcErr := us.authzService.GetAccessibleResources(
		ctx, security.ActionListUsers, security.ResourceTypeOU)
	if svcErr != nil {
		logger.Error("Failed to resolve accessible resources for listing users", log.Any("error", svcErr))
		return nil, &serviceerror.InternalServerError
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
) (*UserListResponse, *serviceerror.ServiceError) {
	totalCount, err := us.entityService.GetEntityListCount(ctx, entity.EntityCategoryUser, filters)
	if err != nil {
		return nil, logErrorAndReturnServerError(logger, "Failed to get user list count", err)
	}

	entities, err := us.entityService.GetEntityList(ctx, entity.EntityCategoryUser, limit, offset, filters)
	if err != nil {
		return nil, logErrorAndReturnServerError(logger, "Failed to get user list", err)
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
) (*UserListResponse, *serviceerror.ServiceError) {
	displayQuery := utils.DisplayQueryParam(includeDisplay)

	if len(ouIDs) == 0 {
		return buildUserListResponse([]User{}, 0, limit, offset, displayQuery), nil
	}

	totalCount, err := us.entityService.GetEntityListCountByOUIDs(ctx, entity.EntityCategoryUser, ouIDs, filters)
	if err != nil {
		return nil, logErrorAndReturnServerError(logger, "Failed to get user list count", err)
	}

	entities, err := us.entityService.GetEntityListByOUIDs(
		ctx, entity.EntityCategoryUser, ouIDs, limit, offset, filters)
	if err != nil {
		return nil, logErrorAndReturnServerError(logger, "Failed to get user list", err)
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
) (*UserListResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Getting users by path", log.String("path", handlePath))

	serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return nil, serviceError
	}

	ou, svcErr := us.ouService.GetOrganizationUnitByPath(ctx, handlePath)
	if svcErr != nil {
		return nil, mapOUServiceError(
			svcErr,
			logger,
			"resolving organization unit by path",
			map[string]*serviceerror.ServiceError{
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
		return nil, mapOUServiceError(
			svcErr,
			logger,
			"listing organization unit users",
			map[string]*serviceerror.ServiceError{
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
			logger.Warn("Failed to batch fetch users for display names, skipping display resolution", log.Error(err))
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
func (us *userService) CreateUser(ctx context.Context, user *User) (*User, *serviceerror.ServiceError) {
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
	user.ID, err = utils.GenerateUUIDv7()
	if err != nil {
		logger.Error("Failed to generate UUID", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	e := userToEntity(user)
	created, err := us.entityService.CreateEntity(ctx, e, nil)
	if err != nil {
		if svcErr := mapEntityError(err); svcErr != nil {
			return nil, svcErr
		}
		return nil, logErrorAndReturnServerError(logger, "Failed to create user", err)
	}

	// Sync cleaned attributes back — entity service removed credential fields from Attributes.
	user.Attributes = created.Attributes

	logger.Debug("Successfully created user", log.MaskedString(log.LoggerKeyUserID, user.ID))
	return user, nil
}

// CreateUserByPath creates a new user under the organization unit specified by the handle path.
func (us *userService) CreateUserByPath(
	ctx context.Context, handlePath string, request CreateUserByPathRequest,
) (*User, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Creating user by path", log.String("path", handlePath), log.String("type", request.Type))

	serviceError := validateAndProcessHandlePath(handlePath)
	if serviceError != nil {
		return nil, serviceError
	}

	ou, svcErr := us.ouService.GetOrganizationUnitByPath(ctx, handlePath)
	if svcErr != nil {
		return nil, mapOUServiceError(
			svcErr,
			logger,
			"resolving organization unit by path",
			map[string]*serviceerror.ServiceError{
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
) (*User, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Retrieving user", log.MaskedString(log.LoggerKeyUserID, userID))

	if userID == "" {
		return nil, &ErrorMissingUserID
	}

	e, err := us.entityService.GetEntity(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			logger.Debug("User not found", log.MaskedString(log.LoggerKeyUserID, userID))
			return nil, &ErrorUserNotFound
		}
		return nil, logErrorAndReturnServerError(logger, "Failed to retrieve user", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if e.Category != entity.EntityCategoryUser {
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
			logger.Warn("Failed to resolve OU handle for user, skipping",
				log.Any("error", svcErr))
		} else if handle, ok := handleMap[user.OUID]; ok {
			user.OUHandle = handle
		}
	}

	logger.Debug("Successfully retrieved user", log.MaskedString(log.LoggerKeyUserID, userID))
	return &user, nil
}

// GetUserGroups retrieves groups of a user with pagination.
func (as *userService) GetUserGroups(ctx context.Context, userID string, limit, offset int) (
	*UserGroupListResponse, *serviceerror.ServiceError) {
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
			logger.Debug("User not found", log.MaskedString(log.LoggerKeyUserID, userID))
			return nil, &ErrorUserNotFound
		}
		return nil, logErrorAndReturnServerError(logger, "Failed to retrieve user", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if userEntity.Category != entity.EntityCategoryUser {
		return nil, &ErrorUserNotFound
	}

	// Check authz using the user's OU ID.
	if svcErr := as.checkUserAccess(
		ctx, security.ActionReadUser, userEntity.OUID, userID); svcErr != nil {
		return nil, svcErr
	}

	totalCount, err := as.entityService.GetGroupCountForEntity(ctx, userID)
	if err != nil {
		logger.Error("Failed to get group count for user",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	entityGroups, err := as.entityService.GetEntityGroups(ctx, userID, limit, offset)
	if err != nil {
		logger.Error("Failed to get user groups", log.MaskedString(log.LoggerKeyUserID, userID), log.Error(err))
		return nil, &serviceerror.InternalServerError
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
	ctx context.Context, userID string, user *User) (*User, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Updating user", log.MaskedString(log.LoggerKeyUserID, userID))

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
			logger.Debug("User not found", log.MaskedString(log.LoggerKeyUserID, userID))
			return nil, &ErrorUserNotFound
		}
		return nil, logErrorAndReturnServerError(logger, "Failed to retrieve user", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if existingEntity.Category != entity.EntityCategoryUser {
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

	// Entity service handles schema validation, credential extraction from attributes,
	// hashing, merging with existing credentials, and entity update.
	e := userToEntity(user)
	e.SystemAttributes = existingEntity.SystemAttributes
	_, err = us.entityService.UpdateEntity(ctx, userID, e)
	if err != nil {
		if svcErr := mapEntityError(err); svcErr != nil {
			return nil, svcErr
		}
		return nil, logErrorAndReturnServerError(logger, "Failed to update user", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}

	logger.Debug("Successfully updated user", log.MaskedString(log.LoggerKeyUserID, userID))
	return user, nil
}

// UpdateUserAttributes updates only the attributes of a user while preserving immutable fields.
func (us *userService) UpdateUserAttributes(
	ctx context.Context, userID string, attributes json.RawMessage,
) (*User, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Updating user attributes", log.MaskedString(log.LoggerKeyUserID, userID))

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
			logger.Debug("User not found", log.MaskedString(log.LoggerKeyUserID, userID))
			return nil, &ErrorUserNotFound
		}
		return nil, logErrorAndReturnServerError(logger, "Failed to get user", getErr,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if existingEntity.Category != entity.EntityCategoryUser {
		return nil, &ErrorUserNotFound
	}
	existingUser := entityToUser(existingEntity)

	// Reject credential fields here: this endpoint is for attribute updates only.
	// Credentials must go through UpdateUserCredentials, which enforces its own authz and validation.
	if us.entityTypeService == nil {
		logger.Error("Entity type service is not configured for user operations")
		return nil, &serviceerror.InternalServerError
	}
	schemaCredentialInfos, svcErr := us.entityTypeService.GetAttributes(ctx,
		entitytype.TypeCategoryUser, existingUser.Type, true, false, false)
	if svcErr != nil {
		if svcErr.Code == entitytype.ErrorEntityTypeNotFound.Code {
			return nil, &ErrorEntityTypeNotFound
		}
		return nil, logErrorAndReturnServerError(logger, "Failed to get credential attributes from schema",
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
		return nil, logErrorAndReturnServerError(logger, "Failed to update user attributes", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}

	logger.Debug("Successfully updated user attributes", log.MaskedString(log.LoggerKeyUserID, userID))
	return &existingUser, nil
}

// UpdateUserCredentials updates schema-defined credentials for a user.
func (us *userService) UpdateUserCredentials(
	ctx context.Context,
	userID string,
	credentials json.RawMessage,
) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Updating user credentials", log.MaskedString(log.LoggerKeyUserID, userID))

	if strings.TrimSpace(userID) == "" {
		return &ErrorAuthenticationFailed
	}

	if len(credentials) == 0 {
		return &ErrorMissingCredentials
	}

	// Parse credentials to extract credential types
	var credentialsMap map[string]json.RawMessage
	if err := json.Unmarshal(credentials, &credentialsMap); err != nil {
		logger.Debug("Failed to parse credentials", log.Error(err))
		return &ErrorInvalidRequestFormat
	}

	if len(credentialsMap) == 0 {
		return &ErrorMissingCredentials
	}

	// Fetch user outside the transaction to resolve the OU ID for the authorization check.
	existingEntity, err := us.entityService.GetEntity(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			logger.Debug("User not found", log.MaskedString(log.LoggerKeyUserID, userID))
			return &ErrorUserNotFound
		}
		return logErrorAndReturnServerError(logger, "Failed to retrieve user", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if existingEntity.Category != entity.EntityCategoryUser {
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
		return logErrorAndReturnServerError(logger, "Failed to marshal credentials", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if err = us.entityService.UpdateCredentials(ctx, userID, plaintextJSON); err != nil {
		if svcErr := mapEntityError(err); svcErr != nil {
			return svcErr
		}
		return logErrorAndReturnServerError(logger, "Failed to update user credentials", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}

	logger.Debug("Successfully updated user credentials",
		log.MaskedString(log.LoggerKeyUserID, userID),
		log.Int("credentialTypesCount", len(credentialsMap)))
	return nil
}

// DeleteUser delete the user for given user id.
func (us *userService) DeleteUser(ctx context.Context, userID string) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug("Deleting user", log.MaskedString(log.LoggerKeyUserID, userID))

	if userID == "" {
		return &ErrorMissingUserID
	}

	// Fetch the user to resolve the OU ID for the authorization check.
	existingEntity, err := us.entityService.GetEntity(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			logger.Debug("User not found", log.MaskedString(log.LoggerKeyUserID, userID))
			return &ErrorUserNotFound
		}
		return logErrorAndReturnServerError(logger, "Failed to retrieve user", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}
	if existingEntity.Category != entity.EntityCategoryUser {
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

	err = us.entityService.DeleteEntity(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			logger.Debug("User not found", log.MaskedString(log.LoggerKeyUserID, userID))
			return &ErrorUserNotFound
		}
		return logErrorAndReturnServerError(logger, "Failed to delete user", err,
			log.MaskedString(log.LoggerKeyUserID, userID))
	}

	logger.Debug("Successfully deleted user", log.MaskedString(log.LoggerKeyUserID, userID))
	return nil
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
		logger.Warn("Failed to resolve OU handles, skipping", log.Any("error", svcErr))
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
) *serviceerror.ServiceError {
	if strings.TrimSpace(userType) == "" {
		return &ErrorEntityTypeNotFound
	}

	if strings.TrimSpace(oUID) == "" {
		return &ErrorInvalidOUID
	}

	if us.ouService == nil {
		logger.Error("Organization unit service is not configured for user operations")
		return &serviceerror.InternalServerError
	}

	exists, svcErr := us.ouService.IsOrganizationUnitExists(ctx, oUID)
	if svcErr != nil {
		return mapOUServiceError(
			svcErr,
			logger,
			"verifying organization unit existence",
			map[string]*serviceerror.ServiceError{
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
		logger.Error("Entity type service is not configured for user operations")
		return &serviceerror.InternalServerError
	}

	entityType, svcErr := us.entityTypeService.GetEntityTypeByName(ctx,
		entitytype.TypeCategoryUser, userType)
	if svcErr != nil {
		if svcErr.Code == entitytype.ErrorEntityTypeNotFound.Code {
			return &ErrorEntityTypeNotFound
		}
		logger.Error("Failed to retrieve user type",
			log.String("userType", userType), log.Any("error", svcErr))
		return &serviceerror.InternalServerError
	}

	if entityType == nil {
		logger.Error("Entity type service returned nil response", log.String("userType", userType))
		return &serviceerror.InternalServerError
	}

	if entityType.OUID == oUID {
		return nil
	}

	isParent, svcErr := us.ouService.IsParent(ctx, entityType.OUID, oUID)
	if svcErr != nil {
		return mapOUServiceError(
			svcErr,
			logger,
			"validating organization unit hierarchy",
			map[string]*serviceerror.ServiceError{
				oupkg.ErrorOrganizationUnitNotFound.Code: &ErrorOrganizationUnitNotFound,
			},
			log.String("userType", userType),
			log.String("oUID", oUID),
			log.String("schemaOUID", entityType.OUID),
		)
	}

	if !isParent {
		logger.Debug("Organization unit mismatch for user type",
			log.String("userType", userType),
			log.String("oUID", oUID),
			log.String("schemaOUID", entityType.OUID))
		return &ErrorOrganizationUnitMismatch
	}

	return nil
}

// validateAndProcessHandlePath validates and processes the handle path.
func validateAndProcessHandlePath(handlePath string) *serviceerror.ServiceError {
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
func validatePaginationParams(limit, offset int) *serviceerror.ServiceError {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return &ErrorInvalidLimit
	}
	if offset < 0 {
		return &ErrorInvalidOffset
	}
	return nil
}

// logErrorAndReturnServerError logs the error and returns a server error.
func logErrorAndReturnServerError(
	logger *log.Logger,
	message string,
	err error,
	additionalFields ...log.Field,
) *serviceerror.ServiceError {
	fields := additionalFields
	if err != nil {
		fields = append(fields, log.Error(err))
	}
	logger.Error(message, fields...)
	return &serviceerror.InternalServerError
}

// mapEntityError maps entity service errors to user service errors.
// Returns nil if the error is not a recognized entity error.
func mapEntityError(err error) *serviceerror.ServiceError {
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
func mapOUServiceError(
	svcErr *serviceerror.ServiceError,
	logger *log.Logger,
	context string,
	mappings map[string]*serviceerror.ServiceError,
	fields ...log.Field,
) *serviceerror.ServiceError {
	if svcErr == nil {
		return nil
	}

	if mappedErr, ok := mappings[svcErr.Code]; ok {
		return mappedErr
	}

	if svcErr.Type == serviceerror.ClientErrorType {
		logFields := append([]log.Field{}, fields...)
		logFields = append(logFields, log.Any("error", svcErr))
		logger.Error(fmt.Sprintf("Unexpected organization unit client error while %s", context), logFields...)
		return &serviceerror.InternalServerError
	}

	logFields := append([]log.Field{}, fields...)
	logFields = append(logFields, log.Any("error", svcErr))
	logger.Error(fmt.Sprintf("Organization unit service error while %s", context), logFields...)
	return &serviceerror.InternalServerError
}

// checkUserDeclarative checks if a user is declarative and returns an error if it is.
func (us *userService) checkUserDeclarative(
	ctx context.Context, userID string, logger *log.Logger,
) *serviceerror.ServiceError {
	isDeclarative, err := us.entityService.IsEntityDeclarative(ctx, userID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return &ErrorUserNotFound
		}
		logger.Error("Failed to check if user is declarative",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Error(err))
		return &serviceerror.InternalServerError
	}
	if isDeclarative {
		return &ErrorCannotModifyDeclarativeResource
	}
	return nil
}

// checkUserAccess validates that the caller is authorized to perform the given action on a user.
func (us *userService) checkUserAccess(
	ctx context.Context, action security.Action, ouID string, resourceID string,
) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	allowed, svcErr := us.authzService.IsActionAllowed(ctx, action,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeUser, OUID: ouID, ResourceID: resourceID})
	if svcErr != nil {
		logger.Error("Failed to check authorization for action",
			log.String("action", string(action)), log.Any("error", svcErr))
		return &serviceerror.InternalServerError
	}
	if !allowed {
		return &serviceerror.ErrorUnauthorized
	}
	return nil
}

// buildTreePaginationLinks builds pagination links for user responses.
func buildTreePaginationLinks(handlePath string, limit, offset, totalResults int, displayQuery string) []utils.Link {
	treePath := fmt.Sprintf("/users/tree/%s", path.Clean(handlePath))
	return utils.BuildPaginationLinks(treePath, limit, offset, totalResults, displayQuery)
}
