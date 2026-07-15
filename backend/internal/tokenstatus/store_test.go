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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment"

type StatusStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *statusStore
}

func TestStatusStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StatusStoreTestSuite))
}

func (s *StatusStoreTestSuite) SetupTest() {
	_ = config.InitializeServerRuntime("test", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: testDeploymentID},
	})

	s.mockDBProvider = providermock.NewDBProviderInterfaceMock(s.T())
	s.mockDBClient = providermock.NewDBClientInterfaceMock(s.T())
	s.store = &statusStore{
		dbProvider:   s.mockDBProvider,
		deploymentID: testDeploymentID,
		capacity:     100,
		bits:         1,
		logger:       log.GetLogger().With(log.String(log.LoggerKeyComponentName, "StatusListStore")),
	}
}

// expectClient wires GetOperationDBClient to return the mock client for any number of calls, since a
// single store method may fetch it more than once (e.g. allocate → create → seal).
func (s *StatusStoreTestSuite) expectClient() {
	s.mockDBProvider.EXPECT().GetOperationDBClient().Return(s.mockDBClient, nil)
}

// capacity is kept explicit at call sites so the seal-trigger cases (nextIdx == capacity) read clearly.
func activeListRow(id string, nextIdx, capacity int64) []map[string]interface{} { //nolint:unparam
	return []map[string]interface{}{{"id": id, "next_idx": nextIdx, "capacity": capacity}}
}

func (s *StatusStoreTestSuite) TestAllocateIndexHappyPath() {
	s.expectClient()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
		Return(activeListRow("list-1", 5, 100), nil).Once()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryBumpNextIdx, "list-1", int64(5), testDeploymentID).
		Return(int64(1), nil).Once()

	listID, idx, err := s.store.allocateIndex(context.Background())

	s.NoError(err)
	s.Equal("list-1", listID)
	s.Equal(int64(5), idx)
}

func (s *StatusStoreTestSuite) TestAllocateIndexRetriesOnLostCAS() {
	s.expectClient()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
		Return(activeListRow("list-1", 5, 100), nil).Once()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryBumpNextIdx, "list-1", int64(5), testDeploymentID).
		Return(int64(0), nil).Once()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
		Return(activeListRow("list-1", 6, 100), nil).Once()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryBumpNextIdx, "list-1", int64(6), testDeploymentID).
		Return(int64(1), nil).Once()

	listID, idx, err := s.store.allocateIndex(context.Background())

	s.NoError(err)
	s.Equal("list-1", listID)
	s.Equal(int64(6), idx)
}

func (s *StatusStoreTestSuite) TestAllocateIndexCreatesListWhenNone() {
	s.expectClient()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryInsertList,
		mock.Anything, 1, int64(100), mock.Anything, testDeploymentID, testDeploymentID).
		Return(int64(1), nil).Once()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
		Return(activeListRow("list-new", 0, 100), nil).Once()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryBumpNextIdx, "list-new", int64(0), testDeploymentID).
		Return(int64(1), nil).Once()

	listID, idx, err := s.store.allocateIndex(context.Background())

	s.NoError(err)
	s.Equal("list-new", listID)
	s.Equal(int64(0), idx)
}

func (s *StatusStoreTestSuite) TestAllocateIndexSealsAndRollsWhenFull() {
	s.expectClient()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
		Return(activeListRow("list-full", 100, 100), nil).Once()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, querySealList,
		mock.Anything, "list-full", testDeploymentID).
		Return(int64(1), nil).Once()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryInsertList,
		mock.Anything, 1, int64(100), mock.Anything, testDeploymentID, testDeploymentID).
		Return(int64(1), nil).Once()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
		Return(activeListRow("list-2", 0, 100), nil).Once()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryBumpNextIdx, "list-2", int64(0), testDeploymentID).
		Return(int64(1), nil).Once()

	listID, idx, err := s.store.allocateIndex(context.Background())

	s.NoError(err)
	s.Equal("list-2", listID)
	s.Equal(int64(0), idx)
}

