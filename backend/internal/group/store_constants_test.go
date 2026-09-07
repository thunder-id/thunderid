// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// StoreConstantsTestSuite is the test suite for store_constants.go functions.
type StoreConstantsTestSuite struct {
	suite.Suite
}

// TestStoreConstantsTestSuite runs the test suite.
func TestStoreConstantsTestSuite(t *testing.T) {
	suite.Run(t, new(StoreConstantsTestSuite))
}

// Test buildGetGroupsByIDsQuery

func (suite *StoreConstantsTestSuite) TestBuildGetGroupsByIDsQuery_EmptyIDs() {
	query, args, err := buildGetGroupsByIDsQuery([]string{}, testDeploymentID)

	suite.Error(err)
	suite.Contains(err.Error(), "groupIDs list cannot be empty")
	suite.Nil(args)
	suite.Empty(query.Query)
}

func (suite *StoreConstantsTestSuite) TestBuildGetGroupsByIDsQuery_SingleID() {
	query, args, err := buildGetGroupsByIDsQuery([]string{"group-1"}, testDeploymentID)

	suite.NoError(err)
	suite.Equal("GRQ-GROUP_MGT-19", query.ID)
	suite.Equal(
		`SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" WHERE ID IN ($1) AND DEPLOYMENT_ID = $2`,
		query.PostgresQuery,
	)
	suite.Equal(
		`SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" WHERE ID IN ($1) AND DEPLOYMENT_ID = $2`,
		query.SQLiteQuery,
	)
	suite.Len(args, 2)
	suite.Equal("group-1", args[0])
	suite.Equal(testDeploymentID, args[1])
}

func (suite *StoreConstantsTestSuite) TestBuildGetGroupsByIDsQuery_MultipleIDs() {
	query, args, err := buildGetGroupsByIDsQuery(
		[]string{"group-1", "group-2", "group-3"}, testDeploymentID,
	)

	suite.NoError(err)
	suite.Equal("GRQ-GROUP_MGT-19", query.ID)
	suite.Equal(
		`SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" WHERE ID IN ($1,$2,$3) AND DEPLOYMENT_ID = $4`,
		query.PostgresQuery,
	)
	suite.Equal(
		`SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" WHERE ID IN ($1,$2,$3) AND DEPLOYMENT_ID = $4`,
		query.SQLiteQuery,
	)
	suite.Len(args, 4)
	suite.Equal("group-1", args[0])
	suite.Equal("group-2", args[1])
	suite.Equal("group-3", args[2])
	suite.Equal(testDeploymentID, args[3])
}

// Test buildBulkGroupExistsQuery

func (suite *StoreConstantsTestSuite) TestBuildBulkGroupExistsQuery_EmptyIDs() {
	query, args, err := buildBulkGroupExistsQuery([]string{}, testDeploymentID)

	suite.Error(err)
	suite.Contains(err.Error(), "groupIDs list cannot be empty")
	suite.Nil(args)
	suite.Empty(query.Query)
}

func (suite *StoreConstantsTestSuite) TestBuildBulkGroupExistsQuery_SingleID() {
	query, args, err := buildBulkGroupExistsQuery([]string{"group-1"}, testDeploymentID)

	suite.NoError(err)
	suite.Equal("GRQ-GROUP_MGT-18", query.ID)
	suite.Equal(
		`SELECT ID FROM "GROUP" WHERE ID IN ($1) AND DEPLOYMENT_ID = $2`,
		query.PostgresQuery,
	)
	suite.Equal(
		`SELECT ID FROM "GROUP" WHERE ID IN ($1) AND DEPLOYMENT_ID = $2`,
		query.SQLiteQuery,
	)
	suite.Len(args, 2)
	suite.Equal("group-1", args[0])
	suite.Equal(testDeploymentID, args[1])
}

func (suite *StoreConstantsTestSuite) TestBuildBulkGroupExistsQuery_MultipleIDs() {
	query, args, err := buildBulkGroupExistsQuery(
		[]string{"group-1", "group-2", "group-3"}, testDeploymentID,
	)

	suite.NoError(err)
	suite.Equal("GRQ-GROUP_MGT-18", query.ID)
	suite.Equal(
		`SELECT ID FROM "GROUP" WHERE ID IN ($1,$2,$3) AND DEPLOYMENT_ID = $4`,
		query.PostgresQuery,
	)
	suite.Equal(
		`SELECT ID FROM "GROUP" WHERE ID IN ($1,$2,$3) AND DEPLOYMENT_ID = $4`,
		query.SQLiteQuery,
	)
	suite.Len(args, 4)
	suite.Equal("group-1", args[0])
	suite.Equal("group-2", args[1])
	suite.Equal("group-3", args[2])
	suite.Equal(testDeploymentID, args[3])
}

