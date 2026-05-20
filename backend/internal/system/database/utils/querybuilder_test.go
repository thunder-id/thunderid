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
	assert.Len(suite.T(), args, 2)

	// Verify args order due to sorting of keys
	assert.Equal(suite.T(), int(30), args[0])
	assert.Equal(suite.T(), "admin", args[1])

	// Test Postgres query
	postgresQuery := query.GetQuery("postgres")
	assert.Contains(suite.T(), postgresQuery, baseQuery)
	assert.Contains(suite.T(), postgresQuery, "attributes->>'age' = $1")
	assert.Contains(suite.T(), postgresQuery, "attributes->>'role' = $2")

	// Test SQLite query
	sqliteQuery := query.GetQuery("sqlite")
	assert.Contains(suite.T(), sqliteQuery, baseQuery)
	assert.Contains(suite.T(), sqliteQuery, "json_extract(attributes, '$.age') = ?")
	assert.Contains(suite.T(), sqliteQuery, "json_extract(attributes, '$.role') = ?")

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

func (suite *QueryBuilderTestSuite) TestBuildFilterQueryWithInvalidFilterKey() {
	queryID := "invalid_filter_key"
	baseQuery := testBaseQuery
	columnName := testColumnName
	filters := map[string]interface{}{
		"valid":              "value",
		"invalid-filter-key": "value", // Contains invalid character '-'
	}

	query, args, err := BuildFilterQuery(queryID, baseQuery, columnName, filters)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "invalid filter key")
	assert.Equal(suite.T(), model.DBQuery{}, query)
	assert.Nil(suite.T(), args)
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
	assert.Len(suite.T(), args, 2)

	// Verify arguments are in sorted order (email, name)
	assert.Equal(suite.T(), "test@example.com", args[0])
	assert.Equal(suite.T(), "John Doe", args[1])

	// Test PostgreSQL-specific query
	postgresQuery := query.GetQuery("postgres")
	expectedPostgres := testUserBaseQuery +
		" AND ATTRIBUTES->>'email' = $1" +
		" AND ATTRIBUTES->>'name' = $2"
	assert.Equal(suite.T(), expectedPostgres, postgresQuery)

	// Test SQLite-specific query
	sqliteQuery := query.GetQuery("sqlite")
	expectedSQLite := testUserBaseQuery +
		" AND json_extract(ATTRIBUTES, '$.email') = ?" +
		" AND json_extract(ATTRIBUTES, '$.name') = ?"
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
	assert.Len(suite.T(), args, 1)
	assert.Equal(suite.T(), "engineering", args[0])

	// PostgreSQL query
	postgresQuery := query.GetQuery("postgres")
	expectedPostgres := "SELECT * FROM users WHERE active = true" +
		" AND metadata->>'department' = $1"
	assert.Equal(suite.T(), expectedPostgres, postgresQuery)

	// SQLite query
	sqliteQuery := query.GetQuery("sqlite")
	expectedSQLite := "SELECT * FROM users WHERE active = true" +
		" AND json_extract(metadata, '$.department') = ?"
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
	assert.Len(suite.T(), args, 2)

	// Verify args order (sorted by key)
	assert.Equal(suite.T(), "Mountain View", args[0])
	assert.Equal(suite.T(), "94040", args[1])

	// PostgreSQL query - should use #>> operator for nested paths
	postgresQuery := query.GetQuery("postgres")
	expectedPostgres := testUserBaseQuery +
		" AND ATTRIBUTES#>>'{address,city}' = $1" +
		" AND ATTRIBUTES#>>'{address,zip}' = $2"
	assert.Equal(suite.T(), expectedPostgres, postgresQuery)

	// SQLite query - should use json_extract with dot notation
	sqliteQuery := query.GetQuery("sqlite")
	expectedSQLite := testUserBaseQuery +
		" AND json_extract(ATTRIBUTES, '$.address.city') = ?" +
		" AND json_extract(ATTRIBUTES, '$.address.zip') = ?"
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
	assert.Len(suite.T(), args, 3)

	// Verify args order (sorted by key: address.city, age, username)
	assert.Equal(suite.T(), "San Francisco", args[0])
	assert.Equal(suite.T(), 30, args[1])
	assert.Equal(suite.T(), "john.doe", args[2])

	// PostgreSQL query
	postgresQuery := query.GetQuery("postgres")
	expectedPostgres := testUserBaseQuery +
		" AND ATTRIBUTES#>>'{address,city}' = $1" +
		" AND ATTRIBUTES->>'age' = $2" +
		" AND ATTRIBUTES->>'username' = $3"
	assert.Equal(suite.T(), expectedPostgres, postgresQuery)

	// SQLite query
	sqliteQuery := query.GetQuery("sqlite")
	expectedSQLite := testUserBaseQuery +
		" AND json_extract(ATTRIBUTES, '$.address.city') = ?" +
		" AND json_extract(ATTRIBUTES, '$.age') = ?" +
		" AND json_extract(ATTRIBUTES, '$.username') = ?"
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
	assert.Len(suite.T(), args, 1)
	assert.Equal(suite.T(), "New York", args[0])

	// PostgreSQL query - should handle deeply nested paths
	postgresQuery := query.GetQuery("postgres")
	expectedPostgres := "SELECT * FROM users WHERE 1=1" +
		" AND data#>>'{company,location,address,city}' = $1"
	assert.Equal(suite.T(), expectedPostgres, postgresQuery)

	// SQLite query
	sqliteQuery := query.GetQuery("sqlite")
	expectedSQLite := "SELECT * FROM users WHERE 1=1" +
		" AND json_extract(data, '$.company.location.address.city') = ?"
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
	assert.Len(suite.T(), args, 2)

	// Append server ID
	updatedQuery, updatedArgs := AppendDeploymentIDToFilterQuery(query, args, deploymentID)

	// Verify query ID is preserved
	assert.Equal(suite.T(), queryID, updatedQuery.ID)

	// Verify args - should have original args plus server ID
	assert.Len(suite.T(), updatedArgs, 3)
	assert.Equal(suite.T(), "user@example.com", updatedArgs[0])
	assert.Equal(suite.T(), "admin", updatedArgs[1])
	assert.Equal(suite.T(), deploymentID, updatedArgs[2])

	// Verify PostgreSQL query
	expectedPostgres := testUserBaseQuery +
		" AND ATTRIBUTES->>'email' = $1" +
		" AND ATTRIBUTES->>'role' = $2" +
		" AND DEPLOYMENT_ID = $3"
	assert.Equal(suite.T(), expectedPostgres, updatedQuery.PostgresQuery)
	assert.Equal(suite.T(), expectedPostgres, updatedQuery.Query)

	// Verify SQLite query
	expectedSQLite := testUserBaseQuery +
		" AND json_extract(ATTRIBUTES, '$.email') = ?" +
		" AND json_extract(ATTRIBUTES, '$.role') = ?" +
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
	assert.Len(suite.T(), updatedArgs, 2)
	assert.Equal(suite.T(), "engineering", updatedArgs[0])
	assert.Equal(suite.T(), deploymentID, updatedArgs[1])

	// Verify PostgreSQL query
	expectedPostgres := "SELECT USER_ID FROM \"USER\" WHERE active = true" +
		" AND metadata->>'department' = $1" +
		" AND DEPLOYMENT_ID = $2"
	assert.Equal(suite.T(), expectedPostgres, updatedQuery.PostgresQuery)

	// Verify SQLite query
	expectedSQLite := "SELECT USER_ID FROM \"USER\" WHERE active = true" +
		" AND json_extract(metadata, '$.department') = ?" +
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
	assert.Len(suite.T(), updatedArgs, 3)
	assert.Equal(suite.T(), "San Francisco", updatedArgs[0])
	assert.Equal(suite.T(), "John Doe", updatedArgs[1])
	assert.Equal(suite.T(), deploymentID, updatedArgs[2])

	// Verify PostgreSQL query
	expectedPostgres := testUserBaseQuery +
		" AND ATTRIBUTES#>>'{address,city}' = $1" +
		" AND ATTRIBUTES->>'name' = $2" +
		" AND DEPLOYMENT_ID = $3"
	assert.Equal(suite.T(), expectedPostgres, updatedQuery.PostgresQuery)

	// Verify SQLite query
	expectedSQLite := testUserBaseQuery +
		" AND json_extract(ATTRIBUTES, '$.address.city') = ?" +
		" AND json_extract(ATTRIBUTES, '$.name') = ?" +
		" AND DEPLOYMENT_ID = ?"
	assert.Equal(suite.T(), expectedSQLite, updatedQuery.SQLiteQuery)
}