// When a retention window is configured, sealing a full list opportunistically reaps expired sealed
// lists (the seal winner runs the drop before rolling the successor).
func (s *StatusStoreTestSuite) TestAllocateIndexReapsOnSealWhenRetentionSet() {
	s.store.retention = 48 * time.Hour
	s.expectClient()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
		Return(activeListRow("list-full", 100, 100), nil).Once()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, querySealList,
		mock.Anything, "list-full", testDeploymentID).
		Return(int64(1), nil).Once()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryDropExpiredSealedLists,
		mock.Anything, testDeploymentID).
		Return(int64(2), nil).Once()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryInsertList,
		mock.Anything, 1, int64(100), mock.Anything, testDeploymentID, testDeploymentID).
		Return(int64(1), nil).Once()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
		Return(activeListRow("list-2", 0, 100), nil).Once()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryBumpNextIdx, "list-2", int64(0), testDeploymentID).
		Return(int64(1), nil).Once()

	_, _, err := s.store.allocateIndex(context.Background())

	s.NoError(err)
}

// A caller that loses the seal race (zero rows affected) must not create a successor; it re-selects
// and allocates from the winner's already-rolled active list.
func (s *StatusStoreTestSuite) TestAllocateIndexSealLostRaceDoesNotCreate() {
	s.expectClient()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
		Return(activeListRow("list-full", 100, 100), nil).Once()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, querySealList,
		mock.Anything, "list-full", testDeploymentID).
		Return(int64(0), nil).Once()
	// No queryInsertList expectation: the loser must not create a successor.
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
		Return(activeListRow("list-2", 0, 100), nil).Once()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryBumpNextIdx, "list-2", int64(0), testDeploymentID).
		Return(int64(1), nil).Once()

	listID, idx, err := s.store.allocateIndex(context.Background())

	s.NoError(err)
	s.Equal("list-2", listID)
	s.Equal(int64(0), idx)
}

func (s *StatusStoreTestSuite) TestAllocateIndexExhaustsRetries() {
	s.expectClient()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
		Return(activeListRow("list-1", 5, 100), nil)
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryBumpNextIdx, "list-1", int64(5), testDeploymentID).
		Return(int64(0), nil)

	_, _, err := s.store.allocateIndex(context.Background())

	s.ErrorIs(err, errAllocationExhausted)
}

func (s *StatusStoreTestSuite) TestAllocateIndexDBClientError() {
	s.mockDBProvider.EXPECT().GetOperationDBClient().Return(nil, errors.New("connection error"))

	_, _, err := s.store.allocateIndex(context.Background())

	s.Error(err)
	s.Contains(err.Error(), "operation database client")
}

func (s *StatusStoreTestSuite) TestSetStatus() {
	expiry := time.Now().Add(time.Hour)
	s.expectClient()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryUpsertEntry,
		"list-1", int64(7), int(statusInvalid), mock.Anything, mock.Anything, testDeploymentID).
		Return(int64(1), nil).Once()

	err := s.store.setStatus(context.Background(), "list-1", 7, statusInvalid, expiry)

	s.NoError(err)
}

func (s *StatusStoreTestSuite) TestGetStatusRevoked() {
	s.expectClient()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetEntryStatus, "list-1", int64(7), testDeploymentID).
		Return([]map[string]interface{}{{"status": int64(statusInvalid)}}, nil).Once()

	status, err := s.store.getStatus(context.Background(), "list-1", 7)

	s.NoError(err)
	s.Equal(statusInvalid, status)
}

func (s *StatusStoreTestSuite) TestGetStatusValidWhenAbsent() {
	s.expectClient()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetEntryStatus, "list-1", int64(7), testDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()

	status, err := s.store.getStatus(context.Background(), "list-1", 7)

	s.NoError(err)
	s.Equal(statusValid, status)
}

