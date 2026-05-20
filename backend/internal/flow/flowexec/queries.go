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

package flowexec

import (
	"github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// QueryCreateFlowContext is the query to create a new flow context.
	QueryCreateFlowContext = model.DBQuery{
		ID:    "FLQ-FLOW_CTX-01",
		Query: `INSERT INTO "FLOW_CONTEXT" (FLOW_ID, DEPLOYMENT_ID, CONTEXT, EXPIRY_TIME) VALUES ($1, $2, $3, $4)`,
	}

	// QueryGetFlowContext is the query to get a flow context by ID.
	QueryGetFlowContext = model.DBQuery{
		ID: "FLQ-FLOW_CTX-02",
		Query: `SELECT FLOW_ID, CONTEXT, EXPIRY_TIME, CREATED_AT, UPDATED_AT FROM "FLOW_CONTEXT" ` +
			`WHERE FLOW_ID = $1 AND DEPLOYMENT_ID = $2 AND EXPIRY_TIME > $3`,
	}

	// QueryUpdateFlowContext is the query to update a flow context.
	QueryUpdateFlowContext = model.DBQuery{
		ID: "FLQ-FLOW_CTX-03",
		Query: `UPDATE "FLOW_CONTEXT" SET CONTEXT = $2, UPDATED_AT = CURRENT_TIMESTAMP ` +
			`WHERE FLOW_ID = $1 AND DEPLOYMENT_ID = $3`,
	}

	// QueryDeleteFlowContext is the query to delete a flow context.
	QueryDeleteFlowContext = model.DBQuery{
		ID:    "FLQ-FLOW_CTX-04",
		Query: `DELETE FROM "FLOW_CONTEXT" WHERE FLOW_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)
