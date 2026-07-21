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

package dbstore

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

// columnNameValue is the lowercased result-set key for the VALUE column.
const columnNameValue = "value"

// queryPutRuntimeStore upserts an entry, overwriting the value and resetting the TTL on conflict.
var queryPutRuntimeStore = dbmodel.DBQuery{
	ID: "RTS-01",
	Query: `INSERT INTO "RUNTIME_STORE" (DEPLOYMENT_ID, NAMESPACE, KEY, VALUE, EXPIRY_TIME) ` +
		`VALUES ($1, $2, $3, $4, $5) ` +
		`ON CONFLICT (DEPLOYMENT_ID, NAMESPACE, KEY) ` +
		`DO UPDATE SET VALUE = EXCLUDED.VALUE, EXPIRY_TIME = EXCLUDED.EXPIRY_TIME, UPDATED_AT = CURRENT_TIMESTAMP`,
}

// queryGetRuntimeStore fetches a non-expired value.
var queryGetRuntimeStore = dbmodel.DBQuery{
	ID: "RTS-02",
	Query: `SELECT VALUE FROM "RUNTIME_STORE" ` +
		`WHERE DEPLOYMENT_ID = $1 AND NAMESPACE = $2 AND KEY = $3 ` +
		`AND (EXPIRY_TIME IS NULL OR EXPIRY_TIME > $4)`,
}

// queryUpdateRuntimeStore replaces the value of an existing, non-expired entry, preserving its TTL.
var queryUpdateRuntimeStore = dbmodel.DBQuery{
	ID: "RTS-03",
	Query: `UPDATE "RUNTIME_STORE" SET VALUE = $4, UPDATED_AT = CURRENT_TIMESTAMP ` +
		`WHERE DEPLOYMENT_ID = $1 AND NAMESPACE = $2 AND KEY = $3 ` +
		`AND (EXPIRY_TIME IS NULL OR EXPIRY_TIME > $5)`,
}

// queryDeleteRuntimeStore removes an entry. Used by Delete.
var queryDeleteRuntimeStore = dbmodel.DBQuery{
	ID:    "RTS-04",
	Query: `DELETE FROM "RUNTIME_STORE" WHERE DEPLOYMENT_ID = $1 AND NAMESPACE = $2 AND KEY = $3`,
}

// queryTakeRuntimeStore atomically deletes a non-expired entry and returns its value in a single
// statement, so a concurrent writer cannot slip a new value in between the read and the delete.
var queryTakeRuntimeStore = dbmodel.DBQuery{
	ID: "RTS-05",
	Query: `DELETE FROM "RUNTIME_STORE" ` +
		`WHERE DEPLOYMENT_ID = $1 AND NAMESPACE = $2 AND KEY = $3 ` +
		`AND (EXPIRY_TIME IS NULL OR EXPIRY_TIME > $4) ` +
		`RETURNING VALUE`,
}

// queryExtendTTLRuntimeStore extends the TTL of an existing, non-expired entry.
var queryExtendTTLRuntimeStore = dbmodel.DBQuery{
	ID: "RTS-06",
	Query: `UPDATE "RUNTIME_STORE" SET EXPIRY_TIME = $4, UPDATED_AT = CURRENT_TIMESTAMP ` +
		`WHERE DEPLOYMENT_ID = $1 AND NAMESPACE = $2 AND KEY = $3 ` +
		`AND (EXPIRY_TIME IS NULL OR EXPIRY_TIME > $5)`,
}
