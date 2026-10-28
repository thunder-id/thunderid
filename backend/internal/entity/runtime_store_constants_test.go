// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// RuntimeStoreConstantsTestSuite checks the runtime statement text in both dialects. What a statement
// does to a stored row needs a database, so it is left to integration tests.
type RuntimeStoreConstantsTestSuite struct {
	suite.Suite
}

func TestRuntimeStoreConstantsTestSuite(t *testing.T) {
	suite.Run(t, new(RuntimeStoreConstantsTestSuite))
}

const testMethod = providers.AuthenticationMethodCredential

// Every runtime statement has a distinct ID, numbered in the order the statements are declared.
func (s *RuntimeStoreConstantsTestSuite) TestQueryIDsAreDistinctAndAscending() {
	getQuery, _ := buildGetRuntimeDataQuery([]string{"e1"}, testDeploymentID)
	initQuery, _ := buildInitializeRuntimeDataQuery([]entityRuntimeData{{ID: "e1"}}, time.Now(), testDeploymentID)
	clearQuery, _, err := buildClearAccessStateQuery("e1", []providers.AuthenticationMethod{testMethod},
		time.Now(), nil, testDeploymentID)
	s.Require().NoError(err)
	revision := int64(1)
	guardedClearQuery, _, err := buildClearAccessStateQuery("e1", []providers.AuthenticationMethod{testMethod},
		time.Now(), &revision, testDeploymentID)
	s.Require().NoError(err)
	incrementQuery, err := buildIncrementAccessFailureQuery(testMethod)
	s.Require().NoError(err)
	formQuery, err := buildFormAccessLockQuery(testMethod)
	s.Require().NoError(err)

	queries := []model.DBQuery{queryResetRuntimeData, queryDeleteRuntimeData, querySetAccessSuspension,
		queryClearAccessSuspension, queryClearAccessSuspensionWithHold, queryRecordAccessLogin, getQuery,
		initQuery, clearQuery, guardedClearQuery, incrementQuery, formQuery, querySetAccessOwnerSuspension,
		queryClearAccessOwnerSuspension, querySetAccessCascadeSuspension, queryClearAccessCascadeSuspension}
	for i, query := range queries {
		s.Equal(fmt.Sprintf("ERD-ENTITY_MGT-%02d", i+1), query.ID)
	}
}

// allRuntimeStatements returns every runtime statement: the fixed ones, the built ones and one increment
// and one formation per registered method.
func (s *RuntimeStoreConstantsTestSuite) allRuntimeStatements() []model.DBQuery {
	getQuery, _ := buildGetRuntimeDataQuery([]string{"e1"}, testDeploymentID)
	initQuery, _ := buildInitializeRuntimeDataQuery([]entityRuntimeData{{ID: "e1"}}, time.Now(), testDeploymentID)
	revision := int64(1)
	clearQuery, _, err := buildClearAccessStateQuery("e1", identitygovernance.AllAuthenticationMethods,
		time.Now(), nil, testDeploymentID)
	s.Require().NoError(err)
	guardedClearQuery, _, err := buildClearAccessStateQuery("e1", []providers.AuthenticationMethod{testMethod},
		time.Now(), &revision, testDeploymentID)
	s.Require().NoError(err)

	queries := make([]model.DBQuery, 0, 10+2*len(identitygovernance.AllAuthenticationMethods))
	queries = append(queries, queryResetRuntimeData, queryDeleteRuntimeData, querySetAccessSuspension,
		queryClearAccessSuspension, queryClearAccessSuspensionWithHold, queryRecordAccessLogin, getQuery,
		initQuery, clearQuery, guardedClearQuery, querySetAccessOwnerSuspension, queryClearAccessOwnerSuspension,
		querySetAccessCascadeSuspension, queryClearAccessCascadeSuspension)
	for _, method := range identitygovernance.AllAuthenticationMethods {
		increment, err := methodQuery(incrementAccessQueries, method)
		s.Require().NoError(err)
		form, err := methodQuery(formAccessQueries, method)
		s.Require().NoError(err)
		queries = append(queries, increment, form)
	}
	return queries
}

// Runtime statements name only the runtime table, never the profile table, so a runtime write cannot
// touch a profile row and a profile update cannot reach the runtime row.
func (s *RuntimeStoreConstantsTestSuite) TestRuntimeStatementsNeverNameTheProfileTable() {
	profileTable := regexp.MustCompile(`(?i)"ENTITY"|\b(from|join|update|into)\s+ENTITY\b`)
	for _, query := range s.allRuntimeStatements() {
		for _, dialect := range []string{"postgres", "sqlite"} {
			statement := query.GetQuery(dialect)
			s.Contains(statement, `"ENTITY_RUNTIME_DATA"`, "%s (%s)", query.ID, dialect)
			s.False(profileTable.MatchString(statement), "%s (%s) names the profile table: %s",
				query.ID, dialect, statement)
		}
	}
}

