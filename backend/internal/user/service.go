// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package user provides user management functionality.
package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"slices"
	"sort"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/attributeverification"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/sharing"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/projectscope"
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
	GetUserListByProject(ctx context.Context, projectID string, limit, offset int,
		filters map[string]interface{}, includeDisplay bool) (*UserListResponse, *tidcommon.ServiceError)
	GetUsersByPath(ctx context.Context, handlePath string, limit, offset int,
		filters map[string]interface{}, includeDisplay bool) (*UserListResponse, *tidcommon.ServiceError)
	CreateUser(ctx context.Context, user *providers.User) (*providers.User, *tidcommon.ServiceError)
	CreateUserByPath(ctx context.Context, handlePath string,
		request CreateUserByPathRequest) (*providers.User, *tidcommon.ServiceError)
	GetUser(ctx context.Context, userID string, includeDisplay bool) (*providers.User, *tidcommon.ServiceError)
	GetUserInOU(ctx context.Context, userID, ouID string,
		includeDisplay bool) (*providers.User, *tidcommon.ServiceError)
	GetUserGroups(ctx context.Context, userID string,
		limit, offset int) (*UserGroupListResponse, *tidcommon.ServiceError)
	UpdateUser(ctx context.Context, userID string, user *providers.User) (*providers.User, *tidcommon.ServiceError)
	UpdateUserAttributes(ctx context.Context, userID string,
		attributes json.RawMessage) (*providers.User, *tidcommon.ServiceError)
	GetUserMetadata(ctx context.Context, userID string) (*entitytype.EntityType, *tidcommon.ServiceError)
	UpdateUserCredentials(ctx context.Context, userID string,
		credentials json.RawMessage) *tidcommon.ServiceError
	DeleteUser(ctx context.Context, userID string) *tidcommon.ServiceError
	ResolveUserOUHandle(ctx context.Context, user *providers.User) *tidcommon.ServiceError
	SetDependencyRegistry(r resourcedependency.Registry)
	GetUserUsages(ctx context.Context, userID string) (
		*resourcedependency.DependenciesResponse, *tidcommon.ServiceError)
	CreateSharingPolicy(ctx context.Context, userID string,
		req sharing.PolicyRequest) (sharing.Policy, *tidcommon.ServiceError)
	GetSharingPolicy(ctx context.Context, userID, policyID string) (sharing.Policy, *tidcommon.ServiceError)
	ListSharingPolicies(ctx context.Context, userID string,
		limit, offset int) (sharing.PolicyList, *tidcommon.ServiceError)
	UpdateSharingPolicy(ctx context.Context, userID, policyID string,
		req sharing.PolicyRequest) (sharing.Policy, *tidcommon.ServiceError)
	DeleteSharingPolicy(ctx context.Context, userID, policyID string) *tidcommon.ServiceError
	ResolveSharingOverlay(ctx context.Context, userID, ouID string) (sharing.ResolvedOverlay, *tidcommon.ServiceError)
	GetAccessibleOrganizationUnits(ctx context.Context, scope string,
		limit, offset int) (*AccessibleOrganizationUnitListResponse, *tidcommon.ServiceError)
	GetUserOrganizationUnits(ctx context.Context, userID string,
		limit, offset int) (*UserOrganizationUnitListResponse, *tidcommon.ServiceError)
}

// ConfigurableUserService extends UserServiceInterface with the role assignment service injected at startup. It is
// separate from the main interface so consumers don't see the startup-only method.
type ConfigurableUserService interface {
	UserServiceInterface
	// SetRoleAssignmentService injects the role service user shares mandate through, applying the declared mandates.
	SetRoleAssignmentService(ctx context.Context, roleAssignments role.RoleAssignmentServiceInterface) error
}

// projectOUValidator checks that the organization unit of a resource scoped to a project belongs to it.
type projectOUValidator interface {
	ValidateProjectOU(ctx context.Context, projectID, ouID string) *tidcommon.ServiceError
}

// userService is the default implementation of the UserServiceInterface.
type userService struct {
	authzService       sysauthz.SystemAuthorizationServiceInterface
	entityService      entity.EntityServiceInterface
	governance         identitygovernance.IdentityGovernanceServiceInterface
	verification       attributeverification.AttributeVerificationServiceInterface
	ouService          oupkg.OrganizationUnitServiceInterface
	entityTypeService  entitytype.EntityTypeServiceInterface
	uuidGenerator      func() (string, error)
	dependencyRegistry resourcedependency.Registry
	valueCapturer      declarativeresource.ValueCapturer
	sharingService     sharing.SharingServiceInterface
	sharingDeclaration *userSharingDeclaration
	logger             *log.Logger
	bus                eventbus.EventBusInterface
	// projectValidator is nil on a deployment that serves no projects.
	projectValidator projectOUValidator
	// managed says which users a control plane deployed, which stay read only here, credentials included.
	// Nil where nothing is deployed from one.
	managed managedresource.StoreInterface
}

// newUserService creates a new instance of userService with injected dependencies.
func newUserService(
	authzService sysauthz.SystemAuthorizationServiceInterface,
	entityService entity.EntityServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	governance identitygovernance.IdentityGovernanceServiceInterface,
	verification attributeverification.AttributeVerificationServiceInterface,
	valueCapturer declarativeresource.ValueCapturer,
	sharingService sharing.SharingServiceInterface,
	bus eventbus.EventBusInterface,
	projectValidator projectOUValidator,
) *userService {
	us := &userService{
		authzService:      authzService,
		entityService:     entityService,
		governance:        governance,
		verification:      verification,
		ouService:         ouService,
		entityTypeService: entityTypeService,
		uuidGenerator:     utils.GenerateUUIDv7,
		valueCapturer:     valueCapturer,
		sharingService:    sharingService,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
		bus:               bus,
		projectValidator:  projectValidator,
	}
	us.sharingDeclaration = newUserSharingDeclaration(us, sharingService, us.authorizeShareInto)
	return us
}

// GetUserList retrieves a list of users with pagination and filtering. An organization unit filter
// lists the users that unit owns, followed by the ones shared into it.
func (us *userService) GetUserList(ctx context.Context, limit, offset int,
	filters map[string]interface{}, includeDisplay bool) (*UserListResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	listQuery := utils.FilterQueryParam(filters) + utils.DisplayQueryParam(includeDisplay)
	ouID, filters, err := filter.SplitOUIDFilter(filters)
	if err != nil {
		return nil, &ErrorInvalidFilter
	}
	// Other conditions cannot narrow the users shared into an organization unit yet, so they are
	// refused alongside one rather than applied to the owned users only.
	if ouID != "" && len(filters) > 0 {
		return nil, &ErrorInvalidFilter
	}

	// Resolve the set of organization units the caller is authorized to list users from.
	accessible, svcErr := us.authzService.GetAccessibleResources(
		ctx, security.ActionListUsers, security.ResourceTypeOU)
	if svcErr != nil {
		logger.Error(ctx, "Failed to resolve accessible resources for listing users",
			log.Any("error", svcErr))
		return nil, &tidcommon.InternalServerError
	}

	if ouID != "" {
		// An organization unit the caller cannot list users from has none to show.
		if !accessible.AllAllowed && !slices.Contains(accessible.IDs, ouID) {
			return buildUserListResponse([]providers.User{}, 0, limit, offset, listQuery), nil
		}
		users, totalCount, listErr := us.listUsersInOU(ctx, ouID, limit, offset, filters, includeDisplay, logger)
		if listErr != nil {
			return nil, listErr
		}
		us.markManaged(ctx, users, logger)
		return buildUserListResponse(users, totalCount, limit, offset, listQuery), nil
	}

	// Unfiltered path: system-level caller — return all users.
	if accessible.AllAllowed {
		return us.listAllUsers(ctx, limit, offset, filters, includeDisplay, listQuery, logger)
	}

	// A caller that may list one organization unit lists what that unit sees: the users it owns, then the ones
	// shared into it. Other conditions cannot match shared users yet, so a list they narrow keeps to owned users.
	if len(accessible.IDs) == 1 && len(filters) == 0 {
		users, totalCount, listErr := us.listUsersInOU(ctx, accessible.IDs[0], limit, offset, filters,
			includeDisplay, logger)
		if listErr != nil {
			return nil, listErr
		}
		return buildUserListResponse(users, totalCount, limit, offset, listQuery), nil
	}

	// Filtered path: return users belonging to the accessible OUs.
	return us.listUsersByOUIDs(ctx, accessible.IDs, limit, offset, filters, includeDisplay, listQuery, logger)
}

