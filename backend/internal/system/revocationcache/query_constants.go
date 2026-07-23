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

package revocationcache

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

// querySnapshotRevokedTokens reads the full set of non-expired single-token deny-list entries for this
// deployment. It is read-only: this package holds no insert/update/delete query against the deny list.
var querySnapshotRevokedTokens = dbmodel.DBQuery{
	ID:    "RVC-SRC-01",
	Query: `SELECT JTI, EXPIRY_TIME FROM "REVOKED_TOKEN" WHERE EXPIRY_TIME > $1 AND DEPLOYMENT_ID = $2`,
}

// querySnapshotRevokedTokenFamilies reads the full set of non-expired token-family revocation entries for
// this deployment from the criteria deny list. It is read-only.
var querySnapshotRevokedTokenFamilies = dbmodel.DBQuery{
	ID: "RVC-SRC-02",
	Query: `SELECT CRITERION_VALUE, EXPIRY_TIME FROM "REVOCATION_CRITERIA" ` +
		`WHERE CRITERION_TYPE = $1 AND EXPIRY_TIME > $2 AND DEPLOYMENT_ID = $3`,
}
