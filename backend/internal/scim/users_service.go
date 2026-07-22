package scim

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/user"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// SCIMUsersServiceInterface defines the Users CRUD operations exposed to the users handler.
type SCIMUsersServiceInterface interface {
	ListUsers(
		ctx context.Context, startIndex, count int, baseURL string,
	) (SCIMUserListResponse, *tidcommon.ServiceError)
	CreateUser(
		ctx context.Context, payload *SCIMUserPayload, baseURL string,
	) (*SCIMUser, *tidcommon.ServiceError)
	GetUser(ctx context.Context, userID, baseURL string) (*SCIMUser, *tidcommon.ServiceError)
	ReplaceUser(
		ctx context.Context, userID string, payload *SCIMUserPayload, ifMatch, baseURL string,
	) (*SCIMUser, *tidcommon.ServiceError)
	DeleteUser(ctx context.Context, userID string, ifMatch string) *tidcommon.ServiceError
}

// scimUsersService implements SCIMUsersServiceInterface.
type scimUsersService struct {
	userService       user.UserServiceInterface
	entityTypeService entitytype.EntityTypeServiceInterface
}

// newSCIMUsersService creates a new scimUsersService.
func newSCIMUsersService(
	userService user.UserServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
) SCIMUsersServiceInterface {
	return &scimUsersService{
		userService:       userService,
		entityTypeService: entityTypeService,
	}
}

func (s *scimUsersService) ListUsers(ctx context.Context, startIndex, count int,
	baseURL string) (SCIMUserListResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if startIndex < 1 {
		startIndex = 1
	}
	if count < 1 {
		count = 20
	}
	offset := startIndex - 1
	listResp, svcErr := s.userService.GetUserList(ctx, count, offset, nil, false)
	if svcErr != nil {
		logger.Error(ctx, "SCIM ListUsers: failed to get user list", log.Any("error", svcErr))
		return SCIMUserListResponse{}, mapUserServiceErrorToSCIM(svcErr)
	}
	scimUsers := make([]SCIMUser, 0, len(listResp.Users))
	credKeysByType := make(map[string]map[string]struct{})
	for _, u := range listResp.Users {
		extensionURN := buildSchemaURN(u.Type)
		credKeys, ok := credKeysByType[u.Type]
		if !ok {
			credKeys = s.getCredentialKeys(ctx, u.Type)
			credKeysByType[u.Type] = credKeys
		}
		scimUsers = append(scimUsers, buildSCIMUserResource(u, extensionURN, baseURL, credKeys))
	}

	return buildSCIMUserListResponse(scimUsers, listResp.TotalResults, startIndex, len(scimUsers)), nil
}

// GetUser fetches a single user by ID and returns a SCIM User resource.
func (s *scimUsersService) GetUser(
	ctx context.Context, userID, baseURL string,
) (*SCIMUser, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	u, svcErr := s.userService.GetUser(ctx, userID, false)
	if svcErr != nil {
		logger.Debug(ctx, "SCIM GetUser: user service error",
			log.String("userID", userID), log.Any("error", svcErr))
		return nil, mapUserServiceErrorToSCIM(svcErr)
	}

	extensionURN := buildSchemaURN(u.Type)
	credKeys := s.getCredentialKeys(ctx, u.Type)
	scimUser := buildSCIMUserResource(*u, extensionURN, baseURL, credKeys)
	return &scimUser, nil
}

// CreateUser validates the entity type, then delegates to user.UserService.CreateUser.
func (s *scimUsersService) CreateUser(
	ctx context.Context, payload *SCIMUserPayload, baseURL string,
) (*SCIMUser, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	runtimeCtx := security.WithRuntimeContext(ctx)
	canonicalName, svcErr := resolveEntityTypeNameForSchemaURN(runtimeCtx, s.entityTypeService, payload.UserTypeName)
	if svcErr != nil || canonicalName == "" {
		logger.Error(ctx, "SCIM CreateUser: entity type not found",
			log.String("userTypeName", payload.UserTypeName), log.Any("error", svcErr))
		return nil, &ErrorUnknownUserType
	}

	et, svcErr := s.entityTypeService.GetEntityTypeByName(runtimeCtx, entitytype.TypeCategoryUser, canonicalName)

	if svcErr != nil {
		logger.Error(ctx, "SCIM CreateUser: entity type not found",
			log.String("userTypeName", canonicalName), log.Any("error", svcErr))
		return nil, &ErrorUnknownUserType
	}
	attrsJSON, err := json.Marshal(payload.ExtensionAttrs)
	if err != nil {
		logger.Error(ctx, "SCIM CreateUser: failed to marshal extension attrs", log.Error(err))
		return nil, &ErrorInvalidRequestBody
	}
	newUser := &user.User{
		OUID:       et.OUID,
		Type:       canonicalName,
		Attributes: attrsJSON,
	}

	created, svcErr := s.userService.CreateUser(ctx, newUser)
	if svcErr != nil {
		logger.Error(ctx, "SCIM CreateUser: user service error", log.Any("error", svcErr))
		return nil, mapUserServiceErrorToSCIM(svcErr)
	}
	extensionURN := buildSchemaURN(created.Type)
	credKeys := s.getCredentialKeys(ctx, canonicalName)
	scimUser := buildSCIMUserResource(*created, extensionURN, baseURL, credKeys)
	return &scimUser, nil
}

