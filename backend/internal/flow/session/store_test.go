// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment-id"

type StoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *store
}

func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (s *StoreTestSuite) SetupTest() {
	s.mockDBProvider = &providermock.DBProviderInterfaceMock{}
	s.mockDBClient = &providermock.DBClientInterfaceMock{}
	s.store = &store{
		dbProvider:   s.mockDBProvider,
		deploymentID: testDeploymentID,
	}
}

func (s *StoreTestSuite) sampleSession() Session {
	base := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	return Session{
		SessionID:       "sess-1",
		SubjectID:       "user-1",
		FlowID:          "flow-1",
		FlowVersion:     2,
		FlowExecutionID: "exec-1",
		HandleID:        "handle-abc",
		AuthenticatedAt: base,
		CreatedAt:       base,
		LastActiveAt:    base,
		// IdleExpiresAt left zero on purpose to exercise the nullable path.
		AbsoluteExpiresAt: base.Add(8 * time.Hour),
		State:             StateActive,
		Version:           1,
	}
}

func (s *StoreTestSuite) TestNewStore() {
	st := newStore(s.mockDBProvider, testDeploymentID)
	s.NotNil(st)
	s.Implements((*sessionStore)(nil), st)
}

func (s *StoreTestSuite) TestCreate_Success() {
	sess := s.sampleSession()

	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryCreateSession,
		sess.SessionID, testDeploymentID, sess.SubjectID, sess.FlowID, sess.FlowVersion,
		sess.FlowExecutionID, sess.HandleID,
		sess.AuthenticatedAt, sess.CreatedAt, sess.LastActiveAt,
		nil, sess.AbsoluteExpiresAt, string(sess.State), sess.Version).
		Return(int64(1), nil)

	err := s.store.Create(context.Background(), sess)

	s.NoError(err)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

func (s *StoreTestSuite) TestCreate_DBError() {
	sess := s.sampleSession()

	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryCreateSession,
		sess.SessionID, testDeploymentID, sess.SubjectID, sess.FlowID, sess.FlowVersion,
		sess.FlowExecutionID, sess.HandleID,
		sess.AuthenticatedAt, sess.CreatedAt, sess.LastActiveAt,
		nil, sess.AbsoluteExpiresAt, string(sess.State), sess.Version).
		Return(int64(0), errors.New("db down"))

	err := s.store.Create(context.Background(), sess)

	s.Error(err)
	s.Contains(err.Error(), "failed to create session")
}

func (s *StoreTestSuite) TestGetByHandle_Hit() {
	base := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	row := map[string]interface{}{
		"session_id":          "sess-1",
		"subject_id":          "user-1",
		"flow_id":             "flow-1",
		"flow_version":        int64(2),
		"flow_execution_id":   "exec-1",
		"handle_id":           "handle-abc",
		"handle_issued_at":    base,
		"handle_expires_at":   base.Add(time.Hour),
		"authenticated_at":    base,
		"created_at":          base,
		"last_active_at":      base,
		"idle_expires_at":     nil,
		"absolute_expires_at": base.Add(8 * time.Hour),
		"state":               "ACTIVE",
		"version":             int64(3),
	}

	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionByHandle,
		"handle-abc", testDeploymentID).
		Return([]map[string]interface{}{row}, nil)

	got, err := s.store.GetByHandle(context.Background(), "handle-abc")

	s.NoError(err)
	s.Require().NotNil(got)
	s.Equal("sess-1", got.SessionID)
	s.Equal("user-1", got.SubjectID)
	s.Equal("flow-1", got.FlowID)
	s.Equal(2, got.FlowVersion)
	s.Equal("handle-abc", got.HandleID)
	s.Equal(StateActive, got.State)
	s.Equal(3, got.Version)
	s.True(got.AbsoluteExpiresAt.Equal(base.Add(8 * time.Hour)))
	s.True(got.IdleExpiresAt.IsZero())
}

func (s *StoreTestSuite) TestGetByHandle_Miss() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionByHandle,
		"missing", testDeploymentID).
		Return([]map[string]interface{}{}, nil)

	got, err := s.store.GetByHandle(context.Background(), "missing")

	s.NoError(err)
	s.Nil(got)
}

