// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"
)

// SessionSchemaTestSuite runs session writes against the runtime persistent SQLite schema, so the table's
// constraints are under test and not only the arguments the store passes.
type SessionSchemaTestSuite struct {
	suite.Suite
	db  *sql.DB
	now time.Time
}

func TestSessionSchemaSuite(t *testing.T) {
	suite.Run(t, new(SessionSchemaTestSuite))
}

func (s *SessionSchemaTestSuite) SetupTest() {
	schema, err := os.ReadFile(filepath.Join("..", "..", "..", "dbscripts", "runtime_persistent", "sqlite.sql"))
	s.Require().NoError(err)
	db, err := sql.Open("sqlite", ":memory:")
	s.Require().NoError(err)
	// Every connection to :memory: is a database of its own.
	db.SetMaxOpenConns(1)
	s.T().Cleanup(func() { s.NoError(db.Close()) })
	_, err = db.Exec(string(schema))
	s.Require().NoError(err)
	s.db = db
	s.now = time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
}

// createSSOSession runs the store's session insert with the given deadlines.
func (s *SessionSchemaTestSuite) createSSOSession(sessionID string, idle, absolute interface{}) error {
	_, err := s.db.Exec(queryCreateSession.Query,
		sessionID, testDeploymentID, "user-1", "flow-1", 1, "exec-"+sessionID, "handle-"+sessionID,
		s.now, s.now, s.now, idle, absolute, nil, string(StateActive), 1)
	return err
}

// insertSession writes a session row of the given type, with or without its handle and flow columns.
func (s *SessionSchemaTestSuite) insertSession(sessionType string, withHandleAndFlow bool) error {
	var flowID, flowVersion, executionID, handleID interface{}
	if withHandleAndFlow {
		flowID, flowVersion, executionID, handleID = "flow-1", 1, "exec-1", "handle-1"
	}
	_, err := s.db.Exec(`INSERT INTO "SESSION" (SESSION_ID, DEPLOYMENT_ID, TYPE, SUBJECT_ID, FLOW_ID, `+
		`FLOW_VERSION, FLOW_EXECUTION_ID, HANDLE_ID, AUTHENTICATED_AT, LAST_ACTIVE_AT, IDLE_EXPIRES_AT, `+
		`ABSOLUTE_EXPIRES_AT, STATE, VERSION) VALUES ('sess-1', $1, $2, 'user-1', $3, $4, $5, $6, $7, $7, $8, $9, `+
		`'ACTIVE', 1)`,
		testDeploymentID, sessionType, flowID, flowVersion, executionID, handleID, s.now,
		s.now.Add(2*time.Hour), s.now.Add(24*time.Hour))
	return err
}

func (s *SessionSchemaTestSuite) TestASessionNeedsBothDeadlines() {
	s.Error(s.createSSOSession("no-idle", nil, s.now.Add(24*time.Hour)))
	s.Error(s.createSSOSession("no-absolute", s.now.Add(2*time.Hour), nil))
	s.NoError(s.createSSOSession("both", s.now.Add(2*time.Hour), s.now.Add(24*time.Hour)))
}

func (s *SessionSchemaTestSuite) TestAnSSOSessionNeedsItsHandleAndFlow() {
	s.Error(s.insertSession("SSO", false))
	s.NoError(s.insertSession("SSO", true))
}

func (s *SessionSchemaTestSuite) TestADirectSessionNeedsNoHandleOrFlow() {
	s.NoError(s.insertSession("DIRECT", false))
}

func (s *SessionSchemaTestSuite) TestAParticipantNeedsATokenFamily() {
	_, err := s.db.Exec(queryUpsertParticipant.Query,
		"sess-1", testDeploymentID, "app-1", "", s.now, s.now, nil)
	s.Error(err)
}

func (s *SessionSchemaTestSuite) TestTheIdleSlideMovesOnlyALiveSession() {
	s.Require().NoError(s.createSSOSession("live", s.now.Add(time.Hour), s.now.Add(24*time.Hour)))
	s.Require().NoError(s.createSSOSession("idle", s.now.Add(-time.Minute), s.now.Add(24*time.Hour)))
	s.Require().NoError(s.createSSOSession("ended", s.now.Add(time.Hour), s.now.Add(-time.Minute)))

	slide := func(sessionID string) int64 {
		result, err := s.db.Exec(querySlideIdleDeadline.Query,
			s.now, s.now.Add(2*time.Hour), sessionID, string(StateActive), s.now, testDeploymentID)
		s.Require().NoError(err)
		moved, err := result.RowsAffected()
		s.Require().NoError(err)
		return moved
	}

	s.EqualValues(1, slide("live"))
	s.EqualValues(0, slide("idle"), "an idle-expired session must not be revived")
	s.EqualValues(0, slide("ended"), "a session past its absolute deadline must not be revived")
}
