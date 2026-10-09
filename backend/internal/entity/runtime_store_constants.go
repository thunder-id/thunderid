// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"strings"
	"time"

	governancemodel "github.com/thunder-id/thunderid/internal/identitygovernance/model"

	"github.com/thunder-id/thunderid/internal/system/database/model"
)

const (
	// runtimeBatchSize is the most runtime rows one statement reads or seeds.
	runtimeBatchSize = 100

	// sqliteAccessStateRoot and pgAccessStateKey address the access-state document in each dialect.
	sqliteAccessStateRoot = `$.accessState`
	pgAccessStateKey      = `accessState`

	// sqliteEntityHoldUnlockAt and pgEntityHoldUnlockAt address the entity-wide lock's unlockAt.
	sqliteEntityHoldUnlockAt = `'` + sqliteAccessStateRoot + `.lock.entity.unlockAt'`
	pgEntityHoldUnlockAt     = `RUNTIME_ATTRIBUTES->'` + pgAccessStateKey + `'->'lock'->'entity'->>'unlockAt'`
)

// Each access-state statement updates one path of the document, so a write to one member never
// replaces another.
var (
	// queryResetRuntimeData writes the empty runtime row a newly created entity starts with. It
	// replaces an orphan row left at the same id, which is safe because the entity insert runs first
	// and fails on a duplicate id.
	queryResetRuntimeData = model.DBQuery{
		ID: "ERD-ENTITY_MGT-03",
		Query: `INSERT INTO "ENTITY_RUNTIME_DATA" ` +
			`(ENTITY_ID, STATE, RUNTIME_ATTRIBUTES, CREATED_AT, UPDATED_AT, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, '{}', $3, $3, $4) ON CONFLICT (DEPLOYMENT_ID, ENTITY_ID) DO UPDATE SET ` +
			`STATE = excluded.STATE, RUNTIME_ATTRIBUTES = '{}', ` +
			`REVISION = "ENTITY_RUNTIME_DATA".REVISION + 1, ` +
			`CREATED_AT = excluded.CREATED_AT, UPDATED_AT = excluded.UPDATED_AT`,
	}

	// queryDeleteRuntimeData deletes an entity's runtime row.
	queryDeleteRuntimeData = model.DBQuery{
		ID:    "ERD-ENTITY_MGT-04",
		Query: `DELETE FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// querySetAccessSuspension sets the state to SUSPENDED and writes accessState.suspend.
	// $2 is when the suspension was placed and $3 the operator's note, possibly empty.
	querySetAccessSuspension = model.DBQuery{
		ID: "ERD-ENTITY_MGT-09",
		Query: `UPDATE "ENTITY_RUNTIME_DATA" SET STATE = 'SUSPENDED', RUNTIME_ATTRIBUTES = ` +
			`COALESCE(RUNTIME_ATTRIBUTES, '{}'::jsonb) || jsonb_build_object('accessState', ` +
			`COALESCE(RUNTIME_ATTRIBUTES->'accessState', '{}'::jsonb) || jsonb_build_object('suspend', ` +
			`jsonb_build_object('suspendedAt', $2::text, 'operatorNote', $3::text))), ` +
			`REVISION = REVISION + 1, UPDATED_AT = $4 ` +
			`WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $5`,
		SQLiteQuery: `UPDATE "ENTITY_RUNTIME_DATA" SET STATE = 'SUSPENDED', RUNTIME_ATTRIBUTES = json_set(` +
			`COALESCE(NULLIF(RUNTIME_ATTRIBUTES, ''), '{}'), ` +
			`'$.accessState.suspend', json_object('suspendedAt', $2, 'operatorNote', $3)), ` +
			`REVISION = REVISION + 1, UPDATED_AT = $4 ` +
			`WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $5`,
	}

	// queryClearAccessSuspension removes the suspension and sets the state to ACTIVE, if the
	// suspension is still the one placed at $3.
	queryClearAccessSuspension = model.DBQuery{
		ID: "ERD-ENTITY_MGT-10",
		Query: `UPDATE "ENTITY_RUNTIME_DATA" SET STATE = 'ACTIVE', RUNTIME_ATTRIBUTES = ` +
			`COALESCE(RUNTIME_ATTRIBUTES, '{}'::jsonb) || jsonb_build_object('accessState', ` +
			`COALESCE(RUNTIME_ATTRIBUTES->'accessState', '{}'::jsonb) - 'suspend'), ` +
			`REVISION = REVISION + 1, UPDATED_AT = $2 ` +
			`WHERE ENTITY_ID = $1 ` +
			`AND COALESCE(RUNTIME_ATTRIBUTES->'accessState'->'suspend'->>'suspendedAt', '') = $3 ` +
			`AND DEPLOYMENT_ID = $4`,
		SQLiteQuery: `UPDATE "ENTITY_RUNTIME_DATA" SET STATE = 'ACTIVE', RUNTIME_ATTRIBUTES = json_remove(` +
			`COALESCE(NULLIF(RUNTIME_ATTRIBUTES, ''), '{}'), ` +
			`'$.accessState.suspend'), REVISION = REVISION + 1, UPDATED_AT = $2 ` +
			`WHERE ENTITY_ID = $1 AND ` +
			`COALESCE(json_extract(RUNTIME_ATTRIBUTES, '$.accessState.suspend.suspendedAt'), '') = $3 ` +
			`AND DEPLOYMENT_ID = $4`,
	}

	// queryClearAccessSuspensionWithHold removes the suspension and replaces the lock member with an
	// entity-wide lock in one statement. $2 is the unlockAt value and $3 the reason.
	queryClearAccessSuspensionWithHold = model.DBQuery{
		ID: "ERD-ENTITY_MGT-11",
		Query: `UPDATE "ENTITY_RUNTIME_DATA" SET STATE = 'ACTIVE', RUNTIME_ATTRIBUTES = ` +
			`COALESCE(RUNTIME_ATTRIBUTES, '{}'::jsonb) || jsonb_build_object('accessState', ` +
			`(COALESCE(RUNTIME_ATTRIBUTES->'accessState', '{}'::jsonb) - 'suspend') || ` +
			`jsonb_build_object('lock', jsonb_build_object('entity', ` +
			`jsonb_build_object('unlockAt', $2::text, 'reason', $3::text)))), ` +
			`REVISION = REVISION + 1, UPDATED_AT = $4 ` +
			`WHERE ENTITY_ID = $1 ` +
			`AND COALESCE(RUNTIME_ATTRIBUTES->'accessState'->'suspend'->>'suspendedAt', '') = $5 ` +
			`AND DEPLOYMENT_ID = $6`,
		SQLiteQuery: `UPDATE "ENTITY_RUNTIME_DATA" SET STATE = 'ACTIVE', RUNTIME_ATTRIBUTES = json_set(json_remove(` +
			`COALESCE(NULLIF(RUNTIME_ATTRIBUTES, ''), '{}'), '$.accessState.suspend'), ` +
			`'$.accessState.lock', json_object('entity', ` +
			`json_object('unlockAt', $2, 'reason', $3))), REVISION = REVISION + 1, UPDATED_AT = $4 ` +
			`WHERE ENTITY_ID = $1 AND ` +
			`COALESCE(json_extract(RUNTIME_ATTRIBUTES, '$.accessState.suspend.suspendedAt'), '') = $5 ` +
			`AND DEPLOYMENT_ID = $6`,
	}

	// queryRecordAccessLogin writes accessState.lastLoginAt ($2) unless the stored value is already
	// within [$3, $2]. The values are fixed-width RFC 3339, so the comparison is textual.
	queryRecordAccessLogin = model.DBQuery{
		ID: "ERD-ENTITY_MGT-12",
		Query: `UPDATE "ENTITY_RUNTIME_DATA" SET RUNTIME_ATTRIBUTES = ` +
			`COALESCE(RUNTIME_ATTRIBUTES, '{}'::jsonb) || jsonb_build_object('accessState', ` +
			`COALESCE(RUNTIME_ATTRIBUTES->'accessState', '{}'::jsonb) || ` +
			`jsonb_build_object('lastLoginAt', $2::text)), REVISION = REVISION + 1, UPDATED_AT = $4 ` +
			`WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $5 AND NOT (` +
			`COALESCE(RUNTIME_ATTRIBUTES->'accessState'->>'lastLoginAt', '') >= $3::text AND ` +
			`COALESCE(RUNTIME_ATTRIBUTES->'accessState'->>'lastLoginAt', '') <= $2::text)`,
		SQLiteQuery: `UPDATE "ENTITY_RUNTIME_DATA" SET RUNTIME_ATTRIBUTES = json_set(` +
			`COALESCE(NULLIF(RUNTIME_ATTRIBUTES, ''), '{}'), ` +
			`'$.accessState.lastLoginAt', $2), REVISION = REVISION + 1, UPDATED_AT = $4 ` +
			`WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $5 AND NOT (` +
			`COALESCE(json_extract(RUNTIME_ATTRIBUTES, '$.accessState.lastLoginAt'), '') >= $3 AND ` +
			`COALESCE(json_extract(RUNTIME_ATTRIBUTES, '$.accessState.lastLoginAt'), '') <= $2)`,
	}

	// incrementAccessQueries and formAccessQueries hold one statement per registered scope, built at
	// start, so the failed sign-in path builds no SQL.
	incrementAccessQueries = compileScopeQueries(compileIncrementAccessFailureQuery)
	formAccessQueries      = compileScopeQueries(compileFormAccessLockQuery)
)

// accessLockPath is where one scope's lock entry lives in the access-state document, in both
// dialects. The entity-wide entry is lock.entity; a method entry is lock.authenticationMethods.<scope>.
type accessLockPath struct {
	// sqlite is the json_extract / json_set path of the entry, without a trailing field.
	sqlite string
	// pgParent is the Postgres arrow chain to the object holding the entry.
	pgParent string
	// pgEntry is the arrow chain to the entry itself.
	pgEntry string
	// pgKey is the entry's own key, as a SQL string literal.
	pgKey string
	// pgPath is the entry's path as the text array the #- operator takes.
	pgPath string
	// pgWrap wraps a new entry object in the levels above it, keeping every sibling.
	pgWrap func(entry string) string
}

// buildGetRuntimeDataQuery builds the read of the runtime rows for one batch of entity ids.
func buildGetRuntimeDataQuery(ids []string, deploymentID string) (model.DBQuery, []interface{}) {
	args := make([]interface{}, 0, len(ids)+1)
	placeholders := make([]string, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}
	args = append(args, deploymentID)

	query := model.DBQuery{
		ID: "ERD-ENTITY_MGT-01",
		Query: fmt.Sprintf(`SELECT ENTITY_ID, STATE, RUNTIME_ATTRIBUTES, REVISION FROM "ENTITY_RUNTIME_DATA" `+
			`WHERE ENTITY_ID IN (%s) AND DEPLOYMENT_ID = $%d`, strings.Join(placeholders, ","), len(args)),
	}
	return query, args
}

// buildInitializeRuntimeDataQuery builds the insert that seeds missing runtime rows for one batch.
// A row that already exists is left as it is.
func buildInitializeRuntimeDataQuery(rows []entityRuntimeData, now time.Time,
	deploymentID string) (model.DBQuery, []interface{}) {
	args := make([]interface{}, 0, len(rows)*4+1)
	values := make([]string, 0, len(rows))
	deploymentParam := len(rows)*4 + 1
	for _, row := range rows {
		n := len(args)
		args = append(args, row.ID, string(row.State), string(row.Attributes), now)
		values = append(values, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d)",
			n+1, n+2, n+3, n+4, n+4, deploymentParam))
	}
	args = append(args, deploymentID)

	query := model.DBQuery{
		ID: "ERD-ENTITY_MGT-02",
		Query: `INSERT INTO "ENTITY_RUNTIME_DATA" ` +
			`(ENTITY_ID, STATE, RUNTIME_ATTRIBUTES, CREATED_AT, UPDATED_AT, DEPLOYMENT_ID) VALUES ` +
			strings.Join(values, ",") + ` ON CONFLICT (DEPLOYMENT_ID, ENTITY_ID) DO NOTHING`,
	}
	return query, args
}

// buildIncrementAccessFailureQuery returns the increment statement for a scope.
func buildIncrementAccessFailureQuery(scope governancemodel.AccessScope) (model.DBQuery, error) {
	return scopeQuery(incrementAccessQueries, scope)
}

// buildFormAccessLockQuery returns the lock formation statement for a scope.
func buildFormAccessLockQuery(scope governancemodel.AccessScope) (model.DBQuery, error) {
	return scopeQuery(formAccessQueries, scope)
}

// buildClearAccessStateQuery builds the statement that removes the given scopes' lock entries,
// guarded on revision when it is set. It does not touch the suspension.
func buildClearAccessStateQuery(entityID string, scopes []governancemodel.AccessScope, updatedAt time.Time,
	revision *int64, deploymentID string) (model.DBQuery, []interface{}, error) {
	pgPaths := make([]string, 0, len(scopes))
	sqlitePaths := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		path, err := buildAccessLockPath(scope)
		if err != nil {
			return model.DBQuery{}, nil, err
		}
		// #- takes a text[], so the array is cast explicitly.
		pgPaths = append(pgPaths, fmt.Sprintf(` #- ARRAY[%s]::text[]`, path.pgPath))
		sqlitePaths = append(sqlitePaths, `, '`+path.sqlite+`'`)
	}

	args := []interface{}{updatedAt, entityID}
	where := `WHERE ENTITY_ID = $2`
	queryID := "ERD-ENTITY_MGT-07"
	if revision != nil {
		args = append(args, *revision)
		where += ` AND REVISION = $3`
		queryID = "ERD-ENTITY_MGT-08"
	}
	args = append(args, deploymentID)
	where += fmt.Sprintf(" AND DEPLOYMENT_ID = $%d", len(args))

	return model.DBQuery{
		ID: queryID,
		Query: fmt.Sprintf(`UPDATE "ENTITY_RUNTIME_DATA" SET RUNTIME_ATTRIBUTES = `+
			`COALESCE(RUNTIME_ATTRIBUTES, '{}'::jsonb)%s, REVISION = REVISION + 1, UPDATED_AT = $1 %s`,
			strings.Join(pgPaths, ""), where),
		SQLiteQuery: fmt.Sprintf(`UPDATE "ENTITY_RUNTIME_DATA" SET RUNTIME_ATTRIBUTES = `+
			`json_remove(COALESCE(NULLIF(RUNTIME_ATTRIBUTES, ''), '{}')%s), `+
			`REVISION = REVISION + 1, UPDATED_AT = $1 %s`,
			strings.Join(sqlitePaths, ""), where),
	}, args, nil
}

