// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
			wantPG: `SELECT ID, CATEGORY, HANDLE, DISPLAY_NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
				`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE 1=0 AND CATEGORY = $1 AND DEPLOYMENT_ID = $2 ORDER BY DISPLAY_NAME, HANDLE LIMIT $3 OFFSET $4`,
			wantSQLite: `SELECT ID, CATEGORY, HANDLE, DISPLAY_NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
				`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE 1=0 AND CATEGORY = ? AND DEPLOYMENT_ID = ? ORDER BY DISPLAY_NAME, HANDLE LIMIT ? OFFSET ?`,
		},
		{
			name:  "Single OUID",
			ouIDs: []string{"ou-1"},
			wantPG: `SELECT ID, CATEGORY, HANDLE, DISPLAY_NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
				`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE OU_ID IN ($1) AND CATEGORY = $2 AND DEPLOYMENT_ID = $3 ` +
				`ORDER BY DISPLAY_NAME, HANDLE LIMIT $4 OFFSET $5`,
			wantSQLite: `SELECT ID, CATEGORY, HANDLE, DISPLAY_NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
				`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE OU_ID IN (?) AND CATEGORY = ? AND DEPLOYMENT_ID = ? ` +
				`ORDER BY DISPLAY_NAME, HANDLE LIMIT ? OFFSET ?`,
		},
		{
			name:  "Multiple OUIDs",
			ouIDs: []string{"ou-1", "ou-2", "ou-3"},
			wantPG: `SELECT ID, CATEGORY, HANDLE, DISPLAY_NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
				`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE OU_ID IN ($1, $2, $3) AND CATEGORY = $4 AND DEPLOYMENT_ID = $5 ` +
				`ORDER BY DISPLAY_NAME, HANDLE LIMIT $6 OFFSET $7`,
			wantSQLite: `SELECT ID, CATEGORY, HANDLE, DISPLAY_NAME, OU_ID, ALLOW_SELF_REGISTRATION, ` +
				`SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE OU_ID IN (?, ?, ?) AND CATEGORY = ? AND DEPLOYMENT_ID = ? ` +
				`ORDER BY DISPLAY_NAME, HANDLE LIMIT ? OFFSET ?`,
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

func TestBuildGetDisplayAttributesByHandlesQuery(t *testing.T) {
	testCases := []struct {
		name       string
		ouIDs      []string
		wantPG     string
		wantSQLite string
	}{
		{
			name:  "Empty handles",
			ouIDs: []string{},
			wantPG: `SELECT HANDLE, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" WHERE 1=0 ` +
				`AND CATEGORY = $1 AND DEPLOYMENT_ID = $2`,
			wantSQLite: `SELECT HANDLE, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" WHERE 1=0 ` +
				`AND CATEGORY = ? AND DEPLOYMENT_ID = ?`,
		},
		{
			name:  "Single handle",
			ouIDs: []string{"schema-a"},
			wantPG: `SELECT HANDLE, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE HANDLE IN ($1) AND CATEGORY = $2 AND DEPLOYMENT_ID = $3`,
			wantSQLite: `SELECT HANDLE, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE HANDLE IN (?) AND CATEGORY = ? AND DEPLOYMENT_ID = ?`,
		},
		{
			name:  "Multiple handles",
			ouIDs: []string{"schema-a", "schema-b", "schema-c"},
			wantPG: `SELECT HANDLE, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE HANDLE IN ($1, $2, $3) AND CATEGORY = $4 AND DEPLOYMENT_ID = $5`,
			wantSQLite: `SELECT HANDLE, SYSTEM_ATTRIBUTES FROM "ENTITY_TYPES" ` +
				`WHERE HANDLE IN (?, ?, ?) AND CATEGORY = ? AND DEPLOYMENT_ID = ?`,
		},
	}
	runBuildEntityTypeQueryTests(t, testCases, buildGetDisplayAttributesByHandlesQuery, "ASQ-ENTITY_TYPE-010")
}
