// Package utils provides utility functions for database operations.
package utils

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/database/model"
)

// BuildFilterQuery constructs a query to filter records based on the provided filters.
func BuildFilterQuery(
	queryID string,
	baseQuery string,
	columnName string,
	filters map[string]interface{},
) (model.DBQuery, []interface{}, error) {
	// Validate the column name.
	if err := ValidateKey(columnName); err != nil {
		return model.DBQuery{}, nil, fmt.Errorf("invalid column name: %w", err)
	}

	args := make([]interface{}, 0, len(filters))

	keys := make([]string, 0, len(filters))
	for key := range filters {
		if err := ValidateKey(key); err != nil {
			return model.DBQuery{}, nil, fmt.Errorf("invalid filter key: %w", err)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	postgresQuery := baseQuery
	sqliteQuery := baseQuery
	for i, key := range keys {
		postgresQuery += BuildPostgresJSONCondition(columnName, key, i+1)
		sqliteQuery += BuildSQLiteJSONCondition(columnName, key)
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

// AppendDeploymentIDToFilterQuery appends a DEPLOYMENT_ID condition to the given filter query.
func AppendDeploymentIDToFilterQuery(
	query model.DBQuery, args []interface{}, deploymentID string,
) (model.DBQuery, []interface{}) {
	postgresQuery := fmt.Sprintf("%s AND DEPLOYMENT_ID = $%d", query.PostgresQuery, len(args)+1)
	sqliteQuery := fmt.Sprintf("%s AND DEPLOYMENT_ID = ?", query.SQLiteQuery)

	argsWithDeploymentID := make([]interface{}, 0, len(args)+1)
	argsWithDeploymentID = append(argsWithDeploymentID, args...)
	argsWithDeploymentID = append(argsWithDeploymentID, deploymentID)

	updatedQuery := &model.DBQuery{
		ID:            query.ID,
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}

	return *updatedQuery, argsWithDeploymentID
}

// BuildPostgresJSONCondition builds a PostgreSQL JSON filter condition.
// For nested paths (e.g., "address.city"), it uses the #>> operator with an array path.
// For simple paths (e.g., "email"), it uses the ->> operator.
func BuildPostgresJSONCondition(columnName, key string, paramIndex int) string {
	if strings.Contains(key, ".") {
		// Handle nested JSON path
		keys := strings.Split(key, ".")
		pathArray := "{" + strings.Join(keys, ",") + "}"
		return fmt.Sprintf(" AND %s#>>'%s' = $%d", columnName, pathArray, paramIndex)
	}
	// Handle simple JSON path
	return fmt.Sprintf(" AND %s->>'%s' = $%d", columnName, key, paramIndex)
}

// BuildSQLiteJSONCondition builds a SQLite JSON filter condition.
// For both nested and simple paths, it uses json_extract with dot notation.
func BuildSQLiteJSONCondition(columnName, key string) string {
	return fmt.Sprintf(" AND json_extract(%s, '$.%s') = ?", columnName, key)
}

// ValidateKey ensures that the provided key contains only safe characters (alphanumeric, underscores, and dots).
// This validation prevents SQL injection by ensuring keys can be safely used in queries.
func ValidateKey(key string) error {
	for _, char := range key {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || char == '_' || char == '.') {
			return fmt.Errorf("key '%s' contains invalid characters", key)
		}
	}
	return nil
}
