// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/database/model"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

type ParticipantStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *store
}

func TestParticipantStoreSuite(t *testing.T) {
	suite.Run(t, new(ParticipantStoreTestSuite))
}

func (s *ParticipantStoreTestSuite) SetupTest() {
	s.mockDBProvider = &providermock.DBProviderInterfaceMock{}
	s.mockDBClient = &providermock.DBClientInterfaceMock{}
	s.store = &store{
		dbProvider:   s.mockDBProvider,
		deploymentID: testDeploymentID,
	}
}

func (s *ParticipantStoreTestSuite) TestRecord_Upserts() {
	now := time.Unix(1700000000, 0).UTC()
	p := Participant{
		SessionID: "sess-1", AppID: "app-1", TokenFamilyID: "tfid-1", FirstJoinedAt: now, LastActiveAt: now,
	}

	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryUpsertParticipant,
		"sess-1", testDeploymentID, "app-1", now, now, "tfid-1").
		Return(int64(1), nil)

	err := s.store.Record(context.Background(), p)

	s.NoError(err)
	// DEPLOYMENT_ID is the second positional parameter, matching the SSO query convention.
	s.mockDBClient.AssertExpectations(s.T())
}

func (s *ParticipantStoreTestSuite) TestRecord_DBError() {
	now := time.Unix(1700000000, 0).UTC()
	p := Participant{
		SessionID: "sess-1", AppID: "app-1", TokenFamilyID: "tfid-1", FirstJoinedAt: now, LastActiveAt: now,
	}

	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryUpsertParticipant,
		"sess-1", testDeploymentID, "app-1", now, now, "tfid-1").
		Return(int64(0), errors.New("db down"))

	err := s.store.Record(context.Background(), p)

	s.Error(err)
	s.Contains(err.Error(), "failed to record session participant")
}

func (s *ParticipantStoreTestSuite) TestListBySessionID() {
	first := time.Unix(1700000000, 0).UTC()
	second := time.Unix(1700000100, 0).UTC()
	rows := []map[string]interface{}{
		{"session_id": "sess-1", "app_id": "app-1", "tfid": "tfid-1",
			"first_joined_at": first, "last_active_at": first},
		{"session_id": "sess-1", "app_id": "app-2", "first_joined_at": second, "last_active_at": second},
	}
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryListParticipantsBySessionID,
		"sess-1", testDeploymentID).
		Return(rows, nil)

	got, err := s.store.ListBySessionID(context.Background(), "sess-1")

	s.NoError(err)
	s.Require().Len(got, 2)
	s.Equal("app-1", got[0].AppID)
	s.Equal("app-2", got[1].AppID)
	s.Equal(first, got[0].FirstJoinedAt)
	s.Equal("tfid-1", got[0].TokenFamilyID)
	s.Empty(got[1].TokenFamilyID)
}

func (s *ParticipantStoreTestSuite) TestListBySessionID_Empty() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryListParticipantsBySessionID,
		"sess-1", testDeploymentID).
		Return([]map[string]interface{}{}, nil)

	got, err := s.store.ListBySessionID(context.Background(), "sess-1")

	s.NoError(err)
	s.Empty(got)
}

func (s *ParticipantStoreTestSuite) TestDeleteBySessionID() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryDeleteParticipantsBySessionID,
		"sess-1", testDeploymentID).
		Return(int64(2), nil)

	err := s.store.DeleteBySessionID(context.Background(), "sess-1")

	s.NoError(err)
	s.mockDBClient.AssertExpectations(s.T())
}

func (s *ParticipantStoreTestSuite) TestListBySessionID_ClientError() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(nil, errors.New("no client"))

	got, err := s.store.ListBySessionID(context.Background(), "sess-1")

	s.Error(err)
	s.Nil(got)
}

func (s *ParticipantStoreTestSuite) TestListBySessionID_QueryError() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryListParticipantsBySessionID,
		"sess-1", testDeploymentID).
		Return(nil, errors.New("query failed"))

	got, err := s.store.ListBySessionID(context.Background(), "sess-1")

	s.Error(err)
	s.Nil(got)
}

func (s *ParticipantStoreTestSuite) TestListBySessionID_BuildError() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryListParticipantsBySessionID,
		"sess-1", testDeploymentID).
		Return([]map[string]interface{}{{"session_id": 42}}, nil) // non-string id fails buildParticipantFromRow

	got, err := s.store.ListBySessionID(context.Background(), "sess-1")

	s.Error(err)
	s.Nil(got)
}

func (s *ParticipantStoreTestSuite) TestDeleteBySessionID_DBError() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryDeleteParticipantsBySessionID,
		"sess-1", testDeploymentID).
		Return(int64(0), errors.New("db down"))

	err := s.store.DeleteBySessionID(context.Background(), "sess-1")

	s.Error(err)
	s.Contains(err.Error(), "failed to delete session participants")
}

