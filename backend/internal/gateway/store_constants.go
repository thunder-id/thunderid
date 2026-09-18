// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

var (
	// queryListGateways returns every gateway of the deployment, oldest first.
	queryListGateways = dbmodel.DBQuery{
		ID: "GTW_MGT-01",
		Query: `SELECT ID, NAME, DATA_PLANE_ID, BASE_URL, MANAGEMENT_KEY, CA_CERTIFICATE, CREATED_AT, UPDATED_AT ` +
			`FROM "GATEWAY" WHERE DEPLOYMENT_ID = $1 ORDER BY CREATED_AT`,
	}
	// queryGetGatewayByID returns one gateway.
	queryGetGatewayByID = dbmodel.DBQuery{
		ID: "GTW_MGT-02",
		Query: `SELECT ID, NAME, DATA_PLANE_ID, BASE_URL, MANAGEMENT_KEY, CA_CERTIFICATE, CREATED_AT, UPDATED_AT ` +
			`FROM "GATEWAY" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryCountGateways backs the registration limit.
	queryCountGateways = dbmodel.DBQuery{
		ID:    "GTW_MGT-03",
		Query: `SELECT COUNT(*) AS total FROM "GATEWAY" WHERE DEPLOYMENT_ID = $1`,
	}
	// queryInsertGateway registers a gateway.
	//
	// The insert carries its own capacity check and hands back the row it wrote.
	//
	// Selecting the count inside the statement keeps the limit with the write rather than in a check
	// that happened earlier and may no longer hold. RETURNING then gives the stored row, including the
	// timestamps the database assigned, so a registration never has to read back what it just wrote
	// and cannot fail after the row is already committed.
	//
	// The number this checks is a guard, not an invariant, and the configuration says so. Under
	// PostgreSQL's READ COMMITTED two concurrent inserts can each count rows the other has not
	// committed and both be admitted, leaving one more gateway than configured. What is exact is the
	// uniqueness of NAME and DATA_PLANE_ID, which the table's constraints enforce.
	queryInsertGateway = dbmodel.DBQuery{
		ID: "GTW_MGT-04",
		Query: `INSERT INTO "GATEWAY" ` +
			`(ID, NAME, DATA_PLANE_ID, BASE_URL, MANAGEMENT_KEY, CA_CERTIFICATE, DEPLOYMENT_ID) ` +
			`SELECT $1, $2, $3, $4, $5, $6, $7 ` +
			`WHERE (SELECT COUNT(*) FROM "GATEWAY" WHERE DEPLOYMENT_ID = $7) < $8 ` +
			`RETURNING ID, NAME, DATA_PLANE_ID, BASE_URL, MANAGEMENT_KEY, CA_CERTIFICATE, ` +
			`CREATED_AT, UPDATED_AT`,
	}

	// queryUpdateGateway replaces a gateway's connection details.
	queryUpdateGateway = dbmodel.DBQuery{
		ID: "GTW_MGT-05",
		Query: `UPDATE "GATEWAY" SET NAME = $2, DATA_PLANE_ID = $3, BASE_URL = $4, MANAGEMENT_KEY = $5, ` +
			`CA_CERTIFICATE = $6, UPDATED_AT = NOW() WHERE ID = $1 AND DEPLOYMENT_ID = $7`,
		SQLiteQuery: `UPDATE "GATEWAY" SET NAME = $2, DATA_PLANE_ID = $3, BASE_URL = $4, ` +
			`MANAGEMENT_KEY = $5, CA_CERTIFICATE = $6, UPDATED_AT = datetime('now') ` +
			`WHERE ID = $1 AND DEPLOYMENT_ID = $7`,
	}
	// queryDeleteGateway removes a gateway.
	queryDeleteGateway = dbmodel.DBQuery{
		ID:    "GTW_MGT-06",
		Query: `DELETE FROM "GATEWAY" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryGetGatewayByName reads the gateway registered under a name.
	//
	// It selects every column rather than just the id, because a declared gateway is looked up this
	// way and then written back with its fields changed. A partial row would carry empty values into
	// that write and blank the stored name.
	queryGetGatewayByName = dbmodel.DBQuery{
		ID: "GTW_MGT-07",
		Query: `SELECT ID, NAME, DATA_PLANE_ID, BASE_URL, MANAGEMENT_KEY, CA_CERTIFICATE, ` +
			`CREATED_AT, UPDATED_AT FROM "GATEWAY" WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryGetGatewayByDataPlaneID supports the rule that one data plane registers once.
	queryGetGatewayByDataPlaneID = dbmodel.DBQuery{
		ID: "GTW_MGT-08",
		Query: `SELECT ID, NAME, DATA_PLANE_ID, BASE_URL, MANAGEMENT_KEY, CA_CERTIFICATE, ` +
			`CREATED_AT, UPDATED_AT FROM "GATEWAY" WHERE DATA_PLANE_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)
