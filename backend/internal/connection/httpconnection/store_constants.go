// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

const (
	// nameIndexName is the unique index on a connection's name, and nameIndexColumns its column
	// list as SQLite reports it.
	nameIndexName    = "idx_http_connection_name_deployment"
	nameIndexColumns = "HTTP_CONNECTION.DEPLOYMENT_ID, HTTP_CONNECTION.NAME"
	// primaryKeyName is the name PostgreSQL gives the table's primary key, and primaryKeyColumns its
	// column list as SQLite reports it.
	primaryKeyName    = "HTTP_CONNECTION_pkey"
	primaryKeyColumns = "HTTP_CONNECTION.DEPLOYMENT_ID, HTTP_CONNECTION.ID"
)

var (
	queryCreateHTTPConnection = dbmodel.DBQuery{
		ID: "HCQ-HTTP_CONN_MGT-01",
		Query: `INSERT INTO "HTTP_CONNECTION"
			(ID, NAME, DESCRIPTION, OU_ID, PROPERTIES, DEPLOYMENT_ID)
			VALUES ($1, $2, $3, $4, $5, $6)`,
	}

	queryGetHTTPConnectionByID = dbmodel.DBQuery{
		ID: "HCQ-HTTP_CONN_MGT-02",
		Query: `SELECT ID, NAME, DESCRIPTION, OU_ID, PROPERTIES
			FROM "HTTP_CONNECTION"
			WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryListHTTPConnections = dbmodel.DBQuery{
		ID: "HCQ-HTTP_CONN_MGT-03",
		Query: `SELECT ID, NAME, DESCRIPTION, OU_ID, PROPERTIES
			FROM "HTTP_CONNECTION"
			WHERE DEPLOYMENT_ID = $1
			ORDER BY NAME ASC, ID ASC`,
	}

	queryUpdateHTTPConnection = dbmodel.DBQuery{
		ID: "HCQ-HTTP_CONN_MGT-04",
		PostgresQuery: `UPDATE "HTTP_CONNECTION"
			SET NAME = $1, DESCRIPTION = $2, PROPERTIES = $3, UPDATED_AT = NOW()
			WHERE ID = $4 AND DEPLOYMENT_ID = $5`,
		Query: `UPDATE "HTTP_CONNECTION"
			SET NAME = $1, DESCRIPTION = $2, PROPERTIES = $3, UPDATED_AT = datetime('now')
			WHERE ID = $4 AND DEPLOYMENT_ID = $5`,
	}

	queryDeleteHTTPConnection = dbmodel.DBQuery{
		ID: "HCQ-HTTP_CONN_MGT-05",
		Query: `DELETE FROM "HTTP_CONNECTION"
			WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryCountHTTPConnections = dbmodel.DBQuery{
		ID: "HCQ-HTTP_CONN_MGT-06",
		Query: `SELECT COUNT(*) AS count FROM "HTTP_CONNECTION"
			WHERE DEPLOYMENT_ID = $1`,
	}

	queryGetHTTPConnectionByName = dbmodel.DBQuery{
		ID: "HCQ-HTTP_CONN_MGT-07",
		Query: `SELECT ID, NAME, DESCRIPTION, OU_ID, PROPERTIES
			FROM "HTTP_CONNECTION"
			WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}
)
