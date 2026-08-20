/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package managedresource

import (
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// queryMarkManagedResource records a resource as owned by the control plane. It is written on every
	// import of the same resource, so it upserts rather than failing on the second write.
	queryMarkManagedResource = dbmodel.DBQuery{
		ID: "MRQ-MANAGED-001",
		Query: `INSERT INTO "MANAGED_RESOURCE" (DEPLOYMENT_ID, RESOURCE_TYPE, RESOURCE_ID) ` +
			`VALUES ($1, $2, $3) ON CONFLICT (DEPLOYMENT_ID, RESOURCE_TYPE, RESOURCE_ID) DO NOTHING`,
	}

	// queryUnmarkManagedResource drops the record when the resource is removed.
	queryUnmarkManagedResource = dbmodel.DBQuery{
		ID: "MRQ-MANAGED-002",
		Query: `DELETE FROM "MANAGED_RESOURCE" ` +
			`WHERE DEPLOYMENT_ID = $1 AND RESOURCE_TYPE = $2 AND RESOURCE_ID = $3`,
	}

	// queryIsManagedResource reports whether a resource is owned by the control plane.
	queryIsManagedResource = dbmodel.DBQuery{
		ID: "MRQ-MANAGED-003",
		Query: `SELECT COUNT(*) AS total FROM "MANAGED_RESOURCE" ` +
			`WHERE DEPLOYMENT_ID = $1 AND RESOURCE_TYPE = $2 AND RESOURCE_ID = $3`,
	}

	// queryListManagedResourceIDs returns the managed ids of one resource type, so a list endpoint can
	// mark its results without a query per row.
	queryListManagedResourceIDs = dbmodel.DBQuery{
		ID: "MRQ-MANAGED-004",
		Query: `SELECT RESOURCE_ID FROM "MANAGED_RESOURCE" ` +
			`WHERE DEPLOYMENT_ID = $1 AND RESOURCE_TYPE = $2`,
	}
)