// Statements whose dialects differ set both forms and no generic one.
func (s *RuntimeStoreConstantsTestSuite) TestDialectSpecificStatementsSetBothForms() {
	clearQuery, _, err := buildClearAccessStateQuery("e1", []providers.AuthenticationMethod{testMethod},
		time.Now(), nil, testDeploymentID)
	s.Require().NoError(err)
	incrementQuery, err := buildIncrementAccessFailureQuery(testMethod)
	s.Require().NoError(err)
	formQuery, err := buildFormAccessLockQuery(testMethod)
	s.Require().NoError(err)

	for _, query := range []model.DBQuery{querySetAccessSuspension, queryClearAccessSuspension,
		queryClearAccessSuspensionWithHold, queryRecordAccessLogin, clearQuery, incrementQuery, formQuery,
		querySetAccessOwnerSuspension, queryClearAccessOwnerSuspension, querySetAccessCascadeSuspension,
		queryClearAccessCascadeSuspension} {
		s.Empty(query.Query, query.ID)
		s.NotEmpty(query.PostgresQuery, query.ID)
		s.NotEmpty(query.SQLiteQuery, query.ID)
		s.Equal(query.PostgresQuery, query.GetQuery("postgres"), query.ID)
		s.Equal(query.SQLiteQuery, query.GetQuery("sqlite"), query.ID)
	}
}

// statementCase is one runtime statement in one dialect and the fragments it must contain, in order.
type statementCase struct {
	name      string
	statement string
	fragments []string
}

// assertFragments checks that each statement holds its fragments in the order given.
func (s *RuntimeStoreConstantsTestSuite) assertFragments(cases []statementCase) {
	for _, tc := range cases {
		s.Run(tc.name, func() {
			rest := tc.statement
			for _, fragment := range tc.fragments {
				at := strings.Index(rest, fragment)
				s.Require().GreaterOrEqual(at, 0, "missing or out of order:\n%s\nin:\n%s", fragment, tc.statement)
				rest = rest[at+len(fragment):]
			}
		})
	}
}

// Paths of the entries the method statements address: a method entry and the entity-wide entry.
const (
	pgCredentialEntry   = `RUNTIME_ATTRIBUTES->'accessState'->'lock'->'authenticationMethods'->'credential'`
	pgEntityEntry       = `RUNTIME_ATTRIBUTES->'accessState'->'lock'->'entity'`
	sqliteCredentialKey = `'$.accessState.lock.authenticationMethods.credential.`
	sqliteEntityKey     = `'$.accessState.lock.entity.`
)

// methodStatement is one method's statement with the paths of its entry in both dialects.
type methodStatement struct {
	method    providers.AuthenticationMethod
	pgEntry   string
	sqliteKey string
}

// methodStatements are the methods the increment and formation tables check: a method and the entity.
var methodStatements = []methodStatement{
	{testMethod, pgCredentialEntry, sqliteCredentialKey},
	{identitygovernance.AuthenticationMethodEntity, pgEntityEntry, sqliteEntityKey},
}

// sqliteFieldOf is the SQLite path of one field of an entry.
func sqliteFieldOf(key, field string) string {
	return key + field + `'`
}

// sqliteValue extracts one field of an entry in SQLite.
func sqliteValue(key, field string) string {
	return `json_extract(RUNTIME_ATTRIBUTES, ` + sqliteFieldOf(key, field) + `)`
}

