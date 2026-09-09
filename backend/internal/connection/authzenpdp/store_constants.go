// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

var (
	queryCreateAuthZENPDPConnection = dbmodel.DBQuery{
		ID: "CON-AUTHZEN-PDP-01",
		Query: `INSERT INTO "AUTHZEN_PDP_CONNECTION"
			(ID, NAME, DESCRIPTION, PROPERTIES, DEPLOYMENT_ID)
			VALUES ($1, $2, $3, $4, $5)`,
	}

	queryGetAuthZENPDPConnectionByID = dbmodel.DBQuery{
		ID: "CON-AUTHZEN-PDP-02",
		Query: `SELECT ID, NAME, DESCRIPTION, PROPERTIES
			FROM "AUTHZEN_PDP_CONNECTION"
			WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryGetAuthZENPDPConnectionByName = dbmodel.DBQuery{
		ID: "CON-AUTHZEN-PDP-06",
		Query: `SELECT ID, NAME, DESCRIPTION, PROPERTIES
			FROM "AUTHZEN_PDP_CONNECTION"
			WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryListAuthZENPDPConnections = dbmodel.DBQuery{
		ID: "CON-AUTHZEN-PDP-03",
		Query: `SELECT ID, NAME, DESCRIPTION, PROPERTIES
			FROM "AUTHZEN_PDP_CONNECTION"
			WHERE DEPLOYMENT_ID = $1
			ORDER BY NAME ASC, ID ASC`,
	}

	queryUpdateAuthZENPDPConnection = dbmodel.DBQuery{
		ID: "CON-AUTHZEN-PDP-04",
		PostgresQuery: `UPDATE "AUTHZEN_PDP_CONNECTION"
			SET NAME = $1, DESCRIPTION = $2, PROPERTIES = $3,
			UPDATED_AT = NOW()
			WHERE ID = $4 AND DEPLOYMENT_ID = $5`,
		SQLiteQuery: `UPDATE "AUTHZEN_PDP_CONNECTION"
			SET NAME = $1, DESCRIPTION = $2, PROPERTIES = $3,
			UPDATED_AT = datetime('now')
			WHERE ID = $4 AND DEPLOYMENT_ID = $5`,
		Query: `UPDATE "AUTHZEN_PDP_CONNECTION"
			SET NAME = $1, DESCRIPTION = $2, PROPERTIES = $3,
			UPDATED_AT = datetime('now')
			WHERE ID = $4 AND DEPLOYMENT_ID = $5`,
	}

	queryDeleteAuthZENPDPConnection = dbmodel.DBQuery{
		ID:    "CON-AUTHZEN-PDP-05",
		Query: `DELETE FROM "AUTHZEN_PDP_CONNECTION" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)
