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