// Each write is guarded in its WHERE clause on the entity, the conditions that keep a concurrent or
// stale write from applying, and the deployment, in both dialects.
func (s *RuntimeStoreConstantsTestSuite) TestStatementsGuardTheirWrites() {
	revision := int64(7)
	clearQuery, _, err := buildClearAccessStateQuery("e1", []providers.AuthenticationMethod{testMethod}, time.Now(),
		nil,
		testDeploymentID)
	s.Require().NoError(err)
	guardedClearQuery, _, err := buildClearAccessStateQuery("e1", []providers.AuthenticationMethod{testMethod},
		time.Now(),
		&revision, testDeploymentID)
	s.Require().NoError(err)
	getQuery, _ := buildGetRuntimeDataQuery([]string{"e1", "e2"}, testDeploymentID)

	cases := make([]statementCase, 0, 14+4*len(methodStatements))
	cases = append(cases, []statementCase{
		{"delete", queryDeleteRuntimeData.GetQuery("postgres"),
			[]string{`DELETE FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID = $1 AND DEPLOYMENT_ID = $2`}},
		{"get", getQuery.GetQuery("postgres"),
			[]string{`WHERE ENTITY_ID IN ($1,$2) AND DEPLOYMENT_ID = $3`}},
		{"owner suspend, postgres", querySetAccessOwnerSuspension.PostgresQuery, []string{
			`WHERE ENTITY_ID = $1 AND RUNTIME_ATTRIBUTES->'accessState'->'ownerSuspend' IS NULL ` +
				`AND DEPLOYMENT_ID = $4`}},
		{"owner suspend, sqlite", querySetAccessOwnerSuspension.SQLiteQuery, []string{
			`WHERE ENTITY_ID = $1 AND json_extract(RUNTIME_ATTRIBUTES, '$.accessState.ownerSuspend') IS NULL ` +
				`AND DEPLOYMENT_ID = $4`}},
		{"owner release, postgres", queryClearAccessOwnerSuspension.PostgresQuery, []string{
			`WHERE ENTITY_ID = $1 AND RUNTIME_ATTRIBUTES->'accessState'->'ownerSuspend' IS NOT NULL ` +
				`AND DEPLOYMENT_ID = $3`}},
		{"owner release, sqlite", queryClearAccessOwnerSuspension.SQLiteQuery, []string{
			`WHERE ENTITY_ID = $1 AND json_extract(RUNTIME_ATTRIBUTES, '$.accessState.ownerSuspend') IS NOT NULL ` +
				`AND DEPLOYMENT_ID = $3`}},
		{"cascade suspend, postgres", querySetAccessCascadeSuspension.PostgresQuery, []string{
			`WHERE ENTITY_ID = $1 AND RUNTIME_ATTRIBUTES->'accessState'->'cascadeSuspend' IS NULL ` +
				`AND DEPLOYMENT_ID = $4`}},
		{"cascade suspend, sqlite", querySetAccessCascadeSuspension.SQLiteQuery, []string{
			`WHERE ENTITY_ID = $1 AND json_extract(RUNTIME_ATTRIBUTES, '$.accessState.cascadeSuspend') IS NULL ` +
				`AND DEPLOYMENT_ID = $4`}},
		{"cascade release, postgres", queryClearAccessCascadeSuspension.PostgresQuery, []string{
			`WHERE ENTITY_ID = $1 AND RUNTIME_ATTRIBUTES->'accessState'->'cascadeSuspend' IS NOT NULL ` +
				`AND DEPLOYMENT_ID = $3`}},
		{"cascade release, sqlite", queryClearAccessCascadeSuspension.SQLiteQuery, []string{
			`WHERE ENTITY_ID = $1 AND json_extract(RUNTIME_ATTRIBUTES, '$.accessState.cascadeSuspend') IS NOT NULL ` +
				`AND DEPLOYMENT_ID = $3`}},
		{"suspend, postgres", querySetAccessSuspension.PostgresQuery, []string{
			`WHERE ENTITY_ID = $1 AND STATE <> 'SUSPENDED' ` +
				`AND RUNTIME_ATTRIBUTES->'accessState'->'suspend' IS NULL AND DEPLOYMENT_ID = $5`}},
		{"suspend, sqlite", querySetAccessSuspension.SQLiteQuery, []string{
			`WHERE ENTITY_ID = $1 AND STATE <> 'SUSPENDED' ` +
				`AND json_extract(RUNTIME_ATTRIBUTES, '$.accessState.suspend') IS NULL AND DEPLOYMENT_ID = $5`}},
		{"release, postgres", queryClearAccessSuspension.PostgresQuery, []string{
			`WHERE ENTITY_ID = $1 ` +
				`AND COALESCE(RUNTIME_ATTRIBUTES->'accessState'->'suspend'->>'suspendedAt', '') = $3 ` +
				`AND DEPLOYMENT_ID = $4`}},
		{"release, sqlite", queryClearAccessSuspension.SQLiteQuery, []string{
			`WHERE ENTITY_ID = $1 AND ` +
				`COALESCE(json_extract(RUNTIME_ATTRIBUTES, '$.accessState.suspend.suspendedAt'), '') = $3 ` +
				`AND DEPLOYMENT_ID = $4`}},
		{"release onto a hold, postgres", queryClearAccessSuspensionWithHold.PostgresQuery, []string{
			`WHERE ENTITY_ID = $1 ` +
				`AND COALESCE(RUNTIME_ATTRIBUTES->'accessState'->'suspend'->>'suspendedAt', '') = $5 ` +
				`AND DEPLOYMENT_ID = $6`}},
		{"release onto a hold, sqlite", queryClearAccessSuspensionWithHold.SQLiteQuery, []string{
			`WHERE ENTITY_ID = $1 AND ` +
				`COALESCE(json_extract(RUNTIME_ATTRIBUTES, '$.accessState.suspend.suspendedAt'), '') = $5 ` +
				`AND DEPLOYMENT_ID = $6`}},
		{"record login, postgres", queryRecordAccessLogin.PostgresQuery, []string{
			`WHERE ENTITY_ID = $1 AND NOT (` +
				`COALESCE(RUNTIME_ATTRIBUTES->'accessState'->>'lastLoginAt', '') >= $3::text AND ` +
				`COALESCE(RUNTIME_ATTRIBUTES->'accessState'->>'lastLoginAt', '') <= $2::text) ` +
				`AND DEPLOYMENT_ID = $5`}},
		{"record login, sqlite", queryRecordAccessLogin.SQLiteQuery, []string{
			`WHERE ENTITY_ID = $1 AND NOT (` +
				`COALESCE(json_extract(RUNTIME_ATTRIBUTES, '$.accessState.lastLoginAt'), '') >= $3 AND ` +
				`COALESCE(json_extract(RUNTIME_ATTRIBUTES, '$.accessState.lastLoginAt'), '') <= $2) ` +
				`AND DEPLOYMENT_ID = $5`}},
		{"clear, postgres", clearQuery.PostgresQuery, []string{`WHERE ENTITY_ID = $2 AND DEPLOYMENT_ID = $3`}},
		{"clear, sqlite", clearQuery.SQLiteQuery, []string{`WHERE ENTITY_ID = $2 AND DEPLOYMENT_ID = $3`}},
		{"guarded clear, postgres", guardedClearQuery.PostgresQuery,
			[]string{`WHERE ENTITY_ID = $2 AND REVISION = $3 AND DEPLOYMENT_ID = $4`}},
		{"guarded clear, sqlite", guardedClearQuery.SQLiteQuery,
			[]string{`WHERE ENTITY_ID = $2 AND REVISION = $3 AND DEPLOYMENT_ID = $4`}},
	}...)
	for _, entry := range methodStatements {
		increment, err := buildIncrementAccessFailureQuery(entry.method)
		s.Require().NoError(err)
		form, err := buildFormAccessLockQuery(entry.method)
		s.Require().NoError(err)
		pg, key := entry.pgEntry, entry.sqliteKey
		cases = append(cases,
			// A live entity-wide lock or a live lock on the method stops the count.
			statementCase{"increment " + string(entry.method) + ", postgres", increment.PostgresQuery, []string{
				`WHERE ENTITY_ID = $1 ` +
					`AND COALESCE((` + pgEntityEntry + `->>'unlockAt')::timestamptz, 'epoch'::timestamptz) ` +
					`<= $3::timestamptz ` +
					`AND COALESCE((` + pg + `->>'unlockAt')::timestamptz, 'epoch'::timestamptz) <= $3::timestamptz ` +
					`AND DEPLOYMENT_ID = $6 RETURNING`}},
			statementCase{"increment " + string(entry.method) + ", sqlite", increment.SQLiteQuery, []string{
				`WHERE ENTITY_ID = $1 ` +
					`AND NOT (COALESCE(` + sqliteValue(sqliteEntityKey, "unlockAt") + `, '') <> '' ` +
					`AND ` + sqliteValue(sqliteEntityKey, "unlockAt") + ` > $3) ` +
					`AND NOT (COALESCE(` + sqliteValue(key, "unlockAt") + `, '') <> '' ` +
					`AND ` + sqliteValue(key, "unlockAt") + ` > $3) ` +
					`AND DEPLOYMENT_ID = $6 RETURNING`}},
			// The formation applies only to the revision and counters it observed, with no lock live.
			statementCase{"form " + string(entry.method) + ", postgres", form.PostgresQuery, []string{
				`WHERE ENTITY_ID = $1 AND REVISION = $8 ` +
					`AND COALESCE((` + pg + `->>'failureCount')::int, 0) = $5::int ` +
					`AND COALESCE((` + pg + `->>'lockCount')::int, 0) = $6::int ` +
					`AND COALESCE((` + pg + `->>'unlockAt')::timestamptz, 'epoch'::timestamptz) <= $2::timestamptz ` +
					`AND DEPLOYMENT_ID = $9`}},
			statementCase{"form " + string(entry.method) + ", sqlite", form.SQLiteQuery, []string{
				`WHERE ENTITY_ID = $1 AND REVISION = $8 ` +
					`AND COALESCE(` + sqliteValue(key, "failureCount") + `, 0) = $5 ` +
					`AND COALESCE(` + sqliteValue(key, "lockCount") + `, 0) = $6 ` +
					`AND COALESCE(` + sqliteValue(key, "unlockAt") + `, '') <= $2 ` +
					`AND DEPLOYMENT_ID = $9`}},
		)
	}
	s.assertFragments(cases)
}

