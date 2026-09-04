// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package users

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/thunder-id/thunderid/internal/entitytype"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/user"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const usersServiceLoggerComponentName = "SCIMUsersService"

// SCIMUsersServiceInterface defines the Users CRUD operations exposed to the users handler.
type SCIMUsersServiceInterface interface {
	ListUsers(
		ctx context.Context, startIndex, count int,
		filters map[string]interface{}, baseURL string,
	) (SCIMUserListResponse, *tidcommon.ServiceError)
	CreateUser(
		ctx context.Context, payload *SCIMUserPayload, baseURL string,
	) (*SCIMUser, *tidcommon.ServiceError)
	GetUser(ctx context.Context, userID, baseURL string) (*SCIMUser, *tidcommon.ServiceError)
	ReplaceUser(
		ctx context.Context, userID string, payload *SCIMUserPayload, ifMatch, baseURL string, isSelf bool,
	) (*SCIMUser, *tidcommon.ServiceError)
	DeleteUser(ctx context.Context, userID string, ifMatch string) *tidcommon.ServiceError
}

// scimUsersService implements SCIMUsersServiceInterface.
type scimUsersService struct {
	userService     user.UserServiceInterface
	userTypeService entitytype.EntityTypeServiceInterface
	cfg             scimconfig.SCIMConfig
	logger          log.Logger
}

// newSCIMUsersService creates a new scimUsersService.
func newSCIMUsersService(
	userService user.UserServiceInterface,
	userTypeService entitytype.EntityTypeServiceInterface,
	cfg scimconfig.SCIMConfig,
) SCIMUsersServiceInterface {
	return &scimUsersService{
		userService:     userService,
		userTypeService: userTypeService,
		cfg:             cfg,
		logger:          *log.GetLogger().With(log.String(log.LoggerKeyComponentName, usersServiceLoggerComponentName)),
	}
}

// ListUsers retrieves a paginated list of SCIM User resources filtered by search criteria.
func (s *scimUsersService) ListUsers(ctx context.Context, startIndex, count int,
	filters map[string]interface{}, baseURL string) (SCIMUserListResponse, *tidcommon.ServiceError) {
	if startIndex < 1 {
		startIndex = 1
	}
	if count < 0 {
		count = 0
	}

	// GetUserList rejects a limit below 1, so a count of 0 (client wants only
	// totalResults, no resources per RFC 7644 §3.4.2.4) fetches a single row
	// and discards it below.
	fetchLimit := count
	if fetchLimit == 0 {
		fetchLimit = 1
	}

	offset := startIndex - 1
	listResp, svcErr := s.userService.GetUserList(ctx, fetchLimit, offset, filters, false)
	if svcErr != nil {
		s.logger.Error(ctx, "SCIM ListUsers: failed to get user list", log.Any("error", svcErr))
		return SCIMUserListResponse{}, mapUserServiceErrorToSCIM(svcErr)
	}
	if count == 0 {
		return buildSCIMUserListResponse(nil, listResp.TotalResults, startIndex, 0), nil
	}
	scimUsers := make([]SCIMUser, 0, len(listResp.Users))
	credKeysByType := make(map[string]map[string]struct{})
	unresolvedTypes := make(map[string]struct{})
	for _, u := range listResp.Users {
		if _, unresolved := unresolvedTypes[u.Type]; unresolved {
			continue
		}
		credKeys, ok := credKeysByType[u.Type]
		if !ok {
			var svcErr *tidcommon.ServiceError
			credKeys, svcErr = s.getCredentialKeys(ctx, u.Type)
			if svcErr != nil {
				s.logger.Warn(ctx, "SCIM ListUsers: omitting user with unresolvable user type",
					log.String("userID", u.ID), log.String("userType", u.Type))
				unresolvedTypes[u.Type] = struct{}{}
				continue
			}
			credKeysByType[u.Type] = credKeys
		}
		extensionURN := scim.BuildSchemaURN(u.Type)
		scimUsers = append(scimUsers, buildSCIMUserResource(
			ctx, s.logger, u, extensionURN, baseURL, credKeys, s.cfg.ReturnMappedCoreAttrsOnGet))
	}

	return buildSCIMUserListResponse(scimUsers, listResp.TotalResults, startIndex, len(scimUsers)), nil
}

