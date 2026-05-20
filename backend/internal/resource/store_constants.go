/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package resource

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

// Resource Server Queries
var (
	// queryCreateResourceServer creates a new resource server.
	queryCreateResourceServer = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-01",
		Query: `INSERT INTO "RESOURCE_SERVER"
			(ID, OU_ID, NAME, DESCRIPTION, HANDLE, IDENTIFIER, PROPERTIES, DEPLOYMENT_ID)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
	}

	// queryGetResourceServerByID retrieves a resource server by ID.
	queryGetResourceServerByID = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-02",
		Query: `SELECT ID, OU_ID, NAME, DESCRIPTION, HANDLE, IDENTIFIER, PROPERTIES
			FROM "RESOURCE_SERVER"
			WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetResourceServerList retrieves a list of resource servers with pagination.
	queryGetResourceServerList = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-03",
		Query: `SELECT ID, OU_ID, NAME, DESCRIPTION, HANDLE, IDENTIFIER, PROPERTIES
			FROM "RESOURCE_SERVER"
			WHERE DEPLOYMENT_ID = $3
			ORDER BY CREATED_AT DESC
			LIMIT $1 OFFSET $2`,
	}

	// queryGetResourceServerListCount retrieves the total count of resource servers.
	queryGetResourceServerListCount = dbmodel.DBQuery{
		ID:    "RSQ-RES_MGT-04",
		Query: `SELECT COUNT(*) as total FROM "RESOURCE_SERVER" WHERE DEPLOYMENT_ID = $1`,
	}

	// queryUpdateResourceServer updates a resource server.
	queryUpdateResourceServer = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-05",
		Query: `UPDATE "RESOURCE_SERVER"
			SET OU_ID = $1, NAME = $2, DESCRIPTION = $3, HANDLE = $4, IDENTIFIER = $5, PROPERTIES = $6
			WHERE ID = $7 AND DEPLOYMENT_ID = $8`,
	}

	// queryDeleteResourceServer deletes a resource server.
	queryDeleteResourceServer = dbmodel.DBQuery{
		ID:    "RSQ-RES_MGT-06",
		Query: `DELETE FROM "RESOURCE_SERVER" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCheckResourceServerNameExists checks if a resource server name already exists.
	queryCheckResourceServerNameExists = dbmodel.DBQuery{
		ID:    "RSQ-RES_MGT-07",
		Query: `SELECT COUNT(*) as count FROM "RESOURCE_SERVER" WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCheckResourceServerHandleExists checks if a resource server handler already exists.
	queryCheckResourceServerHandleExists = dbmodel.DBQuery{
		ID:    "RSQ-RES_MGT-08",
		Query: `SELECT COUNT(*) as count FROM "RESOURCE_SERVER" WHERE HANDLE = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCheckResourceServerIdentifierExists checks if a resource server identifier already exists.
	queryCheckResourceServerIdentifierExists = dbmodel.DBQuery{
		ID:    "RSQ-RES_MGT-33",
		Query: `SELECT COUNT(*) as count FROM "RESOURCE_SERVER" WHERE IDENTIFIER = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryGetResourceServerByIdentifier retrieves a resource server by identifier.
	queryGetResourceServerByIdentifier = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-34",
		Query: `SELECT ID, OU_ID, NAME, DESCRIPTION, HANDLE, IDENTIFIER, PROPERTIES
			FROM "RESOURCE_SERVER"
			WHERE IDENTIFIER = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCheckResourceServerHasDependencies checks if resource server has resources or actions.
	queryCheckResourceServerHasDependencies = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-09",
		Query: `SELECT COUNT(*) as count FROM (
			SELECT 1 FROM "RESOURCE" r
				WHERE r.RESOURCE_SERVER_ID = $1 AND r.DEPLOYMENT_ID = $2
			UNION ALL
			SELECT 1 FROM "ACTION" a
				WHERE a.RESOURCE_SERVER_ID = $1 AND a.RESOURCE_ID IS NULL AND a.DEPLOYMENT_ID = $2
			LIMIT 1
		) as dependencies`,
	}
)

// Resource Queries
var (
	// queryCreateResource creates a new resource.
	queryCreateResource = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-10",
		Query: `INSERT INTO "RESOURCE"
		        (ID, RESOURCE_SERVER_ID, NAME, HANDLE, DESCRIPTION, PERMISSION, PROPERTIES,
				PARENT_RESOURCE_ID, DEPLOYMENT_ID)
		        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
	}

	// queryGetResourceByID retrieves a resource by ID.
	queryGetResourceByID = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-11",
		Query: `SELECT r.ID, r.NAME, r.HANDLE, r.DESCRIPTION, r.PERMISSION,
				r.PROPERTIES, pr.ID as PARENT_RESOURCE_ID
			FROM "RESOURCE" r
			LEFT JOIN "RESOURCE" pr ON r.PARENT_RESOURCE_ID = pr.ID
			WHERE r.ID = $1 AND r.RESOURCE_SERVER_ID = $2 AND r.DEPLOYMENT_ID = $3`,
	}

	// queryGetResourceList retrieves a list of resources with pagination.
	queryGetResourceList = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-12",
		Query: `SELECT r.ID, r.NAME, r.HANDLE, r.DESCRIPTION, r.PERMISSION,
				r.PROPERTIES, pr.ID as PARENT_RESOURCE_ID
			FROM "RESOURCE" r
			LEFT JOIN "RESOURCE" pr ON r.PARENT_RESOURCE_ID = pr.ID
			WHERE r.RESOURCE_SERVER_ID = $1 AND r.DEPLOYMENT_ID = $4
			ORDER BY r.CREATED_AT DESC LIMIT $2 OFFSET $3`,
	}

	// queryGetResourceListByParent retrieves resources by parent ID with pagination.
	queryGetResourceListByParent = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-13",
		Query: `SELECT r.ID, r.NAME, r.HANDLE, r.DESCRIPTION, r.PERMISSION,
				r.PROPERTIES, pr.ID as PARENT_RESOURCE_ID
			FROM "RESOURCE" r
			LEFT JOIN "RESOURCE" pr ON r.PARENT_RESOURCE_ID = pr.ID
			WHERE r.RESOURCE_SERVER_ID = $1 AND r.PARENT_RESOURCE_ID = $2 AND r.DEPLOYMENT_ID = $5
			ORDER BY r.CREATED_AT DESC LIMIT $3 OFFSET $4`,
	}

	// queryGetResourceListByNullParent retrieves top-level resources with pagination.
	queryGetResourceListByNullParent = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-14",
		Query: `SELECT r.ID, r.NAME, r.HANDLE, r.DESCRIPTION, r.PERMISSION,
				r.PROPERTIES, pr.ID as PARENT_RESOURCE_ID
			FROM "RESOURCE" r
		        LEFT JOIN "RESOURCE" pr ON r.PARENT_RESOURCE_ID = pr.ID
		        WHERE r.RESOURCE_SERVER_ID = $1 AND r.PARENT_RESOURCE_ID IS NULL AND r.DEPLOYMENT_ID = $4
		        ORDER BY r.CREATED_AT DESC LIMIT $2 OFFSET $3`,
	}

	// queryGetResourceListCount retrieves the total count of resources.
	queryGetResourceListCount = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-15",
		Query: `SELECT COUNT(*) as total
		        FROM "RESOURCE" r
		        WHERE r.RESOURCE_SERVER_ID = $1 AND r.DEPLOYMENT_ID = $2`,
	}

	// queryGetResourceListCountByParent retrieves count of resources by parent.
	queryGetResourceListCountByParent = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-16",
		Query: `SELECT COUNT(*) as total
		        FROM "RESOURCE" r
		        WHERE r.RESOURCE_SERVER_ID = $1 AND r.PARENT_RESOURCE_ID = $2 AND r.DEPLOYMENT_ID = $3`,
	}

	// queryGetResourceListCountByNullParent retrieves count of top-level resources.
	queryGetResourceListCountByNullParent = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-17",
		Query: `SELECT COUNT(*) as total
		        FROM "RESOURCE" r
		        WHERE r.RESOURCE_SERVER_ID = $1 AND r.PARENT_RESOURCE_ID IS NULL AND r.DEPLOYMENT_ID = $2`,
	}

	// queryUpdateResource updates a resource.
	queryUpdateResource = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-18",
		Query: `UPDATE "RESOURCE"
		        SET NAME = $1,
				    DESCRIPTION = $2,
		            PROPERTIES = $3
		        WHERE ID = $4
		          AND RESOURCE_SERVER_ID = $5
		          AND DEPLOYMENT_ID = $6`,
	}

	// queryUpdateResourcePermission updates only the permission field of a resource.
	queryUpdateResourcePermission = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-36",
		Query: `UPDATE "RESOURCE"
		        SET PERMISSION = $1
		        WHERE ID = $2
		          AND RESOURCE_SERVER_ID = $3
		          AND DEPLOYMENT_ID = $4`,
	}

	// queryDeleteResource deletes a resource.
	queryDeleteResource = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-19",
		Query: `DELETE FROM "RESOURCE"
		        WHERE ID = $1
		          AND RESOURCE_SERVER_ID = $2
		          AND DEPLOYMENT_ID = $3`,
	}

	// queryCheckResourceHandleExistsUnderParent checks if resource handle exists under same parent.
	queryCheckResourceHandleExistsUnderParent = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-20",
		Query: `SELECT COUNT(*) as count
		        FROM "RESOURCE" r WHERE r.RESOURCE_SERVER_ID = $1 AND r.HANDLE = $2
				AND r.PARENT_RESOURCE_ID = $3 AND r.DEPLOYMENT_ID = $4`,
	}

	// queryCheckResourceHandleExistsUnderNullParent checks if resource handle exists at top level.
	queryCheckResourceHandleExistsUnderNullParent = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-21",
		Query: `SELECT COUNT(*) as count
		        FROM "RESOURCE" r WHERE r.RESOURCE_SERVER_ID = $1 AND r.HANDLE = $2
				AND r.PARENT_RESOURCE_ID IS NULL AND r.DEPLOYMENT_ID = $3`,
	}

	// queryCheckResourceHasDependencies checks if resource has sub-resources or actions.
	queryCheckResourceHasDependencies = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-22",
		Query: `SELECT COUNT(*) as count FROM (
			SELECT 1 FROM "RESOURCE" child
				WHERE child.PARENT_RESOURCE_ID = $1 AND child.DEPLOYMENT_ID = $2
			UNION ALL
			SELECT 1 FROM "ACTION" a
				WHERE a.RESOURCE_ID = $1 AND a.DEPLOYMENT_ID = $2
			LIMIT 1
		) as dependencies`,
	}

	// queryCheckCircularDependency checks if setting a parent would create a circular dependency.
	// It traverses UP the parent chain from newParentID to check if resourceID appears as an ancestor.
	queryCheckCircularDependency = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-23",
		Query: `WITH RECURSIVE parent_hierarchy AS (
			SELECT ID, PARENT_RESOURCE_ID, DEPLOYMENT_ID FROM "RESOURCE"
			WHERE ID = $1 AND DEPLOYMENT_ID = $3
			UNION ALL
			SELECT r.ID, r.PARENT_RESOURCE_ID, r.DEPLOYMENT_ID
			FROM "RESOURCE" r
			INNER JOIN parent_hierarchy ph ON ph.PARENT_RESOURCE_ID = r.ID
				AND ph.DEPLOYMENT_ID = r.DEPLOYMENT_ID
		)
		SELECT COUNT(*) as count FROM parent_hierarchy WHERE ID = $2`,
	}
)