func (s *StoreTestSuite) TestGetByExecutionID_Hit() {
	base := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	row := map[string]interface{}{
		"session_id":          "sess-1",
		"subject_id":          "user-1",
		"flow_id":             "flow-1",
		"flow_version":        int64(2),
		"flow_execution_id":   "exec-1",
		"handle_id":           "handle-abc",
		"authenticated_at":    base,
		"created_at":          base,
		"last_active_at":      base,
		"idle_expires_at":     nil,
		"absolute_expires_at": base.Add(8 * time.Hour),
		"state":               "ACTIVE",
		"version":             int64(1),
	}

	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionByExecutionID,
		"exec-1", testDeploymentID).
		Return([]map[string]interface{}{row}, nil)

	got, err := s.store.GetByExecutionID(context.Background(), "exec-1")

	s.NoError(err)
	s.Require().NotNil(got)
	s.Equal("sess-1", got.SessionID)
	s.Equal("exec-1", got.FlowExecutionID)
}

func (s *StoreTestSuite) TestGetByExecutionID_Miss() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionByExecutionID,
		"missing", testDeploymentID).
		Return([]map[string]interface{}{}, nil)

	got, err := s.store.GetByExecutionID(context.Background(), "missing")

	s.NoError(err)
	s.Nil(got)
}

func (s *StoreTestSuite) TestListBySubject() {
	base := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	row := map[string]interface{}{
		"session_id": "sess-1", "subject_id": "user-1", "flow_id": "flow-1",
		"flow_version": int64(2), "flow_execution_id": "exec-1", "handle_id": "handle-abc",
		"authenticated_at": base, "created_at": base, "last_active_at": base,
		"idle_expires_at": nil, "absolute_expires_at": base.Add(8 * time.Hour),
		"state": "ACTIVE", "version": int64(1),
	}
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryListSessionsBySubject,
		"user-1", testDeploymentID).Return([]map[string]interface{}{row}, nil)

	got, err := s.store.ListBySubject(context.Background(), "user-1")

	s.NoError(err)
	s.Require().Len(got, 1)
	s.Equal("sess-1", got[0].SessionID)
	s.Equal("user-1", got[0].SubjectID)
}

func (s *StoreTestSuite) TestUpdate_Success() {
	sess := s.sampleSession()

	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryUpdateSession,
		sess.FlowVersion, sess.HandleID,
		sess.LastActiveAt, nil, sess.AbsoluteExpiresAt,
		string(sess.State), sess.SessionID, testDeploymentID, sess.Version).
		Return(int64(1), nil)

	err := s.store.Update(context.Background(), &sess)

	s.NoError(err)
	s.Equal(2, sess.Version) // optimistic version bumped in memory
	s.mockDBClient.AssertExpectations(s.T())
}

// TestTouchAuthenticatedAt_Success pins the parameter order of the refresh statement. The query
// binds the new authentication time twice (AUTHENTICATED_AT and LAST_ACTIVE_AT) before the idle
// deadline, so a reordering here would silently write the wrong column rather than fail.
func (s *StoreTestSuite) TestTouchAuthenticatedAt_Success() {
	sess := s.sampleSession()
	authAt := sess.LastActiveAt.Add(time.Hour)
	idleAt := authAt.Add(30 * time.Minute)

	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryTouchAuthenticatedAt,
		authAt, authAt, idleAt, sess.SessionID, testDeploymentID).
		Return(int64(1), nil)

	err := s.store.TouchAuthenticatedAt(context.Background(), sess.SessionID, authAt, idleAt)

	s.NoError(err)
	s.mockDBClient.AssertExpectations(s.T())
}

// TestTouchAuthenticatedAt_NoRowsIsNotAnError verifies the deliberate absence of a version guard: a
// session deleted concurrently matches no row, and that is not a failure worth surfacing, unlike
// Update where a zero row count means a lost optimistic-lock race.
func (s *StoreTestSuite) TestTouchAuthenticatedAt_NoRowsIsNotAnError() {
	sess := s.sampleSession()
	authAt := sess.LastActiveAt

	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryTouchAuthenticatedAt,
		authAt, authAt, authAt, sess.SessionID, testDeploymentID).
		Return(int64(0), nil)

	err := s.store.TouchAuthenticatedAt(context.Background(), sess.SessionID, authAt, authAt)

	s.NoError(err, "a vanished session must not fail the refresh")
	s.mockDBClient.AssertExpectations(s.T())
}

