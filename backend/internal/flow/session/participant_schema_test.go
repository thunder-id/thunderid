// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"

	"github.com/thunder-id/thunderid/internal/system/database/model"
)

// ParticipantSchemaTestSuite runs the participant queries against the runtime persistent SQLite schema, so
// the table's key is under test and not only the arguments the store passes.
type ParticipantSchemaTestSuite struct {
	suite.Suite
	db *sql.DB
}

func TestParticipantSchemaSuite(t *testing.T) {
	suite.Run(t, new(ParticipantSchemaTestSuite))
}

func (s *ParticipantSchemaTestSuite) SetupTest() {
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
}

// record runs the participant upsert for app-1 in sess-1.
func (s *ParticipantSchemaTestSuite) record(ouID, tokenFamilyID string, at time.Time) {
	_, err := s.db.Exec(queryUpsertParticipant.Query,
		"sess-1", testDeploymentID, "app-1", ouID, at, at, tokenFamilyID)
	s.Require().NoError(err)
}

// list runs a participant listing and maps its rows as the store does.
func (s *ParticipantSchemaTestSuite) list(query model.DBQuery, args ...interface{}) []Participant {
	rows, err := s.db.Query(query.Query, args...)
	s.Require().NoError(err)
	defer func() { s.NoError(rows.Close()) }()
	columns, err := rows.Columns()
	s.Require().NoError(err)

	var result []Participant
	for rows.Next() {
		values := make([]interface{}, len(columns))
		pointers := make([]interface{}, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		s.Require().NoError(rows.Scan(pointers...))
		row := map[string]interface{}{}
		for i, column := range columns {
			row[strings.ToLower(column)] = values[i]
		}
		p, err := buildParticipantFromRow(row)
		s.Require().NoError(err)
		result = append(result, p)
	}
	s.Require().NoError(rows.Err())
	return result
}

func (s *ParticipantSchemaTestSuite) TestAnApplicationUnderTwoOUsIsTwoParticipants() {
	joined := time.Unix(1700000000, 0).UTC()
	s.record("ou-x", "tfid-x", joined)
	s.record("ou-y", "tfid-y", joined.Add(time.Minute))

	got := s.list(queryListParticipantsBySessionID, "sess-1", testDeploymentID)

	s.Require().Len(got, 2)
	s.Equal("app-1", got[0].AppID)
	s.Equal("ou-x", got[0].OUID)
	s.Equal("tfid-x", got[0].TokenFamilyID)
	s.Equal("app-1", got[1].AppID)
	s.Equal("ou-y", got[1].OUID)
	s.Equal("tfid-y", got[1].TokenFamilyID)
}

func (s *ParticipantSchemaTestSuite) TestRejoiningUnderTheSameOURefreshesItsRow() {
	joined := time.Unix(1700000000, 0).UTC()
	rejoined := joined.Add(time.Hour)
	s.record("ou-x", "tfid-1", joined)
	s.record("ou-x", "tfid-2", rejoined)

	got := s.list(queryListParticipantsBySessionID, "sess-1", testDeploymentID)

	s.Require().Len(got, 1)
	s.Equal("tfid-2", got[0].TokenFamilyID, "the latest grant's family replaces the earlier one")
	s.True(got[0].FirstJoinedAt.Equal(joined), "the first join time is kept")
	s.True(got[0].LastActiveAt.Equal(rejoined))
}

func (s *ParticipantSchemaTestSuite) TestASignInWithNoOUIsItsOwnParticipant() {
	joined := time.Unix(1700000000, 0).UTC()
	s.record("", "tfid-none", joined)
	s.record("ou-x", "tfid-x", joined.Add(time.Minute))

	got := s.list(queryListParticipantsByAppID, "app-1", testDeploymentID)

	s.Require().Len(got, 2)
	s.Empty(got[0].OUID)
	s.Equal("tfid-none", got[0].TokenFamilyID)
	s.Equal("ou-x", got[1].OUID)
}

func (s *ParticipantSchemaTestSuite) TestDeleteParticipantRemovesOneOU() {
	joined := time.Unix(1700000000, 0).UTC()
	s.record("ou-x", "tfid-x", joined)
	s.record("ou-y", "tfid-y", joined.Add(time.Minute))

	_, err := s.db.Exec(queryDeleteParticipant.Query, "sess-1", "app-1", "ou-x", testDeploymentID)
	s.Require().NoError(err)

	got := s.list(queryListParticipantsBySessionID, "sess-1", testDeploymentID)
	s.Require().Len(got, 1)
	s.Equal("ou-y", got[0].OUID)
}
