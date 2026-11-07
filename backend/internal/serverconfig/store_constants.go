// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// queryGetServerConfigByName retrieves a single server config by name.
	queryGetServerConfigByName = dbmodel.DBQuery{
		ID: "SCF-01",
		Query: `SELECT NAME, VALUE FROM "SERVER_CONFIG" ` +
			`WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryUpsertServerConfig inserts or updates a server config by name.
	queryUpsertServerConfig = dbmodel.DBQuery{
		ID: "SCF-03",
		Query: `INSERT INTO "SERVER_CONFIG" (NAME, VALUE, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3) ` +
			`ON CONFLICT (DEPLOYMENT_ID, NAME) ` +
			`DO UPDATE SET VALUE = EXCLUDED.VALUE, UPDATED_AT = NOW()`,
		SQLiteQuery: `INSERT INTO "SERVER_CONFIG" (NAME, VALUE, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3) ` +
			`ON CONFLICT (DEPLOYMENT_ID, NAME) ` +
			`DO UPDATE SET VALUE = excluded.VALUE, UPDATED_AT = datetime('now')`,
	}

	// queryInsertServerConfigIfAbsent records a section's writable layer only while it has none, so a
	// write that read the section unset loses to one that set it in the meantime.
	queryInsertServerConfigIfAbsent = dbmodel.DBQuery{
		ID: "SCF-04",
		Query: `INSERT INTO "SERVER_CONFIG" (NAME, VALUE, DEPLOYMENT_ID) VALUES ($1, $2, $3) ` +
			`ON CONFLICT (DEPLOYMENT_ID, NAME) DO NOTHING`,
	}

	// queryReplaceServerConfigIf replaces a section's writable layer only while it still holds the value
	// the write read, so of two writes that read the same value only one replaces it.
	queryReplaceServerConfigIf = dbmodel.DBQuery{
		ID: "SCF-05",
		Query: `UPDATE "SERVER_CONFIG" SET VALUE = $1, UPDATED_AT = NOW() ` +
			`WHERE NAME = $2 AND DEPLOYMENT_ID = $3 AND VALUE = $4`,
		SQLiteQuery: `UPDATE "SERVER_CONFIG" SET VALUE = $1, UPDATED_AT = datetime('now') ` +
			`WHERE NAME = $2 AND DEPLOYMENT_ID = $3 AND VALUE = $4`,
	}
)
