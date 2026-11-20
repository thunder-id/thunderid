// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

// The narrowing and paging a listing applies, shared by both collections. These are fixed text
// joined at compile time, never built from a request: every optional part is always present and is
// switched off by its parameter rather than left out of the statement.
//
// Parameters, the same for the list and the count:
//
//	$1 the deployment
//	$2 the names= lookup as a JSON array of strings, or NULL when the request named none
//	$3 the exact name from a filter, or empty when there is none
//	$4 the prefix from a filter, or empty when there is none
//	$5 that prefix as a LIKE pattern, its %, _ and \ escaped and a % appended, or empty with no prefix
//
// and for the list only, $6 the limit and $7 the offset.
//
// SQLite's LIKE ignores ASCII case, so there the prefix is also compared exactly with substr. Order
// is by the name's bytes on both, which is what the listing has always promised.
const (
	listWherePostgres = `WHERE DEPLOYMENT_ID = $1 ` +
		`AND ($2::json IS NULL OR NAME IN (SELECT json_array_elements_text($2::json))) ` +
		`AND ($3::text = '' OR NAME = $3::text) ` +
		`AND ($4::text = '' OR NAME LIKE $5::text ESCAPE E'\\') `
	listPagePostgres = `ORDER BY NAME COLLATE "C" LIMIT $6 OFFSET $7`

	listWhereSQLite = `WHERE DEPLOYMENT_ID = $1 ` +
		`AND ($2 IS NULL OR NAME IN (SELECT value FROM json_each($2))) ` +
		`AND ($3 = '' OR NAME = $3) ` +
		`AND ($4 = '' OR (NAME LIKE $5 ESCAPE '\' AND substr(NAME, 1, length($4)) = $4)) `
	listPageSQLite = `ORDER BY NAME LIMIT $6 OFFSET $7`

	// The rest of a delete of many names after the names: the deployment, $2, and how many whole
	// seconds ago a value must have been last set before to be removed, $3, or NULL for any time. The
	// age is measured on the database's own clock, the one UPDATED_AT is written by, so the caller's
	// clock plays no part. The comparison is strict: a row last set exactly at the boundary, or with
	// no UPDATED_AT, stays.
	//
	// SQLite writes UPDATED_AT with datetime('now'), whole seconds as YYYY-MM-DD HH:MM:SS text, and
	// the boundary is computed in the same form. A value stored as second S was set at some time in
	// [S, S+1), so S < floor(now - age) means it was set more than age seconds ago: one set in the
	// boundary's own second is kept.
	deleteBoundPostgres = `AND DEPLOYMENT_ID = $2 ` +
		`AND ($3::bigint IS NULL OR UPDATED_AT < NOW() - make_interval(secs => $3::bigint)) `
	deleteBoundSQLite = `AND DEPLOYMENT_ID = $2 ` +
		`AND ($3 IS NULL OR datetime(UPDATED_AT) < datetime('now', '-' || $3 || ' seconds')) `
)