// GetUser fetches a single user by ID and returns a SCIM User resource.
func (s *scimUsersService) GetUser(
	ctx context.Context, userID, baseURL string,
) (*SCIMUser, *tidcommon.ServiceError) {
	u, svcErr := s.userService.GetUser(ctx, userID, false)
	if svcErr != nil {
		s.logger.Debug(ctx, "SCIM GetUser: user service error",
			log.String("userID", userID), log.Any("error", svcErr))
		return nil, mapUserServiceErrorToSCIM(svcErr)
	}

	extensionURN := scim.BuildSchemaURN(u.Type)
	credKeys, svcErr := s.getCredentialKeys(ctx, u.Type)
	if svcErr != nil {
		return nil, svcErr
	}
	scimUser := buildSCIMUserResource(ctx, s.logger, *u, extensionURN, baseURL, credKeys, s.cfg.ReturnMappedCoreAttrsOnGet)
	return &scimUser, nil
}

// CreateUser validates the user type, then delegates to user.UserService.CreateUser.
func (s *scimUsersService) CreateUser(
	ctx context.Context, payload *SCIMUserPayload, baseURL string,
) (*SCIMUser, *tidcommon.ServiceError) {
	runtimeCtx := security.WithRuntimeContext(ctx)
	var resolvedUserTypeName string
	var svcErr *tidcommon.ServiceError
	if payload.UserTypeName == "" {
		resolvedUserTypeName, svcErr = scim.ResolveCoreUserType(runtimeCtx, s.userTypeService, s.cfg.CoreUserTypeID)
		if svcErr != nil {
			s.logger.Error(ctx, "SCIM CreateUser: no core user type available", log.Any("error", svcErr))
			return nil, svcErr
		}
	} else {
		resolvedUserTypeName, svcErr = scim.ResolveUserTypeNameForSchemaURN(
			runtimeCtx, s.userTypeService, payload.UserTypeName)
		if svcErr != nil || resolvedUserTypeName == "" {
			s.logger.Error(ctx, "SCIM CreateUser: user type not found",
				log.String("userTypeName", payload.UserTypeName), log.Any("error", svcErr))
			return nil, &scim.ErrorUnknownUserType
		}
	}

	et, svcErr := s.userTypeService.GetEntityTypeByName(runtimeCtx, entitytype.TypeCategoryUser, resolvedUserTypeName)

	if svcErr != nil {
		s.logger.Error(ctx, "SCIM CreateUser: user type not found",
			log.String("userTypeName", resolvedUserTypeName), log.Any("error", svcErr))
		return nil, scim.MapEntityTypeServiceErrorToSCIM(svcErr)
	}

	if len(payload.CoreAttrs) > 0 {
		reverseMapped, err := reverseMapCoreAttrsForSchema(payload.CoreAttrs, et.Schema)
		if err != nil {
			s.logger.Error(ctx, "SCIM CreateUser: failed to parse user type schema", log.Error(err))
			return nil, &scim.ErrorInternalServer
		}
		if svcErr := mergeReverseMappedCoreAttrs(payload.ExtensionAttrs, reverseMapped); svcErr != nil {
			s.logger.Debug(ctx, "SCIM CreateUser: conflicting value between core and custom schema",
				log.Any("error", svcErr))
			return nil, svcErr
		}
	}
	missing, err := missingRequiredAttrs(payload.ExtensionAttrs, et.Schema, false)
	if err != nil {
		s.logger.Error(ctx, "SCIM CreateUser: failed to parse user type schema", log.Error(err))
		return nil, &scim.ErrorInternalServer
	}
	if len(missing) > 0 {
		s.logger.Debug(ctx, "SCIM CreateUser: missing required attributes for user type",
			log.String("userType", resolvedUserTypeName), log.Any("missing", missing))
		return nil, scim.NewMissingRequiredAttributesError(resolvedUserTypeName, missing)
	}
	undeclared, err := undeclaredAttrs(payload.ExtensionAttrs, et.Schema)
	if err != nil {
		s.logger.Error(ctx, "SCIM CreateUser: failed to parse user type schema", log.Error(err))
		return nil, &scim.ErrorInternalServer
	}
	if len(undeclared) > 0 {
		s.logger.Debug(ctx, "SCIM CreateUser: undeclared attributes for user type",
			log.String("userType", resolvedUserTypeName), log.Any("undeclared", undeclared))
		return nil, scim.NewUndeclaredAttributesError(resolvedUserTypeName, undeclared)
	}
	attrsJSON, err := json.Marshal(payload.ExtensionAttrs)
	if err != nil {
		s.logger.Error(ctx, "SCIM CreateUser: failed to marshal extension attrs", log.Error(err))
		return nil, &scim.ErrorInvalidRequestBody
	}
	newUser := &user.User{
		OUID:       et.OUID,
		Type:       resolvedUserTypeName,
		Attributes: attrsJSON,
	}

	created, svcErr := s.userService.CreateUser(ctx, newUser)
	if svcErr != nil {
		s.logger.Error(ctx, "SCIM CreateUser: user service error", log.Any("error", svcErr))
		return nil, mapUserServiceErrorToSCIM(svcErr)
	}
	extensionURN := scim.BuildSchemaURN(created.Type)
	credKeys, svcErr := s.getCredentialKeys(ctx, resolvedUserTypeName)
	if svcErr != nil {
		return nil, svcErr
	}
	scimUser := buildSCIMUserResource(ctx, s.logger, *created, extensionURN, baseURL, credKeys, len(payload.CoreAttrs) > 0)
	return &scimUser, nil
}

