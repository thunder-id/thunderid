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
