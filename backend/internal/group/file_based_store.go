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
	"errors"
	"strings"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

type fileBasedGroupStore struct {
	*declarativeresource.GenericFileBasedStore
}

// newFileBasedGroupStore creates a new file-based store for groups.
func newFileBasedGroupStore() (groupStoreInterface, transaction.Transactioner) {
	return &fileBasedGroupStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStore(entity.KeyTypeGroup),
	}, transaction.NewNoOpTransactioner()
}

// Create implements declarativeresource.Storer interface for resource loader.
func (f *fileBasedGroupStore) Create(id string, data interface{}) error {
	grp, ok := data.(*groupDeclarativeResource)
	if !ok {
		return ErrGroupDataCorrupted
	}
	if grp.ID == "" {
		grp.ID = id
	}
	return f.GenericFileBasedStore.Create(id, grp)
}

// GetGroupListCount returns the total count of groups in the file-based store.
func (f *fileBasedGroupStore) GetGroupListCount(ctx context.Context) (int, error) {
	return f.GenericFileBasedStore.Count()
}

// GetGroupList returns a paginated list of root groups from the file-based store.
func (f *fileBasedGroupStore) GetGroupList(ctx context.Context, limit, offset int) ([]GroupBasicDAO, error) {
	if limit <= 0 {
		return []GroupBasicDAO{}, nil
	}
	if offset < 0 {
		offset = 0
	}

	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	groups := make([]GroupBasicDAO, 0, len(list))
	for _, item := range list {
		grpData, err := groupFromDeclarativeData(item.ID.ID, item.Data)
		if err != nil {
			log.GetLogger().Warn(ctx, "Skipping malformed group in GetGroupList",
				log.String("groupID", item.ID.ID),
				log.Error(err))
			continue
		}
		groups = append(groups, GroupBasicDAO{
			ID:          grpData.ID,
			Name:        grpData.Name,
			Description: grpData.Description,
			OUID:        grpData.OUID,
			IsReadOnly:  true,
		})
	}

	start := offset
	if start >= len(groups) {
		return []GroupBasicDAO{}, nil
	}
	end := start + limit
	if end > len(groups) {
		end = len(groups)
	}

	return groups[start:end], nil
}

// GetGroupListCountByOUIDs returns the count of groups belonging to any of the given OUs.
func (f *fileBasedGroupStore) GetGroupListCountByOUIDs(ctx context.Context, ouIDs []string) (int, error) {
	if len(ouIDs) == 0 {
		return 0, nil
	}

	ouSet := make(map[string]bool, len(ouIDs))
	for _, id := range ouIDs {
		ouSet[id] = true
	}

	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, item := range list {
		grpData, err := groupFromDeclarativeData(item.ID.ID, item.Data)
		if err != nil {
			continue
		}
		if ouSet[grpData.OUID] {
			count++
		}
	}

	return count, nil
}

// GetGroupListByOUIDs returns a paginated list of groups belonging to any of the given OUs.
func (f *fileBasedGroupStore) GetGroupListByOUIDs(
	ctx context.Context, ouIDs []string, limit, offset int,
) ([]GroupBasicDAO, error) {
	if len(ouIDs) == 0 {
		return []GroupBasicDAO{}, nil
	}

	ouSet := make(map[string]bool, len(ouIDs))
	for _, id := range ouIDs {
		ouSet[id] = true
	}

	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	groups := make([]GroupBasicDAO, 0)
	for _, item := range list {
		grpData, err := groupFromDeclarativeData(item.ID.ID, item.Data)
		if err != nil {
			continue
		}
		if ouSet[grpData.OUID] {
			groups = append(groups, GroupBasicDAO{
				ID:          grpData.ID,
				Name:        grpData.Name,
				Description: grpData.Description,
				OUID:        grpData.OUID,
				IsReadOnly:  true,
			})
		}
	}

	if limit <= 0 {
		return []GroupBasicDAO{}, nil
	}
	start := offset
	if start < 0 {
		start = 0
	}
	if start >= len(groups) {
		return []GroupBasicDAO{}, nil
	}
	end := start + limit
	if end > len(groups) {
		end = len(groups)
	}

	return groups[start:end], nil
}

