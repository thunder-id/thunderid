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

package passkey

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

// Database column name for WebAuthn session storage.
const (
	dbColumnSessionData = "session_data"
)

// JSON keys for session data serialization.
const (
	jsonKeyChallenge          = "challenge"
	jsonKeyRelyingPartyID     = "rp_id"
	jsonKeyUserID             = "user_id"
	jsonKeyAllowedCredentials = "allowed_credentials"
	jsonKeyExpires            = "expires"
	jsonKeyUserVerification   = "user_verification"
	jsonKeyExtensions         = "extensions"
	jsonKeyCredParams         = "cred_params" // nolint:gosec // This is a JSON key, not a credential
	jsonKeyMediation          = "mediation"
)

// queryInsertSession is the query to insert a new WebAuthn session into the database.
var queryInsertSession = dbmodel.DBQuery{
	ID: "WEBAUTHN-SS-01",
	Query: `INSERT INTO "WEBAUTHN_SESSION" (SESSION_KEY, SESSION_DATA, ` +
		`EXPIRY_TIME, DEPLOYMENT_ID) VALUES ($1, $2, $3, $4)`,
}

// queryGetSession is the query to retrieve a WebAuthn session by session key.
var queryGetSession = dbmodel.DBQuery{
	ID: "WEBAUTHN-SS-02",
	Query: `SELECT SESSION_KEY, SESSION_DATA, EXPIRY_TIME ` +
		`FROM "WEBAUTHN_SESSION" WHERE SESSION_KEY = $1 AND EXPIRY_TIME > $2 AND DEPLOYMENT_ID = $3`,
}

// queryDeleteSession is the query to delete a specific WebAuthn session.
var queryDeleteSession = dbmodel.DBQuery{
	ID:    "WEBAUTHN-SS-03",
	Query: `DELETE FROM "WEBAUTHN_SESSION" WHERE SESSION_KEY = $1 AND DEPLOYMENT_ID = $2`,
}