// TestTouchAuthenticatedAt_IsMonotonic covers two re-authentications whose writes reach the
// database in the reverse order to their timestamps. The statement is monotonic in
// AUTHENTICATED_AT, so the older write matches no row and the newer authentication stands; both
// calls report success, since in each case the column ends up at least as fresh as the value
// written. Without the predicate the late-arriving older write would move the authentication
// backwards, under-reporting auth_time and failing a max_age the subject actually satisfies.
func (s *StoreTestSuite) TestTouchAuthenticatedAt_IsMonotonic() {
	sess := s.sampleSession()
	older := sess.LastActiveAt.Add(time.Minute)
	newer := older.Add(time.Minute)

	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	// The newer authentication lands first and updates the row.
	s.mockDBClient.On("ExecuteContext", context.Background(), queryTouchAuthenticatedAt,
		newer, newer, newer, sess.SessionID, testDeploymentID).
		Return(int64(1), nil)
	// The older one arrives second; the predicate excludes the row, so nothing is overwritten.
	s.mockDBClient.On("ExecuteContext", context.Background(), queryTouchAuthenticatedAt,
		older, older, older, sess.SessionID, testDeploymentID).
		Return(int64(0), nil)

	s.Require().NoError(s.store.TouchAuthenticatedAt(context.Background(), sess.SessionID, newer, newer))
	s.NoError(s.store.TouchAuthenticatedAt(context.Background(), sess.SessionID, older, older),
		"an out-of-order touch that changes nothing is still a success")

	s.mockDBClient.AssertExpectations(s.T())
}

// TestTouchAuthenticatedAt_QueryGuardsMonotonicity pins the predicate itself, so removing it from
// the statement fails here rather than silently reintroducing the backwards-write race.
func (s *StoreTestSuite) TestTouchAuthenticatedAt_QueryGuardsMonotonicity() {
	s.Contains(queryTouchAuthenticatedAt.Query, "AUTHENTICATED_AT <= $1",
		"the refresh must not move a session's authentication time backwards")
}

// TestTouchAuthenticatedAt_ExecuteError surfaces a genuine database failure to the caller, which
// degrades it to a log rather than failing the login.
func (s *StoreTestSuite) TestTouchAuthenticatedAt_ExecuteError() {
	sess := s.sampleSession()
	authAt := sess.LastActiveAt

	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryTouchAuthenticatedAt,
		authAt, authAt, authAt, sess.SessionID, testDeploymentID).
		Return(int64(0), errors.New("db down"))

	err := s.store.TouchAuthenticatedAt(context.Background(), sess.SessionID, authAt, authAt)

	s.Error(err)
	s.mockDBClient.AssertExpectations(s.T())
}

func (s *StoreTestSuite) TestUpdate_VersionConflict() {
	sess := s.sampleSession()

	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryUpdateSession,
		sess.FlowVersion, sess.HandleID,
		sess.LastActiveAt, nil, sess.AbsoluteExpiresAt,
		string(sess.State), sess.SessionID, testDeploymentID, sess.Version).
		Return(int64(0), nil)

	err := s.store.Update(context.Background(), &sess)

	s.ErrorIs(err, errVersionConflict)
	s.Equal(1, sess.Version) // version unchanged on conflict
}

func (s *StoreTestSuite) TestGetByHandle_ClientError() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(nil, errors.New("no client"))

	got, err := s.store.GetByHandle(context.Background(), "handle-abc")

	s.Error(err)
	s.Nil(got)
}

func (s *StoreTestSuite) TestGetByHandle_QueryError() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionByHandle,
		"handle-abc", testDeploymentID).
		Return(nil, errors.New("query failed"))

	got, err := s.store.GetByHandle(context.Background(), "handle-abc")

	s.Error(err)
	s.Nil(got)
}

// TestBuildSessionFromRow_DriverVariants exercises the []byte / string-time / integer
// forms different drivers return for the same logical columns.
func (s *StoreTestSuite) TestBuildSessionFromRow_DriverVariants() {
	row := map[string]interface{}{
		"session_id":          []byte("sess-1"),
		"subject_id":          "user-1",
		"flow_id":             "flow-1",
		"flow_version":        int32(2),
		"flow_execution_id":   "exec-1",
		"handle_id":           "handle-abc",
		"handle_issued_at":    "2026-06-16 10:00:00",
		"handle_expires_at":   "2026-06-16T11:00:00Z",
		"authenticated_at":    "2026-06-16 10:00:00",
		"created_at":          "2026-06-16 10:00:00",
		"last_active_at":      "2026-06-16 10:00:00",
		"idle_expires_at":     "2026-06-16 10:30:00",
		"absolute_expires_at": "2026-06-16 18:00:00",
		"state":               "ACTIVE",
		"version":             3,
	}

	got, err := buildSessionFromRow(row)

	s.NoError(err)
	s.Require().NotNil(got)
	s.Equal("sess-1", got.SessionID)
	s.Equal(2, got.FlowVersion)
	s.Equal(3, got.Version)
	s.False(got.IdleExpiresAt.IsZero())
}

