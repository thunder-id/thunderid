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

package role

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

const storeLoggerComponentName = "RoleStore"

var getDBProvider = provider.GetDBProvider

// roleStoreInterface defines the interface for role store operations.
type roleStoreInterface interface {
	GetRoleListCount(ctx context.Context) (int, error)
	GetRoleList(ctx context.Context, limit, offset int) ([]Role, error)
	CreateRole(ctx context.Context, id string, role RoleCreationDetail) error
	GetRole(ctx context.Context, id string) (RoleWithPermissions, error)
	IsRoleExist(ctx context.Context, id string) (bool, error)
	GetRoleAssignments(ctx context.Context, id string, limit, offset int) ([]RoleAssignment, error)
	GetRoleAssignmentsByType(ctx context.Context, id string,
		limit, offset int, assigneeType string) ([]RoleAssignment, error)
	GetRoleAssignmentsCount(ctx context.Context, id string) (int, error)
	GetRoleAssignmentsCountByType(ctx context.Context, id string, assigneeType string) (int, error)
	UpdateRole(ctx context.Context, id string, role RoleUpdateDetail) error
	DeleteRole(ctx context.Context, id string) error
	DeleteAssignmentsByRoleID(ctx context.Context, id string) error
	AddAssignments(ctx context.Context, id string, assignments []RoleAssignment) error
	RemoveAssignments(ctx context.Context, id string, assignments []RoleAssignment) error
	CheckRoleNameExists(ctx context.Context, ouID, name string) (bool, error)
	CheckRoleNameExistsExcludingID(ctx context.Context, ouID, name, excludeRoleID string) (bool, error)
	GetAuthorizedPermissions(
		ctx context.Context, entityID string, groupIDs []string, requestedPermissions []string) ([]string, error)
	GetUserRoles(ctx context.Context, entityID string, groupIDs []string) ([]string, error)
	// GetEntityRoleIDs returns the set of role IDs assigned to the entity directly or via
	// group membership. Unlike GetUserRoles this does not require the role to exist in the
	// underlying store; it returns raw assignee->role bindings. Used by the composite store
	// to resolve permissions for declarative roles whose definitions live in the file store
	// while their assignments live in the DB.
	GetEntityRoleIDs(ctx context.Context, entityID string, groupIDs []string) ([]string, error)
	IsRoleDeclarative(ctx context.Context, roleID string) (bool, error)
}

// roleStore is the default implementation of roleStoreInterface.
type roleStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newRoleStore creates a new instance of roleStore.
func newRoleStore() (roleStoreInterface, transaction.Transactioner, error) {
	dbProvider := getDBProvider()
	client, err := dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, nil, err
	}
	transactioner, err := client.GetTransactioner()
	if err != nil {
		return nil, nil, err
	}
	return &roleStore{
		dbProvider:   dbProvider,
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}, transactioner, nil
}

// GetRoleListCount retrieves the total count of roles.
func (s *roleStore) GetRoleListCount(ctx context.Context) (int, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return 0, err
	}

	countResults, err := dbClient.QueryContext(ctx, queryGetRoleListCount, s.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("failed to execute count query: %w", err)
	}

	return parseCountResult(countResults)
}

// GetRoleList retrieves roles with pagination.
func (s *roleStore) GetRoleList(ctx context.Context, limit, offset int) ([]Role, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.QueryContext(ctx, queryGetRoleList, limit, offset, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute role list query: %w", err)
	}

	roles := make([]Role, 0)
	for _, row := range results {
		role, err := buildRoleBasicInfoFromResultRow(row)
		if err != nil {
			return nil, fmt.Errorf("failed to build role from result row: %w", err)
		}
		roles = append(roles, role)
	}

	return roles, nil
}

