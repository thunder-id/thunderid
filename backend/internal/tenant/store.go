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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// tenantStoreInterface defines persistence operations for the tenant registry and tenant data purge.
type tenantStoreInterface interface {
	CreateTenant(ctx context.Context, t Tenant) error
	GetTenant(ctx context.Context, deploymentID string) (Tenant, error)
	ListTenants(ctx context.Context) ([]Tenant, error)
	DeleteTenantRecord(ctx context.Context, deploymentID string) error
	IsProvisioned(ctx context.Context, deploymentID string) (bool, error)
	PurgeTenantData(ctx context.Context, deploymentID string) error
}

// tenantStore is the default implementation of tenantStoreInterface. The registry rows it manages are
// owned by (scoped to) the system tenant; the purge operates on an arbitrary target deployment id.
type tenantStore struct {
	dbProvider         provider.DBProviderInterface
	systemDeploymentID string
}

func newTenantStore(systemDeploymentID string) tenantStoreInterface {
	return &tenantStore{dbProvider: provider.GetDBProvider(), systemDeploymentID: systemDeploymentID}
}

// CreateTenant records a managed tenant in the registry.
func (s *tenantStore) CreateTenant(ctx context.Context, t Tenant) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	_, err = dbClient.QueryContext(ctx, queryCreateTenant, t.ID, t.DeploymentID, t.Name, s.systemDeploymentID)
	if err != nil {
		return fmt.Errorf("failed to create tenant record: %w", err)
	}
	return nil
}

// GetTenant retrieves a managed tenant's registry row by deployment id.
func (s *tenantStore) GetTenant(ctx context.Context, deploymentID string) (Tenant, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return Tenant{}, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, queryGetTenantByDeploymentID, deploymentID, s.systemDeploymentID)
	if err != nil {
		return Tenant{}, fmt.Errorf("failed to execute query: %w", err)
	}
	if len(results) == 0 {
		return Tenant{}, errTenantNotFound
	}
	return parseTenantRow(results[0]), nil
}

// ListTenants returns all managed tenants.
func (s *tenantStore) ListTenants(ctx context.Context) ([]Tenant, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, queryListTenants, s.systemDeploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	tenants := make([]Tenant, 0, len(results))
	for _, row := range results {
		tenants = append(tenants, parseTenantRow(row))
	}
	return tenants, nil
}

// DeleteTenantRecord removes a managed tenant's registry row (no-op if absent).
func (s *tenantStore) DeleteTenantRecord(ctx context.Context, deploymentID string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteTenantRecord, deploymentID, s.systemDeploymentID); err != nil {
		return fmt.Errorf("failed to delete tenant record: %w", err)
	}
	return nil
}

// IsProvisioned reports whether a deployment id already has baseline data (any inbound clients).
func (s *tenantStore) IsProvisioned(ctx context.Context, deploymentID string) (bool, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return false, fmt.Errorf("failed to get database client: %w", err)
	}
	results, err := dbClient.QueryContext(ctx, queryCountInboundClientsForTenant, deploymentID)
	if err != nil {
		return false, fmt.Errorf("failed to execute count query: %w", err)
	}
	if len(results) == 0 {
		return false, nil
	}
	count, ok := results[0]["total"].(int64)
	if !ok {
		return false, fmt.Errorf("failed to parse count result")
	}
	return count > 0, nil
}

// PurgeTenantData deletes all rows for a deployment id across the config, entity, and runtime
// databases. Idempotent: re-running removes any rows left by a partial previous run.
func (s *tenantStore) PurgeTenantData(ctx context.Context, deploymentID string) error {
	configClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get config database client: %w", err)
	}
	// Clear the RESOURCE self-reference before deleting RESOURCE rows.
	if _, err := configClient.ExecuteContext(ctx, queryNullResourceParent, deploymentID); err != nil {
		return fmt.Errorf("failed to clear resource parents: %w", err)
	}
	if err := purgeTables(ctx, configClient, configPurgeTables, deploymentID); err != nil {
		return err
	}

	entityClient, err := s.dbProvider.GetEntityDBClient()
	if err != nil {
		return fmt.Errorf("failed to get entity database client: %w", err)
	}
	if err := purgeTables(ctx, entityClient, entityPurgeTables, deploymentID); err != nil {
		return err
	}

	persistentClient, err := s.dbProvider.GetRuntimePersistentDBClient()
	if err != nil {
		return fmt.Errorf("failed to get runtime-persistent database client: %w", err)
	}
	if err := purgeTables(ctx, persistentClient, runtimePersistentPurgeTables, deploymentID); err != nil {
		return err
	}

	transientClient, err := s.dbProvider.GetRuntimeTransientDBClient()
	if err != nil {
		return fmt.Errorf("failed to get runtime-transient database client: %w", err)
	}
	return purgeTables(ctx, transientClient, runtimeTransientPurgeTables, deploymentID)
}

// purgeTables issues DELETE ... WHERE DEPLOYMENT_ID = ? for each table in order. Tables that do not
// exist on this database (e.g. a table added by a later migration that has not been applied) are
// skipped so a deprovision on a partially-migrated database still removes everything present.
func purgeTables(ctx context.Context, client provider.DBClientInterface, tables []string, deploymentID string) error {
	for _, table := range tables {
		if _, err := client.ExecuteContext(ctx, buildDeleteByDeployment(table), deploymentID); err != nil {
			if isMissingTableErr(err) {
				continue
			}
			return fmt.Errorf("failed to purge table %q: %w", table, err)
		}
	}
	return nil
}

// isMissingTableErr reports whether err indicates the target table (or its DEPLOYMENT_ID column) is
// absent, across SQLite and PostgreSQL.
func isMissingTableErr(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "undefined table")
}

func parseTenantRow(row map[string]interface{}) Tenant {
	return Tenant{
		ID:           parseString(row["id"]),
		DeploymentID: parseString(row["tenant_id"]),
		Name:         parseString(row["name"]),
		CreatedAt:    parseTimeString(row["created_at"]),
		UpdatedAt:    parseTimeString(row["updated_at"]),
	}
}

func parseString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

func parseTimeString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case time.Time:
		return v.UTC().Format(time.RFC3339)
	default:
		return ""
	}
}
