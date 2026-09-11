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
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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
		ctx context.Context, userID string, payload *SCIMUserPayload, baseURL string, isSelf bool,
	) (*SCIMUser, *tidcommon.ServiceError)
	DeleteUser(ctx context.Context, userID string) *tidcommon.ServiceError
	ValidateAttributePaths(ctx context.Context, attributes, excludedAttributes []string) *tidcommon.ServiceError
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
	schemaPropsByType := make(map[string]map[string]scim.RawPropertyDef)
	unresolvedTypes := make(map[string]struct{})
	for _, u := range listResp.Users {
		if _, unresolved := unresolvedTypes[u.Type]; unresolved {
			continue
		}
		rawProps, ok := schemaPropsByType[u.Type]
		if !ok {
			var svcErr *tidcommon.ServiceError
			rawProps, svcErr = s.getSchemaProps(ctx, u.Type)
			if svcErr != nil {
				s.logger.Warn(ctx, "SCIM ListUsers: omitting user with unresolvable user type",
					log.String("userID", u.ID), log.String("userType", u.Type))
				unresolvedTypes[u.Type] = struct{}{}
				continue
			}
			schemaPropsByType[u.Type] = rawProps
		}
		extensionURN := scim.BuildSchemaURN(u.Type)
		scimUsers = append(scimUsers, buildSCIMUserResource(
			ctx, s.logger, u, extensionURN, baseURL, rawProps, s.cfg.ReturnMappedCoreAttrsOnGet))
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
	rawProps, svcErr := s.getSchemaProps(ctx, u.Type)
	if svcErr != nil {
		return nil, svcErr
	}
	scimUser := buildSCIMUserResource(
		ctx, s.logger, *u, extensionURN, baseURL, rawProps, s.cfg.ReturnMappedCoreAttrsOnGet)
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
		return nil, scim.BuildUserTypeErrorToSCIM(svcErr)
	}

	if svcErr := s.processInboundPayload(
		runtimeCtx, payload, et, resolvedUserTypeName, false, payload.UserTypeName == ""); svcErr != nil {
		return nil, svcErr
	}
	attrsJSON, err := json.Marshal(payload.ExtensionAttrs)
	if err != nil {
		s.logger.Error(ctx, "SCIM CreateUser: failed to marshal extension attrs", log.Error(err))
		return nil, &scim.ErrorInvalidRequestBody
	}
	newUser := &providers.User{
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
	// et was already fetched above for processInboundPayload, so its schema is reused here
	// rather than re-fetching the entity type a second time.
	rawProps, parseErr := scim.ParseRawProperties(et.Schema)
	if parseErr != nil {
		s.logger.Error(ctx, "SCIM CreateUser: failed to parse user type schema",
			log.String("userType", resolvedUserTypeName), log.Error(parseErr))
		return nil, &tidcommon.InternalServerError
	}
	scimUser := buildSCIMUserResource(
		ctx, s.logger, *created, extensionURN, baseURL, rawProps,
		len(payload.CoreAttrs) > 0 || payload.HasEnterpriseSchema)
	return &scimUser, nil
}

