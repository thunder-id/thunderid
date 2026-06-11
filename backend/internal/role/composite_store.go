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

package role

import (
	"context"
	"errors"
	"fmt"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// compositeRoleStore implements a composite store that combines file-based (immutable) and
// database (mutable) stores.
// - Read operations query both stores and merge results
// - Write operations (Create/Update/Delete) only affect the database store
// - Declarative roles (from YAML files) cannot be modified or deleted
type compositeRoleStore struct {
	fileStore roleStoreInterface
	dbStore   roleStoreInterface
}

// newCompositeRoleStore creates a new composite store with both file-based and database stores.
func newCompositeRoleStore(fileStore, dbStore roleStoreInterface) roleStoreInterface {
	return &compositeRoleStore{
		fileStore: fileStore,
		dbStore:   dbStore,
	}
}

// GetRoleListCount retrieves the total count of unique roles across both stores.
func (c *compositeRoleStore) GetRoleListCount(ctx context.Context) (int, error) {
	capCount := func(fn func(context.Context) (int, error)) func() (int, error) {
		return func() (int, error) {
			count, err := fn(ctx)
			if err != nil {
				return 0, err
			}
			return min(count, serverconst.MaxCompositeStoreRecords), nil
		}
	}
	roles, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		capCount(c.dbStore.GetRoleListCount),
		capCount(c.fileStore.GetRoleListCount),
		func(count int) ([]Role, error) { return c.dbStore.GetRoleList(ctx, count, 0) },
		func(count int) ([]Role, error) { return c.fileStore.GetRoleList(ctx, count, 0) },
		mergeRoles,
		serverconst.MaxCompositeStoreRecords+1,
		0,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return 0, err
	}
	if limitExceeded {
		return 0, errResultLimitExceededInCompositeMode
	}

	return len(roles), nil
}

// GetRoleList retrieves roles from both stores and merges them.
func (c *compositeRoleStore) GetRoleList(ctx context.Context, limit, offset int) ([]Role, error) {
	capCount := func(fn func(context.Context) (int, error)) func() (int, error) {
		return func() (int, error) {
			count, err := fn(ctx)
			if err != nil {
				return 0, err
			}
			return min(count, serverconst.MaxCompositeStoreRecords), nil
		}
	}
	roles, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		capCount(c.dbStore.GetRoleListCount),
		capCount(c.fileStore.GetRoleListCount),
		func(count int) ([]Role, error) { return c.dbStore.GetRoleList(ctx, count, 0) },
		func(count int) ([]Role, error) { return c.fileStore.GetRoleList(ctx, count, 0) },
		mergeRoles,
		limit,
		offset,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, errResultLimitExceededInCompositeMode
	}
	return roles, nil
}

// CreateRole creates a new role in the database store only.
func (c *compositeRoleStore) CreateRole(ctx context.Context, id string, role RoleCreationDetail) error {
	return c.dbStore.CreateRole(ctx, id, role)
}

// GetRole retrieves a role from either store.
// Checks database store first, then falls back to file store.
func (c *compositeRoleStore) GetRole(ctx context.Context, id string) (RoleWithPermissions, error) {
	role, err := declarativeresource.CompositeGetHelper(
		func() (*RoleWithPermissions, error) {
			r, err := c.dbStore.GetRole(ctx, id)
			return &r, err
		},
		func() (*RoleWithPermissions, error) {
			r, err := c.fileStore.GetRole(ctx, id)
			return &r, err
		},
		ErrRoleNotFound,
	)
	if role != nil {
		return *role, err
	}
	return RoleWithPermissions{}, err
}

// IsRoleExist checks if a role exists in either store.
func (c *compositeRoleStore) IsRoleExist(ctx context.Context, id string) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) { return c.fileStore.IsRoleExist(ctx, id) },
		func() (bool, error) { return c.dbStore.IsRoleExist(ctx, id) },
	)
}

