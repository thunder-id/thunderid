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
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

type SessionContextStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *store
}

func TestSessionContextStoreTestSuite(t *testing.T) {
	suite.Run(t, new(SessionContextStoreTestSuite))
}

func (s *SessionContextStoreTestSuite) SetupTest() {
	s.mockDBProvider = &providermock.DBProviderInterfaceMock{}
	s.mockDBClient = &providermock.DBClientInterfaceMock{}
	s.store = &store{
		dbProvider:   s.mockDBProvider,
		deploymentID: testDeploymentID,
	}
}

func sampleSessionContext() SessionContext {
	return SessionContext{
		SessionID:      "sess-1",
		CheckpointID:   "session",
		RuntimeData:    map[string]string{"email": "alice@example.com"},
		AuthUser:       json.RawMessage(`{"entityReference":{"entityId":"user-1"}}`),
		CompletedSteps: map[string]StepFact{"basic_auth": {Executor: "BasicAuthExecutor", Status: "COMPLETE"}},
		ContextVersion: 1,
	}
}

func (s *SessionContextStoreTestSuite) TestCreate_Persists() {
	c := sampleSessionContext()
	payload, err := c.serializePayload()
	s.Require().NoError(err)

	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryCreateSessionContext,
		c.SessionID, testDeploymentID, c.CheckpointID, payload, c.ContextVersion).
		Return(int64(1), nil)

	createErr := s.store.CreateContext(context.Background(), c)

	s.NoError(createErr)
	s.mockDBClient.AssertExpectations(s.T())
}

func (s *SessionContextStoreTestSuite) TestCreate_TooLarge() {
	c := sampleSessionContext()
	c.RuntimeData = map[string]string{"email": strings.Repeat("a", MaxSessionContextBytes+1)}

	createErr := s.store.CreateContext(context.Background(), c)

	s.ErrorIs(createErr, errSessionContextTooLarge)
	// Oversized payloads are rejected before any DB call.
	s.mockDBProvider.AssertNotCalled(s.T(), "GetRuntimePersistentDBClient")
}

func (s *SessionContextStoreTestSuite) TestCreate_DBError() {
	c := sampleSessionContext()
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryCreateSessionContext,
		c.SessionID, testDeploymentID, c.CheckpointID, mustSerialize(s.T(), c), c.ContextVersion).
		Return(int64(0), errors.New("db down"))

	createErr := s.store.CreateContext(context.Background(), c)

	s.Error(createErr)
	s.Contains(createErr.Error(), "failed to create session context")
}

func (s *SessionContextStoreTestSuite) TestGetByCheckpoint_Hit() {
	c := sampleSessionContext()
	payload, err := c.serializePayload()
	s.Require().NoError(err)

	row := map[string]interface{}{
		"session_id":      "sess-1",
		"checkpoint_id":   "session",
		"context":         payload,
		"context_version": int64(1),
	}
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionContextByCheckpoint,
		"sess-1", testDeploymentID, "session").
		Return([]map[string]interface{}{row}, nil)

	got, getErr := s.store.GetByCheckpoint(context.Background(), "sess-1", "session")

	s.NoError(getErr)
	s.Require().NotNil(got)
	s.Equal("sess-1", got.SessionID)
	s.Equal("session", got.CheckpointID)
	s.Equal(1, got.ContextVersion)
	s.Equal("alice@example.com", got.RuntimeData["email"])
	s.JSONEq(`{"entityReference":{"entityId":"user-1"}}`, string(got.AuthUser))
	s.Equal("BasicAuthExecutor", got.CompletedSteps["basic_auth"].Executor)
}

func (s *SessionContextStoreTestSuite) TestGetByCheckpoint_Miss() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionContextByCheckpoint,
		"sess-1", testDeploymentID, "missing").
		Return([]map[string]interface{}{}, nil)

	got, getErr := s.store.GetByCheckpoint(context.Background(), "sess-1", "missing")

	s.NoError(getErr)
	s.Nil(got)
}

func (s *SessionContextStoreTestSuite) TestListCheckpointIDs() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryListCheckpointsBySessionID,
		"sess-1", testDeploymentID).
		Return([]map[string]interface{}{
			{"checkpoint_id": "password"},
			{"checkpoint_id": "step_up"},
		}, nil)

	ids, listErr := s.store.ListCheckpointIDs(context.Background(), "sess-1")

	s.NoError(listErr)
	s.Equal([]string{"password", "step_up"}, ids)
}

