/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package flowmgt

import (
	"github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// queryCreateFlow is the query to creates a new flow definition.
	queryCreateFlow = model.DBQuery{
		ID: "FLQ-FLOW_MGT-01",
		Query: `INSERT INTO "FLOW" (ID, HANDLE, NAME, FLOW_TYPE, ACTIVE_VERSION, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6)`,
	}

	// queryGetFlow is the query to retrieves a flow definition by its ID.
	queryGetFlow = model.DBQuery{
		ID: "FLQ-FLOW_MGT-02",
		Query: `SELECT f.ID, f.HANDLE, f.NAME, f.FLOW_TYPE, f.ACTIVE_VERSION, fv.NODES, f.CREATED_AT, ` +
			`f.UPDATED_AT FROM "FLOW" f INNER JOIN "FLOW_VERSION" fv ON f.ID = fv.FLOW_ID ` +
			`AND f.DEPLOYMENT_ID = fv.DEPLOYMENT_ID AND f.ACTIVE_VERSION = fv.VERSION ` +
			`WHERE f.ID = $1 AND f.DEPLOYMENT_ID = $2`,
	}

	// queryUpdateFlow is the query to updates an existing flow definition.
	queryUpdateFlow = model.DBQuery{
		ID: "FLQ-FLOW_MGT-04",
		Query: `UPDATE "FLOW" SET NAME = $2, ACTIVE_VERSION = $3, ` +
			`UPDATED_AT = datetime('now') WHERE ID = $1 AND DEPLOYMENT_ID = $4`,
		SQLiteQuery: `UPDATE "FLOW" SET NAME = $2, ACTIVE_VERSION = $3, ` +
			`UPDATED_AT = datetime('now') WHERE ID = $1 AND DEPLOYMENT_ID = $4`,
		PostgresQuery: `UPDATE "FLOW" SET NAME = $2, ACTIVE_VERSION = $3, ` +
			`UPDATED_AT = CURRENT_TIMESTAMP WHERE ID = $1 AND DEPLOYMENT_ID = $4`,
	}

	// queryListFlows is the query to retrieves a list of flow definitions.
	queryListFlows = model.DBQuery{
		ID: "FLQ-FLOW_MGT-05",
		Query: `SELECT ID, HANDLE, NAME, FLOW_TYPE, ACTIVE_VERSION, CREATED_AT, UPDATED_AT ` +
			`FROM "FLOW" WHERE DEPLOYMENT_ID = $1 ORDER BY CREATED_AT DESC LIMIT $2 OFFSET $3`,
	}

	// queryListFlowsWithType is the query to retrieves a list of flow definitions filtered by type.
	queryListFlowsWithType = model.DBQuery{
		ID: "FLQ-FLOW_MGT-06",
		Query: `SELECT ID, HANDLE, NAME, FLOW_TYPE, ACTIVE_VERSION, CREATED_AT, UPDATED_AT FROM "FLOW" ` +
			`WHERE FLOW_TYPE = $1 AND DEPLOYMENT_ID = $2 ORDER BY CREATED_AT DESC LIMIT $3 OFFSET $4`,
	}

	// queryCountFlows is the query to count total flow definitions.
	queryCountFlows = model.DBQuery{
		ID:    "FLQ-FLOW_MGT-07",
		Query: `SELECT COUNT(*) AS count FROM "FLOW" WHERE DEPLOYMENT_ID = $1`,
	}

	// queryCountFlowsWithType is the query to count total flow definitions filtered by type.
	queryCountFlowsWithType = model.DBQuery{
		ID:    "FLQ-FLOW_MGT-08",
		Query: `SELECT COUNT(*) AS count FROM "FLOW" WHERE FLOW_TYPE = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryDeleteFlow is the query to delete a flow definition by its ID.
	queryDeleteFlow = model.DBQuery{
		ID:    "FLQ-FLOW_MGT-09",
		Query: `DELETE FROM "FLOW" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryInsertFlowVersion is the query to insert a new version of a flow.
	queryInsertFlowVersion = model.DBQuery{
		ID:    "FLQ-FLOW_MGT-10",
		Query: `INSERT INTO "FLOW_VERSION" (FLOW_ID, VERSION, NODES, DEPLOYMENT_ID) VALUES ($1, $2, $3, $4)`,
	}

	// queryGetFlowVersion is the query to retrieve a specific version of a flow.
	queryGetFlowVersion = model.DBQuery{
		ID: "FLQ-FLOW_MGT-11",
		Query: `SELECT VERSION, NODES, CREATED_AT FROM "FLOW_VERSION" WHERE ` +
			`FLOW_ID = $1 AND VERSION = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryGetFlowVersionWithMetadata is the query to retrieve a specific version with flow metadata.
	queryGetFlowVersionWithMetadata = model.DBQuery{
		ID: "FLQ-FLOW_MGT-12",
		Query: `SELECT f.ID, f.HANDLE, f.NAME, f.FLOW_TYPE, f.ACTIVE_VERSION, fv.VERSION, fv.NODES, ` +
			`fv.CREATED_AT FROM "FLOW" f INNER JOIN "FLOW_VERSION" fv ON f.ID = fv.FLOW_ID ` +
			`AND f.DEPLOYMENT_ID = fv.DEPLOYMENT_ID WHERE f.ID = $1 AND fv.VERSION = $2 ` +
			`AND f.DEPLOYMENT_ID = $3`,
	}

	// queryListFlowVersions is the query to list all versions of a flow.
	queryListFlowVersions = model.DBQuery{
		ID: "FLQ-FLOW_MGT-13",
		Query: `SELECT fv.VERSION, fv.CREATED_AT, f.ACTIVE_VERSION FROM "FLOW_VERSION" fv ` +
			`INNER JOIN "FLOW" f ON fv.FLOW_ID = f.ID AND fv.DEPLOYMENT_ID = f.DEPLOYMENT_ID ` +
			`WHERE fv.FLOW_ID = $1 AND fv.DEPLOYMENT_ID = $2 ` +
			`ORDER BY fv.VERSION DESC`,
	}

	// queryCountFlowVersions is the query to count total versions of a flow.
	queryCountFlowVersions = model.DBQuery{
		ID:    "FLQ-FLOW_MGT-14",
		Query: `SELECT COUNT(*) AS count FROM "FLOW_VERSION" WHERE FLOW_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryDeleteOldestVersion is the query to delete the oldest version of a flow.
	queryDeleteOldestVersion = model.DBQuery{
		ID: "FLQ-FLOW_MGT-15",
		Query: `DELETE FROM "FLOW_VERSION" WHERE FLOW_ID = $1 AND DEPLOYMENT_ID = $2 AND ` +
			`VERSION = (SELECT MIN(VERSION) FROM "FLOW_VERSION" WHERE FLOW_ID = $1 AND DEPLOYMENT_ID = $2)`,
	}

	// queryCheckFlowExistsByHandle is the query to check if a flow exists by handle and flow type.
	queryCheckFlowExistsByHandle = model.DBQuery{
		ID:    "FLQ-FLOW_MGT-17",
		Query: `SELECT 1 FROM "FLOW" WHERE HANDLE = $1 AND FLOW_TYPE = $2 AND DEPLOYMENT_ID = $3 LIMIT 1`,
	}

	// queryGetFlowByHandle retrieves a flow definition by handle and flow type.
	queryGetFlowByHandle = model.DBQuery{
		ID: "FLQ-FLOW_MGT-18",
		Query: `SELECT f.ID, f.HANDLE, f.NAME, f.FLOW_TYPE, f.ACTIVE_VERSION, fv.NODES, f.CREATED_AT, ` +
			`f.UPDATED_AT FROM "FLOW" f INNER JOIN "FLOW_VERSION" fv ON f.ID = fv.FLOW_ID ` +
			`AND f.DEPLOYMENT_ID = fv.DEPLOYMENT_ID AND f.ACTIVE_VERSION = fv.VERSION ` +
			`WHERE f.HANDLE = $1 AND f.FLOW_TYPE = $2 AND f.DEPLOYMENT_ID = $3`,
	}
)
