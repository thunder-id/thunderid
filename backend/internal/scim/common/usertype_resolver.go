// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"

	"github.com/thunder-id/thunderid/internal/entitytype"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ResolveUserTypeForSchemaURN looks up the user type with the handle extracted from a ThunderID
// extension URN. Returns the user type and nil on success, or nil and nil if no user type has it.
func ResolveUserTypeForSchemaURN(
	ctx context.Context, userTypeService entitytype.EntityTypeServiceInterface, userTypeHandle string,
) (*entitytype.EntityType, *tidcommon.ServiceError) {
	et, svcErr := userTypeService.GetEntityTypeByHandle(ctx, entitytype.TypeCategoryUser, userTypeHandle)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, &tidcommon.InternalServerError
		}
		if svcErr.Code == entitytype.ErrorEntityTypeNotFound.Code {
			return nil, nil
		}
		return nil, &ErrorSchemaNotFound
	}
	return et, nil
}

// ResolveCoreUserType resolves the ThunderID user type handle that backs the SCIM core User
// schema (RFC 7643 §4.1): the source of its declared attribute characteristics in
// /scim/v2/Schemas, and the default target for payloads that carry only the core schema URN.
//
// If coreUserTypeID is set, it is resolved directly by ID. Otherwise this falls back to
// ResolveDefaultUserTypeHandle, preserving today's behavior of defaulting to the sole
// configured user type. Returns ErrorMissingCustomSchema (via ResolveDefaultUserTypeHandle)
// when no core user type can be determined — write-path callers should treat that as an
// error, while discovery callers may treat it as "core schema unavailable" and degrade
// gracefully instead of failing the whole request.
func ResolveCoreUserType(
	ctx context.Context, userTypeService entitytype.EntityTypeServiceInterface, coreUserTypeID string,
) (string, *tidcommon.ServiceError) {
	if coreUserTypeID == "" {
		return ResolveDefaultUserTypeHandle(ctx, userTypeService)
	}
	et, svcErr := userTypeService.GetEntityType(ctx, entitytype.TypeCategoryUser, coreUserTypeID, false)
	if svcErr != nil {
		return "", BuildUserTypeErrorToSCIM(svcErr)
	}
	return et.Handle, nil
}

// ResolveDefaultUserTypeHandle returns the sole configured user type's
// handle, for SCIM payloads that carry only core attributes and omit
// the ThunderID extension URN. Errors if zero or more than one user type is
// configured, since the default type is then ambiguous.
func ResolveDefaultUserTypeHandle(
	ctx context.Context, userTypeService entitytype.EntityTypeServiceInterface,
) (string, *tidcommon.ServiceError) {
	page, svcErr := userTypeService.GetEntityTypeList(
		ctx, entitytype.TypeCategoryUser, serverconst.MaxPageSize, 0, false)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return "", &tidcommon.InternalServerError
		}
		return "", &ErrorMissingCustomSchema
	}
	if page.TotalResults != 1 || len(page.Types) != 1 {
		return "", &ErrorMissingCustomSchema
	}
	return page.Types[0].Handle, nil
}