// CreateRole creates a new role in the database.
func (s *roleStore) CreateRole(ctx context.Context, id string, role RoleCreationDetail) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	_, err = dbClient.ExecuteContext(ctx,
		queryCreateRole,
		id,
		role.OUID,
		role.Name,
		role.Description,
		s.deploymentID,
	)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	if err := addPermissionsToRole(ctx, dbClient, id, role.Permissions, s.deploymentID); err != nil {
		return err
	}

	if err := addAssignmentsToRole(ctx, dbClient, id, role.Assignments, s.deploymentID); err != nil {
		return err
	}

	return nil
}

// GetRole retrieves a role by its id.
func (s *roleStore) GetRole(ctx context.Context, id string) (RoleWithPermissions, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return RoleWithPermissions{}, err
	}

	results, err := dbClient.QueryContext(ctx, queryGetRoleByID, id, s.deploymentID)
	if err != nil {
		return RoleWithPermissions{}, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) == 0 {
		return RoleWithPermissions{}, ErrRoleNotFound
	}

	if len(results) != 1 {
		return RoleWithPermissions{}, fmt.Errorf("unexpected number of results: %d", len(results))
	}

	row := results[0]
	roleBasicInfo, err := buildRoleBasicInfoFromResultRow(row)
	if err != nil {
		return RoleWithPermissions{}, err
	}

	permissions, err := s.getRolePermissions(ctx, dbClient, id)
	if err != nil {
		return RoleWithPermissions{}, fmt.Errorf("failed to get role permissions: %w", err)
	}

	return RoleWithPermissions{
		ID:          roleBasicInfo.ID,
		Name:        roleBasicInfo.Name,
		Description: roleBasicInfo.Description,
		OUID:        roleBasicInfo.OUID,
		Permissions: permissions,
	}, nil
}

// IsRoleExist checks if a role exists by its ID without fetching its details.
func (s *roleStore) IsRoleExist(ctx context.Context, id string) (bool, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return false, err
	}

	results, err := dbClient.QueryContext(ctx, queryCheckRoleExists, id, s.deploymentID)
	if err != nil {
		return false, fmt.Errorf("failed to check role existence: %w", err)
	}

	return parseBoolFromCount(results)
}

// GetRoleAssignments retrieves assignments for a role with pagination.
func (s *roleStore) GetRoleAssignments(ctx context.Context, id string, limit, offset int) ([]RoleAssignment, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.QueryContext(ctx, queryGetRoleAssignments, id, limit, offset, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role assignments: %w", err)
	}

	return parseAssignmentResults(results)
}

// GetRoleAssignmentsByType retrieves assignments for a role filtered by assignee type with pagination.
func (s *roleStore) GetRoleAssignmentsByType(
	ctx context.Context, id string, limit, offset int, assigneeType string,
) ([]RoleAssignment, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.QueryContext(
		ctx, queryGetRoleAssignmentsByType, id, limit, offset, s.deploymentID, assigneeType)
	if err != nil {
		return nil, fmt.Errorf("failed to get role assignments: %w", err)
	}

	return parseAssignmentResults(results)
}

// parseAssignmentResults parses database query results into role assignments.
func parseAssignmentResults(results []map[string]interface{}) ([]RoleAssignment, error) {
	assignments := make([]RoleAssignment, 0)
	for _, row := range results {
		assigneeID, err := parseStringField(row, "assignee_id")
		if err != nil {
			return nil, err
		}
		assigneeType, err := parseStringField(row, "assignee_type")
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, RoleAssignment{
			ID:   assigneeID,
			Type: AssigneeType(assigneeType),
		})
	}

	return assignments, nil
}

// GetRoleAssignmentsCount retrieves the total count of assignments for a role.
func (s *roleStore) GetRoleAssignmentsCount(ctx context.Context, id string) (int, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return 0, err
	}

	countResults, err := dbClient.QueryContext(ctx, queryGetRoleAssignmentsCount, id, s.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get role assignments count: %w", err)
	}

	return parseCountResult(countResults)
}

