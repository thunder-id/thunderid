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

package tenant

import (
	"fmt"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
)

var (
	// queryCreateTenant records a managed tenant in the platform registry (owned by the system tenant).
	queryCreateTenant = dbmodel.DBQuery{
		ID:    "TNQ-TENANT-001",
		Query: `INSERT INTO "TENANT" (ID, TENANT_ID, NAME, DEPLOYMENT_ID) VALUES ($1, $2, $3, $4)`,
	}

	// queryGetTenantByDeploymentID retrieves a registry row by managed deployment id.
	queryGetTenantByDeploymentID = dbmodel.DBQuery{
		ID: "TNQ-TENANT-002",
		Query: `SELECT ID, TENANT_ID, NAME, CREATED_AT, UPDATED_AT FROM "TENANT" ` +
			`WHERE TENANT_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryListTenants lists all managed tenants for the system tenant.
	queryListTenants = dbmodel.DBQuery{
		ID: "TNQ-TENANT-003",
		Query: `SELECT ID, TENANT_ID, NAME, CREATED_AT, UPDATED_AT FROM "TENANT" ` +
			`WHERE DEPLOYMENT_ID = $1 ORDER BY TENANT_ID`,
	}

	// queryDeleteTenantRecord removes a managed tenant's registry row.
	queryDeleteTenantRecord = dbmodel.DBQuery{
		ID:    "TNQ-TENANT-004",
		Query: `DELETE FROM "TENANT" WHERE TENANT_ID = $1 AND DEPLOYMENT_ID = $2`,
	}

	// queryCountInboundClientsForTenant counts inbound clients scoped to a deployment id, used to
	// detect whether a tenant has already been provisioned.
	queryCountInboundClientsForTenant = dbmodel.DBQuery{
		ID:    "TNQ-TENANT-005",
		Query: `SELECT COUNT(*) AS total FROM "INBOUND_CLIENT" WHERE DEPLOYMENT_ID = $1`,
	}

	// queryNullResourceParent clears the RESOURCE self-reference before the RESOURCE rows are deleted
	// so the self-referential foreign key does not block the purge.
	queryNullResourceParent = dbmodel.DBQuery{
		ID:    "TNQ-TENANT-006",
		Query: `UPDATE "RESOURCE" SET PARENT_RESOURCE_ID = NULL WHERE DEPLOYMENT_ID = $1`,
	}
)

// buildDeleteByDeployment builds a DELETE-by-deployment-id query for the given table. Table names are
// from a fixed internal allow-list (never user input), so interpolating them is safe.
func buildDeleteByDeployment(table string) dbmodel.DBQuery {
	return dbmodel.DBQuery{
		ID:    "TNQ-PURGE-" + table,
		Query: fmt.Sprintf(`DELETE FROM %q WHERE DEPLOYMENT_ID = $1`, table),
	}
}

// Ordered lists of tables to purge per database when deprovisioning a tenant. Children are deleted
// before parents so restrictive foreign keys never block; cascading foreign keys make some explicit
// deletes redundant but harmless. Every listed table carries a DEPLOYMENT_ID column.
var (
	// configPurgeTables: RESOURCE's self-reference is nulled (queryNullResourceParent) before this
	// runs; ACTION -> RESOURCE -> RESOURCE_SERVER order satisfies the RESTRICT foreign keys.
	configPurgeTables = []string{
		"ACTION",
		"RESOURCE",
		"RESOURCE_SERVER",
		"ROLE_ASSIGNMENT",
		"ROLE_PERMISSION",
		"ROLE",
		"OAUTH_INBOUND_PROFILE",
		"INBOUND_CLIENT",
		"FLOW_VERSION",
		"FLOW",
		"THEME",
		"LAYOUT",
		"IDP",
		"NOTIFICATION_SENDER",
		"CERTIFICATE",
		"TRANSLATION",
		"PRESENTATION_DEFINITION",
		"CREDENTIAL_CONFIGURATION",
		"SERVER_CONFIG",
		"ENTITY_TYPES",
		"SECRETS",
	}
	entityPurgeTables = []string{
		"GROUP_MEMBER_REFERENCE",
		"ENTITY_IDENTIFIER",
		"ENTITY",
		"GROUP",
		"ORGANIZATION_UNIT",
	}
	runtimePersistentPurgeTables = []string{
		"SSO_SESSION_PARTICIPANT",
		"SSO_SESSION_CONTEXT",
		"SSO_SESSION",
		"CONSENT_AUDIT",
		"CONSENT_AUTHORIZATION",
		"CONSENT",
		"REVOKED_TOKEN",
	}
	runtimeTransientPurgeTables = []string{
		"AUTHORIZATION_CODE",
		"AUTHORIZATION_REQUEST",
		"CIBA_AUTH_REQUEST",
		"WEBAUTHN_SESSION",
		"PAR_REQUEST",
		"JTI_RECORD",
		"RUNTIME_STORE",
	}
)