// ReplaceUser performs a full PUT replace on the user. isSelf marks a
// self-service caller (SCIM /Me), whose type and OU are already pinned to
// their existing values below, so only attributes are ever mutated for them.
func (s *scimUsersService) ReplaceUser(
	ctx context.Context, userID string, payload *SCIMUserPayload, ifMatch, baseURL string, isSelf bool,
) (*SCIMUser, *tidcommon.ServiceError) {
	runtimeCtx := security.WithRuntimeContext(ctx)

	existingUser, svcErr := s.userService.GetUser(ctx, userID, false)
	if svcErr != nil {
		s.logger.Debug(ctx, "SCIM ReplaceUser: user service error",
			log.String("userID", userID), log.Any("error", svcErr))
		return nil, mapUserServiceErrorToSCIM(svcErr)
	}

	if trimmed := strings.TrimSpace(ifMatch); trimmed != "" {
		if svcErr := scim.CheckIfMatch(trimmed, scim.GenerateVersion(userVersionState(*existingUser))); svcErr != nil {
			return nil, svcErr
		}
	}

	// The user's type is immutable, so an omitted extension URN defaults to the
	// existing type rather than being treated as ambiguous. A supplied URN must
	// still match the existing type.
	resolvedUserTypeName := existingUser.Type
	if payload.UserTypeName != "" {
		requestedUserTypeName, svcErr := scim.ResolveUserTypeNameForSchemaURN(
			runtimeCtx, s.userTypeService, payload.UserTypeName)
		if svcErr != nil || requestedUserTypeName == "" {
			s.logger.Error(runtimeCtx, "SCIM ReplaceUser: user type not found",
				log.String("userTypeName", payload.UserTypeName), log.Any("error", svcErr))
			return nil, &scim.ErrorUnknownUserType
		}
		if requestedUserTypeName != existingUser.Type {
			s.logger.Error(ctx, "SCIM ReplaceUser: user type mismatch",
				log.String("userID", userID), log.String("existingType", existingUser.Type),
				log.String("requestedType", requestedUserTypeName))
			return nil, &scim.ErrorImmutableUserType
		}
	}

	et, svcErr := s.userTypeService.GetEntityTypeByName(runtimeCtx, entitytype.TypeCategoryUser, resolvedUserTypeName)
	if svcErr != nil {
		s.logger.Error(runtimeCtx, "SCIM ReplaceUser: user type not found",
			log.String("userTypeName", resolvedUserTypeName), log.Any("error", svcErr))
		return nil, scim.MapEntityTypeServiceErrorToSCIM(svcErr)
	}
	if len(payload.CoreAttrs) > 0 {
		reverseMapped, err := reverseMapCoreAttrsForSchema(payload.CoreAttrs, et.Schema)
		if err != nil {
			s.logger.Error(ctx, "SCIM ReplaceUser: failed to parse user type schema", log.Error(err))
			return nil, &scim.ErrorInternalServer
		}
		if svcErr := mergeReverseMappedCoreAttrs(payload.ExtensionAttrs, reverseMapped); svcErr != nil {
			s.logger.Debug(ctx, "SCIM ReplaceUser: conflicting value between core and custom schema",
				log.Any("error", svcErr))
			return nil, svcErr
		}
	}
	missing, err := missingRequiredAttrs(payload.ExtensionAttrs, et.Schema, true)
	if err != nil {
		s.logger.Error(ctx, "SCIM ReplaceUser: failed to parse user type schema", log.Error(err))
		return nil, &scim.ErrorInternalServer
	}
	if len(missing) > 0 {
		s.logger.Debug(ctx, "SCIM ReplaceUser: missing required attributes for user type",
			log.String("userType", resolvedUserTypeName), log.Any("missing", missing))
		return nil, scim.NewMissingRequiredAttributesError(resolvedUserTypeName, missing)
	}
	undeclared, err := undeclaredAttrs(payload.ExtensionAttrs, et.Schema)
	if err != nil {
		s.logger.Error(ctx, "SCIM ReplaceUser: failed to parse user type schema", log.Error(err))
		return nil, &scim.ErrorInternalServer
	}
	if len(undeclared) > 0 {
		s.logger.Debug(ctx, "SCIM ReplaceUser: undeclared attributes for user type",
			log.String("userType", resolvedUserTypeName), log.Any("undeclared", undeclared))
		return nil, scim.NewUndeclaredAttributesError(resolvedUserTypeName, undeclared)
	}
	attrsJSON, err := json.Marshal(payload.ExtensionAttrs)
	if err != nil {
		s.logger.Error(ctx, "SCIM ReplaceUser: failed to marshal extension attrs", log.Error(err))
		return nil, &scim.ErrorInvalidRequestBody
	}
	var result *user.User
	if isSelf {
		// Self-service replace: type and OU can't change (enforced above), so this
		// goes through the same attribute-only update path as native /users/me,
		// skipping user.UpdateUser's OU/type validation. That validation requires
		// system:usertype:view with no self-access bypass, so self-service callers
		// would otherwise hit it and fail.
		result, svcErr = s.userService.UpdateUserAttributes(ctx, userID, attrsJSON)
	} else {
		updatedUser := &user.User{
			ID:         userID,
			OUID:       et.OUID,
			Type:       resolvedUserTypeName,
			Attributes: attrsJSON,
		}
		result, svcErr = s.userService.UpdateUser(ctx, userID, updatedUser)
	}
	if svcErr != nil {
		s.logger.Error(ctx, "SCIM ReplaceUser: user service error",
			log.String("userID", userID), log.Any("error", svcErr))
		return nil, mapUserServiceErrorToSCIM(svcErr)
	}

	extensionURN := scim.BuildSchemaURN(result.Type)
	credKeys, svcErr := s.getCredentialKeys(ctx, resolvedUserTypeName)
	if svcErr != nil {
		return nil, svcErr
	}
	scimUser := buildSCIMUserResource(ctx, s.logger, *result, extensionURN, baseURL, credKeys, len(payload.CoreAttrs) > 0)
	return &scimUser, nil
}