// Queries for both collections. They are written per table rather than parameterised by table name,
// because a table name cannot be a bind parameter and building one by string would be the first step
// toward reading a secret through a query meant for variables.
var (
	queryGetVariable = dbmodel.DBQuery{
		ID: "VARQ-01",
		Query: `SELECT NAME, VALUE, DESCRIPTION, CREATED_AT, UPDATED_AT FROM "VARIABLE" ` +
			`WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryInsertVariable = dbmodel.DBQuery{
		ID: "VARQ-02",
		Query: `INSERT INTO "VARIABLE" (NAME, VALUE, DESCRIPTION, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4) ` +
			`ON CONFLICT (DEPLOYMENT_ID, NAME) DO NOTHING`,
	}

	// queryUpsertVariable inserts only when the name is free, and reports whether it did.
	//
	// Paired with queryUpdateVariable it makes a create-or-replace whose answer comes from the
	// database rather than from a read taken beforehand. Neither statement on its own is the whole
	// operation: see the store, which repeats the pair when a concurrent delete leaves both of them
	// with nothing to do.
	queryUpsertVariable = dbmodel.DBQuery{
		ID: "VARQ-03",
		Query: `INSERT INTO "VARIABLE" (NAME, VALUE, DESCRIPTION, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4) ` +
			`ON CONFLICT (DEPLOYMENT_ID, NAME) DO NOTHING`,
	}

	// queryUpdateVariable replaces a variable that the insert above found already present.
	queryUpdateVariable = dbmodel.DBQuery{
		ID: "VARQ-11",
		Query: `UPDATE "VARIABLE" SET VALUE = $2, DESCRIPTION = $3, UPDATED_AT = NOW() ` +
			`WHERE NAME = $1 AND DEPLOYMENT_ID = $4`,
		SQLiteQuery: `UPDATE "VARIABLE" SET VALUE = $2, DESCRIPTION = $3, ` +
			`UPDATED_AT = datetime('now') WHERE NAME = $1 AND DEPLOYMENT_ID = $4`,
	}

	queryListVariables = dbmodel.DBQuery{
		ID: "VARQ-09",
		Query: `SELECT NAME, VALUE, DESCRIPTION, CREATED_AT, UPDATED_AT FROM "VARIABLE" ` +
			listWherePostgres + listPagePostgres,
		SQLiteQuery: `SELECT NAME, VALUE, DESCRIPTION, CREATED_AT, UPDATED_AT FROM "VARIABLE" ` +
			listWhereSQLite + listPageSQLite,
	}

	queryCountVariables = dbmodel.DBQuery{
		ID:          "VARQ-14",
		Query:       `SELECT COUNT(*) AS total FROM "VARIABLE" ` + listWherePostgres,
		SQLiteQuery: `SELECT COUNT(*) AS total FROM "VARIABLE" ` + listWhereSQLite,
	}

	queryDeleteVariable = dbmodel.DBQuery{
		ID:    "VARQ-04",
		Query: `DELETE FROM "VARIABLE" WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryDeleteVariables removes every listed variable the deployment holds in one statement and
	// answers with the names it removed. $1 is the names as a JSON array of strings, expanded as a
	// names= listing expands it. $3 is an age in whole seconds, or NULL: when given, only a variable
	// last set longer ago than that, by the database's clock, when the statement runs is removed. A
	// variable set again after the caller read it stays only while it is younger than the age, so a
	// caller that must keep every such change passes an age that covers the time since it read.
	queryDeleteVariables = dbmodel.DBQuery{
		ID: "VARQ-16",
		Query: `DELETE FROM "VARIABLE" WHERE NAME IN (SELECT json_array_elements_text($1::json)) ` +
			deleteBoundPostgres + `RETURNING NAME`,
		SQLiteQuery: `DELETE FROM "VARIABLE" WHERE NAME IN (SELECT value FROM json_each($1)) ` +
			deleteBoundSQLite + `RETURNING NAME`,
	}

	querySecretExists = dbmodel.DBQuery{
		ID: "VARQ-05",
		// The value is deliberately not selected. Existence is all any read of a secret reports, so
		// the ciphertext never leaves the database for a read path at all.
		Query: `SELECT NAME, DESCRIPTION, CREATED_AT, UPDATED_AT FROM "SECRET" ` +
			`WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryInsertSecret = dbmodel.DBQuery{
		ID: "VARQ-06",
		Query: `INSERT INTO "SECRET" (NAME, VALUE, DESCRIPTION, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4) ` +
			`ON CONFLICT (DEPLOYMENT_ID, NAME) DO NOTHING`,
	}

	// queryUpsertSecret inserts only when the name is free, for the same reason as the variable above.
	// The store repeats it with queryUpdateSecret when a concurrent delete leaves neither applicable.
	queryUpsertSecret = dbmodel.DBQuery{
		ID: "VARQ-07",
		Query: `INSERT INTO "SECRET" (NAME, VALUE, DESCRIPTION, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4) ` +
			`ON CONFLICT (DEPLOYMENT_ID, NAME) DO NOTHING`,
	}

	// queryUpdateSecret rotates a secret that the insert above found already present.
	queryUpdateSecret = dbmodel.DBQuery{
		ID: "VARQ-12",
		Query: `UPDATE "SECRET" SET VALUE = $2, DESCRIPTION = $3, UPDATED_AT = NOW() ` +
			`WHERE NAME = $1 AND DEPLOYMENT_ID = $4`,
		SQLiteQuery: `UPDATE "SECRET" SET VALUE = $2, DESCRIPTION = $3, ` +
			`UPDATED_AT = datetime('now') WHERE NAME = $1 AND DEPLOYMENT_ID = $4`,
	}

	queryListSecrets = dbmodel.DBQuery{
		ID: "VARQ-10",
		// No VALUE column: a listing of secrets reports names, never contents.
		Query: `SELECT NAME, DESCRIPTION, CREATED_AT, UPDATED_AT FROM "SECRET" ` +
			listWherePostgres + listPagePostgres,
		SQLiteQuery: `SELECT NAME, DESCRIPTION, CREATED_AT, UPDATED_AT FROM "SECRET" ` +
			listWhereSQLite + listPageSQLite,
	}

	queryCountSecrets = dbmodel.DBQuery{
		ID:          "VARQ-15",
		Query:       `SELECT COUNT(*) AS total FROM "SECRET" ` + listWherePostgres,
		SQLiteQuery: `SELECT COUNT(*) AS total FROM "SECRET" ` + listWhereSQLite,
	}

	queryDeleteSecret = dbmodel.DBQuery{
		ID:    "VARQ-08",
		Query: `DELETE FROM "SECRET" WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryDeleteSecrets removes every listed secret, as queryDeleteVariables does variables.
	queryDeleteSecrets = dbmodel.DBQuery{
		ID: "VARQ-17",
		Query: `DELETE FROM "SECRET" WHERE NAME IN (SELECT json_array_elements_text($1::json)) ` +
			deleteBoundPostgres + `RETURNING NAME`,
		SQLiteQuery: `DELETE FROM "SECRET" WHERE NAME IN (SELECT value FROM json_each($1)) ` +
			deleteBoundSQLite + `RETURNING NAME`,
	}

	// querySecretValue reads a secret's ciphertext. Only the import's reference resolution uses it, to put
	// the value where a reference stood; no read served over HTTP selects the value column.
	querySecretValue = dbmodel.DBQuery{
		ID:    "VARQ-13",
		Query: `SELECT VALUE FROM "SECRET" WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}
)
