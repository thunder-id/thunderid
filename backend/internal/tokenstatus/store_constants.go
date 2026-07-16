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

package tokenstatus

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

// querySelectActiveList fetches the deployment's active (unsealed) list to allocate the next index
// from. At most one active list is expected; ORDER BY keeps the choice deterministic if a creation
// race briefly leaves two.
var querySelectActiveList = dbmodel.DBQuery{
	ID: "SLQ-STS-01",
	Query: `SELECT ID, NEXT_IDX, CAPACITY FROM "STATUS_LIST" WHERE STATE = 0 AND DEPLOYMENT_ID = $1 ` +
		`ORDER BY CREATED_AT LIMIT 1`,
}

// queryInsertList creates a new active list with a zeroed allocator counter, but only when the
// deployment has no active list already (WHERE NOT EXISTS). This keeps a creation race from leaving two
// active lists: a loser inserts zero rows and simply re-selects the winner's list on the next attempt.
// DEPLOYMENT_ID is bound twice ($5, $6) to avoid relying on placeholder reuse across dialects.
var queryInsertList = dbmodel.DBQuery{
	ID: "SLQ-STS-02",
	Query: `INSERT INTO "STATUS_LIST" (ID, BITS, STATE, NEXT_IDX, CAPACITY, CREATED_AT, DEPLOYMENT_ID) ` +
		`SELECT $1, $2, 0, 0, $3, $4, $5 ` +
		`WHERE NOT EXISTS (SELECT 1 FROM "STATUS_LIST" WHERE DEPLOYMENT_ID = $6 AND STATE = 0)`,
}

// queryBumpNextIdx atomically claims one index via compare-and-swap: it succeeds (one row affected)
// only if NEXT_IDX still equals the value the caller read, so concurrent allocators never hand out the
// same index. A zero-row result means another node bumped first and the caller must retry.
var queryBumpNextIdx = dbmodel.DBQuery{
	ID: "SLQ-STS-03",
	Query: `UPDATE "STATUS_LIST" SET NEXT_IDX = NEXT_IDX + 1 ` +
		`WHERE ID = $1 AND STATE = 0 AND NEXT_IDX = $2 AND DEPLOYMENT_ID = $3`,
}

// querySealList marks a full active list sealed, stamping the retention clock. Guarded by STATE = 0 so
// only one racing allocator seals it.
var querySealList = dbmodel.DBQuery{
	ID: "SLQ-STS-04",
	Query: `UPDATE "STATUS_LIST" SET STATE = 1, SEALED_AT = $1 ` +
		`WHERE ID = $2 AND STATE = 0 AND DEPLOYMENT_ID = $3`,
}

// queryGetList loads a single list by id, used by the publish path to read its BITS and lifecycle.
var queryGetList = dbmodel.DBQuery{
	ID: "SLQ-STS-05",
	Query: `SELECT ID, BITS, STATE, NEXT_IDX, CAPACITY, CREATED_AT, SEALED_AT FROM "STATUS_LIST" ` +
		`WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
}

// queryUpsertEntry records (or updates) a token's status. Idempotent: re-revoking the same index is a
// no-op beyond refreshing the status and timestamp.
var queryUpsertEntry = dbmodel.DBQuery{
	ID: "SLQ-STS-06",
	Query: `INSERT INTO "STATUS_LIST_ENTRY" (LIST_ID, IDX, STATUS, EXPIRY_TIME, UPDATED_AT, ` +
		`DEPLOYMENT_ID) VALUES ($1, $2, $3, $4, $5, $6) ` +
		`ON CONFLICT (DEPLOYMENT_ID, LIST_ID, IDX) ` +
		`DO UPDATE SET STATUS = excluded.STATUS, UPDATED_AT = excluded.UPDATED_AT`,
}

// queryGetEntryStatus reads one token's status for AS-internal enforcement. Absence of a row means the
// token is VALID (the sparse table stores only non-VALID entries).
var queryGetEntryStatus = dbmodel.DBQuery{
	ID:    "SLQ-STS-07",
	Query: `SELECT STATUS FROM "STATUS_LIST_ENTRY" WHERE LIST_ID = $1 AND IDX = $2 AND DEPLOYMENT_ID = $3`,
}

// queryListEntries reads all non-VALID entries of a list to build the published bit array.
var queryListEntries = dbmodel.DBQuery{
	ID:    "SLQ-STS-08",
	Query: `SELECT IDX, STATUS FROM "STATUS_LIST_ENTRY" WHERE LIST_ID = $1 AND DEPLOYMENT_ID = $2`,
}

// queryDropExpiredSealedLists removes sealed lists whose retention has elapsed. On PostgreSQL the
// ON DELETE CASCADE foreign key drops their entries in the same statement.
var queryDropExpiredSealedLists = dbmodel.DBQuery{
	ID:    "SLQ-STS-09",
	Query: `DELETE FROM "STATUS_LIST" WHERE STATE = 1 AND SEALED_AT < $1 AND DEPLOYMENT_ID = $2`,
}
