/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const storeLoggerComponentName = "GroupStore"

var buildBulkGroupExistsQueryFunc = buildBulkGroupExistsQuery

// groupStoreInterface defines the interface for group store operations.
type groupStoreInterface interface {
	GetGroupListCount(ctx context.Context) (int, error)
	GetGroupList(ctx context.Context, limit, offset int) ([]GroupBasicDAO, error)
	GetGroupListCountByOUIDs(ctx context.Context, ouIDs []string) (int, error)
	GetGroupListByOUIDs(ctx context.Context, ouIDs []string, limit, offset int) ([]GroupBasicDAO, error)
	CreateGroup(ctx context.Context, group GroupDAO) error
	GetGroup(ctx context.Context, id string) (GroupDAO, error)
	GetGroupMembers(ctx context.Context, groupID string, limit, offset int) ([]Member, error)
	GetGroupMemberCount(ctx context.Context, groupID string) (int, error)
	UpdateGroup(ctx context.Context, group GroupDAO) error
	DeleteGroup(ctx context.Context, id string) error
	ValidateGroupIDs(ctx context.Context, groupIDs []string) ([]string, error)
	CheckGroupNameConflictForCreate(ctx context.Context, name string, oUID string) error
	CheckGroupNameConflictForUpdate(ctx context.Context, name string, oUID string, groupID string) error
	GetGroupsByOrganizationUnitCount(ctx context.Context, oUID string) (int, error)
	GetGroupsByOrganizationUnit(
		ctx context.Context, oUID string, limit, offset int) ([]GroupBasicDAO, error)
	AddGroupMembers(ctx context.Context, groupID string, members []Member) error
	RemoveGroupMembers(ctx context.Context, groupID string, members []Member) error
	GetGroupsByIDs(ctx context.Context, groupIDs []string) ([]GroupBasicDAO, error)
}

// groupStore is the default implementation of groupStoreInterface.
type groupStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newGroupStore creates a new instance of groupStore.
func newGroupStore() groupStoreInterface {
	return &groupStore{
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
		dbProvider:   provider.GetDBProvider(),
	}
}

// GetGroupListCount retrieves the total count of root groups.
func (s *groupStore) GetGroupListCount(ctx context.Context) (int, error) {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	countResults, err := dbClient.QueryContext(ctx, QueryGetGroupListCount, s.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("failed to execute group list count query: %w", err)
	}

	var totalCount int
	if len(countResults) > 0 {
		if total, ok := countResults[0]["total"].(int64); ok {
			totalCount = int(total)
		}
	}

	return totalCount, nil
}

// GetGroupList retrieves root groups.
func (s *groupStore) GetGroupList(ctx context.Context, limit, offset int) ([]GroupBasicDAO, error) {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, QueryGetGroupList, limit, offset, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute group list query: %w", err)
	}

	groups := make([]GroupBasicDAO, 0)
	for _, row := range results {
		group, err := buildGroupFromResultRow(row)
		if err != nil {
			return nil, fmt.Errorf("failed to build group from result row: %w", err)
		}

		groupBasic := GroupBasicDAO{
			ID:          group.ID,
			Name:        group.Name,
			Description: group.Description,
			OUID:        group.OUID,
		}

		groups = append(groups, groupBasic)
	}

	return groups, nil
}

// GetGroupListCountByOUIDs retrieves the total count of groups belonging to a set of OUs.
func (s *groupStore) GetGroupListCountByOUIDs(ctx context.Context, ouIDs []string) (int, error) {
	if len(ouIDs) == 0 {
		return 0, nil
	}

	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client for counter query: %w", err)
	}

	query, args := buildGetGroupsCountByOUIDsQuery(ouIDs, s.deploymentID)

	var count int
	countResults, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	if len(countResults) > 0 {
		if countVal, ok := countResults[0]["total"].(int64); ok {
			count = int(countVal)
		} else if countVal, ok := countResults[0]["total"].(float64); ok { // sqlite count result type
			count = int(countVal)
		}
	}

	return count, nil
}