// compileScopeQueries builds one statement per registered scope.
func compileScopeQueries(
	build func(governancemodel.AccessScope) (model.DBQuery, error),
) map[governancemodel.AccessScope]model.DBQuery {
	queries := make(map[governancemodel.AccessScope]model.DBQuery, len(governancemodel.AllAccessScopes))
	for _, scope := range governancemodel.AllAccessScopes {
		if query, err := build(scope); err == nil {
			queries[scope] = query
		}
	}
	return queries
}

// scopeQuery returns a scope's statement, refusing a scope outside the registry.
func scopeQuery(queries map[governancemodel.AccessScope]model.DBQuery,
	scope governancemodel.AccessScope) (model.DBQuery, error) {
	query, ok := queries[scope]
	if !ok {
		return model.DBQuery{}, fmt.Errorf("%w: %q", errInvalidAccessScope, scope)
	}
	return query, nil
}

// buildAccessLockPath resolves a scope to its path. The scope becomes statement text, so a scope
// outside the registry is refused.
func buildAccessLockPath(scope governancemodel.AccessScope) (accessLockPath, error) {
	if !governancemodel.IsAccessScope(scope) {
		return accessLockPath{}, fmt.Errorf("%w: %q", errInvalidAccessScope, scope)
	}

	lockParent := fmt.Sprintf(`RUNTIME_ATTRIBUTES->'%s'->'lock'`, pgAccessStateKey)
	wrapLock := func(inner string) string {
		return fmt.Sprintf(
			`COALESCE(RUNTIME_ATTRIBUTES, '{}'::jsonb) || jsonb_build_object('%s', `+
				`COALESCE(RUNTIME_ATTRIBUTES->'%s', '{}'::jsonb) || jsonb_build_object('lock', `+
				`COALESCE(%s, '{}'::jsonb) || %s))`,
			pgAccessStateKey, pgAccessStateKey, lockParent, inner)
	}

	if scope == governancemodel.AccessScopeEntity {
		return accessLockPath{
			sqlite:   sqliteAccessStateRoot + `.lock.entity`,
			pgParent: lockParent,
			pgEntry:  lockParent + `->'entity'`,
			pgKey:    `'entity'`,
			pgPath:   fmt.Sprintf(`'%s','lock','entity'`, pgAccessStateKey),
			pgWrap: func(entry string) string {
				return wrapLock(fmt.Sprintf(`jsonb_build_object('entity', %s)`, entry))
			},
		}, nil
	}

	authenticationMethods := lockParent + `->'authenticationMethods'`
	return accessLockPath{
		sqlite:   sqliteAccessStateRoot + `.lock.authenticationMethods.` + string(scope),
		pgParent: authenticationMethods,
		pgEntry:  fmt.Sprintf(`%s->'%s'`, authenticationMethods, scope),
		pgKey:    fmt.Sprintf(`'%s'`, scope),
		pgPath:   fmt.Sprintf(`'%s','lock','authenticationMethods','%s'`, pgAccessStateKey, scope),
		pgWrap: func(entry string) string {
			return wrapLock(fmt.Sprintf(
				`jsonb_build_object('authenticationMethods', `+
					`COALESCE(%s, '{}'::jsonb) || jsonb_build_object('%s', %s))`,
				authenticationMethods, scope, entry))
		},
	}, nil
}