// DeleteUser deletes a user by ID.
func (s *scimUsersService) DeleteUser(ctx context.Context, userID string, ifMatch string) *tidcommon.ServiceError {
	if trimmed := strings.TrimSpace(ifMatch); trimmed != "" {
		existingUser, svcErr := s.userService.GetUser(ctx, userID, false)
		if svcErr != nil {
			s.logger.Error(ctx, "SCIM DeleteUser: user service error",
				log.String("userID", userID), log.Any("error", svcErr))
			return mapUserServiceErrorToSCIM(svcErr)
		}
		if svcErr := scim.CheckIfMatch(trimmed, scim.GenerateVersion(userVersionState(*existingUser))); svcErr != nil {
			return svcErr
		}
	}

	svcErr := s.userService.DeleteUser(ctx, userID)
	if svcErr != nil {
		s.logger.Error(ctx, "SCIM DeleteUser: user service error",
			log.String("userID", userID), log.Any("error", svcErr))
		return mapUserServiceErrorToSCIM(svcErr)
	}
	return nil
}

// getCredentialKeys returns a set of attribute names that represent credentials for the given user type.
func (s *scimUsersService) getCredentialKeys(
	ctx context.Context, resolvedUserTypeName string,
) (map[string]struct{}, *tidcommon.ServiceError) {
	credKeys := make(map[string]struct{})
	// Use elevated runtime context if necessary, but we are just reading schema info.
	credentialInfos, err := s.userTypeService.GetAttributes(security.WithRuntimeContext(ctx),
		entitytype.TypeCategoryUser, resolvedUserTypeName,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: false, RequiredOnly: false})

	if err != nil {
		s.logger.Error(ctx, "SCIM: failed to resolve credential attribute keys",
			log.String("userType", resolvedUserTypeName), log.Any("error", err))
		return nil, &scim.ErrorInternalServer
	}
	for _, info := range credentialInfos {
		credKeys[info.Attribute] = struct{}{}
	}
	return credKeys, nil
}