func (s *SessionContextStoreTestSuite) TestDelete() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryDeleteSessionContext,
		"sess-1", testDeploymentID).
		Return(int64(1), nil)

	delErr := s.store.Delete(context.Background(), "sess-1")

	s.NoError(delErr)
	s.mockDBClient.AssertExpectations(s.T())
}

func (s *SessionContextStoreTestSuite) TestGetByCheckpoint_QueryError() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionContextByCheckpoint,
		"sess-1", testDeploymentID, "session").
		Return(nil, errors.New("query failed"))

	got, err := s.store.GetByCheckpoint(context.Background(), "sess-1", "session")
	s.Error(err)
	s.Nil(got)
}

func (s *SessionContextStoreTestSuite) TestGetByCheckpoint_MultipleRows() {
	row := map[string]interface{}{
		"session_id": "sess-1", "checkpoint_id": "session",
		"context": "{}", "context_version": int64(1),
	}
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionContextByCheckpoint,
		"sess-1", testDeploymentID, "session").
		Return([]map[string]interface{}{row, row}, nil)

	got, err := s.store.GetByCheckpoint(context.Background(), "sess-1", "session")
	s.Error(err)
	s.Nil(got)
	s.Contains(err.Error(), "unexpected number of results")
}

func (s *SessionContextStoreTestSuite) TestGetByCheckpoint_BuildError() {
	row := map[string]interface{}{"session_id": 42, "checkpoint_id": "session", "context_version": int64(1)}
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionContextByCheckpoint,
		"sess-1", testDeploymentID, "session").
		Return([]map[string]interface{}{row}, nil)

	got, err := s.store.GetByCheckpoint(context.Background(), "sess-1", "session")
	s.Error(err)
	s.Nil(got)
}

func (s *SessionContextStoreTestSuite) TestGetByCheckpoint_BadContextVersion() {
	row := map[string]interface{}{
		"session_id": "sess-1", "checkpoint_id": "session",
		"context": "{}", "context_version": "nope",
	}
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionContextByCheckpoint,
		"sess-1", testDeploymentID, "session").
		Return([]map[string]interface{}{row}, nil)

	got, err := s.store.GetByCheckpoint(context.Background(), "sess-1", "session")
	s.Error(err)
	s.Nil(got)
}

func (s *SessionContextStoreTestSuite) TestGetByCheckpoint_BadPayload() {
	// A context column that is not valid JSON fails to parse.
	row := map[string]interface{}{
		"session_id": "sess-1", "checkpoint_id": "session",
		"context": "not-json", "context_version": int64(1),
	}
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryGetSessionContextByCheckpoint,
		"sess-1", testDeploymentID, "session").
		Return([]map[string]interface{}{row}, nil)

	got, err := s.store.GetByCheckpoint(context.Background(), "sess-1", "session")
	s.Error(err)
	s.Nil(got)
}

func (s *SessionContextStoreTestSuite) TestListCheckpointIDs_QueryError() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryListCheckpointsBySessionID,
		"sess-1", testDeploymentID).
		Return(nil, errors.New("query failed"))

	got, err := s.store.ListCheckpointIDs(context.Background(), "sess-1")
	s.Error(err)
	s.Nil(got)
}

func (s *SessionContextStoreTestSuite) TestListCheckpointIDs_ParseError() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", context.Background(), queryListCheckpointsBySessionID,
		"sess-1", testDeploymentID).
		Return([]map[string]interface{}{{"checkpoint_id": 42}}, nil) // non-string fails parseString

	got, err := s.store.ListCheckpointIDs(context.Background(), "sess-1")
	s.Error(err)
	s.Nil(got)
}

func (s *SessionContextStoreTestSuite) TestDelete_DBError() {
	s.mockDBProvider.On("GetRuntimePersistentDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", context.Background(), queryDeleteSessionContext,
		"sess-1", testDeploymentID).
		Return(int64(0), errors.New("db down"))

	err := s.store.Delete(context.Background(), "sess-1")
	s.Error(err)
	s.Contains(err.Error(), "failed to delete session context")
}

func mustSerialize(t *testing.T, c SessionContext) string {
	t.Helper()
	payload, err := c.serializePayload()
	if err != nil {
		t.Fatalf("serialize payload: %v", err)
	}
	return payload
}
