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
	// queryEnqueueJob records work for a Data Plane, pending delivery.
	queryEnqueueJob = dbmodel.DBQuery{
		ID: "EMQ-JOB-001",
		Query: `INSERT INTO "DATA_PLANE_JOB" ` +
			`(DEPLOYMENT_ID, ID, DATA_PLANE_ID, ENV_ID, TYPE, PAYLOAD, ENCRYPTED, STATUS) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
	}

	// queryGetJob reads one job, which is how a caller collects the answer it was given an id for.
	queryGetJob = dbmodel.DBQuery{
		ID: "EMQ-JOB-002",
		Query: `SELECT ID, DATA_PLANE_ID, ENV_ID, TYPE, STATUS, RESULT, ERROR, ATTEMPTS ` +
			`FROM "DATA_PLANE_JOB" WHERE DEPLOYMENT_ID = $1 AND ID = $2`,
	}

	// queryNextPendingJob finds the work a Data Plane should be given next.
	//
	// The oldest pending row wins, and only when that Data Plane has nothing in flight: two applies
	// delivered out of order would leave it matching neither version. Claiming is completed by
	// queryClaimJob, which is what settles a race between pods.
	queryNextPendingJob = dbmodel.DBQuery{
		ID: "EMQ-JOB-003",
		Query: `SELECT DEPLOYMENT_ID, ID, ENV_ID, TYPE, PAYLOAD, ENCRYPTED ` +
			`FROM "DATA_PLANE_JOB" WHERE DATA_PLANE_ID = $1 AND STATUS = 'pending' ` +
			`AND NOT EXISTS (SELECT 1 FROM "DATA_PLANE_JOB" inflight ` +
			`WHERE inflight.DATA_PLANE_ID = $1 AND inflight.STATUS = 'claimed') ` +
			`ORDER BY CREATED_AT ASC, ID ASC LIMIT 1`,
	}

	// queryClaimJob takes ownership of a pending job. The STATUS condition is what makes it safe for
	// several pods to try at once: the row moves out of pending exactly once, so only one claim
	// reports a row changed and the losers move on.
	queryClaimJob = dbmodel.DBQuery{
		ID: "EMQ-JOB-004",
		Query: `UPDATE "DATA_PLANE_JOB" SET STATUS = 'claimed', CLAIMED_BY = $3, ` +
			`ATTEMPTS = ATTEMPTS + 1, UPDATED_AT = CURRENT_TIMESTAMP ` +
			`WHERE DEPLOYMENT_ID = $1 AND ID = $2 AND STATUS = 'pending'`,
	}

	// queryCompleteJob records what the Data Plane answered.
	queryCompleteJob = dbmodel.DBQuery{
		ID: "EMQ-JOB-005",
		Query: `UPDATE "DATA_PLANE_JOB" SET STATUS = $3, RESULT = $4, ERROR = $5, ` +
			`UPDATED_AT = CURRENT_TIMESTAMP, COMPLETED_AT = CURRENT_TIMESTAMP ` +
			`WHERE DEPLOYMENT_ID = $1 AND ID = $2`,
	}

	// queryReleaseJob puts a claimed job back for another pod, for a delivery that could not be
	// attempted rather than one that failed. A failure is recorded; this is not one.
	queryReleaseJob = dbmodel.DBQuery{
		ID: "EMQ-JOB-006",
		Query: `UPDATE "DATA_PLANE_JOB" SET STATUS = 'pending', CLAIMED_BY = NULL, ` +
			`UPDATED_AT = CURRENT_TIMESTAMP WHERE DEPLOYMENT_ID = $1 AND ID = $2 AND STATUS = 'claimed'`,
	}
)
