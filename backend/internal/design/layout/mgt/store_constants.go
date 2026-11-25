// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package layoutmgt

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

var (
	// queryCreateLayout creates a new layout configuration.
	queryCreateLayout = dbmodel.DBQuery{
		ID: "LAQ-LAYOUT_MGT-01",
		Query: `INSERT INTO "LAYOUT" (ID, HANDLE, DISPLAY_NAME, DESCRIPTION, LAYOUT, PROJECT_ID, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7)`,
	}

	// queryGetLayoutByID retrieves a layout configuration by ID.
	queryGetLayoutByID = dbmodel.DBQuery{
		ID: "LAQ-LAYOUT_MGT-02",
		Query: `SELECT ID, HANDLE, DISPLAY_NAME, DESCRIPTION, PROJECT_ID, LAYOUT, CREATED_AT, UPDATED_AT ` +
			`FROM "LAYOUT" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetLayoutList retrieves a list of layout configurations with pagination.
	queryGetLayoutList = dbmodel.DBQuery{
		ID: "LAQ-LAYOUT_MGT-03",
		Query: `SELECT ID, HANDLE, DISPLAY_NAME, DESCRIPTION, PROJECT_ID, CREATED_AT, UPDATED_AT FROM "LAYOUT" ` +
			`WHERE DEPLOYMENT_ID = $3 ORDER BY CREATED_AT DESC LIMIT $1 OFFSET $2`,
	}

	// queryGetLayoutListCount retrieves the total count of layout configurations.
	queryGetLayoutListCount = dbmodel.DBQuery{
		ID:    "LAQ-LAYOUT_MGT-04",
		Query: `SELECT COUNT(*) as total FROM "LAYOUT" WHERE DEPLOYMENT_ID = $1`,
	}

	// queryUpdateLayout updates a layout configuration.
	queryUpdateLayout = dbmodel.DBQuery{
		ID: "LAQ-LAYOUT_MGT-05",
		PostgresQuery: `UPDATE "LAYOUT" SET DISPLAY_NAME = $1, DESCRIPTION = $2, LAYOUT = $3, PROJECT_ID = $4, ` +
			`UPDATED_AT = NOW() WHERE ID = $5 AND DEPLOYMENT_ID = $6`,
		SQLiteQuery: `UPDATE "LAYOUT" SET DISPLAY_NAME = $1, DESCRIPTION = $2, LAYOUT = $3, PROJECT_ID = $4, ` +
			`UPDATED_AT = datetime('now') WHERE ID = $5 AND DEPLOYMENT_ID = $6`,
		Query: `UPDATE "LAYOUT" SET DISPLAY_NAME = $1, DESCRIPTION = $2, LAYOUT = $3, PROJECT_ID = $4, ` +
			`UPDATED_AT = datetime('now') WHERE ID = $5 AND DEPLOYMENT_ID = $6`,
	}

	// queryDeleteLayout deletes a layout configuration.
	queryDeleteLayout = dbmodel.DBQuery{
		ID:    "LAQ-LAYOUT_MGT-06",
		Query: `DELETE FROM "LAYOUT" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCheckLayoutExists checks if a layout exists.
	queryCheckLayoutExists = dbmodel.DBQuery{
		ID:    "LAQ-LAYOUT_MGT-07",
		Query: `SELECT COUNT(*) as total FROM "LAYOUT" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCheckLayoutHandleConflict checks if a layout handle already exists for a deployment (excluding a given ID).
	queryCheckLayoutHandleConflict = dbmodel.DBQuery{
		ID:    "LAQ-LAYOUT_MGT-09",
		Query: `SELECT COUNT(*) as total FROM "LAYOUT" WHERE HANDLE = $1 AND DEPLOYMENT_ID = $2 AND ID != $3`,
	}

	// queryGetLayoutListByProject retrieves a page of the layouts of a project.
	queryGetLayoutListByProject = dbmodel.DBQuery{
		ID: "LAQ-LAYOUT_MGT-10",
		Query: `SELECT ID, HANDLE, DISPLAY_NAME, DESCRIPTION, PROJECT_ID, CREATED_AT, UPDATED_AT FROM "LAYOUT" ` +
			`WHERE PROJECT_ID = $1 AND DEPLOYMENT_ID = $4 ORDER BY CREATED_AT DESC LIMIT $2 OFFSET $3`,
	}

	// queryGetLayoutListCountByProject retrieves the number of layouts of a project.
	queryGetLayoutListCountByProject = dbmodel.DBQuery{
		ID:    "LAQ-LAYOUT_MGT-11",
		Query: `SELECT COUNT(*) as total FROM "LAYOUT" WHERE PROJECT_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)