// GetRoleAssignments retrieves role assignments from both stores.
func (c *compositeRoleStore) GetRoleAssignments(
	ctx context.Context,
	id string,
	limit, offset int,
) ([]RoleAssignment, error) {
	return c.getCompositeAssignments(ctx, id, limit, offset,
		c.dbStore.GetRoleAssignmentsCount, c.fileStore.GetRoleAssignmentsCount,
		func(count int) ([]RoleAssignment, error) { return c.dbStore.GetRoleAssignments(ctx, id, count, 0) },
		func(count int) ([]RoleAssignment, error) { return c.fileStore.GetRoleAssignments(ctx, id, count, 0) },
	)
}

// GetRoleAssignmentsByType retrieves assignments filtered by assignee type across both stores.
func (c *compositeRoleStore) GetRoleAssignmentsByType(
	ctx context.Context,
	id string,
	limit, offset int,
	assigneeType string,
) ([]RoleAssignment, error) {
	return c.getCompositeAssignments(ctx, id, limit, offset,
		func(ctx context.Context, id string) (int, error) {
			return c.dbStore.GetRoleAssignmentsCountByType(ctx, id, assigneeType)
		},
		func(ctx context.Context, id string) (int, error) {
			return c.fileStore.GetRoleAssignmentsCountByType(ctx, id, assigneeType)
		},
		func(count int) ([]RoleAssignment, error) {
			return c.dbStore.GetRoleAssignmentsByType(ctx, id, count, 0, assigneeType)
		},
		func(count int) ([]RoleAssignment, error) {
			return c.fileStore.GetRoleAssignmentsByType(ctx, id, count, 0, assigneeType)
		},
	)
}

// getCompositeAssignments is the shared logic for merging assignments from both stores.
func (c *compositeRoleStore) getCompositeAssignments(
	ctx context.Context,
	id string,
	limit, offset int,
	dbCountFn, fileCountFn func(context.Context, string) (int, error),
	dbListFn, fileListFn func(int) ([]RoleAssignment, error),
) ([]RoleAssignment, error) {
	capCount := func(fn func(context.Context, string) (int, error)) func() (int, error) {
		return func() (int, error) {
			count, err := fn(ctx, id)
			if err != nil {
				return 0, err
			}
			return min(count, serverconst.MaxCompositeStoreRecords), nil
		}
	}
	assignments, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		capCount(dbCountFn),
		capCount(fileCountFn),
		dbListFn,
		fileListFn,
		mergeAssignments,
		limit,
		offset,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, errResultLimitExceededInCompositeMode
	}
	return assignments, nil
}

// GetRoleAssignmentsCount retrieves the count of unique role assignments across both stores.
func (c *compositeRoleStore) GetRoleAssignmentsCount(ctx context.Context, id string) (int, error) {
	return c.getCompositeAssignmentsCount(ctx, id,
		c.dbStore.GetRoleAssignmentsCount, c.fileStore.GetRoleAssignmentsCount,
		func(count int) ([]RoleAssignment, error) { return c.dbStore.GetRoleAssignments(ctx, id, count, 0) },
		func(count int) ([]RoleAssignment, error) { return c.fileStore.GetRoleAssignments(ctx, id, count, 0) },
	)
}

// GetRoleAssignmentsCountByType retrieves the count of unique role assignments filtered by type.
func (c *compositeRoleStore) GetRoleAssignmentsCountByType(
	ctx context.Context, id string, assigneeType string,
) (int, error) {
	return c.getCompositeAssignmentsCount(ctx, id,
		func(ctx context.Context, id string) (int, error) {
			return c.dbStore.GetRoleAssignmentsCountByType(ctx, id, assigneeType)
		},
		func(ctx context.Context, id string) (int, error) {
			return c.fileStore.GetRoleAssignmentsCountByType(ctx, id, assigneeType)
		},
		func(count int) ([]RoleAssignment, error) {
			return c.dbStore.GetRoleAssignmentsByType(ctx, id, count, 0, assigneeType)
		},
		func(count int) ([]RoleAssignment, error) {
			return c.fileStore.GetRoleAssignmentsByType(ctx, id, count, 0, assigneeType)
		},
	)
}

