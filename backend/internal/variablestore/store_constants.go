// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

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
			`WHERE DEPLOYMENT_ID = $1 ORDER BY NAME`,
	}

	queryDeleteVariable = dbmodel.DBQuery{
		ID:    "VARQ-04",
		Query: `DELETE FROM "VARIABLE" WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
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
			`WHERE DEPLOYMENT_ID = $1 ORDER BY NAME`,
	}

	queryDeleteSecret = dbmodel.DBQuery{
		ID:    "VARQ-08",
		Query: `DELETE FROM "SECRET" WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}
)
