// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"fmt"
	"strings"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

// hydrateBatchSize bounds how many policy IDs one hydration statement matches.
const hydrateBatchSize = 100

const policyColumns = `ID, RESOURCE_TYPE, RESOURCE_ID, OWNING_OU_ID, INITIATING_OU_ID,
	POLICY_STAGE, PARENT_POLICY_ID, SECTIONS, VERSION`

var (
	queryCreatePolicy = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-01",
		Query: `INSERT INTO "RESOURCE_SHARING_POLICY"
			(ID, RESOURCE_TYPE, RESOURCE_ID, OWNING_OU_ID, INITIATING_OU_ID, POLICY_STAGE,
			 PARENT_POLICY_ID, SECTIONS, VERSION, DEPLOYMENT_ID)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
	}

	queryGetPolicyByID = dbmodel.DBQuery{
		ID:    "SHQ-SHARING_MGT-02",
		Query: `SELECT ` + policyColumns + ` FROM "RESOURCE_SHARING_POLICY" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetPolicyByInitiator backs the one-policy-per-organization-unit constraint, so a create
	// can name the existing policy rather than failing on a bare uniqueness violation.
	queryGetPolicyByInitiator = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-03",
		Query: `SELECT ` + policyColumns + ` FROM "RESOURCE_SHARING_POLICY"
			WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND INITIATING_OU_ID = $3 AND DEPLOYMENT_ID = $4`,
	}

	// queryListPoliciesForResource returns one page, for a management API to serve. It orders
	// exactly as queryListAllPoliciesForResource does, so a page boundary falls in the same place
	// on every request and means the same thing the evaluation set does.
	queryListPoliciesForResource = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-04",
		Query: `SELECT ` + policyColumns + ` FROM "RESOURCE_SHARING_POLICY"
			WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND DEPLOYMENT_ID = $3
			ORDER BY CREATED_AT, ID LIMIT $4 OFFSET $5`,
	}

	queryDeletePolicy = dbmodel.DBQuery{
		ID:    "SHQ-SHARING_MGT-05",
		Query: `DELETE FROM "RESOURCE_SHARING_POLICY" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryReplacePolicySections rewrites the document saying which sections a policy states,
	// and is the optimistic-concurrency guard while it does: an edit built on a stale read matches
	// no row, so the caller learns the policy moved instead of silently overwriting.
	queryReplacePolicySections = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-06",
		Query: `UPDATE "RESOURCE_SHARING_POLICY"
			SET SECTIONS = $1, VERSION = VERSION + 1
			WHERE ID = $2 AND VERSION = $3 AND DEPLOYMENT_ID = $4`,
	}

	queryInsertUnit = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-07",
		Query: `INSERT INTO "RESOURCE_SHARING_POLICY_UNIT"
			(POLICY_ID, SECTION, OU_ID, REACH, DEPLOYMENT_ID) VALUES ($1, $2, $3, $4, $5)`,
	}

	queryListUnits = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-08",
		Query: `SELECT SECTION, OU_ID, REACH FROM "RESOURCE_SHARING_POLICY_UNIT"
			WHERE POLICY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryDeleteUnits = dbmodel.DBQuery{
		ID:    "SHQ-SHARING_MGT-09",
		Query: `DELETE FROM "RESOURCE_SHARING_POLICY_UNIT" WHERE POLICY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryInsertRule = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-10",
		Query: `INSERT INTO "RESOURCE_SHARING_POLICY_OVERLAY_RULE"
			(ID, POLICY_ID, SECTION, OU_ID, FIELD_KEY, RESOLVED, REQUESTED, DEPLOYMENT_ID)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
	}

	queryListRules = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-11",
		Query: `SELECT ID, SECTION, OU_ID, FIELD_KEY, RESOLVED, REQUESTED
			FROM "RESOURCE_SHARING_POLICY_OVERLAY_RULE"
			WHERE POLICY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryDeleteRules = dbmodel.DBQuery{
		ID:    "SHQ-SHARING_MGT-12",
		Query: `DELETE FROM "RESOURCE_SHARING_POLICY_OVERLAY_RULE" WHERE POLICY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryGetOverlayValue = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-13",
		Query: `SELECT FIELD_KEY, VALUE FROM "RESOURCE_OVERLAY_VALUE"
			WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND OU_ID = $3 AND DEPLOYMENT_ID = $4`,
	}

	queryUpsertOverlayValue = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-14",
		Query: `INSERT INTO "RESOURCE_OVERLAY_VALUE"
			(RESOURCE_TYPE, RESOURCE_ID, OU_ID, FIELD_KEY, VALUE, DEPLOYMENT_ID)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (DEPLOYMENT_ID, RESOURCE_TYPE, RESOURCE_ID, OU_ID, FIELD_KEY)
			DO UPDATE SET VALUE = excluded.VALUE`,
	}

	queryDeleteOverlayValue = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-15",
		Query: `DELETE FROM "RESOURCE_OVERLAY_VALUE"
			WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND OU_ID = $3 AND FIELD_KEY = $4
			AND DEPLOYMENT_ID = $5`,
	}

	queryDeleteOverlayValuesForOU = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-16",
		Query: `DELETE FROM "RESOURCE_OVERLAY_VALUE"
			WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND OU_ID = $3 AND DEPLOYMENT_ID = $4`,
	}

	// queryCountPoliciesForResource backs the paginated listing's total. Evaluation never counts:
	// it reads the whole set and decides coverage from it.
	queryCountPoliciesForResource = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-17",
		Query: `SELECT COUNT(*) AS TOTAL FROM "RESOURCE_SHARING_POLICY"
			WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryListAllPoliciesForResource returns every policy a resource has, deliberately unbounded.
	//
	// Policy evaluation reads through it, and every question it asks is about the set as a whole:
	// whether exactly one policy covers an organization unit, and what coverage looked like before
	// and after an edit. A row left out is not a shorter answer but a different one, so no limit
	// belongs here. It orders as queryListPoliciesForResource does; the two have to agree.
	queryListAllPoliciesForResource = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-18",
		Query: `SELECT ` + policyColumns + ` FROM "RESOURCE_SHARING_POLICY"
			WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND DEPLOYMENT_ID = $3
			ORDER BY CREATED_AT, ID`,
	}

	// queryListPoliciesIssuedBy returns every policy one organization unit issued, for any
	// resource. It backs the check that keeps an issuing unit from being deleted out from under its
	// policies, which runs only when an organization unit is deleted.
	queryListPoliciesIssuedBy = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-20",
		Query: `SELECT ` + policyColumns + ` FROM "RESOURCE_SHARING_POLICY"
			WHERE INITIATING_OU_ID = $1 AND DEPLOYMENT_ID = $2
			ORDER BY CREATED_AT, ID`,
	}
)

// buildRelevantPoliciesQuery returns every policy of resourceType that could plausibly cover any
// organization unit in the chain: one reaching every unit or every tree, one reaching everything
// beneath a chain member, or one naming a chain member. Coverage itself is decided in memory, so
// this is a bounded fetch and no more.
func buildRelevantPoliciesQuery(
	resourceType ResourceType, resourceID string, chainOUIDs []string, deploymentID string,
) (dbmodel.DBQuery, []interface{}) {
	args := []interface{}{string(resourceType)}
	// An empty resourceID spans every resource of the type, which is what the reverse lookup needs.
	// A named one narrows the fetch to the single resource a visibility question is about, so a
	// resource shared to many organization units is not hydrated in full to answer for one of them.
	if resourceID != "" {
		args = append(args, resourceID)
	}
	// The chain is matched twice, against the units issuing a blanket grant and against the units
	// policies name, so it is bound twice.
	for range 2 {
		for _, id := range chainOUIDs {
			args = append(args, id)
		}
	}
	args = append(args, deploymentID)

	// build renders the statement for one dialect. The two disagree only on how an argument is
	// marked, so placeholders are drawn in the same order the arguments were appended above, and in
	// the same order they appear in the statement. SQLite binds a bare ? by its position in the
	// text, so those two orders have to be the one order or the arguments land on the wrong columns.
	build := func(placeholder func(n int) string) string {
		next := 0
		take := func() string {
			next++
			return placeholder(next)
		}
		takeChain := func() string {
			placeholders := make([]string, len(chainOUIDs))
			for i := range chainOUIDs {
				placeholders[i] = take()
			}
			return strings.Join(placeholders, ",")
		}

		resource := take()
		resourceFilter := ""
		if resourceID != "" {
			resourceFilter = fmt.Sprintf(" AND p.RESOURCE_ID = %s", take())
		}

		// An empty chain names no organization unit, so the membership tests are left out entirely
		// rather than emitted as IN (), which is not valid SQL. The deployment-wide grants still
		// match.
		//
		// Which sections a policy states, and in which mode, is read out of its SECTIONS document.
		// Both engines read a key with -> and a text value with ->>, so the statement is the same
		// for the two. The names and modes are rendered from the constants policies are stored
		// with, so the two cannot drift apart and leave blanket policies unmatched.
		reach := fmt.Sprintf("p.SECTIONS->'%s' IS NOT NULL OR p.SECTIONS->'%s'->>'mode' = '%s'",
			SectionAllUnits, SectionOtherTreeRoots, OtherTreeRootsAll)
		if len(chainOUIDs) > 0 {
			reach += fmt.Sprintf(" OR (p.SECTIONS->'%s'->>'mode' = '%s' AND p.INITIATING_OU_ID IN (%s))",
				SectionChildUnits, ChildUnitsAll, takeChain())
			reach += fmt.Sprintf(` OR EXISTS (SELECT 1 FROM "RESOURCE_SHARING_POLICY_UNIT" u
				WHERE u.POLICY_ID = p.ID AND u.DEPLOYMENT_ID = p.DEPLOYMENT_ID AND u.OU_ID IN (%s))`,
				takeChain())
		}
		deployment := take()

		return fmt.Sprintf(
			`SELECT p.ID, p.RESOURCE_TYPE, p.RESOURCE_ID, p.OWNING_OU_ID, p.INITIATING_OU_ID,
				p.POLICY_STAGE, p.PARENT_POLICY_ID, p.SECTIONS, p.VERSION
			FROM "RESOURCE_SHARING_POLICY" p
			WHERE p.RESOURCE_TYPE = %s%s
			AND (%s)
			AND p.DEPLOYMENT_ID = %s`,
			resource, resourceFilter, reach, deployment)
	}

	postgresQuery := build(func(n int) string { return fmt.Sprintf("$%d", n) })
	sqliteQuery := build(func(int) string { return "?" })

	return dbmodel.DBQuery{
		ID:            "SHQ-SHARING_MGT-19",
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}, args
}

// buildListUnitsForPoliciesQuery returns the named units of every policy in policyIDs, so a set of
// policies is hydrated in one statement rather than one per policy.
func buildListUnitsForPoliciesQuery(policyIDs []string, deploymentID string) (dbmodel.DBQuery, []interface{}) {
	return buildPolicyIDsQuery("SHQ-SHARING_MGT-21",
		`SELECT POLICY_ID, SECTION, OU_ID, REACH FROM "RESOURCE_SHARING_POLICY_UNIT"
			WHERE POLICY_ID IN (%s) AND DEPLOYMENT_ID = %s`,
		policyIDs, deploymentID)
}

// buildListRulesForPoliciesQuery returns the overlay rules of every policy in policyIDs, for the same
// reason.
func buildListRulesForPoliciesQuery(policyIDs []string, deploymentID string) (dbmodel.DBQuery, []interface{}) {
	return buildPolicyIDsQuery("SHQ-SHARING_MGT-22",
		`SELECT POLICY_ID, ID, SECTION, OU_ID, FIELD_KEY, RESOLVED, REQUESTED
			FROM "RESOURCE_SHARING_POLICY_OVERLAY_RULE"
			WHERE POLICY_ID IN (%s) AND DEPLOYMENT_ID = %s`,
		policyIDs, deploymentID)
}

// buildPolicyIDsQuery renders a statement that matches a list of policy IDs for both dialects.
func buildPolicyIDsQuery(
	queryID, statement string, policyIDs []string, deploymentID string,
) (dbmodel.DBQuery, []interface{}) {
	args := make([]interface{}, 0, len(policyIDs)+1)
	postgresPlaceholders := make([]string, len(policyIDs))
	sqlitePlaceholders := make([]string, len(policyIDs))
	for i, id := range policyIDs {
		postgresPlaceholders[i] = fmt.Sprintf("$%d", i+1)
		sqlitePlaceholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, deploymentID)

	postgresQuery := fmt.Sprintf(statement, strings.Join(postgresPlaceholders, ","),
		fmt.Sprintf("$%d", len(policyIDs)+1))
	sqliteQuery := fmt.Sprintf(statement, strings.Join(sqlitePlaceholders, ","), "?")
	return dbmodel.DBQuery{
		ID:            queryID,
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}, args
}