// Each write replaces only its own member or entry: defaults apply at each path inside the document,
// every other member is kept, and every write moves the revision.
func (s *RuntimeStoreConstantsTestSuite) TestStatementsWriteOnlyTheirOwnMember() {
	clearQuery, _, err := buildClearAccessStateQuery("e1",
		[]providers.AuthenticationMethod{testMethod,
			identitygovernance.AuthenticationMethodEntity}, time.Now(), nil, testDeploymentID)
	s.Require().NoError(err)
	keepState := `RUNTIME_ATTRIBUTES || jsonb_build_object('accessState', ` +
		`COALESCE(RUNTIME_ATTRIBUTES->'accessState', '{}'::jsonb)`

	cases := make([]statementCase, 0, 10+4*len(methodStatements))
	cases = append(cases, []statementCase{
		// The owner's suspension writes only its own member and never the state column.
		{"owner suspend, postgres", querySetAccessOwnerSuspension.PostgresQuery, []string{
			`SET RUNTIME_ATTRIBUTES = ` + keepState + ` || jsonb_build_object('ownerSuspend', ` +
				`jsonb_build_object('suspendedAt', $2::text))), REVISION = REVISION + 1, UPDATED_AT = $3`}},
		{"owner suspend, sqlite", querySetAccessOwnerSuspension.SQLiteQuery, []string{
			`SET RUNTIME_ATTRIBUTES = json_set(RUNTIME_ATTRIBUTES, ` +
				`'$.accessState.ownerSuspend', json_object('suspendedAt', $2)), ` +
				`REVISION = REVISION + 1, UPDATED_AT = $3`}},
		{"owner release, postgres", queryClearAccessOwnerSuspension.PostgresQuery, []string{
			`SET RUNTIME_ATTRIBUTES = ` + keepState + ` - 'ownerSuspend'), ` +
				`REVISION = REVISION + 1, UPDATED_AT = $2`}},
		{"owner release, sqlite", queryClearAccessOwnerSuspension.SQLiteQuery, []string{
			`SET RUNTIME_ATTRIBUTES = json_remove(RUNTIME_ATTRIBUTES, '$.accessState.ownerSuspend'), ` +
				`REVISION = REVISION + 1, UPDATED_AT = $2`}},
		{"cascade suspend, postgres", querySetAccessCascadeSuspension.PostgresQuery, []string{
			`SET RUNTIME_ATTRIBUTES = ` + keepState + ` || jsonb_build_object('cascadeSuspend', ` +
				`jsonb_build_object('suspendedAt', $2::text))), REVISION = REVISION + 1, UPDATED_AT = $3`}},
		{"cascade suspend, sqlite", querySetAccessCascadeSuspension.SQLiteQuery, []string{
			`SET RUNTIME_ATTRIBUTES = json_set(RUNTIME_ATTRIBUTES, ` +
				`'$.accessState.cascadeSuspend', json_object('suspendedAt', $2)), ` +
				`REVISION = REVISION + 1, UPDATED_AT = $3`}},
		{"cascade release, postgres", queryClearAccessCascadeSuspension.PostgresQuery, []string{
			`SET RUNTIME_ATTRIBUTES = ` + keepState + ` - 'cascadeSuspend'), ` +
				`REVISION = REVISION + 1, UPDATED_AT = $2`}},
		{"cascade release, sqlite", queryClearAccessCascadeSuspension.SQLiteQuery, []string{
			`SET RUNTIME_ATTRIBUTES = json_remove(RUNTIME_ATTRIBUTES, '$.accessState.cascadeSuspend'), ` +
				`REVISION = REVISION + 1, UPDATED_AT = $2`}},
		{"suspend, postgres", querySetAccessSuspension.PostgresQuery, []string{
			`SET STATE = 'SUSPENDED', RUNTIME_ATTRIBUTES = ` + keepState + ` || jsonb_build_object('suspend', ` +
				`jsonb_build_object('suspendedAt', $2::text, 'operatorNote', $3::text))), ` +
				`REVISION = REVISION + 1, UPDATED_AT = $4`}},
		{"suspend, sqlite", querySetAccessSuspension.SQLiteQuery, []string{
			`SET STATE = 'SUSPENDED', RUNTIME_ATTRIBUTES = json_set(RUNTIME_ATTRIBUTES, ` +
				`'$.accessState.suspend', json_object('suspendedAt', $2, 'operatorNote', $3)), ` +
				`REVISION = REVISION + 1, UPDATED_AT = $4`}},
		{"release, postgres", queryClearAccessSuspension.PostgresQuery, []string{
			`SET STATE = 'ACTIVE', RUNTIME_ATTRIBUTES = ` + keepState + ` - 'suspend'), ` +
				`REVISION = REVISION + 1, UPDATED_AT = $2`}},
		{"release, sqlite", queryClearAccessSuspension.SQLiteQuery, []string{
			`SET STATE = 'ACTIVE', RUNTIME_ATTRIBUTES = json_remove(RUNTIME_ATTRIBUTES, '$.accessState.suspend'), ` +
				`REVISION = REVISION + 1, UPDATED_AT = $2`}},
		// The hold replaces the whole lock member, so no failure count is carried across.
		{"release onto a hold, postgres", queryClearAccessSuspensionWithHold.PostgresQuery, []string{
			`SET STATE = 'ACTIVE', RUNTIME_ATTRIBUTES = RUNTIME_ATTRIBUTES || jsonb_build_object('accessState', ` +
				`(COALESCE(RUNTIME_ATTRIBUTES->'accessState', '{}'::jsonb) - 'suspend') || ` +
				`jsonb_build_object('lock', jsonb_build_object('entity', ` +
				`jsonb_build_object('unlockAt', $2::text, 'reason', $3::text)))), ` +
				`REVISION = REVISION + 1, UPDATED_AT = $4`}},
		{"release onto a hold, sqlite", queryClearAccessSuspensionWithHold.SQLiteQuery, []string{
			`SET STATE = 'ACTIVE', RUNTIME_ATTRIBUTES = json_set(json_remove(RUNTIME_ATTRIBUTES, ` +
				`'$.accessState.suspend'), '$.accessState.lock', json_object('entity', ` +
				`json_object('unlockAt', $2, 'reason', $3))), REVISION = REVISION + 1, UPDATED_AT = $4`}},
		{"record login, postgres", queryRecordAccessLogin.PostgresQuery, []string{
			`SET RUNTIME_ATTRIBUTES = ` + keepState + ` || jsonb_build_object('lastLoginAt', $2::text)), ` +
				`REVISION = REVISION + 1, UPDATED_AT = $4`}},
		{"record login, sqlite", queryRecordAccessLogin.SQLiteQuery, []string{
			`SET RUNTIME_ATTRIBUTES = json_set(RUNTIME_ATTRIBUTES, '$.accessState.lastLoginAt', $2), ` +
				`REVISION = REVISION + 1, UPDATED_AT = $4`}},
		{"clear, postgres", clearQuery.PostgresQuery, []string{
			`SET RUNTIME_ATTRIBUTES = RUNTIME_ATTRIBUTES` +
				` #- ARRAY['accessState','lock','authenticationMethods','credential']::text[]` +
				` #- ARRAY['accessState','lock','entity']::text[], REVISION = REVISION + 1, UPDATED_AT = $1`}},
		{"clear, sqlite", clearQuery.SQLiteQuery, []string{
			`SET RUNTIME_ATTRIBUTES = json_remove(RUNTIME_ATTRIBUTES, ` +
				`'$.accessState.lock.authenticationMethods.credential', '$.accessState.lock.entity'), ` +
				`REVISION = REVISION + 1, UPDATED_AT = $1`}},
	}...)
	for _, entry := range methodStatements {
		increment, err := buildIncrementAccessFailureQuery(entry.method)
		s.Require().NoError(err)
		form, err := buildFormAccessLockQuery(entry.method)
		s.Require().NoError(err)
		pg, key := entry.pgEntry, entry.sqliteKey
		keepEntry := keepState + ` || jsonb_build_object('lock', ` +
			`COALESCE(RUNTIME_ATTRIBUTES->'accessState'->'lock', '{}'::jsonb) || `
		if entry.method == identitygovernance.AuthenticationMethodEntity {
			keepEntry += `jsonb_build_object('entity', `
		} else {
			keepEntry += `jsonb_build_object('authenticationMethods', ` +
				`COALESCE(RUNTIME_ATTRIBUTES->'accessState'->'lock'->'authenticationMethods', '{}'::jsonb) || ` +
				`jsonb_build_object('credential', `
		}
		keepEntry += `COALESCE(` + pg + `, '{}'::jsonb) || jsonb_build_object(`
		cases = append(cases,
			statementCase{"increment " + string(entry.method) + ", postgres", increment.PostgresQuery, []string{
				`SET RUNTIME_ATTRIBUTES = ` + keepEntry + `'failureCount', CASE WHEN`,
				`'lastFailedAt', $4::text, 'consecutiveFailures', `,
				`REVISION = REVISION + 1, UPDATED_AT = $5 WHERE`}},
			statementCase{"increment " + string(entry.method) + ", sqlite", increment.SQLiteQuery, []string{
				`SET RUNTIME_ATTRIBUTES = json_set(json_set(json_set(RUNTIME_ATTRIBUTES, ` +
					sqliteFieldOf(key, "failureCount") + `, CASE WHEN`,
				`), ` + sqliteFieldOf(key, "lastFailedAt") + `, $4), ` +
					sqliteFieldOf(key, "consecutiveFailures") + `, `,
				`REVISION = REVISION + 1, UPDATED_AT = $5 WHERE`}},
			statementCase{"form " + string(entry.method) + ", postgres", form.PostgresQuery, []string{
				`SET RUNTIME_ATTRIBUTES = ` + keepEntry + `'failureCount', 0, 'lockCount', $3::int, ` +
					`'unlockAt', $4::text)`,
				`REVISION = REVISION + 1, UPDATED_AT = $7 WHERE`}},
			statementCase{"form " + string(entry.method) + ", sqlite", form.SQLiteQuery, []string{
				`SET RUNTIME_ATTRIBUTES = json_set(json_set(json_set(RUNTIME_ATTRIBUTES, ` +
					sqliteFieldOf(key, "failureCount") + `, 0), ` + sqliteFieldOf(key, "lockCount") + `, $3), ` +
					sqliteFieldOf(key, "unlockAt") + `, $4), REVISION = REVISION + 1, UPDATED_AT = $7 WHERE`}},
		)
	}
	s.assertFragments(cases)
}

