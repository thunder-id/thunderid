// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"fmt"
	"strings"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// queryGetEntityTypeCount retrieves the total count of entity types for a category.
	queryGetEntityTypeCount = dbmodel.DBQuery{
		ID:    "ASQ-ENTITY_TYPE-001",
		Query: `SELECT COUNT(*) AS total FROM "ENTITY_TYPES" WHERE CATEGORY = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetEntityTypeList retrieves a paginated list of entity types for a category.
	queryGetEntityTypeList = dbmodel.DBQuery{
		ID: "ASQ-ENTITY_TYPE-002",
		Query: `SELECT ID, CATEGORY, HANDLE, DISPLAY_NAME, OU_ID, ALLOW_SELF_REGISTRATION, SYSTEM_ATTRIBUTES ` +
			`FROM "ENTITY_TYPES" WHERE CATEGORY = $4 AND DEPLOYMENT_ID = $3 ` +
			`ORDER BY DISPLAY_NAME, HANDLE LIMIT $1 OFFSET $2`,
	}

	// queryCreateEntityType creates a new entity type.
	queryCreateEntityType = dbmodel.DBQuery{
		ID: "ASQ-ENTITY_TYPE-003",
		Query: `INSERT INTO "ENTITY_TYPES" ` +
			`(ID, CATEGORY, HANDLE, DISPLAY_NAME, OU_ID, ALLOW_SELF_REGISTRATION, SCHEMA_DEF, SYSTEM_ATTRIBUTES, ` +
			`DEPLOYMENT_ID) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
	}

	// queryGetEntityTypeByID retrieves an entity type by its ID within a category.
	queryGetEntityTypeByID = dbmodel.DBQuery{
		ID: "ASQ-ENTITY_TYPE-004",
		Query: `SELECT ID, CATEGORY, HANDLE, DISPLAY_NAME, OU_ID, ALLOW_SELF_REGISTRATION, SCHEMA_DEF, ` +
			`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" WHERE ID = $1 AND CATEGORY = $3 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetEntityTypeByHandle retrieves an entity type by its handle within a category.
	queryGetEntityTypeByHandle = dbmodel.DBQuery{
		ID: "ASQ-ENTITY_TYPE-005",
		Query: `SELECT ID, CATEGORY, HANDLE, DISPLAY_NAME, OU_ID, ALLOW_SELF_REGISTRATION, SCHEMA_DEF, ` +
			`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" WHERE HANDLE = $1 AND CATEGORY = $3 AND DEPLOYMENT_ID = $2`,
	}

	// queryUpdateEntityTypeByID updates an entity type by its ID within a category.
	queryUpdateEntityTypeByID = dbmodel.DBQuery{
		ID: "ASQ-ENTITY_TYPE-006",
		Query: `UPDATE "ENTITY_TYPES"
			SET DISPLAY_NAME = $1, OU_ID = $2, ALLOW_SELF_REGISTRATION = $3, SCHEMA_DEF = $4, SYSTEM_ATTRIBUTES = $5
			WHERE ID = $6 AND CATEGORY = $8 AND DEPLOYMENT_ID = $7`,
	}

	// queryDeleteEntityTypeByID deletes an entity type by its ID within a category.
	queryDeleteEntityTypeByID = dbmodel.DBQuery{
		ID:    "ASQ-ENTITY_TYPE-007",
		Query: `DELETE FROM "ENTITY_TYPES" WHERE ID = $1 AND CATEGORY = $3 AND DEPLOYMENT_ID = $2`,
	}
)

// buildGetEntityTypeListByOUIDsQuery dynamically builds a query to retrieve entity types
// filtered by a list of OU IDs and category with pagination.
func buildGetEntityTypeListByOUIDsQuery(ouIDs []string) dbmodel.DBQuery {
	n := len(ouIDs)

	if n == 0 {
		return dbmodel.DBQuery{
			ID: "ASQ-ENTITY_TYPE-008",
			PostgresQuery: `SELECT ID, CATEGORY, HANDLE, DISPLAY_NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
				`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE 1=0 AND CATEGORY = $1 AND DEPLOYMENT_ID = $2 ORDER BY DISPLAY_NAME, HANDLE LIMIT $3 OFFSET $4`,
			SQLiteQuery: `SELECT ID, CATEGORY, HANDLE, DISPLAY_NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
				`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE 1=0 AND CATEGORY = ? AND DEPLOYMENT_ID = ? ORDER BY DISPLAY_NAME, HANDLE LIMIT ? OFFSET ?`,
		}
	}

	pgPlaceholders := make([]string, n)
	for i := range ouIDs {
		pgPlaceholders[i] = fmt.Sprintf("$%d", i+1)
	}
	pgInClause := strings.Join(pgPlaceholders, ", ")
	pgCategory := fmt.Sprintf("$%d", n+1)
	pgDeploymentID := fmt.Sprintf("$%d", n+2)
	pgLimit := fmt.Sprintf("$%d", n+3)
	pgOffset := fmt.Sprintf("$%d", n+4)

	sqlitePlaceholders := make([]string, n)
	for i := range ouIDs {
		sqlitePlaceholders[i] = "?"
	}
	sqliteInClause := strings.Join(sqlitePlaceholders, ", ")

	return dbmodel.DBQuery{
		ID: "ASQ-ENTITY_TYPE-008",
		PostgresQuery: `SELECT ID, CATEGORY, HANDLE, DISPLAY_NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
			`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
			`WHERE OU_ID IN (` + pgInClause + `) AND CATEGORY = ` + pgCategory +
			` AND DEPLOYMENT_ID = ` + pgDeploymentID +
			` ORDER BY DISPLAY_NAME, HANDLE LIMIT ` + pgLimit + ` OFFSET ` + pgOffset,
		SQLiteQuery: `SELECT ID, CATEGORY, HANDLE, DISPLAY_NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
			`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
			`WHERE OU_ID IN (` + sqliteInClause + `) AND CATEGORY = ? AND DEPLOYMENT_ID = ? ` +
			`ORDER BY DISPLAY_NAME, HANDLE LIMIT ? OFFSET ?`,
	}
}

