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

package layoutmgt

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

var (
	// queryCreateLayout creates a new layout configuration.
	queryCreateLayout = dbmodel.DBQuery{
		ID: "LAQ-LAYOUT_MGT-01",
		Query: `INSERT INTO "LAYOUT" (ID, HANDLE, DISPLAY_NAME, DESCRIPTION, LAYOUT, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6)`,
	}

	// queryGetLayoutByID retrieves a layout configuration by ID.
	queryGetLayoutByID = dbmodel.DBQuery{
		ID: "LAQ-LAYOUT_MGT-02",
		Query: `SELECT ID, HANDLE, DISPLAY_NAME, DESCRIPTION, LAYOUT, CREATED_AT, UPDATED_AT FROM "LAYOUT" ` +
			`WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetLayoutList retrieves a list of layout configurations with pagination.
	queryGetLayoutList = dbmodel.DBQuery{
		ID: "LAQ-LAYOUT_MGT-03",
		Query: `SELECT ID, HANDLE, DISPLAY_NAME, DESCRIPTION, CREATED_AT, UPDATED_AT FROM "LAYOUT" ` +
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
		PostgresQuery: `UPDATE "LAYOUT" SET DISPLAY_NAME = $1, DESCRIPTION = $2, LAYOUT = $3, ` +
			`UPDATED_AT = NOW() WHERE ID = $4 AND DEPLOYMENT_ID = $5`,
		SQLiteQuery: `UPDATE "LAYOUT" SET DISPLAY_NAME = $1, DESCRIPTION = $2, LAYOUT = $3, ` +
			`UPDATED_AT = datetime('now') WHERE ID = $4 AND DEPLOYMENT_ID = $5`,
		Query: `UPDATE "LAYOUT" SET DISPLAY_NAME = $1, DESCRIPTION = $2, LAYOUT = $3, ` +
			`UPDATED_AT = datetime('now') WHERE ID = $4 AND DEPLOYMENT_ID = $5`,
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

	// queryGetApplicationsCountByLayoutID retrieves the count of inbound auth profiles using a layout.
	queryGetApplicationsCountByLayoutID = dbmodel.DBQuery{
		ID:    "LAQ-LAYOUT_MGT-08",
		Query: `SELECT COUNT(*) as total FROM "INBOUND_CLIENT" WHERE LAYOUT_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCheckLayoutHandleConflict checks if a layout handle already exists for a deployment (excluding a given ID).
	queryCheckLayoutHandleConflict = dbmodel.DBQuery{
		ID:    "LAQ-LAYOUT_MGT-09",
		Query: `SELECT COUNT(*) as total FROM "LAYOUT" WHERE HANDLE = $1 AND DEPLOYMENT_ID = $2 AND ID != $3`,
	}
)