// getCompositeAssignmentsCount is the shared logic for counting merged assignments.
func (c *compositeRoleStore) getCompositeAssignmentsCount(
	ctx context.Context,
	id string,
	dbCountFn, fileCountFn func(context.Context, string) (int, error),
	dbListFn, fileListFn func(int) ([]RoleAssignment, error),
) (int, error) {
	capCount := func(fn func(context.Context, string) (int, error)) func() (int, error) {
		return func() (int, error) {
			count, err := fn(ctx, id)
			if err != nil {
				return 0, err
			}
			return min(count, serverconst.MaxCompositeStoreRecords), nil
		}
	}
	assignments, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		capCount(dbCountFn),
		capCount(fileCountFn),
		dbListFn,
		fileListFn,
		mergeAssignments,
		serverconst.MaxCompositeStoreRecords+1,
		0,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return 0, err
	}
	if limitExceeded {
		return 0, errResultLimitExceededInCompositeMode
	}

	count := len(assignments)
	if count > serverconst.MaxCompositeStoreRecords*9/10 {
		log.GetLogger().Warn(ctx,
			"Role assignment count approaches composite store limit; consider API pagination",
			log.String("id", id),
			log.Int("count", count),
			log.Int("limit", serverconst.MaxCompositeStoreRecords))
	}
	return count, nil
}

// UpdateRole updates a role in the database store only.
// Immutability checks are handled at the service layer.
func (c *compositeRoleStore) UpdateRole(ctx context.Context, id string, role RoleUpdateDetail) error {
	return c.dbStore.UpdateRole(ctx, id, role)
}

// DeleteRole deletes a role from the database store only.
// Immutability checks are handled at the service layer.
func (c *compositeRoleStore) DeleteRole(ctx context.Context, id string) error {
	return c.dbStore.DeleteRole(ctx, id)
}

// DeleteAssignmentsByRoleID deletes all assignments for a role from the database store only.
func (c *compositeRoleStore) DeleteAssignmentsByRoleID(ctx context.Context, id string) error {
	return c.dbStore.DeleteAssignmentsByRoleID(ctx, id)
}

// AddAssignments adds assignments to a role in the database store only.
func (c *compositeRoleStore) AddAssignments(ctx context.Context, id string, assignments []RoleAssignment) error {
	return c.dbStore.AddAssignments(ctx, id, assignments)
}

// RemoveAssignments removes assignments from a role in the database store only.
func (c *compositeRoleStore) RemoveAssignments(ctx context.Context, id string, assignments []RoleAssignment) error {
	return c.dbStore.RemoveAssignments(ctx, id, assignments)
}

// CheckRoleNameExists checks if a role with the given name exists in either store.
func (c *compositeRoleStore) CheckRoleNameExists(ctx context.Context, ouID, name string) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) { return c.fileStore.CheckRoleNameExists(ctx, ouID, name) },
		func() (bool, error) { return c.dbStore.CheckRoleNameExists(ctx, ouID, name) },
	)
}

// CheckRoleNameExistsExcludingID checks if a role exists excluding a specific ID in either store.
func (c *compositeRoleStore) CheckRoleNameExistsExcludingID(
	ctx context.Context,
	ouID, name, excludeRoleID string,
) (bool, error) {
	return declarativeresource.CompositeBooleanCheckHelper(
		func() (bool, error) {
			return c.fileStore.CheckRoleNameExistsExcludingID(ctx, ouID, name, excludeRoleID)
		},
		func() (bool, error) { return c.dbStore.CheckRoleNameExistsExcludingID(ctx, ouID, name, excludeRoleID) },
	)
}