// sqliteField is the json path of one field inside the entry.
func (p accessLockPath) sqliteField(field string) string {
	return `'` + p.sqlite + `.` + field + `'`
}

// pgField is the Postgres text extraction of one field inside the entry.
func (p accessLockPath) pgField(field string) string {
	return fmt.Sprintf(`%s->>'%s'`, p.pgEntry, field)
}

// noLiveHoldSQLite skips the write while a live entity-wide lock or a live lock on this scope exists.
func noLiveHoldSQLite(path accessLockPath, nowParam string) string {
	return liveHoldExclusionSQLite(sqliteEntityHoldUnlockAt, nowParam) + ` ` +
		liveHoldExclusionSQLite(path.sqliteField("unlockAt"), nowParam)
}

// noLiveHoldPostgres is noLiveHoldSQLite for Postgres.
func noLiveHoldPostgres(path accessLockPath, nowParam string) string {
	return liveHoldExclusionPostgres(pgEntityHoldUnlockAt, nowParam) + ` ` +
		liveHoldExclusionPostgres(path.pgField("unlockAt"), nowParam)
}

// liveHoldExclusionSQLite excludes rows whose unlockAt at the path is in the future. An absent value
// excludes nothing.
func liveHoldExclusionSQLite(unlockAtPath, nowParam string) string {
	return fmt.Sprintf(
		`AND NOT (COALESCE(json_extract(RUNTIME_ATTRIBUTES, %s), '') <> '' `+
			`AND json_extract(RUNTIME_ATTRIBUTES, %s) > %s)`,
		unlockAtPath, unlockAtPath, nowParam)
}

