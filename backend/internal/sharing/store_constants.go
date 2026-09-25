// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sharing

import (
	"fmt"
	"strings"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

const policyColumns = `ID, RESOURCE_TYPE, RESOURCE_ID, OWNING_OU_ID, INITIATING_OU_ID,
	POLICY_STAGE, PARENT_POLICY_ID, DECLARED, VERSION`

var (
	queryCreatePolicy = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-01",
		Query: `INSERT INTO "RESOURCE_SHARING_POLICY"
			(ID, RESOURCE_TYPE, RESOURCE_ID, OWNING_OU_ID, INITIATING_OU_ID, POLICY_STAGE,
			 PARENT_POLICY_ID, DECLARED, VERSION, DEPLOYMENT_ID)
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

	queryListPoliciesForResource = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-04",
		Query: `SELECT ` + policyColumns + ` FROM "RESOURCE_SHARING_POLICY"
			WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND DEPLOYMENT_ID = $3
			ORDER BY CREATED_AT, ID`,
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
		Query: `INSERT INTO "RESOURCE_SHARING_POLICY_EXCLUSION"
			(POLICY_ID, EXCLUDED_OU_ID, DEPLOYMENT_ID) VALUES ($1, $2, $3)`,
	}

	queryListExclusions = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-11",
		Query: `SELECT EXCLUDED_OU_ID FROM "RESOURCE_SHARING_POLICY_EXCLUSION"
			WHERE POLICY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryDeleteExclusions = dbmodel.DBQuery{
		ID:    "SHQ-SHARING_MGT-12",
		Query: `DELETE FROM "RESOURCE_SHARING_POLICY_EXCLUSION" WHERE POLICY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryInsertRule = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-13",
		Query: `INSERT INTO "RESOURCE_SHARING_POLICY_OVERLAY_RULE"
			(ID, POLICY_ID, TARGET_ID, FIELD_KEY, EDITABLE, VALUE_SET, ALLOWED_SET, EXCLUDED_SET,
			 REQUESTED_EDITABLE, REQ_VALUE_SET, REQ_ALLOWED_SET, REQ_EXCLUDED_SET, DEPLOYMENT_ID)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
	}

	queryListRules = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-14",
		Query: `SELECT ID, TARGET_ID, FIELD_KEY, EDITABLE, VALUE_SET, ALLOWED_SET, EXCLUDED_SET,
			REQUESTED_EDITABLE, REQ_VALUE_SET, REQ_ALLOWED_SET, REQ_EXCLUDED_SET
			FROM "RESOURCE_SHARING_POLICY_OVERLAY_RULE" WHERE POLICY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryDeleteRules = dbmodel.DBQuery{
		ID:    "SHQ-SHARING_MGT-15",
		Query: `DELETE FROM "RESOURCE_SHARING_POLICY_OVERLAY_RULE" WHERE POLICY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryInsertRuleMember = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-16",
		Query: `INSERT INTO "RESOURCE_SHARING_POLICY_OVERLAY_RULE_MEMBER"
			(RULE_ID, MEMBER_ROLE, MEMBER_KEY, DEPLOYMENT_ID) VALUES ($1, $2, $3, $4)`,
	}

	queryListRuleMembers = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-17",
		Query: `SELECT MEMBER_ROLE, MEMBER_KEY FROM "RESOURCE_SHARING_POLICY_OVERLAY_RULE_MEMBER"
			WHERE RULE_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	queryGetOverlayValue = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-18",
		Query: `SELECT FIELD_KEY, VALUE FROM "RESOURCE_OVERLAY_VALUE"
			WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND OU_ID = $3 AND DEPLOYMENT_ID = $4`,
	}

	queryUpsertOverlayValue = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-19",
		Query: `INSERT INTO "RESOURCE_OVERLAY_VALUE"
			(RESOURCE_TYPE, RESOURCE_ID, OU_ID, FIELD_KEY, VALUE, DEPLOYMENT_ID)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (DEPLOYMENT_ID, RESOURCE_TYPE, RESOURCE_ID, OU_ID, FIELD_KEY)
			DO UPDATE SET VALUE = excluded.VALUE`,
	}

	queryDeleteOverlayValue = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-20",
		Query: `DELETE FROM "RESOURCE_OVERLAY_VALUE"
			WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND OU_ID = $3 AND FIELD_KEY = $4
			AND DEPLOYMENT_ID = $5`,
	}

	// Ids alone, because the only question asked of them is which declared policies have been
	// written to the database. Hydrating a policy to answer that would defeat the narrowed fetch
	// this exists to support.
	queryListPolicyIDsForResource = dbmodel.DBQuery{
		ID: "SHQ-SHARING_MGT-22",
		Query: `SELECT ID FROM "RESOURCE_SHARING_POLICY"
			WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND DEPLOYMENT_ID = $3`,
	}
)

// buildRelevantPoliciesQuery returns every policy of resourceType that could plausibly cover any
// organization unit in the chain: one reaching everything, one reaching any root, or one naming a
// chain member. Coverage itself is decided in memory, so this is a bounded fetch and no more.
func buildRelevantPoliciesQuery(
	resourceType ResourceType, resourceID string, chainOUIDs []string, deploymentID string,
) (dbmodel.DBQuery, []interface{}) {
	args := []interface{}{deploymentID, string(resourceType)}
	// An empty resourceID spans every resource of the type, which is what the reverse lookup needs.
	// A named one narrows the fetch to the single resource a visibility question is about, so a
	// resource shared to many organization units is not hydrated in full to answer for one of them.
	if resourceID != "" {
		args = append(args, resourceID)
	}
	for _, id := range chainOUIDs {
		args = append(args, id)
	}

	// build renders the statement for one dialect. The two disagree only on how an argument is
	// marked, so placeholders are drawn in the same order the arguments were appended above and
	// the two can never fall out of step.
	build := func(placeholder func(n int) string) string {
		next := 0
		take := func() string {
			next++
			return placeholder(next)
		}

		deployment := take()
		resource := take()
		resourceFilter := ""
		if resourceID != "" {
			resourceFilter = fmt.Sprintf(" AND p.RESOURCE_ID = %s", take())
		}

		// An empty chain names no organization unit, so the membership test is left out entirely
		// rather than emitted as IN (), which is not valid SQL. The blanket scopes still match.
		reach := "t.TARGET_SCOPE IN ('all_ous', 'all_roots')"
		if len(chainOUIDs) > 0 {
			placeholders := make([]string, len(chainOUIDs))
			for i := range chainOUIDs {
				placeholders[i] = take()
			}
			reach += fmt.Sprintf(" OR t.TARGET_OU_ID IN (%s)", strings.Join(placeholders, ","))
		}

		return fmt.Sprintf(
			`SELECT DISTINCT p.ID, p.RESOURCE_TYPE, p.RESOURCE_ID, p.OWNING_OU_ID, p.INITIATING_OU_ID,
				p.POLICY_STAGE, p.PARENT_POLICY_ID, p.DECLARED, p.VERSION
			FROM "RESOURCE_SHARING_POLICY" p
			JOIN "RESOURCE_SHARING_POLICY_TARGET" t
				ON t.POLICY_ID = p.ID AND t.DEPLOYMENT_ID = p.DEPLOYMENT_ID
			WHERE p.DEPLOYMENT_ID = %s AND p.RESOURCE_TYPE = %s%s
			AND (%s)`,
			deployment, resource, resourceFilter, reach)
	}

	postgresQuery := build(func(n int) string { return fmt.Sprintf("$%d", n) })
	sqliteQuery := build(func(int) string { return "?" })

	return dbmodel.DBQuery{
		ID:            "SHQ-SHARING_MGT-21",
		Query:         postgresQuery,
		PostgresQuery: postgresQuery,
		SQLiteQuery:   sqliteQuery,
	}, args
}
