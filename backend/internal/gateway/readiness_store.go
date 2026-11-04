// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"
	"time"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

var (
	// queryGetGatewayStatus reads what a gateway's last health check found.
	queryGetGatewayStatus = dbmodel.DBQuery{
		ID: "GTS_MGT-01",
		Query: `SELECT REACHABLE, CHECKED_AT, PROVISIONED_AT FROM "GATEWAY_STATUS" ` +
			`WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryRecordGatewayStatus records a health check. PROVISIONED_AT is set by the first check the
	// gateway answers and kept by every check after it, in the statement, so two nodes recording at
	// once cannot clear it.
	queryRecordGatewayStatus = dbmodel.DBQuery{
		ID: "GTS_MGT-02",
		Query: `INSERT INTO "GATEWAY_STATUS" (GATEWAY_ID, REACHABLE, CHECKED_AT, PROVISIONED_AT, DEPLOYMENT_ID) ` +
			`VALUES ($1, $2, NOW(), CASE WHEN $2 THEN NOW() END, $3) ` +
			`ON CONFLICT (DEPLOYMENT_ID, GATEWAY_ID) DO UPDATE SET REACHABLE = excluded.REACHABLE, ` +
			`CHECKED_AT = excluded.CHECKED_AT, ` +
			`PROVISIONED_AT = COALESCE("GATEWAY_STATUS".PROVISIONED_AT, excluded.PROVISIONED_AT)`,
		SQLiteQuery: `INSERT INTO "GATEWAY_STATUS" (GATEWAY_ID, REACHABLE, CHECKED_AT, PROVISIONED_AT, ` +
			`DEPLOYMENT_ID) VALUES ($1, $2, datetime('now'), CASE WHEN $2 THEN datetime('now') END, $3) ` +
			`ON CONFLICT (DEPLOYMENT_ID, GATEWAY_ID) DO UPDATE SET REACHABLE = excluded.REACHABLE, ` +
			`CHECKED_AT = excluded.CHECKED_AT, ` +
			`PROVISIONED_AT = COALESCE("GATEWAY_STATUS".PROVISIONED_AT, excluded.PROVISIONED_AT)`,
	}
	// queryDeleteGatewayStatus forgets a removed gateway's status.
	queryDeleteGatewayStatus = dbmodel.DBQuery{
		ID:    "GTS_MGT-03",
		Query: `DELETE FROM "GATEWAY_STATUS" WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)

// checkedStatus is what a gateway's last health check found. A gateway never checked has none.
type checkedStatus struct {
	Reachable     bool
	CheckedAt     time.Time
	ProvisionedAt *time.Time
}

// statusStoreInterface keeps what each gateway's health checks found.
type statusStoreInterface interface {
	// Get reads a gateway's last check, or nil when it has never been checked.
	Get(ctx context.Context, gatewayID string) (*checkedStatus, error)
	// Record keeps the outcome of a check.
	Record(ctx context.Context, gatewayID string, reachable bool) error
	// Delete forgets a removed gateway's checks.
	Delete(ctx context.Context, gatewayID string) error
}

type statusStore struct {
	dbProvider dbprovider.DBProviderInterface
}

func newStatusStore() statusStoreInterface {
	return &statusStore{dbProvider: dbprovider.GetDBProvider()}
}

func (s *statusStore) Get(ctx context.Context, gatewayID string) (*checkedStatus, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	rows, err := dbClient.QueryContext(ctx, queryGetGatewayStatus, gatewayID, deployment.Resolve(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to read the gateway's status: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	status := &checkedStatus{Reachable: asBool(rows[0]["reachable"]), CheckedAt: asTime(rows[0]["checked_at"])}
	if provisioned := asTime(rows[0]["provisioned_at"]); !provisioned.IsZero() {
		status.ProvisionedAt = &provisioned
	}
	return status, nil
}

func (s *statusStore) Record(ctx context.Context, gatewayID string, reachable bool) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryRecordGatewayStatus, gatewayID, reachable,
		deployment.Resolve(ctx)); err != nil {
		return fmt.Errorf("failed to record the gateway's status: %w", err)
	}
	return nil
}

func (s *statusStore) Delete(ctx context.Context, gatewayID string) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	if _, err := dbClient.ExecuteContext(ctx, queryDeleteGatewayStatus, gatewayID,
		deployment.Resolve(ctx)); err != nil {
		return fmt.Errorf("failed to forget the gateway's status: %w", err)
	}
	return nil
}
