// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
	"github.com/thunder-id/thunderid/tests/mocks/transactionmock"
)

// testPermanentUnlockAt and testPostSuspensionReason are the values governance writes for the
// post-suspension hold; the store passes them through.
const (
	testPermanentUnlockAt                                  = "9999-12-31T23:59:59Z"
	testPostSuspensionReason identitygovernance.LockReason = "POST_SUSPENDED"
)

type RuntimeStoreTestSuite struct {
	suite.Suite
	provider *providermock.DBProviderInterfaceMock
	client   *providermock.DBClientInterfaceMock
	store    *entityRuntimeDBStore
	ctx      context.Context
	now      time.Time
	testErr  error
}

func TestRuntimeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(RuntimeStoreTestSuite))
}

func (s *RuntimeStoreTestSuite) SetupTest() {
	s.provider = providermock.NewDBProviderInterfaceMock(s.T())
	s.client = providermock.NewDBClientInterfaceMock(s.T())
	s.store = &entityRuntimeDBStore{dbProvider: s.provider}
	s.ctx = deployment.WithID(context.Background(), testDeploymentID)
	s.now = time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	s.testErr = errors.New("db error")
}

func (s *RuntimeStoreTestSuite) expectClient() {
	s.provider.EXPECT().GetRuntimePersistentDBClient().Return(s.client, nil).Once()
}

func (s *RuntimeStoreTestSuite) expectClientError() {
	s.provider.EXPECT().GetRuntimePersistentDBClient().Return(nil, s.testErr).Once()
}

// anyArgs returns n matchers that accept any argument, for variadic calls whose length varies.
func anyArgs(n int) []interface{} {
	args := make([]interface{}, n)
	for i := range args {
		args[i] = mock.Anything
	}
	return args
}

// runtimeRow is a runtime row as the client returns it.
func runtimeRow(id string, revision interface{}) map[string]interface{} {
	return map[string]interface{}{
		"entity_id": id, "state": "SUSPENDED", "runtime_attributes": `{"accessState":{}}`, "revision": revision,
	}
}

// The runtime store is built over the server's database provider.
func (s *RuntimeStoreTestSuite) TestNewEntityRuntimeDBStoreUsesTheServersProvider() {
	original := getDBProvider
	s.T().Cleanup(func() { getDBProvider = original })
	getDBProvider = func() provider.DBProviderInterface { return s.provider }

	store, ok := newEntityRuntimeDBStore().(*entityRuntimeDBStore)

	s.Require().True(ok)
	s.Same(s.provider, store.dbProvider)
}

func (s *RuntimeStoreTestSuite) TestGetRuntimeDataWithNoIDsReadsNothing() {
	records, err := s.store.GetRuntimeData(s.ctx, nil)

	s.Require().NoError(err)
	s.Empty(records)
	s.provider.AssertNotCalled(s.T(), "GetRuntimePersistentDBClient")
}

func (s *RuntimeStoreTestSuite) TestGetRuntimeDataReadsTheDeploymentsRows() {
	s.expectClient()
	query, args := buildGetRuntimeDataQuery([]string{"e1", "e2"}, testDeploymentID)
	s.client.On("QueryContext", append([]interface{}{s.ctx, query}, args...)...).
		Return([]map[string]interface{}{runtimeRow("e1", int64(7))}, nil).Once()

	records, err := s.store.GetRuntimeData(s.ctx, []string{"e1", "e2"})

	s.Require().NoError(err)
	s.Equal(map[string]entityRuntimeData{"e1": {ID: "e1", State: providers.EntityStateSuspended,
		Attributes: json.RawMessage(`{"accessState":{}}`), Revision: 7}}, records,
		"an entity with no row is absent")
}

// Each batch is one statement over its own slice of the ids.
func (s *RuntimeStoreTestSuite) TestGetRuntimeDataReadsInBatches() {
	ids := make([]string, 2*runtimeBatchSize+1)
	for i := range ids {
		ids[i] = fmt.Sprintf("e%d", i)
	}
	s.expectClient()
	for start := 0; start < len(ids); start += runtimeBatchSize {
		batch := ids[start:min(start+runtimeBatchSize, len(ids))]
		query, args := buildGetRuntimeDataQuery(batch, testDeploymentID)
		s.client.On("QueryContext", append([]interface{}{s.ctx, query}, args...)...).
			Return([]map[string]interface{}{runtimeRow(batch[0], int64(1))}, nil).Once()
	}

	records, err := s.store.GetRuntimeData(s.ctx, ids)

	s.Require().NoError(err)
	s.Len(records, 3)
	s.Contains(records, ids[2*runtimeBatchSize])
	s.client.AssertNumberOfCalls(s.T(), "QueryContext", 3)
}

