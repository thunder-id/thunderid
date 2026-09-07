// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	"github.com/thunder-id/thunderid/internal/system/database/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const (
	testBaseQuery        = "SELECT * FROM users"
	testColumnName       = "attributes"
	testUserBaseQuery    = "SELECT USER_ID FROM \"USER\" WHERE 1=1"
	testAttributesColumn = "ATTRIBUTES"
)

type QueryBuilderTestSuite struct {
	suite.Suite
}

func TestQueryBuilderSuite(t *testing.T) {
	suite.Run(t, new(QueryBuilderTestSuite))
}

func (suite *QueryBuilderTestSuite) TestBuildFilterQuery() {
	queryID := "test_query"
	baseQuery := testBaseQuery
	columnName := testColumnName
	filters := map[string]interface{}{
		"role": "admin",
		"age":  30,
	}

	query, args, err := BuildFilterQuery(queryID, baseQuery, columnName, filters)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), queryID, query.ID)
	// Each key contributes its path segments followed by the compared value.
	assert.Len(suite.T(), args, 4)

	// Verify args order due to sorting of keys
	assert.Equal(suite.T(), "age", args[0])
	assert.Equal(suite.T(), int(30), args[1])
	assert.Equal(suite.T(), "role", args[2])
	assert.Equal(suite.T(), "admin", args[3])

	// Test Postgres query
	postgresQuery := query.GetQuery("postgres")
	assert.Contains(suite.T(), postgresQuery, baseQuery)
	assert.Contains(suite.T(), postgresQuery, "attributes->>($1)::text = $2")
	assert.Contains(suite.T(), postgresQuery, "attributes->>($3)::text = $4")

	// Test SQLite query
	sqliteQuery := query.GetQuery("sqlite")
	assert.Contains(suite.T(), sqliteQuery, baseQuery)
	assert.Contains(suite.T(), sqliteQuery, "json_extract(attributes, '$' || '.' || json_quote(?)) = ?")

	// Test default query (should return PostgreSQL query)
	defaultQuery := query.GetQuery("unknown")
	assert.Equal(suite.T(), postgresQuery, defaultQuery)
}

func (suite *QueryBuilderTestSuite) TestBuildFilterQueryWithEmptyFilters() {
	queryID := "empty_filters"
	baseQuery := testBaseQuery
	columnName := testColumnName
	filters := map[string]interface{}{}

	query, args, err := BuildFilterQuery(queryID, baseQuery, columnName, filters)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), queryID, query.ID)
	assert.Empty(suite.T(), args)

	// Both Postgres and SQLite queries should be the same as base query when no filters
	postgresQuery := query.GetQuery("postgres")
	sqliteQuery := query.GetQuery("sqlite")
	assert.Equal(suite.T(), baseQuery, postgresQuery)
	assert.Equal(suite.T(), baseQuery, sqliteQuery)
	assert.Equal(suite.T(), baseQuery, query.Query)
}

func (suite *QueryBuilderTestSuite) TestBuildFilterQueryWithInvalidColumnName() {
	queryID := "invalid_column"
	baseQuery := testBaseQuery
	columnName := "attributes;DROP TABLE users"
	filters := map[string]interface{}{
		"role": "admin",
	}

	query, args, err := BuildFilterQuery(queryID, baseQuery, columnName, filters)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "invalid column name")
	assert.Equal(suite.T(), model.DBQuery{}, query)
	assert.Nil(suite.T(), args)
}

