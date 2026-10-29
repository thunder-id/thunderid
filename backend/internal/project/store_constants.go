// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

// DBQuery definitions for the project store.
var (
	queryCreateProject = dbmodel.DBQuery{
		ID:    "PRQ-PROJECT_MGT-01",
		Query: `INSERT INTO "PROJECT" (ID, ROOT_OU_ID, CREATED_AT, DEPLOYMENT_ID) VALUES ($1, $2, $3, $4)`,
	}
	queryGetProjectByID = dbmodel.DBQuery{
		ID:    "PRQ-PROJECT_MGT-02",
		Query: `SELECT ID, ROOT_OU_ID, CREATED_AT FROM "PROJECT" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryGetProjectList = dbmodel.DBQuery{
		ID: "PRQ-PROJECT_MGT-03",
		Query: `SELECT ID, ROOT_OU_ID, CREATED_AT FROM "PROJECT" ` +
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
	queryGetProjectByRootOUID = dbmodel.DBQuery{
		ID:    "PRQ-PROJECT_MGT-06",
		Query: `SELECT ID, ROOT_OU_ID, CREATED_AT FROM "PROJECT" WHERE ROOT_OU_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)
