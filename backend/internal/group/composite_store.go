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

package group

import (
	"context"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// compositeGroupStore implements a composite store that combines file-based (immutable) and
// database (mutable) stores.
// - Read operations query both stores and merge results
// - Write operations (Create/Update/Delete) only affect the database store
// - Declarative groups (from YAML files) cannot be modified or deleted
type compositeGroupStore struct {
	fileStore groupStoreInterface
	dbStore   groupStoreInterface
}

// newCompositeGroupStore creates a new composite store backed by both file and database stores.
func newCompositeGroupStore(fileStore, dbStore groupStoreInterface) groupStoreInterface {
	return &compositeGroupStore{
		fileStore: fileStore,
		dbStore:   dbStore,
	}
}

// GetGroupListCount returns the total count of unique groups across both stores.
func (c *compositeGroupStore) GetGroupListCount(ctx context.Context) (int, error) {
	capCount := func(fn func(context.Context) (int, error)) func() (int, error) {
		return func() (int, error) {
			count, err := fn(ctx)
			if err != nil {
				return 0, err
			}
			return min(count, serverconst.MaxCompositeStoreRecords), nil
		}
	}
	groups, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		capCount(c.dbStore.GetGroupListCount),
		capCount(c.fileStore.GetGroupListCount),
		func(count int) ([]GroupBasicDAO, error) { return c.dbStore.GetGroupList(ctx, count, 0) },
		func(count int) ([]GroupBasicDAO, error) { return c.fileStore.GetGroupList(ctx, count, 0) },
		mergeGroupBasicDAOs,
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

	return len(groups), nil
}

// GetGroupList returns a paginated merged list of groups from both stores.
func (c *compositeGroupStore) GetGroupList(ctx context.Context, limit, offset int) ([]GroupBasicDAO, error) {
	capCount := func(fn func(context.Context) (int, error)) func() (int, error) {
		return func() (int, error) {
			count, err := fn(ctx)
			if err != nil {
				return 0, err
			}
			return min(count, serverconst.MaxCompositeStoreRecords), nil
		}
	}
	groups, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		capCount(c.dbStore.GetGroupListCount),
		capCount(c.fileStore.GetGroupListCount),
		func(count int) ([]GroupBasicDAO, error) { return c.dbStore.GetGroupList(ctx, count, 0) },
		func(count int) ([]GroupBasicDAO, error) { return c.fileStore.GetGroupList(ctx, count, 0) },
		mergeGroupBasicDAOs,
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

	return groups, nil
}

// GetGroupListCountByOUIDs returns the count of unique groups belonging to any of the given OUs.
func (c *compositeGroupStore) GetGroupListCountByOUIDs(ctx context.Context, ouIDs []string) (int, error) {
	capCount := func(fn func(context.Context, []string) (int, error)) func() (int, error) {
		return func() (int, error) {
			count, err := fn(ctx, ouIDs)
			if err != nil {
				return 0, err
			}
			return min(count, serverconst.MaxCompositeStoreRecords), nil
		}
	}
	groups, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		capCount(c.dbStore.GetGroupListCountByOUIDs),
		capCount(c.fileStore.GetGroupListCountByOUIDs),
		func(count int) ([]GroupBasicDAO, error) {
			return c.dbStore.GetGroupListByOUIDs(ctx, ouIDs, count, 0)
		},
		func(count int) ([]GroupBasicDAO, error) {
			return c.fileStore.GetGroupListByOUIDs(ctx, ouIDs, count, 0)
		},
		mergeGroupBasicDAOs,
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

	return len(groups), nil
}

// GetGroupListByOUIDs returns a paginated merged list of groups belonging to any of the given OUs.
func (c *compositeGroupStore) GetGroupListByOUIDs(
	ctx context.Context, ouIDs []string, limit, offset int,
) ([]GroupBasicDAO, error) {
	capCount := func(fn func(context.Context, []string) (int, error)) func() (int, error) {
		return func() (int, error) {
			count, err := fn(ctx, ouIDs)
			if err != nil {
				return 0, err
			}
			return min(count, serverconst.MaxCompositeStoreRecords), nil
		}
	}
	groups, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		capCount(c.dbStore.GetGroupListCountByOUIDs),
		capCount(c.fileStore.GetGroupListCountByOUIDs),
		func(count int) ([]GroupBasicDAO, error) {
			return c.dbStore.GetGroupListByOUIDs(ctx, ouIDs, count, 0)
		},
		func(count int) ([]GroupBasicDAO, error) {
			return c.fileStore.GetGroupListByOUIDs(ctx, ouIDs, count, 0)
		},
		mergeGroupBasicDAOs,
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

	return groups, nil
}

// CreateGroup creates a group in the database store only.
func (c *compositeGroupStore) CreateGroup(ctx context.Context, group GroupDAO) error {
	return c.dbStore.CreateGroup(ctx, group)
}

// GetGroup retrieves a group from either store, checking the database store first.
func (c *compositeGroupStore) GetGroup(ctx context.Context, id string) (GroupDAO, error) {
	grp, err := declarativeresource.CompositeGetHelper(
		func() (*GroupDAO, error) {
			g, err := c.dbStore.GetGroup(ctx, id)
			return &g, err
		},
		func() (*GroupDAO, error) {
			g, err := c.fileStore.GetGroup(ctx, id)
			return &g, err
		},
		ErrGroupNotFound,
	)
	if grp != nil {
		return *grp, err
	}
	return GroupDAO{}, err
}

// GetGroupMembers returns a paginated merged list of members from both stores.
// DB members take precedence over file-based members with the same ID+Type.
func (c *compositeGroupStore) GetGroupMembers(
	ctx context.Context, groupID string, limit, offset int,
) ([]Member, error) {
	return compositeMergeByID(
		ctx, groupID,
		c.dbStore.GetGroupMemberCount, c.fileStore.GetGroupMemberCount,
		c.dbStore.GetGroupMembers, c.fileStore.GetGroupMembers,
		mergeMembers,
		limit, offset,
	)
}

// GetGroupMemberCount returns the total count of unique members across both stores.
func (c *compositeGroupStore) GetGroupMemberCount(ctx context.Context, groupID string) (int, error) {
	members, err := c.GetGroupMembers(ctx, groupID, serverconst.MaxCompositeStoreRecords+1, 0)
	if err != nil {
		return 0, err
	}
	return len(members), nil
}

// UpdateGroup updates a group in the database store only.
// Immutability checks are handled at the service layer.
func (c *compositeGroupStore) UpdateGroup(ctx context.Context, group GroupDAO) error {
	return c.dbStore.UpdateGroup(ctx, group)
}

// DeleteGroup deletes a group from the database store only.
// Immutability checks are handled at the service layer.
func (c *compositeGroupStore) DeleteGroup(ctx context.Context, id string) error {
	return c.dbStore.DeleteGroup(ctx, id)
}

// ValidateGroupIDs checks if all provided group IDs exist in either store.
func (c *compositeGroupStore) ValidateGroupIDs(ctx context.Context, groupIDs []string) ([]string, error) {
	dbInvalid, err := c.dbStore.ValidateGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	if len(dbInvalid) == 0 {
		return []string{}, nil
	}

	// Re-check IDs that the DB store did not find against the file store.
	fileInvalid, err := c.fileStore.ValidateGroupIDs(ctx, dbInvalid)
	if err != nil {
		return nil, err
	}

	return fileInvalid, nil
}

// CheckGroupNameConflictForCreate checks for name conflicts in either store.
func (c *compositeGroupStore) CheckGroupNameConflictForCreate(
	ctx context.Context, name string, oUID string,
) error {
	if err := c.dbStore.CheckGroupNameConflictForCreate(ctx, name, oUID); err != nil {
		return err
	}
	return c.fileStore.CheckGroupNameConflictForCreate(ctx, name, oUID)
}

// CheckGroupNameConflictForUpdate checks for name conflicts excluding a specific ID in either store.
func (c *compositeGroupStore) CheckGroupNameConflictForUpdate(
	ctx context.Context, name string, oUID string, groupID string,
) error {
	if err := c.dbStore.CheckGroupNameConflictForUpdate(ctx, name, oUID, groupID); err != nil {
		return err
	}
	return c.fileStore.CheckGroupNameConflictForUpdate(ctx, name, oUID, groupID)
}

// GetGroupsByOrganizationUnitCount returns the count of groups in the given OU across both stores.
func (c *compositeGroupStore) GetGroupsByOrganizationUnitCount(ctx context.Context, oUID string) (int, error) {
	groups, err := c.GetGroupsByOrganizationUnit(ctx, oUID, serverconst.MaxCompositeStoreRecords+1, 0)
	if err != nil {
		return 0, err
	}
	return len(groups), nil
}

// GetGroupsByOrganizationUnit returns a paginated merged list of groups in the given OU.
func (c *compositeGroupStore) GetGroupsByOrganizationUnit(
	ctx context.Context, oUID string, limit, offset int,
) ([]GroupBasicDAO, error) {
	return compositeMergeByID(
		ctx, oUID,
		c.dbStore.GetGroupsByOrganizationUnitCount, c.fileStore.GetGroupsByOrganizationUnitCount,
		c.dbStore.GetGroupsByOrganizationUnit, c.fileStore.GetGroupsByOrganizationUnit,
		mergeGroupBasicDAOs,
		limit, offset,
	)
}

// compositeMergeByID runs a paginated merge across the db and file stores keyed by a single ID.
func compositeMergeByID[T any](
	ctx context.Context, id string,
	dbCount, fileCount func(context.Context, string) (int, error),
	dbList, fileList func(context.Context, string, int, int) ([]T, error),
	merge func([]T, []T) []T,
	limit, offset int,
) ([]T, error) {
	capCount := func(fn func(context.Context, string) (int, error)) func() (int, error) {
		return func() (int, error) {
			count, err := fn(ctx, id)
			if err != nil {
				return 0, err
			}
			return min(count, serverconst.MaxCompositeStoreRecords), nil
		}
	}
	result, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		capCount(dbCount),
		capCount(fileCount),
		func(count int) ([]T, error) { return dbList(ctx, id, count, 0) },
		func(count int) ([]T, error) { return fileList(ctx, id, count, 0) },
		merge,
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
	return result, nil
}

// AddGroupMembers adds members to a group in the database store only.
func (c *compositeGroupStore) AddGroupMembers(ctx context.Context, groupID string, members []Member) error {
	return c.dbStore.AddGroupMembers(ctx, groupID, members)
}

// RemoveGroupMembers removes members from a group in the database store only.
func (c *compositeGroupStore) RemoveGroupMembers(ctx context.Context, groupID string, members []Member) error {
	return c.dbStore.RemoveGroupMembers(ctx, groupID, members)
}

// GetGroupsByIDs returns groups matching the given IDs, merged from both stores.
func (c *compositeGroupStore) GetGroupsByIDs(ctx context.Context, groupIDs []string) ([]GroupBasicDAO, error) {
	dbGroups, err := c.dbStore.GetGroupsByIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(dbGroups))
	for _, g := range dbGroups {
		seen[g.ID] = true
	}

	var missing []string
	for _, id := range groupIDs {
		if !seen[id] {
			missing = append(missing, id)
		}
	}

	if len(missing) == 0 {
		return dbGroups, nil
	}

	fileGroups, err := c.fileStore.GetGroupsByIDs(ctx, missing)
	if err != nil {
		return nil, err
	}

	return append(dbGroups, fileGroups...), nil
}

