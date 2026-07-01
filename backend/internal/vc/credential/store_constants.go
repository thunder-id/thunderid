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

package credential

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

// DBQuery definitions for the credential-configuration config store.
var (
	queryCreateConfiguration = dbmodel.DBQuery{
		ID: "OVCIQ-CC_MGT-01",
		Query: `INSERT INTO "CREDENTIAL_CONFIGURATION" ` +
			`(ID, HANDLE, OU_ID, NAME, DESCRIPTION, FORMAT, VCT, CLAIMS, DISPLAY, VALIDITY_SECONDS, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
	}
	queryGetConfigurationByID = dbmodel.DBQuery{
		ID: "OVCIQ-CC_MGT-02",
		Query: `SELECT ID, HANDLE, OU_ID, NAME, DESCRIPTION, FORMAT, VCT, CLAIMS, DISPLAY, VALIDITY_SECONDS ` +
			`FROM "CREDENTIAL_CONFIGURATION" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryGetConfigurationByHandle = dbmodel.DBQuery{
		ID: "OVCIQ-CC_MGT-03",
		Query: `SELECT ID, HANDLE, OU_ID, NAME, DESCRIPTION, FORMAT, VCT, CLAIMS, DISPLAY, VALIDITY_SECONDS ` +
			`FROM "CREDENTIAL_CONFIGURATION" WHERE HANDLE = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryListConfigurations = dbmodel.DBQuery{
		ID: "OVCIQ-CC_MGT-04",
		Query: `SELECT ID, HANDLE, OU_ID, NAME, DESCRIPTION, FORMAT, VCT, CLAIMS, DISPLAY, VALIDITY_SECONDS ` +
			`FROM "CREDENTIAL_CONFIGURATION" WHERE DEPLOYMENT_ID = $1`,
	}
	queryListConfigurationSummaries = dbmodel.DBQuery{
		ID: "OVCIQ-CC_MGT-07",
		Query: `SELECT ID, HANDLE, OU_ID, NAME, FORMAT, VCT ` +
			`FROM "CREDENTIAL_CONFIGURATION" WHERE DEPLOYMENT_ID = $1`,
	}
	queryUpdateConfiguration = dbmodel.DBQuery{
		ID: "OVCIQ-CC_MGT-05",
		Query: `UPDATE "CREDENTIAL_CONFIGURATION" SET HANDLE = $2, OU_ID = $3, NAME = $4, DESCRIPTION = $5, ` +
			`FORMAT = $6, VCT = $7, CLAIMS = $8, DISPLAY = $9, VALIDITY_SECONDS = $10, ` +
			`UPDATED_AT = CURRENT_TIMESTAMP WHERE ID = $1 AND DEPLOYMENT_ID = $11`,
	}
	queryDeleteConfiguration = dbmodel.DBQuery{
		ID:    "OVCIQ-CC_MGT-06",
		Query: `DELETE FROM "CREDENTIAL_CONFIGURATION" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)
