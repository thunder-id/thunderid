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

package inboundclient

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

var (
	// queryCreateInboundClient creates a new inbound client entry for an entity.
	queryCreateInboundClient = dbmodel.DBQuery{
		ID: "ASQ-INBC_MGT-01",
		Query: `INSERT INTO "INBOUND_CLIENT" (ENTITY_ID, AUTH_FLOW_ID, REGISTRATION_FLOW_ID, ` +
			`IS_REGISTRATION_FLOW_ENABLED, RECOVERY_FLOW_ID, IS_RECOVERY_FLOW_ENABLED, ` +
			`THEME_ID, LAYOUT_ID, PROPERTIES, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
	}
	// queryCreateOAuthProfile creates a new OAuth inbound profile entry keyed by entity ID.
	queryCreateOAuthProfile = dbmodel.DBQuery{
		ID:    "ASQ-INBC_MGT-02",
		Query: `INSERT INTO "OAUTH_INBOUND_PROFILE" (ENTITY_ID, OAUTH_CONFIG, DEPLOYMENT_ID) VALUES ($1, $2, $3)`,
	}
	// queryGetInboundClientByEntityID retrieves an inbound client by entity ID.
	queryGetInboundClientByEntityID = dbmodel.DBQuery{
		ID: "ASQ-INBC_MGT-03",
		Query: `SELECT app.ENTITY_ID, app.AUTH_FLOW_ID, app.REGISTRATION_FLOW_ID, ` +
			`app.IS_REGISTRATION_FLOW_ENABLED, app.RECOVERY_FLOW_ID, app.IS_RECOVERY_FLOW_ENABLED, ` +
			`app.THEME_ID, app.LAYOUT_ID, app.PROPERTIES ` +
			`FROM "INBOUND_CLIENT" app WHERE app.ENTITY_ID = $1 AND app.DEPLOYMENT_ID = $2`,
	}
	// queryGetOAuthProfileByEntityID retrieves an OAuth inbound profile by entity ID.
	queryGetOAuthProfileByEntityID = dbmodel.DBQuery{
		ID: "ASQ-INBC_MGT-05",
		Query: `SELECT ENTITY_ID, OAUTH_CONFIG FROM "OAUTH_INBOUND_PROFILE" ` +
			`WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryGetInboundClientList lists all inbound clients.
	queryGetInboundClientList = dbmodel.DBQuery{
		ID: "ASQ-INBC_MGT-06",
		Query: `SELECT app.ENTITY_ID, app.AUTH_FLOW_ID, app.REGISTRATION_FLOW_ID, ` +
			`app.IS_REGISTRATION_FLOW_ENABLED, app.RECOVERY_FLOW_ID, app.IS_RECOVERY_FLOW_ENABLED, ` +
			`app.THEME_ID, app.LAYOUT_ID, app.PROPERTIES ` +
			`FROM "INBOUND_CLIENT" app WHERE app.DEPLOYMENT_ID = $1 LIMIT $2`,
	}
	// queryUpdateInboundClientByEntityID updates an inbound client by entity ID.
	queryUpdateInboundClientByEntityID = dbmodel.DBQuery{
		ID: "ASQ-INBC_MGT-07",
		Query: `UPDATE "INBOUND_CLIENT" SET AUTH_FLOW_ID=$2, REGISTRATION_FLOW_ID=$3, ` +
			`IS_REGISTRATION_FLOW_ENABLED=$4, RECOVERY_FLOW_ID=$5, IS_RECOVERY_FLOW_ENABLED=$6, ` +
			`THEME_ID=$7, LAYOUT_ID=$8, PROPERTIES=$9 ` +
			`WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $10`,
	}
	// queryUpdateOAuthProfileByEntityID updates an OAuth inbound profile by entity ID.
	queryUpdateOAuthProfileByEntityID = dbmodel.DBQuery{
		ID:    "ASQ-INBC_MGT-08",
		Query: `UPDATE "OAUTH_INBOUND_PROFILE" SET OAUTH_CONFIG=$2 WHERE ENTITY_ID=$1 AND DEPLOYMENT_ID=$3`,
	}
	// queryDeleteInboundClientByEntityID deletes an inbound client by entity ID. Cascades to OAuth profile.
	queryDeleteInboundClientByEntityID = dbmodel.DBQuery{
		ID:    "ASQ-INBC_MGT-09",
		Query: `DELETE FROM "INBOUND_CLIENT" WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryGetInboundClientCount gets the total count of inbound clients.
	queryGetInboundClientCount = dbmodel.DBQuery{
		ID:    "ASQ-INBC_MGT-10",
		Query: `SELECT COUNT(*) as total FROM "INBOUND_CLIENT" WHERE DEPLOYMENT_ID = $1`,
	}
	// queryDeleteOAuthProfileByEntityID deletes an OAuth inbound profile by entity ID.
	queryDeleteOAuthProfileByEntityID = dbmodel.DBQuery{
		ID:    "ASQ-INBC_MGT-11",
		Query: `DELETE FROM "OAUTH_INBOUND_PROFILE" WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryCheckInboundClientExistsByEntityID checks if an inbound client exists by entity ID.
	queryCheckInboundClientExistsByEntityID = dbmodel.DBQuery{
		ID:    "ASQ-INBC_MGT-12",
		Query: `SELECT COUNT(*) as count FROM "INBOUND_CLIENT" WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)
