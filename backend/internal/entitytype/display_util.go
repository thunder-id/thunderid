// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ResolveDisplayAttributePaths collects the unique entity type handles and resolves their display
// attribute paths within the given category. Returns nil if there are no types to resolve or if the
// lookup fails, so that callers fall back to the entity ID.
func ResolveDisplayAttributePaths(
	ctx context.Context, category TypeCategory, typeHandles []string, schemaService EntityTypeServiceInterface,
	logger *log.Logger,
) map[string]string {
	if schemaService == nil || len(typeHandles) == 0 {
		return nil
	}

	uniqueTypes := utils.UniqueNonEmptyStrings(typeHandles)
	if len(uniqueTypes) == 0 {
		return nil
	}

	displayPaths, svcErr := schemaService.GetDisplayAttributesByHandles(ctx, category, uniqueTypes)
	if svcErr != nil {
		if logger != nil {
			logger.Warn(ctx, "Failed to resolve display attribute paths, skipping display resolution",
				log.Any("error", svcErr))
		}
		return nil
	}

	return displayPaths
}

// ResolveEntityDisplays resolves the display value of each user and agent entity, keyed by entity ID.
// Display paths are looked up once per category, so a failure in one category leaves the other intact.
// Entities of other categories are not included.
func ResolveEntityDisplays(
	ctx context.Context, entities []providers.Entity, schemaService EntityTypeServiceInterface, logger *log.Logger,
) map[string]string {
	typeHandles := map[TypeCategory][]string{}
	for _, e := range entities {
		if category, ok := typeCategoryOf(e.Category); ok {
			typeHandles[category] = append(typeHandles[category], e.Type)
		}
	}

	displayPaths := make(map[TypeCategory]map[string]string, len(typeHandles))
	for category, handles := range typeHandles {
		displayPaths[category] = ResolveDisplayAttributePaths(ctx, category, handles, schemaService, logger)
	}

	displays := make(map[string]string, len(entities))
	for _, e := range entities {
		if category, ok := typeCategoryOf(e.Category); ok {
			displays[e.ID] = utils.ResolveDisplay(e.ID, e.Type, e.Attributes, displayPaths[category])
		}
	}

	return displays
}

func typeCategoryOf(category providers.EntityCategory) (TypeCategory, bool) {
	switch category {
	case providers.EntityCategoryUser:
		return TypeCategoryUser, true
	case providers.EntityCategoryAgent:
		return TypeCategoryAgent, true
	default:
		return "", false
	}
}
