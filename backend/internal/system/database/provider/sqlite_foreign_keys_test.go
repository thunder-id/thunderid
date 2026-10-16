// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// The DSN turns foreign keys on whatever options are configured, joins them to a path that already
// carries a query, and does not repeat a pragma the options or the path already carry.
func TestSQLiteDSNAddsForeignKeys(t *testing.T) {
	for _, tc := range []struct{ path, configured, want string }{
		{"/srv/db.db", "", "/srv/db.db?_pragma=foreign_keys(1)"},
		{"/srv/db.db", "cache=shared", "/srv/db.db?cache=shared&_pragma=foreign_keys(1)"},
		{"/srv/db.db", "?cache=shared", "/srv/db.db?cache=shared&_pragma=foreign_keys(1)"},
		{"/srv/db.db", "_pragma=foreign_keys(1)&cache=shared", "/srv/db.db?_pragma=foreign_keys(1)&cache=shared"},
		{"/srv/db.db", "?_pragma=busy_timeout(5000)", "/srv/db.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"},
		{"file::memory:?cache=shared", "", "file::memory:?cache=shared&_pragma=foreign_keys(1)"},
		{"file::memory:?cache=shared", "mode=memory", "file::memory:?cache=shared&mode=memory&_pragma=foreign_keys(1)"},
		{"file::memory:?_pragma=foreign_keys(0)", "", "file::memory:?_pragma=foreign_keys(0)"},
	} {
		assert.Equal(t, tc.want, sqliteDSN(tc.path, tc.configured), tc.path+" "+tc.configured)
	}
}

func TestSQLiteDSNCarriesTheForeignKeysPragma(t *testing.T) {
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("/srv/thunderid", &config.Config{}))
	defer config.ResetServerRuntime()

	dbConfig := (&dbProvider{}).getDBConfig(config.DataSource{
		Type:   dataSourceTypeSQLite,
		SQLite: config.SQLiteDataSource{Path: "database/configdb.db", Options: "cache=shared"},
	})

	assert.Equal(t, dataSourceTypeSQLite, dbConfig.driverName)
	assert.Equal(t, "/srv/thunderid/database/configdb.db?cache=shared&_pragma=foreign_keys(1)", dbConfig.dsn)
}

// Every connection the pool opens enforces foreign keys, so a cascade fires whichever connection
// runs the delete. Each pinned connection is a distinct one, as the pool opens as many as are held.
func TestEveryPooledSQLiteConnectionEnforcesForeignKeys(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open(dataSourceTypeSQLite, sqliteDSN(filepath.Join(t.TempDir(), "fk.db"), "cache=shared"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(3)
	_, err = db.ExecContext(ctx, `CREATE TABLE parent (id TEXT PRIMARY KEY);
		CREATE TABLE child (id TEXT PRIMARY KEY, parent_id TEXT NOT NULL,
			FOREIGN KEY (parent_id) REFERENCES parent (id) ON DELETE CASCADE)`)
	require.NoError(t, err)

	conns := make([]*sql.Conn, 3)
	for i := range conns {
		conns[i], err = db.Conn(ctx)
		require.NoError(t, err)
		t.Cleanup(func() { _ = conns[i].Close() })
	}
	for i, conn := range conns {
		var enabled int
		require.NoError(t, conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled))
		assert.Equal(t, 1, enabled, "connection %d has foreign keys off", i)

		_, err = conn.ExecContext(ctx, "INSERT INTO parent (id) VALUES ('p'); INSERT INTO child VALUES ('c', 'p')")
		require.NoError(t, err)
		_, err = conn.ExecContext(ctx, "DELETE FROM parent WHERE id = 'p'")
		require.NoError(t, err)
		var children int
		require.NoError(t, conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM child").Scan(&children))
		assert.Equal(t, 0, children, "connection %d left the child behind", i)
	}
}

// A path that merely spells the pragma is not taken for one; only a _pragma parameter counts.
func TestSQLiteDSNReadsThePragmaFromTheParametersOnly(t *testing.T) {
	dsn := sqliteDSN("/data/_pragma=foreign_keys.db", "")
	assert.Equal(t, "/data/_pragma=foreign_keys.db?"+sqliteForeignKeysPragma, dsn)
	assert.Equal(t, "/data/app.db?_pragma=foreign_keys(0)", sqliteDSN("/data/app.db?_pragma=foreign_keys(0)", ""),
		"a pragma already carried in the path is kept as it is")
	assert.Equal(t, "/data/app.db?_pragma=busy_timeout(5000)&"+sqliteForeignKeysPragma,
		sqliteDSN("/data/app.db", "?_pragma=busy_timeout(5000)"))
}