// buildGetEntityTypeCountByOUIDsQuery dynamically builds a query to count entity types
// filtered by a list of OU IDs and category.
func buildGetEntityTypeCountByOUIDsQuery(ouIDs []string) dbmodel.DBQuery {
	n := len(ouIDs)

	if n == 0 {
		return dbmodel.DBQuery{
			ID: "ASQ-ENTITY_TYPE-009",
			PostgresQuery: `SELECT COUNT(*) AS total FROM "ENTITY_TYPES" ` +
				`WHERE 1=0 AND CATEGORY = $1 AND DEPLOYMENT_ID = $2`,
			SQLiteQuery: `SELECT COUNT(*) AS total FROM "ENTITY_TYPES" ` +
				`WHERE 1=0 AND CATEGORY = ? AND DEPLOYMENT_ID = ?`,
		}
	}

	pgPlaceholders := make([]string, n)
	for i := range ouIDs {
		pgPlaceholders[i] = fmt.Sprintf("$%d", i+1)
	}
	pgInClause := strings.Join(pgPlaceholders, ", ")
	pgCategory := fmt.Sprintf("$%d", n+1)
	pgDeploymentID := fmt.Sprintf("$%d", n+2)

	sqlitePlaceholders := make([]string, n)
	for i := range ouIDs {
		sqlitePlaceholders[i] = "?"
	}
	sqliteInClause := strings.Join(sqlitePlaceholders, ", ")

	return dbmodel.DBQuery{
		ID: "ASQ-ENTITY_TYPE-009",
		PostgresQuery: `SELECT COUNT(*) AS total FROM "ENTITY_TYPES" ` +
			`WHERE OU_ID IN (` + pgInClause + `) AND CATEGORY = ` + pgCategory +
			` AND DEPLOYMENT_ID = ` + pgDeploymentID,
		SQLiteQuery: `SELECT COUNT(*) AS total FROM "ENTITY_TYPES" ` +
			`WHERE OU_ID IN (` + sqliteInClause + `) AND CATEGORY = ? AND DEPLOYMENT_ID = ?`,
	}
}

// buildGetDisplayAttributesByHandlesQuery dynamically builds a query to retrieve display attributes
// for a list of entity type handles within a category.
func buildGetDisplayAttributesByHandlesQuery(handles []string) dbmodel.DBQuery {
	n := len(handles)

	if n == 0 {
		return dbmodel.DBQuery{
			ID: "ASQ-ENTITY_TYPE-010",
			PostgresQuery: `SELECT HANDLE, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE 1=0 AND CATEGORY = $1 AND DEPLOYMENT_ID = $2`,
			SQLiteQuery: `SELECT HANDLE, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE 1=0 AND CATEGORY = ? AND DEPLOYMENT_ID = ?`,
		}
	}

	pgPlaceholders := make([]string, n)
	for i := range handles {
		pgPlaceholders[i] = fmt.Sprintf("$%d", i+1)
	}
	pgInClause := strings.Join(pgPlaceholders, ", ")
	pgCategory := fmt.Sprintf("$%d", n+1)
	pgDeploymentID := fmt.Sprintf("$%d", n+2)

	sqlitePlaceholders := make([]string, n)
	for i := range handles {
		sqlitePlaceholders[i] = "?"
	}
	sqliteInClause := strings.Join(sqlitePlaceholders, ", ")

	return dbmodel.DBQuery{
		ID: "ASQ-ENTITY_TYPE-010",
		PostgresQuery: `SELECT HANDLE, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
			`WHERE HANDLE IN (` + pgInClause + `) AND CATEGORY = ` + pgCategory +
			` AND DEPLOYMENT_ID = ` + pgDeploymentID,
		SQLiteQuery: `SELECT HANDLE, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
			`WHERE HANDLE IN (` + sqliteInClause + `) AND CATEGORY = ? AND DEPLOYMENT_ID = ?`,
	}
}
