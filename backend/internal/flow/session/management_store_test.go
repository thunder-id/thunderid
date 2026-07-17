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
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// ManagementStoreTestSuite exercises ListBySubject/CountBySubject/ListByApp/CountByApp against a
// real, in-memory SQLite operation database rather than a mocked DB client. Liveness filtering,
// ordering, and pagination live in the SQL query text itself, so a mocked client (which just
// returns canned rows) cannot verify them; only executing the query proves it.
type ManagementStoreTestSuite struct {
	suite.Suite
	store *store
}

func TestManagementStoreTestSuite(t *testing.T) {
	suite.Run(t, new(ManagementStoreTestSuite))
}

// SetupSuite initializes a shared in-memory SQLite operation database (via the real DBProvider
// singleton, not a mock) and ensures the SSO_SESSION and SSO_SESSION_PARTICIPANT tables exist.
// Setup is suite-scoped, not per-test: the DBProvider caches the first SQLite client for the life
// of the process. The database is purely in-memory — no on-disk directory is involved, so the
// cached client never outlives its backing store. The single pooled connection
// (MaxOpenConns/MaxIdleConns 1) keeps the shared-cache in-memory database alive across the suite.
// ServerHome must stay "" because the provider joins it onto the SQLite path, which would turn the
// in-memory DSN into a file path. Disjoint subject, session, and app ids across test methods keep
// their seeded rows from interfering with each other.
func (s *ManagementStoreTestSuite) SetupSuite() {
	inMemoryDataSource := func() config.DataSource {
		return config.DataSource{
			Type: "sqlite",
			SQLite: config.SQLiteDataSource{
				Path:         "file::memory:?cache=shared",
				MaxOpenConns: 1,
				MaxIdleConns: 1,
			},
		}
	}
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			Config:            inMemoryDataSource(),
			RuntimeTransient:  inMemoryDataSource(),
			Entity:            inMemoryDataSource(),
			RuntimePersistent: inMemoryDataSource(),
		},
	}
	s.Require().NoError(config.InitializeServerRuntime("", testConfig))

	dbProvider := provider.GetDBProvider()
	dbClient, err := dbProvider.GetRuntimePersistentDBClient()
	s.Require().NoError(err)

	_, err = dbClient.ExecuteContext(context.Background(), dbmodel.DBQuery{
		ID: "TEST-CREATE-SSO-SESSION-TABLE",
		Query: `CREATE TABLE IF NOT EXISTS "SSO_SESSION" (
			SESSION_ID VARCHAR(36) NOT NULL,
			DEPLOYMENT_ID VARCHAR(255) NOT NULL,
			SUBJECT_ID VARCHAR(36) NOT NULL,
			FLOW_ID VARCHAR(36) NOT NULL,
			FLOW_VERSION INTEGER NOT NULL,
			FLOW_EXECUTION_ID VARCHAR(255) NOT NULL,
			HANDLE_ID VARCHAR(255) NOT NULL,
			AUTHENTICATED_AT DATETIME NOT NULL,
			CREATED_AT TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			LAST_ACTIVE_AT DATETIME NOT NULL,
			IDLE_EXPIRES_AT DATETIME,
			ABSOLUTE_EXPIRES_AT DATETIME,
			STATE VARCHAR(50) NOT NULL,
			VERSION INTEGER NOT NULL,
			UPDATED_AT TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (SESSION_ID, DEPLOYMENT_ID)
		)`,
	})
	s.Require().NoError(err)

	_, err = dbClient.ExecuteContext(context.Background(), dbmodel.DBQuery{
		ID: "TEST-CREATE-SSO-SESSION-FLOW-EXEC-IDX",
		Query: `CREATE UNIQUE INDEX IF NOT EXISTS idx_sso_session_flow_execution ` +
			`ON "SSO_SESSION" (FLOW_EXECUTION_ID, DEPLOYMENT_ID)`,
	})
	s.Require().NoError(err)

	_, err = dbClient.ExecuteContext(context.Background(), dbmodel.DBQuery{
		ID: "TEST-CREATE-SSO-SESSION-PARTICIPANT-TABLE",
		Query: `CREATE TABLE IF NOT EXISTS "SSO_SESSION_PARTICIPANT" (
			SESSION_ID VARCHAR(36) NOT NULL,
			DEPLOYMENT_ID VARCHAR(255) NOT NULL,
			APP_ID VARCHAR(36) NOT NULL,
			FIRST_JOINED_AT DATETIME NOT NULL,
			LAST_ACTIVE_AT DATETIME NOT NULL,
			PRIMARY KEY (SESSION_ID, DEPLOYMENT_ID, APP_ID)
		)`,
	})
	s.Require().NoError(err)

	s.store = newStore(dbProvider, testDeploymentID)
}

func (s *ManagementStoreTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
}

// createSession seeds a session row via st.Create so the new queries are exercised against rows
// written through the same path production code uses.
func (s *ManagementStoreTestSuite) createSession(sessionID, subjectID, handleID, flowExecutionID string,
	state State, lastActiveAt, idleExpiresAt, absoluteExpiresAt time.Time) {
	sess := Session{
		SessionID:         sessionID,
		SubjectID:         subjectID,
		FlowID:            "flow-1",
		FlowVersion:       1,
		FlowExecutionID:   flowExecutionID,
		HandleID:          handleID,
		AuthenticatedAt:   lastActiveAt,
		CreatedAt:         lastActiveAt,
		LastActiveAt:      lastActiveAt,
		IdleExpiresAt:     idleExpiresAt,
		AbsoluteExpiresAt: absoluteExpiresAt,
		State:             state,
		Version:           1,
	}
	s.Require().NoError(s.store.Create(context.Background(), sess))
}