// ReplaceUser performs a full PUT replace on the user.
func (s *scimUsersService) ReplaceUser(
	ctx context.Context, userID string, payload *SCIMUserPayload, baseURL string, isSelf bool,
) (*SCIMUser, *tidcommon.ServiceError) {
	runtimeCtx := security.WithRuntimeContext(ctx)

	existingUser, svcErr := s.userService.GetUser(ctx, userID, false)
	if svcErr != nil {
		s.logger.Debug(ctx, "SCIM ReplaceUser: user service error",
			log.String("userID", userID), log.Any("error", svcErr))
		return nil, mapUserServiceErrorToSCIM(svcErr)
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
		return nil, scim.BuildUserTypeErrorToSCIM(svcErr)
	}
	if svcErr := s.processInboundPayload(runtimeCtx, payload, et, resolvedUserTypeName, true, false); svcErr != nil {
		return nil, svcErr
	}
	attrsJSON, err := json.Marshal(payload.ExtensionAttrs)
	if err != nil {
		s.logger.Error(ctx, "SCIM ReplaceUser: failed to marshal extension attrs", log.Error(err))
		return nil, &scim.ErrorInvalidRequestBody
	}
	var result *providers.User
	if isSelf {
		// Self-service replace: type and OU can't change (enforced above), so this
		// goes through the same attribute-only update path as native /users/me,
		// skipping user.UpdateUser's OU/type validation. That validation requires
		// system:usertype:view with no self-access bypass, so self-service callers
		// would otherwise hit it and fail.
		result, svcErr = s.userService.UpdateUserAttributes(ctx, userID, attrsJSON)
	} else {
		updatedUser := &providers.User{
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
	// et was already fetched above for processInboundPayload, so its schema is reused here
	// rather than re-fetching the entity type a second time.
	rawProps, parseErr := scim.ParseRawProperties(et.Schema)
	if parseErr != nil {
		s.logger.Error(ctx, "SCIM ReplaceUser: failed to parse user type schema",
			log.String("userType", resolvedUserTypeName), log.Error(parseErr))
		return nil, &tidcommon.InternalServerError
	}
	scimUser := buildSCIMUserResource(
		ctx, s.logger, *result, extensionURN, baseURL, rawProps,
		len(payload.CoreAttrs) > 0 || payload.HasEnterpriseSchema)
	return &scimUser, nil
}

// DeleteUser deletes a user by ID.
func (s *scimUsersService) DeleteUser(ctx context.Context, userID string) *tidcommon.ServiceError {
	svcErr := s.userService.DeleteUser(ctx, userID)
	if svcErr != nil {
		s.logger.Error(ctx, "SCIM DeleteUser: user service error",
			log.String("userID", userID), log.Any("error", svcErr))
		return mapUserServiceErrorToSCIM(svcErr)
	}
	return nil
}

// ValidateAttributePaths enforces RFC 7644 §3.9: "attributes" and "excludedAttributes" are
// mutually exclusive, and every attribute path must resolve to a known core/enterprise
// attribute or a registered custom schema URN.
func (s *scimUsersService) ValidateAttributePaths(
	ctx context.Context, attributes, excludedAttributes []string,
) *tidcommon.ServiceError {
	if len(attributes) > 0 && len(excludedAttributes) > 0 {
		return &scim.ErrorConflictingAttributesParams
	}
	// At most one of the two is non-empty at this point, so a single pass covers both.
	for _, attr := range append(attributes, excludedAttributes...) {
		if svcErr := s.validateAttributePath(ctx, attr); svcErr != nil {
			return svcErr
		}
	}
	return nil
}

// validateAttributePath rejects a bare custom attribute path not qualified with its schema
// URN, a dotted sub-attribute path (projection only supports root-level attributes), and a
// schema URN prefix that does not resolve to an actually-registered user type.
func (s *scimUsersService) validateAttributePath(ctx context.Context, attr string) *tidcommon.ServiceError {
	if strings.EqualFold(attr, scim.SCIMCoreUserSchemaURN) ||
		strings.EqualFold(attr, scim.SCIMEnterpriseUserSchemaURN) {
		return nil
	}
	if _, ok := scim.ParseUserTypeFromSchemaURN(attr); ok {
		return nil
	}

	lower := strings.ToLower(attr)
	corePrefix := strings.ToLower(scim.SCIMCoreUserSchemaURN) + ":"
	entPrefix := strings.ToLower(scim.SCIMEnterpriseUserSchemaURN) + ":"

	switch {
	case strings.HasPrefix(lower, corePrefix):
		field := attr[len(corePrefix):]
		if strings.Contains(field, ".") {
			return scim.NewSubAttrProjectionError(attr)
		}
		if isCoreSCIMAttrPath(field) {
			return nil
		}
		return scim.NewUnrecognizedAttributeError(attr)
	case strings.HasPrefix(lower, entPrefix):
		field := attr[len(entPrefix):]
		if strings.Contains(field, ".") {
			return scim.NewSubAttrProjectionError(attr)
		}
		if isEnterpriseSCIMAttrPath(field) {
			return nil
		}
		return scim.NewUnrecognizedAttributeError(attr)
	}

	if idx := strings.LastIndex(attr, ":"); idx >= 0 {
		urn := attr[:idx]
		field := attr[idx+1:]
		if strings.Contains(field, ".") {
			return scim.NewSubAttrProjectionError(attr)
		}
		userTypeName, ok := scim.ParseUserTypeFromSchemaURN(urn)
		if !ok {
			return scim.NewUnrecognizedSchemaURNError(attr)
		}
		resolved, svcErr := scim.ResolveUserTypeNameForSchemaURN(ctx, s.userTypeService, userTypeName)
		if svcErr != nil {
			return svcErr
		}
		if resolved == "" {
			return scim.NewUnrecognizedSchemaURNError(attr)
		}
		return nil
	}

	if strings.Contains(attr, ".") {
		return scim.NewSubAttrProjectionError(attr)
	}
	if isCoreSCIMAttrPath(attr) {
		return nil
	}
	return scim.NewCustomAttributeRequiresURNError(attr)
}

// getSchemaProps returns the parsed schema property definitions for the given user type.
func (s *scimUsersService) getSchemaProps(
	ctx context.Context, resolvedUserTypeName string,
) (map[string]scim.RawPropertyDef, *tidcommon.ServiceError) {
	// Use elevated runtime context if necessary, but we are just reading schema info.
	et, err := s.userTypeService.GetEntityTypeByName(
		security.WithRuntimeContext(ctx), entitytype.TypeCategoryUser, resolvedUserTypeName)
	if err != nil {
		s.logger.Error(ctx, "SCIM: failed to resolve user type schema",
			log.String("userType", resolvedUserTypeName), log.Any("error", err))
		return nil, &tidcommon.InternalServerError
	}
	rawProps, parseErr := scim.ParseRawProperties(et.Schema)
	if parseErr != nil {
		s.logger.Error(ctx, "SCIM: failed to parse user type schema",
			log.String("userType", resolvedUserTypeName), log.Error(parseErr))
		return nil, &tidcommon.InternalServerError
	}
	return rawProps, nil
}

// processInboundPayload reverse-maps core and enterprise attributes into payload.ExtensionAttrs
// and validates them against the entity type schema.
func (s *scimUsersService) processInboundPayload(
	ctx context.Context, payload *SCIMUserPayload, et *entitytype.EntityType,
	resolvedUserTypeName string, isReplace bool, isCoreUserType bool,
) *tidcommon.ServiceError {
	if len(payload.CoreAttrs) > 0 {
		reverseMapped, err := reverseMapCoreAttrsForSchema(payload.CoreAttrs, et.Schema)
		if err != nil {
			s.logger.Error(ctx, "SCIM: failed to parse user type schema", log.Error(err))
			return &tidcommon.InternalServerError
		}
		if svcErr := mergeReverseMappedAttrs(payload.ExtensionAttrs, reverseMapped); svcErr != nil {
			s.logger.Debug(ctx, "SCIM: conflicting value between core and custom schema", log.Any("error", svcErr))
			return svcErr
		}
	}
	if payload.HasEnterpriseSchema {
		if !isCoreUserType {
			targetCoreTypeName, err := scim.ResolveCoreUserType(ctx, s.userTypeService, s.cfg.CoreUserTypeID)
			if err != nil || !strings.EqualFold(resolvedUserTypeName, targetCoreTypeName) {
				s.logger.Debug(ctx, "SCIM: enterprise schema not supported for user type",
					log.String("userType", resolvedUserTypeName))
				return &scim.ErrorEnterpriseSchemaNotSupported
			}
		}
		if len(payload.EnterpriseAttrs) > 0 {
			reverseMappedEnt, undeclaredEnt, err := reverseMapEnterpriseAttrsForSchema(
				payload.EnterpriseAttrs, et.Schema)
			if err != nil {
				s.logger.Error(ctx, "SCIM: failed to parse user type schema for enterprise attrs", log.Error(err))
				return &tidcommon.InternalServerError
			}
			if len(undeclaredEnt) > 0 {
				s.logger.Debug(ctx, "SCIM: undeclared enterprise attributes for user type",
					log.String("userType", resolvedUserTypeName), log.Any("undeclared", undeclaredEnt))
				return scim.NewUndeclaredAttributesError(resolvedUserTypeName, undeclaredEnt)
			}
			if svcErr := mergeReverseMappedAttrs(payload.ExtensionAttrs, reverseMappedEnt); svcErr != nil {
				s.logger.Debug(ctx, "SCIM: conflicting value between enterprise and custom schema",
					log.Any("error", svcErr))
				return svcErr
			}
		}
	}
	missing, err := missingRequiredAttrs(payload.ExtensionAttrs, et.Schema, isReplace)
	if err != nil {
		s.logger.Error(ctx, "SCIM: failed to parse user type schema", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if len(missing) > 0 {
		s.logger.Debug(ctx, "SCIM: missing required attributes for user type",
			log.String("userType", resolvedUserTypeName), log.Any("missing", missing))
		return scim.NewMissingRequiredAttributesError(resolvedUserTypeName, missing)
	}
	undeclared, err := undeclaredAttrs(payload.ExtensionAttrs, et.Schema)
	if err != nil {
		s.logger.Error(ctx, "SCIM: failed to parse user type schema", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if len(undeclared) > 0 {
		s.logger.Debug(ctx, "SCIM: undeclared attributes for user type",
			log.String("userType", resolvedUserTypeName), log.Any("undeclared", undeclared))
		return scim.NewUndeclaredAttributesError(resolvedUserTypeName, undeclared)
	}
	return nil
}

// mergeReverseMappedAttrs merges reverse-mapped attributes into extensionAttrs.
// Returns a conflict error if the same attribute already exists with a different value.
func mergeReverseMappedAttrs(
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

// jsonRawValuesEqual reports whether two JSON-encoded values are semantically equal.
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
