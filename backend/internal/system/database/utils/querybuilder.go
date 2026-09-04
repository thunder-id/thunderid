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
		keys = append(keys, key)
	}
	sort.Strings(keys)

	postgresQuery := baseQuery
	sqliteQuery := baseQuery
	paramIndex := 1
	for _, key := range keys {
		postgresQuery += BuildPostgresJSONCondition(columnName, key, paramIndex)
		sqliteQuery += BuildSQLiteJSONCondition(columnName, key)
		keyArgs := JSONConditionArgs(key, filters[key])
		args = append(args, keyArgs...)
		paramIndex += len(keyArgs)
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

// SplitJSONKey splits a filter key into its JSON path segments. A dot separates nested
// levels, so "address.city" addresses the "city" member of the "address" object.
func SplitJSONKey(key string) []string {
	return strings.Split(key, ".")
}

// BuildPostgresJSONPathExpr builds a PostgreSQL expression that extracts, as text, the value at a
// JSON path of segmentCount segments within columnName. Each segment is a bind parameter, numbered
// consecutively from paramIndex, so a segment may contain any character. The ::text casts keep the
// -> and ->> operator overloads unambiguous when the right operand is a bind parameter.
func BuildPostgresJSONPathExpr(columnName string, segmentCount, paramIndex int) string {
	expr := columnName
	for i := 0; i < segmentCount-1; i++ {
		expr += fmt.Sprintf("->($%d)::text", paramIndex+i)
	}
	return expr + fmt.Sprintf("->>($%d)::text", paramIndex+segmentCount-1)
}

// BuildSQLiteJSONPathExpr builds the SQLite counterpart of BuildPostgresJSONPathExpr. The JSON path
// is assembled from bind parameters with json_quote, which quotes and escapes each segment so that
// every character is taken as part of an object label instead of as path syntax.
func BuildSQLiteJSONPathExpr(columnName string, segmentCount int) string {
	path := "'$'"
	for i := 0; i < segmentCount; i++ {
		path += " || '.' || json_quote(?)"
	}
	return fmt.Sprintf("json_extract(%s, %s)", columnName, path)
}

// BuildPostgresJSONCondition builds a PostgreSQL JSON filter condition. The key's path segments and
// the compared value are bind parameters starting at paramIndex; use JSONConditionArgs to supply them.
func BuildPostgresJSONCondition(columnName, key string, paramIndex int) string {
	segments := SplitJSONKey(key)
	return fmt.Sprintf(" AND %s = $%d",
		BuildPostgresJSONPathExpr(columnName, len(segments), paramIndex), paramIndex+len(segments))
}

// BuildSQLiteJSONCondition builds the SQLite counterpart of BuildPostgresJSONCondition. It binds the
// same arguments in the same order, so both dialects can share a single argument list.
func BuildSQLiteJSONCondition(columnName, key string) string {
	return fmt.Sprintf(" AND %s = ?", BuildSQLiteJSONPathExpr(columnName, len(SplitJSONKey(key))))
}

// JSONConditionArgs returns the bind arguments for a condition built by BuildPostgresJSONCondition
// or BuildSQLiteJSONCondition: the key's path segments in order, followed by the compared value.
func JSONConditionArgs(key string, value interface{}) []interface{} {
	segments := SplitJSONKey(key)
	args := make([]interface{}, 0, len(segments)+1)
	for _, segment := range segments {
		args = append(args, segment)
	}
	return append(args, value)
}

// ValidateKey ensures that the provided key contains only safe characters (alphanumeric, underscores,
// and dots). It guards identifiers such as column names, which form part of the query text and cannot
// be bound as parameters. Filter keys are bound as parameters and are deliberately not restricted.
func ValidateKey(key string) error {
	for _, char := range key {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || char == '_' || char == '.') {
			return fmt.Errorf("key '%s' contains invalid characters", key)
		}
	}
	return nil
}
