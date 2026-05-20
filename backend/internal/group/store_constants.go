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
	"fmt"
	"strings"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// QueryGetGroupListCount is the query to get total count of groups.
	QueryGetGroupListCount = dbmodel.DBQuery{
		ID:    "GRQ-GROUP_MGT-01",
		Query: `SELECT COUNT(*) as total FROM "GROUP" WHERE DEPLOYMENT_ID = $1`,
	}

	// QueryGetGroupList is the query to get groups with pagination.
	QueryGetGroupList = dbmodel.DBQuery{
		ID: "GRQ-GROUP_MGT-02",
		Query: `SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" ` +
			`WHERE DEPLOYMENT_ID = $3 ORDER BY NAME LIMIT $1 OFFSET $2`,
	}
)

// buildGetGroupsCountByOUIDsQuery returns the query and args to count groups
// belonging to the specified list of organization unit IDs.
func buildGetGroupsCountByOUIDsQuery(
	ouIDs []string, deploymentID string,
) (dbmodel.DBQuery, []interface{}) {
	if len(ouIDs) == 0 {
		return dbmodel.DBQuery{
			ID:            "GRQ-GROUP_MGT-03",
			Query:         "SELECT 0 WHERE 1=0",
			PostgresQuery: "SELECT 0 WHERE 1=0",
			SQLiteQuery:   "SELECT 0 WHERE 1=0",
		}, []interface{}{}
	}

	postgresPlaceholders := make([]string, len(ouIDs))
	sqlitePlaceholders := make([]string, len(ouIDs))
	for i := range ouIDs {
		postgresPlaceholders[i] = fmt.Sprintf("$%d", i+1)
		sqlitePlaceholders[i] = "?"
	}
	deploymentIDIdx := len(ouIDs) + 1

	postgresQuery := fmt.Sprintf(
		`SELECT COUNT(*) as total FROM "GROUP" WHERE OU_ID IN (%s) AND DEPLOYMENT_ID = $%d`,
		strings.Join(postgresPlaceholders, ","), deploymentIDIdx)
	sqliteQuery := fmt.Sprintf(
		`SELECT COUNT(*) as total FROM "GROUP" WHERE OU_ID IN (%s) AND DEPLOYMENT_ID = ?`,
		strings.Join(sqlitePlaceholders, ","))

	args := make([]interface{}, 0, len(ouIDs)+1)
	for _, id := range ouIDs {
		args = append(args, id)
	}
	args = append(args, deploymentID)

	return dbmodel.DBQuery{
		ID:            "GRQ-GROUP_MGT-03",
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}, args
}

// buildGetGroupsByOUIDsQuery returns the query and args to retrieve paginated groups
// filtered by the specified list of organization unit IDs.
func buildGetGroupsByOUIDsQuery(
	ouIDs []string, limit, offset int, deploymentID string,
) (dbmodel.DBQuery, []interface{}) {
	if len(ouIDs) == 0 {
		return dbmodel.DBQuery{
			ID:            "GRQ-GROUP_MGT-04",
			Query:         `SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" WHERE 1=0`,
			PostgresQuery: `SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" WHERE 1=0`,
			SQLiteQuery:   `SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" WHERE 1=0`,
		}, []interface{}{}
	}

	postgresPlaceholders := make([]string, len(ouIDs))
	sqlitePlaceholders := make([]string, len(ouIDs))
	for i := range ouIDs {
		postgresPlaceholders[i] = fmt.Sprintf("$%d", i+1)
		sqlitePlaceholders[i] = "?"
	}
	deploymentIDIdx := len(ouIDs) + 1
	limitIdx := len(ouIDs) + 2
	offsetIdx := len(ouIDs) + 3

	postgresQuery := fmt.Sprintf(
		`SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" `+
			`WHERE OU_ID IN (%s) AND DEPLOYMENT_ID = $%d ORDER BY NAME LIMIT $%d OFFSET $%d`,
		strings.Join(postgresPlaceholders, ","), deploymentIDIdx, limitIdx, offsetIdx)
	sqliteQuery := fmt.Sprintf(
		`SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" `+
			`WHERE OU_ID IN (%s) AND DEPLOYMENT_ID = ? ORDER BY NAME LIMIT ? OFFSET ?`,
		strings.Join(sqlitePlaceholders, ","))

	args := make([]interface{}, 0, len(ouIDs)+3)
	for _, id := range ouIDs {
		args = append(args, id)
	}
	args = append(args, deploymentID, limit, offset)

	return dbmodel.DBQuery{
		ID:            "GRQ-GROUP_MGT-04",
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}, args
}