// TestBuildFilterQuerySpecialCharacterKeys verifies that a filter key is not restricted to any
// character set: it is bound as a parameter rather than written into the query text.
func (suite *QueryBuilderTestSuite) TestBuildFilterQuerySpecialCharacterKeys() {
	specialKeys := []string{
		"silver-mail",
		"x@y",
		"a b",
		"o'k",
		"h\u00e9llo",
		"double\"quote",
		"back\\slash",
		"sql;injection",
		"drop') = 1 OR 1=1 --",
	}

	for _, key := range specialKeys {
		filters := map[string]interface{}{key: "value"}
		query, args, err := BuildFilterQuery("special_key", testBaseQuery, testColumnName, filters)

		assert.NoError(suite.T(), err, "Key should be accepted: %s", key)
		assert.Len(suite.T(), args, 2, "Key should be bound: %s", key)
		assert.Equal(suite.T(), key, args[0])
		assert.Equal(suite.T(), "value", args[1])

		// The key must never appear in the query text of either dialect.
		assert.NotContains(suite.T(), query.GetQuery("postgres"), key)
		assert.NotContains(suite.T(), query.GetQuery("sqlite"), key)
	}
}

func (suite *QueryBuilderTestSuite) TestValidateKey() {
	validKeys := []string{
		"name",
		"user_id",
		"role123",
		"UPPERCASE",
		"mixedCASE",
		"with_underscore",
		"_leading_underscore",
		"trailing_underscore_",
	}

	for _, key := range validKeys {
		err := ValidateKey(key)
		assert.NoError(suite.T(), err, "Key should be valid: %s", key)
	}
}

func (suite *QueryBuilderTestSuite) TestValidateKeyInvalid() {
	invalidKeys := []string{
		"space key",
		"hyphen-key",
		"special!char",
		"sql;injection",
		"quote'test",
		"double\"quote",
	}

	for _, key := range invalidKeys {
		err := ValidateKey(key)
		assert.Error(suite.T(), err, "Key should be invalid: %s", key)
		assert.Contains(suite.T(), err.Error(), "invalid characters")
	}
}

func (suite *QueryBuilderTestSuite) TestBuildFilterQueryDatabaseSpecificQueries() {
	queryID := "db_specific_test"
	baseQuery := testUserBaseQuery
	columnName := testAttributesColumn
	filters := map[string]interface{}{
		"email": "test@example.com",
		"name":  "John Doe",
	}

	query, args, err := BuildFilterQuery(queryID, baseQuery, columnName, filters)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), queryID, query.ID)
	assert.Len(suite.T(), args, 4)

	// Verify arguments are in sorted order (email, name), each preceded by its path segments
	assert.Equal(suite.T(), "email", args[0])
	assert.Equal(suite.T(), "test@example.com", args[1])
	assert.Equal(suite.T(), "name", args[2])
	assert.Equal(suite.T(), "John Doe", args[3])

	// Test PostgreSQL-specific query
	postgresQuery := query.GetQuery("postgres")
	expectedPostgres := testUserBaseQuery +
		" AND ATTRIBUTES->>($1)::text = $2" +
		" AND ATTRIBUTES->>($3)::text = $4"
	assert.Equal(suite.T(), expectedPostgres, postgresQuery)

	// Test SQLite-specific query
	sqliteQuery := query.GetQuery("sqlite")
	expectedSQLite := testUserBaseQuery +
		" AND json_extract(ATTRIBUTES, '$' || '.' || json_quote(?)) = ?" +
		" AND json_extract(ATTRIBUTES, '$' || '.' || json_quote(?)) = ?"
	assert.Equal(suite.T(), expectedSQLite, sqliteQuery)

	// Test that both queries are stored in the struct
	assert.Equal(suite.T(), expectedPostgres, query.PostgresQuery)
	assert.Equal(suite.T(), expectedSQLite, query.SQLiteQuery)
	assert.Equal(suite.T(), expectedPostgres, query.Query) // Default should be PostgreSQL
}

