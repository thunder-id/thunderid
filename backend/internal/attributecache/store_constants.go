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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package attributecache

import dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

var (
	// queryInsertAttributeCache inserts a new attribute cache entry.
	queryInsertAttributeCache = dbmodel.DBQuery{
		ID: "ACS-01",
		Query: `INSERT INTO "ATTRIBUTE_CACHE" (ID, ATTRIBUTES, EXPIRY_TIME, CREATED_AT, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5)`,
	}

	// queryGetAttributeCache retrieves an attribute cache entry by ID.
	queryGetAttributeCache = dbmodel.DBQuery{
		ID: "ACS-02",
		Query: `SELECT ID, ATTRIBUTES, EXPIRY_TIME FROM "ATTRIBUTE_CACHE" ` +
			`WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryUpdateAttributeCacheExpiry updates the expiry time of an attribute cache entry.
	queryUpdateAttributeCacheExpiry = dbmodel.DBQuery{
		ID: "ACS-03",
		Query: `UPDATE "ATTRIBUTE_CACHE" SET EXPIRY_TIME = $2 ` +
			`WHERE ID = $1 AND DEPLOYMENT_ID = $3`,
	}

	// queryDeleteAttributeCache deletes an attribute cache entry by ID.
	queryDeleteAttributeCache = dbmodel.DBQuery{
		ID:    "ACS-04",
		Query: `DELETE FROM "ATTRIBUTE_CACHE" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)
