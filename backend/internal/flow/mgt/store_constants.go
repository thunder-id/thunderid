// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// queryCreateFlow is the query to creates a new flow definition.
	queryCreateFlow = model.DBQuery{
		ID: "FLQ-FLOW_MGT-01",
		Query: `INSERT INTO "FLOW" (ID, HANDLE, NAME, FLOW_TYPE, ACTIVE_VERSION, OU_ID, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6, $7)`,
	}

	// queryGetFlow is the query to retrieves a flow definition by its ID.
	queryGetFlow = model.DBQuery{
		ID: "FLQ-FLOW_MGT-02",
		Query: `SELECT f.ID, f.HANDLE, f.NAME, f.FLOW_TYPE, f.ACTIVE_VERSION, f.OU_ID, ` +
			`fv.NODES, fv.INTERCEPTORS, f.CREATED_AT, ` +
			`f.UPDATED_AT FROM "FLOW" f INNER JOIN "FLOW_VERSION" fv ON f.ID = fv.FLOW_ID ` +
			`AND f.DEPLOYMENT_ID = fv.DEPLOYMENT_ID AND f.ACTIVE_VERSION = fv.VERSION ` +
			`WHERE f.ID = $1 AND f.DEPLOYMENT_ID = $2`,
	}

	// queryUpdateFlow is the query to updates an existing flow definition.
	queryUpdateFlow = model.DBQuery{
		ID: "FLQ-FLOW_MGT-04",
		Query: `UPDATE "FLOW" SET NAME = $2, ACTIVE_VERSION = $3, ` +
			`UPDATED_AT = datetime('now') WHERE ID = $1 AND DEPLOYMENT_ID = $4`,
		SQLiteQuery: `UPDATE "FLOW" SET NAME = $2, ACTIVE_VERSION = $3, ` +
			`UPDATED_AT = datetime('now') WHERE ID = $1 AND DEPLOYMENT_ID = $4`,
		PostgresQuery: `UPDATE "FLOW" SET NAME = $2, ACTIVE_VERSION = $3, ` +
			`UPDATED_AT = CURRENT_TIMESTAMP WHERE ID = $1 AND DEPLOYMENT_ID = $4`,
	}

	// queryListFlows is the query to retrieves a list of flow definitions.
	queryListFlows = model.DBQuery{
		ID: "FLQ-FLOW_MGT-05",
		Query: `SELECT ID, HANDLE, NAME, FLOW_TYPE, ACTIVE_VERSION, OU_ID, CREATED_AT, UPDATED_AT ` +
			`FROM "FLOW" WHERE DEPLOYMENT_ID = $1 ORDER BY CREATED_AT DESC LIMIT $2 OFFSET $3`,
	}

	// queryListActiveFlowsWithNodes retrieves every flow's active version including its nodes, in a
	// single query, for dependency lookups that must scan node definitions across all flows.
	queryListActiveFlowsWithNodes = model.DBQuery{
		ID: "FLQ-FLOW_MGT-19",
		Query: `SELECT f.ID, f.HANDLE, f.NAME, f.FLOW_TYPE, f.ACTIVE_VERSION, f.OU_ID, ` +
			`fv.NODES, fv.INTERCEPTORS, f.CREATED_AT, f.UPDATED_AT ` +
			`FROM "FLOW" f INNER JOIN "FLOW_VERSION" fv ON f.ID = fv.FLOW_ID ` +
			`AND f.DEPLOYMENT_ID = fv.DEPLOYMENT_ID AND f.ACTIVE_VERSION = fv.VERSION ` +
			`WHERE f.DEPLOYMENT_ID = $1`,
	}

	// queryListFlowsWithType is the query to retrieves a list of flow definitions filtered by type.
	queryListFlowsWithType = model.DBQuery{
		ID: "FLQ-FLOW_MGT-06",
		Query: `SELECT ID, HANDLE, NAME, FLOW_TYPE, ACTIVE_VERSION, OU_ID, CREATED_AT, UPDATED_AT FROM "FLOW" ` +
			`WHERE FLOW_TYPE = $1 AND DEPLOYMENT_ID = $2 ORDER BY CREATED_AT DESC LIMIT $3 OFFSET $4`,
	}

	// queryCountFlows is the query to count total flow definitions.
	queryCountFlows = model.DBQuery{
		ID:    "FLQ-FLOW_MGT-07",
		Query: `SELECT COUNT(*) AS count FROM "FLOW" WHERE DEPLOYMENT_ID = $1`,
	}

	// queryCountFlowsWithType is the query to count total flow definitions filtered by type.
	queryCountFlowsWithType = model.DBQuery{
		ID:    "FLQ-FLOW_MGT-08",
		Query: `SELECT COUNT(*) AS count FROM "FLOW" WHERE FLOW_TYPE = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryDeleteFlow is the query to delete a flow definition by its ID.
	queryDeleteFlow = model.DBQuery{
		ID:    "FLQ-FLOW_MGT-09",
		Query: `DELETE FROM "FLOW" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryInsertFlowVersion is the query to insert a new version of a flow.
	queryInsertFlowVersion = model.DBQuery{
		ID: "FLQ-FLOW_MGT-10",
		Query: `INSERT INTO "FLOW_VERSION" (FLOW_ID, VERSION, NODES, INTERCEPTORS, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5)`,
	}

	// queryGetFlowVersion is the query to retrieve a specific version of a flow.
	queryGetFlowVersion = model.DBQuery{
		ID: "FLQ-FLOW_MGT-11",
		Query: `SELECT VERSION, NODES, INTERCEPTORS, CREATED_AT FROM "FLOW_VERSION" WHERE ` +
			`FLOW_ID = $1 AND VERSION = $2 AND DEPLOYMENT_ID = $3`,
	}

	// queryGetFlowVersionWithMetadata is the query to retrieve a specific version with flow metadata.
	queryGetFlowVersionWithMetadata = model.DBQuery{
		ID: "FLQ-FLOW_MGT-12",
		Query: `SELECT f.ID, f.HANDLE, f.NAME, f.FLOW_TYPE, f.ACTIVE_VERSION, fv.VERSION, fv.NODES, fv.INTERCEPTORS, ` +
			`fv.CREATED_AT FROM "FLOW" f INNER JOIN "FLOW_VERSION" fv ON f.ID = fv.FLOW_ID ` +
			`AND f.DEPLOYMENT_ID = fv.DEPLOYMENT_ID WHERE f.ID = $1 AND fv.VERSION = $2 ` +
			`AND f.DEPLOYMENT_ID = $3`,
	}

	// queryListFlowVersions is the query to list all versions of a flow.
	queryListFlowVersions = model.DBQuery{
		ID: "FLQ-FLOW_MGT-13",
		Query: `SELECT fv.VERSION, fv.CREATED_AT, f.ACTIVE_VERSION FROM "FLOW_VERSION" fv ` +
			`INNER JOIN "FLOW" f ON fv.FLOW_ID = f.ID AND fv.DEPLOYMENT_ID = f.DEPLOYMENT_ID ` +
			`WHERE fv.FLOW_ID = $1 AND fv.DEPLOYMENT_ID = $2 ` +
			`ORDER BY fv.VERSION DESC`,
	}

	// queryCountFlowVersions is the query to count total versions of a flow.
	queryCountFlowVersions = model.DBQuery{
		ID:    "FLQ-FLOW_MGT-14",
		Query: `SELECT COUNT(*) AS count FROM "FLOW_VERSION" WHERE FLOW_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryDeleteOldestVersion is the query to delete the oldest version of a flow.
	queryDeleteOldestVersion = model.DBQuery{
		ID: "FLQ-FLOW_MGT-15",
		Query: `DELETE FROM "FLOW_VERSION" WHERE FLOW_ID = $1 AND DEPLOYMENT_ID = $2 AND ` +
			`VERSION = (SELECT MIN(VERSION) FROM "FLOW_VERSION" WHERE FLOW_ID = $1 AND DEPLOYMENT_ID = $2)`,
	}

	// queryCheckFlowExistsByHandle is the query to check if a flow exists by handle and flow type.
	queryCheckFlowExistsByHandle = model.DBQuery{
		ID:    "FLQ-FLOW_MGT-17",
		Query: `SELECT 1 FROM "FLOW" WHERE HANDLE = $1 AND FLOW_TYPE = $2 AND DEPLOYMENT_ID = $3 LIMIT 1`,
	}

	// queryGetFlowByHandle retrieves a flow definition by handle and flow type.
	queryGetFlowByHandle = model.DBQuery{
		ID: "FLQ-FLOW_MGT-18",
		Query: `SELECT f.ID, f.HANDLE, f.NAME, f.FLOW_TYPE, f.ACTIVE_VERSION, f.OU_ID, ` +
			`fv.NODES, fv.INTERCEPTORS, f.CREATED_AT, ` +
			`f.UPDATED_AT FROM "FLOW" f INNER JOIN "FLOW_VERSION" fv ON f.ID = fv.FLOW_ID ` +
			`AND f.DEPLOYMENT_ID = fv.DEPLOYMENT_ID AND f.ACTIVE_VERSION = fv.VERSION ` +
			`WHERE f.HANDLE = $1 AND f.FLOW_TYPE = $2 AND f.DEPLOYMENT_ID = $3`,
	}

	// queryListFlowsByOU lists the flow definitions an organization unit owns.
	queryListFlowsByOU = model.DBQuery{
		ID: "FLQ-FLOW_MGT-20",
		Query: `SELECT ID, HANDLE, NAME, FLOW_TYPE, ACTIVE_VERSION, OU_ID, CREATED_AT, UPDATED_AT FROM "FLOW" ` +
			`WHERE OU_ID = $1 AND DEPLOYMENT_ID = $2 ORDER BY CREATED_AT DESC LIMIT $3 OFFSET $4`,
	}

	// queryListFlowsByOUWithType lists the flow definitions of one type an organization unit owns.
	queryListFlowsByOUWithType = model.DBQuery{
		ID: "FLQ-FLOW_MGT-21",
		Query: `SELECT ID, HANDLE, NAME, FLOW_TYPE, ACTIVE_VERSION, OU_ID, CREATED_AT, UPDATED_AT FROM "FLOW" ` +
			`WHERE OU_ID = $1 AND FLOW_TYPE = $2 AND DEPLOYMENT_ID = $3 ` +
			`ORDER BY CREATED_AT DESC LIMIT $4 OFFSET $5`,
	}

	// queryCountFlowsByOU counts the flow definitions an organization unit owns.
	queryCountFlowsByOU = model.DBQuery{
		ID:    "FLQ-FLOW_MGT-22",
		Query: `SELECT COUNT(*) AS count FROM "FLOW" WHERE OU_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCountFlowsByOUWithType counts the flow definitions of one type an organization unit owns.
	queryCountFlowsByOUWithType = model.DBQuery{
		ID:    "FLQ-FLOW_MGT-23",
		Query: `SELECT COUNT(*) AS count FROM "FLOW" WHERE OU_ID = $1 AND FLOW_TYPE = $2 AND DEPLOYMENT_ID = $3`,
	}
)

// buildListFlowsByIDsQuery builds the query that reads the flows with the given IDs, of one type when flowType is set.
// The placeholders differ by engine, so both forms are given.
func buildListFlowsByIDsQuery(ids []string, flowType, deploymentID string) (model.DBQuery, []interface{}) {
	const columns = `SELECT ID, HANDLE, NAME, FLOW_TYPE, ACTIVE_VERSION, OU_ID, CREATED_AT, UPDATED_AT FROM "FLOW" `
	args := []interface{}{deploymentID}
	postgres := columns + `WHERE DEPLOYMENT_ID = $1`
	sqlite := columns + `WHERE DEPLOYMENT_ID = ?`
	if flowType != "" {
		args = append(args, flowType)
		postgres += fmt.Sprintf(" AND FLOW_TYPE = $%d", len(args))
		sqlite += " AND FLOW_TYPE = ?"
	}
	pg := make([]string, len(ids))
	lite := make([]string, len(ids))
	for i, id := range ids {
		args = append(args, id)
		pg[i] = fmt.Sprintf("$%d", len(args))
		lite[i] = "?"
	}
	postgres += " AND ID IN (" + strings.Join(pg, ", ") + ")"
	sqlite += " AND ID IN (" + strings.Join(lite, ", ") + ")"

	return model.DBQuery{
		ID:            "FLQ-FLOW_MGT-24",
		Query:         postgres,
		PostgresQuery: postgres,
		SQLiteQuery:   sqlite,
	}, args
}