func (s *StatusStoreTestSuite) TestGetList() {
	created := time.Now().UTC()
	s.expectClient()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetList, "list-1", testDeploymentID).
		Return([]map[string]interface{}{{
			"id": "list-1", "bits": int64(1), "state": int64(listStateActive),
			"next_idx": int64(5), "capacity": int64(100), "created_at": created, "sealed_at": nil,
		}}, nil).Once()

	rec, found, err := s.store.getList(context.Background(), "list-1")

	s.NoError(err)
	s.True(found)
	s.Equal("list-1", rec.id)
	s.Equal(1, rec.bits)
	s.Equal(listStateActive, rec.state)
	s.Equal(int64(5), rec.nextIdx)
	s.Equal(int64(100), rec.capacity)
	s.True(rec.sealedAt.IsZero())
}

func (s *StatusStoreTestSuite) TestGetListNotFound() {
	s.expectClient()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetList, "missing", testDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()

	_, found, err := s.store.getList(context.Background(), "missing")

	s.NoError(err)
	s.False(found)
}

func (s *StatusStoreTestSuite) TestListEntries() {
	s.expectClient()
	s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryListEntries, "list-1", testDeploymentID).
		Return([]map[string]interface{}{
			{"idx": int64(3), "status": int64(statusInvalid)},
			{"idx": int64(9), "status": int64(statusInvalid)},
		}, nil).Once()

	entries, err := s.store.listEntries(context.Background(), "list-1")

	s.NoError(err)
	s.Len(entries, 2)
	s.Equal(int64(3), entries[0].idx)
	s.Equal(statusInvalid, entries[0].status)
	s.Equal(int64(9), entries[1].idx)
}

func (s *StatusStoreTestSuite) TestDropExpiredSealedLists() {
	s.expectClient()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryDropExpiredSealedLists,
		mock.Anything, testDeploymentID).
		Return(int64(2), nil).Once()

	dropped, err := s.store.dropExpiredSealedLists(context.Background(), time.Now())

	s.NoError(err)
	s.Equal(int64(2), dropped)
}

func (s *StatusStoreTestSuite) TestReapExpiredSealedListsSkippedWhenRetentionDisabled() {
	// A non-positive retention disables reaping; the drop query must never run.
	s.store.retention = 0
	s.store.reapExpiredSealedLists(context.Background())
}

func (s *StatusStoreTestSuite) TestReapExpiredSealedListsLogsOnError() {
	// A drop failure must be logged, not propagated: reaping is best-effort so it never blocks
	// allocation, and the next rollover retries.
	s.store.retention = time.Hour
	s.expectClient()
	s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryDropExpiredSealedLists,
		mock.Anything, testDeploymentID).
		Return(int64(0), errors.New("db down")).Once()

	s.store.reapExpiredSealedLists(context.Background())
}

// expectClientError wires GetOperationDBClient to fail, so a store method's database-acquisition guard
// can be exercised.
func (s *StatusStoreTestSuite) expectClientError() {
	s.mockDBProvider.EXPECT().GetOperationDBClient().Return(nil, errors.New("no db"))
}

func (s *StatusStoreTestSuite) TestStoreMethodsFailWhenClientUnavailable() {
	ctx := context.Background()
	s.Run("allocateIndex", func() {
		s.expectClientError()
		_, _, err := s.store.allocateIndex(ctx)
		s.Error(err)
	})
	s.Run("createActiveList", func() {
		s.expectClientError()
		s.Error(s.store.createActiveList(ctx))
	})
	s.Run("sealAndRoll", func() {
		s.expectClientError()
		s.Error(s.store.sealAndRoll(ctx, "list-1"))
	})
	s.Run("setStatus", func() {
		s.expectClientError()
		s.Error(s.store.setStatus(ctx, "list-1", 1, statusInvalid, time.Now()))
	})
	s.Run("getStatus", func() {
		s.expectClientError()
		_, err := s.store.getStatus(ctx, "list-1", 1)
		s.Error(err)
	})
	s.Run("getList", func() {
		s.expectClientError()
		_, _, err := s.store.getList(ctx, "list-1")
		s.Error(err)
	})
	s.Run("listEntries", func() {
		s.expectClientError()
		_, err := s.store.listEntries(ctx, "list-1")
		s.Error(err)
	})
	s.Run("dropExpiredSealedLists", func() {
		s.expectClientError()
		_, err := s.store.dropExpiredSealedLists(ctx, time.Now())
		s.Error(err)
	})
}

