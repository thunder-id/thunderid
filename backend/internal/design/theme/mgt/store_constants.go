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

package thememgt

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

var (
	// queryCreateTheme creates a new theme configuration.
	queryCreateTheme = dbmodel.DBQuery{
		ID: "THQ-THEME_MGT-01",
		Query: `INSERT INTO "THEME" (ID, HANDLE, DISPLAY_NAME, DESCRIPTION, THEME, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6)`,
	}

	// queryGetThemeByID retrieves a theme configuration by ID.
	queryGetThemeByID = dbmodel.DBQuery{
		ID: "THQ-THEME_MGT-02",
		Query: `SELECT ID, HANDLE, DISPLAY_NAME, DESCRIPTION, THEME, CREATED_AT, UPDATED_AT FROM "THEME" ` +
			`WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetThemeList retrieves a list of theme configurations with pagination.
	queryGetThemeList = dbmodel.DBQuery{
		ID: "THQ-THEME_MGT-03",
		Query: `SELECT ID, HANDLE, DISPLAY_NAME, DESCRIPTION, THEME, CREATED_AT, UPDATED_AT FROM "THEME" ` +
			`WHERE DEPLOYMENT_ID = $3 ORDER BY CREATED_AT DESC LIMIT $1 OFFSET $2`,
	}

	// queryGetThemeListCount retrieves the total count of theme configurations.
	queryGetThemeListCount = dbmodel.DBQuery{
		ID:    "THQ-THEME_MGT-04",
		Query: `SELECT COUNT(*) as total FROM "THEME" WHERE DEPLOYMENT_ID = $1`,
	}

	// queryUpdateTheme updates a theme configuration.
	queryUpdateTheme = dbmodel.DBQuery{
		ID: "THQ-THEME_MGT-05",
		PostgresQuery: `UPDATE "THEME" SET DISPLAY_NAME = $1, DESCRIPTION = $2, THEME = $3, ` +
			`UPDATED_AT = NOW() WHERE ID = $4 AND DEPLOYMENT_ID = $5`,
		SQLiteQuery: `UPDATE "THEME" SET DISPLAY_NAME = $1, DESCRIPTION = $2, THEME = $3, ` +
			`UPDATED_AT = datetime('now') WHERE ID = $4 AND DEPLOYMENT_ID = $5`,
		Query: `UPDATE "THEME" SET DISPLAY_NAME = $1, DESCRIPTION = $2, THEME = $3, ` +
			`UPDATED_AT = datetime('now') WHERE ID = $4 AND DEPLOYMENT_ID = $5`,
	}

	// queryDeleteTheme deletes a theme configuration.
	queryDeleteTheme = dbmodel.DBQuery{
		ID:    "THQ-THEME_MGT-06",
		Query: `DELETE FROM "THEME" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCheckThemeExists checks if a theme exists.
	queryCheckThemeExists = dbmodel.DBQuery{
		ID:    "THQ-THEME_MGT-07",
		Query: `SELECT COUNT(*) as total FROM "THEME" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetApplicationsCountByThemeID retrieves the count of inbound auth profiles using a theme.
	queryGetApplicationsCountByThemeID = dbmodel.DBQuery{
		ID:    "THQ-THEME_MGT-08",
		Query: `SELECT COUNT(*) as total FROM "INBOUND_CLIENT" WHERE THEME_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCheckThemeHandleConflict checks if a theme handle already exists for a deployment (excluding a given ID).
	queryCheckThemeHandleConflict = dbmodel.DBQuery{
		ID:    "THQ-THEME_MGT-09",
		Query: `SELECT COUNT(*) as total FROM "THEME" WHERE HANDLE = $1 AND DEPLOYMENT_ID = $2 AND ID != $3`,
	}
)
