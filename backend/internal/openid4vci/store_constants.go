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

package openid4vci

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

// DBQuery definitions for the OpenID4VCI runtime stores.
var (
	queryInsertNonce = dbmodel.DBQuery{
		ID:    "OVCIQ-NS-01",
		Query: `INSERT INTO "OPENID4VCI_NONCE" (NONCE, DEPLOYMENT_ID, EXPIRY_TIME) VALUES ($1, $2, $3)`,
	}
	queryGetNonce = dbmodel.DBQuery{
		ID:    "OVCIQ-NS-02",
		Query: `SELECT NONCE, EXPIRY_TIME FROM "OPENID4VCI_NONCE" WHERE NONCE = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryDeleteNonce = dbmodel.DBQuery{
		ID:    "OVCIQ-NS-03",
		Query: `DELETE FROM "OPENID4VCI_NONCE" WHERE NONCE = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryInsertOffer = dbmodel.DBQuery{
		ID: "OVCIQ-OS-01",
		Query: `INSERT INTO "OPENID4VCI_CREDENTIAL_OFFER"` +
			` (ID, DEPLOYMENT_ID, OFFER, EXPIRY_TIME) VALUES ($1, $2, $3, $4)`,
	}
	queryGetOffer = dbmodel.DBQuery{
		ID:    "OVCIQ-OS-02",
		Query: `SELECT ID, OFFER, EXPIRY_TIME FROM "OPENID4VCI_CREDENTIAL_OFFER" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)
