// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

const (
	// primaryKeyName is the name PostgreSQL gives the table's primary key, and primaryKeyColumns its
	// columns as SQLite reports them.
	primaryKeyName    = "PROJECT_pkey"
	primaryKeyColumns = "PROJECT.DEPLOYMENT_ID, PROJECT.ID"
	// handleIndexName is the unique index on a project's handle, and handleIndexColumns its columns as
	// SQLite reports them.
	handleIndexName    = "idx_project_handle"
	handleIndexColumns = "PROJECT.DEPLOYMENT_ID, PROJECT.HANDLE"
)

// DBQuery definitions for the project store.
var (
	queryCreateProject = dbmodel.DBQuery{
		ID: "PRQ-PROJECT_MGT-01",
		Query: `INSERT INTO "PROJECT" (ID, HANDLE, DISPLAY_NAME, DESCRIPTION, CREATED_AT, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6)`,
	}
	queryGetProjectByID = dbmodel.DBQuery{
		ID: "PRQ-PROJECT_MGT-02",
		Query: `SELECT ID, HANDLE, DISPLAY_NAME, DESCRIPTION, CREATED_AT FROM "PROJECT" ` +
			`WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryGetProjectList = dbmodel.DBQuery{
		ID: "PRQ-PROJECT_MGT-03",
		Query: `SELECT ID, HANDLE, DISPLAY_NAME, DESCRIPTION, CREATED_AT FROM "PROJECT" ` +
			`WHERE DEPLOYMENT_ID = $3 ORDER BY CREATED_AT, ID LIMIT $1 OFFSET $2`,
	}
	queryGetProjectListCount = dbmodel.DBQuery{
		ID:    "PRQ-PROJECT_MGT-04",
		Query: `SELECT COUNT(*) AS total FROM "PROJECT" WHERE DEPLOYMENT_ID = $1`,
	}
	queryDeleteProject = dbmodel.DBQuery{
		ID:    "PRQ-PROJECT_MGT-05",
		Query: `DELETE FROM "PROJECT" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryGetProjectByHandle = dbmodel.DBQuery{
		ID: "PRQ-PROJECT_MGT-06",
		Query: `SELECT ID, HANDLE, DISPLAY_NAME, DESCRIPTION, CREATED_AT FROM "PROJECT" ` +
			`WHERE HANDLE = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryUpdateProject = dbmodel.DBQuery{
		ID:    "PRQ-PROJECT_MGT-07",
		Query: `UPDATE "PROJECT" SET DISPLAY_NAME = $2, DESCRIPTION = $3 WHERE ID = $1 AND DEPLOYMENT_ID = $4`,
	}
)