var (
	// QueryCreateGroup is the query to create a new group.
	QueryCreateGroup = dbmodel.DBQuery{
		ID: "GRQ-GROUP_MGT-05",
		Query: `INSERT INTO "GROUP" ` +
			`(ID, OU_ID, NAME, DESCRIPTION, DEPLOYMENT_ID, CREATED_AT, UPDATED_AT) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7)`,
	}

	// QueryGetGroupByID is the query to get a group by id.
	QueryGetGroupByID = dbmodel.DBQuery{
		ID:    "GRQ-GROUP_MGT-06",
		Query: `SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// QueryGetGroupMembers is the query to get members assigned to a group.
	QueryGetGroupMembers = dbmodel.DBQuery{
		ID: "GRQ-GROUP_MGT-07",
		Query: `SELECT MEMBER_ID, MEMBER_TYPE FROM "GROUP_MEMBER_REFERENCE" ` +
			`WHERE GROUP_ID = $1 AND DEPLOYMENT_ID = $4 ORDER BY MEMBER_TYPE, MEMBER_ID LIMIT $2 OFFSET $3`,
	}

	// QueryGetGroupMemberCount is the query to get total count of members in a group.
	QueryGetGroupMemberCount = dbmodel.DBQuery{
		ID:    "GRQ-GROUP_MGT-08",
		Query: `SELECT COUNT(*) as total FROM "GROUP_MEMBER_REFERENCE" WHERE GROUP_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// QueryUpdateGroup is the query to update a group.
	QueryUpdateGroup = dbmodel.DBQuery{
		ID: "GRQ-GROUP_MGT-09",
		Query: `UPDATE "GROUP" SET OU_ID = $2, NAME = $3, DESCRIPTION = $4, UPDATED_AT = $5 ` +
			`WHERE ID = $1 AND DEPLOYMENT_ID = $6`,
	}

	// QueryDeleteGroup is the query to delete a group.
	QueryDeleteGroup = dbmodel.DBQuery{
		ID:    "GRQ-GROUP_MGT-10",
		Query: `DELETE FROM "GROUP" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// QueryDeleteGroupMembers is the query to delete all members assigned to a group.
	QueryDeleteGroupMembers = dbmodel.DBQuery{
		ID:    "GRQ-GROUP_MGT-11",
		Query: `DELETE FROM "GROUP_MEMBER_REFERENCE" WHERE GROUP_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// QueryAddMemberToGroup is the query to assign member to a group.
	QueryAddMemberToGroup = dbmodel.DBQuery{
		ID: "GRQ-GROUP_MGT-12",
		Query: `INSERT INTO "GROUP_MEMBER_REFERENCE" ` +
			`(GROUP_ID, MEMBER_TYPE, MEMBER_ID, DEPLOYMENT_ID, CREATED_AT, UPDATED_AT) ` +
			`VALUES ($1, $2, $3, $4, $5, $6) ` +
			`ON CONFLICT (GROUP_ID, MEMBER_TYPE, MEMBER_ID, DEPLOYMENT_ID) DO NOTHING`,
	}

	// QueryCheckGroupNameConflict is the query to check if a group name conflicts within the same organization unit.
	QueryCheckGroupNameConflict = dbmodel.DBQuery{
		ID:    "GRQ-GROUP_MGT-13",
		Query: `SELECT COUNT(*) as count FROM "GROUP" WHERE NAME = $1 AND OU_ID = $2 AND DEPLOYMENT_ID = $3`,
	}

	// QueryCheckGroupNameConflictForUpdate is the query to check name conflict during update.
	QueryCheckGroupNameConflictForUpdate = dbmodel.DBQuery{
		ID: "GRQ-GROUP_MGT-14",
		Query: `SELECT COUNT(*) as count FROM "GROUP" ` +
			`WHERE NAME = $1 AND OU_ID = $2 AND ID != $3 AND DEPLOYMENT_ID = $4`,
	}

	// QueryGetGroupsByOrganizationUnitCount is the query to get total count of groups by organization unit.
	QueryGetGroupsByOrganizationUnitCount = dbmodel.DBQuery{
		ID:    "GRQ-GROUP_MGT-15",
		Query: `SELECT COUNT(*) as total FROM "GROUP" WHERE OU_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// QueryGetGroupsByOrganizationUnit is the query to get groups by organization unit with pagination.
	QueryGetGroupsByOrganizationUnit = dbmodel.DBQuery{
		ID: "GRQ-GROUP_MGT-16",
		Query: `SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" ` +
			`WHERE OU_ID = $1 AND DEPLOYMENT_ID = $4 ORDER BY NAME LIMIT $2 OFFSET $3`,
	}

	// QueryDeleteGroupMember is the query to delete a specific member from a group.
	QueryDeleteGroupMember = dbmodel.DBQuery{
		ID: "GRQ-GROUP_MGT-17",
		Query: `DELETE FROM "GROUP_MEMBER_REFERENCE" ` +
			`WHERE GROUP_ID = $1 AND MEMBER_TYPE = $2 AND MEMBER_ID = $3 AND DEPLOYMENT_ID = $4`,
	}
)

// buildGroupINClauseQuery constructs a query with an IN clause for group IDs.
func buildGroupINClauseQuery(
	queryID, baseQuery string, groupIDs []string, deploymentID string,
) (dbmodel.DBQuery, []interface{}, error) {
	if len(groupIDs) == 0 {
		return dbmodel.DBQuery{}, nil, fmt.Errorf("groupIDs list cannot be empty")
	}

	args := make([]interface{}, len(groupIDs)+1)

	postgresPlaceholders := make([]string, len(groupIDs))
	sqlitePlaceholders := make([]string, len(groupIDs))

	for i, groupID := range groupIDs {
		postgresPlaceholders[i] = fmt.Sprintf("$%d", i+1)
		sqlitePlaceholders[i] = "?"
		args[i] = groupID
	}
	args[len(groupIDs)] = deploymentID

	deploymentPlaceholder := fmt.Sprintf("$%d", len(groupIDs)+1)
	postgresQuery := fmt.Sprintf(baseQuery, strings.Join(postgresPlaceholders, ","), deploymentPlaceholder)
	sqliteQuery := fmt.Sprintf(baseQuery, strings.Join(sqlitePlaceholders, ","), "?")

	query := dbmodel.DBQuery{
		ID:            queryID,
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}

	return query, args, nil
}

// buildBulkGroupExistsQuery constructs a query to check which group IDs exist from a list.
func buildBulkGroupExistsQuery(groupIDs []string, deploymentID string) (dbmodel.DBQuery, []interface{}, error) {
	return buildGroupINClauseQuery(
		"GRQ-GROUP_MGT-18",
		`SELECT ID FROM "GROUP" WHERE ID IN (%s) AND DEPLOYMENT_ID = %s`,
		groupIDs, deploymentID,
	)
}

// buildGetGroupsByIDsQuery constructs a query to fetch groups by a list of IDs.
func buildGetGroupsByIDsQuery(groupIDs []string, deploymentID string) (dbmodel.DBQuery, []interface{}, error) {
	return buildGroupINClauseQuery(
		"GRQ-GROUP_MGT-19",
		`SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" WHERE ID IN (%s) AND DEPLOYMENT_ID = %s`,
		groupIDs, deploymentID,
	)
}
