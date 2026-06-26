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
	"testing"

	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type StoreConstantsTestSuite struct {
	suite.Suite
}

func TestStoreConstantsTestSuite(t *testing.T) {
	suite.Run(t, new(StoreConstantsTestSuite))
}

const testDeploymentID = "test-deployment"

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

func (s *StoreConstantsTestSuite) TestBuildEntityListQuery_WithContainsAttributeSearch() {
	filters := map[string]interface{}{
		"name": providers.AttributeSearch{Operator: tidcommon.OperatorContains, Value: "Foo"},
	}
	q, args, err := buildEntityListQuery("app", filters, 10, 0, testDeploymentID)
	s.NoError(err)

	// A single case-insensitive contains condition against both SYSTEM_ATTRIBUTES and ATTRIBUTES.
	s.Contains(q.PostgresQuery, "COALESCE(SYSTEM_ATTRIBUTES->>'name', ATTRIBUTES->>'name', '') ILIKE $")
	s.Contains(q.SQLiteQuery, "LOWER(COALESCE(json_extract(SYSTEM_ATTRIBUTES, '$.name')")
	s.Contains(q.SQLiteQuery, "LIKE ?")

	// category + one bound pattern + deployment id + limit + offset.
	s.Len(args, 5)
	s.Equal("%foo%", args[1])
}

func (s *StoreConstantsTestSuite) TestBuildEntityCountQuery_WithContainsAttributeSearch() {
	filters := map[string]interface{}{
		"name": providers.AttributeSearch{Operator: tidcommon.OperatorContains, Value: "bar"},
	}
	q, args, err := buildEntityCountQuery("app", filters, testDeploymentID)
	s.NoError(err)
	s.Contains(q.PostgresQuery, "ILIKE $")
	// category + one pattern + deployment id.
	s.Len(args, 3)
	s.Equal("%bar%", args[1])
}

func (s *StoreConstantsTestSuite) TestBuildEntityListQuery_EqAttributeSearch() {
	filters := map[string]interface{}{
		"name": providers.AttributeSearch{Operator: tidcommon.OperatorEq, Value: "Exact"},
	}
	q, args, err := buildEntityListQuery("app", filters, 10, 0, testDeploymentID)
	s.NoError(err)
	s.Contains(q.PostgresQuery, "LOWER(COALESCE(SYSTEM_ATTRIBUTES->>'name', ATTRIBUTES->>'name', '')) = LOWER($")
	s.Equal("Exact", args[1])
}

func (s *StoreConstantsTestSuite) TestBuildEntityListQuery_AttributeSearchEscapesWildcards() {
	filters := map[string]interface{}{
		"name": providers.AttributeSearch{Operator: tidcommon.OperatorContains, Value: "50%_off"},
	}
	_, args, err := buildEntityListQuery("app", filters, 10, 0, testDeploymentID)
	s.NoError(err)
	s.Equal(`%50\%\_off%`, args[1])
}

func (s *StoreConstantsTestSuite) TestBuildEntityListQuery_UnsupportedAttributeSearchOperatorErrors() {
	filters := map[string]interface{}{
		"name": providers.AttributeSearch{Operator: tidcommon.OperatorGt, Value: "x"},
	}
	_, _, err := buildEntityListQuery("app", filters, 10, 0, testDeploymentID)
	s.Error(err)
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
	base := `SELECT * FROM "ENTITY" WHERE CATEGORY = $1`
	filters := map[string]interface{}{"email": "a@b.com"}
	q, args, err := buildFilterQueryWithOffset("test-qid", base, filters, 1)
	s.NoError(err)
	s.NotEmpty(q.Query)
	s.NotEmpty(args)
}

func (s *StoreConstantsTestSuite) TestBuildFilterQueryWithOffset_NoFilters() {
	base := `SELECT * FROM "ENTITY" WHERE CATEGORY = $1`
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
	s.Contains(q.PostgresQuery, "ATTRIBUTES->>'clientId'")
	s.Contains(q.PostgresQuery, "SYSTEM_ATTRIBUTES->>'clientId'")
	s.Contains(q.PostgresQuery, "$1")
	s.Contains(q.PostgresQuery, "$2")
	s.Len(args, 2)
	s.Equal("app123", args[0])
	s.Equal(testDeploymentID, args[1])
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQuery_COALESCE_SQLiteQuery() {
	q, args, err := buildIdentifyQuery(map[string]interface{}{"clientId": "app123"}, testDeploymentID)
	s.NoError(err)
	s.Contains(q.SQLiteQuery, "COALESCE")
	s.Contains(q.SQLiteQuery, "json_extract(ATTRIBUTES, '$.clientId')")
	s.Contains(q.SQLiteQuery, "json_extract(SYSTEM_ATTRIBUTES, '$.clientId')")
	_ = args
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQuery_MultipleFilters_CorrectParamIndexes() {
	// keys are sorted: clientId < name, so clientId=$1, name=$2, deploymentID=$3
	filters := map[string]interface{}{"clientId": "app123", "name": "myapp"}
	q, args, err := buildIdentifyQuery(filters, testDeploymentID)
	s.NoError(err)
	s.Contains(q.PostgresQuery, "$1")
	s.Contains(q.PostgresQuery, "$2")
	s.Contains(q.PostgresQuery, "$3")
	s.Len(args, 3)
	s.Equal(testDeploymentID, args[2])
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQuery_NestedKey_UsesPathSyntax() {
	q, _, err := buildIdentifyQuery(map[string]interface{}{"address.city": "NYC"}, testDeploymentID)
	s.NoError(err)
	s.Contains(q.PostgresQuery, "#>>")
	s.Contains(q.PostgresQuery, "{address,city}")
	s.Contains(q.SQLiteQuery, "json_extract")
	s.Contains(q.SQLiteQuery, "$.address.city")
}

func (s *StoreConstantsTestSuite) TestBuildIdentifyQueryHybrid_NonIndexed_UsesCOALESCE() {
	indexed := map[string]interface{}{"email": "a@b.com"}
	nonIndexed := map[string]interface{}{"clientId": "app123"}
	q, _, err := buildIdentifyQueryHybrid(indexed, nonIndexed, testDeploymentID)
	s.NoError(err)
	s.Contains(q.PostgresQuery, "COALESCE")
	s.Contains(q.PostgresQuery, "e.ATTRIBUTES->>'clientId'")
	s.Contains(q.PostgresQuery, "e.SYSTEM_ATTRIBUTES->>'clientId'")
	s.Contains(q.SQLiteQuery, "COALESCE")
	s.Contains(q.SQLiteQuery, "json_extract(e.ATTRIBUTES, '$.clientId')")
	s.Contains(q.SQLiteQuery, "json_extract(e.SYSTEM_ATTRIBUTES, '$.clientId')")
}
