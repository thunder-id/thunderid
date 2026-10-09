// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	governancemodel "github.com/thunder-id/thunderid/internal/identitygovernance/model"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// entityRuntimeStoreInterface persists the runtime state of entities: the lifecycle state and the
// access-state document. It uses the runtime persistent database, so database and declarative
// entities share it.
type entityRuntimeStoreInterface interface {
	GetRuntimeData(ctx context.Context, ids []string) (map[string]entityRuntimeData, error)
	InitializeRuntimeData(ctx context.Context, data []entityRuntimeData) error
	ResetRuntimeData(ctx context.Context, entityID string, state providers.EntityState) error
	DeleteRuntimeData(ctx context.Context, entityID string) error

	IncrementAccessFailure(ctx context.Context, entityID string, scope governancemodel.AccessScope,
		now, windowStart time.Time) (governancemodel.ScopeLock, bool, error)
	FormAccessLock(ctx context.Context, entityID string, scope governancemodel.AccessScope,
		observed governancemodel.ScopeLock,
		episode governancemodel.LockEpisode, now time.Time) (bool, error)
	ClearAccessState(ctx context.Context, entityID string, scopes []governancemodel.AccessScope) error
	ClearAccessStateIfUnchanged(ctx context.Context, entityID string, scope governancemodel.AccessScope,
		revision int64) (bool, error)
	SetAccessSuspension(ctx context.Context, entityID string, at time.Time, operatorNote string) error
	RecordAccessLogin(ctx context.Context, entityID string, at, notBefore time.Time) error
	ClearAccessSuspension(ctx context.Context, entityID string, hold *governancemodel.EntityHold,
		suspendedAt string) error
}

// entityRuntimeDBStore is the runtime persistent database implementation of
// entityRuntimeStoreInterface.
type entityRuntimeDBStore struct {
	dbProvider provider.DBProviderInterface
}

// newEntityRuntimeDBStore creates the runtime store over the runtime persistent database.
func newEntityRuntimeDBStore() entityRuntimeStoreInterface {
	return &entityRuntimeDBStore{dbProvider: getDBProvider()}
}

// GetRuntimeData reads the runtime rows of the given entities, in batches. An entity with no row is
// absent from the result.
func (rs *entityRuntimeDBStore) GetRuntimeData(ctx context.Context,
	ids []string) (map[string]entityRuntimeData, error) {
	result := make(map[string]entityRuntimeData, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	client, err := rs.client()
	if err != nil {
		return nil, err
	}

	for start := 0; start < len(ids); start += runtimeBatchSize {
		end := min(start+runtimeBatchSize, len(ids))
		query, args := buildGetRuntimeDataQuery(ids[start:end], deployment.Resolve(ctx))

		rows, err := client.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("read entity runtime data: %w", err)
		}

		for _, row := range rows {
			record, err := buildRuntimeDataFromRow(row)
			if err != nil {
				return nil, err
			}
			result[record.ID] = record
		}
	}

	return result, nil
}

// InitializeRuntimeData seeds missing runtime rows and never replaces an existing one.
func (rs *entityRuntimeDBStore) InitializeRuntimeData(ctx context.Context, data []entityRuntimeData) error {
	if len(data) == 0 {
		return nil
	}
	for _, row := range data {
		if row.ID == "" {
			return ErrEntityNotFound
		}
	}

	client, err := rs.client()
	if err != nil {
		return err
	}

	for start := 0; start < len(data); start += runtimeBatchSize {
		end := min(start+runtimeBatchSize, len(data))
		query, args := buildInitializeRuntimeDataQuery(data[start:end], time.Now().UTC(), deployment.Resolve(ctx))

		if _, err := client.ExecuteContext(ctx, query, args...); err != nil {
			return fmt.Errorf("initialize entity runtime data: %w", err)
		}
	}

	return nil
}

// ResetRuntimeData writes the empty runtime row a newly created entity starts with, replacing any
// orphan row at the same id.
func (rs *entityRuntimeDBStore) ResetRuntimeData(ctx context.Context, entityID string,
	state providers.EntityState) error {
	if state == "" {
		state = providers.EntityStateActive
	}

	client, err := rs.client()
	if err != nil {
		return err
	}

	if _, err := client.ExecuteContext(ctx, queryResetRuntimeData,
		entityID, string(state), time.Now().UTC(), deployment.Resolve(ctx)); err != nil {
		return fmt.Errorf("create entity runtime data: %w", err)
	}
	return nil
}

// DeleteRuntimeData removes an entity's runtime row. A missing row is not an error.
func (rs *entityRuntimeDBStore) DeleteRuntimeData(ctx context.Context, entityID string) error {
	client, err := rs.client()
	if err != nil {
		return err
	}

	if _, err := client.ExecuteContext(ctx, queryDeleteRuntimeData, entityID, deployment.Resolve(ctx)); err != nil {
		return fmt.Errorf("delete entity runtime data: %w", err)
	}
	return nil
}

