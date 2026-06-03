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
	dbColumnExecutionID      = "execution_id"
	dbColumnClientID         = "client_id"
	dbColumnUserID           = "user_id"
	dbColumnScopes           = "scopes"
	dbColumnState            = "state"
	dbColumnAttributeCacheID = "attribute_cache_id"
	dbColumnCompletedACR     = "completed_acr"
	dbColumnAuthTime         = "auth_time"
	dbColumnLastPolledAt     = "last_polled_at"
	dbColumnExpiryTime       = "expiry_time"
)

// queryInsertCIBAAuthRequest inserts a new CIBA authentication request.
var queryInsertCIBAAuthRequest = dbmodel.DBQuery{
	ID: "CBQ-CRS-01",
	Query: `INSERT INTO "CIBA_AUTH_REQUEST" (AUTH_REQ_ID, EXECUTION_ID, CLIENT_ID, USER_ID, SCOPES, STATE, ` +
		`EXPIRY_TIME, DEPLOYMENT_ID) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
}

// queryGetCIBAAuthRequest retrieves a CIBA authentication request by ID.
var queryGetCIBAAuthRequest = dbmodel.DBQuery{
	ID: "CBQ-CRS-02",
	Query: `SELECT AUTH_REQ_ID, EXECUTION_ID, CLIENT_ID, USER_ID, SCOPES, STATE, ATTRIBUTE_CACHE_ID, ` +
		`COMPLETED_ACR, AUTH_TIME, LAST_POLLED_AT, EXPIRY_TIME FROM "CIBA_AUTH_REQUEST" ` +
		`WHERE AUTH_REQ_ID = $1 AND DEPLOYMENT_ID = $2`,
}

// queryMarkCIBAAuthRequestAuthenticated marks a pending request as authenticated and records the
// attribute cache ID, completed ACR, and authentication time from the assertion.
var queryMarkCIBAAuthRequestAuthenticated = dbmodel.DBQuery{
	ID: "CBQ-CRS-03",
	Query: `UPDATE "CIBA_AUTH_REQUEST" SET STATE = $1, ATTRIBUTE_CACHE_ID = $2, COMPLETED_ACR = $3, ` +
		`AUTH_TIME = $4 WHERE AUTH_REQ_ID = $5 AND STATE = $6 AND DEPLOYMENT_ID = $7`,
}

// queryUpdateCIBAAuthRequestState updates the state of a CIBA authentication request.
var queryUpdateCIBAAuthRequestState = dbmodel.DBQuery{
	ID:    "CBQ-CRS-04",
	Query: `UPDATE "CIBA_AUTH_REQUEST" SET STATE = $1 WHERE AUTH_REQ_ID = $2 AND DEPLOYMENT_ID = $3`,
}

// queryConsumeCIBAAuthRequest atomically consumes an authenticated request (AUTHENTICATED → CONSUMED).
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