// The increment restarts the count at one after a failure older than the window, and once after a lock
// lapsed; otherwise it adds one. The consecutive count only ever adds one.
func (s *RuntimeStoreConstantsTestSuite) TestTheIncrementRestartsTheCountOnlyWhereItShould() {
	cases := make([]statementCase, 0, 2*len(methodStatements))
	for _, entry := range methodStatements {
		increment, err := buildIncrementAccessFailureQuery(entry.method)
		s.Require().NoError(err)
		pg, key := entry.pgEntry, entry.sqliteKey
		cases = append(cases,
			statementCase{string(entry.method) + ", postgres", increment.PostgresQuery, []string{
				`'failureCount', CASE WHEN (` +
					`COALESCE((` + pg + `->>'unlockAt')::timestamptz, 'infinity'::timestamptz) <= $3::timestamptz ` +
					`AND COALESCE((` + pg + `->>'lastFailedAt')::timestamptz, 'epoch'::timestamptz) < ` +
					`(` + pg + `->>'unlockAt')::timestamptz) ` +
					`OR COALESCE((` + pg + `->>'lastFailedAt')::timestamptz, 'epoch'::timestamptz) < $2::timestamptz ` +
					`THEN 1 ELSE COALESCE((` + pg + `->>'failureCount')::int, 0) + 1 END`,
				`'consecutiveFailures', COALESCE((` + pg + `->>'consecutiveFailures')::int, 0) + 1)`}},
			statementCase{string(entry.method) + ", sqlite", increment.SQLiteQuery, []string{
				`CASE WHEN (COALESCE(` + sqliteValue(key, "unlockAt") + `, '') <> '' ` +
					`AND ` + sqliteValue(key, "unlockAt") + ` <= $3 ` +
					`AND COALESCE(` + sqliteValue(key, "lastFailedAt") + `, '') < ` +
					sqliteValue(key, "unlockAt") + `) ` +
					`OR COALESCE(` + sqliteValue(key, "lastFailedAt") + `, '') < $2 ` +
					`THEN 1 ELSE COALESCE(` + sqliteValue(key, "failureCount") + `, 0) + 1 END)`,
				sqliteFieldOf(key, "consecutiveFailures") + `, COALESCE(` + sqliteValue(key, "consecutiveFailures") +
					`, 0) + 1)`}},
		)
	}
	s.assertFragments(cases)
}