func (suite *QueryBuilderTestSuite) TestBuildFilterQuerySingleFilter() {
	queryID := "single_filter"
	baseQuery := "SELECT * FROM users WHERE active = true"
	columnName := "metadata"
	filters := map[string]interface{}{
		"department": "engineering",
	}

	query, args, err := BuildFilterQuery(queryID, baseQuery, columnName, filters)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), args, 2)
	assert.Equal(suite.T(), "department", args[0])
	assert.Equal(suite.T(), "engineering", args[1])

	// PostgreSQL query
	postgresQuery := query.GetQuery("postgres")
	expectedPostgres := "SELECT * FROM users WHERE active = true" +
		" AND metadata->>($1)::text = $2"
	assert.Equal(suite.T(), expectedPostgres, postgresQuery)

	// SQLite query
	sqliteQuery := query.GetQuery("sqlite")
	expectedSQLite := "SELECT * FROM users WHERE active = true" +
		" AND json_extract(metadata, '$' || '.' || json_quote(?)) = ?"
	assert.Equal(suite.T(), expectedSQLite, sqliteQuery)
}

func (suite *QueryBuilderTestSuite) TestBuildFilterQueryNestedPath() {
	queryID := "nested_path_filter"
	baseQuery := testUserBaseQuery
	columnName := testAttributesColumn
	filters := map[string]interface{}{
		"address.city": "Mountain View",
		"address.zip":  "94040",
	}

	query, args, err := BuildFilterQuery(queryID, baseQuery, columnName, filters)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), args, 6)

	// Verify args order (sorted by key), each key expanded into its path segments
	assert.Equal(suite.T(), []interface{}{
		"address", "city", "Mountain View",
		"address", "zip", "94040",
	}, args)

	// PostgreSQL query - should chain -> for each nested segment
	postgresQuery := query.GetQuery("postgres")
	expectedPostgres := testUserBaseQuery +
		" AND ATTRIBUTES->($1)::text->>($2)::text = $3" +
		" AND ATTRIBUTES->($4)::text->>($5)::text = $6"
	assert.Equal(suite.T(), expectedPostgres, postgresQuery)

	// SQLite query - should quote each segment into the json_extract path
	sqliteQuery := query.GetQuery("sqlite")
	expectedSQLite := testUserBaseQuery +
		" AND json_extract(ATTRIBUTES, '$' || '.' || json_quote(?) || '.' || json_quote(?)) = ?" +
		" AND json_extract(ATTRIBUTES, '$' || '.' || json_quote(?) || '.' || json_quote(?)) = ?"
	assert.Equal(suite.T(), expectedSQLite, sqliteQuery)
}

func (suite *QueryBuilderTestSuite) TestBuildFilterQueryMixedSimpleAndNestedPaths() {
	queryID := "mixed_paths_filter"
	baseQuery := testUserBaseQuery
	columnName := testAttributesColumn
	filters := map[string]interface{}{
		"username":     "john.doe",
		"address.city": "San Francisco",
		"age":          30,
	}

	query, args, err := BuildFilterQuery(queryID, baseQuery, columnName, filters)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), args, 7)

	// Verify args order (sorted by key: address.city, age, username)
	assert.Equal(suite.T(), []interface{}{
		"address", "city", "San Francisco",
		"age", 30,
		"username", "john.doe",
	}, args)

	// PostgreSQL query
	postgresQuery := query.GetQuery("postgres")
	expectedPostgres := testUserBaseQuery +
		" AND ATTRIBUTES->($1)::text->>($2)::text = $3" +
		" AND ATTRIBUTES->>($4)::text = $5" +
		" AND ATTRIBUTES->>($6)::text = $7"
	assert.Equal(suite.T(), expectedPostgres, postgresQuery)

	// SQLite query
	sqliteQuery := query.GetQuery("sqlite")
	expectedSQLite := testUserBaseQuery +
		" AND json_extract(ATTRIBUTES, '$' || '.' || json_quote(?) || '.' || json_quote(?)) = ?" +
		" AND json_extract(ATTRIBUTES, '$' || '.' || json_quote(?)) = ?" +
		" AND json_extract(ATTRIBUTES, '$' || '.' || json_quote(?)) = ?"
	assert.Equal(suite.T(), expectedSQLite, sqliteQuery)
}

