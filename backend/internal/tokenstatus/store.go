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

package tokenstatus

import (
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// statusStoreInterface is the persistence seam for the Token Status List subsystem, backed by the
// operation database. It is unexported: only the subsystem's own service consumes it, and the seam
// exists so that service can be tested against a fake store.
type statusStoreInterface interface {
	// allocateIndex claims the next status index for the deployment, creating or rolling the active
	// list as needed, and returns the list id and the allocated index.
	allocateIndex(ctx context.Context) (listID string, idx int64, err error)
	// setStatus records (idempotently) the status of the token at (listID, idx), expiring at expiry.
	setStatus(ctx context.Context, listID string, idx int64, status byte, expiry time.Time) error
	// getStatus returns the stored status of (listID, idx); a missing entry means statusValid.
	getStatus(ctx context.Context, listID string, idx int64) (byte, error)
	// getList loads a list by id. found is false when no such list exists.
	getList(ctx context.Context, listID string) (record listRecord, found bool, err error)
	// listEntries returns every non-VALID entry of a list, for building the published bit array.
	listEntries(ctx context.Context, listID string) ([]entryRecord, error)
	// dropExpiredSealedLists deletes sealed lists whose SEALED_AT precedes sealedBefore.
	dropExpiredSealedLists(ctx context.Context, sealedBefore time.Time) (int64, error)
}

// statusStore implements statusStoreInterface against the operation database. capacity and bits are the
// parameters stamped into any list it creates; retention is how long a sealed list is kept before it is
// eligible for reaping (must exceed the longest token lifetime; a non-positive value disables reaping).
type statusStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
	capacity     int64
	bits         int
	retention    time.Duration
	logger       *log.Logger
}

// newStatusStore builds a statusStore bound to the operation database and the current deployment. An
// unsupported bits width or non-positive capacity falls back to the format-neutral defaults.
func newStatusStore(capacity int64, bits int, retention time.Duration) statusStoreInterface {
	if capacity <= 0 {
		capacity = defaultListCapacity
	}
	if !validBits(bits) {
		bits = 1
	}
	return &statusStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
		capacity:     capacity,
		bits:         bits,
		retention:    retention,
		logger:       log.GetLogger().With(log.String(log.LoggerKeyComponentName, "StatusListStore")),
	}
}

// allocateIndex hands out the next index on the deployment's active list. It reads the active list,
// seals-and-rolls it when full, creates one when none exists, and claims the index with a
// compare-and-swap bump (see queryBumpNextIdx). A lost CAS race simply retries within the attempt
// bound; the allocated index is the pre-bump counter value, so allocation is dense and gap-free per
// successful commit.
func (s *statusStore) allocateIndex(ctx context.Context) (string, int64, error) {
	dbClient, err := s.dbProvider.GetOperationDBClient()
	if err != nil {
		return "", 0, fmt.Errorf("failed to get operation database client: %w", err)
	}

	for attempt := 0; attempt < maxAllocationAttempts; attempt++ {
		rows, err := dbClient.QueryContext(ctx, querySelectActiveList, s.deploymentID)
		if err != nil {
			return "", 0, fmt.Errorf("error selecting active status list: %w", err)
		}

		if len(rows) == 0 {
			if err := s.createActiveList(ctx); err != nil {
				return "", 0, err
			}
			continue
		}

		listID, err := rowString(rows[0], "id")
		if err != nil {
			return "", 0, err
		}
		nextIdx, err := rowInt64(rows[0], "next_idx")
		if err != nil {
			return "", 0, err
		}
		capacity, err := rowInt64(rows[0], "capacity")
		if err != nil {
			return "", 0, err
		}

		if nextIdx >= capacity {
			if err := s.sealAndRoll(ctx, listID); err != nil {
				return "", 0, err
			}
			continue
		}

		affected, err := dbClient.ExecuteContext(ctx, queryBumpNextIdx, listID, nextIdx, s.deploymentID)
		if err != nil {
			return "", 0, fmt.Errorf("error allocating status index: %w", err)
		}
		if affected == 1 {
			return listID, nextIdx, nil
		}
		// affected == 0: another node claimed this index; retry with a fresh read.
	}

	return "", 0, errAllocationExhausted
}