// GetRoleAssignmentsCountByType retrieves the total count of assignments for a role filtered by type.
func (s *roleStore) GetRoleAssignmentsCountByType(ctx context.Context, id string, assigneeType string) (int, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return 0, err
	}

	countResults, err := dbClient.QueryContext(
		ctx, queryGetRoleAssignmentsCountByType, id, s.deploymentID, assigneeType)
	if err != nil {
		return 0, fmt.Errorf("failed to get role assignments count: %w", err)
	}

	return parseCountResult(countResults)
}

// UpdateRole updates an existing role.
func (s *roleStore) UpdateRole(ctx context.Context, id string, role RoleUpdateDetail) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx,
		queryUpdateRole,
		role.OUID,
		role.Name,
		role.Description,
		id,
		s.deploymentID,
	)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	if rowsAffected == 0 {
		return ErrRoleNotFound
	}

	if err := updateRolePermissions(ctx, dbClient, id, role.Permissions, s.deploymentID); err != nil {
		return err
	}

	return nil
}

// DeleteRole deletes a role.
func (s *roleStore) DeleteRole(ctx context.Context, id string) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, storeLoggerComponentName))

	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, queryDeleteRole, id, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	if rowsAffected == 0 {
		logger.Debug(ctx, "Role not found with id: "+id)
	}

	return nil
}

// DeleteAssignmentsByRoleID deletes all assignments for a role. Used for cascade delete.
func (s *roleStore) DeleteAssignmentsByRoleID(ctx context.Context, id string) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	_, err = dbClient.ExecuteContext(ctx, queryDeleteAllRoleAssignments, id, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to delete assignments for role: %w", err)
	}
	return nil
}

// AddAssignments adds assignments to a role.
func (s *roleStore) AddAssignments(ctx context.Context, id string, assignments []RoleAssignment) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	return addAssignmentsToRole(ctx, dbClient, id, assignments, s.deploymentID)
}

// RemoveAssignments removes assignments from a role.
func (s *roleStore) RemoveAssignments(ctx context.Context, id string, assignments []RoleAssignment) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	for _, assignment := range assignments {
		_, err := dbClient.ExecuteContext(
			ctx, queryDeleteRoleAssignmentsByIDs, id, assignment.Type, assignment.ID, s.deploymentID)
		if err != nil {
			return fmt.Errorf("failed to remove assignment from role: %w", err)
		}
	}
	return nil
}

// getRolePermissions retrieves all permissions for a role.
func (s *roleStore) getRolePermissions(
	ctx context.Context, dbClient provider.DBClientInterface, id string) ([]ResourcePermissions, error) {
	results, err := dbClient.QueryContext(ctx, queryGetRolePermissions, id, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role permissions: %w", err)
	}

	// Group permissions by resource server
	permMap := make(map[string][]string)
	var resourceServerOrder []string

	for _, row := range results {
		permission, ok := row["permission"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse permission as string")
		}

		resourceServerID, ok := row["resource_server_id"].(string)
		if !ok {
			return nil, fmt.Errorf("failed to parse resource_server_id as string")
		}

		// Track order of resource servers as they appear
		if _, exists := permMap[resourceServerID]; !exists {
			resourceServerOrder = append(resourceServerOrder, resourceServerID)
		}

		permMap[resourceServerID] = append(permMap[resourceServerID], permission)
	}

	// Convert map to array of ResourcePermissions
	permissions := make([]ResourcePermissions, 0, len(permMap))
	for _, rsID := range resourceServerOrder {
		permissions = append(permissions, ResourcePermissions{
			ResourceServerID: rsID,
			Permissions:      permMap[rsID],
		})
	}

	return permissions, nil
}

// buildRoleSummaryFromResultRow constructs a Role from a database result row.
func buildRoleBasicInfoFromResultRow(row map[string]interface{}) (Role, error) {
	fields, err := parseStringFields(row, "id", "name", "description", "ou_id")
	if err != nil {
		return Role{}, err
	}

	return Role{
		ID:          fields[0],
		Name:        fields[1],
		Description: fields[2],
		OUID:        fields[3],
	}, nil
}

