// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"fmt"
	"strings"
	"time"

	sharedrevocation "github.com/thunder-id/thunderid/internal/revocation"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

// boundaryReasonSQLList renders the boundary reasons as a SQL literal list for an IN predicate.
// The values are compile-time package constants from the revocation package, never caller input, so
// they cannot carry injected SQL. Reasons are validated against the same source before any write, so
// a value outside this set can never reach the column.
func boundaryReasonSQLList() string {
	reasons := sharedrevocation.BoundaryReasons()
	values := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		values = append(values, fmt.Sprintf("'%s'", reason))
	}
	return strings.Join(values, ", ")
}

// queryInsertRevokedToken inserts a JTI into the deny list. The write is idempotent: a duplicate
// (DEPLOYMENT_ID, JTI) is a no-op, enforced by the unique index backing the conflict target.
var queryInsertRevokedToken = dbmodel.DBQuery{
	ID: "RVQ-RTS-01",
	Query: `INSERT INTO "REVOKED_TOKEN" (ID, JTI, REVOCATION_REASON, REVOKED_AT, EXPIRY_TIME, ` +
		`DEPLOYMENT_ID) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (DEPLOYMENT_ID, JTI) DO NOTHING`,
}

// queryIsTokenRevoked checks whether a non-expired deny-list entry exists for the given JTI.
var queryIsTokenRevoked = dbmodel.DBQuery{
	ID:    "RVQ-RTS-02",
	Query: `SELECT 1 FROM "REVOKED_TOKEN" WHERE JTI = $1 AND EXPIRY_TIME > $2 AND DEPLOYMENT_ID = $3`,
}

// queryInsertRevocationCriterion records a criteria-based (many-token) revocation. The write is
// idempotent: re-revoking the same (deployment, type, value) refreshes the reason and time bounds
// rather than inserting a duplicate, enforced by the unique index backing the conflict target.
var queryInsertRevocationCriterion = dbmodel.DBQuery{
	ID: "RVQ-RCS-01",
	Query: fmt.Sprintf(`INSERT INTO "REVOCATION_CRITERIA" (ID, CRITERION_TYPE, CRITERION_VALUE, REASON, `+
		`REVOKED_AT, EXPIRY_TIME, DEPLOYMENT_ID) VALUES ($1, $2, $3, $4, $5, $6, $7) `+
		`ON CONFLICT (DEPLOYMENT_ID, CRITERION_TYPE, CRITERION_VALUE) DO UPDATE SET `+
		`REASON = CASE WHEN "REVOCATION_CRITERIA".REASON NOT IN (%s) `+
		`THEN "REVOCATION_CRITERIA".REASON `+
		`ELSE excluded.REASON END, `+
		`REVOKED_AT = CASE WHEN "REVOCATION_CRITERIA".REASON NOT IN (%s) `+
		`OR "REVOCATION_CRITERIA".REVOKED_AT > excluded.REVOKED_AT `+
		`THEN "REVOCATION_CRITERIA".REVOKED_AT `+
		`ELSE excluded.REVOKED_AT END, `+
		`EXPIRY_TIME = CASE WHEN "REVOCATION_CRITERIA".EXPIRY_TIME > excluded.EXPIRY_TIME `+
		`THEN "REVOCATION_CRITERIA".EXPIRY_TIME ELSE excluded.EXPIRY_TIME END`,
		boundaryReasonSQLList(), boundaryReasonSQLList()),
}

// criteriaInsertColumns is how many columns one criteria row writes, which sets how many query
// parameters each row in a batch consumes.
const criteriaInsertColumns = 7

// maxCriteriaRowsPerStatement bounds how many rows one INSERT ... VALUES statement carries, kept well
// under Postgres's 65535-parameter ceiling. It is a statement-shape limit only, separate from
// oauth.revocation.criteria.max_criteria, which bounds one revocation's total rows across statements.
const maxCriteriaRowsPerStatement = 500

// insertRevocationCriteriaQueryID identifies the bulk criteria insert, built per call because its text
// varies with the batch size.
const insertRevocationCriteriaQueryID = "RVQ-RCS-04"

