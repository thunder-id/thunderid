// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

var (
	// queryInsertMaster records a gateway's new master, numbered one past its latest. The number is
	// taken inside the statement, so two applies recording at the same moment collide on the unique
	// number rather than both taking it.
	queryInsertMaster = dbmodel.DBQuery{
		ID: "GTM_MGT-01",
		Query: `INSERT INTO "GATEWAY_MASTER" (ID, GATEWAY_ID, SEQ, HASH, RESOURCES, MANIFEST, CREATED_BY, ` +
			`PROJECT_ID, PROJECT_REVERT, DEPLOYMENT_ID) SELECT $1, $2, COALESCE(MAX(SEQ), 0) + 1, $3, $4, $5, $6, ` +
			`$7, $8, $9 FROM "GATEWAY_MASTER" WHERE GATEWAY_ID = $10 AND DEPLOYMENT_ID = $11 ` +
			`RETURNING SEQ, CREATED_AT`,
	}
	// queryInsertMasterUnderLease records a gateway's new master as queryInsertMaster does, only while
	// the apply writing it still holds the gateway's lease: one superseded after a take-over records
	// nothing, as it records no applied version either.
	queryInsertMasterUnderLease = dbmodel.DBQuery{
		ID: "GTM_MGT-06",
		Query: `INSERT INTO "GATEWAY_MASTER" (ID, GATEWAY_ID, SEQ, HASH, RESOURCES, MANIFEST, CREATED_BY, ` +
			`PROJECT_ID, PROJECT_REVERT, DEPLOYMENT_ID) SELECT $1, $2, COALESCE(MAX(SEQ), 0) + 1, $3, $4, $5, $6, ` +
			`$7, $8, $9 FROM "GATEWAY_MASTER" WHERE GATEWAY_ID = $10 AND DEPLOYMENT_ID = $11 ` +
			`HAVING EXISTS (SELECT 1 FROM "GATEWAY_APPLY_LEASE" ` +
			`WHERE GATEWAY_ID = $10 AND DEPLOYMENT_ID = $11 AND TOKEN = $12) RETURNING SEQ, CREATED_AT`,
	}
	// queryGetMaster reads the master a gateway holds now, with its resources.
	queryGetMaster = dbmodel.DBQuery{
		ID: "GTM_MGT-02",
		Query: `SELECT SEQ, HASH, RESOURCES, MANIFEST, CREATED_AT, CREATED_BY, PROJECT_ID, PROJECT_REVERT ` +
			`FROM "GATEWAY_MASTER" ` +
			`WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2 ORDER BY SEQ DESC LIMIT 1`,
	}
	// queryListMasters lists a gateway's masters, newest first, with their manifests but not their
	// resources.
	queryListMasters = dbmodel.DBQuery{
		ID: "GTM_MGT-03",
		Query: `SELECT SEQ, HASH, MANIFEST, CREATED_AT, CREATED_BY, PROJECT_ID, PROJECT_REVERT FROM "GATEWAY_MASTER" ` +
			`WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2 ORDER BY SEQ DESC`,
	}
	// queryPruneMasters removes a gateway's masters numbered from one number through another, which
	// pruning runs for each stretch below the retention bound between the masters it keeps.
	queryPruneMasters = dbmodel.DBQuery{
		ID: "GTM_MGT-04",
		Query: `DELETE FROM "GATEWAY_MASTER" WHERE GATEWAY_ID = $1 AND SEQ >= $2 AND SEQ <= $3 ` +
			`AND DEPLOYMENT_ID = $4`,
	}
	// queryGetMasterAt reads one of a gateway's masters by its number, with its resources.
	queryGetMasterAt = dbmodel.DBQuery{
		ID: "GTM_MGT-07",
		Query: `SELECT SEQ, HASH, RESOURCES, MANIFEST, CREATED_AT, CREATED_BY, PROJECT_ID, PROJECT_REVERT ` +
			`FROM "GATEWAY_MASTER" ` +
			`WHERE GATEWAY_ID = $1 AND SEQ = $2 AND DEPLOYMENT_ID = $3`,
	}
	// queryDeleteMasters forgets a removed gateway's masters.
	queryDeleteMasters = dbmodel.DBQuery{
		ID:    "GTM_MGT-05",
		Query: `DELETE FROM "GATEWAY_MASTER" WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)

// errMasterNumberTaken is AddMaster finding the master's number taken by another write at the same
// moment. It is the one failure that leaves a master recorded, the other write's.
var errMasterNumberTaken = errors.New("another write took the master's number")

// errLeaseLost is AddMaster finding that the apply writing the master no longer holds the gateway's
// lease, so it recorded nothing.
var errLeaseLost = errors.New("the apply no longer holds the gateway's lease")

const (
	// masterNumberIndexName is the name PostgreSQL gives the unique key on a gateway's master numbers,
	// and masterNumberIndexColumns its column list as SQLite reports it.
	masterNumberIndexName    = "GATEWAY_MASTER_deployment_id_gateway_id_seq_key"
	masterNumberIndexColumns = "GATEWAY_MASTER.DEPLOYMENT_ID, GATEWAY_MASTER.GATEWAY_ID, GATEWAY_MASTER.SEQ"
)

func (s *versionStore) AddMaster(ctx context.Context, master Master, token string) (*Master, error) {
	manifest, err := manifestOf(master.Entries)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the master's manifest: %w", err)
	}
	deploymentID := deployment.Resolve(ctx)
	args := []interface{}{uuid.New().String(), master.GatewayID, master.Hash, master.Resources, manifest,
		actorColumn(master.CreatedBy), nullable(master.Project), master.ProjectRevert, deploymentID,
		master.GatewayID, deploymentID}
	insert := queryInsertMaster
	if token != "" {
		insert = queryInsertMasterUnderLease
		args = append(args, token)
	}
	rows, err := s.query(ctx, insert, args...)
	if dbprovider.IsUniqueIndexViolation(err, masterNumberIndexName, masterNumberIndexColumns) {
		return nil, fmt.Errorf("%w: %w", errMasterNumberTaken, err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to record the master: %w", err)
	}
	if len(rows) == 0 {
		if token != "" {
			return nil, errLeaseLost
		}
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

func (s *versionStore) GetMasterAt(ctx context.Context, gatewayID string, seq int) (*Master, error) {
	rows, err := s.query(ctx, queryGetMasterAt, gatewayID, seq, deployment.Resolve(ctx))
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
		Project:   asString(row["project_id"]),
		Summary:   summarizeMaster(entries),
		Entries:   entries,
		// ProjectRevert is read apart from the row's other columns: a driver answers a boolean as one.
		ProjectRevert: asBool(row["project_revert"]),
	}, nil
}

func (s *versionStore) PruneMasters(ctx context.Context, gatewayID string, through int, keep []int) error {
	for _, stretch := range stretchesWithout(through, keep) {
		if err := s.execute(ctx, queryPruneMasters, gatewayID, stretch[0], stretch[1],
			deployment.Resolve(ctx)); err != nil {
			return fmt.Errorf("failed to remove old masters: %w", err)
		}
	}
	return nil
}

// stretchesWithout splits the numbers from 1 through through into the stretches between the kept ones,
// each as its first and last number.
func stretchesWithout(through int, keep []int) [][2]int {
	kept := append([]int(nil), keep...)
	sort.Ints(kept)
	var stretches [][2]int
	from := 1
	for _, seq := range kept {
		if seq > through {
			break
		}
		if seq > from {
			stretches = append(stretches, [2]int{from, seq - 1})
		}
		if seq >= from {
			from = seq + 1
		}
	}
	if from <= through {
		stretches = append(stretches, [2]int{from, through})
	}
	return stretches
}

func (s *versionStore) DeleteMasters(ctx context.Context, gatewayID string) error {
	if err := s.execute(ctx, queryDeleteMasters, gatewayID, deployment.Resolve(ctx)); err != nil {
		return fmt.Errorf("failed to forget the gateway's masters: %w", err)
	}
	return nil
}

func (s *versionStore) Transact(ctx context.Context, work func(txCtx context.Context) error) error {
	transactioner, err := s.dbProvider.GetConfigDBTransactioner()
	if err != nil {
		return fmt.Errorf("failed to get database transactioner: %w", err)
	}
	return transactioner.Transact(ctx, work)
}