func (s *StatusStoreTestSuite) TestStoreMethodsPropagateQueryErrors() {
	ctx := context.Background()
	qErr := errors.New("query failed")
	s.Run("allocateIndex select", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
			Return(nil, qErr).Once()
		_, _, err := s.store.allocateIndex(ctx)
		s.Error(err)
	})
	s.Run("allocateIndex bump", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
			Return(activeListRow("list-1", 5, 100), nil).Once()
		s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryBumpNextIdx, "list-1", int64(5), testDeploymentID).
			Return(int64(0), qErr).Once()
		_, _, err := s.store.allocateIndex(ctx)
		s.Error(err)
	})
	s.Run("createActiveList insert", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryInsertList,
			mock.Anything, 1, int64(100), mock.Anything, testDeploymentID, testDeploymentID).
			Return(int64(0), qErr).Once()
		s.Error(s.store.createActiveList(ctx))
	})
	s.Run("allocateIndex create-list failure", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
			Return([]map[string]interface{}{}, nil).Once()
		s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryInsertList,
			mock.Anything, 1, int64(100), mock.Anything, testDeploymentID, testDeploymentID).
			Return(int64(0), qErr).Once()
		_, _, err := s.store.allocateIndex(ctx)
		s.Error(err)
	})
	s.Run("allocateIndex seal failure", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
			Return(activeListRow("list-full", 100, 100), nil).Once()
		s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, querySealList,
			mock.Anything, "list-full", testDeploymentID).
			Return(int64(0), qErr).Once()
		_, _, err := s.store.allocateIndex(ctx)
		s.Error(err)
	})
	s.Run("sealAndRoll seal", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, querySealList,
			mock.Anything, "list-1", testDeploymentID).
			Return(int64(0), qErr).Once()
		s.Error(s.store.sealAndRoll(ctx, "list-1"))
	})
	s.Run("setStatus upsert", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryUpsertEntry,
			"list-1", int64(1), int(statusInvalid), mock.Anything, mock.Anything, testDeploymentID).
			Return(int64(0), qErr).Once()
		s.Error(s.store.setStatus(ctx, "list-1", 1, statusInvalid, time.Now()))
	})
	s.Run("getStatus query", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetEntryStatus, "list-1", int64(1), testDeploymentID).
			Return(nil, qErr).Once()
		_, err := s.store.getStatus(ctx, "list-1", 1)
		s.Error(err)
	})
	s.Run("getList query", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetList, "list-1", testDeploymentID).
			Return(nil, qErr).Once()
		_, _, err := s.store.getList(ctx, "list-1")
		s.Error(err)
	})
	s.Run("listEntries query", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryListEntries, "list-1", testDeploymentID).
			Return(nil, qErr).Once()
		_, err := s.store.listEntries(ctx, "list-1")
		s.Error(err)
	})
	s.Run("dropExpiredSealedLists execute", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().ExecuteContext(mock.Anything, queryDropExpiredSealedLists,
			mock.Anything, testDeploymentID).
			Return(int64(0), qErr).Once()
		_, err := s.store.dropExpiredSealedLists(ctx, time.Now())
		s.Error(err)
	})
}

