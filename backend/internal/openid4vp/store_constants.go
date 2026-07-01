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

package openid4vp

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

// DBQuery definitions for the OpenID4VP request-state runtime store.
var (
	queryUpsertRequestState = dbmodel.DBQuery{
		ID: "OVPQ-RS-01",
		Query: `INSERT INTO "OPENID4VP_REQUEST_STATE" ` +
			`(STATE, DEPLOYMENT_ID, DEFINITION_ID, NONCE, EPHEMERAL_KEY, CLIENT_ID, RP_ID, REQUEST_URI, ` +
			`STATUS, RESULT, FAILURE_REASON, EXPIRY_TIME) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) ` +
			`ON CONFLICT (STATE) DO UPDATE SET ` +
			`DEFINITION_ID = EXCLUDED.DEFINITION_ID, NONCE = EXCLUDED.NONCE, ` +
			`EPHEMERAL_KEY = EXCLUDED.EPHEMERAL_KEY, CLIENT_ID = EXCLUDED.CLIENT_ID, ` +
			`RP_ID = EXCLUDED.RP_ID, REQUEST_URI = EXCLUDED.REQUEST_URI, STATUS = EXCLUDED.STATUS, ` +
			`RESULT = EXCLUDED.RESULT, FAILURE_REASON = EXCLUDED.FAILURE_REASON, ` +
			`EXPIRY_TIME = EXCLUDED.EXPIRY_TIME`,
	}
	queryGetRequestState = dbmodel.DBQuery{
		ID: "OVPQ-RS-02",
		Query: `SELECT STATE, DEFINITION_ID, NONCE, EPHEMERAL_KEY, CLIENT_ID, RP_ID, REQUEST_URI, ` +
			`STATUS, RESULT, FAILURE_REASON, EXPIRY_TIME FROM "OPENID4VP_REQUEST_STATE" ` +
			`WHERE STATE = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryDeleteRequestState = dbmodel.DBQuery{
		ID:    "OVPQ-RS-03",
		Query: `DELETE FROM "OPENID4VP_REQUEST_STATE" WHERE STATE = $1 AND DEPLOYMENT_ID = $2`,
	}
)