// GetGroupListByOUIDs retrieves groups belonging to a set of OUs with pagination.
func (s *groupStore) GetGroupListByOUIDs(
	ctx context.Context, ouIDs []string, limit, offset int) ([]GroupBasicDAO, error) {
	if len(ouIDs) == 0 {
		return []GroupBasicDAO{}, nil
	}

	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client for query: %w", err)
	}
	query, args := buildGetGroupsByOUIDsQuery(ouIDs, limit, offset, s.deploymentID)

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	groups := make([]GroupBasicDAO, 0, len(results))
	for _, row := range results {
		group, err := buildGroupFromResultRow(row)
		if err != nil {
			return nil, fmt.Errorf("failed to build group from result row: %w", err)
		}

		groupBasic := GroupBasicDAO{
			ID:          group.ID,
			Name:        group.Name,
			Description: group.Description,
			OUID:        group.OUID,
		}

		groups = append(groups, groupBasic)
	}

	return groups, nil
}

// CreateGroup adds a new group record to the database.
func (s *groupStore) CreateGroup(ctx context.Context, group GroupDAO) error {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	now := time.Now().UTC()
	_, err = dbClient.ExecuteContext(
		ctx,
		QueryCreateGroup,
		group.ID,
		group.OUID,
		group.Name,
		group.Description,
		s.deploymentID,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	err = addMembersToGroup(ctx, dbClient, group.ID, group.Members, s.deploymentID)
	if err != nil {
		return err
	}

	return nil
}

// GetGroup retrieves a group by its id.
func (s *groupStore) GetGroup(ctx context.Context, id string) (GroupDAO, error) {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return GroupDAO{}, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, QueryGetGroupByID, id, s.deploymentID)
	if err != nil {
		return GroupDAO{}, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) == 0 {
		return GroupDAO{}, ErrGroupNotFound
	}

	if len(results) != 1 {
		return GroupDAO{}, fmt.Errorf("unexpected number of results: %d", len(results))
	}

	row := results[0]
	group, err := buildGroupFromResultRow(row)
	if err != nil {
		return GroupDAO{}, err
	}

	return group, nil
}

// GetGroupMembers retrieves members of a group with pagination.
func (s *groupStore) GetGroupMembers(ctx context.Context, groupID string, limit, offset int) ([]Member, error) {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, QueryGetGroupMembers, groupID, limit, offset, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group members: %w", err)
	}

	members := make([]Member, 0)
	for _, row := range results {
		if memberID, ok := row["member_id"].(string); ok {
			if memberType, ok := row["member_type"].(string); ok {
				members = append(members, Member{
					ID:   memberID,
					Type: MemberType(memberType),
				})
			}
		}
	}

	return members, nil
}

// GetGroupMemberCount retrieves the total count of members in a group.
func (s *groupStore) GetGroupMemberCount(ctx context.Context, groupID string) (int, error) {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	countResults, err := dbClient.QueryContext(ctx, QueryGetGroupMemberCount, groupID, s.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get group member count: %w", err)
	}

	if len(countResults) == 0 {
		return 0, nil
	}

	if count, ok := countResults[0]["total"].(int64); ok {
		return int(count), nil
	}

	return 0, nil
}

// UpdateGroup updates an existing group.
func (s *groupStore) UpdateGroup(ctx context.Context, group GroupDAO) error {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	resultRows, err := dbClient.ExecuteContext(
		ctx,
		QueryUpdateGroup,
		group.ID,
		group.OUID,
		group.Name,
		group.Description,
		time.Now().UTC(),
		s.deploymentID,
	)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	if resultRows == 0 {
		return ErrGroupNotFound
	}

	return nil
}

// DeleteGroup deletes a group.
func (s *groupStore) DeleteGroup(ctx context.Context, id string) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, storeLoggerComponentName))

	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, QueryDeleteGroupMembers, id, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to delete group members: %w", err)
	}

	result, err := dbClient.ExecuteContext(ctx, QueryDeleteGroup, id, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	if result == 0 {
		logger.Debug("Group not found with id: " + id)
	}

	return nil
}