func (suite *QueryBuilderTestSuite) TestBuildFilterQueryDeeplyNestedPath() {
	queryID := "deeply_nested_filter"
	baseQuery := "SELECT * FROM users WHERE 1=1"
	columnName := "data"
	filters := map[string]interface{}{
		"company.location.address.city": "New York",
	}

	query, args, err := BuildFilterQuery(queryID, baseQuery, columnName, filters)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), args, 5)
	assert.Equal(suite.T(), []interface{}{"company", "location", "address", "city", "New York"}, args)

	// PostgreSQL query - should handle deeply nested paths
	postgresQuery := query.GetQuery("postgres")
	expectedPostgres := "SELECT * FROM users WHERE 1=1" +
		" AND data->($1)::text->($2)::text->($3)::text->>($4)::text = $5"
	assert.Equal(suite.T(), expectedPostgres, postgresQuery)

	// SQLite query
	sqliteQuery := query.GetQuery("sqlite")
	expectedSQLite := "SELECT * FROM users WHERE 1=1" +
		" AND json_extract(data, '$' || '.' || json_quote(?) || '.' || json_quote(?)" +
		" || '.' || json_quote(?) || '.' || json_quote(?)) = ?"
	assert.Equal(suite.T(), expectedSQLite, sqliteQuery)
}

func (suite *QueryBuilderTestSuite) TestAppendDeploymentIDToFilterQueryWithNoExistingArgs() {
	queryID := "test_query"
	baseQuery := "SELECT * FROM users WHERE 1=1"
	deploymentID := "server-123"

	// Create initial query with no filters
	initialQuery := model.DBQuery{
		ID:            queryID,
		Query:         baseQuery,
		PostgresQuery: baseQuery,
		SQLiteQuery:   baseQuery,
	}
	initialArgs := []interface{}{}

	// Append server ID
	updatedQuery, updatedArgs := AppendDeploymentIDToFilterQuery(initialQuery, initialArgs, deploymentID)

	// Verify query ID is preserved
	assert.Equal(suite.T(), queryID, updatedQuery.ID)

	// Verify args
	assert.Len(suite.T(), updatedArgs, 1)
	assert.Equal(suite.T(), deploymentID, updatedArgs[0])

	// Verify PostgreSQL query
	expectedPostgres := baseQuery + " AND DEPLOYMENT_ID = $1"
	assert.Equal(suite.T(), expectedPostgres, updatedQuery.PostgresQuery)
	assert.Equal(suite.T(), expectedPostgres, updatedQuery.Query)

	// Verify SQLite query
	expectedSQLite := baseQuery + " AND DEPLOYMENT_ID = ?"
	assert.Equal(suite.T(), expectedSQLite, updatedQuery.SQLiteQuery)
}

func (suite *QueryBuilderTestSuite) TestAppendDeploymentIDToFilterQueryWithExistingArgs() {
	queryID := "filter_query"
	baseQuery := testUserBaseQuery
	columnName := testAttributesColumn
	filters := map[string]interface{}{
		"email": "user@example.com",
		"role":  "admin",
	}
	deploymentID := "server-456"

	// Build initial filter query
	query, args, err := BuildFilterQuery(queryID, baseQuery, columnName, filters)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), args, 4)

	// Append server ID
	updatedQuery, updatedArgs := AppendDeploymentIDToFilterQuery(query, args, deploymentID)

	// Verify query ID is preserved
	assert.Equal(suite.T(), queryID, updatedQuery.ID)

	// Verify args - should have original args plus server ID, which stays last
	assert.Equal(suite.T(), []interface{}{
		"email", "user@example.com",
		"role", "admin",
		deploymentID,
	}, updatedArgs)

	// Verify PostgreSQL query
	expectedPostgres := testUserBaseQuery +
		" AND ATTRIBUTES->>($1)::text = $2" +
		" AND ATTRIBUTES->>($3)::text = $4" +
		" AND DEPLOYMENT_ID = $5"
	assert.Equal(suite.T(), expectedPostgres, updatedQuery.PostgresQuery)
	assert.Equal(suite.T(), expectedPostgres, updatedQuery.Query)

	// Verify SQLite query
	expectedSQLite := testUserBaseQuery +
		" AND json_extract(ATTRIBUTES, '$' || '.' || json_quote(?)) = ?" +
		" AND json_extract(ATTRIBUTES, '$' || '.' || json_quote(?)) = ?" +
		" AND DEPLOYMENT_ID = ?"
	assert.Equal(suite.T(), expectedSQLite, updatedQuery.SQLiteQuery)
}