// GetUserListByProject returns a paginated list of the users of a project, narrowed to those an
// organization unit owns when the filter names one. A project that does not exist has none. A caller
// limited to some organization units lists only the project's users in those units.
func (us *userService) GetUserListByProject(ctx context.Context, projectID string, limit, offset int,
	filters map[string]interface{}, includeDisplay bool) (*UserListResponse, *tidcommon.ServiceError) {
	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	listQuery := "&" + queryParamProjectID + "=" + url.QueryEscape(projectID) +
		utils.FilterQueryParam(filters) + utils.DisplayQueryParam(includeDisplay)
	ouID, rest, err := filter.SplitOUIDFilter(filters)
	if err != nil || len(rest) > 0 {
		return nil, &ErrorInvalidFilter
	}
	empty := buildUserListResponse([]providers.User{}, 0, limit, offset, listQuery)
	if projectID == "" {
		return empty, nil
	}

	accessible, svcErr := us.authzService.GetAccessibleResources(
		ctx, security.ActionListUsers, security.ResourceTypeOU)
	if svcErr != nil {
		us.logger.Error(ctx, "Failed to resolve accessible resources for listing users",
			log.Any("error", svcErr))
		return nil, &tidcommon.InternalServerError
	}
	var ouIDs []string
	switch {
	case ouID != "" && !accessible.AllAllowed && !slices.Contains(accessible.IDs, ouID):
		return empty, nil
	case ouID != "":
		ouIDs = []string{ouID}
	case !accessible.AllAllowed:
		if len(accessible.IDs) == 0 {
			return empty, nil
		}
		ouIDs = append([]string{}, accessible.IDs...)
	}

	totalCount, err := us.entityService.GetEntityListCountByProject(
		ctx, providers.EntityCategoryUser, projectID, ouIDs)
	if err != nil {
		return nil, logErrorAndReturnServerError(ctx, us.logger, "Failed to count project users", err,
			log.String("projectID", projectID))
	}
	entities, err := us.entityService.GetEntityListByProject(
		ctx, providers.EntityCategoryUser, projectID, ouIDs, limit, offset)
	if err != nil {
		return nil, logErrorAndReturnServerError(ctx, us.logger, "Failed to list project users", err,
			log.String("projectID", projectID))
	}

	users := us.usersWithAccess(ctx, entities)
	markOwned(users, true)
	if includeDisplay {
		us.populateUserDisplayNames(ctx, users, us.logger)
		us.populateOUHandles(ctx, users, us.logger)
	}
	return buildUserListResponse(users, totalCount, limit, offset, listQuery), nil
}

// listAllUsers retrieves users without OU filtering.
func (us *userService) listAllUsers(
	ctx context.Context, limit, offset int, filters map[string]interface{},
	includeDisplay bool, listQuery string, logger *log.Logger,
) (*UserListResponse, *tidcommon.ServiceError) {
	totalCount, err := us.entityService.GetEntityListCount(ctx, providers.EntityCategoryUser, filters)
	if err != nil {
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to get user list count", err)
	}

	entities, err := us.entityService.GetEntityList(ctx, providers.EntityCategoryUser, limit, offset, filters)
	if err != nil {
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to get user list", err)
	}

	users := us.usersWithAccess(ctx, entities)
	markOwned(users, true)
	if includeDisplay {
		us.populateUserDisplayNames(ctx, users, logger)
		us.populateOUHandles(ctx, users, logger)
	}

	us.markManaged(ctx, users, logger)
	return buildUserListResponse(users, totalCount, limit, offset, listQuery), nil
}