// CreateGroup is not supported in file-based store.
func (f *fileBasedGroupStore) CreateGroup(ctx context.Context, group GroupDAO) error {
	return errors.New("CreateGroup is not supported in file-based store")
}

// GetGroup returns a group from the file-based store.
func (f *fileBasedGroupStore) GetGroup(ctx context.Context, id string) (GroupDAO, error) {
	data, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		if isGroupNotFoundError(err) {
			return GroupDAO{}, ErrGroupNotFound
		}
		return GroupDAO{}, err
	}

	grpData, err := groupFromDeclarativeData(id, data)
	if err != nil {
		return GroupDAO{}, err
	}

	members := append([]Member{}, grpData.Members...)

	return GroupDAO{
		ID:          grpData.ID,
		Name:        grpData.Name,
		Description: grpData.Description,
		OUID:        grpData.OUID,
		Members:     members,
		IsReadOnly:  true,
	}, nil
}

// GetGroupMembers returns members of a group with pagination.
func (f *fileBasedGroupStore) GetGroupMembers(
	ctx context.Context, groupID string, limit, offset int,
) ([]Member, error) {
	if limit <= 0 {
		return []Member{}, nil
	}
	if offset < 0 {
		offset = 0
	}

	data, err := f.GenericFileBasedStore.Get(groupID)
	if err != nil {
		if isGroupNotFoundError(err) {
			return []Member{}, nil
		}
		return nil, err
	}

	grpData, err := groupFromDeclarativeData(groupID, data)
	if err != nil {
		return nil, err
	}

	members := grpData.Members
	start := offset
	if start >= len(members) {
		return []Member{}, nil
	}
	end := start + limit
	if end > len(members) {
		end = len(members)
	}

	return members[start:end], nil
}

// GetGroupMemberCount returns the total count of members in a group.
func (f *fileBasedGroupStore) GetGroupMemberCount(ctx context.Context, groupID string) (int, error) {
	data, err := f.GenericFileBasedStore.Get(groupID)
	if err != nil {
		if isGroupNotFoundError(err) {
			return 0, nil
		}
		return 0, err
	}

	grpData, err := groupFromDeclarativeData(groupID, data)
	if err != nil {
		return 0, err
	}

	return len(grpData.Members), nil
}

// UpdateGroup is not supported in file-based store.
func (f *fileBasedGroupStore) UpdateGroup(ctx context.Context, group GroupDAO) error {
	return errors.New("UpdateGroup is not supported in file-based store")
}

// DeleteGroup is not supported in file-based store.
func (f *fileBasedGroupStore) DeleteGroup(ctx context.Context, id string) error {
	return errors.New("DeleteGroup is not supported in file-based store")
}

// ValidateGroupIDs checks which of the given group IDs do not exist in the file-based store.
func (f *fileBasedGroupStore) ValidateGroupIDs(ctx context.Context, groupIDs []string) ([]string, error) {
	if len(groupIDs) == 0 {
		return []string{}, nil
	}

	var invalid []string
	for _, id := range groupIDs {
		_, err := f.GenericFileBasedStore.Get(id)
		if err != nil {
			if isGroupNotFoundError(err) {
				invalid = append(invalid, id)
				continue
			}
			return nil, err
		}
	}

	return invalid, nil
}

// CheckGroupNameConflictForCreate checks if a group with the given name already exists in the OU.
func (f *fileBasedGroupStore) CheckGroupNameConflictForCreate(
	ctx context.Context, name string, oUID string,
) error {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return err
	}

	for _, item := range list {
		grpData, err := groupFromDeclarativeData(item.ID.ID, item.Data)
		if err != nil {
			continue
		}
		if grpData.OUID == oUID && grpData.Name == name {
			return ErrGroupNameConflict
		}
	}

	return nil
}

// CheckGroupNameConflictForUpdate checks for a name conflict excluding the group being updated.
func (f *fileBasedGroupStore) CheckGroupNameConflictForUpdate(
	ctx context.Context, name string, oUID string, groupID string,
) error {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return err
	}

	for _, item := range list {
		grpData, err := groupFromDeclarativeData(item.ID.ID, item.Data)
		if err != nil {
			continue
		}
		if grpData.ID == groupID {
			continue
		}
		if grpData.OUID == oUID && grpData.Name == name {
			return ErrGroupNameConflict
		}
	}

	return nil
}

