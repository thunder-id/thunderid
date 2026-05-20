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

package entity

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/utils"
)

const (
	// AttributesColumn represents the ATTRIBUTES column name in the database.
	AttributesColumn = "ATTRIBUTES"

	// SystemAttributesColumn represents the SYSTEM_ATTRIBUTES column name in the database.
	SystemAttributesColumn = "SYSTEM_ATTRIBUTES"

	// MaxIndexedAttributesCount is the maximum number of indexed attributes allowed.
	MaxIndexedAttributesCount = 20
)

var (
	// QueryGetEntityCount is the query to get total count of entities by category.
	QueryGetEntityCount = model.DBQuery{
		ID:    "ASQ-ENTITY_MGT-01",
		Query: `SELECT COUNT(*) as total FROM "ENTITY" WHERE CATEGORY = $1 AND DEPLOYMENT_ID = $2`,
	}
	// QueryGetEntityList is the query to get a list of entities by category.
	QueryGetEntityList = model.DBQuery{
		ID: "ASQ-ENTITY_MGT-02",
		Query: `SELECT ID, OU_ID, CATEGORY, TYPE, STATE, ATTRIBUTES, SYSTEM_ATTRIBUTES FROM "ENTITY" ` +
			`WHERE CATEGORY = $4 AND DEPLOYMENT_ID = $3 ORDER BY ID LIMIT $1 OFFSET $2`,
	}
	// QuerySearchEntityList is the query to search entities across all categories.
	QuerySearchEntityList = model.DBQuery{
		ID: "ASQ-ENTITY_MGT-03",
		Query: `SELECT ID, OU_ID, CATEGORY, TYPE, STATE, ATTRIBUTES, SYSTEM_ATTRIBUTES FROM "ENTITY" ` +
			`WHERE DEPLOYMENT_ID = $3 ORDER BY ID LIMIT $1 OFFSET $2`,
	}
	// QueryCreateEntity is the query to create a new entity.
	QueryCreateEntity = model.DBQuery{
		ID: "ASQ-ENTITY_MGT-04",
		Query: `INSERT INTO "ENTITY" ` +
			`(ID, DEPLOYMENT_ID, CATEGORY, TYPE, STATE, OU_ID, ` +
			`ATTRIBUTES, SYSTEM_ATTRIBUTES, CREDENTIALS, SYSTEM_CREDENTIALS, CREATED_AT, UPDATED_AT) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
	}
	// QueryGetEntityByID is the query to get an entity by ID.
	QueryGetEntityByID = model.DBQuery{
		ID: "ASQ-ENTITY_MGT-05",
		Query: `SELECT ID, OU_ID, CATEGORY, TYPE, STATE, ATTRIBUTES, SYSTEM_ATTRIBUTES ` +
			`FROM "ENTITY" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// QueryUpdateEntity is the query to fully update an entity including system attributes.
	QueryUpdateEntity = model.DBQuery{
		ID: "ASQ-ENTITY_MGT-06",
		Query: `UPDATE "ENTITY" SET OU_ID = $2, TYPE = $3, STATE = $4, ATTRIBUTES = $5, SYSTEM_ATTRIBUTES = $6, ` +
			`UPDATED_AT = $7 WHERE ID = $1 AND DEPLOYMENT_ID = $8`,
	}
	// QueryUpdateAttributes is the query to update only the schema attributes of an entity.
	QueryUpdateAttributes = model.DBQuery{
		ID:    "ASQ-ENTITY_MGT-07",
		Query: `UPDATE "ENTITY" SET ATTRIBUTES = $2, UPDATED_AT = $3 WHERE ID = $1 AND DEPLOYMENT_ID = $4`,
	}
	// QueryUpdateSystemAttributes is the query to update system attributes.
	QueryUpdateSystemAttributes = model.DBQuery{
		ID:    "ASQ-ENTITY_MGT-08",
		Query: `UPDATE "ENTITY" SET SYSTEM_ATTRIBUTES = $2, UPDATED_AT = $3 WHERE ID = $1 AND DEPLOYMENT_ID = $4`,
	}
	// QueryUpdateCredentials is the query to update credentials.
	QueryUpdateCredentials = model.DBQuery{
		ID:    "ASQ-ENTITY_MGT-09",
		Query: `UPDATE "ENTITY" SET CREDENTIALS = $2, UPDATED_AT = $3 WHERE ID = $1 AND DEPLOYMENT_ID = $4`,
	}
	// QueryUpdateSystemCredentials is the query to update system credentials.
	QueryUpdateSystemCredentials = model.DBQuery{
		ID:    "ASQ-ENTITY_MGT-10",
		Query: `UPDATE "ENTITY" SET SYSTEM_CREDENTIALS = $2, UPDATED_AT = $3 WHERE ID = $1 AND DEPLOYMENT_ID = $4`,
	}
	// QueryDeleteEntity is the query to delete an entity.
	QueryDeleteEntity = model.DBQuery{
		ID:    "ASQ-ENTITY_MGT-11",
		Query: `DELETE FROM "ENTITY" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// QueryGetEntityWithCredentials is the query to get an entity with all credential columns.
	QueryGetEntityWithCredentials = model.DBQuery{
		ID: "ASQ-ENTITY_MGT-12",
		Query: `SELECT ID, OU_ID, CATEGORY, TYPE, STATE, ATTRIBUTES, ` +
			`SYSTEM_ATTRIBUTES, CREDENTIALS, SYSTEM_CREDENTIALS ` +
			`FROM "ENTITY" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// QueryGetGroupCountForEntity is the query to get the count of groups for a given entity.
	QueryGetGroupCountForEntity = model.DBQuery{
		ID:    "ASQ-ENTITY_MGT-13",
		Query: `SELECT COUNT(*) AS total FROM "GROUP_MEMBER_REFERENCE" WHERE MEMBER_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// QueryGetGroupsForEntity is the query to get groups for a given entity with pagination.
	QueryGetGroupsForEntity = model.DBQuery{
		ID: "ASQ-ENTITY_MGT-14",
		Query: `SELECT G.ID, G.OU_ID, G.NAME FROM "GROUP_MEMBER_REFERENCE" GMR ` +
			`INNER JOIN "GROUP" G ON GMR.GROUP_ID = G.ID AND GMR.DEPLOYMENT_ID = $4 AND G.DEPLOYMENT_ID = $4 ` +
			`WHERE GMR.MEMBER_ID = $1 AND GMR.DEPLOYMENT_ID = $4 ` +
			`ORDER BY G.NAME LIMIT $2 OFFSET $3`,
	}
	// QueryGetTransitiveGroupsForEntity retrieves all groups an entity belongs to, including groups
	// inherited through nested group membership, using a recursive CTE.
	QueryGetTransitiveGroupsForEntity = model.DBQuery{
		ID: "ASQ-ENTITY_MGT-15",
		Query: `WITH RECURSIVE transitive_groups AS (
			SELECT GMR.GROUP_ID
			FROM "GROUP_MEMBER_REFERENCE" GMR
			WHERE GMR.MEMBER_ID = $1 AND GMR.DEPLOYMENT_ID = $2
			UNION
			SELECT GMR.GROUP_ID
			FROM "GROUP_MEMBER_REFERENCE" GMR
			INNER JOIN transitive_groups tg ON GMR.MEMBER_ID = tg.GROUP_ID
			WHERE GMR.MEMBER_TYPE = 'group' AND GMR.DEPLOYMENT_ID = $2
		)
		SELECT G.ID, G.OU_ID, G.NAME
		FROM transitive_groups tg
		INNER JOIN "GROUP" G ON tg.GROUP_ID = G.ID AND G.DEPLOYMENT_ID = $2
		ORDER BY G.NAME`,
	}
	// QueryBatchInsertIdentifiers is the base query for batch inserting entity identifiers.
	QueryBatchInsertIdentifiers = model.DBQuery{
		ID: "ASQ-ENTITY_MGT-16",
		Query: `INSERT INTO "ENTITY_IDENTIFIER" ` +
			`(ENTITY_ID, NAME, VALUE, SOURCE, DEPLOYMENT_ID, CREATED_AT) VALUES `,
	}
	// QueryDeleteIdentifiersByEntity is the query to delete all identifiers for an entity.
	QueryDeleteIdentifiersByEntity = model.DBQuery{
		ID:    "ASQ-ENTITY_MGT-17",
		Query: `DELETE FROM "ENTITY_IDENTIFIER" WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// QueryDeleteAttributeIdentifiersByEntity is the query to delete only attribute-sourced identifiers for an entity.
	QueryDeleteAttributeIdentifiersByEntity = model.DBQuery{
		ID:    "ASQ-ENTITY_MGT-18",
		Query: `DELETE FROM "ENTITY_IDENTIFIER" WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $2 AND SOURCE = 'attribute'`,
	}
	// QueryDeleteSystemIdentifiersByEntity is the query to delete only system-sourced identifiers for an entity.
	QueryDeleteSystemIdentifiersByEntity = model.DBQuery{
		ID:    "ASQ-ENTITY_MGT-19",
		Query: `DELETE FROM "ENTITY_IDENTIFIER" WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $2 AND SOURCE = 'system'`,
	}
)

// appendOUIDsINClause appends an "AND OU_ID IN (...)" condition to a query for the given OU IDs.
func appendOUIDsINClause(
	query model.DBQuery, args []interface{}, ouIDs []string,
) (model.DBQuery, []interface{}) {
	if len(ouIDs) == 0 {
		denyClause := " AND 1=0"
		return model.DBQuery{
			ID:            query.ID,
			Query:         query.Query + denyClause,
			PostgresQuery: query.PostgresQuery + denyClause,
			SQLiteQuery:   query.SQLiteQuery + denyClause,
		}, args
	}
	startIdx := len(args) + 1

	pgPlaceholders := make([]string, len(ouIDs))
	for i := range ouIDs {
		pgPlaceholders[i] = fmt.Sprintf("$%d", startIdx+i)
	}
	inClausePostgres := fmt.Sprintf(" AND OU_ID IN (%s)", strings.Join(pgPlaceholders, ", "))

	sqlitePlaceholders := make([]string, len(ouIDs))
	for i := range ouIDs {
		sqlitePlaceholders[i] = "?"
	}
	inClauseSQLite := fmt.Sprintf(" AND OU_ID IN (%s)", strings.Join(sqlitePlaceholders, ", "))

	for _, id := range ouIDs {
		args = append(args, id)
	}

	return model.DBQuery{
		ID:            query.ID,
		Query:         query.Query + inClausePostgres,
		PostgresQuery: query.PostgresQuery + inClausePostgres,
		SQLiteQuery:   query.SQLiteQuery + inClauseSQLite,
	}, args
}

// buildEntityCountQueryByOUIDs constructs a count query scoped to a list of organization unit IDs.
func buildEntityCountQueryByOUIDs(
	category string, ouIDs []string, filters map[string]interface{}, deploymentID string,
) (model.DBQuery, []interface{}, error) {
	queryID := "ASQ-ENTITY_MGT-20"
	baseQuery := `SELECT COUNT(*) as total FROM "ENTITY" WHERE CATEGORY = $1`
	args := []interface{}{category}

	if len(filters) > 0 {
		fq, filterArgs, err := buildFilterQueryWithOffset(queryID, baseQuery, filters, len(args))
		if err != nil {
			return model.DBQuery{}, nil, err
		}
		args = append(args, filterArgs...)
		fq, args = appendOUIDsINClause(fq, args, ouIDs)
		fq, args = utils.AppendDeploymentIDToFilterQuery(fq, args, deploymentID)
		return fq, args, nil
	}

	query := model.DBQuery{
		ID:            queryID,
		Query:         baseQuery,
		PostgresQuery: baseQuery,
		SQLiteQuery:   strings.Replace(baseQuery, "$1", "?", 1),
	}

	query, args = appendOUIDsINClause(query, args, ouIDs)
	query, args = utils.AppendDeploymentIDToFilterQuery(query, args, deploymentID)
	return query, args, nil
}

// buildEntityListQueryByOUIDs constructs a paginated list query scoped to a list of organization unit IDs.
func buildEntityListQueryByOUIDs(
	category string, ouIDs []string, filters map[string]interface{}, limit, offset int, deploymentID string,
) (model.DBQuery, []interface{}, error) {
	queryID := "ASQ-ENTITY_MGT-21"
	baseQuery := `SELECT ID, OU_ID, CATEGORY, TYPE, STATE, ATTRIBUTES, SYSTEM_ATTRIBUTES ` +
		`FROM "ENTITY" WHERE CATEGORY = $1`
	args := []interface{}{category}
	var query model.DBQuery

	if len(filters) > 0 {
		fq, filterArgs, err := buildFilterQueryWithOffset(queryID, baseQuery, filters, len(args))
		if err != nil {
			return model.DBQuery{}, nil, err
		}
		args = append(args, filterArgs...)
		fq, args = appendOUIDsINClause(fq, args, ouIDs)
		fq, args = utils.AppendDeploymentIDToFilterQuery(fq, args, deploymentID)
		query = fq
	} else {
		query = model.DBQuery{
			ID:            queryID,
			Query:         baseQuery,
			PostgresQuery: baseQuery,
			SQLiteQuery:   strings.Replace(baseQuery, "$1", "?", 1),
		}
		query, args = appendOUIDsINClause(query, args, ouIDs)
		query, args = utils.AppendDeploymentIDToFilterQuery(query, args, deploymentID)
	}

	postgresQuery, err := buildPaginatedQuery(query.PostgresQuery, len(args), "$")
	if err != nil {
		return model.DBQuery{}, nil, err
	}

	sqliteQuery, err := buildPaginatedQuery(query.SQLiteQuery, len(args), "?")
	if err != nil {
		return model.DBQuery{}, nil, err
	}

	args = append(args, limit, offset)
	return model.DBQuery{
		ID:            queryID,
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}, args, nil
}

// buildIdentifyQuery constructs a query to identify an entity based on the provided filters.
// It searches both ATTRIBUTES and SYSTEM_ATTRIBUTES columns so that any entity can be found
// regardless of which column holds the filter key.
func buildIdentifyQuery(filters map[string]interface{}, deploymentID string) (model.DBQuery, []interface{}, error) {
	if len(filters) == 0 {
		return model.DBQuery{}, nil, fmt.Errorf("filters cannot be empty")
	}

	keys := make([]string, 0, len(filters))
	for key := range filters {
		if err := utils.ValidateKey(key); err != nil {
			return model.DBQuery{}, nil, fmt.Errorf("invalid filter key: %w", err)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pgQuery := `SELECT ID FROM "ENTITY" WHERE 1=1`
	sqQuery := `SELECT ID FROM "ENTITY" WHERE 1=1`
	args := make([]interface{}, 0, len(keys)+1)

	for i, key := range keys {
		pg, sq := buildDualColumnConditions("", key, i+1)
		pgQuery += pg
		sqQuery += sq
		args = append(args, filters[key])
	}

	pgQuery += fmt.Sprintf(" AND DEPLOYMENT_ID = $%d", len(keys)+1)
	sqQuery += " AND DEPLOYMENT_ID = ?"
	args = append(args, deploymentID)

	return model.DBQuery{
		ID:            "ASQ-ENTITY_MGT-22",
		Query:         pgQuery,
		PostgresQuery: pgQuery,
		SQLiteQuery:   sqQuery,
	}, args, nil
}

// buildEntityINClauseQuery constructs a query with an IN clause for entity IDs.
func buildEntityINClauseQuery(
	queryID, baseQuery string, entityIDs []string, deploymentID string,
) (model.DBQuery, []interface{}, error) {
	if len(entityIDs) == 0 {
		return model.DBQuery{}, nil, fmt.Errorf("entityIDs list cannot be empty")
	}

	args := make([]interface{}, len(entityIDs)+1)

	postgresPlaceholders := make([]string, len(entityIDs))
	sqlitePlaceholders := make([]string, len(entityIDs))

	for i, entityID := range entityIDs {
		postgresPlaceholders[i] = fmt.Sprintf("$%d", i+1)
		sqlitePlaceholders[i] = "?"
		args[i] = entityID
	}
	args[len(entityIDs)] = deploymentID

	deploymentPlaceholder := fmt.Sprintf("$%d", len(entityIDs)+1)
	postgresQuery := fmt.Sprintf(baseQuery, strings.Join(postgresPlaceholders, ","), deploymentPlaceholder)
	sqliteQuery := fmt.Sprintf(baseQuery, strings.Join(sqlitePlaceholders, ","), "?")

	query := model.DBQuery{
		ID:            queryID,
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}

	return query, args, nil
}

// buildBulkEntityExistsQuery constructs a query to check which entity IDs exist from a list.
func buildBulkEntityExistsQuery(entityIDs []string, deploymentID string) (model.DBQuery, []interface{}, error) {
	return buildEntityINClauseQuery(
		"ASQ-ENTITY_MGT-23",
		`SELECT ID FROM "ENTITY" WHERE ID IN (%s) AND DEPLOYMENT_ID = %s`,
		entityIDs, deploymentID,
	)
}

// buildBulkEntityExistsQueryInOUs constructs a query that returns which of the provided entity IDs
// exist AND belong to one of the given organization unit IDs.
func buildBulkEntityExistsQueryInOUs(
	entityIDs []string, ouIDs []string, deploymentID string,
) (model.DBQuery, []interface{}, error) {
	if len(entityIDs) == 0 {
		return model.DBQuery{}, nil, fmt.Errorf("entityIDs list cannot be empty")
	}
	if len(ouIDs) == 0 {
		return model.DBQuery{}, nil, fmt.Errorf("ouIDs list cannot be empty")
	}

	args := make([]interface{}, 0, 1+len(ouIDs)+len(entityIDs))
	args = append(args, deploymentID)

	postgresOUPlaceholders := make([]string, len(ouIDs))
	sqliteOUPlaceholders := make([]string, len(ouIDs))
	for i, ouID := range ouIDs {
		postgresOUPlaceholders[i] = fmt.Sprintf("$%d", i+2)
		sqliteOUPlaceholders[i] = "?"
		args = append(args, ouID)
	}

	idBase := 2 + len(ouIDs)
	postgresIDPlaceholders := make([]string, len(entityIDs))
	sqliteIDPlaceholders := make([]string, len(entityIDs))
	for i, entityID := range entityIDs {
		postgresIDPlaceholders[i] = fmt.Sprintf("$%d", idBase+i)
		sqliteIDPlaceholders[i] = "?"
		args = append(args, entityID)
	}

	baseQueryTpl := `SELECT ID FROM "ENTITY" WHERE DEPLOYMENT_ID = %s AND OU_ID IN (%s) AND ID IN (%s)`
	postgresQuery := fmt.Sprintf(baseQueryTpl, "$1",
		strings.Join(postgresOUPlaceholders, ","), strings.Join(postgresIDPlaceholders, ","))
	sqliteQuery := fmt.Sprintf(baseQueryTpl, "?",
		strings.Join(sqliteOUPlaceholders, ","), strings.Join(sqliteIDPlaceholders, ","))

	query := model.DBQuery{
		ID:            "ASQ-ENTITY_MGT-24",
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}

	return query, args, nil
}

// buildEntityListQuery constructs a query to get entities with optional filtering.
func buildEntityListQuery(
	category string, filters map[string]interface{}, limit, offset int, deploymentID string,
) (model.DBQuery, []interface{}, error) {
	baseQuery := `SELECT ID, OU_ID, CATEGORY, TYPE, STATE, ATTRIBUTES, SYSTEM_ATTRIBUTES FROM "ENTITY"`
	queryID := "ASQ-ENTITY_MGT-25"

	if len(filters) > 0 {
		var baseWithCategory string
		var args []interface{}
		if category != "" {
			baseWithCategory = baseQuery + " WHERE CATEGORY = $1"
			args = []interface{}{category}
		} else {
			baseWithCategory = baseQuery + " WHERE 1=1"
			args = []interface{}{}
		}
		fq, fArgs, err := buildFilterQueryWithOffset(queryID, baseWithCategory, filters, len(args))
		if err != nil {
			return model.DBQuery{}, nil, err
		}
		args = append(args, fArgs...)
		fq, args = utils.AppendDeploymentIDToFilterQuery(fq, args, deploymentID)

		postgresQuery, err := buildPaginatedQuery(fq.PostgresQuery, len(args), "$")
		if err != nil {
			return model.DBQuery{}, nil, err
		}

		sqliteQuery, err := buildPaginatedQuery(fq.SQLiteQuery, len(args), "?")
		if err != nil {
			return model.DBQuery{}, nil, err
		}

		args = append(args, limit, offset)
		return model.DBQuery{
			ID:            queryID,
			Query:         postgresQuery,
			PostgresQuery: postgresQuery,
			SQLiteQuery:   sqliteQuery,
		}, args, nil
	}

	if category == "" {
		return QuerySearchEntityList, []interface{}{limit, offset, deploymentID}, nil
	}

	// No filters, use the pre-defined query
	return QueryGetEntityList, []interface{}{limit, offset, deploymentID, category}, nil
}

// buildEntityCountQuery constructs a query to count entities with optional filtering.
func buildEntityCountQuery(
	category string, filters map[string]interface{}, deploymentID string,
) (model.DBQuery, []interface{}, error) {
	baseQuery := `SELECT COUNT(*) as total FROM "ENTITY"`
	queryID := "ASQ-ENTITY_MGT-26"

	if len(filters) > 0 {
		baseWithCategory := baseQuery + " WHERE CATEGORY = $1"
		args := []interface{}{category}
		fq, fArgs, err := buildFilterQueryWithOffset(queryID, baseWithCategory, filters, len(args))
		if err != nil {
			return model.DBQuery{}, nil, err
		}
		args = append(args, fArgs...)
		fq, args = utils.AppendDeploymentIDToFilterQuery(fq, args, deploymentID)
		return fq, args, nil
	}

	return QueryGetEntityCount, []interface{}{category, deploymentID}, nil
}

// buildIdentifyQueryFromIdentifiers constructs a query to identify
// an entity using only indexed identifiers.
func buildIdentifyQueryFromIdentifiers(
	filters map[string]interface{}, deploymentID string,
) (model.DBQuery, []interface{}, error) {
	if len(filters) == 0 {
		return model.DBQuery{}, nil, fmt.Errorf("filters cannot be empty")
	}

	keys := make([]string, 0, len(filters))
	for key := range filters {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pgBase := `SELECT DISTINCT ia1.ENTITY_ID AS id FROM "ENTITY_IDENTIFIER" ia1`
	sqBase := pgBase
	pgConditions := []string{}
	sqConditions := []string{}
	args := []interface{}{}
	paramIndex := 1

	pgConditions = append(pgConditions, fmt.Sprintf("ia1.NAME = $%d AND ia1.VALUE = $%d",
		paramIndex, paramIndex+1))
	sqConditions = append(sqConditions, "ia1.NAME = ? AND ia1.VALUE = ?")
	args = append(args, keys[0], fmt.Sprintf("%v", filters[keys[0]]))
	paramIndex += 2

	for i := 1; i < len(keys); i++ {
		alias := fmt.Sprintf("ia%d", i+1)
		joinClause := fmt.Sprintf(
			` INNER JOIN "ENTITY_IDENTIFIER" %s ON ia1.ENTITY_ID = %s.ENTITY_ID `+
				`AND ia1.DEPLOYMENT_ID = %s.DEPLOYMENT_ID`,
			alias, alias, alias)
		pgBase += joinClause
		sqBase += joinClause
		pgConditions = append(pgConditions, fmt.Sprintf("%s.NAME = $%d AND %s.VALUE = $%d",
			alias, paramIndex, alias, paramIndex+1))
		sqConditions = append(sqConditions, fmt.Sprintf("%s.NAME = ? AND %s.VALUE = ?", alias, alias))
		args = append(args, keys[i], fmt.Sprintf("%v", filters[keys[i]]))
		paramIndex += 2
	}

	pgQueryString := pgBase + " WHERE " + strings.Join(pgConditions, " AND ") +
		fmt.Sprintf(" AND ia1.DEPLOYMENT_ID = $%d", paramIndex)
	sqQueryString := sqBase + " WHERE " + strings.Join(sqConditions, " AND ") +
		" AND ia1.DEPLOYMENT_ID = ?"
	args = append(args, deploymentID)

	return model.DBQuery{
		ID:            "ASQ-ENTITY_MGT-27",
		Query:         pgQueryString,
		PostgresQuery: pgQueryString,
		SQLiteQuery:   sqQueryString,
	}, args, nil
}

// buildIdentifyQueryHybrid constructs a query using indexed identifiers
// for initial filtering, then JSON attributes for remaining filters.
func buildIdentifyQueryHybrid(
	indexedFilters, nonIndexedFilters map[string]interface{},
	deploymentID string,
) (model.DBQuery, []interface{}, error) {
	if len(indexedFilters) == 0 {
		return model.DBQuery{}, nil, fmt.Errorf("indexed filters cannot be empty for hybrid query")
	}

	postgresQuery := `SELECT DISTINCT e.ID FROM "ENTITY" e`
	postgresQuery += ` INNER JOIN "ENTITY_IDENTIFIER" ia1 ON e.ID = ia1.ENTITY_ID ` +
		`AND e.DEPLOYMENT_ID = ia1.DEPLOYMENT_ID`

	sqliteQuery := `SELECT DISTINCT e.ID FROM "ENTITY" e`
	sqliteQuery += ` INNER JOIN "ENTITY_IDENTIFIER" ia1 ON e.ID = ia1.ENTITY_ID ` +
		`AND e.DEPLOYMENT_ID = ia1.DEPLOYMENT_ID`

	indexedKeys := make([]string, 0, len(indexedFilters))
	for key := range indexedFilters {
		indexedKeys = append(indexedKeys, key)
	}
	sort.Strings(indexedKeys)

	whereConditions := []string{}
	args := []interface{}{}
	paramIndex := 1

	whereConditions = append(whereConditions, fmt.Sprintf("ia1.NAME = $%d AND ia1.VALUE = $%d",
		paramIndex, paramIndex+1))
	args = append(args, indexedKeys[0], fmt.Sprintf("%v", indexedFilters[indexedKeys[0]]))
	paramIndex += 2

	for i := 1; i < len(indexedKeys); i++ {
		alias := fmt.Sprintf("ia%d", i+1)
		joinClause := fmt.Sprintf(
			` INNER JOIN "ENTITY_IDENTIFIER" %s ON e.ID = %s.ENTITY_ID `+
				`AND e.DEPLOYMENT_ID = %s.DEPLOYMENT_ID`,
			alias, alias, alias)
		postgresQuery += joinClause
		sqliteQuery += joinClause
		whereConditions = append(whereConditions, fmt.Sprintf("%s.NAME = $%d AND %s.VALUE = $%d",
			alias, paramIndex, alias, paramIndex+1))
		args = append(args, indexedKeys[i], fmt.Sprintf("%v", indexedFilters[indexedKeys[i]]))
		paramIndex += 2
	}

	postgresQuery += " WHERE " + strings.Join(whereConditions, " AND ")
	sqliteQuery += " WHERE " + strings.Join(whereConditions, " AND ")

	nonIndexedKeys := make([]string, 0, len(nonIndexedFilters))
	for key := range nonIndexedFilters {
		if err := utils.ValidateKey(key); err != nil {
			return model.DBQuery{}, nil, fmt.Errorf("invalid non-indexed filter key: %w", err)
		}
		nonIndexedKeys = append(nonIndexedKeys, key)
	}
	sort.Strings(nonIndexedKeys)

	for _, key := range nonIndexedKeys {
		pg, sq := buildDualColumnConditions("e.", key, paramIndex)
		postgresQuery += pg
		sqliteQuery += sq
		args = append(args, nonIndexedFilters[key])
		paramIndex++
	}

	postgresQuery += fmt.Sprintf(" AND e.DEPLOYMENT_ID = $%d", paramIndex)
	sqliteQuery += " AND e.DEPLOYMENT_ID = ?"
	args = append(args, deploymentID)

	query := model.DBQuery{
		ID:            "ASQ-ENTITY_MGT-28",
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}

	return query, args, nil
}

// buildGetEntitiesByIDsQuery constructs a query to fetch entities by a list of IDs.
func buildGetEntitiesByIDsQuery(entityIDs []string, deploymentID string) (model.DBQuery, []interface{}, error) {
	return buildEntityINClauseQuery(
		"ASQ-ENTITY_MGT-29",
		`SELECT ID, OU_ID, CATEGORY, TYPE, STATE, ATTRIBUTES, SYSTEM_ATTRIBUTES `+
			`FROM "ENTITY" WHERE ID IN (%s) AND DEPLOYMENT_ID = %s`,
		entityIDs, deploymentID,
	)
}

// buildDualColumnConditions returns AND conditions for both Postgres and SQLite that match a key
// against both ATTRIBUTES and SYSTEM_ATTRIBUTES using COALESCE (one parameter per key).
func buildDualColumnConditions(tablePrefix, key string, paramIndex int) (pgCond, sqCond string) {
	attrCol := tablePrefix + AttributesColumn
	sysCol := tablePrefix + SystemAttributesColumn
	sqCond = fmt.Sprintf(" AND COALESCE(json_extract(%s, '$.%s'), json_extract(%s, '$.%s')) = ?",
		sysCol, key, attrCol, key)
	if strings.Contains(key, ".") {
		parts := strings.Split(key, ".")
		pathArray := "{" + strings.Join(parts, ",") + "}"
		pgCond = fmt.Sprintf(" AND COALESCE(%s#>>'%s', %s#>>'%s') = $%d",
			sysCol, pathArray, attrCol, pathArray, paramIndex)
		return
	}
	pgCond = fmt.Sprintf(" AND COALESCE(%s->>'%s', %s->>'%s') = $%d",
		sysCol, key, attrCol, key, paramIndex)
	return
}

// buildPaginatedQuery constructs a paginated query string with ORDER BY, LIMIT, and OFFSET clauses.
func buildPaginatedQuery(baseQuery string, paramCount int, placeholder string) (string, error) {
	switch placeholder {
	case "?":
		return fmt.Sprintf("%s ORDER BY ID LIMIT %s OFFSET %s",
			baseQuery, placeholder, placeholder), nil
	case "$":
		limitPlaceholder := fmt.Sprintf("%s%d", placeholder, paramCount+1)
		offsetPlaceholder := fmt.Sprintf("%s%d", placeholder, paramCount+2)
		return fmt.Sprintf("%s ORDER BY ID LIMIT %s OFFSET %s",
			baseQuery, limitPlaceholder, offsetPlaceholder), nil
	}
	return "", fmt.Errorf("unsupported placeholder: %s", placeholder)
}

// buildFilterQueryWithOffset constructs a filter query where parameter numbering starts at the given offset.
// This is used when the base query already has parameters (e.g., CATEGORY = $1).
func buildFilterQueryWithOffset(
	queryID string, baseQuery string, filters map[string]interface{}, paramOffset int,
) (model.DBQuery, []interface{}, error) {
	columnName := AttributesColumn

	args := make([]interface{}, 0, len(filters))

	keys := make([]string, 0, len(filters))
	for key := range filters {
		if err := utils.ValidateKey(key); err != nil {
			return model.DBQuery{}, nil, fmt.Errorf("invalid filter key: %w", err)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	postgresQuery := baseQuery
	sqliteQuery := strings.Replace(baseQuery, "$1", "?", 1)

	for i, key := range keys {
		postgresQuery += utils.BuildPostgresJSONCondition(columnName, key, paramOffset+i+1)
		sqliteQuery += utils.BuildSQLiteJSONCondition(columnName, key)
		args = append(args, filters[key])
	}

	resultQuery := model.DBQuery{
		ID:            queryID,
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}

	return resultQuery, args, nil
}
