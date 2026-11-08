// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"fmt"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

// ouFilterableColumns maps API attribute names to ORGANIZATION_UNIT table column names.
var ouFilterableColumns = map[string]string{
	"name":        "NAME",
	"handle":      "HANDLE",
	"description": "DESCRIPTION",
	"createdAt":   "CREATED_AT",
	"updatedAt":   "UPDATED_AT",
}

// ouTextColumns is the set of ORGANIZATION_UNIT columns that hold free-form text.
// The eq operator on these columns uses LOWER() for case-insensitive matching,
// keeping the DB store consistent with the in-memory file-based store (strings.EqualFold).
var ouTextColumns = map[string]bool{
	"NAME":        true,
	"HANDLE":      true,
	"DESCRIPTION": true,
}

// likePrefix turns a filter operand into a LIKE pattern matching anything that starts with it.
//
// The operand is escaped first: % and _ are wildcards to LIKE, so a name containing either would
// otherwise match more than the caller asked for. The backslash escape is declared by the ESCAPE
// clause on the comparison, and has to be escaped first or it would escape the escapes.
func likePrefix(value interface{}) interface{} {
	text, ok := value.(string)
	if !ok {
		return value
	}
	replacer := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)
	return replacer.Replace(text) + "%"
}

// buildOUFilterGroup generates a SQL WHERE fragment for a FilterGroup and returns the bound args.
// startParamIdx is the positional parameter index for the first filter value.
// Returns an empty string and no args when g is nil.
// For multi-clause groups the fragment is wrapped in AND (...); single-clause groups omit the parens.
func buildOUFilterGroup(g *tidcommon.FilterGroup, startParamIdx int) (cond string, args []interface{}, err error) {
	if g == nil || len(g.Clauses) == 0 {
		return "", nil, nil
	}

	var sb strings.Builder
	idx := startParamIdx

	for i, clause := range g.Clauses {
		col, ok := ouFilterableColumns[clause.Expr.Attribute]
		if !ok {
			return "", nil, fmt.Errorf("attribute %q is not filterable", clause.Expr.Attribute)
		}

		var clauseCond string
		arg := clause.Expr.Value
		switch clause.Expr.Operator {
		case tidcommon.OperatorEq:
			if ouTextColumns[col] {
				clauseCond = fmt.Sprintf("LOWER(%s) = LOWER($%d)", col, idx)
			} else {
				clauseCond = fmt.Sprintf("%s = $%d", col, idx)
			}
		case tidcommon.OperatorGt:
			clauseCond = fmt.Sprintf("%s > $%d", col, idx)
		case tidcommon.OperatorLt:
			clauseCond = fmt.Sprintf("%s < $%d", col, idx)
		case tidcommon.OperatorSw:
			clauseCond = fmt.Sprintf("LOWER(%s) LIKE LOWER($%d) ESCAPE '\\'", col, idx)
			arg = likePrefix(clause.Expr.Value)
		default:
			return "", nil, fmt.Errorf("unsupported operator %q", clause.Expr.Operator)
		}

		if i > 0 {
			sb.WriteString(" ")
			sb.WriteString(string(clause.Connector))
			sb.WriteString(" ")
		}
		sb.WriteString(clauseCond)
		args = append(args, arg)
		idx++
	}

	if len(g.Clauses) == 1 {
		cond = " AND " + sb.String()
	} else {
		cond = " AND (" + sb.String() + ")"
	}
	return cond, args, nil
}

// buildRootOUCountQuery constructs a count query for root-level OUs with an optional filter group.
// Args order: deploymentID=$1 [, filterArgs...]
func buildRootOUCountQuery(g *tidcommon.FilterGroup) (dbmodel.DBQuery, []interface{}, error) {
	query := `SELECT COUNT(*) as total FROM "ORGANIZATION_UNIT" WHERE PARENT_ID IS NULL AND DEPLOYMENT_ID = $1`

	filterArgs := []interface{}{}
	if g != nil {
		cond, args, err := buildOUFilterGroup(g, 2)
		if err != nil {
			return dbmodel.DBQuery{}, nil, err
		}
		query += cond
		filterArgs = append(filterArgs, args...)
	}

	return dbmodel.DBQuery{ID: "OUQ-OU_MGT-01", Query: query}, filterArgs, nil
}