// IsGroupDeclarative checks if the group exists in the file-based store.
func (c *compositeGroupStore) IsGroupDeclarative(ctx context.Context, id string) (bool, error) {
	return c.fileStore.IsGroupDeclarative(ctx, id)
}

// mergeMembers deduplicates and merges members from database and file stores.
// Database members take precedence over file-based members with the same ID and type.
func mergeMembers(dbMembers, fileMembers []Member) []Member {
	seen := make(map[string]bool, len(dbMembers))
	result := make([]Member, 0, len(dbMembers)+len(fileMembers))

	for _, m := range dbMembers {
		key := string(m.Type) + ":" + m.ID
		result = append(result, m)
		seen[key] = true
	}

	for _, m := range fileMembers {
		key := string(m.Type) + ":" + m.ID
		if !seen[key] {
			result = append(result, m)
			seen[key] = true
		}
	}

	return result
}

// mergeGroupBasicDAOs deduplicates and merges groups from database and file stores.
// Database groups take precedence over file-based groups with the same ID.
func mergeGroupBasicDAOs(dbGroups, fileGroups []GroupBasicDAO) []GroupBasicDAO {
	seen := make(map[string]bool, len(dbGroups))
	result := make([]GroupBasicDAO, 0, len(dbGroups)+len(fileGroups))

	for i := range dbGroups {
		dbGroups[i].IsReadOnly = false
		result = append(result, dbGroups[i])
		seen[dbGroups[i].ID] = true
	}

	for i := range fileGroups {
		if !seen[fileGroups[i].ID] {
			fileGroups[i].IsReadOnly = true
			result = append(result, fileGroups[i])
			seen[fileGroups[i].ID] = true
		}
	}

	return result
}