// The increment returns the counters under the column names the store reads, and the read returns the
// columns the store parses.
func (s *RuntimeStoreConstantsTestSuite) TestStatementsReturnTheColumnsTheStoreReads() {
	getQuery, _ := buildGetRuntimeDataQuery([]string{"e1"}, testDeploymentID)
	cases := make([]statementCase, 0, 1+2*len(methodStatements))
	cases = append(cases, statementCase{"get", getQuery.GetQuery("postgres"),
		[]string{`SELECT ENTITY_ID, STATE, RUNTIME_ATTRIBUTES, REVISION FROM "ENTITY_RUNTIME_DATA" `}})
	for _, entry := range methodStatements {
		increment, err := buildIncrementAccessFailureQuery(entry.method)
		s.Require().NoError(err)
		pg, key := entry.pgEntry, entry.sqliteKey
		cases = append(cases,
			statementCase{"increment " + string(entry.method) + ", postgres", increment.PostgresQuery, []string{
				`RETURNING (` + pg + `->>'failureCount')::int AS FAILURE_COUNT, ` +
					`COALESCE((` + pg + `->>'lockCount')::int, 0) AS LOCK_COUNT, ` +
					`COALESCE(` + pg + `->>'unlockAt', '') AS UNLOCK_AT, ` +
					`COALESCE((` + pg + `->>'consecutiveFailures')::int, 0) AS CONSECUTIVE_FAILURES, REVISION`}},
			statementCase{"increment " + string(entry.method) + ", sqlite", increment.SQLiteQuery, []string{
				`RETURNING ` + sqliteValue(key, "failureCount") + ` AS FAILURE_COUNT, ` +
					`COALESCE(` + sqliteValue(key, "lockCount") + `, 0) AS LOCK_COUNT, ` +
					`COALESCE(` + sqliteValue(key, "unlockAt") + `, '') AS UNLOCK_AT, ` +
					`COALESCE(` + sqliteValue(key, "consecutiveFailures") + `, 0) AS CONSECUTIVE_FAILURES, REVISION`}},
		)
	}
	s.assertFragments(cases)
	for _, tc := range cases[1:] {
		s.True(strings.HasSuffix(tc.statement, "AS CONSECUTIVE_FAILURES, REVISION"), tc.name)
	}
}

