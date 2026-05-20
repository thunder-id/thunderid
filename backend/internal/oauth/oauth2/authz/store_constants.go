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

package authz

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

// JSON keys for authorization request context serialization.
const (
	jsonKeyState               = "state"
	jsonKeyClientID            = "client_id"
	jsonKeyRedirectURI         = "redirect_uri"
	jsonKeyResponseType        = "response_type"
	jsonKeyStandardScopes      = "standard_scopes"
	jsonKeyPermissionScopes    = "permission_scopes"
	jsonKeyCodeChallenge       = "code_challenge"
	jsonKeyCodeChallengeMethod = "code_challenge_method"
	jsonKeyResource            = "resource"
	jsonKeyClaimsRequest       = "claims_request"
	jsonKeyClaimsLocales       = "claims_locales"
	jsonKeyNonce               = "nonce"
)

// Database column names for authorization request storage.
const (
	dbColumnRequestData = "request_data"
)

// queryInsertAuthorizationCode is the query to insert a new authorization code into the database.
var queryInsertAuthorizationCode = dbmodel.DBQuery{
	ID: "AZQ-ACS-01",
	Query: `INSERT INTO "AUTHORIZATION_CODE" (CODE_ID, AUTHORIZATION_CODE, CLIENT_ID, STATE, AUTHZ_DATA, ` +
		`TIME_CREATED, EXPIRY_TIME, DEPLOYMENT_ID) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
}

// queryGetAuthorizationCode is the query to retrieve an authorization code by code value.
var queryGetAuthorizationCode = dbmodel.DBQuery{
	ID: "AZQ-ACS-02",
	Query: `SELECT CODE_ID, AUTHORIZATION_CODE, CLIENT_ID, STATE, AUTHZ_DATA, TIME_CREATED, ` +
		`EXPIRY_TIME FROM "AUTHORIZATION_CODE" WHERE AUTHORIZATION_CODE = $1 AND DEPLOYMENT_ID = $2`,
}

// queryConsumeAuthorizationCode atomically consumes an authorization code (ACTIVE → INACTIVE).
var queryConsumeAuthorizationCode = dbmodel.DBQuery{
	ID: "AZQ-ACS-04",
	Query: `UPDATE "AUTHORIZATION_CODE" SET STATE = $1 WHERE AUTHORIZATION_CODE = $2 ` +
		`AND STATE = $3 AND DEPLOYMENT_ID = $4`,
}

// queryInsertAuthRequest is the query to insert a new authorization request context.
var queryInsertAuthRequest = dbmodel.DBQuery{
	ID: "AZQ-ARS-01",
	Query: `INSERT INTO "AUTHORIZATION_REQUEST" (AUTH_ID, REQUEST_DATA, EXPIRY_TIME, DEPLOYMENT_ID) ` +
		`VALUES ($1, $2, $3, $4)`,
}

// queryGetAuthRequest is the query to retrieve an authorization request context by ID.
var queryGetAuthRequest = dbmodel.DBQuery{
	ID: "AZQ-ARS-02",
	Query: `SELECT AUTH_ID, REQUEST_DATA, EXPIRY_TIME ` +
		`FROM "AUTHORIZATION_REQUEST" WHERE AUTH_ID = $1 AND EXPIRY_TIME > $2 AND DEPLOYMENT_ID = $3`,
}

// queryDeleteAuthRequest is the query to delete a specific authorization request context.
var queryDeleteAuthRequest = dbmodel.DBQuery{
	ID:    "AZQ-ARS-03",
	Query: `DELETE FROM "AUTHORIZATION_REQUEST" WHERE AUTH_ID = $1 AND DEPLOYMENT_ID = $2`,
}