func (s *RuntimeStoreTestSuite) TestGetRuntimeDataFailures() {
	s.Run("client", func() {
		s.SetupTest()
		s.expectClientError()
		_, err := s.store.GetRuntimeData(s.ctx, []string{"e1"})
		s.ErrorIs(err, s.testErr)
	})
	s.Run("query", func() {
		s.SetupTest()
		s.expectClient()
		s.client.On("QueryContext", append([]interface{}{s.ctx}, anyArgs(3)...)...).Return(nil, s.testErr).Once()
		_, err := s.store.GetRuntimeData(s.ctx, []string{"e1"})
		s.ErrorIs(err, s.testErr)
	})
}

// The drivers return the revision as different types; each one a driver can return is read.
func (s *RuntimeStoreTestSuite) TestGetRuntimeDataReadsEveryRevisionType() {
	for name, tc := range map[string]struct {
		revision interface{}
		want     int64
	}{
		"int64": {int64(7), 7}, "int32": {int32(7), 7}, "float64": {float64(7), 7},
		"bytes": {[]byte("7"), 7}, "text": {"7", 7}, "null": {nil, 0},
	} {
		s.Run(name, func() {
			s.SetupTest()
			s.expectClient()
			s.client.On("QueryContext", append([]interface{}{s.ctx}, anyArgs(3)...)...).
				Return([]map[string]interface{}{runtimeRow("e1", tc.revision)}, nil).Once()

			records, err := s.store.GetRuntimeData(s.ctx, []string{"e1"})

			s.Require().NoError(err)
			s.Equal(tc.want, records["e1"].Revision)
		})
	}
}

// A row that does not hold a readable runtime record fails the read rather than reading as a default.
func (s *RuntimeStoreTestSuite) TestGetRuntimeDataRefusesAnUnreadableRow() {
	for name, edit := range map[string]func(row map[string]interface{}){
		"no entity id":           func(row map[string]interface{}) { delete(row, "entity_id") },
		"no state":               func(row map[string]interface{}) { row["state"] = nil },
		"attributes not json":    func(row map[string]interface{}) { row["runtime_attributes"] = "{" },
		"attributes not object":  func(row map[string]interface{}) { row["runtime_attributes"] = `["x"]` },
		"no attributes":          func(row map[string]interface{}) { row["runtime_attributes"] = nil },
		"revision of other type": func(row map[string]interface{}) { row["revision"] = true },
		"revision not a number":  func(row map[string]interface{}) { row["revision"] = "seven" },
	} {
		s.Run(name, func() {
			s.SetupTest()
			row := runtimeRow("e1", int64(1))
			edit(row)
			s.expectClient()
			s.client.On("QueryContext", append([]interface{}{s.ctx}, anyArgs(3)...)...).
				Return([]map[string]interface{}{row}, nil).Once()

			records, err := s.store.GetRuntimeData(s.ctx, []string{"e1"})

			s.Error(err)
			s.Nil(records)
		})
	}
}

// Rows are seeded in batches, each one statement that keeps rows already present.
func (s *RuntimeStoreTestSuite) TestInitializeRuntimeDataSeedsInBatches() {
	rows := make([]entityRuntimeData, runtimeBatchSize+1)
	for i := range rows {
		rows[i] = entityRuntimeData{ID: fmt.Sprintf("e%d", i), State: providers.EntityStateActive,
			Attributes: json.RawMessage(`{}`)}
	}
	var calls []mock.Arguments
	s.expectClient()
	s.client.On("ExecuteContext", append([]interface{}{s.ctx}, anyArgs(2+4*runtimeBatchSize)...)...).
		Run(func(args mock.Arguments) { calls = append(calls, args) }).Return(int64(1), nil).Twice()

	s.Require().NoError(s.store.InitializeRuntimeData(s.ctx, rows))

	s.Require().Len(calls, 2)
	for i, batch := range [][]entityRuntimeData{rows[:runtimeBatchSize], rows[runtimeBatchSize:]} {
		now, ok := calls[i].Get(5).(time.Time)
		s.Require().True(ok, "the first row's creation time")
		query, args := buildInitializeRuntimeDataQuery(batch, now, testDeploymentID)
		s.Equal(query, calls[i].Get(1), "batch %d", i)
		s.Equal(args, []interface{}(calls[i][2:]), "batch %d", i)
	}
}