// ReplaceUser performs a full PUT replace on the user.
func (s *scimUsersService) ReplaceUser(
	ctx context.Context, userID string, payload *SCIMUserPayload, ifMatch, baseURL string,
) (*SCIMUser, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	runtimeCtx := security.WithRuntimeContext(ctx)
	canonicalName, svcErr := resolveEntityTypeNameForSchemaURN(runtimeCtx, s.entityTypeService, payload.UserTypeName)
	if svcErr != nil || canonicalName == "" {
		logger.Error(runtimeCtx, "SCIM ReplaceUser: entity type not found",
			log.String("userTypeName", payload.UserTypeName), log.Any("error", svcErr))
		return nil, &ErrorUnknownUserType
	}

	existingUser, svcErr := s.userService.GetUser(ctx, userID, false)
	if svcErr != nil {
		logger.Debug(ctx, "SCIM ReplaceUser: user service error",
			log.String("userID", userID), log.Any("error", svcErr))
		return nil, mapUserServiceErrorToSCIM(svcErr)
	}

	if trimmed := strings.TrimSpace(ifMatch); trimmed != "" {
		if svcErr := checkIfMatch(trimmed, generateVersion(userVersionState(*existingUser))); svcErr != nil {
			return nil, svcErr
		}
	}

	if existingUser.Type != canonicalName {
		logger.Error(ctx, "SCIM ReplaceUser: user type mismatch",
			log.String("userID", userID), log.String("existingType", existingUser.Type),
			log.String("requestedType", canonicalName))
		return nil, &ErrorImmutableUserType
	}

	et, svcErr := s.entityTypeService.GetEntityTypeByName(runtimeCtx, entitytype.TypeCategoryUser, canonicalName)
	if svcErr != nil {
		logger.Error(runtimeCtx, "SCIM ReplaceUser: entity type not found",
			log.String("userTypeName", canonicalName), log.Any("error", svcErr))
		return nil, &ErrorUnknownUserType
	}
	attrsJSON, err := json.Marshal(payload.ExtensionAttrs)
	if err != nil {
		logger.Error(ctx, "SCIM ReplaceUser: failed to marshal extension attrs", log.Error(err))
		return nil, &ErrorInvalidRequestBody
	}
	updatedUser := &user.User{
		ID:         userID,
		OUID:       et.OUID,
		Type:       canonicalName,
		Attributes: attrsJSON,
	}
	result, svcErr := s.userService.UpdateUser(ctx, userID, updatedUser)
	if svcErr != nil {
		logger.Error(ctx, "SCIM ReplaceUser: user service error",
			log.String("userID", userID), log.Any("error", svcErr))
		return nil, mapUserServiceErrorToSCIM(svcErr)
	}

	extensionURN := buildSchemaURN(result.Type)
	credKeys := s.getCredentialKeys(ctx, canonicalName)
	scimUser := buildSCIMUserResource(*result, extensionURN, baseURL, credKeys)
	return &scimUser, nil
}

// DeleteUser deletes a user by ID.
func (s *scimUsersService) DeleteUser(ctx context.Context, userID string, ifMatch string) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if trimmed := strings.TrimSpace(ifMatch); trimmed != "" {
		existingUser, svcErr := s.userService.GetUser(ctx, userID, false)
		if svcErr != nil {
			logger.Error(ctx, "SCIM DeleteUser: user service error",
				log.String("userID", userID), log.Any("error", svcErr))
			return mapUserServiceErrorToSCIM(svcErr)
		}
		if svcErr := checkIfMatch(trimmed, generateVersion(userVersionState(*existingUser))); svcErr != nil {
			return svcErr
		}
	}

	svcErr := s.userService.DeleteUser(ctx, userID)
	if svcErr != nil {
		logger.Error(ctx, "SCIM DeleteUser: user service error",
			log.String("userID", userID), log.Any("error", svcErr))
		return mapUserServiceErrorToSCIM(svcErr)
	}
	return nil
}

func (s *scimUsersService) getCredentialKeys(ctx context.Context, canonicalName string) map[string]struct{} {
	credKeys := make(map[string]struct{})
	// Use elevated runtime context if necessary, but we are just reading schema info.
	credentialInfos, err := s.entityTypeService.GetAttributes(security.WithRuntimeContext(ctx),
		entitytype.TypeCategoryUser, canonicalName, true, false, false)
	if err == nil {
		for _, info := range credentialInfos {
			credKeys[info.Attribute] = struct{}{}
		}
	}
	return credKeys
}

// mapUserServiceErrorToSCIM translates a user service error into a SCIM package error.
// Uses the exact error codes from user/error_constants.go.
func mapUserServiceErrorToSCIM(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr == nil {
		return nil
	}
	switch svcErr.Code {
	case "USR-1003": // user.ErrorUserNotFound
		return &ErrorUserNotFound
	case "USR-1014": // user.ErrorAttributeConflict
		return &ErrorUniquenessConflict
	case "USR-1019": // user.ErrorSchemaValidationFailed
		return &ErrorSchemaValidationFailed
	case "USR-1021": // user.ErrorEntityTypeNotFound
		return &ErrorUnknownUserType
	case "USR-1025": // user.ErrorCannotModifyDeclarativeResource
		return &ErrorMutabilityViolation
	case tidcommon.ErrorUnauthorized.Code:
		return svcErr
	default:
		if svcErr.Type == tidcommon.ServerErrorType {
			return &tidcommon.InternalServerError
		}
		return &ErrorInvalidRequestBody
	}
}