// liveHoldExclusionPostgres is liveHoldExclusionSQLite for Postgres, compared as timestamps.
func liveHoldExclusionPostgres(unlockAtExpr, nowParam string) string {
	return fmt.Sprintf(
		`AND COALESCE((%s)::timestamptz, 'epoch'::timestamptz) <= %s::timestamptz`,
		unlockAtExpr, nowParam)
}

// compileIncrementAccessFailureQuery builds the statement that records one failed attempt and
// returns the resulting counters. The count restarts at 1 outside the failure window and on the first
// failure after a lock expired.
//
// Parameters: $1 entity id, $2 window start, $3 now (compared), $4 now (stored), $5 now (UPDATED_AT),
// $6 deployment id. $3 and $4 are the same instant, typed differently for Postgres.
func compileIncrementAccessFailureQuery(scope governancemodel.AccessScope) (model.DBQuery, error) {
	path, err := buildAccessLockPath(scope)
	if err != nil {
		return model.DBQuery{}, err
	}

	pgEntry := fmt.Sprintf(
		`COALESCE(%s, '{}'::jsonb) || jsonb_build_object('failureCount', CASE WHEN (`+
			`COALESCE((%s)::timestamptz, 'infinity'::timestamptz) <= $3::timestamptz AND `+
			`COALESCE((%s)::timestamptz, 'epoch'::timestamptz) < (%s)::timestamptz`+
			`) OR COALESCE((%s)::timestamptz, 'epoch'::timestamptz) < $2::timestamptz THEN 1 ELSE `+
			`COALESCE((%s)::int, 0) + 1 END, 'lastFailedAt', $4::text)`,
		path.pgEntry,
		path.pgField("unlockAt"),
		path.pgField("lastFailedAt"), path.pgField("unlockAt"),
		path.pgField("lastFailedAt"),
		path.pgField("failureCount"))

	postgres := fmt.Sprintf(
		`UPDATE "ENTITY_RUNTIME_DATA" SET RUNTIME_ATTRIBUTES = %s, REVISION = REVISION + 1, UPDATED_AT = $5 `+
			`WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $6 %s `+
			`RETURNING (%s)::int AS FAILURE_COUNT, COALESCE((%s)::int, 0) AS LOCK_COUNT, `+
			`COALESCE(%s, '') AS UNLOCK_AT, REVISION`,
		path.pgWrap(pgEntry), noLiveHoldPostgres(path, "$3"),
		path.pgField("failureCount"), path.pgField("lockCount"), path.pgField("unlockAt"))

	sqlite := fmt.Sprintf(
		`UPDATE "ENTITY_RUNTIME_DATA" SET RUNTIME_ATTRIBUTES = json_set(json_set(`+
			`COALESCE(NULLIF(RUNTIME_ATTRIBUTES, ''), '{}'), `+
			`%s, CASE WHEN (`+
			`COALESCE(json_extract(RUNTIME_ATTRIBUTES, %s), '') <> '' AND `+
			`json_extract(RUNTIME_ATTRIBUTES, %s) <= $3 AND `+
			`COALESCE(json_extract(RUNTIME_ATTRIBUTES, %s), '') < `+
			`json_extract(RUNTIME_ATTRIBUTES, %s)`+
			`) OR COALESCE(json_extract(RUNTIME_ATTRIBUTES, %s), '') < $2 THEN 1 ELSE `+
			`COALESCE(json_extract(RUNTIME_ATTRIBUTES, %s), 0) + 1 END), `+
			`%s, $4), REVISION = REVISION + 1, UPDATED_AT = $5 `+
			`WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $6 %s `+
			`RETURNING json_extract(RUNTIME_ATTRIBUTES, %s) AS FAILURE_COUNT, `+
			`COALESCE(json_extract(RUNTIME_ATTRIBUTES, %s), 0) AS LOCK_COUNT, `+
			`COALESCE(json_extract(RUNTIME_ATTRIBUTES, %s), '') AS UNLOCK_AT, REVISION`,
		path.sqliteField("failureCount"),
		path.sqliteField("unlockAt"),
		path.sqliteField("unlockAt"),
		path.sqliteField("lastFailedAt"),
		path.sqliteField("unlockAt"),
		path.sqliteField("lastFailedAt"),
		path.sqliteField("failureCount"),
		path.sqliteField("lastFailedAt"),
		noLiveHoldSQLite(path, "$3"),
		path.sqliteField("failureCount"),
		path.sqliteField("lockCount"),
		path.sqliteField("unlockAt"))

	return model.DBQuery{
		ID:          "ERD-ENTITY_MGT-05",
		Query:       postgres,
		SQLiteQuery: sqlite,
	}, nil
}

