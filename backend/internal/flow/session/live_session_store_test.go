// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"errors"
	"time"
)

func liveSessionRow(sessionID string, base time.Time) map[string]interface{} {
	return map[string]interface{}{
		"session_id": sessionID, "subject_id": "user-1", "flow_id": "flow-1",
		"flow_version": int64(2), "flow_execution_id": "exec-" + sessionID, "handle_id": "handle-" + sessionID,
		"authenticated_at": base, "created_at": base, "last_active_at": base,
		"idle_expires_at": base.Add(30 * time.Minute), "absolute_expires_at": base.Add(8 * time.Hour),
		"properties": `{"userAgent":"Mozilla/5.0","lastActiveIp":"203.0.113.10"}`,
		"state":      "ACTIVE", "version": int64(1),
	}
}

func (s *StoreTestSuite) TestListLiveBySubject() {
	base := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	now := base.Add(time.Minute)
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryListLiveSessionsBySubject,
		"user-1", now, 30, 10, testDeploymentID).
		Return([]map[string]interface{}{liveSessionRow("sess-2", base), liveSessionRow("sess-1", base)}, nil)

	got, err := s.store.ListLiveBySubject(context.Background(), "user-1", now, 30, 10)

	s.NoError(err)
	s.Require().Len(got, 2)
	s.Equal("sess-2", got[0].SessionID, "the query's order is kept")
	s.Equal("sess-1", got[1].SessionID)
	s.mockDBClient.AssertExpectations(s.T())
}

func (s *StoreTestSuite) TestListLiveByApp() {
	base := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryListLiveSessionsByApp,
		"app-1", base, 5, 0, testDeploymentID).Return([]map[string]interface{}{}, nil)

	got, err := s.store.ListLiveByApp(context.Background(), "app-1", base, 5, 0)

	s.NoError(err)
	s.NotNil(got, "an empty page is an empty slice, not nil")
	s.Empty(got)
}

func (s *StoreTestSuite) TestListLive_QueryError() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryListLiveSessionsBySubject,
		"user-1", time.Time{}, 30, 0, testDeploymentID).Return(nil, errors.New("db down"))

	got, err := s.store.ListLiveBySubject(context.Background(), "user-1", time.Time{}, 30, 0)

	s.ErrorContains(err, "failed to list live sessions")
	s.Nil(got)
}

func (s *StoreTestSuite) TestListLive_BuildError() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryListLiveSessionsByApp,
		"app-1", time.Time{}, 30, 0, testDeploymentID).
		Return([]map[string]interface{}{{"session_id": 42}}, nil)

	got, err := s.store.ListLiveByApp(context.Background(), "app-1", time.Time{}, 30, 0)

	s.Error(err)
	s.Nil(got)
}

func (s *StoreTestSuite) TestCountLiveBySubject() {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryCountLiveSessionsBySubject,
		"user-1", now, testDeploymentID).Return([]map[string]interface{}{{"total": int64(3)}}, nil)

	got, err := s.store.CountLiveBySubject(context.Background(), "user-1", now)

	s.NoError(err)
	s.Equal(3, got)
}

func (s *StoreTestSuite) TestCountLiveByApp() {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryCountLiveSessionsByApp,
		"app-1", now, testDeploymentID).Return([]map[string]interface{}{{"total": int64(0)}}, nil)

	got, err := s.store.CountLiveByApp(context.Background(), "app-1", now)

	s.NoError(err)
	s.Equal(0, got)
}

func (s *StoreTestSuite) TestCountLive_Errors() {
	cases := []struct {
		name    string
		rows    []map[string]interface{}
		err     error
		wantErr string
	}{
		{"query error", nil, errors.New("db down"), "failed to count live sessions"},
		{"no rows", []map[string]interface{}{}, nil, "unexpected number of results"},
		{"bad total", []map[string]interface{}{{"total": "three"}}, nil, "failed to parse total"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
			s.mockDBClient.On("QueryContext", context.Background(), queryCountLiveSessionsBySubject,
				"user-1", time.Time{}, testDeploymentID).Return(tc.rows, tc.err)

			got, err := s.store.CountLiveBySubject(context.Background(), "user-1", time.Time{})

			s.ErrorContains(err, tc.wantErr)
			s.Zero(got)
		})
	}
}