// ValidateGroupIDs checks if all provided group IDs exist.
func (s *groupStore) ValidateGroupIDs(ctx context.Context, groupIDs []string) ([]string, error) {
	if len(groupIDs) == 0 {
		return []string{}, nil
	}

	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	query, args, err := buildBulkGroupExistsQueryFunc(groupIDs, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to build bulk group exists query: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	existingGroupIDs := make(map[string]bool)
	for _, row := range results {
		if groupID, ok := row["id"].(string); ok {
			existingGroupIDs[groupID] = true
		}
	}

	var invalidGroupIDs []string
	for _, groupID := range groupIDs {
		if !existingGroupIDs[groupID] {
			invalidGroupIDs = append(invalidGroupIDs, groupID)
		}
	}

	return invalidGroupIDs, nil
}

// CheckGroupNameConflictForCreate checks if the new group name conflicts with existing groups
// in the same organization unit.
func (s *groupStore) CheckGroupNameConflictForCreate(
	ctx context.Context, name string, oUID string) error {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	return checkGroupNameConflictForCreate(ctx, dbClient, name, oUID, s.deploymentID)
}

// CheckGroupNameConflictForUpdate checks if the new group name conflicts with other groups
// in the same organization unit.
func (s *groupStore) CheckGroupNameConflictForUpdate(
	ctx context.Context, name string, oUID string, groupID string) error {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	return checkGroupNameConflictForUpdate(ctx, dbClient, name, oUID, groupID, s.deploymentID)
}

// GetGroupsByOrganizationUnitCount retrieves the total count of groups in a specific organization unit.
func (s *groupStore) GetGroupsByOrganizationUnitCount(ctx context.Context, oUID string) (int, error) {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	countResults, err := dbClient.QueryContext(
		ctx, QueryGetGroupsByOrganizationUnitCount, oUID, s.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get group count by organization unit: %w", err)
	}

	if len(countResults) == 0 {
		return 0, nil
	}

	if count, ok := countResults[0]["total"].(int64); ok {
		return int(count), nil
	}

	return 0, fmt.Errorf("unexpected response format for group count")
}

// GetGroupsByOrganizationUnit retrieves a list of groups in a specific organization unit with pagination.
func (s *groupStore) GetGroupsByOrganizationUnit(
	ctx context.Context, oUID string, limit, offset int,
) ([]GroupBasicDAO, error) {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(
		ctx, QueryGetGroupsByOrganizationUnit, oUID, limit, offset, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get groups by organization unit: %w", err)
	}

	groups := make([]GroupBasicDAO, 0, len(results))
	for _, result := range results {
		group, err := buildGroupFromResultRow(result)
		if err != nil {
			return nil, fmt.Errorf("failed to build group from result row: %w", err)
		}

		groups = append(groups, GroupBasicDAO{
			ID:          group.ID,
			OUID:        group.OUID,
			Name:        group.Name,
			Description: group.Description,
		})
	}

	return groups, nil
}

// AddGroupMembers adds members to a group.
func (s *groupStore) AddGroupMembers(ctx context.Context, groupID string, members []Member) error {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	return addMembersToGroup(ctx, dbClient, groupID, members, s.deploymentID)
}

// RemoveGroupMembers removes members from a group.
func (s *groupStore) RemoveGroupMembers(ctx context.Context, groupID string, members []Member) error {
	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	for _, member := range members {
		_, err := dbClient.ExecuteContext(
			ctx, QueryDeleteGroupMember,
			groupID, member.Type, member.ID, s.deploymentID,
		)
		if err != nil {
			return fmt.Errorf("failed to remove member from group: %w", err)
		}
	}

	return nil
}

// GetGroupsByIDs retrieves groups by a list of IDs.
func (s *groupStore) GetGroupsByIDs(ctx context.Context, groupIDs []string) ([]GroupBasicDAO, error) {
	const batchSize = 100

	if len(groupIDs) == 0 {
		return []GroupBasicDAO{}, nil
	}

	dbClient, err := s.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	groups := make([]GroupBasicDAO, 0, len(groupIDs))

	for start := 0; start < len(groupIDs); start += batchSize {
		end := start + batchSize
		if end > len(groupIDs) {
			end = len(groupIDs)
		}
		chunk := groupIDs[start:end]

		query, args, err := buildGetGroupsByIDsQuery(chunk, s.deploymentID)
		if err != nil {
			return nil, fmt.Errorf("failed to build get groups by IDs query: %w", err)
		}

		results, err := dbClient.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %w", err)
		}

		for _, row := range results {
			group, err := buildGroupFromResultRow(row)
			if err != nil {
				return nil, fmt.Errorf("failed to build group from result row: %w", err)
			}
			groups = append(groups, GroupBasicDAO{
				ID:          group.ID,
				Name:        group.Name,
				Description: group.Description,
				OUID:        group.OUID,
			})
		}
	}

	return groups, nil
}