func (s *ParticipantStoreTestSuite) TestBuildParticipantFromRow_BadFields() {
	now := time.Unix(1700000000, 0).UTC()
	valid := func() map[string]interface{} {
		return map[string]interface{}{
			"session_id": "sess-1", "app_id": "app-1", "first_joined_at": now, "last_active_at": now,
		}
	}
	_, err := buildParticipantFromRow(valid())
	s.Require().NoError(err)

	for _, f := range []string{"session_id", "app_id"} {
		row := valid()
		row[f] = 42
		_, buildErr := buildParticipantFromRow(row)
		s.Error(buildErr, "expected error for bad %s", f)
	}
	for _, f := range []string{"first_joined_at", "last_active_at"} {
		row := valid()
		row[f] = 42
		_, buildErr := buildParticipantFromRow(row)
		s.Error(buildErr, "expected error for bad %s", f)
	}
}

// participantsBySessionIDsQuery matches the bulk participant query built for n session ids.
func participantsBySessionIDsQuery(n int) interface{} {
	return mock.MatchedBy(func(q model.DBQuery) bool {
		return q.ID == "SSO-SESS-17" &&
			strings.Contains(q.Query, "SESSION_ID IN ("+placeholderList(n)+")") &&
			strings.Contains(q.Query, "DEPLOYMENT_ID = $"+strconv.Itoa(n+1))
	})
}

func placeholderList(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "$" + strconv.Itoa(i+1)
	}
	return strings.Join(parts, ", ")
}

func (s *ParticipantStoreTestSuite) TestListBySessionIDs() {
	joined := time.Unix(1700000000, 0).UTC()
	rows := []map[string]interface{}{
		{"session_id": "sess-1", "app_id": "app-1", "tfid": "tfid-1",
			"first_joined_at": joined, "last_active_at": joined},
		{"session_id": "sess-2", "app_id": "app-1", "tfid": "tfid-2",
			"first_joined_at": joined, "last_active_at": joined},
	}
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), participantsBySessionIDsQuery(2),
		"sess-1", "sess-2", testDeploymentID).
		Return(rows, nil)

	got, err := s.store.ListBySessionIDs(context.Background(), []string{"sess-1", "sess-2"})

	s.NoError(err)
	s.Require().Len(got, 2)
	s.Equal("sess-1", got[0].SessionID)
	s.Equal("sess-2", got[1].SessionID)
	s.Equal("tfid-2", got[1].TokenFamilyID)
}

func (s *ParticipantStoreTestSuite) TestListBySessionIDs_EmptyInputSkipsDatabase() {
	got, err := s.store.ListBySessionIDs(context.Background(), nil)

	s.NoError(err)
	s.Nil(got)
	s.mockDBProvider.AssertNotCalled(s.T(), "GetRuntimePersistentDBClient")
}

func (s *ParticipantStoreTestSuite) TestListBySessionIDs_ChunksAtTheParameterCap() {
	total := participantsBySessionIDsChunkSize + 1
	ids := make([]string, total)
	for i := range ids {
		ids[i] = "sess-" + strconv.Itoa(i)
	}
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)

	// First call carries a full chunk of ids plus the deployment id.
	fullArgs := []interface{}{context.Background(), participantsBySessionIDsQuery(participantsBySessionIDsChunkSize)}
	for i := 0; i < participantsBySessionIDsChunkSize; i++ {
		fullArgs = append(fullArgs, ids[i])
	}
	fullArgs = append(fullArgs, testDeploymentID)
	earlier := time.Unix(1700000000, 0).UTC()
	later := earlier.Add(time.Minute)
	s.mockDBClient.On("QueryContext", fullArgs...).Return([]map[string]interface{}{
		{"session_id": ids[5], "app_id": "app-0", "first_joined_at": earlier, "last_active_at": earlier},
		{"session_id": ids[5], "app_id": "app-1", "first_joined_at": later, "last_active_at": later},
	}, nil).Once()
	// Second call carries the single remaining id.
	s.mockDBClient.On("QueryContext", context.Background(), participantsBySessionIDsQuery(1),
		ids[total-1], testDeploymentID).Return([]map[string]interface{}{
		{"session_id": ids[total-1], "app_id": "app-2", "first_joined_at": earlier, "last_active_at": earlier},
	}, nil).Once()

	original := append([]string(nil), ids...)
	got, err := s.store.ListBySessionIDs(context.Background(), ids)

	s.NoError(err)
	s.Require().Len(got, 3)
	s.Equal(ids[5], got[0].SessionID)
	s.Equal("app-0", got[0].AppID, "a session's participants keep their join order")
	s.Equal("app-1", got[1].AppID)
	s.Equal(ids[total-1], got[2].SessionID, "the remaining chunk's rows follow")
	s.Equal(original, ids, "the caller's slice is not reordered")
	s.mockDBClient.AssertExpectations(s.T())
}

func (s *ParticipantStoreTestSuite) TestListBySessionIDs_QueryError() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), participantsBySessionIDsQuery(1),
		"sess-1", testDeploymentID).Return(nil, errors.New("db down"))

	got, err := s.store.ListBySessionIDs(context.Background(), []string{"sess-1"})

	s.Error(err)
	s.Nil(got)
}