// addPermissionsToRole adds a list of permissions to a role.
func addPermissionsToRole(
	ctx context.Context,
	dbClient provider.DBClientInterface,
	id string,
	permissions []ResourcePermissions,
	deploymentID string,
) error {
	for _, resPerm := range permissions {
		for _, permission := range resPerm.Permissions {
			_, err := dbClient.ExecuteContext(
				ctx, queryCreateRolePermission, id, resPerm.ResourceServerID, permission, deploymentID)
			if err != nil {
				return fmt.Errorf("failed to add permission to role: %w", err)
			}
		}
	}
	return nil
}

// addAssignmentsToRole adds a list of assignments to a role.
func addAssignmentsToRole(
	ctx context.Context,
	dbClient provider.DBClientInterface,
	id string,
	assignments []RoleAssignment,
	deploymentID string,
) error {
	for _, assignment := range assignments {
		_, err := dbClient.ExecuteContext(
			ctx, queryCreateRoleAssignment, id, assignment.Type, assignment.ID, deploymentID)
		if err != nil {
			return fmt.Errorf("failed to add assignment to role: %w", err)
		}
	}
	return nil
}

// updateRolePermissions updates the permissions assigned to the role by first deleting existing permissions and
// then adding new ones.
func updateRolePermissions(
	ctx context.Context,
	dbClient provider.DBClientInterface,
	id string,
	permissions []ResourcePermissions,
	deploymentID string,
) error {
	_, err := dbClient.ExecuteContext(ctx, queryDeleteRolePermissions, id, deploymentID)
	if err != nil {
		return fmt.Errorf("failed to delete existing role permissions: %w", err)
	}

	err = addPermissionsToRole(ctx, dbClient, id, permissions, deploymentID)
	if err != nil {
		return fmt.Errorf("failed to assign permissions to role: %w", err)
	}
	return nil
}

// CheckRoleNameExists checks if a role with the given name exists in the specified organization unit.
func (s *roleStore) CheckRoleNameExists(ctx context.Context, ouID, name string) (bool, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return false, err
	}

	results, err := dbClient.QueryContext(ctx, queryCheckRoleNameExists, ouID, name, s.deploymentID)
	if err != nil {
		return false, fmt.Errorf("failed to check role name existence: %w", err)
	}

	return parseBoolFromCount(results)
}

// CheckRoleNameExistsExcludingID checks if a role with the given name exists in the specified organization unit,
// excluding the role with the given ID.
func (s *roleStore) CheckRoleNameExistsExcludingID(
	ctx context.Context, ouID, name, excludeRoleID string) (bool, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return false, err
	}

	results, err := dbClient.QueryContext(
		ctx, queryCheckRoleNameExistsExcludingID, ouID, name, excludeRoleID, s.deploymentID)
	if err != nil {
		return false, fmt.Errorf("failed to check role name existence: %w", err)
	}

	return parseBoolFromCount(results)
}

// GetAuthorizedPermissions retrieves the permissions that an entity is authorized for based on their
// direct role assignments and group memberships.
func (s *roleStore) GetAuthorizedPermissions(
	ctx context.Context,
	entityID string,
	groupIDs []string,
	requestedPermissions []string,
) ([]string, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return nil, err
	}

	// Handle nil groupIDs slice
	if groupIDs == nil {
		groupIDs = []string{}
	}

	// Build dynamic query based on provided parameters
	query, args := buildAuthorizedPermissionsQuery(entityID, groupIDs, requestedPermissions, s.deploymentID)

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get authorized permissions: %w", err)
	}

	permissions := make([]string, 0)
	for _, row := range results {
		if permission, ok := row["permission"].(string); ok {
			permissions = append(permissions, permission)
		}
	}

	return permissions, nil
}