// buildGroupFromResultRow constructs a GroupDAO from a database result row.
func buildGroupFromResultRow(row map[string]interface{}) (GroupDAO, error) {
	groupID, ok := row["id"].(string)
	if !ok {
		return GroupDAO{}, fmt.Errorf("failed to parse id as string")
	}

	name, ok := row["name"].(string)
	if !ok {
		return GroupDAO{}, fmt.Errorf("failed to parse name as string")
	}

	description, ok := row["description"].(string)
	if !ok {
		return GroupDAO{}, fmt.Errorf("failed to parse description as string")
	}

	ouID, ok := row["ou_id"].(string)
	if !ok {
		return GroupDAO{}, fmt.Errorf("failed to parse ou_id as string")
	}

	group := GroupDAO{
		ID:          groupID,
		Name:        name,
		Description: description,
		OUID:        ouID,
	}

	return group, nil
}

// addMembersToGroup adds a list of members to a group.
func addMembersToGroup(
	ctx context.Context,
	dbClient provider.DBClientInterface,
	groupID string,
	members []Member,
	deploymentID string,
) error {
	now := time.Now().UTC()
	for _, member := range members {
		_, err := dbClient.ExecuteContext(
			ctx, QueryAddMemberToGroup, groupID, member.Type, member.ID, deploymentID, now, now)
		if err != nil {
			return fmt.Errorf("failed to add member to group: %w", err)
		}
	}
	return nil
}

// checkGroupNameConflictForCreate checks if the new group name conflicts with existing groups
// in the same organization unit.
func checkGroupNameConflictForCreate(
	ctx context.Context,
	dbClient provider.DBClientInterface,
	name string,
	oUID string,
	deploymentID string,
) error {
	var results []map[string]interface{}
	var err error

	results, err = dbClient.QueryContext(ctx, QueryCheckGroupNameConflict, name, oUID, deploymentID)

	if err != nil {
		return fmt.Errorf("failed to check group name conflict: %w", err)
	}

	if len(results) > 0 {
		if count, ok := results[0]["count"].(int64); ok && count > 0 {
			return ErrGroupNameConflict
		}
	}

	return nil
}

// checkGroupNameConflictForUpdate checks if the new group name conflicts with other groups
// in the same organization unit.
func checkGroupNameConflictForUpdate(
	ctx context.Context,
	dbClient provider.DBClientInterface,
	name string,
	oUID string,
	groupID string,
	deploymentID string,
) error {
	var results []map[string]interface{}
	var err error

	results, err = dbClient.QueryContext(
		ctx, QueryCheckGroupNameConflictForUpdate, name, oUID, groupID, deploymentID)

	if err != nil {
		return fmt.Errorf("failed to check group name conflict: %w", err)
	}

	if len(results) > 0 {
		if count, ok := results[0]["count"].(int64); ok && count > 0 {
			return ErrGroupNameConflict
		}
	}

	return nil
}
