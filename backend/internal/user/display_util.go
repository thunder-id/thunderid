/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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

	displayPaths, svcErr := schemaService.GetDisplayAttributesByNames(ctx, entitytype.TypeCategoryUser, uniqueTypes)
	if svcErr != nil {
		if logger != nil {
			logger.Warn("Failed to resolve display attribute paths, skipping display resolution",
				log.Any("error", svcErr))
		}
		return nil
	}

	return displayPaths
}
