// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// ResolveDisplayAttributePaths collects unique user types and resolves their display
// attribute paths from the entity type service.
// Returns nil if there are no types to resolve or if the lookup fails.
func ResolveDisplayAttributePaths(
	ctx context.Context, userTypes []string, schemaService entitytype.EntityTypeServiceInterface,
	logger *log.Logger,
) map[string]string {
	if schemaService == nil || len(userTypes) == 0 {
		return nil
	}

	uniqueTypes := utils.UniqueNonEmptyStrings(userTypes)
	if len(uniqueTypes) == 0 {
		return nil
	}

	displayPaths, svcErr := schemaService.GetDisplayAttributesByHandles(ctx, entitytype.TypeCategoryUser, uniqueTypes)
	if svcErr != nil {
		if logger != nil {
			logger.Warn(ctx, "Failed to resolve display attribute paths, skipping display resolution",
				log.Any("error", svcErr))
		}
		return nil
	}

	return displayPaths
}