// createActiveList inserts a fresh active list with a new opaque id, but only if the deployment has no
// active list (the INSERT is guarded by WHERE NOT EXISTS). A concurrent caller that loses the race
// inserts zero rows, which is not an error: the allocation loop re-selects and finds the existing list.
func (s *statusStore) createActiveList(ctx context.Context) error {
	dbClient, err := s.dbProvider.GetOperationDBClient()
	if err != nil {
		return fmt.Errorf("failed to get operation database client: %w", err)
	}

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		return fmt.Errorf("failed to generate status list id: %w", err)
	}

	if _, err := dbClient.ExecuteContext(ctx, queryInsertList,
		id, s.bits, s.capacity, time.Now().UTC(), s.deploymentID, s.deploymentID); err != nil {
		return fmt.Errorf("error creating status list: %w", err)
	}
	return nil
}

// sealAndRoll seals a full list and creates its successor. The seal is a compare-and-swap on STATE = 0:
// only the winner (one row affected) creates the successor, so N racing allocators produce exactly one
// new list rather than N. A loser affects zero rows and returns; the allocation loop then finds the
// winner's successor (or creates one if the winner has not yet). It also opportunistically reaps sealed
// lists whose retention has elapsed — a natural, low-frequency moment to run cleanup without a
// background job.
func (s *statusStore) sealAndRoll(ctx context.Context, listID string) error {
	dbClient, err := s.dbProvider.GetOperationDBClient()
	if err != nil {
		return fmt.Errorf("failed to get operation database client: %w", err)
	}

	affected, err := dbClient.ExecuteContext(ctx, querySealList,
		time.Now().UTC(), listID, s.deploymentID)
	if err != nil {
		return fmt.Errorf("error sealing status list: %w", err)
	}
	if affected == 0 {
		// Lost the seal race; the winner rolls the successor. Retry the allocation loop.
		return nil
	}

	s.reapExpiredSealedLists(ctx)
	return s.createActiveList(ctx)
}

// reapExpiredSealedLists drops sealed lists whose retention window has elapsed. Retention must exceed
// the longest token lifetime (a list is dropped only once every token it covers has expired), so it is
// derived from the token validities at the composition root. It is best-effort: a failure is ignored so
// cleanup never blocks index allocation, and the next rollover retries. A non-positive retention (the
// horizon was not configured) disables reaping to avoid dropping a list while live tokens reference it.
func (s *statusStore) reapExpiredSealedLists(ctx context.Context) {
	if s.retention <= 0 {
		return
	}
	if _, err := s.dropExpiredSealedLists(ctx, time.Now().UTC().Add(-s.retention)); err != nil {
		// Non-blocking: allocation must not fail because cleanup did. Log so a persistent cleanup
		// failure (and the operation-database growth it causes) is visible rather than silent.
		s.logger.Warn(ctx, "Failed to reap expired sealed status lists", log.Error(err))
	}
}

// setStatus records the status of a token by index. The write is idempotent on (deployment, list, idx).
func (s *statusStore) setStatus(
	ctx context.Context, listID string, idx int64, status byte, expiry time.Time,
) error {
	dbClient, err := s.dbProvider.GetOperationDBClient()
	if err != nil {
		return fmt.Errorf("failed to get operation database client: %w", err)
	}

	if _, err := dbClient.ExecuteContext(ctx, queryUpsertEntry,
		listID, idx, int(status), expiry.UTC(), time.Now().UTC(), s.deploymentID); err != nil {
		return fmt.Errorf("error writing status entry: %w", err)
	}
	return nil
}

// getStatus returns the stored status of a token. A missing entry means statusValid, because the sparse
// table stores only non-VALID indices.
func (s *statusStore) getStatus(ctx context.Context, listID string, idx int64) (byte, error) {
	dbClient, err := s.dbProvider.GetOperationDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get operation database client: %w", err)
	}

	rows, err := dbClient.QueryContext(ctx, queryGetEntryStatus, listID, idx, s.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("error reading status entry: %w", err)
	}
	if len(rows) == 0 {
		return statusValid, nil
	}

	status, err := rowInt64(rows[0], "status")
	if err != nil {
		return 0, err
	}
	return byte(status), nil
}