// GetGroupsByOrganizationUnitCount returns the count of groups in the given OU.
func (f *fileBasedGroupStore) GetGroupsByOrganizationUnitCount(ctx context.Context, oUID string) (int, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, item := range list {
		grpData, err := groupFromDeclarativeData(item.ID.ID, item.Data)
		if err != nil {
			continue
		}
		if grpData.OUID == oUID {
			count++
		}
	}

	return count, nil
}

// GetGroupsByOrganizationUnit returns a paginated list of groups in the given OU.
func (f *fileBasedGroupStore) GetGroupsByOrganizationUnit(
	ctx context.Context, oUID string, limit, offset int,
) ([]GroupBasicDAO, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	groups := make([]GroupBasicDAO, 0)
	for _, item := range list {
		grpData, err := groupFromDeclarativeData(item.ID.ID, item.Data)
		if err != nil {
			continue
		}
		if grpData.OUID == oUID {
			groups = append(groups, GroupBasicDAO{
				ID:          grpData.ID,
				Name:        grpData.Name,
				Description: grpData.Description,
				OUID:        grpData.OUID,
				IsReadOnly:  true,
			})
		}
	}

	if limit <= 0 {
		return []GroupBasicDAO{}, nil
	}
	start := offset
	if start < 0 {
		start = 0
	}
	if start >= len(groups) {
		return []GroupBasicDAO{}, nil
	}
	end := start + limit
	if end > len(groups) {
		end = len(groups)
	}

	return groups[start:end], nil
}

// AddGroupMembers is not supported in file-based store.
func (f *fileBasedGroupStore) AddGroupMembers(ctx context.Context, groupID string, members []Member) error {
	return errors.New("AddGroupMembers is not supported in file-based store")
}

// RemoveGroupMembers is not supported in file-based store.
func (f *fileBasedGroupStore) RemoveGroupMembers(ctx context.Context, groupID string, members []Member) error {
	return errors.New("RemoveGroupMembers is not supported in file-based store")
}

// GetGroupsByIDs returns groups matching any of the given IDs.
func (f *fileBasedGroupStore) GetGroupsByIDs(ctx context.Context, groupIDs []string) ([]GroupBasicDAO, error) {
	if len(groupIDs) == 0 {
		return []GroupBasicDAO{}, nil
	}

	wanted := make(map[string]bool, len(groupIDs))
	for _, id := range groupIDs {
		wanted[id] = true
	}

	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	groups := make([]GroupBasicDAO, 0, len(groupIDs))
	for _, item := range list {
		if !wanted[item.ID.ID] {
			continue
		}
		grpData, err := groupFromDeclarativeData(item.ID.ID, item.Data)
		if err != nil {
			log.GetLogger().Warn(ctx, "Skipping malformed group in GetGroupsByIDs",
				log.String("groupID", item.ID.ID),
				log.Error(err))
			continue
		}
		groups = append(groups, GroupBasicDAO{
			ID:          grpData.ID,
			Name:        grpData.Name,
			Description: grpData.Description,
			OUID:        grpData.OUID,
			IsReadOnly:  true,
		})
	}

	return groups, nil
}

// IsGroupDeclarative returns true for all groups in the file-based store.
func (f *fileBasedGroupStore) IsGroupDeclarative(ctx context.Context, id string) (bool, error) {
	_, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		if isGroupNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// groupFromDeclarativeData converts raw data from the declarative store to a groupDeclarativeResource.
func groupFromDeclarativeData(id string, data interface{}) (groupDeclarativeResource, error) {
	grp, ok := data.(*groupDeclarativeResource)
	if !ok || grp == nil {
		declarativeresource.LogTypeAssertionError("group", id)
		return groupDeclarativeResource{}, ErrGroupDataCorrupted
	}
	return *grp, nil
}

// isGroupNotFoundError checks whether the error signals a missing entity.
func isGroupNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return msg == "entity not found" || strings.Contains(msg, "not found")
}
