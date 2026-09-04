// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"strings"

	"github.com/thunder-id/thunderid/internal/entitytype"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ResolveUserTypeNameForSchemaURN searches all user types for one
// whose name matches userTypeName (case-insensitive). Returns the resolved,
// correctly-cased name and nil on success, or empty string and nil if no match is found.
func ResolveUserTypeNameForSchemaURN(
	ctx context.Context, userTypeService entitytype.EntityTypeServiceInterface, userTypeName string,
) (string, *tidcommon.ServiceError) {
	offset := 0
	for {
		page, svcErr := userTypeService.GetEntityTypeList(
			ctx, entitytype.TypeCategoryUser, serverconst.MaxPageSize, offset, false,
		)
		if svcErr != nil {
			if svcErr.Type == tidcommon.ServerErrorType {
				return "", &ErrorInternalServer
			}
			return "", &ErrorSchemaNotFound
		}

		for _, item := range page.Types {
			if strings.EqualFold(item.Name, userTypeName) {
				return item.Name, nil
			}
		}

		offset += len(page.Types)
		if offset >= page.TotalResults || len(page.Types) == 0 {
			return "", nil
		}
	}
}

// ResolveCoreUserType resolves the ThunderID user type name that backs the SCIM core User
// schema (RFC 7643 §4.1): the source of its declared attribute characteristics in
// /scim2/Schemas, and the default target for payloads that carry only the core schema URN.
//
// If coreUserTypeID is set, it is resolved directly by ID. Otherwise this falls back to
// ResolveDefaultUserTypeName, preserving today's behavior of defaulting to the sole
// configured user type. Returns ErrorMissingCustomSchema (via ResolveDefaultUserTypeName)
// when no core user type can be determined — write-path callers should treat that as an
// error, while discovery callers may treat it as "core schema unavailable" and degrade
// gracefully instead of failing the whole request.
func ResolveCoreUserType(
	ctx context.Context, userTypeService entitytype.EntityTypeServiceInterface, coreUserTypeID string,
) (string, *tidcommon.ServiceError) {
	if coreUserTypeID == "" {
		return ResolveDefaultUserTypeName(ctx, userTypeService)
	}
	et, svcErr := userTypeService.GetEntityType(ctx, entitytype.TypeCategoryUser, coreUserTypeID, false)
	if svcErr != nil {
		return "", MapEntityTypeServiceErrorToSCIM(svcErr)
	}
	return et.Name, nil
}

// ResolveDefaultUserTypeName returns the sole configured user type's
// resolved name, for SCIM payloads that carry only core attributes and omit
// the ThunderID extension URN. Errors if zero or more than one user type is
// configured, since the default type is then ambiguous.
func ResolveDefaultUserTypeName(
	ctx context.Context, userTypeService entitytype.EntityTypeServiceInterface,
) (string, *tidcommon.ServiceError) {
	page, svcErr := userTypeService.GetEntityTypeList(
		ctx, entitytype.TypeCategoryUser, serverconst.MaxPageSize, 0, false)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return "", &ErrorInternalServer
		}
		return "", &ErrorMissingCustomSchema
	}
	if page.TotalResults != 1 || len(page.Types) != 1 {
		return "", &ErrorMissingCustomSchema
	}
	return page.Types[0].Name, nil
}