// GetAuthorizedPermissions retrieves authorized permissions assembled from three sources:
//
//  1. dbStore — DB-managed roles, where both ROLE_PERMISSION and ROLE_ASSIGNMENT rows exist.
//     The single SQL INNER JOIN resolves these.
//  2. fileStore (static) — declarative roles whose YAML carries an explicit `assignments:`
//     list for the entity/group. Returned by fileStore.GetAuthorizedPermissions.
//  3. Cross-store (file/db) — declarative roles whose definition lives in the file store
//     but whose assignment was added at runtime via the role assignments API and therefore
//     lives in the DB. Neither store can answer this alone: dbStore drops the row in its
//     INNER JOIN against the (absent) ROLE_PERMISSION rows, and fileStore's matching only
//     consults the YAML-declared assignments. The composite resolves it by reading the
//     DB-stored role IDs for the entity, looking each role up in the file store, and
//     intersecting its permissions with the requested set.
func (c *compositeRoleStore) GetAuthorizedPermissions(
	ctx context.Context,
	entityID string,
	groupIDs []string,
	requestPermissions []string,
) ([]string, error) {
	if len(requestPermissions) == 0 {
		return []string{}, nil
	}

	dbPerms, err := c.dbStore.GetAuthorizedPermissions(ctx, entityID, groupIDs, requestPermissions)
	if err != nil {
		return nil, err
	}

	filePerms, err := c.fileStore.GetAuthorizedPermissions(ctx, entityID, groupIDs, requestPermissions)
	if err != nil {
		return nil, err
	}

	crossStorePerms, err := c.crossStoreAuthorizedPermissions(ctx, entityID, groupIDs, requestPermissions)
	if err != nil {
		return nil, err
	}

	return mergePermissions(mergePermissions(dbPerms, filePerms), crossStorePerms), nil
}

// crossStoreAuthorizedPermissions resolves permissions for the (declarative role definition
// in file store) + (runtime assignment row in DB) case. It is intentionally narrow: it skips
// any role ID that does not exist in the file store, because such roles are entirely DB-backed
// and were already covered by dbStore.GetAuthorizedPermissions.
func (c *compositeRoleStore) crossStoreAuthorizedPermissions(
	ctx context.Context,
	entityID string,
	groupIDs []string,
	requestPermissions []string,
) ([]string, error) {
	if entityID == "" && len(groupIDs) == 0 {
		return []string{}, nil
	}

	roleIDs, err := c.dbStore.GetEntityRoleIDs(ctx, entityID, groupIDs)
	if err != nil {
		return nil, err
	}
	if len(roleIDs) == 0 {
		return []string{}, nil
	}

	requestedSet := make(map[string]bool, len(requestPermissions))
	for _, p := range requestPermissions {
		requestedSet[p] = true
	}

	granted := make(map[string]bool)
	for _, id := range roleIDs {
		exists, err := c.fileStore.IsRoleExist(ctx, id)
		if err != nil {
			return nil, err
		}
		if !exists {
			// Role is DB-only; permissions already covered by dbStore.GetAuthorizedPermissions.
			continue
		}
		role, err := c.fileStore.GetRole(ctx, id)
		if err != nil {
			// Benign cases — skip silently:
			//   * ErrRoleNotFound: YAML was removed between assignment-time and lookup-time.
			//   * ErrRoleDataCorrupted: parse/type-assertion failure, already logged by fileStore.
			// Anything else is an actionable storage/IO error and must not be dropped:
			// authorization decisions made on a silently-empty permission set could
			// over- or under-authorize, so propagate with context.
			if errors.Is(err, ErrRoleNotFound) || errors.Is(err, ErrRoleDataCorrupted) {
				continue
			}
			log.GetLogger().Error(ctx,
				"Failed to load declarative role for cross-store permission resolution",
				log.String("roleID", id), log.Error(err))
			return nil, fmt.Errorf("composite role store: load declarative role %q: %w", id, err)
		}
		for _, rp := range role.Permissions {
			for _, perm := range rp.Permissions {
				if requestedSet[perm] {
					granted[perm] = true
				}
			}
		}
	}

	result := make([]string, 0, len(granted))
	for _, perm := range requestPermissions {
		if granted[perm] {
			result = append(result, perm)
		}
	}
	return result, nil
}