func (s *RuntimeStoreTestSuite) TestInitializeRuntimeDataRefusals() {
	s.Run("nothing to seed", func() {
		s.SetupTest()
		s.NoError(s.store.InitializeRuntimeData(s.ctx, nil))
		s.provider.AssertNotCalled(s.T(), "GetRuntimePersistentDBClient")
	})
	s.Run("a row with no id", func() {
		s.SetupTest()
		err := s.store.InitializeRuntimeData(s.ctx, []entityRuntimeData{{ID: "e1"}, {}})
		s.Error(err)
		s.provider.AssertNotCalled(s.T(), "GetRuntimePersistentDBClient")
	})
	s.Run("client", func() {
		s.SetupTest()
		s.expectClientError()
		s.ErrorIs(s.store.InitializeRuntimeData(s.ctx, []entityRuntimeData{{ID: "e1"}}), s.testErr)
	})
	s.Run("statement", func() {
		s.SetupTest()
		s.expectClient()
		s.client.On("ExecuteContext", append([]interface{}{s.ctx}, anyArgs(6)...)...).Return(int64(0), s.testErr).Once()
		s.ErrorIs(s.store.InitializeRuntimeData(s.ctx, []entityRuntimeData{{ID: "e1"}}), s.testErr)
	})
}

// A new entity starts ACTIVE unless a state is given.
func (s *RuntimeStoreTestSuite) TestResetRuntimeDataWritesTheStartingRow() {
	for given, want := range map[providers.EntityState]string{
		"": "ACTIVE", providers.EntityStateSuspended: "SUSPENDED",
	} {
		s.Run(want, func() {
			s.SetupTest()
			s.expectClient()
			s.client.On("ExecuteContext", s.ctx, queryResetRuntimeData, "e1", want,
				mock.AnythingOfType("time.Time"), testDeploymentID).Return(int64(1), nil).Once()

			s.NoError(s.store.ResetRuntimeData(s.ctx, "e1", given))
		})
	}
}

func (s *RuntimeStoreTestSuite) TestDeleteRuntimeDataDeletesTheDeploymentsRow() {
	s.expectClient()
	s.client.On("ExecuteContext", s.ctx, queryDeleteRuntimeData, "e1", testDeploymentID).Return(int64(0), nil).Once()

	s.NoError(s.store.DeleteRuntimeData(s.ctx, "e1"), "a missing row is not an error")
}

// Every write reports a client or statement failure as an error.
func (s *RuntimeStoreTestSuite) TestWritesReportStoreFailures() {
	revision := int64(3)
	writes := map[string]func() error{
		"reset":  func() error { return s.store.ResetRuntimeData(s.ctx, "e1", "") },
		"delete": func() error { return s.store.DeleteRuntimeData(s.ctx, "e1") },
		"form lock": func() error {
			_, err := s.store.FormAccessLock(s.ctx, "e1", testMethod, identitygovernance.LockEntry{},
				identitygovernance.LockEpisode{}, s.now)
			return err
		},
		"clear": func() error {
			return s.store.ClearAccessState(s.ctx, "e1", []providers.AuthenticationMethod{testMethod})
		},
		"guarded clear": func() error {
			_, err := s.store.ClearAccessStateIfUnchanged(s.ctx, "e1", testMethod, revision)
			return err
		},
		"suspend": func() error {
			_, err := s.store.SetAccessSuspension(s.ctx, "e1", s.now, "")
			return err
		},
		"record login": func() error { return s.store.RecordAccessLogin(s.ctx, "e1", s.now, s.now) },
		"release": func() error {
			_, err := s.store.ClearAccessSuspension(s.ctx, "e1", nil, "")
			return err
		},
		"release onto a hold": func() error {
			_, err := s.store.ClearAccessSuspension(s.ctx, "e1",
				&identitygovernance.AccountHold{UnlockAt: testPermanentUnlockAt}, "")
			return err
		},
	}
	for name, write := range writes {
		s.Run(name+" client", func() {
			s.SetupTest()
			s.expectClientError()
			s.ErrorIs(write(), s.testErr)
		})
		s.Run(name+" statement", func() {
			s.SetupTest()
			s.expectClient()
			s.client.On("ExecuteContext", append([]interface{}{s.ctx}, anyArgs(10)...)...).
				Return(int64(0), s.testErr).Once()
			s.ErrorIs(write(), s.testErr)
		})
	}
}

