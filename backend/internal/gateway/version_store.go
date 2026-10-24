// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

var (
	// queryInsertVersion captures a version, numbered one past the deployment's latest. The number is
	// taken inside the statement, so it cannot be read stale; two captures at the same moment collide
	// on the unique number instead, and the second is refused.
	queryInsertVersion = dbmodel.DBQuery{
		ID: "GTV_MGT-01",
		Query: `INSERT INTO "CONFIGURATION_VERSION" (ID, SEQ, HASH, NAME, RESOURCES, VARIABLES, NOTE, IS_CURRENT, ` +
			`CREATED_BY, DEPLOYMENT_ID) ` +
			`SELECT $1, COALESCE(MAX(SEQ), 0) + 1, $2, $3, $4, $5, $6, $7, $8, $9 ` +
			`FROM "CONFIGURATION_VERSION" WHERE DEPLOYMENT_ID = $10 RETURNING SEQ, CREATED_AT`,
	}
	// queryReplaceCurrentVersion puts a configuration in place of the current version: the newest one,
	// when it was captured as current and no gateway holds it or could be reverted to it. The checks
	// are part of the statement, so an apply recording the version at the same moment either sees the
	// old content and is refused for its hash, or the replacement finds the version held and leaves it.
	// '1' is currentYes: only a version replaced in place is replaced again.
	queryReplaceCurrentVersion = dbmodel.DBQuery{
		ID: "GTV_MGT-14",
		Query: `UPDATE "CONFIGURATION_VERSION" SET HASH = $1, NAME = $2, RESOURCES = $3, VARIABLES = $4, ` +
			`NOTE = $5, CREATED_BY = $6, CREATED_AT = NOW() ` +
			`WHERE DEPLOYMENT_ID = $7 AND IS_CURRENT = '1' ` +
			`AND SEQ = (SELECT MAX(SEQ) FROM "CONFIGURATION_VERSION" WHERE DEPLOYMENT_ID = $8) ` +
			`AND SEQ NOT IN (SELECT APPLIED_SEQ FROM "GATEWAY_APPLIED_VERSION" WHERE DEPLOYMENT_ID = $9) ` +
			`AND SEQ NOT IN (SELECT PREVIOUS_SEQ FROM "GATEWAY_APPLIED_VERSION" ` +
			`WHERE PREVIOUS_SEQ IS NOT NULL AND DEPLOYMENT_ID = $10) RETURNING SEQ, CREATED_AT`,
		SQLiteQuery: `UPDATE "CONFIGURATION_VERSION" SET HASH = $1, NAME = $2, RESOURCES = $3, VARIABLES = $4, ` +
			`NOTE = $5, CREATED_BY = $6, CREATED_AT = datetime('now') ` +
			`WHERE DEPLOYMENT_ID = $7 AND IS_CURRENT = '1' ` +
			`AND SEQ = (SELECT MAX(SEQ) FROM "CONFIGURATION_VERSION" WHERE DEPLOYMENT_ID = $8) ` +
			`AND SEQ NOT IN (SELECT APPLIED_SEQ FROM "GATEWAY_APPLIED_VERSION" WHERE DEPLOYMENT_ID = $9) ` +
			`AND SEQ NOT IN (SELECT PREVIOUS_SEQ FROM "GATEWAY_APPLIED_VERSION" ` +
			`WHERE PREVIOUS_SEQ IS NOT NULL AND DEPLOYMENT_ID = $10) RETURNING SEQ, CREATED_AT`,
	}
	// querySettleVersion keeps a version as it is from now on. A current version stays current, marked
	// kept, until a later capture, but a capture no longer replaces it in place. It matches the hash an
	// apply read, so a version replaced or removed since is not settled.
	// '1' is currentYes and '2' currentKept: a version replaced in place becomes kept as it is.
	querySettleVersion = dbmodel.DBQuery{
		ID: "GTV_MGT-15",
		Query: `UPDATE "CONFIGURATION_VERSION" SET IS_CURRENT = CASE WHEN IS_CURRENT = '1' THEN '2' ` +
			`ELSE IS_CURRENT END WHERE SEQ = $1 AND HASH = $2 AND DEPLOYMENT_ID = $3`,
	}
	// queryListVersions lists the deployment's versions, newest first, without their content.
	queryListVersions = dbmodel.DBQuery{
		ID: "GTV_MGT-02",
		Query: `SELECT SEQ, HASH, NAME, NOTE, IS_CURRENT, CREATED_AT, CREATED_BY FROM "CONFIGURATION_VERSION" ` +
			`WHERE DEPLOYMENT_ID = $1 ORDER BY SEQ DESC`,
	}
	// queryGetVersion reads one version with its content.
	queryGetVersion = dbmodel.DBQuery{
		ID: "GTV_MGT-03",
		Query: `SELECT SEQ, HASH, NAME, RESOURCES, VARIABLES, NOTE, CREATED_AT, CREATED_BY ` +
			`FROM "CONFIGURATION_VERSION" WHERE SEQ = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryLatestVersion reads the deployment's newest version number.
	queryLatestVersion = dbmodel.DBQuery{
		ID:    "GTV_MGT-04",
		Query: `SELECT COALESCE(MAX(SEQ), 0) AS latest FROM "CONFIGURATION_VERSION" WHERE DEPLOYMENT_ID = $1`,
	}
	// queryPruneVersions removes versions older than the retention bound, except one a gateway holds
	// or could be reverted to.
	queryPruneVersions = dbmodel.DBQuery{
		ID: "GTV_MGT-05",
		Query: `DELETE FROM "CONFIGURATION_VERSION" WHERE SEQ <= $1 ` +
			`AND SEQ NOT IN (SELECT APPLIED_SEQ FROM "GATEWAY_APPLIED_VERSION" WHERE DEPLOYMENT_ID = $2) ` +
			`AND SEQ NOT IN (SELECT PREVIOUS_SEQ FROM "GATEWAY_APPLIED_VERSION" ` +
			`WHERE PREVIOUS_SEQ IS NOT NULL AND DEPLOYMENT_ID = $3) ` +
			`AND DEPLOYMENT_ID = $4`,
	}
	// queryUnsettleVersion gives a version back its place as the current one after an apply settled
	// it and then wrote nothing to the gateway. Only the version as settled is touched: by number and
	// hash, still the newest, and run or reverted from by no gateway, so a later capture or apply is
	// never undone.
	queryUnsettleVersion = dbmodel.DBQuery{
		ID: "GTV_MGT-16",
		Query: `UPDATE "CONFIGURATION_VERSION" SET IS_CURRENT = '1' ` +
			`WHERE SEQ = $1 AND HASH = $2 AND DEPLOYMENT_ID = $3 AND IS_CURRENT = '2' ` +
			`AND SEQ = (SELECT MAX(SEQ) FROM "CONFIGURATION_VERSION" WHERE DEPLOYMENT_ID = $4) ` +
			`AND SEQ NOT IN (SELECT APPLIED_SEQ FROM "GATEWAY_APPLIED_VERSION" WHERE DEPLOYMENT_ID = $5) ` +
			`AND SEQ NOT IN (SELECT PREVIOUS_SEQ FROM "GATEWAY_APPLIED_VERSION" ` +
			`WHERE PREVIOUS_SEQ IS NOT NULL AND DEPLOYMENT_ID = $6)`,
	}
	// queryGetAppliedVersion reads what a gateway holds.
	queryGetAppliedVersion = dbmodel.DBQuery{
		ID: "GTV_MGT-06",
		Query: `SELECT A.APPLIED_SEQ, A.PREVIOUS_SEQ, A.APPLIED_AT, A.APPLIED_BY, A.HELD_VALUES, ` +
			`V.HASH AS APPLIED_HASH, ` +
			`P.HASH AS PREVIOUS_HASH FROM "GATEWAY_APPLIED_VERSION" A ` +
			`LEFT JOIN "CONFIGURATION_VERSION" V ON V.SEQ = A.APPLIED_SEQ AND V.DEPLOYMENT_ID = A.DEPLOYMENT_ID ` +
			`LEFT JOIN "CONFIGURATION_VERSION" P ON P.SEQ = A.PREVIOUS_SEQ AND P.DEPLOYMENT_ID = A.DEPLOYMENT_ID ` +
			`WHERE A.GATEWAY_ID = $1 AND A.DEPLOYMENT_ID = $2`,
	}
	// queryUpsertAppliedVersion records what a gateway now holds, only while that version is still kept,
	// the gateway still holds the version the apply worked from, and the apply still holds the
	// gateway's lease: one superseded after a take-over records nothing.
	//
	// A capture can prune the version an apply is writing; recording it anyway would leave the gateway
	// holding a version no longer kept, and the next apply could not work out what to remove. Two
	// applies to one gateway can also run at once, each working out its deletions from the same held
	// version; the update applies only where the row still holds it, so the one that finishes second is
	// refused rather than recording a state its own import did not leave.
	queryUpsertAppliedVersion = dbmodel.DBQuery{
		ID: "GTV_MGT-07",
		Query: `INSERT INTO "GATEWAY_APPLIED_VERSION" (GATEWAY_ID, APPLIED_SEQ, PREVIOUS_SEQ, APPLIED_BY, ` +
			`DEPLOYMENT_ID, HELD_VALUES) ` +
			`SELECT CAST($1 AS VARCHAR(36)), CAST($2 AS INTEGER), CAST($3 AS INTEGER), ` +
			`CAST($4 AS VARCHAR(255)), CAST($5 AS VARCHAR(255)), CAST($11 AS TEXT) ` +
			`WHERE EXISTS (SELECT 1 FROM "CONFIGURATION_VERSION" ` +
			`WHERE SEQ = $6 AND HASH = $7 AND DEPLOYMENT_ID = $8) ` +
			`AND EXISTS (SELECT 1 FROM "GATEWAY_APPLY_LEASE" ` +
			`WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $5 AND TOKEN = $10) ` +
			`ON CONFLICT (GATEWAY_ID, DEPLOYMENT_ID) DO UPDATE SET ` +
			`APPLIED_SEQ = excluded.APPLIED_SEQ, PREVIOUS_SEQ = excluded.PREVIOUS_SEQ, APPLIED_AT = NOW(), ` +
			`APPLIED_BY = excluded.APPLIED_BY, HELD_VALUES = excluded.HELD_VALUES ` +
			`WHERE "GATEWAY_APPLIED_VERSION".APPLIED_SEQ = $9`,
		SQLiteQuery: `INSERT INTO "GATEWAY_APPLIED_VERSION" (GATEWAY_ID, APPLIED_SEQ, PREVIOUS_SEQ, APPLIED_BY, ` +
			`DEPLOYMENT_ID, HELD_VALUES) ` +
			`SELECT $1, $2, $3, $4, $5, $11 WHERE EXISTS (SELECT 1 FROM "CONFIGURATION_VERSION" ` +
			`WHERE SEQ = $6 AND HASH = $7 AND DEPLOYMENT_ID = $8) ` +
			`AND EXISTS (SELECT 1 FROM "GATEWAY_APPLY_LEASE" ` +
			`WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $5 AND TOKEN = $10) ` +
			`ON CONFLICT (GATEWAY_ID, DEPLOYMENT_ID) DO UPDATE SET ` +
			`APPLIED_SEQ = excluded.APPLIED_SEQ, PREVIOUS_SEQ = excluded.PREVIOUS_SEQ, ` +
			`APPLIED_AT = datetime('now'), APPLIED_BY = excluded.APPLIED_BY, HELD_VALUES = excluded.HELD_VALUES ` +
			`WHERE "GATEWAY_APPLIED_VERSION".APPLIED_SEQ = $9`,
	}
	// queryFindVersions finds the versions whose hash starts with a prefix, newest first. Two are enough
	// to tell a prefix that names one version from one that names several.
	queryFindVersions = dbmodel.DBQuery{
		ID: "GTV_MGT-13",
		Query: `SELECT SEQ FROM "CONFIGURATION_VERSION" WHERE HASH LIKE $1 AND DEPLOYMENT_ID = $2 ` +
			`ORDER BY SEQ DESC LIMIT 2`,
	}
	// queryDeleteAppliedVersion forgets what a removed gateway held.
	queryDeleteAppliedVersion = dbmodel.DBQuery{
		ID:    "GTV_MGT-08",
		Query: `DELETE FROM "GATEWAY_APPLIED_VERSION" WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
	// queryGetExcluded reads the resources a gateway is set to leave alone, and those owed to it from a
	// skipped apply.
	queryGetExcluded = dbmodel.DBQuery{
		ID: "GTV_MGT-09",
		Query: `SELECT RESOURCE_KEY, RESOURCE_TYPE, RESOURCE_ID, RESOURCE_NAME, ONCE ` +
			`FROM "GATEWAY_EXCLUDED_RESOURCE" WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2 ORDER BY RESOURCE_KEY`,
	}
	// queryExclude sets a gateway to leave a resource alone, for good or for one apply. A standing
	// choice is never turned into a one-time one: a resource both skipped and left alone stays left alone.
	queryExclude = dbmodel.DBQuery{
		ID: "GTV_MGT-10",
		Query: `INSERT INTO "GATEWAY_EXCLUDED_RESOURCE" (GATEWAY_ID, RESOURCE_KEY, RESOURCE_TYPE, RESOURCE_ID, ` +
			`RESOURCE_NAME, DEPLOYMENT_ID, ONCE) VALUES ($1, $2, $3, $4, $5, $6, $7) ` +
			`ON CONFLICT (DEPLOYMENT_ID, GATEWAY_ID, RESOURCE_KEY) ` +
			`DO UPDATE SET ONCE = ("GATEWAY_EXCLUDED_RESOURCE".ONCE AND excluded.ONCE)`,
	}
	// queryInclude lets an apply write a resource to a gateway again.
	queryInclude = dbmodel.DBQuery{
		ID: "GTV_MGT-11",
		Query: `DELETE FROM "GATEWAY_EXCLUDED_RESOURCE" WHERE GATEWAY_ID = $1 AND RESOURCE_KEY = $2 ` +
			`AND DEPLOYMENT_ID = $3`,
	}
	// queryDeleteExcluded forgets what a removed gateway was set to leave alone.
	queryDeleteExcluded = dbmodel.DBQuery{
		ID:    "GTV_MGT-12",
		Query: `DELETE FROM "GATEWAY_EXCLUDED_RESOURCE" WHERE GATEWAY_ID = $1 AND DEPLOYMENT_ID = $2`,
	}
)

// versionStoreInterface is the persistence the versions need.
type versionStoreInterface interface {
	// Add captures a version. Its hash is unique within the deployment, so capturing a hash already
	// kept fails.
	Add(ctx context.Context, version Version) (*Version, error)
	// ReplaceCurrent puts a version in place of the current one, and returns nil when there is none to
	// replace: the newest version was not captured as current, an apply sent it, or a gateway holds it.
	ReplaceCurrent(ctx context.Context, version Version) (*Version, error)
	// Settle keeps a version as it is from now on, so it is never replaced in place, and reports false
	// when the deployment has no version with that number and hash.
	Settle(ctx context.Context, seq int, hash string) (bool, error)
	// Unsettle makes a settled version current again, when it is still the newest and no gateway runs
	// or reverted from it, and reports whether it did.
	Unsettle(ctx context.Context, seq int, hash string) (bool, error)
	List(ctx context.Context) ([]Version, error)
	// Get reads one version with its content, or nil when the deployment has no such version.
	Get(ctx context.Context, seq int) (*Version, error)
	Latest(ctx context.Context) (int, error)
	// Find returns the places of up to two versions whose hash starts with prefix, newest first.
	Find(ctx context.Context, prefix string) ([]int, error)
	// Prune removes versions numbered up to and including through, except those a gateway holds or
	// could be reverted to.
	Prune(ctx context.Context, through int) error
	// GetApplied reads what a gateway holds, or nil when nothing has been applied to it.
	GetApplied(ctx context.Context, gatewayID string) (*AppliedVersion, error)
	// SetApplied records what a gateway now holds, provided the applied version still has the hash the
	// apply wrote, the gateway still holds held, the version the apply worked out its deletions from
	// (0 for none), and token still holds the gateway's lease. By is who applied it.
	SetApplied(ctx context.Context, gatewayID string, applied int, hash string, previous, held int,
		by *Actor, values, token string) error
	DeleteApplied(ctx context.Context, gatewayID string) error
	// ClaimLease takes the gateway's lease for token when none holds it, or when the apply holding it
	// took it longer ago than an apply can run, and reports whether it did.
	ClaimLease(ctx context.Context, gatewayID, token string) (bool, error)
	// HoldsLease reports whether token still holds the gateway's lease.
	HoldsLease(ctx context.Context, gatewayID, token string) (bool, error)
	// ReleaseLease gives the lease back, when token holds it.
	ReleaseLease(ctx context.Context, gatewayID, token string) error
	// MarkPendingLatest asks for the latest version to be applied again after the apply holding the
	// lease, for by, and reports false when none holds it.
	MarkPendingLatest(ctx context.Context, gatewayID string, by *Actor) (bool, error)
	// TakePendingLatest clears that ask and reports whether there was one and for whom.
	TakePendingLatest(ctx context.Context, gatewayID string) (*Actor, bool, error)
	// DeleteLease forgets a removed gateway's lease.
	DeleteLease(ctx context.Context, gatewayID string) error
	// GetExcluded reads the resources a gateway is set to leave alone, and those a skipped apply left
	// owed to it, sorted by key.
	GetExcluded(ctx context.Context, gatewayID string) ([]excludedResource, error)
	// SetExcluded takes the include keys off what a gateway leaves alone and then sets it to leave the
	// exclude resources alone, in one transaction. A resource in both is replaced as given, so a
	// caller can put a row back exactly as it was; one only excluded keeps a standing choice standing.
	SetExcluded(ctx context.Context, gatewayID string, exclude []excludedResource, include []string) error
	DeleteExcluded(ctx context.Context, gatewayID string) error
}

// versionStore keeps versions in the configuration database. It is used whatever the gateway store
// mode is: a gateway declared in a file is held in memory, but what is applied to it is not.
type versionStore struct {
	dbProvider dbprovider.DBProviderInterface
}

func newVersionStore() versionStoreInterface {
	return &versionStore{
		dbProvider: dbprovider.GetDBProvider(),
	}
}

func (s *versionStore) query(ctx context.Context, q dbmodel.DBQuery,
	args ...interface{}) ([]map[string]interface{}, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	return dbClient.QueryContext(ctx, q, args...)
}

func (s *versionStore) execute(ctx context.Context, q dbmodel.DBQuery, args ...interface{}) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	_, err = dbClient.ExecuteContext(ctx, q, args...)
	return err
}

