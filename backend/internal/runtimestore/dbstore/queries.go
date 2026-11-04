// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package dbstore

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

// columnNameValue is the lowercased result-set key for the VALUE column.
const columnNameValue = "value"

// queryPutRuntimeStore upserts an entry, overwriting the value and resetting the TTL on conflict.
var queryPutRuntimeStore = dbmodel.DBQuery{
	ID: "RTS-01",
	Query: `INSERT INTO "RUNTIME_STORE" (DEPLOYMENT_ID, NAMESPACE, KEY, VALUE, EXPIRY_TIME) ` +
		`VALUES ($1, $2, $3, $4, $5) ` +
		`ON CONFLICT (DEPLOYMENT_ID, NAMESPACE, KEY) ` +
		`DO UPDATE SET VALUE = EXCLUDED.VALUE, EXPIRY_TIME = EXCLUDED.EXPIRY_TIME, UPDATED_AT = CURRENT_TIMESTAMP`,
}

// queryGetRuntimeStore fetches a non-expired value.
var queryGetRuntimeStore = dbmodel.DBQuery{
	ID: "RTS-02",
	Query: `SELECT VALUE FROM "RUNTIME_STORE" ` +
		`WHERE DEPLOYMENT_ID = $1 AND NAMESPACE = $2 AND KEY = $3 ` +
		`AND (EXPIRY_TIME IS NULL OR EXPIRY_TIME > $4)`,
}

// queryUpdateRuntimeStore replaces the value of an existing, non-expired entry, preserving its TTL.
var queryUpdateRuntimeStore = dbmodel.DBQuery{
	ID: "RTS-03",
	Query: `UPDATE "RUNTIME_STORE" SET VALUE = $4, UPDATED_AT = CURRENT_TIMESTAMP ` +
		`WHERE DEPLOYMENT_ID = $1 AND NAMESPACE = $2 AND KEY = $3 ` +
		`AND (EXPIRY_TIME IS NULL OR EXPIRY_TIME > $5)`,
}

// queryDeleteRuntimeStore removes an entry. Used by Delete.
var queryDeleteRuntimeStore = dbmodel.DBQuery{
	ID:    "RTS-04",
	Query: `DELETE FROM "RUNTIME_STORE" WHERE DEPLOYMENT_ID = $1 AND NAMESPACE = $2 AND KEY = $3`,
}

// queryTakeRuntimeStore atomically deletes a non-expired entry and returns its value in a single
// statement, so a concurrent writer cannot slip a new value in between the read and the delete.
var queryTakeRuntimeStore = dbmodel.DBQuery{
	ID: "RTS-05",
	Query: `DELETE FROM "RUNTIME_STORE" ` +
		`WHERE DEPLOYMENT_ID = $1 AND NAMESPACE = $2 AND KEY = $3 ` +
		`AND (EXPIRY_TIME IS NULL OR EXPIRY_TIME > $4) ` +
		`RETURNING VALUE`,
}

// queryExtendTTLRuntimeStore extends the TTL of an existing, non-expired entry.
var queryExtendTTLRuntimeStore = dbmodel.DBQuery{
	ID: "RTS-06",
	Query: `UPDATE "RUNTIME_STORE" SET EXPIRY_TIME = $4, UPDATED_AT = CURRENT_TIMESTAMP ` +
		`WHERE DEPLOYMENT_ID = $1 AND NAMESPACE = $2 AND KEY = $3 ` +
		`AND (EXPIRY_TIME IS NULL OR EXPIRY_TIME > $5)`,
}

// queryPutIfNotExistsRuntimeStore inserts an entry, or overwrites it in place if the existing entry
// has already expired. The conflicting row is left untouched (and no row is returned) when it is
// still live, so the caller can tell a fresh claim from a blocked one by whether a row came back.
var queryPutIfNotExistsRuntimeStore = dbmodel.DBQuery{
	ID: "RTS-07",
	Query: `INSERT INTO "RUNTIME_STORE" (DEPLOYMENT_ID, NAMESPACE, KEY, VALUE, EXPIRY_TIME) ` +
		`VALUES ($1, $2, $3, $4, $5) ` +
		`ON CONFLICT (DEPLOYMENT_ID, NAMESPACE, KEY) ` +
		`DO UPDATE SET VALUE = EXCLUDED.VALUE, EXPIRY_TIME = EXCLUDED.EXPIRY_TIME, UPDATED_AT = CURRENT_TIMESTAMP ` +
		`WHERE "RUNTIME_STORE".EXPIRY_TIME IS NOT NULL AND "RUNTIME_STORE".EXPIRY_TIME <= $6 ` +
		`RETURNING KEY`,
}

// queryCompareFieldAndSwapRuntimeStore replaces the value of a non-expired entry, but only when the
// top-level JSON string field named by $6 in the stored value equals $7, preserving its TTL. The
// field name and expected value are bind parameters; the JSON extraction differs by dialect.
var queryCompareFieldAndSwapRuntimeStore = dbmodel.DBQuery{
	ID: "RTS-08",
	PostgresQuery: `UPDATE "RUNTIME_STORE" SET VALUE = $4, UPDATED_AT = CURRENT_TIMESTAMP ` +
		`WHERE DEPLOYMENT_ID = $1 AND NAMESPACE = $2 AND KEY = $3 ` +
		`AND (EXPIRY_TIME IS NULL OR EXPIRY_TIME > $5) ` +
		`AND (VALUE ->> $6) = $7`,
	SQLiteQuery: `UPDATE "RUNTIME_STORE" SET VALUE = $4, UPDATED_AT = CURRENT_TIMESTAMP ` +
		`WHERE DEPLOYMENT_ID = $1 AND NAMESPACE = $2 AND KEY = $3 ` +
		`AND (EXPIRY_TIME IS NULL OR EXPIRY_TIME > $5) ` +
		`AND json_extract(VALUE, '$.' || $6) = $7`,
}

// queryCompareFieldAndSwapBeforeRuntimeStoreExpiry replaces a value only when its JSON field
// matches and the supplied application-level expiry is still in the future.
var queryCompareFieldAndSwapBeforeRuntimeStoreExpiry = dbmodel.DBQuery{
	ID: "RTS-09",
	PostgresQuery: `UPDATE "RUNTIME_STORE" SET VALUE = $4, UPDATED_AT = CURRENT_TIMESTAMP ` +
		`WHERE DEPLOYMENT_ID = $1 AND NAMESPACE = $2 AND KEY = $3 ` +
		`AND (EXPIRY_TIME IS NULL OR EXPIRY_TIME > $5) ` +
		`AND (VALUE ->> $6) = $7 AND $8 > $5`,
	SQLiteQuery: `UPDATE "RUNTIME_STORE" SET VALUE = $4, UPDATED_AT = CURRENT_TIMESTAMP ` +
		`WHERE DEPLOYMENT_ID = $1 AND NAMESPACE = $2 AND KEY = $3 ` +
		`AND (EXPIRY_TIME IS NULL OR EXPIRY_TIME > $5) ` +
		`AND json_extract(VALUE, '$.' || $6) = $7 ` +
		`AND $8 > $5`,
}
