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

// ResolveCoreUserType resolves the user type handle backing the SCIM core User schema (RFC 7643 §4.1):
// the type flagged as the SCIM core type, or the sole configured user type when none is flagged.
// Returns ErrorMissingCustomSchema when no core type can be determined. Discovery callers may treat
// that as "core schema unavailable" and degrade gracefully, while write-path callers should treat it
// as an error.
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
				return item.Handle, nil
			}
		}
		offset += len(page.Types)
		if offset >= page.TotalResults || len(page.Types) == 0 {
			if page.TotalResults == 1 && len(page.Types) == 1 {
				return page.Types[0].Handle, nil
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
	handle, svcErr := ResolveCoreUserType(ctx, userTypeService)
	if svcErr != nil {
		return nil, nil, svcErr
	}
	et, svcErr := userTypeService.GetEntityTypeByHandle(ctx, entitytype.TypeCategoryUser, handle)
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