// GetUserRoles retrieves the names of roles assigned to an entity directly and/or through group membership.
func (s *roleStore) GetUserRoles(
	ctx context.Context, entityID string, groupIDs []string,
) ([]string, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return nil, err
	}

	if groupIDs == nil {
		groupIDs = []string{}
	}

	query, args := buildUserRolesQuery(entityID, groupIDs, s.deploymentID)

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity roles: %w", err)
	}

	roles := make([]string, 0)
	for _, row := range results {
		if name, ok := row["name"].(string); ok {
			roles = append(roles, name)
		}
	}

	return roles, nil
}

// GetEntityRoleIDs retrieves the IDs of roles assigned to an entity directly and/or via
// group membership, without joining the ROLE table. This surfaces assignments to roles
// whose definitions live only in the file-based declarative store. Callers (notably the
// composite store) use this to resolve permissions for declarative roles whose permission
// rows are absent from the DB.
func (s *roleStore) GetEntityRoleIDs(
	ctx context.Context, entityID string, groupIDs []string,
) ([]string, error) {
	if groupIDs == nil {
		groupIDs = []string{}
	}
	// Short-circuit before touching the DB when there's nothing to look up.
	if entityID == "" && len(groupIDs) == 0 {
		return []string{}, nil
	}

	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return nil, err
	}

	query, args := buildEntityRoleIDsQuery(entityID, groupIDs, s.deploymentID)

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity role IDs: %w", err)
	}

	roleIDs := make([]string, 0, len(results))
	for _, row := range results {
		if id, ok := row["role_id"].(string); ok {
			roleIDs = append(roleIDs, id)
		}
	}

	return roleIDs, nil
}

// IsRoleDeclarative checks if a role is defined in declarative configuration.
func (s *roleStore) IsRoleDeclarative(ctx context.Context, roleID string) (bool, error) {
	// A role is considered declarative if:
	// 1. Store mode is "composite" or "declarative"
	// 2. The role exists in the declarative configuration
	storeMode := getRoleStoreMode()
	if storeMode == serverconst.StoreModeMutable {
		// Mutable mode: no roles are declarative
		return false, nil
	}

	// For declarative and composite modes, check if role is in declarative config
	// This would require integration with the declarative resource system
	// For now, return false as placeholder; actual implementation would check config
	return false, nil
}

// getConfigDBClient is a helper method to get the database client for the config database.
func (s *roleStore) getConfigDBClient() (provider.DBClientInterface, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	return dbClient, nil
}

// parseCountResult parses a count result from a database query result.
func parseCountResult(results []map[string]interface{}) (int, error) {
	if len(results) == 0 {
		return 0, nil
	}

	if countVal, ok := results[0]["total"].(int64); ok {
		return int(countVal), nil
	}
	return 0, fmt.Errorf("failed to parse total from query result")
}

// parseBoolFromCount parses a count result and returns true if count > 0.
func parseBoolFromCount(results []map[string]interface{}) (bool, error) {
	if len(results) == 0 {
		return false, nil
	}

	if countVal, ok := results[0]["count"].(int64); ok {
		return countVal > 0, nil
	}
	return false, fmt.Errorf("failed to parse count from query result")
}

// parseStringField extracts a string field from a database result row.
func parseStringField(row map[string]interface{}, fieldName string) (string, error) {
	value, ok := row[fieldName].(string)
	if !ok {
		return "", fmt.Errorf("failed to parse %s as string", fieldName)
	}
	return value, nil
}

// parseStringFields extracts multiple string fields from a database result row.
func parseStringFields(row map[string]interface{}, fieldNames ...string) ([]string, error) {
	result := make([]string, len(fieldNames))
	for i, fieldName := range fieldNames {
		value, err := parseStringField(row, fieldName)
		if err != nil {
			return nil, err
		}
		result[i] = value
	}
	return result, nil
}