func TestBuildGetGroupListCountByOUIDsQuery(t *testing.T) {
	deploymentID := "dep1"
	testCases := []struct {
		name           string
		ouIDs          []string
		expectedPG     string
		expectedSQLite string
		expectedArgs   []interface{}
	}{
		{
			name:           "Empty list",
			ouIDs:          []string{},
			expectedPG:     "SELECT 0 WHERE 1=0",
			expectedSQLite: "SELECT 0 WHERE 1=0",
			expectedArgs:   []interface{}{},
		},
		{
			name:           "Single item",
			ouIDs:          []string{"ou1"},
			expectedPG:     `SELECT COUNT(*) as total FROM "GROUP" WHERE OU_ID IN ($1) AND DEPLOYMENT_ID = $2`,
			expectedSQLite: `SELECT COUNT(*) as total FROM "GROUP" WHERE OU_ID IN ($1) AND DEPLOYMENT_ID = $2`,
			expectedArgs:   []interface{}{"ou1", deploymentID},
		},
		{
			name:           "Multiple items",
			ouIDs:          []string{"ou1", "ou2", "ou3"},
			expectedPG:     `SELECT COUNT(*) as total FROM "GROUP" WHERE OU_ID IN ($1,$2,$3) AND DEPLOYMENT_ID = $4`,
			expectedSQLite: `SELECT COUNT(*) as total FROM "GROUP" WHERE OU_ID IN ($1,$2,$3) AND DEPLOYMENT_ID = $4`,
			expectedArgs:   []interface{}{"ou1", "ou2", "ou3", deploymentID},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, args := buildGetGroupsCountByOUIDsQuery(tc.ouIDs, deploymentID)
			require.Equal(t, tc.expectedPG, result.PostgresQuery)
			require.Equal(t, tc.expectedSQLite, result.SQLiteQuery)
			require.Equal(t, tc.expectedArgs, args)
		})
	}
}

func TestBuildGetGroupListByOUIDsQuery(t *testing.T) {
	limit, offset, deploymentID := 10, 5, "dep1"
	testCases := []struct {
		name           string
		ouIDs          []string
		expectedPG     string
		expectedSQLite string
		expectedArgs   []interface{}
	}{
		{
			name:           "Empty list",
			ouIDs:          []string{},
			expectedPG:     `SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" WHERE 1=0`,
			expectedSQLite: `SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" WHERE 1=0`,
			expectedArgs:   []interface{}{},
		},
		{
			name:  "Single item",
			ouIDs: []string{"ou1"},
			expectedPG: `SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" ` +
				`WHERE OU_ID IN ($1) AND DEPLOYMENT_ID = $2 ORDER BY NAME LIMIT $3 OFFSET $4`,
			expectedSQLite: `SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" ` +
				`WHERE OU_ID IN ($1) AND DEPLOYMENT_ID = $2 ORDER BY NAME LIMIT $3 OFFSET $4`,
			expectedArgs: []interface{}{"ou1", deploymentID, limit, offset},
		},
		{
			name:  "Multiple items",
			ouIDs: []string{"ou1", "ou2", "ou3"},
			expectedPG: `SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" ` +
				`WHERE OU_ID IN ($1,$2,$3) AND DEPLOYMENT_ID = $4 ORDER BY NAME LIMIT $5 OFFSET $6`,
			expectedSQLite: `SELECT ID, OU_ID, NAME, DESCRIPTION FROM "GROUP" ` +
				`WHERE OU_ID IN ($1,$2,$3) AND DEPLOYMENT_ID = $4 ORDER BY NAME LIMIT $5 OFFSET $6`,
			expectedArgs: []interface{}{"ou1", "ou2", "ou3", deploymentID, limit, offset},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, args := buildGetGroupsByOUIDsQuery(tc.ouIDs, limit, offset, deploymentID)
			require.Equal(t, tc.expectedPG, result.PostgresQuery)
			require.Equal(t, tc.expectedSQLite, result.SQLiteQuery)
			require.Equal(t, tc.expectedArgs, args)
		})
	}
}
