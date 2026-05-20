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

package idp

import "github.com/thunder-id/thunderid/internal/system/database/model"

var (
	// queryCreateIdentityProvider is the query to create a new IdP.
	queryCreateIdentityProvider = model.DBQuery{
		ID: "IPQ-IDP_MGT-01",
		Query: `INSERT INTO "IDP" (ID, NAME, DESCRIPTION, TYPE, PROPERTIES, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, $3, $4, $5, $6)`,
	}
	// queryGetIdentityProviderByID is the query to get a IdP by IdP ID.
	queryGetIdentityProviderByID = model.DBQuery{
		ID:    "IPQ-IDP_MGT-02",
		Query: `SELECT ID, NAME, DESCRIPTION, TYPE, PROPERTIES FROM "IDP" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryGetIdentityProviderList is the query to get a list of IdPs.
	queryGetIdentityProviderList = model.DBQuery{
		ID:    "IPQ-IDP_MGT-03",
		Query: `SELECT ID, NAME, DESCRIPTION, TYPE, PROPERTIES FROM "IDP" WHERE DEPLOYMENT_ID = $1`,
	}
	// queryUpdateIdentityProviderByID is the query to update a IdP by IdP ID.
	queryUpdateIdentityProviderByID = model.DBQuery{
		ID: "IPQ-IDP_MGT-04",
		Query: `UPDATE "IDP" SET NAME = $2, DESCRIPTION = $3, TYPE = $4, PROPERTIES = $5 ` +
			`WHERE ID = $1 AND DEPLOYMENT_ID = $6`,
	}
	// queryDeleteIdentityProviderByID is the query to delete a IdP by IdP ID.
	queryDeleteIdentityProviderByID = model.DBQuery{
		ID:    "IPQ-IDP_MGT-05",
		Query: `DELETE FROM "IDP" WHERE ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryGetIdentityProviderByName is the query to get a IdP by IdP name.
	queryGetIdentityProviderByName = model.DBQuery{
		ID:    "IPQ-IDP_MGT-06",
		Query: `SELECT ID, NAME, DESCRIPTION, TYPE, PROPERTIES FROM "IDP" WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryGetIdentityProviderListCount is the query to get a count of IdPs.
	queryGetIdentityProviderListCount = model.DBQuery{
		ID:    "IPQ-IDP_MGT-07",
		Query: `SELECT COUNT(*) AS count FROM "IDP" WHERE DEPLOYMENT_ID = $1`,
	}
	// queryGetIdentityProviderByIssuer is the query to get an IDP by its issuer property value.
	queryGetIdentityProviderByIssuer = model.DBQuery{
		ID: "IPQ-IDP_MGT-08",
		PostgresQuery: "SELECT ID, NAME, DESCRIPTION, TYPE, PROPERTIES FROM IDP " +
			"WHERE PROPERTIES @> $1::jsonb AND DEPLOYMENT_ID = $2 LIMIT 1",
		SQLiteQuery: "SELECT ID, NAME, DESCRIPTION, TYPE, PROPERTIES FROM IDP " +
			"WHERE EXISTS (SELECT 1 FROM json_each(PROPERTIES) " +
			"WHERE json_extract(value, '$.name') = 'issuer' " +
			"AND json_extract(value, '$.value') = $1) " +
			"AND DEPLOYMENT_ID = $2 LIMIT 1",
	}
)