// expectTransaction runs the increment's transaction with a context that marks it.
func (s *RuntimeStoreTestSuite) expectTransaction() context.Context {
	txCtx := context.WithValue(s.ctx, txMarker{}, true)
	transactioner := transactionmock.NewTransactionerMock(s.T())
	transactioner.EXPECT().Transact(s.ctx, mock.Anything).RunAndReturn(
		func(_ context.Context, fn func(context.Context) error) error { return fn(txCtx) }).Once()
	s.provider.EXPECT().GetRuntimePersistentDBTransactioner().Return(transactioner, nil).Once()
	return txCtx
}

// txMarker is the key expectTransaction marks the transaction's context with.
type txMarker struct{}

// The increment runs inside a transaction with the method's statement, and returns the counters and
// revision the statement returned.
func (s *RuntimeStoreTestSuite) TestIncrementAccessFailureReturnsTheCounters() {
	windowStart := s.now.Add(-15 * time.Minute)
	s.expectClient()
	txCtx := s.expectTransaction()
	s.client.On("QueryContext", txCtx, incrementAccessQueries[testMethod], "e1", "2026-09-08T09:45:00Z",
		"2026-09-08T10:00:00Z", "2026-09-08T10:00:00Z", s.now, testDeploymentID).
		Return([]map[string]interface{}{{"failure_count": int64(3), "lock_count": []byte("2"),
			"unlock_at": "2026-09-08T09:55:00Z", "consecutive_failures": float64(9), "revision": "12"}}, nil).Once()

	observed, recorded, err := s.store.IncrementAccessFailure(s.ctx, "e1", testMethod, s.now, windowStart)

	s.Require().NoError(err)
	s.True(recorded)
	s.Equal(identitygovernance.LockEntry{FailureCount: 3, LockCount: 2, UnlockAt: "2026-09-08T09:55:00Z",
		ConsecutiveFailures: 9, Revision: 12}, observed)
}

// No returned row means nothing was written: no runtime row, or a live lock.
func (s *RuntimeStoreTestSuite) TestIncrementAccessFailureWithNoRowRecordsNothing() {
	s.expectClient()
	txCtx := s.expectTransaction()
	s.client.On("QueryContext", append([]interface{}{txCtx}, anyArgs(7)...)...).
		Return([]map[string]interface{}{}, nil).Once()

	observed, recorded, err := s.store.IncrementAccessFailure(s.ctx, "e1",
		identitygovernance.AuthenticationMethodAccount,
		s.now, time.Time{})

	s.Require().NoError(err)
	s.False(recorded)
	s.Equal(identitygovernance.LockEntry{}, observed)
}

func (s *RuntimeStoreTestSuite) TestIncrementAccessFailureFailures() {
	s.Run("client", func() {
		s.SetupTest()
		s.expectClientError()
		_, _, err := s.store.IncrementAccessFailure(s.ctx, "e1", testMethod, s.now, s.now)
		s.ErrorIs(err, s.testErr)
	})
	s.Run("transactioner", func() {
		s.SetupTest()
		s.expectClient()
		s.provider.EXPECT().GetRuntimePersistentDBTransactioner().Return(nil, s.testErr).Once()
		_, _, err := s.store.IncrementAccessFailure(s.ctx, "e1", testMethod, s.now, s.now)
		s.ErrorIs(err, s.testErr)
	})
	s.Run("statement", func() {
		s.SetupTest()
		s.expectClient()
		txCtx := s.expectTransaction()
		s.client.On("QueryContext", append([]interface{}{txCtx}, anyArgs(7)...)...).Return(nil, s.testErr).Once()
		_, recorded, err := s.store.IncrementAccessFailure(s.ctx, "e1", testMethod, s.now, s.now)
		s.ErrorIs(err, s.testErr)
		s.False(recorded)
	})
}

