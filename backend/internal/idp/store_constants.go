// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package idp

import "github.com/thunder-id/thunderid/internal/system/database/model"

var (
	// queryCreateIdentityProvider is the query to create a new IdP.
	queryCreateIdentityProvider = model.DBQuery{
		ID: "IPQ-IDP_MGT-01",
		Query: `INSERT INTO "IDP" (ID, NAME, DESCRIPTION, TYPE, PROPERTIES, ATTRIBUTE_CONFIGURATION, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7)`,
	}
	// queryGetIdentityProviderByID is the query to get a IdP by IdP ID.
	queryGetIdentityProviderByID = model.DBQuery{
		ID: "IPQ-IDP_MGT-02",
		Query: `SELECT ID, NAME, DESCRIPTION, TYPE, PROPERTIES, ATTRIBUTE_CONFIGURATION FROM "IDP" ` +
			`WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryGetIdentityProviderList is the query to get a list of IdPs.
	queryGetIdentityProviderList = model.DBQuery{
		ID: "IPQ-IDP_MGT-03",
		Query: `SELECT ID, NAME, DESCRIPTION, TYPE, PROPERTIES, ATTRIBUTE_CONFIGURATION FROM "IDP" ` +
			`WHERE DEPLOYMENT_ID = $1`,
	}
	// queryUpdateIdentityProviderByID is the query to update a IdP by IdP ID.
	queryUpdateIdentityProviderByID = model.DBQuery{
		ID: "IPQ-IDP_MGT-04",
		Query: `UPDATE "IDP" SET NAME = $2, DESCRIPTION = $3, TYPE = $4, PROPERTIES = $5, ` +
			`ATTRIBUTE_CONFIGURATION = $6 WHERE ID = $1 AND DEPLOYMENT_ID = $7`,
	}
	// queryDeleteIdentityProviderByID is the query to delete a IdP by IdP ID.
	queryDeleteIdentityProviderByID = model.DBQuery{
		ID:    "IPQ-IDP_MGT-05",
		Query: `DELETE FROM "IDP" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryGetIdentityProviderByName is the query to get a IdP by IdP name.
	queryGetIdentityProviderByName = model.DBQuery{
		ID: "IPQ-IDP_MGT-06",
		Query: `SELECT ID, NAME, DESCRIPTION, TYPE, PROPERTIES, ATTRIBUTE_CONFIGURATION FROM "IDP" ` +
			`WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryGetIdentityProviderListCount is the query to get a count of IdPs.
	queryGetIdentityProviderListCount = model.DBQuery{
		ID:    "IPQ-IDP_MGT-07",
		Query: `SELECT COUNT(*) AS count FROM "IDP" WHERE DEPLOYMENT_ID = $1`,
	}
	// queryGetIdentityProvidersByProperty is the query to get IDPs by a property key and value.
	queryGetIdentityProvidersByProperty = model.DBQuery{
		ID: "IPQ-IDP_MGT-08",
		PostgresQuery: `SELECT ID, NAME, DESCRIPTION, TYPE, PROPERTIES, ATTRIBUTE_CONFIGURATION FROM "IDP" ` +
			`WHERE PROPERTIES->$1->>'value' = $2 AND DEPLOYMENT_ID = $3`,
		SQLiteQuery: `SELECT ID, NAME, DESCRIPTION, TYPE, PROPERTIES, ATTRIBUTE_CONFIGURATION FROM "IDP" ` +
			`WHERE json_extract(PROPERTIES, '$.' || $1 || '.value') = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryCheckIdentityProviderOUColumn checks whether the IDP table has the OU_ID column. A deployment created
	// before the column was added keeps working without it. On PostgreSQL the table is resolved from the search path,
	// as the unqualified "IDP" of the other queries is, so the check looks at the table the store reads and writes.
	queryCheckIdentityProviderOUColumn = model.DBQuery{
		ID: "IPQ-IDP_MGT-09",
		PostgresQuery: `SELECT 1 FROM pg_attribute WHERE attrelid = to_regclass('"IDP"') ` +
			`AND attname = 'ou_id' AND attnum > 0 AND NOT attisdropped`,
		SQLiteQuery: `SELECT 1 FROM pragma_table_info('IDP') WHERE name = 'OU_ID'`,
	}
	// queryCreateIdentityProviderWithOU is the query to create a new IdP that records its owning organization unit.
	queryCreateIdentityProviderWithOU = model.DBQuery{
		ID: "IPQ-IDP_MGT-10",
		Query: `INSERT INTO "IDP" (ID, NAME, DESCRIPTION, TYPE, OU_ID, PROPERTIES, ATTRIBUTE_CONFIGURATION, ` +
			`DEPLOYMENT_ID) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
	}
	// queryGetIdentityProviderByIDWithOU is the query to get a IdP by IdP ID, with its owning organization unit.
	queryGetIdentityProviderByIDWithOU = model.DBQuery{
		ID: "IPQ-IDP_MGT-11",
		Query: `SELECT ID, NAME, DESCRIPTION, TYPE, OU_ID, PROPERTIES, ATTRIBUTE_CONFIGURATION FROM "IDP" ` +
			`WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryGetIdentityProviderListWithOU is the query to get a list of IdPs, with their owning organization units.
	queryGetIdentityProviderListWithOU = model.DBQuery{
		ID: "IPQ-IDP_MGT-12",
		Query: `SELECT ID, NAME, DESCRIPTION, TYPE, OU_ID, PROPERTIES, ATTRIBUTE_CONFIGURATION FROM "IDP" ` +
			`WHERE DEPLOYMENT_ID = $1`,
	}
	// queryGetIdentityProviderByNameWithOU is the query to get a IdP by IdP name, with its owning organization unit.
	queryGetIdentityProviderByNameWithOU = model.DBQuery{
		ID: "IPQ-IDP_MGT-13",
		Query: `SELECT ID, NAME, DESCRIPTION, TYPE, OU_ID, PROPERTIES, ATTRIBUTE_CONFIGURATION FROM "IDP" ` +
			`WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryGetIdentityProvidersByPropertyWithOU is the query to get IDPs by a property key and value, with their
	// owning organization units.
	queryGetIdentityProvidersByPropertyWithOU = model.DBQuery{
		ID: "IPQ-IDP_MGT-14",
		PostgresQuery: `SELECT ID, NAME, DESCRIPTION, TYPE, OU_ID, PROPERTIES, ATTRIBUTE_CONFIGURATION FROM "IDP" ` +
			`WHERE PROPERTIES->$1->>'value' = $2 AND DEPLOYMENT_ID = $3`,
		SQLiteQuery: `SELECT ID, NAME, DESCRIPTION, TYPE, OU_ID, PROPERTIES, ATTRIBUTE_CONFIGURATION FROM "IDP" ` +
			`WHERE json_extract(PROPERTIES, '$.' || $1 || '.value') = $2 AND DEPLOYMENT_ID = $3`,
	}
)