// Action Queries
var (
	// queryCreateAction creates a new action.
	queryCreateAction = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-24",
		Query: `INSERT INTO "ACTION"
		        (ID, RESOURCE_SERVER_ID, RESOURCE_ID, NAME, HANDLE, DESCRIPTION, PERMISSION,
				PROPERTIES, DEPLOYMENT_ID)
		        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
	}

	// queryGetActionByID retrieves an action by ID.
	queryGetActionByID = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-25",
		Query: `SELECT a.ID, a.NAME, a.HANDLE, a.DESCRIPTION, a.PERMISSION, a.PROPERTIES
		        FROM "ACTION" a
		        WHERE a.ID = $1
		          AND a.RESOURCE_SERVER_ID = $2
		          AND (a.RESOURCE_ID = $3 OR (a.RESOURCE_ID IS NULL AND $3 IS NULL))
		          AND a.DEPLOYMENT_ID = $4`,
	}

	// queryGetActionList retrieves actions with pagination.
	queryGetActionList = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-26",
		Query: `SELECT a.ID, a.NAME, a.HANDLE, a.DESCRIPTION, a.PERMISSION, a.PROPERTIES
		        FROM "ACTION" a
		        WHERE a.RESOURCE_SERVER_ID = $1
		          AND (a.RESOURCE_ID = $2 OR (a.RESOURCE_ID IS NULL AND $2 IS NULL))
		          AND a.DEPLOYMENT_ID = $5
		        ORDER BY a.CREATED_AT DESC LIMIT $3 OFFSET $4`,
	}

	// queryGetActionListCount retrieves count of actions.
	queryGetActionListCount = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-27",
		Query: `SELECT COUNT(*) as total
		        FROM "ACTION" a
		        WHERE a.RESOURCE_SERVER_ID = $1
		          AND (a.RESOURCE_ID = $2 OR (a.RESOURCE_ID IS NULL AND $2 IS NULL))
		          AND a.DEPLOYMENT_ID = $3`,
	}

	// queryUpdateAction updates an action.
	queryUpdateAction = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-28",
		Query: `UPDATE "ACTION"
		        SET NAME = $1, DESCRIPTION = $2, PROPERTIES = $3
		        WHERE ID = $4
		          AND RESOURCE_SERVER_ID = $5
		          AND (RESOURCE_ID = $6 OR (RESOURCE_ID IS NULL AND $6 IS NULL))
		          AND DEPLOYMENT_ID = $7`,
	}

	// queryUpdateActionPermission updates only the permission field of an action.
	queryUpdateActionPermission = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-37",
		Query: `UPDATE "ACTION"
		        SET PERMISSION = $1
		        WHERE ID = $2
		          AND RESOURCE_SERVER_ID = $3
		          AND (RESOURCE_ID = $4 OR (RESOURCE_ID IS NULL AND $4 IS NULL))
		          AND DEPLOYMENT_ID = $5`,
	}

	// queryDeleteAction deletes an action.
	queryDeleteAction = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-29",
		Query: `DELETE FROM "ACTION"
		        WHERE ID = $1
		          AND RESOURCE_SERVER_ID = $2
		          AND (RESOURCE_ID = $3 OR (RESOURCE_ID IS NULL AND $3 IS NULL))
		          AND DEPLOYMENT_ID = $4`,
	}

	// queryCheckActionExists checks if an action exists by ID.
	queryCheckActionExists = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-30",
		Query: `SELECT COUNT(*) as count
		        FROM "ACTION" a
		        WHERE a.ID = $1
		          AND a.RESOURCE_SERVER_ID = $2
		          AND (a.RESOURCE_ID = $3 OR (a.RESOURCE_ID IS NULL AND $3 IS NULL))
		          AND a.DEPLOYMENT_ID = $4`,
	}

	// queryCheckActionHandleExists checks if action handle exists.
	queryCheckActionHandleExists = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-31",
		Query: `SELECT COUNT(*) as count
		        FROM "ACTION" a
		        WHERE a.RESOURCE_SERVER_ID = $1
		          AND (a.RESOURCE_ID = $2 OR (a.RESOURCE_ID IS NULL AND $2 IS NULL))
		          AND a.HANDLE = $3
		          AND a.DEPLOYMENT_ID = $4`,
	}

	// queryFindResourceServersByPermissions returns distinct resource servers that define at least
	// one of the supplied permissions (as a RESOURCE.PERMISSION or ACTION.PERMISSION). Results are
	// ordered by IDENTIFIER for deterministic output. Parameter $2 must be a JSON array string.
	queryFindResourceServersByPermissions = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-35",
		PostgresQuery: `SELECT DISTINCT rs.ID, rs.OU_ID, rs.NAME, rs.DESCRIPTION, rs.HANDLE,
		               rs.IDENTIFIER, rs.PROPERTIES
		        FROM "RESOURCE_SERVER" rs
		        WHERE rs.DEPLOYMENT_ID = $1
		          AND rs.IDENTIFIER IS NOT NULL
		          AND (
		              EXISTS (
		                  SELECT 1 FROM "RESOURCE" r
		                  JOIN json_array_elements_text($2::json) AS p ON r.PERMISSION = p.value::text
		                  WHERE r.RESOURCE_SERVER_ID = rs.ID AND r.DEPLOYMENT_ID = $1
		              )
		              OR EXISTS (
		                  SELECT 1 FROM "ACTION" a
		                  JOIN json_array_elements_text($2::json) AS p ON a.PERMISSION = p.value::text
		                  WHERE a.RESOURCE_SERVER_ID = rs.ID AND a.DEPLOYMENT_ID = $1
		              )
		          )
		        ORDER BY rs.IDENTIFIER`,
		SQLiteQuery: `SELECT DISTINCT rs.ID, rs.OU_ID, rs.NAME, rs.DESCRIPTION, rs.HANDLE,
		              rs.IDENTIFIER, rs.PROPERTIES
		        FROM "RESOURCE_SERVER" rs
		        WHERE rs.DEPLOYMENT_ID = $1
		          AND rs.IDENTIFIER IS NOT NULL
		          AND (
		              EXISTS (
		                  SELECT 1 FROM "RESOURCE" r
		                  JOIN json_each($2) AS p ON r.PERMISSION = p.value
		                  WHERE r.RESOURCE_SERVER_ID = rs.ID AND r.DEPLOYMENT_ID = $1
		              )
		              OR EXISTS (
		                  SELECT 1 FROM "ACTION" a
		                  JOIN json_each($2) AS p ON a.PERMISSION = p.value
		                  WHERE a.RESOURCE_SERVER_ID = rs.ID AND a.DEPLOYMENT_ID = $1
		              )
		          )
		        ORDER BY rs.IDENTIFIER`,
	}

	// queryValidatePermissions validates if permissions exist for a resource server.
	// Returns only the INVALID permissions (those not found in the database).
	// Parameter $3 must be a JSON array string (e.g., ["read", "write", "admin"]).
	queryValidatePermissions = dbmodel.DBQuery{
		ID: "RSQ-RES_MGT-32",
		// PostgreSQL-optimized version using json_array_elements_text()
		PostgresQuery: `SELECT p.value::text AS permission
		        FROM json_array_elements_text($3::json) AS p
		        WHERE NOT EXISTS (
		            SELECT 1
		            FROM "RESOURCE" r
		            WHERE r.RESOURCE_SERVER_ID = $1
		              AND r.DEPLOYMENT_ID = $2
		              AND r.PERMISSION = p.value::text
		        )
		        AND NOT EXISTS (
		            SELECT 1
		            FROM "ACTION" a
		            WHERE a.RESOURCE_SERVER_ID = $1
		              AND a.DEPLOYMENT_ID = $2
		              AND a.PERMISSION = p.value::text
		        )`,
		// SQLite-optimized version using json_each()
		SQLiteQuery: `SELECT p.value AS permission
		        FROM json_each($3) AS p
		        WHERE NOT EXISTS (
		            SELECT 1
		            FROM "RESOURCE" r
		            WHERE r.RESOURCE_SERVER_ID = $1
		              AND r.DEPLOYMENT_ID = $2
		              AND r.PERMISSION = p.value
		        )
		        AND NOT EXISTS (
		            SELECT 1
		            FROM "ACTION" a
		            WHERE a.RESOURCE_SERVER_ID = $1
		              AND a.DEPLOYMENT_ID = $2
		              AND a.PERMISSION = p.value
		        )`,
	}
)