func (s *StoreTestSuite) TestBuildSessionFromRow_BadField() {
	row := map[string]interface{}{"session_id": 42}

	got, err := buildSessionFromRow(row)

	s.Error(err)
	s.Nil(got)
}

func (s *StoreTestSuite) TestBuildSessionFromRow_BadIntField() {
	row := map[string]interface{}{
		"session_id":   "sess-1",
		"subject_id":   "user-1",
		"flow_id":      "flow-1",
		"flow_version": "not-an-int",
	}

	got, err := buildSessionFromRow(row)

	s.Error(err)
	s.Nil(got)
}

func (s *StoreTestSuite) TestGetByHandle_MultipleRows() {
	row := map[string]interface{}{"session_id": "sess-1"}
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionByHandle,
		"handle-abc", testDeploymentID).
		Return([]map[string]interface{}{row, row}, nil)

	got, err := s.store.GetByHandle(context.Background(), "handle-abc")

	s.Error(err)
	s.Nil(got)
	s.Contains(err.Error(), "unexpected number of results")
}

func (s *StoreTestSuite) TestGetByHandle_BuildError() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionByHandle,
		"handle-abc", testDeploymentID).
		Return([]map[string]interface{}{{"session_id": 42}}, nil) // non-string id fails buildSessionFromRow

	got, err := s.store.GetByHandle(context.Background(), "handle-abc")

	s.Error(err)
	s.Nil(got)
}

func (s *StoreTestSuite) TestGetByExecutionID_QueryError() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionByExecutionID,
		"exec-1", testDeploymentID).
		Return(nil, errors.New("query failed"))

	got, err := s.store.GetByExecutionID(context.Background(), "exec-1")

	s.Error(err)
	s.Nil(got)
}

func (s *StoreTestSuite) TestUpdate_DBError() {
	sess := s.sampleSession()
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryUpdateSession,
		sess.FlowVersion, sess.HandleID, sess.LastActiveAt, nil, sess.AbsoluteExpiresAt,
		string(sess.State), sess.SessionID, testDeploymentID, sess.Version).
		Return(int64(0), errors.New("db down"))

	err := s.store.Update(context.Background(), &sess)

	s.Error(err)
	s.Contains(err.Error(), "failed to update session")
}

func (s *StoreTestSuite) TestBuildSessionFromRow_BadRequiredFields() {
	base := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	valid := func() map[string]interface{} {
		return map[string]interface{}{
			"session_id": "sess-1", "subject_id": "user-1", "flow_id": "flow-1",
			"flow_version": int64(1), "flow_execution_id": "exec-1", "handle_id": "handle-abc",
			"authenticated_at": base, "created_at": base, "last_active_at": base,
			"absolute_expires_at": base, "state": "ACTIVE", "version": int64(1),
		}
	}
	// Sanity: a complete row builds cleanly.
	_, err := buildSessionFromRow(valid())
	s.Require().NoError(err)

	// Each required string field errors when non-string.
	for _, f := range []string{"session_id", "subject_id", "flow_id", "flow_execution_id", "handle_id"} {
		row := valid()
		row[f] = 42
		_, buildErr := buildSessionFromRow(row)
		s.Error(buildErr, "expected error for bad %s", f)
	}
	// Each required time field errors when non-time.
	for _, f := range []string{"authenticated_at", "created_at", "last_active_at"} {
		row := valid()
		row[f] = 42
		_, buildErr := buildSessionFromRow(row)
		s.Error(buildErr, "expected error for bad %s", f)
	}
	// Version errors when non-numeric.
	row := valid()
	row["version"] = "nope"
	_, err = buildSessionFromRow(row)
	s.Error(err)
}

func (s *StoreTestSuite) TestParseInt_Variants() {
	for _, v := range []interface{}{int(1), int32(1), int64(1), float64(1)} {
		got, err := parseInt(v, "n")
		s.NoError(err)
		s.Equal(1, got)
	}
	_, err := parseInt("nope", "n")
	s.Error(err)
}

func (s *StoreTestSuite) TestParseNullableTime_Variants() {
	s.True(parseNullableTime(nil).IsZero())
	s.True(parseNullableTime(42).IsZero()) // unparseable falls back to zero
	base := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	s.Equal(base, parseNullableTime(base))
}

func (s *StoreTestSuite) TestParseNullableString_Variants() {
	s.Equal("x", parseNullableString([]byte("x")))
	s.Empty(parseNullableString(nil))
}
