// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocationcache

import (
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	columnNameJTI            = "jti"
	columnNameCriterionValue = "criterion_value"
	columnNameExpiryTime     = "expiry_time"
	columnNameRevokedAt      = "revoked_at"
	columnNameReason         = "reason"
	// criterionTypeTokenFamily mirrors the revocation package's token_family criterion type. It is
	// duplicated here (not imported) so this read-only RS package stays decoupled from the write path.
	criterionTypeTokenFamily = "token_family"
	criterionTypeSubject     = "subject"
	criterionTypeAppKey      = "app.key"
)

// dbSource reads the deny-list snapshot from the runtime persistent database. It is the only source today; it
// issues a single read-only bulk query per sync and never writes.
type dbSource struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newDBSource creates a dbSource bound to the runtime persistent database and this deployment's identifier.
func newDBSource() syncSource {
	return &dbSource{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// Snapshot returns the non-expired deny-list entries enforced by the Resource Server.
func (s *dbSource) Snapshot(ctx context.Context) (revokedSnapshot, error) {
	dbClient, err := s.dbProvider.GetRuntimePersistentDBClient()
	if err != nil {
		return revokedSnapshot{}, fmt.Errorf("failed to get runtime persistent database client: %w", err)
	}

	now := time.Now().UTC()

	tokenRows, err := dbClient.QueryContext(ctx, querySnapshotRevokedTokens, now, s.deploymentID)
	if err != nil {
		return revokedSnapshot{}, fmt.Errorf("error reading revoked token snapshot: %w", err)
	}
	tokens, err := parseEntries(tokenRows, columnNameJTI)
	if err != nil {
		return revokedSnapshot{}, err
	}

	tokenFamilyRows, err := dbClient.QueryContext(ctx, querySnapshotRevokedTokenFamilies,
		criterionTypeTokenFamily, now, s.deploymentID)
	if err != nil {
		return revokedSnapshot{}, fmt.Errorf("error reading revoked token family snapshot: %w", err)
	}
	families, err := parseEntries(tokenFamilyRows, columnNameCriterionValue)
	if err != nil {
		return revokedSnapshot{}, err
	}

	subjectRows, err := dbClient.QueryContext(ctx, querySnapshotBoundedCriteria,
		criterionTypeSubject, now, s.deploymentID)
	if err != nil {
		return revokedSnapshot{}, fmt.Errorf("error reading revoked subject snapshot: %w", err)
	}
	subjects, err := parseBoundedEntries(subjectRows)
	if err != nil {
		return revokedSnapshot{}, err
	}

	appKeyRows, err := dbClient.QueryContext(ctx, querySnapshotBoundedCriteria,
		criterionTypeAppKey, now, s.deploymentID)
	if err != nil {
		return revokedSnapshot{}, fmt.Errorf("error reading revoked application snapshot: %w", err)
	}
	appKeys, err := parseBoundedEntries(appKeyRows)
	if err != nil {
		return revokedSnapshot{}, err
	}

	return revokedSnapshot{Tokens: tokens, Families: families, Subjects: subjects, AppKeys: appKeys}, nil
}

// parseBoundedEntries maps criteria of one dimension and retains their establishment cutoff, so a
// bounded reason rejects only artifacts established at or before it.
func parseBoundedEntries(rows []map[string]interface{}) ([]revokedEntry, error) {
	entries, err := parseEntries(rows, columnNameCriterionValue)
	if err != nil {
		return nil, err
	}
	for i, row := range rows {
		revokedAt, parseErr := utils.ParseDBTimeField(row[columnNameRevokedAt], columnNameRevokedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("error parsing revocation snapshot: %w", parseErr)
		}
		entries[i].RevokedAt = revokedAt
		reason, ok := row[columnNameReason].(string)
		if !ok || reason == "" {
			return nil, fmt.Errorf("invalid or missing %s in revocation snapshot", columnNameReason)
		}
		entries[i].Boundary = revocation.IsBoundaryReason(revocation.Reason(reason))
	}
	return entries, nil
}

// parseEntries maps deny-list rows into revoked entries, reading the lookup value from valueColumn and
// the expiry from the standard expiry-time column.
func parseEntries(rows []map[string]interface{}, valueColumn string) ([]revokedEntry, error) {
	entries := make([]revokedEntry, 0, len(rows))
	for _, row := range rows {
		value, ok := row[valueColumn].(string)
		if !ok || value == "" {
			return nil, fmt.Errorf("invalid or missing %s in revocation snapshot", valueColumn)
		}
		expiryTime, err := utils.ParseDBTimeField(row[columnNameExpiryTime], columnNameExpiryTime)
		if err != nil {
			return nil, fmt.Errorf("error parsing revocation snapshot: %w", err)
		}
		entries = append(entries, revokedEntry{Value: value, ExpiryTime: expiryTime})
	}
	return entries, nil
}