// mergeReverseMappedCoreAttrs merges core-mapped attribute values (reverse-mapped from the
// top-level SCIM core fields) into the extension attrs map. If the same attribute is already
// present in the extension object with a different value, this is a conflicting-value error
// rather than a silent overwrite - the client supplied two different values for the same
// underlying attribute through the core and custom channels, and one of them would otherwise
// be silently discarded.
func mergeReverseMappedCoreAttrs(
	extensionAttrs map[string]json.RawMessage, reverseMapped map[string]json.RawMessage,
) *tidcommon.ServiceError {
	for k, v := range reverseMapped {
		existing, exists := extensionAttrs[k]
		if !exists {
			extensionAttrs[k] = v
			continue
		}
		if !jsonRawValuesEqual(existing, v) {
			return scim.NewConflictingAttributeValueError(k)
		}
	}
	return nil
}

// jsonRawValuesEqual reports whether two JSON-encoded values are semantically equal,
// regardless of formatting differences (whitespace, key order for objects). Falls back
// to a byte comparison if either value fails to unmarshal.
func jsonRawValuesEqual(a, b json.RawMessage) bool {
	var av, bv interface{}
	if err := json.Unmarshal(a, &av); err != nil {
		return string(a) == string(b)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return string(a) == string(b)
	}
	return reflect.DeepEqual(av, bv)
}

// mapUserServiceErrorToSCIM translates a user service error into a SCIM package error.
func mapUserServiceErrorToSCIM(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr == nil {
		return nil
	}
	switch svcErr.Code {
	case user.ErrorUserNotFound.Code:
		return &scim.ErrorUserNotFound
	case user.ErrorAttributeConflict.Code:
		return &scim.ErrorUniquenessConflict
	case user.ErrorSchemaValidationFailed.Code:
		return &scim.ErrorSchemaValidationFailed
	case user.ErrorEntityTypeNotFound.Code:
		return &scim.ErrorUnknownUserType
	case user.ErrorCannotModifyDeclarativeResource.Code:
		return &scim.ErrorMutabilityViolation
	case tidcommon.ErrorUnauthorized.Code:
		return svcErr
	default:
		if svcErr.Type == tidcommon.ServerErrorType {
			return &tidcommon.InternalServerError
		}
		return &scim.ErrorInvalidRequestBody
	}
}