func (s *ManagementStoreTestSuite) TestListBySubjectFiltersLiveness() {
	now := time.Now().UTC()
	future := now.Add(time.Hour)
	farFuture := now.Add(8 * time.Hour)
	past := now.Add(-time.Minute)

	// live: state ACTIVE, idle/absolute deadlines in the future.
	s.createSession("live", "sub-1", "handle-live", "exec-live", StateActive, now, future, farFuture)
	// idleExpired: ACTIVE but IdleExpiresAt in the past.
	s.createSession("idle-expired", "sub-1", "handle-idle-expired", "exec-idle-expired",
		StateActive, now, past, farFuture)
	// absoluteExpired: ACTIVE but AbsoluteExpiresAt in the past.
	s.createSession("absolute-expired", "sub-1", "handle-absolute-expired", "exec-absolute-expired",
		StateActive, now, future, past)
	// ended: state ENDED, deadlines in the future.
	s.createSession("ended", "sub-1", "handle-ended", "exec-ended", StateEnded, now, future, farFuture)
	// plus one live session for subject "sub-2" (must not appear).
	s.createSession("other-subject-live", "sub-2", "handle-other-subject", "exec-other-subject",
		StateActive, now, future, farFuture)

	sessions, err := s.store.ListBySubject(context.Background(), "sub-1", now, 10, 0)
	s.Require().NoError(err)
	s.Require().Len(sessions, 1)
	s.Equal("live", sessions[0].SessionID)

	count, err := s.store.CountBySubject(context.Background(), "sub-1", now)
	s.Require().NoError(err)
	s.Equal(1, count)
}

func (s *ManagementStoreTestSuite) TestListBySubjectOrdersAndPaginates() {
	now := time.Now().UTC()
	farFuture := now.Add(time.Hour)

	s.createSession("page-1", "sub-3", "handle-page-1", "exec-page-1",
		StateActive, now.Add(-3*time.Minute), farFuture, farFuture)
	s.createSession("page-2", "sub-3", "handle-page-2", "exec-page-2",
		StateActive, now.Add(-2*time.Minute), farFuture, farFuture)
	s.createSession("page-3", "sub-3", "handle-page-3", "exec-page-3",
		StateActive, now.Add(-1*time.Minute), farFuture, farFuture)

	page, err := s.store.ListBySubject(context.Background(), "sub-3", now, 2, 0)
	s.Require().NoError(err)
	s.Require().Len(page, 2)
	// newest first.
	s.True(page[0].LastActiveAt.After(page[1].LastActiveAt))

	rest, err := s.store.ListBySubject(context.Background(), "sub-3", now, 2, 2)
	s.Require().NoError(err)
	s.Require().Len(rest, 1)
}

func (s *ManagementStoreTestSuite) TestListBySubjectNilDeadlinesNeverExpire() {
	now := time.Now().UTC()

	// zero time.Time is stored as NULL via nullableTime.
	s.createSession("nil-deadlines", "sub-4", "handle-nil-deadlines", "exec-nil-deadlines",
		StateActive, now, time.Time{}, time.Time{})

	sessions, err := s.store.ListBySubject(context.Background(), "sub-4", now, 10, 0)
	s.Require().NoError(err)
	s.Len(sessions, 1)
}

// recordParticipant seeds a participant row via st.Record so the new queries are exercised
// against rows written through the same path production code uses.
func (s *ManagementStoreTestSuite) recordParticipant(sessionID, appID string, at time.Time) {
	s.Require().NoError(s.store.Record(context.Background(), Participant{
		SessionID:     sessionID,
		AppID:         appID,
		FirstJoinedAt: at,
		LastActiveAt:  at,
	}))
}

func (s *ManagementStoreTestSuite) TestListByAppReturnsSessionsTheAppJoined() {
	now := time.Now().UTC()
	farFuture := now.Add(time.Hour)
	past := now.Add(-time.Minute)

	// live session A (subject sub-app-a) with participants app-1 and app-2.
	s.createSession("session-app-a", "sub-app-a", "handle-app-a", "exec-app-a",
		StateActive, now, farFuture, farFuture)
	s.recordParticipant("session-app-a", "app-1", now)
	s.recordParticipant("session-app-a", "app-2", now)

	// live session B (subject sub-app-b) with participant app-2 only.
	s.createSession("session-app-b", "sub-app-b", "handle-app-b", "exec-app-b",
		StateActive, now, farFuture, farFuture)
	s.recordParticipant("session-app-b", "app-2", now)

	// expired session C with participant app-1.
	s.createSession("session-app-c", "sub-app-c", "handle-app-c", "exec-app-c",
		StateActive, now, past, farFuture)
	s.recordParticipant("session-app-c", "app-1", now)

	forApp1, err := s.store.ListByApp(context.Background(), "app-1", now, 10, 0)
	s.Require().NoError(err)
	s.Require().Len(forApp1, 1) // only session A; C is expired

	forApp2, err := s.store.ListByApp(context.Background(), "app-2", now, 10, 0)
	s.Require().NoError(err)
	s.Require().Len(forApp2, 2) // A and B

	count, err := s.store.CountByApp(context.Background(), "app-2", now)
	s.Require().NoError(err)
	s.Equal(2, count)
}