// A returned counter that cannot be read fails the increment, naming the counter.
func (s *RuntimeStoreTestSuite) TestIncrementAccessFailureRefusesAnUnreadableCounter() {
	for column, want := range map[string]string{
		"failure_count": "failureCount", "lock_count": "lockCount",
		"consecutive_failures": "consecutiveFailures", "revision": "revision",
	} {
		s.Run(column, func() {
			s.SetupTest()
			row := map[string]interface{}{"failure_count": int64(1), "lock_count": int64(0),
				"consecutive_failures": int64(1), "revision": int64(1)}
			row[column] = true
			s.expectClient()
			txCtx := s.expectTransaction()
			s.client.On("QueryContext", append([]interface{}{txCtx}, anyArgs(7)...)...).
				Return([]map[string]interface{}{row}, nil).Once()

			_, recorded, err := s.store.IncrementAccessFailure(s.ctx, "e1", testMethod, s.now, s.now)

			s.Require().Error(err)
			s.Contains(err.Error(), want)
			s.False(recorded)
		})
	}
}

// A method outside the registry becomes no statement and reaches no database.
func (s *RuntimeStoreTestSuite) TestAnUnregisteredMethodIsRefused() {
	for _, method := range []providers.AuthenticationMethod{"credential.unlockAt", `credential"]`, "", "notAMethod"} {
		_, _, err := s.store.IncrementAccessFailure(s.ctx, "e1", method, s.now, s.now)
		s.ErrorIs(err, errInvalidAuthenticationMethod, "increment %q", method)

		_, err = s.store.FormAccessLock(s.ctx, "e1", method, identitygovernance.LockEntry{},
			identitygovernance.LockEpisode{}, s.now)
		s.ErrorIs(err, errInvalidAuthenticationMethod, "formation %q", method)

		s.ErrorIs(s.store.ClearAccessState(s.ctx, "e1", []providers.AuthenticationMethod{testMethod, method}),
			errInvalidAuthenticationMethod, "clear %q", method)
	}
	s.provider.AssertNotCalled(s.T(), "GetRuntimePersistentDBClient")
}

// The formation is guarded on the observed counters and revision, and reports whether it applied.
func (s *RuntimeStoreTestSuite) TestFormAccessLockIsGuardedOnTheObservedCounters() {
	observed := identitygovernance.LockEntry{FailureCount: 5, LockCount: 1, Revision: 8}
	episode := identitygovernance.LockEpisode{LockCount: 2, UnlockAt: "2026-09-08T10:15:00Z"}
	for rowsAffected, formed := range map[int64]bool{1: true, 0: false} {
		s.Run(fmt.Sprint(formed), func() {
			s.SetupTest()
			s.expectClient()
			s.client.On("ExecuteContext", s.ctx,
				formAccessQueries[identitygovernance.AuthenticationMethodAccount], "e1",
				"2026-09-08T10:00:00Z", 2, "2026-09-08T10:15:00Z", 5, 1, s.now, int64(8), testDeploymentID).
				Return(rowsAffected, nil).Once()

			got, err := s.store.FormAccessLock(s.ctx, "e1", identitygovernance.AuthenticationMethodAccount, observed,
				episode, s.now)

			s.Require().NoError(err)
			s.Equal(formed, got)
		})
	}
}

func (s *RuntimeStoreTestSuite) TestClearAccessStateRemovesTheGivenMethods() {
	methods := []providers.AuthenticationMethod{testMethod, identitygovernance.AuthenticationMethodAccount}
	query, _, err := buildClearAccessStateQuery("e1", methods, s.now, nil, testDeploymentID)
	s.Require().NoError(err)
	s.expectClient()
	s.client.On("ExecuteContext", s.ctx, query, mock.AnythingOfType("time.Time"), "e1", testDeploymentID).
		Return(int64(0), nil).Once()

	s.NoError(s.store.ClearAccessState(s.ctx, "e1", methods), "a missing row is not an error")
}