// compileFormAccessLockQuery builds the statement that opens a lock episode. It applies only if the
// failure count, lock count and revision are the ones observed and no lock is live, and it resets
// the failure count to 0.
//
// Parameters: $1 entity id, $2 now (compared), $3 new lockCount, $4 new unlockAt, $5 observed
// failureCount, $6 observed lockCount, $7 now (UPDATED_AT), $8 observed revision, $9 deployment id.
func compileFormAccessLockQuery(scope governancemodel.AccessScope) (model.DBQuery, error) {
	path, err := buildAccessLockPath(scope)
	if err != nil {
		return model.DBQuery{}, err
	}

	pgEntry := fmt.Sprintf(
		`COALESCE(%s, '{}'::jsonb) || jsonb_build_object(`+
			`'failureCount', 0, 'lockCount', $3::int, 'unlockAt', $4::text)`, path.pgEntry)

	postgres := fmt.Sprintf(
		`UPDATE "ENTITY_RUNTIME_DATA" SET RUNTIME_ATTRIBUTES = %s, REVISION = REVISION + 1, UPDATED_AT = $7 `+
			`WHERE ENTITY_ID = $1 AND REVISION = $8 AND DEPLOYMENT_ID = $9 `+
			`AND COALESCE((%s)::int, 0) = $5::int `+
			`AND COALESCE((%s)::int, 0) = $6::int `+
			`AND COALESCE((%s)::timestamptz, 'epoch'::timestamptz) <= $2::timestamptz`,
		path.pgWrap(pgEntry),
		path.pgField("failureCount"), path.pgField("lockCount"), path.pgField("unlockAt"))

	sqlite := fmt.Sprintf(
		`UPDATE "ENTITY_RUNTIME_DATA" SET RUNTIME_ATTRIBUTES = json_set(json_set(json_set(`+
			`COALESCE(NULLIF(RUNTIME_ATTRIBUTES, ''), '{}'), %s, 0), %s, $3), %s, $4), `+
			`REVISION = REVISION + 1, UPDATED_AT = $7 `+
			`WHERE ENTITY_ID = $1 AND REVISION = $8 AND DEPLOYMENT_ID = $9 `+
			`AND COALESCE(json_extract(RUNTIME_ATTRIBUTES, %s), 0) = $5 `+
			`AND COALESCE(json_extract(RUNTIME_ATTRIBUTES, %s), 0) = $6 `+
			`AND COALESCE(json_extract(RUNTIME_ATTRIBUTES, %s), '') <= $2`,
		path.sqliteField("failureCount"), path.sqliteField("lockCount"), path.sqliteField("unlockAt"),
		path.sqliteField("failureCount"), path.sqliteField("lockCount"), path.sqliteField("unlockAt"))

	return model.DBQuery{
		ID:          "ERD-ENTITY_MGT-06",
		Query:       postgres,
		SQLiteQuery: sqlite,
	}, nil
}
