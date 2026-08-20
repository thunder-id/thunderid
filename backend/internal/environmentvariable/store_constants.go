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

package environmentvariable

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// queryCreateEnvironmentVariable inserts a new environment variable.
	queryCreateEnvironmentVariable = dbmodel.DBQuery{
		ID: "EVQ-ENVVAR-001",
		Query: `INSERT INTO "ENVIRONMENT_VARIABLE" (ID, ENV_ID, KEY, VALUE, DESCRIPTION, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6)`,
	}

	// queryGetEnvironmentVariableCount retrieves the total count of an environment's variables.
	queryGetEnvironmentVariableCount = dbmodel.DBQuery{
		ID: "EVQ-ENVVAR-002",
		Query: `SELECT COUNT(*) AS total FROM "ENVIRONMENT_VARIABLE" ` +
			`WHERE ENV_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetEnvironmentVariableList retrieves a paginated list of environment variables.
	queryGetEnvironmentVariableList = dbmodel.DBQuery{
		ID: "EVQ-ENVVAR-003",
		Query: `SELECT ID, KEY, VALUE, DESCRIPTION, CREATED_AT, UPDATED_AT FROM "ENVIRONMENT_VARIABLE" ` +
			`WHERE ENV_ID = $3 AND DEPLOYMENT_ID = $4 ORDER BY KEY LIMIT $1 OFFSET $2`,
	}

	// queryGetEnvironmentVariableByID retrieves a single environment variable by id.
	queryGetEnvironmentVariableByID = dbmodel.DBQuery{
		ID: "EVQ-ENVVAR-004",
		Query: `SELECT ID, KEY, VALUE, DESCRIPTION, CREATED_AT, UPDATED_AT ` +
			`FROM "ENVIRONMENT_VARIABLE" WHERE ID = $1 AND ENV_ID = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryGetEnvironmentVariableByKey retrieves a single environment variable by key.
	queryGetEnvironmentVariableByKey = dbmodel.DBQuery{
		ID: "EVQ-ENVVAR-005",
		Query: `SELECT ID, KEY, VALUE, DESCRIPTION, CREATED_AT, UPDATED_AT ` +
			`FROM "ENVIRONMENT_VARIABLE" WHERE KEY = $1 AND ENV_ID = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryUpdateEnvironmentVariableByID updates an environment variable's description and value by id.
	queryUpdateEnvironmentVariableByID = dbmodel.DBQuery{
		ID: "EVQ-ENVVAR-006",
		Query: `UPDATE "ENVIRONMENT_VARIABLE" SET DESCRIPTION = $1, VALUE = $2, UPDATED_AT = NOW() ` +
			`WHERE ID = $3 AND ENV_ID = $4 AND DEPLOYMENT_ID = $5`,
		SQLiteQuery: `UPDATE "ENVIRONMENT_VARIABLE" SET DESCRIPTION = $1, VALUE = $2, ` +
			`UPDATED_AT = datetime('now') WHERE ID = $3 AND ENV_ID = $4 AND DEPLOYMENT_ID = $5`,
	}

	// queryDeleteEnvironmentVariableByID deletes an environment variable by id.
	queryDeleteEnvironmentVariableByID = dbmodel.DBQuery{
		ID:    "EVQ-ENVVAR-007",
		Query: `DELETE FROM "ENVIRONMENT_VARIABLE" WHERE ID = $1 AND ENV_ID = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryGetEnvironmentVariableValues retrieves every key and value of one environment. Used by the
	// resolve path that substitutes declarative placeholders at export/apply time.
	queryGetEnvironmentVariableValues = dbmodel.DBQuery{
		ID: "EVQ-ENVVAR-008",
		Query: `SELECT KEY, VALUE FROM "ENVIRONMENT_VARIABLE" WHERE ENV_ID = $1 AND DEPLOYMENT_ID = $2 ` +
			`ORDER BY KEY`,
	}
)
