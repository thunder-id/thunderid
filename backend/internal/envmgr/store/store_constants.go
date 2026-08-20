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

package store

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// querySaveEnvironment inserts an environment, or replaces the document of one already there.
	querySaveEnvironment = dbmodel.DBQuery{
		ID: "EMQ-ENV-001",
		Query: `INSERT INTO "ENVIRONMENT" (DEPLOYMENT_ID, ID, DATA) VALUES ($1, $2, $3) ` +
			`ON CONFLICT (DEPLOYMENT_ID, ID) ` +
			`DO UPDATE SET DATA = EXCLUDED.DATA, UPDATED_AT = CURRENT_TIMESTAMP`,
	}

	// queryListDeployments retrieves every deployment that has an environment. It is the one query
	// that is not scoped to a deployment: seeding a new tenant has to find which organization's chain
	// already manages the tenant it is copied from.
	queryListDeployments = dbmodel.DBQuery{
		ID:    "EMQ-ENV-010",
		Query: `SELECT DISTINCT DEPLOYMENT_ID FROM "ENVIRONMENT" ORDER BY DEPLOYMENT_ID`,
	}

	// queryGetEnvironment retrieves a single environment document.
	queryGetEnvironment = dbmodel.DBQuery{
		ID:    "EMQ-ENV-002",
		Query: `SELECT DATA FROM "ENVIRONMENT" WHERE DEPLOYMENT_ID = $1 AND ID = $2`,
	}

	// queryListEnvironments retrieves every environment document for the deployment. Ordering is
	// resolved in the server, which sorts by rank and then name, neither of which is a column.
	queryListEnvironments = dbmodel.DBQuery{
		ID:    "EMQ-ENV-003",
		Query: `SELECT DATA FROM "ENVIRONMENT" WHERE DEPLOYMENT_ID = $1`,
	}

	// queryDeleteEnvironment removes an environment. Its versions go with it, through the foreign
	// key that cascades.
	queryDeleteEnvironment = dbmodel.DBQuery{
		ID:    "EMQ-ENV-004",
		Query: `DELETE FROM "ENVIRONMENT" WHERE DEPLOYMENT_ID = $1 AND ID = $2`,
	}

	// queryInsertVersion stores one captured version at an already-assigned sequence.
	queryInsertVersion = dbmodel.DBQuery{
		ID: "EMQ-ENV-005",
		Query: `INSERT INTO "ENVIRONMENT_VERSION" (DEPLOYMENT_ID, ENV_ID, SEQ, DATA) ` +
			`VALUES ($1, $2, $3, $4)`,
	}

	// queryGetVersion retrieves one version document.
	queryGetVersion = dbmodel.DBQuery{
		ID: "EMQ-ENV-006",
		Query: `SELECT DATA FROM "ENVIRONMENT_VERSION" ` +
			`WHERE DEPLOYMENT_ID = $1 AND ENV_ID = $2 AND SEQ = $3`,
	}

	// queryListVersions retrieves an environment's versions, newest first.
	queryListVersions = dbmodel.DBQuery{
		ID: "EMQ-ENV-007",
		Query: `SELECT SEQ, DATA FROM "ENVIRONMENT_VERSION" ` +
			`WHERE DEPLOYMENT_ID = $1 AND ENV_ID = $2 ORDER BY SEQ DESC`,
	}

	// queryVersionSeqs retrieves an environment's version sequences, oldest first. Used to assign the
	// next sequence and to decide what pruning removes.
	queryVersionSeqs = dbmodel.DBQuery{
		ID: "EMQ-ENV-008",
		Query: `SELECT SEQ FROM "ENVIRONMENT_VERSION" ` +
			`WHERE DEPLOYMENT_ID = $1 AND ENV_ID = $2 ORDER BY SEQ ASC`,
	}

	// queryDeleteVersion removes a single pruned version.
	queryDeleteVersion = dbmodel.DBQuery{
		ID: "EMQ-ENV-009",
		Query: `DELETE FROM "ENVIRONMENT_VERSION" ` +
			`WHERE DEPLOYMENT_ID = $1 AND ENV_ID = $2 AND SEQ = $3`,
	}
)
