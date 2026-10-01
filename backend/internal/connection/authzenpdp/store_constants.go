// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

const authZENPDPConnectionType = "authzen-pdp"

var (
	queryCreateAuthZENPDPConnection = dbmodel.DBQuery{
		ID: "AZQ-AUTHZEN_PDP_MGT-01",
		Query: `INSERT INTO "AUTHORIZATION_PDP_CONNECTION"
			(ID, NAME, DESCRIPTION, TYPE, PROPERTIES, DEPLOYMENT_ID)
			VALUES ($1, $2, $3, $4, $5, $6)`,
	}

	queryGetAuthZENPDPConnectionByID = dbmodel.DBQuery{
		ID: "AZQ-AUTHZEN_PDP_MGT-02",
		Query: `SELECT ID, NAME, DESCRIPTION, PROPERTIES
			FROM "AUTHORIZATION_PDP_CONNECTION"
			WHERE ID = $1 AND TYPE = $2 AND DEPLOYMENT_ID = $3`,
	}

	queryGetAuthZENPDPConnectionByName = dbmodel.DBQuery{
		ID: "AZQ-AUTHZEN_PDP_MGT-03",
		Query: `SELECT ID, NAME, DESCRIPTION, PROPERTIES
			FROM "AUTHORIZATION_PDP_CONNECTION"
			WHERE NAME = $1 AND TYPE = $2 AND DEPLOYMENT_ID = $3`,
	}

	queryListAuthZENPDPConnections = dbmodel.DBQuery{
		ID: "AZQ-AUTHZEN_PDP_MGT-04",
		Query: `SELECT ID, NAME, DESCRIPTION, PROPERTIES
			FROM "AUTHORIZATION_PDP_CONNECTION"
			WHERE TYPE = $1 AND DEPLOYMENT_ID = $2
			ORDER BY NAME ASC, ID ASC`,
	}

	queryCountAuthZENPDPConnections = dbmodel.DBQuery{
		ID: "AZQ-AUTHZEN_PDP_MGT-07",
		Query: `SELECT COUNT(*) AS count FROM "AUTHORIZATION_PDP_CONNECTION"
			WHERE TYPE = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryUpdateAuthZENPDPConnection = dbmodel.DBQuery{
		ID: "AZQ-AUTHZEN_PDP_MGT-05",
		PostgresQuery: `UPDATE "AUTHORIZATION_PDP_CONNECTION"
			SET NAME = $1, DESCRIPTION = $2, PROPERTIES = $3,
			UPDATED_AT = NOW()
			WHERE ID = $4 AND TYPE = $5 AND DEPLOYMENT_ID = $6`,
		SQLiteQuery: `UPDATE "AUTHORIZATION_PDP_CONNECTION"
			SET NAME = $1, DESCRIPTION = $2, PROPERTIES = $3,
			UPDATED_AT = datetime('now')
			WHERE ID = $4 AND TYPE = $5 AND DEPLOYMENT_ID = $6`,
		Query: `UPDATE "AUTHORIZATION_PDP_CONNECTION"
			SET NAME = $1, DESCRIPTION = $2, PROPERTIES = $3,
			UPDATED_AT = datetime('now')
			WHERE ID = $4 AND TYPE = $5 AND DEPLOYMENT_ID = $6`,
	}

	queryDeleteAuthZENPDPConnection = dbmodel.DBQuery{
		ID: "AZQ-AUTHZEN_PDP_MGT-06",
		Query: `DELETE FROM "AUTHORIZATION_PDP_CONNECTION"
			WHERE ID = $1 AND TYPE = $2 AND DEPLOYMENT_ID = $3`,
	}
)