// buildInsertRevocationCriteriaQuery builds one INSERT ... VALUES ON CONFLICT statement for up to
// maxCriteriaRowsPerStatement rows, the batched form of queryInsertRevocationCriterion. It panics
// rather than truncate, since a dropped row is a revocation that was planned but never written.
// Callers must deduplicate (type, value) first, as one statement cannot hit the same target twice.
func buildInsertRevocationCriteriaQuery(rows []revocationCriterion, deploymentID string) (
	dbmodel.DBQuery, []interface{}) {
	if len(rows) > maxCriteriaRowsPerStatement {
		panic(fmt.Sprintf(
			"buildInsertRevocationCriteriaQuery: %d rows exceeds the %d-row limit for one statement",
			len(rows), maxCriteriaRowsPerStatement))
	}

	args := make([]interface{}, 0, len(rows)*criteriaInsertColumns)
	valueGroups := make([]string, 0, len(rows))
	paramIndex := 1
	for _, row := range rows {
		valueGroups = append(valueGroups, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			paramIndex, paramIndex+1, paramIndex+2, paramIndex+3,
			paramIndex+4, paramIndex+5, paramIndex+6))
		args = append(args, row.ID, string(row.Type), row.Value, string(row.Reason),
			row.RevokedAt, row.ExpiryTime, deploymentID)
		paramIndex += criteriaInsertColumns
	}

	query := fmt.Sprintf(`INSERT INTO "REVOCATION_CRITERIA" (ID, CRITERION_TYPE, CRITERION_VALUE, REASON, `+
		`REVOKED_AT, EXPIRY_TIME, DEPLOYMENT_ID) VALUES %s `+
		`ON CONFLICT (DEPLOYMENT_ID, CRITERION_TYPE, CRITERION_VALUE) DO UPDATE SET `+
		`REASON = CASE WHEN "REVOCATION_CRITERIA".REASON NOT IN (%s) `+
		`THEN "REVOCATION_CRITERIA".REASON `+
		`ELSE excluded.REASON END, `+
		`REVOKED_AT = CASE WHEN "REVOCATION_CRITERIA".REASON NOT IN (%s) `+
		`OR "REVOCATION_CRITERIA".REVOKED_AT > excluded.REVOKED_AT `+
		`THEN "REVOCATION_CRITERIA".REVOKED_AT `+
		`ELSE excluded.REVOKED_AT END, `+
		`EXPIRY_TIME = CASE WHEN "REVOCATION_CRITERIA".EXPIRY_TIME > excluded.EXPIRY_TIME `+
		`THEN "REVOCATION_CRITERIA".EXPIRY_TIME ELSE excluded.EXPIRY_TIME END`,
		strings.Join(valueGroups, ", "), boundaryReasonSQLList(), boundaryReasonSQLList())

	return dbmodel.DBQuery{ID: insertRevocationCriteriaQueryID, Query: query}, args
}

// criteriaRevokedQueryID identifies the criteria deny-list existence check. The query text varies
// with the number of criteria supplied, so it is built per call rather than declared as a constant.
const criteriaRevokedQueryID = "RVQ-RCS-02"

// buildCriteriaRevokedQuery returns an existence check covering every supplied criterion in one
// round trip, along with its arguments. Token validation is a hot path, so the dimensions are ORed
// into a single statement instead of issuing one query per criterion.
//
// A row matches when it has not expired and either carries a terminal reason or was recorded at or
// after the artifact's establishment time. Callers must supply at least one criterion with a
// non-empty value; an empty slice would render a query that matches nothing and is rejected upstream.
func buildCriteriaRevokedQuery(criteria []Criterion, establishedAt, now time.Time,
	deploymentID string) (dbmodel.DBQuery, []interface{}) {
	args := make([]interface{}, 0, 3+2*len(criteria))
	args = append(args, deploymentID, now, establishedAt)

	// $1 deployment id, $2 now, $3 established at; criterion pairs start at $4.
	paramIndex := 4
	pairs := make([]string, 0, len(criteria))
	for _, criterion := range criteria {
		pairs = append(pairs,
			fmt.Sprintf("(CRITERION_TYPE = $%d AND CRITERION_VALUE = $%d)", paramIndex, paramIndex+1))
		args = append(args, string(criterion.Type), criterion.Value)
		paramIndex += 2
	}

	query := fmt.Sprintf(`SELECT 1 FROM "REVOCATION_CRITERIA" `+
		`WHERE DEPLOYMENT_ID = $1 AND EXPIRY_TIME > $2 `+
		`AND (REASON NOT IN (%s) OR $3 <= REVOKED_AT) `+
		`AND (%s) LIMIT 1`, boundaryReasonSQLList(), strings.Join(pairs, " OR "))

	return dbmodel.DBQuery{ID: criteriaRevokedQueryID, Query: query}, args
}