// GetEntityRoleIDs returns the IDs of roles assigned to an entity (directly or via groups).
// Delegates to the database store since assignments are persisted there even for declarative
// roles. The file store has no independent record of API-added assignments.
func (c *compositeRoleStore) GetEntityRoleIDs(
	ctx context.Context, entityID string, groupIDs []string,
) ([]string, error) {
	return c.dbStore.GetEntityRoleIDs(ctx, entityID, groupIDs)
}

// GetUserRoles retrieves role names assigned to an entity from both stores.
func (c *compositeRoleStore) GetUserRoles(
	ctx context.Context, entityID string, groupIDs []string,
) ([]string, error) {
	dbRoleNames, err := c.dbStore.GetUserRoles(ctx, entityID, groupIDs)
	if err != nil {
		return nil, err
	}

	fileRoleNames, err := c.fileStore.GetUserRoles(ctx, entityID, groupIDs)
	if err != nil {
		return nil, err
	}

	return mergePermissions(dbRoleNames, fileRoleNames), nil
}

// IsRoleDeclarative checks if a role is immutable (exists in file store).
func (c *compositeRoleStore) IsRoleDeclarative(ctx context.Context, roleID string) (bool, error) {
	fileExists, err := c.fileStore.IsRoleExist(ctx, roleID)
	if err != nil {
		return false, err
	}
	return fileExists, nil
}

// mergeRoles deduplicates and merges roles from database and file stores.
// Database roles take precedence over file-based roles with the same ID.
func mergeRoles(dbRoles, fileRoles []Role) []Role {
	seen := make(map[string]bool)
	result := make([]Role, 0, len(dbRoles)+len(fileRoles))

	// Add database roles first (they take precedence) - mark as mutable (IsReadOnly=false)
	for i := range dbRoles {
		dbRoles[i].IsReadOnly = false
		result = append(result, dbRoles[i])
		seen[dbRoles[i].ID] = true
	}

	// Add file-based roles only if not already seen - mark as immutable (IsReadOnly=true)
	for i := range fileRoles {
		if !seen[fileRoles[i].ID] {
			fileRoles[i].IsReadOnly = true
			result = append(result, fileRoles[i])
			seen[fileRoles[i].ID] = true
		}
	}

	return result
}

// mergeAssignments deduplicates and merges assignments from database and file stores.
// Database assignments take precedence over file-based assignments with the same ID and type.
func mergeAssignments(dbAssignments, fileAssignments []RoleAssignment) []RoleAssignment {
	seen := make(map[string]bool)
	result := make([]RoleAssignment, 0, len(dbAssignments)+len(fileAssignments))

	// Add database assignments first (they take precedence)
	for _, assignment := range dbAssignments {
		key := string(assignment.Type) + ":" + assignment.ID
		result = append(result, assignment)
		seen[key] = true
	}

	// Add file-based assignments only if not already seen
	for _, assignment := range fileAssignments {
		key := string(assignment.Type) + ":" + assignment.ID
		if !seen[key] {
			result = append(result, assignment)
			seen[key] = true
		}
	}

	return result
}

// mergePermissions deduplicates and merges permissions from database and file stores.
func mergePermissions(dbPerms, filePerms []string) []string {
	permMap := make(map[string]bool)

	for _, perm := range filePerms {
		permMap[perm] = true
	}

	for _, perm := range dbPerms {
		permMap[perm] = true
	}

	result := make([]string, 0, len(permMap))
	for perm := range permMap {
		result = append(result, perm)
	}
	return result
}
