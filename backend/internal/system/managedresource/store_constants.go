// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package managedresource

import (
	"fmt"
	"strings"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// queryMark records a resource, or one entry of it, as deployed by a control plane. Marking it
	// again keeps the row it has.
	queryMark = dbmodel.DBQuery{
		ID: "MRQ-MANAGED_MGT-01",
		Query: `INSERT INTO "MANAGED_RESOURCE" (RESOURCE_TYPE, RESOURCE_ID, PART, PART_KEY, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
	}
	// queryUnmark forgets a resource and every entry of it.
	queryUnmark = dbmodel.DBQuery{
		ID: "MRQ-MANAGED_MGT-02",
		Query: `DELETE FROM "MANAGED_RESOURCE" ` +
			`WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND DEPLOYMENT_ID = $3`,
	}
	// queryGetTypes reads the types a resource id is marked under, which is one unless two types share
	// an id space.
	queryGetTypes = dbmodel.DBQuery{
		ID: "MRQ-MANAGED_MGT-04",
		Query: `SELECT RESOURCE_TYPE FROM "MANAGED_RESOURCE" ` +
			`WHERE RESOURCE_ID = $1 AND PART = '' AND DEPLOYMENT_ID = $2`,
	}
	// queryGetPartKeys reads the entries of one collection of a resource that are marked.
	queryGetPartKeys = dbmodel.DBQuery{
		ID: "MRQ-MANAGED_MGT-05",
		Query: `SELECT PART_KEY FROM "MANAGED_RESOURCE" ` +
			`WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND PART = $3 AND DEPLOYMENT_ID = $4`,
	}
	// queryIsPartMarked reads whether one entry of a resource is marked.
	queryIsPartMarked = dbmodel.DBQuery{
		ID: "MRQ-MANAGED_MGT-06",
		Query: `SELECT 1 AS marked FROM "MANAGED_RESOURCE" ` +
			`WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND PART = $3 AND PART_KEY = $4 AND DEPLOYMENT_ID = $5`,
	}
)

// buildManagedIDsQuery reads which of the ids are marked, and under which types, in one query.
func buildManagedIDsQuery(ids []string, deploymentID string) (dbmodel.DBQuery, []interface{}) {
	placeholders := make([]string, len(ids))
	args := make([]interface{}, 0, len(ids)+1)
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args = append(args, id)
	}
	args = append(args, deploymentID)
	return dbmodel.DBQuery{
		ID: "MRQ-MANAGED_MGT-07",
		Query: fmt.Sprintf(`SELECT RESOURCE_ID, RESOURCE_TYPE FROM "MANAGED_RESOURCE" `+
			`WHERE RESOURCE_ID IN (%s) AND PART = '' AND DEPLOYMENT_ID = $%d`,
			strings.Join(placeholders, ","), len(ids)+1),
	}, args
}

// partsChunk bounds how many entries one statement marks or forgets, which keeps its parameters well under
// what a database takes.
const partsChunk = 100

// buildMarkPartsQuery records entries of one collection of a resource as deployed, in one statement. Entries
// already marked keep the row they have.
func buildMarkPartsQuery(resourceType, id, part string, keys []string, deploymentID string) (
	dbmodel.DBQuery, []interface{}) {
	rows := make([]string, len(keys))
	args := make([]interface{}, 0, len(keys)*5)
	for i, key := range keys {
		n := i * 5
		rows[i] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)", n+1, n+2, n+3, n+4, n+5)
		args = append(args, resourceType, id, part, key, deploymentID)
	}
	return dbmodel.DBQuery{
		ID: "MRQ-MANAGED_MGT-08",
		Query: `INSERT INTO "MANAGED_RESOURCE" (RESOURCE_TYPE, RESOURCE_ID, PART, PART_KEY, DEPLOYMENT_ID) VALUES ` +
			strings.Join(rows, ", ") + ` ON CONFLICT DO NOTHING`,
	}, args
}

// buildUnmarkPartsQuery forgets entries of one collection of a resource, in one statement.
func buildUnmarkPartsQuery(resourceType, id, part string, keys []string, deploymentID string) (
	dbmodel.DBQuery, []interface{}) {
	placeholders := make([]string, len(keys))
	args := make([]interface{}, 0, len(keys)+4)
	args = append(args, resourceType, id, part, deploymentID)
	for i, key := range keys {
		placeholders[i] = fmt.Sprintf("$%d", i+5)
		args = append(args, key)
	}
	return dbmodel.DBQuery{
		ID: "MRQ-MANAGED_MGT-09",
		Query: `DELETE FROM "MANAGED_RESOURCE" WHERE RESOURCE_TYPE = $1 AND RESOURCE_ID = $2 AND PART = $3 ` +
			`AND DEPLOYMENT_ID = $4 AND PART_KEY IN (` + strings.Join(placeholders, ",") + `)`,
	}, args
}