// Inserts conflict on the deployment's key: a reset replaces an orphan row at the id and moves its
// revision, and a seed keeps a row that already exists. Each seeded row binds its own values.
func (s *RuntimeStoreConstantsTestSuite) TestInsertsResolveConflictsOnTheDeploymentKey() {
	initQuery, args := buildInitializeRuntimeDataQuery([]entityRuntimeData{
		{ID: "e1", State: "ACTIVE", Attributes: []byte(`{}`)},
		{ID: "e2", State: "SUSPENDED", Attributes: []byte(`{"accessState":{}}`)},
	}, time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC), testDeploymentID)

	s.assertFragments([]statementCase{
		{"reset", queryResetRuntimeData.GetQuery("postgres"), []string{
			`INSERT INTO "ENTITY_RUNTIME_DATA" (ENTITY_ID, STATE, RUNTIME_ATTRIBUTES, CREATED_AT, UPDATED_AT, ` +
				`DEPLOYMENT_ID) VALUES ($1, $2, '{}', $3, $3, $4) ` +
				`ON CONFLICT (DEPLOYMENT_ID, ENTITY_ID) DO UPDATE SET ` +
				`STATE = excluded.STATE, RUNTIME_ATTRIBUTES = '{}', REVISION = "ENTITY_RUNTIME_DATA".REVISION + 1`}},
		{"seed", initQuery.GetQuery("postgres"), []string{
			`(ENTITY_ID, STATE, RUNTIME_ATTRIBUTES, CREATED_AT, UPDATED_AT, DEPLOYMENT_ID) VALUES ` +
				`($1,$2,$3,$4,$4,$9),($5,$6,$7,$8,$8,$9) ON CONFLICT (DEPLOYMENT_ID, ENTITY_ID) DO NOTHING`}},
	})
	s.Equal([]interface{}{"e1", "ACTIVE", `{}`, time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC),
		"e2", "SUSPENDED", `{"accessState":{}}`, time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC),
		testDeploymentID}, args)
}