// buildRootOUListQuery constructs the paginated root-OU list query with an optional filter group.
// Args order: limit=$1, offset=$2, deploymentID=$3 [, filterArgs...]
func buildRootOUListQuery(g *tidcommon.FilterGroup) (dbmodel.DBQuery, []interface{}, error) {
	query := `SELECT OU_ID, PROJECT_ID, HANDLE, NAME, DESCRIPTION, PARENT_ID, METADATA, CREATED_AT, UPDATED_AT ` +
		`FROM "ORGANIZATION_UNIT" ` +
		`WHERE PARENT_ID IS NULL AND DEPLOYMENT_ID = $3`

	filterArgs := []interface{}{}
	if g != nil {
		cond, args, err := buildOUFilterGroup(g, 4)
		if err != nil {
			return dbmodel.DBQuery{}, nil, err
		}
		query += cond
		filterArgs = append(filterArgs, args...)
	}

	// OU_ID breaks a tie between siblings sharing a name, so a listing, and an export built from one,
	// comes back in the same order every time.
	query += " ORDER BY NAME, OU_ID LIMIT $1 OFFSET $2"
	return dbmodel.DBQuery{ID: "OUQ-OU_MGT-02", Query: query}, filterArgs, nil
}

// projectTopOUCondition keeps the organization units of a project whose parent is not in the project: its
// root organization units, and any unit whose parent belongs to no project or to another one. Every other
// unit of the project is listed under its parent.
const projectTopOUCondition = `NOT EXISTS (SELECT 1 FROM "ORGANIZATION_UNIT" P ` +
	`WHERE P.OU_ID = OU.PARENT_ID AND P.DEPLOYMENT_ID = OU.DEPLOYMENT_ID AND P.PROJECT_ID = OU.PROJECT_ID)`

// buildProjectOUCountQuery constructs a count query for the top organization units of a project with an
// optional filter group.
// Args order: projectID=$1 [, filterArgs...], deploymentID=$N
func buildProjectOUCountQuery(g *tidcommon.FilterGroup) (dbmodel.DBQuery, []any, error) {
	query := `SELECT COUNT(*) as total FROM "ORGANIZATION_UNIT" OU ` +
		`WHERE OU.PROJECT_ID = $1 AND ` + projectTopOUCondition

	filterArgs := []any{}
	if g != nil {
		cond, args, err := buildOUFilterGroup(g, 2)
		if err != nil {
			return dbmodel.DBQuery{}, nil, err
		}
		query += cond
		filterArgs = append(filterArgs, args...)
	}
	query += fmt.Sprintf(" AND OU.DEPLOYMENT_ID = $%d", 2+len(filterArgs))

	return dbmodel.DBQuery{ID: "OUQ-OU_MGT-22", Query: query}, filterArgs, nil
}

// buildProjectOUListQuery constructs the paginated list query for the top organization units of a project
// with an optional filter group.
// Args order: limit=$1, offset=$2, projectID=$3 [, filterArgs...], deploymentID=$N
func buildProjectOUListQuery(g *tidcommon.FilterGroup) (dbmodel.DBQuery, []any, error) {
	query := `SELECT OU.OU_ID, OU.PROJECT_ID, OU.HANDLE, OU.NAME, OU.DESCRIPTION, OU.PARENT_ID, OU.METADATA, ` +
		`OU.CREATED_AT, OU.UPDATED_AT FROM "ORGANIZATION_UNIT" OU ` +
		`WHERE OU.PROJECT_ID = $3 AND ` + projectTopOUCondition

	filterArgs := []any{}
	if g != nil {
		cond, args, err := buildOUFilterGroup(g, 4)
		if err != nil {
			return dbmodel.DBQuery{}, nil, err
		}
		query += cond
		filterArgs = append(filterArgs, args...)
	}
	query += fmt.Sprintf(" AND OU.DEPLOYMENT_ID = $%d", 4+len(filterArgs))

	// OU_ID breaks a tie between units sharing a name, as the root listing does.
	query += " ORDER BY OU.NAME, OU.OU_ID LIMIT $1 OFFSET $2"
	return dbmodel.DBQuery{ID: "OUQ-OU_MGT-23", Query: query}, filterArgs, nil
}

// buildChildrenOUCountQuery constructs a count query for child OUs under a parent with an optional filter group.
// Args order: parentID=$1, deploymentID=$2 [, filterArgs...]
func buildChildrenOUCountQuery(g *tidcommon.FilterGroup) (dbmodel.DBQuery, []interface{}, error) {
	query := `SELECT COUNT(*) as total FROM "ORGANIZATION_UNIT" WHERE PARENT_ID = $1 AND DEPLOYMENT_ID = $2`

	filterArgs := []interface{}{}
	if g != nil {
		cond, args, err := buildOUFilterGroup(g, 3)
		if err != nil {
			return dbmodel.DBQuery{}, nil, err
		}
		query += cond
		filterArgs = append(filterArgs, args...)
	}

	return dbmodel.DBQuery{ID: "OUQ-OU_MGT-10", Query: query}, filterArgs, nil
}

