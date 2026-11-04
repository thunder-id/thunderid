// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package inboundclient

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

var (
	// queryCreateInboundClient creates a new inbound client entry for an entity.
	queryCreateInboundClient = dbmodel.DBQuery{
		ID: "ASQ-INBC_MGT-01",
		Query: `INSERT INTO "INBOUND_CLIENT" (ENTITY_ID, AUTH_FLOW_ID, DEVICE_GRANT_FLOW_ID, REGISTRATION_FLOW_ID, ` +
			`IS_REGISTRATION_FLOW_ENABLED, RECOVERY_FLOW_ID, IS_RECOVERY_FLOW_ENABLED, ` +
			`SIGNOUT_FLOW_ID, ` +
			`THEME_ID, LAYOUT_ID, PROPERTIES, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
	}
	// queryCreateOAuthProfile creates a new OAuth inbound profile entry keyed by entity ID.
	queryCreateOAuthProfile = dbmodel.DBQuery{
		ID:    "ASQ-INBC_MGT-02",
		Query: `INSERT INTO "OAUTH_INBOUND_PROFILE" (ENTITY_ID, OAUTH_CONFIG, DEPLOYMENT_ID) VALUES ($1, $2, $3)`,
	}
	// queryGetInboundClientByEntityID retrieves an inbound client by entity ID.
	queryGetInboundClientByEntityID = dbmodel.DBQuery{
		ID: "ASQ-INBC_MGT-03",
		Query: `SELECT app.ENTITY_ID, app.AUTH_FLOW_ID, app.DEVICE_GRANT_FLOW_ID, app.REGISTRATION_FLOW_ID, ` +
			`app.IS_REGISTRATION_FLOW_ENABLED, app.RECOVERY_FLOW_ID, app.IS_RECOVERY_FLOW_ENABLED, ` +
			`app.SIGNOUT_FLOW_ID, ` +
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
		Query: `SELECT app.ENTITY_ID, app.AUTH_FLOW_ID, app.DEVICE_GRANT_FLOW_ID, app.REGISTRATION_FLOW_ID, ` +
			`app.IS_REGISTRATION_FLOW_ENABLED, app.RECOVERY_FLOW_ID, app.IS_RECOVERY_FLOW_ENABLED, ` +
			`app.SIGNOUT_FLOW_ID, ` +
			`app.THEME_ID, app.LAYOUT_ID, app.PROPERTIES ` +
			`FROM "INBOUND_CLIENT" app WHERE app.DEPLOYMENT_ID = $1 LIMIT $2`,
	}
	// queryUpdateInboundClientByEntityID updates an inbound client by entity ID.
	queryUpdateInboundClientByEntityID = dbmodel.DBQuery{
		ID: "ASQ-INBC_MGT-07",
		Query: `UPDATE "INBOUND_CLIENT" SET AUTH_FLOW_ID=$2, DEVICE_GRANT_FLOW_ID=$3, REGISTRATION_FLOW_ID=$4, ` +
			`IS_REGISTRATION_FLOW_ENABLED=$5, RECOVERY_FLOW_ID=$6, IS_RECOVERY_FLOW_ENABLED=$7, ` +
			`SIGNOUT_FLOW_ID=$8, ` +
			`THEME_ID=$9, LAYOUT_ID=$10, PROPERTIES=$11 ` +
			`WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $12`,
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

	// queryGetEntityIDsByThemeID retrieves paginated entity IDs for inbound clients using a specific theme.
	queryGetEntityIDsByThemeID = dbmodel.DBQuery{
		ID: "ASQ-INBC_MGT-13",
		Query: `SELECT ENTITY_ID FROM "INBOUND_CLIENT" WHERE THEME_ID = $1 AND DEPLOYMENT_ID = $2 ` +
			`ORDER BY ENTITY_ID ASC LIMIT $3 OFFSET $4`,
	}

	// queryGetEntityIDsByThemeIDCount retrieves the total count of inbound clients using a specific theme.
	queryGetEntityIDsByThemeIDCount = dbmodel.DBQuery{
		ID:    "ASQ-INBC_MGT-14",
		Query: `SELECT COUNT(*) as total FROM "INBOUND_CLIENT" WHERE THEME_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetEntityIDsByLayoutID retrieves paginated entity IDs for inbound clients using a specific layout.
	queryGetEntityIDsByLayoutID = dbmodel.DBQuery{
		ID: "ASQ-INBC_MGT-17",
		Query: `SELECT ENTITY_ID FROM "INBOUND_CLIENT" WHERE LAYOUT_ID = $1 AND DEPLOYMENT_ID = $2 ` +
			`ORDER BY ENTITY_ID ASC LIMIT $3 OFFSET $4`,
	}

	// queryGetEntityIDsByLayoutIDCount retrieves the total count of inbound clients using a specific layout.
	queryGetEntityIDsByLayoutIDCount = dbmodel.DBQuery{
		ID:    "ASQ-INBC_MGT-18",
		Query: `SELECT COUNT(*) as total FROM "INBOUND_CLIENT" WHERE LAYOUT_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetEntityIDsByFlowID retrieves paginated entity IDs for inbound clients referencing a specific
	// flow through any of the authentication, registration, recovery, or sign-out flow slots.
	queryGetEntityIDsByFlowID = dbmodel.DBQuery{
		ID: "ASQ-INBC_MGT-15",
		Query: `SELECT ENTITY_ID FROM "INBOUND_CLIENT" WHERE ` +
			`(AUTH_FLOW_ID = $1 OR DEVICE_GRANT_FLOW_ID = $2 OR REGISTRATION_FLOW_ID = $3 OR ` +
			`RECOVERY_FLOW_ID = $4 OR SIGNOUT_FLOW_ID = $5) ` +
			`AND DEPLOYMENT_ID = $6 ORDER BY ENTITY_ID ASC LIMIT $7 OFFSET $8`,
	}

	// queryGetEntityIDsByFlowIDCount retrieves the total count of inbound clients referencing a specific flow.
	queryGetEntityIDsByFlowIDCount = dbmodel.DBQuery{
		ID: "ASQ-INBC_MGT-16",
		Query: `SELECT COUNT(*) as total FROM "INBOUND_CLIENT" WHERE ` +
			`(AUTH_FLOW_ID = $1 OR DEVICE_GRANT_FLOW_ID = $2 OR REGISTRATION_FLOW_ID = $3 OR ` +
			`RECOVERY_FLOW_ID = $4 OR SIGNOUT_FLOW_ID = $5) AND DEPLOYMENT_ID = $6`,
	}
)