// getList loads a single list by id.
func (s *statusStore) getList(ctx context.Context, listID string) (listRecord, bool, error) {
	dbClient, err := s.dbProvider.GetOperationDBClient()
	if err != nil {
		return listRecord{}, false, fmt.Errorf("failed to get operation database client: %w", err)
	}

	rows, err := dbClient.QueryContext(ctx, queryGetList, listID, s.deploymentID)
	if err != nil {
		return listRecord{}, false, fmt.Errorf("error loading status list: %w", err)
	}
	if len(rows) == 0 {
		return listRecord{}, false, nil
	}

	rec, err := parseListRecord(rows[0])
	if err != nil {
		return listRecord{}, false, err
	}
	return rec, true, nil
}

// listEntries returns every stored (non-VALID) entry of a list.
func (s *statusStore) listEntries(ctx context.Context, listID string) ([]entryRecord, error) {
	dbClient, err := s.dbProvider.GetOperationDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get operation database client: %w", err)
	}

	rows, err := dbClient.QueryContext(ctx, queryListEntries, listID, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("error listing status entries: %w", err)
	}

	entries := make([]entryRecord, 0, len(rows))
	for _, row := range rows {
		idx, err := rowInt64(row, "idx")
		if err != nil {
			return nil, err
		}
		status, err := rowInt64(row, "status")
		if err != nil {
			return nil, err
		}
		entries = append(entries, entryRecord{idx: idx, status: byte(status)})
	}
	return entries, nil
}

// dropExpiredSealedLists deletes sealed lists whose retention has elapsed and returns the count removed.
func (s *statusStore) dropExpiredSealedLists(ctx context.Context, sealedBefore time.Time) (int64, error) {
	dbClient, err := s.dbProvider.GetOperationDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get operation database client: %w", err)
	}

	affected, err := dbClient.ExecuteContext(ctx, queryDropExpiredSealedLists, sealedBefore.UTC(), s.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("error dropping expired status lists: %w", err)
	}
	return affected, nil
}

// parseListRecord maps a result row to a listRecord, treating a NULL SEALED_AT as the zero time.
func parseListRecord(row map[string]interface{}) (listRecord, error) {
	id, err := rowString(row, "id")
	if err != nil {
		return listRecord{}, err
	}
	bits, err := rowInt64(row, "bits")
	if err != nil {
		return listRecord{}, err
	}
	state, err := rowInt64(row, "state")
	if err != nil {
		return listRecord{}, err
	}
	nextIdx, err := rowInt64(row, "next_idx")
	if err != nil {
		return listRecord{}, err
	}
	capacity, err := rowInt64(row, "capacity")
	if err != nil {
		return listRecord{}, err
	}
	createdAt, err := utils.ParseDBTimeField(row["created_at"], "created_at")
	if err != nil {
		return listRecord{}, err
	}

	rec := listRecord{
		id:        id,
		bits:      int(bits),
		state:     int(state),
		nextIdx:   nextIdx,
		capacity:  capacity,
		createdAt: createdAt,
	}
	if row["sealed_at"] != nil {
		sealedAt, err := utils.ParseDBTimeField(row["sealed_at"], "sealed_at")
		if err != nil {
			return listRecord{}, err
		}
		rec.sealedAt = sealedAt
	}
	return rec, nil
}

// rowString reads a string column from a result row.
func rowString(row map[string]interface{}, key string) (string, error) {
	switch v := row[key].(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return "", fmt.Errorf("tokenstatus: column %q is not a string (%T)", key, row[key])
	}
}

// rowInt64 reads an integer column from a result row, tolerating the numeric types the supported
// drivers return (int64, int, or float64).
func rowInt64(row map[string]interface{}, key string) (int64, error) {
	switch v := row[key].(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("tokenstatus: column %q is not an integer (%T)", key, row[key])
	}
}
