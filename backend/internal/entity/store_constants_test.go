// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type StoreConstantsTestSuite struct {
	suite.Suite
}

func TestStoreConstantsTestSuite(t *testing.T) {
	suite.Run(t, new(StoreConstantsTestSuite))
}

const testDeploymentID = "test-deployment"

const testCategoryBaseQuery = `SELECT * FROM "ENTITY" WHERE CATEGORY = $1`

func (s *StoreConstantsTestSuite) TestAppendOUIDsINClause_EmptyOUIDs() {
	q, args := appendOUIDsINClause(QueryGetEntityByID, []interface{}{"e1", "dep1"}, []string{})
	s.Contains(q.Query, "1=0")
	s.Len(args, 2)
}

func (s *StoreConstantsTestSuite) TestAppendOUIDsINClause_WithOUIDs() {
	q, args := appendOUIDsINClause(QueryGetEntityByID, []interface{}{"e1", "dep1"}, []string{"ou1", "ou2"})
	s.Contains(q.Query, "OU_ID IN")
	s.Len(args, 4) // original 2 + 2 OU IDs
}

func (s *StoreConstantsTestSuite) TestBuildEntityCountQueryByOUIDs_NoFilters() {
	q, args, err := buildEntityCountQueryByOUIDs("user", []string{"ou1"}, nil, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildEntityCountQueryByOUIDs_WithFilters() {
	filters := map[string]interface{}{"email": "a@b.com"}
	q, args, err := buildEntityCountQueryByOUIDs("user", []string{"ou1"}, filters, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildEntityListQueryByOUIDs_NoFilters() {
	q, args, err := buildEntityListQueryByOUIDs("user", []string{"ou1"}, nil, 10, 0, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildEntityListQueryByOUIDs_WithFilters() {
	filters := map[string]interface{}{"email": "a@b.com"}
	q, args, err := buildEntityListQueryByOUIDs("user", []string{"ou1"}, filters, 10, 0, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQuery_EmptyFilters() {
	_, _, err := buildIdentifyQuery(map[string]interface{}{}, testDeploymentID)
	s.Error(err)
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQuery_WithFilters() {
	q, args, err := buildIdentifyQuery(map[string]interface{}{"email": "a@b.com"}, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildEntityINClauseQuery_EmptyIDs() {
	baseQuery := `SELECT ID FROM "ENTITY" WHERE ID IN (%s) AND DEPLOYMENT_ID = %s`
	_, _, err := buildEntityINClauseQuery("qid", baseQuery, []string{}, testDeploymentID)
	s.Error(err)
}

func (s *StoreConstantsTestSuite) TestBuildEntityINClauseQuery_WithIDs() {
	baseQuery := `SELECT ID FROM "ENTITY" WHERE ID IN (%s) AND DEPLOYMENT_ID = %s`
	q, args, err := buildEntityINClauseQuery("qid", baseQuery, []string{"id1", "id2"}, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildBulkEntityExistsQuery_Success() {
	q, args, err := buildBulkEntityExistsQuery([]string{"id1", "id2"}, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildBulkEntityExistsQueryInOUs_EmptyEntityIDs() {
	_, _, err := buildBulkEntityExistsQueryInOUs([]string{}, []string{"ou1"}, testDeploymentID)
	s.Error(err)
}

func (s *StoreConstantsTestSuite) TestBuildBulkEntityExistsQueryInOUs_EmptyOUIDs() {
	_, _, err := buildBulkEntityExistsQueryInOUs([]string{"id1"}, []string{}, testDeploymentID)
	s.Error(err)
}

func (s *StoreConstantsTestSuite) TestBuildBulkEntityExistsQueryInOUs_WithBoth() {
	q, args, err := buildBulkEntityExistsQueryInOUs([]string{"id1", "id2"}, []string{"ou1"}, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildEntityListQuery_NoFilters() {
	q, args, err := buildEntityListQuery("user", nil, 10, 0, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildEntityListQuery_WithFilters() {
	filters := map[string]interface{}{"email": "a@b.com"}
	q, args, err := buildEntityListQuery("user", filters, 10, 0, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildEntityCountQuery_NoFilters() {
	q, args, err := buildEntityCountQuery("user", nil, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildEntityCountQuery_WithFilters() {
	filters := map[string]interface{}{"email": "a@b.com"}
	q, args, err := buildEntityCountQuery("user", filters, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQueryFromIdentifiers_EmptyFilters() {
	_, _, err := buildIdentifyQueryFromIdentifiers(map[string]interface{}{}, testDeploymentID)
	s.Error(err)
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQueryFromIdentifiers_SingleFilter() {
	q, args, err := buildIdentifyQueryFromIdentifiers(map[string]interface{}{"email": "a@b.com"}, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQueryFromIdentifiers_MultipleFilters() {
	filters := map[string]interface{}{"email": "a@b.com", "username": "user1"}
	q, args, err := buildIdentifyQueryFromIdentifiers(filters, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.Contains(q.Query, "INNER JOIN")
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQueryHybrid_EmptyIndexedFilters() {
	_, _, err := buildIdentifyQueryHybrid(map[string]interface{}{}, map[string]interface{}{"k": "v"}, testDeploymentID)
	s.Error(err)
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQueryHybrid_Success() {
	indexed := map[string]interface{}{"email": "a@b.com"}
	nonIndexed := map[string]interface{}{"username": "user1"}
	q, args, err := buildIdentifyQueryHybrid(indexed, nonIndexed, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQueryHybrid_MultipleIndexed() {
	indexed := map[string]interface{}{"email": "a@b.com", "phone": "123"}
	nonIndexed := map[string]interface{}{"username": "user1"}
	q, args, err := buildIdentifyQueryHybrid(indexed, nonIndexed, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildGetEntitiesByIDsQuery_Success() {
	q, args, err := buildGetEntitiesByIDsQuery([]string{"id1", "id2"}, testDeploymentID)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildPaginatedQuery_Success() {
	base := `SELECT * FROM "ENTITY" WHERE DEPLOYMENT_ID = $1`
	result, err := buildPaginatedQuery(base, 1, "$")
	s.NoError(err)
	s.Contains(result, "LIMIT")
	s.Contains(result, "OFFSET")
}

func (s *StoreConstantsTestSuite) TestBuildFilterQueryWithOffset_Success() {
	base := testCategoryBaseQuery
	filters := map[string]interface{}{"email": "a@b.com"}
	q, args, err := buildFilterQueryWithOffset("test-qid", base, filters, 1)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildFilterQueryWithOffset_NoFilters() {
	base := testCategoryBaseQuery
	q, args, err := buildFilterQueryWithOffset("test-qid", base, nil, 1)
	s.NoError(err)
	s.NotEmpty(q.Query)
	_ = args
}

// TestBuildIdentifyQuery_COALESCE_* verify that the JSON fallback query searches both
// ATTRIBUTES and SYSTEM_ATTRIBUTES using COALESCE so that an entity can be found
// regardless of which column holds the filter key (e.g. clientId in SYSTEM_ATTRIBUTES).
func (s *StoreConstantsTestSuite) TestBuildIdentifyQuery_COALESCE_PostgresQuery() {
	q, args, err := buildIdentifyQuery(map[string]interface{}{"clientId": "app123"}, testDeploymentID)
	s.NoError(err)
	s.Contains(q.PostgresQuery, "COALESCE")
	s.Contains(q.PostgresQuery, "SYSTEM_ATTRIBUTES->>($1)::text")
	s.Contains(q.PostgresQuery, "ATTRIBUTES->>($2)::text")
	s.Contains(q.PostgresQuery, "= $3")
	s.Contains(q.PostgresQuery, "DEPLOYMENT_ID = $4")
	// The key is bound once per column, then the value, then the deployment ID last.
	s.Equal([]interface{}{"clientId", "clientId", "app123", testDeploymentID}, args)
	s.NotContains(q.PostgresQuery, "'clientId'")
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQuery_COALESCE_SQLiteQuery() {
	q, args, err := buildIdentifyQuery(map[string]interface{}{"clientId": "app123"}, testDeploymentID)
	s.NoError(err)
	s.Contains(q.SQLiteQuery, "COALESCE")
	s.Contains(q.SQLiteQuery, "json_extract(ATTRIBUTES, '$' || '.' || json_quote(?))")
	s.Contains(q.SQLiteQuery, "json_extract(SYSTEM_ATTRIBUTES, '$' || '.' || json_quote(?))")
	s.NotContains(q.SQLiteQuery, "clientId")
	_ = args
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQuery_MultipleFilters_CorrectParamIndexes() {
	// keys are sorted: clientId < name. Each key binds its segments once per column, then its
	// value, so clientId takes $1-$3, name takes $4-$6 and the deployment ID is last at $7.
	filters := map[string]interface{}{"clientId": "app123", "name": "myapp"}
	q, args, err := buildIdentifyQuery(filters, testDeploymentID)
	s.NoError(err)
	s.Contains(q.PostgresQuery, "COALESCE(SYSTEM_ATTRIBUTES->>($1)::text, ATTRIBUTES->>($2)::text) = $3")
	s.Contains(q.PostgresQuery, "COALESCE(SYSTEM_ATTRIBUTES->>($4)::text, ATTRIBUTES->>($5)::text) = $6")
	s.Contains(q.PostgresQuery, "DEPLOYMENT_ID = $7")
	s.Equal([]interface{}{
		"clientId", "clientId", "app123",
		"name", "name", "myapp",
		testDeploymentID,
	}, args)
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQuery_NestedKey_UsesPathSyntax() {
	q, args, err := buildIdentifyQuery(map[string]interface{}{"address.city": "NYC"}, testDeploymentID)
	s.NoError(err)
	s.Contains(q.PostgresQuery, "SYSTEM_ATTRIBUTES->($1)::text->>($2)::text")
	s.Contains(q.PostgresQuery, "ATTRIBUTES->($3)::text->>($4)::text")
	s.Contains(q.SQLiteQuery, "json_extract(SYSTEM_ATTRIBUTES, '$' || '.' || json_quote(?) || '.' || json_quote(?))")
	s.Equal([]interface{}{
		"address", "city",
		"address", "city",
		"NYC", testDeploymentID,
	}, args)
}

// TestBuildIdentifyQuery_SpecialCharacterKeys covers keys well outside the entity type property
// name charset, including the hyphenated name from the reported issue. Schema validation keeps
// most of these out, but not every filter key comes from a schema — the agent list handler passes
// the requested attribute through verbatim — so the builder must bind any key it is handed rather
// than write it into the query text.
func (s *StoreConstantsTestSuite) TestBuildIdentifyQuery_SpecialCharacterKeys() {
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
		q, args, err := buildIdentifyQuery(map[string]interface{}{key: "value"}, testDeploymentID)
		s.NoError(err, "Key should be accepted: %s", key)
		s.Equal([]interface{}{key, key, "value", testDeploymentID}, args)
		s.NotContains(q.PostgresQuery, key)
		s.NotContains(q.SQLiteQuery, key)
	}
}

// TestBuildFilterQueryWithOffset_SpecialCharacterKey covers the entity listing path, which
// shares the same key handling as the uniqueness path.
func (s *StoreConstantsTestSuite) TestBuildFilterQueryWithOffset_SpecialCharacterKey() {
	base := testCategoryBaseQuery
	q, args, err := buildFilterQueryWithOffset(
		"test-qid", base, map[string]interface{}{"silver-mail": "a@b.com"}, 1)
	s.NoError(err)
	s.Contains(q.PostgresQuery, "ATTRIBUTES->>($2)::text = $3")
	s.Contains(q.SQLiteQuery, "json_extract(ATTRIBUTES, '$' || '.' || json_quote(?)) = ?")
	s.Equal([]interface{}{"silver-mail", "a@b.com"}, args)
	s.NotContains(q.PostgresQuery, "silver-mail")
}

// TestBuildIdentifyQueryHybrid_SpecialCharacterNonIndexedKey covers the hybrid path, where an
// indexed identifier narrows the search and a non-indexed JSON attribute completes it.
func (s *StoreConstantsTestSuite) TestBuildIdentifyQueryHybrid_SpecialCharacterNonIndexedKey() {
	q, args, err := buildIdentifyQueryHybrid(
		map[string]interface{}{"email": "a@b.com"},
		map[string]interface{}{"silver-mail": "c@d.com"},
		testDeploymentID)
	s.NoError(err)
	s.Contains(q.PostgresQuery, "e.SYSTEM_ATTRIBUTES->>($3)::text")
	s.Contains(q.PostgresQuery, "e.ATTRIBUTES->>($4)::text")
	s.Contains(q.PostgresQuery, "e.DEPLOYMENT_ID = $6")
	s.Equal([]interface{}{
		"email", "a@b.com",
		"silver-mail", "silver-mail", "c@d.com",
		testDeploymentID,
	}, args)
	s.NotContains(q.PostgresQuery, "silver-mail")
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQueryHybrid_NonIndexed_UsesCOALESCE() {
	indexed := map[string]interface{}{"email": "a@b.com"}
	nonIndexed := map[string]interface{}{"clientId": "app123"}
	q, _, err := buildIdentifyQueryHybrid(indexed, nonIndexed, testDeploymentID)
	s.NoError(err)
	s.Contains(q.PostgresQuery, "COALESCE")
	s.Contains(q.PostgresQuery, "e.ATTRIBUTES->>($4)::text")
	s.Contains(q.PostgresQuery, "e.SYSTEM_ATTRIBUTES->>($3)::text")
	s.Contains(q.SQLiteQuery, "COALESCE")
	s.Contains(q.SQLiteQuery, "json_extract(e.ATTRIBUTES, '$' || '.' || json_quote(?))")
	s.Contains(q.SQLiteQuery, "json_extract(e.SYSTEM_ATTRIBUTES, '$' || '.' || json_quote(?))")
}