func (s *RuntimeStoreConstantsTestSuite) TestGetRuntimeDataBindsEachIDAndTheDeployment() {
	q, args := buildGetRuntimeDataQuery([]string{"e1", "e2"}, testDeploymentID)

	s.Equal(`SELECT ENTITY_ID, STATE, RUNTIME_ATTRIBUTES, REVISION FROM "ENTITY_RUNTIME_DATA" `+
		`WHERE ENTITY_ID IN ($1,$2) AND DEPLOYMENT_ID = $3`, q.Query)
	s.Equal([]interface{}{"e1", "e2", testDeploymentID}, args)
}

// The clear binds the update time and entity, then the revision when guarded, then the deployment.
func (s *RuntimeStoreConstantsTestSuite) TestClearAccessStateBindsItsGuard() {
	updatedAt := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	revision := int64(7)
	for name, tc := range map[string]struct {
		revision *int64
		args     []interface{}
	}{
		"unguarded": {nil, []interface{}{updatedAt, "e1", testDeploymentID}},
		"guarded":   {&revision, []interface{}{updatedAt, "e1", revision, testDeploymentID}},
	} {
		_, args, err := buildClearAccessStateQuery("e1", []providers.AuthenticationMethod{testMethod}, updatedAt,
			tc.revision, testDeploymentID)
		s.Require().NoError(err, name)
		s.Equal(tc.args, args, name)
	}
}

// A method outside the registry never becomes statement text, in either dialect.
func (s *RuntimeStoreConstantsTestSuite) TestAnUnknownMethodBuildsNoStatement() {
	_, err := buildIncrementAccessFailureQuery("x'; DROP TABLE")
	s.ErrorIs(err, errInvalidAuthenticationMethod)
	_, err = buildFormAccessLockQuery("x'; DROP TABLE")
	s.ErrorIs(err, errInvalidAuthenticationMethod)
	_, _, err = buildClearAccessStateQuery("e1", []providers.AuthenticationMethod{"x'; DROP TABLE"},
		time.Now(), nil, testDeploymentID)
	s.ErrorIs(err, errInvalidAuthenticationMethod)
}

// Profile reads and writes no longer carry the lifecycle state, which lives in the runtime row.
func (s *RuntimeStoreConstantsTestSuite) TestProfileStatementsDoNotCarryTheState() {
	for _, query := range []model.DBQuery{QueryGetEntityList, QuerySearchEntityList, QueryCreateEntity,
		QueryGetEntityByID, QueryUpdateEntity, QueryGetEntityWithCredentials} {
		s.NotContains(query.GetQuery("postgres"), "STATE", query.ID)
		s.NotContains(query.GetQuery("sqlite"), "STATE", query.ID)
	}
}

// The column holds a JSON object by constraint, so no statement replaces it wholesale with a default.
// Defaults apply only to paths inside the document.
func (s *RuntimeStoreConstantsTestSuite) TestNoStatementDefaultsTheWholeColumn() {
	for _, query := range s.allRuntimeStatements() {
		for _, dialect := range []string{"postgres", "sqlite"} {
			statement := query.GetQuery(dialect)
			s.NotContains(statement, "COALESCE(RUNTIME_ATTRIBUTES,", "%s (%s)", query.ID, dialect)
			s.NotContains(statement, "NULLIF(RUNTIME_ATTRIBUTES", "%s (%s)", query.ID, dialect)
		}
	}
}

// Every runtime statement is bound to one deployment's rows.
func (s *RuntimeStoreConstantsTestSuite) TestEveryStatementIsDeploymentMethodd() {
	deploymentScoped := regexp.MustCompile(`DEPLOYMENT_ID = \$\d+|\(DEPLOYMENT_ID, ENTITY_ID\)`)
	for _, query := range s.allRuntimeStatements() {
		for _, dialect := range []string{"postgres", "sqlite"} {
			s.Regexp(deploymentScoped, query.GetQuery(dialect), "%s (%s)", query.ID, dialect)
		}
	}
}

func BenchmarkAccessStateQueries(b *testing.B) {
	b.Run("increment", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := buildIncrementAccessFailureQuery(testMethod); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("formation", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := buildFormAccessLockQuery(testMethod); err != nil {
				b.Fatal(err)
			}
		}
	})
}