func (s *RuntimeStoreTestSuite) TestClearAccessStateWithNoMethodsWritesNothing() {
	s.NoError(s.store.ClearAccessState(s.ctx, "e1", nil))
	s.provider.AssertNotCalled(s.T(), "GetRuntimePersistentDBClient")
}

// The guarded clear passes the revision and reports whether the row still had it.
func (s *RuntimeStoreTestSuite) TestClearAccessStateIfUnchangedIsGuardedOnTheRevision() {
	revision := int64(7)
	query, _, err := buildClearAccessStateQuery("e1", []providers.AuthenticationMethod{testMethod}, s.now, &revision,
		testDeploymentID)
	s.Require().NoError(err)
	for rowsAffected, cleared := range map[int64]bool{1: true, 0: false} {
		s.Run(fmt.Sprint(cleared), func() {
			s.SetupTest()
			s.expectClient()
			s.client.On("ExecuteContext", s.ctx, query, mock.AnythingOfType("time.Time"), "e1", revision,
				testDeploymentID).Return(rowsAffected, nil).Once()

			got, err := s.store.ClearAccessStateIfUnchanged(s.ctx, "e1", testMethod, revision)

			s.Require().NoError(err)
			s.Equal(cleared, got)
		})
	}
}

// The suspension records its placement time to the nanosecond and the note, and reports whether it was
// placed.
func (s *RuntimeStoreTestSuite) TestSetAccessSuspensionRecordsThePlacement() {
	at := s.now.Add(123 * time.Nanosecond)
	for rowsAffected, placed := range map[int64]bool{1: true, 0: false} {
		s.Run(fmt.Sprint(placed), func() {
			s.SetupTest()
			s.expectClient()
			s.client.On("ExecuteContext", s.ctx, querySetAccessSuspension, "e1", "2026-09-08T10:00:00.000000123Z",
				"Credential exposure", mock.AnythingOfType("time.Time"), testDeploymentID).
				Return(rowsAffected, nil).Once()

			got, err := s.store.SetAccessSuspension(s.ctx, "e1", at, "Credential exposure")

			s.Require().NoError(err)
			s.Equal(placed, got)
		})
	}
}

func (s *RuntimeStoreTestSuite) TestRecordAccessLoginPassesTheWindow() {
	s.expectClient()
	s.client.On("ExecuteContext", s.ctx, queryRecordAccessLogin, "e1", "2026-09-08T10:00:00Z",
		"2026-09-08T09:00:00Z", mock.AnythingOfType("time.Time"), testDeploymentID).Return(int64(0), nil).Once()

	s.NoError(s.store.RecordAccessLogin(s.ctx, "e1", s.now, s.now.Add(-time.Hour)),
		"a skipped write is not an error")
}

// A release with no hold clears only the suspension; with a hold the same statement writes it. Either is
// guarded on the observed placement.
func (s *RuntimeStoreTestSuite) TestClearAccessSuspensionWritesTheHoldInTheSameStatement() {
	hold := &identitygovernance.AccountHold{UnlockAt: testPermanentUnlockAt,
		Reason: testPostSuspensionReason}
	s.Run("without a hold", func() {
		s.SetupTest()
		s.expectClient()
		s.client.On("ExecuteContext", s.ctx, queryClearAccessSuspension, "e1", mock.AnythingOfType("time.Time"),
			"2026-09-08T09:00:00Z", testDeploymentID).Return(int64(1), nil).Once()

		cleared, err := s.store.ClearAccessSuspension(s.ctx, "e1", nil, "2026-09-08T09:00:00Z")

		s.Require().NoError(err)
		s.True(cleared)
	})
	s.Run("with a hold", func() {
		s.SetupTest()
		s.expectClient()
		s.client.On("ExecuteContext", s.ctx, queryClearAccessSuspensionWithHold, "e1",
			testPermanentUnlockAt, "POST_SUSPENDED", mock.AnythingOfType("time.Time"),
			"2026-09-08T09:00:00Z", testDeploymentID).Return(int64(0), nil).Once()

		cleared, err := s.store.ClearAccessSuspension(s.ctx, "e1", hold, "2026-09-08T09:00:00Z")

		s.Require().NoError(err)
		s.False(cleared, "that suspension is no longer there")
	})
}
