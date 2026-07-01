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

package presentation

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

// DBQuery definitions for the presentation-definition config store.
var (
	queryCreateDefinition = dbmodel.DBQuery{
		ID: "OVPQ-PD_MGT-01",
		Query: `INSERT INTO "PRESENTATION_DEFINITION" ` +
			`(ID, HANDLE, OU_ID, NAME, DESCRIPTION, VCT, FORMAT, CLAIMS, ` +
			`ENFORCE_TRUSTED_ISSUER, TRUSTED_AUTHORITIES, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
	}
	queryGetDefinitionByID = dbmodel.DBQuery{
		ID: "OVPQ-PD_MGT-02",
		Query: `SELECT ID, HANDLE, OU_ID, NAME, DESCRIPTION, VCT, FORMAT, CLAIMS, ` +
			`ENFORCE_TRUSTED_ISSUER, TRUSTED_AUTHORITIES ` +
			`FROM "PRESENTATION_DEFINITION" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryGetDefinitionByHandle = dbmodel.DBQuery{
		ID: "OVPQ-PD_MGT-03",
		Query: `SELECT ID, HANDLE, OU_ID, NAME, DESCRIPTION, VCT, FORMAT, CLAIMS, ` +
			`ENFORCE_TRUSTED_ISSUER, TRUSTED_AUTHORITIES ` +
			`FROM "PRESENTATION_DEFINITION" WHERE HANDLE = $1 AND DEPLOYMENT_ID = $2`,
	}
	queryListDefinitions = dbmodel.DBQuery{
		ID: "OVPQ-PD_MGT-04",
		Query: `SELECT ID, HANDLE, OU_ID, NAME, DESCRIPTION, VCT, FORMAT, CLAIMS, ` +
			`ENFORCE_TRUSTED_ISSUER, TRUSTED_AUTHORITIES ` +
			`FROM "PRESENTATION_DEFINITION" WHERE DEPLOYMENT_ID = $1`,
	}
	queryListDefinitionSummaries = dbmodel.DBQuery{
		ID: "OVPQ-PD_MGT-07",
		Query: `SELECT ID, HANDLE, OU_ID, NAME, VCT, FORMAT ` +
			`FROM "PRESENTATION_DEFINITION" WHERE DEPLOYMENT_ID = $1`,
	}
	queryUpdateDefinition = dbmodel.DBQuery{
		ID: "OVPQ-PD_MGT-05",
		Query: `UPDATE "PRESENTATION_DEFINITION" SET HANDLE = $2, OU_ID = $3, NAME = $4, DESCRIPTION = $5, ` +
			`VCT = $6, FORMAT = $7, CLAIMS = $8, ` +
			`ENFORCE_TRUSTED_ISSUER = $9, TRUSTED_AUTHORITIES = $10, UPDATED_AT = CURRENT_TIMESTAMP ` +
			`WHERE ID = $1 AND DEPLOYMENT_ID = $11`,
	}
	queryDeleteDefinition = dbmodel.DBQuery{
		ID:    "OVPQ-PD_MGT-06",
		Query: `DELETE FROM "PRESENTATION_DEFINITION" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)
