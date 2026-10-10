// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"fmt"
	"strings"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

const policyColumns = `ID, RESOURCE_TYPE, RESOURCE_ID, OWNING_OU_ID, INITIATING_OU_ID,
	POLICY_STAGE, PARENT_POLICY_ID, VERSION`

var (
	queryCreatePolicy = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-01",
		Query: `INSERT INTO "RESOURCE_SHARING_POLICY"
			(ID, RESOURCE_TYPE, RESOURCE_ID, OWNING_OU_ID, INITIATING_OU_ID, POLICY_STAGE,
			 PARENT_POLICY_ID, VERSION, DEPLOYMENT_ID)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
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

	// queryBumpPolicyVersion is the optimistic-concurrency guard: an edit built on a stale read
	// matches no row, so the caller learns the policy moved instead of silently overwriting.
	queryBumpPolicyVersion = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-06",
		Query: `UPDATE "RESOURCE_SHARING_POLICY" SET VERSION = VERSION + 1
			WHERE ID = $1 AND VERSION = $2 AND DEPLOYMENT_ID = $3`,
	}

	queryInsertTarget = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-07",
		Query: `INSERT INTO "RESOURCE_SHARING_POLICY_TARGET"
			(ID, POLICY_ID, TARGET_SCOPE, TARGET_OU_ID, DEPLOYMENT_ID) VALUES ($1, $2, $3, $4, $5)`,
	}

	queryListTargets = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-08",
		Query: `SELECT ID, TARGET_SCOPE, TARGET_OU_ID FROM "RESOURCE_SHARING_POLICY_TARGET"
			WHERE POLICY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryDeleteTargets = dbmodel.DBQuery{
		ID:    "SHQ-SHARING_MGT-09",
		Query: `DELETE FROM "RESOURCE_SHARING_POLICY_TARGET" WHERE POLICY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryInsertExclusion = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-10",
		Query: `INSERT INTO "RESOURCE_SHARING_POLICY_TARGET_EXCLUSIONS"
			(POLICY_ID, TARGET_ID, EXCLUDED_OU_ID, DEPLOYMENT_ID) VALUES ($1, $2, $3, $4)`,
	}

	queryListExclusions = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-11",
		Query: `SELECT TARGET_ID, EXCLUDED_OU_ID FROM "RESOURCE_SHARING_POLICY_TARGET_EXCLUSIONS"
			WHERE POLICY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryDeleteExclusions = dbmodel.DBQuery{
		ID:    "SHQ-SHARING_MGT-12",
		Query: `DELETE FROM "RESOURCE_SHARING_POLICY_TARGET_EXCLUSIONS" WHERE POLICY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryInsertRule = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-13",
		Query: `INSERT INTO "RESOURCE_SHARING_POLICY_OVERLAY_RULE"
			(ID, POLICY_ID, TARGET_ID, FIELD_KEY, RESOLVED, REQUESTED, DEPLOYMENT_ID)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
	}

	queryListRules = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-14",
		Query: `SELECT ID, TARGET_ID, FIELD_KEY, RESOLVED, REQUESTED
			FROM "RESOURCE_SHARING_POLICY_OVERLAY_RULE"
			WHERE POLICY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryDeleteRules = dbmodel.DBQuery{
		ID:    "SHQ-SHARING_MGT-15",
		Query: `DELETE FROM "RESOURCE_SHARING_POLICY_OVERLAY_RULE" WHERE POLICY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryGetOverlayValue = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-16",
		Query: `SELECT FIELD_KEY, VALUE FROM "RESOURCE_OVERLAY_VALUE"
			WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND OU_ID = $3 AND DEPLOYMENT_ID = $4`,
	}

	queryUpsertOverlayValue = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-17",
		Query: `INSERT INTO "RESOURCE_OVERLAY_VALUE"
			(RESOURCE_TYPE, RESOURCE_ID, OU_ID, FIELD_KEY, VALUE, DEPLOYMENT_ID)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (DEPLOYMENT_ID, RESOURCE_TYPE, RESOURCE_ID, OU_ID, FIELD_KEY)
			DO UPDATE SET VALUE = excluded.VALUE`,
	}

	queryDeleteOverlayValue = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-18",
		Query: `DELETE FROM "RESOURCE_OVERLAY_VALUE"
			WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND OU_ID = $3 AND FIELD_KEY = $4
			AND DEPLOYMENT_ID = $5`,
	}

	queryDeleteOverlayValuesForOU = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-19",
		Query: `DELETE FROM "RESOURCE_OVERLAY_VALUE"
			WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND OU_ID = $3 AND DEPLOYMENT_ID = $4`,
	}

	// queryCountPoliciesForResource backs the paginated listing's total. Evaluation never counts:
	// it reads the whole set and decides coverage from it.
	queryCountPoliciesForResource = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-21",
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
		ID: "SHQ-SHARING_MGT-22",
		Query: `SELECT ` + policyColumns + ` FROM "RESOURCE_SHARING_POLICY"
			WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND DEPLOYMENT_ID = $3
			ORDER BY CREATED_AT, ID`,
	}
)

// buildRelevantPoliciesQuery returns every policy of resourceType that could plausibly cover any
// organization unit in the chain: one reaching everything, one reaching any root, or one naming a
// chain member. Coverage itself is decided in memory, so this is a bounded fetch and no more.
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
	for _, id := range chainOUIDs {
		args = append(args, id)
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

		resource := take()
		resourceFilter := ""
		if resourceID != "" {
			resourceFilter = fmt.Sprintf(" AND p.RESOURCE_ID = %s", take())
		}

		// An empty chain names no organization unit, so the membership test is left out entirely
		// rather than emitted as IN (), which is not valid SQL. The blanket scopes still match.
		// They are rendered from the scope constants, the values targets are stored with, so the
		// two cannot drift apart and leave blanket policies unmatched.
		reach := fmt.Sprintf("t.TARGET_SCOPE IN ('%s', '%s')", ScopeAllOUs, ScopeAllRoots)
		if len(chainOUIDs) > 0 {
			placeholders := make([]string, len(chainOUIDs))
			for i := range chainOUIDs {
				placeholders[i] = take()
			}
			reach += fmt.Sprintf(" OR t.TARGET_OU_ID IN (%s)", strings.Join(placeholders, ","))
		}
		deployment := take()

		return fmt.Sprintf(
			`SELECT DISTINCT p.ID, p.RESOURCE_TYPE, p.RESOURCE_ID, p.OWNING_OU_ID, p.INITIATING_OU_ID,
				p.POLICY_STAGE, p.PARENT_POLICY_ID, p.VERSION
			FROM "RESOURCE_SHARING_POLICY" p
			JOIN "RESOURCE_SHARING_POLICY_TARGET" t
				ON t.POLICY_ID = p.ID AND t.DEPLOYMENT_ID = p.DEPLOYMENT_ID
			WHERE p.RESOURCE_TYPE = %s%s
			AND (%s)
			AND p.DEPLOYMENT_ID = %s`,
			resource, resourceFilter, reach, deployment)
	}

	postgresQuery := build(func(n int) string { return fmt.Sprintf("$%d", n) })
	sqliteQuery := build(func(int) string { return "?" })

	return dbmodel.DBQuery{
		ID:            "SHQ-SHARING_MGT-20",
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}, args
}