// listUsersByOUIDs retrieves users scoped to the given organization unit IDs.
func (us *userService) listUsersByOUIDs(
	ctx context.Context, ouIDs []string, limit, offset int, filters map[string]interface{},
	includeDisplay bool, listQuery string, logger *log.Logger,
) (*UserListResponse, *tidcommon.ServiceError) {
	if len(ouIDs) == 0 {
		return buildUserListResponse([]providers.User{}, 0, limit, offset, listQuery), nil
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

	users := us.usersWithAccess(ctx, entities)
	markOwned(users, true)
	if includeDisplay {
		us.populateUserDisplayNames(ctx, users, logger)
		us.populateOUHandles(ctx, users, logger)
	}

	us.markManaged(ctx, users, logger)
	return buildUserListResponse(users, totalCount, limit, offset, listQuery), nil
}

// listUsersInOU lists the users an organization unit owns, followed by the ones shared into it, and
// returns the total across every page.
func (us *userService) listUsersInOU(
	ctx context.Context, ouID string, limit, offset int, filters map[string]interface{},
	includeDisplay bool, logger *log.Logger,
) ([]providers.User, int, *tidcommon.ServiceError) {
	page, err := us.entityService.GetEntityPageInOU(
		ctx, providers.EntityCategoryUser, ouID, entity.OUEntityScopeAll, limit, offset, filters)
	if err != nil {
		return nil, 0, logErrorAndReturnServerError(ctx, logger, "Failed to list users in organization unit", err)
	}

	users := us.usersWithAccess(ctx, slices.Concat(page.Owned, page.Shared))
	markOwned(users[:len(page.Owned)], true)
	markOwned(users[len(page.Owned):], false)
	// A shared user is listed under the organization unit it is shared into, not the one owning it.
	for i := len(page.Owned); i < len(users); i++ {
		users[i].OUID = ouID
	}
	if includeDisplay {
		us.populateUserDisplayNames(ctx, users, logger)
		us.populateOUHandles(ctx, users, logger)
	}
	return users, page.TotalResults, nil
}

// buildUserListResponse constructs a paginated UserListResponse.
func buildUserListResponse(
	users []providers.User, totalCount, limit, offset int, listQuery string,
) *UserListResponse {
	return &UserListResponse{
		TotalResults: totalCount,
		StartIndex:   offset + 1,
		Count:        len(users),
		Users:        users,
		Links:        utils.BuildPaginationLinks("/users", limit, offset, totalCount, listQuery),
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
	// The path names the organization unit, and a filter cannot narrow the users shared into it yet,
	// so a filter is refused rather than applied to the owned users only.
	if len(filters) > 0 {
		return nil, &ErrorInvalidFilter
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

	users, totalCount, svcErr := us.listUsersInOU(ctx, oUID, limit, offset, nil, includeDisplay, logger)
	if svcErr != nil {
		return nil, svcErr
	}

	us.markManaged(ctx, users, logger)
	return &UserListResponse{
		TotalResults: totalCount,
		StartIndex:   offset + 1,
		Count:        len(users),
		Users:        users,
		Links: buildTreePaginationLinks(
			handlePath, limit, offset, totalCount, utils.DisplayQueryParam(includeDisplay)),
	}, nil
}

// CreateUser creates the user.
func (us *userService) CreateUser(
	ctx context.Context, user *providers.User,
) (*providers.User, *tidcommon.ServiceError) {
	created, svcErr := us.createUser(ctx, user)
	return us.publishCreate(ctx, created, svcErr)
}

// createUser creates the user without publishing its audit event.
func (us *userService) createUser(
	ctx context.Context, user *providers.User,
) (*providers.User, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if user == nil {
		return nil, &ErrorInvalidRequestFormat
	}
	if user.LinkedAccount != nil && (user.LinkedAccount.IdpID == "" || user.LinkedAccount.Sub == "") {
		return nil, &ErrorInvalidRequestFormat
	}

	// A user in a project that names no organization unit goes to the system organization unit.
	if user.OUID == "" && user.ProjectID != "" {
		systemOUID, svcErr := us.ouService.GetSystemOUID(ctx)
		if svcErr != nil {
			return nil, svcErr
		}
		user.OUID = systemOUID
	}

	// Check if caller is authorized to create users in the target OU.
	if svcErr := us.checkUserAccess(ctx, security.ActionCreateUser, user.OUID, ""); svcErr != nil {
		return nil, svcErr
	}

	if svcErr := us.validateOrganizationUnitForUserType(ctx, user.Type, user.OUID, logger); svcErr != nil {
		return nil, svcErr
	}

	// A project checks that the user's organization unit belongs to it. A user in no project belongs to
	// the organization.
	if user.ProjectID != "" {
		if svcErr := us.validateProjectOU(ctx, user.ProjectID, user.OUID); svcErr != nil {
			return nil, svcErr
		}
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
	submitted := user.Attributes
	created, err := us.entityService.CreateEntity(ctx, e, nil, user.LinkedAccount)
	if err != nil {
		if svcErr := mapEntityError(err); svcErr != nil {
			return nil, svcErr
		}
		return nil, logErrorAndReturnServerError(ctx, logger, "Failed to create user", err)
	}

	// Sync cleaned attributes back — entity service removed credential fields from Attributes.
	user.Attributes = created.Attributes
	// A new user has no verified attributes: the entity service drops them on create.
	user.VerifiedAttributes = map[string]providers.VerifiedAttribute{}
	us.captureSubmittedCredentials(ctx, user, submitted)

	logger.Debug(ctx, "Successfully created user", log.MaskedString(log.LoggerKeyUserID, user.ID))
	return user, nil
}

// CreateUserByPath creates a new user under the organization unit specified by the handle path.
func (us *userService) CreateUserByPath(
	ctx context.Context, handlePath string, request CreateUserByPathRequest,
) (*providers.User, *tidcommon.ServiceError) {
	created, svcErr := us.createUserByPath(ctx, handlePath, request)
	return us.publishCreate(ctx, created, svcErr)
}

// createUserByPath creates a new user under the organization unit of the handle path without publishing
// its audit event.
func (us *userService) createUserByPath(
	ctx context.Context, handlePath string, request CreateUserByPathRequest,
) (*providers.User, *tidcommon.ServiceError) {
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

	user := &providers.User{
		OUID:       ou.ID,
		Type:       request.Type,
		Attributes: request.Attributes,
	}

	return us.createUser(ctx, user)
}

// GetUser retrieves a user by ID.
func (us *userService) GetUser(
	ctx context.Context, userID string, includeDisplay bool,
) (*providers.User, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Retrieving user", log.MaskedString(log.LoggerKeyUserID, userID))

	if userID == "" {
		return nil, &ErrorMissingUserID
	}

	e, svcErr := us.getUserEntity(ctx, userID, logger)
	if svcErr != nil {
		return nil, svcErr
	}
	user := entityToUser(e)

	// Check authz using the user's OU ID (fetched from store).
	if svcErr := us.checkUserAccess(ctx, security.ActionReadUser, user.OUID, userID); svcErr != nil {
		return us.getSharedUser(ctx, userID, user.OUID, includeDisplay, svcErr)
	}
	// Read in its owning organization unit, as a shared user is read in the one it is shared into.
	user.IsOwnedByOU = new(true)
	us.completeUserRead(ctx, e, &user, includeDisplay, logger)
	if !user.IsReadOnly {
		deployed, err := managedresource.IsDeployed(ctx, us.managed, user.ID, managedresource.TypeUser)
		if err != nil {
			// Every write to such a user is refused whatever the read says, so it is served without the flag.
			logger.Error(ctx, "Failed to read whether a control plane deployed the user",
				log.MaskedString(log.LoggerKeyUserID, user.ID), log.Error(err))
		}
		user.IsReadOnly = deployed
	}

	logger.Debug(ctx, "Successfully retrieved user", log.MaskedString(log.LoggerKeyUserID, userID))
	return &user, nil
}

// getSharedUser answers a read refused in the user's owning organization unit with the user as the organization
// unit the caller is confined to sees it, when the user is shared there, and with the refusal otherwise.
func (us *userService) getSharedUser(ctx context.Context, userID, ownerOUID string, includeDisplay bool,
	refusal *tidcommon.ServiceError) (*providers.User, *tidcommon.ServiceError) {
	ouID := sysauthz.SharedAccessOU(ctx, us.authzService, ownerOUID, refusal)
	if ouID == "" {
		return nil, refusal
	}
	user, svcErr := us.GetUserInOU(ctx, userID, ouID, includeDisplay)
	if svcErr != nil && svcErr.Code == ErrorUserNotFound.Code {
		return nil, refusal
	}
	return user, svcErr
}

// sharedUserRefusal answers a write refused in the user's owning organization unit with ErrorSharedUserReadOnly
// when the user is shared into the organization unit the caller is confined to, and with the refusal otherwise.
func (us *userService) sharedUserRefusal(ctx context.Context, userID, ownerOUID string,
	refusal *tidcommon.ServiceError) *tidcommon.ServiceError {
	ouID, svcErr := sharing.SharedIntoCallerOU(ctx, us.authzService, us.sharingService, us.logger,
		UserSharingType, userID, ownerOUID, refusal)
	if svcErr != nil {
		return svcErr
	}
	if ouID != "" {
		return &ErrorSharedUserReadOnly
	}
	return refusal
}

// GetUserInOU retrieves a user as an organization unit sees it: one the unit owns, or one shared into it.
func (us *userService) GetUserInOU(
	ctx context.Context, userID, ouID string, includeDisplay bool,
) (*providers.User, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Retrieving user in organization unit", log.MaskedString(log.LoggerKeyUserID, userID),
		log.String("ouID", ouID))

	if userID == "" {
		return nil, &ErrorMissingUserID
	}
	// Names no user, so the self-service rule does not stand in for access to the organization unit.
	if svcErr := us.checkUserAccess(ctx, security.ActionReadUser, ouID, ""); svcErr != nil {
		return nil, svcErr
	}

	e, svcErr := us.getUserEntity(ctx, userID, logger)
	if svcErr != nil {
		return nil, svcErr
	}
	owned := e.OUID == ouID
	if !owned {
		// Asked as the runtime, as the caller may not read the organization units above its own.
		visible, svcErr := us.sharingService.IsVisible(
			security.WithRuntimeContext(ctx), UserSharingType, userID, ouID)
		if svcErr != nil {
			if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
				return nil, &ErrorUserNotFound
			}
			logger.Error(ctx, "Failed to check whether the user is shared into the organization unit",
				log.MaskedString(log.LoggerKeyUserID, userID), log.String("ouID", ouID), log.Any("error", svcErr))
			return nil, &tidcommon.InternalServerError
		}
		if !visible {
			return nil, &ErrorUserNotFound
		}
	}

	user := entityToUser(e)
	user.IsOwnedByOU = new(owned)
	// A shared user is read under the organization unit it is shared into, as the user list does.
	user.OUID = ouID
	us.completeUserRead(ctx, e, &user, includeDisplay, logger)
	if !user.IsReadOnly {
		deployed, err := managedresource.IsDeployed(ctx, us.managed, user.ID, managedresource.TypeUser)
		if err != nil {
			// Every write to such a user is refused whatever the read says, so it is served without the flag.
			logger.Error(ctx, "Failed to read whether a control plane deployed the user",
				log.MaskedString(log.LoggerKeyUserID, user.ID), log.Error(err))
		}
		user.IsReadOnly = deployed
	}

	logger.Debug(ctx, "Successfully retrieved user in organization unit",
		log.MaskedString(log.LoggerKeyUserID, userID), log.String("ouID", ouID))
	return &user, nil
}

// getUserEntity reads the entity behind a user ID, answering ErrorUserNotFound for any other category.
func (us *userService) getUserEntity(
	ctx context.Context, userID string, logger *log.Logger,
) (*providers.Entity, *tidcommon.ServiceError) {
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
	return e, nil
}

// completeUserRead adds the access status a read reports and, with includeDisplay, the display name
// and organization unit handle.
func (us *userService) completeUserRead(
	ctx context.Context, e *providers.Entity, user *providers.User, includeDisplay bool, logger *log.Logger,
) {
	access := us.governance.ReadEntityAccess(ctx, *e)
	user.Status = &access.Status
	user.LastLoginAt = access.LastLoginAt
	user.VerifiedAttributes = us.verification.GetVerifiedAttributes(ctx, *e)

	if includeDisplay {
		displayAttrPaths := entitytype.ResolveDisplayAttributePaths(
			ctx, entitytype.TypeCategoryUser, []string{user.Type}, us.entityTypeService, logger)
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
}

// GetUserMetadata retrieves the schema metadata for the authenticated user's type.
func (us *userService) GetUserMetadata(
	ctx context.Context, userID string,
) (*entitytype.EntityType, *tidcommon.ServiceError) {
	user, svcErr := us.GetUser(ctx, userID, false)
	if svcErr != nil {
		return nil, svcErr
	}

	schema, svcErr := us.entityTypeService.GetEntityTypeSchema(ctx, entitytype.TypeCategoryUser, user.Type)
	if svcErr != nil {
		return nil, svcErr
	}

	return schema, nil
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
	userEntity, err := as.entityService.GetEntityProfile(ctx, userID)
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
	ctx context.Context, userID string, user *providers.User) (*providers.User, *tidcommon.ServiceError) {
	updated, svcErr := us.updateUser(ctx, userID, user)
	us.publishUserEvent(ctx, audit.EventUserUpdate, audit.ActionUpdate, userID, svcErr)
	return updated, svcErr
}

// updateUser updates the user for the given user id without publishing its audit event.
func (us *userService) updateUser(
	ctx context.Context, userID string, user *providers.User) (*providers.User, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Updating user", log.MaskedString(log.LoggerKeyUserID, userID))

	if userID == "" {
		return nil, &ErrorMissingUserID
	}

	if user == nil {
		return nil, &ErrorInvalidRequestFormat
	}

	// Fetch the existing user to obtain its OU ID for the authorization check.
	existingEntity, err := us.entityService.GetEntityProfile(ctx, userID)
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
		return nil, us.sharedUserRefusal(ctx, userID, existingUser.OUID, svcErr)
	}
	// Attributes such as an email or phone number lead to account recovery, so changing them is
	// control over the user.
	if svcErr := us.authzService.CanControlEntity(ctx, userID); svcErr != nil {
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
	if svcErr := us.checkUserMutable(ctx, userID, logger); svcErr != nil {
		return nil, svcErr
	}

	// An update that names no project keeps the user in its project.
	if user.ProjectID == "" {
		user.ProjectID = existingUser.ProjectID
	}

	// A shared user stays in its organization unit and project: its policies were issued against them.
	if user.OUID != existingUser.OUID || user.ProjectID != existingUser.ProjectID {
		policies, svcErr := us.sharingService.ListPolicies(ctx, UserSharingType, userID)
		if svcErr != nil {
			return nil, translateSharingError(svcErr)
		}
		if len(policies) > 0 {
			return nil, &ErrorSharedUserCannotMove
		}
	}

	// Ensure the user object has the correct ID
	user.ID = userID

	if svcErr := us.validateOrganizationUnitForUserType(
		ctx, user.Type, user.OUID, logger,
	); svcErr != nil {
		return nil, svcErr
	}

	// A project checks that the user's organization unit belongs to it. It is checked only when the update
	// changes the project or the organization unit, so a user whose project is gone can still be edited.
	if user.ProjectID != "" && (user.ProjectID != existingUser.ProjectID || user.OUID != existingUser.OUID) {
		if svcErr := us.validateProjectOU(ctx, user.ProjectID, user.OUID); svcErr != nil {
			return nil, svcErr
		}
	}

	// Reject credential fields: this endpoint is for attribute and user metadata updates only.
	// Credentials must go through the dedicated update-credentials endpoint.
	if len(user.Attributes) > 0 {
		schemaCredentialInfos, svcErr := us.entityTypeService.GetAttributes(ctx,
			entitytype.TypeCategoryUser, user.Type, entitytype.AttributeFilter{AllowCredential: true})
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
	// The status is derived from the updated entity, never taken from the request.
	access := us.governance.ReadEntityAccess(ctx, *updated)
	user.Status = &access.Status
	user.LastLoginAt = access.LastLoginAt
	user.VerifiedAttributes = us.verification.GetVerifiedAttributes(ctx, *updated)
	logger.Debug(ctx, "Successfully updated user", log.MaskedString(log.LoggerKeyUserID, userID))
	return user, nil
}

// UpdateUserAttributes updates only the attributes of a user while preserving immutable fields.
func (us *userService) UpdateUserAttributes(
	ctx context.Context, userID string, attributes json.RawMessage,
) (*providers.User, *tidcommon.ServiceError) {
	updated, svcErr := us.updateUserAttributes(ctx, userID, attributes)
	us.publishUserEvent(ctx, audit.EventUserUpdate, audit.ActionUpdate, userID, svcErr)
	return updated, svcErr
}

// updateUserAttributes updates only the attributes of a user without publishing its audit event.
func (us *userService) updateUserAttributes(
	ctx context.Context, userID string, attributes json.RawMessage,
) (*providers.User, *tidcommon.ServiceError) {
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
		entitytype.TypeCategoryUser, existingUser.Type, entitytype.AttributeFilter{AllowCredential: true})
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
		return nil, us.sharedUserRefusal(ctx, userID, existingUser.OUID, svcErr)
	}
	if svcErr := us.authzService.CanControlEntity(ctx, userID); svcErr != nil {
		return nil, svcErr
	}

	// Check if user is declarative (immutable)
	if svcErr := us.checkUserMutable(ctx, userID, logger); svcErr != nil {
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

	// Return attributes and verification from the same stored snapshot.
	if stored, err := us.entityService.GetEntity(ctx, userID); err == nil {
		existingEntity = stored
		existingUser = entityToUser(stored)
		existingUser.VerifiedAttributes = us.verification.GetVerifiedAttributes(ctx, *stored)
	} else {
		logger.Warn(ctx, "Failed to read the updated user",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Error(err))
	}
	access := us.governance.ReadEntityAccess(ctx, *existingEntity)
	existingUser.Status = &access.Status
	existingUser.LastLoginAt = access.LastLoginAt

	logger.Debug(ctx, "Successfully updated user attributes", log.MaskedString(log.LoggerKeyUserID, userID))
	return &existingUser, nil
}

// UpdateUserCredentials updates schema-defined credentials for a user.
func (us *userService) UpdateUserCredentials(
	ctx context.Context,
	userID string,
	credentials json.RawMessage,
) *tidcommon.ServiceError {
	var written bool
	svcErr := us.updateUserCredentials(ctx, userID, credentials, &written)
	auditErr := svcErr
	if written {
		auditErr = nil
	}
	us.publishCredentialUpdate(ctx, userID, auditErr)
	return svcErr
}

// publishCredentialUpdate publishes the audit event of an attempt to set a user's credentials. A flow on the
// server's authority, such as a password reset through the recovery flow, runs with the subject of the
// request it came in on, which a public path lets anyone's bearer token be. Its actor is therefore set
// explicitly: the user whose credentials were set, or anonymous when they were not.
func (us *userService) publishCredentialUpdate(ctx context.Context, userID string, svcErr *tidcommon.ServiceError) {
	target := audit.Ref{Type: audit.TargetTypeUser, ID: userID}
	payload := audit.ChangePayload(ctx, target, audit.ActionUpdate, svcErr)
	if security.IsRuntimeContext(ctx) {
		actor := audit.Actor{Type: audit.ActorTypeAnonymous}
		if svcErr == nil {
			actor = audit.Actor{Type: audit.ActorTypeUser, ID: userID}
		}
		payload = payload.WithActor(actor)
	}
	eventbus.TryPublish(ctx, us.bus, eventbus.NewEvent(ctx, audit.EventUserCredentialUpdate).With(payload))
}

// updateUserCredentials updates schema-defined credentials for a user without publishing its audit event,
// and sets written once the credentials are stored, since a failure after that leaves them changed.
func (us *userService) updateUserCredentials(
	ctx context.Context,
	userID string,
	credentials json.RawMessage,
	written *bool,
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
	existingEntity, err := us.entityService.GetEntityProfile(ctx, userID)
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
		return us.sharedUserRefusal(ctx, userID, existingUser.OUID, svcErr)
	}

	if svcErr := us.authzService.CanControlEntity(ctx, userID); svcErr != nil {
		return svcErr
	}

	// Check if user is declarative (immutable)
	if svcErr := us.checkUserMutable(ctx, userID, logger); svcErr != nil {
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
	// Refused values are reported before anything is revoked.
	if err = us.entityService.ValidateCredentials(ctx, userID, plaintextJSON); err != nil {
		return us.credentialUpdateError(ctx, logger, userID, err)
	}

	// Replacing a credential the user holds revokes what they were given before it and ends their sessions.
	if err = us.entityService.UpdateCredentials(ctx, userID, plaintextJSON); err != nil {
		*written = errors.Is(err, entity.ErrRevocationIncomplete)
		return us.credentialUpdateError(ctx, logger, userID, err)
	}

	logger.Debug(ctx, "Successfully updated user credentials",
		log.MaskedString(log.LoggerKeyUserID, userID),
		log.Int("credentialTypesCount", len(credentialsMap)))
	us.captureCredentials(ctx, &existingUser, existingUser.Attributes, plaintextCreds)
	return nil
}

// credentialUpdateError maps an entity service error from a credential update to the error it reports.
func (us *userService) credentialUpdateError(ctx context.Context, logger *log.Logger, userID string,
	err error) *tidcommon.ServiceError {
	if errors.Is(err, entity.ErrSchemaValidationFailed) {
		return &ErrorInvalidCredentialValue
	}
	if svcErr := mapEntityError(err); svcErr != nil {
		return svcErr
	}
	return logErrorAndReturnServerError(ctx, logger, "Failed to update user credentials", err,
		log.MaskedString(log.LoggerKeyUserID, userID))
}

// DeleteUser deletes the user with the given ID.
func (us *userService) DeleteUser(ctx context.Context, userID string) *tidcommon.ServiceError {
	svcErr := us.deleteUser(ctx, userID)
	us.publishUserEvent(ctx, audit.EventUserDelete, audit.ActionDelete, userID, svcErr)
	return svcErr
}

// deleteUser deletes the user with the given ID without publishing its audit event.
func (us *userService) deleteUser(ctx context.Context, userID string) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Deleting user", log.MaskedString(log.LoggerKeyUserID, userID))

	if userID == "" {
		return &ErrorMissingUserID
	}

	// Fetch the user to resolve the OU ID for the authorization check.
	existingEntity, err := us.entityService.GetEntityProfile(ctx, userID)
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
		return us.sharedUserRefusal(ctx, userID, existingUser.OUID, svcErr)
	}
	if svcErr := us.authzService.CanControlEntity(ctx, userID); svcErr != nil {
		return svcErr
	}

	// Check if user is declarative (immutable)
	if svcErr := us.checkUserMutable(ctx, userID, logger); svcErr != nil {
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

	// The sharing policies recorded against the user go with it, in the same position as the cascade
	// above: nothing else removes them, and a user later created with this id would come back already
	// shared. Reshares go with the policy beneath them.
	if svcErr := us.sharingService.DeletePoliciesForResource(ctx, UserSharingType, userID); svcErr != nil {
		return translateSharingError(svcErr)
	}

	// Deleting the entity revokes what was issued to the user and ends their sessions.
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

// publishCreate publishes the audit event of an attempt to create a user and returns its result.
func (us *userService) publishCreate(ctx context.Context, created *providers.User,
	svcErr *tidcommon.ServiceError) (*providers.User, *tidcommon.ServiceError) {
	var userID string
	if svcErr == nil {
		userID = created.ID
	}
	us.publishUserEvent(ctx, audit.EventUserCreate, audit.ActionCreate, userID, svcErr)
	return created, svcErr
}

// publishUserEvent publishes the audit event of an attempt on the user with userID, after the change
// committed or was refused. An update or a delete is denied only on a user that exists.
func (us *userService) publishUserEvent(ctx context.Context, name string, action audit.Action, userID string,
	svcErr *tidcommon.ServiceError) {
	// A failed attempt keeps the ID the caller asked for, even one that matches no user, as forensic
	// evidence of what was probed; the payload caps it and the sink escapes it.
	target := audit.Ref{Type: audit.TargetTypeUser, ID: userID}
	eventbus.TryPublish(ctx, us.bus,
		eventbus.NewEvent(ctx, name).With(audit.ChangePayload(ctx, target, action, svcErr)))
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
	dependencies := summarizeBlockingUsages(blocking)
	return tidcommon.CustomServiceError(ErrorUserHasBlockingDependencies, tidcommon.I18nMessage{
		Key: "error.userservice.user_has_blocking_dependencies_description",
		DefaultValue: fmt.Sprintf(
			"The user cannot be deleted because %s depend on it. Remove or reassign them first.", dependencies),
		Params: map[string]string{"dependencies": dependencies},
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

// SetRoleAssignmentService injects the role assignment service that user shares mandate roles through.
func (us *userService) SetRoleAssignmentService(
	ctx context.Context, roleAssignments role.RoleAssignmentServiceInterface,
) error {
	return us.sharingDeclaration.setRoleAssignments(ctx, roleAssignments)
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

	existingEntity, err := us.entityService.GetEntityProfile(ctx, userID)
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
func (us *userService) populateUserDisplayNames(ctx context.Context, users []providers.User, logger *log.Logger) {
	// Collect user types for display attribute resolution.
	userTypes := make([]string, 0, len(users))
	for _, u := range users {
		userTypes = append(userTypes, u.Type)
	}

	displayAttrPaths := entitytype.ResolveDisplayAttributePaths(
		ctx, entitytype.TypeCategoryUser, userTypes, us.entityTypeService, logger)

	// Resolve display for each user.
	for i := range users {
		users[i].Display = utils.ResolveDisplay(
			users[i].ID, users[i].Type, users[i].Attributes, displayAttrPaths)
	}
}

// populateOUHandles resolves OU handles for a slice of users in-place.
func (us *userService) populateOUHandles(ctx context.Context, users []providers.User, logger *log.Logger) {
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
			log.String("ouID", oUID),
		)
	}
	if !exists {
		return &ErrorOrganizationUnitNotFound
	}

	if us.entityTypeService == nil {
		logger.Error(ctx, "Entity type service is not configured for user operations")
		return &tidcommon.InternalServerError
	}

	// Callers authorize the user action first; reading the type it names needs no permission of its own.
	entityType, svcErr := us.entityTypeService.GetEntityTypeByHandle(security.WithRuntimeContext(ctx),
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
			log.String("ouID", oUID),
			log.String("schemaOUID", entityType.OUID),
		)
	}

	if !isParent {
		logger.Debug(ctx, "Organization unit mismatch for user type",
			log.String("userType", userType),
			log.String("ouID", oUID),
			log.String("schemaOUID", entityType.OUID))
		return &ErrorOrganizationUnitMismatch
	}

	return nil
}

// validateProjectOU checks that a user's organization unit belongs to its project. It returns
// ErrorInvalidProject when no such project exists, and ErrorOUNotInProject when the organization unit is
// outside the project.
//
// A deployment that serves no projects, such as a Data Plane, cannot check the project. It keeps the
// project ID and organization unit as given so that imported configuration retains them.
func (us *userService) validateProjectOU(ctx context.Context, projectID, ouID string) *tidcommon.ServiceError {
	if us.projectValidator == nil {
		return nil
	}
	svcErr := us.projectValidator.ValidateProjectOU(ctx, projectID, ouID)
	switch {
	case svcErr == nil:
		return nil
	case svcErr.Code == projectscope.ProjectNotFoundErrorCode:
		return &ErrorInvalidProject
	case svcErr.Code == projectscope.OUNotInProjectErrorCode:
		return &ErrorOUNotInProject
	default:
		return &tidcommon.InternalServerError
	}
}

// markOwned records on each user whether the listed organization unit owns it.
func markOwned(users []providers.User, owned bool) {
	for i := range users {
		users[i].IsOwnedByOU = new(owned)
	}
}

// usersWithAccess converts entities to users with the access facts governance derived for each, in one
// read, and the verified contacts of each.
func (us *userService) usersWithAccess(ctx context.Context, entities []providers.Entity) []providers.User {
	reads := us.governance.ReadAccess(ctx, entities)
	users := entitiesToUsers(entities)
	for i := range users {
		users[i].Status = &reads[i].Status
		users[i].LastLoginAt = reads[i].LastLoginAt
		users[i].VerifiedAttributes = us.verification.GetVerifiedAttributes(ctx, entities[i])
	}
	return users
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

	if svcErr.Code == tidcommon.ErrorUnauthorized.Code {
		return &tidcommon.ErrorUnauthorized
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

// checkUserMutable refuses a change to a user declared in a file, or deployed by a control plane unless
// the change is the managed import's. Either way the user's credentials and attributes stay as they are.
func (us *userService) checkUserMutable(
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
	return us.refuseManaged(ctx, userID, logger)
}

// refuseManaged refuses a change to a user a control plane deployed, unless the change is the managed
// import's: only a deploy from the control plane changes or removes it.
func (us *userService) refuseManaged(
	ctx context.Context, userID string, logger *log.Logger,
) *tidcommon.ServiceError {
	managed, err := managedresource.IsReadOnly(ctx, us.managed, userID, managedresource.TypeUser)
	if err != nil {
		logger.Error(ctx, "Failed to read whether a control plane deployed the user",
			log.MaskedString(log.LoggerKeyUserID, userID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if managed {
		return &ErrorManagedUser
	}
	return nil
}

// markManaged reports the listed users a control plane deployed as read only.
func (us *userService) markManaged(ctx context.Context, users []providers.User, logger *log.Logger) {
	if err := managedresource.MarkDeployed(ctx, us.managed, users,
		func(u *providers.User) string { return u.ID }, func(u *providers.User) { u.IsReadOnly = true },
		managedresource.TypeUser); err != nil {
		// Every write to such a user is refused whatever the list says, so it is served without the flag.
		logger.Error(ctx, "Failed to read which users a control plane deployed", log.Error(err))
	}
}

// authorizeShareInto refuses unless the caller may create users in targetOUID. Users that load from resource files,
// and a declared user, keep their sharing there. userID is empty for a user about to be created.
func (us *userService) authorizeShareInto(ctx context.Context, userID, targetOUID string) *tidcommon.ServiceError {
	if svcErr := us.checkUserAccess(ctx, security.ActionCreateUser, targetOUID, ""); svcErr != nil {
		return svcErr
	}
	if isDeclarativeModeEnabled() {
		return &ErrorCannotModifyDeclarativeResource
	}
	if userID == "" {
		return nil
	}
	return us.checkUserMutable(ctx, userID, us.logger)
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

// mandatedRoleRefusals are the refusals this package's sharing hooks raise, which keep their own codes.
var mandatedRoleRefusals = []string{
	ErrorRolesNotOnNamedUnit.Code, ErrorRolesRuleNotFixed.Code,
	role.ErrorMandatedRoleNotInOU.Code, role.ErrorMandatedRolesNotSupported.Code, sysauthz.ErrorGrantNotPermitted.Code,
}

// sharingErrors answers the sharing framework's client errors in this service's own codes.
var sharingErrors = sharing.ErrorTranslation{
	PolicyNotFound:  ErrorSharingPolicyNotFound,
	PolicyExists:    ErrorSharingPolicyExists,
	VersionMismatch: ErrorSharingPolicyVersionMismatch,
	PolicyDeclared:  ErrorSharingPolicyDeclared,
	InvalidLimit:    ErrorInvalidLimit,
	InvalidOffset:   ErrorInvalidOffset,
	InvalidPolicy:   ErrorInvalidSharingPolicy,
}

// translateSharingError maps a sharing framework error into this service's own codes, except a mandate refusal.
func translateSharingError(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr != nil && slices.Contains(mandatedRoleRefusals, svcErr.Code) {
		return svcErr
	}
	return sharingErrors.Translate(svcErr)
}

// checkSharingAccess authorizes a sharing call against one organization unit. It names no resource,
// so the self-service rule letting users act on their own record does not apply: sharing a user is
// an administrator's decision, never the user's own.
func (us *userService) checkSharingAccess(
	ctx context.Context, action security.Action, ouID string,
) *tidcommon.ServiceError {
	return us.checkUserAccess(ctx, action, ouID, "")
}

// checkSharingReadAccess allows a read when the caller may read users in any of the given
// organization units.
func (us *userService) checkSharingReadAccess(ctx context.Context, ouIDs ...string) *tidcommon.ServiceError {
	denied := &tidcommon.ErrorUnauthorized
	for _, ouID := range ouIDs {
		svcErr := us.checkSharingAccess(ctx, security.ActionReadUser, ouID)
		if svcErr == nil {
			return nil
		}
		if svcErr.Code != tidcommon.ErrorUnauthorized.Code {
			return svcErr
		}
	}
	return denied
}

// requireSharing resolves the user a sharing call names and returns its owning organization unit.
// The user is read without the caller's authorization, because a sharing call is authorized against
// the organization unit whose decision it concerns, which need not be the user's own.
func (us *userService) requireSharing(ctx context.Context, userID string) (string, *tidcommon.ServiceError) {
	if userID == "" {
		return "", &ErrorMissingUserID
	}
	user, svcErr := us.GetUser(security.WithRuntimeContext(ctx), userID, false)
	if svcErr != nil {
		return "", svcErr
	}
	return user.OUID, nil
}

// requireOwnPolicy fetches a policy and refuses one belonging to another resource. Without it a
// policy id alone would reach any policy in the deployment through any user's path.
func (us *userService) requireOwnPolicy(
	ctx context.Context, userID, policyID string,
) (string, sharing.Policy, *tidcommon.ServiceError) {
	owningOUID, svcErr := us.requireSharing(ctx, userID)
	if svcErr != nil {
		return "", sharing.Policy{}, svcErr
	}
	policy, svcErr := us.sharingService.GetPolicy(ctx, policyID)
	if svcErr != nil {
		return "", sharing.Policy{}, translateSharingError(svcErr)
	}
	if policy.ResourceType != UserSharingType || policy.ResourceID != userID {
		return "", sharing.Policy{}, &ErrorSharingPolicyNotFound
	}
	return owningOUID, policy, nil
}

// CreateSharingPolicy records which organization units a user reaches. A policy is the initiating
// organization unit's decision, so that unit is the one the caller must be allowed to act for: the
// owner for a share, or a unit the user already reached for a reshare.
func (us *userService) CreateSharingPolicy(
	ctx context.Context, userID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	policy, svcErr := us.createSharingPolicy(ctx, userID, req)
	var policyID string
	if svcErr == nil {
		policyID = policy.ID
	}
	us.publishSharingPolicyEvent(ctx, audit.EventUserSharingPolicyCreate, audit.ActionCreate, userID,
		policyID, svcErr)
	return policy, svcErr
}

// createSharingPolicy records a sharing policy of a user without publishing its audit event.
func (us *userService) createSharingPolicy(
	ctx context.Context, userID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	owningOUID, svcErr := us.requireSharing(ctx, userID)
	if svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	initiatingOUID := req.InitiatingOUID
	if initiatingOUID == "" {
		initiatingOUID = owningOUID
	}
	if svcErr := us.checkSharingAccess(ctx, security.ActionUpdateUser, initiatingOUID); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	// Users that load from resource files only have all their sharing declared there.
	if isDeclarativeModeEnabled() {
		return sharing.Policy{}, &ErrorCannotModifyDeclarativeResource
	}
	// A declared user's own sharing is declared with it; a unit it is shared into may still reshare.
	if initiatingOUID == owningOUID {
		logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
		if svcErr := us.checkUserMutable(ctx, userID, logger); svcErr != nil {
			return sharing.Policy{}, svcErr
		}
	}
	policy, svcErr := us.sharingService.CreatePolicy(ctx, UserSharingType, userID, owningOUID, req)
	return policy, translateSharingError(svcErr)
}

// GetSharingPolicy returns one of the user's sharing policies. The owner sees every decision about
// its user; any other organization unit sees its own.
func (us *userService) GetSharingPolicy(
	ctx context.Context, userID, policyID string,
) (sharing.Policy, *tidcommon.ServiceError) {
	owningOUID, policy, svcErr := us.requireOwnPolicy(ctx, userID, policyID)
	if svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	if svcErr := us.checkSharingReadAccess(ctx, owningOUID, policy.InitiatingOUID); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	return policy, nil
}

// ListSharingPolicies returns one page of the user's sharing policies. A caller who may read users
// in the owning organization unit sees them all; any other caller sees the ones its own
// organization units made.
func (us *userService) ListSharingPolicies(
	ctx context.Context, userID string, limit, offset int,
) (sharing.PolicyList, *tidcommon.ServiceError) {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return sharing.PolicyList{}, &ErrorInvalidLimit
	}
	// An offset this close to MaxInt would overflow the page bounds, the start index and the links.
	if offset < 0 || offset > math.MaxInt-limit {
		return sharing.PolicyList{}, &ErrorInvalidOffset
	}

	owningOUID, svcErr := us.requireSharing(ctx, userID)
	if svcErr != nil {
		return sharing.PolicyList{}, svcErr
	}
	return sharing.ListReadablePolicies(ctx, us.sharingService, UserSharingType, userID, owningOUID, limit,
		offset, func(ouID string) *tidcommon.ServiceError {
			return us.checkSharingAccess(ctx, security.ActionReadUser, ouID)
		}, translateSharingError)
}

// UpdateSharingPolicy replaces a sharing policy's contents. Only the organization unit whose
// decision it is may change it.
func (us *userService) UpdateSharingPolicy(
	ctx context.Context, userID, policyID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	updated, svcErr := us.updateSharingPolicy(ctx, userID, policyID, req)
	us.publishSharingPolicyEvent(ctx, audit.EventUserSharingPolicyUpdate, audit.ActionUpdate, userID,
		policyID, svcErr)
	return updated, svcErr
}

// updateSharingPolicy replaces a sharing policy of a user without publishing its audit event.
func (us *userService) updateSharingPolicy(
	ctx context.Context, userID, policyID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	owningOUID, policy, svcErr := us.requireOwnPolicy(ctx, userID, policyID)
	if svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	if svcErr := us.checkSharingAccess(ctx, security.ActionUpdateUser, policy.InitiatingOUID); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	if isDeclarativeModeEnabled() {
		return sharing.Policy{}, &ErrorCannotModifyDeclarativeResource
	}
	// A deployed user's own sharing is deployed with it; a unit the user is shared into may still reshare.
	if policy.InitiatingOUID == owningOUID {
		if svcErr := us.refuseManaged(ctx, userID, us.logger); svcErr != nil {
			return sharing.Policy{}, svcErr
		}
	}
	updated, svcErr := us.sharingService.UpdatePolicy(ctx, policyID, req)
	return updated, translateSharingError(svcErr)
}

// DeleteSharingPolicy removes a sharing policy and everything that depended on it. A policy that is
// not there is already gone, so the call succeeds.
func (us *userService) DeleteSharingPolicy(ctx context.Context, userID, policyID string) *tidcommon.ServiceError {
	svcErr := us.deleteSharingPolicy(ctx, userID, policyID)
	// A delete that found no policy of the user succeeds without an event, so no caller can record a
	// deletion of an ID it made up.
	if svcErr == errSharingPolicyNotFoundNoop {
		return nil
	}
	us.publishSharingPolicyEvent(ctx, audit.EventUserSharingPolicyDelete, audit.ActionDelete, userID,
		policyID, svcErr)
	return svcErr
}

// deleteSharingPolicy removes a sharing policy of a user without publishing its audit event.
func (us *userService) deleteSharingPolicy(ctx context.Context, userID, policyID string) *tidcommon.ServiceError {
	owningOUID, policy, svcErr := us.requireOwnPolicy(ctx, userID, policyID)
	if svcErr != nil {
		if svcErr.Code == ErrorSharingPolicyNotFound.Code {
			return errSharingPolicyNotFoundNoop
		}
		return svcErr
	}
	if svcErr := us.checkSharingAccess(ctx, security.ActionUpdateUser, policy.InitiatingOUID); svcErr != nil {
		return svcErr
	}
	if isDeclarativeModeEnabled() {
		return &ErrorCannotModifyDeclarativeResource
	}
	if policy.InitiatingOUID == owningOUID {
		if svcErr := us.refuseManaged(ctx, userID, us.logger); svcErr != nil {
			return svcErr
		}
	}
	if policy.Declared {
		return &ErrorSharingPolicyDeclared
	}
	return translateSharingError(us.sharingService.DeletePolicy(ctx, policyID))
}

// publishSharingPolicyEvent publishes the audit event of an attempt on the sharing policy with policyID
// of the user with userID, after the change committed or was refused. The policy is the target and the
// user the subject, which a succeeded or denied attempt names: a sharing call is refused only once the
// user is found. Reshares the framework re-narrows or removes beneath a changed policy write no event of
// their own.
func (us *userService) publishSharingPolicyEvent(ctx context.Context, name string, action audit.Action,
	userID, policyID string, svcErr *tidcommon.ServiceError) {
	// A failed attempt keeps the IDs the caller asked for, even ones that match nothing, as forensic
	// evidence of what was probed; the payload caps them and the sink escapes them.
	target := audit.Ref{Type: audit.TargetTypeUserSharingPolicy, ID: policyID}
	subject := audit.Ref{Type: audit.TargetTypeUser, ID: userID}
	eventbus.TryPublish(ctx, us.bus, eventbus.NewEvent(ctx, name).With(
		audit.ChangePayloadWithSubject(ctx, target, subject, action, svcErr)))
}

// ResolveSharingOverlay answers the rules, such as mandated roles, one organization unit holds the user on.
func (us *userService) ResolveSharingOverlay(
	ctx context.Context, userID, ouID string,
) (sharing.ResolvedOverlay, *tidcommon.ServiceError) {
	owningOUID, svcErr := us.requireSharing(ctx, userID)
	if svcErr != nil {
		return sharing.ResolvedOverlay{}, svcErr
	}
	if ouID == "" {
		return sharing.ResolvedOverlay{}, &ErrorMissingOUIDParameter
	}
	if svcErr := us.checkSharingReadAccess(ctx, owningOUID, ouID); svcErr != nil {
		return sharing.ResolvedOverlay{}, svcErr
	}
	overlay, svcErr := us.sharingService.ResolveOverlayRules(ctx, UserSharingType, userID, ouID)
	if svcErr != nil {
		// An id naming no organization unit leaves nothing to resolve either.
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			return sharing.ResolvedOverlay{}, &ErrorUserNotSharedToOU
		}
		return sharing.ResolvedOverlay{}, translateSharingError(svcErr)
	}
	if !overlay.Visible {
		return sharing.ResolvedOverlay{}, &ErrorUserNotSharedToOU
	}
	return overlay, nil
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
	ctx context.Context, user *providers.User,
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

// GetAccessibleOrganizationUnits lists the organization units the signed-in user can access: the one
// it lives in and every one it is shared into, narrowed by scope relative to the current one.
func (us *userService) GetAccessibleOrganizationUnits(
	ctx context.Context, scope string, limit, offset int,
) (*AccessibleOrganizationUnitListResponse, *tidcommon.ServiceError) {
	if scope != "" && scope != accessibleOUScopeSiblings {
		return nil, &ErrorInvalidScope
	}
	if svcErr := validateOUPage(limit, offset); svcErr != nil {
		return nil, svcErr
	}

	userID := security.GetSubject(ctx)
	if strings.TrimSpace(userID) == "" {
		return nil, &ErrorAuthenticationFailed
	}
	// Units are read on the user's behalf: listing them would otherwise need a permission the user
	// does not hold, and these are units the user can already access.
	internalCtx := security.WithRuntimeContext(ctx)
	user, svcErr := us.GetUser(internalCtx, userID, false)
	if svcErr != nil {
		return nil, svcErr
	}
	units := newOUReader(us.ouService)
	accessible, svcErr := us.accessibleOUs(ctx, units, user)
	if svcErr != nil {
		return nil, svcErr
	}
	current := security.GetOUID(ctx)
	if current == "" {
		current = user.OUID
	}

	inScope := accessible
	if scope == accessibleOUScopeSiblings {
		currentUnit, found, svcErr := units.get(internalCtx, current)
		if svcErr != nil {
			return nil, svcErr
		}
		inScope = make([]oupkg.OrganizationUnit, 0, len(accessible))
		for _, ou := range accessible {
			if found && sameParent(ou.Parent, currentUnit.Parent) {
				inScope = append(inScope, ou)
			}
		}
	}

	scopeQuery := ""
	if scope != "" {
		scopeQuery = "&scope=" + scope
	}
	return &AccessibleOrganizationUnitListResponse{
		Current:                          current,
		UserOrganizationUnitListResponse: pageOUs(inScope, "/users/me/ous", limit, offset, scopeQuery),
	}, nil
}

// GetUserOrganizationUnits lists the organization units a user can access: the one it lives in and
// every one it is shared into. The caller must be able to read the user.
func (us *userService) GetUserOrganizationUnits(
	ctx context.Context, userID string, limit, offset int,
) (*UserOrganizationUnitListResponse, *tidcommon.ServiceError) {
	if svcErr := validateOUPage(limit, offset); svcErr != nil {
		return nil, svcErr
	}
	user, svcErr := us.GetUser(ctx, userID, false)
	if svcErr != nil {
		return nil, svcErr
	}
	accessible, svcErr := us.accessibleOUs(ctx, newOUReader(us.ouService), user)
	if svcErr != nil {
		return nil, svcErr
	}
	page := pageOUs(accessible, "/users/"+userID+"/ous", limit, offset, "")
	return &page, nil
}

// validateOUPage refuses a limit outside its bounds, and an offset so close to MaxInt that it would
// overflow the page bounds, the start index and the links.
func validateOUPage(limit, offset int) *tidcommon.ServiceError {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return &ErrorInvalidLimit
	}
	if offset < 0 || offset > math.MaxInt-limit {
		return &ErrorInvalidOffset
	}
	return nil
}

// accessibleOUs reads the organization units a user can access: its own first, then the ones it is
// shared into, by ID.
func (us *userService) accessibleOUs(
	ctx context.Context, units *ouReader, user *providers.User,
) ([]oupkg.OrganizationUnit, *tidcommon.ServiceError) {
	reached, svcErr := us.sharingService.ListResourceSharedOUIDs(ctx, UserSharingType, user.ID)
	if svcErr != nil {
		return nil, translateSharingError(svcErr)
	}
	// The reach comes in no particular order, so it is sorted for a page to fall in the same place on
	// every request.
	slices.Sort(reached)

	// Read as the runtime, as the caller may not read the units the user is shared into.
	internalCtx := security.WithRuntimeContext(ctx)
	accessible := make([]oupkg.OrganizationUnit, 0, len(reached)+1)
	for _, ouID := range append([]string{user.OUID}, reached...) {
		ou, found, svcErr := units.get(internalCtx, ouID)
		if svcErr != nil {
			return nil, svcErr
		}
		// A unit removed since it was shared is no longer anywhere to go.
		if !found {
			continue
		}
		accessible = append(accessible, ou)
	}
	return accessible, nil
}

// pageOUs returns one page of the listed organization units, linked under path.
func pageOUs(
	ous []oupkg.OrganizationUnit, path string, limit, offset int, query string,
) UserOrganizationUnitListResponse {
	page := ous[min(offset, len(ous)):min(offset+limit, len(ous))]
	items := make([]AccessibleOrganizationUnit, 0, len(page))
	for _, ou := range page {
		items = append(items, AccessibleOrganizationUnit{ID: ou.ID, Handle: ou.Handle, Name: ou.Name,
			LogoURL: ou.LogoURL})
	}
	return UserOrganizationUnitListResponse{
		TotalResults:      len(ous),
		StartIndex:        offset + 1,
		Count:             len(items),
		OrganizationUnits: items,
		Links:             utils.BuildPaginationLinks(path, limit, offset, len(ous), query),
	}
}

// sameParent reports whether two units sit under the same parent, top-level units included.
func sameParent(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// ouReader reads organization units for one request, each at most once.
type ouReader struct {
	ouService oupkg.OrganizationUnitServiceInterface
	units     map[string]oupkg.OrganizationUnit
	missing   map[string]struct{}
}

// newOUReader returns a reader over the given organization unit service.
func newOUReader(ouService oupkg.OrganizationUnitServiceInterface) *ouReader {
	return &ouReader{
		ouService: ouService,
		units:     make(map[string]oupkg.OrganizationUnit),
		missing:   make(map[string]struct{}),
	}
}

// get returns one organization unit, reporting whether it exists.
func (r *ouReader) get(
	ctx context.Context, ouID string,
) (oupkg.OrganizationUnit, bool, *tidcommon.ServiceError) {
	if ou, ok := r.units[ouID]; ok {
		return ou, true, nil
	}
	if _, ok := r.missing[ouID]; ok {
		return oupkg.OrganizationUnit{}, false, nil
	}
	ou, svcErr := r.ouService.GetOrganizationUnit(ctx, ouID)
	if svcErr != nil {
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			r.missing[ouID] = struct{}{}
			return oupkg.OrganizationUnit{}, false, nil
		}
		return oupkg.OrganizationUnit{}, false, &tidcommon.InternalServerError
	}
	r.units[ouID] = ou
	return ou, true, nil
}
