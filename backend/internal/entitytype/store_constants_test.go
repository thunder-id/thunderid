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

package entitytype

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/system/database/model"
)

type buildQueryFunc func([]string) model.DBQuery

func runBuildEntityTypeQueryTests(t *testing.T, cases []struct {
	name       string
	ouIDs      []string
	wantPG     string
	wantSQLite string
}, fn buildQueryFunc, expectedID string) {
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			query := fn(tc.ouIDs)
			assert.Equal(t, expectedID, query.ID)
			assert.Equal(t, tc.wantPG, query.PostgresQuery)
			assert.Equal(t, tc.wantSQLite, query.SQLiteQuery)
		})
	}
}

func TestBuildGetEntityTypeListByOUIDsQuery(t *testing.T) {
	testCases := []struct {
		name       string
		ouIDs      []string
		wantPG     string
		wantSQLite string
	}{
		{
			name:  "Empty OUIDs",
			ouIDs: []string{},
			wantPG: `SELECT ID, CATEGORY, NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
				`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE 1=0 AND CATEGORY = $1 AND DEPLOYMENT_ID = $2 ORDER BY NAME LIMIT $3 OFFSET $4`,
			wantSQLite: `SELECT ID, CATEGORY, NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
				`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE 1=0 AND CATEGORY = ? AND DEPLOYMENT_ID = ? ORDER BY NAME LIMIT ? OFFSET ?`,
		},
		{
			name:  "Single OUID",
			ouIDs: []string{"ou-1"},
			wantPG: `SELECT ID, CATEGORY, NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
				`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE OU_ID IN ($1) AND CATEGORY = $2 AND DEPLOYMENT_ID = $3 ORDER BY NAME LIMIT $4 OFFSET $5`,
			wantSQLite: `SELECT ID, CATEGORY, NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
				`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE OU_ID IN (?) AND CATEGORY = ? AND DEPLOYMENT_ID = ? ORDER BY NAME LIMIT ? OFFSET ?`,
		},
		{
			name:  "Multiple OUIDs",
			ouIDs: []string{"ou-1", "ou-2", "ou-3"},
			wantPG: `SELECT ID, CATEGORY, NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
				`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE OU_ID IN ($1, $2, $3) AND CATEGORY = $4 AND DEPLOYMENT_ID = $5 ORDER BY NAME LIMIT $6 OFFSET $7`,
			wantSQLite: `SELECT ID, CATEGORY, NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
				`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE OU_ID IN (?, ?, ?) AND CATEGORY = ? AND DEPLOYMENT_ID = ? ORDER BY NAME LIMIT ? OFFSET ?`,
		},
	}
	runBuildEntityTypeQueryTests(t, testCases, buildGetEntityTypeListByOUIDsQuery, "ASQ-ENTITY_TYPE-008")
}

func TestBuildGetEntityTypeCountByOUIDsQuery(t *testing.T) {
	testCases := []struct {
		name       string
		ouIDs      []string
		wantPG     string
		wantSQLite string
	}{
		{
			name:  "Empty OUIDs",
			ouIDs: []string{},
			wantPG: `SELECT COUNT(*) AS total FROM "ENTITY_TYPES" ` +
				`WHERE 1=0 AND CATEGORY = $1 AND DEPLOYMENT_ID = $2`,
			wantSQLite: `SELECT COUNT(*) AS total FROM "ENTITY_TYPES" ` +
				`WHERE 1=0 AND CATEGORY = ? AND DEPLOYMENT_ID = ?`,
		},
		{
			name:  "Single OUID",
			ouIDs: []string{"ou-1"},
			wantPG: `SELECT COUNT(*) AS total FROM "ENTITY_TYPES" ` +
				`WHERE OU_ID IN ($1) AND CATEGORY = $2 AND DEPLOYMENT_ID = $3`,
			wantSQLite: `SELECT COUNT(*) AS total FROM "ENTITY_TYPES" ` +
				`WHERE OU_ID IN (?) AND CATEGORY = ? AND DEPLOYMENT_ID = ?`,
		},
		{
			name:  "Multiple OUIDs",
			ouIDs: []string{"ou-1", "ou-2", "ou-3"},
			wantPG: `SELECT COUNT(*) AS total FROM "ENTITY_TYPES" ` +
				`WHERE OU_ID IN ($1, $2, $3) AND CATEGORY = $4 AND DEPLOYMENT_ID = $5`,
			wantSQLite: `SELECT COUNT(*) AS total FROM "ENTITY_TYPES" ` +
				`WHERE OU_ID IN (?, ?, ?) AND CATEGORY = ? AND DEPLOYMENT_ID = ?`,
		},
	}
	runBuildEntityTypeQueryTests(t, testCases, buildGetEntityTypeCountByOUIDsQuery, "ASQ-ENTITY_TYPE-009")
}

func TestBuildGetDisplayAttributesByNamesQuery(t *testing.T) {
	testCases := []struct {
		name       string
		ouIDs      []string
		wantPG     string
		wantSQLite string
	}{
		{
			name:  "Empty names",
			ouIDs: []string{},
			wantPG: `SELECT NAME, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" WHERE 1=0 ` +
				`AND CATEGORY = $1 AND DEPLOYMENT_ID = $2`,
			wantSQLite: `SELECT NAME, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" WHERE 1=0 ` +
				`AND CATEGORY = ? AND DEPLOYMENT_ID = ?`,
		},
		{
			name:  "Single name",
			ouIDs: []string{"SchemaA"},
			wantPG: `SELECT NAME, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE NAME IN ($1) AND CATEGORY = $2 AND DEPLOYMENT_ID = $3`,
			wantSQLite: `SELECT NAME, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE NAME IN (?) AND CATEGORY = ? AND DEPLOYMENT_ID = ?`,
		},
		{
			name:  "Multiple names",
			ouIDs: []string{"SchemaA", "SchemaB", "SchemaC"},
			wantPG: `SELECT NAME, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE NAME IN ($1, $2, $3) AND CATEGORY = $4 AND DEPLOYMENT_ID = $5`,
			wantSQLite: `SELECT NAME, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE NAME IN (?, ?, ?) AND CATEGORY = ? AND DEPLOYMENT_ID = ?`,
		},
	}
	runBuildEntityTypeQueryTests(t, testCases, buildGetDisplayAttributesByNamesQuery, "ASQ-ENTITY_TYPE-010")
}
