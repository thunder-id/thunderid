// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// sqliteValueTables are the variable and secret tables as dbscripts/configdb/sqlite.sql creates them.
const sqliteValueTables = `
CREATE TABLE "VARIABLE" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    NAME          VARCHAR(255) NOT NULL,
    VALUE         TEXT         NOT NULL,
    DESCRIPTION   TEXT,
    CREATED_AT    TEXT         DEFAULT (datetime('now')),
    UPDATED_AT    TEXT         DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, NAME)
);
CREATE TABLE "SECRET" (
    DEPLOYMENT_ID VARCHAR(255) NOT NULL,
    NAME          VARCHAR(255) NOT NULL,
    VALUE         TEXT         NOT NULL,
    DESCRIPTION   TEXT,
    CREATED_AT    TEXT         DEFAULT (datetime('now')),
    UPDATED_AT    TEXT         DEFAULT (datetime('now')),
    PRIMARY KEY (DEPLOYMENT_ID, NAME)
);`

func openSQLiteValueStore(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	// One connection, so every statement sees the same in-memory database.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	_, err = db.Exec(sqliteValueTables)
	require.NoError(t, err)
	return db
}

// deleteOnSQLite runs a delete of many names on SQLite as the store does and returns the names removed.
func deleteOnSQLite(t *testing.T, db *sql.DB, query string, names string, age interface{}) []string {
	t.Helper()
	rows, err := db.Query(query, names, testDeploymentID, age)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()
	var deleted []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		deleted = append(deleted, name)
	}
	require.NoError(t, rows.Err())
	return deleted
}

// sqliteNow is the database's own second, the one UPDATED_AT is written in.
func sqliteNow(t *testing.T, db *sql.DB) string {
	t.Helper()
	var now string
	require.NoError(t, db.QueryRow(`SELECT datetime('now')`).Scan(&now))
	return now
}

// sqliteValueTable is one of the two tables with the fixed statements a test runs against it.
type sqliteValueTable struct {
	name, query, clear, insert, insertUnset, insertOther, countOther string
}

// The SQLite delete measures a value's age on the database's clock and removes only one last set
// strictly longer ago than the age: a value set in the boundary's own second, one set just now, and
// one with no UPDATED_AT all stay, for variables and secrets alike.
func TestSQLiteDeleteKeepsAValueNotOlderThanTheAge(t *testing.T) {
	for _, table := range []sqliteValueTable{
		{
			name:  "VARIABLE",
			query: queryDeleteVariables.SQLiteQuery,
			clear: `DELETE FROM "VARIABLE"`,
			insert: `INSERT INTO "VARIABLE" (DEPLOYMENT_ID, NAME, VALUE, UPDATED_AT) ` +
				`VALUES ($1, $2, 'v', datetime('now', $3))`,
			insertUnset: `INSERT INTO "VARIABLE" (DEPLOYMENT_ID, NAME, VALUE, UPDATED_AT) ` +
				`VALUES ($1, 'UNSET', 'v', NULL)`,
			insertOther: `INSERT INTO "VARIABLE" (DEPLOYMENT_ID, NAME, VALUE, UPDATED_AT) ` +
				`VALUES ('other', 'OLD', 'v', datetime('now', '-1 hours'))`,
			countOther: `SELECT COUNT(*) FROM "VARIABLE" WHERE DEPLOYMENT_ID = 'other'`,
		},
		{
			name:  "SECRET",
			query: queryDeleteSecrets.SQLiteQuery,
			clear: `DELETE FROM "SECRET"`,
			insert: `INSERT INTO "SECRET" (DEPLOYMENT_ID, NAME, VALUE, UPDATED_AT) ` +
				`VALUES ($1, $2, 'v', datetime('now', $3))`,
			insertUnset: `INSERT INTO "SECRET" (DEPLOYMENT_ID, NAME, VALUE, UPDATED_AT) ` +
				`VALUES ($1, 'UNSET', 'v', NULL)`,
			insertOther: `INSERT INTO "SECRET" (DEPLOYMENT_ID, NAME, VALUE, UPDATED_AT) ` +
				`VALUES ('other', 'OLD', 'v', datetime('now', '-1 hours'))`,
			countOther: `SELECT COUNT(*) FROM "SECRET" WHERE DEPLOYMENT_ID = 'other'`,
		},
	} {
		t.Run(table.name, func(t *testing.T) {
			db := openSQLiteValueStore(t)
			// The rows and the delete must fall in one second of the database's clock for the boundary
			// to be exact, so the round is repeated until they do.
			for attempt := 0; attempt < 5; attempt++ {
				_, err := db.Exec(table.clear)
				require.NoError(t, err)
				before := sqliteNow(t, db)
				for name, offset := range map[string]string{
					"OLD": "-61 seconds", "BOUNDARY": "-60 seconds", "RECENT": "-1 seconds", "NOW": "+0 seconds",
				} {
					_, err := db.Exec(table.insert, testDeploymentID, name, offset)
					require.NoError(t, err)
				}
				_, err = db.Exec(table.insertUnset, testDeploymentID)
				require.NoError(t, err)
				_, err = db.Exec(table.insertOther)
				require.NoError(t, err)

				deleted := deleteOnSQLite(t, db, table.query,
					`["OLD","BOUNDARY","RECENT","NOW","UNSET","MISSING"]`, int64(60))
				if sqliteNow(t, db) != before {
					continue
				}
				require.Equal(t, []string{"OLD"}, deleted)

				// An age of one second keeps a value set in the current second, the one a value set again
				// while the caller decided was written in. Read only while the clock has not moved on.
				kept := deleteOnSQLite(t, db, table.query, `["NOW"]`, int64(1))
				if sqliteNow(t, db) != before {
					continue
				}
				require.Empty(t, kept)
				// Without an age every named value of the deployment goes, and another deployment's stays.
				require.ElementsMatch(t, []string{"BOUNDARY", "RECENT", "NOW", "UNSET"},
					deleteOnSQLite(t, db, table.query, `["BOUNDARY","RECENT","NOW","UNSET","OLD"]`, nil))
				var others int
				require.NoError(t, db.QueryRow(table.countOther).Scan(&others))
				require.Equal(t, 1, others)
				return
			}
			t.Fatal("the database's clock moved on during every attempt")
		})
	}
}
