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
	"fmt"
	"strings"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// queryCreateRole creates a new role.
	queryCreateRole = dbmodel.DBQuery{
		ID:    "RLQ-ROLE_MGT-01",
		Query: `INSERT INTO "ROLE" (ID, OU_ID, NAME, DESCRIPTION, DEPLOYMENT_ID) VALUES ($1, $2, $3, $4, $5)`,
	}

	// queryGetRoleByID retrieves a role by ID.
	queryGetRoleByID = dbmodel.DBQuery{
		ID:    "RLQ-ROLE_MGT-02",
		Query: `SELECT ID, OU_ID, NAME, DESCRIPTION FROM "ROLE" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetRoleList retrieves a list of roles with pagination.
	queryGetRoleList = dbmodel.DBQuery{
		ID: "RLQ-ROLE_MGT-03",
		Query: `SELECT ID, OU_ID, NAME, DESCRIPTION FROM "ROLE" ` +
			`WHERE DEPLOYMENT_ID = $3 ORDER BY CREATED_AT DESC LIMIT $1 OFFSET $2`,
	}

	// queryGetRoleListCount retrieves the total count of roles.
	queryGetRoleListCount = dbmodel.DBQuery{
		ID:    "RLQ-ROLE_MGT-04",
		Query: `SELECT COUNT(*) as total FROM "ROLE" WHERE DEPLOYMENT_ID = $1`,
	}

	// queryUpdateRole updates a role.
	queryUpdateRole = dbmodel.DBQuery{
		ID:    "RLQ-ROLE_MGT-05",
		Query: `UPDATE "ROLE" SET OU_ID = $1, NAME = $2, DESCRIPTION = $3 WHERE ID = $4 AND DEPLOYMENT_ID = $5`,
	}

	// queryDeleteRole deletes a role.
	queryDeleteRole = dbmodel.DBQuery{
		ID:    "RLQ-ROLE_MGT-06",
		Query: `DELETE FROM "ROLE" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCreateRolePermission creates a new role permission.
	queryCreateRolePermission = dbmodel.DBQuery{
		ID: "RLQ-ROLE_MGT-07",
		Query: `INSERT INTO "ROLE_PERMISSION" (ROLE_ID, RESOURCE_SERVER_ID, PERMISSION, ` +
			`DEPLOYMENT_ID) VALUES ($1, $2, $3, $4)`,
	}

	// queryGetRolePermissions retrieves all permissions for a role.
	queryGetRolePermissions = dbmodel.DBQuery{
		ID: "RLQ-ROLE_MGT-08",
		Query: `SELECT RESOURCE_SERVER_ID, PERMISSION FROM "ROLE_PERMISSION" WHERE ` +
			`ROLE_ID = $1 AND DEPLOYMENT_ID = $2 ORDER BY CREATED_AT`,
	}

	// queryDeleteRolePermissions deletes all permissions for a role.
	queryDeleteRolePermissions = dbmodel.DBQuery{
		ID:    "RLQ-ROLE_MGT-09",
		Query: `DELETE FROM "ROLE_PERMISSION" WHERE ROLE_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCreateRoleAssignment creates a new role assignment.
	queryCreateRoleAssignment = dbmodel.DBQuery{
		ID: "RLQ-ROLE_MGT-10",
		Query: `INSERT INTO "ROLE_ASSIGNMENT" (ROLE_ID, ASSIGNEE_TYPE, ASSIGNEE_ID, DEPLOYMENT_ID)
			VALUES ($1, $2, $3, $4) ON CONFLICT (ROLE_ID, DEPLOYMENT_ID, ASSIGNEE_TYPE, ASSIGNEE_ID) DO NOTHING`,
	}

	// queryGetRoleAssignments retrieves all assignments for a role with pagination.
	queryGetRoleAssignments = dbmodel.DBQuery{
		ID: "RLQ-ROLE_MGT-11",
		Query: `SELECT ASSIGNEE_ID, ASSIGNEE_TYPE FROM "ROLE_ASSIGNMENT"
			WHERE ROLE_ID = $1 AND DEPLOYMENT_ID = $4 ORDER BY CREATED_AT LIMIT $2 OFFSET $3`,
	}

	// queryGetRoleAssignmentsCount retrieves the total count of assignments for a role.
	queryGetRoleAssignmentsCount = dbmodel.DBQuery{
		ID:    "RLQ-ROLE_MGT-12",
		Query: `SELECT COUNT(*) as total FROM "ROLE_ASSIGNMENT" WHERE ROLE_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryDeleteRoleAssignmentsByIDs deletes specific assignments for a role.
	queryDeleteRoleAssignmentsByIDs = dbmodel.DBQuery{
		ID: "RLQ-ROLE_MGT-13",
		Query: `DELETE FROM "ROLE_ASSIGNMENT" ` +
			`WHERE ROLE_ID = $1 AND ASSIGNEE_TYPE = $2 AND ASSIGNEE_ID = $3 AND DEPLOYMENT_ID = $4`,
	}

	// queryDeleteAllRoleAssignments deletes all assignments for a role (used for cascade delete).
	queryDeleteAllRoleAssignments = dbmodel.DBQuery{
		ID:    "RLQ-ROLE_MGT-19",
		Query: `DELETE FROM "ROLE_ASSIGNMENT" WHERE ROLE_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryDeleteRoleAssignmentsByAssignee deletes all assignments for a given assignee across roles
	// (used to cascade-delete assignments when the assignee principal is deleted).
	queryDeleteRoleAssignmentsByAssignee = dbmodel.DBQuery{
		ID: "RLQ-ROLE_MGT-25",
		Query: `DELETE FROM "ROLE_ASSIGNMENT" ` +
			`WHERE ASSIGNEE_TYPE = $1 AND ASSIGNEE_ID = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryCheckRoleNameExists checks if a role name already exists for a given organization unit.
	queryCheckRoleNameExists = dbmodel.DBQuery{
		ID:    "RLQ-ROLE_MGT-14",
		Query: `SELECT COUNT(*) as count FROM "ROLE" WHERE OU_ID = $1 AND NAME = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryCheckRoleNameExistsExcludingID checks if a role name exists for an OU excluding a specific role ID.
	queryCheckRoleNameExistsExcludingID = dbmodel.DBQuery{
		ID: "RLQ-ROLE_MGT-15",
		Query: `SELECT COUNT(*) as count FROM "ROLE"
			WHERE OU_ID = $1 AND NAME = $2 AND ID != $3 AND DEPLOYMENT_ID = $4`,
	}

	// queryCheckRoleExists checks if a role exists by its ID.
	queryCheckRoleExists = dbmodel.DBQuery{
		ID:    "RLQ-ROLE_MGT-16",
		Query: `SELECT COUNT(*) as count FROM "ROLE" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetRoleAssignmentsByType retrieves assignments for a role filtered by assignee type with pagination.
	queryGetRoleAssignmentsByType = dbmodel.DBQuery{
		ID: "RLQ-ROLE_MGT-17",
		Query: `SELECT ASSIGNEE_ID, ASSIGNEE_TYPE FROM "ROLE_ASSIGNMENT"
			WHERE ROLE_ID = $1 AND ASSIGNEE_TYPE = $5 AND DEPLOYMENT_ID = $4 ORDER BY CREATED_AT LIMIT $2 OFFSET $3`,
	}

	// queryGetRoleAssignmentsCountByType retrieves the total count of assignments for a role filtered by type.
	queryGetRoleAssignmentsCountByType = dbmodel.DBQuery{
		ID: "RLQ-ROLE_MGT-18",
		Query: `SELECT COUNT(*) as total FROM "ROLE_ASSIGNMENT"
			WHERE ROLE_ID = $1 AND ASSIGNEE_TYPE = $3 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetRoleListByOUID retrieves a list of roles belonging to an organization unit with pagination.
	queryGetRoleListByOUID = dbmodel.DBQuery{
		ID: "RLQ-ROLE_MGT-23",
		Query: `SELECT ID, OU_ID, NAME, DESCRIPTION FROM "ROLE" ` +
			`WHERE OU_ID = $1 AND DEPLOYMENT_ID = $4 ORDER BY CREATED_AT DESC LIMIT $2 OFFSET $3`,
	}

	// queryGetRoleListCountByOUID retrieves the total count of roles belonging to an organization unit.
	queryGetRoleListCountByOUID = dbmodel.DBQuery{
		ID:    "RLQ-ROLE_MGT-24",
		Query: `SELECT COUNT(*) as total FROM "ROLE" WHERE OU_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)

// buildAuthorizedPermissionsQuery constructs a database-specific query to retrieve authorized permissions
// for an entity and/or groups from their assigned roles.
// It builds separate queries for PostgreSQL and SQLite to handle array parameters correctly.
func buildAuthorizedPermissionsQuery(
	entityID string,
	groupIDs []string,
	resourceServerID string,
	requestedPermissions []string,
	deploymentID string,
) (dbmodel.DBQuery, []interface{}) {
	// Base query structure
	baseQuery := `SELECT DISTINCT rp.PERMISSION
		FROM "ROLE_PERMISSION" rp
		INNER JOIN "ROLE_ASSIGNMENT" ra ON rp.ROLE_ID = ra.ROLE_ID AND rp.DEPLOYMENT_ID = $1 AND ra.DEPLOYMENT_ID = $1
		WHERE rp.DEPLOYMENT_ID = $1 AND `

	var postgresWhere []string
	var sqliteWhere []string

	// Pre-allocate args slice with estimated capacity
	argsCapacity := 1 + len(groupIDs) + len(requestedPermissions) // +1 for DEPLOYMENT_ID
	if entityID != "" {
		argsCapacity++
	}
	if resourceServerID != "" {
		argsCapacity++
	}
	args := make([]interface{}, 0, argsCapacity)
	args = append(args, deploymentID)
	paramIndex := 2 // Start from $2 since $1 is DEPLOYMENT_ID

	// Build entity condition if entityID is provided
	if entityID != "" {
		postgresWhere = append(postgresWhere,
			fmt.Sprintf("(ra.ASSIGNEE_TYPE = 'entity' AND ra.ASSIGNEE_ID = $%d)", paramIndex))
		sqliteWhere = append(sqliteWhere,
			"(ra.ASSIGNEE_TYPE = 'entity' AND ra.ASSIGNEE_ID = ?)")
		args = append(args, entityID)
		paramIndex++
	}

	// Build group condition if groupIDs are provided
	if len(groupIDs) > 0 {
		groupPlaceholdersPostgres := make([]string, len(groupIDs))
		groupPlaceholdersSqlite := make([]string, len(groupIDs))

		for i, groupID := range groupIDs {
			groupPlaceholdersPostgres[i] = fmt.Sprintf("$%d", paramIndex+i)
			groupPlaceholdersSqlite[i] = "?"
			args = append(args, groupID)
		}

		postgresWhere = append(postgresWhere,
			fmt.Sprintf("(ra.ASSIGNEE_TYPE = 'group' AND ra.ASSIGNEE_ID IN (%s))",
				strings.Join(groupPlaceholdersPostgres, ",")))
		sqliteWhere = append(sqliteWhere,
			fmt.Sprintf("(ra.ASSIGNEE_TYPE = 'group' AND ra.ASSIGNEE_ID IN (%s))",
				strings.Join(groupPlaceholdersSqlite, ",")))
		paramIndex += len(groupIDs)
	}

	var postgresScopeWhere []string
	var sqliteScopeWhere []string
	if resourceServerID != "" {
		postgresScopeWhere = append(postgresScopeWhere, fmt.Sprintf("rp.RESOURCE_SERVER_ID = $%d", paramIndex))
		sqliteScopeWhere = append(sqliteScopeWhere, "rp.RESOURCE_SERVER_ID = ?")
		args = append(args, resourceServerID)
		paramIndex++
	}

	// Build permission condition
	permPlaceholdersPostgres := make([]string, len(requestedPermissions))
	permPlaceholdersSqlite := make([]string, len(requestedPermissions))

	for i, perm := range requestedPermissions {
		permPlaceholdersPostgres[i] = fmt.Sprintf("$%d", paramIndex+i)
		permPlaceholdersSqlite[i] = "?"
		args = append(args, perm)
	}

	// Construct PostgreSQL query: AND together the subject block, the optional
	// resource-server scope, and the permission block.
	postgresConditions := []string{"(" + strings.Join(postgresWhere, " OR ") + ")"}
	postgresConditions = append(postgresConditions, postgresScopeWhere...)
	postgresConditions = append(postgresConditions,
		fmt.Sprintf("rp.PERMISSION IN (%s)", strings.Join(permPlaceholdersPostgres, ",")))
	postgresQuery := baseQuery + strings.Join(postgresConditions, " AND ") + " ORDER BY rp.PERMISSION"

	// Construct SQLite query
	sqliteConditions := []string{"(" + strings.Join(sqliteWhere, " OR ") + ")"}
	sqliteConditions = append(sqliteConditions, sqliteScopeWhere...)
	sqliteConditions = append(sqliteConditions,
		fmt.Sprintf("rp.PERMISSION IN (%s)", strings.Join(permPlaceholdersSqlite, ",")))
	sqliteQuery := baseQuery + strings.Join(sqliteConditions, " AND ") + " ORDER BY rp.PERMISSION"

	query := dbmodel.DBQuery{
		ID:            "RLQ-ROLE_MGT-20",
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}

	return query, args
}

// buildUserRolesQuery constructs a database-specific query to retrieve role names
// assigned to an entity directly and/or through group membership.
func buildUserRolesQuery(
	entityID string,
	groupIDs []string,
	deploymentID string,
) (dbmodel.DBQuery, []interface{}) {
	baseQuery := `SELECT DISTINCT r.NAME
		FROM "ROLE" r
		INNER JOIN "ROLE_ASSIGNMENT" ra ON r.ID = ra.ROLE_ID AND r.DEPLOYMENT_ID = $1 AND ra.DEPLOYMENT_ID = $1
		WHERE r.DEPLOYMENT_ID = $1 AND `

	var postgresWhere []string
	var sqliteWhere []string

	argsCapacity := 1 + len(groupIDs) // +1 for DEPLOYMENT_ID
	if entityID != "" {
		argsCapacity++
	}
	args := make([]interface{}, 0, argsCapacity)
	args = append(args, deploymentID)
	paramIndex := 2 // Start from $2 since $1 is DEPLOYMENT_ID

	// Build entity condition if entityID is provided
	if entityID != "" {
		postgresWhere = append(postgresWhere,
			fmt.Sprintf("(ra.ASSIGNEE_TYPE = 'entity' AND ra.ASSIGNEE_ID = $%d)", paramIndex))
		sqliteWhere = append(sqliteWhere,
			"(ra.ASSIGNEE_TYPE = 'entity' AND ra.ASSIGNEE_ID = ?)")
		args = append(args, entityID)
		paramIndex++
	}

	// Build group condition if groupIDs are provided
	if len(groupIDs) > 0 {
		groupPlaceholdersPostgres := make([]string, len(groupIDs))
		groupPlaceholdersSqlite := make([]string, len(groupIDs))

		for i, groupID := range groupIDs {
			groupPlaceholdersPostgres[i] = fmt.Sprintf("$%d", paramIndex+i)
			groupPlaceholdersSqlite[i] = "?"
			args = append(args, groupID)
		}

		postgresWhere = append(postgresWhere,
			fmt.Sprintf("(ra.ASSIGNEE_TYPE = 'group' AND ra.ASSIGNEE_ID IN (%s))",
				strings.Join(groupPlaceholdersPostgres, ",")))
		sqliteWhere = append(sqliteWhere,
			fmt.Sprintf("(ra.ASSIGNEE_TYPE = 'group' AND ra.ASSIGNEE_ID IN (%s))",
				strings.Join(groupPlaceholdersSqlite, ",")))
	}

	// Construct PostgreSQL query
	postgresQuery := baseQuery +
		"(" + strings.Join(postgresWhere, " OR ") + ")" +
		" ORDER BY r.NAME"

	// Construct SQLite query
	sqliteQuery := baseQuery +
		"(" + strings.Join(sqliteWhere, " OR ") + ")" +
		" ORDER BY r.NAME"

	query := dbmodel.DBQuery{
		ID:            "RLQ-ROLE_MGT-21",
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}

	return query, args
}

// buildEntityRoleIDsQuery constructs a database-specific query to retrieve the IDs of roles
// assigned to an entity directly and/or through group membership. Unlike buildUserRolesQuery
// this does not join the ROLE table, so it returns assignments even when the role itself
// lives only in a declarative file-based store. Used by the composite store to bridge the
// gap between DB-stored assignments and file-stored role definitions for permission lookup.
func buildEntityRoleIDsQuery(
	entityID string,
	groupIDs []string,
	deploymentID string,
) (dbmodel.DBQuery, []interface{}) {
	baseQuery := `SELECT DISTINCT ra.ROLE_ID
		FROM "ROLE_ASSIGNMENT" ra
		WHERE ra.DEPLOYMENT_ID = $1 AND `

	var postgresWhere []string
	var sqliteWhere []string

	argsCapacity := 1 + len(groupIDs)
	if entityID != "" {
		argsCapacity++
	}
	args := make([]interface{}, 0, argsCapacity)
	args = append(args, deploymentID)
	paramIndex := 2

	if entityID != "" {
		postgresWhere = append(postgresWhere,
			fmt.Sprintf("(ra.ASSIGNEE_TYPE = 'entity' AND ra.ASSIGNEE_ID = $%d)", paramIndex))
		sqliteWhere = append(sqliteWhere,
			"(ra.ASSIGNEE_TYPE = 'entity' AND ra.ASSIGNEE_ID = ?)")
		args = append(args, entityID)
		paramIndex++
	}

	if len(groupIDs) > 0 {
		groupPlaceholdersPostgres := make([]string, len(groupIDs))
		groupPlaceholdersSqlite := make([]string, len(groupIDs))

		for i, groupID := range groupIDs {
			groupPlaceholdersPostgres[i] = fmt.Sprintf("$%d", paramIndex+i)
			groupPlaceholdersSqlite[i] = "?"
			args = append(args, groupID)
		}

		postgresWhere = append(postgresWhere,
			fmt.Sprintf("(ra.ASSIGNEE_TYPE = 'group' AND ra.ASSIGNEE_ID IN (%s))",
				strings.Join(groupPlaceholdersPostgres, ",")))
		sqliteWhere = append(sqliteWhere,
			fmt.Sprintf("(ra.ASSIGNEE_TYPE = 'group' AND ra.ASSIGNEE_ID IN (%s))",
				strings.Join(groupPlaceholdersSqlite, ",")))
	}

	postgresQuery := baseQuery +
		"(" + strings.Join(postgresWhere, " OR ") + ")"
	sqliteQuery := baseQuery +
		"(" + strings.Join(sqliteWhere, " OR ") + ")"

	query := dbmodel.DBQuery{
		ID:            "RLQ-ROLE_MGT-22",
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}

	return query, args
}
