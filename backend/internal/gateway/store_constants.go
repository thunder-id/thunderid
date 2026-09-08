// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

var (
	// queryListGateways returns every gateway of the deployment, oldest first.
	queryListGateways = dbmodel.DBQuery{
		ID: "GTW_MGT-01",
		Query: `SELECT ID, NAME, BASE_URL, CLIENT_ID, CLIENT_SECRET, SCOPE, CREATED_AT, UPDATED_AT ` +
			`FROM "GATEWAY" WHERE DEPLOYMENT_ID = $1 ORDER BY CREATED_AT`,
	}
	// queryGetGatewayByID returns one gateway.
	queryGetGatewayByID = dbmodel.DBQuery{
		ID: "GTW_MGT-02",
		Query: `SELECT ID, NAME, BASE_URL, CLIENT_ID, CLIENT_SECRET, SCOPE, CREATED_AT, UPDATED_AT ` +
			`FROM "GATEWAY" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryCountGateways backs the registration limit.
	queryCountGateways = dbmodel.DBQuery{
		ID:    "GTW_MGT-03",
		Query: `SELECT COUNT(*) AS total FROM "GATEWAY" WHERE DEPLOYMENT_ID = $1`,
	}
	// queryInsertGateway registers a gateway.
	queryInsertGateway = dbmodel.DBQuery{
		ID: "GTW_MGT-04",
		Query: `INSERT INTO "GATEWAY" (ID, NAME, BASE_URL, CLIENT_ID, CLIENT_SECRET, SCOPE, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7)`,
	}
	// queryUpdateGateway replaces a gateway's connection details.
	queryUpdateGateway = dbmodel.DBQuery{
		ID: "GTW_MGT-05",
		Query: `UPDATE "GATEWAY" SET NAME = $2, BASE_URL = $3, CLIENT_ID = $4, CLIENT_SECRET = $5, ` +
			`SCOPE = $6 WHERE ID = $1 AND DEPLOYMENT_ID = $7`,
	}
	// queryDeleteGateway removes a gateway.
	queryDeleteGateway = dbmodel.DBQuery{
		ID:    "GTW_MGT-06",
		Query: `DELETE FROM "GATEWAY" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryGetGatewayByName supports the name uniqueness check.
	queryGetGatewayByName = dbmodel.DBQuery{
		ID:    "GTW_MGT-07",
		Query: `SELECT ID FROM "GATEWAY" WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}
)