func (s *versionStore) Add(ctx context.Context, version Version) (*Version, error) {
	rows, err := s.query(ctx, queryInsertVersion, uuid.New().String(), version.Hash, version.Name,
		version.Resources, version.variables, version.Note, flag(version.Current), actorColumn(version.CreatedBy),
		deployment.Resolve(ctx), deployment.Resolve(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to capture the version: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("capturing the version returned no row")
	}
	added := version
	added.Seq = asInt(rows[0]["seq"])
	added.CreatedAt = asTime(rows[0]["created_at"])
	added.variables = ""
	return &added, nil
}

func (s *versionStore) ReplaceCurrent(ctx context.Context, version Version) (*Version, error) {
	deploymentID := deployment.Resolve(ctx)
	rows, err := s.query(ctx, queryReplaceCurrentVersion, version.Hash, version.Name, version.Resources,
		version.variables, version.Note, actorColumn(version.CreatedBy), deploymentID, deploymentID,
		deploymentID, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to replace the current version: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	replaced := version
	replaced.Seq = asInt(rows[0]["seq"])
	replaced.CreatedAt = asTime(rows[0]["created_at"])
	replaced.Current = true
	replaced.variables = ""
	return &replaced, nil
}

func (s *versionStore) Settle(ctx context.Context, seq int, hash string) (bool, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return false, fmt.Errorf("failed to get database client: %w", err)
	}
	settled, err := dbClient.ExecuteContext(ctx, querySettleVersion, seq, hash, deployment.Resolve(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to settle the version: %w", err)
	}
	return settled > 0, nil
}

func (s *versionStore) Unsettle(ctx context.Context, seq int, hash string) (bool, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return false, fmt.Errorf("failed to get database client: %w", err)
	}
	deploymentID := deployment.Resolve(ctx)
	restored, err := dbClient.ExecuteContext(ctx, queryUnsettleVersion, seq, hash, deploymentID, deploymentID,
		deploymentID, deploymentID)
	if err != nil {
		return false, fmt.Errorf("failed to make the version current again: %w", err)
	}
	return restored > 0, nil
}

// How IS_CURRENT keeps whether a version was captured as current: not current, current and replaced
// in place by the next capture, or current and kept as it is because an apply sent it, which
// querySettleVersion sets.
const (
	currentNo   = "0"
	currentYes  = "1"
	currentKept = "2"
)

// isCurrent reads IS_CURRENT: a version replaced in place and one kept as it is are both current.
func isCurrent(value string) bool {
	return value == currentYes || value == currentKept
}

// flag is how a capture keeps whether it is the current version.
func flag(current bool) string {
	if current {
		return currentYes
	}
	return currentNo
}

func (s *versionStore) List(ctx context.Context) ([]Version, error) {
	rows, err := s.query(ctx, queryListVersions, deployment.Resolve(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to list the versions: %w", err)
	}
	versions := make([]Version, 0, len(rows))
	for _, row := range rows {
		versions = append(versions, Version{
			Seq:       asInt(row["seq"]),
			Hash:      asString(row["hash"]),
			Name:      asString(row["name"]),
			Note:      asString(row["note"]),
			Current:   isCurrent(asString(row["is_current"])),
			CreatedAt: asTime(row["created_at"]),
			CreatedBy: actorFrom(row["created_by"]),
		})
	}
	return versions, nil
}

func (s *versionStore) Get(ctx context.Context, seq int) (*Version, error) {
	rows, err := s.query(ctx, queryGetVersion, seq, deployment.Resolve(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to read the version: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &Version{
		Seq:       asInt(rows[0]["seq"]),
		Hash:      asString(rows[0]["hash"]),
		Name:      asString(rows[0]["name"]),
		Resources: asString(rows[0]["resources"]),
		variables: asString(rows[0]["variables"]),
		Note:      asString(rows[0]["note"]),
		CreatedAt: asTime(rows[0]["created_at"]),
		CreatedBy: actorFrom(rows[0]["created_by"]),
	}, nil
}

func (s *versionStore) Latest(ctx context.Context) (int, error) {
	rows, err := s.query(ctx, queryLatestVersion, deployment.Resolve(ctx))
	if err != nil {
		return 0, fmt.Errorf("failed to read the latest version: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return asInt(rows[0]["latest"]), nil
}

func (s *versionStore) Find(ctx context.Context, prefix string) ([]int, error) {
	rows, err := s.query(ctx, queryFindVersions, prefix+"%", deployment.Resolve(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to find the version: %w", err)
	}
	found := make([]int, 0, len(rows))
	for _, row := range rows {
		found = append(found, asInt(row["seq"]))
	}
	return found, nil
}

func (s *versionStore) Prune(ctx context.Context, through int) error {
	if through <= 0 {
		return nil
	}
	if err := s.execute(ctx, queryPruneVersions,
		through, deployment.Resolve(ctx), deployment.Resolve(ctx), deployment.Resolve(ctx)); err != nil {
		return fmt.Errorf("failed to remove old versions: %w", err)
	}
	return nil
}

func (s *versionStore) GetApplied(ctx context.Context, gatewayID string) (*AppliedVersion, error) {
	rows, err := s.query(ctx, queryGetAppliedVersion, gatewayID, deployment.Resolve(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to read what the gateway holds: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	appliedAt := asTime(rows[0]["applied_at"])
	return &AppliedVersion{
		GatewayID:       gatewayID,
		AppliedVersion:  asInt(rows[0]["applied_seq"]),
		PreviousVersion: asInt(rows[0]["previous_seq"]),
		Applied:         asString(rows[0]["applied_hash"]),
		Previous:        asString(rows[0]["previous_hash"]),
		AppliedAt:       &appliedAt,
		AppliedBy:       actorFrom(rows[0]["applied_by"]),
		heldValues:      asString(rows[0]["held_values"]),
	}, nil
}

func (s *versionStore) SetApplied(ctx context.Context, gatewayID string, applied int, hash string,
	previous, held int, by *Actor, values, token string) error {
	var previousSeq interface{}
	if previous > 0 {
		previousSeq = previous
	}
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}
	// A held version of 0 never matches a recorded one, since versions are numbered from 1, so a
	// first apply is refused when another apply recorded the gateway first.
	recorded, err := dbClient.ExecuteContext(ctx, queryUpsertAppliedVersion,
		gatewayID, applied, previousSeq, actorColumn(by), deployment.Resolve(ctx), applied, hash,
		deployment.Resolve(ctx), held, token, nullable(values))
	if err != nil {
		return fmt.Errorf("failed to record what the gateway holds: %w", err)
	}
	if recorded > 0 {
		return nil
	}
	kept, err := s.Get(ctx, applied)
	if err != nil {
		return err
	}
	if kept == nil || kept.Hash != hash {
		return errVersionRemoved
	}
	return errAppliedChanged
}

func (s *versionStore) DeleteApplied(ctx context.Context, gatewayID string) error {
	if err := s.execute(ctx, queryDeleteAppliedVersion, gatewayID, deployment.Resolve(ctx)); err != nil {
		return fmt.Errorf("failed to forget what the gateway held: %w", err)
	}
	return nil
}

func (s *versionStore) GetExcluded(ctx context.Context, gatewayID string) ([]excludedResource, error) {
	rows, err := s.query(ctx, queryGetExcluded, gatewayID, deployment.Resolve(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to read what the gateway leaves alone: %w", err)
	}
	excluded := make([]excludedResource, 0, len(rows))
	for _, row := range rows {
		key, _ := row["resource_key"].(string)
		resourceType, _ := row["resource_type"].(string)
		id, _ := row["resource_id"].(string)
		name, _ := row["resource_name"].(string)
		excluded = append(excluded, excludedResource{Key: key, Type: resourceType, ID: id, Name: name,
			Once: asBool(row["once"])})
	}
	return excluded, nil
}

// SetExcluded writes every key in one transaction, so a failure leaves the choice as it was.
func (s *versionStore) SetExcluded(ctx context.Context, gatewayID string, exclude []excludedResource,
	include []string) error {
	transactioner, err := s.dbProvider.GetConfigDBTransactioner()
	if err != nil {
		return fmt.Errorf("failed to get database transactioner: %w", err)
	}
	return transactioner.Transact(ctx, func(txCtx context.Context) error {
		for _, key := range include {
			if err := s.execute(txCtx, queryInclude, gatewayID, key, deployment.Resolve(txCtx)); err != nil {
				return fmt.Errorf("failed to set the gateway to take a resource again: %w", err)
			}
		}
		for _, resource := range exclude {
			if err := s.execute(txCtx, queryExclude, gatewayID, resource.Key, resource.Type,
				nullable(resource.ID), nullable(resource.Name), deployment.Resolve(txCtx), resource.Once); err != nil {
				return fmt.Errorf("failed to set the gateway to leave a resource alone: %w", err)
			}
		}
		return nil
	})
}

// nullable stores an empty string as NULL.
func nullable(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func (s *versionStore) DeleteExcluded(ctx context.Context, gatewayID string) error {
	if err := s.execute(ctx, queryDeleteExcluded, gatewayID, deployment.Resolve(ctx)); err != nil {
		return fmt.Errorf("failed to forget what the gateway left alone: %w", err)
	}
	return nil
}

// errVersionRemoved reports that the version being recorded was removed, or replaced as the current
// version, while it was applied.
var errVersionRemoved = errors.New("the version was removed while it was applied")

// errAppliedChanged reports that another apply recorded the gateway after this one read it.
var errAppliedChanged = errors.New("another apply to the gateway finished first")

// asInt reads an integer column, which PostgreSQL and SQLite both return as int64. A NULL reads as 0.
func asInt(v interface{}) int {
	number, _ := v.(int64)
	return int(number)
}