// IncrementAccessFailure records one failed attempt against scope and returns the resulting counters.
// The second return is false when nothing was written (no runtime row, or a live lock). It runs in
// a transaction so a client retry cannot count the failure twice.
func (rs *entityRuntimeDBStore) IncrementAccessFailure(ctx context.Context, entityID string,
	scope governancemodel.AccessScope, now, windowStart time.Time) (governancemodel.ScopeLock, bool, error) {
	query, err := buildIncrementAccessFailureQuery(scope)
	if err != nil {
		return governancemodel.ScopeLock{}, false, err
	}

	dbClient, err := rs.client()
	if err != nil {
		return governancemodel.ScopeLock{}, false, fmt.Errorf("failed to get database client: %w", err)
	}

	transactioner, err := rs.dbProvider.GetRuntimePersistentDBTransactioner()
	if err != nil {
		return governancemodel.ScopeLock{}, false, fmt.Errorf("failed to get transactioner: %w", err)
	}

	nowText := formatAccessTime(now)
	var results []map[string]interface{}
	err = transactioner.Transact(ctx, func(txCtx context.Context) error {
		var queryErr error
		results, queryErr = dbClient.QueryContext(txCtx, query,
			entityID, formatAccessTime(windowStart), nowText, nowText, now, deployment.Resolve(ctx))
		return queryErr
	})
	if err != nil {
		return governancemodel.ScopeLock{}, false,
			fmt.Errorf("failed to execute increment access failure query: %w", err)
	}
	if len(results) == 0 {
		return governancemodel.ScopeLock{}, false, nil
	}

	snapshot, err := buildAccessStateSnapshotFromRow(results[0])
	if err != nil {
		return governancemodel.ScopeLock{}, false, err
	}
	return snapshot, true, nil
}

// FormAccessLock opens a lock episode on scope, guarded on the counters the increment observed. It
// returns false, without an error, when the row changed since then.
func (rs *entityRuntimeDBStore) FormAccessLock(ctx context.Context, entityID string,
	scope governancemodel.AccessScope, observed governancemodel.ScopeLock, episode governancemodel.LockEpisode,
	now time.Time) (bool, error) {
	query, err := buildFormAccessLockQuery(scope)
	if err != nil {
		return false, err
	}

	dbClient, err := rs.client()
	if err != nil {
		return false, fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, query,
		entityID, formatAccessTime(now), episode.LockCount, episode.UnlockAt,
		observed.FailureCount, observed.LockCount, now, observed.Revision, deployment.Resolve(ctx))
	if err != nil {
		return false, fmt.Errorf("failed to execute form access lock query: %w", err)
	}

	return rowsAffected > 0, nil
}

// ClearAccessState removes the lock entries of the given scopes. A missing row is not an error.
func (rs *entityRuntimeDBStore) ClearAccessState(ctx context.Context, entityID string,
	scopes []governancemodel.AccessScope) error {
	_, err := rs.clearAccessState(ctx, entityID, scopes, nil)
	return err
}

// ClearAccessStateIfUnchanged removes one scope's lock entry only if the row still has the given
// revision, so state written after the caller's read is kept. It reports whether the entry was
// removed.
func (rs *entityRuntimeDBStore) ClearAccessStateIfUnchanged(ctx context.Context, entityID string,
	scope governancemodel.AccessScope, revision int64) (bool, error) {
	return rs.clearAccessState(ctx, entityID, []governancemodel.AccessScope{scope}, &revision)
}

// SetAccessSuspension sets the state to SUSPENDED and records when and with what operator note.
// The note may be empty.
func (rs *entityRuntimeDBStore) SetAccessSuspension(ctx context.Context, entityID string,
	at time.Time, operatorNote string) error {
	dbClient, err := rs.client()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, querySetAccessSuspension,
		entityID, at.UTC().Format(time.RFC3339Nano), operatorNote, time.Now().UTC(), deployment.Resolve(ctx))
	if err != nil {
		return fmt.Errorf("failed to execute set access suspension query: %w", err)
	}
	if rowsAffected == 0 {
		return ErrEntityNotFound
	}
	return nil
}

// RecordAccessLogin stores when access was granted, unless the stored value is already within
// [notBefore, at]. A missing row or a skipped write is not an error.
func (rs *entityRuntimeDBStore) RecordAccessLogin(ctx context.Context, entityID string,
	at, notBefore time.Time) error {
	dbClient, err := rs.client()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	if _, err := dbClient.ExecuteContext(ctx, queryRecordAccessLogin, entityID,
		formatAccessTime(at), formatAccessTime(notBefore), time.Now().UTC(),
		deployment.Resolve(ctx)); err != nil {
		return fmt.Errorf("failed to execute record access login query: %w", err)
	}
	return nil
}

