// Copyright 2026 The ThunderID Authors
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
				return "", &tidcommon.InternalServerError
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

// ResolveCoreUserType resolves the user type name backing the SCIM core User schema (RFC 7643 §4.1):
// the type flagged as the SCIM core type, or the sole configured user type when none is flagged.
// Returns ErrorMissingCustomSchema (via ResolveDefaultUserTypeName) when no core type can be
// determined. Discovery callers may treat that as "core schema unavailable" and degrade
// gracefully, while write-path callers should treat it as an error.
func ResolveCoreUserType(
	ctx context.Context, userTypeService entitytype.EntityTypeServiceInterface,
) (string, *tidcommon.ServiceError) {
	offset := 0
	for {
		page, svcErr := userTypeService.GetEntityTypeList(
			ctx, entitytype.TypeCategoryUser, serverconst.MaxPageSize, offset, false)
		if svcErr != nil {
			if svcErr.Type == tidcommon.ServerErrorType {
				return "", &tidcommon.InternalServerError
			}
			return "", &ErrorMissingCustomSchema
		}
		for _, item := range page.Types {
			if item.SystemAttributes != nil && item.SystemAttributes.IsScimCoreType {
				return item.Name, nil
			}
		}
		offset += len(page.Types)
		if offset >= page.TotalResults || len(page.Types) == 0 {
			if page.TotalResults == 1 && len(page.Types) == 1 {
				return page.Types[0].Name, nil
			}
			return "", &ErrorMissingCustomSchema
		}
	}
}

// ResolveCoreUserTypeRules builds the SCIM attribute rules from the core user type's stored mapping.
// A core type without a mapping has no rules.
func ResolveCoreUserTypeRules(
	ctx context.Context, userTypeService entitytype.EntityTypeServiceInterface,
) ([]CoreAttrRule, []EnterpriseAttrRule, *tidcommon.ServiceError) {
	name, svcErr := ResolveCoreUserType(ctx, userTypeService)
	if svcErr != nil {
		return nil, nil, svcErr
	}
	et, svcErr := userTypeService.GetEntityTypeByName(ctx, entitytype.TypeCategoryUser, name)
	if svcErr != nil {
		return nil, nil, BuildUserTypeErrorToSCIM(svcErr)
	}
	if et.SystemAttributes == nil || et.SystemAttributes.ScimMapping == nil {
		return nil, nil, nil
	}
	core, enterprise := BuildRulesFromMapping(
		et.SystemAttributes.ScimMapping.AttributeMap, et.SystemAttributes.ScimMapping.MultiValuedMeta)
	return core, enterprise, nil
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
			return "", &tidcommon.InternalServerError
		}
		return "", &ErrorMissingCustomSchema
	}
	if page.TotalResults != 1 || len(page.Types) != 1 {
		return "", &ErrorMissingCustomSchema
	}
	return page.Types[0].Name, nil
}
