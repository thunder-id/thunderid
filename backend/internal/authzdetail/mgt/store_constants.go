// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

// Authorization details type queries, against the config database.
var (
	// queryCreateType creates an authorization details type.
	queryCreateType = dbmodel.DBQuery{
		ID: "ADQ-ADT_MGT-01",
		Query: `INSERT INTO "AUTHORIZATION_DETAIL_TYPE"
			(ID, TYPE, NAME, DESCRIPTION, VERSION, SCHEMA_DEFINITION, PROPERTIES, DEPLOYMENT_ID)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
	}

	// queryGetTypeByID retrieves an authorization details type by id.
	queryGetTypeByID = dbmodel.DBQuery{
		ID: "ADQ-ADT_MGT-02",
		Query: `SELECT ID, TYPE, NAME, DESCRIPTION, VERSION, SCHEMA_DEFINITION, PROPERTIES
			FROM "AUTHORIZATION_DETAIL_TYPE"
			WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetTypeByValue retrieves an authorization details type by its type value.
	queryGetTypeByValue = dbmodel.DBQuery{
		ID: "ADQ-ADT_MGT-03",
		Query: `SELECT ID, TYPE, NAME, DESCRIPTION, VERSION, SCHEMA_DEFINITION, PROPERTIES
			FROM "AUTHORIZATION_DETAIL_TYPE"
			WHERE TYPE = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryListTypes lists the authorization details types of the deployment.
	queryListTypes = dbmodel.DBQuery{
		ID: "ADQ-ADT_MGT-04",
		Query: `SELECT ID, TYPE, NAME, DESCRIPTION, VERSION, SCHEMA_DEFINITION, PROPERTIES
			FROM "AUTHORIZATION_DETAIL_TYPE"
			WHERE DEPLOYMENT_ID = $1
			ORDER BY TYPE`,
	}

	// queryUpdateType updates an authorization details type still at version $7 and adds $6 to its version.
	queryUpdateType = dbmodel.DBQuery{
		ID: "ADQ-ADT_MGT-05",
		PostgresQuery: `UPDATE "AUTHORIZATION_DETAIL_TYPE"
			SET NAME = $2, DESCRIPTION = $3, SCHEMA_DEFINITION = $4, PROPERTIES = $5, UPDATED_AT = NOW(),
			VERSION = VERSION + $6
			WHERE ID = $1 AND VERSION = $7 AND DEPLOYMENT_ID = $8`,
		SQLiteQuery: `UPDATE "AUTHORIZATION_DETAIL_TYPE"
			SET NAME = $2, DESCRIPTION = $3, SCHEMA_DEFINITION = $4, PROPERTIES = $5, UPDATED_AT = datetime('now'),
			VERSION = VERSION + $6
			WHERE ID = $1 AND VERSION = $7 AND DEPLOYMENT_ID = $8`,
		Query: `UPDATE "AUTHORIZATION_DETAIL_TYPE"
			SET NAME = $2, DESCRIPTION = $3, SCHEMA_DEFINITION = $4, PROPERTIES = $5, UPDATED_AT = datetime('now'),
			VERSION = VERSION + $6
			WHERE ID = $1 AND VERSION = $7 AND DEPLOYMENT_ID = $8`,
	}

	// queryDeleteType deletes an authorization details type.
	queryDeleteType = dbmodel.DBQuery{
		ID: "ADQ-ADT_MGT-06",
		Query: `DELETE FROM "AUTHORIZATION_DETAIL_TYPE"
			WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCountTypes counts the authorization details types of the deployment.
	queryCountTypes = dbmodel.DBQuery{
		ID:    "ADQ-ADT_MGT-07",
		Query: `SELECT COUNT(*) AS total FROM "AUTHORIZATION_DETAIL_TYPE" WHERE DEPLOYMENT_ID = $1`,
	}

	// queryListTypePage lists one page of the authorization details types of the deployment.
	queryListTypePage = dbmodel.DBQuery{
		ID: "ADQ-ADT_MGT-08",
		Query: `SELECT ID, TYPE, NAME, DESCRIPTION, VERSION, SCHEMA_DEFINITION, PROPERTIES
			FROM "AUTHORIZATION_DETAIL_TYPE"
			WHERE DEPLOYMENT_ID = $3
			ORDER BY TYPE
			LIMIT $1 OFFSET $2`,
	}
)
