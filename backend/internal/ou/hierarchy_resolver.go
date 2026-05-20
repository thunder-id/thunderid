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

package ou

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

const loggerComponentNameHierarchyResolver = "OUHierarchyResolver"

// ouHierarchyAdapter implements sysauthz.OUHierarchyResolver using direct store access.
// It intentionally bypasses the service layer (which applies authz checks) to avoid
// recursive authorization calls when the policy engine traverses the OU tree.
type ouHierarchyAdapter struct {
	store organizationUnitStoreInterface
}

// newOUHierarchyAdapter returns a new sysauthz.OUHierarchyResolver backed by the given store.
func newOUHierarchyAdapter(store organizationUnitStoreInterface) sysauthz.OUHierarchyResolver {
	return &ouHierarchyAdapter{store: store}
}

// IsAncestor returns true when ancestorOUID appears anywhere in the parent
// chain above descendantOUID.
//
// The walk is upward from descendantOUID; reaching the root without finding ancestorOUID
// returns false. A broken chain (OU not found) is treated as deny-safe: false is returned
// with no error so the authz layer can decide accordingly.
func (r *ouHierarchyAdapter) IsAncestor(
	ctx context.Context, ancestorOUID, descendantOUID string,
) (bool, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameHierarchyResolver))

	if ancestorOUID == "" || descendantOUID == "" {
		return false, nil
	}

	current := descendantOUID
	visited := make(map[string]struct{})
	for {
		if _, ok := visited[current]; ok {
			logger.Error("Cyclic organization unit parent chain detected during ancestry check",
				log.String("ouID", current))
			return false, nil
		}
		visited[current] = struct{}{}

		ou, err := r.store.GetOrganizationUnit(ctx, current)
		if err != nil {
			if errors.Is(err, ErrOrganizationUnitNotFound) {
				// Broken chain — cannot confirm ancestry; deny-safe.
				logger.Debug("Encountered missing organization unit during ancestry check",
					log.String("ouID", current))
				return false, nil
			}
			logger.Error("Failed to traverse organization unit hierarchy during ancestry check",
				log.Error(err))
			return false, &serviceerror.InternalServerError
		}

		if ou.Parent == nil {
			break
		}
		current = *ou.Parent
		if current == ancestorOUID {
			return true, nil
		}
	}

	return false, nil
}

// GetAncestorOUIDs returns every ancestor OU ID, walking up to
// the root of the tree.
//
// If the chain is broken (an OU in the hierarchy is not found), the walk stops and a
// ServiceError is returned so callers can handle incomplete results explicitly.
func (r *ouHierarchyAdapter) GetAncestorOUIDs(
	ctx context.Context, ouID string,
) ([]string, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentNameHierarchyResolver))

	if ouID == "" {
		return []string{}, nil
	}

	var result []string
	current := ouID
	visited := make(map[string]struct{})

	for {
		if _, ok := visited[current]; ok {
			logger.Error("Cyclic organization unit parent chain detected while collecting ancestors",
				log.String("ouID", current))
			return nil, &serviceerror.InternalServerError
		}
		visited[current] = struct{}{}

		ou, err := r.store.GetOrganizationUnit(ctx, current)
		if err != nil {
			if errors.Is(err, ErrOrganizationUnitNotFound) {
				logger.Debug("Encountered missing organization unit while collecting ancestors",
					log.String("ouID", current))
				return nil, &ErrorOrganizationUnitNotFound
			}
			logger.Error("Failed to traverse organization unit hierarchy while collecting ancestors",
				log.Error(err))
			return nil, &serviceerror.InternalServerError
		}

		if ou.Parent == nil {
			break
		}
		current = *ou.Parent
		result = append(result, current)
	}

	if result == nil {
		result = []string{}
	}

	return result, nil
}
