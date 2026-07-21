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

package revocation

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

// queryInsertRevokedToken inserts a JTI into the deny list. The write is idempotent: a duplicate
// (DEPLOYMENT_ID, JTI) is a no-op, enforced by the unique index backing the conflict target.
var queryInsertRevokedToken = dbmodel.DBQuery{
	ID: "RVQ-RTS-01",
	Query: `INSERT INTO "REVOKED_TOKEN" (ID, JTI, REVOCATION_REASON, REVOKED_AT, EXPIRY_TIME, ` +
		`DEPLOYMENT_ID) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (DEPLOYMENT_ID, JTI) DO NOTHING`,
}

// queryIsTokenRevoked checks whether a non-expired deny-list entry exists for the given JTI.
var queryIsTokenRevoked = dbmodel.DBQuery{
	ID:    "RVQ-RTS-02",
	Query: `SELECT 1 FROM "REVOKED_TOKEN" WHERE JTI = $1 AND EXPIRY_TIME > $2 AND DEPLOYMENT_ID = $3`,
}
