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

package jti

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

// queryInsertJTI inserts a JTI into the replay cache; RowsAffected==0 indicates a replay.
var queryInsertJTI = dbmodel.DBQuery{
	ID: "JTQ-JRS-01",
	Query: `INSERT INTO "JTI_RECORD" (NAMESPACE, JTI, EXPIRY_TIME, DEPLOYMENT_ID) ` +
		`VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
	SQLiteQuery: `INSERT OR IGNORE INTO "JTI_RECORD" (NAMESPACE, JTI, EXPIRY_TIME, DEPLOYMENT_ID) ` +
		`VALUES ($1, $2, $3, $4)`,
}
