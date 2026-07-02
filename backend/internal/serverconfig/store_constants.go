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

package serverconfig

import (
	"fmt"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

// serverConfigVersionModulus bounds the rotating VERSION token below the 32-bit INTEGER max to avoid
// overflow; VERSION is compared only for equality, not ordering.
const serverConfigVersionModulus = 2000000000

var (
	// queryGetServerConfigByName retrieves a single server config by name.
	queryGetServerConfigByName = dbmodel.DBQuery{
		ID: "SCF-01",
		Query: `SELECT NAME, VALUE, VERSION FROM "SERVER_CONFIG" ` +
			`WHERE NAME = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryUpsertServerConfig inserts or updates a server config by name, rotating VERSION on update.
	queryUpsertServerConfig = dbmodel.DBQuery{
		ID: "SCF-02",
		Query: fmt.Sprintf(`INSERT INTO "SERVER_CONFIG" (NAME, VALUE, DEPLOYMENT_ID) `+
			`VALUES ($1, $2, $3) `+
			`ON CONFLICT (DEPLOYMENT_ID, NAME) `+
			`DO UPDATE SET VALUE = EXCLUDED.VALUE, `+
			`VERSION = ("SERVER_CONFIG".VERSION %% %d) + 1, UPDATED_AT = NOW()`, serverConfigVersionModulus),
		SQLiteQuery: fmt.Sprintf(`INSERT INTO "SERVER_CONFIG" (NAME, VALUE, DEPLOYMENT_ID) `+
			`VALUES ($1, $2, $3) `+
			`ON CONFLICT (DEPLOYMENT_ID, NAME) `+
			`DO UPDATE SET VALUE = excluded.VALUE, `+
			`VERSION = (VERSION %% %d) + 1, UPDATED_AT = datetime('now')`, serverConfigVersionModulus),
	}
)
