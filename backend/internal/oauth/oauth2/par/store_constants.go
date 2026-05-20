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

package par

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

// Database column names for PAR request storage.
const (
	dbColumnRequestURI    = "request_uri"
	dbColumnRequestParams = "request_params"
)

var queryInsertPARRequest = dbmodel.DBQuery{
	ID: "PARQ-PRS-01",
	Query: `INSERT INTO "PAR_REQUEST" (REQUEST_URI, DEPLOYMENT_ID, REQUEST_PARAMS, EXPIRY_TIME) ` +
		`VALUES ($1, $2, $3, $4)`,
}

var queryGetPARRequest = dbmodel.DBQuery{
	ID: "PARQ-PRS-02",
	Query: `SELECT REQUEST_URI, REQUEST_PARAMS FROM "PAR_REQUEST" ` +
		`WHERE REQUEST_URI = $1 AND EXPIRY_TIME > $2 AND DEPLOYMENT_ID = $3`,
}

var queryDeletePARRequest = dbmodel.DBQuery{
	ID:    "PARQ-PRS-03",
	Query: `DELETE FROM "PAR_REQUEST" WHERE REQUEST_URI = $1 AND DEPLOYMENT_ID = $2`,
}
