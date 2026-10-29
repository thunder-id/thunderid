// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

var (
	// queryInsertMaster records a gateway's new master, numbered one past its latest. The number is
	// taken inside the statement, so two applies recording at the same moment collide on the unique
	// number rather than both taking it.
	queryInsertMaster = dbmodel.DBQuery{
		ID: "GTM_MGT-01",
		Query: `INSERT INTO "GATEWAY_MASTER" (ID, GATEWAY_ID, SEQ, HASH, RESOURCES, MANIFEST, CREATED_BY, ` +
			`DEPLOYMENT_ID) SELECT $1, $2, COALESCE(MAX(SEQ), 0) + 1, $3, $4, $5, $6, $7 ` +
			`FROM "GATEWAY_MASTER" WHERE GATEWAY_ID = $8 AND DEPLOYMENT_ID = $9 RETURNING SEQ, CREATED_AT`,
	}
	// queryGetMaster reads the master a gateway holds now, with its resources.
	queryGetMaster = dbmodel.DBQuery{
		ID: "GTM_MGT-02",
		Query: `SELECT SEQ, HASH, RESOURCES, MANIFEST, CREATED_AT, CREATED_BY FROM "GATEWAY_MASTER" ` +
			`WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2 ORDER BY SEQ DESC LIMIT 1`,
	}
	// queryListMasters lists a gateway's masters, newest first, with their manifests but not their
	// resources.
	queryListMasters = dbmodel.DBQuery{
		ID: "GTM_MGT-03",
		Query: `SELECT SEQ, HASH, MANIFEST, CREATED_AT, CREATED_BY FROM "GATEWAY_MASTER" ` +
			`WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2 ORDER BY SEQ DESC`,
	}
	// queryPruneMasters removes a gateway's masters older than the retention bound.
	queryPruneMasters = dbmodel.DBQuery{
		ID:    "GTM_MGT-04",
		Query: `DELETE FROM "GATEWAY_MASTER" WHERE GATEWAY_ID = $1 AND SEQ <= $2 AND DEPLOYMENT_ID = $3`,
	}
	// queryDeleteMasters forgets a removed gateway's masters.
	queryDeleteMasters = dbmodel.DBQuery{
		ID:    "GTM_MGT-05",
		Query: `DELETE FROM "GATEWAY_MASTER" WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)

func (s *versionStore) AddMaster(ctx context.Context, master Master) (*Master, error) {
	manifest, err := manifestOf(master.Entries)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the master's manifest: %w", err)
	}
	deploymentID := deployment.Resolve(ctx)
	rows, err := s.query(ctx, queryInsertMaster, uuid.New().String(), master.GatewayID, master.Hash,
		master.Resources, manifest, actorColumn(master.CreatedBy), deploymentID, master.GatewayID, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to record the master: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("recording the master returned no row")
	}
	added := master
	added.Seq = asInt(rows[0]["seq"])
	added.CreatedAt = asTime(rows[0]["created_at"])
	return &added, nil
}

func (s *versionStore) GetMaster(ctx context.Context, gatewayID string) (*Master, error) {
	rows, err := s.query(ctx, queryGetMaster, gatewayID, deployment.Resolve(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to read the gateway's master: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	master, err := masterFrom(gatewayID, rows[0])
	if err != nil {
		return nil, err
	}
	master.Resources = asString(rows[0]["resources"])
	return master, nil
}

func (s *versionStore) ListMasters(ctx context.Context, gatewayID string) ([]Master, error) {
	rows, err := s.query(ctx, queryListMasters, gatewayID, deployment.Resolve(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to list the gateway's masters: %w", err)
	}
	masters := make([]Master, 0, len(rows))
	for _, row := range rows {
		master, err := masterFrom(gatewayID, row)
		if err != nil {
			return nil, err
		}
		masters = append(masters, *master)
	}
	return masters, nil
}

// masterFrom reads a master's row, without its resources.
func masterFrom(gatewayID string, row map[string]interface{}) (*Master, error) {
	entries, err := entriesOf(asString(row["manifest"]))
	if err != nil {
		return nil, fmt.Errorf("failed to read the master's manifest: %w", err)
	}
	return &Master{
		GatewayID: gatewayID,
		Seq:       asInt(row["seq"]),
		Hash:      asString(row["hash"]),
		CreatedAt: asTime(row["created_at"]),
		CreatedBy: actorFrom(row["created_by"]),
		Summary:   summarizeMaster(entries),
		Entries:   entries,
	}, nil
}

func (s *versionStore) PruneMasters(ctx context.Context, gatewayID string, through int) error {
	if through <= 0 {
		return nil
	}
	if err := s.execute(ctx, queryPruneMasters, gatewayID, through, deployment.Resolve(ctx)); err != nil {
		return fmt.Errorf("failed to remove old masters: %w", err)
	}
	return nil
}

func (s *versionStore) DeleteMasters(ctx context.Context, gatewayID string) error {
	if err := s.execute(ctx, queryDeleteMasters, gatewayID, deployment.Resolve(ctx)); err != nil {
		return fmt.Errorf("failed to forget the gateway's masters: %w", err)
	}
	return nil
}
