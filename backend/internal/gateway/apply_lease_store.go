// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"
	"strconv"
	"time"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// applyLease is what a gateway's lease row holds: who applies to it now, and whether the latest
// version is to be applied again once that apply finishes.
type applyLease struct {
	// Token names the apply that holds the lease, empty when none does; Since is when it took it.
	Token string
	Since time.Time
	// PendingLatest records that the latest version is to be applied again after the running apply,
	// for PendingBy.
	PendingLatest bool
	PendingBy     *Actor
}

// leaseTimeoutSeconds is leaseTimeout as the claim writes it into its query.
var leaseTimeoutSeconds = strconv.Itoa(int(leaseTimeout / time.Second))

var (
	// queryClaimApplyLease takes a gateway's lease for a token where no apply holds it, or where the
	// apply that holds it took it longer ago than any apply can run, by the database's own clock, so
	// a lease an apply never released is taken over and the nodes' clocks need not agree.
	queryClaimApplyLease = dbmodel.DBQuery{
		ID: "GTV_MGT-21",
		Query: `INSERT INTO "GATEWAY_APPLY_LEASE" (DEPLOYMENT_ID, GATEWAY_ID, TOKEN, APPLYING_SINCE) ` +
			`VALUES ($1, $2, $3, NOW()) ON CONFLICT (DEPLOYMENT_ID, GATEWAY_ID) DO UPDATE ` +
			`SET TOKEN = excluded.TOKEN, APPLYING_SINCE = NOW() ` +
			`WHERE "GATEWAY_APPLY_LEASE".TOKEN IS NULL ` +
			`OR "GATEWAY_APPLY_LEASE".APPLYING_SINCE < NOW() - INTERVAL '` + leaseTimeoutSeconds + ` seconds'`,
		SQLiteQuery: `INSERT INTO "GATEWAY_APPLY_LEASE" (DEPLOYMENT_ID, GATEWAY_ID, TOKEN, APPLYING_SINCE) ` +
			`VALUES ($1, $2, $3, datetime('now')) ON CONFLICT (DEPLOYMENT_ID, GATEWAY_ID) DO UPDATE ` +
			`SET TOKEN = excluded.TOKEN, APPLYING_SINCE = datetime('now') ` +
			`WHERE "GATEWAY_APPLY_LEASE".TOKEN IS NULL ` +
			`OR "GATEWAY_APPLY_LEASE".APPLYING_SINCE < datetime('now', '-` + leaseTimeoutSeconds + ` seconds')`,
	}
	// queryHoldsApplyLease reads whether a token still holds a gateway's lease.
	queryHoldsApplyLease = dbmodel.DBQuery{
		ID: "GTV_MGT-26",
		Query: `SELECT 1 AS held FROM "GATEWAY_APPLY_LEASE" ` +
			`WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2 AND TOKEN = $3`,
	}
	// queryReleaseApplyLease gives a gateway's lease back, only by the apply that holds it.
	queryReleaseApplyLease = dbmodel.DBQuery{
		ID: "GTV_MGT-22",
		Query: `UPDATE "GATEWAY_APPLY_LEASE" SET TOKEN = NULL, APPLYING_SINCE = NULL ` +
			`WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2 AND TOKEN = $3`,
	}
	// queryMarkPendingLatest asks for the latest version to be applied again after the running apply,
	// only while one runs: with none running, the asker applies it now instead.
	queryMarkPendingLatest = dbmodel.DBQuery{
		ID: "GTV_MGT-23",
		Query: `UPDATE "GATEWAY_APPLY_LEASE" SET PENDING_LATEST = TRUE, PENDING_BY = $1 ` +
			`WHERE GATEWAY_ID = $2 AND DEPLOYMENT_ID = $3 AND TOKEN IS NOT NULL`,
		SQLiteQuery: `UPDATE "GATEWAY_APPLY_LEASE" SET PENDING_LATEST = 1, PENDING_BY = $1 ` +
			`WHERE GATEWAY_ID = $2 AND DEPLOYMENT_ID = $3 AND TOKEN IS NOT NULL`,
	}
	// queryTakePendingLatest clears the ask and answers with who made it, in one statement, so the
	// apply that took it is the one that answers it, for the right user: another node marking a new
	// ask between a read and the clear would otherwise be cleared under the older user's name. The
	// asker's id stays in the row, ignored while nothing is pending and replaced by the next ask.
	queryTakePendingLatest = dbmodel.DBQuery{
		ID: "GTV_MGT-24",
		Query: `UPDATE "GATEWAY_APPLY_LEASE" SET PENDING_LATEST = FALSE ` +
			`WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2 AND PENDING_LATEST = TRUE RETURNING PENDING_BY`,
		SQLiteQuery: `UPDATE "GATEWAY_APPLY_LEASE" SET PENDING_LATEST = 0 ` +
			`WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2 AND PENDING_LATEST = 1 RETURNING PENDING_BY`,
	}
	// queryDeleteApplyLease forgets a removed gateway's lease.
	queryDeleteApplyLease = dbmodel.DBQuery{
		ID:    "GTV_MGT-25",
		Query: `DELETE FROM "GATEWAY_APPLY_LEASE" WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)

// ClaimLease takes a gateway's lease for token when no apply holds it, or when the one holding it
// took it longer ago than an apply can run, and reports whether it did.
func (s *versionStore) ClaimLease(ctx context.Context, gatewayID, token string) (bool, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return false, fmt.Errorf("failed to get database client: %w", err)
	}
	claimed, err := dbClient.ExecuteContext(ctx, queryClaimApplyLease, deployment.Resolve(ctx), gatewayID, token)
	if err != nil {
		return false, fmt.Errorf("failed to claim the gateway's apply lease: %w", err)
	}
	return claimed > 0, nil
}

// HoldsLease reports whether token still holds the gateway's lease.
func (s *versionStore) HoldsLease(ctx context.Context, gatewayID, token string) (bool, error) {
	rows, err := s.query(ctx, queryHoldsApplyLease, gatewayID, deployment.Resolve(ctx), token)
	if err != nil {
		return false, fmt.Errorf("failed to read the gateway's apply lease: %w", err)
	}
	return len(rows) > 0, nil
}

// ReleaseLease gives a gateway's lease back, when token holds it.
func (s *versionStore) ReleaseLease(ctx context.Context, gatewayID, token string) error {
	if err := s.execute(ctx, queryReleaseApplyLease, gatewayID, deployment.Resolve(ctx), token); err != nil {
		return fmt.Errorf("failed to release the gateway's apply lease: %w", err)
	}
	return nil
}

// MarkPendingLatest asks for the latest version to be applied to the gateway again once the apply
// that holds its lease finishes, for by, and reports false when no apply holds it.
func (s *versionStore) MarkPendingLatest(ctx context.Context, gatewayID string, by *Actor) (bool, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return false, fmt.Errorf("failed to get database client: %w", err)
	}
	marked, err := dbClient.ExecuteContext(ctx, queryMarkPendingLatest, actorColumn(by), gatewayID,
		deployment.Resolve(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to ask for the latest version to be applied again: %w", err)
	}
	return marked > 0, nil
}

// TakePendingLatest clears an ask for the latest version to be applied again, and reports whether
// there was one and for whom.
func (s *versionStore) TakePendingLatest(ctx context.Context, gatewayID string) (*Actor, bool, error) {
	rows, err := s.query(ctx, queryTakePendingLatest, gatewayID, deployment.Resolve(ctx))
	if err != nil {
		return nil, false, fmt.Errorf("failed to take the ask for the latest version: %w", err)
	}
	if len(rows) == 0 {
		return nil, false, nil
	}
	return actorFrom(rows[0]["pending_by"]), true, nil
}

// DeleteLease forgets a gateway's lease.
func (s *versionStore) DeleteLease(ctx context.Context, gatewayID string) error {
	if err := s.execute(ctx, queryDeleteApplyLease, gatewayID, deployment.Resolve(ctx)); err != nil {
		return fmt.Errorf("failed to forget the gateway's apply lease: %w", err)
	}
	return nil
}