func (suite *QueryBuilderTestSuite) TestAppendDeploymentIDToFilterQueryWithSingleFilter() {
	queryID := "single_filter_query"
	baseQuery := "SELECT USER_ID FROM \"USER\" WHERE active = true"
	columnName := "metadata"
	filters := map[string]interface{}{
		"department": "engineering",
	}
	deploymentID := "primary-server"

	// Build initial filter query
	query, args, err := BuildFilterQuery(queryID, baseQuery, columnName, filters)
	assert.NoError(suite.T(), err)

	// Append server ID
	updatedQuery, updatedArgs := AppendDeploymentIDToFilterQuery(query, args, deploymentID)

	// Verify args
	assert.Equal(suite.T(), []interface{}{"department", "engineering", deploymentID}, updatedArgs)

	// Verify PostgreSQL query
	expectedPostgres := "SELECT USER_ID FROM \"USER\" WHERE active = true" +
		" AND metadata->>($1)::text = $2" +
		" AND DEPLOYMENT_ID = $3"
	assert.Equal(suite.T(), expectedPostgres, updatedQuery.PostgresQuery)

	// Verify SQLite query
	expectedSQLite := "SELECT USER_ID FROM \"USER\" WHERE active = true" +
		" AND json_extract(metadata, '$' || '.' || json_quote(?)) = ?" +
		" AND DEPLOYMENT_ID = ?"
	assert.Equal(suite.T(), expectedSQLite, updatedQuery.SQLiteQuery)
}

func (suite *QueryBuilderTestSuite) TestAppendDeploymentIDToFilterQueryWithNestedFilters() {
	queryID := "nested_filter_query"
	baseQuery := testUserBaseQuery
	columnName := testAttributesColumn
	filters := map[string]interface{}{
		"address.city": "San Francisco",
		"name":         "John Doe",
	}
	deploymentID := "west-coast-server"

	// Build initial filter query
	query, args, err := BuildFilterQuery(queryID, baseQuery, columnName, filters)
	assert.NoError(suite.T(), err)

	// Append server ID
	updatedQuery, updatedArgs := AppendDeploymentIDToFilterQuery(query, args, deploymentID)

	// Verify args
	assert.Equal(suite.T(), []interface{}{
		"address", "city", "San Francisco",
		"name", "John Doe",
		deploymentID,
	}, updatedArgs)

	// Verify PostgreSQL query
	expectedPostgres := testUserBaseQuery +
		" AND ATTRIBUTES->($1)::text->>($2)::text = $3" +
		" AND ATTRIBUTES->>($4)::text = $5" +
		" AND DEPLOYMENT_ID = $6"
	assert.Equal(suite.T(), expectedPostgres, updatedQuery.PostgresQuery)

	// Verify SQLite query
	expectedSQLite := testUserBaseQuery +
		" AND json_extract(ATTRIBUTES, '$' || '.' || json_quote(?) || '.' || json_quote(?)) = ?" +
		" AND json_extract(ATTRIBUTES, '$' || '.' || json_quote(?)) = ?" +
		" AND DEPLOYMENT_ID = ?"
	assert.Equal(suite.T(), expectedSQLite, updatedQuery.SQLiteQuery)
}
