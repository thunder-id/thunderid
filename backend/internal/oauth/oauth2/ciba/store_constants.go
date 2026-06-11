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

package ciba

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

// Database column names for CIBA authentication request storage.
const (
	dbColumnAuthReqID        = "auth_req_id"
	dbColumnClientID         = "client_id"
	dbColumnUserID           = "user_id"
	dbColumnStandardScopes   = "standard_scopes"
	dbColumnAuthorizedScopes = "authorized_scopes"
	dbColumnState            = "state"
	dbColumnAttributeCacheID = "attribute_cache_id"
	dbColumnCompletedACR     = "completed_acr"
	dbColumnAuthTime         = "auth_time"
	dbColumnLastPolledAt     = "last_polled_at"
	dbColumnExpiryTime       = "expiry_time"
)

// queryInsertCIBAAuthRequest inserts a new CIBA authentication request.
// USER_ID is omitted — it is unknown at creation and populated at callback via MarkAuthenticated.
var queryInsertCIBAAuthRequest = dbmodel.DBQuery{
	ID: "CBQ-CRS-01",
	Query: `INSERT INTO "CIBA_AUTH_REQUEST" (AUTH_REQ_ID, CLIENT_ID, STANDARD_SCOPES, STATE, ` +
		`EXPIRY_TIME, DEPLOYMENT_ID) VALUES ($1, $2, $3, $4, $5, $6)`,
}

// queryGetCIBAAuthRequest retrieves a CIBA authentication request by ID.
var queryGetCIBAAuthRequest = dbmodel.DBQuery{
	ID: "CBQ-CRS-02",
	Query: `SELECT AUTH_REQ_ID, CLIENT_ID, USER_ID, STANDARD_SCOPES, AUTHORIZED_SCOPES, STATE, ` +
		`ATTRIBUTE_CACHE_ID, COMPLETED_ACR, AUTH_TIME, LAST_POLLED_AT, EXPIRY_TIME ` +
		`FROM "CIBA_AUTH_REQUEST" WHERE AUTH_REQ_ID = $1 AND DEPLOYMENT_ID = $2`,
}

// queryMarkCIBAAuthRequestAuthenticated transitions a pending request to authenticated and
// records the user ID, authorized scopes, attribute cache ID, completed ACR, and authentication
// time. AUTHORIZED_SCOPES stores the intersection of requested and user-permitted scopes as
// resolved by the AuthorizationExecutor — mirroring how auth code filters permission scopes.
// The WHERE STATE = 'PENDING' guard prevents a double-callback race.
var queryMarkCIBAAuthRequestAuthenticated = dbmodel.DBQuery{
	ID: "CBQ-CRS-03",
	Query: `UPDATE "CIBA_AUTH_REQUEST" SET STATE = $1, USER_ID = $2, AUTHORIZED_SCOPES = $3, ` +
		`ATTRIBUTE_CACHE_ID = $4, COMPLETED_ACR = $5, AUTH_TIME = $6 ` +
		`WHERE AUTH_REQ_ID = $7 AND STATE = $8 AND DEPLOYMENT_ID = $9`,
}

// queryUpdateCIBAAuthRequestState updates the state of a CIBA authentication request.
var queryUpdateCIBAAuthRequestState = dbmodel.DBQuery{
	ID:    "CBQ-CRS-04",
	Query: `UPDATE "CIBA_AUTH_REQUEST" SET STATE = $1 WHERE AUTH_REQ_ID = $2 AND DEPLOYMENT_ID = $3`,
}

// queryConsumeCIBAAuthRequest atomically transitions an authenticated request to consumed
// (AUTHENTICATED → CONSUMED). Returns zero rows affected when the request was already consumed,
// enabling one-time-use enforcement without a separate read.
var queryConsumeCIBAAuthRequest = dbmodel.DBQuery{
	ID: "CBQ-CRS-05",
	Query: `UPDATE "CIBA_AUTH_REQUEST" SET STATE = $1 WHERE AUTH_REQ_ID = $2 AND STATE = $3 ` +
		`AND DEPLOYMENT_ID = $4`,
}

// queryUpdateCIBALastPolled updates the last polled timestamp of a CIBA authentication request.
var queryUpdateCIBALastPolled = dbmodel.DBQuery{
	ID:    "CBQ-CRS-06",
	Query: `UPDATE "CIBA_AUTH_REQUEST" SET LAST_POLLED_AT = $1 WHERE AUTH_REQ_ID = $2 AND DEPLOYMENT_ID = $3`,
}