// ClearAccessSuspension removes the suspension if it is still the one placed at suspendedAt. When
// hold is set, the same statement replaces the lock member with that entity-wide hold.
func (rs *entityRuntimeDBStore) ClearAccessSuspension(ctx context.Context, entityID string,
	hold *governancemodel.EntityHold, suspendedAt string) error {
	dbClient, err := rs.client()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	var rowsAffected int64
	if hold == nil {
		rowsAffected, err = dbClient.ExecuteContext(ctx, queryClearAccessSuspension,
			entityID, time.Now().UTC(), suspendedAt, deployment.Resolve(ctx))
	} else {
		rowsAffected, err = dbClient.ExecuteContext(ctx, queryClearAccessSuspensionWithHold,
			entityID, hold.UnlockAt, hold.Reason, time.Now().UTC(), suspendedAt, deployment.Resolve(ctx))
	}
	if err != nil {
		return fmt.Errorf("failed to execute clear access suspension query: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("suspension changed or entity not found: %w", ErrEntityNotFound)
	}
	return nil
}

// client returns the runtime persistent database client.
func (rs *entityRuntimeDBStore) client() (provider.DBClientInterface, error) {
	return rs.dbProvider.GetRuntimePersistentDBClient()
}

// clearAccessState removes the lock entries of the given scopes, guarded on revision when it is set.
// It reports whether a row was updated.
func (rs *entityRuntimeDBStore) clearAccessState(ctx context.Context, entityID string,
	scopes []governancemodel.AccessScope, revision *int64) (bool, error) {
	if len(scopes) == 0 {
		return false, nil
	}
	query, args, err := buildClearAccessStateQuery(entityID, scopes, time.Now().UTC(), revision,
		deployment.Resolve(ctx))
	if err != nil {
		return false, err
	}

	dbClient, err := rs.client()
	if err != nil {
		return false, fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("failed to execute clear access state query: %w", err)
	}
	return rowsAffected > 0, nil
}

// buildRuntimeDataFromRow reads one runtime row. The attributes must be a JSON object.
func buildRuntimeDataFromRow(row map[string]interface{}) (entityRuntimeData, error) {
	id, ok := row["entity_id"].(string)
	if !ok {
		return entityRuntimeData{}, fmt.Errorf("invalid runtime entity id")
	}
	state, ok := row["state"].(string)
	if !ok {
		return entityRuntimeData{}, fmt.Errorf("invalid runtime state for entity")
	}

	attrs := parseJSONColumn(row, "runtime_attributes")
	if !json.Valid(attrs) || len(attrs) == 0 || strings.TrimSpace(string(attrs))[0] != '{' {
		return entityRuntimeData{}, fmt.Errorf("invalid runtime attributes for entity")
	}

	revision, err := coerceAccessCount(row["revision"])
	if err != nil {
		return entityRuntimeData{}, err
	}

	return entityRuntimeData{ID: id, State: providers.EntityState(state), Attributes: attrs,
		Revision: int64(revision)}, nil
}

// formatAccessTime formats an instant as fixed-width RFC 3339 UTC, which SQLite compares as text.
func formatAccessTime(at time.Time) string {
	return at.UTC().Format(time.RFC3339)
}

// buildAccessStateSnapshotFromRow reads the counters and revision a lock statement returns.
func buildAccessStateSnapshotFromRow(row map[string]interface{}) (governancemodel.ScopeLock, error) {
	failureCount, err := coerceAccessCount(row["failure_count"])
	if err != nil {
		return governancemodel.ScopeLock{}, fmt.Errorf("failed to read failureCount: %w", err)
	}
	lockCount, err := coerceAccessCount(row["lock_count"])
	if err != nil {
		return governancemodel.ScopeLock{}, fmt.Errorf("failed to read lockCount: %w", err)
	}
	revision, err := coerceAccessCount(row["revision"])
	if err != nil {
		return governancemodel.ScopeLock{}, fmt.Errorf("failed to read runtime revision: %w", err)
	}
	unlockAt, _ := row["unlock_at"].(string)
	return governancemodel.ScopeLock{FailureCount: failureCount, LockCount: lockCount,
		UnlockAt: unlockAt, Revision: int64(revision)}, nil
}

// coerceAccessCount converts a count column to int. The drivers return different types for it.
func coerceAccessCount(value interface{}) (int, error) {
	switch v := value.(type) {
	case nil:
		return 0, nil
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case []byte:
		return strconv.Atoi(string(v))
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("unsupported count type %T", value)
	}
}