// buildChildrenOUListQuery constructs the paginated child-OU list query with an optional filter group.
// Args order: parentID=$1, limit=$2, offset=$3, deploymentID=$4 [, filterArgs...]
func buildChildrenOUListQuery(g *tidcommon.FilterGroup) (dbmodel.DBQuery, []interface{}, error) {
	query := `SELECT OU_ID, PROJECT_ID, HANDLE, NAME, DESCRIPTION, METADATA, CREATED_AT, UPDATED_AT ` +
		`FROM "ORGANIZATION_UNIT" WHERE PARENT_ID = $1 AND DEPLOYMENT_ID = $4`

	filterArgs := []interface{}{}
	if g != nil {
		cond, args, err := buildOUFilterGroup(g, 5)
		if err != nil {
			return dbmodel.DBQuery{}, nil, err
		}
		query += cond
		filterArgs = append(filterArgs, args...)
	}

	// OU_ID breaks a tie between siblings sharing a name, as the root listing does.
	query += " ORDER BY NAME, OU_ID LIMIT $2 OFFSET $3"
	return dbmodel.DBQuery{ID: "OUQ-OU_MGT-11", Query: query}, filterArgs, nil
}

var (
	// queryCreateOrganizationUnit is the query to create a new organization unit.
	queryCreateOrganizationUnit = dbmodel.DBQuery{
		ID: "OUQ-OU_MGT-03",
		Query: `INSERT INTO "ORGANIZATION_UNIT" (
			OU_ID, PARENT_ID, HANDLE, NAME, DESCRIPTION, METADATA, CREATED_AT, UPDATED_AT, PROJECT_ID,
			DEPLOYMENT_ID
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)`,
	}

	// queryGetOrganizationUnitByID is the query to get an organization unit by id.
	queryGetOrganizationUnitByID = dbmodel.DBQuery{
		ID: "OUQ-OU_MGT-04",
		Query: `SELECT OU_ID, PARENT_ID, PROJECT_ID, HANDLE, NAME, DESCRIPTION, METADATA, CREATED_AT, UPDATED_AT
		FROM "ORGANIZATION_UNIT"
		WHERE OU_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetRootOrganizationUnitByHandle is the query to get a root organization unit by handle.
	queryGetRootOrganizationUnitByHandle = dbmodel.DBQuery{
		ID: "OUQ-OU_MGT-05",
		Query: `SELECT OU_ID, PARENT_ID, PROJECT_ID, HANDLE, NAME, DESCRIPTION, METADATA, CREATED_AT, UPDATED_AT
		FROM "ORGANIZATION_UNIT"
		WHERE HANDLE = $1 AND PARENT_ID IS NULL AND DEPLOYMENT_ID = $2`,
	}

	// queryGetOrganizationUnitByHandle is the query to get an organization unit by handle and parent.
	queryGetOrganizationUnitByHandle = dbmodel.DBQuery{
		ID: "OUQ-OU_MGT-06",
		Query: `SELECT OU_ID, PARENT_ID, PROJECT_ID, HANDLE, NAME, DESCRIPTION, METADATA, CREATED_AT, UPDATED_AT
		FROM "ORGANIZATION_UNIT"
		WHERE HANDLE = $1 AND PARENT_ID = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryCheckOrganizationUnitExists is the query to check if an organization unit exists.
	queryCheckOrganizationUnitExists = dbmodel.DBQuery{
		ID:    "OUQ-OU_MGT-07",
		Query: `SELECT COUNT(*) as count FROM "ORGANIZATION_UNIT" WHERE OU_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryUpdateOrganizationUnit is the query to update an organization unit.
	queryUpdateOrganizationUnit = dbmodel.DBQuery{
		ID: "OUQ-OU_MGT-08",
		Query: `UPDATE "ORGANIZATION_UNIT" SET PARENT_ID = $2, HANDLE = $3, NAME = $4, DESCRIPTION = $5, ` +
			`METADATA = $6, UPDATED_AT = $7, PROJECT_ID = $8 ` +
			`WHERE OU_ID = $1 AND DEPLOYMENT_ID = $9`,
	}

	// queryDeleteOrganizationUnit is the query to delete an organization unit.
	queryDeleteOrganizationUnit = dbmodel.DBQuery{
		ID:    "OUQ-OU_MGT-09",
		Query: `DELETE FROM "ORGANIZATION_UNIT" WHERE OU_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCheckOrganizationUnitNameConflict is the query to check if an organization
	// unit name conflicts under the same parent.
	queryCheckOrganizationUnitNameConflict = dbmodel.DBQuery{
		ID: "OUQ-OU_MGT-16",
		Query: `SELECT COUNT(*) as count FROM "ORGANIZATION_UNIT" ` +
			`WHERE NAME = $1 AND PARENT_ID = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryCheckOrganizationUnitNameConflictRoot is the query to check if an organization
	// unit name conflicts at root level.
	queryCheckOrganizationUnitNameConflictRoot = dbmodel.DBQuery{
		ID: "OUQ-OU_MGT-17",
		Query: `SELECT COUNT(*) as count FROM "ORGANIZATION_UNIT" ` +
			`WHERE NAME = $1 AND PARENT_ID IS NULL AND DEPLOYMENT_ID = $2`,
	}

	// queryCheckOrganizationUnitHandleConflict is the query to check if an organization
	// unit handle conflicts under the same parent.
	queryCheckOrganizationUnitHandleConflict = dbmodel.DBQuery{
		ID: "OUQ-OU_MGT-18",
		Query: `SELECT COUNT(*) as count FROM "ORGANIZATION_UNIT" ` +
			`WHERE HANDLE = $1 AND PARENT_ID = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryCheckOrganizationUnitHandleConflictRoot is the query to check if an organization
	// unit handle conflicts at root level.
	queryCheckOrganizationUnitHandleConflictRoot = dbmodel.DBQuery{
		ID: "OUQ-OU_MGT-19",
		Query: `SELECT COUNT(*) as count FROM "ORGANIZATION_UNIT" ` +
			`WHERE HANDLE = $1 AND PARENT_ID IS NULL AND DEPLOYMENT_ID = $2`,
	}
)

// buildGetOrganizationUnitsByIDsQuery dynamically builds a query to retrieve organization units by a list of IDs.
// For PostgreSQL: WHERE OU_ID IN ($1, $2, ...) AND DEPLOYMENT_ID = $N
// For SQLite: WHERE OU_ID IN (?, ?, ...) AND DEPLOYMENT_ID = ?
func buildGetOrganizationUnitsByIDsQuery(ids []string) dbmodel.DBQuery {
	n := len(ids)

	// Build PostgreSQL placeholders: $1, $2, ..., $N
	pgPlaceholders := make([]string, n)
	for i := range ids {
		pgPlaceholders[i] = fmt.Sprintf("$%d", i+1)
	}
	pgInClause := strings.Join(pgPlaceholders, ", ")
	deploymentIDParam := fmt.Sprintf("$%d", n+1)

	// Build SQLite placeholders: ?, ?, ...
	sqlitePlaceholders := make([]string, n)
	for i := range ids {
		sqlitePlaceholders[i] = "?"
	}
	sqliteInClause := strings.Join(sqlitePlaceholders, ", ")

	return dbmodel.DBQuery{
		ID: "OUQ-OU_MGT-21",
		PostgresQuery: `SELECT OU_ID, PROJECT_ID, HANDLE, NAME, DESCRIPTION, METADATA, CREATED_AT, UPDATED_AT ` +
			`FROM "ORGANIZATION_UNIT" ` +
			`WHERE OU_ID IN (` + pgInClause + `) AND DEPLOYMENT_ID = ` + deploymentIDParam + ` ORDER BY NAME`,
		SQLiteQuery: `SELECT OU_ID, PROJECT_ID, HANDLE, NAME, DESCRIPTION, METADATA, CREATED_AT, UPDATED_AT ` +
			`FROM "ORGANIZATION_UNIT" ` +
			`WHERE OU_ID IN (` + sqliteInClause + `) AND DEPLOYMENT_ID = ? ORDER BY NAME`,
	}
}

// buildUpdateOrganizationUnitsProjectQuery builds the query that moves a set of organization units to a project.
// Args order: projectID=$1, updatedAt=$2, ids=$3..$N+2, deploymentID=$N+3
func buildUpdateOrganizationUnitsProjectQuery(ids []string) dbmodel.DBQuery {
	pgPlaceholders := make([]string, len(ids))
	sqlitePlaceholders := make([]string, len(ids))
	for i := range ids {
		pgPlaceholders[i] = fmt.Sprintf("$%d", i+3)
		sqlitePlaceholders[i] = "?"
	}

	return dbmodel.DBQuery{
		ID: "OUQ-OU_MGT-24",
		PostgresQuery: `UPDATE "ORGANIZATION_UNIT" SET PROJECT_ID = $1, UPDATED_AT = $2 ` +
			`WHERE OU_ID IN (` + strings.Join(pgPlaceholders, ", ") + `) ` +
			fmt.Sprintf("AND DEPLOYMENT_ID = $%d", len(ids)+3),
		SQLiteQuery: `UPDATE "ORGANIZATION_UNIT" SET PROJECT_ID = ?, UPDATED_AT = ? ` +
			`WHERE OU_ID IN (` + strings.Join(sqlitePlaceholders, ", ") + `) AND DEPLOYMENT_ID = ?`,
	}
}