func (s *StatusStoreTestSuite) TestStoreMethodsRejectMalformedRows() {
	ctx := context.Background()
	s.Run("allocateIndex bad id column", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
			Return([]map[string]interface{}{{"id": 123, "next_idx": int64(0), "capacity": int64(100)}}, nil).Once()
		_, _, err := s.store.allocateIndex(ctx)
		s.Error(err)
	})
	s.Run("allocateIndex bad next_idx column", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
			Return([]map[string]interface{}{{"id": "l", "next_idx": "nope", "capacity": int64(100)}}, nil).Once()
		_, _, err := s.store.allocateIndex(ctx)
		s.Error(err)
	})
	s.Run("allocateIndex bad capacity column", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().QueryContext(mock.Anything, querySelectActiveList, testDeploymentID).
			Return([]map[string]interface{}{{"id": "l", "next_idx": int64(0), "capacity": "nope"}}, nil).Once()
		_, _, err := s.store.allocateIndex(ctx)
		s.Error(err)
	})
	s.Run("listEntries bad status column", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryListEntries, "list-1", testDeploymentID).
			Return([]map[string]interface{}{{"idx": int64(1), "status": "nope"}}, nil).Once()
		_, err := s.store.listEntries(ctx, "list-1")
		s.Error(err)
	})
	s.Run("getStatus bad status column", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetEntryStatus, "list-1", int64(1), testDeploymentID).
			Return([]map[string]interface{}{{"status": "nope"}}, nil).Once()
		_, err := s.store.getStatus(ctx, "list-1", 1)
		s.Error(err)
	})
	s.Run("getList bad row", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryGetList, "list-1", testDeploymentID).
			Return([]map[string]interface{}{{"id": 123}}, nil).Once()
		_, _, err := s.store.getList(ctx, "list-1")
		s.Error(err)
	})
	s.Run("listEntries bad row", func() {
		s.expectClient()
		s.mockDBClient.EXPECT().QueryContext(mock.Anything, queryListEntries, "list-1", testDeploymentID).
			Return([]map[string]interface{}{{"idx": "nope"}}, nil).Once()
		_, err := s.store.listEntries(ctx, "list-1")
		s.Error(err)
	})
}

func (s *StatusStoreTestSuite) TestNewStatusStoreFallsBackOnInvalidArgs() {
	// A non-positive capacity and an unsupported bit width fall back to the format-neutral defaults.
	st := newStatusStore(0, 3, 0).(*statusStore)
	s.Equal(defaultListCapacity, st.capacity)
	s.Equal(1, st.bits)
}

func (s *StatusStoreTestSuite) TestRowString() {
	got, err := rowString(map[string]interface{}{"k": "v"}, "k")
	s.NoError(err)
	s.Equal("v", got)

	got, err = rowString(map[string]interface{}{"k": []byte("v")}, "k")
	s.NoError(err)
	s.Equal("v", got)

	_, err = rowString(map[string]interface{}{"k": 123}, "k")
	s.Error(err)
}

func (s *StatusStoreTestSuite) TestRowInt64() {
	for _, v := range []interface{}{int64(5), int(5), int32(5), float64(5)} {
		got, err := rowInt64(map[string]interface{}{"k": v}, "k")
		s.NoError(err)
		s.Equal(int64(5), got)
	}
	_, err := rowInt64(map[string]interface{}{"k": "nope"}, "k")
	s.Error(err)
}

func (s *StatusStoreTestSuite) TestParseListRecord() {
	now := time.Now()
	base := func() map[string]interface{} {
		return map[string]interface{}{
			"id": "list-1", "bits": int64(1), "state": int64(0),
			"next_idx": int64(3), "capacity": int64(100), "created_at": now,
		}
	}

	s.Run("valid without sealed_at", func() {
		rec, err := parseListRecord(base())
		s.NoError(err)
		s.Equal("list-1", rec.id)
		s.Equal(1, rec.bits)
		s.Equal(int64(3), rec.nextIdx)
		s.True(rec.sealedAt.IsZero())
	})
	s.Run("valid with sealed_at", func() {
		row := base()
		row["sealed_at"] = now
		rec, err := parseListRecord(row)
		s.NoError(err)
		s.False(rec.sealedAt.IsZero())
	})
	for _, badCol := range []string{"id", "bits", "state", "next_idx", "capacity", "created_at"} {
		s.Run("bad "+badCol, func() {
			row := base()
			row[badCol] = struct{}{} // a type none of the readers accept
			_, err := parseListRecord(row)
			s.Error(err)
		})
	}
	s.Run("bad sealed_at", func() {
		row := base()
		row["sealed_at"] = struct{}{}
		_, err := parseListRecord(row)
		s.Error(err)
	})
}
